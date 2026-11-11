package connector

import (
	"context"
	"errors"
	"maps"
	"time"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var _ bridgev2.ChatViewingNetworkAPI = (*LinkedInClient)(nil)
var _ bridgev2.UserBlockingNetworkAPI = (*LinkedInClient)(nil)

func (l *LinkedInClient) HandleMatrixBlockUser(ctx context.Context, msg *bridgev2.MatrixBlockUser) error {
	if msg.Portal.RoomType != database.RoomTypeDM {
		return errors.New("blocking is only supported in DMs")
	} else if msg.Portal.OtherUserID == l.userID {
		return errors.New("cannot block yourself")
	} else if msg.Content.ReportSpam {
		return bridgev2.WrapErrorInStatus(errors.New("spam reporting is not supported")).
			WithIsCertain(true).WithErrorAsMessage().WithErrorReason(event.MessageStatusUnsupported).WithStatus(event.MessageStatusFail)
	} else if !msg.Content.Block {
		return bridgev2.WrapErrorInStatus(errors.New("unblock this member in LinkedIn settings: https://www.linkedin.com/mypreferences/d/manage-by-blocked-list")).
			WithIsCertain(true).WithErrorAsMessage().WithErrorReason(event.MessageStatusUnsupported).WithStatus(event.MessageStatusFail)
	} else if !l.IsLoggedIn() {
		return errors.New("not logged in")
	}
	l.blockSyncLock.Lock()
	defer l.blockSyncLock.Unlock()
	l.blockedUsers = nil
	l.blockSyncTime = time.Time{}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return withoutNotice(wrapLinkedInError(l.client.BlockUser(ctx, linkedingo.NewURN(msg.Portal.OtherUserID).AsFsdProfile(), msg.Portal.Name)))
}

func withoutNotice(err error) error {
	var ms bridgev2.MessageStatus
	if errors.As(err, &ms) {
		return ms.WithSendNotice(false)
	}
	return err
}

func (l *LinkedInClient) HandleMatrixViewingChat(ctx context.Context, chat *bridgev2.MatrixViewingChat) error {
	l.resyncUserBlock(ctx, chat.Portal)
	return nil
}

func (l *LinkedInClient) resyncUserBlock(ctx context.Context, portal *bridgev2.Portal) {
	if portal == nil || portal.RoomType != database.RoomTypeDM || portal.Receiver == "" || portal.Receiver != l.userLogin.ID || portal.OtherUserID == "" || portal.OtherUserID == l.userID || !l.IsLoggedIn() {
		return
	}
	go l.syncUserBlock(context.WithoutCancel(ctx), portal)
}

func (l *LinkedInClient) getBlockedUsers(ctx context.Context, maxAge time.Duration) map[networkid.UserID]bool {
	l.blockSyncLock.Lock()
	defer l.blockSyncLock.Unlock()
	if time.Since(l.blockSyncTime) >= maxAge {
		l.blockSyncTime = time.Now()
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		users, err := l.client.GetBlockedUsers(ctx)
		if err != nil {
			l.blockedUsers = nil
			l.userLogin.Log.Warn().Err(err).Msg("Failed to fetch blocked users")
			return nil
		}
		l.blockedUsers = make(map[networkid.UserID]bool, len(users))
		for _, user := range users {
			l.blockedUsers[networkid.UserID(user.ID())] = true
		}
	}
	return maps.Clone(l.blockedUsers)
}

func (l *LinkedInClient) syncUserBlock(ctx context.Context, portal *bridgev2.Portal) {
	if l.getBlockedUsers(ctx, time.Hour) == nil {
		return
	}
	l.queueBlockInfoChange(portal.PortalKey, portal.OtherUserID)
}

func (l *LinkedInClient) syncBlockedUsers(ctx context.Context) {
	blocked := l.getBlockedUsers(ctx, 5*time.Minute)
	if blocked == nil {
		return
	}
	log := l.userLogin.Log.With().Str("action", "sync blocked users").Logger()
	portals := l.main.Bridge.DB.Portal
	targets := make(map[networkid.PortalKey]networkid.UserID)
	blockedDMs, err := portals.GetAllBlockedDMsOf(ctx, l.userLogin.ID)
	if err != nil {
		log.Err(err).Msg("Failed to get blocked DMs")
		return
	}
	for _, key := range blockedDMs {
		portal, err := portals.GetByKey(ctx, key)
		if err != nil {
			log.Err(err).Msg("Failed to get blocked DM")
			return
		} else if portal != nil && portal.OtherUserID != "" && !blocked[portal.OtherUserID] {
			targets[key] = portal.OtherUserID
		}
	}
	for userID := range blocked {
		portal, err := portals.GetDM(ctx, l.userLogin.ID, userID)
		if err != nil {
			log.Err(err).Msg("Failed to get DM with blocked user")
			return
		} else if portal != nil && !portal.UserBlocked {
			targets[portal.PortalKey] = userID
		}
	}
	if len(targets) > 0 {
		log.Debug().Int("count", len(targets)).Msg("Updating block state of DMs changed on LinkedIn")
	}
	for key, userID := range targets {
		l.queueBlockInfoChange(key, userID)
	}
}

func (l *LinkedInClient) queueBlockInfoChange(key networkid.PortalKey, userID networkid.UserID) {
	l.userLogin.QueueRemoteEvent(&blockInfoChange{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventChatInfoChange, PortalKey: key, Timestamp: time.Now(),
		},
		client: l, userID: userID,
	})
}

type blockInfoChange struct {
	simplevent.EventMeta
	client *LinkedInClient
	userID networkid.UserID
}

func (evt *blockInfoChange) GetChatInfoChange(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfoChange, error) {
	evt.client.blockSyncLock.Lock()
	defer evt.client.blockSyncLock.Unlock()
	if evt.client.blockedUsers == nil {
		return nil, nil
	}
	blocked := evt.client.blockedUsers[evt.userID]
	return &bridgev2.ChatInfoChange{ChatInfo: &bridgev2.ChatInfo{UserBlocked: &blocked}}, nil
}
