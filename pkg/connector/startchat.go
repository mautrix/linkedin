package connector

import (
	"context"
	"fmt"

	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

var (
	_ bridgev2.GhostDMCreatingNetworkAPI = (*LinkedInClient)(nil)
	_ bridgev2.GroupCreatingNetworkAPI   = (*LinkedInClient)(nil)
)

func (l *LinkedInClient) ResolveIdentifier(ctx context.Context, identifier string, createChat bool) (*bridgev2.ResolveIdentifierResponse, error) {
	id := networkid.UserID(identifier)
	ghost, _ := l.main.Bridge.GetGhostByID(ctx, id)
	var chat *bridgev2.CreateChatResponse
	if createChat {
		portal, _ := ghost.Bridge.GetDMPortal(ctx, l.userLogin.ID, id)
		if portal != nil {
			chat = &bridgev2.CreateChatResponse{
				PortalKey: portal.PortalKey,
			}
		} else {
			chatInfo := &bridgev2.ChatInfo{
				Type: ptr.Ptr(database.RoomTypeDM),
				Members: &bridgev2.ChatMemberList{
					MemberMap: map[networkid.UserID]bridgev2.ChatMember{},
				},
			}
			participants := []networkid.UserID{
				networkid.UserID(identifier),
			}
			var err error
			chat, err = l.createChat(ctx, chatInfo, participants)
			if err != nil {
				return nil, fmt.Errorf("failed to create dm chat: %w", err)
			}
		}
	}
	return &bridgev2.ResolveIdentifierResponse{
		UserID: id,
		Ghost:  ghost,
		Chat:   chat,
	}, nil
}

func (l *LinkedInClient) CreateChatWithGhost(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.CreateChatResponse, error) {
	resp, err := l.ResolveIdentifier(ctx, string(ghost.ID), true)
	if err != nil {
		return nil, err
	} else if resp == nil {
		return nil, nil
	}
	return resp.Chat, nil
}

func (l *LinkedInClient) CreateGroup(ctx context.Context, params *bridgev2.GroupCreateParams) (*bridgev2.CreateChatResponse, error) {
	chatInfo := &bridgev2.ChatInfo{
		Type: ptr.Ptr(database.RoomTypeGroupDM),
		Name: ptr.Ptr(params.Name.Name),
		Members: &bridgev2.ChatMemberList{
			MemberMap: map[networkid.UserID]bridgev2.ChatMember{},
		},
	}
	chat, err := l.createChat(ctx, chatInfo, params.Participants)
	if err != nil {
		return nil, fmt.Errorf("failed to create group chat: %w", err)
	}
	return chat, nil
}

func (l *LinkedInClient) createChat(ctx context.Context, chatInfo *bridgev2.ChatInfo, _participants []networkid.UserID) (*bridgev2.CreateChatResponse, error) {
	participants := make([]linkedingo.URN, len(_participants))
	for i, participant := range _participants {
		participants[i] = linkedingo.NewURN(participant).AsFsdProfile()
		sender := l.makeSender(linkedingo.MessagingParticipant{
			EntityURN: participants[i],
		})
		chatInfo.Members.MemberMap[sender.Sender] = bridgev2.ChatMember{
			EventSender: sender,
			Membership:  event.MembershipJoin,
		}
	}
	resp, err := l.client.NewChat(ctx, ptr.Val(chatInfo.Name), participants)

	if err != nil {
		return nil, err
	}

	portalKey := networkid.PortalKey{
		ID:       networkid.PortalID(resp.Data.ConversationURN.String()),
		Receiver: l.userLogin.ID,
	}
	return &bridgev2.CreateChatResponse{
		PortalKey:  portalKey,
		PortalInfo: chatInfo,
	}, nil
}
