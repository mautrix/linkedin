package linkedingo

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
	Bold        *Bold        `json:"bold,omitempty"`
	Entity      *Entity      `json:"entity,omitempty"` // Entity is a user mention.
	Hyperlink   *Hyperlink   `json:"hyperlink,omitempty"`
	Italic      *Italic      `json:"italic,omitempty"`
	LineBreak   *LineBreak   `json:"lineBreak,omitempty"`
	List        *List        `json:"list,omitempty"`
	ListItem    *ListItem    `json:"listItem,omitempty"`
	Paragraph   *Paragraph   `json:"paragraph,omitempty"`
	Subscript   *Subscript   `json:"subscript,omitempty"`
	Superscript *Superscript `json:"superscript,omitempty"`
	Underline   *Underline   `json:"underline,omitempty"`
}

// Bold represents a com.linkedin.pemberly.text.Bold object.
type Bold struct{}

// Entity represents a com.linkedin.pemberly.text.Entity object.
type Entity struct {
	URN URN `json:"urn,omitempty"`
}

// Hyperlink represents a com.linkedin.pemberly.text.Hyperlink object.
type Hyperlink struct {
	URL string `json:"url,omitempty"`
}

// Italic represents a com.linkedin.pemberly.text.Italic object.
type Italic struct{}

// LineBreak represents a com.linkedin.pemberly.text.LineBreak object.
type LineBreak struct{}

// List represents a com.linkedin.pemberly.text.List object.
type List struct {
	Ordered bool `json:"ordered,omitempty"`
}

// ListItem represents a com.linkedin.pemberly.text.ListItem object.
type ListItem struct{}

// Paragraph represents a com.linkedin.pemberly.text.Paragraph object.
type Paragraph struct{}

// Subscript represents a com.linkedin.pemberly.text.Subscript object.
type Subscript struct{}

// Superscript represents a com.linkedin.pemberly.text.Superscript object.
type Superscript struct{}

// Underline represents a com.linkedin.pemberly.text.Underline object.
type Underline struct{}
