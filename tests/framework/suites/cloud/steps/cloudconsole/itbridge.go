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

package cloudconsole

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/catalog/testbench"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

const (
	// itBackendEndpoint is the testbench endpoint an IT API's upstream points at.
	itBackendEndpoint = "backend"
	// itHealthPath is the operation the IT deployment scenarios invoke.
	itHealthPath = "/health"

	// keyITProjectHandle is the handle the scenario gave the project that itCreateProject selected.
	keyITProjectHandle = "cloudITProjectHandle"
)

// registerITBridge binds, against the cloud API, the phrases that integration-suite feature files
// use for creating and invoking a REST API. A cloud runner can then list those feature files
// (or one scenario of them, by line) without the IT step packages being registered: they address
// local gateway and platform-api containers, which a cloud run does not have.
//
// Add cloud integration features to an apip-cloud-it runner and bind each phrase here exactly
// once, using the same text; otherwise scenarios are undefined or ambiguous.
func (s *Steps) registerITBridge(sc *godog.ScenarioContext) {
	sc.Step(`^the gateway services are running$`, s.itGatewayReady)
	sc.Step(`^I authenticate using basic auth as "([^"]*)"$`, s.itAuthenticate)
	sc.Step(`^I generate a unique value from "([^"]*)" and store it as "([^"]*)"$`, stepscommon.GenerateAndStore)
	sc.Step(`^I generate a unique resource name from "([^"]*)" and store it as "([^"]*)"$`,
		stepscommon.GenerateResourceAndStore)
	sc.Step(`^I generate a unique API context from "([^"]*)" and store it as "([^"]*)"$`,
		stepscommon.GenerateContextAndStore)
	sc.Step(`^I create a project "([^"]*)" on the control plane$`, s.itCreateProject)
	sc.Step(`^I create a REST API "([^"]*)" via the control plane in project "([^"]*)" with context "([^"]*)"$`,
		s.itCreateRESTAPI)
	sc.Step(`^I deploy the "RestApi" "([^"]*)" to the gateway via the control plane$`, s.itDeployRESTAPI)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)"$`, s.sendOnce)
	sc.Step(`^the response status code should be (\d+)$`, s.assertResponseStatus)
}

// itGatewayReady selects the active gateway. The IT API's upstream is the testbench, so without
// a configured testbench the scenario is skipped before it creates anything in the organization.
func (s *Steps) itGatewayReady(ctx context.Context) error {
	configured, err := testbench.Configured(itBackendEndpoint)
	if err != nil {
		return err
	}
	if !configured {
		return fmt.Errorf("%s is not set, so the API has no testbench upstream: %w", testbench.EnvURL, godog.ErrSkip)
	}
	return s.findActiveGateway(ctx)
}

// itAuthenticate obtains the console token. The IT suite signs in as a local admin; a cloud
// organization is selected by the console token alone, so the user name is not used.
func (s *Steps) itAuthenticate(ctx context.Context, _ string) error {
	return s.obtainToken(ctx)
}

// itCreateProject selects the organization's default project for the project the scenario
// names. An organization's subscription limits how many projects it may hold (creating another
// returns 402 quota_exceeded), and apip-basic already runs in the default project, so nothing
// is created and nothing needs cleanup.
func (s *Steps) itCreateProject(ctx context.Context, handle string) error {
	id, err := itHandle(ctx, handle)
	if err != nil {
		return err
	}
	if err := s.findDefaultProject(ctx); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyITProjectHandle, id)
}

// itCreateRESTAPI creates a REST API with one GET /health operation whose upstream is the
// testbench backend, registering it for cleanup before the request.
func (s *Steps) itCreateRESTAPI(ctx context.Context, handle, project, apiContext string) error {
	apiID, err := itHandle(ctx, handle)
	if err != nil {
		return err
	}
	named, err := itHandle(ctx, project)
	if err != nil {
		return err
	}
	selected, err := requiredValue(ctx, keyITProjectHandle)
	if err != nil {
		return err
	}
	if named != selected {
		return fmt.Errorf("the scenario puts the API in project %q but selected project %q", named, selected)
	}
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	apiContext, err = expandCloudValue(ctx, apiContext)
	if err != nil {
		return err
	}
	upstream, err := testbench.UpstreamURL(s.topo, itBackendEndpoint)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: apiID, Actor: resourceActor, Description: "REST API " + apiID,
	}); err != nil {
		return fmt.Errorf("registering API %q for cleanup: %w", apiID, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis", map[string]any{
		"id": apiID, "displayName": "Synthetic " + apiID,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"context":     apiContext, "version": "1.0.0", "projectId": projectID,
		"upstream": map[string]any{"main": map[string]string{"url": upstream}},
		"security": map[string]bool{"enabled": false},
		"operations": []any{map[string]any{
			"name": "getHealth", "request": map[string]string{"method": http.MethodGet, "path": itHealthPath},
		}},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP REST API: %w", err)
	}
	if createdID != apiID {
		deregister(ctx, cleanup.KindAPI, apiID)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindAPI, ID: createdID, Actor: resourceActor, Description: "REST API " + createdID,
		}); err != nil {
			return fmt.Errorf("registering API %q for cleanup: %w", createdID, err)
		}
	}
	return tcontext.Set(ctx, keyAPIID, createdID)
}

// itDeployRESTAPI deploys the scenario's API to the active gateway and waits until the
// deployment is DEPLOYED, so the next data-plane step starts from a live deployment.
func (s *Steps) itDeployRESTAPI(ctx context.Context, handle string) error {
	apiID, err := itHandle(ctx, handle)
	if err != nil {
		return err
	}
	created, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	if created != apiID {
		return fmt.Errorf("the scenario deploys API %q but created API %q", apiID, created)
	}
	return s.deployAndAwait(ctx)
}

// deployAndAwait is the one place the bridge calls the deploy and wait steps.
func (s *Steps) deployAndAwait(ctx context.Context) error {
	if err := s.deployAPI(ctx); err != nil {
		return err
	}
	return s.waitDeployment(ctx)
}

// sendOnce sends one request to a data-plane target and publishes the response for the status
// assertion that follows. A target that starts with "/" is a path on the active gateway.
func (s *Steps) sendOnce(ctx context.Context, method, target string) error {
	if s.funnel == nil {
		return fmt.Errorf("sending %s request: HTTP funnel is required", strings.ToUpper(method))
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return fmt.Errorf("sending request: HTTP method is required")
	}
	target, err := expandCloudValue(ctx, target)
	if err != nil {
		return err
	}
	target, err = s.resolveDataPlaneURL(ctx, target)
	if err != nil {
		return err
	}
	_, err = s.funnel.Send(ctx, httpx.Request{Method: method, URL: target})
	return err
}

// assertResponseStatus checks the exact status of the last published response.
func (s *Steps) assertResponseStatus(ctx context.Context, want int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != want {
		return fmt.Errorf("expected status %d, got %s", want, response.Describe())
	}
	return nil
}

// itHandle expands an IT handle and normalises it the way generated cloud resource names are,
// so the same handle names the same resource in every step that uses it.
func itHandle(ctx context.Context, value string) (string, error) {
	expanded, err := expandCloudValue(ctx, value)
	if err != nil {
		return "", err
	}
	handle := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(expanded), "_", "-"))
	if handle == "" {
		return "", fmt.Errorf("a resource handle is required")
	}
	return handle, nil
}
