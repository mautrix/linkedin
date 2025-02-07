package eventold

import (
	"time"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/responseold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type MessageEvent struct {
	Message responseold.MessageElement
}

type SystemMessageEvent struct {
	Message responseold.MessageElement
}

type MessageEditedEvent struct {
	Message responseold.MessageElement
}

type MessageDeleteEvent struct {
	Message responseold.MessageElement
}

type MessageSeenEvent struct {
	Receipt responseold.MessageSeenReceipt
}

type MessageReactionEvent struct {
	Reaction responseold.MessageReaction
}

type UserPresenceEvent struct {
	FsdProfileId string
	Status       typesold.PresenceAvailabilityStatus
	LastActiveAt time.Time
}

type TypingIndicatorEvent struct {
	Indicator responseold.TypingIndicator
}

// this event is responsible for most thread updates like:
// Title changes, archived, unarchived etc
type ThreadUpdateEvent struct {
	Thread responseold.ThreadElement
}

type ThreadDeleteEvent struct {
	Thread responseold.Conversation
}

type ConnectionReady struct{}

type ConnectionClosed struct {
	Reason typesold.ConnectionClosedReason
}
