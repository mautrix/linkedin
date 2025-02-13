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
	ar.params = ar.url.Query()
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

func (a *authedRequest) WithRealtimeHeaders() *authedRequest {
	return a.
		WithHeader("referer", linkedInMessagingBaseURL+"/").
		WithHeader("x-li-accept", contentTypeJSONLinkedInNormalized).
		WithHeader("x-li-page-instance", "urn:li:page:messaging_index;"+a.client.clientPageInstanceID).
		WithHeader("x-li-query-accept", contentTypeGraphQL).
		WithHeader("x-li-query-map", realtimeQueryMap).
		WithHeader("x-li-realtime-session", a.client.realtimeSessionID.String()).
		WithHeader("x-li-recipe-accept", contentTypeJSONLinkedInNormalized).
		WithHeader("x-li-recipe-map", realtimeRecipeMap).
		WithHeader("x-li-track", a.client.xLITrack).
		WithHeader("x-restli-protocol-version", "2.0.0")
}

func (a *authedRequest) WithWebpageHeaders() *authedRequest {
	return a.
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
