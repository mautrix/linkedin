package connector

import (
	"context"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo2/types2"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"
)

func (c *LinkedInClient) convertToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, msg *types2.DecoratedMessageRealtime) (cm *bridgev2.ConvertedMessage, err error) {
	return &bridgev2.ConvertedMessage{
		Parts: []*bridgev2.ConvertedMessagePart{
			{
				Type: event.EventMessage,
				Content: &event.MessageEventContent{
					MsgType: event.MsgText,
					Body:    msg.Result.Body.Text,
				},
			},
		},
	}, nil
}
