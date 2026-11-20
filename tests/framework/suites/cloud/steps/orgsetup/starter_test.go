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

package orgsetup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/setup/finalizer"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// leftoverStub serves per-collection lists and records the deletions it was asked for.
type leftoverStub struct {
	mu      sync.Mutex
	lists   map[string][]map[string]any
	failing map[string]int
	deleted []string
	// listedProjects records, per collection, the project of each per-project listing.
	listedProjects map[string][]string
	// published holds the publication routes that answer 200; every other one answers 404.
	published map[string]bool
}

func (s *leftoverStub) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer console-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/publication") {
		if s.published[r.URL.Path] {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"status": "PUBLISHED"})
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"code": "PUBLICATION_NOT_FOUND"})
		return
	}
	if r.Method == http.MethodGet {
		key := r.URL.Path
		if slices.Contains(projectScopedCollections, r.URL.Path) {
			project := r.URL.Query().Get("projectId")
			if project == "" {
				cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": "VALIDATION_FAILED", "message": "projectId query parameter is required"})
				return
			}
			s.listedProjects[r.URL.Path] = append(s.listedProjects[r.URL.Path], project)
			key += "?projectId=" + project
		}
		if status, bad := s.failing[key]; bad {
			w.WriteHeader(status)
			return
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": s.lists[key]})
		return
	}
	if status, bad := s.failing[r.URL.Path]; bad {
		w.WriteHeader(status)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/")
	if slices.Contains(s.deleted, id) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.deleted = append(s.deleted, id)
	w.WriteHeader(http.StatusNoContent)
}

func newLeftoverStub() *leftoverStub {
	return &leftoverStub{failing: map[string]int{}, listedProjects: map[string][]string{}, published: map[string]bool{
		"/api-portals/dap-abcde234/apis/rest-api/syn-api-abcde234-1/publication": true,
		"/api-portals/ops-portal/apis/rest-api/syn-api-abcde234-2/publication":   true,
		"/api-portals/ops-portal/apis/rest-api/ops-api/publication":              true,
	}, lists: map[string][]map[string]any{
		"/secrets": {{"id": "syn-secret-abcde234-1"}, {"id": "ops-secret"}},
		"/managed-api-portals": {
			{"id": "dap-abcde234", "name": "dap-abcde234", "status": "active"},
			{"id": "syn-portal-abcde234-3", "name": "Synthetic syn-portal-abcde234-3", "status": "pending"},
			{"id": "ops-portal", "name": "Operations portal", "status": "active"},
		},
		"/self-hosted-gateways": {
			{"id": "development-syn-shgw-abcde234-1", "displayName": "syn-shgw-abcde234-1", "environment": "development"},
			{"id": "development-ops-shgw", "displayName": "ops-shgw", "environment": "development"},
		},
		"/managed-gateways": {
			{"id": "development-syn-gw-abcde234-1", "environment": "development"},
			{"id": "development-default", "environment": "development"},
			{"id": "development-syn-gw-zzzzz222-1", "environment": "development"},
			{"id": "development-syn-gw-ab", "displayName": "syn-gw-abcde234-12", "environment": "development"},
		},
		"/projects": {{"id": cloudbase.ProjectOne}, {"id": "syn-project-abcde234-2"}, {"id": cloudbase.ProjectTwo}},
		"/rest-apis?projectId=" + cloudbase.ProjectOne:    {{"id": "syn-api-abcde234-1"}, {"id": "ops-api"}},
		"/rest-apis?projectId=syn-project-abcde234-2":     {{"id": "syn-api-abcde234-2"}},
		"/rest-apis?projectId=" + cloudbase.ProjectTwo:    {{"id": "syn-api-abcde234-9"}, {"id": "ops-project-api"}},
		"/graphql-apis?projectId=" + cloudbase.ProjectOne: {{"id": "syn-gql-abcde234-1"}, {"id": "ops-gql"}},
		"/graphql-apis?projectId=" + cloudbase.ProjectTwo: {{"id": "syn-gql-zzzzz222-1"}},
		"/subscription-plans":                             {{"id": "syn-plan-abcde234-1"}, {"id": "syn-plan-zzzzz222-1"}, {"id": "ops-plan"}},
	}}
}

// sweepLeftovers runs the cloud sweeps for the suffix the stub's leftovers carry.
func sweepLeftovers(ctx context.Context, base string, timeout time.Duration) ([]string, error) {
	finals := finalizer.NewTracker(slog.Default())
	finals.Track("abcde234")
	client := httpx.NewClient(httpx.Options{Timeout: timeout})
	return finals.Sweep(ctx, cloudSweeps(client, base, "console-token", finals.Owns))
}

func TestCloudSweepsDeleteOnlyTheRunsLeftoversInDependencyOrder(t *testing.T) {
	stub := newLeftoverStub()
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, []string{
		"api-portals/dap-abcde234/apis/rest-api/syn-api-abcde234-1/unpublish",
		"api-portals/ops-portal/apis/rest-api/syn-api-abcde234-2/unpublish",
		"managed-api-portals/syn-portal-abcde234-3",
		"rest-apis/syn-api-abcde234-1",
		"rest-apis/syn-api-abcde234-9",
		"rest-apis/syn-api-abcde234-2",
		"graphql-apis/syn-gql-abcde234-1",
		"subscription-plans/syn-plan-abcde234-1",
		"secrets/syn-secret-abcde234-1",
		"self-hosted-gateways/development-syn-shgw-abcde234-1",
		"managed-gateways/development-syn-gw-abcde234-1",
		"managed-gateways/development-syn-gw-ab",
	}, stub.deleted[:12])
	require.Len(t, removed, 12)
	for _, id := range stub.deleted {
		require.NotContains(t, id, "projects/syn-project", "the finalizer never deletes a project")
	}
	for _, collection := range projectScopedCollections {
		listed := slices.Clone(stub.listedProjects[collection])
		slices.Sort(listed)
		require.Equal(t, []string{cloudbase.ProjectOne, cloudbase.ProjectTwo, "syn-project-abcde234-2"}, slices.Compact(listed),
			collection+" is listed for the suite's projects and the run's projects only")
	}
	for _, id := range stub.deleted {
		require.NotEqual(t, "managed-gateways/development-default", id, "the default gateway is never deleted")
		require.NotEqual(t, "managed-gateways/development-syn-gw-zzzzz222-1", id, "another run's gateway stays")
		require.NotEqual(t, "self-hosted-gateways/development-ops-shgw", id)
		require.NotEqual(t, "secrets/ops-secret", id)
		require.NotContains(t, id, "ops-api", "an API the organisation owns stays")
		require.NotContains(t, id, "ops-project-api")
		require.NotContains(t, id, "ops-gql")
		require.NotContains(t, id, "zzzzz222", "another run's APIs and plans stay")
		require.NotEqual(t, "subscription-plans/ops-plan", id, "a plan the organisation owns stays")
		require.NotContains(t, id, "managed-api-portals/ops-portal")
		require.NotContains(t, id, "managed-api-portals/dap-")
		require.False(t, strings.HasPrefix(id, "projects/apip-test-"), "the suite's projects are never deleted: "+id)
	}
}

func TestCloudSweepsStillSweepTheSuiteProjectsWhenTheProjectsCannotBeListed(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/projects"] = http.StatusForbidden
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.ErrorContains(t, err, "listing the projects that hold /rest-apis")
	require.Contains(t, removed, "/rest-apis/syn-api-abcde234-1", "the suite's projects are still swept")
	require.NotContains(t, removed, "/rest-apis/syn-api-abcde234-2")
}

func TestCloudSweepsKeepSweepingTheOtherProjectsWhenOneProjectCannotBeListed(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/rest-apis?projectId="+cloudbase.ProjectOne] = http.StatusForbidden
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.ErrorContains(t, err, "project "+cloudbase.ProjectOne)
	require.Contains(t, removed, "/rest-apis/syn-api-abcde234-2")
	require.NotContains(t, removed, "/rest-apis/syn-api-abcde234-1")
}

func TestCloudSweepsCarryOnPastAListingThatFails(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/secrets"] = http.StatusForbidden
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.ErrorContains(t, err, "listing /secrets")
	require.Contains(t, removed, "/managed-gateways/development-syn-gw-abcde234-1")
	require.NotContains(t, removed, "/projects/syn-project-abcde234-2")
}

func TestCloudSweepsSkipACollectionThePlatformDoesNotServe(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/self-hosted-gateways"] = http.StatusNotFound
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.NoError(t, err, "a route the platform does not serve holds no leftovers")
	require.NotContains(t, removed, "/projects/syn-project-abcde234-2")
	require.NotContains(t, removed, "/self-hosted-gateways/development-syn-shgw-abcde234-1")
}

func TestCloudSweepsReportALeftoverThatCannotBeDeleted(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/managed-gateways/development-syn-gw-abcde234-1"] = http.StatusConflict
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	removed, err := sweepLeftovers(ctx, base, 2*time.Second)
	require.ErrorContains(t, err, "/managed-gateways/development-syn-gw-abcde234-1")
	require.Contains(t, removed, "/secrets/syn-secret-abcde234-1")
	require.NotContains(t, removed, "/managed-gateways/development-syn-gw-abcde234-1")
}

func TestFinalizerSweepsTheRunnersItObserved(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stub := newLeftoverStub()
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	finalizer := NewFinalizer()

	removed, err := finalizer.Finalize(context.Background())
	require.NoError(t, err)
	require.Empty(t, removed, "a run that observed no runner has nothing to look for")

	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, unique.Install(ctx, generator))
	require.NoError(t, finalizer.observe(ctx, topo))
	stub.lists["/managed-gateways"] = []map[string]any{
		{"id": "development-syn-gw-" + generator.Suffix() + "-1"}, {"id": "development-syn-gw-zzzzz222-1"},
	}

	removed, err = finalizer.Finalize(context.Background())
	require.NoError(t, err)
	require.Contains(t, removed, "/managed-gateways/development-syn-gw-"+generator.Suffix()+"-1")
	require.NotContains(t, removed, "/managed-gateways/development-syn-gw-zzzzz222-1", "another run's gateway stays")
}

func TestFinalizerNeedsTheConsoleTokenToLookForLeftovers(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "")
	topo := cloudbase.CloudAPIStub(t, newLeftoverStub().handler)
	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, unique.Install(ctx, generator))
	finalizer := NewFinalizer()
	require.NoError(t, finalizer.observe(ctx, topo))

	_, err = finalizer.Finalize(context.Background())
	require.ErrorContains(t, err, cloudbase.CloudConsoleAccessTokenEnv)
}

func TestTheDefaultGatewayIsInAStageTheProjectDeploysToDirectly(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{
		paths:    cloudbase.PromotionChainPaths("development", "apip-test-e1"),
		gateways: chainGateways("apip-test-e1", "development"),
	}

	outcome, err := ensureForTest(t, platform)

	require.NoError(t, err)
	require.Equal(t, "development-gateway", outcome.ID, "a gateway in a promotion target is listed first and passed over")
	require.Equal(t, "development", outcome.Environment)
	require.Empty(t, platform.created)
}

// starterPlatform is a fake organisation for the starter: a default project bound to a pipeline,
// a list of managed gateways, and a record of the gateways it was asked to create.
type starterPlatform struct {
	mu        sync.Mutex
	paths     []cloudbase.PromotionPath
	gateways  []map[string]any
	created   []map[string]any
	createAs  int
	polls     int
	activeAt  int
	failsWith string
	// absent makes a gateway lookup answer 404, as when a refused create left nothing behind.
	absent bool

	// portals are the managed API Portals the organisation holds; the zero value is an
	// organisation with one active portal, so tests of the gateway need not mention portals.
	portals        []map[string]any
	noPortals      bool
	createdPortals []map[string]any
	portalCreateAs int
	portalPolls    int
	portalActiveAt int
	portalFails    bool
	// portalAbsent makes a portal lookup answer 404, as when a refused create left nothing behind.
	portalAbsent bool

	// noProjectOne and noProjectTwo make that project of the suite answer 404; noPipelineBinding
	// makes project one's deployment pipeline answer 404.
	noProjectOne      bool
	noProjectTwo      bool
	noPipelineBinding bool
	// projectCreate is the status a request to create a project is answered with; zero creates it.
	// createdProjects are the handles of the projects the starter created.
	projectCreate   int
	createdProjects []string
	// environments are what GET /environments lists.
	environments []map[string]any
	// paths are the promotion paths of project one's pipeline, and pipelinePaths those of project
	// two's.
	pipelinePaths []cloudbase.PromotionPath
	// gatewayPipelineName and pipelineName are the names of those pipelines; empty means
	// "apip-test-pipeline-1" and "apip-test-pipeline-2".
	gatewayPipelineName string
	pipelineName        string
	// pipelinesAnswer, when set, is the status the pipeline list answers with instead of the list.
	pipelinesAnswer int
	// deleted records the gateways and portals the starter was asked to remove.
	deleted []string
	// requests counts every request the organisation answered; gatewayLists those that listed the
	// managed gateways.
	requests     int
	gatewayLists int
}

// boundPipelineName is the name the fake lists project two's pipeline under.
func (p *starterPlatform) boundPipelineName() string {
	if p.pipelineName != "" {
		return p.pipelineName
	}
	return "apip-test-pipeline-2"
}

// gatewayPipeline is the name the fake lists project one's pipeline under.
func (p *starterPlatform) gatewayPipeline() string {
	if p.gatewayPipelineName != "" {
		return p.gatewayPipelineName
	}
	return "apip-test-pipeline-1"
}

func (p *starterPlatform) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		require.Equal(t, "Bearer console-token", r.Header.Get("Authorization"))
		p.requests++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			var request map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			if p.projectCreate != 0 {
				w.WriteHeader(p.projectCreate)
				_, _ = w.Write([]byte(`{"code":"refused","message":"the project was not created"}`))
				return
			}
			p.createdProjects = append(p.createdProjects, request["id"])
			p.noProjectOne = p.noProjectOne && request["id"] != cloudbase.ProjectOne
			p.noProjectTwo = p.noProjectTwo && request["id"] != cloudbase.ProjectTwo
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": request["id"]})
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectOne && p.noProjectOne:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectOne:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": cloudbase.ProjectOne})
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectTwo && p.noProjectTwo:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectTwo:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": cloudbase.ProjectTwo})
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectTwo+"/deployment-pipeline":
			_ = json.NewEncoder(w).Encode(map[string]any{"pipeline": p.boundPipelineName()})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/projects/") && strings.Count(r.URL.Path, "/") == 2:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/environments":
			environments := p.environments
			if environments == nil {
				environments = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"list": environments})
		case r.Method == http.MethodDelete:
			p.deleted = append(p.deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectOne+"/deployment-pipeline" && p.noPipelineBinding:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/"+cloudbase.ProjectOne+"/deployment-pipeline":
			_ = json.NewEncoder(w).Encode(map[string]any{"pipeline": p.gatewayPipeline()})
		case r.Method == http.MethodGet && r.URL.Path == "/pipelines" && p.pipelinesAnswer != 0:
			w.WriteHeader(p.pipelinesAnswer)
		case r.Method == http.MethodGet && r.URL.Path == "/pipelines":
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
				{"name": "other", "promotionPaths": []cloudbase.PromotionPath{{SourceEnvironment: "elsewhere"}}},
				{"name": p.gatewayPipeline(), "promotionPaths": p.paths},
				{"name": p.boundPipelineName(), "promotionPaths": p.pipelinePaths},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/managed-gateways":
			p.gatewayLists++
			if kind := r.URL.Query().Get("functionalityType"); kind != "" {
				require.Equal(t, cloudbase.RegularGatewayKind, kind)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"list": p.gateways})
		case r.Method == http.MethodPost && r.URL.Path == "/managed-gateways":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if handle, _ := body["handle"].(string); len(handle) == cloudbase.HandleProbeLength {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(cloudbase.HandleRefusal(body["environment"].(string))))
				return
			}
			p.created = append(p.created, body)
			if p.createAs != 0 {
				w.WriteHeader(p.createAs)
				_, _ = w.Write([]byte(`{"error":"conflict"}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": cloudbase.ExpectedGatewayID(body["environment"].(string), body["displayName"].(string), ""), "status": "PROVISIONING"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/managed-gateways/") && p.absent:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/managed-gateways/"):
			p.polls++
			status := "PROVISIONING"
			if p.polls >= p.activeAt {
				status = "ACTIVE"
			}
			answer := map[string]any{"id": strings.TrimPrefix(r.URL.Path, "/managed-gateways/"), "status": status}
			if p.failsWith != "" {
				answer["status"], answer["statusReason"] = "FAILED", p.failsWith
			}
			_ = json.NewEncoder(w).Encode(answer)
		case r.Method == http.MethodGet && r.URL.Path == "/managed-api-portals":
			portals := p.portals
			if portals == nil && !p.noPortals {
				portals = []map[string]any{{"id": "apip-test-existing", "status": "active"}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"list": portals})
		case r.Method == http.MethodPost && r.URL.Path == "/managed-api-portals":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			p.createdPortals = append(p.createdPortals, body)
			if p.portalCreateAs != 0 {
				w.WriteHeader(p.portalCreateAs)
				_, _ = w.Write([]byte(`{"error":"conflict"}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": body["handle"], "status": "pending"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/managed-api-portals/") && p.portalAbsent:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/managed-api-portals/"):
			p.portalPolls++
			status := "pending"
			if p.portalPolls >= p.portalActiveAt {
				status = "active"
			}
			if p.portalFails {
				status = "failed"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": strings.TrimPrefix(r.URL.Path, "/managed-api-portals/"), "status": status})
		case r.Method == http.MethodGet && slices.Contains([]string{"/projects", "/environments", "/secrets", "/rest-apis", "/graphql-apis", "/subscription-plans", "/self-hosted-gateways"}, r.URL.Path):
			// What the finalizer reads: this organisation holds none of it.
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotImplemented)
		}
	}
}

func (p *starterPlatform) api(t *testing.T) cloudbase.OrganisationAPI {
	server := httptest.NewServer(p.handler(t))
	t.Cleanup(server.Close)
	return cloudbase.OrganisationAPI{Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), Base: server.URL, Token: "console-token", PollInterval: retry.BaseInterval}
}

// testDefaultGateway is the display name a starter with the suffix "abcde234" gives its gateway.
const testDefaultGateway = defaultGatewayDisplayName + "-abcde234"

func ensureForTest(t *testing.T, platform *starterPlatform) (gatewayOutcome, error) {
	return ensureDefaultGateway(context.Background(), platform.api(t), testDefaultGateway, cloudbase.ProjectOne, slog.Default())
}

func developmentPipeline() []cloudbase.PromotionPath {
	return []cloudbase.PromotionPath{{SourceEnvironment: "development", TargetEnvironments: []string{"production"}}}
}

func TestEnsureDefaultGatewayLeavesAnActiveGatewayAlone(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "ai-gw", "functionalityType": "ai", "environment": "development", "status": "ACTIVE", "isActive": true},
		{"id": "sgw", "displayName": "sgw-abcde234-1", "environment": "development", "status": "ACTIVE", "isActive": true},
		{"id": "elsewhere-gw", "environment": "elsewhere", "status": "ACTIVE", "isActive": true},
		{"id": "development-default", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "development-default", Environment: "development"}, outcome, "an existing gateway is used, not created")
	require.Empty(t, platform.created)
}

func TestEnsureDefaultGatewayCreatesOneWhenNoneIsUsable(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 2, gateways: []map[string]any{
		{"id": "sgw", "displayName": "sgw-abcde234-1", "environment": "development", "status": "ACTIVE", "isActive": true},
		{"id": "stopped", "environment": "development", "status": "DISCONNECTED", "isActive": false},
	}}
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "development-" + testDefaultGateway, Environment: "development", Created: true}, outcome)
	require.Len(t, platform.created, 1)
	require.Equal(t, map[string]any{
		"displayName": testDefaultGateway, "environment": "development", "functionalityType": cloudbase.RegularGatewayKind,
		"description": platform.created[0]["description"],
	}, platform.created[0])
	require.False(t, notSyntheticGatewayRejects(testDefaultGateway), "the scenarios must be able to choose the default gateway")
	require.GreaterOrEqual(t, platform.polls, 2, "it waits for the gateway to become active")
}

func notSyntheticGatewayRejects(displayName string) bool {
	return cloudbase.NotSyntheticGateway(map[string]any{"displayName": displayName}) != ""
}

func TestEnsureDefaultGatewayWaitsForAGatewayThatIsBeingBuilt(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 2, gateways: []map[string]any{
		{"id": "development-" + testDefaultGateway, "environment": "development", "status": "PROVISIONING"},
	}}
	_, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Empty(t, platform.created, "a gateway that is being built is not asked for again")
}

func TestEnsureDefaultGatewayReportsAFailedGateway(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), failsWith: "no capacity"}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "no capacity")
	require.ErrorContains(t, err, "development-"+testDefaultGateway)
}

func TestEnsureDefaultGatewayWaitsForTheGatewayAConflictNames(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusConflict}
	_, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Len(t, platform.created, 1)
}

func TestEnsureDefaultGatewayRefusesOtherAnswers(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusForbidden}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "creating the default gateway")
	require.ErrorIs(t, err, errUnavailable, "a platform that refuses the token cannot be asked to build a gateway")

	platform = &starterPlatform{paths: developmentPipeline(), createAs: http.StatusInternalServerError}
	_, err = ensureForTest(t, platform)
	require.ErrorContains(t, err, "creating the default gateway")
	require.NotErrorIs(t, err, errUnavailable, "a gateway the platform failed to create is a failure")
}

func TestEnsureDefaultGatewayNeverCreatesTheGatewayAPinNames(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "wanted")
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "other", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, cloudbase.CloudGatewayPinEnv)
	require.NotErrorIs(t, err, errUnavailable, "a pinned gateway the organisation lacks fails the run")
	require.Empty(t, platform.created)

	platform.gateways = append(platform.gateways, map[string]any{"id": "wanted", "environment": "development", "status": "ACTIVE", "isActive": true})
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "wanted", Environment: "development"}, outcome)
}

func TestEnsureDefaultGatewayUsesOnlyAGatewayOfAnEntryEnvironment(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	paths := []cloudbase.PromotionPath{{SourceEnvironment: "development", TargetEnvironments: []string{"apip-test-e1"}}}
	targetGateway := map[string]any{"id": "apip-test-e1-default", "environment": "apip-test-e1", "status": "ACTIVE", "isActive": true}

	platform := &starterPlatform{paths: paths, gateways: []map[string]any{
		targetGateway,
		{"id": "development-default", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "development-default", Environment: "development"}, outcome,
		"a gateway of a promotion target is passed over even when it is listed first")
	require.Empty(t, platform.created)

	platform = &starterPlatform{paths: paths, activeAt: 2, gateways: []map[string]any{targetGateway}}
	outcome, err = ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "development-" + testDefaultGateway, Environment: "development", Created: true}, outcome,
		"with no gateway in the entry environment, one is created there")
}

func TestEnsureDefaultGatewayRefusesAPinnedGatewayOfAPromotionTarget(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "apip-test-e1-default")
	platform := &starterPlatform{
		paths: []cloudbase.PromotionPath{{SourceEnvironment: "development", TargetEnvironments: []string{"apip-test-e1"}}},
		gateways: []map[string]any{
			{"id": "apip-test-e1-default", "environment": "apip-test-e1", "status": "ACTIVE", "isActive": true},
		},
	}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "not an active regular gateway in an entry environment")
	require.NotErrorIs(t, err, errUnavailable, "a pinned gateway the platform cannot deploy to fails the run")
	require.Empty(t, platform.created)
}

func TestEnsureDefaultGatewayNeedsAPipeline(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "no environment")
	require.NotErrorIs(t, err, errUnavailable, "a pipeline with no stage to build in fails the run: the starter cannot fix it")
}

func TestDefaultGatewayEnvironmentPrefersDevelopmentThenTheFirstStage(t *testing.T) {
	for name, tc := range map[string]struct {
		paths []cloudbase.PromotionPath
		want  string
		fails bool
	}{
		"development is reached": {developmentPipeline(), "development", false},
		"development is a target": {[]cloudbase.PromotionPath{{SourceEnvironment: "dev-a", TargetEnvironments: []string{"development"}}},
			"dev-a", false},
		"development is between two stages": {[]cloudbase.PromotionPath{
			{SourceEnvironment: "build", TargetEnvironments: []string{"development"}},
			{SourceEnvironment: "development", TargetEnvironments: []string{"prod"}},
		}, "build", false},
		"development is one of several entries": {[]cloudbase.PromotionPath{
			{SourceEnvironment: "alpha", TargetEnvironments: []string{"prod"}},
			{SourceEnvironment: "development", TargetEnvironments: []string{"prod"}},
		}, "development", false},
		"the first stage in name order": {[]cloudbase.PromotionPath{
			{SourceEnvironment: "zeta", TargetEnvironments: []string{"prod"}},
			{SourceEnvironment: "alpha", TargetEnvironments: []string{"prod"}},
		}, "alpha", false},
		"a stage that is a target of another is not first": {[]cloudbase.PromotionPath{
			{SourceEnvironment: "qa", TargetEnvironments: []string{"prod"}},
			{SourceEnvironment: "build", TargetEnvironments: []string{"qa"}},
		}, "build", false},
		"no paths":       {nil, "", true},
		"a blank source": {[]cloudbase.PromotionPath{{TargetEnvironments: []string{"prod"}}}, "", true},
		"a cycle":        {[]cloudbase.PromotionPath{{SourceEnvironment: "a", TargetEnvironments: []string{"b"}}, {SourceEnvironment: "b", TargetEnvironments: []string{"a"}}}, "", true},
	} {
		got, err := defaultGatewayEnvironment(tc.paths)
		if tc.fails {
			require.Error(t, err, name)
			continue
		}
		require.NoError(t, err, name)
		require.Equal(t, tc.want, got, name)
	}
}

func TestStarterStartNeedsTheConsoleToken(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "")
	topo := cloudbase.CloudAPIStub(t, func(http.ResponseWriter, *http.Request) {})
	starter, err := NewStarter()
	require.NoError(t, err)
	require.ErrorContains(t, starter.Start(context.Background(), topo), cloudbase.CloudConsoleAccessTokenEnv)
}

func TestStarterSharesAPreconditionAcrossScenarios(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), gateways: activeGatewayList()}
	starter := starterAgainst(t, platform)

	require.NoError(t, starter.checkScenario(context.Background(), "first", []precondition{gatewayOne}))
	require.NoError(t, starter.checkScenario(context.Background(), "second", []precondition{gatewayOne}))

	require.Equal(t, 1, platform.gatewayLists, "the second scenario shares the first one's check")
}

func TestFinalizerRemovesTheStartersGatewayAfterTheRunnersGateways(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stub := newLeftoverStub()
	stub.lists["/managed-gateways"] = []map[string]any{
		{"id": "development-" + testDefaultGateway, "displayName": testDefaultGateway, "environment": "development"},
		{"id": "development-syn-gw-abcde234-1", "environment": "development"},
		{"id": "development-" + defaultGatewayDisplayName + "-zzzzz222", "displayName": defaultGatewayDisplayName + "-zzzzz222"},
		{"id": "development-default", "environment": "development"},
	}
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	finalizer := NewFinalizer()
	finalizer.Watch(base, "abcde234")

	removed, err := finalizer.Finalize(context.Background())
	require.NoError(t, err)
	var gateways []string
	for _, resource := range removed {
		if strings.HasPrefix(resource, "/managed-gateways/") {
			gateways = append(gateways, resource)
		}
	}
	require.Equal(t, []string{
		"/managed-gateways/development-syn-gw-abcde234-1",
		"/managed-gateways/development-" + testDefaultGateway,
	}, gateways, "the starter's gateway goes last, and another run's default gateway stays")
}

func TestStarterChecksNothingUntilAScenarioNeedsIt(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), gateways: activeGatewayList(), noPortals: true}
	starter := starterAgainst(t, platform)
	require.Zero(t, platform.requests, "starting the run asks the organisation nothing")

	require.NoError(t, starter.checkScenario(context.Background(), "a scenario that needs nothing", nil))
	require.Zero(t, platform.requests)

	require.NoError(t, starter.checkScenario(context.Background(), "a scenario that needs its project", []precondition{projectOneNeed}))
	require.Positive(t, platform.requests)
	require.Zero(t, platform.gatewayLists, "no scenario needed a gateway")
	require.Empty(t, platform.created)
	require.Empty(t, platform.createdPortals, "no scenario needed a portal")
	require.Empty(t, starter.Report())
}

func TestStarterGatewayNameFitsTheHandleLimitOfTheDevelopmentEnvironment(t *testing.T) {
	starter, err := NewStarter()
	require.NoError(t, err)
	const developmentHandleLimit = 15 // the platform allows 37 minus twice the environment name's length
	require.LessOrEqual(t, len(starter.gatewayDisplayName()), developmentHandleLimit)
	require.True(t, isStarterGateway([]string{"development-" + starter.gatewayDisplayName(), starter.gatewayDisplayName()}))
	require.False(t, isStarterGateway([]string{"development-default", "development-syn-gw-abcde234-1", "mydgw-abcde234"}))
}

func TestAGatewayTheOrganisationAlreadyHadIsUsedAndSurvivesTheFinalizer(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "development-default", "displayName": "default", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	starter := starterAgainst(t, platform)
	require.NoError(t, starter.checkScenario(context.Background(), "a scenario", []precondition{gatewayOne}))
	id, created := starter.Gateway()
	require.Equal(t, "development-default", id)
	require.False(t, created)
	require.Empty(t, platform.created)

	var mu sync.Mutex
	var deletes []string
	handler := platform.handler(t)
	topo := cloudbase.CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			deletes = append(deletes, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		handler(w, r)
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	finalizer := NewFinalizer()
	finalizer.Watch(base, "abcde234")

	removed, err := finalizer.Finalize(context.Background())
	require.NoError(t, err)
	require.Empty(t, removed)
	require.Empty(t, deletes, "nothing the run did not create is deleted")
}

func TestWhatTheStarterBuiltSurvivesTheFinalizerOfARun(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 1, noPortals: true, portalActiveAt: 1}
	starter := starterAgainst(t, platform)
	require.NoError(t, starter.checkScenario(context.Background(), "a scenario", []precondition{gatewayOne, preAPIPortal}))
	gateway, gatewayCreated := starter.Gateway()
	require.True(t, gatewayCreated)
	_, portalCreated := starter.Portal()
	require.True(t, portalCreated)

	stub := newLeftoverStub()
	stub.lists["/managed-gateways"] = []map[string]any{
		{"id": gateway, "displayName": starter.gatewayDisplayName(), "environment": "development"},
		{"id": "development-syn-gw-abcde234-1", "environment": "development"},
	}
	stub.lists["/managed-api-portals"] = []map[string]any{
		{"id": starter.portalName(), "name": starter.portalName(), "status": "active"},
	}
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	finalizer := NewFinalizer()
	finalizer.Watch(base, "abcde234")

	removed, err := finalizer.Finalize(context.Background())
	require.NoError(t, err)
	require.Contains(t, removed, "/managed-gateways/development-syn-gw-abcde234-1", "the run's own gateway goes")
	require.NotContains(t, removed, "/managed-gateways/"+gateway, "the starter's gateway stays for the next run")
	for _, id := range stub.deleted {
		require.NotContains(t, id, starter.portalName())
		require.NotContains(t, id, starter.gatewayDisplayName())
	}
}

func TestEnsureDefaultGatewayDoesNotWaitForAGatewayAConflictLeftNowhere(t *testing.T) {
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusConflict, absent: true}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "creating the default gateway")
	require.ErrorContains(t, err, "409")
}

// skipWhilePortalCreationIsOff skips a test that builds a portal while starterCreatesPortal is false.
func skipWhilePortalCreationIsOff(t *testing.T) {
	t.Helper()
	if !starterCreatesPortal {
		t.Skip("the starter does not create a portal while starterCreatesPortal is false")
	}
}

func TestStarterSkipsThePortalScenariosAndFailsTheRunInsteadOfCreatingAPortalWhileCreationIsOff(t *testing.T) {
	if starterCreatesPortal {
		t.Skip("the starter creates a portal")
	}
	platform := &starterPlatform{noPortals: true, portalActiveAt: 1}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a portal scenario", []precondition{preAPIPortal}), godog.ErrSkip)

	require.Empty(t, platform.createdPortals, "no portal is created")
	require.ErrorContains(t, starter.Failures(), "the starter does not create one", "a portal that is not found fails the run")
	require.Contains(t, strings.Join(starter.Report(), "\n"), "api-portal could not be prepared, 1 scenario(s) skipped")

	existing := &starterPlatform{portals: []map[string]any{{"id": "apip-test-acme", "status": "active"}}}
	require.NoError(t, starterAgainst(t, existing).checkScenario(context.Background(), "a portal scenario", []precondition{preAPIPortal}),
		"an active portal named with the suite's prefix is used")

	other := starterAgainst(t, &starterPlatform{portals: []map[string]any{{"id": "ops-portal", "name": "Operations portal", "status": "active"}}})
	require.ErrorIs(t, other.checkScenario(context.Background(), "a portal scenario", []precondition{preAPIPortal}), godog.ErrSkip,
		"another console's portal is not used")
	require.ErrorContains(t, other.Failures(), "named with the prefix")
}

func ensurePortalForTest(t *testing.T, platform *starterPlatform) (portalOutcome, error) {
	return ensureDefaultPortal(context.Background(), platform.api(t), defaultPortalName, slog.Default())
}

func TestEnsureDefaultPortalLeavesAnActivePortalAlone(t *testing.T) {
	platform := &starterPlatform{portals: []map[string]any{
		{"id": "broken", "status": "failed"},
		{"id": "apip-test-acme", "status": "active"},
	}}
	got, err := ensurePortalForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, portalOutcome{ID: "apip-test-acme"}, got)
	require.Empty(t, platform.createdPortals, "an organisation with a portal gets no second one")
}

func TestEnsureDefaultPortalWaitsForAPortalThatIsBeingBuilt(t *testing.T) {
	platform := &starterPlatform{noPortals: true, portalActiveAt: 3, portals: []map[string]any{
		{"id": "apip-test-building", "status": "pending"},
	}}
	got, err := ensurePortalForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, portalOutcome{ID: "apip-test-building"}, got, "a portal that is being built is waited for, not created again")
	require.Empty(t, platform.createdPortals)
	require.GreaterOrEqual(t, platform.portalPolls, 3)
}

func TestEnsureDefaultPortalCreatesOneWhenTheOrganisationHasNone(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{noPortals: true, portalActiveAt: 2}
	got, err := ensurePortalForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, portalOutcome{ID: defaultPortalName, Created: true}, got)
	require.Len(t, platform.createdPortals, 1)
	require.Equal(t, defaultPortalName, platform.createdPortals[0]["name"])
	require.Equal(t, defaultPortalName, platform.createdPortals[0]["handle"])
	require.GreaterOrEqual(t, platform.portalPolls, 2, "the portal is waited for until it is active")
}

func TestEnsureDefaultPortalIgnoresPortalsThatCannotServe(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{noPortals: true, portalActiveAt: 1, portals: []map[string]any{
		{"id": "broken", "status": "failed"},
		{"id": "going", "status": "deleting"},
	}}
	got, err := ensurePortalForTest(t, platform)
	require.NoError(t, err)
	require.True(t, got.Created, "a failed or deleting portal is no portal to publish to")
	require.Len(t, platform.createdPortals, 1)
}

func TestEnsureDefaultPortalStopsAtOnceWhenThePortalFails(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{noPortals: true, portalFails: true}
	_, err := ensurePortalForTest(t, platform)
	require.ErrorContains(t, err, "failed to provision")
}

func TestEnsureDefaultPortalDoesNotWaitForAPortalAConflictLeftNowhere(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{noPortals: true, portalCreateAs: http.StatusConflict, portalAbsent: true}
	_, err := ensurePortalForTest(t, platform)
	require.ErrorContains(t, err, "creating the managed API Portal")
	require.ErrorContains(t, err, "409")
}

func TestEnsureDefaultPortalWaitsForThePortalAConflictPointsAt(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{noPortals: true, portalCreateAs: http.StatusConflict, portalActiveAt: 1}
	got, err := ensurePortalForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, "dap-abcde234", got.ID)
}

func TestStarterUsesThePortalTheOrganisationAlreadyHas(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 1,
		portals: []map[string]any{{"id": "apip-test-acme", "status": "active"}}}
	starter := starterAgainst(t, platform)

	require.NoError(t, starter.checkScenario(context.Background(), "a portal scenario", []precondition{preAPIPortal}))

	require.Empty(t, platform.createdPortals)
	id, created := starter.Portal()
	require.Equal(t, "apip-test-acme", id)
	require.False(t, created)
}

func TestStarterPortalHandleIsValid(t *testing.T) {
	starter, err := NewStarter()
	require.NoError(t, err)
	handle := starter.portalName()
	require.GreaterOrEqual(t, len(handle), 3)
	require.LessOrEqual(t, len(handle), 40)
	require.Regexp(t, `^[a-z0-9-]+$`, handle)
}

func TestFinalizerEmptiesThePortalsAndNeverDeletesThePortalTheStarterBuilt(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	stub := newLeftoverStub()
	stub.lists["/managed-api-portals"] = []map[string]any{
		{"id": "dap-abcde234", "name": "dap-abcde234", "handle": "dap-abcde234", "status": "active"},
		{"id": "apip-test-acme", "name": "Acme portal", "handle": "apip-test-acme", "status": "active"},
	}
	stub.published = map[string]bool{
		"/api-portals/dap-abcde234/apis/rest-api/syn-api-abcde234-1/publication":   true,
		"/api-portals/apip-test-acme/apis/rest-api/syn-api-abcde234-1/publication": true,
		"/api-portals/apip-test-acme/apis/rest-api/ops-api/publication":            true,
	}
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	finalizer := NewFinalizer()
	finalizer.Watch(base, "abcde234")

	removed, err := finalizer.Finalize(context.Background())
	require.NoError(t, err)
	require.Contains(t, removed, "portal-publications/dap-abcde234/rest-api/syn-api-abcde234-1")
	require.Contains(t, removed, "portal-publications/apip-test-acme/rest-api/syn-api-abcde234-1", "the run's listing leaves a portal it did not build")
	for _, id := range removed {
		require.NotContains(t, id, "ops-api", "a listing the run did not create stays")
		require.NotEqual(t, "managed-api-portals/dap-abcde234", id, "the starter's portal is emptied, never deleted")
		require.NotEqual(t, "managed-api-portals/apip-test-acme", id)
	}
	for _, id := range stub.deleted {
		require.False(t, strings.HasPrefix(id, "managed-api-portals/dap-") || id == "managed-api-portals/apip-test-acme", id)
	}
}

func activeGatewayList() []map[string]any {
	return []map[string]any{{"id": "development-default", "environment": "development", "status": "ACTIVE", "isActive": true}}
}

// starterAgainst returns a starter that has been started against the fake organisation.
func starterAgainst(t *testing.T, platform *starterPlatform) *Starter {
	t.Helper()
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudbase.CloudGatewayPinEnv, "")
	starter, err := NewStarter()
	require.NoError(t, err)
	starter.pollInterval = retry.BaseInterval
	require.NoError(t, starter.StartAt(context.Background(), platform.api(t).Base))
	return starter
}

var (
	projectOneNeed = scoped(preProject, cloudbase.ProjectOne)
	projectTwoNeed = scoped(preProject, cloudbase.ProjectTwo)
	gatewayOne     = scoped(preDefaultGateway, cloudbase.ProjectOne)
	pipelineTwo    = scoped(prePipelineProj, cloudbase.ProjectTwo)
	chainOfTwo     = scoped(prePipelineChain2, cloudbase.ProjectTwo)
	chainOfThree   = scoped(prePipelineChain3, cloudbase.ProjectTwo)
)

// chainGateways lists an active regular gateway with an endpoint in each environment.
func chainGateways(environments ...string) []map[string]any {
	gateways := make([]map[string]any, 0, len(environments))
	for _, environment := range environments {
		gateways = append(gateways, map[string]any{
			"id": environment + "-gateway", "environment": environment, "status": "ACTIVE", "isActive": true,
			"endpoints": []any{"https://" + environment + ".gateway.example"},
		})
	}
	return gateways
}

func TestNeedsOfStepsNamesWhatEachStepAsksOfTheOrganisation(t *testing.T) {
	chainOf2 := "I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline"
	chainOf3 := "I find a promotion chain of 3 environments with an active gateway each in the project's deployment pipeline"
	pipelineOne := scoped(prePipelineProj, cloudbase.ProjectOne)
	for name, tc := range map[string]struct {
		steps   []string
		project string
		want    []precondition
	}{
		"nothing, as in a sign-in scenario": {[]string{"I obtain an APIP cloud console token", `I send a "GET" request to "x" until status 200`}, cloudbase.ProjectOne,
			[]precondition(nil)},
		"reading the test project": {[]string{"I read the APIP test project"}, cloudbase.ProjectTwo, []precondition{projectTwoNeed}},
		"creating an API in the test project": {[]string{"I create a synthetic REST API in the APIP test project with:"}, cloudbase.ProjectOne,
			[]precondition{projectOneNeed}},
		"project and gateway": {[]string{"I find the APIP test project", "I find an active APIP gateway"}, cloudbase.ProjectOne,
			[]precondition{projectOneNeed, gatewayOne}},
		"portal": {[]string{"I find an active managed API Portal"}, cloudbase.ProjectTwo,
			[]precondition{preAPIPortal}},
		"environment to host a managed gateway": {[]string{"I select an existing APIP environment for a managed gateway"}, cloudbase.ProjectOne,
			[]precondition{preEnvironment}},
		"environment to register a self-hosted gateway": {[]string{"I select an existing APIP environment for a self-hosted gateway"}, cloudbase.ProjectTwo,
			[]precondition{preEnvironment}},
		"a second environment": {[]string{"I select an existing APIP environment other than the active gateway's for a managed gateway"}, cloudbase.ProjectOne,
			[]precondition{gatewayOne, preSecondEnv}},
		"the gateway the scenario before built there": {[]string{"I work with synthetic managed APIP gateway 2"}, cloudbase.ProjectOne,
			[]precondition{gatewayOne, preSecondEnv}},
		"the first gateway the scenario before built": {[]string{"I work with synthetic managed APIP gateway 1"}, cloudbase.ProjectOne,
			[]precondition{gatewayOne, preSecondEnv}},
		"the configuration of the gateway the setup scenario built": {[]string{"I work with the configuration of synthetic managed APIP gateway 1"}, cloudbase.ProjectOne,
			[]precondition{gatewayOne}},
		"the pipeline project of project one": {[]string{"I find the pipeline-bound APIP project"}, cloudbase.ProjectOne,
			[]precondition{pipelineOne}},
		"the pipeline project of project two": {[]string{"I find the pipeline-bound APIP project"}, cloudbase.ProjectTwo,
			[]precondition{pipelineTwo}},
		"a chain of 2": {[]string{chainOf2}, cloudbase.ProjectTwo, []precondition{pipelineTwo, chainOfTwo}},
		"a chain of 3": {[]string{chainOf3}, cloudbase.ProjectTwo, []precondition{pipelineTwo, chainOfThree}},
		"report order, each once": {[]string{"I find an active managed API Portal", "I find an active APIP gateway", "I find an active APIP gateway", "I find the APIP test project"}, cloudbase.ProjectOne,
			[]precondition{projectOneNeed, gatewayOne, preAPIPortal}},
	} {
		require.Equal(t, tc.want, needsOfSteps(tc.steps, tc.project), name)
	}
}

func TestEveryPreconditionIsAskedForByAScenarioOfTheFeatureFiles(t *testing.T) {
	var mu sync.Mutex
	needs := map[string][]precondition{}
	var out bytes.Buffer
	godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
				mu.Lock()
				defer mu.Unlock()
				needs[scenario.Name] = needsOfScenario(scenario)
				return ctx, godog.ErrSkip
			})
		},
		Options: &godog.Options{Format: "progress", Paths: []string{"../../features"}, NoColors: true, Output: &out},
	}.Run()

	asked := map[precondition]bool{}
	var needingNothing int
	for _, scenarioNeeds := range needs {
		for _, need := range scenarioNeeds {
			asked[need] = true
		}
		if len(scenarioNeeds) == 0 {
			needingNothing++
		}
	}
	for _, need := range allPreconditions {
		if need == scoped(prePipelineChain3, cloudbase.ProjectOne) {
			continue // the only scenario of project one that promotes needs a chain of 2
		}
		require.True(t, asked[need], "no scenario asks for %s: its step text may have changed", need)
	}
	require.Positive(t, needingNothing, "a scenario that uses no project, such as a sign-in one, asks for nothing")
	for name, scenarioNeeds := range needs {
		if slices.Contains(scenarioNeeds, projectOneNeed) {
			require.Equal(t, projectOneNeed, scenarioNeeds[0], "the project is asked for first: "+name)
		}
		if slices.Contains(scenarioNeeds, projectTwoNeed) {
			require.Equal(t, projectTwoNeed, scenarioNeeds[0], "the project is asked for first: "+name)
		}
	}
	require.Equal(t, []precondition{pipelineTwo, chainOfThree},
		needs["Promotion follows the environment chain of the pipeline in order"])
	require.Equal(t, []precondition{projectOneNeed, gatewayOne},
		needs["A REST API can be deployed and invoked through the gateway"])
	require.Equal(t, []precondition{projectOneNeed, gatewayOne, preSecondEnv},
		needs["A gateway requested with only a name and an environment is built, connects and reports its version, and the name can be used in another environment"])
}

func TestPreconditionsNamedFindsEveryPrecondition(t *testing.T) {
	for _, need := range allPreconditions {
		got, err := preconditionsNamed(string(need))
		require.NoError(t, err)
		require.Equal(t, []precondition{need}, got)
	}
	require.Equal(t, []string{"project", "default-gateway", "environment", "second-environment",
		"pipeline-project", "pipeline-chain-2", "pipeline-chain-3", "api-portal"}, Preconditions())
	got, err := preconditionsNamed("pipeline-chain-2")
	require.NoError(t, err)
	require.Equal(t, []precondition{scoped(prePipelineChain2, cloudbase.ProjectOne), chainOfTwo}, got, "a project-scoped name stands for every project")
	_, err = preconditionsNamed("a gateway")
	require.ErrorContains(t, err, "unknown precondition")
}

func TestStarterBuildsAGatewayAndAPortalWhenAScenarioNeedsThem(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 1, noPortals: true, portalActiveAt: 1}
	starter := starterAgainst(t, platform)

	require.NoError(t, starter.checkScenario(context.Background(), "a scenario", []precondition{gatewayOne, preAPIPortal}))

	require.Len(t, platform.created, 1)
	require.Equal(t, starter.gatewayDisplayName(), platform.created[0]["displayName"])
	require.Len(t, platform.createdPortals, 1)
	require.Equal(t, starter.portalName(), platform.createdPortals[0]["handle"])
	gateway, created := starter.Gateway()
	require.Equal(t, "development-"+starter.gatewayDisplayName(), gateway)
	require.True(t, created)
	portal, created := starter.Portal()
	require.Equal(t, starter.portalName(), portal)
	require.True(t, created)
	require.NoError(t, starter.Failures())
	require.Empty(t, starter.Report())
	require.Empty(t, platform.deleted, "what the starter built and made ready stays")
}

func TestStarterCreatesAProjectTheOrganisationLacks(t *testing.T) {
	platform := &starterPlatform{noProjectOne: true}
	starter := starterAgainst(t, platform)

	require.NoError(t, starter.checkScenario(context.Background(), "a scenario", []precondition{projectOneNeed}))
	require.NoError(t, starter.checkScenario(context.Background(), "another", []precondition{projectOneNeed}))
	require.NoError(t, starter.checkScenario(context.Background(), "a runner of project two", []precondition{projectTwoNeed}))

	require.Equal(t, []string{cloudbase.ProjectOne}, platform.createdProjects, "only the missing project is created, once")
	require.NoError(t, starter.Failures())
	require.Empty(t, starter.Report())
}

func TestStarterFailsTheRunAndSkipsTheRunnersOfAProjectItCannotCreate(t *testing.T) {
	for name, status := range map[string]int{
		"the platform fails":                        http.StatusInternalServerError,
		"the project is not there after a conflict": http.StatusConflict,
	} {
		platform := &starterPlatform{noProjectOne: true, projectCreate: status}
		starter := starterAgainst(t, platform)

		require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{projectOneNeed}), godog.ErrSkip, name)
		require.ErrorIs(t, starter.checkScenario(context.Background(), "another", []precondition{projectOneNeed}), godog.ErrSkip, name)
		require.NoError(t, starter.checkScenario(context.Background(), "a runner of project two", []precondition{projectTwoNeed}),
			"%s: the runners of the other project are not skipped", name)

		require.ErrorContains(t, starter.Failures(), `creating project "apip-test-project-1"`, name)
		require.Contains(t, strings.Join(starter.Report(), "\n"), "project@apip-test-project-1 could not be prepared, 2 scenario(s) skipped", name)
	}
}

func TestStarterSkipsWithoutFailingWhenTheTokenMayNotCreateAProject(t *testing.T) {
	platform := &starterPlatform{noProjectOne: true, projectCreate: http.StatusForbidden}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{projectOneNeed}), godog.ErrSkip)

	require.NoError(t, starter.Failures(), "a platform that refuses the token the request leaves the project unavailable")
	require.Contains(t, strings.Join(starter.Report(), "\n"), "project@apip-test-project-1 is not met, 1 scenario(s) skipped")
}

func TestStarterChecksAndCreatesTheProjectsFirstAndOnlyThem(t *testing.T) {
	platform := &starterPlatform{noProjectTwo: true}
	starter := starterAgainst(t, platform)

	starter.CheckProjects(context.Background())

	require.Equal(t, []string{cloudbase.ProjectTwo}, platform.createdProjects)
	require.Empty(t, starter.Report())
	require.NoError(t, starter.Failures())
	require.Equal(t, 3, platform.requests, "two reads and the creation of the missing project: nothing else is looked at")

	starter.CheckProjects(context.Background())
	require.Equal(t, 3, platform.requests, "each project is checked once")
}

func TestARunnerOfAMissingProjectIsSkippedBeforeAnythingElseIsLookedAt(t *testing.T) {
	platform := &starterPlatform{noProjectOne: true, projectCreate: http.StatusForbidden, paths: developmentPipeline(), gateways: activeGatewayList()}
	starter := starterAgainst(t, platform)

	err := starter.checkScenario(context.Background(), "a gateway scenario", []precondition{projectOneNeed, gatewayOne})

	require.ErrorIs(t, err, godog.ErrSkip)
	require.Zero(t, platform.gatewayLists, "the gateways are not looked at while the project is missing")
	require.Equal(t, 2, platform.requests, "the project is read and its creation refused: nothing else is looked at")
}

func TestEachProjectGetsItsOwnDefaultGateway(t *testing.T) {
	gatewayTwo := scoped(preDefaultGateway, cloudbase.ProjectTwo)
	platform := &starterPlatform{noProjectOne: true, projectCreate: http.StatusForbidden,
		pipelinePaths: developmentPipeline(), gateways: activeGatewayList()}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario of project one", []precondition{gatewayOne}), godog.ErrSkip,
		"project one is missing")
	require.NoError(t, starter.checkScenario(context.Background(), "a scenario of project two", []precondition{gatewayTwo}),
		"project two is usable, so its scenarios are not skipped for project one")
	require.Empty(t, platform.created, "the organisation's gateway is used")
	id, created := starter.Gateway()
	require.Equal(t, "development-default", id)
	require.False(t, created)

	noPipeline := &starterPlatform{gateways: activeGatewayList()}
	other := starterAgainst(t, noPipeline)
	require.ErrorIs(t, other.checkScenario(context.Background(), "a scenario of project two", []precondition{gatewayTwo}), godog.ErrSkip,
		"project two's pipeline has no stage to look for a gateway in")
	require.ErrorContains(t, other.Failures(), cloudbase.ProjectTwo)
}

func TestTheGatewayNeedsProjectOneItsPipelineToo(t *testing.T) {
	platform := &starterPlatform{noProjectOne: true, projectCreate: http.StatusForbidden, paths: developmentPipeline(), gateways: activeGatewayList()}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a gateway scenario", []precondition{gatewayOne}), godog.ErrSkip)
	require.NoError(t, starter.Failures())
	require.Contains(t, strings.Join(starter.Report(), "\n"), "project \"apip-test-project-1\" is not available")

	other := starterAgainst(t, &starterPlatform{paths: developmentPipeline(), gateways: activeGatewayList(), gatewayPipelineName: "default"})
	require.ErrorIs(t, other.checkScenario(context.Background(), "a gateway scenario", []precondition{gatewayOne}), godog.ErrSkip,
		"the default pipeline is not used")
	require.Contains(t, strings.Join(other.Report(), "\n"), `bound to deployment pipeline "default"`)
}

func TestStarterSkipsScenariosAndFailsTheRunWhenTheGatewaysPipelineCannotBeRead(t *testing.T) {
	platform := &starterPlatform{noPipelineBinding: true}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{gatewayOne}), godog.ErrSkip)

	require.Error(t, starter.Failures())
	require.Empty(t, platform.created)
	require.Contains(t, strings.Join(starter.Report(), "\n"), "default-gateway@apip-test-project-1 could not be prepared")
}

func TestStarterSkipsWithoutFailingWhenTheTokenMayNotBuildAGateway(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusForbidden}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "gateway scenario", []precondition{gatewayOne}), godog.ErrSkip)

	require.NoError(t, starter.Failures(), "a token that may not build the resource is the organisation's limit, not a failure")
	require.Len(t, starter.Report(), 1)
	require.Empty(t, platform.deleted, "nothing was built, so nothing is removed")
}

func TestStarterFailsAndRemovesAGatewayItCouldNotMakeActive(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), failsWith: "no capacity"}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{gatewayOne}), godog.ErrSkip)

	err := starter.Failures()
	require.ErrorContains(t, err, "default-gateway")
	require.ErrorContains(t, err, "no capacity")
	require.ErrorContains(t, err, "was removed")
	require.Equal(t, []string{"/managed-gateways/development-" + starter.gatewayDisplayName()}, platform.deleted)
	_, created := starter.Gateway()
	require.False(t, created, "a gateway that did not become active is not the one the scenarios use")
}

func TestStarterFailsAndRemovesAPortalItCouldNotMakeActive(t *testing.T) {
	skipWhilePortalCreationIsOff(t)
	platform := &starterPlatform{noPortals: true, portalFails: true}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a portal scenario", []precondition{preAPIPortal}), godog.ErrSkip)

	err := starter.Failures()
	require.ErrorContains(t, err, "api-portal")
	require.ErrorContains(t, err, "failed to provision")
	require.Equal(t, []string{"/managed-api-portals/" + starter.portalName()}, platform.deleted)
}

func TestStarterDoesNotRemoveAGatewayItDidNotBuild(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), failsWith: "no capacity", gateways: []map[string]any{
		{"id": "development-building", "environment": "development", "status": "PROVISIONING"},
	}}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{gatewayOne}), godog.ErrSkip)

	require.Error(t, starter.Failures())
	require.Empty(t, platform.deleted, "a gateway the organisation already had is never removed")
}

func TestAFailedPreconditionSkipsOnlyTheScenariosThatNeedIt(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), failsWith: "no capacity", gateways: nil}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "needs a gateway", []precondition{projectOneNeed, gatewayOne}), godog.ErrSkip)
	require.NoError(t, starter.checkScenario(context.Background(), "needs a portal", []precondition{preAPIPortal}),
		"the organisation's portal is untouched by the gateway's failure")
	require.NoError(t, starter.checkScenario(context.Background(), "needs the project", []precondition{projectOneNeed}))
}

func TestStarterChecksTheEnvironments(t *testing.T) {
	t.Run("an organisation that lists none skips and fails the run", func(t *testing.T) {
		starter := starterAgainst(t, &starterPlatform{})
		require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{preEnvironment}), godog.ErrSkip)
		require.Error(t, starter.Failures(), "an environment the organisation lacks fails the run")
		require.Contains(t, strings.Join(starter.Report(), "\n"), "environment could not be prepared")
	})
	t.Run("the default environment passes", func(t *testing.T) {
		starter := starterAgainst(t, &starterPlatform{environments: []map[string]any{{"name": "development"}}})
		require.NoError(t, starter.checkScenario(context.Background(), "a scenario", []precondition{preEnvironment}))
	})
	t.Run("an environment named with the suite's prefix passes", func(t *testing.T) {
		starter := starterAgainst(t, &starterPlatform{environments: []map[string]any{{"name": "qa"}, {"name": "apip-test-e1"}}})
		require.NoError(t, starter.checkScenario(context.Background(), "a scenario", []precondition{preEnvironment}))
	})
	t.Run("an environment whose name leaves no room for the suite's gateway names does not count", func(t *testing.T) {
		starter := starterAgainst(t, &starterPlatform{environments: []map[string]any{{"name": "apip-test-environment"}}})
		require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{preEnvironment}), godog.ErrSkip)
		require.Error(t, starter.Failures())
		require.Contains(t, strings.Join(starter.Report(), "\n"), "allows a gateway handle of 13 characters")
	})
	t.Run("environments other consoles own are not selected", func(t *testing.T) {
		starter := starterAgainst(t, &starterPlatform{environments: []map[string]any{{"name": "qa"}, {"name": "staging"}}})
		require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{preEnvironment}), godog.ErrSkip)
		require.Error(t, starter.Failures())
	})
}

func TestStarterChecksForASecondEnvironment(t *testing.T) {
	gateways := chainGateways("development")
	for name, tc := range map[string]struct {
		environments []map[string]any
		met          bool
	}{
		"development only": {[]map[string]any{{"name": "development"}}, false},
		"a production environment does not count": {[]map[string]any{
			{"name": "development"}, {"name": "production", "isProduction": true}}, false},
		"an environment an earlier run created does not count": {[]map[string]any{
			{"name": "development"}, {"name": "syn-env-abcde234-1"}}, false},
		"an environment other consoles own does not count": {[]map[string]any{
			{"name": "development"}, {"name": "qa", "isProduction": false}}, false},
		"an environment with a name too long for gateway names does not count": {[]map[string]any{
			{"name": "development"}, {"name": "apip-test-environment", "isProduction": false}}, false},
		"a second non-production environment named with the prefix": {[]map[string]any{
			{"name": "development"}, {"name": "apip-test-e1", "isProduction": false}}, true},
	} {
		starter := starterAgainst(t, &starterPlatform{paths: developmentPipeline(), gateways: gateways, environments: tc.environments})
		err := starter.checkScenario(context.Background(), "a scenario", []precondition{preSecondEnv})
		if tc.met {
			require.NoError(t, err, name)
			continue
		}
		require.ErrorIs(t, err, godog.ErrSkip, name)
		require.Error(t, starter.Failures(), name)
	}
}

func TestASecondEnvironmentNeedsTheDefaultGateway(t *testing.T) {
	platform := &starterPlatform{noProjectOne: true, projectCreate: http.StatusForbidden, noPipelineBinding: true, environments: []map[string]any{{"name": "development"}, {"name": "apip-test-e1"}}}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a scenario", []precondition{preSecondEnv}), godog.ErrSkip)
	require.NoError(t, starter.Failures())
}

func TestStarterChecksThePipelineProject(t *testing.T) {
	chain := cloudbase.PromotionChainPaths("development", "apip-test-e1")
	for name, tc := range map[string]struct {
		platform *starterPlatform
		met      bool
		report   string
	}{
		"the project is absent": {&starterPlatform{noProjectTwo: true, projectCreate: http.StatusForbidden}, false, `creating project "apip-test-project-2"`},
		"its pipeline is another console's": {&starterPlatform{pipelinePaths: chain, pipelineName: "release"}, false,
			`bound to deployment pipeline "release"`},
		"the default pipeline is not used": {&starterPlatform{pipelinePaths: chain, pipelineName: "default"}, false,
			`bound to deployment pipeline "default"`},
		"its pipeline has no path":     {&starterPlatform{}, false, "has no promotion path"},
		"its pipeline has paths":       {&starterPlatform{pipelinePaths: chain}, true, ""},
		"its pipeline is named a test": {&starterPlatform{pipelinePaths: chain, pipelineName: "apip-test-pipeline-9"}, true, ""},
	} {
		starter := starterAgainst(t, tc.platform)
		err := starter.checkScenario(context.Background(), "a scenario", []precondition{pipelineTwo})
		if tc.met {
			require.NoError(t, err, name)
			require.Empty(t, starter.Report(), name)
			continue
		}
		require.ErrorIs(t, err, godog.ErrSkip, name)
		if name == "the project is absent" {
			require.NoError(t, starter.Failures(), "%s: a token that may not create the project is not a failure", name)
		} else {
			require.Error(t, starter.Failures(), name)
		}
		require.Contains(t, strings.Join(starter.Report(), "\n"), tc.report, name)
	}
}

func TestAPipelineListThatCannotBeReadIsAFailureNotAMissingPipeline(t *testing.T) {
	chain := cloudbase.PromotionChainPaths("development", "apip-test-e1")
	for name, tc := range map[string]struct {
		platform *starterPlatform
		failed   bool
	}{
		"the list answers 404": {&starterPlatform{pipelinePaths: chain, pipelinesAnswer: http.StatusNotFound}, true},
		"the list answers 500": {&starterPlatform{pipelinePaths: chain, pipelinesAnswer: http.StatusInternalServerError}, true},
	} {
		starter := starterAgainst(t, tc.platform)
		err := starter.checkScenario(context.Background(), "a scenario", []precondition{pipelineTwo})
		require.ErrorIs(t, err, godog.ErrSkip, name)
		if tc.failed {
			require.Error(t, starter.Failures(), name)
		} else {
			require.NoError(t, starter.Failures(), name)
		}
	}
}

func TestAProjectWithoutAPipelineBindingIsMissingNotFailed(t *testing.T) {
	require.True(t, pipelineMissing(&cloudbase.PipelineBindingMissingError{Cause: errors.New("binding is absent")}))
	require.True(t, pipelineMissing(&cloudbase.PipelineNotListedError{Pipeline: "p"}))
	require.False(t, pipelineMissing(&cloudbase.StatusError{Status: http.StatusNotFound}), "a 404 that is not the binding route's says nothing about the binding")
}

func TestAPromotionChainNeedsEveryGatewayItCountsToHaveAnID(t *testing.T) {
	gateways := chainGateways("development", "apip-test-e1")
	gateways[1]["id"] = " "
	platform := &starterPlatform{pipelinePaths: cloudbase.PromotionChainPaths("development", "apip-test-e1"), gateways: gateways}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a promotion", []precondition{chainOfTwo}), godog.ErrSkip)
}

func TestPrepareNeedsStartBeforeItChecksAnything(t *testing.T) {
	starter, err := NewStarter()
	require.NoError(t, err)

	require.ErrorContains(t, starter.Prepare(context.Background(), []string{"project"}), "Start must be called")
	require.ErrorContains(t, starter.Prepare(context.Background(), nil), "Start must be called")
}

func TestStarterChecksThePromotionChains(t *testing.T) {
	platform := &starterPlatform{
		pipelinePaths: cloudbase.PromotionChainPaths("development", "apip-test-qa", "apip-test-production"),
		gateways:      chainGateways("development", "apip-test-qa"),
	}
	starter := starterAgainst(t, platform)

	require.NoError(t, starter.checkScenario(context.Background(), "a promotion of 2", []precondition{pipelineTwo, chainOfTwo}))
	require.ErrorIs(t, starter.checkScenario(context.Background(), "a promotion of 3", []precondition{pipelineTwo, chainOfThree}), godog.ErrSkip,
		"production holds no active gateway")
	require.Error(t, starter.Failures(), "a chain the organisation lacks fails the run")
	require.Contains(t, strings.Join(starter.Report(), "\n"), "no promotion chain of 3 environments")

	platform.gateways = append(platform.gateways, chainGateways("apip-test-production")...)
	again := starterAgainst(t, platform)
	require.NoError(t, again.checkScenario(context.Background(), "a promotion of 3", []precondition{pipelineTwo, chainOfThree}))
}

func TestAPromotionChainNeedsAnActiveGatewayInEveryEnvironment(t *testing.T) {
	gateways := chainGateways("development", "apip-test-qa")
	gateways[1]["isActive"] = false
	platform := &starterPlatform{pipelinePaths: cloudbase.PromotionChainPaths("development", "apip-test-qa"), gateways: gateways}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a promotion", []precondition{chainOfTwo}), godog.ErrSkip)
}

func TestAPromotionChainNeedsEnvironmentsTheSuiteMayUse(t *testing.T) {
	platform := &starterPlatform{
		pipelinePaths: cloudbase.PromotionChainPaths("development", "qa"),
		gateways:      chainGateways("development", "qa"),
	}
	starter := starterAgainst(t, platform)

	require.ErrorIs(t, starter.checkScenario(context.Background(), "a promotion", []precondition{chainOfTwo}), godog.ErrSkip,
		"qa has an active gateway but belongs to other consoles")
	require.Error(t, starter.Failures())
}

func TestPrepareChecksTheNamedPreconditionsAndReportsFailures(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), gateways: activeGatewayList(), noProjectTwo: true, projectCreate: http.StatusForbidden}
	starter := starterAgainst(t, platform)

	require.NoError(t, starter.Prepare(context.Background(), []string{"default-gateway@apip-test-project-1"}))
	id, created := starter.Gateway()
	require.Equal(t, "development-default", id)
	require.False(t, created)
	require.Empty(t, starter.Report(), "only the named precondition was checked")

	require.NoError(t, starter.Prepare(context.Background(), []string{"project"}), "an unavailable precondition is reported, not an error")
	require.Len(t, starter.Report(), 1, "project two is missing; project one is there")

	require.ErrorContains(t, starter.Prepare(context.Background(), []string{"a gateway"}), "unknown precondition")
}

func TestPrepareReturnsWhatTheStarterFailedToPrepare(t *testing.T) {
	platform := &starterPlatform{paths: developmentPipeline(), failsWith: "no capacity"}
	starter := starterAgainst(t, platform)

	err := starter.Prepare(context.Background(), []string{"default-gateway@apip-test-project-1"})

	require.ErrorContains(t, err, "no capacity")
}

func TestStartedStarterNeedsStartBeforeAPreconditionIsChecked(t *testing.T) {
	starter, err := NewStarter()
	require.NoError(t, err)
	require.ErrorContains(t, starter.require(context.Background(), projectOneNeed), "Start must be called")
}

func TestCloudSweepsLeaveAProjectOfTheSuiteTheOrganisationDoesNotListAlone(t *testing.T) {
	stub := newLeftoverStub()
	stub.lists["/projects"] = []map[string]any{{"id": cloudbase.ProjectOne}, {"id": "syn-project-abcde234-2"}}
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	_, err = sweepLeftovers(context.Background(), base, 5*time.Second)

	require.NoError(t, err, "a project the runners skipped for is not a leftover the finalizer must fail on")
	require.NotContains(t, stub.listedProjects["/rest-apis"], cloudbase.ProjectTwo)
	require.Contains(t, stub.listedProjects["/rest-apis"], cloudbase.ProjectOne)
}

func TestCloudSweepsReadTheAPIsOfTheSuiteProjects(t *testing.T) {
	stub := newLeftoverStub()
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)

	require.NoError(t, err)
	require.Contains(t, stub.listedProjects["/rest-apis"], cloudbase.ProjectTwo, "the project the scenarios deploy through is read for leftovers")
	require.Contains(t, removed, "/rest-apis/syn-api-abcde234-9", "the run's API in that project is removed")
	require.NotContains(t, removed, "/rest-apis/ops-api", "an API the organisation owns stays")
	require.NotContains(t, removed, "/rest-apis/ops-project-api", "an API the organisation owns in the second project stays too")
}

func TestCloudSweepsListAProjectOnceWhenTwoRulesSelectIt(t *testing.T) {
	stub := newLeftoverStub()
	stub.lists["/projects"] = []map[string]any{
		{"id": cloudbase.ProjectOne, "name": "syn-project-abcde234-8"}, {"id": "syn-project-abcde234-2"}, {"id": cloudbase.ProjectTwo},
	}
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	_, err = sweepLeftovers(context.Background(), base, 5*time.Second)

	require.NoError(t, err)
	listings := func(id string) int {
		count := 0
		for _, project := range stub.listedProjects["/rest-apis"] {
			if project == id {
				count++
			}
		}
		return count
	}
	require.Positive(t, listings(cloudbase.ProjectOne))
	require.Equal(t, listings(cloudbase.ProjectTwo), listings(cloudbase.ProjectOne),
		"a project that two rules select is listed as often as one that a single rule selects")
}

func TestCloudSweepsJudgeAProjectScopedItemByItsIDAlone(t *testing.T) {
	stub := newLeftoverStub()
	stub.lists["/graphql-apis?projectId="+cloudbase.ProjectOne] = []map[string]any{
		{"id": "syn-graphql-abcde234-1"}, {"id": "ops-graphql", "displayName": "Synthetic syn-graphql-abcde234-9"},
	}
	stub.lists["/rest-apis?projectId="+cloudbase.ProjectOne] = append(stub.lists["/rest-apis?projectId="+cloudbase.ProjectOne],
		map[string]any{"id": "ops-named-api", "displayName": "Synthetic syn-api-abcde234-7"})
	topo := cloudbase.CloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	_, err = sweepLeftovers(context.Background(), base, 5*time.Second)
	require.NoError(t, err)
	require.Contains(t, stub.deleted, "graphql-apis/syn-graphql-abcde234-1")
	require.NotContains(t, stub.deleted, "graphql-apis/ops-graphql", "a display name with a run's suffix does not make the run its owner")
	require.NotContains(t, stub.deleted, "rest-apis/ops-named-api")
}

func TestStarterRunsWithoutAPortalAdministratorAndSaysWhy(t *testing.T) {
	const identityTokenEnv, identityURLEnv = "CLOUD_IAM_ACCESS_TOKEN", "CLOUD_IAM_API_URL"
	for name, setup := range map[string]func(t *testing.T){
		"no Identity Platform token": func(t *testing.T) {
			t.Setenv(identityTokenEnv, "")
		},
		"the portal reports no URL": func(t *testing.T) {
			t.Setenv(identityTokenEnv, "identity-token")
			t.Setenv(identityURLEnv, "https://identity.example.test/iam/v1")
		},
	} {
		t.Run(name, func(t *testing.T) {
			setup(t)
			platform := &starterPlatform{}
			starter := starterAgainst(t, platform)

			require.NoError(t, starter.checkScenario(context.Background(), "a publishing scenario", []precondition{preAPIPortal}),
				"the scenario runs")
			require.NoError(t, starter.Failures(), "the run does not fail for it")
			require.Contains(t, strings.Join(starter.Report(), "\n"), "the portal administrator was not made")

			swept, err := starter.SweepPortal(context.Background(), nil)
			require.NoError(t, err)
			require.Empty(t, swept)
		})
	}
}
