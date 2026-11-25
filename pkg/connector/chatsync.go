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
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) deleteConversation(ctx context.Context, conv linkedingo.Conversation) {
	l.deletePortal(ctx, l.makePortalKey(conv))
}

func (l *LinkedInClient) deletePortal(ctx context.Context, portalKey networkid.PortalKey) {
	l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatDelete{
		EventMeta: simplevent.EventMeta{
			Type:      bridgev2.RemoteEventChatDelete,
			PortalKey: portalKey,
		},
		OnlyForMe: true,
	})
}

func (l *LinkedInClient) deleteURN(ctx context.Context, urn linkedingo.URN) {
	portalKey := networkid.PortalKey{
		ID:       networkid.PortalID(urn.String()),
		Receiver: l.userLogin.ID,
	}
	l.deletePortal(ctx, portalKey)
	if !l.main.Bridge.Config.SplitPortals {
		portalKey.Receiver = ""
		l.deletePortal(ctx, portalKey)
	}
}

func (l *LinkedInClient) handleConversations(ctx context.Context, convs []linkedingo.Conversation, synced *int, handled *[]<-chan struct{}) (limitReached bool) {
	log := zerolog.Ctx(ctx)

	for _, conv := range convs {
		if slices.Contains(conv.Categories, "SPAM") {
			l.deleteConversation(ctx, conv)
			continue
		}
		inInbox := slices.Contains(conv.Categories, "INBOX")
		if !inInbox && (l.main.Config.ArchiveTag == "" || !slices.Contains(conv.Categories, "ARCHIVE")) {
			continue
		}

		isMember := false
		for _, participant := range conv.ConversationParticipants {
			if participant.EntityURN.ID() == string(l.userID) {
				isMember = true
				break
			}
		}
		if !isMember {
			l.deleteConversation(ctx, conv)
			continue
		}

		log := log.With().
			Stringer("conversation_urn", conv.EntityURN).
			Time("last_activity_at", conv.LastActivityAt.Time).
			Logger()

		l.conversationStateLock.Lock()
		l.conversationLastRead[conv.EntityURN] = conv.LastReadAt
		lastReadState, ok := l.conversationReadState[conv.EntityURN]
		readStatusChanged := ok && lastReadState.Read != conv.Read
		l.conversationReadState[conv.EntityURN] = ConversationReadState{
			LastReadAt: conv.LastReadAt,
			Read:       conv.Read,
		}
		l.conversationStateLock.Unlock()

		portalKey := l.makePortalKey(conv)
		meta := simplevent.EventMeta{
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.Str("update", "sync")
			},
			PortalKey:    portalKey,
			CreatePortal: inInbox && (l.main.Config.Sync.CreateLimit == 0 || *synced < l.main.Config.Sync.CreateLimit),
		}
		if inInbox {
			*synced++
		}

		var latestMessageTS time.Time
		var latestMessageID networkid.MessageID
		for _, msg := range conv.Messages.Elements {
			if msg.DeliveredAt.After(latestMessageTS) {
				latestMessageTS = msg.DeliveredAt.Time
				latestMessageID = msg.MessageID()
			}
		}
		chatInfo, userInChat := l.conversationToChatInfo(conv)
		if !userInChat {
			log.Debug().Msg("User not in chat")
			continue
		}
		var handledCh chan struct{}
		if handled != nil {
			waitForHandle := meta.CreatePortal && l.main.Bridge.Config.PortalCreateFilter.ShouldAllow(l.userLogin.ID, portalKey)
			if !waitForHandle {
				portal, err := l.main.Bridge.GetExistingPortalByKey(ctx, portalKey)
				waitForHandle = err == nil && portal != nil && portal.MXID != ""
			}
			if waitForHandle {
				handledCh = make(chan struct{})
				closeHandled := sync.OnceFunc(func() { close(handledCh) })
				meta.PostHandleFunc = func(context.Context, *bridgev2.Portal) {
					closeHandled()
				}
			}
		}
		res := l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.ChatResync{
			ChatInfo:        &chatInfo,
			EventMeta:       meta.WithType(bridgev2.RemoteEventChatResync),
			LatestMessageTS: latestMessageTS,
			CheckNeedsBackfillFunc: func(_ context.Context, latestMessage *database.Message) (bool, error) {
				if latestMessage == nil {
					return !latestMessageTS.IsZero(), nil
				}
				return latestMessage.ID != latestMessageID && latestMessageTS.After(latestMessage.Timestamp), nil
			},
		})
		if handledCh != nil && res.Queued {
			*handled = append(*handled, handledCh)
		}
		if readStatusChanged {
			sender := bridgev2.EventSender{
				IsFromMe:    true,
				Sender:      networkid.UserID(l.userLogin.ID),
				SenderLogin: l.userLogin.ID,
			}
			if conv.Read {
				l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.Receipt{
					EventMeta: simplevent.EventMeta{
						Type:      bridgev2.RemoteEventReadReceipt,
						PortalKey: portalKey,
						Sender:    sender,
						Timestamp: conv.LastReadAt.Time,
					},
					LastTarget: latestMessageID,
					ReadUpTo:   conv.LastReadAt.Time,
				})
			} else {
				l.main.Bridge.QueueRemoteEvent(l.userLogin, &simplevent.MarkUnread{
					EventMeta: simplevent.EventMeta{
						Type:      bridgev2.RemoteEventMarkUnread,
						PortalKey: portalKey,
						Sender:    sender,
					},
					Unread: true,
				})
			}
		}

		if l.main.Config.Sync.UpdateLimit > 0 && *synced >= l.main.Config.Sync.UpdateLimit {
			log.Info().Msg("Update limit reached")
			return true
		}
	}
	return false
}

func (l *LinkedInClient) syncConversations(ctx context.Context) {
	l.syncConversationsLock.Lock()
	defer l.syncConversationsLock.Unlock()
	meta := l.userLogin.Metadata.(*UserLoginMetadata)
	log := zerolog.Ctx(ctx).With().
		Str("action", "sync_conversations").
		Time("synced_up_to", meta.ConversationsSyncedUpTo.Time).
		Logger()
	log.Info().Msg("starting conversation sync")

	syncStart := time.Now()
	oldestActivity := syncStart
	partialStart := meta.PartialSyncStart.Time
	partialOldest := meta.PartialSyncOldest.Time
	var cursor string
	seenCursors := map[string]struct{}{}
	var synced int
	pagesHandled := true
	for {
		if ctx.Err() != nil {
			log.Info().Msg("sync canceled")
			return
		}

		log := log.With().
			Time("oldest_activity", oldestActivity).
			Bool("has_cursor", cursor != "").
			Logger()
		log.Info().Msg("fetching conversations")

		conversations, err := l.getConversationsPage(ctx, log, cursor)
		if err != nil {
			log.Err(err).Msg("failed to fetch conversations")
			return
		} else if conversations == nil {
			log.Warn().Msg("no conversations found")
			return
		}

		for _, conv := range conversations.Elements {
			if !conv.LastActivityAt.IsZero() && conv.LastActivityAt.Before(oldestActivity) {
				oldestActivity = conv.LastActivityAt.Time
			}
		}
		limitReached := false
		var handled []<-chan struct{}
		for i, conv := range conversations.Elements {
			covered := !partialStart.IsZero() && !conv.LastActivityAt.IsZero() &&
				!conv.LastActivityAt.Before(partialOldest) &&
				conv.LastActivityAt.Before(partialStart)
			if !covered {
				limitReached = l.handleConversations(ctx, conversations.Elements[i:i+1], &synced, &handled)
			} else if slices.Contains(conv.Categories, "INBOX") && !slices.Contains(conv.Categories, "SPAM") &&
				slices.ContainsFunc(conv.ConversationParticipants, func(p linkedingo.MessagingParticipant) bool {
					return p.EntityURN.ID() == string(l.userID)
				}) {
				synced++
				limitReached = l.main.Config.Sync.UpdateLimit > 0 && synced >= l.main.Config.Sync.UpdateLimit
			}
			if limitReached {
				break
			}
		}
		if pagesHandled && !waitForConversationsHandled(ctx, log, handled) {
			pagesHandled = false
		}
		if limitReached {
			break
		} else if oldestActivity.Before(meta.ConversationsSyncedUpTo.Time) {
			log.Info().Msg("reached conversations synced previously")
			break
		} else if conversations.Metadata.NextCursor == "" {
			log.Info().Msg("no more conversations found")
			break
		}
		if _, seen := seenCursors[conversations.Metadata.NextCursor]; seen || len(conversations.Elements) == 0 {
			log.Warn().Int("page_size", len(conversations.Elements)).Msg("conversation paging stopped advancing")
			return
		}
		cursor = conversations.Metadata.NextCursor
		seenCursors[cursor] = struct{}{}

		if pagesHandled && (partialStart.IsZero() || !oldestActivity.After(partialStart)) {
			meta.PartialSyncStart = jsontime.UM(syncStart)
			meta.PartialSyncOldest = jsontime.UM(oldestActivity)
			if !partialStart.IsZero() && partialOldest.Before(oldestActivity) {
				meta.PartialSyncOldest = jsontime.UM(partialOldest)
			}
			if err := l.userLogin.Save(ctx); err != nil {
				log.Err(err).Msg("failed to save partial conversation sync progress")
			}
		}
	}
	if ctx.Err() != nil {
		return
	} else if !pagesHandled {
		log.Warn().Msg("Not saving conversation sync boundary because some synced conversations weren't handled")
		return
	}
	meta.ConversationsSyncedUpTo = jsontime.UM(syncStart)
	meta.PartialSyncStart = jsontime.UnixMilli{}
	meta.PartialSyncOldest = jsontime.UnixMilli{}
	if err := l.userLogin.Save(ctx); err != nil {
		log.Err(err).Msg("failed to save conversation sync timestamp")
	}
}

const (
	conversationPageHandleTimeout = 5 * time.Minute
	conversationPageIdleTimeout   = 3 * time.Minute
)

func waitForConversationsHandled(ctx context.Context, log zerolog.Logger, handled []<-chan struct{}) bool {
	if len(handled) == 0 {
		return true
	}
	stop := make(chan struct{})
	defer close(stop)
	done := make(chan struct{}, len(handled))
	for _, ch := range handled {
		go func() {
			select {
			case <-ch:
				done <- struct{}{}
			case <-stop:
			}
		}()
	}
	deadline := time.NewTimer(conversationPageHandleTimeout)
	defer deadline.Stop()
	idle := time.NewTimer(conversationPageIdleTimeout)
	defer idle.Stop()
	for pending := len(handled); pending > 0; {
		select {
		case <-done:
			pending--
			idle.Reset(conversationPageIdleTimeout)
		case <-ctx.Done():
			return false
		case <-idle.C:
			log.Warn().
				Int("pending", pending).
				Msg("No synced conversation was handled recently, continuing sync")
			return false
		case <-deadline.C:
			log.Warn().
				Int("pending", pending).
				Msg("Timed out waiting for synced conversations to be handled")
			return false
		}
	}
	return true
}

const conversationPageRateLimitRetries = 5

func (l *LinkedInClient) getConversationsPage(ctx context.Context, log zerolog.Logger, cursor string) (*linkedingo.CollectionResponse[linkedingo.ConversationCursorMetadata, linkedingo.Conversation], error) {
	retryIn := time.Minute
	for attempt := 0; ; attempt++ {
		conversations, err := l.client.GetConversationsPage(ctx, cursor)
		if err == nil || attempt >= conversationPageRateLimitRetries || responseStatusCode(err) != http.StatusTooManyRequests {
			return conversations, err
		}
		log.Warn().Err(err).Dur("retry_in", retryIn).Msg("rate limited while fetching conversations, retrying")
		select {
		case <-time.After(retryIn):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		retryIn *= 2
	}
}

func (l *LinkedInClient) getConversationsBySyncToken(ctx context.Context) {
	convs, err := l.client.GetConversationsBySyncToken(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("failed to get conversations by sync token")
		return
	}
	if convs == nil {
		return
	}
	l.handleConversations(ctx, convs.Elements, new(int), nil)
	for _, item := range convs.Metadata.DeletedURNs {
		l.deleteURN(ctx, item.Conversation.EntityURN)
	}
}
