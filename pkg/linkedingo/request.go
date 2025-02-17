package linkedingo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.mau.fi/util/exerrors"
)

func (c *Client) getCSRFToken() string {
	return c.jar.GetCookie(LinkedInCookieJSESSIONID)
}

type authedRequest struct {
	parseErr error

	method string
	url    *url.URL
	header http.Header
	params url.Values
	body   io.Reader

	client *Client
}

func (c *Client) newAuthedRequest(method, urlStr string) *authedRequest {
	ar := authedRequest{header: http.Header{}, method: method, client: c}
	ar.url, ar.parseErr = url.Parse(urlStr)

	if ar.parseErr != nil {
		ar.params = ar.url.Query()
	} else {
		ar.params = url.Values{}
	}

	// Add default headers for every request
	ar.header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	ar.header.Add("Accept-Language", "en-US,en;q=0.9")
	ar.header.Add("sec-ch-prefers-color-scheme", "light")
	ar.header.Add("sec-ch-ua", `"Chromium";v="131", "Not_A Brand";v="24"`)
	ar.header.Add("sec-ch-ua-mobile", "?0")
	ar.header.Add("sec-ch-ua-platform", `"Linux"`)

	return &ar
}

func (a *authedRequest) WithHeader(key, value string) *authedRequest {
	a.header.Set(key, value)
	return a
}

func (a *authedRequest) WithParam(key, value string) *authedRequest {
	a.params.Add(key, value)
	return a
}

func (a *authedRequest) WithCSRF() *authedRequest {
	return a.WithHeader("csrf-token", a.client.getCSRFToken())
}

func (a *authedRequest) WithJSONPayload(payload any) *authedRequest {
	a.body = bytes.NewReader(exerrors.Must(json.Marshal(payload)))
	return a
}

func (a *authedRequest) WithBody(r io.Reader) *authedRequest {
	a.body = r
	return a
}

func (a *authedRequest) WithContentType(contentType string) *authedRequest {
	return a.WithHeader("content-type", contentType)
}

func (a *authedRequest) WithXLIHeaders() *authedRequest {
	return a.
		WithHeader("Referer", linkedInMessagingBaseURL+"/").
		WithHeader("X-LI-Accept", contentTypeJSONLinkedInNormalized).
		WithHeader("X-LI-Page-Instance", "urn:li:page:messaging_index;"+a.client.clientPageInstanceID).
		WithHeader("X-LI-Query-Accept", contentTypeGraphQL).
		WithHeader("X-LI-Query-Map", realtimeQueryMap).
		WithHeader("X-LI-Realtime-Session", a.client.realtimeSessionID.String()).
		WithHeader("X-LI-Recipe-Accept", contentTypeJSONLinkedInNormalized).
		WithHeader("X-LI-Recipe-Map", realtimeRecipeMap).
		WithHeader("X-LI-Track", a.client.xLITrack).
		WithHeader("X-RestLI-Protocol-Version", "2.0.0")
}

func (a *authedRequest) WithRealtimeConnectHeaders() *authedRequest {
	return a.
		WithHeader("Priority", "u=1, i").
		WithHeader("Sec-Fetch-Dest", "empty").
		WithHeader("Sec-Fetch-Mode", "cors").
		WithHeader("Sec-Fetch-Site", "same-origin").
		WithXLIHeaders()
}

func (a *authedRequest) WithWebpageHeaders() *authedRequest {
	return a.
		WithHeader("Priority", "u=0, i").
		WithHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7").
		WithHeader("Cache-Control", "max-age=0").
		WithHeader("Sec-Fetch-Dest", "document").
		WithHeader("Sec-Fetch-Mode", "navigate").
		WithHeader("Sec-Fetch-Site", "none").
		WithHeader("Sec-Fetch-User", "?1").
		WithHeader("Upgrade-Insecure-Requests", "1")
}

func (a *authedRequest) Do(ctx context.Context) (*http.Response, error) {
	if a.parseErr != nil {
		return nil, a.parseErr
	}
	a.url.RawQuery = a.params.Encode()

	req, err := http.NewRequestWithContext(ctx, a.method, a.url.String(), a.body)
	if err != nil {
		return nil, fmt.Errorf("failed to perform authed request %s %s: %w", a.method, a.url, err)
	}
	req.Header = a.header
	return a.client.http.Do(req)
}
