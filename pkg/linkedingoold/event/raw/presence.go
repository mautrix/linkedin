package raw

import (
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/event"

	"time"
)

func (p *DecoratedEventPayload) ToPresenceStatusUpdateEvent(fsdProfileId string) event.UserPresenceEvent {
	return event.UserPresenceEvent{
		FsdProfileId: fsdProfileId,
		Status:       p.Availability,
		LastActiveAt: time.UnixMilli(p.LastActiveAt),
	}
}
