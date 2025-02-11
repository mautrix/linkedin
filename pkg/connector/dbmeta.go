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
	"maunium.net/go/mautrix/bridgev2/database"

	"go.mau.fi/mautrix-linkedin/pkg/stringcookiejar"
)

func (lc *LinkedInConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Reaction:  nil,
		Portal:    nil,
		Message:   nil,
		Ghost:     nil,
		UserLogin: func() any { return &UserLoginMetadata{} },
	}
}

type UserLoginMetadata struct {
	Cookies *stringcookiejar.Jar `json:"cookies,omitempty"`
}
