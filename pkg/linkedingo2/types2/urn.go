package types2

import (
	"encoding/json"
	"fmt"
	"strings"
)

type URN struct {
	prefix  string
	idParts []string
}

var _ json.Marshaler = (*URN)(nil)
var _ json.Unmarshaler = (*URN)(nil)

func (u URN) ID() string {
	if len(u.idParts) != 1 {
		panic(fmt.Sprintf("wrong number of ID parts %d", len(u.idParts)))
	}
	return u.idParts[0]
}

func (u URN) String() string {
	var builder strings.Builder
	builder.WriteString(u.prefix)
	builder.WriteRune(':')
	if len(u.idParts) > 1 {
		builder.WriteRune('(')
	}
	for _, part := range u.idParts {
		builder.WriteString(part)
	}
	if len(u.idParts) > 1 {
		builder.WriteRune(')')
	}
	return builder.String()
}

func (u *URN) UnmarshalJSON(data []byte) (err error) {
	var urn string
	err = json.Unmarshal(data, &urn)
	if err != nil {
		return err
	}
	parts := strings.Split(urn, ":")
	u.prefix = strings.Join(parts[:len(parts)-1], ":")
	u.idParts = strings.Split(strings.Trim(parts[len(parts)-1], "()"), ",")
	return nil
}

func (u *URN) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}
