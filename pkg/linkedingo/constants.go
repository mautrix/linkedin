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
	linkedInMessagingBaseURL                         = "https://www.linkedin.com/messaging"
	linkedInVoyagerCommonMeURL                       = "https://www.linkedin.com/voyager/api/me"
	linkedInRealtimeConnectURL                       = "https://www.linkedin.com/realtime/connect?rc=1"
	linkedInRealtimeHeartbeatURL                     = "https://www.linkedin.com/realtime/realtimeFrontendClientConnectivityTracking?action=sendHeartbeat"
	linkedInLogoutURL                                = "https://www.linkedin.com/uas/logout"
	linkedInVoyagerMessagingDashMessengerMessagesURL = "https://www.linkedin.com/voyager/api/voyagerMessagingDashMessengerMessages"
	linkedInVoyagerMediaUploadMetadataURL            = "https://www.linkedin.com/voyager/api/voyagerVideoDashMediaUploadMetadata"
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
