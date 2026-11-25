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

package promotion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

const (
	keyPipelineEnvironments = "cloudPipelineEnvironments"
	// The numbered keys name the nth environment of the chosen promotion chain, its gateway and
	// that gateway's endpoint.
	keyPipelineEnvironmentPrefix = "cloudPipelineEnvironment"
	keyPipelineGatewayPrefix     = "cloudPipelineGateway"
	keyPipelineEndpointPrefix    = "cloudPipelineEndpoint"
	// maxPromotionEnvironments bounds the numbered keys a scenario can address and the state a
	// new scenario clears.
	maxPromotionEnvironments = 5
)

// resetPromotionState drops what an earlier scenario of the runner recorded about its
// environments, gateways and deployments, so a numbered key can only name this scenario's own.
func resetPromotionState(ctx context.Context) error {
	keys := []string{keyPipelineEnvironments, cloudbase.KeyDeployedBuildID, cloudbase.KeyDeploymentID}
	for n := 1; n <= maxPromotionEnvironments; n++ {
		keys = append(keys, cloudbase.NumberedKey(keyPipelineEnvironmentPrefix, n),
			cloudbase.NumberedKey(keyPipelineGatewayPrefix, n), cloudbase.NumberedKey(keyPipelineEndpointPrefix, n))
	}
	for _, key := range keys {
		if err := tcontext.Set(ctx, key, ""); err != nil {
			return err
		}
	}
	return nil
}

// findPipelineProject makes the project the scenario's feature file works in the scenario's project.
// The project must already be bound to a deployment pipeline whose environments hold an active
// regular gateway each: the suite creates no environments or pipelines and never changes a
// project's binding.
func (s *Steps) findPipelineProject(ctx context.Context) error {
	project, err := cloudbase.RequiredValue(ctx, cloudbase.KeyScenarioProject)
	if err != nil {
		return fmt.Errorf("the scenario's project is not known: %w", err)
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodGet, "/projects/"+url.PathEscape(project), nil)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return tcontext.Set(ctx, cloudbase.KeyProjectID, project)
	case http.StatusNotFound:
		return fmt.Errorf("the organization has no %q project", project)
	default:
		return cloudbase.ResponseError(resp)
	}
}

// findPromotionChain chooses a promotion chain (see promotionChain) of the given length in the
// deployment pipeline of the scenario's project, whose environments each hold an active regular
// gateway, and records it as pipeline environments 1 to length with their gateways and endpoints.
// The gateway of the first, the entry environment, also becomes the scenario's active gateway. A
// pipeline without such a chain skips the scenario, as the suite creates no environments or
// pipelines of its own.
func (s *Steps) findPromotionChain(ctx context.Context, length int) error {
	if length < 2 || length > maxPromotionEnvironments {
		return fmt.Errorf("a promotion chain has between 2 and %d environments, asked for %d", maxPromotionEnvironments, length)
	}
	if err := resetPromotionState(ctx); err != nil {
		return err
	}
	projectID, err := cloudbase.RequiredValue(ctx, cloudbase.KeyProjectID)
	if err != nil {
		return fmt.Errorf("the scenario has found no project: %w", err)
	}
	pipeline, paths, err := cloudbase.BoundPipelineOf(ctx, projectID, func(ctx context.Context, path string) (*httpx.Response, error) {
		return s.DoAuthenticatedRaw(ctx, http.MethodGet, path, nil)
	})
	if err != nil {
		return fmt.Errorf("reading the deployment pipeline of project %q: %w", projectID, err)
	}
	if !cloudbase.OwnedPipeline(pipeline) {
		slog.Warn("skipping: the project's deployment pipeline is not named with the suite's prefix",
			"project", projectID, "pipeline", pipeline, "prefix", cloudbase.TestResourcePrefix)
		return godog.ErrSkip
	}
	items, err := s.FetchGatewayList(ctx, cloudbase.RegularGatewayKind)
	if err != nil {
		return err
	}
	gateways := map[string]cloudbase.GatewayChoice{}
	var unusable []string
	for _, environment := range cloudbase.SortedKeys(cloudbase.EnvironmentsOf(paths)) {
		if !cloudbase.OwnedEnvironment(environment) {
			unusable = append(unusable, fmt.Sprintf("environment %q is neither the default one nor named with the prefix %q", environment, cloudbase.TestResourcePrefix))
			continue
		}
		choice, err := s.ChooseActiveGateway(ctx, items, "gateway of environment "+environment,
			append(cloudbase.GatewayFilters(""), cloudbase.InEnvironment(environment))...)
		if errors.Is(err, cloudbase.ErrCloudDataPlaneDomains) {
			return err
		}
		if err != nil {
			unusable = append(unusable, err.Error())
			continue
		}
		gateways[environment] = choice
	}
	chain := cloudbase.PromotionChain(paths, length, func(environment string) bool {
		_, ok := gateways[environment]
		return ok
	})
	if chain == nil {
		slog.Warn("skipping: the project's deployment pipeline has no promotion chain of the length the scenario needs with an active gateway in each environment",
			"project", projectID, "pipeline", pipeline, "length", length, "paths", paths, "unusable", strings.Join(unusable, "; "))
		return godog.ErrSkip
	}
	for i, environment := range chain {
		choice := gateways[environment]
		for key, value := range map[string]string{
			cloudbase.NumberedKey(keyPipelineEnvironmentPrefix, i+1): environment,
			cloudbase.NumberedKey(keyPipelineGatewayPrefix, i+1):     choice.ID,
			cloudbase.NumberedKey(keyPipelineEndpointPrefix, i+1):    choice.Endpoint,
		} {
			if err := tcontext.Set(ctx, key, value); err != nil {
				return err
			}
		}
	}
	entry := gateways[chain[0]]
	for key, value := range map[string]string{
		keyPipelineEnvironments:      strings.Join(chain, ","),
		cloudbase.KeyGatewayID:       entry.ID,
		cloudbase.KeyGatewayEnv:      chain[0],
		cloudbase.KeyGatewayEndpoint: entry.Endpoint,
	} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func pipelineEnvironmentNames(ctx context.Context) []string {
	recorded := cloudbase.OptionalValue(ctx, keyPipelineEnvironments)
	if recorded == "" {
		return nil
	}
	return strings.Split(recorded, ",")
}

// pipelineEnvironmentAt resolves the nth (from one) environment of the scenario's promotion chain.
func pipelineEnvironmentAt(ctx context.Context, n int) (string, error) {
	names := pipelineEnvironmentNames(ctx)
	if n < 1 || n > len(names) {
		return "", fmt.Errorf("pipeline environment %d does not exist: the scenario's promotion chain has %d", n, len(names))
	}
	return names[n-1], nil
}

// createAPIInPipelineProject creates the synthetic REST API in the project of the scenario's feature
// file, and refuses to run against any other project.
func (s *Steps) createAPIInPipelineProject(ctx context.Context) error {
	project := cloudbase.OptionalValue(ctx, cloudbase.KeyProjectID)
	if want := cloudbase.OptionalValue(ctx, cloudbase.KeyScenarioProject); want == "" || project != want {
		return fmt.Errorf("the scenario has not found its project %q, so the API would be created in %q", want, project)
	}
	return s.CreateAPI(ctx, cloudbase.PrimaryAPI, nil)
}

// assertStageHoldsNothing checks no gateway of the nth pipeline environment shows a deployment
// of the API. The stage must exist, so an environment missing from the pipeline cannot pass.
func (s *Steps) assertStageHoldsNothing(ctx context.Context, n int) error {
	environment, err := pipelineEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	stages, err := s.Stages(ctx)
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

// pipelineGatewayAt resolves the gateway of the nth pipeline environment.
func pipelineGatewayAt(ctx context.Context, n int) (string, error) {
	id := cloudbase.OptionalValue(ctx, cloudbase.NumberedKey(keyPipelineGatewayPrefix, n))
	if id == "" {
		return "", fmt.Errorf("pipeline environment %d has no gateway", n)
	}
	return id, nil
}

// assertDeploymentListsGatewayOf checks the last deployment response lists a deployment on the
// gateway of the nth pipeline environment.
func (s *Steps) assertDeploymentListsGatewayOf(ctx context.Context, n int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := pipelineGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	entry, err := cloudbase.DeploymentEntry(response, gatewayID)
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

// assertStageRunsRenderedBuild waits until the gateway of the nth pipeline environment runs the
// build the last deployment rendered.
func (s *Steps) assertStageRunsRenderedBuild(ctx context.Context, n int) error {
	environment, err := pipelineEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	gatewayID, err := pipelineGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	want, err := cloudbase.LatestPreparedBuild(ctx)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: cloudbase.PropagationTimeout(s.Topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			stages, err := s.Stages(ctx)
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

// assertStageNoLongerDeployed waits until the gateway of the nth pipeline environment stops
// serving the API.
func (s *Steps) assertStageNoLongerDeployed(ctx context.Context, n int) error {
	environment, err := pipelineEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	gatewayID, err := pipelineGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: cloudbase.PropagationTimeout(s.Topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			stages, err := s.Stages(ctx)
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
		func(status string) bool { return !cloudbase.StillDeployed[status] },
		fmt.Sprintf("waiting for gateway %q of environment %q to stop serving the API", gatewayID, environment))
}

// undeployFromPipelineEnvironment undeploys what the last deployment request put on the gateway
// of the nth pipeline environment, through the project route, and requires it to be accepted.
func (s *Steps) undeployFromPipelineEnvironment(ctx context.Context, n int) error {
	environment, err := pipelineEnvironmentAt(ctx, n)
	if err != nil {
		return err
	}
	gatewayID, err := pipelineGatewayAt(ctx, n)
	if err != nil {
		return err
	}
	targets, err := cloudbase.RecordedTargets(ctx)
	if err != nil {
		return err
	}
	deploymentID := targets[gatewayID]
	if deploymentID == "" {
		return fmt.Errorf("the last deployment request created no deployment on gateway %q", gatewayID)
	}
	scope, err := cloudbase.APIScopePath(ctx)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPost, scope+"/deployments/"+url.PathEscape(deploymentID)+
		"/undeploy?environment="+url.QueryEscape(environment)+"&gatewayId="+url.QueryEscape(gatewayID), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return cloudbase.ResponseError(resp)
	}
	return nil
}
