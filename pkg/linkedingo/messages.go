package linkedingo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"go.mau.fi/util/random"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

type SendMessagePayload struct {
	Message                      SendMessage `json:"message,omitempty"`
	MailboxURN                   types.URN   `json:"mailboxUrn,omitempty"`
	TrackingID                   string      `json:"trackingId,omitempty"`
	DedupeByClientGeneratedToken bool        `json:"dedupeByClientGeneratedToken"`
	HostRecipientURNs            []types.URN `json:"hostRecipientUrns,omitempty"`
	ConversationTitle            string      `json:"conversationTitle,omitempty"`
}

type SendMessage struct {
	Body                SendMessageBody `json:"body,omitempty"`
	RenderContentUnions []any           `json:"renderContentUnions,omitempty"`
	ConversationURN     types.URN       `json:"conversationUrn,omitempty"`
	OriginToken         uuid.UUID       `json:"originToken,omitempty"`
}

type SendMessageBody struct {
	Text       string                 `json:"text"`
	Attributes []SendMessageAttribute `json:"attributes,omitempty"`
}

type SendMessageAttribute struct {
	Start              int                 `json:"start"`
	Length             int                 `json:"length"`
	AttributeKindUnion types.AttributeKind `json:"attributeKindUnion"`
}

type AttributeType struct {
	Entity *types.Entity `json:"com.linkedin.pemberly.text.Entity,omitempty"`
}

type MessageSentResponse struct {
	Data types.Message `json:"value,omitempty"`
}

func (c *Client) SendMessage(ctx context.Context, payload SendMessagePayload) (*MessageSentResponse, error) {
	payload.MailboxURN = c.userEntityURN.WithPrefix("urn", "li", "fsd_profile")
	payload.TrackingID = random.String(16)
	payload.Message.OriginToken = uuid.New()

	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithJSONPayload(payload).
		WithParam("action", "createMessage").
		WithCSRF().
		WithContentType(contentTypePlaintextUTF8).
		WithRealtimeHeaders().
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to send message to conversation with urn %s (statusCode=%d)", payload.Message.ConversationURN, resp.StatusCode)
	}

	var messageSentResponse MessageSentResponse
	return &messageSentResponse, json.NewDecoder(resp.Body).Decode(&messageSentResponse)
}

type GraphQLPatchBody struct {
	Patch Patch `json:"patch,omitempty"`
}

// TODO: genericise?
type Patch struct {
	Set any `json:"$set,omitempty"`
}

type EditMessagePayload struct {
	Body SendMessageBody `json:"body,omitempty"`
}

func (c *Client) EditMessage(ctx context.Context, messageURN types.URN, p SendMessageBody) error {
	url, err := url.JoinPath(linkedInVoyagerMessagingDashMessengerMessagesURL, messageURN.URLEscaped())
	if err != nil {
		return err
	}
	resp, err := c.newAuthedRequest(http.MethodPost, url).
		WithCSRF().
		WithJSONPayload(GraphQLPatchBody{Patch: Patch{Set: EditMessagePayload{Body: p}}}).
		WithHeader("accept", contentTypeJSON).
		WithRealtimeHeaders().
		Do(ctx)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to edit message with urn %s (statusCode=%d)", messageURN, resp.StatusCode)
	}
	return nil
}

func (c *Client) RecallMessage(ctx context.Context, messageURN types.URN) error {
	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithParam("action", "recall").
		WithCSRF().
		WithRealtimeHeaders().
		WithJSONPayload(map[string]any{"messageUrn": messageURN}).
		Do(ctx)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to edit message with urn %s (statusCode=%d)", messageURN, resp.StatusCode)
	}
	return nil
}
