package types

import (
	"github.com/google/uuid"
	"go.mau.fi/util/jsontime"
)

type RealtimeEvent struct {
	Heartbeat        *Heartbeat        `json:"com.linkedin.realtimefrontend.Heartbeat,omitempty"`
	ClientConnection *ClientConnection `json:"com.linkedin.realtimefrontend.ClientConnection,omitempty"`
	DecoratedEvent   *DecoratedEvent   `json:"com.linkedin.realtimefrontend.DecoratedEvent,omitempty"`
}

type Heartbeat struct{}

type ClientConnection struct {
	ID uuid.UUID `json:"id"`
}

type DecoratedEvent struct {
	Topic               URN                   `json:"topic,omitempty"`
	LeftServerAt        jsontime.UnixMilli    `json:"leftServerAt,omitempty"`
	ID                  string                `json:"id,omitempty"`
	Payload             DecoratedEventPayload `json:"payload,omitempty"`
	TrackingID          string                `json:"trackingId,omitempty"`
	PublisherTrackingID string                `json:"publisherTrackingId,omitempty"`
}

type DecoratedEventPayload struct {
	Data DecoratedEventData `json:"data,omitempty"`
}

type DecoratedEventData struct {
	Type             string          `json:"_type,omitempty"`
	DecoratedMessage *ActionResponse `json:"doDecorateMessageMessengerRealtimeDecoration,omitempty"`
}

// Conversation represents a com.linkedin.messenger.Conversation object
type Conversation struct {
	Title      string `json:"title,omitempty"`
	BackendURN URN    `json:"backendUrn,omitempty"`
	// EntityURN                URN                    `json:"entityUrn,omitempty"`
	GroupChat                bool                   `json:"groupChat,omitempty"`
	ConversationParticipants []MessagingParticipant `json:"conversationParticipants,omitempty"`
}

// AttributedText represents a com.linkedin.pemberly.text.AttributedText
// object.
type AttributedText struct {
	Text string `json:"text,omitempty"`
}

// MemberParticipantInfo represents a
// com.linkedin.messenger.MemberParticipantInfo object.
type MemberParticipantInfo struct {
	ProfileURL     string         `json:"profileUrl,omitempty"`
	FirstName      AttributedText `json:"firstName,omitempty"`
	LastName       AttributedText `json:"lastName,omitempty"`
	ProfilePicture *VectorImage   `json:"profilePicture,omitempty"`
	Pronoun        any            `json:"pronoun,omitempty"`
	Headline       AttributedText `json:"headline,omitempty"`
}

type ParticipantType struct {
	Member MemberParticipantInfo `json:"member,omitempty"`
}

// MessagingParticipant represents a
// com.linkedin.messenger.MessagingParticipant object.
type MessagingParticipant struct {
	ParticipantType ParticipantType `json:"participantType,omitempty"`
	BackendURN      URN             `json:"backendUrn,omitempty"`
}

type MessageBodyRenderFormat string

const (
	MessageBodyRenderFormatDefault  MessageBodyRenderFormat = "DEFAULT"
	MessageBodyRenderFormatEdited   MessageBodyRenderFormat = "EDITED"
	MessageBodyRenderFormatRecalled MessageBodyRenderFormat = "RECALLED"
	MessageBodyRenderFormatSystem   MessageBodyRenderFormat = "SYSTEM"
)

type RenderContent struct {
	VectorImage *VectorImage    `json:"vectorImage,omitempty"`
	File        *FileAttachment `json:"file,omitempty"`
}

// Message represents a com.linkedin.messenger.Message object.
type Message struct {
	Body                    AttributedText          `json:"body,omitempty"`
	BackendURN              URN                     `json:"backendUrn,omitempty"`
	DeliveredAt             jsontime.UnixMilli      `json:"deliveredAt,omitempty"`
	EntityURN               URN                     `json:"entityUrn,omitempty"`
	Sender                  MessagingParticipant    `json:"sender,omitempty"`
	MessageBodyRenderFormat MessageBodyRenderFormat `json:"messageBodyRenderFormat,omitempty"`
	BackendConversationURN  URN                     `json:"backendConversationUrn,omitempty"`
	Conversation            Conversation            `json:"conversation,omitempty"`
	RenderContent           []RenderContent         `json:"renderContent,omitempty"`
}

// ActionResponse represents a com.linkedin.restli.common.ActionResponse
// object.
type ActionResponse struct {
	Result Message `json:"result,omitempty"`
}
