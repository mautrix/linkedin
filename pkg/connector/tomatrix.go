package connector

import (
	"context"
	"crypto/sha256"
	"fmt"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *LinkedInClient) convertToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, msg types.Message) (*bridgev2.ConvertedMessage, error) {
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

func (lc *LinkedInClient) convertEditToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, msg types.Message) (*bridgev2.ConvertedEdit, error) {
	if len(existing) != 1 {
		return nil, fmt.Errorf("editing a message that was bridged as multiple parts is not supported")
	}
	converted, err := lc.convertToMatrix(ctx, portal, intent, msg)
	if err != nil {
		return nil, err
	}
	if len(converted.Parts) != 1 {
		return nil, fmt.Errorf("editing a message in a way that creates multiple parts is not supported")
	}

	var convertedEdit bridgev2.ConvertedEdit
	for i, part := range converted.Parts {
		convertedEdit.ModifiedParts = append(convertedEdit.ModifiedParts, part.ToEditPart(existing[i]))
	}
	return &convertedEdit, nil
}
