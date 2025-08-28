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
		Forward:  fetchParams.Forward,
		MarkRead: true,
	}

	convURN := linkedingo.NewURN(fetchParams.Portal.ID)
	var messages []linkedingo.Message
	if fetchParams.Cursor != "" {
		msgs, err := l.client.GetMessagesWithPrevCursor(ctx, convURN, string(fetchParams.Cursor), fetchParams.Count)
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

	lastRead := l.conversationLastRead[convURN]

	for _, msg := range messages {
		log := log.With().Stringer("entity_urn", msg.EntityURN).Logger()
		ctx := log.WithContext(ctx)
		if !stopAt.IsZero() {
			if fetchParams.Forward {
				if !msg.DeliveredAt.Time.After(stopAt) {
					// If we are doing forward backfill skip any messages that are before the anchor message
					log.Debug().Msg("skipping message before anchor message")
					continue
				}
			} else if !msg.DeliveredAt.Time.Before(stopAt) {
				// If we are doing backwards backfill and we got to a message
				// more recent or equal to the anchor message, skip it.
				log.Debug().Msg("skipping message past anchor message")
				continue
			}
		}

		sender := l.makeSender(msg.Sender)

		intent, ok := portal.GetIntentFor(ctx, sender, l.userLogin, bridgev2.RemoteEventBackfill)
		if !ok {
			continue
		}
		converted, err := l.convertToMatrix(ctx, portal, intent, msg)
		if err != nil {
			return nil, err
		}

		backfillMessage := bridgev2.BackfillMessage{
			ConvertedMessage: converted,
			Sender:           sender,
			ID:               msg.MessageID(),
			Timestamp:        msg.DeliveredAt.Time,
			StreamOrder:      msg.DeliveredAt.UnixMilli(),
		}

		for _, rs := range msg.ReactionSummaries {
			reactors, err := l.client.GetReactors(ctx, msg.EntityURN, rs.Emoji)
			if err != nil {
				log.Err(err).Msg("failed to get reactors")
				continue
			}
			for _, reactor := range reactors.Elements {
				backfillMessage.Reactions = append(backfillMessage.Reactions, &bridgev2.BackfillReaction{
					Sender:  l.makeSender(reactor),
					EmojiID: networkid.EmojiID(rs.Emoji),
					Emoji:   rs.Emoji,
				})
			}
		}

		resp.Messages = append(resp.Messages, &backfillMessage)

		if resp.MarkRead && msg.DeliveredAt.UnixMilli() > lastRead.UnixMilli() {
			resp.MarkRead = false
		}
	}

	resp.HasMore = len(resp.Messages) > 0
	return &resp, nil
}
