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

package matrixfmt

import (
	"context"

	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/connector/linkedinfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func toLinkedInAttribute(br linkedinfmt.BodyRange) linkedingo.SendMessageAttribute {
	switch val := br.Value.(type) {
	case linkedinfmt.Mention:
		return linkedingo.SendMessageAttribute{
			Start:  br.Start,
			Length: br.Length,
			AttributeKindUnion: types.AttributeKind{
				Entity: &types.Entity{
					URN: types.NewURN(val.UserID).WithPrefix("urn", "li", "fsd_profile"),
				},
			},
		}
	case linkedinfmt.Style:
		switch val.Type {
		default:
			panic("unsupported style type")
		}
	default:
		panic("unknown body range value")
	}
}

func Parse(ctx context.Context, parser *HTMLParser, content *event.MessageEventContent) (body linkedingo.SendMessageBody) {
	if content.MsgType.IsMedia() && (content.FileName == "" || content.FileName == content.Body) {
		// The body is the filename.
		return
	}

	if content.Format != event.FormatHTML {
		body.Text = content.Body
		return
	}
	parseCtx := NewContext(ctx)
	parseCtx.AllowedMentions = content.Mentions
	parsed := parser.Parse(content.FormattedBody, parseCtx)
	if parsed == nil {
		return
	}
	body.Text = parsed.String.String()
	body.Attributes = make([]linkedingo.SendMessageAttribute, len(parsed.Entities))
	for i, ent := range parsed.Entities {
		body.Attributes[i] = toLinkedInAttribute(ent)
	}
	return
}
