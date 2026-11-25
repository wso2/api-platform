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

	// backendEnv overrides the backend the synthetic REST APIs call.
	backendEnv = "CLOUD_IT_BACKEND_URL"
	// defaultBackend is the testbench backend service, which answers any path with the request it received.
	defaultBackend = "https://development-wso2cloud.gateway.dev.cloud.wso2.com/apip-testbench-mocks/backend"
)

// apiRef names the scenario-state keys that describe one synthetic API, so a step can act on the
// REST API created first, on a second version of it, or on a GraphQL API.
type apiRef struct {
	id         string
	deployment string
	definition string
	// kind is the artifact kind the project deployment routes are told; empty means a REST API.
	kind string
	// collection is the platform route that reads the API and its deployments.
	collection string
	// builds lists the builds the scenario saw for the API, oldest first, and buildsOwner the API
	// they belong to. Both are empty for an API whose builds are not followed.
	builds      string
	buildsOwner string
}

var (
	primaryAPI = apiRef{
		id: keyAPIID, deployment: keyDeploymentID, definition: keyAPIDefinition,
		collection: restAPIsCollection, builds: keyBuildIDs, buildsOwner: keyBuildsOwner,
	}
	versionAPI = apiRef{
		id: keyVersionAPIID, deployment: keyVersionDeploymentID, definition: keyVersionAPIDefinition,
		collection: restAPIsCollection,
	}
	graphQLAPI = apiRef{
		id: keyGraphQLAPIID, deployment: keyGraphQLDeploymentID, definition: keyGraphQLDefinition,
		kind: kindGraphQLAPI, collection: graphQLAPIsCollection,
		builds: keyGraphQLBuildIDs, buildsOwner: keyGraphQLBuildsOwner,
	}
)

// apiRefFor resolves the label used in step text to the state keys it refers to.
func apiRefFor(label string) apiRef {
	switch label {
	case newVersionAPILabel:
		return versionAPI
	case graphQLAPILabel:
		return graphQLAPI
	default:
		return primaryAPI
	}
}

// withKind adds the artifact kind to a project deployment route; a REST API needs none.
func withKind(path, kind string) string {
	if kind == "" {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "kind=" + url.QueryEscape(kind)
}

// apiOverrideKeys lists the definition fields a values table may replace: the first three are
// plain strings, upstreamUrl is the main upstream's URL, and the rest carry JSON.
var apiOverrideKeys = map[string]bool{
	"displayName": true, "context": true, "version": true, "upstreamUrl": true,
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
		case "upstreamUrl":
			definition["upstream"] = map[string]any{"main": map[string]string{"url": value}}
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
	backend, err := testbenchURL(backendEnv, defaultBackend)
	if err != nil {
		return nil, err
	}
	definition := map[string]any{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"context":     "/" + id + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream":   map[string]any{"main": map[string]string{"url": backend}},
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
	if refusedCreate(resp) {
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
	resp, definition, err := s.sendAPIUpdate(ctx, table)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return storeDefinition(ctx, primaryAPI, definition)
}

// tryUpdateAPI sends an update the platform may refuse and leaves its answer for the assertion
// steps. The stored definition stays as it was, since a refused update changes nothing.
func (s *Steps) tryUpdateAPI(ctx context.Context, table *godog.Table) error {
	_, _, err := s.sendAPIUpdate(ctx, table)
	return err
}

// sendAPIUpdate sends the stored definition of the synthetic REST API with the overrides applied,
// and returns the answer and the definition it sent.
func (s *Steps) sendAPIUpdate(ctx context.Context, table *godog.Table) (*httpx.Response, map[string]any, error) {
	// A request that fails before it is sent must not leave an earlier answer to be asserted.
	httpx.ClearPublished(ctx)
	overrides, err := tableValues(table)
	if err != nil {
		return nil, nil, err
	}
	id, err := requiredValue(ctx, primaryAPI.id)
	if err != nil {
		return nil, nil, err
	}
	definition, err := s.storedDefinition(ctx, primaryAPI)
	if err != nil {
		return nil, nil, err
	}
	if err := applyAPIOverrides(ctx, definition, overrides); err != nil {
		return nil, nil, err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPut, "/rest-apis/"+url.PathEscape(id), definition)
	if err != nil {
		return nil, nil, err
	}
	return resp, definition, nil
}

// assertUpstreamIsTheBackend checks that the platform holds the testbench backend as the synthetic
// REST API's main upstream.
func (s *Steps) assertUpstreamIsTheBackend(ctx context.Context) error {
	backend, err := testbenchURL(backendEnv, defaultBackend)
	if err != nil {
		return err
	}
	definition, err := s.readDefinitionOf(ctx, primaryAPI)
	if err != nil {
		return err
	}
	upstream, _ := definition["upstream"].(map[string]any)
	main, _ := upstream["main"].(map[string]any)
	if got, _ := main["url"].(string); strings.TrimRight(got, "/") != backend {
		return fmt.Errorf("expected the main upstream %q, the platform holds %v", backend, definition["upstream"])
	}
	return nil
}

func (s *Steps) deployAPI(ctx context.Context, label string) error {
	return s.deployAPIAs(ctx, apiRefFor(label))
}

func (s *Steps) redeployAPI(ctx context.Context) error {
	return s.deployAPIAs(ctx, primaryAPI)
}

// deployAPIAs deploys the current definition of the API a ref points at to the active gateway,
// keeps the id of the deployment the platform created for it, and adds the build it rendered to
// the builds the scenario follows for the API.
func (s *Steps) deployAPIAs(ctx context.Context, ref apiRef) error {
	deployed, err := s.deployRefToActiveGateway(ctx, ref, "")
	if err != nil {
		return err
	}
	if ref.builds == "" || deployed.build == "" {
		return nil
	}
	return appendBuild(ctx, ref, deployed.build)
}

// deployRefToActiveGateway deploys the API a ref points at to the active gateway, naming a build
// when one is given, and keeps the id of the deployment created on that gateway.
func (s *Steps) deployRefToActiveGateway(ctx context.Context, ref apiRef, buildID string) (deployment, error) {
	apiID, err := requiredValue(ctx, ref.id)
	if err != nil {
		return deployment{}, err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return deployment{}, err
	}
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return deployment{}, fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	deployed, err := s.deployArtifact(ctx, ref.kind, apiID, environment, gatewayID, buildID)
	if err != nil {
		return deployment{}, fmt.Errorf("deploying APIP API %s: %w", apiID, err)
	}
	return deployed, tcontext.Set(ctx, ref.deployment, deployed.id)
}

// deployment is what a deployment request created on one gateway: the deployment and the build
// it runs.
type deployment struct {
	id    string
	build string
}

// deployToGateway deploys the current definition of a REST API to one gateway of an environment
// through the API's project and returns the id of the deployment created on that gateway.
func (s *Steps) deployToGateway(ctx context.Context, apiID, environment, gatewayID string) (string, error) {
	deployed, err := s.deployArtifact(ctx, "", apiID, environment, gatewayID, "")
	return deployed.id, err
}

// deployArtifact deploys an API of a kind to one gateway of an environment through the API's
// project. The platform renders a build of the definition as it is now when no build is named.
func (s *Steps) deployArtifact(ctx context.Context, kind, apiID, environment, gatewayID, buildID string) (deployment, error) {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return deployment{}, err
	}
	body := map[string]any{"environment": environment, "gateways": []map[string]string{{"gatewayId": gatewayID}}}
	if buildID != "" {
		body["buildId"] = buildID
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost,
		withKind("/projects/"+url.PathEscape(projectID)+"/apis/"+url.PathEscape(apiID)+"/deployments", kind), body)
	if err != nil {
		return deployment{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return deployment{}, responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return deployment{}, fmt.Errorf("decoding the deployment response: %w", err)
	}
	for _, item := range items {
		if item["gatewayId"] != gatewayID {
			continue
		}
		if id, _ := item["deploymentId"].(string); id != "" {
			build, _ := item["buildId"].(string)
			return deployment{id: id, build: build}, nil
		}
	}
	return deployment{}, fmt.Errorf("the deployment response lists no deployment on gateway %q", gatewayID)
}

// undeployAPI stops the deployment of the labelled API on the active gateway.
func (s *Steps) undeployAPI(ctx context.Context, label string) error {
	resp, err := s.sendUndeploy(ctx, apiRefFor(label), "")
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return nil
}

// tryUndeployAPI asks to stop the synthetic REST API's deployment on the active gateway, naming
// the given environment or else the gateway's own, and leaves the answer for the scenario.
func (s *Steps) tryUndeployAPI(ctx context.Context, environment string) error {
	expanded, err := expandCloudValue(ctx, environment)
	if err != nil {
		return err
	}
	_, err = s.sendUndeploy(ctx, primaryAPI, strings.TrimSpace(expanded))
	return err
}

// sendUndeploy sends the undeploy of the deployment a ref points at on the active gateway through
// the API's project, in the active gateway's environment unless another is given.
func (s *Steps) sendUndeploy(ctx context.Context, ref apiRef, environment string) (*httpx.Response, error) {
	deploymentID, err := requiredValue(ctx, ref.deployment)
	if err != nil {
		return nil, err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return nil, err
	}
	if environment == "" {
		if environment, err = requiredValue(ctx, keyGatewayEnv); err != nil {
			return nil, fmt.Errorf("the active gateway does not report its environment: %w", err)
		}
	}
	scope, err := refScopePath(ctx, ref)
	if err != nil {
		return nil, err
	}
	return s.doAuthenticated(ctx, http.MethodPost, withKind(
		scope+"/deployments/"+url.PathEscape(deploymentID)+"/undeploy?environment="+url.QueryEscape(environment)+
			"&gatewayId="+url.QueryEscape(gatewayID), ref.kind), nil)
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
		ref.collection+"/"+url.PathEscape(apiID)+"/deployments?gatewayId="+url.QueryEscape(gatewayID), nil)
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
		return fmt.Errorf("waiting for the %s deployment to be %s: %w", label, status, err)
	}
	if last != status {
		return fmt.Errorf("the %s deployment never reached %s; last status %q", label, status, last)
	}
	return nil
}

const (
	redeployProbeInterval  = retry.BaseInterval
	redeployStatusInterval = 2 * time.Second
	redeployTolerableDrops = 1
)

// keepServingUntilDeployed probes a data-plane URL for as long as the redeployment is in flight
// and fails on the first response that is not the expected status, or when more than
// redeployTolerableDrops requests in a row get no answer at all.
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
	var (
		status     string
		polledAt   time.Time
		unanswered int
	)
	last, err := retry.Until(ctx, retry.Options{Timeout: 3 * time.Minute, Interval: redeployProbeInterval},
		func(ctx context.Context) (string, error) {
			probe, probeErr := s.client.Do(ctx, httpx.Request{Method: method, URL: target}, 0, 0)
			switch {
			case probeErr != nil:
				unanswered++
				if unanswered > redeployTolerableDrops {
					return "", fmt.Errorf("traffic was interrupted during the redeployment: %d requests in a row got no answer: %w",
						unanswered, probeErr)
				}
			case probe.StatusCode != want:
				return "", fmt.Errorf("traffic was interrupted during the redeployment: expected %d, got %s",
					want, probe.Describe())
			default:
				unanswered = 0
			}
			if status != "" && time.Since(polledAt) < redeployStatusInterval {
				return status, nil
			}
			current, statusErr := s.deploymentStatus(ctx, primaryAPI)
			if statusErr != nil {
				return "", statusErr
			}
			status, polledAt = current, time.Now()
			return status, nil
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

// keepReturningFor sends a data-plane request for the whole window and fails on the first answer
// that is not the expected status, proving the gateway keeps answering it that way over time.
func (s *Steps) keepReturningFor(ctx context.Context, method, target string, want, seconds int) error {
	if seconds < 1 {
		return fmt.Errorf("a window of %d seconds is too short to observe", seconds)
	}
	request, err := s.prepareDataPlaneRequest(ctx, httpx.Request{Method: method, URL: target})
	if err != nil {
		return err
	}
	// A probe that gets no answer is not the expected status, so it fails the window too.
	noRetry := retry.Options{Retryable: func(error) bool { return false }}
	last, err := retry.Never(ctx, noRetry, time.Duration(seconds)*time.Second,
		func(ctx context.Context) (*httpx.Response, error) {
			return s.funnel.Send(ctx, request)
		},
		func(response *httpx.Response) bool { return response.StatusCode != want })
	if err != nil {
		if last != nil {
			return fmt.Errorf("%s %s did not keep returning %d: %w (last %s)", request.Method, request.URL, want, err, last.Describe())
		}
		return fmt.Errorf("%s %s did not keep returning %d: %w", request.Method, request.URL, want, err)
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

// credentialKeys maps each credential a gateway step can send to its scenario-state key.
var credentialKeys = map[string]string{
	"API key":                              keyAPIKey,
	"previous API key":                     keyPreviousAPIKey,
	"subscription token":                   keyConsumerSubscriptionToken,
	"previous subscription token":          keyPreviousSubscriptionToken,
	"access token":                         keyAccessToken,
	"access token with a forged signature": keyAccessToken,
}

// sendWithCredentialUntilStatus invokes a data-plane URL with a stored credential in the named
// header. An access token is sent as a bearer token.
func (s *Steps) sendWithCredentialUntilStatus(ctx context.Context, method, target, header, which string, want int) error {
	stateKey, ok := credentialKeys[which]
	if !ok {
		return fmt.Errorf("unsupported credential %q", which)
	}
	key, err := requiredValue(ctx, stateKey)
	if err != nil {
		return err
	}
	if stateKey == keyAccessToken {
		if which == "access token with a forged signature" {
			if key, err = forgeSignature(key); err != nil {
				return err
			}
		}
		key = "Bearer " + key
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("sending request: header name is required")
	}
	return s.sendUntilStatusWith(ctx, method, target, want, map[string]string{header: key})
}

// forgeSignature returns the JWT with its signature changed.
func forgeSignature(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		return "", fmt.Errorf("the access token is not a signed JWT")
	}
	signature := []byte(parts[2])
	if signature[0] == 'A' {
		signature[0] = 'B'
	} else {
		signature[0] = 'A'
	}
	parts[2] = string(signature)
	return strings.Join(parts, "."), nil
}
