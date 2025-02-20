package connector

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

func (l *LinkedInClient) syncConversations(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("action", "sync_conversations").Logger()

	conversations, err := l.client.GetConversations(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch initial inbox state")
		return
	}
	fmt.Printf("%+v\n", conversations)
}
