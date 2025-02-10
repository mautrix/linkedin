package connector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func (l *LinkedInClient) convertToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, msg types.Message) (*bridgev2.ConvertedMessage, error) {
	var cm bridgev2.ConvertedMessage
	hasher := sha256.New()

	if len(msg.Body.Text) > 0 {
		hasher.Write([]byte(msg.Body.Text))
		cm.Parts = []*bridgev2.ConvertedMessagePart{
			{
				Type: event.EventMessage,

				// TODO handle the attributes
				Content: &event.MessageEventContent{
					MsgType: event.MsgText,
					Body:    msg.Body.Text,
				},
			},
		}
	}

	for _, rc := range msg.RenderContent {
		var err error
		var part *bridgev2.ConvertedMessagePart
		switch {
		case rc.VectorImage != nil:
			part, err = l.convertVectorImageToMatrix(ctx, portal, intent, rc.VectorImage)
		case rc.File != nil:
			part, err = l.convertFileToMatrix(ctx, portal, intent, rc.File)
		case rc.ExternalMedia != nil:
			part, err = l.convertExternalMediaToMatrix(ctx, portal, intent, rc.ExternalMedia)
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

func (l *LinkedInClient) convertVectorImageToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, img *types.VectorImage) (cmp *bridgev2.ConvertedMessagePart, err error) {
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

func (l *LinkedInClient) convertEditToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, msg types.Message) (*bridgev2.ConvertedEdit, error) {
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

func (l *LinkedInClient) convertFileToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, attachment *types.FileAttachment) (cmp *bridgev2.ConvertedMessagePart, err error) {
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

func (l *LinkedInClient) convertExternalMediaToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, attachment *types.ExternalMedia) (cmp *bridgev2.ConvertedMessagePart, err error) {
	content := event.MessageEventContent{
		Info:    &event.FileInfo{MimeType: "image/gif"},
		MsgType: event.MsgImage,
		Body:    attachment.Title,
	}

	content.URL, content.File, err = intent.UploadMediaStream(ctx, portal.MXID, 0, true, func(w io.Writer) (*bridgev2.FileStreamResult, error) {
		err := l.client.Download(ctx, w, attachment.Media.URL)
		return &bridgev2.FileStreamResult{MimeType: content.Info.MimeType}, err
	})

	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}, err
}
