/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	defaultAPIOperations = `[{"name":"getPost","request":{"method":"GET","path":"/posts/1"}}]`
	newVersionAPILabel   = "new API version"
)

// apiRef names the scenario-state keys that describe one synthetic REST API, so a step can act on
// the API created first or on a second version of it.
type apiRef struct {
	id         string
	deployment string
	definition string
}

var (
	primaryAPI = apiRef{id: keyAPIID, deployment: keyDeploymentID, definition: keyAPIDefinition}
	versionAPI = apiRef{id: keyVersionAPIID, deployment: keyVersionDeploymentID, definition: keyVersionAPIDefinition}
)

// apiRefFor resolves the label used in step text to the state keys it refers to.
func apiRefFor(label string) apiRef {
	if label == newVersionAPILabel {
		return versionAPI
	}
	return primaryAPI
}

// apiOverrideKeys lists the definition fields a values table may replace; the first three are
// plain strings and the rest carry JSON.
var apiOverrideKeys = map[string]bool{
	"displayName": true, "context": true, "version": true,
	"operations": true, "policies": true, "security": true,
}

// tableValues reads a two-column key/value Gherkin table.
func tableValues(table *godog.Table) (map[string]string, error) {
	values := map[string]string{}
	if table == nil {
		return values, nil
	}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("values table rows must have exactly two cells, got %d", len(row.Cells))
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		if key == "" {
			return nil, fmt.Errorf("values table contains an empty key")
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("values table repeats key %q", key)
		}
		values[key] = row.Cells[1].Value
	}
	return values, nil
}

// applyAPIOverrides replaces fields of a REST API definition with the values a scenario supplies.
func applyAPIOverrides(ctx context.Context, definition map[string]any, overrides map[string]string) error {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !apiOverrideKeys[key] {
			return fmt.Errorf("unsupported REST API value %q", key)
		}
		value, err := expandCloudValue(ctx, overrides[key])
		if err != nil {
			return err
		}
		switch key {
		case "operations", "policies", "security":
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return fmt.Errorf("REST API value %q is not valid JSON: %w", key, err)
			}
			definition[key] = parsed
		default:
			definition[key] = value
		}
	}
	return nil
}

// newAPIDefinition returns the default synthetic REST API with the overrides applied.
func newAPIDefinition(ctx context.Context, id, projectID string, overrides map[string]string) (map[string]any, error) {
	var operations any
	if err := json.Unmarshal([]byte(defaultAPIOperations), &operations); err != nil {
		return nil, err
	}
	definition := map[string]any{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"context":     "/" + id + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream":   map[string]any{"main": map[string]string{"url": "https://jsonplaceholder.typicode.com"}},
		"security":   map[string]bool{"enabled": false},
		"operations": operations,
	}
	if err := applyAPIOverrides(ctx, definition, overrides); err != nil {
		return nil, err
	}
	return definition, nil
}

// registerAPIForCleanup records an API so the cleanup registry deletes it if the scenario does not.
func registerAPIForCleanup(ctx context.Context, id string) error {
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: id, Actor: resourceActor, Description: "REST API " + id,
	}); err != nil {
		return fmt.Errorf("registering API %q for cleanup: %w", id, err)
	}
	return nil
}

func (s *Steps) createDefaultAPI(ctx context.Context) error {
	return s.createAPI(ctx, primaryAPI, nil)
}

func (s *Steps) createAPIWithValues(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	return s.createAPI(ctx, primaryAPI, overrides)
}

// createNewAPIVersion creates a second API that shares the first API's display name and differs
// in version, which is how the platform models a new version of an API.
func (s *Steps) createNewAPIVersion(ctx context.Context, version string, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	definition, err := s.storedDefinition(ctx, primaryAPI)
	if err != nil {
		return err
	}
	if _, set := overrides["displayName"]; !set {
		name, _ := definition["displayName"].(string)
		overrides["displayName"] = name
	}
	overrides["version"] = version
	if _, set := overrides["context"]; !set {
		overrides["context"] = "/${CTX:" + keyVersionAPIID + "}/" + version
	}
	return s.createAPI(ctx, versionAPI, overrides)
}

// createAPI creates the REST API a ref names, applying the overrides to the default definition.
func (s *Steps) createAPI(ctx context.Context, ref apiRef, overrides map[string]string) error {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-api", ref.id); err != nil {
		return err
	}
	apiID, err := requiredValue(ctx, ref.id)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: apiID, Actor: resourceActor, Description: "REST API " + apiID,
	}); err != nil {
		return fmt.Errorf("registering API %q for cleanup: %w", apiID, err)
	}
	definition, err := newAPIDefinition(ctx, apiID, projectID, overrides)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis", definition)
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
		if err := registerAPIForCleanup(ctx, createdID); err != nil {
			return err
		}
		definition["id"] = createdID
	}
	if err := storeDefinition(ctx, ref, definition); err != nil {
		return err
	}
	return tcontext.Set(ctx, ref.id, createdID)
}

// submitAPICreation sends a REST API creation request and publishes the response without
// requiring success, so a scenario can assert on a rejection.
func (s *Steps) submitAPICreation(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-api", keyRejectedAPIID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyRejectedAPIID)
	if err != nil {
		return err
	}
	if err := registerAPIForCleanup(ctx, id); err != nil {
		return err
	}
	definition, err := newAPIDefinition(ctx, id, projectID, overrides)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis", definition)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		deregister(ctx, cleanup.KindAPI, id)
	}
	return nil
}

func (s *Steps) assertCloudResponseStatus(ctx context.Context, want int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != want {
		return fmt.Errorf("expected response status %d, got %s", want, response.Describe())
	}
	return nil
}

func storeDefinition(ctx context.Context, ref apiRef, definition map[string]any) error {
	encoded, err := json.Marshal(definition)
	if err != nil {
		return fmt.Errorf("encoding REST API definition: %w", err)
	}
	return tcontext.Set(ctx, ref.definition, string(encoded))
}

func (s *Steps) storedDefinition(ctx context.Context, ref apiRef) (map[string]any, error) {
	encoded, err := requiredValue(ctx, ref.definition)
	if err != nil {
		return nil, err
	}
	var definition map[string]any
	if err := json.Unmarshal([]byte(encoded), &definition); err != nil {
		return nil, fmt.Errorf("decoding stored REST API definition: %w", err)
	}
	return definition, nil
}

// updateAPI replaces the synthetic REST API's definition with the stored one plus the overrides.
func (s *Steps) updateAPI(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, primaryAPI.id)
	if err != nil {
		return err
	}
	definition, err := s.storedDefinition(ctx, primaryAPI)
	if err != nil {
		return err
	}
	if err := applyAPIOverrides(ctx, definition, overrides); err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPut, "/rest-apis/"+url.PathEscape(id), definition)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return storeDefinition(ctx, primaryAPI, definition)
}

func (s *Steps) deployAPI(ctx context.Context, label string) error {
	return s.deployAPIAs(ctx, apiRefFor(label))
}

func (s *Steps) redeployAPI(ctx context.Context) error {
	return s.deployAPIAs(ctx, primaryAPI)
}

// deployAPIAs deploys the current definition of the API a ref points at to the active gateway and
// keeps the id of the deployment the platform created for it.
func (s *Steps) deployAPIAs(ctx context.Context, ref apiRef) error {
	apiID, err := requiredValue(ctx, ref.id)
	if err != nil {
		return err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	deploymentID, err := s.deployToGateway(ctx, apiID, environment, gatewayID)
	if err != nil {
		return fmt.Errorf("deploying APIP REST API: %w", err)
	}
	return tcontext.Set(ctx, ref.deployment, deploymentID)
}

// deployToGateway deploys the current definition of an API to one gateway of an environment
// through the API's project and returns the id of the deployment created on that gateway. The
// platform renders a build of the definition as it is now when no build is named.
func (s *Steps) deployToGateway(ctx context.Context, apiID, environment, gatewayID string) (string, error) {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return "", err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost,
		"/projects/"+url.PathEscape(projectID)+"/apis/"+url.PathEscape(apiID)+"/deployments",
		map[string]any{"environment": environment, "gateways": []map[string]string{{"gatewayId": gatewayID}}})
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return "", fmt.Errorf("decoding the deployment response: %w", err)
	}
	for _, item := range items {
		if item["gatewayId"] != gatewayID {
			continue
		}
		if deploymentID, _ := item["deploymentId"].(string); deploymentID != "" {
			return deploymentID, nil
		}
	}
	return "", fmt.Errorf("the deployment response lists no deployment on gateway %q", gatewayID)
}

func (s *Steps) undeployAPI(ctx context.Context) error {
	apiID, err := requiredValue(ctx, primaryAPI.id)
	if err != nil {
		return err
	}
	deploymentID, err := requiredValue(ctx, primaryAPI.deployment)
	if err != nil {
		return err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost,
		"/projects/"+url.PathEscape(projectID)+"/apis/"+url.PathEscape(apiID)+
			"/deployments/"+url.PathEscape(deploymentID)+"/undeploy?environment="+url.QueryEscape(environment)+
			"&gatewayId="+url.QueryEscape(gatewayID), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return nil
}

// deploymentRecord reads the record of the deployment a ref points at; it is nil when the list
// does not contain that deployment.
func (s *Steps) deploymentRecord(ctx context.Context, ref apiRef) (map[string]any, error) {
	apiID, err := requiredValue(ctx, ref.id)
	if err != nil {
		return nil, err
	}
	deploymentID, err := requiredValue(ctx, ref.deployment)
	if err != nil {
		return nil, err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return nil, err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet,
		"/rest-apis/"+url.PathEscape(apiID)+"/deployments?gatewayId="+url.QueryEscape(gatewayID), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding APIP deployments: %w", err))
	}
	for _, item := range items {
		if item["deploymentId"] == deploymentID {
			return item, nil
		}
	}
	return nil, nil
}

// deploymentStatus reads the status of the deployment a ref points at; it is empty when the list
// does not contain that deployment.
func (s *Steps) deploymentStatus(ctx context.Context, ref apiRef) (string, error) {
	record, err := s.deploymentRecord(ctx, ref)
	if err != nil || record == nil {
		return "", err
	}
	status, _ := record["status"].(string)
	return status, nil
}

func (s *Steps) waitDeployment(ctx context.Context, label, status string) error {
	ref := apiRefFor(label)
	last, err := retry.Until(ctx, retry.Options{Timeout: 3 * time.Minute, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) { return s.deploymentStatus(ctx, ref) },
		func(got string) bool { return got == status })
	if err != nil {
		return fmt.Errorf("waiting for APIP REST API deployment to be %s: %w", status, err)
	}
	if last != status {
		return fmt.Errorf("APIP REST API deployment never reached %s; last status %q", status, last)
	}
	return nil
}

// keepServingUntilDeployed probes a data-plane URL for as long as the redeployment is in flight
// and fails on the first response that is not the expected status.
func (s *Steps) keepServingUntilDeployed(ctx context.Context, method, target string, want int) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	target, err := expandCloudValue(ctx, target)
	if err != nil {
		return err
	}
	target, err = s.resolveDataPlaneURL(ctx, target)
	if err != nil {
		return err
	}
	last, err := retry.Until(ctx, retry.Options{Timeout: 3 * time.Minute, Interval: 2 * time.Second},
		func(ctx context.Context) (string, error) {
			probe, probeErr := s.client.Do(ctx, httpx.Request{Method: method, URL: target}, 0, 0)
			if probeErr != nil {
				return "", retry.Transient(probeErr)
			}
			if probe.StatusCode != want {
				return "", fmt.Errorf("traffic was interrupted during the redeployment: expected %d, got %s",
					want, probe.Describe())
			}
			return s.deploymentStatus(ctx, primaryAPI)
		},
		func(status string) bool { return status == "DEPLOYED" })
	if err != nil {
		return err
	}
	if last != "DEPLOYED" {
		return fmt.Errorf("the redeployment never reached DEPLOYED; last status %q", last)
	}
	return nil
}

// createAPIKey issues an API key for the synthetic REST API and keeps its value in scenario state
// only; the response is not published so the key never reaches a step's response slot.
func (s *Steps) createAPIKey(ctx context.Context) error {
	apiID, err := requiredValue(ctx, primaryAPI.id)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodPost, "/rest-apis/"+url.PathEscape(apiID)+"/api-keys",
		map[string]string{"displayName": "Synthetic key for " + apiID})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	key, err := stringField(resp, "apiKey")
	if err != nil {
		return fmt.Errorf("creating an API key for %s: %w", apiID, err)
	}
	return tcontext.Set(ctx, keyAPIKey, key)
}

// sendWithAPIKeyUntilStatus invokes a data-plane URL with the stored API key in the named header.
func (s *Steps) sendWithAPIKeyUntilStatus(ctx context.Context, method, target, header string, want int) error {
	key, err := requiredValue(ctx, keyAPIKey)
	if err != nil {
		return err
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("sending request: API key header name is required")
	}
	return s.sendUntilStatusWith(ctx, method, target, want, map[string]string{header: key})
}
