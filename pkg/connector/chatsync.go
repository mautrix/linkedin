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
	"maunium.net/go/mautrix/bridgev2"
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

// handleConversations processes a page of conversations. Returns true if create limit was reached.
func (l *LinkedInClient) handleConversations(ctx context.Context, convs []linkedingo.Conversation, created, updated *int) bool {
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

		l.conversationLastRead[conv.EntityURN] = conv.LastReadAt
		readStatusChanged := false
		lastReadState, ok := l.conversationReadState[conv.EntityURN]
		if (ok && lastReadState.Read != conv.Read) || !ok {
			readStatusChanged = true
		}
		l.conversationReadState[conv.EntityURN] = ConversationReadState{
			LastReadAt: conv.LastReadAt,
			Read:       conv.Read,
		}

		portalKey := l.makePortalKey(conv)
		portal, err := l.main.Bridge.GetPortalByKey(ctx, portalKey)
		if err != nil {
			log.Err(err).Msg("Failed to get portal")
			continue
		}

		meta := simplevent.EventMeta{
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.Str("update", "sync")
			},
			PortalKey:    portalKey,
			CreatePortal: true,
		}

		if portal == nil || portal.MXID == "" {
			if l.main.Config.Sync.CreateLimit > 0 && *created >= l.main.Config.Sync.CreateLimit {
				log.Info().Int("created", *created).Msg("Create limit reached")
				return true
			}
			*created++
		}
		*updated++

		var latestMessageTS time.Time
		for _, msg := range conv.Messages.Elements {
			if msg.DeliveredAt.After(latestMessageTS) {
				latestMessageTS = msg.DeliveredAt.Time
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

		if l.main.Config.Sync.UpdateLimit > 0 && *updated >= l.main.Config.Sync.UpdateLimit {
			log.Info().Msg("Update limit reached")
			return true
		}
	}
	return false
}

func (l *LinkedInClient) syncConversations(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("action", "sync_conversations").Logger()
	log.Info().Msg("starting conversation sync")

	var nextCursor string
	var created, updated int
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			log.Info().Msg("sync canceled")
			return
		}

		log := log.With().Int("page", page).Int("created", created).Int("updated", updated).Logger()
		log.Info().Msg("fetching conversations")

		var conversations *linkedingo.CollectionResponse[linkedingo.ConversationCursorMetadata, linkedingo.Conversation]
		var err error
		if nextCursor == "" {
			conversations, err = l.client.GetConversationsUpdatedBefore(ctx, time.Now())
		} else {
			conversations, err = l.client.GetConversationsWithCursor(ctx, nextCursor)
		}
		if err != nil {
			log.Err(err).Msg("failed to fetch conversations")
			return
		} else if conversations == nil || len(conversations.Elements) == 0 {
			log.Info().Msg("no more conversations found")
			return
		}

		if l.handleConversations(ctx, conversations.Elements, &created, &updated) {
			return
		}

		nextCursor = conversations.Metadata.NextCursor
		if nextCursor == "" {
			log.Info().Msg("no more pages (no nextCursor)")
			return
		}
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
	var created, updated int
	_ = l.handleConversations(ctx, convs.Elements, &created, &updated)
	for _, item := range convs.Metadata.DeletedURNs {
		l.deleteURN(ctx, item.Conversation.EntityURN)
	}
}
