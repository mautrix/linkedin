package linkedingo

import (
	"context"
	"net/http"
)

func (c *Client) Search(ctx context.Context, keywords string) ([]URN, error) {
	query := queriesToString(map[string]string{
		"keywords":                 keywords,
		"flagshipSearchIntent":     "SEARCH_SRP",
		"queryParameters":          "List((key:network,value:List(F)),(key:resultType,value:List(PEOPLE)))",
		"includeFiltersInResponse": "false",
	})
	var response GraphQlResponse
	_, err := c.newAuthedRequest(http.MethodGet, linkedInVoyagerGraphQLURL).
		WithGraphQLQuery(graphQLQueryIDVoyagerSearchDashClusters, map[string]string{
			"start":  "0",
			"origin": "GLOBAL_SEARCH_HEADER",
			"query":  query,
		}).
		WithHeader("accept", contentTypeJSONLinkedInNormalized).
		Do(ctx, &response)
	if err != nil {
		return nil, err
	}

	entityURNs := []URN{}
	for _, data := range response.Included {
		if data.Type == "com.linkedin.voyager.dash.identity.profile.Profile" && data.EntityURN != nil {
			entityURNs = append(entityURNs, *data.EntityURN)
		}

	}
	return entityURNs, nil
}
