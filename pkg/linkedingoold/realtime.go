package linkedingoold

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/eventold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/eventold/rawold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/responseold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"

	"github.com/google/uuid"
)

type RealtimeClient struct {
	client    *Client
	http      *http.Client
	sessionID string
}

func (c *Client) newRealtimeClient() *RealtimeClient {
	return &RealtimeClient{
		client: c,
		http: &http.Client{
			Transport: &http.Transport{
				Proxy: c.httpProxy,
			},
		},
		sessionID: uuid.NewString(),
	}
}

func (rc *RealtimeClient) ProcessEvents(data map[typesold.RealtimeEvent]json.RawMessage) {
	for eventType, eventDataBytes := range data {
		switch eventType {
		case typesold.RealtimeEventDecoratedEvent:
			var decoratedEventResponse rawold.DecoratedEventResponse
			err := json.Unmarshal(eventDataBytes, &decoratedEventResponse)
			if err != nil {
				log.Fatalf("failed to unmarshal eventold bytes with type %s into rawold.DecoratedEventResponse", eventType)
			}
			log.Println(string(eventDataBytes))
			rc.ProcessDecoratedEvent(decoratedEventResponse)
		case typesold.RealtimeEventHeartbeat:
			log.Println("received heartbeat")
		case typesold.RealtimeEventClientConnection:
			if rc.client.eventHandler != nil {
				rc.client.eventHandler(eventold.ConnectionReady{})
			}
		default:
			rc.client.Logger.Warn().Str("json_data", string(eventDataBytes)).Str("eventold_type", string(eventType)).Msg("Received unknown eventold")
		}
	}
}

func (rc *RealtimeClient) ProcessDecoratedEvent(data rawold.DecoratedEventResponse) {
	var evtData any
	topic, topicChunks := parseRealtimeTopic(data.Topic)
	switch topic {
	case typesold.RealtimeEventTopicMessages:
		renderFormat := data.Payload.Data.DecoratedMessage.Result.MessageBodyRenderFormat
		switch renderFormat {
		case responseold.RenderFormatDefault:
			evtData = data.Payload.Data.ToMessageEvent()
		case responseold.RenderFormatEdited:
			evtData = data.Payload.Data.ToMessageEditedEvent()
		case responseold.RenderFormatReCalled:
			evtData = data.Payload.Data.ToMessageDeleteEvent()
		case responseold.RenderFormatSystem:
			evtData = data.Payload.Data.ToSystemMessageEvent()
		default:
			rc.client.Logger.Warn().Any("json_data", data.Payload).Str("format", string(renderFormat)).Msg("Received unknown message body render format")
		}
	case typesold.RealtimeEventTopicMessageReactionSummaries:
		evtData = data.Payload.Data.ToMessageReactionEvent()
	case typesold.RealtimeEventTopicTypingIndicators:
		evtData = data.Payload.Data.ToTypingIndicatorEvent()
	case typesold.RealtimeEventTopicPresenceStatus:
		fsdProfileId := topicChunks[:-0]
		log.Println("presence updated for user id:", fsdProfileId)
		evtData = data.Payload.ToPresenceStatusUpdateEvent(fsdProfileId[0])
	case typesold.RealtimeEventTopicMessageSeenReceipts:
		evtData = data.Payload.Data.ToMessageSeenEvent()
	case typesold.RealtimeEventTopicConversations:
		evtData = data.Payload.Data.ToThreadUpdateEvent()
	case typesold.RealtimeEventTopicConversationsDelete:
		evtData = data.Payload.Data.ToThreadDeleteEvent()
	/* Ignored eventold topics */
	case typesold.RealtimeEventTopicJobPostingPersonal:
	case typesold.RealtimeEventTopicSocialPermissionsPersonal:
	case typesold.RealtimeEventTopicMessagingProgressIndicator:
	case typesold.RealtimeEventTopicMessagingDataSync:
	case typesold.RealtimeEventTopicInvitations:
	case typesold.RealtimeEventTopicInAppAlerts:
	case typesold.RealtimeEventTopicReplySuggestionV2:
	case typesold.RealtimeEventTopicTabBadgeUpdate:
		break
	default:
		rc.client.Logger.Warn().Any("json_data", data.Payload).Str("eventold_topic", string(data.Topic)).Msg("Received unknown eventold topic")
	}

	if evtData != nil {
		rc.client.eventHandler(evtData)
	}
}

func parseRealtimeTopic(topic string) (typesold.RealtimeEventTopic, []string) {
	topicChunks := strings.Split(topic, ":")
	return typesold.RealtimeEventTopic(topicChunks[2]), topicChunks
}
