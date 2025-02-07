package linkedingoold

import (
	"net/url"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/methodsold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routingold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type CurrentUser struct {
	FsdProfileID string
}

func (u *CurrentUser) GetEncodedFsdID() string {
	return url.QueryEscape(u.FsdProfileID)
}

type PageLoader struct {
	client          *Client
	CurrentUser     *CurrentUser
	XLiDeviceTrack  *typesold.DeviceTrack
	XLiPageInstance string
	XLiLang         string
}

func (c *Client) newPageLoader() *PageLoader {
	return &PageLoader{
		client:      c,
		CurrentUser: &CurrentUser{},
	}
}

func (pl *PageLoader) LoadMessagesPage() error {
	messagesDefinition := routingold.RequestStoreDefinition[routingold.LinkedInMessagingBaseURL]
	headers := pl.client.buildHeaders(messagesDefinition.HeaderOpts)
	_, respBody, err := pl.client.MakeRequest(string(routingold.LinkedInMessagingBaseURL), messagesDefinition.Method, headers, nil, "")
	if err != nil {
		return err
	}

	mainPageHTML := string(respBody)

	pl.XLiDeviceTrack = pl.ParseDeviceTrackInfo(mainPageHTML)
	pl.XLiPageInstance = pl.ParseXLiPageInstance(mainPageHTML)
	pl.XLiLang = methodsold.ParseMetaTagValue(mainPageHTML, "i18nLocale")

	// fsdProfileId := methodsold.ParseFsdProfileID(mainPageHTML)
	// if fsdProfileId == "" {
	// 	return fmt.Errorf("failed to find current user fsd profile id in html response to messaging page")
	// }
	//
	// pl.CurrentUser.FsdProfileID = fsdProfileId

	return nil
}

func (pl *PageLoader) ParseDeviceTrackInfo(html string) *typesold.DeviceTrack {
	serviceVersion := methodsold.ParseMetaTagValue(html, "serviceVersion")
	return &typesold.DeviceTrack{
		ClientVersion:    serviceVersion,
		MpVersion:        serviceVersion,
		OsName:           "web",
		TimezoneOffset:   2,
		Timezone:         "Europe/Stockholm", // TODO scrutinize?
		DeviceFormFactor: "DESKTOP",
		MpName:           "voyager-web",
		DisplayDensity:   1.125,
		DisplayWidth:     2560.5,
		DisplayHeight:    1440,
	}
}

func (pl *PageLoader) ParseXLiPageInstance(html string) string {
	clientPageInstanceId := methodsold.ParseMetaTagValue(html, "clientPageInstanceId")
	return "urn:li:page:messaging_index;" + clientPageInstanceId
}
