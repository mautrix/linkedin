// Copyright (C) 2026 Beeper
// SPDX-License-Identifier: AGPL-3.0-or-later

package linkedingo

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func passwordBootstrapFixture(t *testing.T, encryption ...*loginEncryption) string {
	t.Helper()
	payload := map[string]any{
		"authenticationType": "AuthenticationType_PASSWORD", "isLoginWithProfile": false,
		"bcookie": "from-bootstrap", "bscookie": "server-action-cookie",
	}
	var requestedKeys []any
	for _, field := range []string{"rememberMeOptInCheckboxState", "apfc", "identifier", "password"} {
		payload[field] = map[string]any{"key": field + "-state", "namespace": "MemoryNamespace"}
		requestedKeys = append(requestedKeys, map[string]any{"key": map[string]any{"value": map[string]any{"$case": "id", "id": field + "-state"}}, "isEncrypted": field == "password" && len(encryption) > 0})
	}
	action := map[string]any{
		"requestId": passwordAuthenticationRequest,
		"requestedArguments": map[string]any{
			"$type":   "proto.sdui.actions.requests.RequestedArguments",
			"payload": payload, "requestedStateKeys": requestedKeys,
			"requestMetadata": map[string]any{"$type": "proto.sdui.common.RequestMetadata"},
		},
		"isApfcEnabled": false, "isStreaming": false,
	}
	row, err := json.Marshal([]any{"$", "div", nil, map[string]any{"onClick": action}})
	require.NoError(t, err)
	rows := []string{"0:I[123]\n", "1:" + string(row) + "\n"}
	if len(encryption) > 0 && encryption[0] != nil {
		// Encryption metadata may occur after the first password action.
		e := encryption[0]
		rows = append(rows, "2:"+string(mustJSON(t, map[string]any{"rsaPublicKey": e.PublicKey, "encryptionSalt": e.Salt, "tokenTtlMs": e.TokenTTL}))+"\n")
	}
	chunks, err := json.Marshal(rows)
	require.NoError(t, err)
	return `<html><script id="rehydrate-data">window.__como_rehydration__ = ` + string(chunks) + `</script></html>`
}

func TestPasswordLoginEncryptsMarkedFields(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	salt := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	encryption := &loginEncryption{PublicKey: base64.StdEncoding.EncodeToString(der), Salt: base64.StdEncoding.EncodeToString(salt)}
	const password = " spaces \" and ü🔑 preserved "
	posts := 0
	client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			resp := loginBootstrapResponse(t, req)
			resp.Body = io.NopCloser(strings.NewReader(passwordBootstrapFixture(t, encryption)))
			return resp, nil
		}
		posts++
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "preserved")
		var wire struct {
			States             []struct{ Key, Value string }
			RequestedArguments struct{ States []struct{ Key, Value string } }
		}
		require.NoError(t, json.Unmarshal(body, &wire))
		require.Len(t, wire.States, 4)
		assert.Equal(t, wire.States, wire.RequestedArguments.States)
		assert.Equal(t, "alice@example.invalid", wire.States[2].Value)
		token, err := base64.RawURLEncoding.DecodeString(wire.States[3].Value)
		require.NoError(t, err)
		require.Len(t, token, 258)
		assert.Equal(t, []byte{1, 1}, token[:2])
		plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, privateKey, token[2:], nil)
		require.NoError(t, err)
		assert.Equal(t, salt, plaintext[:8])
		assert.Equal(t, password, string(plaintext[8:]))
		return loginHTTPResponse(req, 200, loginResponseFixture(t, "/feed/"), http.Header{"Set-Cookie": {"li_at=test-session; Path=/; Secure"}}), nil
	})})
	_, err = client.Login(context.Background(), "alice@example.invalid", password)
	require.NoError(t, err)
	assert.Equal(t, 1, posts)

	t.Run("salt advances with bootstrap age", func(t *testing.T) {
		encryption.TokenTTL = 7200000
		encryption.Observed = time.Now().Add(-time.Second)
		value, err := encryption.encrypt(password)
		require.NoError(t, err)
		token, err := base64.RawURLEncoding.DecodeString(value)
		require.NoError(t, err)
		plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, privateKey, token[2:], nil)
		require.NoError(t, err)
		advance := binary.BigEndian.Uint64(plaintext[:8]) - binary.BigEndian.Uint64(salt)
		assert.GreaterOrEqual(t, advance, uint64(1000))
		assert.LessOrEqual(t, advance, uint64(time.Since(encryption.Observed).Milliseconds()))
	})
}

func TestPasswordLoginEncryptionFailsClosed(t *testing.T) {
	for _, encryption := range []*loginEncryption{nil, {PublicKey: "invalid", Salt: "invalid"}} {
		posts := 0
		client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				posts++
				return nil, errors.New("credentials must not be posted")
			}
			resp := loginBootstrapResponse(t, req)
			resp.Body = io.NopCloser(strings.NewReader(passwordBootstrapFixture(t, encryption)))
			return resp, nil
		})})
		_, err := client.Login(context.Background(), "alice@example.invalid", "secret-password")
		require.Error(t, err)
		assert.True(t, IsPasswordLoginError(err, PasswordLoginUnsupported))
		assert.NotContains(t, err.Error(), "secret-password")
		assert.Zero(t, posts)
	}
}

func loginResponseFixture(t *testing.T, destination string) string {
	t.Helper()
	response := passwordLoginResponse{}
	// A deliberately explicit fixture mirrors the observed Flight wire envelope,
	// including an unrelated record before the root action result.
	data := `{"states":[],"response":{"errors":[],"isRetryable":false,"completionAction":{"actions":[{"value":{"content":{"url":{"urlValue":{"url":` + string(mustJSON(t, destination)) + `}}}}}]}}}`
	require.NoError(t, json.Unmarshal([]byte(data), &response))
	return "1:[]\n0:" + data + "\n"
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

type loginRoundTripper func(*http.Request) (*http.Response, error)

func (rt loginRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return rt(req) }

func loginHTTPResponse(req *http.Request, status int, body string, headers http.Header) *http.Response {
	return &http.Response{Request: req, StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}
}

func loginBootstrapResponse(t *testing.T, req *http.Request) *http.Response {
	return loginHTTPResponse(req, http.StatusOK, passwordBootstrapFixture(t), http.Header{
		"X-Li-Application-Version":       {"test-version"},
		"X-Li-Application-Instance":      {"test-instance"},
		"X-Li-Page-Instance-Tracking-Id": {"test-tracking"},
		"Set-Cookie":                     {`JSESSIONID="ajax:test-csrf"; Path=/; Secure`, "bcookie=current-cookie; Path=/; Secure"},
	})
}

func TestPasswordLoginRequestUsesFreshActionAndPlainStateValues(t *testing.T) {
	page, err := parsePasswordLoginPage([]byte(passwordBootstrapFixture(t)))
	require.NoError(t, err)
	body, err := page.request("alice@example.invalid", " spaces \" remain ")
	require.NoError(t, err)
	var wire struct {
		States             []struct{ Key, Value, OriginalProtoCase string }
		RequestedArguments struct {
			States  []struct{ Key, Value, OriginalProtoCase string }
			Payload map[string]any
		}
	}
	require.NoError(t, json.Unmarshal(body, &wire))
	require.Len(t, wire.States, 4)
	assert.Equal(t, wire.States, wire.RequestedArguments.States)
	assert.Equal(t, "Checked", wire.States[0].Value)
	assert.Equal(t, "", wire.States[1].Value)
	assert.Empty(t, wire.States[1].OriginalProtoCase)
	assert.Equal(t, "alice@example.invalid", wire.States[2].Value)
	assert.Equal(t, " spaces \" remain ", wire.States[3].Value)
	assert.Equal(t, "stringValue", wire.States[3].OriginalProtoCase)
	assert.Equal(t, "server-action-cookie", wire.RequestedArguments.Payload["bscookie"])
}

func TestPasswordLoginSuccessAndSafeLogs(t *testing.T) {
	var logs bytes.Buffer
	log := zerolog.New(&logs).Level(zerolog.DebugLevel)
	requests := 0
	client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		require.Equal(t, "www.linkedin.com", req.URL.Host)
		if req.URL.Path == "/login" {
			return loginBootstrapResponse(t, req), nil
		}
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/flagship-web/rsc-action/actions/server-request", req.URL.Path)
		require.Equal(t, "ajax:test-csrf", req.Header.Get("Csrf-Token"))
		require.Equal(t, "true", req.Header.Get("X-Li-Rsc-Stream"))
		return loginHTTPResponse(req, 200, loginResponseFixture(t, "/feed/"), http.Header{
			"Set-Cookie": {"li_at=secret-session-sentinel; Path=/; Secure; HttpOnly"},
		}), nil
	})})
	session, err := client.Login(log.WithContext(context.Background()), "identity-sentinel@example.invalid", "password-sentinel")
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "secret-session-sentinel", session.Cookies.GetCookie("li_at"))
	assert.NotEmpty(t, session.BrowserHeaders.Get("User-Agent"))
	assert.Equal(t, 2, requests)
	for _, secret := range []string{"identity-sentinel", "password-sentinel", "secret-session-sentinel", "ajax:test-csrf"} {
		assert.NotContains(t, logs.String(), secret)
	}
}

func TestPasswordLoginFailuresDoNotRetryOrLeak(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		kind   PasswordLoginErrorKind
	}{
		{"rate limit", 429, "secret-response", PasswordLoginRateLimited},
		{"blocked", 403, "secret-response", PasswordLoginBlocked},
		{"unavailable", 503, "secret-response", PasswordLoginUnavailable},
		{"post redirect", 307, "secret-response", PasswordLoginChallenge},
		{"challenge", 200, loginResponseFixture(t, "/checkpoint/challenge?token=secret-token"), PasswordLoginChallenge},
		{"external completion", 200, loginResponseFixture(t, "https://attacker.invalid/feed/"), PasswordLoginBadResponse},
		{"invalid result", 200, "0:{\"secret-response\":true}", PasswordLoginBadResponse},
		{"missing session", 200, loginResponseFixture(t, "/feed/"), PasswordLoginBadResponse},
		{"provider errors", 200, "0:{\"response\":{\"errors\":[{\"message\":\"secret-response\"}]}}", PasswordLoginRejected},
		{"inline credential error", 200, `0:{"response":{"errors":[],"completionAction":{"actions":[{"$type":"proto.sdui.actions.core.ReplaceComponent","value":{"content":{"newComponent":["$","div",null,{"viewTrackingSpecs":{"viewName":"identifier-password-inline-feedback"},"children":{"textProps":{"children":["Wrong email or password."]}}}]}}}]}}}`, PasswordLoginRejected},
		{"unknown inline feedback", 200, `0:{"response":{"errors":[],"completionAction":{"actions":[{"$type":"proto.sdui.actions.core.ReplaceComponent","value":{"content":{"newComponent":{"viewTrackingSpecs":{"viewName":"identifier-password-inline-feedback"},"children":"secret-response"}}}}]}}}`, PasswordLoginBadResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet {
					return loginBootstrapResponse(t, req), nil
				}
				posts++
				return loginHTTPResponse(req, tc.status, tc.body, http.Header{"Location": {"https://attacker.invalid/?secret-token"}}), nil
			})})
			session, err := client.Login(context.Background(), "alice@example.invalid", "password-sentinel")
			require.Error(t, err)
			require.Nil(t, session)
			var nativeErr *PasswordLoginError
			require.ErrorAs(t, err, &nativeErr)
			assert.Equal(t, tc.kind, nativeErr.Kind)
			assert.Equal(t, 1, posts)
			assert.NotContains(t, err.Error(), "secret-")
		})
	}
}

func TestPasswordLoginRejectsCrossOriginBootstrap(t *testing.T) {
	calls := 0
	client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		return loginHTTPResponse(req, 302, "", http.Header{"Location": {"https://attacker.invalid/"}}), nil
	})})
	_, err := client.Login(context.Background(), "alice@example.invalid", "password")
	require.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.NotContains(t, err.Error(), "attacker")
}

func TestPasswordLoginCheckpointPreservesSessionAndDoesNotReplayCredentials(t *testing.T) {
	for _, externalRedirect := range []bool{false, true} {
		t.Run(fmt.Sprint("external_redirect=", externalRedirect), func(t *testing.T) {
			var logs bytes.Buffer
			log := zerolog.New(&logs).Level(zerolog.DebugLevel)
			posts, checkpointGets := 0, 0
			client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "www.linkedin.com", req.URL.Host)
				switch req.URL.Path {
				case "/login":
					return loginBootstrapResponse(t, req), nil
				case "/flagship-web/rsc-action/actions/server-request":
					posts++
					return loginHTTPResponse(req, 200, loginResponseFixture(t, "/checkpoint/challenge/session-sentinel?ut=token-sentinel"), http.Header{
						"Set-Cookie": {"checkpoint=checkpoint-cookie-sentinel; Path=/; Secure; HttpOnly"},
					}), nil
				case "/checkpoint/challenge/session-sentinel":
					checkpointGets++
					require.Equal(t, http.MethodGet, req.Method)
					require.Zero(t, req.ContentLength)
					cookie, err := req.Cookie("checkpoint")
					require.NoError(t, err)
					assert.Equal(t, "checkpoint-cookie-sentinel", cookie.Value)
					assert.Equal(t, "token-sentinel", req.URL.Query().Get("ut"))
					if externalRedirect {
						return loginHTTPResponse(req, 302, "", http.Header{"Location": {"https://attacker.invalid/?secret-token"}}), nil
					}
					return loginHTTPResponse(req, 200, "<html>private-challenge-sentinel</html>", nil), nil
				default:
					t.Fatal("unexpected request")
					return nil, errors.New("unexpected request")
				}
			})})
			_, err := client.Login(log.WithContext(context.Background()), "alice@example.invalid", "password-sentinel")
			var nativeErr *PasswordLoginError
			require.ErrorAs(t, err, &nativeErr)
			assert.Equal(t, PasswordLoginChallenge, nativeErr.Kind)
			checkpoint := nativeErr.Checkpoint()
			require.NotNil(t, checkpoint)
			assert.Same(t, client, checkpoint.client)
			assert.Equal(t, 1, posts)
			assert.Equal(t, 1, checkpointGets)
			if externalRedirect {
				assert.Empty(t, checkpoint.page)
			} else {
				assert.Contains(t, string(checkpoint.page), "private-challenge-sentinel")
			}
			serialized, err := json.Marshal(nativeErr)
			require.NoError(t, err)
			for _, output := range []string{logs.String(), string(serialized), nativeErr.Error()} {
				assert.NotContains(t, output, "sentinel")
				assert.NotContains(t, output, "attacker.invalid")
			}
		})
	}
}

func TestPasswordLoginCancellationAndTransportErrorRedaction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
		cancel()
		return nil, &url.Error{Op: http.MethodPost, URL: "https://www.linkedin.com/?secret-token", Err: errors.New("secret-password")}
	})})
	_, err := client.Login(ctx, "alice@example.invalid", "password")
	require.ErrorIs(t, err, context.Canceled)
	client = NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: http.MethodPost, URL: "https://www.linkedin.com/?secret-token", Err: errors.New("secret-password")}
	})})
	_, err = client.Login(context.Background(), "alice@example.invalid", "password")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-")
}

// Optional local evidence check. The file stays outside the repository; neither
// assertions nor failures print its contents or any generated request values.
func TestPasswordLoginCapturedBootstrap(t *testing.T) {
	path := os.Getenv("LINKEDIN_LOGIN_BOOTSTRAP")
	if path == "" {
		t.Skip("no private bootstrap capture provided")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	page, err := parsePasswordLoginPage(body)
	require.NoError(t, err)
	_, err = page.request("synthetic@example.invalid", "synthetic-password")
	require.NoError(t, err)
}

func emailCheckpointFixture(csrf string) string {
	return `<html><form id="unrelated"><input type="hidden" name="unrelated" value="ignore"></form>
<form id="email-pin-challenge" method="post" action="/checkpoint/challenge/verify">
<input type="hidden" name="csrfToken" value="` + csrf + `">
<input type="hidden" name="challengeId" value="challenge-sentinel">
<input type="hidden" name="challengeData" value="opaque&amp;value=preserved">
<input name="pin" type="number" maxlength="6" autocomplete="one-time-code">
</form></html>`
}

func TestEmailCheckpointOnlyAcceptsObservedForm(t *testing.T) {
	valid := emailCheckpointFixture("csrf-sentinel")
	fields := parseEmailCheckpoint([]byte(valid))
	require.NotNil(t, fields)
	assert.Equal(t, "opaque&value=preserved", fields.Get("challengeData"))
	assert.Empty(t, fields.Get("unrelated"))
	for _, replacement := range [][2]string{
		{"email-pin-challenge", "other-challenge"},
		{"method=\"post\"", "method=\"get\""},
		{"/checkpoint/challenge/verify", "https://attacker.invalid/verify"},
		{"one-time-code", "password"},
		{"csrf-sentinel", ""},
		{"challenge-sentinel", ""},
	} {
		assert.Nil(t, parseEmailCheckpoint([]byte(strings.ReplaceAll(valid, replacement[0], replacement[1]))))
	}
}

func TestEmailCheckpointSubmissionAndRetry(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprint("retry=", retry), func(t *testing.T) {
			var logs bytes.Buffer
			ctx := zerolog.New(&logs).WithContext(context.Background())
			verifies, feeds := 0, 0
			client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/login":
					return loginBootstrapResponse(t, req), nil
				case "/flagship-web/rsc-action/actions/server-request":
					return loginHTTPResponse(req, 200, loginResponseFixture(t, "/checkpoint/challenge/token-sentinel?ut=private-sentinel"), http.Header{"Set-Cookie": {"checkpoint=cookie-sentinel; Path=/; Secure; HttpOnly"}}), nil
				case "/checkpoint/challenge/token-sentinel":
					return loginHTTPResponse(req, 200, emailCheckpointFixture("csrf-sentinel"), nil), nil
				case "/checkpoint/challenge/verify":
					verifies++
					require.Equal(t, http.MethodPost, req.Method)
					require.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
					cookie, err := req.Cookie("checkpoint")
					require.NoError(t, err)
					assert.Equal(t, "cookie-sentinel", cookie.Value)
					body, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					fields, err := url.ParseQuery(string(body))
					require.NoError(t, err)
					assert.Equal(t, "012345", fields.Get("pin"))
					assert.Equal(t, "challenge-sentinel", fields.Get("challengeId"))
					assert.Equal(t, "opaque&value=preserved", fields.Get("challengeData"))
					assert.NotContains(t, string(body), "password-sentinel")
					if verifies == 1 {
						assert.Equal(t, "csrf-sentinel", fields.Get("csrfToken"))
					} else {
						assert.Equal(t, "rotated-csrf-sentinel", fields.Get("csrfToken"))
					}
					if retry && verifies == 1 {
						return loginHTTPResponse(req, 400, emailCheckpointFixture("rotated-csrf-sentinel"), nil), nil
					}
					return loginHTTPResponse(req, 303, "", http.Header{"Location": {"/feed/"}, "Set-Cookie": {"li_at=auth-session-sentinel; Path=/; Secure; HttpOnly"}}), nil
				case "/feed/":
					feeds++
					require.Equal(t, http.MethodGet, req.Method)
					return loginHTTPResponse(req, 200, "<html>feed</html>", nil), nil
				default:
					t.Fatal("unexpected request")
					return nil, errors.New("unexpected request")
				}
			})})
			_, err := client.Login(ctx, "alice@example.invalid", "password-sentinel")
			var nativeErr *PasswordLoginError
			require.ErrorAs(t, err, &nativeErr)
			checkpoint := nativeErr.Checkpoint()
			require.NotNil(t, checkpoint)
			require.True(t, checkpoint.IsEmailCode())
			_, err = checkpoint.SubmitEmailCode(ctx, "invalid")
			require.Error(t, err)
			assert.Zero(t, verifies)
			if retry {
				_, err = checkpoint.SubmitEmailCode(ctx, "012345")
				require.True(t, IsPasswordLoginError(err, PasswordLoginRejected))
				require.True(t, checkpoint.IsEmailCode())
			}
			session, err := checkpoint.SubmitEmailCode(ctx, "012345")
			require.NoError(t, err)
			require.NotNil(t, session)
			assert.Equal(t, "auth-session-sentinel", session.Cookies.GetCookie("li_at"))
			if retry {
				assert.Equal(t, 2, verifies)
			} else {
				assert.Equal(t, 1, verifies)
			}
			assert.Equal(t, 1, feeds)
			assert.NotContains(t, logs.String(), "sentinel")
			assert.NotContains(t, logs.String(), "opaque&value")
		})
	}
}

func TestEmailCheckpointNeverReplaysCodeOrFollowsExternalRedirect(t *testing.T) {
	for _, status := range []int{303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				return loginHTTPResponse(req, status, "", http.Header{"Location": {"https://attacker.invalid/?secret"}}), nil
			})})
			checkpoint := &PasswordCheckpoint{client: client, path: "/checkpoint/challenge/private", emailForm: parseEmailCheckpoint([]byte(emailCheckpointFixture("csrf")))}
			_, err := checkpoint.SubmitEmailCode(context.Background(), "012345")
			require.Error(t, err)
			assert.Equal(t, 1, calls)
			assert.NotContains(t, err.Error(), "attacker.invalid")
		})
	}
}

func TestCapturedEmailCheckpoint(t *testing.T) {
	path := os.Getenv("LINKEDIN_CHECKPOINT_HTML")
	if path == "" {
		t.Skip("no private checkpoint capture provided")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	fields := parseEmailCheckpoint(body)
	require.NotNil(t, fields)
	assert.NotEmpty(t, fields.Get("challengeData"))
	assert.NotEmpty(t, fields.Get("requestSubmissionId"))
}

func checkpointCompletionFixture(t *testing.T) string {
	t.Helper()
	action := map[string]any{
		"requestId": passwordAuthenticationRequest,
		"requestedArguments": map[string]any{
			"requestedStateKeys": []any{},
			"payload":            map[string]any{"authenticationType": "AuthenticationType_UNKNOWN", "chpToken": "checkpoint-proof-sentinel", "vcd": "verification-proof-sentinel", "bcookie": "fresh-action-cookie"},
		},
	}
	root := map[string]any{
		"screenId": "com.linkedin.sdui.flagshipnav.login.Login",
		"onAppear": map[string]any{"actions": []any{map[string]any{"$type": "proto.sdui.actions.core.ServerRequest", "value": action}}},
	}
	chunks := mustJSON(t, []string{"0:" + string(mustJSON(t, root)) + "\n"})
	return `<script id="rehydrate-data">window.__como_rehydration__ = ` + string(chunks) + `</script>`
}

func TestCheckpointCompletionRequiresSpecificAutomaticAction(t *testing.T) {
	fixture := checkpointCompletionFixture(t)
	require.NotNil(t, parseCheckpointCompletion([]byte(fixture)))
	for _, replacement := range [][2]string{
		{"onAppear", "onClick"},
		{"AuthenticationType_UNKNOWN", "AuthenticationType_PASSWORD"},
		{"com.linkedin.sdui.requests.login.authenticate", "com.linkedin.sdui.requests.other"},
		{"checkpoint-proof-sentinel", ""},
		{"verification-proof-sentinel", "$unresolved"},
		{"proto.sdui.actions.core.ServerRequest", "proto.sdui.actions.core.Navigate"},
	} {
		assert.Nil(t, parseCheckpointCompletion([]byte(strings.ReplaceAll(fixture, replacement[0], replacement[1]))))
	}
}

func TestEmailCheckpointExchangesCompletionProof(t *testing.T) {
	for _, failExchange := range []bool{false, true} {
		t.Run(fmt.Sprint("fail_exchange=", failExchange), func(t *testing.T) {
			var logs bytes.Buffer
			ctx := zerolog.New(&logs).WithContext(context.Background())
			authPosts, codePosts := 0, 0
			client := NewPasswordLoginClient(&http.Client{Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/login":
					if req.URL.RawQuery == "" {
						return loginBootstrapResponse(t, req), nil
					}
					resp := loginBootstrapResponse(t, req)
					resp.Header.Set("X-Li-Application-Version", "completion-version")
					resp.Body = io.NopCloser(strings.NewReader(checkpointCompletionFixture(t)))
					return resp, nil
				case "/flagship-web/rsc-action/actions/server-request":
					authPosts++
					if authPosts == 1 {
						return loginHTTPResponse(req, 200, loginResponseFixture(t, "/checkpoint/challenge/private"), nil), nil
					}
					require.Equal(t, 2, authPosts)
					require.Equal(t, "completion-version", req.Header.Get("X-Li-Application-Version"))
					require.Contains(t, req.Header.Get("Referer"), "/login?completed=private")
					body, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					var wire struct {
						States             []any
						RequestedArguments struct {
							States  []any
							Payload map[string]any
						}
					}
					require.NoError(t, json.Unmarshal(body, &wire))
					assert.NotNil(t, wire.States)
					assert.Empty(t, wire.States)
					assert.NotNil(t, wire.RequestedArguments.States)
					assert.Empty(t, wire.RequestedArguments.States)
					assert.Equal(t, "AuthenticationType_UNKNOWN", wire.RequestedArguments.Payload["authenticationType"])
					assert.Equal(t, "checkpoint-proof-sentinel", wire.RequestedArguments.Payload["chpToken"])
					assert.Equal(t, "verification-proof-sentinel", wire.RequestedArguments.Payload["vcd"])
					assert.NotContains(t, string(body), "password-sentinel")
					assert.NotContains(t, string(body), "012345")
					if failExchange {
						return loginHTTPResponse(req, 503, "private-error-sentinel", nil), nil
					}
					return loginHTTPResponse(req, 200, loginResponseFixture(t, "/feed/"), http.Header{"Set-Cookie": {"li_at=completed-session-sentinel; Path=/; Secure; HttpOnly"}}), nil
				case "/checkpoint/challenge/private":
					return loginHTTPResponse(req, 200, emailCheckpointFixture("csrf-sentinel"), nil), nil
				case "/checkpoint/challenge/verify":
					codePosts++
					return loginHTTPResponse(req, 303, "", http.Header{"Location": {"/login?completed=private"}}), nil
				default:
					t.Fatal("unexpected request")
					return nil, errors.New("unexpected request")
				}
			})})
			_, err := client.Login(ctx, "alice@example.invalid", "password-sentinel")
			var loginErr *PasswordLoginError
			require.ErrorAs(t, err, &loginErr)
			checkpoint := loginErr.Checkpoint()
			require.NotNil(t, checkpoint)
			session, err := checkpoint.SubmitEmailCode(ctx, "012345")
			if failExchange {
				require.True(t, IsPasswordLoginError(err, PasswordLoginUnavailable))
				assert.Nil(t, session)
			} else {
				require.NoError(t, err)
				require.NotNil(t, session)
				assert.Equal(t, "completed-session-sentinel", session.Cookies.GetCookie("li_at"))
			}
			assert.Equal(t, 2, authPosts)
			assert.Equal(t, 1, codePosts)
			assert.NotContains(t, logs.String(), "sentinel")
		})
	}
}

func TestCapturedCheckpointCompletion(t *testing.T) {
	path := os.Getenv("LINKEDIN_COMPLETION_HTML")
	if path == "" {
		t.Skip("no private completion capture provided")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	page := parseCheckpointCompletion(body)
	require.NotNil(t, page)
	_, err = marshalLoginAction(page.Action, []any{}, page.ScreenID)
	require.NoError(t, err)
}
