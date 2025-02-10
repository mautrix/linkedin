package linkedingo

import (
	"context"
	"io"
	"net/http"

	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (c *Client) Download(ctx context.Context, vi types.VectorImage) (io.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vi.RootURL+vi.GetLargestArtifact().FileIdentifyingURLPathSegment, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) DownloadBytes(ctx context.Context, vi types.VectorImage) ([]byte, error) {
	reader, err := c.Download(ctx, vi)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

func (c *Client) GetFileInfo(ctx context.Context, vi types.VectorImage) (info event.FileInfo, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, vi.RootURL+vi.GetLargestArtifact().FileIdentifyingURLPathSegment, nil)
	if err != nil {
		return info, err
	}

	headResp, err := c.http.Do(req)
	if err != nil {
		return info, err
	}
	info.MimeType = headResp.Header.Get("Content-Type")
	info.Size = int(headResp.ContentLength)
	return
}
