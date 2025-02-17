package linkedingo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *Client) Download(ctx context.Context, w io.Writer, url string) error {
	resp, err := c.newAuthedRequest(http.MethodGet, url).Do(ctx)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func (c *Client) DownloadBytes(ctx context.Context, url string) ([]byte, error) {
	var buf bytes.Buffer
	err := c.Download(ctx, &buf, url)
	return buf.Bytes(), err
}

func (c *Client) getFileInfoFromHeadRequest(ctx context.Context, url string) (info event.FileInfo, filename string, err error) {
	headResp, err := c.newAuthedRequest(http.MethodHead, url).Do(ctx)
	if err != nil {
		return info, "", err
	}
	info.MimeType = headResp.Header.Get("Content-Type")
	info.Size = int(headResp.ContentLength)
	_, params, _ := mime.ParseMediaType(headResp.Header.Get("Content-Disposition"))
	filename = params["filename"]
	return
}

func (c *Client) GetVectorImageFileInfo(ctx context.Context, vi *types.VectorImage) (info event.FileInfo, filename string, err error) {
	info, filename, err = c.getFileInfoFromHeadRequest(ctx, vi.GetLargestArtifactURL())
	if filename == "" {
		filename = "image"
	}
	return
}

func (c *Client) GetAudioFileInfo(ctx context.Context, audio *types.AudioMetadata) (info event.FileInfo, filename string, err error) {
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
	URN             types.URN `json:"urn,omitempty"`
	SingleUploadURL string    `json:"singleUploadUrl,omitempty"`
}

func (c *Client) UploadMedia(ctx context.Context, mediaUploadType MediaUploadType, filename, contentType string, size int, r io.Reader) (types.URN, error) {
	resp, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerMediaUploadMetadataURL).
		WithParam("action", "upload").
		WithCSRF().
		WithXLIHeaders().
		WithHeader("accept", contentTypeJSONLinkedInNormalized).
		WithContentType(contentTypeJSONPlaintextUTF8).
		WithJSONPayload(UploadMediaMetadataPayload{
			MediaUploadType: mediaUploadType,
			FileSize:        size,
			Filename:        filename,
		}).
		Do(ctx)
	if err != nil {
		return types.URN{}, err
	} else if resp.StatusCode != http.StatusOK {
		return types.URN{}, fmt.Errorf("failed to get upload media metadata (statusCode=%d)", resp.StatusCode)
	}

	var uploadMetadata UploadMediaMetadataResponse
	if err = json.NewDecoder(resp.Body).Decode(&uploadMetadata); err != nil {
		return types.URN{}, err
	}

	resp, err = c.newAuthedRequest(http.MethodPut, uploadMetadata.Data.Value.SingleUploadURL).
		WithCSRF().
		WithHeader("content-length", strconv.Itoa(size)).
		WithContentType(contentType).
		WithBody(r).
		Do(ctx)
	if err != nil {
		return types.URN{}, err
	} else if resp.StatusCode != http.StatusCreated {
		return types.URN{}, fmt.Errorf("failed to upload media: status=%d", resp.StatusCode)
	}
	return uploadMetadata.Data.Value.URN, nil
}
