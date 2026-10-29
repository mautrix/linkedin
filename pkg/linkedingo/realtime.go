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
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exerrors"
	"go.mau.fi/util/jsontime"
)

const (
	realtimeMinBackoff   = 2 * time.Second
	realtimeMaxBackoff   = 5 * time.Minute
	realtimeStableUptime = time.Minute
)

//go:embed x-li-recipe-map.json
var realtimeRecipeMapJSON []byte

//go:embed x-li-query-map.json
var realtimeQueryMapJSON []byte

var realtimeRecipeMap, realtimeQueryMap string

func init() {
	var x any
	exerrors.PanicIfNotNil(json.Unmarshal(realtimeRecipeMapJSON, &x))
	realtimeRecipeMap = string(exerrors.Must(json.Marshal(x)))
	exerrors.PanicIfNotNil(json.Unmarshal(realtimeQueryMapJSON, &x))
	realtimeQueryMap = string(exerrors.Must(json.Marshal(x)))
}

type RealtimeEvent struct {
	Heartbeat        *Heartbeat        `json:"com.linkedin.realtimefrontend.Heartbeat,omitempty"`
	ClientConnection *ClientConnection `json:"com.linkedin.realtimefrontend.ClientConnection,omitempty"`
	DecoratedEvent   *DecoratedEvent   `json:"com.linkedin.realtimefrontend.DecoratedEvent,omitempty"`
}

type Heartbeat struct{}

type ClientConnection struct {
	ID     uuid.UUID `json:"id"`
	SessID uuid.UUID `json:"-"`
}

type DecoratedEvent struct {
	Topic        URN                   `json:"topic,omitempty"`
	LeftServerAt jsontime.UnixMilli    `json:"leftServerAt,omitempty"`
	ID           string                `json:"id,omitempty"`
	Payload      DecoratedEventPayload `json:"payload,omitempty"`
}

type DecoratedEventPayload struct {
	Data DecoratedEventData `json:"data,omitempty"`
}

type DecoratedEventData struct {
	Type                        string                    `json:"_type,omitempty"`
	DecoratedConversation       *DecoratedConversation    `json:"doDecorateConversationMessengerRealtimeDecoration,omitempty"`
	DecoratedConversationDelete *DecoratedConversation    `json:"doDecorateConversationDeleteMessengerRealtimeDecoration,omitempty"`
	DecoratedMessage            *DecoratedMessage         `json:"doDecorateMessageMessengerRealtimeDecoration,omitempty"`
	DecoratedTypingIndicator    *DecoratedTypingIndicator `json:"doDecorateTypingIndicatorMessengerRealtimeDecoration,omitempty"`
	DecoratedSeenReceipt        *DecoratedSeenReceipt     `json:"doDecorateSeenReceiptMessengerRealtimeDecoration,omitempty"`
	DecoratedReactionSummary    *DecoratedReactionSummary `json:"doDecorateRealtimeReactionSummaryMessengerRealtimeDecoration,omitempty"`
}

func (c *Client) RealtimeConnect(ctx context.Context) error {
	log := zerolog.Ctx(ctx).With().
		Str("loop", "realtime_connect").
		Str("page_instance", c.pageInstance).
		Logger()
	ctx, c.realtimeCancelFn = context.WithCancel(log.WithContext(ctx))

	c.realtimeWaitGroup.Add(1)
	go func() {
		defer c.realtimeWaitGroup.Done()
		c.realtimeConnectLoop(ctx)
	}()

	return nil
}

func (c *Client) runHeartbeatsLoop(ctx context.Context, stopStream context.CancelCauseFunc) {
	isFirst := true
	userURN := c.userEntityURN.WithPrefix("urn", "li", "fsd_profile").String()

	log := zerolog.Ctx(ctx).With().Str("user_urn", userURN).Logger()
	log.Info().Msg("Starting heartbeats loop")
	defer log.Info().Msg("Exited heartbeats loop")

	for {
		log.Debug().Stringer("realtime_session_id", c.realtimeSessionID).Msg("Sending heartbeat")

		_, err := c.newAuthedRequest(http.MethodPost, linkedInRealtimeHeartbeatURL).
			WithQueryParam("action", "sendHeartbeat").
			WithHeader("accept", "*/*").
			WithContentType(contentTypePlaintextUTF8).
			WithCSRF().
			WithHeader("origin", "https://www.linkedin.com").
			WithHeader("Priority", "u=1, i").
			WithXLIHeaders().
			WithJSONPayload(map[string]any{
				"isFirstHeartbeat":  !isFirst,
				"isLastHeartbeat":   false,
				"realtimeSessionId": c.realtimeSessionID.String(),
				"mpName":            "voyager-web",
				"mpVersion":         c.serviceVersion,
				"clientId":          "voyager-web",
				"actorUrn":          userURN,
				"contextUrns":       []string{userURN},
			}).
			DoRaw(ctx)
		if errors.Is(err, context.Canceled) {
			log.Info().Msg("Heartbeats loop canceled")
			return
		} else if err != nil {
			log.Err(err).Msg("Failed to send heartbeat")
			stopStream(fmt.Errorf("failed to send heartbeat: %w", err))
			return
		}

		isFirst = false
		select {
		case <-ctx.Done():
			log.Info().Msg("Heartbeats loop canceled")
			return
		case <-time.After(time.Minute):
		}
	}
}

func (c *Client) realtimeConnectLoop(ctx context.Context) {
	log := zerolog.Ctx(ctx)
	log.Info().Msg("Starting realtime connection loop")
	defer log.Info().Msg("Exited realtime connection loop")

	backoff := realtimeMinBackoff

	// Continually reconnect to the realtime connection endpoint until the context is done
	for {
		uptime, err := c.readRealtimeStream(ctx)
		if ctx.Err() != nil || !c.IsLoggedIn() {
			return
		}
		stable := uptime >= realtimeStableUptime
		if stable {
			backoff = realtimeMinBackoff
		}
		if stable && errors.Is(err, io.EOF) {
			log.Info().
				Stringer("realtime_session_id", c.realtimeSessionID).
				Msg("Realtime stream closed")
		} else {
			c.handlers.onTransientDisconnect(ctx, err)
		}

		select {
		case <-time.After(backoff/2 + rand.N(backoff/2)):
		case <-ctx.Done():
			return
		}
		backoff = min(backoff*2, realtimeMaxBackoff)
	}
}

// readRealtimeStream connects to the realtime stream and dispatches its events until it ends, returning how long
// the stream was open and why it ended.
func (c *Client) readRealtimeStream(ctx context.Context) (time.Duration, error) {
	log := zerolog.Ctx(ctx)
	streamCtx, stopStream := context.WithCancelCause(ctx)
	defer stopStream(nil)

	realtimeResp, err := c.newAuthedRequest(http.MethodGet, linkedInRealtimeConnectURL).
		WithQueryParam("rc", "1").
		WithCSRF().
		WithRealtimeConnectHeaders().
		WithHeader("Accept", contentTypeTextEventStream).
		DoRaw(streamCtx)
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %w", err)
	}
	defer realtimeResp.Body.Close()

	switch realtimeResp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusFound:
		err = fmt.Errorf("failed to connect: %w", &ResponseError{StatusCode: realtimeResp.StatusCode})
		c.onAuthRejected(ctx, err)
		return 0, err
	case http.StatusBadRequest:
		log.Warn().Msg("Got 400 on connect, resetting realtime session ID")
		c.realtimeSessionID = uuid.New()
		fallthrough
	default:
		return 0, fmt.Errorf("failed to connect: %w", &ResponseError{StatusCode: realtimeResp.StatusCode})
	}

	connectedAt := time.Now()
	c.realtimeWaitGroup.Add(1)
	go func() {
		defer c.realtimeWaitGroup.Done()
		c.runHeartbeatsLoop(streamCtx, stopStream)
	}()

	log.Info().Stringer("realtime_session_id", c.realtimeSessionID).Msg("Reading realtime stream")
	reader := bufio.NewReader(realtimeResp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if streamCtx.Err() != nil {
			return time.Since(connectedAt), context.Cause(streamCtx)
		} else if err != nil {
			return time.Since(connectedAt), fmt.Errorf("failed to read realtime stream: %w", err)
		}

		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}

		var realtimeEvent RealtimeEvent
		if err = json.Unmarshal(line[6:], &realtimeEvent); err != nil {
			return time.Since(connectedAt), fmt.Errorf("failed to unmarshal realtime event: %w", err)
		}

		c.dispatchRealtimeEvent(ctx, realtimeEvent)
	}
}

func (c *Client) dispatchRealtimeEvent(ctx context.Context, realtimeEvent RealtimeEvent) {
	defer func() {
		if err := recover(); err != nil {
			zerolog.Ctx(ctx).
				Err(fmt.Errorf("%v", err)).
				Bytes(zerolog.ErrorStackFieldName, debug.Stack()).
				Type("event_type", realtimeEvent).
				Msg("Panic in event handler")
		}
	}()

	log := zerolog.Ctx(ctx)
	switch {
	case realtimeEvent.Heartbeat != nil:
		log.Trace().Msg("Received heartbeat")
		c.handlers.onHeartbeat(ctx)
	case realtimeEvent.ClientConnection != nil:
		log.Info().Msg("Client connected")
		realtimeEvent.ClientConnection.SessID = c.realtimeSessionID
		c.handlers.onClientConnection(ctx, realtimeEvent.ClientConnection)
	case realtimeEvent.DecoratedEvent != nil:
		log.Debug().
			Stringer("topic", realtimeEvent.DecoratedEvent.Topic).
			Str("payload_type", realtimeEvent.DecoratedEvent.Payload.Data.Type).
			Msg("Received decorated event")
		c.handlers.onDecoratedEvent(ctx, realtimeEvent.DecoratedEvent)
	}
}

func (c *Client) RealtimeDisconnect() {
	if c.realtimeCancelFn != nil {
		c.realtimeCancelFn()
	}
	c.realtimeWaitGroup.Wait()
}
