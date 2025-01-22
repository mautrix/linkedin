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
	"encoding/json"
	"net/http"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo2/types2"
)

func (c *Client) GetCurrentUserProfile() (*types2.UserProfile, error) {
	req, err := http.NewRequest(http.MethodGet, LinkedInVoyagerCommonMeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("csrf-token", c.csrfToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	var profile types2.UserProfile
	return &profile, json.NewDecoder(resp.Body).Decode(&profile)
}
