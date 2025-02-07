package linkedingoold

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/proxy"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type EventHandler func(evt any)
type ClientOpts struct {
	EventHandler EventHandler
}
type Client struct {
	Logger       zerolog.Logger
	PageLoader   *PageLoader
	rc           *RealtimeClient
	http         *http.Client
	httpProxy    func(*http.Request) (*url.URL, error)
	socksProxy   proxy.Dialer
	eventHandler EventHandler
}

func NewClient(opts *ClientOpts, logger zerolog.Logger) *Client {
	cli := Client{
		http: &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 40 * time.Second,
				ForceAttemptHTTP2:     true,
			},
			Timeout: 60 * time.Second,
		},
		Logger: logger,
	}

	if opts.EventHandler != nil {
		cli.SetEventHandler(opts.EventHandler)
	}

	// if opts.Cookies != nil {
	// 	cli.cookies = opts.Cookies
	// } else {
	// 	cli.cookies = cookies.NewCookies()
	// }

	cli.rc = cli.newRealtimeClient()
	cli.PageLoader = cli.newPageLoader()

	return &cli
}

func (c *Client) Connect() error {
	// return c.rc.Connect()
	return nil
}

func (c *Client) Disconnect() error {
	// return c.rc.Disconnect()
	return nil
}

func (c *Client) GetCookieString() string {
	// return c.cookies.String()
	return ""
}

func (c *Client) LoadMessagesPage() error {
	return c.PageLoader.LoadMessagesPage()
}

func (c *Client) GetCurrentUserID() string {
	return c.PageLoader.CurrentUser.FsdProfileID
}

func (c *Client) GetCurrentUserProfile() (*typesold.UserLoginProfile, error) {
	headers := c.buildHeaders(typesold.HeaderOpts{
		WithCookies:         true,
		WithCsrfToken:       true,
		WithXLiTrack:        true,
		WithXLiPageInstance: true,
		WithXLiProtocolVer:  true,
		WithXLiLang:         true,
	})

	_, data, err := c.MakeRequest(string(routingold.LinkedInVoyagerCommonMeURL), http.MethodGet, headers, make([]byte, 0), typesold.ContentTypeJSONLinkedInNormalized)
	if err != nil {
		return nil, err
	}

	response := &typesold.UserLoginProfile{}

	err = json.Unmarshal(data, response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) SetProxy(proxyAddr string) error {
	proxyParsed, err := url.Parse(proxyAddr)
	if err != nil {
		return err
	}

	if proxyParsed.Scheme == "http" || proxyParsed.Scheme == "https" {
		c.httpProxy = http.ProxyURL(proxyParsed)
		c.http.Transport.(*http.Transport).Proxy = c.httpProxy
	} else if proxyParsed.Scheme == "socks5" {
		c.socksProxy, err = proxy.FromURL(proxyParsed, &net.Dialer{Timeout: 20 * time.Second})
		if err != nil {
			return err
		}
		c.http.Transport.(*http.Transport).DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return c.socksProxy.Dial(network, addr)
		}
		contextDialer, ok := c.socksProxy.(proxy.ContextDialer)
		if ok {
			c.http.Transport.(*http.Transport).DialContext = contextDialer.DialContext
		}
	}

	c.Logger.Debug().
		Str("scheme", proxyParsed.Scheme).
		Str("host", proxyParsed.Host).
		Msg("Using proxy")
	return nil
}

func (c *Client) SetEventHandler(handler EventHandler) {
	c.eventHandler = handler
}
