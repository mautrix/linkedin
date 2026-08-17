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

func TestBrowserHintsFromUserAgent(t *testing.T) {
	tests := []struct {
		name             string
		userAgent        string
		expectedOK       bool
		expectedUA       string
		expectedPlatform string
	}{
		{
			name:             "chrome on macos",
			userAgent:        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36",
			expectedOK:       true,
			expectedUA:       `"Chromium";v="151", "Google Chrome";v="151", "Not-A.Brand";v="99"`,
			expectedPlatform: `"macOS"`,
		},
		{
			name:             "chrome on linux",
			userAgent:        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
			expectedOK:       true,
			expectedUA:       `"Chromium";v="141", "Google Chrome";v="141", "Not-A.Brand";v="99"`,
			expectedPlatform: `"Linux"`,
		},
		{
			name:             "chrome on windows",
			userAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
			expectedOK:       true,
			expectedUA:       `"Chromium";v="150", "Google Chrome";v="150", "Not-A.Brand";v="99"`,
			expectedPlatform: `"Windows"`,
		},
		{
			name:       "non-chromium is rejected rather than half-populated",
			userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0",
			expectedOK: false,
		},
		{
			name:       "empty",
			userAgent:  "",
			expectedOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			secCHUA, platform, ok := browserHintsFromUserAgent(test.userAgent)
			assert.Equal(t, test.expectedOK, ok)
			if !test.expectedOK {
				return
			}
			assert.Equal(t, test.expectedUA, secCHUA)
			assert.Equal(t, test.expectedPlatform, platform)
		})
	}
}
