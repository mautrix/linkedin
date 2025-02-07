package connector

import (
	"context"
	"crypto/sha256"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *LinkedInClient) convertToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, msg *types.Message) (*bridgev2.ConvertedMessage, error) {
	var cm bridgev2.ConvertedMessage
	hasher := sha256.New()

	if len(msg.Body.Text) > 0 {
		hasher.Write([]byte(msg.Body.Text))
		cm.Parts = []*bridgev2.ConvertedMessagePart{
			{
				Type: event.EventMessage,

				// TODO handle the attributes
				Content: &event.MessageEventContent{
					MsgType: event.MsgText,
					Body:    msg.Body.Text,
				},
			},
		}
	}

	// TODO: link previews?

	cm.MergeCaption()

	return &cm, nil
}
