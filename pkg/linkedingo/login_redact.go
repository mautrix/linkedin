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
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"go.mau.fi/util/redact"
)

var loginRedactPolicy = makeLoginRedactPolicy()

func makeLoginRedactPolicy() redact.Policy {
	p := redact.MakeDefaultPolicy()
	// Error text can contain user info, so only keep known protocol structure and not arbitrary error messages.
	p.KeepPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^(?:proto\.sdui|com\.linkedin\.sdui)\.[A-Za-z0-9_.]+$`),
		regexp.MustCompile(`^AuthenticationType_[A-Z_]+$`),
		regexp.MustCompile(`^/(?:login|flagship-web/login|authwall|feed/?|checkpoint/challenge(?:/(?:verify|verifyV2|resend))?)$`),
		regexp.MustCompile(`^(?:CREATED|SHOWN|SOLVED|LINKEDIN_APP_CHALLENGE)$`),
		regexp.MustCompile(`^\$(?:[0-9a-f]+|L[0-9a-f]+|undefined)?$`),
		regexp.MustCompile(`^(?:MemoryNamespace|Checked|Unchecked|stringValue|id|div|span|input|form|button|script|html|head|body)$`),
	}
	for _, key := range []string{
		"identifier", "bcookie", "bscookie", "li_at", "jsessionid", "csrftoken",
		"chptoken", "vcd", "challengeid", "challengedata", "challengedetails",
		"requestsubmissionid", "flowtreeid", "pageinstance", "encryptionsalt", "apfc", "_s",
		"encryptedchallengeviewdata", "encryptedrecognizeddeviceflag",
	} {
		p.SensitiveKeys[key] = true
	}
	return p
}

// HTML attributes get their own policy: an input's scalar "value" is sensitive,
// while a Flight action's object "value" contains its structure.
var loginHTMLRedactPolicy = func() redact.Policy {
	p := makeLoginRedactPolicy()
	p.ClassifyKey = func(key string) redact.Decision {
		switch key {
		case "value", "content", "nonce":
			return redact.Hash
		case "name", "id", "class", "method", "type", "autocomplete", "inputmode", "maxlength", "minlength", "role":
			return redact.Keep
		}
		if strings.HasPrefix(key, "data-") || strings.HasPrefix(key, "on") {
			return redact.Hash
		}
		return redact.Auto
	}
	return p
}()

// Flight responses and hydrated pages are logged as redacted Flight rows, so
// readers parse them the same way as the originals. Non-JSON rows are dropped.
func redactLoginResponse(body []byte) ([]byte, string) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, "empty"
	}
	if data, err := loginRedactPolicy.JSON(body); err == nil {
		return data, "json"
	}
	var rows bytes.Buffer
	redactRow := func(id string, value any) {
		loginRedactPolicy.Value(&value)
		data, _ := json.Marshal(value)
		_, _ = fmt.Fprintf(&rows, "%s:%s\n", id, data)
	}
	if body[0] == '<' {
		_ = visitLoginHydration(body, redactRow)
		if rows.Len() > 0 {
			return rows.Bytes(), "flight"
		}
		if data, err := loginHTMLRedactPolicy.HTML(body); err == nil {
			return data, "html"
		}
	} else {
		visitFlightRows(body, redactRow)
		if rows.Len() > 0 {
			return rows.Bytes(), "flight"
		}
	}
	// Malformed and unknown formats must not turn on an unredacted fallback.
	return []byte(loginRedactPolicy.String(string(body))), "opaque"
}

func loginResponseRoute(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "other"
	}
	switch parsed.Path {
	case "/login", "/flagship-web/login", "/uas/login", "/checkpoint/lg/login-submit":
		return "login"
	case "/authwall":
		return "authwall"
	case "/flagship-web/rsc-action/actions/server-request":
		return "action"
	case "/feed", "/feed/":
		return "feed"
	default:
		if strings.HasPrefix(parsed.Path, "/checkpoint/") {
			return "checkpoint"
		}
		return "other"
	}
}

// Bound both parsing work and log-event size. Oversized or incomplete responses
// retain metadata only; truncating raw HTML/JSON before redaction is unsafe.
const maxLoginDiagnosticBytes = 1024 * 1024
const maxLoginDiagnosticEncodedBytes = 64 * 1024

func (c *PasswordLoginClient) logRedactedLoginResponse(ctx context.Context, stage, method, path string, resp *http.Response, body []byte, incomplete bool) {
	if !c.LogRedactedLoginResponses {
		return
	}
	evt := zerolog.Ctx(ctx).Debug()
	if !evt.Enabled() {
		return
	}
	evt = evt.
		Str("stage", stage).
		Str("method", method).
		Int("status", resp.StatusCode).
		Int("response_bytes", len(body))
	if location := resp.Header.Get("Location"); location != "" {
		evt.Str("redirect_route", loginResponseRoute(location))
	}
	route := loginResponseRoute(path)
	if route == "feed" || route == "other" {
		evt.Str("response_omitted", "non_login_route")
	} else if incomplete || len(body) > maxLoginDiagnosticBytes {
		evt.Str("response_omitted", "incomplete_or_too_large")
	} else {
		redacted, format := redactLoginResponse(body)
		evt.Str("response_format", format)
		if len(redacted) > 0 {
			var compressed bytes.Buffer
			writer := gzip.NewWriter(&compressed)
			_, _ = writer.Write(redacted)
			_ = writer.Close()
			if base64.StdEncoding.EncodedLen(compressed.Len()) > maxLoginDiagnosticEncodedBytes {
				evt.Str("response_omitted", "redacted_body_too_large")
			} else {
				evt.Str("response_redacted_gz", base64.StdEncoding.EncodeToString(compressed.Bytes()))
			}
		}
	}
	evt.Msg("LinkedIn native login response (redacted)")
}
