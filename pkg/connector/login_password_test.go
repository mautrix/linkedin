// Copyright (C) 2026 Beeper
// SPDX-License-Identifier: AGPL-3.0-or-later

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

func TestPasswordLoginValidationAndCancellation(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	p.login = func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error) {
		t.Fatal("invalid or canceled input must not reach LinkedIn")
		return nil, nil
	}
	step, err := p.SubmitUserInput(context.Background(), map[string]string{"identifier": "alice@example.invalid"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginStepID, step.StepID)
	p.Cancel()
	_, err = p.SubmitUserInput(context.Background(), map[string]string{"identifier": "alice@example.invalid", "password": "sentinel"})
	assert.ErrorIs(t, err, context.Canceled)
}

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
}

func (f *fakeEmailCheckpoint) IsEmailCode() bool { return true }

func (f *fakeEmailCheckpoint) SubmitEmailCode(_ context.Context, code string) (*linkedingo.PasswordLoginSession, error) {
	f.calls++
	f.code = code
	return nil, &linkedingo.PasswordLoginError{Kind: linkedingo.PasswordLoginRejected, Stage: "email_code"}
}

func TestPasswordLoginNativeEmailCodeRetryAndCancellation(t *testing.T) {
	p := newPasswordLogin(nil, nil)
	defer p.Cancel()
	fake := &fakeEmailCheckpoint{}
	p.checkpoint = fake
	p.login = func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error) {
		t.Fatal("email verification must not resubmit the password")
		return nil, nil
	}
	step, err := p.SubmitUserInput(context.Background(), map[string]string{"code": "abc"})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginEmailCodeStepID, step.StepID)
	assert.Zero(t, fake.calls)
	step, err = p.SubmitUserInput(context.Background(), map[string]string{"code": " 012345 "})
	require.NoError(t, err)
	assert.Equal(t, PasswordLoginEmailCodeStepID, step.StepID)
	assert.Equal(t, "012345", fake.code)
	assert.Equal(t, 1, fake.calls)
	assert.Nil(t, p.browser)
	p.Cancel()
	_, err = p.SubmitUserInput(context.Background(), map[string]string{"code": "012345"})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, fake.calls)
}
