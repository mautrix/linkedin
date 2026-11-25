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
	"strings"

	"go.mau.fi/util/l10n"
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
			Name:        msgFlowCookiesName.Message(),
			Description: msgFlowCookiesDescription.Message(),
			ID:          FlowIDCookies,
		},
		{
			Name:        msgFlowPasswordName.Message(),
			Description: msgFlowPasswordDescription.Message(),
			ID:          FlowIDPassword,
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

// Hides third-party sign-in buttons, which don't work in webviews, and keeps the login screen sized to the viewport.
const cookieLoginExtractJS = `(() => {
	if (!window.__mautrixLinkedInLogin) {
		window.__mautrixLinkedInLogin = true;
		const hide = el => el.style.setProperty("display", "none", "important");
		const hideSSO = () => {
			const starts = [
				...document.querySelectorAll('iframe[src*="accounts.google.com/gsi"]'),
				...[...document.querySelectorAll("button")].filter(b => b.textContent.length < 60 && /\b(apple|passkey)\b/i.test(b.textContent)),
			];
			for (const start of starts) {
				let box = start;
				while (box.parentElement && !box.parentElement.querySelector("input")) box = box.parentElement;
				if (!box.parentElement || box.style.display === "none") continue;
				hide(box);
				const divider = box.nextElementSibling;
				if (divider && !divider.querySelector("input") && divider.textContent.trim().toLowerCase() === "or") hide(divider);
			}
		};
		const fixHeight = () => {
			const screen = document.querySelector("[data-sdui-screen]")?.firstElementChild;
			if (screen && screen.offsetHeight !== innerHeight) screen.style.setProperty("height", innerHeight + "px", "important");
		};
		const update = () => {
			if (!/^\/(login|checkpoint|uas)/.test(location.pathname)) return;
			hideSSO();
			fixHeight();
		};
		const start = () => {
			if (!document.head) return setTimeout(start, 50);
			const style = document.createElement("style");
			style.textContent = ".alternate-signin-container, #checkpointGoogleOneTapContainerId { display: none !important; }";
			document.head.appendChild(style);
			update();
			new MutationObserver(update).observe(document.documentElement, { childList: true, subtree: true });
			(window.visualViewport || window).addEventListener("resize", update);
		};
		start();
	}
	return Promise.resolve({%q: navigator.userAgent});
})()`

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
		Instructions: msgStepCookiesInstructions.Message(),
		CookiesParams: &bridgev2.LoginCookiesParams{
			URL:       "https://www.linkedin.com/login",
			ExtractJS: fmt.Sprintf(cookieLoginExtractJS, cookieLoginBrowserHeaderPrefix+"user-agent"),
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

func (c *CookieLogin) SubmitCookies(ctx context.Context, cookies map[string]string) (*bridgev2.LoginStep, error) {
	jar, err := linkedingo.NewJarFromCookieHeader(cookies[CookieLoginCookieHeaderField])
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

	return c.main.completeLinkedInLogin(ctx, c.user, jar, pageInstance, xLiTrack, headers)
}

func (lc *LinkedInConnector) completeLinkedInLogin(ctx context.Context, user *bridgev2.User, jar *linkedingo.StringCookieJar, pageInstance, xLiTrack string, headers http.Header) (*bridgev2.LoginStep, error) {
	loginClient := linkedingo.NewClient(ctx, linkedingo.NewURN(""), jar, pageInstance, xLiTrack, "", linkedingo.Handlers{})
	loginClient.SetBrowserHeaders(headers)
	profile, err := loginClient.GetCurrentUserProfile(ctx)
	if err != nil {
		return nil, wrapLinkedInLoginError(err)
	}
	if profile.MiniProfile.EntityURN.ID() == "" {
		return nil, ErrLoginBadCookies
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
	ul.Client.Connect(ul.Log.WithContext(lc.Bridge.BackgroundCtx))

	return &bridgev2.LoginStep{
		Type:           bridgev2.LoginStepTypeComplete,
		StepID:         CookieLoginStepIDComplete,
		Instructions:   msgStepCompleteInstructions.Render(l10n.Params{"name": remoteName}),
		CompleteParams: &bridgev2.LoginCompleteParams{UserLoginID: ul.ID, UserLogin: ul},
	}, nil
}
