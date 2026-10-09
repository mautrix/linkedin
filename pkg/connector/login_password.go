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
	"strings"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

const FlowIDPassword = "password"
const PasswordLoginStepID = "fi.mau.linkedin.login.credentials"
const PasswordLoginFallbackStepID = "fi.mau.linkedin.login.browser_fallback"
const PasswordLoginEmailCodeStepID = "fi.mau.linkedin.login.email_code"
const PasswordLoginSMSCodeStepID = "fi.mau.linkedin.login.sms_code"
const PasswordLoginTOTPCodeStepID = "fi.mau.linkedin.login.totp_code"
const PasswordLoginAppMethodStepID = "fi.mau.linkedin.login.app_method"
const PasswordLoginAppApprovalStepID = "fi.mau.linkedin.login.app_approval"

type passwordCheckpoint interface {
	Kind() linkedingo.PasswordCheckpointKind
	CanTryAnotherWay() bool
	SubmitCode(context.Context, string) (*linkedingo.PasswordLoginSession, error)
	WaitForApp(context.Context) (*linkedingo.PasswordLoginSession, error)
	TryAnotherWay(context.Context) (*linkedingo.PasswordLoginSession, error)
}

type PasswordLogin struct {
	user       *bridgev2.User
	main       *LinkedInConnector
	clientHTTP http.RoundTripper
	browser    *CookieLogin
	checkpoint passwordCheckpoint
	login      func(context.Context, string, string) (*linkedingo.PasswordLoginSession, error)
}

var _ bridgev2.LoginProcessWithParams = (*PasswordLogin)(nil)
var _ bridgev2.LoginProcessUserInput = (*PasswordLogin)(nil)
var _ bridgev2.LoginProcessCookies = (*PasswordLogin)(nil)
var _ bridgev2.LoginProcessDisplayAndWait = (*PasswordLogin)(nil)
var _ bridgev2.LoginProcessStepCancel = (*PasswordLogin)(nil)

func newPasswordLogin(user *bridgev2.User, main *LinkedInConnector) *PasswordLogin {
	return &PasswordLogin{user: user, main: main}
}

func passwordLoginStep(instructions string) *bridgev2.LoginStep {
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       PasswordLoginStepID,
		Instructions: instructions,
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{
			{Type: bridgev2.LoginInputFieldTypeUsername, ID: "identifier", Name: "Email or phone number"},
			{Type: bridgev2.LoginInputFieldTypePassword, ID: "password", Name: "Password"},
		}},
	}
}

func (p *PasswordLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return p.StartWithParams(ctx, bridgev2.LoginStartParams{})
}

func (p *PasswordLogin) StartWithParams(ctx context.Context, params bridgev2.LoginStartParams) (*bridgev2.LoginStep, error) {
	if params.HTTP != nil {
		p.clientHTTP = params.HTTP
	}
	zerolog.Ctx(ctx).Debug().Bool("client_http", p.clientHTTP != nil).Msg("Starting LinkedIn native login")
	return passwordLoginStep("Enter your LinkedIn email or phone number and password."), nil
}

func (p *PasswordLogin) Cancel() {
	// No-op, there are no persistent connections to cancel.
}

func passwordCodeStep(kind linkedingo.PasswordCheckpointKind, instructions string) *bridgev2.LoginStep {
	stepID, name := PasswordLoginEmailCodeStepID, "Email verification code"
	if kind == linkedingo.PasswordCheckpointSMS {
		stepID, name = PasswordLoginSMSCodeStepID, "SMS verification code"
	} else if kind == linkedingo.PasswordCheckpointTOTP {
		stepID, name = PasswordLoginTOTPCodeStepID, "Authenticator app code"
	}
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       stepID,
		Instructions: instructions,
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{{
			Type:      bridgev2.LoginInputFieldType2FACode,
			ID:        "code",
			Name:      name,
			Pattern:   `^[0-9]{6}$`,
			MinLength: 6,
			MaxLength: 6,
		}}},
	}
}

func (p *PasswordLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	if p.checkpoint != nil && p.browser == nil {
		if p.checkpoint.Kind() == linkedingo.PasswordCheckpointApp {
			switch input["action"] {
			case "Approve in LinkedIn app":
				return p.appApprovalStep(), nil
			case "Try another way":
				if p.checkpoint.CanTryAnotherWay() {
					session, err := p.checkpoint.TryAnotherWay(ctx)
					return p.finishCheckpoint(ctx, session, err)
				}
			}
			return p.appMethodStep(), nil
		}
		session, err := p.checkpoint.SubmitCode(ctx, strings.TrimSpace(input["code"]))
		return p.finishCheckpoint(ctx, session, err)
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

func (p *PasswordLogin) appMethodStep() *bridgev2.LoginStep {
	options := []string{"Approve in LinkedIn app"}
	if p.checkpoint.CanTryAnotherWay() {
		options = append(options, "Try another way")
	}
	return &bridgev2.LoginStep{
		Type: bridgev2.LoginStepTypeUserInput, StepID: PasswordLoginAppMethodStepID,
		Instructions: "LinkedIn sent a sign-in request to your LinkedIn app. Approve it there, or choose another verification method.",
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{{
			Type: bridgev2.LoginInputFieldTypeSelect, ID: "action", Name: "Verification method", Options: options,
		}}},
	}
}

func (p *PasswordLogin) appApprovalStep() *bridgev2.LoginStep {
	return &bridgev2.LoginStep{
		Type: bridgev2.LoginStepTypeDisplayAndWait, StepID: PasswordLoginAppApprovalStepID,
		Instructions:         "Open the LinkedIn app on your phone and approve this sign-in. To use another verification method, go back.",
		DisplayAndWaitParams: &bridgev2.LoginDisplayAndWaitParams{Type: bridgev2.LoginDisplayTypeNothing, CanCancel: true},
	}
}

func (p *PasswordLogin) Wait(ctx context.Context) (*bridgev2.LoginStep, error) {
	if p.checkpoint == nil || p.browser != nil || p.checkpoint.Kind() != linkedingo.PasswordCheckpointApp {
		return nil, ErrLoginUnknown
	}
	session, err := p.checkpoint.WaitForApp(ctx)
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if errors.Is(err, bridgev2.ErrLoginStepCancelled) {
		return nil, err
	}
	return p.finishCheckpoint(ctx, session, err)
}

func (p *PasswordLogin) CancelStep(ctx context.Context) (*bridgev2.LoginStep, error) {
	if p.checkpoint == nil || p.browser != nil || p.checkpoint.Kind() != linkedingo.PasswordCheckpointApp {
		return nil, ErrLoginUnknown
	}
	return p.appMethodStep(), nil
}

func (p *PasswordLogin) finishCheckpoint(ctx context.Context, session *linkedingo.PasswordLoginSession, err error) (*bridgev2.LoginStep, error) {
	if err != nil {
		return p.handleLoginError(ctx, err)
	}
	p.checkpoint = nil
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
		case "email_code":
			return passwordCodeStep(linkedingo.PasswordCheckpointEmail, "LinkedIn did not accept that code. Check the latest verification email and try again."), nil
		case "sms_code":
			return passwordCodeStep(linkedingo.PasswordCheckpointSMS, "LinkedIn did not accept that code. Check the latest text message and try again."), nil
		case "totp_code":
			return passwordCodeStep(linkedingo.PasswordCheckpointTOTP, "LinkedIn did not accept that code. Enter the current six-digit code from your authenticator app and try again."), nil
		case "app_approval_expired":
			return p.fallbackStep("The LinkedIn app approval request expired. Try signing in again or continue through the browser."), nil
		case "authenticate", "input":
			p.checkpoint = nil
			return passwordLoginStep("LinkedIn rejected the sign-in. Check your details and try again, or choose the Cookies login method to sign in through a browser."), nil
		case "checkpoint_complete":
			return p.fallbackStep("LinkedIn verified your sign-in but could not finish logging in. Continue through the browser."), nil
		default:
			return p.fallbackStep("LinkedIn could not complete this sign-in directly. You can finish signing in through the browser."), nil
		}
	case linkedingo.PasswordLoginChallenge:
		// Avoid assigning a typed nil to the checkpoint interface.
		p.checkpoint = nil
		if checkpoint := loginErr.Checkpoint(); checkpoint != nil {
			p.checkpoint = checkpoint
			switch checkpoint.Kind() {
			case linkedingo.PasswordCheckpointEmail:
				return passwordCodeStep(checkpoint.Kind(), "LinkedIn sent a verification code to your email address. Enter the six-digit code to finish signing in."), nil
			case linkedingo.PasswordCheckpointSMS:
				return passwordCodeStep(checkpoint.Kind(), "LinkedIn sent a text message to your phone. Enter the six-digit code to finish signing in."), nil
			case linkedingo.PasswordCheckpointTOTP:
				return passwordCodeStep(checkpoint.Kind(), "Enter the current six-digit LinkedIn code from your authenticator app to finish signing in."), nil
			case linkedingo.PasswordCheckpointApp:
				return p.appMethodStep(), nil
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
	return p.main.completeLinkedInLogin(ctx, p.user, session.Cookies, "", "", session.BrowserHeaders)
}

func (p *PasswordLogin) fallbackStep(instructions string) *bridgev2.LoginStep {
	p.browser = &CookieLogin{user: p.user, main: p.main}
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       PasswordLoginFallbackStepID,
		Instructions: instructions,
		UserInputParams: &bridgev2.LoginUserInputParams{Fields: []bridgev2.LoginInputDataField{{
			Type:    bridgev2.LoginInputFieldTypeSelect,
			ID:      "action",
			Name:    "Sign-in method",
			Options: []string{"Continue in browser", "Try email and password again"},
		}}},
	}
}

func (p *PasswordLogin) SubmitCookies(ctx context.Context, cookies map[string]string) (*bridgev2.LoginStep, error) {
	if p.browser == nil {
		return nil, ErrLoginUnknown
	}
	return p.browser.SubmitCookies(ctx, cookies)
}
