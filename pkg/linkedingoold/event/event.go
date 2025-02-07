package event

import (
	"time"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/response"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type MessageEvent struct {
	Message response.MessageElement
}

type SystemMessageEvent struct {
	Message response.MessageElement
}

type MessageEditedEvent struct {
	Message response.MessageElement
}

type MessageDeleteEvent struct {
	Message response.MessageElement
}

type MessageSeenEvent struct {
	Receipt response.MessageSeenReceipt
}

type MessageReactionEvent struct {
	Reaction response.MessageReaction
}

type UserPresenceEvent struct {
	FsdProfileId string
	Status       typesold.PresenceAvailabilityStatus
	LastActiveAt time.Time
}

type TypingIndicatorEvent struct {
	Indicator response.TypingIndicator
}

// this event is responsible for most thread updates like:
// Title changes, archived, unarchived etc
type ThreadUpdateEvent struct {
	Thread response.ThreadElement
}

type ThreadDeleteEvent struct {
	Thread response.Conversation
}

type ConnectionReady struct{}

type ConnectionClosed struct {
	Reason typesold.ConnectionClosedReason
}
