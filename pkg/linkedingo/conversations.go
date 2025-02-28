package linkedingo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
)

type GraphQlResponse struct {
	Data GraphQLData `json:"data,omitempty"`
}

type GraphQLData struct {
	MessengerConversationsByCategoryQuery           *CollectionResponse[ConversationCursorMetadata, Conversation] `json:"messengerConversationsByCategoryQuery,omitempty"`
	MessengerMessagesByAnchorTimestamp              *CollectionResponse[MessageMetadata, Message]                 `json:"messengerMessagesByAnchorTimestamp,omitempty"`
	MessengerMessagesByConversation                 *CollectionResponse[MessageMetadata, Message]                 `json:"messengerMessagesByConversation,omitempty"`
	MessengerMessagingParticipantsByMessageAndEmoji *CollectionResponse[any, MessagingParticipant]                `json:"messengerMessagingParticipantsByMessageAndEmoji,omitempty"`
}

// CollectionResponse represents a
// com.linkedin.restli.common.CollectionResponse object.
type CollectionResponse[M, T any] struct {
	Metadata M   `json:"metadata,omitempty"`
	Elements []T `json:"elements,omitempty"`
}

// ConversationCursorMetadata represents a com.linkedin.messenger.ConversationCursorMetadata object.
type ConversationCursorMetadata struct {
	NextCursor string `json:"nextCursor,omitempty"`
}

// MessageMetadata represents a com.linkedin.messenger.MessageMetadata object.
type MessageMetadata struct {
	NextCursor string `json:"nextCursor,omitempty"`
	PrevCursor string `json:"prevCursor,omitempty"`
}

// Conversation represents a com.linkedin.messenger.Conversation object
type Conversation struct {
	Title                    string                           `json:"title,omitempty"`
	EntityURN                URN                              `json:"entityUrn,omitempty"`
	LastActivityAt           jsontime.UnixMilli               `json:"lastActivityAt,omitempty"`
	GroupChat                bool                             `json:"groupChat,omitempty"`
	ConversationParticipants []MessagingParticipant           `json:"conversationParticipants,omitempty"`
	Read                     bool                             `json:"read,omitempty"`
	Messages                 CollectionResponse[any, Message] `json:"messages,omitempty"`
}

// MessagingParticipant represents a
// com.linkedin.messenger.MessagingParticipant object.
type MessagingParticipant struct {
	ParticipantType ParticipantType `json:"participantType,omitempty"`
	EntityURN       URN             `json:"entityUrn,omitempty"`
}

type ParticipantType struct {
	Member       *MemberParticipantInfo       `json:"member,omitempty"`
	Organization *OrganizationParticipantInfo `json:"organization,omitempty"`
}

// MemberParticipantInfo represents a
// com.linkedin.messenger.MemberParticipantInfo object.
type MemberParticipantInfo struct {
	ProfileURL     string         `json:"profileUrl,omitempty"`
	FirstName      AttributedText `json:"firstName,omitempty"`
	LastName       AttributedText `json:"lastName,omitempty"`
	ProfilePicture *VectorImage   `json:"profilePicture,omitempty"`
	Pronoun        any            `json:"pronoun,omitempty"`
	Headline       AttributedText `json:"headline,omitempty"`
}

// OrganizationParticipantInfo represents a
// com.linkedin.messenger.OrganizationParticipantInfo object.
type OrganizationParticipantInfo struct {
	Name    AttributedText `json:"name,omitempty"`
	Logo    *VectorImage   `json:"logo,omitempty"`
	PageURL string         `json:"pageUrl,omitempty"`
}

func (c *Client) GetConversationsUpdatedBefore(ctx context.Context, updatedBefore time.Time) (*CollectionResponse[ConversationCursorMetadata, Conversation], error) {
	zerolog.Ctx(ctx).Info().
		Time("updated_before", updatedBefore).
		Msg("Getting conversations updated before")
	resp, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
		WithGraphQLQuery(graphQLQueryIDMessengerConversationsWithCursor, map[string]string{
			"mailboxUrn":        url.QueryEscape(c.userEntityURN.WithPrefix("urn", "li", "fsd_profile").String()),
			"lastUpdatedBefore": strconv.Itoa(int(updatedBefore.UnixMilli())),
			"count":             "20",
			"query":             "(predicateUnions:List((conversationCategoryPredicate:(category:PRIMARY_INBOX))))",
		}).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	var response GraphQlResponse
	return response.Data.MessengerConversationsByCategoryQuery, json.NewDecoder(resp.Body).Decode(&response)
}
