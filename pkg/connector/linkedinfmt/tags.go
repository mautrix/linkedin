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

package linkedinfmt

import (
	"fmt"

	"maunium.net/go/mautrix/bridgev2/networkid"
)

type BodyRangeValue interface {
	String() string
	Format(message string) string
}

type Mention struct {
	UserInfo
	UserID networkid.UserID
}

var _ BodyRangeValue = Mention{}

func (m Mention) String() string {
	return fmt.Sprintf("Mention{MXID: id.UserID(%q), Name: %q}", m.MXID, m.Name)
}

//go:generate stringer -type=StyleType
type StyleType int

const (
	StyleNone StyleType = iota
	StyleBold
	StyleItalic
	StyleLineBreak
	StyleList
	StyleListItem
	StyleParagraph
	StyleSubscript
	StyleSuperscript
	StyleHyperlink
	StyleUnderline
)

// Style represents a style to apply to a range of text.
type Style struct {
	// Type is the type of style.
	Type StyleType

	// Ordered indicates whether the list is ordered.
	Ordered bool

	// URL is the URL to link to, if applicable.
	URL string
}

var _ BodyRangeValue = Style{}

func (s Style) String() string {
	return fmt.Sprintf("Style{Type: %s, URL: %s}", s.Type, s.URL)
}
