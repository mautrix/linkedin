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
	"net/http"
	"regexp"
	"strings"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/status"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

const FlowIDCookies = "cookies"

func (lc *LinkedInConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{
		{
			Name:        "Email and password",
			Description: "Sign in with your LinkedIn email or phone number and password",
			ID:          FlowIDPassword,
		},
		{
			Name:        "Cookies",
			Description: "Log in with your LinkedIn account using your cookies",
			ID:          FlowIDCookies,
		},
	}
}

func (l *LinkedInConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	switch flowID {
	case FlowIDPassword:
		return newPasswordLogin(user, l), nil
	case FlowIDCookies:
		return &CookieLogin{user: user, main: l}, nil
	default:
		return nil, bridgev2.ErrInvalidLoginFlowID
	}
}

type CookieLogin struct {
	user *bridgev2.User
	main *LinkedInConnector
}

var (
	CookieLoginStepIDCookies  = "fi.mau.linkedin.login.enter_cookies"
	CookieLoginStepIDComplete = "fi.mau.linkedin.login.complete"

	CookieLoginCookieHeaderField    = "fi.mau.linkedin.login.cookie_header"
	CookieLoginXLITrackField        = "fi.mau.linkedin.login.x_li_track"
	CookieLoginXLIPageInstanceField = "fi.mau.linkedin.login.x_li_page_instance"
)

var _ bridgev2.LoginProcessCookies = (*CookieLogin)(nil)

const cookieLoginBrowserHeaderPrefix = "fi.mau.linkedin.login.header."

var browserHeaderNames = []string{
	"User-Agent",
	"Sec-Ch-Ua",
	"Sec-Ch-Ua-Platform",
	"Sec-Ch-Ua-Mobile",
	"Sec-Ch-Prefers-Color-Scheme",
}

func (c *CookieLogin) Cancel() {}

func (c *CookieLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	step := &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeCookies,
		StepID:       CookieLoginStepIDCookies,
		Instructions: "Enter a JSON object with your cookies, or a cURL command copied from browser devtools. It is recommended that you use a tab opened in Incognito/Private browsing mode and close the browser **before** pasting the cookies.",
		CookiesParams: &bridgev2.LoginCookiesParams{
			URL:       "https://linkedin.com/login",
			ExtractJS: fmt.Sprintf(`Promise.resolve({%q: navigator.userAgent})`, cookieLoginBrowserHeaderPrefix+"user-agent"),
			// Keep the webview open until the feed loads so it doesn't close
			// before the user finishes 2FA.
			WaitForURLPattern: `^https://www\.linkedin\.com/feed.*$`,
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
				{
					ID:       CookieLoginXLITrackField,
					Required: true,
					Sources: []bridgev2.LoginCookieFieldSource{
						{
							Type:            bridgev2.LoginCookieTypeRequestHeader,
							Name:            "X-LI-Track",
							RequestURLRegex: "https://www.linkedin.com",
						},
					},
					Pattern: "clientVersion",
				},
				{
					ID:       CookieLoginXLIPageInstanceField,
					Required: true,
					Sources: []bridgev2.LoginCookieFieldSource{
						{
							Type:            bridgev2.LoginCookieTypeRequestHeader,
							Name:            "X-LI-Page-Instance",
							RequestURLRegex: "https://www.linkedin.com",
						},
					},
					Pattern: "urn:li:page",
				},
			},
		},
	}
	for _, name := range browserHeaderNames {
		field := bridgev2.LoginCookieField{
			ID: cookieLoginBrowserHeaderPrefix + strings.ToLower(name),
			Sources: []bridgev2.LoginCookieFieldSource{{
				Type:            bridgev2.LoginCookieTypeRequestHeader,
				Name:            name,
				RequestURLRegex: `^https://www\.linkedin\.com/`,
			}},
		}
		if name == "User-Agent" {
			field.Sources = append(field.Sources, bridgev2.LoginCookieFieldSource{
				Type: bridgev2.LoginCookieTypeSpecial,
				Name: field.ID,
			})
		}
		step.CookiesParams.Fields = append(step.CookiesParams.Fields, field)
	}
	return step, nil
}

var gStateRegex = regexp.MustCompile(`g_state={.*?};`)

func (c *CookieLogin) SubmitCookies(ctx context.Context, cookies map[string]string) (*bridgev2.LoginStep, error) {
	cookieStr := cookies[CookieLoginCookieHeaderField]
	cookieStr = gStateRegex.ReplaceAllString(cookieStr, "")
	jar, err := linkedingo.NewJarFromCookieHeader(cookieStr)
	if err != nil {
		return nil, err
	}

	pageInstance := cookies[CookieLoginXLIPageInstanceField]
	xLiTrack := cookies[CookieLoginXLITrackField]
	headers := make(http.Header)
	for _, name := range browserHeaderNames {
		if value := cookies[cookieLoginBrowserHeaderPrefix+strings.ToLower(name)]; value != "" {
			headers.Set(name, value)
		}
	}

	return completeLinkedInLogin(ctx, c.user, jar, pageInstance, xLiTrack, headers)
}

func completeLinkedInLogin(ctx context.Context, user *bridgev2.User, jar *linkedingo.StringCookieJar, pageInstance, xLiTrack string, headers http.Header) (*bridgev2.LoginStep, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	loginClient := linkedingo.NewClient(ctx, linkedingo.NewURN(""), jar, pageInstance, xLiTrack, "", linkedingo.Handlers{})
	loginClient.SetBrowserHeaders(headers)
	profile, err := loginClient.GetCurrentUserProfile(ctx)
	if err != nil {
		return nil, wrapLinkedInLoginError(err)
	}
	if profile.MiniProfile.EntityURN.ID() == "" {
		return nil, ErrLoginBadCookies
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}

	remoteName := fmt.Sprintf("%s %s", profile.MiniProfile.FirstName, profile.MiniProfile.LastName)
	ul, err := user.NewLogin(
		ctx,
		&database.UserLogin{
			ID: networkid.UserLoginID(profile.MiniProfile.EntityURN.ID()),
			Metadata: &UserLoginMetadata{
				Cookies:         jar,
				XLIPageInstance: pageInstance,
				XLITrack:        xLiTrack,
				BrowserHeaders:  headers,
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
