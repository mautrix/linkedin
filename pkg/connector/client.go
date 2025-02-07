// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2025 Sumner Evans
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package connector

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridge/status"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

type LinkedInClient struct {
	main      *LinkedInConnector
	userID    networkid.UserID
	userLogin *bridgev2.UserLogin
	client    *linkedingo.Client
}

var (
	_ bridgev2.NetworkAPI = (*LinkedInClient)(nil)
	// _ bridgev2.EditHandlingNetworkAPI          = (*LinkedInClient)(nil)
	// _ bridgev2.ReactionHandlingNetworkAPI      = (*LinkedInClient)(nil)
	// _ bridgev2.RedactionHandlingNetworkAPI     = (*LinkedInClient)(nil)
	// _ bridgev2.ReadReceiptHandlingNetworkAPI   = (*LinkedInClient)(nil)
	// _ bridgev2.TypingHandlingNetworkAPI        = (*LinkedInClient)(nil)
	// _ bridgev2.BackfillingNetworkAPI           = (*LinkedInClient)(nil)
	// _ bridgev2.BackfillingNetworkAPIWithLimits = (*LinkedInClient)(nil)
	// _ bridgev2.IdentifierResolvingNetworkAPI   = (*LinkedInClient)(nil)
	// _ bridgev2.ContactListingNetworkAPI        = (*LinkedInClient)(nil)
	// _ bridgev2.UserSearchingNetworkAPI         = (*LinkedInClient)(nil)
	// _ bridgev2.GroupCreatingNetworkAPI         = (*LinkedInClient)(nil)
	// _ bridgev2.MuteHandlingNetworkAPI          = (*LinkedInClient)(nil)
	// _ bridgev2.TagHandlingNetworkAPI           = (*LinkedInClient)(nil)
)

func NewLinkedInClient(ctx context.Context, lc *LinkedInConnector, login *bridgev2.UserLogin) *LinkedInClient {
	userID := networkid.UserID(login.ID)
	client := &LinkedInClient{
		main:      lc,
		userID:    userID,
		userLogin: login,
	}
	client.client = linkedingo.NewClient(ctx, login.Metadata.(*UserLoginMetadata).Cookies, linkedingo.Handlers{
		Heartbeat: func(ctx context.Context) {
			login.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
		},
		ClientConnection: func(context.Context, *types.ClientConnection) {
			login.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
		},
		RealtimeConnectError: client.onRealtimeConnectError,
		DecoratedMessage:     client.onDecoratedMessage,
	})
	return client
}

func (l *LinkedInClient) Connect(ctx context.Context) {
	if !l.IsLoggedIn() {
		zerolog.Ctx(ctx).Warn().Msg("user is not logged in, sending bad credentials state")
		l.userLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "linkedin-no-auth",
			Message:    "User does not have the necessary cookies",
		})
		return
	}

	if err := l.client.RealtimeConnect(ctx); err != nil {
		l.userLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateUnknownError,
			Error:      "linkedin-realtime-connect-failed",
			Message:    fmt.Sprintf("Failed to connect to the realtime stream: %v", err),
		})
	}
}

func (l *LinkedInClient) onRealtimeConnectError(ctx context.Context, err error) {
	zerolog.Ctx(ctx).Err(err).Msg("failed to read from event stream")
}

func (l *LinkedInClient) onDecoratedMessage(ctx context.Context, msg *types.Message) {
	meta := simplevent.EventMeta{
		LogContext: func(c zerolog.Context) zerolog.Context {
			return c.
				Stringer("backend_urn", msg.BackendURN).
				Stringer("sender", msg.Sender.BackendURN)
		},
		PortalKey:    l.makePortalKey(msg.BackendURN),
		CreatePortal: true,
		Sender:       l.makeSender(msg.Sender),
		Timestamp:    msg.DeliveredAt.Time,
	}

	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatResync{
		EventMeta:       meta.WithType(bridgev2.RemoteEventChatResync),
		ChatInfo:        ptr.Ptr(l.conversationToChatInfo(msg.Conversation)),
		LatestMessageTS: msg.DeliveredAt.Time,
	})

	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.Message[*types.Message]{
		EventMeta:          meta.WithType(bridgev2.RemoteEventMessage),
		ID:                 networkid.MessageID(msg.BackendURN.ID()),
		Data:               msg,
		ConvertMessageFunc: l.convertToMatrix,
	})
}

func (l *LinkedInClient) getAvatar(img types.VectorImage) (avatar bridgev2.Avatar) {
	avatar.ID = networkid.AvatarID(img.RootURL)
	avatar.Remove = img.RootURL == ""
	avatar.Get = func(ctx context.Context) ([]byte, error) {
		var largestVersion types.VectorArtifact
		for _, a := range img.Artifacts {
			if a.Height > largestVersion.Height {
				largestVersion = a
			}
		}

		resp, err := http.DefaultClient.Get(img.RootURL + largestVersion.FileIdentifyingURLPathSegment)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(resp.Body)
	}
	return
}

func (l *LinkedInClient) getMessagingParticipantUserInfo(participant types.MessagingParticipant) (ui bridgev2.UserInfo) {
	ui.Name = ptr.Ptr(fmt.Sprintf("%s %s", participant.ParticipantType.Member.FirstName.Text, participant.ParticipantType.Member.LastName.Text)) // TODO use a displayname template
	ui.Avatar = ptr.Ptr(l.getAvatar(participant.ParticipantType.Member.ProfilePicture))
	ui.Identifiers = []string{fmt.Sprintf("linkedin:%s", participant.BackendURN.ID())}
	return
}

func (l *LinkedInClient) conversationToChatInfo(conv types.Conversation) (ci bridgev2.ChatInfo) {
	if conv.Title != "" {
		ci.Name = &conv.Title
	}

	// TODO: topic is probably headlineText of the conversation, or set it to the headline of the other user in the chat

	// TODO: avatar

	ci.Type = ptr.Ptr(database.RoomTypeDM)
	if conv.GroupChat {
		ci.Type = ptr.Ptr(database.RoomTypeGroupDM)
	} else {
	}

	ci.CanBackfill = true

	ci.Members = &bridgev2.ChatMemberList{
		IsFull:           true,
		TotalMemberCount: len(conv.ConversationParticipants),
		MemberMap:        map[networkid.UserID]bridgev2.ChatMember{},
	}
	for _, participant := range conv.ConversationParticipants {
		sender := l.makeSender(participant)
		ci.Members.MemberMap[sender.Sender] = bridgev2.ChatMember{
			EventSender: sender,
			Membership:  event.MembershipJoin,
			UserInfo:    ptr.Ptr(l.getMessagingParticipantUserInfo(participant)),
		}
	}

	return
}

func (l *LinkedInClient) Disconnect() {
	l.client.RealtimeDisconnect()
}

func (l *LinkedInClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	panic("unimplemented")
}

func (l *LinkedInClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	panic("unimplemented")
}

func (l *LinkedInClient) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (message *bridgev2.MatrixMessageResponse, err error) {
	panic("unimplemented")
}

func (l *LinkedInClient) IsLoggedIn() bool {
	return l.userLogin.Metadata.(*UserLoginMetadata).Cookies.GetCookie(linkedingo.LinkedInCookieJSESSIONID) != ""
}

func (l *LinkedInClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	return l.userID == userID
}

func (l *LinkedInClient) LogoutRemote(ctx context.Context) {
	if err := l.client.Logout(ctx); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("error logging out of remote")
	}
}
