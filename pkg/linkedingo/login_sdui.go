// Copyright (C) 2026 Beeper
// SPDX-License-Identifier: AGPL-3.0-or-later

package linkedingo

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const passwordAuthenticationRequest = "com.linkedin.sdui.requests.login.authenticate"

type loginBinding struct {
	Key       string `json:"key"`
	Namespace string `json:"namespace"`
	Encrypted bool
}

type passwordLoginPage struct {
	Action     map[string]any
	Arguments  map[string]any
	Payload    map[string]any
	ScreenID   string
	Encryption *loginEncryption
}

// parsePasswordLoginPage reads the server's React hydration data as data, never
// executing JavaScript. There can be several login panels, including Google,
// passkeys and profile-based login. Only the plain password action is usable.
func parsePasswordLoginPage(body []byte) (*passwordLoginPage, error) {
	var page *passwordLoginPage
	var encryption *loginEncryption
	err := visitLoginHydration(body, func(_ string, value any) {
		if page == nil {
			page = findPasswordAction(value, 0)
		}
		if encryption == nil {
			encryption = findLoginEncryption(value, 0)
		}
	})
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, errors.New("password authentication action was not found")
	}
	page.Encryption = encryption
	return page, nil
}

func visitLoginHydration(body []byte, visit func(id string, value any)) error {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return errors.New("login bootstrap was not found")
		case html.StartTagToken:
			token := tokenizer.Token()
			if token.Data != "script" {
				continue
			}
			isHydration := false
			for _, attr := range token.Attr {
				isHydration = isHydration || (attr.Key == "id" && attr.Val == "rehydrate-data")
			}
			if !isHydration || tokenizer.Next() != html.TextToken {
				continue
			}
			text := strings.TrimSpace(string(tokenizer.Text()))
			const assignment = "window.__como_rehydration__"
			if !strings.HasPrefix(text, assignment) {
				continue
			}
			text = strings.TrimSpace(strings.TrimPrefix(text, assignment))
			if !strings.HasPrefix(text, "=") {
				continue
			}
			var chunks []string
			decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(text[1:])))
			if decoder.Decode(&chunks) != nil {
				return errors.New("unsupported login bootstrap")
			}
			for _, chunk := range chunks {
				visitFlightRows([]byte(chunk), visit)
			}
			return nil
		}
	}
}

func visitFlightRows(data []byte, visit func(id string, value any)) {
	for _, row := range bytes.Split(data, []byte{'\n'}) {
		id, data, ok := bytes.Cut(row, []byte{':'})
		if !ok {
			continue
		}
		var value any
		if json.Unmarshal(data, &value) != nil {
			continue // Other Flight row types are not action JSON.
		}
		visit(string(id), value)
	}
}

func object(value any) map[string]any {
	obj, _ := value.(map[string]any)
	return obj
}

// After email verification, LinkedIn redirects to a login screen with an
// automatic onAppear ServerRequest that exchanges the checkpoint proof. Only
// recognize that specific action; never execute arbitrary hydration actions.
func parseCheckpointCompletion(body []byte) *passwordLoginPage {
	var page *passwordLoginPage
	_ = visitLoginHydration(body, func(_ string, value any) {
		if page == nil {
			page = findCheckpointCompletion(value, 0)
		}
	})
	return page
}

func findCheckpointCompletion(value any, depth int) *passwordLoginPage {
	if depth > 100 {
		return nil
	}
	switch value := value.(type) {
	case map[string]any:
		screen, _ := value["screenId"].(string)
		if strings.HasPrefix(screen, "com.linkedin.sdui.") {
			actions, _ := object(value["onAppear"])["actions"].([]any)
			for _, item := range actions {
				wrapper := object(item)
				action := object(wrapper["value"])
				args := object(action["requestedArguments"])
				payload := object(args["payload"])
				states, hasStates := args["requestedStateKeys"].([]any)
				chp, _ := payload["chpToken"].(string)
				vcd, _ := payload["vcd"].(string)
				if wrapper["$type"] == "proto.sdui.actions.core.ServerRequest" && action["requestId"] == passwordAuthenticationRequest &&
					payload["authenticationType"] == "AuthenticationType_UNKNOWN" && hasStates && len(states) == 0 &&
					chp != "" && vcd != "" && len(chp) <= 8192 && len(vcd) <= 8192 && !strings.HasPrefix(chp, "$") && !strings.HasPrefix(vcd, "$") {
					return &passwordLoginPage{Action: action, Arguments: args, Payload: payload, ScreenID: screen}
				}
			}
		}
		for _, child := range value {
			if page := findCheckpointCompletion(child, depth+1); page != nil {
				return page
			}
		}
	case []any:
		for _, child := range value {
			if page := findCheckpointCompletion(child, depth+1); page != nil {
				return page
			}
		}
	}
	return nil
}

func findPasswordAction(value any, depth int) *passwordLoginPage {
	if depth > 100 {
		return nil
	}
	switch value := value.(type) {
	case map[string]any:
		args := object(value["requestedArguments"])
		payload := object(args["payload"])
		if value["requestId"] == passwordAuthenticationRequest && payload["authenticationType"] == "AuthenticationType_PASSWORD" && payload["isLoginWithProfile"] == false {
			page := &passwordLoginPage{Action: value, Arguments: args, Payload: payload, ScreenID: "com.linkedin.sdui.flagshipnav.login.Login"}
			if _, err := page.binding("identifier"); err == nil {
				if _, err = page.binding("password"); err == nil {
					return page
				}
			}
		}
		// JSON objects have no ordering; all accepted actions must have the same
		// plain-password semantics. Per-render tracking values come from that action.
		for _, child := range value {
			if result := findPasswordAction(child, depth+1); result != nil {
				return result
			}
		}
	case []any:
		for _, child := range value {
			if result := findPasswordAction(child, depth+1); result != nil {
				return result
			}
		}
	}
	return nil
}

func (p *passwordLoginPage) binding(name string) (loginBinding, error) {
	value := object(p.Payload[name])
	key, _ := value["key"].(string)
	namespace, _ := value["namespace"].(string)
	if key == "" || len(key) > 128 || namespace != "MemoryNamespace" {
		return loginBinding{}, errors.New("unsupported password login field binding")
	}
	encrypted, _ := value["isEncrypted"].(bool)
	requested, _ := p.Arguments["requestedStateKeys"].([]any)
	for _, item := range requested {
		state := object(item)
		stateKey := object(object(state["key"])["value"])
		if stateKey["$case"] == "id" && stateKey["id"] == key && state["isEncrypted"] == true {
			encrypted = true
		}
	}
	return loginBinding{Key: key, Namespace: namespace, Encrypted: encrypted}, nil
}

func (p *passwordLoginPage) request(identifier, password string) ([]byte, error) {
	states := make([]any, 0, 4)
	for _, field := range []struct{ name, value string }{
		{"rememberMeOptInCheckboxState", "Checked"},
		// No captured fingerprint or token is replayed. Whether an empty device
		// signal is accepted must be established by a live native-login test.
		{"apfc", ""},
		{"identifier", identifier},
		{"password", password},
	} {
		binding, err := p.binding(field.name)
		if err != nil {
			return nil, err
		}
		if binding.Encrypted {
			field.value, err = p.Encryption.encrypt(field.value)
			if err != nil {
				return nil, err
			}
		}
		state := map[string]any{
			"key": binding.Key, "namespace": binding.Namespace,
			"value": field.value, "originalProtoCase": "stringValue",
			"protoKey": map[string]any{"$type": "proto.sdui.Key", "value": map[string]any{"$case": "id", "id": binding.Key}},
		}
		if field.name == "apfc" {
			state["value"] = ""
			delete(state, "originalProtoCase")
		}
		states = append(states, state)
	}
	return marshalLoginAction(p.Action, states, p.ScreenID)
}

func marshalLoginAction(action map[string]any, states []any, screenID string) ([]byte, error) {
	arguments := object(action["requestedArguments"])
	args := make(map[string]any, len(arguments)+3)
	for key, value := range arguments {
		args[key] = value
	}
	args["states"] = states
	args["screenId"] = screenID
	args["knownTemplateIds"] = []string{}
	return json.Marshal(map[string]any{
		"requestId": passwordAuthenticationRequest, "serverRequest": action,
		"states": states, "requestedArguments": args,
	})
}

type passwordLoginResponse struct {
	InlineCredentialError bool `json:"-"`
	Response              struct {
		Errors           []json.RawMessage `json:"errors"`
		IsRetryable      bool              `json:"isRetryable"`
		CompletionAction struct {
			Actions []struct {
				Type  string `json:"$type"`
				Value struct {
					Content struct {
						NewComponent any `json:"newComponent"`
						URL          struct {
							URLValue struct {
								URL string `json:"url"`
							} `json:"urlValue"`
						} `json:"url"`
					} `json:"content"`
				} `json:"value"`
			} `json:"actions"`
		} `json:"completionAction"`
	} `json:"response"`
}

func parsePasswordLoginResponse(body []byte) (*passwordLoginResponse, error) {
	for _, line := range bytes.Split(body, []byte{'\n'}) {
		id, data, found := bytes.Cut(line, []byte{':'})
		if !found || string(id) != "0" {
			continue
		}
		var envelope map[string]json.RawMessage
		if json.Unmarshal(data, &envelope) != nil || envelope["response"] == nil {
			return nil, errors.New("unsupported login response envelope")
		}
		var result passwordLoginResponse
		decoder := json.NewDecoder(bytes.NewReader(data))
		if decoder.Decode(&result) != nil {
			return nil, errors.New("invalid login response")
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, errors.New("invalid login response framing")
		}
		for _, action := range result.Response.CompletionAction.Actions {
			if action.Type == "proto.sdui.actions.core.ReplaceComponent" && hasInlineCredentialError(action.Value.Content.NewComponent, false, 0) {
				result.InlineCredentialError = true
			}
		}
		return &result, nil
	}
	return nil, errors.New("login response record was not found")
}

// LinkedIn can report credential errors through UI replacement actions while
// leaving response.errors empty. Match only the observed password feedback;
// arbitrary provider text stays out of logs and client-facing errors.
func hasInlineCredentialError(value any, feedback bool, depth int) bool {
	if depth > 100 {
		return false
	}
	switch value := value.(type) {
	case map[string]any:
		feedback = feedback || object(value["viewTrackingSpecs"])["viewName"] == "identifier-password-inline-feedback"
		for _, child := range value {
			if hasInlineCredentialError(child, feedback, depth+1) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if hasInlineCredentialError(child, feedback, depth+1) {
				return true
			}
		}
	case string:
		return feedback && value == "Wrong email or password."
	}
	return false
}

type loginEncryption struct {
	PublicKey string
	Salt      string
	TokenTTL  float64
	Observed  time.Time
}

func findLoginEncryption(value any, depth int) *loginEncryption {
	if depth > 100 {
		return nil
	}
	switch value := value.(type) {
	case map[string]any:
		key, hasKey := value["rsaPublicKey"].(string)
		salt, hasSalt := value["encryptionSalt"].(string)
		if hasKey && hasSalt {
			ttl, _ := value["tokenTtlMs"].(float64)
			return &loginEncryption{PublicKey: key, Salt: salt, TokenTTL: ttl, Observed: time.Now()}
		}
		for _, child := range value {
			if encryption := findLoginEncryption(child, depth+1); encryption != nil {
				return encryption
			}
		}
	case []any:
		for _, child := range value {
			if encryption := findLoginEncryption(child, depth+1); encryption != nil {
				return encryption
			}
		}
	}
	return nil
}

// Match the login application's cloak-token format: version bytes 1,1 followed
// by RSA-2048 OAEP(SHA-1) of the eight-byte server salt and UTF-8 field value.
// The public key and salt must come from this attempt's fresh bootstrap. SHA-1
// is required by LinkedIn's WebCrypto importKey contract, not a local choice.
func (e *loginEncryption) encrypt(value string) (string, error) {
	invalid := errors.New("unsupported login field encryption")
	if e == nil {
		return "", invalid
	}
	der, err := base64.StdEncoding.DecodeString(e.PublicKey)
	if err != nil {
		return "", invalid
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return "", invalid
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok || key.N.BitLen() != 2048 {
		return "", invalid
	}
	salt, err := base64.StdEncoding.DecodeString(e.Salt)
	if err != nil || len(salt) != 8 {
		return "", invalid
	}
	if e.TokenTTL > 0 && !e.Observed.IsZero() {
		elapsed := time.Since(e.Observed).Milliseconds()
		serverTime := binary.BigEndian.Uint64(salt)
		if serverTime != 0 && elapsed > 0 {
			binary.BigEndian.PutUint64(salt, serverTime+uint64(elapsed))
		}
	}
	plaintext := make([]byte, 8+len(value))
	copy(plaintext, salt)
	copy(plaintext[8:], value)
	defer clear(plaintext)
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, key, plaintext, nil)
	if err != nil || len(ciphertext) != 256 {
		return "", invalid
	}
	token := append([]byte{1, 1}, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(token), nil
}
