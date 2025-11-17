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
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exerrors"
	"go.mau.fi/util/jsontime"
)

var MaxConnectionAttempts = 50

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
		c.runHeartbeatsLoop(ctx)
	}()

	c.realtimeWaitGroup.Add(1)
	go func() {
		defer c.realtimeWaitGroup.Done()
		c.realtimeConnectLoop(ctx)
	}()

	return nil
}

func (c *Client) runHeartbeatsLoop(ctx context.Context) {
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

	connectAttempts := 0

	// Continually reconnect to the realtime connection endpoint until the context is done
	for {
		realtimeResp, err := c.newAuthedRequest(http.MethodGet, linkedInRealtimeConnectURL).
			WithQueryParam("rc", "1").
			WithCSRF().
			WithRealtimeConnectHeaders().
			WithHeader("Accept", contentTypeTextEventStream).
			DoRaw(ctx)
		if errors.Is(err, ErrTokenInvalidated) {
			c.handlers.onBadCredentials(ctx, err)
			return
		} else if err != nil {
			connectAttempts += 1
			if connectAttempts > MaxConnectionAttempts {
				c.handlers.onUnknownError(ctx, fmt.Errorf("failed to connect: %w", err))
				return
			}
			c.handlers.onTransientDisconnect(ctx, fmt.Errorf("failed to connect: %w", err))
			backoff := time.Duration(connectAttempts*2) * time.Second
			if backoff > time.Minute {
				backoff = time.Minute
			}
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				log.Info().Msg("Realtime connection loop canceled")
				return
			}
		} else if realtimeResp.StatusCode != http.StatusOK {
			switch realtimeResp.StatusCode {
			case http.StatusUnauthorized, http.StatusFound:
				c.handlers.onBadCredentials(ctx, fmt.Errorf("got %d on connect", realtimeResp.StatusCode))
				return
			case http.StatusBadRequest:
				log.Warn().Msg("Got 400 on connect, resetting realtime session ID")
				c.realtimeSessionID = uuid.New()
				fallthrough
			default:
				connectAttempts += 1
				if connectAttempts > MaxConnectionAttempts {
					c.handlers.onUnknownError(ctx, fmt.Errorf("failed to connect due to status code %d", realtimeResp.StatusCode))
					return
				}
				c.handlers.onTransientDisconnect(ctx, fmt.Errorf("failed to connect due to status code: %d", realtimeResp.StatusCode))
				backoff := time.Duration(connectAttempts*2) * time.Second
				if backoff > time.Minute {
					backoff = time.Minute
				}
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					log.Info().Msg("Realtime connection loop canceled")
					return
				}
			}
		}

		// Reset connection attempts
		connectAttempts = 0

		log.Info().Stringer("realtime_session_id", c.realtimeSessionID).Msg("Reading realtime stream")
		reader := bufio.NewReader(realtimeResp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, context.Canceled) {
					log.Info().Msg("Realtime connection loop canceled")
					return
				} else if errors.Is(err, io.EOF) {
					log.Info().
						Stringer("realtime_session_id", c.realtimeSessionID).
						Msg("Realtime stream closed")
					break
				} else {
					c.handlers.onTransientDisconnect(ctx, fmt.Errorf("failed to read realtime stream: %w", err))
					break
				}
			}

			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}

			var realtimeEvent RealtimeEvent
			if err = json.Unmarshal(line[6:], &realtimeEvent); err != nil {
				c.handlers.onTransientDisconnect(ctx, fmt.Errorf("failed to unmarshal realtime event: %w", err))
				break
			}

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
	}
}

func (c *Client) RealtimeDisconnect() {
	if c.realtimeCancelFn != nil {
		c.realtimeCancelFn()
	}
	c.realtimeWaitGroup.Wait()
}
