package connector

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var (
	_ bridgev2.GroupCreatingNetworkAPI = (*LinkedInClient)(nil)
)

func (l *LinkedInClient) ResolveIdentifier(ctx context.Context, id string, _ bool) (*bridgev2.ResolveIdentifierResponse, error) {
	zerolog.Ctx(ctx).Warn().Msg("ResolveIdentifier called")
	return nil, nil
}

func (l *LinkedInClient) CreateGroup(ctx context.Context, params *bridgev2.GroupCreateParams) (*bridgev2.CreateChatResponse, error) {
	chatInfo := bridgev2.ChatInfo{
		Name: ptr.Ptr(params.Name.Name),
		Members: &bridgev2.ChatMemberList{
			MemberMap: map[networkid.UserID]bridgev2.ChatMember{},
		},
	}
	participants := make([]linkedingo.URN, len(params.Participants))
	for i, participant := range params.Participants {
		participants[i] = linkedingo.NewURN(participant).AsFsdProfile()
		sender := l.makeSender(linkedingo.MessagingParticipant{
			EntityURN: participants[i],
		})
		chatInfo.Members.MemberMap[sender.Sender] = bridgev2.ChatMember{
			EventSender: sender,
			Membership:  event.MembershipJoin,
		}
	}
	resp, err := l.client.NewGroupChat(ctx, ptr.Val(params.Name).Name, ptr.Val(params.Topic).Topic, participants)

	if err != nil {
		return nil, fmt.Errorf("failed to create group chat: %w", err)
	}

	portalKey := networkid.PortalKey{
		ID:       networkid.PortalID(resp.Data.ConversationURN.String()),
		Receiver: l.userLogin.ID,
	}
	return &bridgev2.CreateChatResponse{
		PortalKey:  portalKey,
		PortalInfo: &chatInfo,
	}, nil
}
