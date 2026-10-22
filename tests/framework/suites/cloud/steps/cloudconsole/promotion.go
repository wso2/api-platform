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
	keyPipelineID            = "cloudPipelineID"
	keyProviderID            = "cloudProviderID"
	keySyntheticEnvironments = "cloudSyntheticEnvironments"

	// The numbered keys name the nth synthetic environment, its gateway and that gateway's endpoint.
	keySyntheticEnvironmentPrefix = "cloudSyntheticEnvironment"
	keySyntheticGatewayPrefix     = "cloudSyntheticGateway"
	keySyntheticEndpointPrefix    = "cloudSyntheticEndpoint"

	// maxPromotionEnvironments bounds the numbered keys a scenario can address and the state a
	// new scenario clears.
	maxPromotionEnvironments = 5
)

// A project holds the pipeline it is bound to, a pipeline holds the environments it names and
// is what makes a gateway its environment's default, and a gateway must be gone before its
// environment is. A promotion scenario therefore deletes project, pipeline, gateways and
// environments in that order, which the framework's own order (project last) cannot express.
var (
	kindPromotionProject = cleanup.Kind{Name: cleanup.KindProject.Name, Order: 86}
	kindPipeline         = cleanup.Kind{Name: "pipeline", Order: 87}
)

// stillDeployed are the statuses a deployment listing gives a gateway that serves, or is about to serve, the API.
var stillDeployed = map[string]bool{"DEPLOYED": true, "DEPLOYING": true}

func numberedKey(prefix string, n int) string { return fmt.Sprintf("%s%d", prefix, n) }

// resetPromotionState drops what an earlier scenario of the runner recorded about its
// environments, gateways and deployments, so a numbered key can only name this scenario's own.
func resetPromotionState(ctx context.Context) error {
	keys := []string{keySyntheticEnvironments, keyPipelineID, keyDeployedTargets, keyDeployedBuildID, keyDeploymentID}
	for n := 1; n <= maxPromotionEnvironments; n++ {
		keys = append(keys, numberedKey(keySyntheticEnvironmentPrefix, n),
			numberedKey(keySyntheticGatewayPrefix, n), numberedKey(keySyntheticEndpointPrefix, n))
	}
	for _, key := range keys {
		if err := tcontext.Set(ctx, key, ""); err != nil {
			return err
		}
	}
	return nil
}

// createSyntheticEnvironments creates environments, each registered for cleanup before it is
// requested, and records them as synthetic environments 1 to count. The first is also the
// scenario's synthetic environment for the steps that take no number.
func (s *Steps) createSyntheticEnvironments(ctx context.Context, count int) error {
	if count < 1 || count > maxPromotionEnvironments {
		return fmt.Errorf("a scenario creates between 1 and %d synthetic environments, asked for %d", maxPromotionEnvironments, count)
	}
	if err := resetPromotionState(ctx); err != nil {
		return err
	}
	names := make([]string, 0, count)
	for n := 1; n <= count; n++ {
		if err := s.createEnvironment(ctx); err != nil {
			return err
		}
		name, err := requiredValue(ctx, keyEnvironmentName)
		if err != nil {
			return err
		}
		names = append(names, name)
		if err := tcontext.Set(ctx, numberedKey(keySyntheticEnvironmentPrefix, n), name); err != nil {
			return err
		}
	}
	if err := tcontext.Set(ctx, keyEnvironmentName, names[0]); err != nil {
		return err
	}
	return tcontext.Set(ctx, keySyntheticEnvironments, strings.Join(names, ","))
}

func syntheticEnvironmentNames(ctx context.Context) []string {
	recorded := optionalValue(ctx, keySyntheticEnvironments)
	if recorded == "" {
		return nil
	}
	return strings.Split(recorded, ",")
}

// syntheticEnvironmentAt resolves the nth (from one) environment the scenario created.
func syntheticEnvironmentAt(ctx context.Context, n int) (string, error) {
	names := syntheticEnvironmentNames(ctx)
	if n < 1 || n > len(names) {
		return "", fmt.Errorf("synthetic APIP environment %d does not exist: the scenario created %d", n, len(names))
	}
	return names[n-1], nil
}

// syntheticEnvironmentGatewayAt resolves the gateway the scenario created in the nth environment.
func syntheticEnvironmentGatewayAt(ctx context.Context, n int) (string, error) {
	id := optionalValue(ctx, numberedKey(keySyntheticGatewayPrefix, n))
	if id == "" {
		return "", fmt.Errorf("the scenario created no gateway in synthetic APIP environment %d", n)
	}
	return id, nil
}

// createGatewayInEachSyntheticEnvironment creates one managed gateway in every synthetic
// environment, records it as that environment's gateway, and makes the list of them the
// scenario's synthetic gateways.
func (s *Steps) createGatewayInEachSyntheticEnvironment(ctx context.Context) error {
	names := syntheticEnvironmentNames(ctx)
	if len(names) == 0 {
		return fmt.Errorf("the scenario has created no synthetic environment")
	}
	ids := make([]string, 0, len(names))
	for index, environment := range names {
		id, err := s.createManagedGatewayIn(ctx, environment)
		if err != nil {
			return err
		}
		ids = append(ids, id)
		if err := tcontext.Set(ctx, numberedKey(keySyntheticGatewayPrefix, index+1), id); err != nil {
			return err
		}
		if err := tcontext.Set(ctx, keySyntheticGateways, strings.Join(ids, ",")); err != nil {
			return err
		}
	}
	return nil
}

// promotionPathsFromTable reads a table whose header row is "source | targets" into promotion
// paths: one row per source environment, its targets comma separated.
func promotionPathsFromTable(table *godog.Table, expand func(string) (string, error)) ([]map[string]any, error) {
	if table == nil || len(table.Rows) < 2 {
		return nil, fmt.Errorf("a promotion paths table needs a header row and at least one path")
	}
	header := table.Rows[0].Cells
	if len(header) != 2 || strings.TrimSpace(header[0].Value) != "source" || strings.TrimSpace(header[1].Value) != "targets" {
		return nil, fmt.Errorf("a promotion paths table has the header \"source | targets\"")
	}
	paths := make([]map[string]any, 0, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("a promotion path row has a source and its targets, got %d cells", len(row.Cells))
		}
		source, err := expand(row.Cells[0].Value)
		if err != nil {
			return nil, err
		}
		source = strings.TrimSpace(source)
		if source == "" {
			return nil, fmt.Errorf("a promotion path has no source environment")
		}
		var targets []string
		for _, target := range strings.Split(row.Cells[1].Value, ",") {
			expanded, err := expand(target)
			if err != nil {
				return nil, err
			}
			if expanded = strings.TrimSpace(expanded); expanded != "" {
				targets = append(targets, expanded)
			}
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("the promotion path from %q has no target environment", source)
		}
		paths = append(paths, map[string]any{"sourceEnvironment": source, "targetEnvironments": targets})
	}
	return paths, nil
}

// createPipeline creates a deployment pipeline with the table's promotion paths. The pipeline is
// registered for cleanup before it is requested.
func (s *Steps) createPipeline(ctx context.Context, table *godog.Table) error {
	paths, err := promotionPathsFromTable(table, func(value string) (string, error) { return expandCloudValue(ctx, value) })
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-pipeline", keyPipelineID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyPipelineID)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindPipeline, ID: id, Actor: resourceActor, Description: "deployment pipeline " + id,
	}); err != nil {
		return fmt.Errorf("registering deployment pipeline %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/pipelines", map[string]any{"name": id, "promotionPaths": paths})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating a deployment pipeline: %w", err)
	}
	if createdID != id {
		deregister(ctx, kindPipeline, id)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: kindPipeline, ID: createdID, Actor: resourceActor, Description: "deployment pipeline " + createdID,
		}); err != nil {
			return fmt.Errorf("registering deployment pipeline %q for cleanup: %w", createdID, err)
		}
	}
	return tcontext.Set(ctx, keyPipelineID, createdID)
}

// replacePipelinePaths replaces the promotion paths of the scenario's pipeline and publishes the
// response, so a scenario can assert on how the platform answered.
func (s *Steps) replacePipelinePaths(ctx context.Context, table *godog.Table) error {
	paths, err := promotionPathsFromTable(table, func(value string) (string, error) { return expandCloudValue(ctx, value) })
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyPipelineID)
	if err != nil {
		return fmt.Errorf("the scenario has created no deployment pipeline: %w", err)
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, "/pipelines/"+url.PathEscape(id),
		map[string]any{"name": id, "promotionPaths": paths})
	return err
}

// createProjectBoundToPipeline creates a project that the scenario then keeps as its project, and
// binds it to the scenario's pipeline. The project is registered for cleanup before it is
// requested, ahead of the pipeline it is bound to.
func (s *Steps) createProjectBoundToPipeline(ctx context.Context) error {
	pipeline, err := requiredValue(ctx, keyPipelineID)
	if err != nil {
		return fmt.Errorf("the scenario has created no deployment pipeline: %w", err)
	}
	if err := s.createProjectOf(ctx, kindPromotionProject); err != nil {
		return err
	}
	project, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	var bound *httpx.Response
	err = retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodPut,
				"/projects/"+url.PathEscape(project)+"/deployment-pipeline", map[string]string{"pipeline": pipeline})
			if err != nil {
				return nil, retry.Transient(err)
			}
			switch {
			case resp.StatusCode == http.StatusOK:
				return resp, nil
			case resp.StatusCode == http.StatusNotFound || resp.StatusCode >= http.StatusInternalServerError:
				// The project is created asynchronously and may not be readable yet.
				return nil, retry.Transient(responseError(resp))
			default:
				return nil, responseError(resp)
			}
		},
		func(resp *httpx.Response) bool {
			bound = resp
			return resp != nil
		},
		"binding project "+project+" to deployment pipeline "+pipeline)
	if err != nil {
		return err
	}
	got, err := stringField(bound, "pipeline")
	if err != nil {
		return fmt.Errorf("binding project %q to a deployment pipeline: %w", project, err)
	}
	if got != pipeline {
		return fmt.Errorf("project %q is bound to deployment pipeline %q, want %q", project, got, pipeline)
	}
	return nil
}

// createAPIInSyntheticProject creates the synthetic REST API in the project the scenario created,
// and refuses to run against the default project.
func (s *Steps) createAPIInSyntheticProject(ctx context.Context, table *godog.Table) error {
	if project := optionalValue(ctx, keyProjectID); project == "" || project == "default" {
		return fmt.Errorf("the scenario has created no synthetic project, so the API would be created in %q", project)
	}
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	return s.createAPI(ctx, primaryAPI, overrides)
}

// requestProviderDeployment asks to deploy an LLM provider that does not exist, and publishes the
// response without requiring success. Nothing is created, so nothing is registered for cleanup.
func (s *Steps) requestProviderDeployment(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	body, err := buildDeploymentRequest(ctx, values)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-provider", keyProviderID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyProviderID)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPost, "/llm-providers/"+url.PathEscape(id)+"/environment-deployments", body)
	return err
}

// assertDeploymentListsGatewayOf checks the last deployment response lists a deployment on the
// gateway of the nth synthetic environment.
func (s *Steps) assertDeploymentListsGatewayOf(ctx context.Context, n int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := syntheticEnvironmentGatewayAt(ctx, n)
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

// stageGateway finds the entry a deployment listing gives one gateway of one environment.
func stageGateway(stages []map[string]any, environment, gatewayID string) (map[string]any, error) {
	for _, stage := range stages {
		if stage["environment"] != environment {
			continue
		}
		gateways, _ := stage["gateways"].([]any)
		for _, raw := range gateways {
			if gateway, _ := raw.(map[string]any); gateway["gatewayId"] == gatewayID {
				return gateway, nil
			}
		}
		return nil, fmt.Errorf("the stage for environment %q does not list gateway %q", environment, gatewayID)
	}
	return nil, fmt.Errorf("the deployment listing has no stage for environment %q", environment)
}

// stageOf finds the gateways a deployment listing gives one environment.
func stageOf(stages []map[string]any, environment string) ([]map[string]any, error) {
	for _, stage := range stages {
		if stage["environment"] != environment {
			continue
		}
		gateways, _ := stage["gateways"].([]any)
		out := make([]map[string]any, 0, len(gateways))
		for _, raw := range gateways {
			if gateway, ok := raw.(map[string]any); ok {
				out = append(out, gateway)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("the deployment listing has no stage for environment %q", environment)
}

// assertStageRunsRenderedBuild waits until the gateway of the nth synthetic environment shows the
// deployment the scenario last rendered a build for.
func (s *Steps) assertStageRunsRenderedBuild(ctx context.Context, n int) error {
	environment, err := syntheticEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	gatewayID, err := syntheticEnvironmentGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	want, err := latestPreparedBuild(ctx)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			stages, err := s.stages(ctx)
			if err != nil {
				return "", retry.Transient(err)
			}
			entry, err := stageGateway(stages, environment, gatewayID)
			if err != nil {
				return "", err
			}
			if status, _ := entry["status"].(string); status == "NOT_DEPLOYED" {
				return "NOT_DEPLOYED", nil
			}
			build, _ := entry["buildId"].(string)
			return build, nil
		},
		func(build string) bool { return build == want },
		fmt.Sprintf("waiting for gateway %q of environment %q to run build %s", gatewayID, environment, want))
}

// assertStageHoldsNothing checks no gateway of the nth synthetic environment shows a deployment
// of the API. The stage must exist, so an environment missing from the pipeline cannot pass.
func (s *Steps) assertStageHoldsNothing(ctx context.Context, n int) error {
	environment, err := syntheticEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	stages, err := s.stages(ctx)
	if err != nil {
		return err
	}
	gateways, err := stageOf(stages, environment)
	if err != nil {
		return err
	}
	for _, gateway := range gateways {
		if status, _ := gateway["status"].(string); status != "NOT_DEPLOYED" {
			return fmt.Errorf("gateway %v of environment %q shows the API as %q, want it untouched", gateway["gatewayId"], environment, status)
		}
	}
	return nil
}

// assertStageNoLongerDeployed waits until the gateway of the nth synthetic environment no longer
// shows the API as deployed or being deployed.
func (s *Steps) assertStageNoLongerDeployed(ctx context.Context, n int) error {
	environment, err := syntheticEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	gatewayID, err := syntheticEnvironmentGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			stages, err := s.stages(ctx)
			if err != nil {
				return "", retry.Transient(err)
			}
			entry, err := stageGateway(stages, environment, gatewayID)
			if err != nil {
				return "", err
			}
			status, _ := entry["status"].(string)
			return status, nil
		},
		func(status string) bool { return !stillDeployed[status] },
		fmt.Sprintf("waiting for gateway %q of environment %q to stop serving the API", gatewayID, environment))
}

// undeployFromSyntheticEnvironment undeploys what the last deployment request put on the gateway
// of the nth synthetic environment, through the project route, and requires it to be accepted.
func (s *Steps) undeployFromSyntheticEnvironment(ctx context.Context, n int) error {
	environment, err := syntheticEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	gatewayID, err := syntheticEnvironmentGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	targets, err := recordedTargets(ctx)
	if err != nil {
		return err
	}
	deploymentID := targets[gatewayID]
	if deploymentID == "" {
		return fmt.Errorf("the last deployment request created no deployment on gateway %q", gatewayID)
	}
	scope, err := apiScopePath(ctx)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, scope+"/deployments/"+url.PathEscape(deploymentID)+
		"/undeploy?environment="+url.QueryEscape(environment)+"&gatewayId="+url.QueryEscape(gatewayID), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return nil
}

// gatewayEndpoint reads the first approved data-plane endpoint of a managed gateway.
func (s *Steps) gatewayEndpoint(ctx context.Context, id string) (string, error) {
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return "", err
	}
	endpoints, _ := document["endpoints"].([]any)
	endpoint, ok := firstApprovedDataPlaneEndpoint(ctx, s, endpoints)
	if !ok {
		return "", fmt.Errorf("managed gateway %q lists no endpoint that passed the data-plane host check: %v", id, endpoints)
	}
	return endpoint, nil
}

// recordSyntheticEnvironmentEndpoint keeps the endpoint of the gateway of the nth synthetic
// environment, for a request that goes to that gateway directly.
func (s *Steps) recordSyntheticEnvironmentEndpoint(ctx context.Context, n int) error {
	gatewayID, err := syntheticEnvironmentGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	endpoint, err := s.gatewayEndpoint(ctx, gatewayID)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, numberedKey(keySyntheticEndpointPrefix, n), endpoint)
}

// defaultGatewayEnvironment finds the environment of the organisation's own default gateway: a
// regular gateway named "<environment>-default" that the suite did not create. The gateway
// need not be active, so a scenario that only names the environment does not depend on it.
func defaultGatewayEnvironment(items []map[string]any) (string, error) {
	var found []string
	for _, item := range items {
		id, _ := item["id"].(string)
		environment, _ := item["environment"].(string)
		if environment == "" || id != environment+"-default" || gatewayKind(item) != regularGatewayKind || strings.HasPrefix(environment, "syn-") {
			continue
		}
		found = append(found, environment)
	}
	sort.Strings(found)
	if len(found) == 0 {
		return "", fmt.Errorf("none of the %d listed managed gateways is an organisation default gateway", len(items))
	}
	return found[0], nil
}

// findDefaultGatewayEnvironment records the environment of the default gateway as the scenario's
// gateway environment, whether or not that gateway is active.
func (s *Steps) findDefaultGatewayEnvironment(ctx context.Context) error {
	items, err := s.fetchGatewayList(ctx, "")
	if err != nil {
		return err
	}
	environment, err := defaultGatewayEnvironment(items)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyGatewayEnv, environment)
}
