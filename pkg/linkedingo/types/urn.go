package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

type URN struct {
	parts   []string
	idParts []string
}

func NewURN(urnStr string) (u URN) {
	u.parts = strings.Split(urnStr, ":")
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

func (u *URN) NthPart(n int) string {
	return u.parts[n]
}
