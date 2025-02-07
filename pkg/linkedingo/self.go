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
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *Client) getCSRFToken() string {
	return c.jar.GetCookie(LinkedInJSESSIONID)
}

func (c *Client) GetCurrentUserProfile(ctx context.Context) (*types.UserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LinkedInVoyagerCommonMeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("csrf-token", c.getCSRFToken())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var profile types.UserProfile
	return &profile, json.NewDecoder(resp.Body).Decode(&profile)
}

func (c *Client) Logout(ctx context.Context) error {
	params := url.Values{}
	params.Add("csrfToken", c.getCSRFToken())
	url, err := url.Parse(LinkedInLogoutURL)
	if err != nil {
		return err
	}
	url.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return err
	}
	_, err = c.http.Do(req)
	return err
}
