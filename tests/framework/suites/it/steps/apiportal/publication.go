/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package apiportal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// registerPublication binds the steps that read a published API back from the portal.
func registerPublication(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^the API Portal should list the API "([^"]*)" offering plan "([^"]*)"$`, s.listsAPIOfferingPlan)
	sc.Step(`^the API Portal should list the API "([^"]*)" with status "([^"]*)"$`, s.listsAPIWithStatus)
	sc.Step(`^the API Portal should list the API "([^"]*)" with display name "([^"]*)"$`, s.listsAPIWithDisplayName)
	sc.Step(`^the API Portal should not list the API "([^"]*)"$`, s.doesNotListAPI)
	sc.Step(`^an application subscribes to the published API "([^"]*)" with plan "([^"]*)" in the API Portal and the subscription is stored as "([^"]*)"$`,
		s.subscribeToPublishedAPI)
	sc.Step(`^an API key is generated in the API Portal for the published API "([^"]*)" and the key handle is stored as "([^"]*)"$`,
		s.generateKeyForPublishedAPI)
	registerOutage(sc, s)
	sc.Step(`^the subscription "([^"]*)" is deleted from the API Portal$`, s.deletePublishedSubscription)
	sc.Step(`^the API key "([^"]*)" of the published API "([^"]*)" is revoked in the API Portal$`, s.revokePublishedKey)
}

// trackPublishedPlan registers the removal of a plan from the portal at the scenario's end.
// Publishing creates the plans the portal does not have yet, and the portal keeps them when the
// control plane deletes its own; a plan the portal never received is already gone.
func (s *Steps) trackPublishedPlan(ctx context.Context, handle string) error {
	return s.registerPortalResource(ctx, portalSubscriptionPlanKind, handle, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/subscription-plans"})
}

// subscribeToPublishedAPI creates an application and subscribes it to an API the control plane
// published, which the portal lists under the control plane's own handle. Both are removed at
// the scenario's end.
func (s *Steps) subscribeToPublishedAPI(ctx context.Context, apiHandle, planHandle, storeAs string) error {
	api, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	plan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	app, err := unique.Unique(ctx, "publish-app")
	if err != nil {
		return err
	}
	subscriptionID, _, err := s.subscribeToArtifact(ctx, api, app, plan)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, subscriptionID)
}

// deletePublishedSubscription deletes a subscription made to a published API; the cleanup that
// was registered for it tolerates its absence.
func (s *Steps) deletePublishedSubscription(ctx context.Context, subscriptionID string) error {
	id, err := stepscommon.Expand(ctx, subscriptionID)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodDelete,
		URL:     base + apiPrefix + "/subscriptions/" + url.PathEscape(id),
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("DELETE /subscriptions/%s: %w", id, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("DELETE /subscriptions/%s failed: %s", id, resp.Describe())
	}
	return nil
}

// revokePublishedKey revokes an API key of a published API.
func (s *Steps) revokePublishedKey(ctx context.Context, keyHandle, apiHandle string) error {
	key, err := stepscommon.Expand(ctx, keyHandle)
	if err != nil {
		return err
	}
	api, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	return s.revokeKeyOfArtifact(ctx, api, key)
}

// generateKeyForPublishedAPI generates an API key for an API the control plane published and
// revokes it at the scenario's end.
func (s *Steps) generateKeyForPublishedAPI(ctx context.Context, apiHandle, storeAs string) error {
	api, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	keyID, err := unique.Unique(ctx, "publish-key")
	if err != nil {
		return err
	}
	_, handle, err := s.generateKeyForArtifact(ctx, api, keyID)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, handle)
}

// fetchListedAPI reads one API's metadata from the portal as the administrator and returns the
// response, whatever its status.
func (s *Steps) fetchListedAPI(ctx context.Context, apiHandle string) (api string, resp *httpx.Response, err error) {
	if api, err = stepscommon.Expand(ctx, apiHandle); err != nil {
		return "", nil, err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return "", nil, err
	}
	resp, err = s.getJSON(ctx, base, token, "/apis/"+url.PathEscape(api))
	return api, resp, err
}

// listedAPI reads one API's metadata from the portal and fails unless the portal lists it.
func (s *Steps) listedAPI(ctx context.Context, apiHandle string) (string, []byte, error) {
	api, resp, err := s.fetchListedAPI(ctx, apiHandle)
	if err != nil {
		return "", nil, err
	}
	if !resp.Succeeded() {
		return "", nil, fmt.Errorf("the API Portal does not list %q: %s", api, resp.Describe())
	}
	return api, resp.Body, nil
}

// listsAPIOfferingPlan asserts the portal itself lists the API and offers the plan, whatever the
// API's status.
func (s *Steps) listsAPIOfferingPlan(ctx context.Context, apiHandle, planHandle string) error {
	plan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	api, body, err := s.listedAPI(ctx, apiHandle)
	if err != nil {
		return err
	}
	return apiOffersPlan(body, api, plan)
}

// listsAPIWithStatus asserts the portal lists the API with the given status.
func (s *Steps) listsAPIWithStatus(ctx context.Context, apiHandle, status string) error {
	api, body, err := s.listedAPI(ctx, apiHandle)
	if err != nil {
		return err
	}
	return apiHasField(body, api, "status", status)
}

// listsAPIWithDisplayName asserts the portal lists the API under the display name.
func (s *Steps) listsAPIWithDisplayName(ctx context.Context, apiHandle, displayName string) error {
	want, err := stepscommon.Expand(ctx, displayName)
	if err != nil {
		return err
	}
	api, body, err := s.listedAPI(ctx, apiHandle)
	if err != nil {
		return err
	}
	return apiHasField(body, api, "name", want)
}

// doesNotListAPI asserts the portal has no listing for the API.
func (s *Steps) doesNotListAPI(ctx context.Context, apiHandle string) error {
	api, resp, err := s.fetchListedAPI(ctx, apiHandle)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("the API Portal should not list %q, got: %s", api, resp.Describe())
	}
	return nil
}

// apiHasField checks one top-level string field of the portal's API metadata.
func apiHasField(body []byte, apiHandle, field, want string) error {
	var metadata map[string]any
	if err := json.Unmarshal(body, &metadata); err != nil {
		return fmt.Errorf("decoding the API Portal's metadata for %q: %w", apiHandle, err)
	}
	if got, _ := metadata[field].(string); got != want {
		return fmt.Errorf("the API Portal lists %q with %s %q, want %q", apiHandle, field, got, want)
	}
	return nil
}

// apiOffersPlan checks the portal's API metadata names the API and links the plan.
func apiOffersPlan(body []byte, apiHandle, planHandle string) error {
	var metadata struct {
		ID                string `json:"id"`
		SubscriptionPlans []struct {
			ID string `json:"id"`
		} `json:"subscriptionPlans"`
	}
	if err := json.Unmarshal(body, &metadata); err != nil {
		return fmt.Errorf("decoding the API Portal's metadata for %q: %w", apiHandle, err)
	}
	if metadata.ID != apiHandle {
		return fmt.Errorf("the API Portal returned %q, want %q", metadata.ID, apiHandle)
	}
	for _, plan := range metadata.SubscriptionPlans {
		if plan.ID == planHandle {
			return nil
		}
	}
	return fmt.Errorf("the API Portal lists %q without plan %q", apiHandle, planHandle)
}
