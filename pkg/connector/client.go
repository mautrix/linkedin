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

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/status"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/connector/linkedinfmt"
	"go.mau.fi/mautrix-linkedin/pkg/connector/matrixfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

type LinkedInClient struct {
	main      *LinkedInConnector
	userID    networkid.UserID
	userLogin *bridgev2.UserLogin
	client    *linkedingo.Client

	sessID               uuid.UUID
	conversationLastRead map[linkedingo.URN]jsontime.UnixMilli

	linkedinFmtParams linkedinfmt.FormatParams
	matrixParser      *matrixfmt.HTMLParser
}

var (
	_ bridgev2.NetworkAPI = (*LinkedInClient)(nil)
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
		main:                 lc,
		userID:               userID,
		userLogin:            login,
		conversationLastRead: map[linkedingo.URN]jsontime.UnixMilli{},
	}
	client.client = linkedingo.NewClient(
		ctx,
		linkedingo.NewURN(login.ID),
		login.Metadata.(*UserLoginMetadata).Cookies,
		login.Metadata.(*UserLoginMetadata).XLIPageInstance,
		login.Metadata.(*UserLoginMetadata).XLITrack,
		linkedingo.Handlers{
			Heartbeat: func(ctx context.Context) {
				if login.BridgeState.GetPrevUnsent().StateEvent != status.StateConnected {
					login.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
				}
			},
			ClientConnection: func(ctx context.Context, conn *linkedingo.ClientConnection) {
				login.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})

				if client.sessID != conn.SessID {
					go client.syncConversations(ctx)
					client.sessID = conn.ID
				}
			},
			TransientDisconnect: client.onTransientDisconnect,
			BadCredentials:      client.onBadCredentials,
			UnknownError:        client.onUnknownError,
			DecoratedEvent:      client.onDecoratedEvent,
		},
	)

	client.linkedinFmtParams = linkedinfmt.FormatParams{
		GetMXIDByURN: func(ctx context.Context, entityURN linkedingo.URN) (id.UserID, error) {
			ghost, err := lc.Bridge.GetGhostByID(ctx, networkid.UserID(entityURN.ID()))
			if err != nil {
				return "", err
			}
			// FIXME this should look for user logins by ID, not hardcode the current user
			if networkid.UserID(entityURN.ID()) == client.userID {
				return client.userLogin.UserMXID, nil
			}
			return ghost.Intent.GetMXID(), nil
		},
	}
	client.matrixParser = &matrixfmt.HTMLParser{
		GetGhostDetails: func(ctx context.Context, ui id.UserID) (networkid.UserID, string, bool) {
			if ui == client.userLogin.UserMXID {
				return client.userID, client.userLogin.RemoteName, true
			}
			if userID, ok := lc.Bridge.Matrix.ParseGhostMXID(ui); !ok {
				return "", "", false
			} else if ghost, err := lc.Bridge.DB.Ghost.GetByID(ctx, userID); err != nil {
				return "", "", false
			} else {
				return userID, ghost.Name, true
			}
		},
	}
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
			StateEvent: status.StateBadCredentials,
			Error:      "linkedin-logged-out",
			Message:    fmt.Sprintf("Failed to connect to the realtime stream: %v", err),
		})
	}
}

func (l *LinkedInClient) Disconnect() {
	l.client.RealtimeDisconnect()
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
