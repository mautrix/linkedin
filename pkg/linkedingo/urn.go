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
