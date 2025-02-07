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

	"github.com/rs/zerolog"
	"go.mau.fi/util/exerrors"
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

func (c *Client) cacheMetaValues(ctx context.Context) error {
	if c.clientPageInstanceID != "" && c.xLITrack != "" && c.i18nLocale != "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, linkedInMessagingBaseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Sec-Fetch-Dest", "document")
	req.Header.Add("Sec-Fetch-Mode", "navigate")
	req.Header.Add("Sec-Fetch-Site", "none")
	req.Header.Add("Sec-Fetch-User", "?1")
	req.Header.Add("Upgrade-Insecure-Requests", "1")

	resp, err := c.http.Do(req)
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

	log.Info().Msg("Starting realtime connection loop")

	c.realtimeCtx, c.realtimeCancelFn = context.WithCancel(ctx)
	// TODO run sendHeartbeat loop
	go c.realtimeConnectLoop()
	return nil
}

func (c *Client) realtimeConnectLoop() {
	log := zerolog.Ctx(c.realtimeCtx)
	// Continually reconnect to the realtime connection endpoint until the
	// context is done.
	for {
		select {
		case <-c.realtimeCtx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(c.realtimeCtx, http.MethodGet, linkedInRealtimeConnectURL, nil)
		if err != nil {
			c.handlers.onRealtimeConnectError(c.realtimeCtx, err)
			return
		}
		req.Header.Add("Accept", contentTypeTextEventStream)
		req.Header.Add("x-li-realtime-session", c.realtimeSessionID.String())
		req.Header.Add("x-li-recipe-accept", contentTypeJSONLinkedInNormalized)
		req.Header.Add("x-li-query-accept", contentTypeGraphQL)
		req.Header.Add("x-li-accept", contentTypeJSONLinkedInNormalized)
		req.Header.Add("x-li-recipe-map", realtimeRecipeMap)
		req.Header.Add("x-li-query-map", realtimeQueryMap)
		req.Header.Add("csrf-token", c.getCSRFToken())
		req.Header.Add("referer", linkedInMessagingBaseURL+"/")
		req.Header.Add("x-restli-protocol-version", "2.0.0")
		req.Header.Add("x-li-track", c.xLITrack)
		req.Header.Add("x-li-page-instance", "urn:li:page:messaging_index;"+c.clientPageInstanceID)

		c.realtimeResp, err = c.http.Do(req)
		if err != nil {
			c.handlers.onRealtimeConnectError(c.realtimeCtx, err)
			return
		}
		if c.realtimeResp.StatusCode != http.StatusOK {
			c.handlers.onRealtimeConnectError(c.realtimeCtx, fmt.Errorf("failed to connect due to status code %d", c.realtimeResp.StatusCode))
			return
		}

		log.Info().Msg("Reading realtime stream")
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
				c.handlers.onRealtimeConnectError(c.realtimeCtx, err)
				continue
			}

			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}

			var realtimeEvent types.RealtimeEvent
			if err = json.Unmarshal(line[6:], &realtimeEvent); err != nil {
				c.handlers.onRealtimeConnectError(c.realtimeCtx, err)
				continue
			}

			switch {
			case realtimeEvent.Heartbeat != nil:
				c.handlers.onHeartbeat(c.realtimeCtx)
			case realtimeEvent.ClientConnection != nil:
				c.handlers.onClientConnection(c.realtimeCtx, realtimeEvent.ClientConnection)
			case realtimeEvent.DecoratedEvent != nil:
				log.Debug().
					Stringer("topic", realtimeEvent.DecoratedEvent.Topic).
					Str("payload_type", realtimeEvent.DecoratedEvent.Payload.Data.Type).
					Msg("Received decorated event")
				fmt.Printf("%s\n", line)
				fmt.Printf("decoratedEvent %+v\n", realtimeEvent.DecoratedEvent)
				c.handlers.onDecoratedEvent(c.realtimeCtx, realtimeEvent.DecoratedEvent)
			}
		}
	}
}

func (c *Client) RealtimeDisconnect() {
	if c.realtimeCancelFn != nil {
		c.realtimeCancelFn()
	}
}
