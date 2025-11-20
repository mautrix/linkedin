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
	"net/http"
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
const ServiceVersion = "1.13.39523"
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

	syncToken string
}

func NewClient(ctx context.Context, userEntityURN URN, jar *StringCookieJar, pageInstance, xLiTrack string, handlers Handlers) *Client {
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

	cli := &Client{
		userEntityURN:     userEntityURN,
		jar:               jar,
		pageInstance:      pageInstance,
		xLITrack:          xLiTrack,
		serviceVersion:    serviceVersion,
		realtimeSessionID: uuid.New(),
		handlers:          handlers,
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

type Handlers struct {
	Heartbeat           func(context.Context)
	ClientConnection    func(context.Context, *ClientConnection)
	TransientDisconnect func(context.Context, error)
	BadCredentials      func(context.Context, error)
	UnknownError        func(context.Context, error)
	DecoratedEvent      func(context.Context, *DecoratedEvent)
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
