package types

import (
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

// Conversation represents a com.linkedin.messenger.Conversation object
type Conversation struct {
	Title                    string                 `json:"title,omitempty"`
	EntityURN                URN                    `json:"entityUrn,omitempty"`
	GroupChat                bool                   `json:"groupChat,omitempty"`
	ConversationParticipants []MessagingParticipant `json:"conversationParticipants,omitempty"`
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
	EntityURN       URN             `json:"entityUrn,omitempty"`
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

// RepliedMessage represents a com.linkedin.messenger.RepliedMessage object.
type RepliedMessage struct {
	OriginalMessage Message `json:"originalMessage,omitempty"`
}

// RealtimeTypingIndicator represents a
// com.linkedin.messenger.RealtimeTypingIndicator object.
type RealtimeTypingIndicator struct {
	TypingParticipant MessagingParticipant `json:"typingParticipant,omitempty"`
	Conversation      Conversation         `json:"conversation,omitempty"`
}

// SeenReceipt represents a com.linkedin.messenger.SeenReceipt object.
type SeenReceipt struct {
	SeenAt            jsontime.UnixMilli   `json:"seenAt,omitempty"`
	Message           Message              `json:"message,omitempty"`
	SeenByParticipant MessagingParticipant `json:"seenByParticipant,omitempty"`
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
