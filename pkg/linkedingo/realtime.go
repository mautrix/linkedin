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
	"golang.org/x/net/html"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
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
	ID uuid.UUID `json:"id"`
}

type DecoratedEvent struct {
	Topic               types.URN             `json:"topic,omitempty"`
	LeftServerAt        jsontime.UnixMilli    `json:"leftServerAt,omitempty"`
	ID                  string                `json:"id,omitempty"`
	Payload             DecoratedEventPayload `json:"payload,omitempty"`
	TrackingID          string                `json:"trackingId,omitempty"`
	PublisherTrackingID string                `json:"publisherTrackingId,omitempty"`
}

type DecoratedEventPayload struct {
	Data DecoratedEventData `json:"data,omitempty"`
}

type DecoratedEventData struct {
	Type                     string                    `json:"_type,omitempty"`
	DecoratedMessage         *DecoratedMessage         `json:"doDecorateMessageMessengerRealtimeDecoration,omitempty"`
	DecoratedTypingIndicator *DecoratedTypingIndicator `json:"doDecorateTypingIndicatorMessengerRealtimeDecoration,omitempty"`
	DecoratedSeenReceipt     *DecoratedSeenReceipt     `json:"doDecorateSeenReceiptMessengerRealtimeDecoration,omitempty"`
	DecoratedReactionSummary *DecoratedReactionSummary `json:"doDecorateRealtimeReactionSummaryMessengerRealtimeDecoration,omitempty"`
}

type DecoratedMessage struct {
	Result types.Message `json:"result,omitempty"`
}

type DecoratedTypingIndicator struct {
	Result types.RealtimeTypingIndicator `json:"result,omitempty"`
}

type DecoratedSeenReceipt struct {
	Result types.SeenReceipt `json:"result,omitempty"`
}

type DecoratedReactionSummary struct {
	Result types.RealtimeReactionSummary `json:"result,omitempty"`
}

func (c *Client) cacheMetaValues(ctx context.Context) error {
	if c.clientPageInstanceID != "" && c.xLITrack != "" && c.i18nLocale != "" {
		return nil
	}

	resp, err := c.newAuthedRequest(http.MethodGet, linkedInMessagingBaseURL).
		WithWebpageHeaders().
		Do(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("messages page returned status code %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}
	var crawl func(*html.Node) error
	crawl = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var name, content string
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				}
				if a.Key == "content" {
					content = a.Val
				}
			}
			switch name {
			case "clientPageInstanceId":
				c.clientPageInstanceID = content
			case "serviceVersion":
				c.serviceVersion = content
				xLITrack, err := json.Marshal(map[string]any{
					"clientVersion":    content,
					"mpVersion":        content,
					"osName":           "web",
					"timezoneOffset":   2,                  // TODO scrutinize
					"timezone":         "Europe/Stockholm", // TODO scrutinize
					"deviceFormFactor": "DESKTOP",
					"mpName":           "voyager-web",
					"displayDensity":   1.125,
					"displayWidth":     2560.5,
					"displayHeight":    1440,
				})
				if err != nil {
					return err
				}
				c.xLITrack = string(xLITrack)
			case "i18nLocale":
				c.i18nLocale = content
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if err := crawl(child); err != nil {
				return err
			}
		}
		return nil
	}
	return crawl(doc)
}

func (c *Client) RealtimeConnect(ctx context.Context) error {
	if err := c.cacheMetaValues(ctx); err != nil {
		return err
	}
	log := zerolog.Ctx(ctx).With().
		Str("loop", "realtime_connect").
		Str("client_page_instance_id", c.clientPageInstanceID).
		Logger()
	ctx = log.WithContext(ctx)

	c.realtimeCtx, c.realtimeCancelFn = context.WithCancel(ctx)
	go c.runHeartbeatsLoop(c.realtimeCtx)
	go c.realtimeConnectLoop(c.realtimeCtx)
	return nil
}

func (c *Client) runHeartbeatsLoop(ctx context.Context) {
	isFirst := true
	userURN := c.userEntityURN.WithPrefix("urn", "li", "fsd_profile").String()

	log := zerolog.Ctx(ctx).With().Str("user_urn", userURN).Logger()
	log.Info().Msg("Starting heartbeats loop")
	for {
		log.Debug().Stringer("realtime_session_id", c.realtimeSessionID).Msg("Sending heartbeat")

		_, err := c.newAuthedRequest(http.MethodPost, linkedInRealtimeHeartbeatURL).
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
			Do(ctx)
		if err != nil {
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
	// Continually reconnect to the realtime connection endpoint until the
	// context is done.
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Realtime connection loop canceled")
			return
		default:
		}

		var err error
		c.realtimeResp, err = c.newAuthedRequest(http.MethodGet, linkedInRealtimeConnectURL).
			WithCSRF().
			WithRealtimeConnectHeaders().
			WithHeader("Accept", contentTypeTextEventStream).
			Do(ctx)
		if err != nil {
			c.handlers.onRealtimeConnectError(ctx, err)
			return
		}
		if c.realtimeResp.StatusCode != http.StatusOK {
			c.handlers.onRealtimeConnectError(ctx, fmt.Errorf("failed to connect due to status code %d", c.realtimeResp.StatusCode))
			return
		}

		log.Info().Stringer("realtime_session_id", c.realtimeSessionID).Msg("Reading realtime stream")
		reader := bufio.NewReader(c.realtimeResp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if errors.Is(err, io.EOF) {
					break
				}
				c.handlers.onRealtimeConnectError(ctx, err)
				break
			}

			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}

			var realtimeEvent RealtimeEvent
			if err = json.Unmarshal(line[6:], &realtimeEvent); err != nil {
				c.handlers.onRealtimeConnectError(ctx, err)
				break
			}

			switch {
			case realtimeEvent.Heartbeat != nil:
				c.handlers.onHeartbeat(ctx)
			case realtimeEvent.ClientConnection != nil:
				c.handlers.onClientConnection(ctx, realtimeEvent.ClientConnection)
			case realtimeEvent.DecoratedEvent != nil:
				log.Debug().
					Stringer("topic", realtimeEvent.DecoratedEvent.Topic).
					Str("payload_type", realtimeEvent.DecoratedEvent.Payload.Data.Type).
					Msg("Received decorated event")
				fmt.Printf("%s\n", line)
				c.handlers.onDecoratedEvent(ctx, realtimeEvent.DecoratedEvent)
			}
		}
	}
}

func (c *Client) RealtimeDisconnect() {
	if c.realtimeCancelFn != nil {
		c.realtimeCancelFn()
	}
}
