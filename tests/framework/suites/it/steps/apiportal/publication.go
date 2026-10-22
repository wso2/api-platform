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
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// registerPublication binds the step that reads a published API back from the portal.
func registerPublication(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^the API Portal should list the API "([^"]*)" offering plan "([^"]*)"$`, s.listsAPIOfferingPlan)
}

// trackPublishedPlan registers the removal of a plan from the portal at the scenario's end.
// Publishing creates the plans the portal does not have yet, and the portal keeps them when the
// control plane deletes its own; a plan the portal never received is already gone.
func (s *Steps) trackPublishedPlan(ctx context.Context, handle string) error {
	return s.registerPortalResource(ctx, portalSubscriptionPlanKind, handle, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/subscription-plans"})
}

// listsAPIOfferingPlan asserts the portal itself lists the API as published and offering the plan.
func (s *Steps) listsAPIOfferingPlan(ctx context.Context, apiHandle, planHandle string) error {
	api, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	plan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	resp, err := s.getJSON(ctx, base, token, "/apis/"+url.PathEscape(api))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("the API Portal does not list %q: %s", api, resp.Describe())
	}
	return apiOffersPlan(resp.Body, api, plan)
}

// apiOffersPlan checks the portal's API metadata names the API, is published and links the plan.
func apiOffersPlan(body []byte, apiHandle, planHandle string) error {
	var metadata struct {
		ID                string `json:"id"`
		Status            string `json:"status"`
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
	if metadata.Status != "PUBLISHED" {
		return fmt.Errorf("the API Portal lists %q with status %q, want PUBLISHED", apiHandle, metadata.Status)
	}
	for _, plan := range metadata.SubscriptionPlans {
		if plan.ID == planHandle {
			return nil
		}
	}
	return fmt.Errorf("the API Portal lists %q without plan %q", apiHandle, planHandle)
}
