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
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sync"
)

// StringCookieJar is an [http.CookieJar] implementation that is backed by a
// dictionary of name -> [http.Cookie] values. It also implements
// [json.Marshaler] and [json.Unmarshaler] which allow it to be saved as a
// string.
//
// The zero value is not a valid [StringCookieJar]. Use [NewEmptyStringCookieJar] to create
// a new [StringCookieJar].
type StringCookieJar struct {
	cookies map[string]*http.Cookie
	lock    sync.RWMutex
}

var _ http.CookieJar = (*StringCookieJar)(nil)
var _ json.Marshaler = (*StringCookieJar)(nil)
var _ json.Unmarshaler = (*StringCookieJar)(nil)

// NewEmptyStringCookieJar creates an empty [StringCookieJar].
func NewEmptyStringCookieJar() *StringCookieJar {
	return &StringCookieJar{
		cookies: make(map[string]*http.Cookie),
	}
}

// NewJarFromCookieHeader creates a [StringCookieJar] from a cookie header string. It
// errors if parsing the cookie header fails.
func NewJarFromCookieHeader(cookieHeader string) (*StringCookieJar, error) {
	cookies, err := parseCookieHeaderString(cookieHeader)
	return &StringCookieJar{cookies: cookies}, err
}

func (s *StringCookieJar) Cookies(u *url.URL) []*http.Cookie {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return slices.Collect(maps.Values(s.cookies))
}

func (s *StringCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, c := range cookies {
		s.cookies[c.Name] = c
	}
}

func (s *StringCookieJar) Clear() {
	s.lock.Lock()
	defer s.lock.Unlock()
	clear(s.cookies)
}

func (s *StringCookieJar) AddCookie(cookie *http.Cookie) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cookies[cookie.Name] = cookie
}

func (s *StringCookieJar) GetCookie(name string) (value string) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if c, ok := s.cookies[name]; ok {
		value = c.Value
	}
	return
}

func (s *StringCookieJar) UnmarshalJSON(data []byte) (err error) {
	var cookieHeader string
	err = json.Unmarshal(data, &cookieHeader)
	if err != nil {
		return
	}
	s.cookies, err = parseCookieHeaderString(cookieHeader)
	return
}

func (s *StringCookieJar) MarshalJSON() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request")
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
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
