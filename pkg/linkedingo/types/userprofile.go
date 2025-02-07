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

package types

type MiniProfile struct {
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Occupation       string `json:"occupation"`
	PublicIdentifier string `json:"publicIdentifier"`
	Memorialized     bool   `json:"memorialized"`

	EntityURN     URN `json:"entityUrn"`
	ObjectURN     URN `json:"objectUrn"`
	DashEntityURN URN `json:"dashEntityUrn"`

	TrackingID string `json:"trackingId"`

	Picture Picture `json:"picture,omitempty"`
}

type UserProfile struct {
	MiniProfile MiniProfile `json:"miniProfile"`
}
