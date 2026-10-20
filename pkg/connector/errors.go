// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2026 Nick Mills-Barrett
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
	"errors"
	"net/http"
	"net/url"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

// LinkedIn answers 999 when it decides a client looks automated.
const statusLinkedInBlocked = 999

func responseStatusCode(err error) int {
	var respErr *linkedingo.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode
	}
	return 0
}

// wrapConversionError turns a failure to prepare a Matrix message for LinkedIn into a message status.
// Nothing has been sent to LinkedIn at that point, so the failure is certain.
func wrapConversionError(err error) error {
	if err == nil || errors.As(err, &bridgev2.MessageStatus{}) {
		return err
	}
	return bridgev2.WrapErrorInStatus(err).
		WithMessage("failed to prepare message for LinkedIn").
		WithIsCertain(true).
		WithSendNotice(true)
}

// wrapLinkedInError turns an error from a LinkedIn request made while handling a Matrix event into a
// message status.
func wrapLinkedInError(err error) error {
	if err == nil || errors.As(err, &bridgev2.MessageStatus{}) {
		return err
	}
	statusCode := responseStatusCode(err)
	var urlErr *url.Error
	switch {
	case errors.Is(err, linkedingo.ErrClientIsNil), errors.Is(err, linkedingo.ErrTokenInvalidated),
		statusCode == http.StatusUnauthorized:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("not logged in to LinkedIn").
			WithIsCertain(true).
			WithSendNotice(true)
	case statusCode == http.StatusTooManyRequests:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("rate limited by LinkedIn").
			WithErrorReason(event.MessageStatusNetworkError).
			WithIsCertain(true).
			WithSendNotice(true)
	case statusCode == http.StatusForbidden:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("you don't have permission to do that on LinkedIn").
			WithErrorReason(event.MessageStatusNoPermission).
			WithStatus(event.MessageStatusFail).
			WithIsCertain(true).
			WithSendNotice(true)
	case statusCode == statusLinkedInBlocked:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("LinkedIn blocked the request").
			WithIsCertain(true).
			WithSendNotice(true)
	case statusCode >= 500:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("LinkedIn returned a server error").
			WithIsCertain(false).
			WithSendNotice(true)
	case statusCode != 0:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("LinkedIn rejected the request").
			WithIsCertain(true).
			WithSendNotice(true)
	case errors.Is(err, context.DeadlineExceeded):
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("LinkedIn didn't respond in time").
			WithErrorReason(event.MessageStatusNetworkError).
			WithIsCertain(false).
			WithSendNotice(true)
	case errors.As(err, &urlErr):
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("failed to reach LinkedIn").
			WithErrorReason(event.MessageStatusNetworkError).
			WithIsCertain(false).
			WithSendNotice(true)
	default:
		return bridgev2.WrapErrorInStatus(err).
			WithMessage("unexpected error from LinkedIn").
			WithSendNotice(true)
	}
}
