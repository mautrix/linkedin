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
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"maunium.net/go/mautrix/event"

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

// AudioMetadata represents a com.linkedin.messenger.AudioMetadata object.
type AudioMetadata struct {
	Duration jsontime.Milliseconds `json:"duration,omitempty"`
	URL      string                `json:"url,omitempty"`
}

func (c *Client) DownloadHTTP(ctx context.Context, url string) (*http.Response, error) {
	return c.newAuthedRequest(http.MethodGet, url).DoRaw(ctx)
}

func (c *Client) Download(ctx context.Context, w io.Writer, url string) error {
	resp, err := c.DownloadHTTP(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	return nil
}

func (c *Client) DownloadBytes(ctx context.Context, url string) ([]byte, error) {
	var buf bytes.Buffer
	err := c.Download(ctx, &buf, url)
	return buf.Bytes(), err
}

func (c *Client) getFileInfoFromHeadRequest(ctx context.Context, url string) (info event.FileInfo, filename string, err error) {
	headResp, err := c.newAuthedRequest(http.MethodHead, url).Do(ctx, nil)
	if err != nil {
		return info, "", err
	}
	info.MimeType = headResp.Header.Get("Content-Type")
	info.Size = int(headResp.ContentLength)
	_, params, _ := mime.ParseMediaType(headResp.Header.Get("Content-Disposition"))
	filename = params["filename"]
	return
}

func (c *Client) GetVectorImageFileInfo(ctx context.Context, vi *VectorImage) (info event.FileInfo, filename string, err error) {
	info, filename, err = c.getFileInfoFromHeadRequest(ctx, vi.GetLargestArtifactURL())
	if filename == "" {
		filename = "image"
	}
	return
}

func (c *Client) GetAudioFileInfo(ctx context.Context, audio *AudioMetadata) (info event.FileInfo, filename string, err error) {
	info, filename, err = c.getFileInfoFromHeadRequest(ctx, audio.URL)
	if filename == "" {
		filename = "voice_message"
	}
	return
}

type MediaUploadType string

const (
	MediaUploadTypePhotoAttachment MediaUploadType = "MESSAGING_PHOTO_ATTACHMENT"
	MediaUploadTypeFileAttachment  MediaUploadType = "MESSAGING_FILE_ATTACHMENT"
	MediaUploadTypeVideoAttachment MediaUploadType = "MESSAGING_VIDEO_ATTACHMENT"
	MediaUploadTypeVoiceMessage    MediaUploadType = "VOICE_MESSAGE"
)

type UploadMediaMetadataPayload struct {
	MediaUploadType MediaUploadType `json:"mediaUploadType,omitempty"`
	FileSize        int             `json:"fileSize,omitempty"`
	Filename        string          `json:"filename,omitempty"`
}

type UploadMediaMetadataResponse struct {
	Data ActionResponse `json:"data,omitempty"`
}

// ActionResponse represents a com.linkedin.restli.common.ActionResponse
// object.
type ActionResponse struct {
	Value MediaUploadMetadata `json:"value,omitempty"`
}

// MediaUploadMetadata represents a
// com.linkedin.mediauploader.MediaUploadMetadata object.
type MediaUploadMetadata struct {
	URN             URN    `json:"urn,omitempty"`
	SingleUploadURL string `json:"singleUploadUrl,omitempty"`
}

func (c *Client) UploadMedia(ctx context.Context, mediaUploadType MediaUploadType, filename, contentType string, size int, r io.Reader) (URN, error) {
	var uploadMetadata UploadMediaMetadataResponse
	_, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMediaUploadMetadataURL).
		WithQueryParam("action", "upload").
		WithCSRF().
		WithXLIHeaders().
		WithHeader("accept", contentTypeJSONLinkedInNormalized).
		WithContentType(contentTypeJSONPlaintextUTF8).
		WithJSONPayload(UploadMediaMetadataPayload{
			MediaUploadType: mediaUploadType,
			FileSize:        size,
			Filename:        filename,
		}).
		Do(ctx, &uploadMetadata)
	if err != nil {
		return URN{}, err
	}

	_, err = c.newAuthedRequest(http.MethodPut, uploadMetadata.Data.Value.SingleUploadURL).
		WithCSRF().
		WithHeader("content-length", strconv.Itoa(size)).
		WithContentType(contentType).
		WithBody(r).
		Do(ctx, nil)
	if err != nil {
		return URN{}, err
	}
	return uploadMetadata.Data.Value.URN, nil
}
