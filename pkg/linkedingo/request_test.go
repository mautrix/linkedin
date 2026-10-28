// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2026 Nick Mills-Barrett
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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnauthorizedRejectsLoginOnlyForSessionRequests(t *testing.T) {
	ctx := context.Background()
	jar := NewEmptyStringCookieJar()
	jar.SetCookies(CookieBaseURL, []*http.Cookie{{Name: LinkedInCookieJSESSIONID, Value: "ajax:1"}})
	var rejections []error
	client := NewClient(ctx, NewURN("me"), jar, "", "", "", Handlers{
		BadCredentials: func(_ context.Context, err error) { rejections = append(rejections, err) },
	})
	client.http.Transport = loginRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})

	_, err := client.DownloadHTTP(ctx, "https://www.linkedin.com/dms/prv/image/expired")
	require.Error(t, err)
	assert.Empty(t, rejections)
	assert.True(t, client.IsLoggedIn())

	for range 2 {
		_, err = client.GetCurrentUserProfile(ctx)
		require.Error(t, err)
	}
	require.Len(t, rejections, 1)
	var respErr *ResponseError
	require.ErrorAs(t, rejections[0], &respErr)
	assert.Equal(t, http.StatusUnauthorized, respErr.StatusCode)
	assert.False(t, client.IsLoggedIn())
}
