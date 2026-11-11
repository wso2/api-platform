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
	"slices"
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

func numberedKey(prefix string, n int) string { return fmt.Sprintf("%s%d", prefix, n) }

// resetPromotionState drops what an earlier scenario of the runner recorded about its
// environments, gateways and deployments, so a numbered key can only name this scenario's own.
func resetPromotionState(ctx context.Context) error {
	keys := []string{keySyntheticEnvironments, keyPipelineID, keyDeployedBuildID, keyDeploymentID}
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
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, bound); err != nil {
			return err
		}
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

// stageOf finds the gateways a deployment listing gives one environment.
func stageOf(stages []map[string]any, environment string) ([]map[string]any, error) {
	for _, stage := range stages {
		if stage["environment"] != environment {
			continue
		}
		gateways, ok := stage["gateways"].([]any)
		if !ok {
			return nil, fmt.Errorf("the deployment listing gives environment %q no gateways array: %v", environment, stage["gateways"])
		}
		out := make([]map[string]any, 0, len(gateways))
		for i, raw := range gateways {
			gateway, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("gateway %d of environment %q in the deployment listing is not an object: %v", i, environment, raw)
			}
			out = append(out, gateway)
		}
		return out, nil
	}
	return nil, fmt.Errorf("the deployment listing has no stage for environment %q", environment)
}

// baseGatewayWait is how long a new environment is given to show a gateway of its own before the
// runner builds one. The platform may provision an environment's default gateway itself, and
// building a second one beside it would spend a gateway of the organisation's allowance for nothing.
const baseGatewayWait = 90 * time.Second

// syntheticEnvironmentGatewayAt resolves the gateway of the nth synthetic environment.
func syntheticEnvironmentGatewayAt(ctx context.Context, n int) (string, error) {
	id := optionalValue(ctx, numberedKey(keySyntheticGatewayPrefix, n))
	if id == "" {
		return "", fmt.Errorf("synthetic APIP environment %d has no gateway", n)
	}
	return id, nil
}

// environmentsExist reports which of the names the platform still lists as environments.
func (s *Steps) environmentsExist(ctx context.Context, names []string) (map[string]bool, error) {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/environments", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, fmt.Errorf("decoding APIP environments: %w", err)
	}
	listed := map[string]bool{}
	for _, item := range items {
		if name, _ := item["name"].(string); name != "" {
			listed[name] = true
		}
	}
	present := map[string]bool{}
	for _, name := range names {
		present[name] = listed[name]
	}
	return present, nil
}

// ensureRunnerEnvironments makes sure the runner holds the given number of synthetic environments,
// each with a regular gateway that is active, and publishes them as synthetic environments 1 to
// count. The environments and their gateways last for the runner: a gateway takes minutes to be
// built and the organisation allows few of them, so the scenarios that promote along a pipeline
// share these instead of building their own. A scenario that finds them in place changes nothing.
func (s *Steps) ensureRunnerEnvironments(ctx context.Context, count int) error {
	if count < 1 || count > maxPromotionEnvironments {
		return fmt.Errorf("a runner holds between 1 and %d synthetic environments, asked for %d", maxPromotionEnvironments, count)
	}
	names := syntheticEnvironmentNames(ctx)
	valid := 0
	if len(names) > 0 {
		present, err := s.environmentsExist(ctx, names)
		if err != nil {
			return err
		}
		// What the runner recorded is kept only while every environment is still there and
		// each has its gateway; anything else is state another scenario left behind.
		for n, name := range names {
			if !present[name] || optionalValue(ctx, numberedKey(keySyntheticGatewayPrefix, n+1)) == "" ||
				optionalValue(ctx, numberedKey(keySyntheticEnvironmentPrefix, n+1)) != name {
				break
			}
			valid++
		}
		if valid < len(names) {
			valid = 0
		}
	}
	if valid >= count {
		return s.publishRunnerEnvironments(ctx, names[:count])
	}
	if valid == 0 {
		if err := resetPromotionState(ctx); err != nil {
			return err
		}
		names = nil
	}
	var fresh []string
	for n := len(names) + 1; n <= count; n++ {
		if err := s.createRunnerEnvironment(ctx); err != nil {
			return err
		}
		name, err := requiredValue(ctx, keyEnvironmentName)
		if err != nil {
			return err
		}
		names = append(names, name)
		fresh = append(fresh, name)
		if err := tcontext.Set(ctx, numberedKey(keySyntheticEnvironmentPrefix, n), name); err != nil {
			return err
		}
	}
	first := len(names) - len(fresh)
	gateways := make([]string, 0, len(fresh))
	for i, name := range fresh {
		id, err := s.ensureEnvironmentGateway(ctx, name)
		if err != nil {
			return fmt.Errorf("synthetic APIP environment %d (%s): %w", first+i+1, name, err)
		}
		gateways = append(gateways, id)
		if err := tcontext.Set(ctx, numberedKey(keySyntheticGatewayPrefix, first+i+1), id); err != nil {
			return err
		}
	}
	for i, id := range gateways {
		if err := s.waitGatewayActive(ctx, id); err != nil {
			return fmt.Errorf("synthetic APIP environment %d: %w", first+i+1, err)
		}
		endpoint, err := s.gatewayEndpoint(ctx, id)
		if err != nil {
			return err
		}
		if err := tcontext.Set(ctx, numberedKey(keySyntheticEndpointPrefix, first+i+1), endpoint); err != nil {
			return err
		}
	}
	return s.publishRunnerEnvironments(ctx, names)
}

// publishRunnerEnvironments records the environments as the ones the scenario's steps name.
func (s *Steps) publishRunnerEnvironments(ctx context.Context, names []string) error {
	if err := tcontext.Set(ctx, keyEnvironmentName, names[0]); err != nil {
		return err
	}
	return tcontext.Set(ctx, keySyntheticEnvironments, strings.Join(names, ","))
}

// regularGatewayOf finds the regular gateway an environment lists, the one that is active first,
// or "" when it lists none.
func regularGatewayOf(items []map[string]any, environment string) string {
	var found []map[string]any
	for _, item := range items {
		if item["environment"] == environment && gatewayKind(item) == regularGatewayKind {
			found = append(found, item)
		}
	}
	slices.SortStableFunc(found, func(a, b map[string]any) int {
		return boolRank(b["isActive"] == true) - boolRank(a["isActive"] == true)
	})
	if len(found) == 0 {
		return ""
	}
	id, _ := found[0]["id"].(string)
	return id
}

// ensureEnvironmentGateway returns the regular gateway of a new environment. It is the gateway the
// platform provisions with the environment when it does, and one the runner builds when it does not
// provision any.
func (s *Steps) ensureEnvironmentGateway(ctx context.Context, environment string) (string, error) {
	var found string
	var listErr error
	listed := false
	_ = retry.Await(ctx, retry.Options{Timeout: baseGatewayWait, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) {
			items, err := s.fetchGatewayList(ctx, "")
			if err != nil {
				listErr = err
				return "", err
			}
			listed = true
			found = regularGatewayOf(items, environment)
			return found, nil
		},
		func(id string) bool { return id != "" },
		"waiting for environment "+environment+" to list a gateway")
	if found != "" {
		return found, nil
	}
	if !listed {
		return "", fmt.Errorf("listing the gateways of environment %q: %w", environment, listErr)
	}
	budget, err := s.probeHandleBudget(ctx, environment)
	if err != nil {
		return "", err
	}
	name, err := freshGatewayName(ctx)
	if err != nil {
		return "", err
	}
	handle := handleOfLength(name, budget)
	if err := s.requestManagedGatewayScoped(ctx, map[string]string{
		"displayName": handle, "handle": handle, "environment": environment,
	}, cleanup.RunnerScope); err != nil {
		return "", err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return "", err
	}
	if !gatewayRequestAccepted(resp.StatusCode) {
		return "", fmt.Errorf("creating a gateway in environment %q: %w", environment, responseError(resp))
	}
	return stringField(resp, "id")
}

// replacePipelinePaths replaces the promotion paths of the scenario's pipeline and publishes the
// response.
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
	gateways, err := stageOf(stages, environment)
	if err != nil {
		return nil, err
	}
	for _, gateway := range gateways {
		if gateway["gatewayId"] == gatewayID {
			return gateway, nil
		}
	}
	return nil, fmt.Errorf("the stage for environment %q does not list gateway %q", environment, gatewayID)
}

// assertStageRunsRenderedBuild waits until the gateway of the nth synthetic environment runs the
// build the last deployment rendered.
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

// assertStageNoLongerDeployed waits until the gateway of the nth synthetic environment stops
// serving the API.
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
