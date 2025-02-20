package connector

import (
	"context"

	"maunium.net/go/mautrix/bridgev2"
)

func (l *LinkedInClient) FetchMessages(ctx context.Context, fetchParams bridgev2.FetchMessagesParams) (*bridgev2.FetchMessagesResponse, error) {
	// variables := queryold.FetchMessagesVariables{
	// 	ConversationURN: linkedingo.NewURN(fetchParams.Portal.ID),
	// 	CountBefore:     20,
	// }
	//
	// if fetchParams.Cursor == "" {
	// 	if fetchParams.AnchorMessage != nil {
	// 		variables.DeliveredAt = jsontime.UM(fetchParams.AnchorMessage.Timestamp)
	// 	}
	// } else {
	// 	deliveredAt, err := strconv.ParseInt(string(fetchParams.Cursor), 10, 64)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	variables.DeliveredAt = jsontime.UnixMilli{Time: time.UnixMilli(deliveredAt)}
	// }
	//
	// fetchMessages, err := l.client.FetchMessages(ctx, variables)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// fmt.Printf("%+v\n", fetchMessages)
	//
	panic("here")
	//
	// messages := fetchMessages.Messages
	// sort.Slice(messages, func(j, i int) bool {
	// 	return messages[j].DeliveredAt < messages[i].DeliveredAt
	// })
	//
	// backfilledMessages := make([]*bridgev2.BackfillMessage, len(messages))
	// cursor := networkid.PaginationCursor("")
	// if len(messages) > 0 {
	// 	cursor = networkid.PaginationCursor(strconv.FormatInt(messages[0].DeliveredAt, 10))
	//
	// 	backfilledMessages, err = l.MessagesToBackfillMessages(ctx, messages, fetchParams.Portal)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	//
	// return &bridgev2.FetchMessagesResponse{
	// 	Messages: backfilledMessages,
	// 	Cursor:   cursor,
	// 	HasMore:  len(messages) >= 20,
	// 	Forward:  fetchParams.Forward,
	// }, nil
	//
}
