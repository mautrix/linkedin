package connector

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) FetchMessages(ctx context.Context, fetchParams bridgev2.FetchMessagesParams) (*bridgev2.FetchMessagesResponse, error) {
	log := zerolog.Ctx(ctx).With().Str("method", "fetch_messages").Logger()
	ctx = log.WithContext(ctx)

	portal, err := l.main.Bridge.GetPortalByKey(ctx, fetchParams.Portal.PortalKey)
	if err != nil {
		return nil, err
	}

	resp := bridgev2.FetchMessagesResponse{
		Forward: fetchParams.Forward,
		// MarkRead: markRead,
	}

	var messages []linkedingo.Message
	if fetchParams.Cursor != "" {
		msgs, err := l.client.GetMessagesWithPrevCursor(ctx, linkedingo.NewURN(fetchParams.Portal.ID), string(fetchParams.Cursor), fetchParams.Count)
		if err != nil {
			return nil, err
		} else if len(msgs.Elements) == 0 {
			return &bridgev2.FetchMessagesResponse{HasMore: false, Forward: fetchParams.Forward}, nil
		}
		messages = msgs.Elements
		resp.Cursor = networkid.PaginationCursor(msgs.Metadata.PrevCursor)
	} else {
		msgs, err := l.client.GetMessagesBefore(ctx, linkedingo.NewURN(fetchParams.Portal.ID), time.Now(), fetchParams.Count)
		if err != nil {
			return nil, err
		} else if len(msgs.Elements) == 0 {
			return &bridgev2.FetchMessagesResponse{HasMore: false, Forward: fetchParams.Forward}, nil
		}
		messages = msgs.Elements
		resp.Cursor = networkid.PaginationCursor(msgs.Metadata.PrevCursor)
	}

	var stopAt time.Time
	if fetchParams.AnchorMessage != nil {
		stopAt = fetchParams.AnchorMessage.Timestamp
		log = log.With().Time("stop_at", stopAt).Logger()
	}

	for _, msg := range messages {
		log := log.With().Stringer("entity_urn", msg.EntityURN).Logger()
		ctx := log.WithContext(ctx)
		if !stopAt.IsZero() {
			if fetchParams.Forward && !msg.DeliveredAt.Time.After(stopAt) {
				// If we are doing forward backfill and we got to before or at
				// the anchor message, don't convert any more messages.
				log.Debug().Msg("stopping at anchor message")
				break
			} else if !msg.DeliveredAt.Time.Before(stopAt) {
				// If we are doing backwards backfill and we got to a message
				// more recent or equal to the anchor message, skip it.
				log.Debug().Msg("skipping message past anchor message")
				continue
			}
		}

		sender := l.makeSender(msg.Sender)

		intent := portal.GetIntentFor(ctx, sender, l.userLogin, bridgev2.RemoteEventBackfill)
		converted, err := l.convertToMatrix(ctx, portal, intent, msg)
		if err != nil {
			return nil, err
		}

		backfillMessage := bridgev2.BackfillMessage{
			ConvertedMessage: converted,
			Sender:           sender,
			ID:               msg.MessageID(),
			Timestamp:        msg.DeliveredAt.Time,
		}

		// TODO reactions

		resp.Messages = append(resp.Messages, &backfillMessage)
	}

	resp.HasMore = len(resp.Messages) > 0
	return &resp, nil
}
