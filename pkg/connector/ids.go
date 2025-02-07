package connector

import (
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo2/types2"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

func (l *LinkedInClient) makePortalKey(backendURN types2.URN) (key networkid.PortalKey) {
	key.ID = networkid.PortalID(backendURN.ID())
	if l.main.Bridge.Config.SplitPortals {
		key.Receiver = l.userLogin.ID
	}
	return key
}

func (l *LinkedInClient) makeSender(participant types2.MessagingParticipant) (sender bridgev2.EventSender) {
	id := participant.BackendURN.ID()
	sender.IsFromMe = id == string(l.userID)
	sender.Sender = networkid.UserID(id)
	sender.SenderLogin = networkid.UserLoginID(id)
	return
}
