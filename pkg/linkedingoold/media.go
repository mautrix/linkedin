package linkedingoold

import (
	"fmt"
	"net/http"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/payload"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/query"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/response"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

func (c *Client) UploadMedia(mediaUploadType payload.MediaUploadType, fileName string, mediaBytes []byte, contentType typesold.ContentType) (*response.MediaMetadata, error) {
	uploadMetadataQuery := query.DoActionQuery{
		Action: query.ActionUpload,
	}
	uploadMetadataPayload := payload.UploadMediaMetadataPayload{
		MediaUploadType: mediaUploadType,
		FileSize:        len(mediaBytes),
		Filename:        fileName,
	}

	_, respData, err := c.MakeRoutingRequest(routing.LinkedInVoyagerMediaUploadMetadataURL, uploadMetadataPayload, uploadMetadataQuery)
	if err != nil {
		return nil, err
	}

	metaDataResp, ok := respData.(*response.UploadMediaMetadataResponse)
	if !ok {
		return nil, newErrorResponseTypeAssertFailed("*response.UploadMediaMetadataResponse")
	}

	metaData := metaDataResp.Data.Value
	uploadUrl := metaData.SingleUploadURL

	uploadHeaders := c.buildHeaders(typesold.HeaderOpts{WithCookies: true, WithCsrfToken: true})
	resp, _, err := c.MakeRequest(uploadUrl, http.MethodPut, uploadHeaders, mediaBytes, contentType)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 204 {
		return nil, fmt.Errorf("failed to upload media with file name %s (statusCode=%d)", fileName, resp.StatusCode)
	}

	return &metaData, err
}
