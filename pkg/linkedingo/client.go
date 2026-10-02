// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2025 Sumner Evans
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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/random"
)

const BrowserName = "Chrome"
const ChromeVersion = "141"
const UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + ChromeVersion + ".0.0.0 Safari/537.36"
const SecCHUserAgent = `"Chromium";v="` + ChromeVersion + `", "Google Chrome";v="` + ChromeVersion + `", "Not-A.Brand";v="99"`
const OSName = "Linux"
const SecCHPlatform = `"` + OSName + `"`
const SecCHMobile = "?0"
const SecCHPrefersColorScheme = "light"

var chromeVersionRegex = regexp.MustCompile(`Chrome/(\d+)`)

// browserIdentity is the set of headers by which the client identifies itself.
// LinkedIn cross-checks these against each other and against the session the
// cookies were issued to, so they always have to describe one browser.
type browserIdentity struct {
	userAgent string

	// User-Agent Client Hints are a Chromium feature: Firefox and Safari send
	// none of the sec-ch-* headers. When the identity is not Chromium these are
	// unset and the headers are omitted entirely, because a Firefox user agent
	// arriving with Chromium client hints is its own contradiction.
	sendClientHints bool
	secCHUA         string
	secCHUAMobile   string
	secCHUAPlatform string
}

func defaultBrowserIdentity() browserIdentity {
	return browserIdentity{
		userAgent:       UserAgent,
		sendClientHints: true,
		secCHUA:         SecCHUserAgent,
		secCHUAMobile:   SecCHMobile,
		secCHUAPlatform: SecCHPlatform,
	}
}

// browserIdentityFromUserAgent builds a self-consistent identity from a stored
// user agent. An empty user agent yields the compile-time default; a
// non-Chromium one is used as-is with client hints suppressed.
func browserIdentityFromUserAgent(userAgent string) browserIdentity {
	if userAgent == "" {
		return defaultBrowserIdentity()
	}

	identity := browserIdentity{userAgent: userAgent}
	match := chromeVersionRegex.FindStringSubmatch(userAgent)
	if match == nil {
		return identity
	}
	version := match[1]

	identity.sendClientHints = true
	identity.secCHUA = fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not-A.Brand";v="99"`, version, version)
	identity.secCHUAMobile = SecCHMobile
	if strings.Contains(userAgent, "Mobile") || strings.Contains(userAgent, "Android") {
		identity.secCHUAMobile = "?1"
	}
	identity.secCHUAPlatform = fmt.Sprintf("%q", platformFromUserAgent(userAgent))
	return identity
}

func platformFromUserAgent(userAgent string) string {
	switch {
	case strings.Contains(userAgent, "Macintosh"):
		return "macOS"
	case strings.Contains(userAgent, "Windows"):
		return "Windows"
	case strings.Contains(userAgent, "Android"):
		return "Android"
	case strings.Contains(userAgent, "CrOS"):
		return "Chrome OS"
	case strings.Contains(userAgent, "iPhone"), strings.Contains(userAgent, "iPad"):
		return "iOS"
	default:
		return "Linux"
	}
}
const ServiceVersion = "1.13.40953"
const defaultXLiTrack = `{"clientVersion":"` + ServiceVersion + `","mpVersion":"` + ServiceVersion + `","osName":"web","deviceFormFactor":"DESKTOP","mpName":"voyager-web","displayDensity":2,"displayWidth":2880,"displayHeight":1800}`

type Client struct {
	http          *http.Client
	jar           *StringCookieJar
	userEntityURN URN

	realtimeSessionID uuid.UUID
	realtimeCancelFn  context.CancelFunc
	realtimeWaitGroup sync.WaitGroup

	handlers Handlers

	pageInstance   string
	xLITrack       string
	serviceVersion string

	identity browserIdentity

	conversationsSyncToken string
}

func NewClient(ctx context.Context, userEntityURN URN, jar *StringCookieJar, pageInstance, xLiTrack, userAgent, conversationsSyncToken string, handlers Handlers) *Client {
	log := zerolog.Ctx(ctx)
	if xLiTrack == "" {
		log.Warn().Msg("x-li-track is empty, using default")
		xLiTrack = defaultXLiTrack
	}
	if pageInstance == "" {
		log.Warn().Msg("pageInstance is empty, using default")
		pageInstance = "urn:li:page:messaging_thread;5accf988-7540-4d0a-8a28-a0732bf6de20"
	}

	trackingData := map[string]any{}
	if err := json.Unmarshal([]byte(xLiTrack), &trackingData); err != nil {
		log.Warn().Err(err).Msg("failed to parse x-li-track")
	}
	serviceVersion, _ := trackingData["mpVersion"].(string)
	if serviceVersion == "" {
		log.Warn().Msg("mpVersion is empty, using default")
		serviceVersion = ServiceVersion
	}

	// Possible workaround for a/b testing where the frontend appears to be a completely different version
	mpName, _ := trackingData["mpName"].(string)
	if mpName != "voyager-web" {
		log.Warn().Msg("mpName is not voyager-web, using default xLiTrack")
		xLiTrack = defaultXLiTrack
		pageInstance = "urn:li:page:d_flagship3_messaging_conversation_detail;" + base64.StdEncoding.EncodeToString(random.Bytes(16))
	}

	// The cookies were issued to a specific browser, so keep identifying
	// ourselves as that browser rather than as the compile-time default.
	identity := browserIdentityFromUserAgent(userAgent)
	if userAgent == "" {
		log.Warn().Msg("no user agent stored for this login, using default browser identity")
	} else if !identity.sendClientHints {
		log.Debug().Msg("stored user agent is not Chromium, omitting client hint headers")
	}

	cli := &Client{
		userEntityURN:          userEntityURN,
		jar:                    jar,
		pageInstance:           pageInstance,
		xLITrack:               xLiTrack,
		serviceVersion:         serviceVersion,
		identity:               identity,
		realtimeSessionID:      uuid.New(),
		handlers:               handlers,
		conversationsSyncToken: conversationsSyncToken,
	}
	cli.http = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Jar:           jar,
		CheckRedirect: cli.checkHTTPRedirect,
	}
	return cli
}

func (c *Client) IsLoggedIn() bool {
	return c.jar.GetCookie(LinkedInCookieJSESSIONID) != ""
}

type Handlers struct {
	Heartbeat              func(context.Context)
	ClientConnection       func(context.Context, *ClientConnection)
	TransientDisconnect    func(context.Context, error)
	BadCredentials         func(context.Context, error)
	UnknownError           func(context.Context, error)
	DecoratedEvent         func(context.Context, *DecoratedEvent)
	ConversationsSyncToken func(context.Context, string)
}

func (h Handlers) onHeartbeat(ctx context.Context) {
	if h.Heartbeat != nil {
		h.Heartbeat(ctx)
	}
}

func (h Handlers) onClientConnection(ctx context.Context, conn *ClientConnection) {
	if h.ClientConnection != nil {
		h.ClientConnection(ctx, conn)
	}
}

func (h Handlers) onTransientDisconnect(ctx context.Context, err error) {
	if h.TransientDisconnect != nil {
		h.TransientDisconnect(ctx, err)
	}
}

func (h Handlers) onBadCredentials(ctx context.Context, err error) {
	if h.BadCredentials != nil {
		h.BadCredentials(ctx, err)
	}
}

func (h Handlers) onUnknownError(ctx context.Context, err error) {
	if h.UnknownError != nil {
		h.UnknownError(ctx, err)
	}
}

func (h Handlers) onDecoratedEvent(ctx context.Context, decoratedEvent *DecoratedEvent) {
	if h.DecoratedEvent != nil {
		h.DecoratedEvent(ctx, decoratedEvent)
	}
}

func (h Handlers) onConversationsSyncToken(ctx context.Context, conversationsSyncToken string) {
	if h.ConversationsSyncToken != nil {
		h.ConversationsSyncToken(ctx, conversationsSyncToken)
	}
}
