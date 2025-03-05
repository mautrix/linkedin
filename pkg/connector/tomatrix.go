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

package connector

import (
	"context"
	"fmt"
	"io"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/connector/linkedinfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) convertToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, msg linkedingo.Message) (*bridgev2.ConvertedMessage, error) {
	var cm bridgev2.ConvertedMessage

	if len(msg.Body.Text) > 0 {
		content, err := linkedinfmt.Parse(ctx, msg.Body.Text, msg.Body.Attributes, l.linkedinFmtParams)
		if err != nil {
			return nil, err
		}

		cm.Parts = []*bridgev2.ConvertedMessagePart{
			{Type: event.EventMessage, Content: content},
		}
	}

	for _, rc := range msg.RenderContent {
		var err error
		var part *bridgev2.ConvertedMessagePart
		switch {
		case rc.Audio != nil:
			part, err = l.convertAudioToMatrix(ctx, portal, intent, rc.Audio)
		case rc.ExternalMedia != nil:
			part, err = l.convertExternalMediaToMatrix(ctx, portal, intent, rc.ExternalMedia)
		case rc.File != nil:
			part, err = l.convertFileToMatrix(ctx, portal, intent, rc.File)
		case rc.RepliedMessageContent != nil:
			cm.ReplyTo = &networkid.MessageOptionalPartID{
				MessageID: rc.RepliedMessageContent.OriginalMessage.MessageID(),
			}
		case rc.VectorImage != nil:
			part, err = l.convertVectorImageToMatrix(ctx, portal, intent, rc.VectorImage)
		case rc.Video != nil:
			part, err = l.convertVideoToMatrix(ctx, portal, intent, rc.Video)
		default:
		}
		if err != nil {
			return nil, err
		} else if part != nil {
			cm.Parts = append(cm.Parts, part)
		}
	}

	cm.MergeCaption()

	return &cm, nil
}

func (l *LinkedInClient) convertEditToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, msg linkedingo.Message) (*bridgev2.ConvertedEdit, error) {
	if len(existing) != 1 {
		return nil, fmt.Errorf("editing a message that was bridged as multiple parts is not supported")
	}
	converted, err := l.convertToMatrix(ctx, portal, intent, msg)
	if err != nil {
		return nil, err
	}
	if len(converted.Parts) != 1 {
		return nil, fmt.Errorf("editing a message in a way that creates multiple parts is not supported")
	}

	var convertedEdit bridgev2.ConvertedEdit
	for i, part := range converted.Parts {
		convertedEdit.ModifiedParts = append(convertedEdit.ModifiedParts, part.ToEditPart(existing[i]))
	}
	return &convertedEdit, nil
}

func (l *LinkedInClient) convertAudioToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, audio *linkedingo.AudioMetadata) (cmp *bridgev2.ConvertedMessagePart, err error) {
	info, filename, err := l.client.GetAudioFileInfo(ctx, audio)
	if err != nil {
		return nil, err
	}
	info.Duration = int(audio.Duration.Milliseconds())
	content := event.MessageEventContent{
		Info:    &info,
		MsgType: event.MsgAudio,
		Body:    filename,
		MSC1767Audio: &event.MSC1767Audio{
			Duration: int(audio.Duration.Milliseconds()),
		},
		MSC3245Voice: &event.MSC3245Voice{},
	}

	content.URL, content.File, err = intent.UploadMediaStream(ctx, portal.MXID, 0, true, func(w io.Writer) (*bridgev2.FileStreamResult, error) {
		err := l.client.Download(ctx, w, audio.URL)
		return &bridgev2.FileStreamResult{MimeType: content.Info.MimeType}, err
	})

	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}, err
}

func (l *LinkedInClient) convertExternalMediaToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, media *linkedingo.ExternalMedia) (cmp *bridgev2.ConvertedMessagePart, err error) {
	content := event.MessageEventContent{
		Info:    &event.FileInfo{MimeType: "image/gif"},
		MsgType: event.MsgImage,
		Body:    media.Title,
	}

	content.URL, content.File, err = intent.UploadMediaStream(ctx, portal.MXID, 0, true, func(w io.Writer) (*bridgev2.FileStreamResult, error) {
		err := l.client.Download(ctx, w, media.Media.URL)
		return &bridgev2.FileStreamResult{MimeType: content.Info.MimeType}, err
	})

	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}, err
}

func (l *LinkedInClient) convertFileToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, attachment *linkedingo.FileAttachment) (cmp *bridgev2.ConvertedMessagePart, err error) {
	content := event.MessageEventContent{
		Info: &event.FileInfo{
			MimeType: attachment.MediaType,
			Size:     attachment.ByteSize,
		},
		MsgType: event.MsgFile,
		Body:    attachment.Name,
	}

	content.URL, content.File, err = intent.UploadMediaStream(ctx, portal.MXID, int64(attachment.ByteSize), true, func(w io.Writer) (*bridgev2.FileStreamResult, error) {
		err := l.client.Download(ctx, w, attachment.URL)
		return &bridgev2.FileStreamResult{
			FileName: attachment.Name,
			MimeType: content.Info.MimeType,
		}, err
	})

	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}, err
}

func (l *LinkedInClient) convertVectorImageToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, img *linkedingo.VectorImage) (cmp *bridgev2.ConvertedMessagePart, err error) {
	info, filename, err := l.client.GetVectorImageFileInfo(ctx, img)
	if err != nil {
		return nil, err
	}
	content := event.MessageEventContent{
		Info:    &info,
		MsgType: event.MsgImage,
		Body:    filename,
	}

	// TODO use smallest artifact version for thumbnail?

	content.URL, content.File, err = intent.UploadMediaStream(ctx, portal.MXID, int64(info.Size), true, func(w io.Writer) (*bridgev2.FileStreamResult, error) {
		err := l.client.Download(ctx, w, img.GetLargestArtifactURL())
		return &bridgev2.FileStreamResult{MimeType: content.Info.MimeType}, err
	})

	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}, err
}

func (l *LinkedInClient) convertVideoToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, video *linkedingo.VideoPlayMetadata) (cmp *bridgev2.ConvertedMessagePart, err error) {
	if len(video.ProgressiveStreams) == 0 {
		return nil, fmt.Errorf("VideoPlayMetadata had no ProgressiveStreams")
	}
	stream := video.ProgressiveStreams[0]
	if len(stream.StreamingLocations) == 0 {
		return nil, fmt.Errorf("VideoPlayMetadata had no StreamingLocations")
	}

	content := event.MessageEventContent{
		Info: &event.FileInfo{
			MimeType: stream.MediaType,
			Width:    stream.Width,
			Height:   stream.Height,
			Duration: int(video.Duration.Milliseconds()),
			Size:     stream.Size,
		},
		MsgType: event.MsgVideo,
		Body:    "video",
	}

	if video.Thumbnail != nil {
		part, err := l.convertVectorImageToMatrix(ctx, portal, intent, video.Thumbnail)
		if err != nil {
			return nil, err
		}
		content.Info.ThumbnailInfo = part.Content.Info
		content.Info.ThumbnailURL = part.Content.URL
		content.Info.ThumbnailFile = part.Content.File
	}

	content.URL, content.File, err = intent.UploadMediaStream(ctx, portal.MXID, 0, true, func(w io.Writer) (*bridgev2.FileStreamResult, error) {
		err := l.client.Download(ctx, w, stream.StreamingLocations[0].URL)
		return &bridgev2.FileStreamResult{MimeType: content.Info.MimeType}, err
	})

	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}, err
}
