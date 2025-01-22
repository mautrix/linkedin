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

package linkedingo2

import (
	"context"
	"net/http"

	"go.mau.fi/mautrix-linkedin/pkg/stringcookiejar"
)

type Client struct {
	http      *http.Client
	csrfToken string
}

func NewClient(ctx context.Context, jar *stringcookiejar.Jar) *Client {
	return &Client{
		http: &http.Client{
			Jar: jar,
		},
		csrfToken: jar.GetCookie("JSESSIONID"),
	}
}
