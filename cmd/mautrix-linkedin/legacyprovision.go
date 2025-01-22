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

func legacyProvLogin(w http.ResponseWriter, r *http.Request) {
	user := m.Matrix.Provisioning.GetUser(r)
	ctx := r.Context()
	var body map[string]map[string]string
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		exhttp.WriteJSONResponse(w, http.StatusBadRequest, mautrix.MBadJSON.WithMessage(err.Error()))
		return
	}
	cookieString := body["all_headers"]["Cookie"]

	lp, err := m.Connector.CreateLogin(ctx, user, "cookies")
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to create login")
		exhttp.WriteJSONResponse(w, http.StatusInternalServerError, mautrix.MUnknown.WithMessage("Internal error creating login"))
	} else if firstStep, err := lp.Start(ctx); err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to start login")
		exhttp.WriteJSONResponse(w, http.StatusInternalServerError, mautrix.MUnknown.WithMessage("Internal error starting login"))
	} else if firstStep.StepID != connector.CookieLoginStepIDCookies {
		exhttp.WriteJSONResponse(w, http.StatusInternalServerError, mautrix.MUnknown.WithMessage("Unexpected login step"))
	} else if !ValidCookieRegex.MatchString(cookieString) {
		exhttp.WriteJSONResponse(w, http.StatusOK, nil)
	} else if finalStep, err := lp.(bridgev2.LoginProcessCookies).SubmitCookies(ctx, map[string]string{
		connector.CookieLoginCookieHeaderField: cookieString,
	}); err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to log in")
		var respErr bridgev2.RespError
		if errors.As(err, &respErr) {
			exhttp.WriteJSONResponse(w, respErr.StatusCode, &respErr)
		} else {
			exhttp.WriteJSONResponse(w, http.StatusInternalServerError, mautrix.MUnknown.WithMessage("Internal error logging in"))
		}
	} else if finalStep.StepID != connector.CookieLoginStepIDComplete {
		exhttp.WriteJSONResponse(w, http.StatusInternalServerError, mautrix.MUnknown.WithMessage("Unexpected login step"))
	} else {
		exhttp.WriteJSONResponse(w, http.StatusOK, map[string]any{})
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
