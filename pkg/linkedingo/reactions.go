package linkedingo

import (
	"context"
	"encoding/json"
	"fmt"
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
	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithQueryParam("action", action).
		WithContentType(contentTypePlaintextUTF8).
		WithCSRF().
		WithHeader("accept", contentTypeJSON).
		WithXLIHeaders().
		WithJSONPayload(map[string]any{
			"messageUrn": messageURN,
			"emoji":      emoji,
		}).
		Do(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to %s reaction %s to message %s (statusCode=%d)", action, emoji, messageURN, resp.StatusCode)
	}
	return nil
}

func (c *Client) GetReactors(ctx context.Context, messageURN URN, emoji string) (*CollectionResponse[any, MessagingParticipant], error) {
	resp, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
		WithGraphQLQuery("messengerMessagingParticipants.6bedbcf9406fa19045dc627ffc51f286", map[string]string{
			"messageUrn": url.QueryEscape(messageURN.String()),
			"emoji":      url.QueryEscape(emoji),
		}).
		Do(ctx)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get reactors for message %s with emoji %s (statusCode=%d)", messageURN, emoji, resp.StatusCode)
	}

	var response GraphQlResponse
	return response.Data.MessengerMessagingParticipantsByMessageAndEmoji, json.NewDecoder(resp.Body).Decode(&response)
}
