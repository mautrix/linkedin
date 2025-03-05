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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/rs/zerolog"
	"go.mau.fi/util/exhttp"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridge/status"
	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-linkedin/pkg/connector"
)

var ValidCookieRegex = regexp.MustCompile(`\bJSESSIONID=[^;]+`)

type legacyLoginRequest struct {
	AllHeaders   map[string]string `json:"all_headers,omitempty"`
	CookieHeader string            `json:"cookie_header,omitempty"`
	LIAT         string            `json:"li_at,omitempty"`
	JSESSIONID   string            `json:"JSESSIONID,omitempty"`
}

func legacyProvLogin(w http.ResponseWriter, r *http.Request) {
	user := m.Matrix.Provisioning.GetUser(r)
	log := zerolog.Ctx(r.Context()).With().
		Str("component", "legacy_login").
		Logger()
	ctx := log.WithContext(r.Context())
	var req legacyLoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Err(err).Msg("Failed to decode request")
		mautrix.MBadJSON.WithMessage(err.Error()).Write(w)
		return
	}

	var cookieString string
	if len(req.AllHeaders) > 0 {
		cookieString = req.AllHeaders["Cookie"]
	} else if req.CookieHeader != "" {
		cookieString = req.CookieHeader
	} else if req.JSESSIONID != "" && req.LIAT != "" {
		cookieString = fmt.Sprintf("JSESSIONID=%s; li_at=%s", req.JSESSIONID, req.LIAT)
	} else {
		mautrix.MBadJSON.WithMessage("Missing cookie header").Write(w)
		return
	}

	lp, err := m.Connector.CreateLogin(ctx, user, "cookies")
	if err != nil {
		log.Err(err).Msg("Failed to create login")
		mautrix.MUnknown.WithMessage("Internal error creating login").Write(w)
	} else if firstStep, err := lp.Start(ctx); err != nil {
		log.Err(err).Msg("Failed to start login")
		mautrix.MUnknown.WithMessage("Internal error starting login").Write(w)
	} else if firstStep.StepID != connector.CookieLoginStepIDCookies {
		mautrix.MUnknown.WithMessage("Unexpected login step").Write(w)
	} else if !ValidCookieRegex.MatchString(cookieString) {
		mautrix.MBadJSON.WithMessage("JSESSIONID not found in cookie header").Write(w)
	} else if finalStep, err := lp.(bridgev2.LoginProcessCookies).SubmitCookies(ctx, map[string]string{
		connector.CookieLoginCookieHeaderField: cookieString,
	}); err != nil {
		log.Err(err).Msg("Failed to log in")
		var respErr bridgev2.RespError
		if errors.As(err, &respErr) {
			respErr.Write(w)
		} else {
			mautrix.MUnknown.WithMessage("Internal error logging in: %+v", err).Write(w)
		}
	} else if finalStep.StepID != connector.CookieLoginStepIDComplete {
		mautrix.MUnknown.WithMessage("Unexpected login step %s", finalStep.StepID).Write(w)
	} else {
		log.Info().
			Str("user_login_id", string(finalStep.CompleteParams.UserLogin.ID)).
			Msg("Logged in via legacy provisioning ID")
		exhttp.WriteEmptyJSONResponse(w, http.StatusOK)
		go handleLoginComplete(context.WithoutCancel(ctx), user, finalStep.CompleteParams.UserLogin)
	}
}

func handleLoginComplete(ctx context.Context, user *bridgev2.User, newLogin *bridgev2.UserLogin) {
	allLogins := user.GetUserLogins()
	for _, login := range allLogins {
		if login.ID != newLogin.ID {
			login.Delete(ctx, status.BridgeState{StateEvent: status.StateLoggedOut, Reason: "LOGIN_OVERRIDDEN"}, bridgev2.DeleteOpts{})
		}
	}
}

func legacyProvLogout(w http.ResponseWriter, r *http.Request) {
	user := m.Matrix.Provisioning.GetUser(r)
	logins := user.GetUserLogins()
	if len(logins) == 0 {
		exhttp.WriteJSONResponse(w, http.StatusOK, map[string]any{
			"success": false,
			"errcode": "not logged in",
			"error":   "You're not logged in",
		})
		return
	}
	for _, login := range logins {
		login.Client.(*connector.LinkedInClient).LogoutRemote(r.Context())
	}
	exhttp.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"status":  "logged_out",
	})
}
