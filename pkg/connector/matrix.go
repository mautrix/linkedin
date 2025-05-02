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
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/util/variationselector"

	"go.mau.fi/mautrix-linkedin/pkg/connector/matrixfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
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
	conversationURN := linkedingo.NewURN(msg.Portal.ID)

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
	var progressiveStreamsContent []linkedingo.SendProgressiveStreams
	var urls []linkedingo.SendURL
	var artifacts []linkedingo.SendArtifacts

	if msg.Content.MsgType.IsMedia() {
		err := l.main.Bridge.Bot.DownloadMediaToFile(ctx, msg.Content.URL, msg.Content.File, false, func(f *os.File) error {
			attachmentType := linkedingo.MediaUploadTypePhotoAttachment
			if msg.Content.MsgType != event.MsgImage {
				attachmentType = linkedingo.MediaUploadTypeFileAttachment
			}

			if msg.Content.MsgType == event.MsgVideo {
				attachmentType = linkedingo.MediaUploadTypeVideoAttachment
			}

			filename := getMediaFilename(msg.Content)
			urn, err := l.client.UploadMedia(ctx, attachmentType, filename, msg.Content.Info.MimeType, msg.Content.Info.Size, f)
			if err != nil {
				return err
			}

			//handle video attachment
			if msg.Content.MsgType == event.MsgVideo {
				id := uuid.New()
				blob_string := "blob:https://www.linkedin.com/" + id.String()

				urls = append(urls, linkedingo.SendURL{
					URL: blob_string,
				})

				progressiveStreamsContent = append(progressiveStreamsContent, linkedingo.SendProgressiveStreams{
					BitRate:            0,
					Height:             0,
					MediaType:          msg.Content.Info.MimeType,
					Size:               msg.Content.Info.Size,
					Width:              0,
					StreamingLocations: urls,
				})

				artifacts = append(artifacts, linkedingo.SendArtifacts{
					Width:  0,
					Height: 0,
				})

				thumbnails := linkedingo.SendThumbnail{
					RootUrl:   "",
					Artifacts: artifacts,
				}

				renderContent = append(renderContent, linkedingo.SendRenderContent{
					Video: &linkedingo.SendVideo{
						Media:              urn,
						Thumbnail:          thumbnails,
						TrackingID:         urn,
						ProgressiveStreams: progressiveStreamsContent,
					},
				})
			} else {
				renderContent = append(renderContent, linkedingo.SendRenderContent{
					File: &linkedingo.SendFile{
						AssetURN:  urn,
						Name:      filename,
						MediaType: msg.Content.Info.MimeType,
						ByteSize:  msg.Content.Info.Size,
					},
				})
			}
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
		StreamOrder: resp.Data.DeliveredAt.UnixMilli(),
	}, nil
}

func (l *LinkedInClient) HandleMatrixEdit(ctx context.Context, msg *bridgev2.MatrixEdit) error {
	return l.client.EditMessage(ctx, linkedingo.NewURN(msg.EditTarget.ID), matrixfmt.Parse(ctx, l.matrixParser, msg.Content))
}

func (l *LinkedInClient) HandleMatrixMessageRemove(ctx context.Context, msg *bridgev2.MatrixMessageRemove) error {
	return l.client.RecallMessage(ctx, linkedingo.NewURN(msg.TargetMessage.ID))
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
	return &database.Reaction{}, l.client.SendReaction(ctx, linkedingo.NewURN(msg.TargetMessage.ID), msg.PreHandleResp.Emoji)
}

func (l *LinkedInClient) HandleMatrixReactionRemove(ctx context.Context, msg *bridgev2.MatrixReactionRemove) error {
	return l.client.RemoveReaction(ctx, linkedingo.NewURN(msg.TargetReaction.MessageID), msg.TargetReaction.Emoji)
}

func (l *LinkedInClient) HandleMatrixReadReceipt(ctx context.Context, msg *bridgev2.MatrixReadReceipt) error {
	_, err := l.client.MarkConversationRead(ctx, linkedingo.NewURN(msg.Portal.ID))
	return err
}

func (l *LinkedInClient) HandleMatrixTyping(ctx context.Context, msg *bridgev2.MatrixTyping) error {
	if msg.IsTyping && msg.Type == bridgev2.TypingTypeText {
		return l.client.StartTyping(ctx, linkedingo.NewURN(msg.Portal.ID))
	}
	return nil
}
