package rawold

import "go.mau.fi/mautrix-linkedin/pkg/linkedingoold/eventold"

func (p *DecoratedEventData) ToMessageEvent() eventold.MessageEvent {
	return eventold.MessageEvent{
		Message: p.DecoratedMessage.Result,
	}
}

func (p *DecoratedEventData) ToSystemMessageEvent() eventold.SystemMessageEvent {
	return eventold.SystemMessageEvent{
		Message: p.DecoratedMessage.Result,
	}
}

func (p *DecoratedEventData) ToMessageEditedEvent() eventold.MessageEditedEvent {
	return eventold.MessageEditedEvent{
		Message: p.DecoratedMessage.Result,
	}
}

func (p *DecoratedEventData) ToMessageDeleteEvent() eventold.MessageDeleteEvent {
	return eventold.MessageDeleteEvent{
		Message: p.DecoratedMessage.Result,
	}
}

func (p *DecoratedEventData) ToMessageSeenEvent() eventold.MessageSeenEvent {
	return eventold.MessageSeenEvent{
		Receipt: p.DecoratedSeenReceipt.Result,
	}
}

func (p *DecoratedEventData) ToMessageReactionEvent() eventold.MessageReactionEvent {
	return eventold.MessageReactionEvent{
		Reaction: p.DecoratedMessageReaction.Result,
	}
}

func (p *DecoratedEventData) ToTypingIndicatorEvent() eventold.TypingIndicatorEvent {
	return eventold.TypingIndicatorEvent{
		Indicator: p.DecoratedTypingIndicator.Result,
	}
}

func (p *DecoratedEventData) ToThreadUpdateEvent() eventold.ThreadUpdateEvent {
	return eventold.ThreadUpdateEvent{
		Thread: p.DecoratedUpdatedConversation.Result,
	}
}

func (p *DecoratedEventData) ToThreadDeleteEvent() eventold.ThreadDeleteEvent {
	return eventold.ThreadDeleteEvent{
		Thread: p.DecoratedDeletedConversation.Result,
	}
}
