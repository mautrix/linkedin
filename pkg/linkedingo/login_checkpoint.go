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

package linkedingo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/html"
)

type PasswordCheckpointKind string

const (
	PasswordCheckpointEmail PasswordCheckpointKind = "email"
	PasswordCheckpointSMS   PasswordCheckpointKind = "sms"
	PasswordCheckpointTOTP  PasswordCheckpointKind = "totp"
	PasswordCheckpointApp   PasswordCheckpointKind = "app"
)

type PasswordCheckpoint struct {
	client  *PasswordLoginClient
	path    string
	page    []byte
	form    *passwordCheckpointForm
	headers http.Header
}

type passwordCheckpointForm struct {
	kind          PasswordCheckpointKind
	action        string
	fields        url.Values
	pollInterval  time.Duration
	expiresAt     time.Time
	tryAnotherWay bool
}

func (c *PasswordCheckpoint) Kind() PasswordCheckpointKind {
	if c.form == nil {
		return ""
	}
	return c.form.kind
}

func (c *PasswordCheckpoint) CanTryAnotherWay() bool {
	return c.Kind() == PasswordCheckpointApp && c.form.tryAnotherWay
}

func checkpointAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func parsePasswordCheckpoint(body []byte) *passwordCheckpointForm {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var form *html.Node
	codes := make(map[string]string)
	var tryAnotherWay, hasRecoveryForm bool
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			id := checkpointAttr(node, "id")
			if node.Data == "form" && (id == "email-pin-challenge" || id == "two-step-challenge" || id == "in-app-challenge") {
				form = node
			} else if node.Data == "form" && id == "two-step-challenge-recovery-code" && checkpointAttr(node, "action") == "/checkpoint/challenge/verifyCode" {
				// The authenticator page includes a separate recovery-code form.
				// Only the main six-digit form is submitted by this flow.
				hasRecoveryForm = true
			} else if node.Data == "code" && node.FirstChild != nil {
				codes[id] = strings.TrimSpace(node.FirstChild.Data)
			} else if id == "try-another-way" {
				tryAnotherWay = true
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if form == nil || !strings.EqualFold(checkpointAttr(form, "method"), "post") {
		return nil
	}
	result := &passwordCheckpointForm{action: checkpointAttr(form, "action"), fields: make(url.Values)}
	switch checkpointAttr(form, "id") {
	case "email-pin-challenge":
		result.kind = PasswordCheckpointEmail
	case "two-step-challenge":
		result.kind = PasswordCheckpointSMS
		if hasRecoveryForm {
			result.kind = PasswordCheckpointTOTP
		}
	case "in-app-challenge":
		result.kind = PasswordCheckpointApp
	}
	if result.kind == PasswordCheckpointApp {
		if result.action != "/checkpoint/challenge/verifyV2" || codes["flavour"] != `"CONSUMER_LOGIN"` {
			return nil
		}
		start, _ := strconv.ParseInt(codes["startTimestampInMillis"], 10, 64)
		ttl, _ := strconv.ParseInt(codes["totalPollingTimeInSeconds"], 10, 64)
		interval, _ := strconv.ParseInt(codes["pollingFrequencyInSeconds"], 10, 64)
		if start <= 0 || ttl <= 0 || ttl > 86400 || interval <= 0 || interval > ttl {
			return nil
		}
		result.expiresAt = time.UnixMilli(start).Add(time.Duration(ttl) * time.Second)
		result.pollInterval = time.Duration(interval) * time.Second
		result.tryAnotherWay = tryAnotherWay
	} else if result.action != "/checkpoint/challenge/verify" {
		return nil
	}
	var hasPIN bool
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			name := checkpointAttr(node, "name")
			if name == "pin" && checkpointAttr(node, "maxlength") == "6" {
				hasPIN = (result.kind == PasswordCheckpointEmail && checkpointAttr(node, "autocomplete") == "one-time-code") ||
					((result.kind == PasswordCheckpointSMS || result.kind == PasswordCheckpointTOTP) && checkpointAttr(node, "id") == "input__phone_verification_pin" && checkpointAttr(node, "type") == "tel")
			} else if name != "" && name != "pin" && checkpointAttr(node, "type") == "hidden" {
				result.fields.Add(name, checkpointAttr(node, "value"))
			} else if name == "recognizedDevice" && checkpointAttr(node, "type") == "checkbox" {
				for _, attr := range node.Attr {
					if attr.Key == "checked" {
						value := checkpointAttr(node, "value")
						if value == "" {
							value = "on"
						}
						result.fields.Add(name, value)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(form)
	if result.fields.Get("csrfToken") == "" || result.fields.Get("challengeId") == "" || (result.kind != PasswordCheckpointApp && !hasPIN) {
		return nil
	}
	if result.kind == PasswordCheckpointApp && !result.fields.Has("userResponse") {
		return nil
	}
	return result
}

func validCheckpointCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (c *PasswordCheckpoint) SubmitCode(ctx context.Context, code string) (*PasswordLoginSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	kind := c.Kind()
	if kind != PasswordCheckpointEmail && kind != PasswordCheckpointSMS && kind != PasswordCheckpointTOTP {
		return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "verify_code"}
	}
	if !validCheckpointCode(code) {
		return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: string(kind) + "_code", checkpoint: c}
	}
	fields := maps.Clone(c.form.fields)
	fields.Set("pin", code)
	return c.submitForm(ctx, fields)
}

func (c *PasswordCheckpoint) TryAnotherWay(ctx context.Context) (*PasswordLoginSession, error) {
	if !c.CanTryAnotherWay() {
		return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "app_alternative"}
	}
	fields := maps.Clone(c.form.fields)
	fields.Set("userResponse", "TRY_ANOTHER_WAY")
	return c.submitForm(ctx, fields)
}

// LinkedIn's app challenge polls immediately, then at the interval supplied in
// the page. Only SOLVED authorizes submitting the original verification form.
func (c *PasswordCheckpoint) WaitForApp(ctx context.Context) (*PasswordLoginSession, error) {
	if c.Kind() != PasswordCheckpointApp {
		return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "app_poll"}
	}
	form := c.form
	fields := maps.Clone(form.fields)
	fields.Set("recognizedDevice", "true")
	fields.Set("userResponse", "")
	for {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for key, values := range fields {
			for _, value := range values {
				_ = writer.WriteField(key, value)
			}
		}
		_ = writer.Close()
		data, resp, err := c.client.request(ctx, "app_poll", http.MethodPost,
			"/checkpoint/challengesV2/poll/"+url.PathEscape(fields.Get("challengeId")), body.Bytes(), http.Header{
				"Accept": {"*/*"}, "Content-Type": {writer.FormDataContentType()},
				"Origin": {c.client.baseURL}, "Referer": {c.client.baseURL + c.path},
				"X-Requested-With": {"XMLHttpRequest"},
				"Sec-Fetch-Dest":   {"empty"}, "Sec-Fetch-Mode": {"cors"}, "Sec-Fetch-Site": {"same-origin"},
			})
		clear(body.Bytes())
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		} else if err != nil {
			return nil, err
		}
		now, err := http.ParseTime(resp.Header.Get("Date"))
		if err != nil {
			now = time.Now()
		}
		if !now.Before(form.expiresAt) {
			return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: "app_approval_expired"}
		}
		var poll struct {
			State string `json:"pollingResponseChallengeStateV2"`
		}
		if resp.StatusCode != http.StatusOK || json.Unmarshal(data, &poll) != nil {
			return nil, &PasswordLoginError{Kind: PasswordLoginBadResponse, Stage: "app_poll", Status: resp.StatusCode}
		}
		switch poll.State {
		case "SOLVED":
			return c.submitForm(ctx, fields)
		case "CREATED", "SHOWN":
			// The request is still waiting for the user's approval.
		default:
			return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: "app_poll", Status: resp.StatusCode}
		}
		timer := time.NewTimer(min(form.pollInterval, form.expiresAt.Sub(now)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, context.Cause(ctx)
		case <-timer.C:
		}
	}
}

func (c *PasswordCheckpoint) submitForm(ctx context.Context, fields url.Values) (*PasswordLoginSession, error) {
	kind, action := c.Kind(), c.form.action
	stage := "verify_" + string(kind)
	body := []byte(fields.Encode())
	defer clear(body)
	data, resp, err := c.client.request(ctx, stage, http.MethodPost, action, body, http.Header{
		"Accept":         {"text/html,application/xhtml+xml"},
		"Content-Type":   {"application/x-www-form-urlencoded"},
		"Origin":         {c.client.baseURL},
		"Referer":        {c.client.baseURL + c.path},
		"Sec-Fetch-Dest": {"document"},
		"Sec-Fetch-Mode": {"navigate"},
		"Sec-Fetch-Site": {"same-origin"},
	})
	if err != nil {
		var loginErr *PasswordLoginError
		if !errors.As(err, &loginErr) || loginErr.Kind != PasswordLoginRejected {
			return nil, err
		}
		c.path, c.page, c.form, c.headers = action, data, parsePasswordCheckpoint(data), resp.Header.Clone()
		return nil, c.challengeResult(kind, stage, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusMovedPermanently {
		location, err := resp.Location()
		if err != nil || !c.client.sameOrigin(location) {
			return nil, &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: stage, Status: resp.StatusCode}
		}
		c.path = location.RequestURI()
		if err := c.load(ctx); err != nil {
			return nil, err
		}
		location, _ = url.Parse(c.path)
		if (location.Path == "/feed/" || location.Path == "/feed") && c.client.jar.GetCookie("li_at") != "" && c.client.jar.GetCookie(LinkedInCookieJSESSIONID) != "" {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &PasswordLoginSession{Cookies: c.client.jar, BrowserHeaders: c.client.headers.Clone()}, nil
		}
	} else if resp.StatusCode == http.StatusOK {
		c.path, c.page, c.form, c.headers = action, data, parsePasswordCheckpoint(data), resp.Header.Clone()
	} else {
		// Never replay a code or an approval on a 307/308 redirect.
		return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: stage, Status: resp.StatusCode}
	}
	if c.form != nil {
		return nil, c.challengeResult(kind, stage, resp.StatusCode)
	}
	if completion := parseCheckpointCompletion(c.page); completion != nil {
		body, err := marshalLoginAction(completion.Action, []any{}, completion.ScreenID)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).Msg("Failed to marshal checkpoint completion action")
			return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "checkpoint_complete"}
		}
		return c.client.postAuthentication(ctx, body, c.headers, "checkpoint_complete", c.path)
	}
	return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: stage, Status: resp.StatusCode}
}

func (c *PasswordCheckpoint) challengeResult(previous PasswordCheckpointKind, stage string, status int) error {
	if (previous == PasswordCheckpointEmail || previous == PasswordCheckpointSMS || previous == PasswordCheckpointTOTP) && c.Kind() == previous {
		return &PasswordLoginError{Kind: PasswordLoginRejected, Stage: string(previous) + "_code", Status: status, checkpoint: c}
	}
	return &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: stage, Status: status, checkpoint: c}
}

func (c *PasswordLoginClient) checkpointError(ctx context.Context, location *url.URL, stage string, status int) error {
	checkpoint := &PasswordCheckpoint{client: c, path: location.RequestURI()}
	if err := checkpoint.load(ctx); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("LinkedIn checkpoint page could not be loaded")
	}
	return &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: stage, Status: status, checkpoint: checkpoint}
}

func (c *PasswordCheckpoint) load(ctx context.Context) error {
	c.page, c.form = nil, nil
	for range 5 {
		data, resp, err := c.client.request(ctx, "checkpoint", http.MethodGet, c.path, nil, http.Header{
			"Accept":         {"text/html,application/xhtml+xml"},
			"Referer":        {c.client.baseURL + "/login"},
			"Sec-Fetch-Dest": {"document"},
			"Sec-Fetch-Mode": {"navigate"},
			"Sec-Fetch-Site": {"same-origin"},
		})
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location, err := resp.Location()
			if err != nil || !c.client.sameOrigin(location) {
				return &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: "checkpoint", Status: resp.StatusCode}
			}
			c.path = location.RequestURI()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return &PasswordLoginError{Kind: PasswordLoginBadResponse, Stage: "checkpoint", Status: resp.StatusCode}
		}
		location, _ := url.Parse(c.path)
		if location.Path == "/feed/" || location.Path == "/feed" {
			return nil
		}
		c.page, c.headers, c.form = data, resp.Header.Clone(), parsePasswordCheckpoint(data)
		return nil
	}
	return &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: "checkpoint"}
}
