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
	"net/http"

	"github.com/google/uuid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
	"go.mau.fi/mautrix-linkedin/pkg/stringcookiejar"
)

type Client struct {
	http          *http.Client
	jar           *stringcookiejar.Jar
	userEntityURN types.URN

	realtimeSessionID uuid.UUID
	realtimeCtx       context.Context
	realtimeCancelFn  context.CancelFunc
	realtimeResp      *http.Response

	handlers Handlers

	clientPageInstanceID string
	serviceVersion       string
	xLITrack             string
	i18nLocale           string
}

func NewClient(ctx context.Context, userEntityURN types.URN, jar *stringcookiejar.Jar, handlers Handlers) *Client {
	return &Client{
		userEntityURN: userEntityURN,
		http: &http.Client{
			Jar: jar,

			// Disallow redirects entirely:
			// https://stackoverflow.com/a/38150816/2319844
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		jar: jar,

		realtimeSessionID: uuid.New(),

		handlers: handlers,
	}
}

type Handlers struct {
	Heartbeat            func(context.Context)
	ClientConnection     func(context.Context, *types.ClientConnection)
	RealtimeConnectError func(context.Context, error)
	DecoratedEvent       func(context.Context, *types.DecoratedEvent)
}

func (h Handlers) onHeartbeat(ctx context.Context) {
	if h.Heartbeat != nil {
		h.Heartbeat(ctx)
	}
}

func (h Handlers) onClientConnection(ctx context.Context, conn *types.ClientConnection) {
	if h.ClientConnection != nil {
		h.ClientConnection(ctx, conn)
	}
}

func (h Handlers) onRealtimeConnectError(ctx context.Context, err error) {
	if h.RealtimeConnectError != nil {
		h.RealtimeConnectError(ctx, err)
	}
}

func (h Handlers) onDecoratedEvent(ctx context.Context, decoratedEvent *types.DecoratedEvent) {
	if h.DecoratedEvent != nil {
		h.DecoratedEvent(ctx, decoratedEvent)
	}
}
