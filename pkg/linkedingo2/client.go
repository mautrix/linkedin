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

package linkedingo2

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo2/types2"
	"go.mau.fi/mautrix-linkedin/pkg/stringcookiejar"
)

type Handlers struct {
	Heartbeat            func(context.Context)
	ClientConnection     func(context.Context, *types2.ClientConnection)
	RealtimeConnectError func(context.Context, error)
	DecoratedMessage     func(context.Context, *types2.DecoratedMessageRealtime)
}

func (h Handlers) onHeartbeat(ctx context.Context) {
	if h.Heartbeat != nil {
		h.Heartbeat(ctx)
	}
}

func (h Handlers) onClientConnection(ctx context.Context, conn *types2.ClientConnection) {
	if h.ClientConnection != nil {
		h.ClientConnection(ctx, conn)
	}
}

func (h Handlers) onRealtimeConnectError(ctx context.Context, err error) {
	if h.RealtimeConnectError != nil {
		h.RealtimeConnectError(ctx, err)
	}
}

func (h Handlers) onDecoratedMessage(ctx context.Context, msg *types2.DecoratedMessageRealtime) {
	if h.DecoratedMessage != nil {
		h.DecoratedMessage(ctx, msg)
	}
}

type Client struct {
	http *http.Client
	jar  *stringcookiejar.Jar

	realtimeSessionID uuid.UUID
	realtimeCtx       context.Context
	realtimeCancelFn  context.CancelFunc
	realtimeResp      *http.Response

	handlers Handlers

	clientPageInstanceID string
	xLITrack             string
	i18nLocale           string
}

func NewClient(ctx context.Context, jar *stringcookiejar.Jar, handlers Handlers) *Client {
	return &Client{
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
