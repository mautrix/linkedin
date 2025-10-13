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
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"go.mau.fi/util/exerrors"
	"golang.org/x/net/publicsuffix"
)

// StringCookieJar is an [http.CookieJar] implementation that is backed by a
// dictionary of name -> [http.Cookie] values. It also implements
// [json.Marshaler] and [json.Unmarshaler] which allow it to be saved as a
// string.
//
// The zero value is not a valid [StringCookieJar]. Use [NewEmptyStringCookieJar] to create
// a new [StringCookieJar].
type StringCookieJar struct {
	http.CookieJar
}

var CookieBaseURL = exerrors.Must(url.Parse("https://www.linkedin.com"))

var (
	_ http.CookieJar   = (*StringCookieJar)(nil)
	_ json.Marshaler   = (*StringCookieJar)(nil)
	_ json.Unmarshaler = (*StringCookieJar)(nil)
)

// NewEmptyStringCookieJar creates an empty [StringCookieJar].
func NewEmptyStringCookieJar() *StringCookieJar {
	return &StringCookieJar{
		CookieJar: exerrors.Must(cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})),
	}
}

// NewJarFromCookieHeader creates a [StringCookieJar] from a cookie header string. It
// errors if parsing the cookie header fails.
func NewJarFromCookieHeader(cookieHeader string) (*StringCookieJar, error) {
	jar := NewEmptyStringCookieJar()
	return jar, jar.parseCookieHeaderString(cookieHeader)
}

func (s *StringCookieJar) UnmarshalJSON(data []byte) (err error) {
	*s = *NewEmptyStringCookieJar()
	if data[0] == '"' {
		var cookieHeader string
		err = json.Unmarshal(data, &cookieHeader)
		if err != nil {
			return
		}
		return s.parseCookieHeaderString(cookieHeader)
	}
	var cookies []*http.Cookie
	err = json.Unmarshal(data, &cookies)
	if err != nil {
		return
	}
	s.SetCookies(CookieBaseURL, cookies)
	return
}

func (s *StringCookieJar) GetCookie(name string) string {
	for _, cookie := range s.Cookies(CookieBaseURL) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func (s *StringCookieJar) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Cookies(CookieBaseURL))
}

func (s *StringCookieJar) Clear() {
	*s = *NewEmptyStringCookieJar()
}

func (s *StringCookieJar) parseCookieHeaderString(cookieString string) error {
	cookies, err := http.ParseCookie(cookieString)
	if err != nil {
		return err
	}
	s.SetCookies(CookieBaseURL, cookies)
	return nil
}
