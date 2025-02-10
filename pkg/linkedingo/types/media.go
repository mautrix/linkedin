package types

import (
	"go.mau.fi/util/jsontime"
)

// VectorArtifact represents a com.linkedin.common.VectorArtifact object.
type VectorArtifact struct {
	ExpiresAt                     jsontime.UnixMilli `json:"expiresAt,omitempty"`
	FileIdentifyingURLPathSegment string             `json:"fileIdentifyingUrlPathSegment,omitempty"`
	Height                        int                `json:"height,omitempty"`
	Width                         int                `json:"width,omitempty"`
}

// VectorImage represents a com.linkedin.common.VectorImage object.
type VectorImage struct {
	RootURL   string           `json:"rootUrl,omitempty"`
	Artifacts []VectorArtifact `json:"artifacts,omitempty"`
}

func (vi VectorImage) GetLargestArtifactURL() string {
	var largestVersion VectorArtifact
	for _, a := range vi.Artifacts {
		if a.Height > largestVersion.Height {
			largestVersion = a
		}
	}
	return vi.RootURL + largestVersion.FileIdentifyingURLPathSegment
}

type Picture struct {
	VectorImage *VectorImage `json:"com.linkedin.common.VectorImage,omitempty"`
}

// FileAttachment represents a com.linkedin.messenger.FileAttachment object.
type FileAttachment struct {
	AssetURN  URN    `json:"assetUrn,omitempty"`
	ByteSize  int    `json:"byteSize,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Name      string `json:"name,omitempty"`
	URL       string `json:"url,omitempty"`
}
