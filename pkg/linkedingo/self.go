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

type MiniProfile struct {
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Occupation       string `json:"occupation"`
	PublicIdentifier string `json:"publicIdentifier"`
	Memorialized     bool   `json:"memorialized"`

	EntityURN     types.URN `json:"entityUrn"`
	DashEntityURN types.URN `json:"dashEntityUrn"`

	TrackingID string `json:"trackingId"`

	Picture types.Picture `json:"picture,omitempty"`
}

type UserProfile struct {
	MiniProfile MiniProfile `json:"miniProfile"`
}

func (c *Client) GetCurrentUserProfile(ctx context.Context) (*UserProfile, error) {
	resp, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerCommonMeURL, nil).WithCSRF().Do(ctx)
	if err != nil {
		return nil, err
	}

	var profile UserProfile
	return &profile, json.NewDecoder(resp.Body).Decode(&profile)
}

func (c *Client) Logout(ctx context.Context) error {
	params := url.Values{}
	params.Add("csrfToken", c.getCSRFToken())
	url, err := url.Parse(linkedInLogoutURL)
	if err != nil {
		return err
	}
	url.RawQuery = params.Encode()

	_, err = c.newAuthedRequest(http.MethodGet, url.String(), nil).Do(ctx)
	return err
}
