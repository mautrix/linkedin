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

package types2

import "go.mau.fi/util/jsontime"

type Artifact struct {
	ExpiresAt                     jsontime.UnixMilli `json:"expiresAt,omitempty"`
	FileIdentifyingURLPathSegment string             `json:"fileIdentifyingUrlPathSegment,omitempty"`
	Height                        int                `json:"height,omitempty"`
	Width                         int                `json:"width,omitempty"`
}

type VectorImage struct {
	RootURL   string     `json:"rootUrl,omitempty"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

type Picture struct {
	VectorImage *VectorImage `json:"com.linkedin.common.VectorImage,omitempty"`
}

type MiniProfile struct {
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Occupation       string `json:"occupation"`
	PublicIdentifier string `json:"publicIdentifier"`
	Memorialized     bool   `json:"memorialized"`

	EntityURN     string `json:"entityUrn"`
	ObjectURN     string `json:"objectUrn"`
	DashEntityURN string `json:"dashEntityUrn"`

	TrackingID string `json:"trackingId"`

	Picture Picture `json:"picture,omitempty"`
}

type UserProfile struct {
	PlainID     int         `json:"plainId"`
	MiniProfile MiniProfile `json:"miniProfile"`
}
