package rawold

import (
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/eventold"

	"time"
)

func (p *DecoratedEventPayload) ToPresenceStatusUpdateEvent(fsdProfileId string) eventold.UserPresenceEvent {
	return eventold.UserPresenceEvent{
		FsdProfileId: fsdProfileId,
		Status:       p.Availability,
		LastActiveAt: time.UnixMilli(p.LastActiveAt),
	}
}
