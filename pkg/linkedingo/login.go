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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exerrors"
	"golang.org/x/net/html"
)

type PasswordLoginErrorKind string

const (
	PasswordLoginUnavailable PasswordLoginErrorKind = "unavailable"
	PasswordLoginRateLimited PasswordLoginErrorKind = "rate_limited"
	PasswordLoginBlocked     PasswordLoginErrorKind = "blocked"
	PasswordLoginUnsupported PasswordLoginErrorKind = "unsupported_bootstrap"
	PasswordLoginRejected    PasswordLoginErrorKind = "rejected"
	PasswordLoginChallenge   PasswordLoginErrorKind = "challenge"
	PasswordLoginBadResponse PasswordLoginErrorKind = "bad_response"
)

type PasswordLoginError struct {
	Kind       PasswordLoginErrorKind
	Stage      string
	Status     int
	checkpoint *PasswordCheckpoint
}

func (e *PasswordLoginError) Checkpoint() *PasswordCheckpoint {
	return e.checkpoint
}

func (e *PasswordLoginError) Error() string {
	return fmt.Sprintf("LinkedIn password login: %s at %s (HTTP %d)", e.Kind, e.Stage, e.Status)
}

type PasswordLoginSession struct {
	Cookies        *StringCookieJar
	BrowserHeaders http.Header
}

type PasswordLoginClient struct {
	LogRedactedLoginResponses bool

	http    *http.Client
	jar     *StringCookieJar
	headers http.Header
	baseURL string
}

func NewPasswordLoginClient(client *http.Client) *PasswordLoginClient {
	if client == nil {
		client = &http.Client{Transport: http.DefaultTransport}
	}
	cloned := *client
	cloned.Timeout = 45 * time.Second
	jar := NewEmptyStringCookieJar()
	cloned.Jar = jar
	// Redirects are handled explicitly everywhere
	cloned.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &PasswordLoginClient{
		http:    &cloned,
		jar:     jar,
		headers: defaultBrowserHeaders.Clone(),
		baseURL: "https://www.linkedin.com",
	}
}

const maxLoginResponseBytes = 8 << 20

func (c *PasswordLoginClient) request(ctx context.Context, stage, method, path string, body []byte, headers http.Header) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to prepare request")
		return nil, nil, &PasswordLoginError{Kind: PasswordLoginUnavailable, Stage: stage}
	}
	req.Header = c.headers.Clone()
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	maps.Copy(req.Header, headers)
	started := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		zerolog.Ctx(ctx).Err(err).Msg("Failed to send request")
		return nil, nil, &PasswordLoginError{Kind: PasswordLoginUnavailable, Stage: stage}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponseBytes+1))
	zerolog.Ctx(ctx).Debug().
		Str("stage", stage).
		Str("method", method).
		Int("status", resp.StatusCode).
		Int("response_bytes", len(data)).
		Dur("duration", time.Since(started)).
		Msg("LinkedIn native login request")
	c.logRedactedLoginResponse(ctx, stage, method, path, resp, data, err != nil)
	if ctx.Err() != nil {
		return nil, resp, ctx.Err()
	} else if err != nil || len(data) > maxLoginResponseBytes {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to read response")
		return nil, resp, &PasswordLoginError{Kind: PasswordLoginBadResponse, Stage: stage, Status: resp.StatusCode}
	}
	var kind PasswordLoginErrorKind
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		kind = PasswordLoginRateLimited
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == 999:
		kind = PasswordLoginBlocked
	case resp.StatusCode >= 500:
		kind = PasswordLoginUnavailable
	case resp.StatusCode >= 400:
		kind = PasswordLoginRejected
	}
	if kind != "" {
		return data, resp, &PasswordLoginError{Kind: kind, Stage: stage, Status: resp.StatusCode}
	}
	return data, resp, nil
}

func (c *PasswordLoginClient) bootstrap(ctx context.Context) (*passwordLoginPage, http.Header, error) {
	path := "/login"
	for range 5 {
		data, resp, err := c.request(ctx, "bootstrap", http.MethodGet, path, nil, http.Header{
			"Accept": {"text/html,application/xhtml+xml"},
		})
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location, err := resp.Location()
			if err != nil || !c.sameOrigin(location) {
				return nil, nil, &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: "bootstrap", Status: resp.StatusCode}
			}
			if strings.HasPrefix(location.Path, "/checkpoint/") || location.Path == "/authwall" {
				return nil, nil, c.checkpointError(ctx, location, "bootstrap", resp.StatusCode)
			}
			path = location.RequestURI()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, nil, &PasswordLoginError{Kind: PasswordLoginBadResponse, Stage: "bootstrap", Status: resp.StatusCode}
		}
		page, err := parsePasswordLoginPage(data)
		if err != nil {
			return nil, nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "bootstrap", Status: resp.StatusCode}
		}
		return page, resp.Header, nil
	}
	return nil, nil, &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: "bootstrap"}
}

func (c *PasswordLoginClient) sameOrigin(location *url.URL) bool {
	base, _ := url.Parse(c.baseURL)
	return location.Scheme == base.Scheme && location.Host == base.Host && location.User == nil
}

func (c *PasswordLoginClient) Login(ctx context.Context, identifier, password string) (*PasswordLoginSession, error) {
	if strings.TrimSpace(identifier) == "" || password == "" {
		return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: "input"}
	}
	page, bootstrapHeaders, err := c.bootstrap(ctx)
	if err != nil {
		return nil, err
	}
	if c.jar.GetCookie(LinkedInCookieJSESSIONID) == "" {
		return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "bootstrap_cookies"}
	}
	if screen := bootstrapHeaders.Get("X-Li-Leaf-Screen-Id"); strings.HasPrefix(screen, "com.linkedin.sdui.") {
		page.ScreenID = screen
	}
	body, err := page.request(identifier, password)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to prepare password submit request")
		return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "request"}
	}
	return c.postAuthentication(ctx, body, bootstrapHeaders, "authenticate", "/login")
}

func (c *PasswordLoginClient) postAuthentication(ctx context.Context, body []byte, bootstrapHeaders http.Header, stage, refererPath string) (*PasswordLoginSession, error) {
	defer clear(body)
	version := bootstrapHeaders.Get("X-Li-Application-Version")
	if version == "" {
		return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "bootstrap_headers"}
	}
	trackingID := bootstrapHeaders.Get("X-Li-Page-Instance-Tracking-Id")
	if trackingID == "" {
		id := uuid.New()
		trackingID = base64.StdEncoding.EncodeToString(id[:])
	}
	tracking := exerrors.Must(json.Marshal(map[string]any{
		"clientVersion":    version,
		"mpVersion":        version,
		"mpName":           "web",
		"osName":           "web",
		"deviceFormFactor": "DESKTOP",
		"displayDensity":   1,
		"displayWidth":     1920,
		"displayHeight":    1080,
	}))
	headers := http.Header{
		"Accept":                         {"*/*"},
		"Content-Type":                   {"application/json"},
		"Origin":                         {c.baseURL},
		"Referer":                        {c.baseURL + refererPath},
		"Csrf-Token":                     {strings.Trim(c.jar.GetCookie(LinkedInCookieJSESSIONID), `"`)},
		"X-Li-Rsc-Stream":                {"true"},
		"X-Li-Application-Version":       {version},
		"X-Li-Application-Instance":      {bootstrapHeaders.Get("X-Li-Application-Instance")},
		"X-Li-Page-Instance-Tracking-Id": {trackingID},
		"X-Li-Anchor-Page-Key":           {"d_flagship3_login"},
		"X-Li-Page-Instance":             {"urn:li:page:d_flagship3_login;" + trackingID},
		"X-Li-Track":                     {string(tracking)},
		"Sec-Fetch-Dest":                 {"empty"},
		"Sec-Fetch-Mode":                 {"cors"},
		"Sec-Fetch-Site":                 {"same-origin"},
	}
	data, resp, err := c.request(ctx, stage, http.MethodPost, "/flagship-web/rsc-action/actions/server-request", body, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if location, err := resp.Location(); err == nil && c.sameOrigin(location) && strings.HasPrefix(location.Path, "/checkpoint/") {
			return nil, c.checkpointError(ctx, location, stage, resp.StatusCode)
		}
		return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: stage, Status: resp.StatusCode}
	}
	result, err := parsePasswordLoginResponse(data)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to parse password login response")
		return nil, &PasswordLoginError{Kind: PasswordLoginBadResponse, Stage: stage, Status: resp.StatusCode}
	}
	if len(result.Response.Errors) > 0 || result.InlineCredentialError {
		return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: stage, Status: resp.StatusCode}
	}
	for _, action := range result.Response.CompletionAction.Actions {
		location := action.Value.Content.URL.URLValue.URL
		parsed, err := url.Parse(location)
		if err != nil || parsed.User != nil || (parsed.IsAbs() && !c.sameOrigin(parsed)) || parsed.Host != "" && !parsed.IsAbs() {
			continue
		}
		if strings.HasPrefix(parsed.Path, "/checkpoint/") || parsed.Path == "/authwall" {
			return nil, c.checkpointError(ctx, parsed, stage, resp.StatusCode)
		}
		if (parsed.Path == "/feed/" || parsed.Path == "/feed") && c.jar.GetCookie("li_at") != "" && c.jar.GetCookie(LinkedInCookieJSESSIONID) != "" {
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			return &PasswordLoginSession{Cookies: c.jar, BrowserHeaders: c.headers.Clone()}, nil
		}
	}
	return nil, &PasswordLoginError{Kind: PasswordLoginBadResponse, Stage: "completion", Status: resp.StatusCode}
}

func IsPasswordLoginError(err error, kind PasswordLoginErrorKind) bool {
	var loginErr *PasswordLoginError
	return errors.As(err, &loginErr) && loginErr.Kind == kind
}

type PasswordCheckpoint struct {
	client    *PasswordLoginClient
	path      string
	page      []byte
	emailForm url.Values
	headers   http.Header
}

func (c *PasswordCheckpoint) IsEmailCode() bool {
	return len(c.emailForm) > 0
}

func parseEmailCheckpoint(body []byte) url.Values {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	attr := func(node *html.Node, name string) string {
		for _, a := range node.Attr {
			if a.Key == name {
				return a.Val
			}
		}
		return ""
	}
	var findForm func(*html.Node) *html.Node
	findForm = func(node *html.Node) *html.Node {
		if node.Type == html.ElementNode && node.Data == "form" && attr(node, "id") == "email-pin-challenge" {
			return node
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if found := findForm(child); found != nil {
				return found
			}
		}
		return nil
	}
	form := findForm(root)
	if form == nil || !strings.EqualFold(attr(form, "method"), "post") || attr(form, "action") != "/checkpoint/challenge/verify" {
		return nil
	}
	fields := make(url.Values)
	var hasPIN bool
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			name := attr(node, "name")
			if name == "pin" && attr(node, "autocomplete") == "one-time-code" && attr(node, "maxlength") == "6" {
				hasPIN = true
			} else if name != "" && name != "pin" && attr(node, "type") == "hidden" {
				fields.Add(name, attr(node, "value"))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(form)
	if !hasPIN || fields.Get("csrfToken") == "" || fields.Get("challengeId") == "" {
		return nil
	}
	return fields
}

func validEmailCode(code string) bool {
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

func (c *PasswordCheckpoint) SubmitEmailCode(ctx context.Context, code string) (*PasswordLoginSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.IsEmailCode() || !validEmailCode(code) {
		return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: "email_code", checkpoint: c}
	}
	fields := make(url.Values, len(c.emailForm)+1)
	for key, values := range c.emailForm {
		fields[key] = append([]string(nil), values...)
	}
	fields.Set("pin", code)
	body := []byte(fields.Encode())
	defer clear(body)
	data, resp, err := c.client.request(ctx, "verify_email", http.MethodPost, "/checkpoint/challenge/verify", body, http.Header{
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
		if errors.As(err, &loginErr) && loginErr.Kind == PasswordLoginRejected {
			if updated := parseEmailCheckpoint(data); updated != nil {
				c.path = "/checkpoint/challenge/verify"
				c.page, c.emailForm = data, updated
				return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: "email_code", Status: resp.StatusCode, checkpoint: c}
			}
			c.emailForm = nil
			return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: "verify_email", Status: resp.StatusCode, checkpoint: c}
		}
		return nil, err
	}
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusMovedPermanently {
		location, err := resp.Location()
		if err != nil || !c.client.sameOrigin(location) {
			return nil, &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: "verify_email", Status: resp.StatusCode}
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
		c.path = "/checkpoint/challenge/verify"
		c.headers = resp.Header.Clone()
		c.page, c.emailForm = data, parseEmailCheckpoint(data)
	} else {
		return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: "verify_email", Status: resp.StatusCode, checkpoint: c}
	}
	if c.IsEmailCode() {
		return nil, &PasswordLoginError{Kind: PasswordLoginRejected, Stage: "email_code", Status: resp.StatusCode, checkpoint: c}
	}
	if completion := parseCheckpointCompletion(c.page); completion != nil {
		body, err := marshalLoginAction(completion.Action, []any{}, completion.ScreenID)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).Msg("Failed to marshal checkpoint completion action")
			return nil, &PasswordLoginError{Kind: PasswordLoginUnsupported, Stage: "checkpoint_complete"}
		}
		return c.client.postAuthentication(ctx, body, c.headers, "checkpoint_complete", c.path)
	}
	return nil, &PasswordLoginError{Kind: PasswordLoginChallenge, Stage: "verify_email", Status: resp.StatusCode, checkpoint: c}
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
			// We never need the content of the feed page, it's just a signal that the login is done
			c.page, c.emailForm = nil, nil
			return nil
		}
		c.page = data
		c.headers = resp.Header.Clone()
		c.emailForm = parseEmailCheckpoint(data)
		return nil
	}
	return &PasswordLoginError{Kind: PasswordLoginBlocked, Stage: "checkpoint"}
}
