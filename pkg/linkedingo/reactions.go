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

package linkedingo

import (
	"context"
	"net/http"
	"net/url"
)

type DecoratedReactionSummary struct {
	Result RealtimeReactionSummary `json:"result,omitempty"`
}

// RealtimeReactionSummary represents a
// com.linkedin.messenger.RealtimeReactionSummary object.
type RealtimeReactionSummary struct {
	ReactionAdded   bool                 `json:"reactionAdded"`
	Actor           MessagingParticipant `json:"actor"`
	Message         Message              `json:"message"`
	ReactionSummary ReactionSummary      `json:"reactionSummary"`
}

// ReactionSummary represents a com.linkedin.messenger.ReactionSummary object.
type ReactionSummary struct {
	Count          int    `json:"count,omitempty"`
	FirstReactedAt int64  `json:"firstReactedAt,omitempty"`
	Emoji          string `json:"emoji,omitempty"`
	ViewerReacted  bool   `json:"viewerReacted"`
}

func (c *Client) SendReaction(ctx context.Context, messageURN URN, emoji string) error {
	return c.doReactAction(ctx, messageURN, emoji, "reactWithEmoji")
}

func (c *Client) RemoveReaction(ctx context.Context, messageURN URN, emoji string) error {
	return c.doReactAction(ctx, messageURN, emoji, "unreactWithEmoji")
}

func (c *Client) doReactAction(ctx context.Context, messageURN URN, emoji, action string) error {
	_, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithQueryParam("action", action).
		WithContentType(contentTypePlaintextUTF8).
		WithCSRF().
		WithHeader("accept", contentTypeJSON).
		WithXLIHeaders().
		WithJSONPayload(map[string]any{
			"messageUrn": messageURN,
			"emoji":      emoji,
		}).
		Do(ctx, nil)
	return err
}

func (c *Client) GetReactors(ctx context.Context, messageURN URN, emoji string) (*CollectionResponse[any, MessagingParticipant], error) {
	var response GraphQlResponse
	_, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
		WithGraphQLQuery("messengerMessagingParticipants.6bedbcf9406fa19045dc627ffc51f286", map[string]string{
			"messageUrn": url.QueryEscape(messageURN.String()),
			"emoji":      url.QueryEscape(emoji),
		}).
		Do(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response.Data.MessengerMessagingParticipantsByMessageAndEmoji, nil
}
