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
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/setup/finalizer"
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

func valuesTable(t *testing.T, rows ...[]string) *godog.Table {
	t.Helper()
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []row `json:"rows"`
	}{}
	for _, values := range rows {
		cells := make([]cell, len(values))
		for i, value := range values {
			cells[i] = cell{Value: value}
		}
		payload.Rows = append(payload.Rows, row{Cells: cells})
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	var table godog.Table
	require.NoError(t, json.Unmarshal(raw, &table))
	return &table
}

func TestTableValuesReadsKeyValueRows(t *testing.T) {
	values, err := tableValues(valuesTable(t, []string{"version", "2.0.0"}, []string{" context ", "/x"}))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"version": "2.0.0", "context": "/x"}, values)

	values, err = tableValues(nil)
	require.NoError(t, err)
	require.Empty(t, values)
}

func TestTableValuesRejectsMalformedTables(t *testing.T) {
	for name, table := range map[string]*godog.Table{
		"one cell":      valuesTable(t, []string{"version"}),
		"three cells":   valuesTable(t, []string{"a", "b", "c"}),
		"empty key":     valuesTable(t, []string{" ", "value"}),
		"duplicate key": valuesTable(t, []string{"version", "1"}, []string{"version", "2"}),
	} {
		_, err := tableValues(table)
		require.Error(t, err, name)
	}
}

func TestNewAPIDefinitionDefaults(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	t.Setenv(backendEnv, "")
	definition, err := newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"main": map[string]string{"url": defaultBackend}}, definition["upstream"])
	t.Setenv(backendEnv, "https://backend.example.com/mock/")
	overridden, err := newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"main": map[string]string{"url": "https://backend.example.com/mock"}}, overridden["upstream"])
	t.Setenv(backendEnv, "http://backend.example.com")
	_, err = newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.ErrorContains(t, err, backendEnv)
	t.Setenv(backendEnv, "")
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
	require.Equal(t, primaryAPI, apiRefFor("synthetic REST API"))
	require.Equal(t, versionAPI, apiRefFor(newVersionAPILabel))
	require.NotEqual(t, primaryAPI.id, versionAPI.id)
	require.NotEqual(t, primaryAPI.deployment, versionAPI.deployment)
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
	steps := &Steps{funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{
		Timeout:            5 * time.Second,
		InsecureSkipVerify: true,
	}), 0, 0), dataPlaneURLResolver: func(_ context.Context, target string) (string, error) {
		return target, nil
	}}

	require.NoError(t, steps.sendUntilStatusWith(ctx, http.MethodGet, server.URL, http.StatusOK,
		map[string]string{"X-API-Key": "secret"}))
	require.Equal(t, "secret", gotKey)
}

func TestSendWithAPIKeyRequiresStoredKeyAndHeader(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{}

	err := steps.sendWithAPIKeyUntilStatus(ctx, http.MethodGet, "https://x.gateway.wso2.com", "X-API-Key", http.StatusOK)
	require.ErrorContains(t, err, keyAPIKey)

	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(keyAPIKey, "secret")
	err = steps.sendWithAPIKeyUntilStatus(ctx, http.MethodGet, "https://x.gateway.wso2.com", " ", http.StatusOK)
	require.ErrorContains(t, err, "header name is required")
}

func TestAssertCloudResponseStatus(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)
	steps := &Steps{funnel: funnel}

	require.Error(t, steps.assertCloudResponseStatus(ctx, http.StatusConflict))

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusConflict}))
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusConflict))
	require.ErrorContains(t, steps.assertCloudResponseStatus(ctx, http.StatusBadRequest), "expected response status 400")
}

func TestAssertAPIResponseMatchesRequestedPost(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)
	steps := &Steps{funnel: funnel}

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"method":"GET","path":"/posts/2"}`)}))
	require.NoError(t, steps.assertAPIResponse(ctx, 2))
	require.ErrorContains(t, steps.assertAPIResponse(ctx, 1), "contain post 1")
}

// cloudAPIStub serves handler as the organization's platform API endpoint.
func cloudAPIStub(t *testing.T, handler http.HandlerFunc) *frameworkruntime.Topology {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The cloud API's project route is served next to the platform API's; the handler sees
		// both as plain collection paths.
		r.URL.Path = strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, platformAPIRoute), coreAPIRoute)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	inst, err := components.NewExternalInstance(corecatalog.CloudConsole(), 0, 1,
		map[string]string{corecatalog.EndpointAPIPPlatformAPI: server.URL + platformAPIRoute})
	require.NoError(t, err)
	set := components.NewSet()
	require.NoError(t, set.Add(inst))
	return &frameworkruntime.Topology{Instances: set}
}

func TestFindActiveGatewayRecordsItsEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
			{"id": "inactive", "isActive": false},
			{"id": "gw-1", "isActive": true, "environment": " development ", "endpoints": []string{"https://gw.example.com/"}},
		}})
	})
	steps := &Steps{topo: topo, client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}),
		dataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

	require.NoError(t, steps.findActiveGateway(ctx))
	for key, want := range map[string]string{
		keyGatewayID: "gw-1", keyGatewayEnv: "development", keyGatewayEndpoint: "https://gw.example.com",
	} {
		got, err := requiredValue(ctx, key)
		require.NoError(t, err, key)
		require.Equal(t, want, got, key)
	}
}

func TestFindActiveGatewayDropsTheEnvironmentOfAnEarlierChoice(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	topo := cloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
			{"id": "gw-2", "isActive": true, "endpoints": []string{"https://gw2.example.com"}},
		}})
	})
	steps := &Steps{topo: topo, client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}),
		dataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set(keyGatewayID, "gw-1")
	local.Set(keyGatewayEnv, "development")

	require.NoError(t, steps.findActiveGateway(ctx))
	id, err := requiredValue(ctx, keyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "gw-2", id)
	require.Empty(t, optionalValue(ctx, keyGatewayEnv), "the previous gateway's environment must not be paired with the new gateway")
}

func TestRefusedCreateIsAClientErrorOnly(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusBadRequest: true, http.StatusConflict: true, http.StatusUnprocessableEntity: true,
		http.StatusOK: false, http.StatusAccepted: false, http.StatusInternalServerError: false, http.StatusBadGateway: false,
	} {
		require.Equal(t, want, refusedCreate(&httpx.Response{StatusCode: status}), status)
	}
}

func TestUndeployAPIUsesTheProjectAndEnvironmentRoute(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var method, path, environment, gateway string
	status := http.StatusNoContent
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		environment, gateway = r.URL.Query().Get("environment"), r.URL.Query().Get("gatewayId")
		w.WriteHeader(status)
	})
	steps := &Steps{topo: topo, client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyDeploymentID, "dep 1")
	local.Set(keyGatewayID, "gw-1")
	local.Set(keyProjectID, "default")

	err := steps.undeployAPI(ctx)
	require.ErrorContains(t, err, "environment", "the environment is required by the route")
	local.Set(keyGatewayEnv, "development")

	require.NoError(t, steps.undeployAPI(ctx))
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/projects/default/apis/syn-api-1/deployments/dep 1/undeploy", path)
	require.Equal(t, "development", environment)
	require.Equal(t, "gw-1", gateway)

	for _, rejected := range []int{http.StatusOK, http.StatusNotFound} {
		status = rejected
		require.Error(t, steps.undeployAPI(ctx), "only 204 is the documented success")
	}
}

// jwtFor builds an unsigned compact JWT carrying the given header and claims.
func jwtFor(t *testing.T, header, claims map[string]any) string {
	t.Helper()
	encodedHeader, err := encodeJWTSegment(header)
	require.NoError(t, err)
	encodedClaims, err := encodeJWTSegment(claims)
	require.NoError(t, err)
	return encodedHeader + "." + encodedClaims + ".c2ln"
}

// cloudSteps returns steps wired to a stub cloud API, and a scenario context that holds a
// cleanup registry so created resources are recorded.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	topo := cloudAPIStub(t, handler)
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0)
	steps := &Steps{topo: topo, funnel: funnel, client: funnel.Client()}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	registry := cleanup.NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, cleanup.Install(ctx, registry))
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	return steps, ctx, local, registry
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func TestDecodeJWT(t *testing.T) {
	token := jwtFor(t, map[string]any{"alg": "RS256", "kid": "k1"}, map[string]any{"ouId": "org-1", "exp": 4102444800})
	header, claims, err := decodeJWT(token)
	require.NoError(t, err)
	require.Equal(t, "k1", header["kid"])
	require.Equal(t, "org-1", claims["ouId"])

	for _, malformed := range []string{"", "a.b", "a.b.c.d", "!!!.e30.c2ln", "e30.bm90LWpzb24.c2ln"} {
		_, _, err := decodeJWT(malformed)
		require.Error(t, err, malformed)
		if malformed != "" {
			require.NotContains(t, err.Error(), malformed, "an error must not repeat the token")
		}
	}
}

func TestOrganisationClaimPrefersOuIDThenOrganization(t *testing.T) {
	name, value, ok := organisationClaim(map[string]any{"organization": "o", "ouId": "id"})
	require.True(t, ok)
	require.Equal(t, "ouId", name)
	require.Equal(t, "id", value)

	name, value, ok = organisationClaim(map[string]any{"organization": "o"})
	require.True(t, ok)
	require.Equal(t, "organization", name)
	require.Equal(t, "o", value)

	for _, claims := range []map[string]any{{}, {"ouId": ""}, {"ouId": 7}, {"organization": " "}} {
		_, _, ok = organisationClaim(claims)
		require.False(t, ok, "%v", claims)
	}
}

func TestTokenExpiry(t *testing.T) {
	expiry, ok := tokenExpiry(map[string]any{"exp": float64(4102444800)})
	require.True(t, ok)
	require.Equal(t, int64(4102444800), expiry.Unix())

	_, ok = tokenExpiry(map[string]any{"exp": "soon"})
	require.False(t, ok)
	_, ok = tokenExpiry(map[string]any{})
	require.False(t, ok)
}

func TestForgeTokenKinds(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	realHeader := map[string]any{"alg": "RS256", "kid": "real-key"}
	realClaims := map[string]any{"ouId": "real-org", "ouHandle": "real", "exp": float64(now.Add(time.Hour).Unix())}

	resigned, err := forgeToken(forgeBadSignature, realHeader, realClaims, now)
	require.NoError(t, err)
	header, claims, err := decodeJWT(resigned)
	require.NoError(t, err)
	require.Equal(t, "RS256", header["alg"])
	require.Equal(t, "real-key", header["kid"])
	require.Equal(t, "real-org", claims["ouId"])
	require.Equal(t, float64(now.Add(time.Hour).Unix()), claims["exp"], "the claims are the real ones, so only the signature is wrong")
	require.Equal(t, "real-org", realClaims["ouId"], "the real claims are copied, not modified")
	signed, err := base64.RawURLEncoding.DecodeString(strings.Split(resigned, ".")[2])
	require.NoError(t, err)
	require.Len(t, signed, 256)

	unsigned, err := forgeToken(forgeUnsigned, nil, nil, now)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(unsigned, "."), "an unsigned token has an empty signature")
	header, claims, err = decodeJWT(unsigned)
	require.NoError(t, err)
	require.Equal(t, "none", header["alg"])
	require.NotEqual(t, "real-org", claims["ouId"])
	require.Equal(t, "https://issuer.invalid", claims["iss"])

	rsaToken, err := forgeToken(forgeUnknownKey, nil, nil, now)
	require.NoError(t, err)
	header, _, err = decodeJWT(rsaToken)
	require.NoError(t, err)
	require.Equal(t, "RS256", header["alg"])
	require.True(t, strings.HasPrefix(header["kid"].(string), "unknown-"))
	signature, err := base64.RawURLEncoding.DecodeString(strings.Split(rsaToken, ".")[2])
	require.NoError(t, err)
	require.Len(t, signature, 256)

	hmacToken, err := forgeToken(forgeUnknownSecret, nil, nil, now)
	require.NoError(t, err)
	header, _, err = decodeJWT(hmacToken)
	require.NoError(t, err)
	require.Equal(t, "HS256", header["alg"])
	signature, err = base64.RawURLEncoding.DecodeString(strings.Split(hmacToken, ".")[2])
	require.NoError(t, err)
	require.Len(t, signature, sha256.Size)

	again, err := forgeToken(forgeUnsigned, nil, nil, now)
	require.NoError(t, err)
	require.NotEqual(t, unsigned, again, "each forgery names a fresh organisation")

	_, err = forgeToken("with a typo", nil, nil, now)
	require.ErrorContains(t, err, "unsupported token forgery")
}

func TestAssertTokenIsLiveJWT(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{}
	live := jwtFor(t, map[string]any{"alg": "RS256"}, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	t.Setenv(cloudConsoleAccessTokenEnv, live)
	require.NoError(t, steps.assertTokenIsLiveJWT(ctx))

	for name, token := range map[string]string{
		"expired":     jwtFor(t, map[string]any{"alg": "RS256"}, map[string]any{"exp": float64(time.Now().Add(-time.Minute).Unix())}),
		"no expiry":   jwtFor(t, map[string]any{"alg": "RS256"}, map[string]any{"ouId": "o"}),
		"not a token": "opaque",
	} {
		ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
		t.Setenv(cloudConsoleAccessTokenEnv, token)
		err := steps.assertTokenIsLiveJWT(ctx)
		require.Error(t, err, name)
		require.NotContains(t, err.Error(), token)
	}
}

func TestAssertTokenCarriesOrganisation(t *testing.T) {
	steps := &Steps{}
	for name, tc := range map[string]struct {
		claims  map[string]any
		wantErr bool
	}{
		"ouId":         {claims: map[string]any{"ouId": "org-1", "ouHandle": "acme"}},
		"organization": {claims: map[string]any{"organization": "org-1"}},
		"none":         {claims: map[string]any{"sub": "user"}, wantErr: true},
	} {
		ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
		t.Setenv(cloudConsoleAccessTokenEnv, jwtFor(t, map[string]any{"alg": "RS256"}, tc.claims))
		err := steps.assertTokenCarriesOrganisation(ctx)
		if tc.wantErr {
			require.Error(t, err, name)
		} else {
			require.NoError(t, err, name)
		}
	}
}

func TestSendWithoutAuthorizationOmitsTheHeaderAndPublishesTheResponse(t *testing.T) {
	var authorization []string
	var method, path string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		authorization, method, path = r.Header.Values("Authorization"), r.Method, r.URL.Path
		w.WriteHeader(http.StatusUnauthorized)
	})
	require.NoError(t, steps.sendWithoutAuthorization(ctx, "post", "/environments"))
	require.Empty(t, authorization)
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/environments", path)
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusUnauthorized))
}

func TestSendCloudRequestRejectsUnsupportedInput(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request should be sent")
	})
	require.ErrorContains(t, steps.sendWithoutAuthorization(ctx, "TRACE", "/projects"), "unsupported HTTP method")
	require.ErrorContains(t, steps.sendWithoutAuthorization(ctx, "GET", "projects"), "must start with /")
}

func TestSendWithAuthorizationHeaderSendsTheValueVerbatim(t *testing.T) {
	var got string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	})
	require.NoError(t, steps.sendWithAuthorizationHeader(ctx, "GET", "/projects", "Basic dXNlcjpwYXNz"))
	require.Equal(t, "Basic dXNlcjpwYXNz", got)
}

func TestSendWithForgedTokenSendsABearerThatIsNotTheConsoleToken(t *testing.T) {
	real := jwtFor(t, map[string]any{"alg": "RS256", "kid": "k"}, map[string]any{"ouId": "real-org", "exp": float64(time.Now().Add(time.Hour).Unix())})
	t.Setenv(cloudConsoleAccessTokenEnv, real)
	var sent string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	})
	for _, kind := range []string{forgeBadSignature, forgeUnsigned, forgeUnknownKey, forgeUnknownSecret} {
		require.NoError(t, steps.sendWithForgedToken(ctx, "GET", "/projects", kind), kind)
		require.True(t, strings.HasPrefix(sent, "Bearer "), kind)
		require.NotEqual(t, "Bearer "+real, sent, kind)
		_, _, err := decodeJWT(strings.TrimPrefix(sent, "Bearer "))
		require.NoError(t, err, kind)
	}
}

func TestSendWithForgedTokenNeedsTheConsoleTokenOnlyForTheBadSignatureKind(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	require.NoError(t, steps.sendWithForgedToken(ctx, "GET", "/projects", forgeUnknownKey))
	require.ErrorContains(t, steps.sendWithForgedToken(ctx, "GET", "/projects", forgeBadSignature), cloudConsoleAccessTokenEnv)
}

func TestSendWithExpiredTokenSkipsWithoutOne(t *testing.T) {
	t.Setenv(cloudConsoleExpiredTokenEnv, "")
	called := false
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusUnauthorized)
	})
	require.ErrorIs(t, steps.sendWithExpiredToken(ctx, "GET", "/projects"), godog.ErrSkip)
	require.False(t, called, "no request is sent without an expired token")
}

func TestSendWithExpiredTokenSendsTheTokenOnlyWhenItHasExpired(t *testing.T) {
	var sent string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	})
	header := map[string]any{"alg": "RS256", "kid": "k"}

	expired := jwtFor(t, header, map[string]any{"exp": float64(time.Now().Add(-time.Hour).Unix())})
	t.Setenv(cloudConsoleExpiredTokenEnv, expired)
	require.NoError(t, steps.sendWithExpiredToken(ctx, "GET", "/projects"))
	require.Equal(t, "Bearer "+expired, sent)

	sent = ""
	live := jwtFor(t, header, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	t.Setenv(cloudConsoleExpiredTokenEnv, live)
	err := steps.sendWithExpiredToken(ctx, "GET", "/projects")
	require.ErrorContains(t, err, "has not expired")
	require.NotContains(t, err.Error(), live)
	require.Empty(t, sent, "a token that is still valid is never sent")

	t.Setenv(cloudConsoleExpiredTokenEnv, jwtFor(t, header, map[string]any{"ouId": "org"}))
	require.ErrorContains(t, steps.sendWithExpiredToken(ctx, "GET", "/projects"), "no exp claim")

	t.Setenv(cloudConsoleExpiredTokenEnv, "not-a-jwt")
	err = steps.sendWithExpiredToken(ctx, "GET", "/projects")
	require.ErrorContains(t, err, "not a readable JWT")
	require.NotContains(t, err.Error(), "not-a-jwt,")
	require.Empty(t, sent)
}

func TestForeignOrganisationHeaderIsIgnoredOrReported(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	honoured := false
	var seen []string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("X-Org-Id"))
		if honoured && r.Header.Get("X-Org-Id") != "" {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "forbidden"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": "default"})
	})
	require.NoError(t, steps.assertForeignOrganisationHeaderIgnored(ctx, "GET", "/projects/default", "X-Org-Id"))
	require.Len(t, seen, 2)
	require.Empty(t, seen[0])
	require.True(t, strings.HasPrefix(seen[1], "forged-"))
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusOK))

	honoured = true
	require.ErrorContains(t, steps.assertForeignOrganisationHeaderIgnored(ctx, "GET", "/projects/default", "X-Org-Id"), "changed the status")
}

func TestForeignOrganisationHeaderDetectsAChangedBody(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		id := "default"
		if r.Header.Get("X-Org-Id") != "" {
			id = "someone-else"
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
	})
	require.ErrorContains(t, steps.assertForeignOrganisationHeaderIgnored(ctx, "GET", "/projects/default", "X-Org-Id"), "changed the response body")
}

func TestSameJSONIgnoresFormattingAndFallsBackToBytes(t *testing.T) {
	require.True(t, sameJSON([]byte(`{"a":1,"b":[1,2]}`), []byte("{ \"b\": [1, 2], \"a\": 1 }")))
	require.False(t, sameJSON([]byte(`{"a":1}`), []byte(`{"a":2}`)))
	require.True(t, sameJSON([]byte("plain"), []byte("plain")))
	require.False(t, sameJSON([]byte("plain"), []byte("other")))
}

func publishedResponse(t *testing.T, steps *Steps, ctx context.Context, status int, body string) {
	t.Helper()
	require.NoError(t, steps.funnel.Publish(ctx, &httpx.Response{StatusCode: status, Body: []byte(body), Method: "GET", URL: "https://cloud.example.com/x"}))
}

func TestAssertNoInternalDetail(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, nil)
	publishedResponse(t, steps, ctx, 401, "Unauthorized")
	require.NoError(t, steps.assertNoInternalDetail(ctx))
	publishedResponse(t, steps, ctx, 401, `{"code":"unauthorized","message":"Invalid or expired credentials."}`)
	require.NoError(t, steps.assertNoInternalDetail(ctx))

	for _, leak := range []string{
		"panic: runtime error", "goroutine 17 [running]", "main.go:42", "upstream connect error", "Stack Trace follows",
	} {
		publishedResponse(t, steps, ctx, 401, leak)
		require.ErrorContains(t, steps.assertNoInternalDetail(ctx), "internal detail", leak)
	}
}

func TestResponseAssertions(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	_, err := httpx.Published(ctx)
	require.Error(t, err, "nothing has been published yet")
	require.Error(t, steps.assertCloudResponseClientError(ctx))

	publishedResponse(t, steps, ctx, 500, `{"code":"internal_error"}`)
	require.Error(t, steps.assertCloudResponseClientError(ctx))
	publishedResponse(t, steps, ctx, 404, `{"code":"not_found","message":"gone"}`)
	require.NoError(t, steps.assertCloudResponseClientError(ctx))

	require.NoError(t, steps.assertCloudResponseContains(ctx, "gone"))
	require.Error(t, steps.assertCloudResponseContains(ctx, "present"))
	local.Set("gateway", "gw-1")
	publishedResponse(t, steps, ctx, 400, `{"message":"Gateway \"gw-1\" is not in environment"}`)
	require.NoError(t, steps.assertCloudResponseContains(ctx, `${CTX:gateway}`))

	publishedResponse(t, steps, ctx, 409, `{"code":"gateway_set_incomplete","message":"m"}`)
	require.NoError(t, steps.assertCloudErrorCode(ctx, "gateway_set_incomplete"))
	require.Error(t, steps.assertCloudErrorCode(ctx, "build_limit_reached"))
	publishedResponse(t, steps, ctx, 500, "not json")
	require.Error(t, steps.assertCloudErrorCode(ctx, "internal_error"))
}

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
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var bodies []string
	var paths []string
	next := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies, paths = append(bodies, string(raw)), append(paths, r.URL.Path)
		next++
		writeJSON(w, http.StatusCreated, map[string]string{"buildId": "2026-01-31-" + string(rune('0'+next))})
	})
	local.Set(keyProjectID, "default")
	local.Set(keyAPIID, "syn-api-1")

	require.NoError(t, steps.prepareBuild(ctx, valuesTable(t, []string{"commit", "abc"})))
	require.NoError(t, steps.prepareBuild(ctx, nil))
	require.Equal(t, []string{"/projects/default/apis/syn-api-1/builds", "/projects/default/apis/syn-api-1/builds"}, paths)
	require.JSONEq(t, `{"metadata":{"commit":"abc"}}`, bodies[0])
	require.Empty(t, strings.TrimSpace(bodies[1]), "a build without metadata sends no body")

	ids, err := requiredValue(ctx, keyBuildIDs)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-1,2026-01-31-2", ids)
	latest, err := requiredValue(ctx, keyBuildID)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-2", latest)
	require.Empty(t, optionalValue(ctx, keyBuildMetadata), "metadata describes the latest build only")
	require.NoError(t, steps.assertBuildIDShape(ctx))
}

func TestPrepareBuildRejectsAnythingButCreated(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal_error"})
	})
	local.Set(keyProjectID, "default")
	local.Set(keyAPIID, "syn-api-1")
	require.Error(t, steps.prepareBuild(ctx, nil))
	require.Empty(t, optionalValue(ctx, keyBuildIDs))
}

func TestAssertBuildIDShape(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	require.Error(t, steps.assertBuildIDShape(ctx), "no build prepared")
	for id, ok := range map[string]bool{"2026-01-31-1": true, "2026-01-31-12": true, "2026-01-31-0": false, "build-1": false, "2026-1-31-1": false,
		"2026-99-99-1": false, "2026-02-30-1": false} {
		local.Set(keyBuildIDs, "2025-12-31-1,"+id)
		require.Equal(t, ok, steps.assertBuildIDShape(ctx) == nil, id)
	}
}

func TestBuildListingAssertions(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	listing := `{"list":[{"buildId":"2026-01-31-2"},{"buildId":"2026-01-31-1","metadata":{"commit":"abc"}}]}`
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listing))
	})
	local.Set(keyProjectID, "default")
	local.Set(keyAPIID, "syn-api-1")

	local.Set(keyBuildIDs, "2026-01-31-1,2026-01-31-2")
	require.NoError(t, steps.assertBuildsNewestFirst(ctx))
	local.Set(keyBuildIDs, "2026-01-31-2,2026-01-31-1")
	require.ErrorContains(t, steps.assertBuildsNewestFirst(ctx), "not listed before")
	local.Set(keyBuildIDs, "2026-01-31-1")
	require.ErrorContains(t, steps.assertBuildsNewestFirst(ctx), "at least two")

	local.Set(keyBuildIDs, "2026-01-31-1")
	local.Set(keyBuildMetadata, `{"commit":"abc"}`)
	require.NoError(t, steps.assertBuildListed(ctx))
	local.Set(keyBuildMetadata, `{"commit":"def"}`)
	require.ErrorContains(t, steps.assertBuildListed(ctx), "metadata")
	local.Set(keyBuildMetadata, "")
	require.NoError(t, steps.assertBuildListed(ctx))

}

func deploymentCreated() map[string]any {
	return map[string]any{"list": []map[string]any{
		{"gatewayId": "dev-default", "deploymentId": "dep-1", "buildId": "2026-01-31-1", "status": "DEPLOYING", "endpointUrl": "https://echo.example.com/anything"},
		{"gatewayId": "dev-extra", "deploymentId": "dep-2", "buildId": "2026-01-31-1", "status": "DEPLOYING"},
	}}
}

func TestRequestDeploymentRecordsWhatWasDeployed(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var path, body string
	status := http.StatusCreated
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		path, body = r.URL.Path, string(raw)
		if status != http.StatusCreated {
			writeJSON(w, status, map[string]string{"code": "invalid_request"})
			return
		}
		writeJSON(w, status, deploymentCreated())
	})
	local.Set(keyProjectID, "default")
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyGatewayID, "dev-default")
	local.Set(keyBuildIDs, "2026-01-31-1")

	table := valuesTable(t, []string{"environment", "development"}, []string{"gatewayIds", "dev-default,dev-extra"})
	require.NoError(t, steps.requestDeployment(ctx, table))
	require.Equal(t, "/projects/default/apis/syn-api-1/deployments", path)
	require.JSONEq(t, `{"environment":"development","gateways":[{"gatewayId":"dev-default"},{"gatewayId":"dev-extra"}]}`, body)
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusCreated))
	require.NoError(t, steps.assertDeploymentListsActiveGateway(ctx))
	require.NoError(t, steps.assertDeployedBuildIsPrepared(ctx))

	deployment, err := requiredValue(ctx, keyDeploymentID)
	require.NoError(t, err)
	require.Equal(t, "dep-1", deployment)

	local.Set(keyBuildIDs, "2026-01-31-9")
	require.ErrorContains(t, steps.assertDeployedBuildIsPrepared(ctx), "prepared build")

	status = http.StatusBadRequest
	local.Set(keyDeploymentID, "unchanged")
	require.NoError(t, steps.requestDeployment(ctx, table), "a refusal is for the scenario to assert on")
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusBadRequest))
	kept, _ := requiredValue(ctx, keyDeploymentID)
	require.Equal(t, "unchanged", kept, "a refused request records nothing")
	_, err = recordedTargets(ctx)
	require.ErrorContains(t, err, "no deployment has been requested", "the targets of an earlier request do not answer for a refused one")
}

func TestDeploymentAssertionsRequireTheActiveGateway(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keyGatewayID, "dev-default")
	publishedResponse(t, steps, ctx, 201, `{"list":[{"gatewayId":"other","deploymentId":"d"}]}`)
	require.ErrorContains(t, steps.assertDeploymentListsActiveGateway(ctx), "does not list gateway")
	publishedResponse(t, steps, ctx, 201, `{"list":[{"gatewayId":"dev-default"}]}`)
	require.ErrorContains(t, steps.assertDeploymentListsActiveGateway(ctx), "no deployment id")
	require.Error(t, steps.assertDeployedBuildIsPrepared(ctx))
}

func TestEchoBackendSteps(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	t.Setenv(echoBackendEnv, "")
	require.NoError(t, steps.useEchoBackend(ctx))
	backend, _ := requiredValue(ctx, keyEchoBackendURL)
	require.Equal(t, defaultEchoBackend, backend)

	t.Setenv(echoBackendEnv, "https://echo.example.com/base/")
	require.NoError(t, steps.useEchoBackend(ctx))
	backend, _ = requiredValue(ctx, keyEchoBackendURL)
	require.Equal(t, "https://echo.example.com/base", backend)

	for _, invalid := range []string{"http://echo.example.com", "echo.example.com", "https://"} {
		t.Setenv(echoBackendEnv, invalid)
		require.Error(t, steps.useEchoBackend(ctx), invalid)
	}

	publishedResponse(t, steps, ctx, 200, `{"method":"GET","path":"/base/posts/1"}`)
	require.NoError(t, steps.assertEchoBackendReceived(ctx, "/posts/1"))
	publishedResponse(t, steps, ctx, 200, `{"method":"GET","path":"/posts/1"}`)
	require.NoError(t, steps.assertEchoBackendReceived(ctx, "/posts/1"), "the backend's base path may be stripped before it receives the request")
	require.ErrorContains(t, steps.assertEchoBackendReceived(ctx, "/posts/2"), "received path")
	publishedResponse(t, steps, ctx, 200, `{"userId":1,"id":1}`)
	require.ErrorContains(t, steps.assertEchoBackendReceived(ctx, "/posts/1"), "not an echo")

	local.Set(keyGatewayID, "dev-default")
	publishedResponse(t, steps, ctx, 201, `{"list":[{"gatewayId":"dev-default","deploymentId":"d","endpointUrl":"https://echo.example.com/base"}]}`)
	require.NoError(t, steps.assertDeploymentReportsEchoEndpoint(ctx))
	publishedResponse(t, steps, ctx, 201, `{"list":[{"gatewayId":"dev-default","deploymentId":"d","endpointUrl":"https://jsonplaceholder.example.com"}]}`)
	require.ErrorContains(t, steps.assertDeploymentReportsEchoEndpoint(ctx), "production endpoint")
}

func TestStageAssertions(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
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
	local.Set(keyProjectID, "default")
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyGatewayID, "dev-default")
	local.Set(keyGatewayEnv, "development")

	require.NoError(t, steps.assertStagesMatchPipeline(ctx))
	require.NoError(t, steps.assertActiveGatewayStageStatus(ctx, "NOT_DEPLOYED"))
	require.ErrorContains(t, steps.assertActiveGatewayStageStatus(ctx, "DEPLOYED"), "want")

	local.Set(keyGatewayEnv, "staging")
	require.ErrorContains(t, steps.assertActiveGatewayStageStatus(ctx, "NOT_DEPLOYED"), "no deployment stage")
	local.Set(keyGatewayEnv, "production")
	require.ErrorContains(t, steps.assertActiveGatewayStageStatus(ctx, "NOT_DEPLOYED"), "does not list gateway")

	stages = `{"list":[{"environment":"development","gateways":[]}]}`
	require.ErrorContains(t, steps.assertStagesMatchPipeline(ctx), "pipeline has")
	stages = `{"list":[{"environment":"development","gateways":[]},{"environment":"development","gateways":[]}]}`
	require.ErrorContains(t, steps.assertStagesMatchPipeline(ctx), "more than one")
	stages = `{"list":[{"environment":"development"},{"environment":"production","gateways":[]}]}`
	require.ErrorContains(t, steps.assertStagesMatchPipeline(ctx), "does not list its gateways")
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
		writeJSON(w, *status, map[string]any{"list": list})
	}
}

func TestDeployAPIDeploysThroughTheProjectAndKeepsTheDeploymentID(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var requests []map[string]any
	status := http.StatusCreated
	steps, ctx, local, _ := cloudSteps(t, projectDeployStub(t, "/projects/default/apis/syn-api-1/deployments", &requests, &status))
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyGatewayID, "dev-default")

	require.ErrorContains(t, steps.deployAPI(ctx, "synthetic REST API"), "does not report its environment")
	local.Set(keyGatewayEnv, "development")
	require.Error(t, steps.deployAPI(ctx, "synthetic REST API"), "the project is needed to deploy")
	local.Set(keyProjectID, "default")

	require.NoError(t, steps.deployAPI(ctx, "synthetic REST API"))
	require.Equal(t, "d", optionalValue(ctx, keyDeploymentID))
	require.Len(t, requests, 1)
	require.Equal(t, "development", requests[0]["environment"])
	require.Equal(t, []any{map[string]any{"gatewayId": "dev-default"}}, requests[0]["gateways"])

	require.NoError(t, steps.redeployAPI(ctx), "a redeployment is another deployment of the current definition")
	require.Len(t, requests, 2)

	status = http.StatusNotFound
	require.ErrorContains(t, steps.redeployAPI(ctx), "deploying APIP REST API")
}

func TestDeployToGatewayRejectsAResponseThatListsNoDeploymentOnTheGateway(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	body := `{"list":[{"gatewayId":"other","deploymentId":"d"},{"gatewayId":"gw","deploymentId":""}]}`
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(body))
	})
	local.Set(keyProjectID, "default")

	_, err := steps.deployToGateway(ctx, "api", "development", "gw")
	require.ErrorContains(t, err, `no deployment on gateway "gw"`)

	body = `not json`
	_, err = steps.deployToGateway(ctx, "api", "development", "gw")
	require.ErrorContains(t, err, "decoding the deployment response")
}

func TestAssertDeploymentStatusReason(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	reason := "GATEWAY_PROCESSING_ERROR"
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{
			{"deploymentId": "dep-1", "status": "FAILED", "statusReason": reason},
		}})
	})
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyGatewayID, "dev-default")
	require.Error(t, steps.assertDeploymentStatusReason(ctx), "no deployment recorded")
	local.Set(keyDeploymentID, "dep-1")
	require.NoError(t, steps.assertDeploymentStatusReason(ctx))
	reason = "failed to parse: line 3"
	require.ErrorContains(t, steps.assertDeploymentStatusReason(ctx), "error code")
	local.Set(keyDeploymentID, "unknown")
	require.ErrorContains(t, steps.assertDeploymentStatusReason(ctx), "not listed")
}

func TestEmptyEnvironmentDeletesOnlyThatEnvironmentsGateways(t *testing.T) {
	var deleted []string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{
			{"id": "syn-env-default", "environment": "syn-env"},
			{"id": "syn-env-extra", "environment": "syn-env"},
			{"id": "development-default", "environment": "development"},
		}})
	})
	base, err := steps.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	require.NoError(t, emptyEnvironment(ctx, steps.client, base, "console-token", "syn-env"))
	require.Equal(t, []string{"/managed-gateways/syn-env-default", "/managed-gateways/syn-env-extra"}, deleted)
}

func TestEmptyEnvironmentReportsRefusals(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{{"id": "g", "environment": "e"}}})
	})
	base, _ := steps.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.Error(t, emptyEnvironment(ctx, steps.client, base, "console-token", "e"))
	require.NoError(t, emptyEnvironment(ctx, steps.client, base, "console-token", "another"), "nothing to delete in another environment")
}

func TestRecordRenderedBuild(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	require.Error(t, steps.recordRenderedBuild(ctx), "no API to record builds for")
	local.Set(keyAPIID, "syn-api-1")
	require.ErrorContains(t, steps.recordRenderedBuild(ctx), "did not report a build")

	local.Set(keyDeployedBuildID, "2026-01-31-1")
	require.NoError(t, steps.recordRenderedBuild(ctx))
	local.Set(keyDeployedBuildID, "2026-01-31-2")
	require.NoError(t, steps.recordRenderedBuild(ctx))

	ids, err := requiredValue(ctx, keyBuildIDs)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-1,2026-01-31-2", ids)
	latest, err := requiredValue(ctx, keyBuildID)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-2", latest)

	require.ErrorContains(t, steps.recordRenderedBuild(ctx), "reused build", "a second deployment must render a new build")

	local.Set(keyDeployedBuildID, "not-a-build")
	require.ErrorContains(t, steps.recordRenderedBuild(ctx), "not a day and index")
	after, _ := requiredValue(ctx, keyBuildIDs)
	require.Equal(t, ids, after, "a refused build is not recorded")
}

func TestBuildStateBelongsToOneAPI(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyDeployedBuildID, "2026-01-31-1")
	require.NoError(t, steps.recordRenderedBuild(ctx))
	local.Set(keyDeployedBuildID, "2026-01-31-2")
	require.NoError(t, steps.recordRenderedBuild(ctx))

	// The next scenario creates another API, whose first build has the same id.
	local.Set(keyAPIID, "syn-api-2")
	local.Set(keyDeployedBuildID, "2026-01-31-1")
	require.NoError(t, steps.recordRenderedBuild(ctx), "build ids are only unique within one API")
	ids, err := requiredValue(ctx, keyBuildIDs)
	require.NoError(t, err)
	require.Equal(t, "2026-01-31-1", ids)
	latest, _ := requiredValue(ctx, keyBuildID)
	require.Equal(t, "2026-01-31-1", latest)
	require.Empty(t, optionalValue(ctx, keyBuildMetadata))
}

func TestParseConfigCellReadsJSONLiteralsAndOtherwiseText(t *testing.T) {
	require.Equal(t, float64(5000), parseConfigCell("5000"))
	require.Equal(t, true, parseConfigCell(" true "))
	require.Equal(t, nil, parseConfigCell("null"))
	require.Equal(t, "5", parseConfigCell(`"5"`), "a quoted value is text")
	require.Equal(t, "a\u2028b", parseConfigCell(`"a\u2028b"`))
	require.Equal(t, map[string]any{"a": float64(1)}, parseConfigCell(`{"a": 1}`))
	require.Equal(t, "debug", parseConfigCell("debug"))
	require.Equal(t, "1 minute", parseConfigCell("1 minute"))
	require.Equal(t, "[x]\nlevel = 1", parseConfigCell("[x]\nlevel = 1"))
}

func expandNothing(value string) (string, error) { return value, nil }

func TestPromotionPathsFromTable(t *testing.T) {
	table := valuesTable(t,
		[]string{"source", "targets"},
		[]string{" dev ", "staging, prod"},
		[]string{"staging", "prod"})
	paths, err := promotionPathsFromTable(table, expandNothing)
	require.NoError(t, err)
	require.Equal(t, []map[string]any{
		{"sourceEnvironment": "dev", "targetEnvironments": []string{"staging", "prod"}},
		{"sourceEnvironment": "staging", "targetEnvironments": []string{"prod"}},
	}, paths)

	expanded, err := promotionPathsFromTable(valuesTable(t, []string{"source", "targets"}, []string{"${A}", "${B}"}),
		func(value string) (string, error) {
			return strings.NewReplacer("${A}", "one", "${B}", "two").Replace(value), nil
		})
	require.NoError(t, err)
	require.Equal(t, "one", expanded[0]["sourceEnvironment"])
	require.Equal(t, []string{"two"}, expanded[0]["targetEnvironments"])
}

func TestPromotionPathsFromTableRejectsMalformedTables(t *testing.T) {
	for name, table := range map[string]*godog.Table{
		"nil":             nil,
		"header only":     valuesTable(t, []string{"source", "targets"}),
		"wrong header":    valuesTable(t, []string{"from", "to"}, []string{"a", "b"}),
		"three columns":   valuesTable(t, []string{"source", "targets"}, []string{"a", "b", "c"}),
		"no source":       valuesTable(t, []string{"source", "targets"}, []string{" ", "b"}),
		"no targets":      valuesTable(t, []string{"source", "targets"}, []string{"a", " , "}),
		"unknown context": valuesTable(t, []string{"source", "targets"}, []string{"a", "${CTX:nothing}"}),
	} {
		_, err := promotionPathsFromTable(table, func(value string) (string, error) {
			if strings.Contains(value, "${CTX:") {
				return "", errors.New("unresolved")
			}
			return value, nil
		})
		require.Error(t, err, name)
	}
}

func TestCreatePipelineRegistersItBeforeTheRequest(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var pending func() []cleanup.Resource
	registeredBeforeRequest := false
	var sent map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/pipelines", r.URL.Path)
		registeredBeforeRequest = len(pending()) == 1
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeJSON(w, http.StatusCreated, map[string]any{"id": sent["name"], "name": sent["name"]})
	})
	pending = registry.Pending
	local.Set("environmentOne", "development")
	local.Set("environmentTwo", "syn-env-1")

	require.NoError(t, steps.createPipeline(ctx, valuesTable(t,
		[]string{"source", "targets"}, []string{"${CTX:environmentOne}", "${CTX:environmentTwo}"})))

	require.True(t, registeredBeforeRequest, "the pipeline must be registered for cleanup before it is requested")
	id, err := requiredValue(ctx, keyPipelineID)
	require.NoError(t, err)
	require.Equal(t, id, sent["name"])
	require.Equal(t, []any{map[string]any{"sourceEnvironment": "development", "targetEnvironments": []any{"syn-env-1"}}}, sent["promotionPaths"])
	require.Equal(t, "pipeline", registry.Pending()[0].Kind.Name)
}

func TestCreatePipelineDoesNotKeepAFailedRequestUnregistered(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request"})
	})
	err := steps.createPipeline(ctx, valuesTable(t, []string{"source", "targets"}, []string{"a", "b"}))
	require.ErrorContains(t, err, "400")
	require.Len(t, registry.Pending(), 1, "a request that may have created the pipeline stays registered")
}

func TestCreateProjectBoundToPipelineWaitsForTheProjectAndDeletesItBeforeThePipeline(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	binds := 0
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			writeJSON(w, http.StatusCreated, map[string]any{"id": body["id"]})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/deployment-pipeline"):
			binds++
			if binds == 1 {
				writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"pipeline": body["pipeline"]})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	require.ErrorContains(t, steps.createProjectBoundToPipeline(ctx), "no deployment pipeline")

	local.Set(keyPipelineID, "syn-pipeline-1")
	require.NoError(t, steps.createProjectBoundToPipeline(ctx))
	require.Equal(t, 2, binds, "a project that is not readable yet is bound again")
	project, err := requiredValue(ctx, keyProjectID)
	require.NoError(t, err)
	require.NotEqual(t, "default", project)

	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: kindPipeline, ID: "syn-pipeline-1", Actor: resourceActor}))
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindGateway, ID: "gw", Actor: resourceActor}))
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindEnvironment, ID: "env", Actor: resourceActor}))
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindAPI, ID: "api", Actor: resourceActor}))
	var order []string
	for _, resource := range registry.Pending() {
		order = append(order, resource.Kind.Name)
	}
	require.Equal(t, []string{"api", "project", "pipeline", "gateway", "environment"}, order)
}

func TestCreateProjectBoundToPipelineRefusesAMismatchedBinding(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if r.Method == http.MethodPost {
			writeJSON(w, http.StatusCreated, map[string]any{"id": body["id"]})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pipeline": "default"})
	})
	local.Set(keyPipelineID, "syn-pipeline-1")
	require.ErrorContains(t, steps.createProjectBoundToPipeline(ctx), `bound to deployment pipeline "default"`)
}

func TestCreateProjectBoundToPipelineStopsOnARefusedBinding(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if r.Method == http.MethodPost {
			writeJSON(w, http.StatusCreated, map[string]any{"id": body["id"]})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request"})
	})
	local.Set(keyPipelineID, "syn-pipeline-1")
	started := time.Now()
	require.ErrorContains(t, steps.createProjectBoundToPipeline(ctx), "400")
	require.Less(t, time.Since(started), 10*time.Second, "a refused binding is not retried")
}

func TestRegisterDeletersRemovesPromotionResourcesInDependencyOrder(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var mu sync.Mutex
	var deleted []string
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{"list": []any{}})
			return
		}
		mu.Lock()
		deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/"))
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	})
	require.NoError(t, RegisterDeleters(registry, steps.topo))

	for _, resource := range []cleanup.Resource{
		{Kind: cleanup.KindEnvironment, ID: "env"}, {Kind: cleanup.KindGateway, ID: "gw"}, {Kind: kindPipeline, ID: "pipe"},
		{Kind: kindPromotionProject, ID: "proj"}, {Kind: cleanup.KindAPI, ID: "api"},
	} {
		resource.Actor = resourceActor
		require.NoError(t, cleanup.Register(ctx, resource))
	}
	require.NoError(t, registry.Sweep(ctx))
	require.Equal(t, []string{"rest-apis/api", "projects/proj", "pipelines/pipe", "managed-gateways/gw", "environments/env"}, deleted)
}

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
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var seen []string
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "PROJECT_NOT_FOUND"})
	})
	require.NoError(t, steps.readDefaultProject(ctx))
	require.NoError(t, steps.readAbsentProject(ctx))
	absent, err := requiredValue(ctx, keyAbsentProjectID)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /projects/default", "GET /projects/" + absent}, seen)
	require.Empty(t, registry.Pending(), "a project that is never created is never registered")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, published.StatusCode)
}

func TestCreateDuplicateProjectRepeatsTheHandleWithAnotherDisplayName(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var body map[string]string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/projects", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		writeJSON(w, http.StatusConflict, map[string]string{"code": "PROJECT_EXISTS"})
	})
	require.ErrorContains(t, steps.createDuplicateProject(ctx), "no synthetic project")
	local.Set(keyProjectID, "syn-project-1")
	require.NoError(t, steps.createDuplicateProject(ctx))
	require.Equal(t, "syn-project-1", body["id"])
	require.NotEqual(t, "Synthetic syn-project-1", body["displayName"])
	require.Empty(t, registry.Pending(), "the handle is already registered by the create that came first")
}

func TestRequestProjectWithDefectiveHandleRegistersTheHandleBeforeTheRequest(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var registeredAtRequest []cleanup.Resource
	var sent map[string]string
	var steps *Steps
	var ctx context.Context
	var registry *cleanup.Registry
	respond := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
			registeredAtRequest = registry.Pending()
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request"})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "PROJECT_NOT_FOUND"})
	}
	steps, ctx, _, registry = cloudSteps(t, respond)

	require.ErrorContains(t, steps.requestProjectWithDefectiveHandle(ctx, "with a typo"), "unknown handle defect")
	require.NoError(t, steps.requestProjectWithDefectiveHandle(ctx, "uppercase"))
	require.Equal(t, strings.ToUpper(sent["id"]), sent["id"])
	require.Equal(t, []cleanup.Resource{{Kind: cleanup.KindProject, ID: sent["id"], Actor: resourceActor, Description: "project " + sent["id"]}},
		registeredAtRequest, "the handle is registered before the request that may create it")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)

	require.NoError(t, steps.assertRejectedProjectAbsent(ctx))
}

func TestRequestProjectWithDefectiveHandleRegistersTheIDAnAcceptedProjectGot(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"id": "renamed-by-the-platform"})
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
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusNotFound
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, status, map[string]string{"code": "x"})
	})
	require.ErrorContains(t, steps.assertRejectedProjectAbsent(ctx), "requested no project")
	local.Set(keyRejectedProjectID, "Bad-Handle")
	require.NoError(t, steps.assertRejectedProjectAbsent(ctx))
	status = http.StatusOK
	require.ErrorContains(t, steps.assertRejectedProjectAbsent(ctx), "should not exist")
}

func TestAwaitProjectActive(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status := "active"
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": "syn-project-1", "status": status})
	})
	require.Error(t, steps.awaitProjectActive(ctx))
	local.Set(keyProjectID, "syn-project-1")
	require.NoError(t, steps.awaitProjectActive(ctx))
	status = "failed"
	started := time.Now()
	require.ErrorContains(t, steps.awaitProjectActive(ctx), "failed")
	require.Less(t, time.Since(started), 10*time.Second, "a failed project is not waited for")
}

func TestProjectPipelineBindingSteps(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	bound := "default"
	var put map[string]string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects/syn-project-1/deployment-pipeline", r.URL.Path)
		if r.Method == http.MethodPut {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&put))
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pipeline": bound})
	})
	require.ErrorContains(t, steps.bindProjectToPipeline(ctx, "x"), "no synthetic project")
	require.ErrorContains(t, steps.readProjectPipeline(ctx), "no synthetic project")
	local.Set(keyProjectID, "syn-project-1")
	local.Set(keyPipelineID, "syn-pipeline-1")

	require.NoError(t, steps.bindProjectToPipeline(ctx, "${CTX:cloudPipelineID}"))
	require.Equal(t, map[string]string{"pipeline": "syn-pipeline-1"}, put)
	require.NoError(t, steps.bindProjectToPipeline(ctx, ""))
	require.Equal(t, map[string]string{"pipeline": ""}, put, "a blank pipeline is sent as it is")

	require.NoError(t, steps.readProjectPipeline(ctx))
	require.NoError(t, steps.assertProjectBoundTo(ctx, "default"))
	require.ErrorContains(t, steps.assertProjectBoundTo(ctx, "${CTX:cloudPipelineID}"), `bound to deployment pipeline "default"`)
	bound = "syn-pipeline-1"
	require.NoError(t, steps.assertProjectBoundTo(ctx, "${CTX:cloudPipelineID}"))
}

func TestCreateProjectsUntilRefusedStopsAtTheFirstRefusal(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	created := 0
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if created == 3 {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{"code": "quota_exceeded"})
			return
		}
		created++
		writeJSON(w, http.StatusCreated, map[string]any{"id": body["id"]})
	})
	require.NoError(t, steps.createProjectsUntilRefused(ctx))
	require.Len(t, registry.Pending(), 4, "every project asked for is registered, the refused one included")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusPaymentRequired, published.StatusCode)
	require.NotEmpty(t, optionalValue(ctx, keyRejectedProjectID))
}

func TestCreateProjectsUntilRefusedGivesUpWhereThereIsNoQuota(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		writeJSON(w, http.StatusCreated, map[string]any{"id": body["id"]})
	})
	require.ErrorContains(t, steps.createProjectsUntilRefused(ctx), "refused none")
	require.Len(t, registry.Pending(), maxQuotaProbeProjects)
}

// leftoverStub serves per-collection lists and records the deletions it was asked for.
type leftoverStub struct {
	mu      sync.Mutex
	lists   map[string][]map[string]any
	failing map[string]int
	deleted []string
	// listedProjects records, per collection, the project of each per-project listing.
	listedProjects map[string][]string
}

func (s *leftoverStub) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer console-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet {
		key := r.URL.Path
		if slices.Contains(projectScopedCollections, r.URL.Path) {
			project := r.URL.Query().Get("projectId")
			if project == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "VALIDATION_FAILED", "message": "projectId query parameter is required"})
				return
			}
			s.listedProjects[r.URL.Path] = append(s.listedProjects[r.URL.Path], project)
			key += "?projectId=" + project
		}
		if status, bad := s.failing[key]; bad {
			w.WriteHeader(status)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": s.lists[key]})
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
	return &leftoverStub{failing: map[string]int{}, listedProjects: map[string][]string{}, lists: map[string][]map[string]any{
		"/secrets":             {{"id": "syn-secret-abcde234-1"}, {"id": "ops-secret"}},
		"/managed-api-portals": {{"id": "syn-portal-abcde234-3", "name": "Synthetic syn-portal-abcde234-3"}, {"id": "ops-portal", "name": "Operations portal"}},
		"/managed-gateways": {
			{"id": "development-syn-gw-abcde234-1", "environment": "development"},
			{"id": "development-default", "environment": "development"},
			{"id": "development-syn-gw-zzzzz222-1", "environment": "development"},
			{"id": "development-syn-gw-ab", "displayName": "syn-gw-abcde234-12", "environment": "development"},
		},
		"/projects":                                   {{"id": "default"}, {"id": "syn-project-abcde234-2"}, {"id": "ops-project"}},
		"/rest-apis?projectId=default":                {{"id": "syn-api-abcde234-1"}, {"id": "ops-api"}},
		"/rest-apis?projectId=syn-project-abcde234-2": {{"id": "syn-api-abcde234-2"}},
		"/rest-apis?projectId=ops-project":            {{"id": "syn-api-abcde234-9"}},
		"/pipelines":                                  {{"name": "default"}, {"name": "syn-pipeline-abcde234-1"}},
		"/environments":                               {{"name": "development"}, {"name": "syn-env-abcde234-1"}},
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
	topo := cloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, []string{
		"rest-apis/syn-api-abcde234-1",
		"rest-apis/syn-api-abcde234-2",
		"secrets/syn-secret-abcde234-1",
		"managed-api-portals/syn-portal-abcde234-3",
		"managed-gateways/development-syn-gw-abcde234-1",
		"managed-gateways/development-syn-gw-ab",
		"projects/syn-project-abcde234-2",
		"pipelines/syn-pipeline-abcde234-1",
		"environments/syn-env-abcde234-1",
	}, stub.deleted[:9])
	require.Len(t, removed, 9)
	for _, collection := range projectScopedCollections {
		require.ElementsMatch(t, []string{"default", "syn-project-abcde234-2"}, stub.listedProjects[collection],
			collection+" is listed for the default project and the run's projects only")
	}
	for _, id := range stub.deleted {
		require.NotContains(t, id, "default")
		require.NotContains(t, id, "zzzzz222")
		require.NotContains(t, id, "ops-api")
		require.NotContains(t, id, "syn-api-abcde234-9")
		require.NotContains(t, id, "ops-portal")
	}
}

func TestCloudSweepsStillSweepTheDefaultProjectWhenTheProjectsCannotBeListed(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/projects"] = http.StatusForbidden
	topo := cloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.ErrorContains(t, err, "listing the projects that hold /rest-apis")
	require.Contains(t, removed, "/rest-apis/syn-api-abcde234-1", "the default project is still swept")
	require.NotContains(t, removed, "/rest-apis/syn-api-abcde234-2")
}

func TestCloudSweepsKeepSweepingTheOtherProjectsWhenOneProjectCannotBeListed(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/rest-apis?projectId=default"] = http.StatusForbidden
	topo := cloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.ErrorContains(t, err, "project default")
	require.Contains(t, removed, "/rest-apis/syn-api-abcde234-2")
	require.NotContains(t, removed, "/rest-apis/syn-api-abcde234-1")
}

func TestListCollectionKeepsThePagesItReadWhenALaterPageFails(t *testing.T) {
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset >= 2 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}, {"id": "b"}},
			"pagination": map[string]any{"total": 6, "offset": offset, "limit": 2}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.Error(t, err)
	require.Len(t, items, 2, "the items of the pages already read are returned with the error")
}

func TestListCollectionKeepsTheItemsWhenThePageCapIsReached(t *testing.T) {
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "item-" + strconv.Itoa(offset)}},
			"pagination": map[string]any{"total": 100000, "offset": offset, "limit": 1}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.ErrorContains(t, err, "the rest were not read")
	require.Len(t, items, maxListPages)
}

func TestListCollectionReadsEveryAdvertisedPage(t *testing.T) {
	ids := []string{"p1", "p2", "p3", "p4", "p5"}
	var queries []string
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := min(offset+2, len(ids))
		list := make([]map[string]any, 0, 2)
		for _, id := range ids[offset:end] {
			list = append(list, map[string]any{"id": id})
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "list": list,
			"pagination": map[string]any{"total": len(ids), "offset": offset, "limit": 2}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
	require.NoError(t, err)
	require.Len(t, items, 5)
	require.Equal(t, "p5", items[4]["id"])
	require.Equal(t, []string{"", "limit=2&offset=2", "limit=2&offset=4"}, queries, "paging parameters are sent only once the platform advertises more pages")
}

func TestListCollectionReadsOnePageWhenThePlatformDoesNotPage(t *testing.T) {
	requests := 0
	topo := cloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}, {"id": "b"}}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/pipelines")
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, 1, requests)
}

func TestListCollectionRefusesAPlatformThatIgnoresTheOffset(t *testing.T) {
	topo := cloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}, {"id": "b"}},
			"pagination": map[string]any{"total": 10, "offset": 0, "limit": 2}})
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	items, err := listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/projects")
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
			topo := cloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				if requests == 1 {
					fail(w)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "a"}}})
			})
			base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
			require.NoError(t, err)

			items, err := listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/pipelines")
			require.NoError(t, err)
			require.Len(t, items, 1)
			require.Equal(t, 2, requests)
		})
	}
}

func TestListCollectionDoesNotRetryAClientError(t *testing.T) {
	requests := 0
	topo := cloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	_, err = listCollection(context.Background(), httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base, "console-token", "/pipelines")
	require.Error(t, err)
	require.Equal(t, 1, requests)
}

func TestCloudSweepsCarryOnPastAListingThatFails(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/projects"] = http.StatusForbidden
	topo := cloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.ErrorContains(t, err, "listing /projects")
	require.Contains(t, removed, "/pipelines/syn-pipeline-abcde234-1")
	require.Contains(t, removed, "/environments/syn-env-abcde234-1")
}

func TestCloudSweepsSkipACollectionThePlatformDoesNotServe(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/pipelines"] = http.StatusNotFound
	topo := cloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)

	removed, err := sweepLeftovers(context.Background(), base, 5*time.Second)
	require.NoError(t, err, "a route the platform does not serve holds no leftovers")
	require.Contains(t, removed, "/projects/syn-project-abcde234-2")
	require.NotContains(t, removed, "/pipelines/syn-pipeline-abcde234-1")
}

func TestCloudSweepsReportALeftoverThatCannotBeDeleted(t *testing.T) {
	stub := newLeftoverStub()
	stub.failing["/managed-gateways/development-syn-gw-abcde234-1"] = http.StatusConflict
	topo := cloudAPIStub(t, stub.handler)
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
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	stub := newLeftoverStub()
	topo := cloudAPIStub(t, stub.handler)
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
	require.Equal(t, []string{"/managed-gateways/development-syn-gw-" + generator.Suffix() + "-1"}, removed)
}

func TestFinalizerNeedsTheConsoleTokenToLookForLeftovers(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "")
	topo := cloudAPIStub(t, newLeftoverStub().handler)
	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, unique.Install(ctx, generator))
	finalizer := NewFinalizer()
	require.NoError(t, finalizer.observe(ctx, topo))

	_, err = finalizer.Finalize(context.Background())
	require.ErrorContains(t, err, cloudConsoleAccessTokenEnv)
}

func redeploymentStub(t *testing.T, probe func(call int) (answer bool, status int), deployedAfter int) (*Steps, context.Context, string) {
	t.Helper()
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var mu sync.Mutex
	calls := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		if strings.Contains(r.URL.Path, "/data/probe") {
			mu.Lock()
			calls++
			call := calls
			mu.Unlock()
			answer, status := probe(call)
			if !answer {
				conn, _, err := w.(http.Hijacker).Hijack()
				require.NoError(t, err)
				require.NoError(t, conn.Close())
				return
			}
			w.WriteHeader(status)
			return
		}
		mu.Lock()
		state := "DEPLOYING"
		if calls >= deployedAfter {
			state = "DEPLOYED"
		}
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{{"deploymentId": "dep-1", "status": state}}})
	})
	steps.dataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyDeploymentID, "dep-1")
	local.Set(keyGatewayID, "gw-1")
	target, err := steps.apiURL("/data/probe")
	require.NoError(t, err)
	return steps, ctx, target
}

func TestKeepServingUntilDeployedToleratesOneDroppedConnection(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(call int) (bool, int) { return call != 1, http.StatusOK }, 1)
	require.NoError(t, steps.keepServingUntilDeployed(ctx, "GET", target, http.StatusOK))
}

func TestKeepServingUntilDeployedFailsOnTwoDroppedConnectionsInARow(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(int) (bool, int) { return false, 0 }, 1<<30)
	err := steps.keepServingUntilDeployed(ctx, "GET", target, http.StatusOK)
	require.ErrorContains(t, err, "traffic was interrupted")
	require.ErrorContains(t, err, "2 requests in a row got no answer")
}

func TestKeepServingUntilDeployedCountsOnlyConsecutiveDrops(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(call int) (bool, int) { return call%2 == 0, http.StatusOK }, 4)
	require.NoError(t, steps.keepServingUntilDeployed(ctx, "GET", target, http.StatusOK))
}

func TestKeepServingUntilDeployedFailsOnAnotherStatus(t *testing.T) {
	steps, ctx, target := redeploymentStub(t, func(int) (bool, int) { return true, http.StatusServiceUnavailable }, 1)
	err := steps.keepServingUntilDeployed(ctx, "GET", target, http.StatusOK)
	require.ErrorContains(t, err, "expected 200")
	require.ErrorContains(t, err, "503")
}

// gatewayChoiceStub serves the managed-gateway list, a project bound to the "default" pipeline
// and that pipeline, whose only promotion path runs from development to staging.
func gatewayChoiceStub(t *testing.T, gateways []map[string]any, queries *[]string) (*Steps, context.Context) {
	t.Helper()
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudGatewayPinEnv, "")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/managed-gateways":
			if queries != nil {
				*queries = append(*queries, r.URL.RawQuery)
			}
			writeJSON(w, http.StatusOK, map[string]any{"list": gateways})
		case strings.HasSuffix(r.URL.Path, "/deployment-pipeline"):
			writeJSON(w, http.StatusOK, map[string]string{"pipeline": "default"})
		case r.URL.Path == "/pipelines":
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{
				"name":           "default",
				"promotionPaths": []map[string]any{{"sourceEnvironment": "development", "targetEnvironments": []string{"staging"}}},
			}}})
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	steps.dataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(keyProjectID, "default")
	return steps, ctx
}

func listedGateway(id, environment string, extra map[string]any) map[string]any {
	gateway := map[string]any{
		"id": id, "isActive": true, "environment": environment,
		"endpoints": []any{"https://" + id + ".example.com/"},
	}
	for key, value := range extra {
		gateway[key] = value
	}
	return gateway
}

func TestFindActiveGatewayChoosesARegularGatewayOfThePipelineThatNoRunCreated(t *testing.T) {
	var queries []string
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		listedGateway("prod-default", "production", nil),
		listedGateway("dev-ai", "development", map[string]any{"functionalityType": "ai"}),
		listedGateway("dev-sgw-other", "development", map[string]any{"displayName": "sgw-abcdefg2-3"}),
		listedGateway("dev-sgw-upper", "development", map[string]any{"name": "SGW-ABCDEFG2-4"}),
		listedGateway("dev-default", "development", map[string]any{"functionalityType": "regular", "displayName": "default"}),
		listedGateway("stage-default", "staging", nil),
	}, &queries)

	require.NoError(t, steps.findActiveGateway(ctx))
	id, err := tcontext.ResolveString(ctx, keyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-default", id)
	env, err := tcontext.ResolveString(ctx, keyGatewayEnv)
	require.NoError(t, err)
	require.Equal(t, "development", env)
	require.Equal(t, []string{"functionalityType=regular"}, queries, "the platform is asked for regular gateways only")
}

func TestFindActiveGatewayNamesWhyEveryGatewayWasRejected(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		listedGateway("prod-default", "production", nil),
		listedGateway("dev-ai", "development", map[string]any{"functionalityType": "ai"}),
		listedGateway("dev-sgw", "development", map[string]any{"displayName": "sgw-abcdefg2-1"}),
		listedGateway("", "development", nil),
	}, nil)

	err := steps.findActiveGateway(ctx)
	require.ErrorContains(t, err, "no usable APIP gateway among 4 listed")
	require.ErrorContains(t, err, `prod-default: environment "production" is not in the project's deployment pipeline`)
	require.ErrorContains(t, err, `dev-ai: functionality type "ai"`)
	require.ErrorContains(t, err, "dev-sgw: a synthetic gateway of a suite run")
}

func TestFindActiveGatewayWithoutAProjectDoesNotFilterByPipeline(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{listedGateway("prod-default", "production", nil)}, nil)
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(keyProjectID, "")
	require.NoError(t, steps.findActiveGateway(ctx), "no project, so no pipeline to read")
	id, err := tcontext.ResolveString(ctx, keyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "prod-default", id)
}

func TestFindActiveGatewayHonoursAPinnedGateway(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		listedGateway("dev-first", "development", nil),
		listedGateway("dev-pinned", "development", nil),
		listedGateway("dev-operator", "development", map[string]any{"displayName": "frontend-1"}),
	}, nil)

	t.Setenv(cloudGatewayPinEnv, " dev-pinned ")
	require.NoError(t, steps.findActiveGateway(ctx))
	id, err := tcontext.ResolveString(ctx, keyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-pinned", id)

	t.Setenv(cloudGatewayPinEnv, "dev-operator")
	require.NoError(t, steps.findActiveGateway(ctx), "a gateway the operator names is taken even if its name looks like a suite run's")
	id, err = tcontext.ResolveString(ctx, keyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-operator", id)

	t.Setenv(cloudGatewayPinEnv, "dev-missing")
	err = steps.findActiveGateway(ctx)
	require.ErrorContains(t, err, "not the gateway named by "+cloudGatewayPinEnv)
}

func TestFindActiveGatewayRefusesAPinnedGatewayOfASuiteRun(t *testing.T) {
	steps, ctx := gatewayChoiceStub(t, []map[string]any{
		listedGateway("dev-scenario", "development", map[string]any{"displayName": "frontend-1", "description": scenarioGatewayDescription}),
		listedGateway("dev-operator", "development", map[string]any{"displayName": "frontend-2", "description": "the operator's"}),
	}, nil)

	t.Setenv(cloudGatewayPinEnv, "dev-scenario")
	require.ErrorContains(t, steps.findActiveGateway(ctx), "dev-scenario: a scenario gateway of a suite run")

	t.Setenv(cloudGatewayPinEnv, "dev-operator")
	require.NoError(t, steps.findActiveGateway(ctx), "an operator's gateway that looks like a suite run's is still taken")
}

func TestAnEnvironmentDeletionHasABudgetOfItsOwn(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" {
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{}})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	spent, cancel := context.WithCancel(ctx)
	cancel()

	deleter := cloudDeleter(steps.topo, steps.client, "/environments", true)
	require.NoError(t, deleter(spent, cleanup.Resource{Kind: cleanup.KindEnvironment, ID: "e-1"}),
		"a sweep that has used its time does not stop the deletion of the next environment")
}

func TestFindActiveGatewayFailsWhenThePipelineCannotBeRead(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudGatewayPinEnv, "")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" {
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{listedGateway("dev-default", "development", nil)}})
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	steps.dataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	local.Set(keyProjectID, "default")
	require.ErrorContains(t, steps.findActiveGateway(ctx), "deployment pipeline")
}

func TestChooseActiveGatewayAppliesNoFilterToAnAIGateway(t *testing.T) {
	steps := &Steps{dataPlaneURLResolver: func(_ context.Context, target string) (string, error) { return target, nil }}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	choice, err := steps.chooseActiveGateway(ctx, []map[string]any{
		listedGateway("sgw-like", "development", map[string]any{"displayName": "sgw-abcdefg2-1", "functionalityType": "ai"}),
	}, "APIP AI gateway")
	require.NoError(t, err)
	require.Equal(t, "sgw-like", choice.id)
}

func listedManagedGateway(id, environment, status string, active bool) map[string]any {
	return map[string]any{
		"id": id, "environment": environment, "status": status, "isActive": active,
		"isCritical": false, "isDefault": false,
	}
}

func TestBuildManagedGatewayRequest(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)

	_, err := buildManagedGatewayRequest(ctx, map[string]string{"environment": "dev"})
	require.ErrorContains(t, err, "no gateway name chosen")
	local.Set(keyManagedGatewayName, "syn-gw-1")
	local.Set("cloudGatewayEnvironment", "dev")

	body, err := buildManagedGatewayRequest(ctx, map[string]string{
		"environment": "${CTX:cloudGatewayEnvironment}", "isCritical": "true", "properties": `{"owner":"it"}`,
		"version": "1.0",
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"displayName": "syn-gw-1", "environment": "dev", "isCritical": true,
		"properties": map[string]any{"owner": "it"}, "version": "1.0", "description": scenarioGatewayDescription,
	}, body, "a gateway a scenario creates is marked as the suite's own")
	named, err := buildManagedGatewayRequest(ctx, map[string]string{"environment": "dev", "description": "mine"})
	require.NoError(t, err)
	require.Equal(t, "mine", named["description"], "a description the scenario names is kept")

	for name, values := range map[string]map[string]string{
		"no environment":     {"displayName": "x"},
		"unknown key":        {"environment": "dev", "host": "x"},
		"bad bool":           {"environment": "dev", "isDefault": "maybe"},
		"bad properties":     {"environment": "dev", "properties": "[1]"},
		"unresolved context": {"environment": "${CTX:nothing}"},
	} {
		_, err := buildManagedGatewayRequest(ctx, values)
		require.Error(t, err, name)
	}
}

func TestExpectedGatewayID(t *testing.T) {
	require.Equal(t, "dev-my-gateway", expectedGatewayID("dev", " My Gateway ", ""))
	require.Equal(t, "dev-explicit", expectedGatewayID("dev", "My Gateway", " explicit "))
}

func TestGatewayListingSteps(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
			listedManagedGateway("dev-gw", "dev", "ACTIVE", true),
			listedManagedGateway("dev-other", "dev", "DISCONNECTED", false),
		}})
	})
	local.Set(keyGatewayID, "dev-gw")

	require.NoError(t, steps.listManagedGateways(ctx))
	require.NoError(t, steps.assertListingIsValid(ctx))
	require.NoError(t, steps.assertListingIncludesActiveGateway(ctx))

	local.Set(keyGatewayID, "dev-missing")
	require.Error(t, steps.assertListingIncludesActiveGateway(ctx))
}

func TestValidateGatewayListing(t *testing.T) {
	good := []map[string]any{
		listedManagedGateway("dev-default", "dev", "ACTIVE", true),
		listedManagedGateway("dev-new", "dev", "PROVISIONING", false),
		listedManagedGateway("dev-built", "dev", "DISCONNECTED", false),
	}
	good[1]["endpoints"] = []any{"https://gw.example.com"}
	good[1]["functionalityType"] = "ai"
	require.NoError(t, validateGatewayListing(good))

	failed := listedManagedGateway("dev-failed", "dev", "FAILED", false)
	require.ErrorContains(t, validateGatewayListing([]map[string]any{failed}), "without a status reason")
	failed["statusReason"] = "The gateway could not be provisioned."
	require.NoError(t, validateGatewayListing([]map[string]any{failed}))

	require.ErrorContains(t, validateGatewayListing(nil), "lists no managed gateway")
	for name, mutate := range map[string]func(map[string]any){
		"no id":              func(g map[string]any) { delete(g, "id") },
		"no environment":     func(g map[string]any) { g["environment"] = " " },
		"unknown status":     func(g map[string]any) { g["status"] = "" },
		"missing flag":       func(g map[string]any) { delete(g, "isDefault") },
		"reason while built": func(g map[string]any) { g["statusReason"] = "boom" },
		"connected but idle": func(g map[string]any) { g["isActive"] = true; g["status"] = "DISCONNECTED" },
		"active but idle":    func(g map[string]any) { g["status"] = "ACTIVE" },
		"unknown kind":       func(g map[string]any) { g["functionalityType"] = "mcp" },
		"endpoints not list": func(g map[string]any) { g["endpoints"] = "https://gw.example.com" },
		"blank endpoint":     func(g map[string]any) { g["endpoints"] = []any{" "} },
	} {
		gateway := listedManagedGateway("dev-x", "dev", "DISCONNECTED", false)
		mutate(gateway)
		require.Error(t, validateGatewayListing([]map[string]any{gateway}), name)
	}
}

func TestValidateGatewayListingShowsTheItemWithoutAStatus(t *testing.T) {
	err := validateGatewayListing([]map[string]any{{
		"id": "dev-orphan", "environment": "dev", "isActive": false, "isCritical": false, "isDefault": false, "status": "",
	}})
	require.ErrorContains(t, err, "the listing gives it as")
	require.ErrorContains(t, err, "dev-orphan")
}

func TestAssertSelectedEnvironmentListed(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	environments := []map[string]any{{"id": "dev", "name": "dev", "isProduction": false}}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"list": environments})
	})
	require.Error(t, steps.assertSelectedEnvironmentListed(ctx), "nothing was selected")
	local.Set(keyEnvironmentName, "dev")
	require.NoError(t, steps.assertSelectedEnvironmentListed(ctx))
	environments[0]["id"] = "other"
	require.ErrorContains(t, steps.assertSelectedEnvironmentListed(ctx), "own name")
	environments[0]["id"] = "dev"
	delete(environments[0], "isProduction")
	require.ErrorContains(t, steps.assertSelectedEnvironmentListed(ctx), "isProduction")
	local.Set(keyEnvironmentName, "absent")
	require.ErrorContains(t, steps.assertSelectedEnvironmentListed(ctx), "not in the platform's list")
}

func TestRequestManagedGatewayReconcilesTheRegisteredID(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]string{"id": "assigned-by-core"})
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, map[string]string{"environment": "dev"}))
	pending := registry.Pending()
	require.Len(t, pending, 1)
	require.Equal(t, "assigned-by-core", pending[0].ID)
}

func TestRequestManagedGatewayRegistersASuffixedIDTheRepeatWasAnsweredWith(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var answers []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]string{"id": answers[0]})
		answers = answers[1:]
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	values := map[string]string{"environment": "dev"}
	answers = []string{"dev-syn-gw-1", "dev-syn-gw-1-ow9j"}
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))

	var pending []string
	for _, resource := range registry.Pending() {
		pending = append(pending, resource.ID)
	}
	require.ElementsMatch(t, []string{"dev-syn-gw-1", "dev-syn-gw-1-ow9j"}, pending, "the second gateway must not be left behind")
}

func TestRequestManagedGatewayOnceItsNameIsFreeDeletesTheSuffixedGatewayAndTriesAgain(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	gatewayNameReleaseInterval = time.Millisecond
	defer func() { gatewayNameReleaseInterval = 30 * time.Second }()
	answers := []string{"dev-syn-gw-1-ow9j", "dev-syn-gw-1"}
	var deleted []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": answers[0]})
		answers = answers[1:]
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")

	require.NoError(t, steps.requestManagedGatewayOnceItsNameIsFreeWith(ctx, map[string]string{"environment": "dev"}))

	require.Equal(t, []string{"dev-syn-gw-1-ow9j"}, deleted, "only the gateway under the suffixed id is deleted")
	var pending []string
	for _, resource := range registry.Pending() {
		pending = append(pending, resource.ID)
	}
	require.Equal(t, []string{"dev-syn-gw-1"}, pending)
}

func TestRequestManagedGatewayRegistersForCleanupAndKeepsOnlyWhatWasCreated(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusCreated
	var bodies []map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		if status != http.StatusCreated {
			writeJSON(w, status, map[string]string{"code": "conflict", "message": "no"})
			return
		}
		writeJSON(w, status, map[string]string{"id": body["environment"].(string) + "-" + body["displayName"].(string)})
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	values := map[string]string{"environment": "dev"}

	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)

	// A repeat that is refused must not drop the registration the first request made.
	status = http.StatusConflict
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1, "the first gateway stays registered")

	// A refused request for a different gateway leaves nothing registered.
	local.Set(keyManagedGatewayName, "syn-gw-2")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)
	require.Len(t, bodies, 3)

	// A repeat that succeeds again is one gateway, registered once.
	status = http.StatusCreated
	local.Set(keyManagedGatewayName, "syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)

	// Once the gateway was deleted and deregistered, creating the name again registers it again.
	registry.Deregister(cleanup.KindGateway, "dev-syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)
}

func TestAttemptSpecs(t *testing.T) {
	specs, values, err := attemptSpecs(valuesTable(t,
		[]string{"path", "value", "message"},
		[]string{"a.b", "5", " must be low "},
		[]string{"a.b", `"5"`, ""}))
	require.NoError(t, err)
	require.Equal(t, []configAttempt{{Path: "a.b", Want: "must be low"}, {Path: "a.b"}}, specs)
	require.Equal(t, []any{float64(5), "5"}, values)

	specs, _, err = attemptSpecs(valuesTable(t, []string{"path", "value"}, []string{"a", "1"}))
	require.NoError(t, err)
	require.Equal(t, []configAttempt{{Path: "a"}}, specs)

	for name, table := range map[string]*godog.Table{
		"no table":     nil,
		"header only":  valuesTable(t, []string{"path", "value"}),
		"wrong header": valuesTable(t, []string{"key", "value"}, []string{"a", "1"}),
		"third header": valuesTable(t, []string{"path", "value", "why"}, []string{"a", "1", "x"}),
		"short row":    valuesTable(t, []string{"path", "value", "message"}, []string{"a", "1"}),
		"no path":      valuesTable(t, []string{"path", "value"}, []string{" ", "1"}),
	} {
		_, _, err := attemptSpecs(table)
		require.Error(t, err, name)
	}
}

func TestJudgeRejection(t *testing.T) {
	good := configAttempt{Path: "a.b", Status: 400, Code: "invalid_request", Message: "a.b must be low", Want: "must be low"}
	require.Empty(t, judgeRejection(good))
	for name, mutate := range map[string]func(*configAttempt){
		"accepted":       func(a *configAttempt) { a.Status = 200 },
		"server fault":   func(a *configAttempt) { a.Status = 500 },
		"wrong code":     func(a *configAttempt) { a.Code = "conflict" },
		"unnamed":        func(a *configAttempt) { a.Message = "must be low" },
		"other message":  func(a *configAttempt) { a.Message = "a.b must be high" },
		"empty response": func(a *configAttempt) { *a = configAttempt{Path: "a.b", Status: 400} },
	} {
		attempt := good
		mutate(&attempt)
		require.NotEmpty(t, judgeRejection(attempt), name)
	}
	good.Want = ""
	good.Message = "a.b is wrong"
	require.Empty(t, judgeRejection(good), "no expected fragment means any message that names the setting")
}

func TestValidateConfigurationDocument(t *testing.T) {
	encode := func(document map[string]any) []byte {
		raw, err := json.Marshal(document)
		require.NoError(t, err)
		return raw
	}
	require.NoError(t, validateConfigurationDocument(encode(configurationBody()), "dev-gw"))
	require.ErrorContains(t, validateConfigurationDocument(encode(configurationBody()), "other"), "want \"other\"")
	require.Error(t, validateConfigurationDocument([]byte("nope"), "dev-gw"))

	for name, mutate := range map[string]func(map[string]any){
		"no environment": func(d map[string]any) { d["environment"] = "" },
		"no values":      func(d map[string]any) { delete(d, "values") },
		"unknown phase":  func(d map[string]any) { d["status"] = map[string]any{"phase": "ready"} },
		"bad timestamp": func(d map[string]any) {
			d["status"] = map[string]any{"phase": "healthy", "lastTransitionTime": "yesterday"}
		},
		"no editable":        func(d map[string]any) { d["editable"] = []any{} },
		"value not editable": func(d map[string]any) { d["values"] = map[string]any{"gateway.hidden": 1} },
		"null value":         func(d map[string]any) { d["values"] = map[string]any{"gateway.config.router.access_logs.enabled": nil} },
		"duplicate path":     func(d map[string]any) { d["editable"] = append(d["editable"].([]any), d["editable"].([]any)[0]) },
		"unknown constraint": func(d map[string]any) { d["constraints"] = []any{map[string]any{"type": "equal"}} },
		"constraint off list": func(d map[string]any) {
			d["constraints"] = []any{map[string]any{"type": "notGreaterThan", "path": "x", "than": "y"}}
		},
		"unknown setting type": func(d map[string]any) { d["editable"] = []any{map[string]any{"path": "p", "type": "float"}} },
		"enum without values":  func(d map[string]any) { d["editable"] = []any{map[string]any{"path": "p", "type": "enum"}} },
		"integer without max": func(d map[string]any) {
			d["editable"] = []any{map[string]any{"path": "p", "type": "integer", "min": "1"}}
		},
		"string without max":    func(d map[string]any) { d["editable"] = []any{map[string]any{"path": "p", "type": "string"}} },
		"editable without path": func(d map[string]any) { d["editable"] = []any{map[string]any{"type": "boolean"}} },
	} {
		document := configurationBody()
		mutate(document)
		require.Error(t, validateConfigurationDocument(encode(document), "dev-gw"), name)
	}
}

func TestReadAndValidateGatewayConfiguration(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways/dev-gw/configuration", r.URL.Path)
		writeJSON(w, status, configurationBody())
	})
	require.Error(t, steps.readGatewayConfiguration(ctx), "no active gateway was found")
	local.Set(keyGatewayID, "dev-gw")
	require.NoError(t, steps.readGatewayConfiguration(ctx))
	require.NoError(t, steps.assertConfigurationDocument(ctx))

	local.Set(keyConfigGatewayID, "another")
	require.ErrorContains(t, steps.assertConfigurationDocument(ctx), "another")

	status = http.StatusNotFound
	require.NoError(t, steps.readGatewayConfiguration(ctx))
	require.Error(t, steps.assertConfigurationDocument(ctx))
}

func TestAwaitConfigurationReadable(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var reads int
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		reads++
		if reads < 2 {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
			return
		}
		writeJSON(w, http.StatusOK, configurationBody())
	})
	require.Error(t, steps.awaitConfigurationReadable(ctx), "no active gateway was found")
	local.Set(keyGatewayID, "dev-gw")
	require.NoError(t, steps.awaitConfigurationReadable(ctx))
	require.Equal(t, 2, reads, "a gateway whose release does not exist yet is waited for")
}

func TestWriteLongString(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var bodies []map[string]map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "no"})
	})
	require.Error(t, steps.writeLongString(ctx, "gateway.config_toml", 8193), "no active gateway was found")
	local.Set(keyGatewayID, "dev-gw")
	require.NoError(t, steps.writeLongString(ctx, "gateway.config_toml", 8193))
	require.Len(t, bodies[0]["values"]["gateway.config_toml"], 8193)
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusBadRequest))
	require.Error(t, steps.writeLongString(ctx, "gateway.config_toml", 0))
}

func TestWriteEachValueRecordsEveryAnswer(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var sent []map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/managed-gateways/dev-gw/configuration", r.URL.Path)
		var body map[string]map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		sent = append(sent, body["values"])
		if _, accepted := body["values"]["gateway.ok"]; accepted {
			writeJSON(w, http.StatusOK, configurationBody())
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "gateway.bad must be low"})
	})
	local.Set(keyGatewayID, "dev-gw")
	table := valuesTable(t, []string{"path", "value", "message"}, []string{"gateway.bad", "5", "must be low"}, []string{"gateway.bad", `"5"`, ""})
	require.NoError(t, steps.writeEachValue(ctx, table))
	require.Equal(t, []map[string]any{{"gateway.bad": float64(5)}, {"gateway.bad": "5"}}, sent, "each value is a write of its own")
	require.NoError(t, steps.assertEveryWriteRejected(ctx))

	table = valuesTable(t, []string{"path", "value"}, []string{"gateway.bad", "1"}, []string{"gateway.ok", "1"})
	require.NoError(t, steps.writeEachValue(ctx, table))
	err := steps.assertEveryWriteRejected(ctx)
	require.ErrorContains(t, err, "1 of 2 writes")
	require.ErrorContains(t, err, "gateway.ok: answered 200")

	local.Set(keyConfigAttempts, "")
	require.ErrorContains(t, steps.assertEveryWriteRejected(ctx), "no gateway configuration writes")
	require.Error(t, steps.writeEachValue(ctx, valuesTable(t, []string{"path"})))
}

func TestSecretReferenceEscapesTheQuotesForJSON(t *testing.T) {
	reference := secretReference("syn-sec-1")
	require.Equal(t, `{{ secret \"syn-sec-1\" }}`, reference)
	var decoded string
	require.NoError(t, json.Unmarshal([]byte(`"`+reference+`"`), &decoded))
	require.Equal(t, `{{ secret "syn-sec-1" }}`, decoded)
}

func TestSecretFormIsMultipart(t *testing.T) {
	body, contentType, err := secretForm([][2]string{{"id", "syn-sec-1"}, {"value", "sample"}})
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	reader := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	var names []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, part.FormName())
	}
	require.Equal(t, []string{"id", "value"}, names)
}

func TestCreateSecretDeregistersARefusedSecret(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "SERVICE_UNAVAILABLE"})
	})
	require.ErrorContains(t, steps.createSecret(ctx), "503")
	require.Empty(t, registry.Pending())
	require.Empty(t, optionalValue(ctx, keySecretValue))
}

func TestCreateSecretFollowsAnIDThePlatformChanged(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"id": "platform-chosen"})
	})
	require.NoError(t, steps.createSecret(ctx))
	require.Equal(t, "platform-chosen", optionalValue(ctx, keySecretID))
	require.Equal(t, secretReference("platform-chosen"), optionalValue(ctx, keySecretReference))
	pending := registry.Pending()
	require.Len(t, pending, 1)
	require.Equal(t, "platform-chosen", pending[0].ID)
}

func TestCreateSecretKeepsTheValueInScenarioStateAndRegistersCleanup(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	stub := &secretStub{}
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		stub.record(t, r)
		writeJSON(w, http.StatusCreated, map[string]any{"id": stub.fields["id"], "displayName": stub.fields["displayName"]})
	})

	require.NoError(t, steps.createSecret(ctx))
	require.Equal(t, http.MethodPost, stub.method)
	require.Equal(t, "/secrets", stub.path)
	handle, err := requiredValue(ctx, keySecretID)
	require.NoError(t, err)
	require.Equal(t, handle, stub.fields["id"])
	require.LessOrEqual(t, len(handle), 40, "a secret handle is at most 40 characters")
	value, err := requiredValue(ctx, keySecretValue)
	require.NoError(t, err)
	require.Equal(t, value, stub.fields["value"])
	require.Equal(t, secretReference(handle), optionalValue(ctx, keySecretReference))
	require.Equal(t, "GENERIC", stub.fields["type"])
	require.Equal(t, []cleanup.Resource{{Kind: kindSecret, ID: handle, Actor: resourceActor, Description: "secret " + handle}},
		registry.Pending())
}

func TestRotateSecretKeepsThePreviousValue(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	stub := &secretStub{}
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		stub.record(t, r)
		writeJSON(w, status, map[string]any{"id": "syn-sec-1"})
	})
	require.ErrorContains(t, steps.rotateSecret(ctx), keySecretID)
	local.Set(keySecretID, "syn-sec-1")
	local.Set(keySecretValue, "sample-original")

	require.NoError(t, steps.rotateSecret(ctx))
	require.Equal(t, http.MethodPut, stub.method)
	require.Equal(t, "/secrets/syn-sec-1", stub.path)
	require.Equal(t, "sample-original", optionalValue(ctx, keySecretPreviousValue))
	rotated := optionalValue(ctx, keySecretValue)
	require.NotEqual(t, "sample-original", rotated)
	require.Equal(t, rotated, stub.fields["value"])

	status = http.StatusNotFound
	require.Error(t, steps.rotateSecret(ctx))
	require.Equal(t, rotated, optionalValue(ctx, keySecretValue), "a refused rotation keeps the value")
}

func TestReadAndListSecretsRequireSuccess(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var paths []string
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		writeJSON(w, status, map[string]any{"id": "syn-sec-1"})
	})
	require.Error(t, steps.readSecret(ctx), "no secret has been chosen")
	local.Set(keySecretID, "syn-sec-1")
	require.NoError(t, steps.readSecret(ctx))
	require.NoError(t, steps.listSecrets(ctx))
	require.Equal(t, []string{"/secrets/syn-sec-1", "/secrets"}, paths)
	status = http.StatusNotFound
	require.Error(t, steps.readSecret(ctx))
	require.Error(t, steps.listSecrets(ctx))
}

func TestDeleteSecretDeregistersOnlyADeletedSecret(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusConflict
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/secrets/syn-sec-1", r.URL.Path)
		if status == http.StatusNoContent {
			w.WriteHeader(status)
			return
		}
		writeJSON(w, status, map[string]string{"code": "SECRET_IN_USE"})
	})
	local.Set(keySecretID, "syn-sec-1")
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: kindSecret, ID: "syn-sec-1", Actor: resourceActor}))

	require.NoError(t, steps.deleteSecret(ctx), "a refusal is for the scenario to assert on")
	require.Len(t, registry.Pending(), 1, "a refused delete leaves the cleanup in place")
	response, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, response.StatusCode)

	status = http.StatusNoContent
	require.NoError(t, steps.deleteSecret(ctx))
	require.Empty(t, registry.Pending())
}

func TestAssertSecretHash(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySecretValue, "sample-secret-1")

	publishedResponse(t, steps, ctx, 200, `{"id":"s","hash":"hmac-sha256:0123"}`)
	require.NoError(t, steps.assertSecretHash(ctx))
	publishedResponse(t, steps, ctx, 200, `{"id":"s"}`)
	require.ErrorContains(t, steps.assertSecretHash(ctx), "no hash")
	publishedResponse(t, steps, ctx, 200, `{"id":"s","hash":"sample-secret-1"}`)
	require.ErrorContains(t, steps.assertSecretHash(ctx), "reveals its value")
}

func TestAssertSecretValueHidden(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySecretValue, "sample-secret-1")

	publishedResponse(t, steps, ctx, 200, `{"id":"syn-sec-1","hash":"hmac-sha256:abc"}`)
	require.NoError(t, steps.assertSecretValueHidden(ctx))
	publishedResponse(t, steps, ctx, 200, `{"list":[{"id":"a"},{"id":"b"}]}`)
	require.NoError(t, steps.assertSecretValueHidden(ctx))

	publishedResponse(t, steps, ctx, 200, `{"id":"syn-sec-1","note":"sample-secret-1"}`)
	err := steps.assertSecretValueHidden(ctx)
	require.ErrorContains(t, err, "contains the secret's value")
	require.NotContains(t, err.Error(), "sample-secret-1", "a leak is not repeated into the report")

	publishedResponse(t, steps, ctx, 200, `{"list":[{"id":"a","value":"x"}]}`)
	require.ErrorContains(t, steps.assertSecretValueHidden(ctx), `"value" field`)
}

func TestEchoedHeaderIgnoresCaseAndJoinsRepeats(t *testing.T) {
	value, ok := echoedHeader([]byte(`{"headers":{"X-Apip-Secret-Probe":"v1","Host":"h"}}`), "x-apip-secret-probe")
	require.True(t, ok)
	require.Equal(t, "v1", value)
	value, ok = echoedHeader([]byte(`{"headers":{"X-A":["a","b"]}}`), "x-a")
	require.True(t, ok)
	require.Equal(t, "a,b", value)
	for _, body := range []string{`{"headers":{}}`, `not json`, `{"headers":{"X-A":1}}`} {
		_, ok = echoedHeader([]byte(body), "x-a")
		require.False(t, ok, body)
	}
}

func TestClassifyEcho(t *testing.T) {
	echo := func(status int, value string) *httpx.Response {
		return &httpx.Response{StatusCode: status, Body: []byte(`{"headers":{"X-Probe":"` + value + `"}}`)}
	}
	require.Equal(t, echoCurrent, classifyEcho(echo(200, "new"), "x-probe", "new", "old"))
	require.Equal(t, echoPrevious, classifyEcho(echo(200, "old"), "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(echo(200, "else"), "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(echo(503, "new"), "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(nil, "x-probe", "new", "old"))
	require.Equal(t, echoAbsent, classifyEcho(&httpx.Response{StatusCode: 200, Body: []byte(`{"headers":{}}`)}, "x-probe", "new", "old"))
	require.Equal(t, echoOther, classifyEcho(echo(200, ""), "x-probe", "new", ""), "an empty previous value matches nothing")
}

func TestAssertEchoedSecret(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySecretValue, "sample-secret-1")
	publishedResponse(t, steps, ctx, 200, `{"headers":{"X-Probe":"sample-secret-1"}}`)
	require.NoError(t, steps.assertEchoedSecret(ctx, "x-probe"))
	publishedResponse(t, steps, ctx, 200, `{"headers":{}}`)
	require.ErrorContains(t, steps.assertEchoedSecret(ctx, "x-probe"), "no \"x-probe\" header")
	publishedResponse(t, steps, ctx, 200, `{"headers":{"X-Probe":"another"}}`)
	err := steps.assertEchoedSecret(ctx, "x-probe")
	require.ErrorContains(t, err, "not the secret's value")
	require.NotContains(t, err.Error(), "another")
}

func TestSendUntilEchoesRotatedWaitsForTheNewValue(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()
		value := "old"
		if attempt > 1 {
			value = "new"
		}
		writeJSON(w, http.StatusOK, map[string]any{"headers": map[string]string{"X-Probe": value}})
	}))
	defer server.Close()
	steps, ctx, local := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})
	steps.funnel = httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, InsecureSkipVerify: true}), 0, 0)
	steps.client = steps.funnel.Client()
	local.Set(keySecretValue, "new")

	require.ErrorContains(t, steps.sendUntilEchoesRotated(ctx, "GET", server.URL+"/echo", "x-probe"), "not been rotated")
	local.Set(keySecretPreviousValue, "old")
	require.NoError(t, steps.sendUntilEchoesRotated(ctx, "get", server.URL+"/echo", "x-probe"))
	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, calls, 2, "the first answer still carried the old value")
}

func TestCreateAPIInSyntheticProjectRefusesTheDefaultProject(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request may be sent")
	})
	require.ErrorContains(t, steps.createAPIInSyntheticProject(ctx, nil), "no synthetic project")
	local.Set(keyProjectID, "default")
	require.ErrorContains(t, steps.createAPIInSyntheticProject(ctx, nil), `"default"`)
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

func configurationBody() map[string]any {
	return map[string]any{
		"id": "dev-gw", "name": "gw", "environment": "dev",
		"values": map[string]any{"gateway.config.controller.logging.level": "debug"},
		"editable": []any{
			map[string]any{"path": "gateway.config.controller.logging.level", "type": "enum", "values": []any{"debug", "info"}},
			map[string]any{"path": "gateway.controller.deployment.replicaCount", "type": "integer", "min": "1", "max": "5"},
			map[string]any{"path": "gateway.controller.deployment.resources.requests.cpu", "type": "quantity", "min": "50m", "max": "4"},
			map[string]any{"path": "gateway.controller.deployment.resources.limits.cpu", "type": "quantity", "min": "50m", "max": "4"},
			map[string]any{"path": "gateway.config.router.access_logs.enabled", "type": "boolean"},
			map[string]any{"path": "gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval", "type": "duration", "min": "30s", "max": "1h"},
			map[string]any{"path": "gateway.config_toml", "type": "string", "max": "8192"},
		},
		"constraints": []any{map[string]any{
			"type": "notGreaterThan", "path": "gateway.controller.deployment.resources.requests.cpu",
			"than": "gateway.controller.deployment.resources.limits.cpu",
		}},
		"status": map[string]any{"phase": "healthy", "lastTransitionTime": "2026-09-03T08:12:34Z"},
	}
}

// dataPlaneSteps returns steps whose data-plane requests may go to the given local server.
func dataPlaneSteps(t *testing.T, controlPlane http.HandlerFunc) (*Steps, context.Context, *tcontext.Local) {
	t.Helper()
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, controlPlane)
	steps.dataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	return steps, ctx, local
}

// secretStub records the multipart secrets requests it receives and answers like the platform.
type secretStub struct {
	mu     sync.Mutex
	method string
	path   string
	fields map[string]string
}

func (c *secretStub) record(t *testing.T, r *http.Request) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(r.Body, params["boundary"])
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		raw, err := io.ReadAll(part)
		require.NoError(t, err)
		fields[part.FormName()] = string(raw)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.method, c.path, c.fields = r.Method, r.URL.Path, fields
}

func stagesStub(t *testing.T, stages string) (*Steps, context.Context, *tcontext.Local) {
	t.Helper()
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/projects/syn-project-1/apis/syn-api-1/deployments", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stages))
	})
	local.Set(keyProjectID, "syn-project-1")
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keySyntheticEnvironments, "dev,env-1,env-2")
	local.Set(numberedKey(keySyntheticGatewayPrefix, 1), "gw-dev")
	local.Set(numberedKey(keySyntheticGatewayPrefix, 2), "gw-1")
	return steps, ctx, local
}

const promotionStages = `{"list":[
	{"environment":"dev","gateways":[{"gatewayId":"gw-dev","status":"DEPLOYED","buildId":"2026-01-31-1"}]},
	{"environment":"env-1","gateways":[{"gatewayId":"gw-1","status":"NOT_DEPLOYED"}]},
	{"environment":"env-2","gateways":[]}]}`

func TestResolveCloudDataPlaneURLApprovesLocalhostOnlyWhenEnabled(t *testing.T) {
	local := []string{
		"http://gw-development.openchoreoapis.localhost:19080/api",
		"https://gw-development.openchoreoapis.localhost:19443/api",
		"http://gw.localhost/api",
	}
	t.Run("off by default", func(t *testing.T) {
		for _, target := range local {
			_, err := resolveCloudDataPlaneURL(context.Background(), target)
			require.Error(t, err, target)
		}
	})
	t.Run("a false value leaves it off", func(t *testing.T) {
		t.Setenv(cloudLocalDataPlaneEnv, "false")
		_, err := resolveCloudDataPlaneURL(context.Background(), local[0])
		require.Error(t, err)
	})
	t.Run("on approves named localhost hosts", func(t *testing.T) {
		t.Setenv(cloudLocalDataPlaneEnv, "true")
		for _, target := range local {
			got, err := resolveCloudDataPlaneURL(context.Background(), target)
			require.NoError(t, err, target)
			require.Equal(t, target, got)
		}
	})
	t.Run("a local URL without a port goes to the configured data-plane port", func(t *testing.T) {
		t.Setenv(cloudLocalDataPlaneEnv, "true")
		t.Setenv(cloudLocalDataPlanePortEnv, "19080")
		got, err := resolveCloudDataPlaneURL(context.Background(), "https://abc-gw-p1.gateway.openchoreoapis.localhost/api")
		require.NoError(t, err)
		require.Equal(t, "http://abc-gw-p1.gateway.openchoreoapis.localhost:19080/api", got)
		got, err = resolveCloudDataPlaneURL(context.Background(), local[1])
		require.NoError(t, err)
		require.Equal(t, local[1], got, "a URL that names its port is left alone")
		t.Setenv(cloudLocalDataPlanePortEnv, "not-a-port")
		got, err = resolveCloudDataPlaneURL(context.Background(), local[2])
		require.NoError(t, err)
		require.Equal(t, local[2], got, "an invalid port is ignored")
	})
	t.Run("on widens nothing else", func(t *testing.T) {
		t.Setenv(cloudLocalDataPlaneEnv, "true")
		for _, target := range []string{
			"http://127.0.0.1:19080/api", "http://[::1]:19080/api", "http://localhost:19080/api",
			"http://gateway.example.com/api", "http://gw.localhost.evil.example/api",
			"http://user:password@gw.localhost/api", "ftp://gw.localhost/api", "gw.localhost/api",
			"http://synthetic-wso2cloud.gateway.development.example/api",
		} {
			_, err := resolveCloudDataPlaneURL(context.Background(), target)
			require.Error(t, err, target)
		}
		_, err := resolveCloudDataPlaneURL(context.Background(), "https://synthetic-wso2cloud.gateway.wso2.com/api")
		require.NoError(t, err, "the cloud hosts stay approved")
	})
}

// starterPlatform is a fake organisation for the starter: a default project bound to a pipeline,
// a list of managed gateways, and a record of the gateways it was asked to create.
type starterPlatform struct {
	mu        sync.Mutex
	paths     []promotionPath
	gateways  []map[string]any
	created   []map[string]any
	createAs  int
	polls     int
	activeAt  int
	failsWith string
	// absent makes a gateway lookup answer 404, as when a refused create left nothing behind.
	absent bool
}

func (p *starterPlatform) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		require.Equal(t, "Bearer console-token", r.Header.Get("Authorization"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects/default/deployment-pipeline":
			_ = json.NewEncoder(w).Encode(map[string]any{"pipeline": "default"})
		case r.Method == http.MethodGet && r.URL.Path == "/pipelines":
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
				{"name": "other", "promotionPaths": []promotionPath{{SourceEnvironment: "elsewhere"}}},
				{"name": "default", "promotionPaths": p.paths},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/managed-gateways":
			if kind := r.URL.Query().Get("functionalityType"); kind != "" {
				require.Equal(t, regularGatewayKind, kind)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"list": p.gateways})
		case r.Method == http.MethodPost && r.URL.Path == "/managed-gateways":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			p.created = append(p.created, body)
			if p.createAs != 0 {
				w.WriteHeader(p.createAs)
				_, _ = w.Write([]byte(`{"error":"conflict"}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": expectedGatewayID(body["environment"].(string), body["displayName"].(string), ""), "status": "PROVISIONING"})
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
		case r.Method == http.MethodGet && slices.Contains([]string{"/projects", "/environments", "/secrets", "/rest-apis", "/managed-api-portals"}, r.URL.Path):
			// What the finalizer reads: this organisation holds none of it.
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotImplemented)
		}
	}
}

func (p *starterPlatform) api(t *testing.T) organisationAPI {
	server := httptest.NewServer(p.handler(t))
	t.Cleanup(server.Close)
	return organisationAPI{client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), base: server.URL, token: "console-token", pollInterval: retry.BaseInterval}
}

// testDefaultGateway is the display name a starter with the suffix "abcde234" gives its gateway.
const testDefaultGateway = defaultGatewayDisplayName + "-abcde234"

func ensureForTest(t *testing.T, platform *starterPlatform) (gatewayOutcome, error) {
	return ensureDefaultGateway(context.Background(), platform.api(t), testDefaultGateway, slog.Default())
}

func developmentPipeline() []promotionPath {
	return []promotionPath{{SourceEnvironment: "development", TargetEnvironments: []string{"production"}}}
}

func TestEnsureDefaultGatewayLeavesAnActiveGatewayAlone(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "ai-gw", "functionalityType": "ai", "environment": "development", "status": "ACTIVE", "isActive": true},
		{"id": "sgw", "displayName": "sgw-abcde234-1", "environment": "development", "status": "ACTIVE", "isActive": true},
		{"id": "elsewhere-gw", "environment": "elsewhere", "status": "ACTIVE", "isActive": true},
		{"id": "development-default", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "development-default"}, outcome, "an existing gateway is used, not created")
	require.Empty(t, platform.created)
}

func TestEnsureDefaultGatewayCreatesOneWhenNoneIsUsable(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 2, gateways: []map[string]any{
		{"id": "sgw", "displayName": "sgw-abcde234-1", "environment": "development", "status": "ACTIVE", "isActive": true},
		{"id": "stopped", "environment": "development", "status": "DISCONNECTED", "isActive": false},
	}}
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "development-" + testDefaultGateway, Created: true}, outcome)
	require.Len(t, platform.created, 1)
	require.Equal(t, map[string]any{
		"displayName": testDefaultGateway, "environment": "development", "functionalityType": regularGatewayKind,
		"description": platform.created[0]["description"],
	}, platform.created[0])
	require.False(t, notSyntheticGatewayRejects(testDefaultGateway), "the scenarios must be able to choose the default gateway")
	require.GreaterOrEqual(t, platform.polls, 2, "it waits for the gateway to become active")
}

func notSyntheticGatewayRejects(displayName string) bool {
	return notSyntheticGateway(map[string]any{"displayName": displayName}) != ""
}

func TestEnsureDefaultGatewayWaitsForAGatewayThatIsBeingBuilt(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 2, gateways: []map[string]any{
		{"id": "development-" + testDefaultGateway, "environment": "development", "status": "PROVISIONING"},
	}}
	_, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Empty(t, platform.created, "a gateway that is being built is not asked for again")
}

func TestEnsureDefaultGatewayReportsAFailedGateway(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), failsWith: "no capacity"}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "no capacity")
	require.ErrorContains(t, err, "development-"+testDefaultGateway)
}

func TestEnsureDefaultGatewayWaitsForTheGatewayAConflictNames(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusConflict}
	_, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Len(t, platform.created, 1)
}

func TestEnsureDefaultGatewayRefusesOtherAnswers(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusForbidden}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "creating the default gateway")
}

func TestEnsureDefaultGatewayNeverCreatesTheGatewayAPinNames(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "wanted")
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "other", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, cloudGatewayPinEnv)
	require.Empty(t, platform.created)

	platform.gateways = append(platform.gateways, map[string]any{"id": "wanted", "environment": "development", "status": "ACTIVE", "isActive": true})
	outcome, err := ensureForTest(t, platform)
	require.NoError(t, err)
	require.Equal(t, gatewayOutcome{ID: "wanted"}, outcome)
}

func TestEnsureDefaultGatewayNeedsAPipeline(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "no environment")
}

func TestDefaultGatewayEnvironmentPrefersDevelopmentThenTheFirstStage(t *testing.T) {
	for name, tc := range map[string]struct {
		paths []promotionPath
		want  string
		fails bool
	}{
		"development is reached": {developmentPipeline(), "development", false},
		"development is a target": {[]promotionPath{{SourceEnvironment: "dev-a", TargetEnvironments: []string{"development"}}},
			"development", false},
		"the first stage in name order": {[]promotionPath{
			{SourceEnvironment: "zeta", TargetEnvironments: []string{"prod"}},
			{SourceEnvironment: "alpha", TargetEnvironments: []string{"prod"}},
		}, "alpha", false},
		"a stage that is a target of another is not first": {[]promotionPath{
			{SourceEnvironment: "qa", TargetEnvironments: []string{"prod"}},
			{SourceEnvironment: "build", TargetEnvironments: []string{"qa"}},
		}, "build", false},
		"no paths":       {nil, "", true},
		"a blank source": {[]promotionPath{{TargetEnvironments: []string{"prod"}}}, "", true},
		"a cycle":        {[]promotionPath{{SourceEnvironment: "a", TargetEnvironments: []string{"b"}}, {SourceEnvironment: "b", TargetEnvironments: []string{"a"}}}, "", true},
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
	t.Setenv(cloudConsoleAccessTokenEnv, "")
	topo := cloudAPIStub(t, func(http.ResponseWriter, *http.Request) {})
	starter, err := NewStarter(nil)
	require.NoError(t, err)
	require.ErrorContains(t, starter.Start(context.Background(), topo), cloudConsoleAccessTokenEnv)
}

func TestStarterStartPreparesTheOrganisationOnce(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "development-default", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	var lists int
	handler := platform.handler(t)
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" && r.Method == http.MethodGet {
			lists++
		}
		handler(w, r)
	})
	starter, err := NewStarter(nil)
	require.NoError(t, err)
	require.NoError(t, starter.Start(context.Background(), topo))
	require.NoError(t, starter.Start(context.Background(), topo))
	require.Equal(t, 1, lists, "the second runner shares the first one's preparation")
}

func TestFinalizerRemovesTheStartersGatewayAfterTheRunnersGateways(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	stub := newLeftoverStub()
	stub.lists["/managed-gateways"] = []map[string]any{
		{"id": "development-" + testDefaultGateway, "displayName": testDefaultGateway, "environment": "development"},
		{"id": "development-syn-gw-abcde234-1", "environment": "development"},
		{"id": "development-" + defaultGatewayDisplayName + "-zzzzz222", "displayName": defaultGatewayDisplayName + "-zzzzz222"},
		{"id": "development-default", "environment": "development"},
	}
	topo := cloudAPIStub(t, stub.handler)
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

func TestStarterHandsTheFinalizerWhatItWillCreate(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), activeAt: 1}
	topo := cloudAPIStub(t, platform.handler(t))
	finalizer := NewFinalizer()
	starter, err := NewStarter(finalizer)
	require.NoError(t, err)

	require.NoError(t, starter.Start(context.Background(), topo))

	require.Len(t, platform.created, 1)
	require.Equal(t, starter.gatewayDisplayName(), platform.created[0]["displayName"])
	require.True(t, finalizer.finals.Tracked())
	require.True(t, finalizer.finals.Owns([]string{"development-" + starter.gatewayDisplayName()}),
		"the finalizer must recognise the gateway the starter created")
	require.False(t, finalizer.finals.Owns([]string{"development-" + defaultGatewayDisplayName + "-zzzzz222"}))
	finalizer.mu.Lock()
	defer finalizer.mu.Unlock()
	require.Len(t, finalizer.bases, 1, "the platform is known to the finalizer even if no scenario ran")
}

func TestStarterGatewayNameFitsTheHandleLimitOfTheDevelopmentEnvironment(t *testing.T) {
	starter, err := NewStarter(nil)
	require.NoError(t, err)
	const developmentHandleLimit = 15 // the platform allows 37 minus twice the environment name's length
	require.LessOrEqual(t, len(starter.gatewayDisplayName()), developmentHandleLimit)
	require.True(t, isStarterGateway([]string{"development-" + starter.gatewayDisplayName(), starter.gatewayDisplayName()}))
	require.False(t, isStarterGateway([]string{"development-default", "development-syn-gw-abcde234-1", "mydgw-abcde234"}))
}

func TestAGatewayTheOrganisationAlreadyHadIsUsedAndSurvivesTheFinalizer(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), gateways: []map[string]any{
		{"id": "development-default", "displayName": "default", "environment": "development", "status": "ACTIVE", "isActive": true},
	}}
	var mu sync.Mutex
	var deletes []string
	handler := platform.handler(t)
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
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
	starter, err := NewStarter(finalizer)
	require.NoError(t, err)

	require.NoError(t, starter.StartAt(context.Background(), base))
	id, created := starter.Gateway()
	require.Equal(t, "development-default", id)
	require.False(t, created)
	require.Empty(t, platform.created)

	removed, err := finalizer.Finalize(context.Background())
	require.NoError(t, err)
	require.Empty(t, removed)
	require.Empty(t, deletes, "nothing the run did not create is deleted")
}

func TestAGatewayTheStarterCreatesGetsAtLeastFiveMinutesToBecomeActive(t *testing.T) {
	require.GreaterOrEqual(t, gatewayReadyTimeout, 5*time.Minute)
	require.Less(t, gatewayPollInterval, gatewayReadyTimeout)
}

func TestEnsureDefaultGatewayDoesNotWaitForAGatewayAConflictLeftNowhere(t *testing.T) {
	t.Setenv(cloudGatewayPinEnv, "")
	platform := &starterPlatform{paths: developmentPipeline(), createAs: http.StatusConflict, absent: true}
	_, err := ensureForTest(t, platform)
	require.ErrorContains(t, err, "creating the default gateway")
	require.ErrorContains(t, err, "409")
}

func TestFeatureStepsAreAllDefined(t *testing.T) {
	var out bytes.Buffer
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) { Register(sc, nil, &httpx.Funnel{}) },
		Options:             &godog.Options{Format: "progress", Paths: []string{"../../features"}, NoColors: true, Output: &out},
	}
	suite.Run()
	text := out.String()
	require.NotContains(t, text, "undefined", "a step line of a feature file has no definition:\n"+text)
	require.NotContains(t, text, "ambiguous", "a step line of a feature file matches two definitions:\n"+text)
}

func TestHandleBudgetOfReadsThePlatformsRefusal(t *testing.T) {
	budget, err := handleBudgetOf([]byte(`{"message":"The gateway name is too long for the \"e-abcdefgh-1\" environment: its handle \"aaaa\" must be 13 characters or fewer."}`))
	require.NoError(t, err)
	require.Equal(t, 13, budget)
	for _, body := range []string{"", "no limit here", "must be 0 characters or fewer"} {
		_, err := handleBudgetOf([]byte(body))
		require.Error(t, err, body)
	}
}

func TestHandleOfLengthKeepsTheTailAndPadsShortNames(t *testing.T) {
	require.Equal(t, "sg-abcde234-1xx", handleOfLength("sg-abcde234-1", 15), "a short name is padded with a letter")
	require.Equal(t, "abcde234-12", handleOfLength("sg-abcde234-12", 11), "a long name keeps its tail, which is what makes it unique")
	require.Equal(t, "axxxxxxx-12", handleOfLength("-xxxxxxx-12", 11), "a handle never starts with a dash")
	require.Empty(t, handleOfLength("name", 0))
}

func TestSyntheticNamesAreRecognised(t *testing.T) {
	for _, name := range []string{"sgw-abcde234-1", "sg-abcde234-12", "sg-abcde234-1xx", "abcde234-12", "gw-abcde234-1"} {
		require.NotEmpty(t, notSyntheticGateway(map[string]any{"displayName": name}), name)
	}
	for _, name := range []string{"Default", "dgw-abcde234", "production-gateway", "sg-abc-1"} {
		require.Empty(t, notSyntheticGateway(map[string]any{"displayName": name}), name)
	}
	for _, environment := range []string{"e-abcde234-1", "syn-env-abcde234-12"} {
		require.NotEmpty(t, notInSyntheticEnvironment(map[string]any{"environment": environment}), environment)
	}
	for _, environment := range []string{"development", "production", "e-short-1"} {
		require.Empty(t, notInSyntheticEnvironment(map[string]any{"environment": environment}), environment)
	}
}

func TestGatewayRequestAcceptedTakesCreatedAndAccepted(t *testing.T) {
	require.True(t, gatewayRequestAccepted(http.StatusCreated))
	require.True(t, gatewayRequestAccepted(http.StatusAccepted))
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusConflict, http.StatusInternalServerError} {
		require.False(t, gatewayRequestAccepted(status), status)
	}
}

func TestRunnerGatewayIsRegisteredBeforeTheRequestAndNumbered(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var pending func() []cleanup.Resource
	registeredBefore := false
	var sent map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways", r.URL.Path)
		registeredBefore = len(pending()) == 1
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeJSON(w, http.StatusAccepted, map[string]any{"id": "development-" + sent["handle"].(string), "status": "PROVISIONING"})
	})
	pending = registry.Pending

	require.NoError(t, steps.requestRunnerManagedGateway(ctx, valuesTable(t,
		[]string{"environment", "development"}, []string{"displayName", "sg-abcde234-1"}, []string{"handle", "sg-abcde234-1"})))
	require.True(t, registeredBefore, "the gateway must be registered for cleanup before it is requested")
	require.NoError(t, steps.assertGatewayRequestAccepted(ctx))

	id, err := requiredValue(ctx, "cloudSyntheticManagedGateway1")
	require.NoError(t, err)
	require.Equal(t, "development-sg-abcde234-1", id)
	require.Equal(t, "development", optionalValue(ctx, "cloudSyntheticManagedGatewayEnvironment1"))

	local.Set(keyManagedGatewayID, "")
	require.NoError(t, steps.selectSyntheticGateway(ctx, 1))
	current, environment, err := currentGateway(ctx)
	require.NoError(t, err)
	require.Equal(t, id, current)
	require.Equal(t, "development", environment)
	require.Equal(t, scenarioGatewayDescription, optionalValue(ctx, keyScenarioGatewayMark))
	require.ErrorContains(t, steps.selectSyntheticGateway(ctx, 2), "does not exist")

	// A runner-owned gateway is swept with the runner, not with the scenario.
	require.NoError(t, cleanup.Sweep(ctx))
	require.Len(t, registry.Pending(), 1)
}

func TestScenarioGatewayRequestAnsweredWithAnErrorIsNoLongerRegistered(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusConflict, map[string]string{"code": "conflict"})
	})
	require.NoError(t, steps.requestManagedGatewayWith(ctx, map[string]string{
		"environment": "development", "displayName": "sg-abcde234-1", "handle": "sg-abcde234-1"}))
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusConflict))
	require.Empty(t, registry.Pending(), "a refused request created nothing to clean up")
	require.Empty(t, syntheticGatewayIDs(ctx))
}

func TestProbeHandleBudgetReadsTheLimitAndCreatesNothing(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var handles []string
	status := http.StatusBadRequest
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var sent map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		handles = append(handles, sent["handle"].(string))
		if status == http.StatusBadRequest {
			writeJSON(w, status, map[string]string{"code": "invalid_request", "message": "its handle must be 15 characters or fewer."})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"id": "development-" + sent["handle"].(string)})
	})
	budget, err := steps.probeHandleBudget(ctx, "development")
	require.NoError(t, err)
	require.Equal(t, 15, budget)
	require.Len(t, handles[0], handleProbeLength)
	require.Empty(t, registry.Pending(), "a refused probe leaves nothing registered")

	status = http.StatusAccepted
	_, err = steps.probeHandleBudget(ctx, "development")
	require.ErrorContains(t, err, "was not refused")
	require.NotEmpty(t, registry.Pending(), "a probe that was accepted created a gateway, which is cleaned up")
}

func TestChooseGatewayNameAtLimitUsesTheTighterEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var sent map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		limit := map[string]int{"development": 15, "e-abcde234-1": 13}[sent["environment"].(string)]
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "must be " + strconv.Itoa(limit) + " characters or fewer."})
	})
	local.Set(keyGatewayEnv, "development")
	local.Set(keyEnvironmentName, "e-abcde234-1")
	require.NoError(t, steps.chooseGatewayNameAtLimit(ctx))
	name, err := requiredValue(ctx, keyManagedGatewayName)
	require.NoError(t, err)
	require.Len(t, name, 13, "the one name has to fit the tighter environment")
	require.Regexp(t, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, name)
	require.NotEmpty(t, notSyntheticGateway(map[string]any{"displayName": name}), "the finalizer and the gateway choice must still recognise it")
}

func TestGatewayEntryFallsBackToTheListingWhileTheGatewayIsUnbound(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	bound := false
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/managed-gateways/gw-1":
			if bound {
				writeJSON(w, http.StatusOK, map[string]any{"id": "gw-1", "status": "DISCONNECTED"})
				return
			}
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
		case "/managed-gateways":
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "gw-1", "status": "PROVISIONING"}}})
		}
	})
	entry, err := steps.gatewayEntry(ctx, "gw-1")
	require.NoError(t, err)
	require.Equal(t, "PROVISIONING", entry["status"])
	bound = true
	entry, err = steps.gatewayEntry(ctx, "gw-1")
	require.NoError(t, err)
	require.Equal(t, "DISCONNECTED", entry["status"])
	_, err = steps.fetchManagedGateway(ctx, "gw-2")
	require.Error(t, err)
}

func TestStatusProgression(t *testing.T) {
	var seen statusProgression
	for _, status := range []string{"PROVISIONING", "PROVISIONING", "DISCONNECTED", "ACTIVE"} {
		seen.add(status)
	}
	require.Equal(t, statusProgression{"PROVISIONING", "DISCONNECTED", "ACTIVE"}, seen)
	require.NoError(t, seen.check())
	require.ErrorContains(t, statusProgression{"PROVISIONING", "FAILED"}.check(), "FAILED")
	require.ErrorContains(t, statusProgression{"DISCONNECTED", "PROVISIONING"}.check(), "went back")
}

func TestFirstReadStatus(t *testing.T) {
	require.NoError(t, firstReadStatus(map[string]any{"id": "gw", "status": "PROVISIONING"}))
	require.ErrorContains(t, firstReadStatus(map[string]any{"id": "gw", "status": ""}), "recorded provisioning state")
	require.ErrorContains(t, firstReadStatus(map[string]any{"id": "gw", "status": "FAILED", "statusReason": "boom"}), "boom")
}

func TestGatewayDocumentProblem(t *testing.T) {
	document := map[string]any{"id": "gw", "environment": "development", "status": "ACTIVE"}
	require.Empty(t, gatewayDocumentProblem(document, "gw", "development"))
	require.Contains(t, gatewayDocumentProblem(document, "other", "development"), "id")
	require.Contains(t, gatewayDocumentProblem(document, "gw", "elsewhere"), "environment")
	require.Contains(t, gatewayDocumentProblem(map[string]any{"id": "gw", "environment": "development", "status": "?"}, "gw", "development"), "status")
	require.Contains(t, gatewayDocumentProblem(map[string]any{"id": "gw", "environment": "development", "status": "ACTIVE", "functionalityType": "ai"},
		"gw", "development"), "functionality type")
}

func TestDefaultGatewayOfAndUpdateBody(t *testing.T) {
	items := []map[string]any{
		{"id": "a", "environment": "development", "isDefault": true},
		{"id": "b", "environment": "development", "isDefault": false},
		{"id": "c", "environment": "development", "isDefault": true, "functionalityType": "ai"},
		{"id": "d", "environment": "other", "isDefault": true},
	}
	id, err := defaultGatewayOf(items, "development", regularGatewayKind)
	require.NoError(t, err)
	require.Equal(t, "a", id)
	_, err = defaultGatewayOf(items, "empty", regularGatewayKind)
	require.ErrorContains(t, err, "no default")
	items[1]["isDefault"] = true
	_, err = defaultGatewayOf(items, "development", regularGatewayKind)
	require.ErrorContains(t, err, "want one")

	body := updateBodyOf(map[string]any{"displayName": "Name", "description": "d", "isCritical": true, "properties": map[string]any{"k": "v"}})
	require.Equal(t, map[string]any{"isDefault": true, "displayName": "Name", "description": "d", "isCritical": true,
		"properties": map[string]any{"k": "v"}}, body)
	require.Equal(t, map[string]any{"isDefault": true}, updateBodyOf(map[string]any{"isCritical": false}))
}

func TestMakeGatewayDefaultRegistersTheReturnBeforeTheChange(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var pending func() []cleanup.Resource
	registeredBefore := false
	var put map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/managed-gateways":
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
				{"id": "development-held", "environment": "development", "isDefault": true},
				{"id": "development-mine", "environment": "development", "isDefault": false}}})
		case r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"id": "development-mine", "displayName": "Mine", "isDefault": false})
		case r.Method == http.MethodPut:
			registeredBefore = len(pending()) == 1
			require.NoError(t, json.NewDecoder(r.Body).Decode(&put))
			writeJSON(w, http.StatusOK, map[string]any{"id": "development-mine", "isDefault": true})
		}
	})
	pending = registry.Pending
	local.Set(keyManagedGatewayID, "development-mine")
	local.Set(keyManagedGatewayEnv, "development")

	require.NoError(t, steps.makeGatewayDefault(ctx))
	require.True(t, registeredBefore, "the return of the default must be registered before it moves")
	require.Equal(t, map[string]any{"isDefault": true, "displayName": "Mine"}, put)
	held, _ := requiredValue(ctx, keyDefaultGatewayID)
	require.Equal(t, "development-held", held)
	require.Equal(t, kindDefaultRestore.Name, registry.Pending()[0].Kind.Name)
	require.Less(t, kindDefaultRestore.Order, cleanup.KindGateway.Order, "the default goes back before any gateway is deleted")
	handedBack := 0
	require.NoError(t, registry.RegisterDeleter(kindDefaultRestore, func(context.Context, cleanup.Resource) error {
		handedBack++
		return nil
	}))
	require.NoError(t, registry.SweepRunner(ctx))
	require.Zero(t, handedBack, "the default is not left to the end of the runner")
	require.NoError(t, registry.Sweep(ctx))
	require.Equal(t, 1, handedBack, "the default goes back when the scenario ends, however it ends")

	local.Set(keyManagedGatewayID, "development-held")
	require.ErrorContains(t, steps.makeGatewayDefault(ctx), "already the default")
}

func TestRestoreDefaultKeepsFieldsAndSkipsAGatewayThatIsGoneOrAlreadyDefault(t *testing.T) {
	var calls []string
	answer := func(status int, body string) func(context.Context, string, string, any) (*httpx.Response, error) {
		return func(_ context.Context, method, path string, sent any) (*httpx.Response, error) {
			calls = append(calls, method+" "+path)
			if method == http.MethodPut {
				require.Equal(t, map[string]any{"isDefault": true, "displayName": "Held", "isCritical": true}, sent)
				return &httpx.Response{StatusCode: http.StatusOK}, nil
			}
			return &httpx.Response{StatusCode: status, Body: []byte(body)}, nil
		}
	}
	require.NoError(t, restoreDefault(context.Background(), answer(http.StatusOK, `{"displayName":"Held","isCritical":true,"isDefault":false}`), "gw"))
	require.Equal(t, []string{"GET /managed-gateways/gw", "PUT /managed-gateways/gw"}, calls)
	calls = nil
	require.NoError(t, restoreDefault(context.Background(), answer(http.StatusNotFound, ""), "gw"))
	require.NoError(t, restoreDefault(context.Background(), answer(http.StatusOK, `{"isDefault":true}`), "gw"))
	require.Equal(t, []string{"GET /managed-gateways/gw", "GET /managed-gateways/gw"}, calls)
	require.Error(t, restoreDefault(context.Background(), answer(http.StatusInternalServerError, ""), "gw"))
}

func TestVerifyManagedGatewayDeletedWaitsForTheRecordToGo(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" {
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{}})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	require.NoError(t, registerGateway(ctx, cleanup.RunnerScope, "development-gone"))
	local.Set(keyManagedGatewayID, "development-gone")
	require.NoError(t, steps.verifyManagedGatewayDeleted(ctx))
	require.Empty(t, registry.Pending(), "a deleted gateway is no longer waiting to be cleaned up")
}

func TestDeleteManagedGatewayAcceptsOnlyAcceptedAndNoContent(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusAccepted
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(status)
	})
	local.Set(keyManagedGatewayID, "development-gw")
	for _, accepted := range []int{http.StatusAccepted, http.StatusNoContent} {
		status = accepted
		require.NoError(t, steps.deleteManagedGateway(ctx))
	}
	status = http.StatusConflict
	require.ErrorContains(t, steps.deleteManagedGateway(ctx), "409")
	require.NoError(t, steps.requestGatewayDeletion(ctx), "a refusal is for the scenario to assert on")
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusConflict))
}

func TestRegularGatewayOfPrefersAnActiveGatewayOfTheEnvironment(t *testing.T) {
	items := []map[string]any{
		{"id": "ai", "environment": "e-1", "functionalityType": "ai", "isActive": true},
		{"id": "other", "environment": "e-2", "isActive": true},
		{"id": "building", "environment": "e-1", "isActive": false},
		{"id": "ready", "environment": "e-1", "isActive": true},
	}
	require.Equal(t, "ready", regularGatewayOf(items, "e-1"))
	require.Equal(t, "building", regularGatewayOf(items[:3], "e-1"))
	require.Empty(t, regularGatewayOf(items, "e-3"))
}

func TestEnsureEnvironmentGatewayUsesTheOneTheEnvironmentLists(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var posted bool
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "e-1-default", "environment": "e-1", "isActive": false}}})
	})
	id, err := steps.ensureEnvironmentGateway(ctx, "e-1")
	require.NoError(t, err)
	require.Equal(t, "e-1-default", id)
	require.False(t, posted, "no gateway is built beside the one the platform provisioned")
	require.Empty(t, registry.Pending())
}

func TestEnsureEnvironmentGatewayReportsAListingThatNeverWorked(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var posted bool
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal_error"})
	})
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := steps.ensureEnvironmentGateway(ctx, "e-1")
	require.ErrorContains(t, err, `listing the gateways of environment "e-1"`)
	require.False(t, posted, "no gateway is built because the listing failed")
}

func TestEnsureRunnerEnvironmentsKeepsWhatTheRunnerHolds(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var posted bool
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			posted = true
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"name": "e-a-1"}, {"name": "e-b-2"}}})
	})
	for n, name := range []string{"e-a-1", "e-b-2"} {
		local.Set(numberedKey(keySyntheticEnvironmentPrefix, n+1), name)
		local.Set(numberedKey(keySyntheticGatewayPrefix, n+1), "gw-"+name)
	}
	local.Set(keySyntheticEnvironments, "e-a-1,e-b-2")

	require.NoError(t, steps.ensureRunnerEnvironments(ctx, 2))
	require.False(t, posted, "environments the runner already holds are not built again")
	require.Equal(t, "e-a-1", optionalValue(ctx, keyEnvironmentName))
	require.ErrorContains(t, steps.ensureRunnerEnvironments(ctx, 0), "between 1 and")
}

func TestRecordDeploymentsKeepsEachGatewaysDeployment(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	_ = steps
	local.Set(keyGatewayID, "gw-1")
	response := &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"list":[` +
		`{"gatewayId":"gw-1","deploymentId":"dep-1","buildId":"2026-01-31-1"},{"gatewayId":"gw-2","deploymentId":"dep-2","buildId":"2026-01-31-1"}]}`)}
	require.NoError(t, recordDeployments(ctx, response))
	targets, err := recordedTargets(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"gw-1": "dep-1", "gw-2": "dep-2"}, targets)
	local.Set(keyDeployedTargets, "{}")
	_, err = recordedTargets(ctx)
	require.ErrorContains(t, err, "no deployment")
}

func TestUndeployFromSyntheticGatewayUndeploysOnlyWhatIsHeld(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var undeployed []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
				{"deploymentId": "held", "status": "DEPLOYED"}, {"deploymentId": "gone", "status": "UNDEPLOYED"}}})
			return
		}
		undeployed = append(undeployed, r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	})
	local.Set(keyAPIID, "api-1")
	local.Set(keyProjectID, "default")
	require.NoError(t, recordSyntheticGateway(ctx, "development-gw", "development"))
	require.NoError(t, steps.undeployFromSyntheticGateway(ctx, 1))
	require.Equal(t, []string{"/projects/default/apis/api-1/deployments/held/undeploy?environment=development&gatewayId=development-gw"}, undeployed)
}

func TestWaitDeployedOnTargetsWaitsForEveryDeployment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	polls := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		polls++
		second := "DEPLOYING"
		if polls > 1 {
			second = "DEPLOYED"
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
			{"deploymentId": "dep-1", "status": "DEPLOYED"}, {"deploymentId": "dep-2", "status": second}}})
	})
	local.Set(keyAPIID, "api-1")
	local.Set(keyDeployedTargets, `{"gw-1":"dep-1","gw-2":"dep-2"}`)
	require.NoError(t, steps.waitDeployedOnTargets(ctx))
	require.Greater(t, polls, 1)
}

func TestConfigurationStepsTargetTheSelectedGatewayAndNotTheSharedOne(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var paths []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		writeJSON(w, http.StatusOK, map[string]any{"values": map[string]any{"a.b": "debug"}, "status": map[string]any{"phase": "applying"}})
	})
	local.Set(keyGatewayID, "development-shared")
	require.NoError(t, recordSyntheticGateway(ctx, "e-1-throwaway", "e-1"))

	require.NoError(t, steps.saveGatewayConfiguration(ctx, valuesTable(t, []string{"a.b", "debug"})))
	require.Equal(t, []string{"PUT /managed-gateways/development-shared/configuration"}, paths, "without a selection the shared gateway is the target")

	paths = nil
	require.NoError(t, steps.selectConfigurationGateway(ctx, 1))
	require.NoError(t, steps.saveGatewayConfiguration(ctx, valuesTable(t, []string{"a.b", "debug"})))
	require.NoError(t, steps.assertConfigurationPhase(ctx, "applying"))
	require.ErrorContains(t, steps.assertConfigurationPhase(ctx, "healthy"), "want \"healthy\"")
	require.NoError(t, steps.assertConfigurationReports(ctx, valuesTable(t, []string{"a.b", "debug"})))
	require.Equal(t, "PUT /managed-gateways/e-1-throwaway/configuration", paths[0])
	for _, path := range paths {
		require.Contains(t, path, "e-1-throwaway")
	}
	require.ErrorContains(t, steps.selectConfigurationGateway(ctx, 2), "does not exist")
}

func TestAssertConfigurationOmitsWaitsForTheSettingToGo(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	reads := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		reads++
		values := map[string]any{"other": 1}
		if reads == 1 {
			values["gone.soon"] = "x"
		}
		writeJSON(w, http.StatusOK, map[string]any{"values": values})
	})
	local.Set(keyGatewayID, "development-shared")
	require.NoError(t, steps.assertConfigurationOmits(ctx, "gone.soon"))
	require.Greater(t, reads, 1)
}

func TestJudgeRoundAcceptsWritesAndConflictsAndFlagsEverythingElse(t *testing.T) {
	ok := &httpx.Response{StatusCode: http.StatusOK}
	conflict := &httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"conflict"}`)}
	round := judgeRound([]*httpx.Response{ok, conflict, ok})
	require.Equal(t, []int{200, 409, 200}, round.statuses)
	require.Equal(t, conflict, round.conflict)
	require.Empty(t, round.unwanted)

	round = judgeRound([]*httpx.Response{ok, {StatusCode: http.StatusInternalServerError, Body: []byte("boom")},
		{StatusCode: http.StatusConflict, Body: []byte(`{"code":"other"}`)}})
	require.Len(t, round.unwanted, 2)
}

func TestADeploymentGetsAtLeastTenMinutesToBecomeDeployed(t *testing.T) {
	require.GreaterOrEqual(t, deployedTimeout, 10*time.Minute)
}

func TestCoreProjectsBaseReplacesThePlatformAPIRoute(t *testing.T) {
	base, err := coreProjectsBase("http://dev.localhost:19080" + platformAPIRoute + "/")
	require.NoError(t, err)
	require.Equal(t, "http://dev.localhost:19080"+coreAPIRoute, base)

	_, err = coreProjectsBase("http://dev.localhost:19080/some/other/route")
	require.Error(t, err)
}

func TestDeleteProjectThroughCloudAPIRetriesATransientAnswer(t *testing.T) {
	previous := projectDeleteInterval
	projectDeleteInterval = 10 * time.Millisecond
	t.Cleanup(func() { projectDeleteInterval = previous })

	for _, transient := range []int{http.StatusServiceUnavailable, http.StatusRequestTimeout, http.StatusTooManyRequests} {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests++
			if requests == 1 {
				w.WriteHeader(transient)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, MaxRetries: -1})
		err := deleteProjectThroughCloudAPI(context.Background(), client, server.URL+platformAPIRoute, "console-token", "syn-project-1", projectDeleteTimeout)
		server.Close()

		require.NoError(t, err, "status %d is retried until the delete is accepted", transient)
		require.Equal(t, 2, requests, "status %d sends a second DELETE", transient)
	}
}

func TestDeleteProjectThroughCloudAPIReturnsAnUnderivableRoute(t *testing.T) {
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})
	err := deleteProjectThroughCloudAPI(context.Background(), client, "http://dev.localhost:19080/some/other/route", "console-token", "syn-project-1", projectDeleteTimeout)
	require.ErrorContains(t, err, "syn-project-1")
}

func TestDeleteProjectThroughCloudAPISendsOneRequestAndReportsARefusal(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound, http.StatusForbidden} {
		var requests []string
		var authorization string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.Method+" "+r.URL.Path)
			authorization = r.Header.Get("Authorization")
			w.WriteHeader(status)
		}))

		client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})
		err := deleteProjectThroughCloudAPI(context.Background(), client, server.URL+platformAPIRoute, "console-token", "syn-project-1", projectDeleteTimeout)
		server.Close()

		if status == http.StatusForbidden {
			require.ErrorContains(t, err, "syn-project-1", "a refusal is returned so cleanup reports the leak")
		} else {
			require.NoError(t, err, "status %d", status)
		}
		require.Equal(t, []string{"DELETE " + coreAPIRoute + "/projects/syn-project-1"}, requests, "status %d", status)
		require.Equal(t, "Bearer console-token", authorization)
	}
}

// fastPortalPolls lowers the portal poll interval to the retry floor for one test.
func fastPortalPolls(t *testing.T) {
	t.Helper()
	previous := portalPollInterval
	portalPollInterval = retry.BaseInterval
	t.Cleanup(func() { portalPollInterval = previous })
}

func registeredPortals(registry *cleanup.Registry) []string {
	var ids []string
	for _, resource := range registry.Pending() {
		if resource.Kind == kindAPIPortal {
			ids = append(ids, resource.ID)
		}
	}
	return ids
}

func TestRequestRunnerPortalRegistersThePortalBeforeTheRequest(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	var registeredAtRequest []string
	var registry *cleanup.Registry
	answer := http.StatusAccepted
	steps, ctx, _, reg := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, managedPortalsCollection, r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		registeredAtRequest = registeredPortals(registry)
		writeJSON(w, answer, map[string]any{"id": sent["handle"], "name": sent["name"], "status": portalStatusPending})
	})
	registry = reg

	require.ErrorContains(t, steps.assertPortalAccepted(ctx), "requested no managed API Portal")
	require.NoError(t, steps.requestRunnerPortal(ctx))
	handle, err := runnerPortal(ctx)
	require.NoError(t, err)
	require.Equal(t, handle, sent["handle"])
	require.Equal(t, "Synthetic "+handle, sent["name"])
	require.Equal(t, "development", sent["loginEnvironment"], "consumers sign in on the development environment")
	require.LessOrEqual(t, len(handle), portalHandleMaxLength)
	require.Equal(t, []string{handle}, registeredAtRequest, "the portal is registered before the request that creates it")
	require.NoError(t, steps.assertPortalAccepted(ctx))

	answer = http.StatusCreated
	require.NoError(t, steps.requestRunnerPortal(ctx))
	require.NoError(t, steps.assertPortalAccepted(ctx), "a portal the platform built before answering is accepted too")
}

func TestRequestRunnerPortalDropsTheRegistrationOfARefusedPortal(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusPaymentRequired, map[string]string{"code": "payment_required"})
	})
	require.NoError(t, steps.requestRunnerPortal(ctx))
	require.Empty(t, registeredPortals(registry))
	require.ErrorContains(t, steps.assertPortalAccepted(ctx), "not accepted")
}

func TestAssertPortalAcceptedChecksTheHandleAndThePendingStatus(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	body := map[string]any{}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, body)
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	for _, tc := range []struct {
		body map[string]any
		want string
	}{
		{map[string]any{"status": portalStatusPending}, "reading the id"},
		{map[string]any{"id": "another-portal", "status": portalStatusPending}, "want the requested handle"},
		{map[string]any{"id": "syn-portal-1", "status": portalStatusActive}, "should read as pending"},
		{map[string]any{"id": "syn-portal-1"}, "should read as pending"},
	} {
		body = tc.body
		_, err := steps.doAuthenticated(ctx, http.MethodGet, "/answer", nil)
		require.NoError(t, err)
		require.ErrorContains(t, steps.assertPortalAccepted(ctx), tc.want)
	}
}

func TestAwaitPortalActiveWaitsForTheBuildAndRecordsTheURL(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	reads := 0
	activeAt := 3
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, portalPath("syn-portal-1"), r.URL.Path)
		reads++
		if reads == 1 {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
			return
		}
		if reads < activeAt {
			writeJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": portalStatusPending})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": portalStatusActive,
			"url": "https://abc123-portal-p01.example.test", "loginEnvironment": "development", "selfSignup": true})
	})
	require.ErrorContains(t, steps.awaitPortalActive(ctx), "requested no managed API Portal")

	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.awaitPortalActive(ctx))
	require.Equal(t, activeAt, reads, "a portal that briefly reads as not found while it is built is read again")
	portalURL, err := requiredValue(ctx, keyAPIPortalURL)
	require.NoError(t, err)
	require.Equal(t, "https://abc123-portal-p01.example.test", portalURL)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	status, err := stringField(published, "status")
	require.NoError(t, err)
	require.Equal(t, portalStatusActive, status, "the active read is published for the assertions that follow")
	require.NoError(t, steps.assertPortalURL(ctx))

	reads = activeAt
	require.NoError(t, steps.awaitPortalActive(ctx), "an active portal passes on the first read")
	require.Equal(t, activeAt+1, reads)
}

func TestAwaitPortalActiveStopsAtAFailedPortalOrARefusedRead(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	status, code := portalStatusFailed, http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, code, map[string]any{"id": "syn-portal-1", "status": status})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	started := time.Now()
	require.ErrorIs(t, steps.awaitPortalActive(ctx), errPortalFailed)
	code = http.StatusForbidden
	require.ErrorContains(t, steps.awaitPortalActive(ctx), "403")
	require.Less(t, time.Since(started), 10*time.Second, "neither a failed portal nor a refused read is waited for")
}

func TestRequireSecureURL(t *testing.T) {
	t.Setenv(cloudLocalDataPlaneEnv, "")
	require.Error(t, requireSecureURL(nil))
	for _, accepted := range []string{"https://abc-portal-p01.example.test", "https://127.0.0.1:8443/api-portal"} {
		parsed, err := url.Parse(accepted)
		require.NoError(t, err)
		require.NoError(t, requireSecureURL(parsed), accepted)
	}
	for _, refused := range []string{"http://abc-portal-p01.example.test", "https://user:pass@example.test", "/api-portal/login", "http://portal.localhost"} {
		parsed, err := url.Parse(refused)
		require.NoError(t, err)
		err = requireSecureURL(parsed)
		require.Error(t, err, refused)
		require.NotContains(t, err.Error(), "pass@", "an error never repeats a credential")
	}
	t.Setenv(cloudLocalDataPlaneEnv, "true")
	parsed, err := url.Parse("http://portal.localhost:8080")
	require.NoError(t, err)
	require.NoError(t, requireSecureURL(parsed), "a cloud on the developer's machine serves plain http on .localhost")
}

func TestAssertPortalURLRequiresAnHTTPSURL(t *testing.T) {
	t.Setenv(cloudLocalDataPlaneEnv, "")
	_, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	steps := &Steps{}
	require.ErrorContains(t, steps.assertPortalURL(ctx), "reported no URL")
	local.Set(keyAPIPortalURL, "http://abc-portal-p01.example.test")
	require.ErrorContains(t, steps.assertPortalURL(ctx), "https")
	local.Set(keyAPIPortalURL, "https://abc-portal-p01.example.test")
	require.NoError(t, steps.assertPortalURL(ctx))
}

func TestAssertPortalListedRequiresTheActivePortal(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	listed := []map[string]any{{"id": "ops-portal", "status": portalStatusActive}}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, managedPortalsCollection, r.URL.Path)
		writeJSON(w, http.StatusOK, map[string]any{"count": len(listed), "list": listed})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.ErrorContains(t, steps.assertPortalListed(ctx), "does not include")
	listed = append(listed, map[string]any{"id": "syn-portal-1", "status": portalStatusPending})
	require.ErrorContains(t, steps.assertPortalListed(ctx), "status pending")
	listed[1]["status"] = portalStatusActive
	require.NoError(t, steps.assertPortalListed(ctx))
}

func TestRequestDuplicatePortalRepeatsTheHandleWithAnotherName(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeJSON(w, http.StatusConflict, map[string]string{"code": "conflict"})
	})
	require.ErrorContains(t, steps.requestDuplicatePortal(ctx), "requested no managed API Portal")
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.requestDuplicatePortal(ctx))
	require.Equal(t, "syn-portal-1", sent["handle"])
	require.NotEqual(t, "Synthetic syn-portal-1", sent["name"])
	require.Empty(t, registeredPortals(registry), "the handle is the runner's and already registered")
}

func TestRequestDuplicatePortalLeavesAnyAnswerToTheStatusAssertion(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	code, body := http.StatusPaymentRequired, map[string]string{"code": "quota_exceeded", "message": "Hard limit reached: 1 of 1 used"}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, code, body)
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.requestDuplicatePortal(ctx), "a 402 is not skipped; the status assertion fails it")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusPaymentRequired, published.StatusCode)

	code, body = http.StatusConflict, map[string]string{"code": "conflict"}
	require.NoError(t, steps.requestDuplicatePortal(ctx))
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, published.StatusCode)
}

func TestAssertPortalStillActive(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status, code := portalStatusActive, http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, code, map[string]any{"id": "syn-portal-1", "status": status})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.assertPortalStillActive(ctx))
	status = portalStatusPending
	require.ErrorContains(t, steps.assertPortalStillActive(ctx), "want active")
	code = http.StatusNotFound
	require.Error(t, steps.assertPortalStillActive(ctx))
}

func TestRequestDefectivePortalBreaksOneRuleAndRegistersTheHandleFirst(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	var registeredAtRequest []string
	var registry *cleanup.Registry
	steps, ctx, _, reg := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		registeredAtRequest = registeredPortals(registry)
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "bad_request"})
	})
	registry = reg
	require.ErrorContains(t, steps.requestDefectivePortal(ctx, "with a typo"), "unknown managed API Portal request defect")

	checks := map[string]func(handle, name string){
		"an uppercase handle": func(handle, _ string) {
			require.Equal(t, strings.ToUpper(handle), handle)
			require.NotEqual(t, strings.ToLower(handle), handle)
		},
		"a handle with an underscore":        func(handle, _ string) { require.Contains(t, handle, "_") },
		"a handle shorter than 3 characters": func(handle, _ string) { require.Len(t, handle, 2) },
		"a handle longer than 40 characters": func(handle, _ string) { require.Greater(t, len(handle), portalHandleMaxLength) },
		"no name": func(handle, name string) {
			require.Empty(t, name)
			require.Regexp(t, `^[a-z0-9-]{3,40}$`, handle, "only the name is wrong")
		},
	}
	require.Len(t, checks, len(portalRequestDefects), "every defect is checked")
	for defect, check := range checks {
		sent, registeredAtRequest = nil, nil
		require.NoError(t, steps.requestDefectivePortal(ctx, defect), defect)
		handle, _ := sent["handle"].(string)
		name, _ := sent["name"].(string)
		check(handle, name)
		refused, err := requiredValue(ctx, keyRefusedAPIPortalID)
		require.NoError(t, err)
		require.Equal(t, refused, handle, defect)
		require.Equal(t, []string{handle}, registeredAtRequest, defect+": registered before the request")
		require.Empty(t, registeredPortals(registry), defect+": a refused portal is no longer registered")
	}
}

func TestRequestDefectivePortalKeepsTheRegistrationOfAnAcceptedPortal(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]any{"status": portalStatusPending})
	})
	require.NoError(t, steps.requestDefectivePortal(ctx, "no name"))
	refused, err := requiredValue(ctx, keyRefusedAPIPortalID)
	require.NoError(t, err)
	require.Equal(t, []string{refused}, registeredPortals(registry), "a portal accepted against expectation is removed by cleanup")
}

func TestRequestPortalInAbsentEnvironmentNamesAnEnvironmentNobodyCreated(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "bad_request"})
	})
	require.NoError(t, steps.requestPortalInAbsentEnvironment(ctx))
	environment, _ := sent["loginEnvironment"].(string)
	require.True(t, strings.HasPrefix(environment, "syn-no-env-"), environment)
	require.Regexp(t, `^[a-z0-9-]{3,40}$`, sent["handle"])
	require.NotEmpty(t, sent["name"])
	_, overwritten := tcontext.Get(ctx, keyEnvironmentName)
	require.False(t, overwritten, "the environment the other steps name is left alone")
}

func TestAssertRefusedPortalNotListed(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	listed := []map[string]any{{"id": "ops-portal"}}
	code := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, code, map[string]any{"list": listed})
	})
	require.ErrorContains(t, steps.assertRefusedPortalNotListed(ctx), "requested no managed API Portal")
	local.Set(keyRefusedAPIPortalID, "SYN-PORTAL-1")
	require.NoError(t, steps.assertRefusedPortalNotListed(ctx))
	listed = append(listed, map[string]any{"id": "syn-portal-1"})
	require.ErrorContains(t, steps.assertRefusedPortalNotListed(ctx), "is listed", "a handle is compared without regard to case")
	listed = listed[:1]
	code = http.StatusInternalServerError
	require.Error(t, steps.assertRefusedPortalNotListed(ctx), "a list that does not answer proves nothing")
}

func TestReadRunnerAndAbsentPortals(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var seen []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	require.ErrorContains(t, steps.readRunnerPortal(ctx), "requested no managed API Portal")
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.readRunnerPortal(ctx))
	require.NoError(t, steps.readAbsentPortal(ctx))
	absent, err := requiredValue(ctx, keyAbsentAPIPortalID)
	require.NoError(t, err)
	require.NotEqual(t, "syn-portal-1", absent)
	require.Equal(t, []string{"GET " + portalPath("syn-portal-1"), "GET " + portalPath(absent)}, seen)
	require.Empty(t, registry.Pending(), "a portal that is never created is never registered")
}

func TestAssertPortalDeletionAccepted(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	code := http.StatusAccepted
	body := map[string]any{"id": "syn-portal-1", "status": portalStatusDeleting}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		if code == http.StatusNoContent {
			w.WriteHeader(code)
			return
		}
		writeJSON(w, code, body)
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.NoError(t, steps.assertPortalDeletionAccepted(ctx))
	body["status"] = portalStatusActive
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.ErrorContains(t, steps.assertPortalDeletionAccepted(ctx), "should read as deleting")
	code = http.StatusNoContent
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.NoError(t, steps.assertPortalDeletionAccepted(ctx))
	code = http.StatusConflict
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.ErrorContains(t, steps.assertPortalDeletionAccepted(ctx), "not accepted")
}

func TestAwaitPortalGoneWaitsForTheTeardownAndDropsTheRegistration(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	reads := 0
	listed := []map[string]any{}
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == managedPortalsCollection {
			writeJSON(w, http.StatusOK, map[string]any{"list": listed})
			return
		}
		reads++
		if reads < 3 {
			writeJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": portalStatusDeleting})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))
	require.NoError(t, steps.awaitPortalGone(ctx))
	require.Equal(t, 3, reads)
	require.Empty(t, registeredPortals(registry), "a deleted portal is not deleted again by cleanup")

	listed = []map[string]any{{"id": "syn-portal-1", "status": portalStatusDeleting}}
	require.ErrorContains(t, steps.awaitPortalGone(ctx), "still listed")
}

func TestDeletePortalRepeatsARefusalUntilTheBuildEnds(t *testing.T) {
	fastPortalPolls(t)
	var answers []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "Bearer console-token", r.Header.Get("Authorization"))
		require.Equal(t, portalPath("syn-portal-1"), r.URL.Path)
		code := answers[0]
		if len(answers) > 1 {
			answers = answers[1:]
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(server.Close)
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})

	answers = []int{http.StatusConflict, http.StatusConflict, http.StatusAccepted}
	require.NoError(t, deletePortal(context.Background(), client, server.URL, "console-token", "syn-portal-1", time.Minute))
	require.Equal(t, []int{http.StatusAccepted}, answers, "the delete is repeated while the portal is still being built")

	answers = []int{http.StatusNotFound}
	require.NoError(t, deletePortal(context.Background(), client, server.URL, "console-token", "syn-portal-1", time.Minute),
		"an absent portal counts as deleted")

	answers = []int{http.StatusForbidden}
	started := time.Now()
	err := deletePortal(context.Background(), client, server.URL, "console-token", "syn-portal-1", time.Minute)
	require.ErrorContains(t, err, "403")
	require.Less(t, time.Since(started), 10*time.Second, "a refusal that will not change is not repeated")
}

func TestRegisterDeletersDeletesARegisteredPortal(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	var seen []string
	reads := 0
	topo := cloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodDelete {
			writeJSON(w, http.StatusAccepted, map[string]any{"status": portalStatusDeleting})
			return
		}
		reads++
		if reads == 1 {
			writeJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": portalStatusDeleting})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	registry := cleanup.NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, RegisterDeleters(registry, topo))
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, cleanup.Install(ctx, registry))
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))
	require.NoError(t, registry.SweepRunner(ctx))
	require.Equal(t, []string{"DELETE " + portalPath("syn-portal-1"), "GET " + portalPath("syn-portal-1"), "GET " + portalPath("syn-portal-1")}, seen,
		"cleanup waits until the portal is gone, which frees the quota and leaves the finalizer nothing to find")
	require.Empty(t, registry.Pending())
}

// fakePortalSite is a managed API Portal and its identity provider, each on a TLS server of its
// own, that behave as the sign-in steps expect the real ones to. The identity provider serves
// under a path prefix, as one behind a shared gateway does.
type fakePortalSite struct {
	t      *testing.T
	portal *httptest.Server
	idp    *httptest.Server

	mu       sync.Mutex
	users    map[string]string
	sessions map[string]bool
	flows    []map[string]any
	// wrongPasswordStatus is how the identity provider answers a wrong password: 200 with an
	// ERROR flow status, or a client error.
	wrongPasswordStatus int
	// clientID and callbackHost let a test break the authorization request the portal sends.
	clientID     string
	callbackHost string
}

const fakeIdentityPrefix = "/t/acme"

func newFakePortalSite(t *testing.T) *fakePortalSite {
	t.Helper()
	site := &fakePortalSite{t: t, users: map[string]string{}, sessions: map[string]bool{}, clientID: "portal-client",
		wrongPasswordStatus: http.StatusOK}
	site.portal = httptest.NewTLSServer(http.HandlerFunc(site.servePortal))
	site.idp = httptest.NewTLSServer(http.HandlerFunc(site.serveIdentity))
	t.Cleanup(site.portal.Close)
	t.Cleanup(site.idp.Close)
	site.callbackHost = site.portal.URL
	return site
}

// client trusts the fake servers' certificate, which the two servers share.
func (f *fakePortalSite) client() *httpx.Client {
	pool := x509.NewCertPool()
	pool.AddCert(f.portal.Certificate())
	return httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}})
}

func (f *fakePortalSite) sessionOf(r *http.Request) string {
	if cookie, err := r.Cookie("portal.sid"); err == nil {
		return cookie.Value
	}
	return ""
}

func (f *fakePortalSite) servePortal(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	const view = "/api-portal/acme/views/default"
	switch r.URL.Path {
	case "/api-portal/login":
		http.Redirect(w, r, view+"/login", http.StatusFound)
	case view + "/login":
		http.SetCookie(w, &http.Cookie{Name: "portal.sid", Value: "anonymous", Path: "/api-portal"})
		query := url.Values{"response_type": {"code"}, "client_id": {f.clientID}, "state": {"st"},
			"redirect_uri": {f.callbackHost + "/api-portal/acme/callback"}}
		http.Redirect(w, r, f.idp.URL+fakeIdentityPrefix+"/oauth2/authorize?"+query.Encode(), http.StatusFound)
	case "/api-portal/acme/callback":
		if f.sessionOf(r) != "anonymous" || r.URL.Query().Get("code") != "the-code" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		session := "signed-in-" + strconv.Itoa(len(f.sessions))
		f.sessions[session] = true
		http.SetCookie(w, &http.Cookie{Name: "portal.sid", Value: session, Path: "/api-portal"})
		http.Redirect(w, r, view, http.StatusFound)
	case view + "/applications":
		if !f.sessions[f.sessionOf(r)] {
			http.Redirect(w, r, view+"/login", http.StatusSeeOther)
			return
		}
		_, _ = w.Write([]byte("<html>applications</html>"))
	case view + "/logout":
		delete(f.sessions, f.sessionOf(r))
		http.Redirect(w, r, f.idp.URL+fakeIdentityPrefix+"/oauth2/logout", http.StatusFound)
	default:
		f.t.Errorf("unexpected portal request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakePortalSite) serveIdentity(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case fakeIdentityPrefix + "/oauth2/authorize":
		http.Redirect(w, r, f.idp.URL+"/gate/signin?executionId=exec-1&authId=auth-1", http.StatusFound)
	case fakeIdentityPrefix + "/flow/execute":
		var body map[string]any
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&body))
		f.flows = append(f.flows, body)
		require.Equal(f.t, "exec-1", body["executionId"])
		inputs, _ := body["inputs"].(map[string]any)
		switch {
		case body["action"] == nil:
			writeJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": "INCOMPLETE", "challengeToken": "ch-1"})
		case body["action"] == flowActionSignUp:
			require.Equal(f.t, "ch-1", body["challengeToken"])
			writeJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": "INCOMPLETE", "challengeToken": "ch-2"})
		case body["action"] == flowActionSubmit && inputs["email"] != nil:
			require.Equal(f.t, "ch-2", body["challengeToken"])
			f.users[inputs["username"].(string)] = inputs["password"].(string)
			writeJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusComplete, "assertion": "the-assertion"})
		case body["action"] == flowActionSubmit:
			require.Equal(f.t, "ch-1", body["challengeToken"])
			if password, known := f.users[inputs["username"].(string)]; known && password == inputs["password"] {
				writeJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusComplete, "assertion": "the-assertion"})
				return
			}
			if f.wrongPasswordStatus != http.StatusOK {
				writeJSON(w, f.wrongPasswordStatus, map[string]any{"code": "FES-1001", "message": "Invalid credentials"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusError, "failureReason": "Invalid credentials"})
		default:
			f.t.Errorf("unexpected flow action %v", body["action"])
			w.WriteHeader(http.StatusBadRequest)
		}
	case fakeIdentityPrefix + "/oauth2/auth/callback":
		var body map[string]any
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(f.t, "auth-1", body["authId"])
		require.Equal(f.t, "the-assertion", body["assertion"])
		require.Equal(f.t, []any{}, body["sessionData"], "the callback carries a list even when the flow reported no session")
		writeJSON(w, http.StatusOK, map[string]any{"redirect_uri": f.portal.URL + "/api-portal/acme/callback?code=the-code&state=st"})
	default:
		f.t.Errorf("unexpected identity provider request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// signInSteps returns steps whose client trusts the fake site, in a runner whose portal is the
// fake portal.
func signInSteps(t *testing.T, site *fakePortalSite) (*Steps, context.Context) {
	t.Helper()
	client := site.client()
	steps := &Steps{funnel: httpx.NewFunnel(client, 0, 0), client: client}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, tcontext.Set(ctx, keyAPIPortalURL, site.portal.URL))
	return steps, ctx
}

func TestViewSignInPage(t *testing.T) {
	org, view, ok := viewSignInPage("/api-portal/acme/views/default/login")
	require.True(t, ok)
	require.Equal(t, "acme", org)
	require.Equal(t, "default", view)
	for _, other := range []string{"/api-portal/login", "/api-portal/acme/views/default", "/portal/acme/views/default/login",
		"/api-portal//views/default/login", "/api-portal/acme/pages/default/login", "/api-portal/acme/views/default/login/extra"} {
		_, _, ok := viewSignInPage(other)
		require.False(t, ok, other)
	}
}

func TestIdentityBaseKeepsThePrefixAndDropsTheQuery(t *testing.T) {
	authorize, err := url.Parse("https://idp.example.test/t/acme/oauth2/authorize?client_id=c#x")
	require.NoError(t, err)
	base := identityBase(authorize)
	require.Equal(t, "https://idp.example.test/t/acme", base.String())
	require.Equal(t, "https://idp.example.test/t/acme/flow/execute", joinURLPath(base, identityFlowPath))
	require.Equal(t, "https://idp.example.test/t/acme/oauth2/authorize?client_id=c#x", authorize.String(), "the authorization URL is left as it was")
}

func TestOpenSignInFollowsThePortalToItsIdentityProvider(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.ErrorContains(t, steps.assertSentToIdentityProvider(ctx), "opened no managed API Portal")

	require.NoError(t, steps.openSignInAsVisitor(ctx))
	require.NoError(t, steps.assertSentToIdentityProvider(ctx))
	browser, err := scenarioBrowser(ctx)
	require.NoError(t, err)
	require.Equal(t, "acme", browser.org)
	require.Equal(t, "default", browser.view)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, published.StatusCode, "the answer that sent the visitor away is published")

	site.clientID = ""
	require.NoError(t, steps.openSignInAsVisitor(ctx))
	require.ErrorContains(t, steps.assertSentToIdentityProvider(ctx), "names no client")
	site.clientID, site.callbackHost = "portal-client", site.idp.URL
	require.NoError(t, steps.openSignInAsVisitor(ctx))
	require.ErrorContains(t, steps.assertSentToIdentityProvider(ctx), "not to the portal")
}

func TestOpenSignInRefusesAPortalWithoutAnHTTPSURL(t *testing.T) {
	t.Setenv(cloudLocalDataPlaneEnv, "")
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.NoError(t, tcontext.Set(ctx, keyAPIPortalURL, strings.Replace(site.portal.URL, "https://", "http://", 1)))
	require.ErrorContains(t, steps.openSignInAsVisitor(ctx), "https")
}

func TestSignUpThenSignInKeepsTheConsumersSession(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.ErrorContains(t, steps.signInConsumer(ctx), "signed up no consumer")

	require.NoError(t, steps.signUpConsumer(ctx))
	username, password, err := consumerCredentials(ctx)
	require.NoError(t, err)
	require.Equal(t, password, site.users[username])
	require.Equal(t, []any{nil, flowActionSignUp, flowActionSubmit}, flowActions(site.flows))
	require.Equal(t, username+"@example.com", site.flows[2]["inputs"].(map[string]any)["email"])
	require.NoError(t, steps.assertConsumerSignedIn(ctx))
	require.NoError(t, steps.assertVisitorWithoutSessionSentToSignIn(ctx))
	require.ErrorContains(t, steps.assertConsumerNotSignedIn(ctx), "was not refused")

	site.flows = nil
	require.NoError(t, steps.signInConsumer(ctx))
	require.Equal(t, []any{nil, flowActionSubmit}, flowActions(site.flows))
	require.NoError(t, steps.assertConsumerSignedIn(ctx))
	require.ErrorContains(t, steps.assertSignInRefused(ctx), "accepted the sign-in")
}

func flowActions(flows []map[string]any) []any {
	actions := make([]any, 0, len(flows))
	for _, flow := range flows {
		actions = append(actions, flow["action"])
	}
	return actions
}

func TestAWrongPasswordIsRefusedAndLeavesNoSession(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized} {
		site := newFakePortalSite(t)
		site.wrongPasswordStatus = status
		steps, ctx := signInSteps(t, site)
		require.NoError(t, steps.signUpConsumer(ctx))
		_, password, err := consumerCredentials(ctx)
		require.NoError(t, err)

		require.NoError(t, steps.trySignInWithWrongPassword(ctx), "a refusal is the identity provider's answer, not a step failure")
		require.NoError(t, steps.assertSignInRefused(ctx))
		require.NoError(t, steps.assertConsumerNotSignedIn(ctx))
		err = steps.signInConsumer(ctx)
		require.NoError(t, err, "the right password still signs the consumer in")

		require.NoError(t, steps.trySignInWithWrongPassword(ctx))
		err = requireSignInAccepted(ctx)
		require.ErrorContains(t, err, "refused the consumer's sign-in")
		require.NotContains(t, err.Error(), password, "an error never repeats the consumer's password")
	}
}

func TestSignOutEndsTheConsumersSession(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.ErrorContains(t, steps.signOutConsumer(ctx), "opened no managed API Portal")
	require.NoError(t, steps.signUpConsumer(ctx))
	require.NoError(t, steps.assertConsumerSignedIn(ctx))
	require.NoError(t, steps.signOutConsumer(ctx))
	require.NoError(t, steps.assertConsumerNotSignedIn(ctx))
	require.ErrorContains(t, steps.assertConsumerSignedIn(ctx), "could not open")
}

// fakePortalPlatform is the portal routes of the platform for the tests of the runner portal: each
// portal reads with the status the test gives it, a delete turns it into a deleted portal, and a
// create builds a portal whose status the test chose up front.
type fakePortalPlatform struct {
	mu       sync.Mutex
	statuses map[string]string
	builds   []string
	created  []string
	deleted  []string
}

func (f *fakePortalPlatform) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == managedPortalsCollection:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		handle, _ := body["handle"].(string)
		f.created = append(f.created, handle)
		next := portalStatusActive
		if len(f.builds) > 0 {
			next, f.builds = f.builds[0], f.builds[1:]
		}
		f.statuses[handle] = next
		writeJSON(w, http.StatusAccepted, map[string]any{"id": handle, "status": portalStatusPending})
	case r.Method == http.MethodDelete:
		handle := strings.TrimPrefix(r.URL.Path, managedPortalsCollection+"/")
		f.deleted = append(f.deleted, handle)
		delete(f.statuses, handle)
		writeJSON(w, http.StatusAccepted, map[string]any{"id": handle, "status": portalStatusDeleting})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, managedPortalsCollection+"/"):
		handle := strings.TrimPrefix(r.URL.Path, managedPortalsCollection+"/")
		status, ok := f.statuses[handle]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": handle, "status": status, "url": "https://" + handle + "-portal-p01.example.test"})
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func TestEnsureActiveRunnerPortalUsesTheRunnersActivePortal(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	platform := &fakePortalPlatform{statuses: map[string]string{"syn-portal-1": portalStatusActive}}
	steps, ctx, local, _ := cloudSteps(t, platform.handler)
	local.Set(keyAPIPortalID, "syn-portal-1")
	local.Append(keyPortalBuilds, true)
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))

	require.NoError(t, steps.ensureActiveRunnerPortal(ctx))
	require.Empty(t, platform.created, "an active portal is used as it is")
	portalURL, err := requiredValue(ctx, keyAPIPortalURL)
	require.NoError(t, err)
	require.Equal(t, "https://syn-portal-1-portal-p01.example.test", portalURL)
}

func TestEnsureActiveRunnerPortalReplacesAFailedPortalOnce(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	platform := &fakePortalPlatform{statuses: map[string]string{"syn-portal-1": portalStatusFailed}}
	steps, ctx, local, registry := cloudSteps(t, platform.handler)
	local.Set(keyAPIPortalID, "syn-portal-1")
	local.Set(keyAPIPortalURL, "https://syn-portal-1-portal-p01.example.test")
	local.Append(keyPortalBuilds, true)
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))

	require.NoError(t, steps.ensureActiveRunnerPortal(ctx))
	require.Equal(t, []string{"syn-portal-1"}, platform.deleted, "the failed portal is deleted first, which frees the quota")
	require.Len(t, platform.created, 1)
	replacement, err := runnerPortal(ctx)
	require.NoError(t, err)
	require.Equal(t, platform.created[0], replacement)
	require.Equal(t, []string{replacement}, registeredPortals(registry), "only the replacement is left for cleanup")
	portalURL, err := requiredValue(ctx, keyAPIPortalURL)
	require.NoError(t, err)
	require.Equal(t, "https://"+replacement+"-portal-p01.example.test", portalURL, "the URL is the replacement's")
	require.Len(t, local.List(keyPortalBuilds), maxPortalBuilds)

	require.NoError(t, steps.ensureActiveRunnerPortal(ctx), "later scenarios use the replacement")
	require.Len(t, platform.created, 1)
}

func TestEnsureActiveRunnerPortalGivesUpWhenTheReplacementFailsToo(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	platform := &fakePortalPlatform{statuses: map[string]string{"syn-portal-1": portalStatusFailed}, builds: []string{portalStatusFailed}}
	steps, ctx, local, _ := cloudSteps(t, platform.handler)
	local.Set(keyAPIPortalID, "syn-portal-1")
	local.Append(keyPortalBuilds, true)
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))

	err := steps.ensureActiveRunnerPortal(ctx)
	require.ErrorIs(t, err, errPortalFailed)
	require.ErrorContains(t, err, "none became active")
	require.Len(t, platform.created, 1, "a failed replacement is not replaced again")
}

func TestEnsureActiveRunnerPortalBuildsOneWhenTheFirstWasRefused(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	platform := &fakePortalPlatform{statuses: map[string]string{}}
	steps, ctx, local, _ := cloudSteps(t, platform.handler)
	local.Set(keyAPIPortalID, "syn-portal-1")
	local.Append(keyPortalBuilds, true)

	started := time.Now()
	require.NoError(t, steps.ensureActiveRunnerPortal(ctx))
	require.Less(t, time.Since(started), 10*time.Second, "a refused portal is not waited for")
	require.Empty(t, platform.deleted, "a refused portal has nothing to delete")
	require.Len(t, platform.created, 1)
}

func TestEnsureActiveRunnerPortalBuildsOneWhenNoneWasRequested(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	platform := &fakePortalPlatform{statuses: map[string]string{}}
	steps, ctx, local, registry := cloudSteps(t, platform.handler)
	require.NoError(t, steps.ensureActiveRunnerPortal(ctx))
	require.Len(t, platform.created, 1)
	require.Len(t, local.List(keyPortalBuilds), 1)
	require.Equal(t, platform.created, registeredPortals(registry))
}
