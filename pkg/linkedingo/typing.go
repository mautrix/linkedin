package linkedingo

import (
	"context"
	"fmt"
	"net/http"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *Client) StartTyping(ctx context.Context, conversationURN types.URN) error {
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
