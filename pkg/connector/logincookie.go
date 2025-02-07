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

	"maunium.net/go/mautrix/bridge/status"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
	"go.mau.fi/mautrix-linkedin/pkg/stringcookiejar"
)

type CookieLogin struct {
	user *bridgev2.User
	main *LinkedInConnector
}

var (
	CookieLoginStepIDCookies  = "fi.mau.linkedin.login.enter_cookies"
	CookieLoginStepIDComplete = "fi.mau.linkedin.login.complete"

	CookieLoginCookieHeaderField = "fi.mau.linkedin.login.cookie_header"
)

var _ bridgev2.LoginProcessCookies = (*CookieLogin)(nil)

func (c *CookieLogin) Cancel() {}

func (c *CookieLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeCookies,
		StepID:       CookieLoginStepIDCookies,
		Instructions: "Enter a JSON object with your cookies, or a cURL command copied from browser devtools. It is recommended that you use a tab opened in Incognito/Private browsing mode and close the browser **before** pasting the cookies.",
		CookiesParams: &bridgev2.LoginCookiesParams{
			URL: "https://linkedin.com/login",
			Fields: []bridgev2.LoginCookieField{
				{
					ID:       CookieLoginCookieHeaderField,
					Required: true,
					Sources: []bridgev2.LoginCookieFieldSource{
						{
							Type:            bridgev2.LoginCookieTypeRequestHeader,
							Name:            "Cookie",
							RequestURLRegex: "https://www.linkedin.com",
						},
					},
					Pattern: `\bJSESSIONID=[^;]+`,
				},
			},
		},
	}, nil
}

func (c *CookieLogin) SubmitCookies(ctx context.Context, cookies map[string]string) (*bridgev2.LoginStep, error) {
	jar, err := stringcookiejar.NewJarFromCookieHeader(cookies[CookieLoginCookieHeaderField])
	if err != nil {
		return nil, err
	}

	loginClient := linkedingo.NewClient(ctx, types.NewURN(""), jar, linkedingo.Handlers{})
	profile, err := loginClient.GetCurrentUserProfile(ctx)
	if err != nil {
		return nil, err
	}

	remoteName := fmt.Sprintf("%s %s", profile.MiniProfile.FirstName, profile.MiniProfile.LastName)
	ul, err := c.user.NewLogin(
		ctx,
		&database.UserLogin{
			ID: networkid.UserLoginID(profile.MiniProfile.ObjectURN.ID()),
			Metadata: &UserLoginMetadata{
				Cookies:   jar,
				EntityURN: profile.MiniProfile.EntityURN,
			},
			RemoteName: remoteName,
			RemoteProfile: status.RemoteProfile{
				Name: remoteName,
				// Avatar: mxcURI,
			},
		},
		&bridgev2.NewLoginParams{
			DeleteOnConflict: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save new login: %w", err)
	}
	ul.Client.Connect(ul.Log.WithContext(context.Background()))

	return &bridgev2.LoginStep{
		Type:           bridgev2.LoginStepTypeComplete,
		StepID:         CookieLoginStepIDComplete,
		Instructions:   fmt.Sprintf("Successfully logged in as %s", remoteName),
		CompleteParams: &bridgev2.LoginCompleteParams{UserLoginID: ul.ID, UserLogin: ul},
	}, nil
}
