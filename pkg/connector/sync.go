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
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/simplevent"
)

func (l *LinkedInClient) syncConversations(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("action", "sync_conversations").Logger()
	log.Info().Msg("starting conversation sync")

	lastUsedUpdatedBefore := time.Time{}
	updatedBefore := time.Now()
	var updated, created int
	for {
		log := log.With().
			Time("updated_before", updatedBefore).
			Time("last_used_updated_before", lastUsedUpdatedBefore).
			Logger()

		if lastUsedUpdatedBefore.Equal(updatedBefore) {
			log.Info().Msg("no more conversations found")
			return
		}
		lastUsedUpdatedBefore = updatedBefore

		log.Info().Msg("fetching conversations")

		conversations, err := l.client.GetConversationsUpdatedBefore(ctx, updatedBefore)
		if err != nil {
			log.Err(err).Msg("failed to fetch conversations")
			return
		} else if conversations == nil {
			log.Warn().Msg("no conversations found")
			return
		}

		for _, conv := range conversations.Elements {
			log := log.With().
				Stringer("conversation_urn", conv.EntityURN).
				Time("last_activity_at", conv.LastActivityAt.Time).
				Logger()

			if conv.LastActivityAt.Before(updatedBefore) {
				updatedBefore = conv.LastActivityAt.Time
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
				CreatePortal: l.main.Config.Sync.CreateLimit == 0 || created <= l.main.Config.Sync.CreateLimit,
			}

			if portal == nil || portal.MXID == "" {
				created++
			}
			updated++

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

			if l.main.Config.Sync.UpdateLimit > 0 && updated >= l.main.Config.Sync.UpdateLimit {
				log.Info().Msg("Update limit reached")
				return
			}
		}
	}
}
