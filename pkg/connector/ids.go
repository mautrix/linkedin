package connector

import (
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) makePortalKey(entityURN linkedingo.URN) (key networkid.PortalKey) {
	key.ID = networkid.PortalID(entityURN.String())
	if l.main.Bridge.Config.SplitPortals {
		key.Receiver = l.userLogin.ID
	}
	return key
}

func (l *LinkedInClient) makeSender(participant linkedingo.MessagingParticipant) (sender bridgev2.EventSender) {
	id := participant.EntityURN.ID()
	sender.IsFromMe = id == string(l.userID)
	sender.Sender = networkid.UserID(id)
	sender.SenderLogin = networkid.UserLoginID(id)
	return
}
