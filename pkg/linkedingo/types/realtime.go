package types

import (
	"go.mau.fi/util/jsontime"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/response"
)

type RealtimeEvent struct {
	Heartbeat        *Heartbeat        `json:"com.linkedin.realtimefrontend.Heartbeat,omitempty"`
	ClientConnection *ClientConnection `json:"com.linkedin.realtimefrontend.ClientConnection,omitempty"`
	DecoratedEvent   *DecoratedEvent   `json:"com.linkedin.realtimefrontend.DecoratedEvent,omitempty"`
}

type Heartbeat struct{}

type ClientConnection struct {
	ID string `json:"id"`
}

type DecoratedEvent struct {
	Topic               string                `json:"topic,omitempty"`
	LeftServerAt        jsontime.UnixMilli    `json:"leftServerAt,omitempty"`
	ID                  string                `json:"id,omitempty"`
	Payload             DecoratedEventPayload `json:"payload,omitempty"`
	TrackingID          string                `json:"trackingId,omitempty"`
	PublisherTrackingID string                `json:"publisherTrackingId,omitempty"`
}

type DecoratedEventPayload struct {
	Data DecoratedEventData `json:"data,omitempty"`
	// LastActiveAt int64                            `json:"lastActiveAt,omitempty"`
	// Availability types.PresenceAvailabilityStatus `json:"availability,omitempty"`
}

type DecoratedEventData struct {
	RecipeType                   string                        `json:"_recipeType,omitempty"`
	Type                         string                        `json:"_type,omitempty"`
	DecoratedMessage             *DecoratedMessageRealtime     `json:"doDecorateMessageMessengerRealtimeDecoration,omitempty"`
	DecoratedSeenReceipt         *DecoratedSeenReceipt         `json:"doDecorateSeenReceiptMessengerRealtimeDecoration,omitempty"`
	DecoratedTypingIndicator     *DecoratedTypingIndiciator    `json:"doDecorateTypingIndicatorMessengerRealtimeDecoration,omitempty"`
	DecoratedMessageReaction     *DecoratedMessageReaction     `json:"doDecorateRealtimeReactionSummaryMessengerRealtimeDecoration,omitempty"`
	DecoratedDeletedConversation *DecoratedDeletedConversation `json:"doDecorateConversationDeleteMessengerRealtimeDecoration,omitempty"`
	DecoratedUpdatedConversation *DecoratedUpdatedConversation `json:"doDecorateConversationMessengerRealtimeDecoration,omitempty"`
}

// Conversation represents a com.linkedin.messenger.Conversation object
type Conversation struct {
	BackendURN URN `json:"backendUrn,omitempty"`
	// EntityURN                URN                    `json:"entityUrn,omitempty"`
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
	ProfilePicture VectorImage    `json:"profilePicture,omitempty"`
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

// Message represents a com.linkedin.messenger.Message object.
type Message struct {
	Body                   AttributedText       `json:"body,omitempty"`
	BackendURN             URN                  `json:"backendUrn,omitempty"`
	DeliveredAt            jsontime.UnixMilli   `json:"deliveredAt,omitempty"`
	EntityURN              URN                  `json:"entityUrn,omitempty"`
	Sender                 MessagingParticipant `json:"sender,omitempty"`
	BackendConversationURN URN                  `json:"backendConversationUrn,omitempty"`
	Conversation           Conversation         `json:"conversation,omitempty"`
}

type DecoratedMessageRealtime struct {
	Result     Message `json:"result,omitempty"`
	RecipeType string  `json:"_recipeType,omitempty"`
	Type       string  `json:"_type,omitempty"`
}

type DecoratedSeenReceipt struct {
	Result     response.MessageSeenReceipt `json:"result,omitempty"`
	RecipeType string                      `json:"_recipeType,omitempty"`
	Type       string                      `json:"_type,omitempty"`
}

type DecoratedTypingIndiciator struct {
	Result     response.TypingIndicator `json:"result,omitempty"`
	RecipeType string                   `json:"_recipeType,omitempty"`
	Type       string                   `json:"_type,omitempty"`
}

type DecoratedMessageReaction struct {
	Result     response.MessageReaction `json:"result,omitempty"`
	RecipeType string                   `json:"_recipeType,omitempty"`
	Type       string                   `json:"_type,omitempty"`
}

type DecoratedDeletedConversation struct {
	Result     response.Conversation `json:"result,omitempty"`
	RecipeType string                `json:"_recipeType,omitempty"`
	Type       string                `json:"_type,omitempty"`
}

type DecoratedUpdatedConversation struct {
	Result     response.ThreadElement `json:"result,omitempty"`
	RecipeType string                 `json:"_recipeType,omitempty"`
	Type       string                 `json:"_type,omitempty"`
}
