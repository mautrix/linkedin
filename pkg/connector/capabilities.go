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

	"go.mau.fi/util/ffmpeg"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"
)

func (*LinkedInConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{}
}

func (*LinkedInConnector) GetBridgeInfoVersion() (info, capabilities int) {
	return 1, 2
}

const MaxTextLength = 8000
const MaxFileSize = 20 * 1024 * 1024

func supportedIfFFmpeg() event.CapabilitySupportLevel {
	if ffmpeg.Supported() {
		return event.CapLevelPartialSupport
	}
	return event.CapLevelRejected
}

var formattingCaps = event.FormattingFeatureMap{
	event.FmtBold:               event.CapLevelDropped,
	event.FmtItalic:             event.CapLevelDropped,
	event.FmtUnderline:          event.CapLevelDropped,
	event.FmtStrikethrough:      event.CapLevelDropped,
	event.FmtInlineCode:         event.CapLevelDropped,
	event.FmtCodeBlock:          event.CapLevelDropped,
	event.FmtSyntaxHighlighting: event.CapLevelDropped,
	event.FmtBlockquote:         event.CapLevelDropped,
	event.FmtInlineLink:         event.CapLevelDropped,
	event.FmtUserLink:           event.CapLevelFullySupported,
	event.FmtUnorderedList:      event.CapLevelDropped,
	event.FmtOrderedList:        event.CapLevelDropped,
	event.FmtListStart:          event.CapLevelDropped,
	event.FmtListJumpValue:      event.CapLevelDropped,
	event.FmtCustomEmoji:        event.CapLevelDropped,
	event.FmtSpoiler:            event.CapLevelDropped,
	event.FmtSpoilerReason:      event.CapLevelDropped,
	event.FmtHeaders:            event.CapLevelDropped,
}

var fileCaps = event.FileFeatureMap{
	event.MsgImage: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"image/jpeg": event.CapLevelFullySupported,
			"image/png":  event.CapLevelFullySupported,
			"image/webp": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxTextLength,
		MaxSize:          MaxFileSize,
	},
	event.MsgVideo: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"video/mp4": event.CapLevelPartialSupport,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxTextLength,
		MaxSize:          MaxFileSize,
	},
	event.MsgAudio: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"audio/mpeg": event.CapLevelFullySupported,
			"audio/mp4":  event.CapLevelFullySupported,
			// TODO some other formats are probably supported too
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxTextLength,
		MaxSize:          MaxFileSize,
	},
	event.MsgFile: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"*/*": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxTextLength,
		MaxSize:          MaxFileSize,
	},
	event.CapMsgGIF: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"image/gif": event.CapLevelPartialSupport,
			"video/mp4": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxTextLength,
		MaxSize:          MaxFileSize,
	},
	event.CapMsgSticker: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"image/webp": event.CapLevelFullySupported,
			// TODO
			//"image/lottie+json": event.CapLevelFullySupported,
			//"video/webm": event.CapLevelFullySupported,
		},
	},
	event.CapMsgVoice: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"audio/ogg": supportedIfFFmpeg(),
			"audio/mp4": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxTextLength,
		MaxSize:          MaxFileSize,
	},
}
var premiumFileCaps event.FileFeatureMap

func init() {
	premiumFileCaps = make(event.FileFeatureMap, len(fileCaps))
	for k, v := range fileCaps {
		cloned := ptr.Clone(v)
		if k == event.MsgFile || k == event.MsgVideo || k == event.MsgAudio {
			cloned.MaxSize *= 2
		}
		cloned.MaxCaptionLength *= 2
		premiumFileCaps[k] = cloned
	}
}

func (*LinkedInClient) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	return &event.RoomFeatures{
		ID:                  "fi.mau.linkedin.capabilities.2025_01_21",
		Formatting:          formattingCaps,
		File:                fileCaps,
		MaxTextLength:       MaxTextLength,
		LocationMessage:     event.CapLevelDropped,
		Reply:               event.CapLevelDropped,
		Edit:                event.CapLevelFullySupported, // TODO note that edits are restricted to specific msgtypes
		Delete:              event.CapLevelDropped,
		Reaction:            event.CapLevelFullySupported,
		ReadReceipts:        true,
		TypingNotifications: true,
	}
}
