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

package stringcookiejar

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
)

// Jar is an [http.CookieJar] implementation that is backed by a dictionary of
// name -> [http.Cookie] values. It also implements [json.Marshaler] and
// [json.Unmarshaler] which allow it to be saved as a string.
//
// The zero value is not a valid [Jar]. Use [NewEmptyJar] to create a new
// [Jar].
type Jar struct {
	cookies map[string]*http.Cookie
}

var _ http.CookieJar = (*Jar)(nil)
var _ json.Marshaler = (*Jar)(nil)
var _ json.Unmarshaler = (*Jar)(nil)

// NewEmptyJar creates an empty [Jar].
func NewEmptyJar() *Jar {
	return &Jar{
		cookies: map[string]*http.Cookie{},
	}
}

// NewJarFromCookieHeader creates a [Jar] from a cookie header string. It
// errors if parsing the cookie header fails.
func NewJarFromCookieHeader(cookieHeader string) (*Jar, error) {
	cookies, err := parseCookieHeaderString(cookieHeader)
	return &Jar{cookies: cookies}, err
}

func (s *Jar) Cookies(u *url.URL) []*http.Cookie {
	return slices.Collect(maps.Values(s.cookies))
}

func (s *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.cookies = map[string]*http.Cookie{}
	for _, c := range cookies {
		s.cookies[c.Name] = c
	}
}

func (s *Jar) GetCookie(name string) (value string) {
	if c, ok := s.cookies[name]; ok {
		value = c.Value
	}
	return
}

func (s *Jar) UnmarshalJSON(data []byte) (err error) {
	var cookieHeader string
	err = json.Unmarshal(data, &cookieHeader)
	if err != nil {
		return
	}
	s.cookies, err = parseCookieHeaderString(cookieHeader)
	return
}

func (s *Jar) MarshalJSON() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request")
	}
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	return json.Marshal(req.Header.Get("Cookie"))
}

func parseCookieHeaderString(cookieString string) (map[string]*http.Cookie, error) {
	cookies, err := http.ParseCookie(cookieString)
	if err != nil {
		return nil, err
	}
	cache := map[string]*http.Cookie{}
	for _, c := range cookies {
		cache[c.Name] = c
	}
	return cache, nil
}
