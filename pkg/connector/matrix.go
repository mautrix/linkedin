package connector

import (
	"context"
	"fmt"
	"os"
	"strings"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/connector/matrixfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo/types"
)

func getMediaFilename(content *event.MessageEventContent) (filename string) {
	if content.FileName != "" {
		filename = content.FileName
	} else {
		filename = content.Body
	}
	if filename == "" {
		return "image.jpg" // Assume it's a JPEG image
	}
	if content.MsgType == event.MsgImage && (!strings.HasSuffix(filename, ".jpg") && !strings.HasSuffix(filename, ".jpeg") && !strings.HasSuffix(filename, ".png")) {
		if content.Info != nil && content.Info.MimeType != "" {
			return filename + strings.TrimPrefix(content.Info.MimeType, "image/")
		}
		return filename + ".jpg" // Assume it's a JPEG
	}
	return filename
}

func (l *LinkedInClient) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (*bridgev2.MatrixMessageResponse, error) {
	conversationURN := types.NewURN(msg.Portal.ID)

	// Handle emotes by adding a "*" and the user's name to the message
	if msg.Content.MsgType == event.MsgEmote {
		if msg.Content.FormattedBody == "" {
			msg.Content.FormattedBody = msg.Content.Body
		}
		msg.Content.Format = event.FormatHTML
		msg.Content.Body = fmt.Sprintf("* %s %s", l.userLogin.RemoteName, msg.Content.Body)
		msg.Content.FormattedBody = fmt.Sprintf(`* <a href="https://matrix.to/#/%s">%s</a> %s`, l.userLogin.UserMXID, l.userLogin.RemoteName, msg.Content.FormattedBody)
		msg.Content.Mentions = &event.Mentions{UserIDs: []id.UserID{l.userLogin.UserMXID}}
	}

	var renderContent []linkedingo.SendRenderContent
	switch msg.Content.MsgType {
	case event.MsgImage:
		err := l.main.Bridge.Bot.DownloadMediaToFile(ctx, msg.Content.URL, msg.Content.File, false, func(f *os.File) error {
			attachmentType := linkedingo.MediaUploadTypePhotoAttachment
			filename := getMediaFilename(msg.Content)
			urn, err := l.client.UploadMedia(ctx, attachmentType, filename, msg.Content.Info.MimeType, msg.Content.Info.Size, f)
			if err != nil {
				return err
			}
			renderContent = append(renderContent, linkedingo.SendRenderContent{
				File: &linkedingo.SendFile{
					AssetURN:  urn,
					Name:      filename,
					MediaType: msg.Content.Info.MimeType,
					ByteSize:  msg.Content.Info.Size,
				},
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// content := msg.Content
	//
	// switch content.MsgType {
	// case event.MsgText:
	// 	break
	// case event.MsgVideo, event.MsgImage:
	// 	if content.Body == content.FileName {
	// 		sendMessagePayload.Message.Body.Text = ""
	// 	}
	//
	// 	file := content.GetFile()
	// 	data, err := lc.connector.br.Bot.DownloadMedia(ctx, file.URL, file)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	// 	attachmentType := payloadold.MediaUploadFileAttachment
	// 	if content.MsgType == event.MsgImage {
	// 		attachmentType = payloadold.MediaUploadTypePhotoAttachment
	// 	}
	//
	// 	mediaMetadata, err := lc.client.UploadMedia(attachmentType, content.FileName, data, typesold.ContentTypeJSONPlaintextUTF8)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	// 	lc.client.Logger.Debug().Any("media_metadata", mediaMetadata).Msg("Successfully uploaded media to LinkedIn's servers")
	// 	sendMessagePayload.Message.RenderContentUnions = append(sendMessagePayload.Message.RenderContentUnions, payloadold.RenderContent{
	// 		File: &payloadold.File{
	// 			AssetUrn:  mediaMetadata.Urn,
	// 			Name:      content.FileName,
	// 			MediaType: typesold.ContentType(content.Info.MimeType),
	// 			ByteSize:  len(data),
	// 		},
	// 	})
	// default:
	// 	return nil, fmt.Errorf("%w %s", bridgev2.ErrUnsupportedMessageType, content.MsgType)
	// }

	resp, err := l.client.SendMessage(ctx, conversationURN, matrixfmt.Parse(ctx, l.matrixParser, msg.Content), renderContent)
	if err != nil {
		return nil, err
	}
	return &bridgev2.MatrixMessageResponse{
		DB: &database.Message{
			ID:        resp.Data.MessageID(),
			MXID:      msg.Event.ID,
			Room:      msg.Portal.PortalKey,
			SenderID:  l.userID,
			Timestamp: resp.Data.DeliveredAt.Time,
		},
	}, nil
}

func (l *LinkedInClient) HandleMatrixEdit(ctx context.Context, msg *bridgev2.MatrixEdit) error {
	return l.client.EditMessage(ctx, types.NewURN(msg.EditTarget.ID), matrixfmt.Parse(ctx, l.matrixParser, msg.Content))
}

func (l *LinkedInClient) HandleMatrixMessageRemove(ctx context.Context, msg *bridgev2.MatrixMessageRemove) error {
	return l.client.RecallMessage(ctx, types.NewURN(msg.TargetMessage.ID))
}
