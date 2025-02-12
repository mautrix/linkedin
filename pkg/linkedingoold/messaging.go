package linkedingoold

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/methodsold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/payloadold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/queryold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold/responseold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"

	"github.com/google/uuid"
)

// u dont have to pass mailboxUrn if u don't want to
// library will automatically set it for you
func (c *Client) GetThreads(variables queryold.GetThreadsVariables) (*responseold.MessengerConversationsResponse, error) {
	if variables.MailboxUrn == "" {
		variables.MailboxUrn = c.PageLoader.CurrentUser.FsdProfileID
	}

	withCursor := variables.LastUpdatedBefore != 0 && variables.NextCursor != ""
	var queryId typesold.GraphQLQueryIDs
	if withCursor {
		queryId = typesold.GraphQLQueryIDMessengerConversationsWithCursor
	} else if variables.SyncToken != "" {
		queryId = typesold.GraphQLQueryIDMessengerConversationsWithSyncToken
	} else {
		queryId = typesold.GraphQLQueryIDMessengerConversations
	}

	variablesQuery, err := variables.Encode()
	if err != nil {
		return nil, err
	}

	threadQuery := queryold.GraphQLQuery{
		QueryID:   queryId,
		Variables: string(variablesQuery),
	}

	_, respData, err := c.MakeRoutingRequest(routingold.LinkedInVoyagerMessagingGraphQLURL, nil, &threadQuery)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s\n", respData)

	graphQLResponse, ok := respData.(*responseold.GraphQlResponse)
	if !ok || graphQLResponse == nil {
		return nil, newErrorResponseTypeAssertFailed("*responseold.GraphQlResponse")
	}

	graphQLResponseData := graphQLResponse.Data
	fmt.Printf("%+v\n", graphQLResponseData)
	if withCursor {
		return graphQLResponseData.MessengerConversationsByCategory, nil
	}

	return graphQLResponseData.MessengerConversationsBySyncToken, nil
}

func (c *Client) FetchMessages(variables queryold.FetchMessagesVariables) (*responseold.MessengerMessagesResponse, error) {
	withCursor := variables.PrevCursor != ""
	withAnchorTimestamp := variables.DeliveredAt != 0

	var queryId typesold.GraphQLQueryIDs
	if withCursor {
		queryId = typesold.GraphQLQueryIDMessengerMessagesByConversation
	} else if withAnchorTimestamp {
		queryId = typesold.GraphQLQueryIDMessengerMessagesByAnchorTimestamp
	} else {
		queryId = typesold.GraphQLQueryIDMessengerMessagesBySyncToken
	}

	variablesQuery, err := variables.Encode()
	if err != nil {
		return nil, err
	}
	messagesQuery := queryold.GraphQLQuery{
		QueryID:   queryId,
		Variables: string(variablesQuery),
	}

	_, respData, err := c.MakeRoutingRequest(routingold.LinkedInVoyagerMessagingGraphQLURL, nil, &messagesQuery)
	if err != nil {
		return nil, err
	}

	graphQLResponse, ok := respData.(*responseold.GraphQlResponse)
	if !ok || graphQLResponse == nil {
		return nil, newErrorResponseTypeAssertFailed("*responseold.GraphQlResponse")
	}

	graphQLResponseData := graphQLResponse.Data
	if withCursor {
		return graphQLResponseData.MessengerMessagesByConversation, nil
	} else if withAnchorTimestamp {
		return graphQLResponseData.MessengerMessagesByAnchorTimestamp, nil
	}

	return graphQLResponseData.MessengerMessagesBySyncToken, nil
}

func (c *Client) EditMessage(messageUrn string, p payloadold.MessageBody) error {
	editMessageUrl := fmt.Sprintf("%s/%s", routingold.LinkedInVoyagerMessagingDashMessengerMessagesURL, url.QueryEscape(messageUrn))

	headerOpts := typesold.HeaderOpts{
		WithCookies:         true,
		WithCsrfToken:       true,
		Origin:              string(routingold.LinkedInBaseURL),
		WithXLiTrack:        true,
		WithXLiProtocolVer:  true,
		WithXLiPageInstance: true,
		WithXLiLang:         true,
		Extra:               map[string]string{"accept": string(typesold.ContentTypeJSON)},
	}
	headers := c.buildHeaders(headerOpts)

	editMessagePayload := payloadold.GraphQLPatchBody{
		Patch: payloadold.Patch{
			Set: payloadold.Set{
				Body: p,
			},
		},
	}

	payloadBytes, err := editMessagePayload.Encode()
	if err != nil {
		return err
	}

	resp, respBody, err := c.MakeRequest(editMessageUrl, http.MethodPost, headers, payloadBytes, typesold.ContentTypePlaintextUTF8)
	if err != nil {
		return err
	}

	if resp.StatusCode > 204 {
		return fmt.Errorf("failed to edit message with urn %s (statusCode=%d, response_body=%s)", messageUrn, resp.StatusCode, string(respBody))
	}

	return nil
}

// function will set mailboxUrn, originToken and trackingId automatically IF it is empty
// so you do not have to set it if u dont want to
func (c *Client) SendMessage(p payloadold.SendMessagePayload) (*responseold.MessageSentResponse, error) {
	actionQuery := queryold.DoActionQuery{
		Action: queryold.ActionCreateMessage,
	}

	if p.MailboxUrn == "" {
		p.MailboxUrn = c.PageLoader.CurrentUser.FsdProfileID
	}

	if p.TrackingID == "" {
		p.TrackingID = methodsold.GenerateTrackingId()
	}

	if p.Message.OriginToken == "" {
		p.Message.OriginToken = uuid.NewString()
	}

	resp, respData, err := c.MakeRoutingRequest(routingold.LinkedInVoyagerMessagingDashMessengerMessagesURL, p, actionQuery)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 204 {
		return nil, fmt.Errorf("failed to send message to conversation with urn %s (statusCode=%d)", p.Message.ConversationUrn, resp.StatusCode)
	}

	messageSentResponse, ok := respData.(*responseold.MessageSentResponse)
	if !ok {
		return nil, newErrorResponseTypeAssertFailed("*responseold.MessageSentResponse")
	}

	return messageSentResponse, nil
}

func (c *Client) StartTyping(conversationUrn string) error {
	actionQuery := queryold.DoActionQuery{
		Action: queryold.ActionTyping,
	}

	typingPayload := payloadold.StartTypingPayload{
		ConversationUrn: conversationUrn,
	}

	resp, _, err := c.MakeRoutingRequest(routingold.LinkedInMessagingDashMessengerConversationsURL, typingPayload, actionQuery)
	if err != nil {
		return err
	}

	if resp.StatusCode > 204 {
		return fmt.Errorf("failed to start typing in conversation with urn %s (statusCode=%d)", conversationUrn, resp.StatusCode)
	}

	return nil
}

// this endpoint allows you to mark multiple threads as read/unread at a time
// pass false to second arg to unread all conversations and true to read all of them
func (c *Client) MarkThreadRead(conversationUrns []string, read bool) (*responseold.MarkThreadReadResponse, error) {
	queryUrnValues := ""
	entities := make(map[string]payloadold.GraphQLPatchBody, 0)
	for i, convUrn := range conversationUrns {
		if i >= len(conversationUrns)-1 {
			queryUrnValues += url.QueryEscape(convUrn)
		} else {
			queryUrnValues += url.QueryEscape(convUrn) + ","
		}
		entities[convUrn] = payloadold.GraphQLPatchBody{
			Patch: payloadold.Patch{
				Set: payloadold.MarkThreadReadBody{
					Read: read,
				},
			},
		}
	}

	queryStr := fmt.Sprintf("ids=List(%s)", queryUrnValues)
	markReadUrl := fmt.Sprintf("%s?%s", routingold.LinkedInMessagingDashMessengerConversationsURL, queryStr)
	patchEntitiesPayload := payloadold.PatchEntitiesPayload{
		Entities: entities,
	}

	payloadBytes, err := patchEntitiesPayload.Encode()
	if err != nil {
		return nil, err
	}

	headerOpts := typesold.HeaderOpts{
		WithCookies:         true,
		WithCsrfToken:       true,
		Origin:              string(routingold.LinkedInBaseURL),
		WithXLiTrack:        true,
		WithXLiProtocolVer:  true,
		WithXLiPageInstance: true,
		WithXLiLang:         true,
		Extra:               map[string]string{"accept": string(typesold.ContentTypeJSON)},
	}

	headers := c.buildHeaders(headerOpts)
	resp, respBody, err := c.MakeRequest(markReadUrl, http.MethodPost, headers, payloadBytes, typesold.ContentTypePlaintextUTF8)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 204 {
		return nil, fmt.Errorf("failed to read conversations... (statusCode=%d)", resp.StatusCode)
	}

	result := &responseold.MarkThreadReadResponse{}
	return result, json.Unmarshal(respBody, result)
}

func (c *Client) DeleteConversation(conversationUrn string) error {
	deleteConvUrl := fmt.Sprintf("%s/%s", routingold.LinkedInMessagingDashMessengerConversationsURL, url.QueryEscape(conversationUrn))

	headers := c.buildHeaders(typesold.HeaderOpts{
		WithCookies:         true,
		WithCsrfToken:       true,
		WithXLiTrack:        true,
		WithXLiPageInstance: true,
		WithXLiLang:         true,
		WithXLiProtocolVer:  true,
		Origin:              string(routingold.LinkedInBaseURL),
		Extra: map[string]string{
			"accept": string(typesold.ContentTypeGraphQL),
		},
	})

	resp, _, err := c.MakeRequest(deleteConvUrl, http.MethodDelete, headers, nil, "")
	if err != nil {
		return err
	}

	if resp.StatusCode > 204 {
		return fmt.Errorf("failed to delete conversation with conversation urn %s (statusCode=%d)", conversationUrn, resp.StatusCode)
	}

	return nil
}

// pass true to second arg to react and pass false to unreact
func (c *Client) SendReaction(p payloadold.SendReactionPayload, react bool) error {
	action := queryold.ActionReactWithEmoji
	if !react {
		action = queryold.ActionUnreactWithEmoji
	}
	actionQuery := queryold.DoActionQuery{
		Action: action,
	}

	resp, _, err := c.MakeRoutingRequest(routingold.LinkedInVoyagerMessagingDashMessengerMessagesURL, p, actionQuery)
	if err != nil {
		return err
	}

	if resp.StatusCode > 204 {
		return fmt.Errorf("failed to perform reaction with emoji %s on message with urn %s (statusCode=%d)", p.Emoji, p.MessageUrn, resp.StatusCode)
	}

	return nil
}

func (c *Client) GetReactionsForEmoji(vars queryold.GetReactionsForEmojiVariables) ([]typesold.ConversationParticipant, error) {
	variablesQuery, err := vars.Encode()
	if err != nil {
		return nil, err
	}

	gqlQuery := queryold.GraphQLQuery{
		QueryID:   "messengerMessagingParticipants.3d2e0e93494e9dbf4943dc19da98bdf6",
		Variables: string(variablesQuery),
	}

	_, respData, err := c.MakeRoutingRequest(routingold.LinkedInVoyagerMessagingGraphQLURL, nil, &gqlQuery)
	if err != nil {
		return nil, err
	}

	graphQLResponse, ok := respData.(*responseold.GraphQlResponse)
	if !ok || graphQLResponse == nil {
		return nil, newErrorResponseTypeAssertFailed("*responseold.GraphQlResponse")
	}

	graphQLResponseData := graphQLResponse.Data

	return graphQLResponseData.MessengerMessagingParticipantsByMessageAndEmoji.Participants, nil
}
