package connector

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/status"
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
		ui.ExtraUpdates = makeIdentifierUpdater(participant.EntityURN.ID())
		if networkid.UserID(participant.EntityURN.ID()) == l.userID {
			ui.ExtraUpdates = bridgev2.MergeExtraUpdaters(ui.ExtraUpdates, l.makeRemoteProfileUpdater(participant.ParticipantType.Member))
		}
	case participant.ParticipantType.Organization != nil:
		ui.Name = ptr.Ptr(l.main.Config.FormatDisplayname(DisplaynameParams{
			Organization: participant.ParticipantType.Organization.Name.Text,
		}))
		ui.Avatar = l.getAvatar(participant.ParticipantType.Organization.Logo)
		ui.Identifiers = []string{}
	}
	return
}

func (l *LinkedInClient) makeRemoteProfileUpdater(member *linkedingo.MemberParticipantInfo) func(context.Context, *bridgev2.Ghost) bool {
	return func(ctx context.Context, ghost *bridgev2.Ghost) bool {
		l.remoteProfileLock.Lock()
		defer l.remoteProfileLock.Unlock()
		name := l.userLogin.RemoteProfile.Name
		if member.FirstName.Text != "" || member.LastName.Text != "" {
			name = fmt.Sprintf("%s %s", member.FirstName.Text, member.LastName.Text)
		}
		if l.userLogin.RemoteProfile.Name == name && l.userLogin.RemoteProfile.Avatar == ghost.AvatarMXC {
			return false
		}
		l.userLogin.RemoteName = name
		l.userLogin.RemoteProfile.Name = name
		l.userLogin.RemoteProfile.Avatar = ghost.AvatarMXC
		err := l.userLogin.Save(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).Msg("Failed to save remote profile")
		}
		if l.userLogin.BridgeState.GetPrevUnsent().StateEvent == status.StateConnected {
			l.userLogin.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
		}
		zerolog.Ctx(ctx).Info().Msg("Remote profile updated")
		return false
	}
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
		PowerLevels: &bridgev2.PowerLevelOverrides{
			Events: map[event.Type]int{
				event.StateRoomName: 0,
			},
		},
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

func makeIdentifiers(profileID, publicIdentifier string) []string {
	identifiers := []string{fmt.Sprintf("linkedin-internal:%s", profileID)}
	if publicIdentifier != "" {
		identifiers = append(identifiers, fmt.Sprintf("linkedin:%s", publicIdentifier))
	}
	return identifiers
}

func makeIdentifierUpdater(profileID string) bridgev2.ExtraUpdater[*bridgev2.Ghost] {
	return func(ctx context.Context, ghost *bridgev2.Ghost) bool {
		meta := ghost.Metadata.(*GhostMetadata)
		return ghost.UpdateContactInfo(ctx, makeIdentifiers(profileID, meta.PublicIdentifier), nil, nil)
	}
}

func (l *LinkedInClient) fetchPublicIdentifiers(ctx context.Context, participants []linkedingo.MessagingParticipant) {
	log := zerolog.Ctx(ctx).With().Str("action", "fetch public identifiers").Logger()
	var urns []linkedingo.URN
	ghosts := make(map[string]*bridgev2.Ghost)
	for _, participant := range participants {
		profileID := participant.EntityURN.ID()
		if participant.ParticipantType.Member == nil || !l.publicIdentifierAttempts.Add(profileID) {
			continue
		}
		ghost, err := l.main.Bridge.GetGhostByID(ctx, networkid.UserID(profileID))
		if err != nil {
			log.Err(err).Str("profile_id", profileID).Msg("Failed to get ghost")
			continue
		} else if ghost.Metadata.(*GhostMetadata).PublicIdentifier != "" {
			continue
		}
		urns = append(urns, participant.EntityURN)
		ghosts[profileID] = ghost
	}
	if len(urns) == 0 {
		return
	}
	publicIdentifiers, err := l.client.GetPublicIdentifiers(ctx, urns)
	if err != nil {
		log.Err(err).Int("profile_count", len(urns)).Msg("Failed to fetch public identifiers")
		return
	}
	for profileID, ghost := range ghosts {
		publicIdentifier := publicIdentifiers[profileID]
		if publicIdentifier == "" {
			continue
		}
		ghost.UpdateInfo(ctx, &bridgev2.UserInfo{
			ExtraUpdates: func(ctx context.Context, ghost *bridgev2.Ghost) bool {
				ghost.Metadata.(*GhostMetadata).PublicIdentifier = publicIdentifier
				ghost.UpdateContactInfo(ctx, makeIdentifiers(profileID, publicIdentifier), nil, nil)
				return true
			},
		})
	}
}
