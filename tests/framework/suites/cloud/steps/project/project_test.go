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

package project

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestHandleDefectsBreakOneRuleEach(t *testing.T) {
	const valid = "syn-project-abcde-3"
	require.Equal(t, "SYN-PROJECT-ABCDE-3", handleDefects["uppercase"](valid))
	require.Equal(t, "syn_project_abcde_3", handleDefects["containing an underscore"](valid))
	require.Equal(t, "-"+valid, handleDefects["starting with a hyphen"](valid))
	require.Equal(t, valid+"-", handleDefects["ending with a hyphen"](valid))
	require.Equal(t, "syn--project-abcde-3", handleDefects["containing consecutive hyphens"](valid))
	long := handleDefects["longer than 40 characters"](valid)
	require.Equal(t, handleLengthBeyondStorage, len(long))
	require.True(t, strings.HasPrefix(long, valid))
	require.Equal(t, long, handleDefects["longer than 40 characters"](long), "a handle that is long enough is left alone")
}

func TestReadDefaultAndAbsentProjects(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var seen []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "PROJECT_NOT_FOUND"})
	})
	require.ErrorContains(t, steps.readDefaultProject(ctx), "project is not known")
	local.Set(cloudbase.KeyScenarioProject, cloudbase.ProjectOne)
	require.NoError(t, steps.readDefaultProject(ctx))
	require.NoError(t, steps.readAbsentProject(ctx))
	absent, err := cloudbase.RequiredValue(ctx, keyAbsentProjectID)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /projects/" + cloudbase.ProjectOne, "GET /projects/" + absent}, seen)
	require.Empty(t, registry.Pending(), "a project that is never created is never registered")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, published.StatusCode)
}

func TestCreateDuplicateProjectRepeatsTheHandleWithAnotherDisplayName(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var body map[string]string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/projects", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		cloudbase.WriteJSON(w, http.StatusConflict, map[string]string{"code": "PROJECT_EXISTS"})
	})
	require.ErrorContains(t, steps.createDuplicateProject(ctx), "no synthetic project")
	local.Set(cloudbase.KeyProjectID, "syn-project-1")
	require.NoError(t, steps.createDuplicateProject(ctx))
	require.Equal(t, "syn-project-1", body["id"])
	require.NotEqual(t, "Synthetic syn-project-1", body["displayName"])
	require.Empty(t, registry.Pending(), "the handle is already registered by the create that came first")
}

func TestRequestProjectWithDefectiveHandleRegistersTheHandleBeforeTheRequest(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var registeredAtRequest []cleanup.Resource
	var sent map[string]string
	var steps *Steps
	var ctx context.Context
	var registry *cleanup.Registry
	respond := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
			registeredAtRequest = registry.Pending()
			cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request"})
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "PROJECT_NOT_FOUND"})
	}
	steps, ctx, _, registry = cloudSteps(t, respond)

	require.ErrorContains(t, steps.requestProjectWithDefectiveHandle(ctx, "with a typo"), "unknown handle defect")
	require.NoError(t, steps.requestProjectWithDefectiveHandle(ctx, "uppercase"))
	require.Equal(t, strings.ToUpper(sent["id"]), sent["id"])
	require.Equal(t, []cleanup.Resource{{Kind: cleanup.KindProject, ID: sent["id"], Actor: cloudbase.ResourceActor, Description: "project " + sent["id"]}},
		registeredAtRequest, "the handle is registered before the request that may create it")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)

	require.NoError(t, steps.assertRejectedProjectAbsent(ctx))
}

func TestRequestProjectWithDefectiveHandleRegistersTheIDAnAcceptedProjectGot(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]any{"id": "renamed-by-the-platform"})
	})
	require.NoError(t, steps.requestProjectWithDefectiveHandle(ctx, "ending with a hyphen"))
	ids := []string{}
	for _, resource := range registry.Pending() {
		ids = append(ids, resource.ID)
	}
	require.Contains(t, ids, "renamed-by-the-platform")
	require.Len(t, ids, 2)
}

func TestAssertRejectedProjectAbsent(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusNotFound
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, status, map[string]string{"code": "x"})
	})
	require.ErrorContains(t, steps.assertRejectedProjectAbsent(ctx), "requested no project")
	local.Set(keyRejectedProjectID, "Bad-Handle")
	require.NoError(t, steps.assertRejectedProjectAbsent(ctx))
	status = http.StatusOK
	require.ErrorContains(t, steps.assertRejectedProjectAbsent(ctx), "should not exist")
}

func TestAwaitProjectActive(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := "active"
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "syn-project-1", "status": status})
	})
	require.Error(t, steps.awaitProjectActive(ctx))
	local.Set(cloudbase.KeyProjectID, "syn-project-1")
	require.NoError(t, steps.awaitProjectActive(ctx))
	status = "failed"
	started := time.Now()
	require.ErrorContains(t, steps.awaitProjectActive(ctx), "failed")
	require.Less(t, time.Since(started), 10*time.Second, "a failed project is not waited for")
}

func TestProjectPipelineReadSteps(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	bound := "release"
	pipelines := `{"list":[{"id":"default","name":"default"},{"id":"release","name":"Release","promotionPaths":[]}]}`
	var seen []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/projects/payments/deployment-pipeline":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"pipeline": bound})
		case "/pipelines":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(pipelines))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	require.ErrorContains(t, steps.readProjectPipeline(ctx), "no project")
	local.Set(cloudbase.KeyProjectID, "payments")

	require.NoError(t, steps.readProjectPipeline(ctx))
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusOK))
	require.NoError(t, steps.assertBoundPipelineListed(ctx), "a pipeline is matched by its id")
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusOK), "the listing read does not replace the binding")

	bound = "Release"
	require.NoError(t, steps.readProjectPipeline(ctx))
	require.NoError(t, steps.assertBoundPipelineListed(ctx), "a pipeline is matched by its name")

	bound = "retired"
	require.NoError(t, steps.readProjectPipeline(ctx))
	require.ErrorContains(t, steps.assertBoundPipelineListed(ctx), `"retired" is not listed`)
	for _, request := range seen {
		require.True(t, strings.HasPrefix(request, "GET "), "the steps only read: %s", request)
	}
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}
