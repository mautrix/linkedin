package linkedingo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"go.mau.fi/util/jsontime"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/responseold"
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

func (c *Client) MarkConversationRead(ctx context.Context, convURNs ...URN) (*responseold.MarkThreadReadResponse, error) {
	return c.doMarkConversationRead(ctx, true, convURNs...)
}

func (c *Client) MarkConversationUnread(ctx context.Context, convURNs ...URN) (*responseold.MarkThreadReadResponse, error) {
	return c.doMarkConversationRead(ctx, false, convURNs...)
}

func (c *Client) doMarkConversationRead(ctx context.Context, read bool, convURNs ...URN) (*responseold.MarkThreadReadResponse, error) {
	conversationList := make([]string, len(convURNs))
	entities := map[URNString]GraphQLPatchBody{}
	for i, convURN := range convURNs {
		conversationList[i] = url.QueryEscape(convURN.String())
		entities[convURN.URNString()] = GraphQLPatchBody{Patch: Patch{Set: MarkMessageReadBody{Read: read}}}
	}

	resp, err := c.newAuthedRequest(http.MethodPost, linkedInMessagingDashMessengerConversationsURL).
		WithRawQuery(fmt.Sprintf("ids=List(%s)", strings.Join(conversationList, ","))). // Using raw query here because escaping the outer ()s makes this break
		WithContentType(contentTypePlaintextUTF8).
		WithHeader("accept", contentTypeJSON).
		WithHeader("origin", "https://www.linkedin.com").
		WithCSRF().
		WithXLIHeaders().
		WithJSONPayload(PatchEntitiesPayload{Entities: entities}).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to mark conversation read: %w", err)
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to mark conversation read (statusCode=%d)", resp.StatusCode)
	}

	var result responseold.MarkThreadReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	} else {
		return nil, errors.Join(slices.Collect(maps.Values(result.Errors))...)
	}
}
