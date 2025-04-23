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
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"go.mau.fi/util/random"
	"maunium.net/go/mautrix/bridgev2/networkid"
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
	Video *SendVideo `json:"video,omitempty"`
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

type SendVideo struct{
        Media  URN `json:"media,omitempty"`
        Thumbnail SendThumbnail `json:"thumbnail,omitempty"`
        TrackingID URN `json:"trackingId,omitempty"`
        ProgressiveStreams []SendProgressiveStreams `json:"progressiveStreams,omitempty"` 
}

type SendThumbnail struct{
        Artifacts []SendArtifacts `json:"artifacts,omitempty"`
        RootUrl string `json:"rootUrl"`
}

type SendArtifacts struct{
        Width int `json:"width"`
        Height int `json:"height"`
}

type MessageSentResponse struct {
	Data Message `json:"value,omitempty"`
}

type DecoratedMessage struct {
	Result Message `json:"result,omitempty"`
}

type SendProgressiveStreams struct{
        BitRate int `json:"bitRate"`
        Height int `json:"height"`
        MediaType string `json:"mediaType,omitempty"`
        Size int `json:"size"`
        Width int `json:"width"`
        StreamingLocations []SendURL `json:"streamingLocations,omitempty"`
}

type SendURL struct{
        URL string `json:"url,omitempty"`
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
	ReactionSummaries       []ReactionSummary       `json:"reactionSummaries,omitempty"`
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

	var messageSentResponse MessageSentResponse
	_, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithJSONPayload(payload).
		WithQueryParam("action", "createMessage").
		WithCSRF().
		WithContentType(contentTypePlaintextUTF8).
		WithXLIHeaders().
		Do(ctx, &messageSentResponse)
	if err != nil {
		return nil, err
	}

	return &messageSentResponse, nil
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
	_, err = c.newAuthedRequest(http.MethodPost, url).
		WithCSRF().
		WithJSONPayload(GraphQLPatchBody{Patch: Patch{Set: EditMessagePayload{Body: p}}}).
		WithHeader("accept", contentTypeJSON).
		WithXLIHeaders().
		Do(ctx, nil)
	return err
}

func (c *Client) RecallMessage(ctx context.Context, messageURN URN) error {
	_, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMessagingDashMessengerMessagesURL).
		WithQueryParam("action", "recall").
		WithCSRF().
		WithXLIHeaders().
		WithJSONPayload(map[string]any{"messageUrn": messageURN}).
		Do(ctx, nil)
	return err
}

func (c *Client) GetMessagesBefore(ctx context.Context, conversationURN URN, before time.Time, count int) (*CollectionResponse[MessageMetadata, Message], error) {
	zerolog.Ctx(ctx).Info().
		Time("before", before).
		Msg("Getting conversations delivered before")
	var response GraphQlResponse
	_, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
		WithGraphQLQuery(graphQLQueryIDMessengerMessagesByAnchorTimestamp, map[string]string{
			"deliveredAt":     strconv.Itoa(int(before.UnixMilli())),
			"conversationUrn": url.QueryEscape(conversationURN.WithPrefix("urn", "li", "msg_conversation").String()),
			"countBefore":     strconv.Itoa(count),
			"countAfter":      "0",
		}).
		Do(ctx, &response)
	if err != nil {
		return nil, err
	}

	return response.Data.MessengerMessagesByAnchorTimestamp, nil
}

func (c *Client) GetMessagesWithPrevCursor(ctx context.Context, conversationURN URN, prevCursor string, count int) (*CollectionResponse[MessageMetadata, Message], error) {
	zerolog.Ctx(ctx).Info().
		Str("prev_cursor", prevCursor).
		Msg("Getting conversations with prev cursor")
	var response GraphQlResponse
	_, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
		WithGraphQLQuery(graphQLQueryIDMessengerMessagesByPrevCursor, map[string]string{
			"conversationUrn": url.QueryEscape(conversationURN.WithPrefix("urn", "li", "msg_conversation").String()),
			"count":           strconv.Itoa(count),
			"prevCursor":      url.QueryEscape(prevCursor),
		}).
		Do(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response.Data.MessengerMessagesByConversation, nil
}
