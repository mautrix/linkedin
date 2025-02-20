package linkedingo

import (
	"context"
	"fmt"
	"net/http"
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
