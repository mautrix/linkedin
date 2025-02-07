package raw

import (
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/responseold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type DecoratedEventResponse struct {
	Topic               string                `json:"topic,omitempty"`
	PublisherTrackingID string                `json:"publisherTrackingId,omitempty"`
	LeftServerAt        int64                 `json:"leftServerAt,omitempty"`
	ID                  string                `json:"id,omitempty"`
	Payload             DecoratedEventPayload `json:"payload,omitempty"`
	TrackingID          string                `json:"trackingId,omitempty"`
}

type DecoratedEventPayload struct {
	Data         DecoratedEventData                  `json:"data,omitempty"`
	Metadata     Metadata                            `json:"$metadata,omitempty"`
	LastActiveAt int64                               `json:"lastActiveAt,omitempty"`
	Availability typesold.PresenceAvailabilityStatus `json:"availability,omitempty"`
}

type DecoratedMessageRealtime struct {
	Result     responseold.MessageElement `json:"result,omitempty"`
	RecipeType string                     `json:"_recipeType,omitempty"`
	Type       string                     `json:"_type,omitempty"`
}

type DecoratedSeenReceipt struct {
	Result     responseold.MessageSeenReceipt `json:"result,omitempty"`
	RecipeType string                         `json:"_recipeType,omitempty"`
	Type       string                         `json:"_type,omitempty"`
}

type DecoratedTypingIndiciator struct {
	Result     responseold.TypingIndicator `json:"result,omitempty"`
	RecipeType string                      `json:"_recipeType,omitempty"`
	Type       string                      `json:"_type,omitempty"`
}

type DecoratedMessageReaction struct {
	Result     responseold.MessageReaction `json:"result,omitempty"`
	RecipeType string                      `json:"_recipeType,omitempty"`
	Type       string                      `json:"_type,omitempty"`
}

type DecoratedDeletedConversation struct {
	Result     responseold.Conversation `json:"result,omitempty"`
	RecipeType string                   `json:"_recipeType,omitempty"`
	Type       string                   `json:"_type,omitempty"`
}

type DecoratedUpdatedConversation struct {
	Result     responseold.ThreadElement `json:"result,omitempty"`
	RecipeType string                    `json:"_recipeType,omitempty"`
	Type       string                    `json:"_type,omitempty"`
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

type Metadata struct{}
