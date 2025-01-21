package connector

import (
	"context"

	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"
)

func (lc *LinkedInConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{}
}

func (tg *LinkedInConnector) GetBridgeInfoVersion() (info, capabilities int) {
	return 1, 1
}

// TODO get these from getConfig instead of hardcoding?

const MaxTextLength = 4096
const MaxCaptionLength = 1024
const MaxFileSize = 2 * 1024 * 1024 * 1024

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
	event.FmtUserLink:           event.CapLevelDropped,
	// TODO support room links and event links (convert to appropriate t.me links)
	event.FmtUnorderedList: event.CapLevelDropped,
	event.FmtOrderedList:   event.CapLevelDropped,
	event.FmtListStart:     event.CapLevelDropped,
	event.FmtListJumpValue: event.CapLevelDropped,
	// TODO support custom emojis in messages
	event.FmtCustomEmoji:   event.CapLevelDropped,
	event.FmtSpoiler:       event.CapLevelDropped,
	event.FmtSpoilerReason: event.CapLevelDropped,
	event.FmtHeaders:       event.CapLevelDropped,
}

var fileCaps = event.FileFeatureMap{
	event.MsgImage: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"image/jpeg": event.CapLevelFullySupported,
			"image/png":  event.CapLevelPartialSupport,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxCaptionLength,
		MaxSize:          10 * 1024 * 1024,
	},
	event.MsgVideo: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"video/mp4": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxCaptionLength,
		MaxSize:          MaxFileSize,
	},
	event.MsgAudio: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"audio/mpeg": event.CapLevelFullySupported,
			"audio/mp4":  event.CapLevelFullySupported,
			// TODO some other formats are probably supported too
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxCaptionLength,
		MaxSize:          MaxFileSize,
	},
	event.MsgFile: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"*/*": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxCaptionLength,
		MaxSize:          MaxFileSize,
	},
	event.CapMsgGIF: {
		MimeTypes: map[string]event.CapabilitySupportLevel{
			"image/gif": event.CapLevelPartialSupport,
			"video/mp4": event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxCaptionLength,
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
			"audio/ogg":  event.CapLevelFullySupported,
			"audio/mpeg": event.CapLevelFullySupported,
			"audio/mp4":  event.CapLevelFullySupported,
		},
		Caption:          event.CapLevelFullySupported,
		MaxCaptionLength: MaxCaptionLength,
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

func (t *LinkedInClient) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	return &event.RoomFeatures{
		ID:                  "fi.mau.linkedin.capabilities.2025_01_21",
		Formatting:          formattingCaps,
		File:                fileCaps,
		MaxTextLength:       MaxTextLength,
		LocationMessage:     event.CapLevelDropped,
		Reply:               event.CapLevelDropped,
		Edit:                event.CapLevelDropped,
		Delete:              event.CapLevelDropped,
		Reaction:            event.CapLevelDropped,
		ReactionCount:       1,
		ReadReceipts:        true,
		TypingNotifications: true,
	}
}
