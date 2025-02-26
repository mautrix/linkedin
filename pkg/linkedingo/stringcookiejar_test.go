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

package linkedingo_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func TestCookieJarFromHeader(t *testing.T) {
	jar, err := linkedingo.NewJarFromCookieHeader("foo=bar;baz=123")
	require.NoError(t, err)

	cookies := jar.Cookies(nil)
	assert.Len(t, cookies, 2)
	values := make([]string, len(cookies))
	for i, c := range cookies {
		values[i] = c.Value
	}
	assert.ElementsMatch(t, []string{"bar", "123"}, values)

	assert.Equal(t, "bar", jar.GetCookie("foo"))
	assert.Equal(t, "123", jar.GetCookie("baz"))
}

func TestCookieJarSetCookies(t *testing.T) {
	jar := linkedingo.NewEmptyJar()

	assert.Len(t, jar.Cookies(nil), 0)

	jar.SetCookies(nil, []*http.Cookie{
		{Name: "a", Value: "123"},
		{Name: "b", Value: "456"},
		{Name: "c", Value: "789"},
	})

	cookies := jar.Cookies(nil)
	assert.Len(t, cookies, 3)
	cookieStrings := make([]string, len(cookies))
	for i, c := range cookies {
		cookieStrings[i] = c.String()
	}
	assert.Contains(t, cookieStrings, "a=123")
	assert.Contains(t, cookieStrings, "b=456")
	assert.Contains(t, cookieStrings, "c=789")

	assert.Equal(t, "123", jar.GetCookie("a"))
	assert.Equal(t, "456", jar.GetCookie("b"))
	assert.Equal(t, "789", jar.GetCookie("c"))

	jar.SetCookies(nil, []*http.Cookie{
		{Name: "a", Value: "123"},
		{Name: "c", Value: "789"},
	})

	cookies = jar.Cookies(nil)
	assert.Len(t, cookies, 2)
	cookieStrings = make([]string, len(cookies))
	for i, c := range cookies {
		cookieStrings[i] = c.String()
	}
	assert.Contains(t, cookieStrings, "a=123")
	assert.Contains(t, cookieStrings, "c=789")

	assert.Equal(t, "123", jar.GetCookie("a"))
	assert.Equal(t, "", jar.GetCookie("b"))
	assert.Equal(t, "789", jar.GetCookie("c"))
}

func TestMarshal(t *testing.T) {
	jar := linkedingo.NewEmptyJar()
	jar.SetCookies(nil, []*http.Cookie{
		{Name: "123", Value: "this is a test with spaces"},
		{Name: "234", Value: "I'm a value with a quote"},
		{Name: "this is a weird key", Value: "and value"},
	})

	res, err := json.Marshal(jar)
	require.NoError(t, err)
	assert.EqualValues(t, '"', res[0])
	assert.EqualValues(t, '"', res[len(res)-1])
	assert.Contains(t, string(res), `this is a weird key=\"and value\"`)
	assert.Contains(t, string(res), `123=\"this is a test with spaces\"`)
	assert.Contains(t, string(res), `234=\"I'm a value with a quote\"`)
}

type container struct {
	Cookies *linkedingo.Jar `json:"cookies,omitempty"`
}

func TestUnmarshal(t *testing.T) {
	serialized := []byte(`{"cookies":"foo=bar;baz=123"}`)
	var c container
	err := json.Unmarshal(serialized, &c)
	require.NoError(t, err)

	assert.Len(t, c.Cookies.Cookies(nil), 2)
	assert.Equal(t, "bar", c.Cookies.GetCookie("foo"))
	assert.Equal(t, "123", c.Cookies.GetCookie("baz"))
}
