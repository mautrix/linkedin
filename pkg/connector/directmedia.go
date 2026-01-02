package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/mediaproxy"
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
	}
	if err != nil {
		return nil, nil
	} else if msg == nil {
		return nil, fmt.Errorf("message not found")
	}

	dmm := msg.Metadata.(*MessageMetadata).DirectMediaMeta
	if dmm == nil {
		return nil, fmt.Errorf("message does not have direct media metadata")
	}
	var info *DirectMediaMeta
	err = json.Unmarshal(dmm, &info)
	if err != nil {
		return nil, err
	}

	ul := l.Bridge.GetCachedUserLoginByID(mediaInfo.UserID)
	if ul == nil || !ul.Client.IsLoggedIn() {
		return nil, fmt.Errorf("no logged in user found")
	}

	client := ul.Client.(*LinkedInClient)
	resp, err := client.client.DownloadHTTP(ctx, info.URL)
	if err != nil {
		return nil, err
	}
	return &mediaproxy.GetMediaResponseData{
		Reader:        resp.Body,
		ContentType:   resp.Header.Get("content-type"),
		ContentLength: resp.ContentLength,
	}, nil
}
