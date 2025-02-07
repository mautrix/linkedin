package connector

import (
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (l *LinkedInClient) makePortalKey(backendURN types.URN) (key networkid.PortalKey) {
	key.ID = networkid.PortalID(backendURN.ID())
	if l.main.Bridge.Config.SplitPortals {
		key.Receiver = l.userLogin.ID
	}
	return key
}

func (l *LinkedInClient) makeSender(participant types.MessagingParticipant) (sender bridgev2.EventSender) {
	id := participant.BackendURN.ID()
	sender.IsFromMe = id == string(l.userID)
	sender.Sender = networkid.UserID(id)
	sender.SenderLogin = networkid.UserLoginID(id)
	return
}
