package linkedingo

import (
	"context"
	"encoding/json"
	"net/http"
)

// https://www.linkedin.com/voyager/api/voyagerMessagingGraphQL/graphql?queryId=messengerConversations.7b27164c5517548167d9adb4ba603e55&variables=(mailboxUrn:urn%3Ali%3Afsd_profile%3AACoAADZsHU0BD7Cr7MwzvkzsAcCoeOii7kl0mPU)

// https://www.linkedin.com/voyager/api/voyagerMessagingGraphQL/graphql?queryId=messengerConversations.8656fb361a8ad0c178e8d3ff1a84ce26&variables=(query:(predicateUnions:List((conversationCategoryPredicate:(category:PRIMARY_INBOX)))),count:20,mailboxUrn:urn%3Ali%3Afsd_profile%3AACoAADZsHU0BD7Cr7MwzvkzsAcCoeOii7kl0mPU,lastUpdatedBefore:1739209141023)

// curl 'https://www.linkedin.com/voyager/api/voyagerMessagingGraphQL/graphql?queryId=messengerConversations.277103fa0741e804ec5f21e6f64cb598&variables=(mailboxUrn:urn%3Ali%3Afsd_profile%3AACoAADZsHU0BD7Cr7MwzvkzsAcCoeOii7kl0mPU,syncToken:-trA4KJljszB4KJlLnVybjpsaTpmYWJyaWM6cHJvZC1sb3IxAA%3D%3D)' \

// type GetThreadsVariables struct {
// 	InboxCategory     InboxCategory `graphql:"category"`
// 	Count             int64         `graphql:"count"`
// 	MailboxUrn        string        `graphql:"mailboxUrn"`
// 	LastUpdatedBefore int64         `graphql:"lastUpdatedBefore"`
// 	NextCursor        string        `graphql:"nextCursor"`
// 	SyncToken         string        `graphql:"syncToken"`
// }

type GraphQlResponse[T any] struct {
	Data GraphQLData[T] `json:"data,omitempty"`
}

type GraphQLData[T any] struct {
	MessengerConversationsBySyncToken *CollectionResponse[T] `json:"messengerConversationsBySyncToken,omitempty"`
}

// CollectionResponse represents a
// com.linkedin.restli.common.CollectionResponse object.
type CollectionResponse[T any] struct {
	Metadata SyncMetadata `json:"metadata,omitempty"`
	Elements []T          `json:"elements,omitempty"`
}

// SyncMetadata represents a com.linkedin.messenger.SyncMetadata object.
type SyncMetadata struct {
	NewSyncToken string `json:"newSyncToken,omitempty"`
}

// Conversation represents a com.linkedin.messenger.Conversation object
type Conversation struct {
	Title                    string                 `json:"title,omitempty"`
	EntityURN                URN                    `json:"entityUrn,omitempty"`
	GroupChat                bool                   `json:"groupChat,omitempty"`
	ConversationParticipants []MessagingParticipant `json:"conversationParticipants,omitempty"`
	Read                     bool                   `json:"read,omitempty"`
	// Messages                 []CollectionResponse[Message] `json:"messages,omitempty"`
}

// MessagingParticipant represents a
// com.linkedin.messenger.MessagingParticipant object.
type MessagingParticipant struct {
	ParticipantType ParticipantType `json:"participantType,omitempty"`
	EntityURN       URN             `json:"entityUrn,omitempty"`
}

type ParticipantType struct {
	Member MemberParticipantInfo `json:"member,omitempty"`
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

func (c *Client) GetConversations(ctx context.Context) (*CollectionResponse[Conversation], error) {
	variables := map[string]string{
		"mailboxUrn": c.userEntityURN.WithPrefix("urn", "li", "fsd_profile").String(),
	}

	// withCursor := variables.LastUpdatedBefore != 0 && variables.NextCursor != ""

	queryId := graphQLQueryIDMessengerConversations
	// if withCursor {
	// 	queryId = graphQLQueryIDMessengerConversationsWithCursor
	// } else if variables.SyncToken != "" {
	// 	queryId = graphQLQueryIDMessengerConversationsWithSyncToken
	// }

	resp, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerMessagingGraphQLURL).
		WithGraphQLQuery(queryId, variables).
		WithCSRF().
		WithXLIHeaders().
		WithHeader("accept", contentTypeGraphQL).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	var response GraphQlResponse[Conversation]
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Data.MessengerConversationsBySyncToken, nil
}

// func (c *Client) GetThreads(variables queryold.GetThreadsVariables) (*responseold.MessengerConversationsResponse, error) {
// 	if variables.MailboxUrn == "" {
// 		variables.MailboxUrn = c.PageLoader.CurrentUser.FsdProfileID
// 	}
//
// 	withCursor := variables.LastUpdatedBefore != 0 && variables.NextCursor != ""
// 	var queryId typesold.GraphQLQueryIDs
// 	if withCursor {
// 		queryId = typesold.GraphQLQueryIDMessengerConversationsWithCursor
// 	} else if variables.SyncToken != "" {
// 		queryId = typesold.GraphQLQueryIDMessengerConversationsWithSyncToken
// 	} else {
// 		queryId = typesold.GraphQLQueryIDMessengerConversations
// 	}
//
// 	variablesQuery, err := variables.Encode()
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	threadQuery := queryold.GraphQLQuery{
// 		QueryID:   queryId,
// 		Variables: string(variablesQuery),
// 	}
//
// 	_, respData, err := c.MakeRoutingRequest(routingold.LinkedInVoyagerMessagingGraphQLURL, nil, &threadQuery)
// 	if err != nil {
// 		return nil, err
// 	}
// 	fmt.Printf("%s\n", respData)
//
// 	graphQLResponse, ok := respData.(*responseold.GraphQlResponse)
// 	if !ok || graphQLResponse == nil {
// 		return nil, newErrorResponseTypeAssertFailed("*responseold.GraphQlResponse")
// 	}
//
// 	graphQLResponseData := graphQLResponse.Data
// 	fmt.Printf("%+v\n", graphQLResponseData)
// 	if withCursor {
// 		return graphQLResponseData.MessengerConversationsByCategory, nil
// 	}
//
// 	return graphQLResponseData.MessengerConversationsBySyncToken, nil
// }
