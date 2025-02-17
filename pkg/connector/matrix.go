package connector

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/util/variationselector"

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
	if msg.Content.MsgType.IsMedia() {
		err := l.main.Bridge.Bot.DownloadMediaToFile(ctx, msg.Content.URL, msg.Content.File, false, func(f *os.File) error {
			attachmentType := linkedingo.MediaUploadTypePhotoAttachment
			if msg.Content.MsgType != event.MsgImage {
				attachmentType = linkedingo.MediaUploadTypeFileAttachment
			}

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

func (l *LinkedInClient) PreHandleMatrixReaction(ctx context.Context, msg *bridgev2.MatrixReaction) (bridgev2.MatrixReactionPreResponse, error) {
	emojiID := networkid.EmojiID(msg.Content.RelatesTo.Key)
	zerolog.Ctx(ctx).Debug().
		Str("conversion_direction", "to_linkedin").
		Str("handler", "pre_handle_matrix_reaction").
		Str("key", msg.Content.RelatesTo.Key).
		Str("emoji_id", string(emojiID)).
		Msg("Pre-handled reaction")

	return bridgev2.MatrixReactionPreResponse{
		SenderID: l.userID,
		EmojiID:  emojiID,
		Emoji:    variationselector.FullyQualify(msg.Content.RelatesTo.Key),
	}, nil
}

func (l *LinkedInClient) HandleMatrixReaction(ctx context.Context, msg *bridgev2.MatrixReaction) (reaction *database.Reaction, err error) {
	return &database.Reaction{}, l.client.SendReaction(ctx, types.NewURN(msg.TargetMessage.ID), msg.PreHandleResp.Emoji)
}

func (l *LinkedInClient) HandleMatrixReactionRemove(ctx context.Context, msg *bridgev2.MatrixReactionRemove) error {
	return l.client.RemoveReaction(ctx, types.NewURN(msg.TargetReaction.MessageID), msg.TargetReaction.Emoji)
}

func (l *LinkedInClient) HandleMatrixReadReceipt(ctx context.Context, msg *bridgev2.MatrixReadReceipt) error {
	_, err := l.client.MarkConversationRead(ctx, types.NewURN(msg.Portal.ID))
	return err
}

func (l *LinkedInClient) HandleMatrixTyping(ctx context.Context, msg *bridgev2.MatrixTyping) error {
	if msg.IsTyping && msg.Type == bridgev2.TypingTypeText {
		return l.client.StartTyping(ctx, types.NewURN(msg.Portal.ID))
	}
	return nil
}
