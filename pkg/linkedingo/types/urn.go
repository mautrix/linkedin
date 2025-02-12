package types

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type URN struct {
	parts   []string
	idParts []string
}

func NewURN[T ~string](s T) (u URN) {
	u.parts = strings.Split(string(s), ":")
	u.idParts = strings.Split(strings.Trim(u.parts[len(u.parts)-1], "()"), ",")
	return
}

var _ json.Marshaler = (*URN)(nil)
var _ json.Unmarshaler = (*URN)(nil)
var _ fmt.Stringer = (*URN)(nil)

func (u URN) ID() string {
	if len(u.idParts) != 1 {
		panic(fmt.Sprintf("wrong number of ID parts %d", len(u.idParts)))
	}
	return u.idParts[0]
}

func (u URN) String() string {
	return strings.Join(u.parts, ":")
}

func (u URN) URLEscaped() string {
	return url.PathEscape(u.String())
}

func (u URN) IsEmpty() bool {
	return len(u.parts) == 0
}

func (u *URN) UnmarshalJSON(data []byte) (err error) {
	var urn string
	if err = json.Unmarshal(data, &urn); err != nil {
		return err
	}
	newURN := NewURN(urn)
	u.parts = newURN.parts
	u.idParts = newURN.idParts
	return nil
}

func (u URN) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u URN) NthPart(n int) string {
	return u.parts[n]
}

// WithPrefix returns a URN with the given prefix but the same ID (last part)
func (u URN) WithPrefix(prefix ...string) (n URN) {
	n.parts = append(prefix, u.parts[len(u.parts)-1])
	n.idParts = u.idParts
	return
}
