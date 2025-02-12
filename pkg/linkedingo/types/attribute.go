package types

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
	// Entity is a user mention.
	Entity *Entity `json:"entity,omitempty"`
}

// Entity represents a com.linkedin.pemberly.text.Entity object.
type Entity struct {
	URN URN `json:"urn,omitempty"`
}
