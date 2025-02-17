package linkedingo

import (
	"context"
	"fmt"
	"net/http"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *Client) SendReaction(ctx context.Context, messageURN types.URN, emoji string) error {
	return c.doReactAction(ctx, messageURN, emoji, "reactWithEmoji")
}

func (c *Client) RemoveReaction(ctx context.Context, messageURN types.URN, emoji string) error {
	return c.doReactAction(ctx, messageURN, emoji, "unreactWithEmoji")
}

func (c *Client) doReactAction(ctx context.Context, messageURN types.URN, emoji, action string) error {
	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithQueryParam("action", action).
		WithContentType(contentTypePlaintextUTF8).
		WithCSRF().
		WithHeader("accept", contentTypeJSON).
		WithJSONPayload(map[string]any{
			"messageUrn": messageURN,
			"emoji":      emoji,
		}).
		WithXLIHeaders().
		Do(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to %s reaction %s to message %s (statusCode=%d)", action, emoji, messageURN, resp.StatusCode)
	}
	return nil
}
