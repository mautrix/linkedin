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

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type URNString string

func (u URNString) URN() URN {
	return NewURN(string(u))
}

type URN struct {
	prefix string
	id     string
}

var urnRegex = regexp.MustCompile(`^(.*?):(\(.*\)|[^:]*)$`)

func NewURN[T ~string](s T) (u URN) {
	match := urnRegex.FindStringSubmatch(string(s))
	if len(match) == 0 {
		return URN{id: string(s)}
	}
	u.prefix = match[1]
	u.id = match[2]
	return
}

var _ json.Marshaler = (*URN)(nil)
var _ json.Unmarshaler = (*URN)(nil)
var _ fmt.Stringer = (*URN)(nil)

func (u URN) ID() string {
	return u.id
}

func (u URN) URNString() URNString {
	return URNString(u.String())
}

func (u URN) String() string {
	return u.prefix + ":" + u.id
}

func (u URN) URLEscaped() string {
	return url.PathEscape(u.String())
}

func (u URN) IsEmpty() bool {
	return len(u.id) == 0
}

func (u *URN) UnmarshalJSON(data []byte) (err error) {
	var urn string
	if err = json.Unmarshal(data, &urn); err != nil {
		return err
	}
	newURN := NewURN(urn)
	u.prefix = newURN.prefix
	u.id = newURN.id
	return nil
}

func (u URN) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u URN) NthPrefixPart(n int) string {
	return strings.Split(u.prefix, ":")[n]
}

// WithPrefix returns a URN with the given prefix but the same ID (last part)
func (u URN) WithPrefix(prefix ...string) (n URN) {
	n.prefix = strings.Join(prefix, ":")
	n.id = u.id
	return
}
