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

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridge/status"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"

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

func (l *LinkedInClient) onDecoratedMessage(ctx context.Context, msg *types.DecoratedMessageRealtime) {
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.Message[*types.DecoratedMessageRealtime]{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventMessage,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.
					Stringer("backend_urn", msg.Result.BackendURN).
					Stringer("sender", msg.Result.Sender.BackendURN)
			},
			PortalKey:    l.makePortalKey(msg.Result.BackendURN),
			CreatePortal: true,
			Sender:       l.makeSender(msg.Result.Sender),
			Timestamp:    msg.Result.DeliveredAt.Time,
		},
		ID:                 networkid.MessageID(msg.Result.BackendURN.ID()),
		Data:               msg,
		ConvertMessageFunc: l.convertToMatrix,
	})

	// msg.Result.Sender.EntityUrn
	// sender := message.Sender
	// isFromMe := sender.HostIdentityUrn == string(lc.userLogin.ID)
	//
	// msgType := bridgev2.RemoteEventMessage
	// switch rawEvt.(type) {
	// case event.MessageEditedEvent:
	// 	msgType = bridgev2.RemoteEventEdit
	// }
	//
	// lc.connector.br.QueueRemoteEvent(lc.userLogin, &simplevent.Message[*response.MessageElement]{
	// 	EventMeta: simplevent.EventMeta{
	// 		Type: msgType,
	// 		LogContext: func(c zerolog.Context) zerolog.Context {
	// 			return c.
	// 				Str("message_id", message.EntityUrn).
	// 				Str("sender", sender.HostIdentityUrn).
	// 				Str("sender_login", path.Base(sender.ParticipantType.Member.ProfileURL)).
	// 				Bool("is_from_me", isFromMe)
	// 		},
	// 		PortalKey:    lc.MakePortalKey(lc.threadCache[message.Conversation.EntityUrn]),
	// 		CreatePortal: false, // todo debate
	// 		Sender: bridgev2.EventSender{
	// 			IsFromMe:    isFromMe,
	// 			SenderLogin: networkid.UserLoginID(sender.HostIdentityUrn),
	// 			Sender:      networkid.UserID(sender.HostIdentityUrn),
	// 		},
	// 		Timestamp: time.UnixMilli(message.DeliveredAt),
	// 	},
	// 	ID:                 networkid.MessageID(message.EntityUrn),
	// 	TargetMessage:      networkid.MessageID(message.EntityUrn),
	// 	Data:               &message,
	// 	ConvertMessageFunc: lc.convertToMatrix,
	// 	ConvertEditFunc:    lc.convertEditToMatrix,
	// })
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
	return l.userLogin.Metadata.(*UserLoginMetadata).Cookies.GetCookie(linkedingo.LinkedInJSESSIONID) != ""
}

func (l *LinkedInClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	return l.userID == userID
}

func (l *LinkedInClient) LogoutRemote(ctx context.Context) {
	if err := l.client.Logout(ctx); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("error logging out of remote")
	}
}
