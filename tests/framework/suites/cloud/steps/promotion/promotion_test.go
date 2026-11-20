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
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestCreateAPIInPipelineProjectRefusesAnyOtherProject(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request may be sent")
	})
	local.Set(cloudbase.KeyProjectID, "default")
	require.ErrorContains(t, steps.createAPIInPipelineProject(ctx), "has not found its project", "the scenario's project is not known")
	local.Set(cloudbase.KeyScenarioProject, cloudbase.ProjectTwo)
	require.ErrorContains(t, steps.createAPIInPipelineProject(ctx), `"default"`, "the scenario has not found its project")
}

func TestAssertStageHoldsNothing(t *testing.T) {
	steps, ctx, _ := stagesStub(t, promotionStages)
	require.NoError(t, steps.assertStageHoldsNothing(ctx, 2), "a stage whose gateways are all untouched")
	require.NoError(t, steps.assertStageHoldsNothing(ctx, 3), "a stage with no gateway")
	require.ErrorContains(t, steps.assertStageHoldsNothing(ctx, 1), "want it untouched")

	steps, ctx, _ = stagesStub(t, `{"list":[{"environment":"other","gateways":[]}]}`)
	require.ErrorContains(t, steps.assertStageHoldsNothing(ctx, 2), `no stage for environment "env-1"`,
		"an environment missing from the pipeline cannot pass")
}

func TestStageLookups(t *testing.T) {
	var document struct {
		List []map[string]any `json:"list"`
	}
	require.NoError(t, json.Unmarshal([]byte(promotionStages), &document))

	gateways, err := stageOf(document.List, "dev")
	require.NoError(t, err)
	require.Len(t, gateways, 1)
	require.Equal(t, "DEPLOYED", gateways[0]["status"])
	gateways, err = stageOf(document.List, "env-2")
	require.NoError(t, err)
	require.Empty(t, gateways)
	_, err = stageOf(document.List, "missing")
	require.Error(t, err)

	for name, stage := range map[string]map[string]any{
		"no gateways key":     {"environment": "dev"},
		"gateways not a list": {"environment": "dev", "gateways": "none"},
		"gateway not object":  {"environment": "dev", "gateways": []any{"gw-1"}},
	} {
		_, err = stageOf([]map[string]any{stage}, "dev")
		require.Error(t, err, name)
	}
}

func TestFindPipelineProject(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusOK
	var seen []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		cloudbase.WriteJSON(w, status, map[string]any{"id": cloudbase.ProjectTwo})
	})
	require.ErrorContains(t, steps.findPipelineProject(ctx), "project is not known")
	require.Empty(t, seen)

	local.Set(cloudbase.KeyScenarioProject, cloudbase.ProjectTwo)
	require.NoError(t, steps.findPipelineProject(ctx))
	require.Equal(t, cloudbase.ProjectTwo, cloudbase.OptionalValue(ctx, cloudbase.KeyProjectID))
	require.Equal(t, []string{"GET /projects/" + cloudbase.ProjectTwo}, seen)

	status = http.StatusNotFound
	require.ErrorContains(t, steps.findPipelineProject(ctx), "has no")
	status = http.StatusForbidden
	require.ErrorContains(t, steps.findPipelineProject(ctx), "unexpected response")
}

// promotionChainStub serves the project "payments", bound to a pipeline with the given promotion
// paths, and the given managed gateways. It records every request it receives.
func promotionChainStub(t *testing.T, paths string, gateways []map[string]any) (*Steps, context.Context, *tcontext.Local, *[]string) {
	return promotionChainStubOf(t, "apip-test-release", paths, gateways)
}

// promotionChainStubOf is promotionChainStub with the pipeline listed under the given name.
func promotionChainStubOf(t *testing.T, pipelineName, paths string, gateways []map[string]any) (*Steps, context.Context, *tcontext.Local, *[]string) {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var seen []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		switch r.URL.Path {
		case "/projects/apip-test-payments/deployment-pipeline":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"pipeline": "release"})
		case "/pipelines":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"list":[{"id":"release","name":"` + pipelineName + `","promotionPaths":` + paths + `}]}`))
		case "/managed-gateways":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": gateways})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	steps.DataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(cloudbase.KeyProjectID, "apip-test-payments")
	return steps, ctx, local, &seen
}

func TestFindPromotionChainRecordsTheChainAndItsGateways(t *testing.T) {
	steps, ctx, local, seen := promotionChainStub(t,
		`[{"sourceEnvironment":"development","targetEnvironments":["apip-test-staging"]},{"sourceEnvironment":"apip-test-staging","targetEnvironments":["apip-test-prod"]}]`,
		[]map[string]any{
			cloudbase.ListedGateway("dev-sgw", "development", map[string]any{"displayName": "sgw-abcdefg2-3"}),
			cloudbase.ListedGateway("dev-default", "development", nil),
			cloudbase.ListedGateway("staging-default", "apip-test-staging", nil),
			cloudbase.ListedGateway("prod-idle", "apip-test-prod", map[string]any{"isActive": false}),
		})

	require.NoError(t, steps.findPromotionChain(ctx, 2))
	for key, want := range map[string]string{
		keyPipelineEnvironments:                                "development,apip-test-staging",
		cloudbase.NumberedKey(keyPipelineEnvironmentPrefix, 1): "development",
		cloudbase.NumberedKey(keyPipelineGatewayPrefix, 1):     "dev-default",
		cloudbase.NumberedKey(keyPipelineEndpointPrefix, 1):    "https://dev-default.example.com",
		cloudbase.NumberedKey(keyPipelineEnvironmentPrefix, 2): "apip-test-staging",
		cloudbase.NumberedKey(keyPipelineGatewayPrefix, 2):     "staging-default",
		cloudbase.NumberedKey(keyPipelineEndpointPrefix, 2):    "https://staging-default.example.com",
		cloudbase.KeyGatewayID:                                 "dev-default",
		cloudbase.KeyGatewayEnv:                                "development",
		cloudbase.KeyGatewayEndpoint:                           "https://dev-default.example.com",
	} {
		require.Equal(t, want, cloudbase.OptionalValue(ctx, key), key)
	}
	require.Contains(t, *seen, "GET /managed-gateways?functionalityType=regular")
	for _, request := range *seen {
		require.True(t, strings.HasPrefix(request, "GET "), "the chain is read, never written: %s", request)
	}

	require.ErrorIs(t, steps.findPromotionChain(ctx, 3), godog.ErrSkip, "the third environment has no active gateway")
	require.Empty(t, cloudbase.OptionalValue(ctx, cloudbase.NumberedKey(keyPipelineEnvironmentPrefix, 1)), "a skipped choice keeps nothing of an earlier one")
	require.ErrorContains(t, steps.findPromotionChain(ctx, 1), "between 2 and")
	require.ErrorContains(t, steps.findPromotionChain(ctx, maxPromotionEnvironments+1), "between 2 and")
	local.Set(cloudbase.KeyProjectID, "")
	require.ErrorContains(t, steps.findPromotionChain(ctx, 2), "found no project")
}

func TestFindPromotionChainPassesOverEnvironmentsAndPipelinesTheSuiteDoesNotOwn(t *testing.T) {
	paths := `[{"sourceEnvironment":"development","targetEnvironments":["qa"]},{"sourceEnvironment":"qa","targetEnvironments":["apip-test-staging"]}]`
	gateways := []map[string]any{
		cloudbase.ListedGateway("dev-default", "development", nil),
		cloudbase.ListedGateway("qa-default", "qa", nil),
		cloudbase.ListedGateway("staging-default", "apip-test-staging", nil),
	}

	steps, ctx, _, _ := promotionChainStub(t, paths, gateways)
	require.ErrorIs(t, steps.findPromotionChain(ctx, 2), godog.ErrSkip, "qa is another console's environment, so development has no owned next stage")
	require.ErrorIs(t, steps.findPromotionChain(ctx, 3), godog.ErrSkip)

	steps, ctx, _, _ = promotionChainStubOf(t, "release", paths, gateways)
	require.ErrorIs(t, steps.findPromotionChain(ctx, 2), godog.ErrSkip, "a pipeline other consoles share is not the suite's to use")
}

func stagesStub(t *testing.T, stages string) (*Steps, context.Context, *tcontext.Local) {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects/syn-project-1/apis/syn-api-1/deployments", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stages))
	})
	local.Set(cloudbase.KeyProjectID, "syn-project-1")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(keyPipelineEnvironments, "dev,env-1,env-2")
	local.Set(cloudbase.NumberedKey(keyPipelineGatewayPrefix, 1), "gw-dev")
	local.Set(cloudbase.NumberedKey(keyPipelineGatewayPrefix, 2), "gw-1")
	return steps, ctx, local
}

const promotionStages = `{"list":[
	{"environment":"dev","gateways":[{"gatewayId":"gw-dev","status":"DEPLOYED","buildId":"2026-01-31-1"}]},
	{"environment":"env-1","gateways":[{"gatewayId":"gw-1","status":"NOT_DEPLOYED"}]},
	{"environment":"env-2","gateways":[]}]}`

func TestWaitDeployedOnTargetsWaitsForEveryDeployment(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	polls := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		polls++
		second := "DEPLOYING"
		if polls > 1 {
			second = "DEPLOYED"
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
			{"deploymentId": "dep-1", "status": "DEPLOYED"}, {"deploymentId": "dep-2", "status": second}}})
	})
	local.Set(cloudbase.KeyAPIID, "api-1")
	local.Set(cloudbase.KeyDeployedTargets, `{"gw-1":"dep-1","gw-2":"dep-2"}`)
	require.NoError(t, steps.waitDeployedOnTargets(ctx))
	require.Greater(t, polls, 1)
}

func TestADeploymentGetsAtLeastTenMinutesToBecomeDeployed(t *testing.T) {
	require.GreaterOrEqual(t, deployedTimeout, 10*time.Minute)
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}
