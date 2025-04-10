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
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"go.mau.fi/util/jsontime"
)

type DecoratedSeenReceipt struct {
	Result SeenReceipt `json:"result,omitempty"`
}

// SeenReceipt represents a com.linkedin.messenger.SeenReceipt object.
type SeenReceipt struct {
	SeenAt            jsontime.UnixMilli   `json:"seenAt,omitempty"`
	Message           Message              `json:"message,omitempty"`
	SeenByParticipant MessagingParticipant `json:"seenByParticipant,omitempty"`
}

type MarkMessageReadBody struct {
	Read bool `json:"read"`
}

type PatchEntitiesPayload struct {
	Entities map[URNString]GraphQLPatchBody `json:"entities,omitempty"`
}

type MarkThreadReadResponse struct {
	Results map[URNString]MarkThreadReadResult `json:"results,omitempty"`
	Errors  map[string]error                   `json:"errors,omitempty"`
}

type MarkThreadReadResult struct {
	Status int `json:"status,omitempty"`
}

func (c *Client) MarkConversationRead(ctx context.Context, convURNs ...URN) (*MarkThreadReadResponse, error) {
	return c.doMarkConversationRead(ctx, true, convURNs...)
}

func (c *Client) MarkConversationUnread(ctx context.Context, convURNs ...URN) (*MarkThreadReadResponse, error) {
	return c.doMarkConversationRead(ctx, false, convURNs...)
}

func (c *Client) doMarkConversationRead(ctx context.Context, read bool, convURNs ...URN) (*MarkThreadReadResponse, error) {
	conversationList := make([]string, len(convURNs))
	entities := map[URNString]GraphQLPatchBody{}
	for i, convURN := range convURNs {
		conversationList[i] = url.QueryEscape(convURN.String())
		entities[convURN.URNString()] = GraphQLPatchBody{Patch: Patch{Set: MarkMessageReadBody{Read: read}}}
	}

	var result MarkThreadReadResponse
	_, err := c.newAuthedRequest(http.MethodPost, linkedInMessagingDashMessengerConversationsURL).
		WithRawQuery(fmt.Sprintf("ids=List(%s)", strings.Join(conversationList, ","))). // Using raw query here because escaping the outer ()s makes this break
		WithContentType(contentTypePlaintextUTF8).
		WithHeader("accept", contentTypeJSON).
		WithHeader("origin", "https://www.linkedin.com").
		WithCSRF().
		WithXLIHeaders().
		WithJSONPayload(PatchEntitiesPayload{Entities: entities}).
		Do(ctx, &result)
	if err != nil {
		return nil, err
	}

	return &result, errors.Join(slices.Collect(maps.Values(result.Errors))...)
}
