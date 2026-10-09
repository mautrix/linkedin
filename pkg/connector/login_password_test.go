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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPasswordLoginUsesClientHTTPTransport(t *testing.T) {
	p := newPasswordLogin(nil, &LinkedInConnector{})
	defer p.Cancel()
	var requests []string
	step, err := p.StartWithParams(context.Background(), bridgev2.LoginStartParams{HTTP: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.String())
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: http.NoBody, Request: req}, nil
	})})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginStepID, step.StepID)
	_, err = p.SubmitUserInput(context.Background(), map[string]string{"identifier": "alice@example.invalid", "password": "sentinel"})
	assert.ErrorIs(t, err, ErrLoginUnavailable)
	assert.Equal(t, []string{"GET https://www.linkedin.com/login"}, requests)
}

func TestPasswordLoginChallengeOffersBrowserFallback(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	p.login = func(_ context.Context, identifier, password string) (*linkedingo.PasswordLoginSession, error) {
		assert.Equal(t, "alice@example.invalid", identifier)
		assert.Equal(t, " password with spaces ", password)
		return nil, &linkedingo.PasswordLoginError{Kind: linkedingo.PasswordLoginChallenge, Stage: "authenticate"}
	}
	step, err := p.SubmitUserInput(context.Background(), map[string]string{"identifier": " alice@example.invalid ", "password": " password with spaces "})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginFallbackStepID, step.StepID)
	assert.NotContains(t, step.Instructions, "password with spaces")
	step, err = p.SubmitUserInput(context.Background(), map[string]string{"action": "Continue in browser"})
	require.NoError(t, err)
	assert.Equal(t, bridgev2.LoginStepTypeCookies, step.Type)
	assert.Equal(t, "https://linkedin.com/login", step.CookiesParams.URL)
}

func TestPasswordLoginRejectedCredentialsRemainRetryable(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	p.login = func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error) {
		return nil, &linkedingo.PasswordLoginError{Kind: linkedingo.PasswordLoginRejected, Stage: "authenticate"}
	}
	step, err := p.SubmitUserInput(context.Background(), map[string]string{"identifier": "alice@example.invalid", "password": "sentinel"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginStepID, step.StepID)
	assert.Nil(t, p.browser)
}

type fakeEmailCheckpoint struct {
	calls int
	code  string
	err   error
}

func (f *fakeEmailCheckpoint) IsEmailCode() bool { return true }

func (f *fakeEmailCheckpoint) SubmitEmailCode(_ context.Context, code string) (*linkedingo.PasswordLoginSession, error) {
	f.calls++
	f.code = code
	return nil, f.err
}

func TestPasswordLoginRejectedEmailCodeIsRetryable(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	fake := &fakeEmailCheckpoint{err: &linkedingo.PasswordLoginError{Kind: linkedingo.PasswordLoginRejected, Stage: "email_code"}}
	p.checkpoint = fake
	p.login = func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error) {
		t.Fatal("email verification must not resubmit the password")
		return nil, nil
	}
	step, err := p.SubmitUserInput(context.Background(), map[string]string{"code": " 012345 "})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginEmailCodeStepID, step.StepID)
	assert.Equal(t, "012345", fake.code)
	step, err = p.SubmitUserInput(context.Background(), map[string]string{"code": "543210"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginEmailCodeStepID, step.StepID)
	assert.Equal(t, "543210", fake.code)
	assert.Equal(t, 2, fake.calls)
	assert.Nil(t, p.browser)
}

func TestPasswordLoginRejectedCheckpointFallsBackToBrowser(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	p.checkpoint = &fakeEmailCheckpoint{err: &linkedingo.PasswordLoginError{Kind: linkedingo.PasswordLoginRejected, Stage: "checkpoint", Status: http.StatusBadRequest}}
	var logins int
	p.login = func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error) {
		logins++
		return nil, &linkedingo.PasswordLoginError{Kind: linkedingo.PasswordLoginRejected, Stage: "authenticate"}
	}
	step, err := p.SubmitUserInput(context.Background(), map[string]string{"code": "012345"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginFallbackStepID, step.StepID)
	step, err = p.SubmitUserInput(context.Background(), map[string]string{"action": "Try email and password again"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginStepID, step.StepID)
	step, err = p.SubmitUserInput(context.Background(), map[string]string{"identifier": "alice@example.invalid", "password": "sentinel"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginStepID, step.StepID)
	assert.Equal(t, 1, logins)
	assert.Nil(t, p.checkpoint)
}

func TestPasswordLoginCompletionReturnsCancellation(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	p.login = func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error) {
		return &linkedingo.PasswordLoginSession{Cookies: linkedingo.NewEmptyStringCookieJar()}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.SubmitUserInput(ctx, map[string]string{"identifier": "alice@example.invalid", "password": "sentinel"})
	assert.ErrorIs(t, err, context.Canceled)
}
