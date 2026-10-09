// Copyright (C) 2026 Beeper
// SPDX-License-Identifier: AGPL-3.0-or-later

package connector

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

const FlowIDPassword = "password"
const PasswordLoginStepID = "fi.mau.linkedin.login.credentials"
const PasswordLoginFallbackStepID = "fi.mau.linkedin.login.browser_fallback"
const PasswordLoginEmailCodeStepID = "fi.mau.linkedin.login.email_code"

type passwordCheckpoint interface {
	IsEmailCode() bool
	SubmitEmailCode(context.Context, string) (*linkedingo.PasswordLoginSession, error)
}

type PasswordLogin struct {
	user   *bridgev2.User
	main   *LinkedInConnector
	ctx    context.Context
	cancel context.CancelFunc
	// Set when the client can make LinkedIn requests from its own network.
	clientHTTP http.RoundTripper
	// The existing cookie flow provides a working fallback until the actual
	// CAPTCHA/MFA continuation contracts are captured and implemented.
	browser    *CookieLogin
	checkpoint passwordCheckpoint
	login      func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error)
}

var _ bridgev2.LoginProcessWithParams = (*PasswordLogin)(nil)
var _ bridgev2.LoginProcessUserInput = (*PasswordLogin)(nil)
var _ bridgev2.LoginProcessCookies = (*PasswordLogin)(nil)

func newPasswordLogin(user *bridgev2.User, main *LinkedInConnector) *PasswordLogin {
	ctx, cancel := context.WithCancel(context.Background())
	return &PasswordLogin{user: user, main: main, ctx: ctx, cancel: cancel}
}

func passwordLoginStep(instructions string) *bridgev2.LoginStep {
	return &bridgev2.LoginStep{
		Type: bridgev2.LoginStepTypeUserInput, StepID: PasswordLoginStepID,
		Instructions: instructions,
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{
			{Type: bridgev2.LoginInputFieldTypeUsername, ID: "identifier", Name: "Email or phone number"},
			{Type: bridgev2.LoginInputFieldTypePassword, ID: "password", Name: "Password"},
		}},
	}
}

func (p *PasswordLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return passwordLoginStep("Enter your LinkedIn email or phone number and password."), nil
}

func (p *PasswordLogin) StartWithParams(ctx context.Context, params bridgev2.LoginStartParams) (*bridgev2.LoginStep, error) {
	p.clientHTTP = params.HTTP
	zerolog.Ctx(ctx).Debug().Bool("client_http", params.HTTP != nil).Msg("Starting LinkedIn native login")
	return p.Start(ctx)
}

func (p *PasswordLogin) Cancel() {
	p.cancel()
}

func passwordEmailCodeStep(instructions string) *bridgev2.LoginStep {
	return &bridgev2.LoginStep{
		Type: bridgev2.LoginStepTypeUserInput, StepID: PasswordLoginEmailCodeStepID,
		Instructions: instructions,
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{{
			Type: bridgev2.LoginInputFieldType2FACode, ID: "code", Name: "Email verification code",
			Pattern: `^[0-9]{6}$`, MinLength: 6, MaxLength: 6,
		}}},
	}
}

func (p *PasswordLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	stop := context.AfterFunc(p.ctx, cancel)
	defer stop()
	defer cancel()
	if p.ctx.Err() != nil {
		return nil, context.Canceled
	}
	if p.checkpoint != nil && p.browser == nil && p.checkpoint.IsEmailCode() {
		code := strings.TrimSpace(input["code"])
		if len(code) != 6 || strings.IndexFunc(code, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return passwordEmailCodeStep("Enter the six-digit code from LinkedIn's latest verification email."), nil
		}
		session, err := p.checkpoint.SubmitEmailCode(ctx, code)
		if err != nil {
			var loginErr *linkedingo.PasswordLoginError
			if errors.As(err, &loginErr) && loginErr.Kind == linkedingo.PasswordLoginRejected && loginErr.Stage == "email_code" {
				return passwordEmailCodeStep("LinkedIn did not accept that code. Check the latest verification email and try again."), nil
			}
			return p.handleLoginError(ctx, err)
		}
		p.checkpoint = nil
		return p.complete(ctx, session)
	}
	if p.browser != nil {
		switch input["action"] {
		case "Continue in browser":
			p.checkpoint = nil
			step, err := p.browser.Start(ctx)
			if step != nil {
				step.Instructions = "Finish signing in to LinkedIn in the browser, including any verification it requests."
			}
			return step, err
		case "Try email and password again":
			p.browser = nil
			p.checkpoint = nil
			return p.Start(ctx)
		default:
			return p.fallbackStep("Choose how to continue signing in."), nil
		}
	}
	identifier, password := strings.TrimSpace(input["identifier"]), input["password"]
	if identifier == "" || password == "" {
		return passwordLoginStep("Enter both your LinkedIn email or phone number and password."), nil
	}
	login := p.login
	if login == nil {
		login = p.newLoginClient().Login
	}
	session, err := login(ctx, identifier, password)
	if err != nil {
		return p.handleLoginError(ctx, err)
	}
	return p.complete(ctx, session)
}

func (p *PasswordLogin) newLoginClient() *linkedingo.PasswordLoginClient {
	httpClient := &http.Client{Transport: p.clientHTTP}
	if p.clientHTTP == nil {
		httpClient = p.main.Bridge.GetHTTPClientSettings().Compile()
	}
	client := linkedingo.NewPasswordLoginClient(httpClient)
	client.LogRedactedLoginResponses = p.main.Config.LogRedactedLoginResponses
	return client
}

func (p *PasswordLogin) handleLoginError(ctx context.Context, err error) (*bridgev2.LoginStep, error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	var loginErr *linkedingo.PasswordLoginError
	if !errors.As(err, &loginErr) {
		return nil, ErrLoginUnavailable
	}
	zerolog.Ctx(ctx).Warn().Str("kind", string(loginErr.Kind)).Str("stage", loginErr.Stage).
		Int("status", loginErr.Status).Msg("LinkedIn native login did not complete")
	switch loginErr.Kind {
	case linkedingo.PasswordLoginRateLimited:
		return nil, ErrLoginRateLimited
	case linkedingo.PasswordLoginUnavailable:
		return nil, ErrLoginUnavailable
	case linkedingo.PasswordLoginRejected:
		switch loginErr.Stage {
		case "authenticate", "input":
			p.checkpoint = nil
			return passwordLoginStep("LinkedIn rejected the sign-in. Check your details and try again, or choose the Cookies login method to sign in through a browser."), nil
		case "checkpoint_complete":
			return p.fallbackStep("LinkedIn verified your code but could not finish signing in. Continue through the browser."), nil
		default:
			return p.fallbackStep("LinkedIn could not complete this sign-in directly. You can finish signing in through the browser."), nil
		}
	case linkedingo.PasswordLoginChallenge:
		// Avoid assigning a typed nil to the checkpoint interface.
		p.checkpoint = nil
		if checkpoint := loginErr.Checkpoint(); checkpoint != nil {
			p.checkpoint = checkpoint
			if checkpoint.IsEmailCode() && loginErr.Stage != "verify_email" {
				return passwordEmailCodeStep("LinkedIn sent a verification code to your email address. Enter the six-digit code to finish signing in."), nil
			}
		}
		return p.fallbackStep("LinkedIn requires additional verification. Continue signing in through the browser."), nil
	default:
		return p.fallbackStep("LinkedIn could not complete this sign-in directly. You can finish signing in through the browser."), nil
	}
}

func (p *PasswordLogin) complete(ctx context.Context, session *linkedingo.PasswordLoginSession) (*bridgev2.LoginStep, error) {
	if session == nil || session.Cookies == nil {
		return nil, ErrLoginUnknown
	}
	// Authentication uses the new web app, but messaging still uses Voyager.
	// Reuse linkedingo's Voyager defaults, not the login app's tracking profile.
	step, err := completeLinkedInLogin(ctx, p.user, session.Cookies, "", "", session.BrowserHeaders)
	if err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	} else if errors.Is(err, errSaveNewLogin) {
		return nil, err
	} else if err != nil {
		// The shared cookie validator can retain provider errors in its chain.
		// Do not propagate them into a password flow's provisioning logs.
		zerolog.Ctx(ctx).Warn().Msg("LinkedIn native session validation or persistence failed")
		return nil, bridgev2.RespError{ErrCode: "FI.MAU.LINKEDIN.SESSION_VALIDATION_FAILED", StatusCode: http.StatusBadGateway,
			Err: "LinkedIn signed you in, but the bridge could not finish connecting. Please try signing in through the Cookies login method."}
	}
	return step, nil
}

func (p *PasswordLogin) fallbackStep(instructions string) *bridgev2.LoginStep {
	p.browser = &CookieLogin{user: p.user, main: p.main}
	return &bridgev2.LoginStep{
		Type: bridgev2.LoginStepTypeUserInput, StepID: PasswordLoginFallbackStepID,
		Instructions: instructions,
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{{
			Type: bridgev2.LoginInputFieldTypeSelect, ID: "action", Name: "Sign-in method",
			Options: []string{"Continue in browser", "Try email and password again"},
		}}},
	}
}

func (p *PasswordLogin) SubmitCookies(ctx context.Context, cookies map[string]string) (*bridgev2.LoginStep, error) {
	if p.ctx.Err() != nil {
		return nil, context.Canceled
	}
	if p.browser == nil {
		return nil, ErrLoginUnknown
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	stop := context.AfterFunc(p.ctx, cancel)
	defer stop()
	defer cancel()
	return p.browser.SubmitCookies(ctx, cookies)
}
