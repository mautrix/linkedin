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
	"math"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/net/html"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/connector/linkedinfmt"
)

type EntityString struct {
	String   []rune
	Entities linkedinfmt.BodyRangeList
}

var DebugLog = func(format string, args ...any) {}

func NewEntityString(val string) *EntityString {
	DebugLog("NEW %q\n", val)
	return &EntityString{
		String: []rune(val),
	}
}

func (es *EntityString) Split(at rune) []*EntityString {
	if at > 0x7F {
		panic("cannot split at non-ASCII character")
	}
	if es == nil {
		return []*EntityString{}
	}
	DebugLog("SPLIT %q %q %+v\n", es.String, rune(at), es.Entities)
	var output []*EntityString
	prevSplit := 0
	doSplit := func(i int) *EntityString {
		newES := &EntityString{
			String: es.String[prevSplit:i],
		}
		for _, entity := range es.Entities {
			if (entity.End() <= i || entity.End() > prevSplit) && (entity.Start >= prevSplit || entity.Start < i) {
				entity = *entity.TruncateStart(prevSplit).TruncateEnd(i).Offset(-prevSplit)
				if entity.Length > 0 {
					newES.Entities = append(newES.Entities, entity)
				}
			}
		}
		return newES
	}
	for i, chr := range es.String {
		if chr != at {
			continue
		}
		newES := doSplit(i)
		output = append(output, newES)
		DebugLog("  -> %q %+v\n", newES.String, newES.Entities)
		prevSplit = i + 1
	}
	if prevSplit == 0 {
		DebugLog("  -> NOOP\n")
		return []*EntityString{es}
	}
	if prevSplit != len(es.String) {
		newES := doSplit(len(es.String))
		output = append(output, newES)
		DebugLog("  -> %q %+v\n", newES.String, newES.Entities)
	}
	DebugLog("SPLITEND\n")
	return output
}

func (es *EntityString) TrimSpace() *EntityString {
	if es == nil {
		return nil
	}
	DebugLog("TRIMSPACE %q %+v\n", es.String, es.Entities)
	var cutEnd, cutStart int
	for cutStart = 0; cutStart < len(es.String); cutStart++ {
		switch es.String[cutStart] {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			continue
		}
		break
	}
	for cutEnd = len(es.String) - 1; cutEnd >= 0; cutEnd-- {
		switch es.String[cutEnd] {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			continue
		}
		break
	}
	cutEnd++
	if cutStart == 0 && cutEnd == len(es.String) {
		DebugLog("  -> NOOP\n")
		return es
	}
	newEntities := es.Entities[:0]
	for _, ent := range es.Entities {
		ent = *ent.Offset(-cutStart).TruncateEnd(cutEnd)
		if ent.Length > 0 {
			newEntities = append(newEntities, ent)
		}
	}
	es.String = es.String[cutStart:cutEnd]
	es.Entities = newEntities
	DebugLog("  -> %q %+v\n", es.String, es.Entities)
	return es
}

func JoinEntityString(with string, strings ...*EntityString) *EntityString {
	withUTF16 := []rune(with)
	totalLen := 0
	totalEntities := 0
	for _, s := range strings {
		totalLen += len(s.String)
		totalEntities += len(s.Entities)
	}
	str := make([]rune, 0, totalLen+len(strings)*len(withUTF16))
	entities := make(linkedinfmt.BodyRangeList, 0, totalEntities)
	DebugLog("JOIN %q %d\n", with, len(strings))
	for _, s := range strings {
		if s == nil || len(s.String) == 0 {
			continue
		}
		DebugLog("  + %q %+v\n", s.String, s.Entities)
		for _, entity := range s.Entities {
			entity.Start += len(str)
			entities = append(entities, entity)
		}
		str = append(str, s.String...)
		str = append(str, withUTF16...)
	}
	DebugLog("  -> %q %+v\n", str, entities)
	return &EntityString{
		String:   str,
		Entities: entities,
	}
}

func (es *EntityString) Format(value linkedinfmt.BodyRangeValue) *EntityString {
	if es == nil {
		return nil
	}
	newEntity := linkedinfmt.BodyRange{
		Start:  0,
		Length: len(es.String),
		Value:  value,
	}
	es.Entities = append(linkedinfmt.BodyRangeList{newEntity}, es.Entities...)
	DebugLog("FORMAT %v %q %+v\n", value, es.String, es.Entities)
	return es
}

func (es *EntityString) Append(other *EntityString) *EntityString {
	if es == nil {
		return other
	} else if other == nil {
		return es
	}
	DebugLog("APPEND %q %+v\n  + %q %+v\n", es.String, es.Entities, other.String, other.Entities)
	for _, entity := range other.Entities {
		entity.Start += len(es.String)
		es.Entities = append(es.Entities, entity)
	}
	es.String = append(es.String, other.String...)
	DebugLog("  -> %q %+v\n", es.String, es.Entities)
	return es
}

func (es *EntityString) AppendString(other string) *EntityString {
	if es == nil {
		return NewEntityString(other)
	} else if len(other) == 0 {
		return es
	}
	DebugLog("APPENDSTRING %q %+v\n  + %q\n", es.String, es.Entities, other)
	es.String = append(es.String, []rune(other)...)
	DebugLog("  -> %q %+v\n", es.String, es.Entities)
	return es
}

type TagStack []string

func (ts TagStack) Index(tag string) int {
	for i := len(ts) - 1; i >= 0; i-- {
		if ts[i] == tag {
			return i
		}
	}
	return -1
}

func (ts TagStack) Has(tag string) bool {
	return ts.Index(tag) >= 0
}

type Context struct {
	Ctx                context.Context
	AllowedMentions    *event.Mentions
	TagStack           TagStack
	PreserveWhitespace bool
	ListDepth          int
}

func NewContext(ctx context.Context) Context {
	return Context{
		Ctx:      ctx,
		TagStack: make(TagStack, 0, 4),
	}
}

func (ctx Context) WithTag(tag string) Context {
	ctx.TagStack = append(ctx.TagStack, tag)
	return ctx
}

func (ctx Context) WithWhitespace() Context {
	ctx.PreserveWhitespace = true
	return ctx
}

func (ctx Context) WithIncrementedListDepth() Context {
	ctx.ListDepth++
	return ctx
}

// HTMLParser is a somewhat customizable Matrix HTML parser.
type HTMLParser struct {
	GetGhostDetails func(context.Context, id.UserID) (networkid.UserID, string, bool)
}

// TaggedString is a string that also contains a HTML tag.
type TaggedString struct {
	*EntityString
	tag string
}

func (parser *HTMLParser) maybeGetAttribute(node *html.Node, attribute string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == attribute {
			return attr.Val, true
		}
	}
	return "", false
}

func (parser *HTMLParser) getAttribute(node *html.Node, attribute string) string {
	val, _ := parser.maybeGetAttribute(node, attribute)
	return val
}

// Digits counts the number of digits (and the sign, if negative) in an integer.
func Digits(num int) int {
	if num == 0 {
		return 1
	} else if num < 0 {
		return Digits(-num) + 1
	}
	return int(math.Floor(math.Log10(float64(num))) + 1)
}

func (parser *HTMLParser) basicFormatToString(node *html.Node, ctx Context) *EntityString {
	str := parser.nodeToTagAwareString(node.FirstChild, ctx)
	switch node.Data {
	case "b", "strong":
		return NewEntityString("**").Append(str).AppendString("**")
	case "i", "em":
		return NewEntityString("_").Append(str).AppendString("_")
	case "s", "del", "strike":
		return NewEntityString("~~").Append(str).AppendString("~~")
	case "tt", "code":
		return NewEntityString("`").Append(str).AppendString("`")
	case "li":
		return NewEntityString("- ").Append(str).AppendString("\n")
	}
	return str
}

func (parser *HTMLParser) headerToString(node *html.Node, ctx Context) *EntityString {
	length := int(node.Data[1] - '0')
	prefix := strings.Repeat("#", length) + " "
	return NewEntityString(prefix).Append(parser.nodeToString(node.FirstChild, ctx)).Format(linkedinfmt.Style{Type: linkedinfmt.StyleBold})
}

func (parser *HTMLParser) linkToString(node *html.Node, ctx Context) *EntityString {
	str := parser.nodeToTagAwareString(node.FirstChild, ctx)
	href := parser.getAttribute(node, "href")
	if len(href) == 0 {
		return str
	}
	ent := NewEntityString(string(str.String))

	parsedMatrix, err := id.ParseMatrixURIOrMatrixToURL(href)
	if err == nil && parsedMatrix != nil && parsedMatrix.Sigil1 == '@' {
		mxid := parsedMatrix.UserID()
		if ctx.AllowedMentions != nil && !slices.Contains(ctx.AllowedMentions.UserIDs, mxid) {
			// Mention not allowed, use name as-is
			return str
		}
		// FIXME this or GetGhostDetails needs to support non-ghost users too
		userID, username, ok := parser.GetGhostDetails(ctx.Ctx, mxid)
		if !ok {
			return str
		} else {
			return NewEntityString("@" + username).Format(linkedinfmt.Mention{UserID: userID})
		}
	}
	return ent.Format(linkedinfmt.Style{Type: linkedinfmt.StyleHyperlink, URL: href})
}

func (parser *HTMLParser) tagToString(node *html.Node, ctx Context) *EntityString {
	ctx = ctx.WithTag(node.Data)
	switch node.Data {
	case "blockquote":
		return NewEntityString("> ").Append(parser.nodeToString(node.FirstChild, ctx))
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return parser.headerToString(node, ctx)
	case "br":
		return NewEntityString("\n")
	case "b", "strong", "i", "em", "s", "strike", "del", "u", "ins", "tt", "code", "ol", "ul", "li":
		return parser.basicFormatToString(node, ctx)
	case "a":
		return parser.linkToString(node, ctx)
	case "p":
		return parser.nodeToTagAwareString(node.FirstChild, ctx)
	case "hr":
		return NewEntityString("---")
	default:
		return parser.nodeToTagAwareString(node.FirstChild, ctx)
	}
}

func (parser *HTMLParser) singleNodeToString(node *html.Node, ctx Context) TaggedString {
	switch node.Type {
	case html.TextNode:
		if !ctx.PreserveWhitespace {
			node.Data = strings.ReplaceAll(node.Data, "\n", "")
		}
		return TaggedString{NewEntityString(node.Data), "text"}
	case html.ElementNode:
		return TaggedString{parser.tagToString(node, ctx), node.Data}
	case html.DocumentNode:
		return TaggedString{parser.nodeToTagAwareString(node.FirstChild, ctx), "html"}
	default:
		return TaggedString{&EntityString{}, "unknown"}
	}
}

func (parser *HTMLParser) nodeToTaggedStrings(node *html.Node, ctx Context) (strs []TaggedString) {
	for ; node != nil; node = node.NextSibling {
		strs = append(strs, parser.singleNodeToString(node, ctx))
	}
	return
}

var BlockTags = []string{"p", "h1", "h2", "h3", "h4", "h5", "h6", "ol", "ul", "pre", "blockquote", "div", "hr", "table"}

func (parser *HTMLParser) isBlockTag(tag string) bool {
	for _, blockTag := range BlockTags {
		if tag == blockTag {
			return true
		}
	}
	return false
}

func (parser *HTMLParser) nodeToTagAwareString(node *html.Node, ctx Context) *EntityString {
	strs := parser.nodeToTaggedStrings(node, ctx)
	var output *EntityString
	for _, str := range strs {
		tstr := str.EntityString
		if parser.isBlockTag(str.tag) {
			tstr = NewEntityString("\n").Append(tstr).AppendString("\n")
		}
		if output == nil {
			output = tstr
		} else {
			output = output.Append(tstr)
		}
	}
	return output.TrimSpace()
}

func (parser *HTMLParser) nodeToStrings(node *html.Node, ctx Context) (strs []*EntityString) {
	for ; node != nil; node = node.NextSibling {
		strs = append(strs, parser.singleNodeToString(node, ctx).EntityString)
	}
	return
}

func (parser *HTMLParser) nodeToString(node *html.Node, ctx Context) *EntityString {
	return JoinEntityString("", parser.nodeToStrings(node, ctx)...)
}

// Parse converts Matrix HTML into text using the settings in this parser.
func (parser *HTMLParser) Parse(htmlData string, ctx Context) *EntityString {
	node, _ := html.Parse(strings.NewReader(htmlData))
	return parser.nodeToTagAwareString(node, ctx)
}
