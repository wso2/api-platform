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

package cloudbase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestBearerTokenReadsAndCachesEnvironmentToken(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "access-token")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	token, err := BearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)

	t.Setenv(CloudConsoleAccessTokenEnv, "a-different-token")
	token, err = BearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)
}

func TestBearerTokenRequiresEnvironmentToken(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	_, err := BearerToken(ctx)
	require.ErrorContains(t, err, CloudConsoleAccessTokenEnv)
}

func TestCloudAPIURLUsesResolvedEndpointAsBase(t *testing.T) {
	require.Equal(t,
		"https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/projects",
		CloudAPIURL("https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/", "/projects"))
}

func TestCloudResourceURLEscapesID(t *testing.T) {
	require.Equal(t,
		"https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9/projects/id%2Fwith%20separator",
		cloudResourceURL("https://cloud.example.com/apip-platform-api-apip-platform-api-endpoint/api/v0.9", "/projects", "id/with separator"))
}

func TestResolveCloudDataPlaneURLRequiresHTTPSAndApproval(t *testing.T) {
	t.Setenv(cloudDataPlaneDomainsEnv, "")
	for _, target := range []string{
		"", "relative/path", "http://gateway.example.com/api", "ftp://gateway.example.com/api",
		"//gateway.example.com/api", "https://127.0.0.1/api", "https://10.0.0.1/api",
		"https://example.com/api", "https://synthetic-wso2cloud.gateway.development.example:8443/api",
		"https://user:password@synthetic-wso2cloud.gateway.development.example/api",
	} {
		_, err := ResolveCloudDataPlaneURL(context.Background(), target)
		require.Error(t, err, target)
	}
	_, err := ResolveCloudDataPlaneURL(context.Background(), "https://synthetic-wso2cloud.gateway.wso2.com/api")
	require.NoError(t, err)
}

func TestResolveCloudDataPlaneURLApprovesConfiguredDomains(t *testing.T) {
	const wso2Gateway = "https://abc-gw-p1.gateway.dp.dev.cloud.wso2.com/api"
	const stageGateway = "https://abc-gw-p1.dp1.us.stage.example/api"
	t.Run("unset approves gateway hosts under wso2.com only", func(t *testing.T) {
		t.Setenv(cloudDataPlaneDomainsEnv, "")
		_, err := ResolveCloudDataPlaneURL(context.Background(), wso2Gateway)
		require.NoError(t, err)
		_, err = ResolveCloudDataPlaneURL(context.Background(), stageGateway)
		require.Error(t, err)
	})
	t.Run("blank keeps the default", func(t *testing.T) {
		t.Setenv(cloudDataPlaneDomainsEnv, "  ")
		_, err := ResolveCloudDataPlaneURL(context.Background(), wso2Gateway)
		require.NoError(t, err)
		_, err = ResolveCloudDataPlaneURL(context.Background(), stageGateway)
		require.Error(t, err)
	})
	t.Run("a configured domain approves one label in front of it", func(t *testing.T) {
		t.Setenv(cloudDataPlaneDomainsEnv, "dp1.us.stage.example")
		got, err := ResolveCloudDataPlaneURL(context.Background(), stageGateway)
		require.NoError(t, err)
		require.Equal(t, stageGateway, got)
		_, err = ResolveCloudDataPlaneURL(context.Background(), wso2Gateway)
		require.Error(t, err, "a configured domain replaces the default")
	})
	t.Run("a configured domain approves nothing else", func(t *testing.T) {
		t.Setenv(cloudDataPlaneDomainsEnv, "dp1.us.stage.example")
		for _, target := range []string{
			"https://dp1.us.stage.example/api",
			"https://a.abc-gw-p1.dp1.us.stage.example/api",
			"https://abc.notdp1.us.stage.example/api",
			"https://abc.dp1.us.stage.example.evil.test/api",
			"https://abc.dp1.us.stage.example:8443/api",
			"http://abc.dp1.us.stage.example/api",
		} {
			_, err := ResolveCloudDataPlaneURL(context.Background(), target)
			require.Error(t, err, target)
		}
	})
	t.Run("several domains, case and spaces normalised", func(t *testing.T) {
		t.Setenv(cloudDataPlaneDomainsEnv, " Gateway.DP.dev.cloud.wso2.com , dp1.us.stage.example,")
		for _, target := range []string{wso2Gateway, stageGateway} {
			_, err := ResolveCloudDataPlaneURL(context.Background(), target)
			require.NoError(t, err, target)
		}
	})
	t.Run("an invalid domain fails closed", func(t *testing.T) {
		for _, value := range []string{"example", "*.stage.example", ".stage.example", "stage..example", "stage example", "https://stage.example", ","} {
			t.Setenv(cloudDataPlaneDomainsEnv, value)
			_, err := ResolveCloudDataPlaneURL(context.Background(), stageGateway)
			require.ErrorIs(t, err, ErrCloudDataPlaneDomains, value)
			require.Contains(t, err.Error(), cloudDataPlaneDomainsEnv, value)
		}
	})
}

func TestFirstApprovedDataPlaneEndpointSkipsInvalidCandidates(t *testing.T) {
	steps := &Steps{DataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		if target == "https://unapproved.example.com" {
			return "", errors.New("unapproved")
		}
		return target, nil
	}}

	endpoint, ok, err := FirstApprovedDataPlaneEndpoint(context.Background(), steps, []any{
		"https://unapproved.example.com", " https://approved.example.com/// ", 42,
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "https://approved.example.com", endpoint)
}

func TestFirstApprovedDataPlaneEndpointReturnsAnInvalidDomainsSetting(t *testing.T) {
	t.Setenv(cloudDataPlaneDomainsEnv, "*.stage.example")
	_, ok, err := FirstApprovedDataPlaneEndpoint(context.Background(), &Steps{}, []any{
		"https://abc-gw-p1.dp1.us.stage.example",
	})
	require.False(t, ok)
	require.ErrorIs(t, err, ErrCloudDataPlaneDomains)
}

func TestExpandCloudValueResolvesContextValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set("gatewayEndpoint", "https://gateway.example.com")

	got, err := ExpandCloudValue(ctx, "${CTX:gatewayEndpoint}/api/v1/posts/1")
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
	steps := &Steps{Funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{
		Timeout:            5 * time.Second,
		InsecureSkipVerify: true,
	}), 0, 0), DataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		return target, nil
	}}

	require.NoError(t, steps.SendUntilStatus(ctx, http.MethodGet, server.URL, http.StatusOK))
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
	steps := &Steps{Funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)}

	err := steps.SendUntilStatus(ctx, http.MethodGet, server.URL, http.StatusOK)
	require.ErrorContains(t, err, "approved HTTPS")
	require.Zero(t, calls)
}

func TestListItems(t *testing.T) {
	items, err := ListItems(&httpx.Response{Body: []byte(`{"list":[{"id":"default"}]}`)})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "default", items[0]["id"])

	_, err = ListItems(&httpx.Response{Body: []byte("invalid")})
	require.Error(t, err)
}

func TestStringField(t *testing.T) {
	resp := &httpx.Response{Body: []byte(`{"id":"resource"}`)}
	got, err := StringField(resp, "id")
	require.NoError(t, err)
	require.Equal(t, "resource", got)

	_, err = StringField(resp, "missing")
	require.ErrorContains(t, err, "missing or empty")
	_, err = StringField(&httpx.Response{Body: []byte("invalid")}, "id")
	require.Error(t, err)
}

func TestTableValuesReadsKeyValueRows(t *testing.T) {
	values, err := TableValues(ValuesTable(t, []string{"version", "2.0.0"}, []string{" context ", "/x"}))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"version": "2.0.0", "context": "/x"}, values)

	values, err = TableValues(nil)
	require.NoError(t, err)
	require.Empty(t, values)
}

func TestTableValuesRejectsMalformedTables(t *testing.T) {
	for name, table := range map[string]*godog.Table{
		"one cell":      ValuesTable(t, []string{"version"}),
		"three cells":   ValuesTable(t, []string{"a", "b", "c"}),
		"empty key":     ValuesTable(t, []string{" ", "value"}),
		"duplicate key": ValuesTable(t, []string{"version", "1"}, []string{"version", "2"}),
	} {
		_, err := TableValues(table)
		require.Error(t, err, name)
	}
}

func TestNewAPIDefinitionDefaults(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	t.Setenv(BackendEnv, "")
	definition, err := newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"main": map[string]string{"url": DefaultBackend}}, definition["upstream"])
	t.Setenv(BackendEnv, "https://backend.example.com/mock/")
	overridden, err := newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"main": map[string]string{"url": "https://backend.example.com/mock"}}, overridden["upstream"])
	t.Setenv(BackendEnv, "http://backend.example.com")
	_, err = newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.ErrorContains(t, err, BackendEnv)
	t.Setenv(BackendEnv, "")
	require.Equal(t, "syn-api-1", definition["id"])
	require.Equal(t, "Synthetic syn-api-1", definition["displayName"])
	require.Equal(t, "/syn-api-1/v1", definition["context"])
	require.Equal(t, "1.0.0", definition["version"])
	require.Equal(t, "default", definition["projectId"])
	require.Equal(t, map[string]bool{"enabled": false}, definition["security"])
	operations, ok := definition["operations"].([]any)
	require.True(t, ok)
	require.Len(t, operations, 1)
}

func TestApplyAPIOverridesReplacesStringsAndJSON(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set("apiID", "syn-api-2")

	definition := map[string]any{"context": "/old", "version": "1.0.0"}
	err := applyAPIOverrides(ctx, definition, map[string]string{
		"context":  "/${CTX:apiID}/2.0.0",
		"version":  "2.0.0",
		"policies": `[{"name":"api-key-auth","version":"v1"}]`,
	})
	require.NoError(t, err)
	require.Equal(t, "/syn-api-2/2.0.0", definition["context"])
	require.Equal(t, "2.0.0", definition["version"])
	policies, ok := definition["policies"].([]any)
	require.True(t, ok)
	require.Len(t, policies, 1)
}

func TestApplyAPIOverridesRejectsUnsupportedAndMalformedValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	err := applyAPIOverrides(ctx, map[string]any{}, map[string]string{"projectId": "other"})
	require.ErrorContains(t, err, `unsupported REST API value "projectId"`)

	err = applyAPIOverrides(ctx, map[string]any{}, map[string]string{"operations": "not json"})
	require.ErrorContains(t, err, "not valid JSON")

	err = applyAPIOverrides(ctx, map[string]any{}, map[string]string{"context": "${CTX:missing}"})
	require.Error(t, err)
}

func TestAPIRefForSelectsPrimaryOrNewVersion(t *testing.T) {
	require.Equal(t, PrimaryAPI, APIRefFor("synthetic REST API"))
	require.Equal(t, VersionAPI, APIRefFor(newVersionAPILabel))
	require.NotEqual(t, PrimaryAPI.ID, VersionAPI.ID)
	require.NotEqual(t, PrimaryAPI.deployment, VersionAPI.deployment)
}

func TestSendUntilStatusWithSendsHeaders(t *testing.T) {
	var gotKey string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		if gotKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{
		Timeout:            5 * time.Second,
		InsecureSkipVerify: true,
	}), 0, 0), DataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		return target, nil
	}}

	require.NoError(t, steps.sendUntilStatusWith(ctx, http.MethodGet, server.URL, http.StatusOK,
		map[string]string{"X-API-Key": "secret"}))
	require.Equal(t, "secret", gotKey)
}

func TestSendWithCredentialRequiresAStoredCredentialAndAHeader(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{}

	for credential, key := range credentialKeys {
		err := steps.SendWithCredentialUntilStatus(ctx, http.MethodGet, "https://x.gateway.wso2.com", "X-API-Key", credential, http.StatusOK)
		require.ErrorContains(t, err, key, credential)
	}
	err := steps.SendWithCredentialUntilStatus(ctx, http.MethodGet, "https://x.gateway.wso2.com", "X-API-Key", "password", http.StatusOK)
	require.ErrorContains(t, err, `unsupported credential "password"`)

	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(KeyAPIKey, "secret")
	err = steps.SendWithCredentialUntilStatus(ctx, http.MethodGet, "https://x.gateway.wso2.com", " ", "API key", http.StatusOK)
	require.ErrorContains(t, err, "header name is required")
}

func TestAssertCloudResponseStatus(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)
	steps := &Steps{Funnel: funnel}

	require.Error(t, steps.AssertCloudResponseStatus(ctx, http.StatusConflict))

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusConflict}))
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusConflict))
	require.ErrorContains(t, steps.AssertCloudResponseStatus(ctx, http.StatusBadRequest), "expected response status 400")
}

func TestAssertAPIResponseMatchesRequestedPost(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)
	steps := &Steps{Funnel: funnel}

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"method":"GET","path":"/posts/2"}`)}))
	require.NoError(t, steps.AssertAPIResponse(ctx, 2))
	require.ErrorContains(t, steps.AssertAPIResponse(ctx, 1), "contain post 1")
}

func TestFindActiveGatewayRecordsItsEnvironment(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	topo := CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
			{"id": "inactive", "isActive": false},
			{"id": "gw-1", "isActive": true, "environment": " development ", "endpoints": []string{"https://gw.example.com/"}},
		}})
	})
	steps := &Steps{Topo: topo, Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}),
		DataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	require.NoError(t, steps.FindActiveGateway(ctx))
	for key, want := range map[string]string{
		KeyGatewayID: "gw-1", KeyGatewayEnv: "development", KeyGatewayEndpoint: "https://gw.example.com",
	} {
		got, err := RequiredValue(ctx, key)
		require.NoError(t, err, key)
		require.Equal(t, want, got, key)
	}
}

func TestFindActiveGatewayDropsTheEnvironmentOfAnEarlierChoice(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	topo := CloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
			{"id": "gw-2", "isActive": true, "endpoints": []string{"https://gw2.example.com"}},
		}})
	})
	steps := &Steps{Topo: topo, Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}),
		DataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set(KeyGatewayID, "gw-1")
	local.Set(KeyGatewayEnv, "development")

	require.NoError(t, steps.FindActiveGateway(ctx))
	id, err := RequiredValue(ctx, KeyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "gw-2", id)
	require.Empty(t, OptionalValue(ctx, KeyGatewayEnv), "the previous gateway's environment must not be paired with the new gateway")
}

func TestRefusedCreateIsAClientErrorOnly(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusBadRequest: true, http.StatusConflict: true, http.StatusUnprocessableEntity: true,
		http.StatusOK: false, http.StatusAccepted: false, http.StatusInternalServerError: false, http.StatusBadGateway: false,
	} {
		require.Equal(t, want, RefusedCreate(&httpx.Response{StatusCode: status}), status)
	}
}

func TestUndeployAPIUsesTheProjectAndEnvironmentRoute(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	var method, path, environment, gateway string
	status := http.StatusNoContent
	topo := CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		environment, gateway = r.URL.Query().Get("environment"), r.URL.Query().Get("gatewayId")
		w.WriteHeader(status)
	})
	steps := &Steps{Topo: topo, Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set(KeyAPIID, "syn-api-1")
	local.Set(KeyDeploymentID, "dep 1")
	local.Set(KeyGatewayID, "gw-1")
	local.Set(KeyProjectID, "default")

	err := steps.UndeployAPI(ctx, "synthetic REST API")
	require.ErrorContains(t, err, "environment", "the environment is required by the route")
	local.Set(KeyGatewayEnv, "development")

	require.NoError(t, steps.UndeployAPI(ctx, "synthetic REST API"))
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/projects/default/apis/syn-api-1/deployments/dep 1/undeploy", path)
	require.Equal(t, "development", environment)
	require.Equal(t, "gw-1", gateway)

	for _, rejected := range []int{http.StatusOK, http.StatusNotFound} {
		status = rejected
		require.Error(t, steps.UndeployAPI(ctx, "synthetic REST API"), "only 204 is the documented success")
	}
}

func TestSameJSONIgnoresFormattingAndFallsBackToBytes(t *testing.T) {
	require.True(t, SameJSON([]byte(`{"a":1,"b":[1,2]}`), []byte("{ \"b\": [1, 2], \"a\": 1 }")))
	require.False(t, SameJSON([]byte(`{"a":1}`), []byte(`{"a":2}`)))
	require.True(t, SameJSON([]byte("plain"), []byte("plain")))
	require.False(t, SameJSON([]byte("plain"), []byte("other")))
}

// projectDeployStub answers a project-scoped deployment request the way the platform does: one
// deployment per gateway the body names, listed in a "list" envelope.
func projectDeployStub(t *testing.T, path string, requests *[]map[string]any, status *int) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, path, r.URL.Path)
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		*requests = append(*requests, request)
		var list []map[string]string
		for _, gateway := range request["gateways"].([]any) {
			list = append(list, map[string]string{"gatewayId": gateway.(map[string]any)["gatewayId"].(string), "deploymentId": "d"})
		}
		WriteJSON(w, *status, map[string]any{"list": list})
	}
}

func TestDeployAPIDeploysThroughTheProjectAndKeepsTheDeploymentID(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	var requests []map[string]any
	status := http.StatusCreated
	steps, ctx, local, _ := CloudSteps(t, projectDeployStub(t, "/projects/default/apis/syn-api-1/deployments", &requests, &status))
	local.Set(KeyAPIID, "syn-api-1")
	local.Set(KeyGatewayID, "dev-default")

	require.ErrorContains(t, steps.DeployAPI(ctx, "synthetic REST API"), "does not report its environment")
	local.Set(KeyGatewayEnv, "development")
	require.Error(t, steps.DeployAPI(ctx, "synthetic REST API"), "the project is needed to deploy")
	local.Set(KeyProjectID, "default")

	require.NoError(t, steps.DeployAPI(ctx, "synthetic REST API"))
	require.Equal(t, "d", OptionalValue(ctx, KeyDeploymentID))
	require.Len(t, requests, 1)
	require.Equal(t, "development", requests[0]["environment"])
	require.Equal(t, []any{map[string]any{"gatewayId": "dev-default"}}, requests[0]["gateways"])

	require.NoError(t, steps.RedeployAPI(ctx), "a redeployment is another deployment of the current definition")
	require.Len(t, requests, 2)

	status = http.StatusNotFound
	require.ErrorContains(t, steps.RedeployAPI(ctx), "deploying APIP API syn-api-1")
}

func TestParseConfigCellReadsJSONLiteralsAndOtherwiseText(t *testing.T) {
	require.Equal(t, float64(5000), ParseConfigCell("5000"))
	require.Equal(t, true, ParseConfigCell(" true "))
	require.Equal(t, nil, ParseConfigCell("null"))
	require.Equal(t, "5", ParseConfigCell(`"5"`), "a quoted value is text")
	require.Equal(t, "a\u2028b", ParseConfigCell(`"a\u2028b"`))
	require.Equal(t, map[string]any{"a": float64(1)}, ParseConfigCell(`{"a": 1}`))
	require.Equal(t, "debug", ParseConfigCell("debug"))
	require.Equal(t, "1 minute", ParseConfigCell("1 minute"))
	require.Equal(t, "[x]\nlevel = 1", ParseConfigCell("[x]\nlevel = 1"))
}

func TestListCollectionKeepsThePagesItReadWhenALaterPageFails(t *testing.T) {
	topo := CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset >= 2 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}, {"id": "b"}},
			"pagination": map[string]any{"total": 6, "offset": offset, "limit": 2}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.Error(t, err)
	require.Len(t, items, 2, "the items of the pages already read are returned with the error")
}

func TestListCollectionKeepsTheItemsWhenThePageCapIsReached(t *testing.T) {
	topo := CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "item-" + strconv.Itoa(offset)}},
			"pagination": map[string]any{"total": 100000, "offset": offset, "limit": 1}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.ErrorContains(t, err, "the rest were not read")
	require.Len(t, items, maxListPages)
}

func TestListCollectionReadsEveryAdvertisedPage(t *testing.T) {
	ids := []string{"p1", "p2", "p3", "p4", "p5"}
	var queries []string
	topo := CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := min(offset+2, len(ids))
		list := make([]map[string]any, 0, 2)
		for _, id := range ids[offset:end] {
			list = append(list, map[string]any{"id": id})
		}
		WriteJSON(w, http.StatusOK, map[string]any{"count": len(list), "list": list,
			"pagination": map[string]any{"total": len(ids), "offset": offset, "limit": 2}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.NoError(t, err)
	require.Len(t, items, 5)
	require.Equal(t, "p5", items[4]["id"])
	require.Equal(t, []string{"", "limit=2&offset=2", "limit=2&offset=4"}, queries, "paging parameters are sent only once the platform advertises more pages")
}

func TestListCollectionReadsOnePageWhenThePlatformDoesNotPage(t *testing.T) {
	requests := 0
	topo := CloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}, {"id": "b"}}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/pipelines")
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, 1, requests)
}

func TestListCollectionRefusesAPlatformThatIgnoresTheOffset(t *testing.T) {
	topo := CloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}, {"id": "b"}},
			"pagination": map[string]any{"total": 10, "offset": 0, "limit": 2}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.ErrorContains(t, err, "did not honour offset")
	require.Len(t, items, 2, "the page from the wrong offset is dropped, so nothing is listed twice")
}

func TestListCollectionRetriesATransientFailure(t *testing.T) {
	for name, fail := range map[string]func(w http.ResponseWriter){
		"a 5xx answer": func(w http.ResponseWriter) { w.WriteHeader(http.StatusServiceUnavailable) },
		"a 429 answer": func(w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests) },
		"a dropped connection": func(w http.ResponseWriter) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			requests := 0
			topo := CloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				if requests == 1 {
					fail(w)
					return
				}
				WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}}})
			})
			base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
			require.NoError(t, err)

			items, err := ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/pipelines")
			require.NoError(t, err)
			require.Len(t, items, 1)
			require.Equal(t, 2, requests)
		})
	}
}

func TestListCollectionDoesNotRetryAClientError(t *testing.T) {
	requests := 0
	topo := CloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	_, err = ListCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/pipelines")
	require.Error(t, err)
	require.Equal(t, 1, requests)
}

// gatewayChoiceStub serves the managed-gateway list, a project bound to the "default" pipeline
// and that pipeline, whose only promotion path runs from development to staging.
func gatewayChoiceStub(t *testing.T, gateways []map[string]any, queries *[]string) (*Steps, context.Context) {
	t.Helper()
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(CloudGatewayPinEnv, "")
	steps, ctx, local, _ := CloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/managed-gateways":
			if queries != nil {
				*queries = append(*queries, r.URL.RawQuery)
			}
			WriteJSON(w, http.StatusOK, map[string]any{"list": gateways})
		case strings.HasSuffix(r.URL.Path, "/deployment-pipeline"):
			WriteJSON(w, http.StatusOK, map[string]string{"pipeline": "default"})
		case r.URL.Path == "/pipelines":
			WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{
				"name":           "default",
				"promotionPaths": []map[string]any{{"sourceEnvironment": "development", "targetEnvironments": []string{"staging"}}},
			}}})
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	steps.DataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(KeyProjectID, "default")
	return steps, ctx
}

func TestFindActiveGatewayChoosesARegularGatewayOfThePipelineThatNoRunCreated(t *testing.T) {
	var queries []string
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		ListedGateway("prod-default", "production", nil),
		ListedGateway("dev-ai", "development", map[string]any{"functionalityType": "ai"}),
		ListedGateway("dev-sgw-other", "development", map[string]any{"displayName": "sgw-abcdefg2-3"}),
		ListedGateway("dev-sgw-upper", "development", map[string]any{"name": "SGW-ABCDEFG2-4"}),
		ListedGateway("dev-default", "development", map[string]any{"functionalityType": "regular", "displayName": "default"}),
		ListedGateway("stage-default", "staging", nil),
	}, &queries)

	require.NoError(t, steps.FindActiveGateway(ctx))
	id, err := tcontext.ResolveString(ctx, KeyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-default", id)
	env, err := tcontext.ResolveString(ctx, KeyGatewayEnv)
	require.NoError(t, err)
	require.Equal(t, "development", env)
	require.Equal(t, []string{"functionalityType=regular"}, queries, "the platform is asked for regular gateways only")
}

func TestPipelineProjectIsTheFoundProjectElseTheOneOfTheFeatureFile(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.Empty(t, pipelineProjectOf(ctx))

	require.NoError(t, tcontext.Set(ctx, KeyScenarioProject, ProjectTwo))
	require.Equal(t, ProjectTwo, pipelineProjectOf(ctx), "a scenario that never found a project follows its feature file's project")

	require.NoError(t, tcontext.Set(ctx, KeyProjectID, "syn-project-abcde234-1"))
	require.Equal(t, "syn-project-abcde234-1", pipelineProjectOf(ctx), "a project the scenario found or created wins")
}

func TestUsableEnvironmentPrefersAStageTheProjectDeploysToDirectly(t *testing.T) {
	items := []map[string]any{{"name": "apip-test-e1"}, {"name": "development"}, {"name": "qa"}}
	fits := func(string) bool { return true }

	got, err := usableEnvironmentPreferring(items, map[string]bool{"development": true}, fits)
	require.NoError(t, err)
	require.Equal(t, "development", got, "the preferred environment wins over an earlier one")

	got, err = usableEnvironmentPreferring(items, map[string]bool{"development": true}, func(name string) bool { return name != "development" })
	require.NoError(t, err)
	require.Equal(t, "apip-test-e1", got, "a preferred environment that does not fit is passed over")

	got, err = usableEnvironmentPreferring(items, nil, fits)
	require.NoError(t, err)
	require.Equal(t, "apip-test-e1", got, "with no preference the first usable one is chosen")
}

func TestEntryEnvironmentsOfAPipelineAreTheStagesNoPathTargets(t *testing.T) {
	require.Equal(t, map[string]bool{"development": true}, EntryEnvironmentsOf(PromotionChainPaths("development", "apip-test-e1", "apip-test-e2")))
	require.Equal(t, map[string]bool{"alpha": true, "beta": true}, EntryEnvironmentsOf([]PromotionPath{
		{SourceEnvironment: "alpha", TargetEnvironments: []string{"prod"}},
		{SourceEnvironment: "beta", TargetEnvironments: []string{"prod"}},
	}))
	require.Empty(t, EntryEnvironmentsOf(nil))
}

func TestFindActiveGatewayPassesOverAGatewayOfAPromotionTarget(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		ListedGateway("stage-default", "staging", nil),
		ListedGateway("dev-default", "development", nil),
	}, nil)

	require.NoError(t, steps.FindActiveGateway(ctx))
	id, err := tcontext.ResolveString(ctx, KeyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-default", id, "the platform accepts a deployment to the entry environment only")
	env, err := tcontext.ResolveString(ctx, KeyGatewayEnv)
	require.NoError(t, err)
	require.Equal(t, "development", env)
}

func TestFindActiveGatewayNamesWhyEveryGatewayWasRejected(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		ListedGateway("prod-default", "production", nil),
		ListedGateway("stage-default", "staging", nil),
		ListedGateway("dev-ai", "development", map[string]any{"functionalityType": "ai"}),
		ListedGateway("dev-sgw", "development", map[string]any{"displayName": "sgw-abcdefg2-1"}),
		ListedGateway("", "development", nil),
	}, nil)

	err := steps.FindActiveGateway(ctx)
	require.ErrorContains(t, err, "no usable APIP gateway among 5 listed")
	require.ErrorContains(t, err, `prod-default: environment "production" is not in the project's deployment pipeline`)
	require.ErrorContains(t, err, `stage-default: environment "staging" is a promotion target of the project's deployment pipeline`)
	require.ErrorContains(t, err, `dev-ai: functionality type "ai"`)
	require.ErrorContains(t, err, "dev-sgw: a synthetic gateway of a suite run")
}

func TestFindActiveGatewayWithoutAProjectDoesNotFilterByPipeline(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{ListedGateway("prod-default", "production", nil)}, nil)
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(KeyProjectID, "")
	require.NoError(t, steps.FindActiveGateway(ctx), "no project, so no pipeline to read")
	id, err := tcontext.ResolveString(ctx, KeyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "prod-default", id)
}

func TestFindActiveGatewayHonoursAPinnedGateway(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		ListedGateway("dev-first", "development", nil),
		ListedGateway("dev-pinned", "development", nil),
		ListedGateway("dev-operator", "development", map[string]any{"displayName": "frontend-1"}),
	}, nil)

	t.Setenv(CloudGatewayPinEnv, " dev-pinned ")
	require.NoError(t, steps.FindActiveGateway(ctx))
	id, err := tcontext.ResolveString(ctx, KeyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-pinned", id)

	t.Setenv(CloudGatewayPinEnv, "dev-operator")
	require.NoError(t, steps.FindActiveGateway(ctx), "a gateway the operator names is taken even if its name looks like a suite run's")
	id, err = tcontext.ResolveString(ctx, KeyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-operator", id)

	t.Setenv(CloudGatewayPinEnv, "dev-missing")
	err = steps.FindActiveGateway(ctx)
	require.ErrorContains(t, err, "not the gateway named by "+CloudGatewayPinEnv)
}

func TestFindActiveGatewayRefusesAPinnedGatewayOfASuiteRun(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		ListedGateway("dev-scenario", "development", map[string]any{"displayName": "frontend-1", "description": ScenarioGatewayDescription}),
		ListedGateway("dev-operator", "development", map[string]any{"displayName": "frontend-2", "description": "the operator's"}),
	}, nil)

	t.Setenv(CloudGatewayPinEnv, "dev-scenario")
	require.ErrorContains(t, steps.FindActiveGateway(ctx), "dev-scenario: a scenario gateway of a suite run")

	t.Setenv(CloudGatewayPinEnv, "dev-operator")
	require.NoError(t, steps.FindActiveGateway(ctx), "an operator's gateway that looks like a suite run's is still taken")
}

func TestFindActiveGatewayFailsWhenThePipelineCannotBeRead(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(CloudGatewayPinEnv, "")
	steps, ctx, local, _ := CloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" {
			WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{ListedGateway("dev-default", "development", nil)}})
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	steps.DataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(KeyProjectID, "default")
	require.ErrorContains(t, steps.FindActiveGateway(ctx), "deployment pipeline")
}

func TestChooseActiveGatewayAppliesNoFilterToAnAIGateway(t *testing.T) {
	steps := &Steps{DataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	choice, err := steps.ChooseActiveGateway(ctx, []map[string]any{
		ListedGateway("sgw-like", "development", map[string]any{"displayName": "sgw-abcdefg2-1", "functionalityType": "ai"}),
	}, "APIP AI gateway")
	require.NoError(t, err)
	require.Equal(t, "sgw-like", choice.ID)
}

func TestExpectedGatewayID(t *testing.T) {
	require.Equal(t, "dev-my-gateway", ExpectedGatewayID("dev", " My Gateway ", ""))
	require.Equal(t, "dev-explicit", ExpectedGatewayID("dev", "My Gateway", " explicit "))
}

func TestEchoedHeaderIgnoresCaseAndJoinsRepeats(t *testing.T) {
	value, ok := EchoedHeader([]byte(`{"headers":{"X-Apip-Secret-Probe":"v1","Host":"h"}}`), "x-apip-secret-probe")
	require.True(t, ok)
	require.Equal(t, "v1", value)
	value, ok = EchoedHeader([]byte(`{"headers":{"X-A":["a","b"]}}`), "x-a")
	require.True(t, ok)
	require.Equal(t, "a,b", value)
	for _, body := range []string{`{"headers":{}}`, `not json`, `{"headers":{"X-A":1}}`} {
		_, ok = EchoedHeader([]byte(body), "x-a")
		require.False(t, ok, body)
	}
}

func TestPromotionChainFollowsThePipelineFromAnEntryEnvironment(t *testing.T) {
	all := func(string) bool { return true }
	linear := []PromotionPath{
		{SourceEnvironment: "dev", TargetEnvironments: []string{"staging"}},
		{SourceEnvironment: "staging", TargetEnvironments: []string{"prod"}},
	}
	require.Equal(t, []string{"dev", "staging"}, PromotionChain(linear, 2, all))
	require.Equal(t, []string{"dev", "staging", "prod"}, PromotionChain(linear, 3, all))
	require.Nil(t, PromotionChain(linear, 4, all), "the pipeline is shorter than the chain")
	require.Nil(t, PromotionChain(nil, 2, all))
	require.Equal(t, []string{"dev", "staging", "prod"}, PromotionChain([]PromotionPath{linear[1], linear[0]}, 3, all),
		"the chain starts where no path leads, whatever order the paths are listed in")
	require.Nil(t, PromotionChain(linear, 2, func(environment string) bool { return environment != "staging" }),
		"an environment that cannot be used breaks the chain")

	branching := []PromotionPath{
		{SourceEnvironment: "dev", TargetEnvironments: []string{"qa", "staging"}},
		{SourceEnvironment: "staging", TargetEnvironments: []string{"prod"}},
	}
	require.Equal(t, []string{"dev", "qa"}, PromotionChain(branching, 2, all))
	require.Equal(t, []string{"dev", "staging"}, PromotionChain(branching, 2, func(environment string) bool { return environment != "qa" }))
	require.Equal(t, []string{"dev", "staging", "prod"}, PromotionChain(branching, 3, all), "a branch that ends early is passed over")

	shortcut := []PromotionPath{
		{SourceEnvironment: "dev", TargetEnvironments: []string{"staging", "prod"}},
		{SourceEnvironment: "staging", TargetEnvironments: []string{"prod"}},
	}
	require.Equal(t, []string{"dev", "staging"}, PromotionChain(shortcut, 2, all))
	require.Nil(t, PromotionChain(shortcut, 3, all), "the first environment reaches the third directly, so no promotion skips one")

	cyclic := []PromotionPath{
		{SourceEnvironment: "a", TargetEnvironments: []string{"b"}},
		{SourceEnvironment: "b", TargetEnvironments: []string{"a"}},
	}
	require.Nil(t, PromotionChain(cyclic, 2, all), "a pipeline that every environment is promoted into has no entry environment")

	blank := []PromotionPath{
		{SourceEnvironment: " ", TargetEnvironments: []string{"x"}},
		{SourceEnvironment: "dev", TargetEnvironments: []string{" ", "qa"}},
	}
	require.Equal(t, []string{"dev", "qa"}, PromotionChain(blank, 2, all))
}

func TestFindTestProjectFindsTheProjectOfTheScenariosFeature(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	var seen []string
	steps, ctx, local, _ := CloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		WriteJSON(w, http.StatusOK, map[string]any{"id": "x"})
	})
	require.ErrorContains(t, steps.FindTestProject(ctx), "project is not known")

	for feature, want := range map[string]string{
		"features/apip_restapi.feature":                   ProjectOne,
		"/repo/suites/cloud/features/apip_deploy.feature": ProjectOne,
		"apip_gateway.feature":                            ProjectOne,
		"apip_gateway_config.feature":                     ProjectOne,
		"apip_secrets.feature":                            ProjectOne,
		"features/apip_auth.feature":                      ProjectTwo,
		"features/apip_self_hosted_gateway.feature":       ProjectTwo,
		"features/apip_promotion.feature":                 ProjectTwo,
		"features/apip_portal.feature":                    ProjectTwo,
		"features/apip_project.feature":                   ProjectTwo,
	} {
		require.Equal(t, want, ProjectOfFeature(feature), feature)
	}

	local.Set(KeyScenarioProject, ProjectOne)
	require.NoError(t, steps.FindTestProject(ctx))
	require.Equal(t, ProjectOne, OptionalValue(ctx, KeyProjectID))
	require.Equal(t, []string{"GET /projects/" + ProjectOne}, seen)
}

func TestResolveCloudDataPlaneURLApprovesLocalhostOnlyWhenEnabled(t *testing.T) {
	local := []string{
		"http://gw-development.openchoreoapis.localhost:19080/api",
		"https://gw-development.openchoreoapis.localhost:19443/api",
		"http://gw.localhost/api",
	}
	t.Run("off by default", func(t *testing.T) {
		for _, target := range local {
			_, err := ResolveCloudDataPlaneURL(context.Background(), target)
			require.Error(t, err, target)
		}
	})
	t.Run("a false value leaves it off", func(t *testing.T) {
		t.Setenv(CloudLocalDataPlaneEnv, "false")
		_, err := ResolveCloudDataPlaneURL(context.Background(), local[0])
		require.Error(t, err)
	})
	t.Run("on approves named localhost hosts", func(t *testing.T) {
		t.Setenv(CloudLocalDataPlaneEnv, "true")
		for _, target := range local {
			got, err := ResolveCloudDataPlaneURL(context.Background(), target)
			require.NoError(t, err, target)
			require.Equal(t, target, got)
		}
	})
	t.Run("a local URL without a port goes to the configured data-plane port", func(t *testing.T) {
		t.Setenv(CloudLocalDataPlaneEnv, "true")
		t.Setenv(cloudLocalDataPlanePortEnv, "19080")
		got, err := ResolveCloudDataPlaneURL(context.Background(), "https://abc-gw-p1.gateway.openchoreoapis.localhost/api")
		require.NoError(t, err)
		require.Equal(t, "http://abc-gw-p1.gateway.openchoreoapis.localhost:19080/api", got)
		got, err = ResolveCloudDataPlaneURL(context.Background(), local[1])
		require.NoError(t, err)
		require.Equal(t, local[1], got, "a URL that names its port is left alone")
		t.Setenv(cloudLocalDataPlanePortEnv, "not-a-port")
		got, err = ResolveCloudDataPlaneURL(context.Background(), local[2])
		require.NoError(t, err)
		require.Equal(t, local[2], got, "an invalid port is ignored")
	})
	t.Run("on widens nothing else", func(t *testing.T) {
		t.Setenv(cloudDataPlaneDomainsEnv, "")
		t.Setenv(CloudLocalDataPlaneEnv, "true")
		for _, target := range []string{
			"http://127.0.0.1:19080/api", "http://[::1]:19080/api", "http://localhost:19080/api",
			"http://gateway.example.com/api", "http://gw.localhost.evil.example/api",
			"http://user:password@gw.localhost/api", "ftp://gw.localhost/api", "gw.localhost/api",
			"http://synthetic-wso2cloud.gateway.development.example/api",
		} {
			_, err := ResolveCloudDataPlaneURL(context.Background(), target)
			require.Error(t, err, target)
		}
		_, err := ResolveCloudDataPlaneURL(context.Background(), "https://synthetic-wso2cloud.gateway.wso2.com/api")
		require.NoError(t, err, "the cloud hosts stay approved")
	})
}

func TestAGatewayTheStarterCreatesGetsAtLeastFiveMinutesToBecomeActive(t *testing.T) {
	require.GreaterOrEqual(t, GatewayReadyTimeout, 5*time.Minute)
	require.Less(t, GatewayPollInterval, GatewayReadyTimeout)
}

func TestHandleBudgetOfReadsThePlatformsRefusal(t *testing.T) {
	budget, err := HandleBudgetOf([]byte(`{"message":"The gateway name is too long for the \"e-abcdefgh-1\" environment: its handle \"aaaa\" must be 13 characters or fewer."}`))
	require.NoError(t, err)
	require.Equal(t, 13, budget)
	for _, body := range []string{"", "no limit here", "must be 0 characters or fewer"} {
		_, err := HandleBudgetOf([]byte(body))
		require.Error(t, err, body)
	}
}

func TestSyntheticNamesAreRecognised(t *testing.T) {
	for _, name := range []string{"sgw-abcde234-1", "sg-abcde234-12", "sg-abcde234-1xx", "abcde234-12", "gw-abcde234-1"} {
		require.NotEmpty(t, NotSyntheticGateway(map[string]any{"displayName": name}), name)
	}
	for _, name := range []string{"Default", "dgw-abcde234", "production-gateway", "sg-abc-1"} {
		require.Empty(t, NotSyntheticGateway(map[string]any{"displayName": name}), name)
	}
	for _, environment := range []string{"e-abcde234-1", "syn-env-abcde234-12"} {
		require.NotEmpty(t, notInSyntheticEnvironment(map[string]any{"environment": environment}), environment)
	}
	for _, environment := range []string{"development", "production", "e-short-1"} {
		require.Empty(t, notInSyntheticEnvironment(map[string]any{"environment": environment}), environment)
	}
}

func TestGatewayRequestAcceptedTakesCreatedAndAccepted(t *testing.T) {
	require.True(t, GatewayRequestAccepted(http.StatusCreated))
	require.True(t, GatewayRequestAccepted(http.StatusAccepted))
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusConflict, http.StatusInternalServerError} {
		require.False(t, GatewayRequestAccepted(status), status)
	}
}

func TestFeatureStepsExpandsOutlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outline.feature")
	require.NoError(t, os.WriteFile(path, []byte(`Feature: f
  Background:
    Given a start
  Scenario Outline: o
    When I <action> it
      | key | value |
    Then it is <status>

    Examples:
      | action | status |
      | open   | 200    |
      | close  | 409    |
  Scenario: s
    * a last step
`), 0o600))
	require.Equal(t, []string{
		"a start", "I open it", "it is 200", "I close it", "it is 409", "a last step",
	}, FeatureSteps(t, path))
}

func TestTableValues(t *testing.T) {
	values, err := TableValues(nil)
	require.NoError(t, err)
	require.Empty(t, values)

	values, err = TableValues(ValuesTable(t, []string{" displayName ", "Edited"}, []string{"docIds", "[]"}))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"displayName": "Edited", "docIds": "[]"}, values)

	for name, table := range map[string]*godog.Table{
		"three cells": ValuesTable(t, []string{"a", "b", "c"}),
		"empty key":   ValuesTable(t, []string{" ", "b"}),
		"duplicate":   ValuesTable(t, []string{"a", "b"}, []string{"a", "c"}),
	} {
		_, err := TableValues(table)
		require.Error(t, err, name)
	}
}

func TestRefusedCreate(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusCreated: false, http.StatusBadRequest: true, http.StatusConflict: true,
		http.StatusInternalServerError: false, http.StatusBadGateway: false,
	} {
		require.Equal(t, want, RefusedCreate(&httpx.Response{StatusCode: status}), status)
	}
}

func TestPublicationTargetPathAndID(t *testing.T) {
	target := PublicationTarget{Portal: "portal 1", APIType: "rest-api", API: "api/1"}
	require.Equal(t, "/api-portals/portal%201/apis/rest-api/api%2F1/publish", target.Path("/publish"))

	target = PublicationTarget{Portal: "portal-1", APIType: "graphql-api", API: "api-1"}
	parts, err := SplitPublicationID(target.ID())
	require.NoError(t, err)
	require.Equal(t, []string{"portal-1", "graphql-api", "api-1"}, parts)

	for _, id := range []string{"", "a/b", "a/b/c/d", "a//c", "/b/c"} {
		_, err := SplitPublicationID(id)
		require.Error(t, err, id)
	}
}

func TestUnpublishSettled(t *testing.T) {
	for name, tc := range map[string]struct {
		resp *httpx.Response
		want bool
	}{
		"unpublished":       {&httpx.Response{StatusCode: http.StatusNoContent}, true},
		"no API":            {&httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(`{"code":"API_NOT_FOUND"}`)}, false},
		"no portal":         {&httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(`{"code":"API_PORTAL_NOT_FOUND"}`)}, true},
		"unreadable 404":    {&httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(`not json`)}, true},
		"nothing published": {&httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"PUBLICATION_STATE_CONFLICT"}`)}, true},
		"portal conflict":   {&httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"PUBLICATION_PORTAL_CONFLICT"}`)}, false},
		"unreadable":        {&httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`not json`)}, false},
		"server error":      {&httpx.Response{StatusCode: http.StatusServiceUnavailable}, false},
	} {
		require.Equal(t, tc.want, unpublishSettled(tc.resp), name)
	}
}

func TestMultipartFields(t *testing.T) {
	body, contentType, err := MultipartFields([][2]string{{"metadata", `{"id":"api-1"}`}})
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	require.Equal(t, []string{`{"id":"api-1"}`}, form.Value["metadata"])
}

func TestErrorStatusReadsTheStatusOfAnUnexpectedAnswer(t *testing.T) {
	answer := ResponseError(&httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(`{}`)})
	require.Equal(t, http.StatusNotFound, errorStatus(answer))
	require.Equal(t, http.StatusNotFound, errorStatus(fmt.Errorf("reading the project: %w", answer)))
	require.Contains(t, answer.Error(), "unexpected response")
	require.Zero(t, errorStatus(errors.New("no answer")))
	require.Zero(t, errorStatus(nil))
}

func TestTheSuiteSelectsOnlyPrefixedResourcesAndTheDefaultEnvironment(t *testing.T) {
	for name, tc := range map[string]struct {
		check func(string) bool
		given string
		want  bool
	}{
		"the default environment":              {OwnedEnvironment, "development", true},
		"an environment with the prefix":       {OwnedEnvironment, "apip-test-environment", true},
		"the prefix in another case":           {OwnedEnvironment, "APIP-TEST-Environment", true},
		"another console's environment":        {OwnedEnvironment, "qa", false},
		"the prefix without a name":            {OwnedEnvironment, "apip-test-", false},
		"an environment named like a pipeline": {OwnedEnvironment, "default", false},
		"the default pipeline is not used":     {OwnedPipeline, "default", false},
		"a pipeline with the prefix":           {OwnedPipeline, "apip-test-pipeline-2", true},
		"another console's pipeline":           {OwnedPipeline, "release", false},
		"a blank name":                         {OwnedPipeline, "  ", false},
	} {
		require.Equal(t, tc.want, tc.check(tc.given), name)
	}
}

func TestListEnvironmentsReadsEveryPage(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, _ := CloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/environments", r.URL.Path)
		offset := r.URL.Query().Get("offset")
		page := []map[string]any{{"name": "development"}, {"name": "production", "isProduction": true}}
		if offset == "2" {
			page = []map[string]any{{"name": "apip-test-e1"}}
		}
		off := 0
		if offset == "2" {
			off = 2
		}
		WriteJSON(w, http.StatusOK, map[string]any{"list": page,
			"pagination": map[string]any{"total": 3, "offset": off, "limit": 2}})
	})

	items, err := steps.ListEnvironments(ctx)

	require.NoError(t, err)
	require.Len(t, items, 3, "an environment on the second page is listed too")
	require.Equal(t, "apip-test-e1", items[2]["name"])
}

func TestSendWithCredentialSendsEachStoredCredentialAsItShould(t *testing.T) {
	var got [][2]string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, [2]string{r.Header.Get("Authorization"), r.Header.Get("Subscription-Key")})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, InsecureSkipVerify: true}), 0, 0),
		DataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	for key, value := range map[string]string{
		KeyConsumerSubscriptionToken: "token-2", KeyPreviousSubscriptionToken: "token-1", KeyAccessToken: "aGVhZA.Ym9keQ.c2ln",
	} {
		require.NoError(t, tcontext.Set(ctx, key, value))
	}

	for _, send := range []struct{ header, credential string }{
		{"Subscription-Key", "subscription token"}, {"Subscription-Key", "previous subscription token"},
		{"Authorization", "access token"}, {"Authorization", "access token with a forged signature"},
	} {
		require.NoError(t, steps.SendWithCredentialUntilStatus(ctx, http.MethodGet, server.URL, send.header, send.credential, http.StatusOK))
	}
	require.Equal(t, [][2]string{
		{"", "token-2"}, {"", "token-1"}, {"Bearer aGVhZA.Ym9keQ.c2ln", ""}, {"Bearer aGVhZA.Ym9keQ.A2ln", ""},
	}, got, "a subscription token is sent as it is in its own header, an access token as a bearer token, and a forged one with another signature")
}

func TestForgeSignatureChangesOnlyTheSignature(t *testing.T) {
	for token, want := range map[string]string{"h.b.sig": "h.b.Aig", "h.b.Asig": "h.b.Bsig"} {
		forged, err := forgeSignature(token)
		require.NoError(t, err)
		require.Equal(t, want, forged)
	}
	for _, token := range []string{"", "opaque", "h.b.", "h.b"} {
		_, err := forgeSignature(token)
		require.ErrorContains(t, err, "not a signed JWT", token)
	}
}

func TestWithKindAddsTheKindOnlyForAnotherKind(t *testing.T) {
	require.Equal(t, "/p", WithKind("/p", ""))
	require.Equal(t, "/p?kind=GraphQLApi", WithKind("/p", KindGraphQLAPI))
	require.Equal(t, "/p?a=1&kind=GraphQLApi", WithKind("/p?a=1", KindGraphQLAPI))
}

func TestAPIRefForSelectsTheGraphQLAPI(t *testing.T) {
	require.Equal(t, GraphQLAPI, APIRefFor(GraphQLAPILabel))
	require.Equal(t, KindGraphQLAPI, GraphQLAPI.kind)
	require.Equal(t, GraphQLAPIsCollection, GraphQLAPI.Collection)
	require.Empty(t, PrimaryAPI.kind, "a REST API names no kind")
	require.Empty(t, VersionAPI.builds, "the builds of a new version are not followed")
	require.NotEqual(t, PrimaryAPI.builds, GraphQLAPI.builds)
}

func TestDeployAPIFollowsTheBuildItRendered(t *testing.T) {
	t.Setenv(CloudConsoleAccessTokenEnv, "console-token")
	var paths, queries []string
	var bodies []map[string]any
	answered := "2026-11-18-1"
	steps, ctx, local, _ := CloudSteps(t, DeploymentAnswerStub(t, &answered, &paths, &queries, &bodies))
	local.Set(KeyProjectID, "default")
	local.Set(KeyGatewayID, "gw")
	local.Set(KeyGatewayEnv, "development")
	local.Set(KeyAPIID, "api-1")
	local.Set(KeyVersionAPIID, "api-2")

	require.NoError(t, steps.DeployAPI(ctx, "synthetic REST API"))
	answered = "2026-11-18-2"
	require.NoError(t, steps.RedeployAPI(ctx))
	builds, err := SeenBuilds(ctx, PrimaryAPI)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-11-18-1", "2026-11-18-2"}, builds)
	require.Equal(t, "dep-2", OptionalValue(ctx, KeyDeploymentID))
	require.NotContains(t, bodies[0], "buildId", "a deployment of the current definition names no build")
	require.Equal(t, []string{"", ""}, queries, "a REST API is deployed without a kind")

	require.NoError(t, steps.DeployAPI(ctx, newVersionAPILabel))
	require.Equal(t, "/projects/default/apis/api-2/deployments", paths[2])
	builds, err = SeenBuilds(ctx, PrimaryAPI)
	require.NoError(t, err)
	require.Len(t, builds, 2, "the new version's build is not taken for the first API's")
}

func TestNewGraphQLDefinitionUsesTheGraphQLBackend(t *testing.T) {
	t.Setenv(graphQLBackendEnv, "")
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	_, err := NewGraphQLDefinition(ctx, "gql-1")
	require.Error(t, err, "the project is needed")
	local.Set(KeyProjectID, "default")

	definition, err := NewGraphQLDefinition(ctx, "gql-1")
	require.NoError(t, err)
	require.Equal(t, map[string]any{"main": map[string]string{"url": defaultGraphQLBackend}}, definition["upstream"])
	require.Equal(t, "/gql-1/v1", definition["context"])
	require.Equal(t, "inline", definition["schemaSource"])
	require.Equal(t, GraphQLSchema, definition["sdl"])

	t.Setenv(graphQLBackendEnv, "https://graphql.example.test/graphql/")
	definition, err = NewGraphQLDefinition(ctx, "gql-1")
	require.NoError(t, err)
	require.Equal(t, map[string]any{"main": map[string]string{"url": "https://graphql.example.test/graphql"}}, definition["upstream"])
	t.Setenv(graphQLBackendEnv, "http://graphql.example.test")
	_, err = NewGraphQLDefinition(ctx, "gql-1")
	require.Error(t, err, "the backend must be https")
}
