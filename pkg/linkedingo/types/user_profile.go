package types

type VectorImage struct {
	RootURL   string `json:"rootUrl,omitempty"`
	Artifacts []any  `json:"artifacts,omitempty"`
}

type Picture struct {
	VectorImage *VectorImage `json:"com.linkedin.common.VectorImage"`
}

type UserProfile struct {
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Occupation       string `json:"occupation"`
	PublicIdentifier string `json:"publicIdentifier"`
	Memorialized     bool   `json:"memorialized"`

	EntityUrn     string `json:"entityUrn"`
	ObjectUrn     string `json:"objectUrn"`
	DashEntityUrn string `json:"dashEntityUrn"`

	TrackingId string `json:"trackingId"`

	Picture Picture `json:"picture,omitempty"`
}

type UserLoginProfile struct {
	PlainId     int         `json:"plainId"`
	MiniProfile UserProfile `json:"miniProfile"`
}
