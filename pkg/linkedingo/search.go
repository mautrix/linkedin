package linkedingo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
)

const searchProfileType = "com.linkedin.voyager.dash.identity.profile.Profile"

type SearchResult struct {
	ProfileURN     URN
	FirstName      string
	LastName       string
	ProfilePicture *VectorImage
}

type searchResponse struct {
	Included []searchProfile `json:"included,omitempty"`
}

type searchProfile struct {
	Type           string                `json:"$type,omitempty"`
	EntityURN      URN                   `json:"entityUrn,omitempty"`
	FirstName      string                `json:"firstName,omitempty"`
	LastName       string                `json:"lastName,omitempty"`
	ProfilePicture *searchProfilePicture `json:"profilePicture,omitempty"`
}

type searchProfilePicture struct {
	DisplayImageReferenceResolutionResult struct {
		VectorImage *VectorImage `json:"vectorImage,omitempty"`
	} `json:"displayImageReferenceResolutionResult,omitempty"`
}

func (p *searchProfilePicture) vectorImage() *VectorImage {
	if p == nil {
		return nil
	}
	return p.DisplayImageReferenceResolutionResult.VectorImage
}

func (c *Client) Search(ctx context.Context, keywords string) ([]SearchResult, error) {
	log := zerolog.Ctx(ctx).With().Str("action", "linkedin_search").Str("keywords", keywords).Logger()

	var raw json.RawMessage
	_, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerGraphQLURL).
		WithCSRF().
		WithXLIHeaders().
		WithRawQuery(fmt.Sprintf(
			"includeWebMetadata=true&variables=(keyword:%s,types:List(CONNECTIONS,GROUP_THREADS,PEOPLE,COWORKERS))&queryId=%s",
			keywords, graphQLQueryIDVoyagerMessagingTypeahead,
		)).
		WithHeader("accept", contentTypeJSONLinkedInNormalized).
		Do(ctx, &raw)
	if err != nil {
		return nil, err
	}

	var response searchResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	results := []SearchResult{}
	skippedTypes := map[string]int{}
	for _, inc := range response.Included {
		if inc.Type != searchProfileType {
			skippedTypes[inc.Type]++
			continue
		}
		results = append(results, SearchResult{
			ProfileURN:     inc.EntityURN,
			FirstName:      inc.FirstName,
			LastName:       inc.LastName,
			ProfilePicture: inc.ProfilePicture.vectorImage(),
		})
	}

	log.Debug().
		Int("included_count", len(response.Included)).
		Int("result_count", len(results)).
		Interface("skipped_types", skippedTypes).
		Msg("Parsed LinkedIn search results")
	if len(results) == 0 {
		log.Debug().RawJSON("response", raw).Msg("LinkedIn search returned no usable results")
	}

	return results, nil
}
