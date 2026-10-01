package connector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/mediaproxy"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var _ bridgev2.DirectMediableNetwork = (*LinkedInConnector)(nil)

func (l *LinkedInConnector) SetUseDirectMedia() {
	l.DirectMedia = true
}

func (l *LinkedInConnector) Download(ctx context.Context, mediaID networkid.MediaID, params map[string]string) (mediaproxy.GetMediaResponse, error) {
	mediaInfo, err := ParseMediaID(mediaID)
	if err != nil {
		return nil, err
	}
	zerolog.Ctx(ctx).Trace().Any("mediaInfo", mediaInfo).Any("err", err).Msg("download direct media")

	var msg *database.Message
	if mediaInfo.PartID == "" {
		msg, err = l.Bridge.DB.Message.GetFirstPartByID(ctx, mediaInfo.UserID, mediaInfo.MessageID)
	} else {
		msg, err = l.Bridge.DB.Message.GetPartByID(ctx, mediaInfo.UserID, mediaInfo.MessageID, mediaInfo.PartID)
		// The part ID embedded in a media URI is the pre-merge index (e.g. "part_0"),
		// but a single media part with a text caption gets merged by ConvertedMessage.MergeCaption,
		// which resets the stored part ID to the (empty) caption part's ID. Fall back to the first
		// part so those messages remain downloadable.
		if err == nil && msg == nil {
			msg, err = l.Bridge.DB.Message.GetFirstPartByID(ctx, mediaInfo.UserID, mediaInfo.MessageID)
			if msg != nil && msg.PartID != "" {
				msg = nil
			}
		}
	}
	if err != nil {
		return nil, err
	} else if msg == nil {
		return nil, fmt.Errorf("message not found")
	}

	dmm := msg.Metadata.(*MessageMetadata).DirectMediaMeta
	if dmm == nil {
		return nil, fmt.Errorf("message does not have direct media metadata")
	}

	ul := l.Bridge.GetCachedUserLoginByID(mediaInfo.UserID)
	if ul == nil || !ul.Client.IsLoggedIn() {
		return nil, fmt.Errorf("no logged in user found")
	}

	client := ul.Client.(*LinkedInClient)
	resp, err := l.downloadDirectMedia(ctx, client.client, msg, mediaInfo.PartID)
	if err != nil {
		return nil, err
	}
	return &mediaproxy.GetMediaResponseData{
		Reader:        resp.Body,
		ContentType:   resp.Header.Get("content-type"),
		ContentLength: resp.ContentLength,
	}, nil
}

type directMediaClient interface {
	DownloadHTTP(context.Context, string) (*http.Response, error)
	GetMessagesBefore(context.Context, linkedingo.URN, time.Time, int) (*linkedingo.CollectionResponse[linkedingo.MessageMetadata, linkedingo.Message], error)
}

func (l *LinkedInConnector) downloadDirectMedia(ctx context.Context, client directMediaClient, msg *database.Message, partID networkid.PartID) (*http.Response, error) {
	meta := msg.Metadata.(*MessageMetadata)
	info := meta.DirectMediaMeta
	var downloadErr error
	if info.ExpiresAt.IsZero() || time.Now().Before(info.ExpiresAt.Time) {
		var resp *http.Response
		resp, downloadErr = client.DownloadHTTP(ctx, info.URL)
		if downloadErr == nil {
			return resp, nil
		}
		var responseErr *linkedingo.ResponseError
		if !errors.As(downloadErr, &responseErr) {
			return nil, downloadErr
		}
		switch responseErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		default:
			return nil, downloadErr
		}
	}

	zerolog.Ctx(ctx).Debug().Err(downloadErr).Msg("Refreshing direct media URL")
	fresh, err := refreshDirectMedia(ctx, client, msg, partID)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh direct media: %w", errors.Join(downloadErr, err))
	}
	fresh.MimeType = info.MimeType
	meta.DirectMediaMeta = fresh
	if err = l.Bridge.DB.Message.Update(ctx, msg); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("Failed to save refreshed direct media URL")
	}
	return client.DownloadHTTP(ctx, fresh.URL)
}

func refreshDirectMedia(ctx context.Context, client directMediaClient, msg *database.Message, partID networkid.PartID) (*DirectMediaMeta, error) {
	messages, err := client.GetMessagesBefore(ctx, linkedingo.NewURN(msg.Room.ID), msg.Timestamp.Add(time.Millisecond), 20)
	if err != nil {
		return nil, err
	}
	if messages != nil {
		for _, remote := range messages.Elements {
			if remote.MessageID() == msg.ID && remote.MessageBodyRenderFormat != linkedingo.MessageBodyRenderFormatRecalled {
				return directMediaFromMessage(remote, partID)
			}
		}
	}
	return nil, fmt.Errorf("message not found when refreshing direct media")
}

func directMediaFromMessage(msg linkedingo.Message, partID networkid.PartID) (*DirectMediaMeta, error) {
	if partID != "" {
		index, err := strconv.Atoi(strings.TrimPrefix(string(partID), "part_"))
		if err != nil || !strings.HasPrefix(string(partID), "part_") || index < 0 || index >= len(msg.RenderContent) {
			return nil, fmt.Errorf("invalid direct media part ID %q", partID)
		}
		if info := directMediaFromContent(msg.RenderContent[index]); info != nil && info.URL != "" {
			return info, nil
		}
		return nil, fmt.Errorf("media attachment %q not found in refreshed message", partID)
	}

	var result *DirectMediaMeta
	for _, content := range msg.RenderContent {
		if info := directMediaFromContent(content); info != nil && info.URL != "" {
			if result != nil {
				return nil, fmt.Errorf("multiple media attachments in message without a part ID")
			}
			result = info
		}
	}
	if result == nil {
		return nil, fmt.Errorf("no media attachments in refreshed message")
	}
	return result, nil
}

func directMediaFromContent(content linkedingo.RenderContent) *DirectMediaMeta {
	var url string
	var expiresAt jsontime.UnixMilli
	switch {
	case content.Audio != nil:
		url = content.Audio.URL
	case content.ExternalMedia != nil:
		url = content.ExternalMedia.Media.URL
	case content.File != nil:
		url = content.File.URL
	case content.VectorImage != nil:
		url = content.VectorImage.GetLargestArtifactURL()
		expiresAt = content.VectorImage.GetLargestArtifact().ExpiresAt
	case content.Video != nil:
		streams := content.Video.ProgressiveStreams
		if len(streams) > 0 && len(streams[0].StreamingLocations) > 0 {
			url = streams[0].StreamingLocations[0].URL
		}
	default:
		return nil
	}
	return &DirectMediaMeta{URL: url, ExpiresAt: expiresAt}
}
