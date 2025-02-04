package connectorold

import (
	"context"
	"regexp"

	"maunium.net/go/mautrix/bridgev2"
)

var (
	LoginURLRegex    = regexp.MustCompile(`https?:\\/\\/(www\\.)?([\\w-]+\\.)*linkedin\\.com(\\/[^\\s]*)?`)
	ValidCookieRegex = regexp.MustCompile(`\bJSESSIONID=[^;]+`)
)

var (
	LoginStepIDCookies  = "fi.mau.linkedin.login.enter_cookies"
	LoginStepIDComplete = "fi.mau.linkedin.login.complete"
)

func (lc *LinkedInConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{
		{
			Name:        "Cookies",
			Description: "Log in with your LinkedIn account using your cookies",
			ID:          "cookies",
		},
	}
}

func (lc *LinkedInConnector) CreateLogin(_ context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	return nil, nil
}
