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
