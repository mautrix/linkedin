package linkedingo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"go.mau.fi/util/jsontime"
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
	Body                SendMessageBody     `json:"body,omitempty"`
	RenderContentUnions []SendRenderContent `json:"renderContentUnions,omitempty"`
	ConversationURN     types.URN           `json:"conversationUrn,omitempty"`
	OriginToken         uuid.UUID           `json:"originToken,omitempty"`
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

type SendRenderContent struct {
	RepliedMessageContent *SendRepliedMessage `json:"repliedMessageContent,omitempty"`
}

type SendRepliedMessage struct {
	OriginalSenderURN  types.URN            `json:"originalSenderUrn"`
	OriginalSentAt     jsontime.UnixMilli   `json:"originalSendAt"`
	OriginalMessageURN types.URN            `json:"originalMessageUrn"`
	MessageBody        types.AttributedText `json:"messageBody"`
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
