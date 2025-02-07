package connectorold

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridge/status"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	bridgeEvt "maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/responseold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type LinkedInClient struct {
	connector *LinkedInConnector
	client    *linkedingoold.Client

	userLogin *bridgev2.UserLogin

	userCache   map[string]typesold.Member
	threadCache map[string]responseold.ThreadElement
}

var (
	_ bridgev2.NetworkAPI = (*LinkedInClient)(nil)
)

func NewLinkedInClient(ctx context.Context, tc *LinkedInConnector, login *bridgev2.UserLogin) *LinkedInClient {
	log := zerolog.Ctx(ctx).With().
		Str("component", "linkedin_client").
		Str("user_login_id", string(login.ID)).
		Logger()

	// meta := login.Metadata.(*UserLoginMetadata)
	clientOpts := &linkedingoold.ClientOpts{
		// Cookies: cookies.NewCookiesFromString(meta.Cookies),
	}
	linClient := &LinkedInClient{
		client:      linkedingoold.NewClient(clientOpts, log),
		userLogin:   login,
		userCache:   make(map[string]typesold.Member),
		threadCache: make(map[string]responseold.ThreadElement),
	}

	linClient.client.SetEventHandler(linClient.HandleLinkedInEvent)
	linClient.connector = tc
	return linClient
}

func (lc *LinkedInClient) Connect(ctx context.Context) {
	log := zerolog.Ctx(ctx)
	if lc.client == nil {
		lc.userLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "linkedin-not-logged-in",
		})
		return
	}

	err := lc.client.LoadMessagesPage()
	if err != nil {
		log.Err(err).Msg("failed to load messages page")
		return
	}

	profile, err := lc.client.GetCurrentUserProfile()
	if err != nil {
		log.Err(err).Msg("failed to get current user profile")
		return
	}

	lc.userLogin.RemoteName = fmt.Sprintf("%s %s", profile.MiniProfile.FirstName, profile.MiniProfile.LastName)
	lc.userLogin.Save(ctx)

	err = lc.client.Connect()
	if err != nil {
		log.Err(err).Msg("failed to connect to LinkedIn client")
		return
	}
	lc.userLogin.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})

	go lc.syncChannels(ctx)
}

func (lc *LinkedInClient) Disconnect() {
	err := lc.client.Disconnect()
	if err != nil {
		lc.userLogin.Log.Error().Err(err).Msg("failed to disconnect, err:")
	}
}

func (lc *LinkedInClient) IsLoggedIn() bool {
	return ValidCookieRegex.MatchString(lc.userLogin.Metadata.(*UserLoginMetadata).Cookies)
}

func (lc *LinkedInClient) LogoutRemote(ctx context.Context) {
	panic("unimplemented")
}

func (lc *LinkedInClient) IsThisUser(_ context.Context, userID networkid.UserID) bool {
	return networkid.UserID(lc.client.GetCurrentUserID()) == userID
}

func (lc *LinkedInClient) GetCurrentUser() (user *typesold.UserLoginProfile, err error) {
	user, err = lc.client.GetCurrentUserProfile()
	return
}

func (lc *LinkedInClient) GetChatInfo(_ context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	// not supported
	return nil, nil
}

func (lc *LinkedInClient) GetUserInfo(_ context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	userInfo := lc.GetUserInfoBridge(string(ghost.ID))
	if userInfo == nil {
		return nil, fmt.Errorf("failed to find user info in cache by id: %s", ghost.ID)
	}
	return userInfo, nil
}

func (lc *LinkedInClient) convertEditToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, data *responseold.MessageElement) (*bridgev2.ConvertedEdit, error) {
	converted, err := lc.convertToMatrix(ctx, portal, intent, data)
	if err != nil {
		return nil, err
	}
	return &bridgev2.ConvertedEdit{
		ModifiedParts: []*bridgev2.ConvertedEditPart{converted.Parts[0].ToEditPart(existing[0])},
	}, nil
}

func (lc *LinkedInClient) convertToMatrix(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, msg *responseold.MessageElement) (*bridgev2.ConvertedMessage, error) {
	var replyTo *networkid.MessageOptionalPartID
	parts := make([]*bridgev2.ConvertedMessagePart, 0)

	for _, renderContent := range msg.RenderContent {
		if renderContent.RepliedMessageContent != nil {
			replyTo = &networkid.MessageOptionalPartID{
				MessageID: networkid.MessageID(renderContent.RepliedMessageContent.OriginalMessageUrn),
			}
		} else {
			convertedPart, err := lc.LinkedInAttachmentToMatrix(ctx, portal, intent, renderContent)
			if err != nil {
				if !errors.Is(err, ErrUnsupportedAttachmentType) {
					return nil, err
				}
			}
			if convertedPart != nil {
				parts = append(parts, convertedPart)
			}
		}
	}

	textPart := &bridgev2.ConvertedMessagePart{
		ID:   "",
		Type: bridgeEvt.EventMessage,
		Content: &bridgeEvt.MessageEventContent{
			MsgType: bridgeEvt.MsgText,
			Body:    msg.Body.Text,
		},
	}

	if len(textPart.Content.Body) > 0 {
		parts = append(parts, textPart)
	}

	cm := &bridgev2.ConvertedMessage{
		ReplyTo: replyTo,
		Parts:   parts,
	}

	cm.MergeCaption() // merges captions and media onto one part

	return cm, nil
}

func (lc *LinkedInClient) MakePortalKey(thread responseold.ThreadElement) networkid.PortalKey {
	var receiver networkid.UserLoginID
	if !thread.GroupChat {
		receiver = lc.userLogin.ID
	}
	return networkid.PortalKey{
		ID:       networkid.PortalID(thread.EntityUrn),
		Receiver: receiver,
	}
}
