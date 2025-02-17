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
	linkedInVoyagerMessagingGraphQLURL               = "https://www.linkedin.com/voyager/api/voyagerMessagingGraphQL/graphql"
	linkedInLogoutURL                                = "https://www.linkedin.com/uas/logout"
	linkedInMessagingBaseURL                         = "https://www.linkedin.com/messaging"
	linkedInMessagingDashMessengerConversationsURL   = "https://www.linkedin.com/voyager/api/voyagerMessagingDashMessengerConversations"
	linkedInRealtimeConnectURL                       = "https://www.linkedin.com/realtime/connect?rc=1"
	linkedInRealtimeHeartbeatURL                     = "https://www.linkedin.com/realtime/realtimeFrontendClientConnectivityTracking?action=sendHeartbeat"
	linkedInVoyagerCommonMeURL                       = "https://www.linkedin.com/voyager/api/me"
	linkedInVoyagerMediaUploadMetadataURL            = "https://www.linkedin.com/voyager/api/voyagerVideoDashMediaUploadMetadata"
	linkedInVoyagerMessagingDashMessengerMessagesURL = "https://www.linkedin.com/voyager/api/voyagerMessagingDashMessengerMessages"
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
	RealtimeEventTopicConversationsDelete        = "conversationDeletesTopic"
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
	graphQLQueryIDMessengerConversations              = "messengerConversations.95e0a4b80fbc6bc53550e670d34d05d9"
	graphQLQueryIDMessengerConversationsWithCursor    = "messengerConversations.18240d6a3ac199067a703996eeb4b163"
	graphQLQueryIDMessengerConversationsWithSyncToken = "messengerConversations.be2479ed77df3dd407dd90efc8ac41de"
	graphQLQueryIDMessengerMessagesBySyncToken        = "messengerMessages.d1b494ac18c24c8be71ea07b5bd1f831"
	graphQLQueryIDMessengerMessagesByAnchorTimestamp  = "messengerMessages.b52340f92136e74c2aab21dac7cf7ff2"
	graphQLQueryIDMessengerMessagesByConversation     = "messengerMessages.86ca573adc64110d94d8bce89c5b2f3b"
)
