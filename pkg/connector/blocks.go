package connector

import (
	"context"
	"errors"
	"time"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var _ bridgev2.ChatViewingNetworkAPI = (*LinkedInClient)(nil)
var _ bridgev2.UserBlockingNetworkAPI = (*LinkedInClient)(nil)

func (l *LinkedInClient) HandleMatrixBlockUser(ctx context.Context, msg *bridgev2.MatrixBlockUser) error {
	if msg == nil || msg.Content == nil || msg.Portal == nil || msg.Portal.RoomType != database.RoomTypeDM || msg.Portal.Receiver == "" || msg.Portal.Receiver != l.userLogin.ID || msg.Portal.OtherUserID == "" || msg.Portal.OtherUserID == l.userID {
		return errors.New("DM recipient is unknown")
	} else if msg.Content.ReportSpam {
		return errors.New("spam reporting is not supported")
	} else if !msg.Content.Block {
		return errors.New("unblock this member in LinkedIn settings: https://www.linkedin.com/mypreferences/d/manage-by-blocked-list")
	} else if !l.IsLoggedIn() {
		return errors.New("not logged in")
	}
	l.blockSyncLock.Lock()
	defer l.blockSyncLock.Unlock()
	l.blockedUsers = nil
	l.blockSyncTime = time.Time{}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return l.client.BlockUser(ctx, linkedingo.NewURN(msg.Portal.OtherUserID).AsFsdProfile(), msg.Portal.Name)
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

func (l *LinkedInClient) syncUserBlock(ctx context.Context, portal *bridgev2.Portal) {
	l.blockSyncLock.Lock()
	if time.Since(l.blockSyncTime) >= 10*time.Minute {
		l.blockSyncTime = time.Now()
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		users, err := l.client.GetBlockedUsers(ctx)
		if err != nil {
			l.blockedUsers = nil
			l.blockSyncLock.Unlock()
			l.userLogin.Log.Warn().Err(err).Msg("Failed to fetch blocked users")
			return
		}
		l.blockedUsers = make(map[networkid.UserID]bool, len(users))
		for _, user := range users {
			l.blockedUsers[networkid.UserID(user.ID())] = true
		}
	}
	if l.blockedUsers == nil {
		l.blockSyncLock.Unlock()
		return
	}
	l.blockSyncLock.Unlock()
	l.userLogin.QueueRemoteEvent(&blockInfoChange{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventChatInfoChange, PortalKey: portal.PortalKey, Timestamp: time.Now(),
		},
		client: l, userID: portal.OtherUserID,
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
