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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
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

func (c *Client) GetVectorImageFileInfo(ctx context.Context, vi *types.VectorImage) (info event.FileInfo, filename string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, vi.GetLargestArtifactURL(), nil)
	if err != nil {
		return info, "", err
	}

	headResp, err := c.http.Do(req)
	if err != nil {
		return info, "", err
	}
	info.MimeType = headResp.Header.Get("Content-Type")
	info.Size = int(headResp.ContentLength)
	_, params, err := mime.ParseMediaType(headResp.Header.Get("Content-Disposition"))
	return info, params["filename"], err
}
