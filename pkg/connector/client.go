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
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/bridgev2/status"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/connector/linkedinfmt"
	"go.mau.fi/mautrix-linkedin/pkg/connector/matrixfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

type LinkedInClient struct {
	main      *LinkedInConnector
	userID    networkid.UserID
	userLogin *bridgev2.UserLogin
	client    *linkedingo.Client

	firstConnection bool

	linkedinFmtParams linkedinfmt.FormatParams
	matrixParser      *matrixfmt.HTMLParser
}

var (
	_ bridgev2.NetworkAPI                    = (*LinkedInClient)(nil)
	_ bridgev2.EditHandlingNetworkAPI        = (*LinkedInClient)(nil)
	_ bridgev2.ReactionHandlingNetworkAPI    = (*LinkedInClient)(nil)
	_ bridgev2.RedactionHandlingNetworkAPI   = (*LinkedInClient)(nil)
	_ bridgev2.ReadReceiptHandlingNetworkAPI = (*LinkedInClient)(nil)
	_ bridgev2.TypingHandlingNetworkAPI      = (*LinkedInClient)(nil)
	_ bridgev2.BackfillingNetworkAPI         = (*LinkedInClient)(nil)
	// _ bridgev2.IdentifierResolvingNetworkAPI   = (*LinkedInClient)(nil)
	// _ bridgev2.ContactListingNetworkAPI        = (*LinkedInClient)(nil)
	// _ bridgev2.UserSearchingNetworkAPI         = (*LinkedInClient)(nil)
	// _ bridgev2.GroupCreatingNetworkAPI         = (*LinkedInClient)(nil)
	// _ bridgev2.MuteHandlingNetworkAPI          = (*LinkedInClient)(nil)
	// _ bridgev2.TagHandlingNetworkAPI           = (*LinkedInClient)(nil)
)

func NewLinkedInClient(ctx context.Context, lc *LinkedInConnector, login *bridgev2.UserLogin) *LinkedInClient {
	userID := networkid.UserID(login.ID)
	client := &LinkedInClient{
		main:            lc,
		userID:          userID,
		userLogin:       login,
		firstConnection: true,
	}
	client.client = linkedingo.NewClient(
		ctx,
		linkedingo.NewURN(login.ID),
		login.Metadata.(*UserLoginMetadata).Cookies,
		login.Metadata.(*UserLoginMetadata).XLIPageInstance,
		login.Metadata.(*UserLoginMetadata).XLITrack,
		linkedingo.Handlers{
			Heartbeat: func(ctx context.Context) {
				login.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
			},
			ClientConnection: func(context.Context, *linkedingo.ClientConnection) {
				login.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})

				if client.firstConnection {
					go client.syncConversations(ctx)
					client.firstConnection = false
				}
			},
			TransientDisconnect: client.onTransientDisconnect,
			BadCredentials:      client.onBadCredentials,
			UnknownError:        client.onUnknownError,
			DecoratedEvent:      client.onDecoratedEvent,
		},
	)

	client.linkedinFmtParams = linkedinfmt.FormatParams{
		GetMXIDByURN: func(ctx context.Context, entityURN linkedingo.URN) (id.UserID, error) {
			ghost, err := lc.Bridge.GetGhostByID(ctx, networkid.UserID(entityURN.ID()))
			if err != nil {
				return "", err
			}
			// FIXME this should look for user logins by ID, not hardcode the current user
			if networkid.UserID(entityURN.ID()) == client.userID {
				return client.userLogin.UserMXID, nil
			}
			return ghost.Intent.GetMXID(), nil
		},
	}
	client.matrixParser = &matrixfmt.HTMLParser{
		GetGhostDetails: func(ctx context.Context, ui id.UserID) (networkid.UserID, string, bool) {
			if ui == client.userLogin.UserMXID {
				return client.userID, client.userLogin.RemoteName, true
			}
			if userID, ok := lc.Bridge.Matrix.ParseGhostMXID(ui); !ok {
				return "", "", false
			} else if ghost, err := lc.Bridge.DB.Ghost.GetByID(ctx, userID); err != nil {
				return "", "", false
			} else {
				return userID, ghost.Name, true
			}
		},
	}
	return client
}

func (l *LinkedInClient) Connect(ctx context.Context) {
	if !l.IsLoggedIn() {
		zerolog.Ctx(ctx).Warn().Msg("user is not logged in, sending bad credentials state")
		l.userLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "linkedin-no-auth",
			Message:    "User does not have the necessary cookies",
		})
		return
	}

	if err := l.client.RealtimeConnect(ctx); err != nil {
		l.userLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "linkedin-logged-out",
			Message:    fmt.Sprintf("Failed to connect to the realtime stream: %v", err),
		})
	}
}

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

func (l *LinkedInClient) onRealtimeMessage(ctx context.Context, msg linkedingo.Message) {
	log := zerolog.Ctx(ctx)
	meta := simplevent.EventMeta{
		LogContext: func(c zerolog.Context) zerolog.Context {
			return c.
				Stringer("entity_urn", msg.EntityURN).
				Stringer("sender", msg.Sender.EntityURN)
		},
		PortalKey:    l.makePortalKey(msg.Conversation),
		CreatePortal: true,
		Sender:       l.makeSender(msg.Sender),
		Timestamp:    msg.DeliveredAt.Time,
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

func (l *LinkedInClient) getAvatar(img *linkedingo.VectorImage) (avatar *bridgev2.Avatar) {
	if img == nil {
		return nil
	}
	return &bridgev2.Avatar{
		ID:     networkid.AvatarID(img.RootURL),
		Remove: img.RootURL == "",
		Get: func(ctx context.Context) ([]byte, error) {
			return l.client.DownloadBytes(ctx, img.GetLargestArtifactURL())
		},
	}
}

func (l *LinkedInClient) getMessagingParticipantUserInfo(participant linkedingo.MessagingParticipant) (ui bridgev2.UserInfo) {
	switch {
	case participant.ParticipantType.Member != nil:
		ui.Name = ptr.Ptr(l.main.Config.FormatDisplayname(DisplaynameParams{
			FirstName: participant.ParticipantType.Member.FirstName.Text,
			LastName:  participant.ParticipantType.Member.LastName.Text,
		}))
		ui.Avatar = l.getAvatar(participant.ParticipantType.Member.ProfilePicture)
		ui.Identifiers = []string{fmt.Sprintf("linkedin:%s", participant.EntityURN.ID())}
	case participant.ParticipantType.Organization != nil:
		ui.Name = ptr.Ptr(l.main.Config.FormatDisplayname(DisplaynameParams{
			Organization: participant.ParticipantType.Organization.Name.Text,
		}))
		ui.Avatar = l.getAvatar(participant.ParticipantType.Organization.Logo)
		ui.Identifiers = []string{fmt.Sprintf("linkedin:%s", participant.EntityURN.ID())}
	}
	return
}

func (l *LinkedInClient) conversationToChatInfo(conv linkedingo.Conversation) (ci bridgev2.ChatInfo, userInChat bool) {
	if conv.Title != "" {
		ci.Name = &conv.Title
	}

	// TODO: topic is probably headlineText of the conversation, or set it to the headline of the other user in the chat

	ci.Type = ptr.Ptr(database.RoomTypeDM)
	if conv.GroupChat {
		ci.Type = ptr.Ptr(database.RoomTypeGroupDM)
	}

	ci.CanBackfill = true

	ci.Members = &bridgev2.ChatMemberList{
		IsFull:           true,
		TotalMemberCount: len(conv.ConversationParticipants),
		MemberMap:        map[networkid.UserID]bridgev2.ChatMember{},
	}
	for _, participant := range conv.ConversationParticipants {
		userInChat = userInChat || networkid.UserID(participant.EntityURN.ID()) == l.userID
		sender := l.makeSender(participant)
		ci.Members.MemberMap[sender.Sender] = bridgev2.ChatMember{
			EventSender: sender,
			Membership:  event.MembershipJoin,
			UserInfo:    ptr.Ptr(l.getMessagingParticipantUserInfo(participant)),
		}
	}

	return
}

func (l *LinkedInClient) Disconnect() {
	l.client.RealtimeDisconnect()
}

func (l *LinkedInClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	// This is not supported. All of the info should already be populated with
	// the information we get on a per-message basis.
	zerolog.Ctx(ctx).Warn().Msg("GetChatInfo called")
	return nil, nil
}

func (l *LinkedInClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	// This is not supported. All of the info should already be populated with
	// the information we get on a per-message basis.
	zerolog.Ctx(ctx).Warn().Msg("GetUserInfo called")
	return nil, nil
}

func (l *LinkedInClient) IsLoggedIn() bool {
	return l.userLogin.Metadata.(*UserLoginMetadata).Cookies.GetCookie(linkedingo.LinkedInCookieJSESSIONID) != ""
}

func (l *LinkedInClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	return l.userID == userID
}

func (l *LinkedInClient) LogoutRemote(ctx context.Context) {
	if err := l.client.Logout(ctx); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("error logging out of remote")
	}
}
