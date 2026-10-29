// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2025 Sumner Evans
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package connector

import (
	"context"
	"slices"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) deleteConversation(ctx context.Context, conv linkedingo.Conversation) {
	l.deletePortal(ctx, l.makePortalKey(conv))
}

func (l *LinkedInClient) deletePortal(ctx context.Context, portalKey networkid.PortalKey) {
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatDelete{
		EventMeta: simplevent.EventMeta{
			Type:      bridgev2.RemoteEventChatDelete,
			PortalKey: portalKey,
		},
		OnlyForMe: true,
	})
}

func (l *LinkedInClient) deleteURN(ctx context.Context, urn linkedingo.URN) {
	portalKey := networkid.PortalKey{
		ID:       networkid.PortalID(urn.String()),
		Receiver: l.userLogin.ID,
	}
	l.deletePortal(ctx, portalKey)
	if !l.main.Bridge.Config.SplitPortals {
		portalKey.Receiver = ""
		l.deletePortal(ctx, portalKey)
	}
}

func (l *LinkedInClient) handleConversations(ctx context.Context, convs []linkedingo.Conversation, synced *int) (limitReached bool) {
	log := zerolog.Ctx(ctx)

	for _, conv := range convs {
		if slices.Contains(conv.Categories, "SPAM") {
			l.deleteConversation(ctx, conv)
			continue
		}
		if !slices.Contains(conv.Categories, "INBOX") {
			continue
		}

		isMember := false
		for _, participant := range conv.ConversationParticipants {
			if participant.EntityURN.ID() == string(l.userID) {
				isMember = true
				break
			}
		}
		if !isMember {
			l.deleteConversation(ctx, conv)
			continue
		}

		log := log.With().
			Stringer("conversation_urn", conv.EntityURN).
			Time("last_activity_at", conv.LastActivityAt.Time).
			Logger()

		l.conversationStateLock.Lock()
		l.conversationLastRead[conv.EntityURN] = conv.LastReadAt
		lastReadState, ok := l.conversationReadState[conv.EntityURN]
		readStatusChanged := ok && lastReadState.Read != conv.Read
		l.conversationReadState[conv.EntityURN] = ConversationReadState{
			LastReadAt: conv.LastReadAt,
			Read:       conv.Read,
		}
		l.conversationStateLock.Unlock()

		portalKey := l.makePortalKey(conv)
		meta := simplevent.EventMeta{
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.Str("update", "sync")
			},
			PortalKey:    portalKey,
			CreatePortal: l.main.Config.Sync.CreateLimit == 0 || *synced < l.main.Config.Sync.CreateLimit,
		}
		*synced++

		var latestMessageTS time.Time
		var latestMessageID networkid.MessageID
		for _, msg := range conv.Messages.Elements {
			if msg.DeliveredAt.After(latestMessageTS) {
				latestMessageTS = msg.DeliveredAt.Time
				latestMessageID = msg.MessageID()
			}
		}
		chatInfo, userInChat := l.conversationToChatInfo(conv)
		if !userInChat {
			log.Debug().Msg("User not in chat")
			continue
		}
		l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatResync{
			ChatInfo:        &chatInfo,
			EventMeta:       meta.WithType(bridgev2.RemoteEventChatResync),
			LatestMessageTS: latestMessageTS,
			CheckNeedsBackfillFunc: func(_ context.Context, latestMessage *database.Message) (bool, error) {
				if latestMessage == nil {
					return !latestMessageTS.IsZero(), nil
				}
				return latestMessage.ID != latestMessageID && latestMessageTS.After(latestMessage.Timestamp), nil
			},
		})
		if readStatusChanged {
			sender := bridgev2.EventSender{
				IsFromMe:    true,
				Sender:      networkid.UserID(l.userLogin.ID),
				SenderLogin: l.userLogin.ID,
			}
			l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.MarkUnread{
				EventMeta: simplevent.EventMeta{
					Type:      bridgev2.RemoteEventMarkUnread,
					PortalKey: portalKey,
					Sender:    sender,
				},
				Unread: !conv.Read,
			})
		}

		if l.main.Config.Sync.UpdateLimit > 0 && *synced >= l.main.Config.Sync.UpdateLimit {
			log.Info().Msg("Update limit reached")
			return true
		}
	}
	return false
}

func (l *LinkedInClient) syncConversations(ctx context.Context) {
	l.syncConversationsLock.Lock()
	defer l.syncConversationsLock.Unlock()
	meta := l.userLogin.Metadata.(*UserLoginMetadata)
	log := zerolog.Ctx(ctx).With().
		Str("action", "sync_conversations").
		Time("synced_up_to", meta.ConversationsSyncedUpTo.Time).
		Logger()
	log.Info().Msg("starting conversation sync")

	syncStart := time.Now()
	lastUsedUpdatedBefore := time.Time{}
	updatedBefore := syncStart
	var synced int
	for {
		if ctx.Err() != nil {
			log.Info().Msg("sync canceled")
			return
		}

		log := log.With().
			Time("updated_before", updatedBefore).
			Time("last_used_updated_before", lastUsedUpdatedBefore).
			Logger()

		if lastUsedUpdatedBefore.Equal(updatedBefore) {
			log.Info().Msg("no more conversations found")
			break
		}
		lastUsedUpdatedBefore = updatedBefore

		log.Info().Msg("fetching conversations")

		conversations, err := l.client.GetConversationsUpdatedBefore(ctx, updatedBefore)
		if err != nil {
			log.Err(err).Msg("failed to fetch conversations")
			return
		} else if conversations == nil {
			log.Warn().Msg("no conversations found")
			break
		}

		for _, conv := range conversations.Elements {
			if !conv.LastActivityAt.IsZero() && conv.LastActivityAt.Before(updatedBefore) {
				updatedBefore = conv.LastActivityAt.Time
			}
		}
		if l.handleConversations(ctx, conversations.Elements, &synced) {
			break
		} else if updatedBefore.Before(meta.ConversationsSyncedUpTo.Time) {
			log.Info().Msg("reached conversations synced previously")
			break
		}
	}
	meta.ConversationsSyncedUpTo = jsontime.UM(syncStart)
	if err := l.userLogin.Save(ctx); err != nil {
		log.Err(err).Msg("failed to save conversation sync timestamp")
	}
}
func (l *LinkedInClient) getConversationsBySyncToken(ctx context.Context) {
	convs, err := l.client.GetConversationsBySyncToken(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("failed to get conversations by sync token")
		return
	}
	if convs == nil {
		return
	}
	l.handleConversations(ctx, convs.Elements, new(int))
	for _, item := range convs.Metadata.DeletedURNs {
		l.deleteURN(ctx, item.Conversation.EntityURN)
	}
}
