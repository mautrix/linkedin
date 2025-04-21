package linkedingo

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

func (c *Client) RegisterAndroidPush(ctx context.Context, token string) error {
	payload := c.createPushRegistrationPayload(token)
	r := bytes.NewReader(payload)

	deviceId, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to generate device id: %w", err)
	}
	trackHeader := fmt.Sprintf(
		`{"osName":"Android OS","osVersion":"34","clientVersion":"4.1.1059","clientMinorVersion":196700,"model":"Google_Pixel 5a","displayDensity":2.625,"displayWidth":1080,"displayHeight":2201,"dpi":"xhdpi","deviceType":"android","appId":"com.linkedin.android","deviceId":"%s","timezoneOffset":0,"timezone":"Europe\/London","storeId":"us_googleplay","isAdTrackingLimited":false,"mpName":"voyager-android","mpVersion":"2.137.73"}`,
		deviceId.String(),
	)

	response, err := c.newAuthedRequest(http.MethodPost, linkedInVoyagerNotificationsDashPushRegistration).
		WithQueryParam("action", "register").
		WithXLIHeaders().
		WithCSRF().
		WithContentType("application/vnd.linkedin.deduped+x-protobuf; symbol-table=voyager-21129; charset=UTF-8").
		WithHeader("Accept", "application/vnd.linkedin.deduped+x-protobuf+2.0+gql").
		WithHeader("X-LI-Track", trackHeader).
		WithBody(r).
		Do(ctx, nil)

	if err != nil {
		return fmt.Errorf("failed to register push notification: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register push notification: %s", response.Status)
	}
	return nil
}

func (c *Client) createPushRegistrationPayload(token string) []byte {
	const (
		protobufStart              = byte(0x00)
		leadingOrdinal             = byte(0x14)
		arrayStart                 = byte(0x01)
		pushNotificationTokensKey  = "pushNotificationTokens"
		pushNotificationEnabledKey = "pushNotificationEnabled"
	)

	payload := []byte{
		protobufStart,
		2, // number of keys
		leadingOrdinal,
		byte(len(pushNotificationTokensKey)),
	}
	payload = append(payload, pushNotificationTokensKey...)
	payload = append(payload,
		arrayStart,
		1, // Array Length
		leadingOrdinal,
		byte(len(token)),
		0x01, // Unknown
	)
	payload = append(payload, token...)
	payload = append(payload,
		leadingOrdinal,
		byte(len(pushNotificationEnabledKey)),
	)
	payload = append(payload, pushNotificationEnabledKey...)
	payload = append(payload, 0x08)

	return payload
}
