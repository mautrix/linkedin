package linkedingo

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"

	"github.com/google/uuid"
)

var blockedMemberIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (c *Client) GetBlockedUsers(ctx context.Context) ([]URN, error) {
	var response struct {
		Elements []struct {
			MemberIdentifier string `json:"memberIdentifier"`
		} `json:"elements"`
	}
	_, err := c.newAuthedRequest(http.MethodGet, "https://www.linkedin.com/mysettings-api/settingsApiMemberBlocking/").
		WithQueryParam("q", "currentMember").
		WithHeader("Accept", "application/json").
		WithHeader("X-Restli-Protocol-Version", "2.0.0").
		WithCSRF().
		Do(ctx, &response)
	if err != nil {
		return nil, err
	} else if response.Elements == nil {
		return nil, fmt.Errorf("block list response is missing elements")
	}
	users := make([]URN, 0, len(response.Elements))
	for _, member := range response.Elements {
		if !blockedMemberIdentifierPattern.MatchString(member.MemberIdentifier) {
			return nil, fmt.Errorf("block list response contains an invalid member identifier")
		}
		users = append(users, NewURN(member.MemberIdentifier).AsFsdProfile())
	}
	return users, nil
}

var memberIDPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

func (c *Client) resolveBlockMemberID(ctx context.Context, profile URN) (string, error) {
	var response struct {
		Elements []struct {
			EntityURN URN `json:"entityUrn"`
			ObjectURN URN `json:"objectUrn"`
		} `json:"elements"`
	}
	_, err := c.newAuthedRequest(http.MethodGet, "https://www.linkedin.com/voyager/api/identity/dash/profiles").
		WithQueryParam("q", "memberIdentity").
		WithQueryParam("memberIdentity", profile.ID()).
		WithQueryParam("decorationId", "com.linkedin.voyager.dash.deco.identity.profile.FullProfileWithEntities-35").
		WithHeader("Accept", "application/json").
		WithHeader("X-Restli-Protocol-Version", "2.0.0").
		WithCSRF().Do(ctx, &response)
	if err != nil {
		return "", err
	} else if len(response.Elements) != 1 || response.Elements[0].EntityURN != profile.AsFsdProfile() {
		return "", fmt.Errorf("profile response does not match the requested member")
	}
	member := response.Elements[0].ObjectURN
	if member.prefix != "urn:li:member" || !memberIDPattern.MatchString(member.ID()) {
		return "", fmt.Errorf("profile response contains an invalid member ID")
	}
	return member.ID(), nil
}

func (c *Client) BlockUser(ctx context.Context, target URN, displayName string) error {
	if err := ctx.Err(); err != nil {
		return err
	} else if !blockedMemberIdentifierPattern.MatchString(target.ID()) || !blockedMemberIdentifierPattern.MatchString(c.userEntityURN.ID()) ||
		target.ID() == c.userEntityURN.ID() || (target.prefix != "" && target.prefix != "urn:li:fsd_profile") {
		return fmt.Errorf("invalid block target or account")
	}
	target = target.AsFsdProfile()
	users, err := c.GetBlockedUsers(ctx)
	if err != nil {
		return err
	} else if slices.Contains(users, target) {
		return nil
	}
	targetID, err := c.resolveBlockMemberID(ctx, target)
	if err != nil {
		return err
	}
	selfID, err := c.resolveBlockMemberID(ctx, c.userEntityURN)
	if err != nil {
		return err
	} else if selfID == targetID {
		return fmt.Errorf("block target resolves to the current member")
	}
	const requestID = "com.linkedin.sdui.requests.trustReporting.block"
	args := map[string]any{
		"$type":              "proto.sdui.actions.requests.RequestedArguments",
		"requestedStateKeys": []any{},
		"payload": map[string]any{
			"blockeeMemberUrn":      map[string]string{"memberId": targetID},
			"blockerMemberUrn":      map[string]string{"memberId": selfID},
			"blockeeName":           displayName,
			"buttonDisabledBinding": map[string]string{"key": "blockButton", "namespace": "MemoryNamespace"},
			"loadingBinding":        map[string]string{"key": "reportPageLoadingSpinner", "namespace": "MemoryNamespace"},
			"blockScreenId":         "com.linkedin.sdui.flagshipnav.trustreporting.BlockLanding",
			"workflowId":            "BLOCKING_WORKFLOW_" + uuid.NewString(),
		},
		"requestMetadata": map[string]string{"$type": "proto.sdui.common.RequestMetadata"},
	}
	outerArgs := maps.Clone(args)
	outerArgs["states"] = []any{}
	outerArgs["screenId"] = "com.linkedin.sdui.flagshipnav.trustreporting.MemberBlockSubmission"
	outerArgs["knownTemplateIds"] = []any{}
	request := c.newAuthedRequest(http.MethodPost, "https://www.linkedin.com/flagship-web/rsc-action/actions/server-request?sduiid="+requestID).
		WithContentType("application/json").WithHeader("X-Li-Rsc-Stream", "true").WithCSRF().
		WithJSONPayload(map[string]any{
			"requestId":          requestID,
			"states":             []any{},
			"requestedArguments": outerArgs,
			"serverRequest": map[string]any{
				"requestId":          requestID,
				"requestedArguments": args,
				"isApfcEnabled":      false,
				"isStreaming":        false,
				"rumPageKey":         "",
			},
		})
	// Replaying an uncertain block could repeat a non-idempotent account mutation.
	response, err := request.doRawRetry(ctx)
	if err != nil {
		return fmt.Errorf("block request outcome is unknown: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &ResponseError{StatusCode: response.StatusCode}
	}
	if _, err = io.Copy(io.Discard, response.Body); err != nil {
		return fmt.Errorf("failed to read block response: %w", err)
	}
	users, err = c.GetBlockedUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify block: %w", err)
	} else if !slices.Contains(users, target) {
		return fmt.Errorf("LinkedIn did not confirm the block")
	}
	return nil
}
