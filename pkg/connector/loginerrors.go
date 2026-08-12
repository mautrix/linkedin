// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2026 Sumner Evans
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
	"errors"
	"fmt"
	"net/http"

	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var (
	ErrLoginBadCookies = bridgev2.RespError{
		ErrCode:    "FI.MAU.LINKEDIN.BAD_COOKIES",
		Err:        "LinkedIn rejected those cookies. Please log in again and export a fresh set.",
		StatusCode: http.StatusUnauthorized,
	}
	ErrLoginBlocked = bridgev2.RespError{
		ErrCode:    "FI.MAU.LINKEDIN.LOGIN_BLOCKED",
		Err:        "LinkedIn blocked the sign-in attempt. Open LinkedIn in a browser, complete any checks it shows, then try again.",
		StatusCode: http.StatusForbidden,
	}
	ErrLoginRateLimited = bridgev2.RespError{
		ErrCode:    "FI.MAU.LINKEDIN.RATE_LIMITED",
		Err:        "LinkedIn is rate limiting the sign-in. Please wait a few minutes and try again.",
		StatusCode: http.StatusTooManyRequests,
	}
	ErrLoginUnavailable = bridgev2.RespError{
		ErrCode:    "FI.MAU.LINKEDIN.LOGIN_UNAVAILABLE",
		Err:        "LinkedIn couldn't be reached to finish signing in. Please try again.",
		StatusCode: http.StatusBadGateway,
	}
	ErrLoginUnknown = bridgev2.RespError{
		ErrCode:    "M_UNKNOWN",
		Err:        "Internal error logging in to LinkedIn",
		StatusCode: http.StatusInternalServerError,
	}
)

// wrapLinkedInLoginError translates a linkedingo error into one the client can act on,
// keeping the original in the chain with %w so logs are unaffected.
func wrapLinkedInLoginError(err error) error {
	if err == nil {
		return nil
	}
	mapped := ErrLoginUnknown.WithInternalError(err)
	var respErr *linkedingo.ResponseError
	switch {
	case errors.Is(err, linkedingo.ErrTokenInvalidated):
		mapped = ErrLoginBadCookies
	case errors.As(err, &respErr):
		switch {
		case respErr.StatusCode == http.StatusUnauthorized:
			mapped = ErrLoginBadCookies
		case respErr.StatusCode == http.StatusTooManyRequests:
			mapped = ErrLoginRateLimited
		// LinkedIn answers 999 when it decides a client looks automated, and 403 when
		// the session needs a checkpoint completing in a real browser.
		case respErr.StatusCode == http.StatusForbidden || respErr.StatusCode == 999:
			mapped = ErrLoginBlocked
		case respErr.StatusCode >= 500:
			mapped = ErrLoginUnavailable
		default:
			mapped = ErrLoginBadCookies
		}
	}
	return fmt.Errorf("%w: %w", mapped, err)
}
