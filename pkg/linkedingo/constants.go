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

package linkedingo

const (
	linkedInVoyagerGraphQLURL                        = "https://www.linkedin.com/voyager/api/graphql"
	linkedInVoyagerMessagingGraphQLURL               = "https://www.linkedin.com/voyager/api/voyagerMessagingGraphQL/graphql"
	linkedInLogoutURL                                = "https://www.linkedin.com/uas/logout"
	linkedInMessagingBaseURL                         = "https://www.linkedin.com/messaging"
	linkedInMessagingDashMessengerConversationsURL   = "https://www.linkedin.com/voyager/api/voyagerMessagingDashMessengerConversations"
	linkedInRealtimeConnectURL                       = "https://www.linkedin.com/realtime/connect"
	linkedInRealtimeHeartbeatURL                     = "https://www.linkedin.com/realtime/realtimeFrontendClientConnectivityTracking"
	linkedInVoyagerCommonMeURL                       = "https://www.linkedin.com/voyager/api/me"
	linkedInVoyagerMediaUploadMetadataURL            = "https://www.linkedin.com/voyager/api/voyagerVideoDashMediaUploadMetadata"
	linkedInVoyagerMessagingDashMessengerMessagesURL = "https://www.linkedin.com/voyager/api/voyagerMessagingDashMessengerMessages"
	linkedInVoyagerNotificationsDashPushRegistration = "https://www.linkedin.com/voyager/api/voyagerNotificationsDashPushRegistration"
)

const LinkedInCookieJSESSIONID = "JSESSIONID"

const (
	contentTypeJSON                   = "application/json"
	contentTypeJSONPlaintextUTF8      = "application/json; charset=UTF-8"
	contentTypeJSONLinkedInNormalized = "application/vnd.linkedin.normalized+json+2.1"
	contentTypeGraphQL                = "application/graphql"
	contentTypeTextEventStream        = "text/event-stream"
	contentTypePlaintextUTF8          = "text/plain;charset=UTF-8"
)

const (
	RealtimeEventTopicConversations              = "conversationsTopic"
	RealtimeEventTopicConversationDelete         = "conversationDeletesTopic"
	RealtimeEventTopicMessageSeenReceipts        = "messageSeenReceiptsTopic"
	RealtimeEventTopicMessages                   = "messagesTopic"
	RealtimeEventTopicReplySuggestionV2          = "replySuggestionTopicV2"
	RealtimeEventTopicTabBadgeUpdate             = "tabBadgeUpdateTopic"
	RealtimeEventTopicTypingIndicators           = "typingIndicatorsTopic"
	RealtimeEventTopicInvitations                = "invitationsTopic"
	RealtimeEventTopicInAppAlerts                = "inAppAlertsTopic"
	RealtimeEventTopicMessageReactionSummaries   = "messageReactionSummariesTopic"
	RealtimeEventTopicSocialPermissionsPersonal  = "socialPermissionsPersonalTopic"
	RealtimeEventTopicJobPostingPersonal         = "jobPostingPersonalTopic"
	RealtimeEventTopicMessagingProgressIndicator = "messagingProgressIndicatorTopic"
	RealtimeEventTopicMessagingDataSync          = "messagingDataSyncTopic"
	RealtimeEventTopicPresenceStatus             = "presenceStatusTopic"
)

const (
	graphQLQueryIDMessengerConversations              = "messengerConversations.f0873b936b43ed663997b215b2c28359"
	graphQLQueryIDMessengerConversationsWithSyncToken = "messengerConversations.74c17e85611b60b7ba2700481151a316"
	graphQLQueryIDMessengerConversationsWithCursor    = "messengerConversations.8656fb361a8ad0c178e8d3ff1a84ce26"
	graphQLQueryIDMessengerMessagesByAnchorTimestamp  = "messengerMessages.4088d03bc70c91c3fa68965cb42336de"
	graphQLQueryIDMessengerMessagesByPrevCursor       = "messengerMessages.34c9888be71c8010fecfb575cb38308f"
	graphQLQueryIDVoyagerFeedDashUpdates              = "voyagerFeedDashUpdates.c2a318e55b634e20689c80e3dd11952e"
)
