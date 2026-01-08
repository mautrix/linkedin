package connector

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/bridgev2/status"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) onTransientDisconnect(ctx context.Context, err error) {
	zerolog.Ctx(ctx).Err(err).Msg("failed to read from event stream")
	l.userLogin.BridgeState.Send(status.BridgeState{
		StateEvent: status.StateTransientDisconnect,
		Error:      "linkedin-transient-disconnect",
		Message:    err.Error(),
	})
}

func (l *LinkedInClient) onBadCredentials(ctx context.Context, err error) {
	zerolog.Ctx(ctx).Err(err).Msg("bad credentials")
	l.userLogin.BridgeState.Send(status.BridgeState{
		StateEvent: status.StateBadCredentials,
		Error:      "linkedin-bad-credentials",
		Message:    err.Error(),
	})
	l.Disconnect()
	if errors.Is(err, linkedingo.ErrTokenInvalidated) {
		l.userLogin.Metadata.(*UserLoginMetadata).Cookies.Clear()
		err = l.userLogin.Save(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).Msg("failed to clear cookies after token invalidation")
		}
	}
}

func (l *LinkedInClient) onUnknownError(ctx context.Context, err error) {
	zerolog.Ctx(ctx).Err(err).Msg("unknown error")
	l.userLogin.BridgeState.Send(status.BridgeState{
		StateEvent: status.StateUnknownError,
		Error:      "linkedin-unknown-error",
		Message:    err.Error(),
	})
	// TODO probably don't do this unconditionally?
	l.Disconnect()
}

func (l *LinkedInClient) onDecoratedEvent(ctx context.Context, decoratedEvent *linkedingo.DecoratedEvent) {
	log := zerolog.Ctx(ctx).With().
		Str("decorated_event_id", decoratedEvent.ID).
		Stringer("topic", decoratedEvent.Topic).
		Time("left_server_at", decoratedEvent.LeftServerAt.Time).
		Logger()
	log.Debug().Msg("Received decorated event")

	// The topics are always of the form "urn:li-realtime:TOPIC_NAME:<topic_dependent>"
	switch decoratedEvent.Topic.NthPrefixPart(2) {
	case linkedingo.RealtimeEventTopicConversations:
		l.onRealtimeConversations(ctx)
	case linkedingo.RealtimeEventTopicConversationDelete:
		l.onRealtimeConversationDelete(ctx, decoratedEvent.Payload.Data.DecoratedConversationDelete.Result)
	case linkedingo.RealtimeEventTopicMessages:
		l.onRealtimeMessage(ctx, decoratedEvent.Payload.Data.DecoratedMessage.Result)
	case linkedingo.RealtimeEventTopicTypingIndicators:
		l.onRealtimeTypingIndicator(decoratedEvent)
	case linkedingo.RealtimeEventTopicMessageSeenReceipts:
		l.onRealtimeMessageSeenReceipts(ctx, decoratedEvent.Payload.Data.DecoratedSeenReceipt.Result)
	case linkedingo.RealtimeEventTopicMessageReactionSummaries:
		l.onRealtimeReactionSummaries(ctx, decoratedEvent.Payload.Data.DecoratedReactionSummary.Result)
	default:
		log.Warn().Msg("Unsupported event topic")
	}
}

func (l *LinkedInClient) onRealtimeConversations(ctx context.Context) {
	convs, err := l.client.GetConversations(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("failed to get conversations")
	}

	l.handleConversations(ctx, convs.Elements)
}

func (l *LinkedInClient) onRealtimeConversationDelete(ctx context.Context, conv linkedingo.Conversation) {
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatDelete{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventChatDelete,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.Stringer("entity_urn", conv.EntityURN)
			},
			PortalKey: l.makePortalKey(conv),
		},
	})
}

func (l *LinkedInClient) onRealtimeMessage(ctx context.Context, msg linkedingo.Message) {
	log := zerolog.Ctx(ctx)
	log.Trace().
		Str("body_text", msg.Body.Text).
		Int("render_content_count", len(msg.RenderContent)).
		Str("render_format", string(msg.MessageBodyRenderFormat)).
		Stringer("sender_urn", msg.Sender.EntityURN).
		Stringer("message_urn", msg.EntityURN).
		Stringer("conversation_urn", msg.Conversation.EntityURN).
		Msg("Processing realtime message")
	meta := simplevent.EventMeta{
		LogContext: func(c zerolog.Context) zerolog.Context {
			return c.
				Stringer("entity_urn", msg.EntityURN).
				Stringer("sender", msg.Sender.EntityURN)
		},
		PortalKey:   l.makePortalKey(msg.Conversation),
		Sender:      l.makeSender(msg.Sender),
		Timestamp:   msg.DeliveredAt.Time,
		StreamOrder: msg.DeliveredAt.UnixMilli(),
	}

	chatInfo, _ := l.conversationToChatInfo(msg.Conversation)
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatResync{
		EventMeta:       meta.WithType(bridgev2.RemoteEventChatResync),
		ChatInfo:        &chatInfo,
		LatestMessageTS: msg.DeliveredAt.Time,
	})

	evt := simplevent.Message[linkedingo.Message]{
		ID:                 msg.MessageID(),
		TargetMessage:      msg.MessageID(),
		Data:               msg,
		ConvertMessageFunc: l.convertToMatrix,
		ConvertEditFunc:    l.convertEditToMatrix,
	}
	switch msg.MessageBodyRenderFormat {
	case linkedingo.MessageBodyRenderFormatDefault:
		evt.EventMeta = meta.WithType(bridgev2.RemoteEventMessage)
	case linkedingo.MessageBodyRenderFormatEdited:
		evt.EventMeta = meta.WithType(bridgev2.RemoteEventEdit)
	case linkedingo.MessageBodyRenderFormatRecalled:
		l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.MessageRemove{
			EventMeta:     meta.WithType(bridgev2.RemoteEventMessageRemove),
			TargetMessage: msg.MessageID(),
		})
		return
	case linkedingo.MessageBodyRenderFormatSystem:
		log.Info().Msg("Ignoring system message")
		return
	default:
		log.Warn().Str("message_body_render_format", string(msg.MessageBodyRenderFormat)).Msg("Unknown render format")
	}
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &evt)
}

func (l *LinkedInClient) onRealtimeTypingIndicator(decoratedEvent *linkedingo.DecoratedEvent) {
	typingIndicator := decoratedEvent.Payload.Data.DecoratedTypingIndicator.Result
	meta := simplevent.EventMeta{
		Type: bridgev2.RemoteEventTyping,
		LogContext: func(c zerolog.Context) zerolog.Context {
			return c.
				Stringer("conversation_urn", typingIndicator.Conversation.EntityURN).
				Stringer("typing_participant_urn", typingIndicator.TypingParticipant.EntityURN)
		},
		PortalKey: l.makePortalKey(typingIndicator.Conversation),
		Sender:    l.makeSender(typingIndicator.TypingParticipant),
		Timestamp: decoratedEvent.LeftServerAt.Time,
	}

	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.Typing{
		EventMeta: meta,
		Timeout:   10 * time.Second,
		Type:      bridgev2.TypingTypeText,
	})
}

func (l *LinkedInClient) onRealtimeMessageSeenReceipts(ctx context.Context, receipt linkedingo.SeenReceipt) {
	log := zerolog.Ctx(ctx)
	part, err := l.main.Bridge.DB.Message.GetLastPartByID(ctx, l.userLogin.ID, receipt.Message.MessageID())
	if err != nil {
		log.Err(err).Msg("failed to get read message")
	} else if part == nil {
		log.Warn().Msg("couldn't find read message")
		return
	}
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.Receipt{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventReadReceipt,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.
					Time("seen_at", receipt.SeenAt.Time).
					Stringer("message_urn", receipt.Message.EntityURN).
					Stringer("typing_participant_urn", receipt.SeenByParticipant.EntityURN)
			},
			PortalKey: part.Room,
			Sender:    l.makeSender(receipt.SeenByParticipant),
			Timestamp: receipt.SeenAt.Time,
		},
		LastTarget: receipt.Message.MessageID(),
	})
}

func (l *LinkedInClient) onRealtimeReactionSummaries(ctx context.Context, summary linkedingo.RealtimeReactionSummary) {
	messageData, err := l.main.Bridge.DB.Message.GetFirstPartByID(context.TODO(), l.userLogin.ID, summary.Message.MessageID())
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("failed to get reacted to message")
		return
	}

	meta := simplevent.EventMeta{
		Type: bridgev2.RemoteEventReaction,
		LogContext: func(c zerolog.Context) zerolog.Context {
			return c.
				Stringer("message_id", summary.Message.EntityURN).
				Stringer("sender", summary.Actor.EntityURN)
		},
		PortalKey: messageData.Room,
		Timestamp: time.Now(),
		Sender:    l.makeSender(summary.Actor),
	}
	if !summary.ReactionAdded {
		meta.Type = bridgev2.RemoteEventReactionRemove
	}

	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.Reaction{
		EventMeta:     meta,
		EmojiID:       networkid.EmojiID(summary.ReactionSummary.Emoji),
		Emoji:         summary.ReactionSummary.Emoji,
		TargetMessage: summary.Message.MessageID(),
	})
}
