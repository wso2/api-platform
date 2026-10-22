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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package platformapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// RegisterAPIVersions binds the two creation steps GET /rest-apis/versions needs.
// secret_deploy.go's "I create a REST API ..." steps derive the handle from the display name
// and hardcode version v1, so two of them collide; here both are separate arguments, letting
// one display name carry several versions. The lookup itself and every assertion use the
// suite's shared vocabulary.
func RegisterAPIVersions(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I create a REST API "([^"]*)" via the control plane in project "([^"]*)" with display name "([^"]*)" version "([^"]*)" and context "([^"]*)"$`,
		s.createRestAPIVersion)
	sc.Step(`^I attempt to create a REST API "([^"]*)" via the control plane in project "([^"]*)" with display name "([^"]*)" version "([^"]*)" and context "([^"]*)"$`,
		s.attemptRestAPIVersion)
}

// createRestAPIVersion creates one version and fails the step if the control plane refuses,
// for a scenario where the create is setup rather than the thing under test.
func (s *Steps) createRestAPIVersion(ctx context.Context, handle, projectHandle, displayName, version, apiContext string) error {
	resp, resolvedHandle, err := s.sendCreateRestAPIVersion(ctx, handle, projectHandle, displayName, version, apiContext)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("creating REST API %q at version %q: %s", resolvedHandle, version, resp.Describe())
	}
	return nil
}

// attemptRestAPIVersion issues the same create but publishes the response so the scenario can
// assert on the status instead of the step failing.
func (s *Steps) attemptRestAPIVersion(ctx context.Context, handle, projectHandle, displayName, version, apiContext string) error {
	_, _, err := s.sendCreateRestAPIVersion(ctx, handle, projectHandle, displayName, version, apiContext)
	return err
}

// sendCreateRestAPIVersion posts one REST API under an explicit handle, so several versions
// can share a display name, and registers it for cleanup whenever the control plane reports it
// created - including from a scenario that did not expect the create to succeed. The handle is
// sent as "id" because a derived one collides on the second version of the same display name.
func (s *Steps) sendCreateRestAPIVersion(
	ctx context.Context, handle, projectHandle, displayName, version, apiContext string,
) (*httpx.Response, string, error) {
	resolved := make([]string, 0, 5)
	for _, value := range []string{handle, projectHandle, displayName, version, apiContext} {
		expanded, err := stepscommon.Expand(ctx, value)
		if err != nil {
			return nil, "", err
		}
		resolved = append(resolved, expanded)
	}
	resolvedHandle, resolvedProject, resolvedDisplayName := resolved[0], resolved[1], resolved[2]
	resolvedVersion, resolvedContext := resolved[3], resolved[4]

	base, bearer, err := s.authed(ctx)
	if err != nil {
		return nil, resolvedHandle, err
	}

	body, err := json.Marshal(map[string]any{
		"id":          resolvedHandle,
		"displayName": resolvedDisplayName,
		"version":     resolvedVersion,
		"context":     resolvedContext,
		"projectId":   resolvedProject,
		"operations": []map[string]any{
			{"request": map[string]any{"method": "GET", "path": "/health"}},
		},
		"upstream": map[string]any{"main": map[string]any{"url": "http://testbench:3000"}},
	})
	if err != nil {
		return nil, resolvedHandle, err
	}

	resp, err := s.funnel.Send(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + apiBase + "/rest-apis",
		Headers:     map[string]string{"Authorization": "Bearer " + bearer},
		Body:        body,
		ContentType: "application/json",
	})
	if err != nil {
		return nil, resolvedHandle, fmt.Errorf("creating REST API %q at version %q: %w", resolvedHandle, resolvedVersion, err)
	}
	if !resp.Succeeded() {
		return resp, resolvedHandle, nil
	}
	// A different handle coming back means later assertions read the wrong resource.
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil {
		return resp, resolvedHandle, fmt.Errorf("decoding the created REST API %q: %w", resolvedHandle, err)
	}
	if created.ID != resolvedHandle {
		return resp, resolvedHandle, fmt.Errorf("control plane assigned REST API handle %q, expected %q", created.ID, resolvedHandle)
	}
	if err := s.registerPlatformResource(ctx, platformRESTAPIKind, resolvedHandle, "/rest-apis"); err != nil {
		return resp, resolvedHandle, err
	}
	return resp, resolvedHandle, nil
}
