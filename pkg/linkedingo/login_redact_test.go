// Copyright (C) 2026 Beeper
// SPDX-License-Identifier: AGPL-3.0-or-later

package linkedingo

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestLoginRedactionPreservesStructureWithoutSecrets(t *testing.T) {
	flight := `0:{"response":{"completionAction":{"actions":[{"$type":"proto.sdui.actions.core.ServerRequest","value":{"requestId":"com.linkedin.sdui.requests.login.authenticate","requestedArguments":{"payload":{"authenticationType":"AuthenticationType_UNKNOWN","chpToken":"short-proof","vcd":"other-proof","identifier":"alice@example.invalid","bcookie":"cookie-sentinel","pin":"001234"}}}}]}}}`
	for _, tc := range []struct{ name, body, format string }{
		{"flight", flight, "flight"},
		{"json", strings.TrimPrefix(flight, "0:"), "json"},
		{"hydration", `<script id="rehydrate-data">window.__como_rehydration__ = ` + string(mustJSON(t, []string{"1:I[123]\n", flight + "\n"})) + `;</script>`, "flight"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, format := redactLoginResponse([]byte(tc.body))
			assert.Equal(t, tc.format, format)
			for _, structure := range []string{"proto.sdui.actions.core.ServerRequest", "com.linkedin.sdui.requests.login.authenticate", "requestedArguments", "AuthenticationType_UNKNOWN", "chpToken"} {
				assert.Contains(t, string(data), structure)
			}
			for _, secret := range []string{"short-proof", "other-proof", "alice@example.invalid", "cookie-sentinel", "001234"} {
				assert.NotContains(t, string(data), secret)
			}
		})
	}
	data, err := loginRedactPolicy.JSON([]byte(`{"chpToken":"div","vcd":"div","csrfToken":"div"}`))
	require.NoError(t, err)
	var parsed map[string]string
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.NotEqual(t, "div", parsed["chpToken"])
	assert.Equal(t, parsed["chpToken"], parsed["vcd"], "same-process markers should correlate")
}

func TestLoginRedactionHTML(t *testing.T) {
	body := `<form id="sms-pin-challenge" method="post" action="/checkpoint/challenge/verify?token=path-proof">
<input name="csrfToken" type="hidden" value="short secret">
<input name="pin" type="number" value="001234" autocomplete="one-time-code" maxlength="6">
<meta name="csrf-token" content="meta-proof">
<div data-token="data-proof" onclick="submit('handler-proof')">Alice Smith at a***@example.invalid</div>
<script>window.secret = "script-proof";</script>
<script type="application/json">{"chpToken":"json-proof","password":"password-proof"}</script>
<!-- comment-proof -->
</form>`
	data, format := redactLoginResponse([]byte(body))
	assert.Equal(t, "html", format)
	for _, structure := range []string{`id="sms-pin-challenge"`, `method="post"`, `name="csrfToken"`, `name="pin"`, `autocomplete="one-time-code"`, `maxlength="6"`} {
		assert.Contains(t, string(data), structure)
	}
	for _, secret := range []string{"path-proof", "short secret", "001234", "meta-proof", "data-proof", "handler-proof", "Alice Smith", "example.invalid", "script-proof", "json-proof", "password-proof", "comment-proof"} {
		assert.NotContains(t, string(data), secret)
	}
}

func TestLoginRedactionUnknownFormatsFailClosed(t *testing.T) {
	for _, tc := range []struct{ body, format string }{
		{`0:{"broken":"secret-proof"`, "opaque"},
		{`1:Tc,secret-proof` + "\n" + `0:{"response":{}}`, "flight"},
		{`1:I["secret-proof"]`, "opaque"},
		{`<script id="rehydrate-data">window.__como_rehydration__ = ["secret-proof"]; steal("secret-proof")</script>`, "html"},
		{`unknown wire format containing secret-proof`, "opaque"},
	} {
		data, format := redactLoginResponse([]byte(tc.body))
		assert.Equal(t, tc.format, format)
		assert.NotContains(t, string(data), "secret-proof")
		assert.NotEmpty(t, data)
	}
}

func decodeLoginDiagnostic(t *testing.T, event map[string]any) string {
	t.Helper()
	encoded, ok := event["response_redacted_gz"].(string)
	require.True(t, ok, "expected compressed diagnostic body")
	data, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer reader.Close()
	data, err = io.ReadAll(reader)
	require.NoError(t, err)
	return string(data)
}

func TestLoginResponseLoggingOptInAndHTTPFailures(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var logs bytes.Buffer
		log := zerolog.New(&logs).Level(zerolog.DebugLevel)
		client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
			return loginHTTPResponse(req, 400, `<form id="unknown-challenge"><input name="challengeData" value="response-secret"></form>`, http.Header{
				"Set-Cookie": {"li_at=cookie-secret"}, "Location": {"/checkpoint/challenge?token=redirect-secret"},
			}), nil
		})})
		client.LogRedactedLoginResponses = enabled
		_, _, err := client.request(log.WithContext(context.Background()), "verify_email", http.MethodPost, "/checkpoint/challenge/verify?token=request-url-secret", []byte("password=request-secret"), nil)
		require.Error(t, err)
		var event map[string]any
		for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
			var candidate map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &candidate))
			if candidate["message"] == "LinkedIn native login response (redacted)" {
				event = candidate
			}
		}
		if !enabled {
			assert.Nil(t, event)
			continue
		}
		require.NotNil(t, event)
		assert.Equal(t, float64(400), event["status"])
		assert.Equal(t, "checkpoint", event["redirect_route"])
		body := decodeLoginDiagnostic(t, event)
		assert.Contains(t, body, "unknown-challenge")
		for _, secret := range []string{"request-url-secret", "request-secret", "response-secret", "cookie-secret", "redirect-secret"} {
			assert.NotContains(t, logs.String()+body, secret)
		}
	}
}

func TestLoginResponseLoggingOmitsPrivateAndIncompleteBodies(t *testing.T) {
	for _, tc := range []struct {
		name, path, body, reason string
		incomplete               bool
	}{
		{"feed", "/feed/?login=complete", "feed-secret", "non_login_route", false},
		{"other page", "/messaging/", "message-secret", "non_login_route", false},
		{"partial read", "/login", "partial-secret", "incomplete_or_too_large", true},
		{"oversized", "/login", strings.Repeat("x", maxLoginDiagnosticBytes+1), "incomplete_or_too_large", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			log := zerolog.New(&logs).Level(zerolog.TraceLevel)
			client := NewPasswordLoginClient(nil)
			client.LogRedactedLoginResponses = true
			client.logRedactedLoginResponse(log.WithContext(context.Background()), "checkpoint", http.MethodGet, tc.path, &http.Response{StatusCode: 200}, []byte(tc.body), tc.incomplete)
			var event map[string]any
			require.NoError(t, json.Unmarshal(logs.Bytes(), &event))
			assert.Equal(t, tc.reason, event["response_omitted"])
			assert.NotContains(t, event, "response_redacted_gz")
		})
	}
	var logs bytes.Buffer
	log := zerolog.New(&logs).Level(zerolog.InfoLevel)
	client := NewPasswordLoginClient(nil)
	client.LogRedactedLoginResponses = true
	client.logRedactedLoginResponse(log.WithContext(context.Background()), "bootstrap", http.MethodGet, "/login", nil, nil, false)
	assert.Empty(t, logs.String(), "no redaction work or event when debug is disabled")
}

func TestLoginResponseLoggingBoundsEncodedEvent(t *testing.T) {
	values := make([]string, 20000)
	for i := range values {
		values[i] = fmt.Sprintf("opaque-value-%08x", i)
	}
	body := mustJSON(t, values)
	require.Less(t, len(body), maxLoginDiagnosticBytes)
	var logs bytes.Buffer
	log := zerolog.New(&logs).Level(zerolog.DebugLevel)
	client := NewPasswordLoginClient(nil)
	client.LogRedactedLoginResponses = true
	client.logRedactedLoginResponse(log.WithContext(context.Background()), "authenticate", http.MethodPost, "/flagship-web/rsc-action/actions/server-request", &http.Response{StatusCode: 200}, body, false)
	var event map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &event))
	assert.Equal(t, "redacted_body_too_large", event["response_omitted"])
	assert.NotContains(t, event, "response_redacted_gz")
}

// Local-only verification against retained responses. Neither raw captures nor
// their redacted derivatives are test fixtures committed to the repository.
func TestCapturedLoginResponseRedaction(t *testing.T) {
	dir := os.Getenv("LINKEDIN_LOGIN_DIAGNOSTIC_DIR")
	if dir == "" {
		t.Skip("no private diagnostic captures supplied")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.html"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		body, err := os.ReadFile(file)
		require.NoError(t, err)
		var logs bytes.Buffer
		log := zerolog.New(&logs).Level(zerolog.DebugLevel)
		client := NewPasswordLoginClient(nil)
		client.LogRedactedLoginResponses = true
		client.logRedactedLoginResponse(log.WithContext(context.Background()), "checkpoint", http.MethodGet, "/checkpoint/challenge", &http.Response{StatusCode: 200}, body, false)
		var event map[string]any
		require.NoError(t, json.Unmarshal(logs.Bytes(), &event))
		redacted := decodeLoginDiagnostic(t, event)
		root, err := html.Parse(bytes.NewReader(body))
		require.NoError(t, err)
		var check func(*html.Node)
		check = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "input" {
				for _, attr := range n.Attr {
					if attr.Key == "value" && len(attr.Val) >= 8 {
						require.False(t, strings.Contains(redacted, attr.Val), "captured input value leaked")
					}
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				check(child)
			}
		}
		check(root)
		if completion := parseCheckpointCompletion(body); completion != nil {
			payload := object(object(completion.Action["requestedArguments"])["payload"])
			for _, key := range []string{"chpToken", "vcd"} {
				require.False(t, strings.Contains(redacted, payload[key].(string)), "captured completion proof leaked")
			}
			require.Contains(t, redacted, passwordAuthenticationRequest)
		}
	}
}
