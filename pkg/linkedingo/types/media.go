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

// ExternalProxyImage represents a com.linkedin.messenger.ExternalProxyImage
// object.
type ExternalProxyImage struct {
	OriginalHeight int    `json:"originalHeight,omitempty"`
	OriginalWidth  int    `json:"originalWidth,omitempty"`
	URL            string `json:"url,omitempty"`
}

// ExternalMedia represents a com.linkedin.messenger.ExternalMedia object.
type ExternalMedia struct {
	Media        ExternalProxyImage `json:"media,omitempty"`
	Title        string             `json:"title,omitempty"`
	EntityURN    URN                `json:"entityUrn,omitempty"`
	PreviewMedia ExternalProxyImage `json:"previewMedia,omitempty"`
}

// VideoPlayMetadata represents a com.linkedin.videocontent.VideoPlayMetadata
// object.
type VideoPlayMetadata struct {
	Thumbnail          *VectorImage                  `json:"thumbnail,omitempty"`
	ProgressiveStreams []ProgressiveDownloadMetadata `json:"progressiveStreams,omitempty"`
	AspectRatio        float64                       `json:"aspectRatio,omitempty"`
	Media              URN                           `json:"media,omitempty"`
	Duration           jsontime.Milliseconds         `json:"duration,omitempty"`
	EntityURN          URN                           `json:"entityUrn,omitempty"`
}

// ProgressiveDownloadMetadata represents a
// com.linkedin.videocontent.ProgressiveDownloadMetadata object.
type ProgressiveDownloadMetadata struct {
	StreamingLocations []StreamingLocation `json:"streamingLocations,omitempty"`
	Size               int                 `json:"size,omitempty"`
	BitRate            int                 `json:"bitRate,omitempty"`
	Width              int                 `json:"width,omitempty"`
	MediaType          string              `json:"mediaType,omitempty"`
	Height             int                 `json:"height,omitempty"`
}

// StreamingLocation represents a com.linkedin.videocontent.StreamingLocation
// object.
type StreamingLocation struct {
	URL string `json:"url,omitempty"`
}
