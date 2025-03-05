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
	"fmt"
	"net/http"
)

type DecoratedTypingIndicator struct {
	Result RealtimeTypingIndicator `json:"result,omitempty"`
}

// RealtimeTypingIndicator represents a
// com.linkedin.messenger.RealtimeTypingIndicator object.
type RealtimeTypingIndicator struct {
	TypingParticipant MessagingParticipant `json:"typingParticipant,omitempty"`
	Conversation      Conversation         `json:"conversation,omitempty"`
}

func (c *Client) StartTyping(ctx context.Context, conversationURN URN) error {
	resp, err := c.newAuthedRequest(http.MethodPost, linkedInMessagingDashMessengerConversationsURL).
		WithQueryParam("action", "typing").
		WithContentType(contentTypePlaintextUTF8).
		WithCSRF().
		WithHeader("accept", contentTypeJSON).
		WithXLIHeaders().
		WithJSONPayload(map[string]any{
			"conversationUrn": conversationURN,
		}).
		Do(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("failed to start typing on %s (statusCode=%d)", conversationURN, resp.StatusCode)
	}
	return nil
}
