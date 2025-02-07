package routing

import (
	"net/http"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/routing/responseold"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingoold/typesold"
)

type PayloadDataInterface interface {
	Encode() ([]byte, error)
}

type ResponseDataInterface interface {
	Decode(data []byte) (any, error)
}

type RequestEndpointInfo struct {
	Method             string
	HeaderOpts         typesold.HeaderOpts
	ContentType        typesold.ContentType
	ResponseDefinition ResponseDataInterface
}

var RequestStoreDefinition = map[RequestEndpointURL]RequestEndpointInfo{
	LinkedInMessagingBaseURL: {
		Method: http.MethodGet,
		HeaderOpts: typesold.HeaderOpts{
			WithCookies: true,
			Extra: map[string]string{
				"Sec-Fetch-Dest":            "document",
				"Sec-Fetch-Mode":            "navigate",
				"Sec-Fetch-Site":            "none",
				"Sec-Fetch-User":            "?1",
				"Upgrade-Insecure-Requests": "1",
			},
		},
	},
	LinkedInVoyagerMessagingGraphQLURL: {
		Method: http.MethodGet,
		HeaderOpts: typesold.HeaderOpts{
			WithCookies:         true,
			WithCsrfToken:       true,
			WithXLiTrack:        true,
			WithXLiPageInstance: true,
			WithXLiProtocolVer:  true,
			Referer:             string(LinkedInMessagingBaseURL) + "/",
			Extra: map[string]string{
				"accept": string(typesold.ContentTypeGraphQL),
			},
		},
		ResponseDefinition: responseold.GraphQlResponse{},
	},
	LinkedInVoyagerMessagingDashMessengerMessagesURL: {
		Method:      http.MethodPost,
		ContentType: typesold.ContentTypePlaintextUTF8,
		HeaderOpts: typesold.HeaderOpts{
			WithCookies:         true,
			WithCsrfToken:       true,
			WithXLiLang:         true,
			WithXLiPageInstance: true,
			WithXLiTrack:        true,
			WithXLiProtocolVer:  true,
			Origin:              string(LinkedInBaseURL),
			Extra: map[string]string{
				"accept": string(typesold.ContentTypeJSON),
			},
		},
		ResponseDefinition: responseold.MessageSentResponse{},
	},
	LinkedInMessagingDashMessengerConversationsURL: {
		Method:      http.MethodPost,
		ContentType: typesold.ContentTypePlaintextUTF8,
		HeaderOpts: typesold.HeaderOpts{
			WithCookies:         true,
			WithCsrfToken:       true,
			WithXLiTrack:        true,
			WithXLiPageInstance: true,
			WithXLiProtocolVer:  true,
			WithXLiLang:         true,
			Origin:              string(LinkedInBaseURL),
			Extra: map[string]string{
				"accept": string(typesold.ContentTypeJSON),
			},
		},
	},
	LinkedInVoyagerMediaUploadMetadataURL: {
		Method:      http.MethodPost,
		ContentType: typesold.ContentTypeJSONPlaintextUTF8,
		HeaderOpts: typesold.HeaderOpts{
			WithCookies:         true,
			WithCsrfToken:       true,
			WithXLiTrack:        true,
			WithXLiPageInstance: true,
			WithXLiProtocolVer:  true,
			WithXLiLang:         true,
			Extra: map[string]string{
				"accept": string(typesold.ContentTypeJSONLinkedInNormalized),
			},
		},
		ResponseDefinition: responseold.UploadMediaMetadataResponse{},
	},
	LinkedInLogoutURL: {
		Method: http.MethodGet,
		HeaderOpts: typesold.HeaderOpts{
			WithCookies: true,
		},
	},
}
