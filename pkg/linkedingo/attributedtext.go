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

// AttributedText represents a com.linkedin.pemberly.text.AttributedText
// object.
type AttributedText struct {
	Attributes []Attribute `json:"attributes,omitempty"`
	Text       string      `json:"text,omitempty"`
}

// Attribute represents a com.linkedin.pemberly.text.Attribute object.
type Attribute struct {
	Start         int           `json:"start"`
	Length        int           `json:"length"`
	AttributeKind AttributeKind `json:"attributeKind"`
}

type AttributeKind struct {
	Bold        *Bold        `json:"bold,omitempty"`
	Entity      *Entity      `json:"entity,omitempty"` // Entity is a user mention.
	Hyperlink   *Hyperlink   `json:"hyperlink,omitempty"`
	Italic      *Italic      `json:"italic,omitempty"`
	LineBreak   *LineBreak   `json:"lineBreak,omitempty"`
	List        *List        `json:"list,omitempty"`
	ListItem    *ListItem    `json:"listItem,omitempty"`
	Paragraph   *Paragraph   `json:"paragraph,omitempty"`
	Subscript   *Subscript   `json:"subscript,omitempty"`
	Superscript *Superscript `json:"superscript,omitempty"`
	Underline   *Underline   `json:"underline,omitempty"`
}

// Bold represents a com.linkedin.pemberly.text.Bold object.
type Bold struct{}

// Entity represents a com.linkedin.pemberly.text.Entity object.
type Entity struct {
	URN URN `json:"urn,omitempty"`
}

// Hyperlink represents a com.linkedin.pemberly.text.Hyperlink object.
type Hyperlink struct {
	URL string `json:"url,omitempty"`
}

// Italic represents a com.linkedin.pemberly.text.Italic object.
type Italic struct{}

// LineBreak represents a com.linkedin.pemberly.text.LineBreak object.
type LineBreak struct{}

// List represents a com.linkedin.pemberly.text.List object.
type List struct {
	Ordered bool `json:"ordered,omitempty"`
}

// ListItem represents a com.linkedin.pemberly.text.ListItem object.
type ListItem struct{}

// Paragraph represents a com.linkedin.pemberly.text.Paragraph object.
type Paragraph struct{}

// Subscript represents a com.linkedin.pemberly.text.Subscript object.
type Subscript struct{}

// Superscript represents a com.linkedin.pemberly.text.Superscript object.
type Superscript struct{}

// Underline represents a com.linkedin.pemberly.text.Underline object.
type Underline struct{}
