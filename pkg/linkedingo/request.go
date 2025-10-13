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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/exerrors"
)

func (c *Client) getCSRFToken() string {
	return c.jar.GetCookie(LinkedInCookieJSESSIONID)
}

type authedRequest struct {
	parseErr error

	method      string
	url         *url.URL
	header      http.Header
	queryParams url.Values
	rawQuery    string
	body        io.Reader

	client *Client
}

var (
	ErrTokenInvalidated = errors.New("access token is no longer valid")
)

func (c *Client) checkHTTPRedirect(req *http.Request, via []*http.Request) error {
	if req.Response == nil {
		return nil
	}
	respCookies := req.Response.Cookies()
	for _, cookie := range respCookies {
		if cookie.Name == "li_at" && (cookie.Expires.Unix() == 0 || cookie.Value == "delete me") {
			return fmt.Errorf("%w: %s cookie was deleted", ErrTokenInvalidated, cookie.Name)
		}
	}
	// Don't allow redirects
	return http.ErrUseLastResponse
}

func (c *Client) newAuthedRequest(method, urlStr string) *authedRequest {
	ar := authedRequest{header: http.Header{}, method: method, client: c}
	ar.url, ar.parseErr = url.Parse(urlStr)

	if ar.parseErr == nil {
		ar.queryParams = ar.url.Query()
	} else {
		ar.queryParams = url.Values{}
	}

	// Add default headers for every request
	ar.header.Add("User-Agent", UserAgent)
	ar.header.Add("Accept-Language", "en-US,en;q=0.9")
	ar.header.Add("sec-ch-prefers-color-scheme", SecCHPrefersColorScheme)
	ar.header.Add("sec-ch-ua", SecCHUserAgent)
	ar.header.Add("sec-ch-ua-mobile", SecCHMobile)
	ar.header.Add("sec-ch-ua-platform", SecCHPlatform)

	return &ar
}

func (a *authedRequest) WithHeader(key, value string) *authedRequest {
	a.header.Set(key, value)
	return a
}

// WithQueryParam adds a query parameter to the request. If a raw query is set
// with [authedRequest.WithRawQuery], this will be ignored.
func (a *authedRequest) WithQueryParam(key, value string) *authedRequest {
	a.queryParams.Add(key, value)
	return a
}

func (a *authedRequest) WithRawQuery(raw string) *authedRequest {
	a.rawQuery = raw
	return a
}

func (a *authedRequest) WithGraphQLQuery(queryID string, variables map[string]string) *authedRequest {
	a.WithHeader("accept", contentTypeGraphQL)
	a.WithCSRF()
	a.WithXLIHeaders()

	var queryStr strings.Builder
	queryStr.WriteString("queryId=")
	queryStr.WriteString(queryID)
	queryStr.WriteString("&variables=(")
	first := true
	for k, v := range variables {
		if v == "" {
			continue
		}
		if !first {
			queryStr.WriteString(",")
		}
		first = false
		queryStr.WriteString(k)
		queryStr.WriteByte(':')
		queryStr.WriteString(v)
	}
	queryStr.WriteString(")")
	a.rawQuery = queryStr.String()
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
		WithHeader("X-LI-Page-Instance", a.client.pageInstance).
		WithHeader("X-LI-Track", a.client.xLITrack).
		WithHeader("X-RestLI-Protocol-Version", "2.0.0")
}

func (a *authedRequest) WithRealtimeConnectHeaders() *authedRequest {
	return a.
		WithHeader("Priority", "u=1, i").
		WithHeader("Sec-Fetch-Dest", "empty").
		WithHeader("Sec-Fetch-Mode", "cors").
		WithHeader("Sec-Fetch-Site", "same-origin").
		WithHeader("X-LI-Accept", contentTypeJSONLinkedInNormalized).
		WithHeader("X-LI-Query-Accept", contentTypeGraphQL).
		WithHeader("X-LI-Query-Map", realtimeQueryMap).
		WithHeader("X-LI-Recipe-Accept", contentTypeJSONLinkedInNormalized).
		WithHeader("X-LI-Recipe-Map", realtimeRecipeMap).
		WithHeader("X-LI-Realtime-Session", a.client.realtimeSessionID.String()).
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

var reqIDCounter atomic.Int64

func (a *authedRequest) DoRaw(ctx context.Context) (*http.Response, error) {
	if a.parseErr != nil {
		return nil, a.parseErr
	}
	if a.rawQuery != "" {
		a.url.RawQuery = a.rawQuery
	} else {
		_, ok := a.queryParams["v"]
		if ok {
			//avoid rearrange URL parameter to alphabetical order
		} else {
			a.url.RawQuery = a.queryParams.Encode()
		}
	}

	reqID := reqIDCounter.Add(1)
	log := zerolog.Ctx(ctx).With().
		Int64("req_id", reqID).
		Str("method", a.method).
		Stringer("url", a.url).
		Logger()
	ctx = log.WithContext(ctx)

	retryIn := 2 * time.Second
	for retryCount := 0; ; retryCount++ {
		start := time.Now()
		resp, err := a.doRawRetry(ctx)
		dur := time.Since(start)
		if errors.Is(err, ErrTokenInvalidated) {
			logEvt := log.Error()
			if resp != nil {
				_ = resp.Body.Close()
				logEvt.Int("status", resp.StatusCode)
			}
			logEvt.Err(err).
				Dur("duration", dur).
				Msg("Failed to send request")
			return nil, err
		} else if err != nil {
			if retryCount >= 5 {
				log.Err(err).
					Dur("duration", dur).
					Msg("Failed to send request, not retrying anymore")
				return nil, err
			}
			log.Warn().Err(err).
				Dur("duration", dur).
				Dur("retry_in", retryIn).
				Msg("Failed to send request, retrying")
		} else if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
			if retryCount >= 5 {
				log.Error().
					Dur("duration", dur).
					Int("status", resp.StatusCode).
					Msg("HTTP 50x while sending request, not retrying anymore")
				return nil, err
			}
			log.Warn().
				Dur("duration", dur).
				Int("status", resp.StatusCode).
				Dur("retry_in", retryIn).
				Msg("HTTP 50x while sending request, retrying")
		} else {
			log.Debug().
				Int("status", resp.StatusCode).
				Dur("duration", dur).
				Msg("Request completed")
			return resp, nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryIn):
		}
		retryIn *= 2
	}
}

func (a *authedRequest) doRawRetry(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, a.method, a.url.String(), a.body)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header = a.header

	resp, err := a.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	return resp, nil
}

func (a *authedRequest) Do(ctx context.Context, out any) (*http.Response, error) {
	resp, err := a.DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return resp, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	if out == nil {
		return resp, nil
	}

	err = json.NewDecoder(resp.Body).Decode(out)
	if err != nil {
		return resp, fmt.Errorf("failed to decode response body: %w", err)
	}
	return resp, nil
}
