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
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// echoBackendEnv overrides the backend deployment overrides are pointed at.
	echoBackendEnv = "CLOUD_IT_ECHO_BACKEND_URL"
	// defaultEchoBackend is a public service that answers with the URL it was asked for.
	defaultEchoBackend = "https://httpbin.org/anything"

	deployedTimeout = 5 * time.Minute
)

// buildIDPattern is the shape of a build identifier: the day it was prepared and that day's index.
var buildIDPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[1-9]\d*$`)

// deploymentRequestKeys lists the values a deployment request table may carry.
var deploymentRequestKeys = map[string]bool{
	"environment": true, "gatewayId": true, "gatewayIds": true,
	"buildId": true, "fromEnvironment": true, "parameters": true,
}

// apiScopePath is the project-and-API prefix of the cloud-extension build and deployment routes.
func apiScopePath(ctx context.Context) (string, error) {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return "", err
	}
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return "", err
	}
	return "/projects/" + url.PathEscape(projectID) + "/apis/" + url.PathEscape(apiID), nil
}

// optionalValue reads a scenario value that may be unset.
func optionalValue(ctx context.Context, key string) string {
	value, err := tcontext.ResolveString(ctx, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// chooseUnusedAPIHandle picks a REST API handle that no API has, for a request that is refused
// before it reaches the API.
func (s *Steps) chooseUnusedAPIHandle(ctx context.Context) error {
	return generateResourceAndStore(ctx, "syn-api", keyAPIID)
}

// ownBuildState drops the builds a previous scenario recorded. Scenario state lives as long as the
// runner and a build id is a day and an index that is only unique within one API, so builds
// recorded for another API must not be mistaken for this API's.
func ownBuildState(ctx context.Context) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	if optionalValue(ctx, keyBuildsOwner) == apiID {
		return nil
	}
	for _, key := range []string{keyBuildIDs, keyBuildID, keyBuildMetadata} {
		if err := tcontext.Set(ctx, key, ""); err != nil {
			return err
		}
	}
	return tcontext.Set(ctx, keyBuildsOwner, apiID)
}

// prepareBuild prepares a build of the synthetic REST API, optionally recording metadata with it.
func (s *Steps) prepareBuild(ctx context.Context, table *godog.Table) error {
	metadata, err := tableValues(table)
	if err != nil {
		return err
	}
	scope, err := apiScopePath(ctx)
	if err != nil {
		return err
	}
	if err := ownBuildState(ctx); err != nil {
		return err
	}
	var body any
	recorded := map[string]any{}
	for key, value := range metadata {
		expanded, err := expandCloudValue(ctx, value)
		if err != nil {
			return err
		}
		recorded[key] = expanded
	}
	if len(recorded) > 0 {
		body = map[string]any{"metadata": recorded}
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, scope+"/builds", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	buildID, err := stringField(resp, "buildId")
	if err != nil {
		return fmt.Errorf("preparing a build: %w", err)
	}
	encoded := ""
	if len(recorded) > 0 {
		raw, err := json.Marshal(recorded)
		if err != nil {
			return err
		}
		encoded = string(raw)
	}
	if err := tcontext.Set(ctx, keyBuildMetadata, encoded); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyBuildID, buildID); err != nil {
		return err
	}
	if prepared := optionalValue(ctx, keyBuildIDs); prepared != "" {
		buildID = prepared + "," + buildID
	}
	return tcontext.Set(ctx, keyBuildIDs, buildID)
}

// latestPreparedBuild is the most recently prepared build of the scenario.
func latestPreparedBuild(ctx context.Context) (string, error) {
	prepared, err := requiredValue(ctx, keyBuildIDs)
	if err != nil {
		return "", fmt.Errorf("no build has been prepared: %w", err)
	}
	ids := strings.Split(prepared, ",")
	return ids[len(ids)-1], nil
}

func (s *Steps) assertBuildIDShape(ctx context.Context) error {
	id, err := latestPreparedBuild(ctx)
	if err != nil {
		return err
	}
	if !buildIDPattern.MatchString(id) {
		return fmt.Errorf("build id %q is not a day and index such as 2026-01-31-1", id)
	}
	return nil
}

// fetchBuilds lists the synthetic REST API's builds, newest first.
func (s *Steps) fetchBuilds(ctx context.Context) ([]map[string]any, error) {
	scope, err := apiScopePath(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, scope+"/builds", nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding builds: %w", err))
	}
	return items, nil
}

func buildPositions(items []map[string]any) map[string]int {
	positions := make(map[string]int, len(items))
	for index, item := range items {
		if id, ok := item["buildId"].(string); ok {
			positions[id] = index
		}
	}
	return positions
}

// awaitBuilds waits until every given build is listed and returns the list.
func (s *Steps) awaitBuilds(ctx context.Context, ids []string) ([]map[string]any, error) {
	var listed []map[string]any
	err := retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		s.fetchBuilds,
		func(items []map[string]any) bool {
			positions := buildPositions(items)
			for _, id := range ids {
				if _, ok := positions[id]; !ok {
					return false
				}
			}
			listed = items
			return true
		}, "waiting for builds "+strings.Join(ids, ", ")+" to be listed")
	return listed, err
}

// assertBuildListed checks that the latest prepared build is listed with the metadata it was
// prepared with.
func (s *Steps) assertBuildListed(ctx context.Context) error {
	id, err := latestPreparedBuild(ctx)
	if err != nil {
		return err
	}
	listed, err := s.awaitBuilds(ctx, []string{id})
	if err != nil {
		return err
	}
	encoded := optionalValue(ctx, keyBuildMetadata)
	if encoded == "" {
		return nil
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(encoded), &want); err != nil {
		return err
	}
	got, _ := listed[buildPositions(listed)[id]]["metadata"].(map[string]any)
	if !reflect.DeepEqual(want, got) {
		return fmt.Errorf("build %s lists metadata %v, want %v", id, got, want)
	}
	return nil
}

// assertBuildsNewestFirst checks that every prepared build is listed and that a later build is
// listed before an earlier one.
func (s *Steps) assertBuildsNewestFirst(ctx context.Context) error {
	prepared, err := requiredValue(ctx, keyBuildIDs)
	if err != nil {
		return fmt.Errorf("no build has been prepared: %w", err)
	}
	ids := strings.Split(prepared, ",")
	if len(ids) < 2 {
		return fmt.Errorf("ordering needs at least two prepared builds, have %d", len(ids))
	}
	listed, err := s.awaitBuilds(ctx, ids)
	if err != nil {
		return err
	}
	positions := buildPositions(listed)
	for i := 1; i < len(ids); i++ {
		if positions[ids[i]] >= positions[ids[i-1]] {
			return fmt.Errorf("build %s was prepared after %s but is not listed before it (positions %d and %d)",
				ids[i], ids[i-1], positions[ids[i]], positions[ids[i-1]])
		}
	}
	return nil
}

// buildDeploymentRequest turns a values table into the body of a deployment request.
func buildDeploymentRequest(ctx context.Context, values map[string]string) (map[string]any, error) {
	expanded := make(map[string]string, len(values))
	for key, value := range values {
		if !deploymentRequestKeys[key] {
			return nil, fmt.Errorf("unsupported deployment request value %q", key)
		}
		resolved, err := expandCloudValue(ctx, value)
		if err != nil {
			return nil, err
		}
		expanded[key] = strings.TrimSpace(resolved)
	}
	body := map[string]any{}
	for _, key := range []string{"environment", "buildId", "fromEnvironment"} {
		if expanded[key] != "" {
			body[key] = expanded[key]
		}
	}
	if expanded["gatewayId"] != "" && expanded["gatewayIds"] != "" {
		return nil, fmt.Errorf("a deployment request names either gatewayId or gatewayIds, not both")
	}
	var gateways []map[string]any
	if expanded["gatewayId"] != "" {
		gateways = append(gateways, map[string]any{"gatewayId": expanded["gatewayId"]})
	}
	for _, id := range strings.Split(expanded["gatewayIds"], ",") {
		if id = strings.TrimSpace(id); id != "" {
			gateways = append(gateways, map[string]any{"gatewayId": id})
		}
	}
	if expanded["parameters"] != "" {
		if len(gateways) != 1 {
			return nil, fmt.Errorf("parameters apply to exactly one named gateway, have %d", len(gateways))
		}
		var parameters map[string]string
		if err := json.Unmarshal([]byte(expanded["parameters"]), &parameters); err != nil {
			return nil, fmt.Errorf("deployment parameters are not a JSON object of strings: %w", err)
		}
		gateways[0]["parameters"] = parameters
	}
	if len(gateways) > 0 {
		body["gateways"] = gateways
	}
	return body, nil
}

// requestDeployment asks for a deployment through the project and environment route and publishes
// the response without requiring success, so a scenario can assert on a refusal.
func (s *Steps) requestDeployment(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	body, err := buildDeploymentRequest(ctx, values)
	if err != nil {
		return err
	}
	scope, err := apiScopePath(ctx)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, scope+"/deployments", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil
	}
	return recordDeployments(ctx, resp)
}

// recordDeployments keeps the deployments a successful request created: which deployment runs
// on each gateway, which build they run, and the one on the active gateway.
func recordDeployments(ctx context.Context, resp *httpx.Response) error {
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding the deployment response: %w", err)
	}
	targets := map[string]string{}
	build := ""
	for _, item := range items {
		gatewayID, _ := item["gatewayId"].(string)
		deploymentID, _ := item["deploymentId"].(string)
		if gatewayID == "" || deploymentID == "" {
			continue
		}
		targets[gatewayID] = deploymentID
		if id, _ := item["buildId"].(string); id != "" {
			build = id
		}
		if gatewayID == optionalValue(ctx, keyGatewayID) {
			if err := tcontext.Set(ctx, keyDeploymentID, deploymentID); err != nil {
				return err
			}
		}
	}
	encoded, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyDeployedTargets, string(encoded)); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyDeployedBuildID, build)
}

// deploymentEntry finds the response entry for a gateway.
func deploymentEntry(resp *httpx.Response, gatewayID string) (map[string]any, error) {
	items, err := listItems(resp)
	if err != nil {
		return nil, fmt.Errorf("decoding the deployment response: %w", err)
	}
	for _, item := range items {
		if item["gatewayId"] == gatewayID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("the deployment response does not list gateway %q: %s", gatewayID, resp.Describe())
}

func (s *Steps) assertDeploymentListsActiveGateway(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	entry, err := deploymentEntry(response, gatewayID)
	if err != nil {
		return err
	}
	if id, _ := entry["deploymentId"].(string); id == "" {
		return fmt.Errorf("the deployment on gateway %q has no deployment id: %s", gatewayID, response.Describe())
	}
	return nil
}

func (s *Steps) assertDeploymentCount(ctx context.Context, want int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	items, err := listItems(response)
	if err != nil {
		return fmt.Errorf("decoding the deployment response: %w", err)
	}
	if len(items) != want {
		return fmt.Errorf("expected %d deployments, got %d: %s", want, len(items), response.Describe())
	}
	return nil
}

// recordRenderedBuild keeps the build the last deployment rendered from the API's definition as
// the scenario's latest build, so a later step can list it or deploy it by name. Each deployment
// without a build named must render a build of its own.
func (s *Steps) recordRenderedBuild(ctx context.Context) error {
	if err := ownBuildState(ctx); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyDeployedBuildID)
	if err != nil {
		return fmt.Errorf("the deployment did not report a build: %w", err)
	}
	if !buildIDPattern.MatchString(id) {
		return fmt.Errorf("build id %q is not a day and index such as 2026-01-31-1", id)
	}
	rendered := optionalValue(ctx, keyBuildIDs)
	if rendered != "" {
		for _, known := range strings.Split(rendered, ",") {
			if known == id {
				return fmt.Errorf("the deployment reused build %q instead of rendering a new one", id)
			}
		}
		id = rendered + "," + id
	}
	if err := tcontext.Set(ctx, keyBuildIDs, id); err != nil {
		return err
	}
	latest, err := latestPreparedBuild(ctx)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyBuildID, latest)
}

func (s *Steps) assertDeployedBuildIsPrepared(ctx context.Context) error {
	prepared, err := latestPreparedBuild(ctx)
	if err != nil {
		return err
	}
	deployed, err := requiredValue(ctx, keyDeployedBuildID)
	if err != nil {
		return fmt.Errorf("the deployment did not report a build: %w", err)
	}
	if deployed != prepared {
		return fmt.Errorf("the deployment runs build %q, want the prepared build %q", deployed, prepared)
	}
	return nil
}

// assertDeploymentReportsEchoEndpoint checks that the deployment records the echo backend as the
// production endpoint it was made with.
func (s *Steps) assertDeploymentReportsEchoEndpoint(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	entry, err := deploymentEntry(response, gatewayID)
	if err != nil {
		return err
	}
	want, err := requiredValue(ctx, keyEchoBackendURL)
	if err != nil {
		return err
	}
	if got, _ := entry["endpointUrl"].(string); got != want {
		return fmt.Errorf("the deployment reports production endpoint %q, want %q", got, want)
	}
	return nil
}

// useEchoBackend records the backend deployment overrides are pointed at.
func (s *Steps) useEchoBackend(ctx context.Context) error {
	backend := strings.TrimSpace(os.Getenv(echoBackendEnv))
	if backend == "" {
		backend = defaultEchoBackend
	}
	parsed, err := url.Parse(backend)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be an https URL with a host", echoBackendEnv)
	}
	return tcontext.Set(ctx, keyEchoBackendURL, strings.TrimRight(backend, "/"))
}

// assertEchoBackendReceived checks that the data-plane response comes from the echo backend and
// that the backend saw the given path below its own base path.
func (s *Steps) assertEchoBackendReceived(ctx context.Context, path string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	backend, err := requiredValue(ctx, keyEchoBackendURL)
	if err != nil {
		return err
	}
	base, err := url.Parse(backend)
	if err != nil {
		return err
	}
	var echoed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(response.Body, &echoed); err != nil || echoed.URL == "" {
		return fmt.Errorf("the response is not an echo of the request: %s", response.Describe())
	}
	received, err := url.Parse(echoed.URL)
	if err != nil {
		return fmt.Errorf("the echoed URL is not valid: %w", err)
	}
	if want := strings.TrimRight(base.Path, "/") + path; received.Path != want {
		return fmt.Errorf("the echo backend received path %q, want %q", received.Path, want)
	}
	return nil
}

// stages reads the deployment stages of the synthetic REST API through its project.
func (s *Steps) stages(ctx context.Context) ([]map[string]any, error) {
	scope, err := apiScopePath(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, scope+"/deployments", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}
	return listItems(resp)
}

// pipelineEnvironments reads the environments of the deployment pipeline a project is bound to.
func (s *Steps) pipelineEnvironments(ctx context.Context) (map[string]bool, error) {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return nil, err
	}
	bound, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/deployment-pipeline", nil)
	if err != nil {
		return nil, err
	}
	if bound.StatusCode != http.StatusOK {
		return nil, responseError(bound)
	}
	name, err := stringField(bound, "pipeline")
	if err != nil {
		return nil, fmt.Errorf("reading the project's deployment pipeline: %w", err)
	}
	listed, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/pipelines", nil)
	if err != nil {
		return nil, err
	}
	if listed.StatusCode != http.StatusOK {
		return nil, responseError(listed)
	}
	var document struct {
		List []struct {
			Name           string `json:"name"`
			PromotionPaths []struct {
				SourceEnvironment  string   `json:"sourceEnvironment"`
				TargetEnvironments []string `json:"targetEnvironments"`
			} `json:"promotionPaths"`
		} `json:"list"`
	}
	if err := json.Unmarshal(listed.Body, &document); err != nil {
		return nil, fmt.Errorf("decoding deployment pipelines: %w", err)
	}
	for _, pipeline := range document.List {
		if pipeline.Name != name {
			continue
		}
		environments := map[string]bool{}
		for _, path := range pipeline.PromotionPaths {
			environments[path.SourceEnvironment] = true
			for _, target := range path.TargetEnvironments {
				environments[target] = true
			}
		}
		return environments, nil
	}
	return nil, fmt.Errorf("deployment pipeline %q is not listed", name)
}

// assertStagesMatchPipeline checks that the deployment listing has exactly one stage for each
// environment of the project's deployment pipeline.
func (s *Steps) assertStagesMatchPipeline(ctx context.Context) error {
	stages, err := s.stages(ctx)
	if err != nil {
		return err
	}
	want, err := s.pipelineEnvironments(ctx)
	if err != nil {
		return err
	}
	got := map[string]bool{}
	for _, stage := range stages {
		environment, _ := stage["environment"].(string)
		if environment == "" {
			return fmt.Errorf("a deployment stage has no environment")
		}
		if got[environment] {
			return fmt.Errorf("environment %q has more than one deployment stage", environment)
		}
		if _, isList := stage["gateways"].([]any); !isList {
			return fmt.Errorf("the stage for environment %q does not list its gateways", environment)
		}
		got[environment] = true
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("deployment stages cover environments %v, the pipeline has %v", sortedKeys(got), sortedKeys(want))
	}
	return nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// assertActiveGatewayStageStatus checks the status the deployment listing gives the active
// gateway in its own environment.
func (s *Steps) assertActiveGatewayStageStatus(ctx context.Context, want string) error {
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	stages, err := s.stages(ctx)
	if err != nil {
		return err
	}
	for _, stage := range stages {
		if stage["environment"] != environment {
			continue
		}
		gateways, _ := stage["gateways"].([]any)
		for _, raw := range gateways {
			gateway, _ := raw.(map[string]any)
			if gateway["gatewayId"] != gatewayID {
				continue
			}
			if got, _ := gateway["status"].(string); got != want {
				return fmt.Errorf("gateway %q is %q in environment %q, want %q", gatewayID, got, environment, want)
			}
			return nil
		}
		return fmt.Errorf("the stage for environment %q does not list gateway %q", environment, gatewayID)
	}
	return fmt.Errorf("no deployment stage for environment %q", environment)
}

// recordedTargets reads the deployments the last successful request created, by gateway.
func recordedTargets(ctx context.Context) (map[string]string, error) {
	encoded, err := requiredValue(ctx, keyDeployedTargets)
	if err != nil {
		return nil, fmt.Errorf("no deployment has been requested: %w", err)
	}
	targets := map[string]string{}
	if err := json.Unmarshal([]byte(encoded), &targets); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("the deployment request created no deployment")
	}
	return targets, nil
}

// waitDeployedOnTargets waits until every deployment the last request created is DEPLOYED.
func (s *Steps) waitDeployedOnTargets(ctx context.Context) error {
	targets, err := recordedTargets(ctx)
	if err != nil {
		return err
	}
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: deployedTimeout, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/rest-apis/"+url.PathEscape(apiID)+"/deployments", nil)
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return "", retry.Transient(responseError(resp))
			}
			items, err := listItems(resp)
			if err != nil {
				return "", retry.Transient(fmt.Errorf("decoding APIP deployments: %w", err))
			}
			status := map[string]string{}
			for _, item := range items {
				deploymentID, _ := item["deploymentId"].(string)
				state, _ := item["status"].(string)
				status[deploymentID] = state
			}
			pending := make([]string, 0, len(targets))
			for gatewayID, deploymentID := range targets {
				if status[deploymentID] != "DEPLOYED" {
					pending = append(pending, gatewayID+": "+status[deploymentID])
				}
			}
			sort.Strings(pending)
			return strings.Join(pending, ", "), nil
		},
		func(pending string) bool { return pending == "" },
		"waiting for every requested deployment to be DEPLOYED")
}
