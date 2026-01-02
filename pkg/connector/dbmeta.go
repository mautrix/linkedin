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

package connector

import (
	"encoding/json"

	"maunium.net/go/mautrix/bridgev2/database"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (lc *LinkedInConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Reaction: nil,
		Portal:   nil,
		Message: func() any {
			return &MessageMetadata{}
		},
		Ghost: nil,
		UserLogin: func() any {
			return &UserLoginMetadata{}
		},
	}
}

type UserLoginMetadata struct {
	Cookies         *linkedingo.StringCookieJar `json:"cookies,omitempty"`
	XLITrack        string                      `json:"x_li_track,omitempty"`
	XLIPageInstance string                      `json:"x_li_page_instance,omitempty"`
}

type MessageMetadata struct {
	DirectMediaMeta json.RawMessage `json:"direct_media_meta,omitempty"`
}

type DirectMediaMeta struct {
	MimeType string `json:"mime_type"`
	URL      string `json:"url"`
}
