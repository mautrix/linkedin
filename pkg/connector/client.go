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

	"github.com/rs/zerolog"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo2"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

type LinkedInClient struct {
	main      *LinkedInConnector
	userID    networkid.UserID
	userLogin *bridgev2.UserLogin
	client    *linkedingo2.Client
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
	client := linkedingo2.NewClient(ctx, login.Metadata.(*UserLoginMetadata).Cookies)
	return &LinkedInClient{
		main:      lc,
		userID:    userID,
		userLogin: login,
		client:    client,
	}
}

func (l *LinkedInClient) Connect(ctx context.Context) {
	// DEBUG
	// profile, err := l.client.GetCurrentUserProfile(ctx)
	// if err != nil {
	// 	fmt.Printf("%+v\n", err)
	// 	panic("failed to get profile")
	// }
	// fmt.Printf("%s\n", exerrors.Must(json.Marshal(profile)))
}

func (l *LinkedInClient) Disconnect() {
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
	return l.userLogin.Metadata.(*UserLoginMetadata).Cookies.GetCookie(linkedingo2.LinkedInJSESSIONID) != ""
}

func (l *LinkedInClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	panic("unimplemented")
}

func (l *LinkedInClient) LogoutRemote(ctx context.Context) {
	if err := l.client.Logout(ctx); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("error logging out of remote")
	}
}
