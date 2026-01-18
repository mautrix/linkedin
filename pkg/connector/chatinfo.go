package connector

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var moderatorPL = 50

func (l *LinkedInClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	// This is not supported. All of the info should already be populated with
	// the information we get on a per-message basis.
	zerolog.Ctx(ctx).Warn().Msg("GetChatInfo called")
	return nil, nil
}

func (l *LinkedInClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	// This is not supported. All of the info should already be populated with
	// the information we get on a per-message basis.
	zerolog.Ctx(ctx).Warn().Msg("GetUserInfo called")
	return nil, nil
}

func (l *LinkedInClient) getAvatar(img *linkedingo.VectorImage) (avatar *bridgev2.Avatar) {
	if img == nil {
		return nil
	}
	return &bridgev2.Avatar{
		ID:     networkid.AvatarID(img.RootURL),
		Remove: img.RootURL == "",
		Get: func(ctx context.Context) ([]byte, error) {
			return l.client.DownloadBytes(ctx, img.GetLargestArtifactURL())
		},
	}
}

func (l *LinkedInClient) getMessagingParticipantUserInfo(participant linkedingo.MessagingParticipant) (ui bridgev2.UserInfo) {
	switch {
	case participant.ParticipantType.Member != nil:
		ui.Name = ptr.Ptr(l.main.Config.FormatDisplayname(DisplaynameParams{
			FirstName: participant.ParticipantType.Member.FirstName.Text,
			LastName:  participant.ParticipantType.Member.LastName.Text,
		}))
		ui.Avatar = l.getAvatar(participant.ParticipantType.Member.ProfilePicture)
		ui.Identifiers = []string{fmt.Sprintf("linkedin:%s", participant.EntityURN.ID())}
	case participant.ParticipantType.Organization != nil:
		ui.Name = ptr.Ptr(l.main.Config.FormatDisplayname(DisplaynameParams{
			Organization: participant.ParticipantType.Organization.Name.Text,
		}))
		ui.Avatar = l.getAvatar(participant.ParticipantType.Organization.Logo)
		ui.Identifiers = []string{fmt.Sprintf("linkedin:%s", participant.EntityURN.ID())}
	}
	return
}

func (l *LinkedInClient) conversationToChatInfo(conv linkedingo.Conversation) (ci bridgev2.ChatInfo, userInChat bool) {
	if conv.Title != "" {
		ci.Name = &conv.Title
	}

	// TODO: topic is probably headlineText of the conversation, or set it to the headline of the other user in the chat

	ci.Type = ptr.Ptr(database.RoomTypeDM)
	if conv.GroupChat {
		ci.Type = ptr.Ptr(database.RoomTypeDefault)
	}

	ci.CanBackfill = true

	ci.Members = &bridgev2.ChatMemberList{
		IsFull:           true,
		TotalMemberCount: len(conv.ConversationParticipants),
		MemberMap:        map[networkid.UserID]bridgev2.ChatMember{},
	}
	for _, participant := range conv.ConversationParticipants {
		userInChat = userInChat || networkid.UserID(participant.EntityURN.ID()) == l.userID
		sender := l.makeSender(participant)
		powerLevel := 0
		if sender.IsFromMe {
			powerLevel = moderatorPL
		}
		ci.Members.MemberMap[sender.Sender] = bridgev2.ChatMember{
			EventSender: sender,
			Membership:  event.MembershipJoin,
			UserInfo:    ptr.Ptr(l.getMessagingParticipantUserInfo(participant)),
			PowerLevel:  &powerLevel,
		}
	}

	return
}
