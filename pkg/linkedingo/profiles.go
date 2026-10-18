package linkedingo

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type dashProfile struct {
	EntityURN        URN    `json:"entityUrn"`
	PublicIdentifier string `json:"publicIdentifier"`
}

func (c *Client) GetPublicIdentifiers(ctx context.Context, profileURNs []URN) (map[string]string, error) {
	ids := make([]string, len(profileURNs))
	for i, urn := range profileURNs {
		ids[i] = url.QueryEscape(urn.AsFsdProfile().String())
	}
	var resp struct {
		Included []dashProfile `json:"included"`
	}
	_, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerIdentityDashProfilesURL).
		WithCSRF().
		WithXLIHeaders().
		WithRawQuery("ids=List("+strings.Join(ids, ",")+")").
		WithHeader("accept", contentTypeJSONLinkedInNormalized).
		WithHeader("x-restli-protocol-version", "2.0.0").
		Do(ctx, &resp)
	if err != nil {
		return nil, err
	}
	identifiers := make(map[string]string, len(resp.Included))
	for _, profile := range resp.Included {
		if profile.PublicIdentifier != "" {
			identifiers[profile.EntityURN.ID()] = profile.PublicIdentifier
		}
	}
	return identifiers, nil
}
