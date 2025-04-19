package connector

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix/bridgev2"
)

var _ bridgev2.PushableNetworkAPI = (*LinkedInClient)(nil)

var pushCfg = &bridgev2.PushConfig{
	FCM: &bridgev2.FCMPushConfig{SenderID: "789113911969"},
}

func (l *LinkedInClient) GetPushConfigs() *bridgev2.PushConfig {
	return pushCfg
}

func (l *LinkedInClient) RegisterPushNotifications(ctx context.Context, pushType bridgev2.PushType, token string) error {
	if pushType != bridgev2.PushTypeFCM {
		return fmt.Errorf("unsupported push type: %s", pushType)
	}

	return l.client.RegisterAndroidPush(ctx, token)
}
