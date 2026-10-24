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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/catalog/testbench"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestBearerTokenReadsAndCachesEnvironmentToken(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "access-token")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	token, err := bearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)

	t.Setenv(cloudConsoleAccessTokenEnv, "a-different-token")
	token, err = bearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)
}

func TestBearerTokenRequiresEnvironmentToken(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	_, err := bearerToken(ctx)
	require.ErrorContains(t, err, cloudConsoleAccessTokenEnv)
}

func TestCloudAPIURLUsesResolvedEndpointAsBase(t *testing.T) {
	require.Equal(t,
		"https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/projects",
		cloudAPIURL("https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/", "/projects"))
}

func TestCloudResourceURLEscapesID(t *testing.T) {
	require.Equal(t,
		"https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/projects/id%2Fwith%20separator",
		cloudResourceURL("https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9", "/projects", "id/with separator"))
}

func TestResolveCloudDataPlaneURLRequiresHTTPSAndApproval(t *testing.T) {
	for _, target := range []string{
		"", "relative/path", "http://gateway.example.com/api", "ftp://gateway.example.com/api",
		"//gateway.example.com/api", "https://127.0.0.1/api", "https://10.0.0.1/api",
		"https://example.com/api", "https://synthetic-wso2cloud.gateway.development.example:8443/api",
		"https://user:password@synthetic-wso2cloud.gateway.development.example/api",
	} {
		_, err := resolveCloudDataPlaneURL(context.Background(), target)
		require.Error(t, err, target)
	}
	_, err := resolveCloudDataPlaneURL(context.Background(), "https://synthetic-wso2cloud.gateway.wso2.com/api")
	require.NoError(t, err)
}

func TestFirstApprovedDataPlaneEndpointSkipsInvalidCandidates(t *testing.T) {
	steps := &Steps{dataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		if target == "https://unapproved.example.com" {
			return "", errors.New("unapproved")
		}
		return target, nil
	}}

	endpoint, ok := firstApprovedDataPlaneEndpoint(context.Background(), steps, []any{
		"https://unapproved.example.com", " https://approved.example.com/// ", 42,
	})
	require.True(t, ok)
	require.Equal(t, "https://approved.example.com", endpoint)
}

func TestExpandCloudValueResolvesContextValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set("gatewayEndpoint", "https://gateway.example.com")

	got, err := expandCloudValue(ctx, "${CTX:gatewayEndpoint}/api/v1/posts/1")
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.com/api/v1/posts/1", got)
}

func TestSendUntilStatusPollsUntilExpectedStatus(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	methods := make([]string, 0, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		methods = append(methods, r.Method)
		mu.Unlock()
		if attempt == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{
		Timeout:            5 * time.Second,
		InsecureSkipVerify: true,
	}), 0, 0), dataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		return target, nil
	}}

	require.NoError(t, steps.sendUntilStatus(ctx, http.MethodGet, server.URL, http.StatusOK))
	mu.Lock()
	gotCalls := calls
	gotMethods := append([]string(nil), methods...)
	mu.Unlock()
	require.Equal(t, 2, gotCalls)
	require.Equal(t, []string{http.MethodGet, http.MethodGet}, gotMethods)
	response, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, `{"id":1}`, response.Text())
}

func TestSendUntilStatusRejectsUnapprovedDestination(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()

	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)}

	err := steps.sendUntilStatus(ctx, http.MethodGet, server.URL, http.StatusOK)
	require.ErrorContains(t, err, "approved HTTPS")
	require.Zero(t, calls)
}

func TestListItems(t *testing.T) {
	items, err := listItems(&httpx.Response{Body: []byte(`{"list":[{"id":"default"}]}`)})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "default", items[0]["id"])

	_, err = listItems(&httpx.Response{Body: []byte("invalid")})
	require.Error(t, err)
}

func TestStringField(t *testing.T) {
	resp := &httpx.Response{Body: []byte(`{"id":"resource"}`)}
	got, err := stringField(resp, "id")
	require.NoError(t, err)
	require.Equal(t, "resource", got)

	_, err = stringField(resp, "missing")
	require.ErrorContains(t, err, "missing or empty")
	_, err = stringField(&httpx.Response{Body: []byte("invalid")}, "id")
	require.Error(t, err)
}

func TestResolveDataPlaneURLTreatsALeadingSlashAsAGatewayPath(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	steps := &Steps{}

	_, err := steps.resolveDataPlaneURL(ctx, "/ctx/health")
	require.ErrorContains(t, err, keyGatewayEndpoint)

	local.Set(keyGatewayEndpoint, "https://dev-wso2cloud.gateway.dev.cloud.wso2.com")
	got, err := steps.resolveDataPlaneURL(ctx, "/ctx/health")
	require.NoError(t, err)
	require.Equal(t, "https://dev-wso2cloud.gateway.dev.cloud.wso2.com/ctx/health", got)

	_, err = steps.resolveDataPlaneURL(ctx, "//evil.example.com/ctx")
	require.Error(t, err, "a scheme-relative target is not a gateway path")
	_, err = steps.resolveDataPlaneURL(ctx, "ctx/health")
	require.Error(t, err, "a relative path without a leading slash is not accepted")

	local.Set(keyGatewayEndpoint, "https://unapproved.example.com")
	_, err = steps.resolveDataPlaneURL(ctx, "/ctx/health")
	require.ErrorContains(t, err, "unapproved destination", "the gateway endpoint still passes the host check")
}

func TestITGatewayReadySkipsWithoutATestbench(t *testing.T) {
	t.Setenv(testbench.EnvURL, "")
	t.Setenv(testbench.EnvEndpointURLPrefix+"BACKEND", "")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	err := (&Steps{}).itGatewayReady(ctx)
	require.ErrorIs(t, err, godog.ErrSkip)
	require.ErrorContains(t, err, testbench.EnvURL)

	t.Setenv(testbench.EnvURL, "not a url")
	err = (&Steps{}).itGatewayReady(ctx)
	require.Error(t, err)
	require.NotErrorIs(t, err, godog.ErrSkip, "a malformed URL is a failure, not a skip")
}

func TestITHandleNormalisesGeneratedValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set("projectHandle", "Papi_Deploy_ABC")

	got, err := itHandle(ctx, "${CTX:projectHandle}")
	require.NoError(t, err)
	require.Equal(t, "papi-deploy-abc", got)

	_, err = itHandle(ctx, "  ")
	require.Error(t, err)
	_, err = itHandle(ctx, "${CTX:missing}")
	require.Error(t, err)
}

// platformAPIStub serves the cloud API and records the project and API creation bodies.
type platformAPIStub struct {
	mu       sync.Mutex
	requests map[string]map[string]any
}

func newPlatformAPIStub(t *testing.T) (*platformAPIStub, *frameworkruntime.Topology) {
	t.Helper()
	stub := &platformAPIStub{requests: map[string]map[string]any{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		stub.mu.Lock()
		stub.requests[r.Method+" "+r.URL.Path] = body
		stub.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer console-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/projects" {
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{{"id": "default"}}})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": body["id"]})
	}))
	t.Cleanup(server.Close)
	inst, err := components.NewExternalInstance(corecatalog.CloudConsole(), 0, 1,
		map[string]string{corecatalog.EndpointAPIPPlatformAPI: server.URL})
	require.NoError(t, err)
	set := components.NewSet()
	require.NoError(t, set.Add(inst))
	return stub, &frameworkruntime.Topology{Instances: set}
}

func (b *platformAPIStub) body(key string) map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.requests[key]
}

func TestITCreateRESTAPIUsesTheDefaultProjectAndTheTestbenchUpstream(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(testbench.EnvURL, "https://hosted.example.com/inbox/")
	t.Setenv(testbench.EnvEndpointURLPrefix+"BACKEND", "")
	stub, topo := newPlatformAPIStub(t)
	registry := cleanup.NewRegistry(slog.Default())
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, cleanup.Install(ctx, registry))
	local, _ := tcontext.LocalOf(ctx)
	local.Set("projectHandle", "Papi_Project")
	local.Set("apiHandle", "papi-deploy-1")
	local.Set("apiContext", "/papi-deploy-1")
	steps := &Steps{topo: topo, client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}

	require.NoError(t, steps.itCreateProject(ctx, "${CTX:projectHandle}"))
	require.Nil(t, stub.body("POST /projects"), "the organization's project quota means no project is created")
	selected, err := requiredValue(ctx, keyProjectID)
	require.NoError(t, err)
	require.Equal(t, "default", selected)

	require.NoError(t, steps.itCreateRESTAPI(ctx, "${CTX:apiHandle}", "${CTX:projectHandle}", "${CTX:apiContext}"))
	id, err := requiredValue(ctx, keyAPIID)
	require.NoError(t, err)
	require.Equal(t, "papi-deploy-1", id)

	api := stub.body("POST /rest-apis")
	require.Equal(t, "papi-deploy-1", api["id"])
	require.Equal(t, "default", api["projectId"])
	require.Equal(t, "/papi-deploy-1", api["context"])
	require.Equal(t, map[string]any{"main": map[string]any{"url": "https://hosted.example.com/inbox"}}, api["upstream"])
	operation := api["operations"].([]any)[0].(map[string]any)["request"].(map[string]any)
	require.Equal(t, map[string]any{"method": "GET", "path": "/health"}, operation)

	var registered []string
	for _, resource := range registry.Pending() {
		registered = append(registered, resource.Kind.Name+"/"+resource.ID)
	}
	require.Equal(t, []string{cleanup.KindAPI.Name + "/papi-deploy-1"}, registered, "only the API is created")
}

func TestITCreateRESTAPIRequiresTheSelectedProject(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(testbench.EnvURL, "https://hosted.example.com")
	_, topo := newPlatformAPIStub(t)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, cleanup.Install(ctx, cleanup.NewRegistry(slog.Default())))
	steps := &Steps{topo: topo, client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}

	err := steps.itCreateRESTAPI(ctx, "api-1", "never-selected", "/api-1")
	require.Error(t, err, "no project was selected")

	require.NoError(t, steps.itCreateProject(ctx, "project-a"))
	err = steps.itCreateRESTAPI(ctx, "api-1", "project-b", "/api-1")
	require.ErrorContains(t, err, `project "project-b" but selected project "project-a"`)
}

func TestITDeployRESTAPIRequiresTheAPICreatedInTheScenario(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	steps := &Steps{}

	require.Error(t, steps.itDeployRESTAPI(ctx, "papi-deploy-1"), "no API was created")

	local.Set(keyAPIID, "papi-other")
	require.ErrorContains(t, steps.itDeployRESTAPI(ctx, "papi-deploy-1"), "created API")
}

func TestSendOnceAndStatusAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/missing/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0),
		dataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}

	require.NoError(t, steps.sendOnce(ctx, "get", server.URL+"/missing/health"))
	require.NoError(t, steps.assertResponseStatus(ctx, http.StatusNotFound))
	require.ErrorContains(t, steps.assertResponseStatus(ctx, http.StatusOK), "expected status 200")

	require.Error(t, steps.sendOnce(ctx, " ", server.URL))
	require.Error(t, (&Steps{}).sendOnce(ctx, "GET", server.URL), "a funnel is required")
}

// runITFeature runs one IT scenario through the cloud bindings with no testbench configured, so
// the first step skips the scenario. Steps after a skip are still matched against the bindings,
// which makes an undefined or ambiguous phrase a failure under Strict.
func runITFeature(t *testing.T, feature string) int {
	t.Helper()
	t.Setenv(testbench.EnvURL, "")
	t.Setenv(testbench.EnvEndpointURLPrefix+"BACKEND", "")
	return godog.TestSuite{
		Name: "it-binding-check",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			Register(sc, nil, httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0))
			sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				return tcontext.WithLocal(ctx, tcontext.NewLocal("cloud-runner")), nil
			})
		},
		Options: &godog.Options{
			Format: "pretty", Output: io.Discard, Paths: []string{feature}, Strict: true, NoColors: true,
		},
	}.Run()
}

// Every phrase of the integration-suite scenario the cloud suite runs must be bound once, and only
// once, in the cloud binary.
func TestITScenarioPhrasesAreAllBoundOnce(t *testing.T) {
	const file = "../../../it/features/management-portal/deployments/api_deployment.feature"
	require.Zero(t, runITFeature(t, file+":34"))
	// The undeploy scenario uses phrases the cloud suite does not bind yet, which shows the
	// check above is able to fail.
	require.NotZero(t, runITFeature(t, file+":44"))
}
