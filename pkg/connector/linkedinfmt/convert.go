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
	"context"
	"html"

	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

type UserInfo struct {
	MXID id.UserID
	Name string
}

type FormatParams struct {
	GetMXIDByURN func(ctx context.Context, entityURN linkedingo.URN) (id.UserID, error)
}

type formatContext struct {
	IsInCodeblock bool
}

func (ctx formatContext) TextToHTML(text string) string {
	if ctx.IsInCodeblock {
		return html.EscapeString(text)
	}
	return event.TextToHTML(text)
}

func Parse(ctx context.Context, message string, attributes []linkedingo.Attribute, params FormatParams) (content *event.MessageEventContent, err error) {
	log := zerolog.Ctx(ctx).With().Str("func", "Parse").Logger()
	content = &event.MessageEventContent{
		MsgType:  event.MsgText,
		Body:     message,
		Mentions: &event.Mentions{},
	}
	if len(attributes) == 0 {
		return content, nil
	}

	lrt := &LinkedRangeTree{}
	mentions := map[id.UserID]struct{}{}
	utf16Message := NewUTF16String(message)
	maxLength := len(utf16Message)
	for _, a := range attributes {
		br := BodyRange{
			Start:  a.Start,
			Length: a.Length,
		}.TruncateEnd(maxLength)
		switch {
		case a.AttributeKind.Entity != nil:
			urn := a.AttributeKind.Entity.URN
			var userInfo UserInfo
			userInfo.MXID, err = params.GetMXIDByURN(ctx, urn)
			if err != nil {
				log.Warn().Err(err).
					Stringer("mentioned_user", urn).
					Msg("Failed to get user info for mention")
				continue // Skip this mention
			}
			userInfo.Name = utf16Message[a.Start+1 : a.Start+a.Length].String()
			mentions[userInfo.MXID] = struct{}{}
			br.Value = Mention{userInfo, networkid.UserID(urn.ID())}
		default:
			log.Warn().Msg("Unhandled attribute")
		}
		lrt.Add(br)
	}

	content.Mentions.UserIDs = maps.Keys(mentions)
	content.FormattedBody = lrt.Format(utf16Message, formatContext{})
	content.Format = event.FormatHTML
	return content, nil
}
