package connector

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/simplevent"
)

func (l *LinkedInClient) syncConversations(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("action", "sync_conversations").Logger()

	conversations, err := l.client.GetConversations(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch initial inbox state")
		return
	}

	fmt.Printf("%+v\n", conversations)

	for _, conv := range conversations.Elements {
		fmt.Printf("conv=%+v\n", conv)

		l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatResync{
			ChatInfo: ptr.Ptr(l.conversationToChatInfo(conv)),
			EventMeta: simplevent.EventMeta{
				Type: bridgev2.RemoteEventChatResync,
				LogContext: func(c zerolog.Context) zerolog.Context {
					return c.Str("update", "sync")
				},
				PortalKey:    l.makePortalKey(conv.EntityURN),
				CreatePortal: true,
			},
			CheckNeedsBackfillFunc: func(ctx context.Context, latestMessage *database.Message) (bool, error) {
				if latestMessage == nil {
					return true, nil
				}
				return true, nil
				// TODO
				// return TopMessage > latestMessageID, nil
			},
		})
	}
}
