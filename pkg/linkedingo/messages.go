package linkedingo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"go.mau.fi/util/jsontime"
	"go.mau.fi/util/random"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/queryold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/responseold"
)

type sendMessagePayload struct {
	Message                      SendMessage `json:"message,omitempty"`
	MailboxURN                   URN         `json:"mailboxUrn,omitempty"`
	TrackingID                   string      `json:"trackingId,omitempty"`
	DedupeByClientGeneratedToken bool        `json:"dedupeByClientGeneratedToken"`
	HostRecipientURNs            []URN       `json:"hostRecipientUrns,omitempty"`
	ConversationTitle            string      `json:"conversationTitle,omitempty"`
}

type SendMessage struct {
	Body                SendMessageBody     `json:"body,omitempty"`
	RenderContentUnions []SendRenderContent `json:"renderContentUnions,omitempty"`
	ConversationURN     URN                 `json:"conversationUrn,omitempty"`
	OriginToken         uuid.UUID           `json:"originToken,omitempty"`
}

type SendMessageBody struct {
	Text       string                 `json:"text"`
	Attributes []SendMessageAttribute `json:"attributes,omitempty"`
}

type SendMessageAttribute struct {
	Start              int           `json:"start"`
	Length             int           `json:"length"`
	AttributeKindUnion AttributeKind `json:"attributeKindUnion"`
}

type AttributeType struct {
	Entity *Entity `json:"com.linkedin.pemberly.text.Entity,omitempty"`
}

type SendRenderContent struct {
	Audio *SendAudio `json:"audio,omitempty"`
	File  *SendFile  `json:"file,omitempty"`
}

type SendAudio struct {
	AssetURN URN                   `json:"assetUrn,omitempty"`
	ByteSize int                   `json:"byteSize,omitempty"`
	Duration jsontime.Milliseconds `json:"duration,omitempty"`
}

type SendFile struct {
	AssetURN  URN    `json:"assetUrn,omitempty"`
	ByteSize  int    `json:"byteSize,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Name      string `json:"name,omitempty"`
}

type MessageSentResponse struct {
	Data Message `json:"value,omitempty"`
}

type DecoratedMessage struct {
	Result Message `json:"result,omitempty"`
}

// Message represents a com.linkedin.messenger.Message object.
type Message struct {
	Body                    AttributedText          `json:"body,omitempty"`
	DeliveredAt             jsontime.UnixMilli      `json:"deliveredAt,omitempty"`
	EntityURN               URN                     `json:"entityUrn,omitempty"`
	Sender                  MessagingParticipant    `json:"sender,omitempty"`
	MessageBodyRenderFormat MessageBodyRenderFormat `json:"messageBodyRenderFormat,omitempty"`
	BackendConversationURN  URN                     `json:"backendConversationUrn,omitempty"`
	Conversation            Conversation            `json:"conversation,omitempty"`
	RenderContent           []RenderContent         `json:"renderContent,omitempty"`
}

func (m Message) MessageID() networkid.MessageID {
	return networkid.MessageID(m.EntityURN.String())
}

type MessageBodyRenderFormat string

const (
	MessageBodyRenderFormatDefault  MessageBodyRenderFormat = "DEFAULT"
	MessageBodyRenderFormatEdited   MessageBodyRenderFormat = "EDITED"
	MessageBodyRenderFormatRecalled MessageBodyRenderFormat = "RECALLED"
	MessageBodyRenderFormatSystem   MessageBodyRenderFormat = "SYSTEM"
)

type RenderContent struct {
	Audio                 *AudioMetadata     `json:"audio,omitempty"`
	ExternalMedia         *ExternalMedia     `json:"externalMedia,omitempty"`
	File                  *FileAttachment    `json:"file,omitempty"`
	RepliedMessageContent *RepliedMessage    `json:"repliedMessageContent,omitempty"`
	VectorImage           *VectorImage       `json:"vectorImage,omitempty"`
	Video                 *VideoPlayMetadata `json:"video,omitempty"`
}

// RepliedMessage represents a com.linkedin.messenger.RepliedMessage object.
type RepliedMessage struct {
	OriginalMessage Message `json:"originalMessage,omitempty"`
}

func (c *Client) SendMessage(ctx context.Context, conversationURN URN, body SendMessageBody, renderContent []SendRenderContent) (*MessageSentResponse, error) {
	payload := sendMessagePayload{
		Message: SendMessage{
			Body:                body,
			RenderContentUnions: renderContent,
			ConversationURN:     conversationURN,
			OriginToken:         uuid.New(),
		},
		MailboxURN: c.userEntityURN.WithPrefix("urn", "li", "fsd_profile"),
		TrackingID: random.String(16),
	}

	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithJSONPayload(payload).
		WithQueryParam("action", "createMessage").
		WithCSRF().
		WithContentType(contentTypePlaintextUTF8).
		WithXLIHeaders().
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

func (c *Client) EditMessage(ctx context.Context, messageURN URN, p SendMessageBody) error {
	url, err := url.JoinPath(linkedInVoyagerMessagingDashMessengerMessagesURL, messageURN.URLEscaped())
	if err != nil {
		return err
	}
	resp, err := c.newAuthedRequest(http.MethodPost, url).
		WithCSRF().
		WithJSONPayload(GraphQLPatchBody{Patch: Patch{Set: EditMessagePayload{Body: p}}}).
		WithHeader("accept", contentTypeJSON).
		WithXLIHeaders().
		Do(ctx)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to edit message with urn %s (statusCode=%d)", messageURN, resp.StatusCode)
	}
	return nil
}

func (c *Client) RecallMessage(ctx context.Context, messageURN URN) error {
	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithQueryParam("action", "recall").
		WithCSRF().
		WithXLIHeaders().
		WithJSONPayload(map[string]any{"messageUrn": messageURN}).
		Do(ctx)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to edit message with urn %s (statusCode=%d)", messageURN, resp.StatusCode)
	}
	return nil
}

func (c *Client) FetchMessages(ctx context.Context, conversationURN URN, variables queryold.FetchMessagesVariables) (*responseold.MessengerMessagesResponse, error) {
	withCursor := variables.PrevCursor != ""
	withAnchorTimestamp := !variables.DeliveredAt.IsZero()

	var queryID string
	if withCursor {
		queryID = graphQLQueryIDMessengerMessagesByConversation
	} else if withAnchorTimestamp {
		queryID = graphQLQueryIDMessengerMessagesByAnchorTimestamp
	} else {
		queryID = graphQLQueryIDMessengerMessagesBySyncToken
	}
	fmt.Printf("queryID = %s\n", queryID)
	return nil, nil

	// variablesQuery, err := variables.Encode()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// resp, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
	// 	WithGraphQLQuery(queryID, variables).
	// 	WithCSRF().
	// 	WithXLIHeaders().
	// 	WithHeader("accept", contentTypeGraphQL).
	// 	Do(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// var graphQLResponse responseold.GraphQlResponse
	// if err = json.NewDecoder(resp.Body).Decode(&graphQLResponse); err != nil {
	// 	return nil, err
	// }
	//
	// graphQLResponseData := graphQLResponse.Data
	// if withCursor {
	// 	return graphQLResponseData.MessengerMessagesByConversation, nil
	// } else if withAnchorTimestamp {
	// 	return graphQLResponseData.MessengerMessagesByAnchorTimestamp, nil
	// } else {
	// 	return graphQLResponseData.MessengerMessagesBySyncToken, nil
	// }
}
