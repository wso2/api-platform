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

package deploy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestBuildDeploymentRequest(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set("env", "development")
	local.Set("gw", "dev-default")

	body, err := buildDeploymentRequest(ctx, map[string]string{
		"environment": "${CTX:env}", "gatewayId": "${CTX:gw}", "buildId": "2026-01-01-1",
		"parameters": `{"productionEndpoint":"https://echo.example.com"}`,
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"environment": "development", "buildId": "2026-01-01-1",
		"gateways": []map[string]any{{"gatewayId": "dev-default", "parameters": map[string]string{"productionEndpoint": "https://echo.example.com"}}},
	}, body)

	body, err = buildDeploymentRequest(ctx, map[string]string{"environment": "e", "gatewayIds": " a, b ,,c"})
	require.NoError(t, err)
	require.Equal(t, []map[string]any{{"gatewayId": "a"}, {"gatewayId": "b"}, {"gatewayId": "c"}}, body["gateways"])

	body, err = buildDeploymentRequest(ctx, map[string]string{"environment": "e"})
	require.NoError(t, err)
	require.NotContains(t, body, "gateways", "an unnamed gateway set deploys to the environment default")
	require.NotContains(t, body, "buildId")

	for name, values := range map[string]map[string]string{
		"unknown key":        {"environment": "e", "gateway": "g"},
		"both gateway keys":  {"gatewayId": "a", "gatewayIds": "b,c"},
		"parameters for two": {"gatewayIds": "a,b", "parameters": `{"vhostMain":"x.example.com"}`},
		"parameters alone":   {"parameters": `{"vhostMain":"x.example.com"}`},
		"bad parameters":     {"gatewayId": "a", "parameters": `["x"]`},
	} {
		_, err := buildDeploymentRequest(ctx, values)
		require.Error(t, err, name)
	}
}

func TestPrepareBuildRecordsBuildsInOrderWithMetadata(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var bodies []string
	var paths []string
	next := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies, paths = append(bodies, string(raw)), append(paths, r.URL.Path)
		next++
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"buildId": "2026-01-31-" + string(rune('0'+next))})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")

	require.NoError(t, steps.prepareBuild(ctx, cloudbase.ValuesTable(t, []string{"commit", "abc"})))
	require.NoError(t, steps.prepareBuild(ctx, nil))
	require.Equal(t, []string{"/projects/default/apis/syn-api-1/builds", "/projects/default/apis/syn-api-1/builds"}, paths)
	require.JSONEq(t, `{"metadata":{"commit":"abc"}}`, bodies[0])
	require.Empty(t, strings.TrimSpace(bodies[1]), "a build without metadata sends no body")

	ids, err := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildIDs)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-1,2026-01-31-2", ids)
	latest, err := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildID)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-2", latest)
	require.Empty(t, cloudbase.OptionalValue(ctx, cloudbase.KeyBuildMetadata), "metadata describes the latest build only")
	require.NoError(t, steps.assertBuildIDShape(ctx))
}

func TestPrepareBuildRejectsAnythingButCreated(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal_error"})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	require.Error(t, steps.prepareBuild(ctx, nil))
	require.Empty(t, cloudbase.OptionalValue(ctx, cloudbase.KeyBuildIDs))
}

func TestAssertBuildIDShape(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyBuildsOwner, "syn-api-1")
	require.Error(t, steps.assertBuildIDShape(ctx), "no build prepared")
	for id, ok := range map[string]bool{"2026-01-31-1": true, "2026-01-31-12": true, "2026-01-31-0": false, "build-1": false, "2026-1-31-1": false,
		"2026-99-99-1": false, "2026-02-30-1": false} {
		local.Set(cloudbase.KeyBuildIDs, "2025-12-31-1,"+id)
		require.Equal(t, ok, steps.assertBuildIDShape(ctx) == nil, id)
	}
}

func TestBuildListingAssertions(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	listing := `{"list":[{"buildId":"2026-01-31-2"},{"buildId":"2026-01-31-1","metadata":{"commit":"abc"}}]}`
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listing))
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyBuildsOwner, "syn-api-1")

	local.Set(cloudbase.KeyBuildIDs, "2026-01-31-1,2026-01-31-2")
	require.NoError(t, steps.assertBuildsNewestFirst(ctx))
	local.Set(cloudbase.KeyBuildIDs, "2026-01-31-2,2026-01-31-1")
	require.ErrorContains(t, steps.assertBuildsNewestFirst(ctx), "not listed before")
	local.Set(cloudbase.KeyBuildIDs, "2026-01-31-1")
	require.ErrorContains(t, steps.assertBuildsNewestFirst(ctx), "at least two")

	local.Set(cloudbase.KeyBuildIDs, "2026-01-31-1")
	local.Set(cloudbase.KeyBuildMetadata, `{"commit":"abc"}`)
	local.Set(cloudbase.KeyBuildMetadataOf, "2026-01-31-1")
	require.NoError(t, steps.assertBuildListed(ctx))
	local.Set(cloudbase.KeyBuildMetadata, `{"commit":"def"}`)
	require.ErrorContains(t, steps.assertBuildListed(ctx), "metadata")
	local.Set(cloudbase.KeyBuildMetadata, "")
	require.NoError(t, steps.assertBuildListed(ctx))

}

func deploymentCreated() map[string]any {
	return map[string]any{"list": []map[string]any{
		{"gatewayId": "dev-default", "deploymentId": "dep-1", "buildId": "2026-01-31-1", "status": "DEPLOYING", "endpointUrl": "https://echo.example.com/anything"},
		{"gatewayId": "dev-extra", "deploymentId": "dep-2", "buildId": "2026-01-31-1", "status": "DEPLOYING"},
	}}
}

func TestRequestDeploymentRecordsWhatWasDeployed(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var path, body string
	status := http.StatusCreated
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		path, body = r.URL.Path, string(raw)
		if status != http.StatusCreated {
			cloudbase.WriteJSON(w, status, map[string]string{"code": "invalid_request"})
			return
		}
		cloudbase.WriteJSON(w, status, deploymentCreated())
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyGatewayID, "dev-default")
	local.Set(cloudbase.KeyBuildsOwner, "syn-api-1")
	local.Set(cloudbase.KeyBuildIDs, "2026-01-31-1")

	table := cloudbase.ValuesTable(t, []string{"environment", "development"}, []string{"gatewayIds", "dev-default,dev-extra"})
	require.NoError(t, steps.requestDeployment(ctx, table))
	require.Equal(t, "/projects/default/apis/syn-api-1/deployments", path)
	require.JSONEq(t, `{"environment":"development","gateways":[{"gatewayId":"dev-default"},{"gatewayId":"dev-extra"}]}`, body)
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusCreated))
	require.NoError(t, steps.assertDeploymentListsActiveGateway(ctx))
	require.NoError(t, steps.assertDeployedBuildIsPrepared(ctx))

	deployment, err := cloudbase.RequiredValue(ctx, cloudbase.KeyDeploymentID)
	require.NoError(t, err)
	require.Equal(t, "dep-1", deployment)

	local.Set(cloudbase.KeyBuildIDs, "2026-01-31-9")
	require.ErrorContains(t, steps.assertDeployedBuildIsPrepared(ctx), "prepared build")

	status = http.StatusBadRequest
	local.Set(cloudbase.KeyDeploymentID, "unchanged")
	require.NoError(t, steps.requestDeployment(ctx, table), "a refusal is for the scenario to assert on")
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusBadRequest))
	kept, _ := cloudbase.RequiredValue(ctx, cloudbase.KeyDeploymentID)
	require.Equal(t, "unchanged", kept, "a refused request records nothing")
	_, err = cloudbase.RecordedTargets(ctx)
	require.ErrorContains(t, err, "no deployment has been requested", "the targets of an earlier request do not answer for a refused one")
}

func TestDeploymentAssertionsRequireTheActiveGateway(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(cloudbase.KeyGatewayID, "dev-default")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 201, `{"list":[{"gatewayId":"other","deploymentId":"d"}]}`)
	require.ErrorContains(t, steps.assertDeploymentListsActiveGateway(ctx), "does not list gateway")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 201, `{"list":[{"gatewayId":"dev-default"}]}`)
	require.ErrorContains(t, steps.assertDeploymentListsActiveGateway(ctx), "no deployment id")
	require.Error(t, steps.assertDeployedBuildIsPrepared(ctx))
}

func TestEchoBackendSteps(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	t.Setenv(echoBackendEnv, "")
	require.NoError(t, steps.useEchoBackend(ctx))
	backend, _ := cloudbase.RequiredValue(ctx, keyEchoBackendURL)
	require.Equal(t, defaultEchoBackend, backend)

	t.Setenv(echoBackendEnv, "https://echo.example.com/base/")
	require.NoError(t, steps.useEchoBackend(ctx))
	backend, _ = cloudbase.RequiredValue(ctx, keyEchoBackendURL)
	require.Equal(t, "https://echo.example.com/base", backend)

	for _, invalid := range []string{"http://echo.example.com", "echo.example.com", "https://"} {
		t.Setenv(echoBackendEnv, invalid)
		require.Error(t, steps.useEchoBackend(ctx), invalid)
	}

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"method":"GET","path":"/base/posts/1"}`)
	require.NoError(t, steps.assertEchoBackendReceived(ctx, "/posts/1"))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"method":"GET","path":"/posts/1"}`)
	require.NoError(t, steps.assertEchoBackendReceived(ctx, "/posts/1"), "the backend's base path may be stripped before it receives the request")
	require.ErrorContains(t, steps.assertEchoBackendReceived(ctx, "/posts/2"), "received path")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 200, `{"userId":1,"id":1}`)
	require.ErrorContains(t, steps.assertEchoBackendReceived(ctx, "/posts/1"), "not an echo")

	local.Set(cloudbase.KeyGatewayID, "dev-default")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 201, `{"list":[{"gatewayId":"dev-default","deploymentId":"d","endpointUrl":"https://echo.example.com/base"}]}`)
	require.NoError(t, steps.assertDeploymentReportsEchoEndpoint(ctx))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 201, `{"list":[{"gatewayId":"dev-default","deploymentId":"d","endpointUrl":"https://jsonplaceholder.example.com"}]}`)
	require.ErrorContains(t, steps.assertDeploymentReportsEchoEndpoint(ctx), "production endpoint")
}

func TestStageAssertions(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stages := `{"list":[{"environment":"development","gateways":[{"gatewayId":"dev-default","status":"NOT_DEPLOYED"}]},{"environment":"production","gateways":[]}]}`
	paths := `[{"sourceEnvironment":"development","targetEnvironments":["production"]}]`
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/projects/default/apis/syn-api-1/deployments":
			_, _ = w.Write([]byte(stages))
		case "/projects/default/deployment-pipeline":
			_, _ = w.Write([]byte(`{"pipeline":"default"}`))
		case "/pipelines":
			_, _ = w.Write([]byte(`{"list":[{"name":"other","promotionPaths":[]},{"name":"default","promotionPaths":` + paths + `}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyGatewayID, "dev-default")
	local.Set(cloudbase.KeyGatewayEnv, "development")

	require.NoError(t, steps.assertStagesMatchPipeline(ctx))
	require.NoError(t, steps.assertActiveGatewayStageStatus(ctx, "NOT_DEPLOYED"))
	require.ErrorContains(t, steps.assertActiveGatewayStageStatus(ctx, "DEPLOYED"), "want")

	local.Set(cloudbase.KeyGatewayEnv, "staging")
	require.ErrorContains(t, steps.assertActiveGatewayStageStatus(ctx, "NOT_DEPLOYED"), "no deployment stage")
	local.Set(cloudbase.KeyGatewayEnv, "production")
	require.ErrorContains(t, steps.assertActiveGatewayStageStatus(ctx, "NOT_DEPLOYED"), "does not list gateway")

	stages = `{"list":[{"environment":"development","gateways":[]}]}`
	require.ErrorContains(t, steps.assertStagesMatchPipeline(ctx), "pipeline has")
	stages = `{"list":[{"environment":"development","gateways":[]},{"environment":"development","gateways":[]}]}`
	require.ErrorContains(t, steps.assertStagesMatchPipeline(ctx), "more than one")
	stages = `{"list":[{"environment":"development"},{"environment":"production","gateways":[]}]}`
	require.ErrorContains(t, steps.assertStagesMatchPipeline(ctx), "does not list its gateways")
}

func TestAssertDeploymentStatusReason(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	reason := "GATEWAY_PROCESSING_ERROR"
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{
			{"deploymentId": "dep-1", "status": "FAILED", "statusReason": reason},
		}})
	})
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyGatewayID, "dev-default")
	require.Error(t, steps.assertDeploymentStatusReason(ctx), "no deployment recorded")
	local.Set(cloudbase.KeyDeploymentID, "dep-1")
	require.NoError(t, steps.assertDeploymentStatusReason(ctx))
	reason = "failed to parse: line 3"
	require.ErrorContains(t, steps.assertDeploymentStatusReason(ctx), "error code")
	local.Set(cloudbase.KeyDeploymentID, "unknown")
	require.ErrorContains(t, steps.assertDeploymentStatusReason(ctx), "not listed")
}

func TestRecordRenderedBuild(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	require.Error(t, steps.recordRenderedBuild(ctx), "no API to record builds for")
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	require.ErrorContains(t, steps.recordRenderedBuild(ctx), "did not report a build")

	local.Set(cloudbase.KeyDeployedBuildID, "2026-01-31-1")
	require.NoError(t, steps.recordRenderedBuild(ctx))
	local.Set(cloudbase.KeyDeployedBuildID, "2026-01-31-2")
	require.NoError(t, steps.recordRenderedBuild(ctx))

	ids, err := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildIDs)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-1,2026-01-31-2", ids)
	latest, err := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildID)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-2", latest)

	require.ErrorContains(t, steps.recordRenderedBuild(ctx), "reused build", "a second deployment must render a new build")

	local.Set(cloudbase.KeyDeployedBuildID, "not-a-build")
	require.ErrorContains(t, steps.recordRenderedBuild(ctx), "not a day and index")
	after, _ := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildIDs)
	require.Equal(t, ids, after, "a refused build is not recorded")
}

func TestBuildStateBelongsToOneAPI(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(cloudbase.KeyAPIID, "syn-api-1")
	local.Set(cloudbase.KeyDeployedBuildID, "2026-01-31-1")
	require.NoError(t, steps.recordRenderedBuild(ctx))
	local.Set(cloudbase.KeyDeployedBuildID, "2026-01-31-2")
	require.NoError(t, steps.recordRenderedBuild(ctx))

	// The next scenario creates another API, whose first build has the same id.
	local.Set(cloudbase.KeyAPIID, "syn-api-2")
	local.Set(cloudbase.KeyDeployedBuildID, "2026-01-31-1")
	require.NoError(t, steps.recordRenderedBuild(ctx), "build ids are only unique within one API")
	ids, err := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildIDs)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-1", ids)
	latest, _ := cloudbase.RequiredValue(ctx, cloudbase.KeyBuildID)
	require.Equal(t, "2026-01-31-1", latest)
	require.Empty(t, cloudbase.OptionalValue(ctx, cloudbase.KeyBuildMetadata))
}

func TestRecordDeploymentsKeepsEachGatewaysDeployment(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	_ = steps
	local.Set(cloudbase.KeyGatewayID, "gw-1")
	response := &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"list":[` +
		`{"gatewayId":"gw-1","deploymentId":"dep-1","buildId":"2026-01-31-1"},{"gatewayId":"gw-2","deploymentId":"dep-2","buildId":"2026-01-31-1"}]}`)}
	require.NoError(t, recordDeployments(ctx, response))
	targets, err := cloudbase.RecordedTargets(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"gw-1": "dep-1", "gw-2": "dep-2"}, targets)
	local.Set(cloudbase.KeyDeployedTargets, "{}")
	_, err = cloudbase.RecordedTargets(ctx)
	require.ErrorContains(t, err, "no deployment")
}

func TestSeenBuildsBelongToOneAPIAndAreCountedFromOne(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set(cloudbase.KeyAPIID, "api-1")
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-1"))
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-2"))

	first, err := seenBuild(ctx, cloudbase.PrimaryAPI, 1)
	require.NoError(t, err)
	require.Equal(t, "2026-11-18-1", first)
	second, err := seenBuild(ctx, cloudbase.PrimaryAPI, 2)
	require.NoError(t, err)
	require.Equal(t, "2026-11-18-2", second)
	for _, n := range []int{0, 3} {
		_, err := seenBuild(ctx, cloudbase.PrimaryAPI, n)
		require.ErrorContains(t, err, "was not seen")
	}

	local.Set(cloudbase.KeyBuildID, "2026-11-18-2")
	local.Set(cloudbase.KeyAPIID, "api-2")
	builds, err := cloudbase.SeenBuilds(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Empty(t, builds, "another API's builds are not this API's")
	require.Empty(t, cloudbase.OptionalValue(ctx, cloudbase.KeyBuildID), "the related build keys are dropped with them")

	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.GraphQLAPI, "2026-11-18-1"))
	builds, err = cloudbase.SeenBuilds(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Empty(t, builds, "a GraphQL API's builds are kept apart")
	builds, err = cloudbase.SeenBuilds(ctx, cloudbase.GraphQLAPI)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-11-18-1"}, builds)
}

func TestPrepareBuildsPreparesTheNumberAsked(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	prepared := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/default/apis/api-1/builds", r.URL.Path)
		prepared++
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"buildId": "2026-11-18-" + strconv.Itoa(prepared)})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "api-1")

	require.NoError(t, steps.prepareBuilds(ctx, "a"))
	require.Equal(t, 1, prepared)
	require.NoError(t, steps.prepareBuilds(ctx, "3"))
	require.Equal(t, 4, prepared)
	builds, err := cloudbase.SeenBuilds(ctx, cloudbase.PrimaryAPI)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-11-18-1", "2026-11-18-2", "2026-11-18-3", "2026-11-18-4"}, builds)

	for _, count := range []string{"0", "-1", "x"} {
		require.Error(t, steps.prepareBuilds(ctx, count), count)
	}
	require.Equal(t, 4, prepared, "a bad count prepares nothing")
}

func TestDeployBuildNamesTheBuildAndTheKind(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var paths, queries []string
	var bodies []map[string]any
	answered := "2026-11-18-1"
	steps, ctx, local, _ := cloudSteps(t, cloudbase.DeploymentAnswerStub(t, &answered, &paths, &queries, &bodies))
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyGatewayID, "gw")
	local.Set(cloudbase.KeyGatewayEnv, "development")
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.GraphQLAPI, "2026-11-18-1"))

	require.NoError(t, steps.deployBuild(ctx, 1, cloudbase.GraphQLAPILabel))
	require.Equal(t, "/projects/default/apis/gql-1/deployments", paths[0])
	require.Equal(t, "kind=GraphQLApi", queries[0])
	require.Equal(t, "2026-11-18-1", bodies[0]["buildId"])
	require.Equal(t, "dep-1", cloudbase.OptionalValue(ctx, cloudbase.KeyGraphQLDeploymentID))
	builds, err := cloudbase.SeenBuilds(ctx, cloudbase.GraphQLAPI)
	require.NoError(t, err)
	require.Len(t, builds, 1, "deploying a build the scenario saw adds no build")

	answered = "2026-11-18-9"
	require.ErrorContains(t, steps.deployBuild(ctx, 1, cloudbase.GraphQLAPILabel), "runs build")
	require.ErrorContains(t, steps.deployBuild(ctx, 2, cloudbase.GraphQLAPILabel), "was not seen")
}

func TestUndeployAndTryUndeployAddressTheDeploymentOfTheAPI(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var path string
	var query url.Values
	status := http.StatusNoContent
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		path, query = r.URL.Path, r.URL.Query()
		w.WriteHeader(status)
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyGatewayID, "gw")
	local.Set(cloudbase.KeyGatewayEnv, "development")
	local.Set(cloudbase.KeyAPIID, "api-1")
	local.Set(cloudbase.KeyDeploymentID, "dep-a")
	local.Set(cloudbase.KeyGraphQLAPIID, "gql-1")
	local.Set(cloudbase.KeyGraphQLDeploymentID, "dep-g")
	local.Set("otherEnvironment", "missing")

	require.NoError(t, steps.UndeployAPI(ctx, cloudbase.GraphQLAPILabel))
	require.Equal(t, "/projects/default/apis/gql-1/deployments/dep-g/undeploy", path)
	require.Equal(t, url.Values{"environment": {"development"}, "gatewayId": {"gw"}, "kind": {cloudbase.KindGraphQLAPI}}, query)

	status = http.StatusBadRequest
	require.NoError(t, steps.TryUndeployAPI(ctx, "${CTX:otherEnvironment}"), "a refusal is left for the scenario")
	require.Equal(t, "/projects/default/apis/api-1/deployments/dep-a/undeploy", path)
	require.Equal(t, url.Values{"environment": {"missing"}, "gatewayId": {"gw"}}, query)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)

	status = http.StatusConflict
	require.NoError(t, steps.TryUndeployAPI(ctx, ""))
	require.Equal(t, "development", query.Get("environment"), "the active gateway's environment by default")
	require.Error(t, steps.UndeployAPI(ctx, "synthetic REST API"), "only 204 is a completed undeploy")
}

func TestDeleteBuildAddressesTheBuildThroughTheWayNamed(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var requests []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "BUILD_NOT_FOUND"})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "api-1")
	local.Set(cloudbase.KeyVersionAPIID, "api-2")
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-1"))

	require.NoError(t, steps.deleteBuild(ctx, 1, ""))
	require.NoError(t, steps.deleteBuild(ctx, 1, throughNewVersion))
	require.NoError(t, steps.deleteBuild(ctx, 1, asGraphQLAPI))
	require.Equal(t, []string{
		"DELETE /projects/default/apis/api-1/builds/2026-11-18-1?",
		"DELETE /projects/default/apis/api-2/builds/2026-11-18-1?",
		"DELETE /projects/default/apis/api-1/builds/2026-11-18-1?kind=GraphQLApi",
	}, requests)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, published.StatusCode, "the answer is left for the scenario")

	require.ErrorContains(t, steps.deleteBuild(ctx, 1, "sideways"), "unsupported way")
	require.ErrorContains(t, steps.deleteBuild(ctx, 2, ""), "was not seen")
	require.Len(t, requests, 3)
}

func TestBuildPresenceOrderAndIdentifierAssertions(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	listed := []string{"2026-11-18-6", "2026-11-18-5", "2026-11-18-4", "2026-11-18-3", "2026-11-18-1"}
	var served atomic.Int32
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/default/apis/api-1/builds", r.URL.Path)
		served.Add(1)
		items := make([]map[string]string, 0, len(listed))
		for _, id := range listed {
			items = append(items, map[string]string{"buildId": id})
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": items})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "api-1")
	for i := 1; i <= 6; i++ {
		require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-"+strconv.Itoa(i)))
	}

	require.NoError(t, steps.assertBuildPresence(ctx, "include", 1))
	require.NoError(t, steps.assertBuildPresence(ctx, "not include", 2))
	require.NoError(t, steps.assertBuildsExactly(ctx, "6, 5, 4, 3 and 1"))
	// Each negative check gets a context of its own and must have read a listing to fail.
	for name, check := range map[string]func(context.Context) error{
		"a build that was pruned":      func(ctx context.Context) error { return steps.assertBuildPresence(ctx, "include", 2) },
		"a build that is still listed": func(ctx context.Context) error { return steps.assertBuildPresence(ctx, "not include", 1) },
		"another order":                func(ctx context.Context) error { return steps.assertBuildsExactly(ctx, "5, 6, 4, 3 and 1") },
		"a missing build":              func(ctx context.Context) error { return steps.assertBuildsExactly(ctx, "6, 5, 3 and 1") },
		"a build too many":             func(ctx context.Context) error { return steps.assertBuildsExactly(ctx, "6, 5, 4, 3, 2 and 1") },
	} {
		before := served.Load()
		short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		require.Error(t, check(short), name)
		cancel()
		require.Greater(t, served.Load(), before, "%s: the check read a listing", name)
	}
	require.ErrorContains(t, steps.assertBuildsExactly(ctx, "6, five and 1"), "not a list of build numbers")
	require.ErrorContains(t, steps.assertBuildsExactly(ctx, "6, 5 and 9"), "was not seen")
	require.NoError(t, steps.assertBuildsDistinct(ctx))

	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-1"))
	require.ErrorContains(t, steps.assertBuildsDistinct(ctx), "builds 1 and 7 share the identifier")

	local.Set(cloudbase.KeyAPIID, "api-2")
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-1"))
	require.ErrorContains(t, steps.assertBuildsDistinct(ctx), "at least two builds")
}

func TestSendMalformedRequestSendsABodyThatIsNotJSON(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var path, contentType string
	var body []byte
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		path, contentType = r.URL.Path, r.Header.Get("Content-Type")
		body, _ = io.ReadAll(r.Body)
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request"})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "api-1")

	require.NoError(t, steps.sendMalformedRequest(ctx, "build"))
	require.Equal(t, "/projects/default/apis/api-1/builds", path)
	require.Equal(t, "application/json", contentType)
	require.False(t, json.Valid(body), "the body must not be JSON")
	require.NoError(t, steps.sendMalformedRequest(ctx, "deployment"))
	require.Equal(t, "/projects/default/apis/api-1/deployments", path)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)
}

func TestKeepReturningForHoldsForTheWholeWindow(t *testing.T) {
	var mu sync.Mutex
	calls, changeAt := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		changed := changeAt > 0 && calls >= changeAt
		mu.Unlock()
		if changed {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Steps: &cloudbase.Steps{Funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0),
		DataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}}

	require.NoError(t, steps.KeepReturningFor(ctx, http.MethodGet, server.URL+"/posts/2", http.StatusNotFound, 1))
	mu.Lock()
	require.Positive(t, calls, "the window is observed, not assumed")
	calls, changeAt = 0, 2
	mu.Unlock()
	require.ErrorContains(t, steps.KeepReturningFor(ctx, http.MethodGet, server.URL+"/posts/2", http.StatusNotFound, 30),
		"did not keep returning 404")
	require.ErrorContains(t, steps.KeepReturningFor(ctx, http.MethodGet, server.URL, http.StatusNotFound, 0), "too short")
}

func TestAssertBuildListedChecksTheBuildTheMetadataWasPreparedWith(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"buildId": "2026-11-18-1"})
			return
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
			{"buildId": "2026-11-18-2"},
			{"buildId": "2026-11-18-1", "metadata": map[string]any{"commit": "0a1b2c3"}},
		}})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "api-1")

	require.NoError(t, steps.prepareBuild(ctx, cloudbase.ValuesTable(t, []string{"commit", "0a1b2c3"})))
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-2"), "a deployment renders a later build")
	require.NoError(t, steps.assertBuildListed(ctx), "the metadata is checked on the build it was prepared with")

	local.Set(cloudbase.KeyBuildMetadataOf, "2026-11-18-2")
	short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	require.Error(t, steps.assertBuildListed(short), "another build does not carry the metadata")
}

func TestAssertBuildListedChecksThePreparedBuildWithoutMetadata(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	listed := []string{"2026-11-18-2", "2026-11-18-1"}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"buildId": "2026-11-18-1"})
			return
		}
		items := []map[string]any{}
		for _, id := range listed {
			items = append(items, map[string]any{"buildId": id})
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": items})
	})
	local.Set(cloudbase.KeyProjectID, "default")
	local.Set(cloudbase.KeyAPIID, "api-1")

	require.NoError(t, steps.prepareBuilds(ctx, "a"))
	require.NoError(t, cloudbase.AppendBuild(ctx, cloudbase.PrimaryAPI, "2026-11-18-2"), "a deployment renders a later build")
	require.NoError(t, steps.assertBuildListed(ctx))
	listed = []string{"2026-11-18-2"}
	short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	require.Error(t, steps.assertBuildListed(short), "the prepared build is the one checked, not the latest")
}

func TestKeepReturningForFailsWhenTheGatewayStopsAnswering(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(server.Close)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Steps: &cloudbase.Steps{Funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0),
		DataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}}

	require.ErrorContains(t, steps.KeepReturningFor(ctx, http.MethodGet, server.URL, http.StatusNotFound, 30), "did not keep returning 404",
		"a probe that gets no answer fails the window")
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}
