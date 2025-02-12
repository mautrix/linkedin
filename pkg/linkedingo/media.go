package linkedingo

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"

	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *Client) Download(ctx context.Context, w io.Writer, url string) error {
	resp, err := c.newAuthedRequest(http.MethodGet, url, nil).Do(ctx)
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
	headResp, err := c.newAuthedRequest(http.MethodHead, url, nil).Do(ctx)
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
