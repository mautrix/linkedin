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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserIdentityFromUserAgent(t *testing.T) {
	const (
		chromeMacOS   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
		chromeLinux   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
		chromeWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
		chromeAndroid = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Mobile Safari/537.36"
		edgeWindows   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36 Edg/150.0.0.0"
		firefoxMacOS  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0"
		safariMacOS   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"
	)

	t.Run("empty falls back to the compile-time default", func(t *testing.T) {
		identity := browserIdentityFromUserAgent("")
		assert.Equal(t, defaultBrowserIdentity(), identity)
		assert.True(t, identity.sendClientHints)
	})

	t.Run("chromium derives matching hints", func(t *testing.T) {
		tests := []struct {
			name             string
			userAgent        string
			expectedUA       string
			expectedPlatform string
			expectedMobile   string
		}{
			{"macos", chromeMacOS, `"Chromium";v="151", "Google Chrome";v="151", "Not-A.Brand";v="99"`, `"macOS"`, "?0"},
			{"linux", chromeLinux, `"Chromium";v="141", "Google Chrome";v="141", "Not-A.Brand";v="99"`, `"Linux"`, "?0"},
			{"windows", chromeWindows, `"Chromium";v="150", "Google Chrome";v="150", "Not-A.Brand";v="99"`, `"Windows"`, "?0"},
			{"android is mobile", chromeAndroid, `"Chromium";v="150", "Google Chrome";v="150", "Not-A.Brand";v="99"`, `"Android"`, "?1"},
			{"edge is chromium", edgeWindows, `"Chromium";v="150", "Google Chrome";v="150", "Not-A.Brand";v="99"`, `"Windows"`, "?0"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				identity := browserIdentityFromUserAgent(test.userAgent)
				assert.Equal(t, test.userAgent, identity.userAgent)
				assert.True(t, identity.sendClientHints)
				assert.Equal(t, test.expectedUA, identity.secCHUA)
				assert.Equal(t, test.expectedPlatform, identity.secCHUAPlatform)
				assert.Equal(t, test.expectedMobile, identity.secCHUAMobile)
			})
		}
	})

	// Firefox and Safari send no sec-ch-* headers at all, so the user agent is
	// kept and the hints are suppressed rather than filled in with Chromium
	// values, which would contradict the user agent.
	t.Run("non-chromium keeps the user agent and omits client hints", func(t *testing.T) {
		for _, userAgent := range []string{firefoxMacOS, safariMacOS} {
			identity := browserIdentityFromUserAgent(userAgent)
			assert.Equal(t, userAgent, identity.userAgent)
			assert.False(t, identity.sendClientHints)
			assert.Empty(t, identity.secCHUA)
			assert.Empty(t, identity.secCHUAPlatform)
			assert.Empty(t, identity.secCHUAMobile)
		}
	})
}
