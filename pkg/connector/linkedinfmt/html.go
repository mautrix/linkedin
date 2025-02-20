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
	"strings"
	"unicode/utf16"
)

func (m Mention) Format(message string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, m.MXID.URI().MatrixToURL(), m.Name)
}

func (s Style) Format(message string) string {
	switch s.Type {
	case StyleBold:
		return fmt.Sprintf("<strong>%s</strong>", message)
	case StyleItalic:
		return fmt.Sprintf("<em>%s</em>", message)
	case StyleLineBreak:
		return "<br>"
	case StyleList:
		if s.Ordered {
			return fmt.Sprintf("<ol>%s</ol>", message)
		} else {
			return fmt.Sprintf("<ul>%s</ul>", message)
		}
	case StyleListItem:
		return fmt.Sprintf("<li>%s</li>", message)
	case StyleParagraph:
		return fmt.Sprintf("<p>%s</p>", message)
	case StyleSubscript:
		return fmt.Sprintf("<sub>%s</sub>", message)
	case StyleSuperscript:
		return fmt.Sprintf("<sup>%s</sup>", message)
	case StyleHyperlink:
		if strings.HasPrefix(s.URL, "https://matrix.to/#") {
			return s.URL
		}
		return fmt.Sprintf(`<a href='%s'>%s</a>`, s.URL, message)
	case StyleUnderline:
		return fmt.Sprintf("<u>%s</u>", message)
	default:
		return message
	}
}

type UTF16String []uint16

func NewUTF16String(s string) UTF16String {
	return utf16.Encode([]rune(s))
}

func (u UTF16String) String() string {
	return string(utf16.Decode(u))
}

func (lrt *LinkedRangeTree) Format(message UTF16String, ctx formatContext) string {
	if lrt == nil || lrt.Node == nil {
		return ctx.TextToHTML(message.String())
	}
	head := message[:lrt.Node.Start]
	headStr := ctx.TextToHTML(head.String())
	inner := message[lrt.Node.Start:lrt.Node.End()]
	tail := message[lrt.Node.End():]
	ourCtx := ctx
	childMessage := lrt.Child.Format(inner, ourCtx)
	formattedChildMessage := lrt.Node.Value.Format(childMessage)
	siblingMessage := lrt.Sibling.Format(tail, ctx)
	return headStr + formattedChildMessage + siblingMessage
}
