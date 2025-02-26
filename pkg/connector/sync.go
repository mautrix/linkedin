package connector

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/simplevent"
)

// TODO limits
func (l *LinkedInClient) syncConversations(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("action", "sync_conversations").Logger()
	log.Info().Msg("starting conversation sync")

	lastUsedUpdatedBefore := time.Time{}
	updatedBefore := time.Now()
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
			if conv.LastActivityAt.Before(updatedBefore) {
				updatedBefore = conv.LastActivityAt.Time
			}

			meta := simplevent.EventMeta{
				LogContext: func(c zerolog.Context) zerolog.Context {
					return c.Str("update", "sync")
				},
				PortalKey:    l.makePortalKey(conv.EntityURN),
				CreatePortal: true,
			}

			var latestMessageTS time.Time
			for _, msg := range conv.Messages.Elements {
				if msg.DeliveredAt.After(latestMessageTS) {
					latestMessageTS = msg.DeliveredAt.Time
				}
			}
			l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatResync{
				ChatInfo:        ptr.Ptr(l.conversationToChatInfo(conv)),
				EventMeta:       meta.WithType(bridgev2.RemoteEventChatResync),
				LatestMessageTS: latestMessageTS,
			})
		}
	}
}
