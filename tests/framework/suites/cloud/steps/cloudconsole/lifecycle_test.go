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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/finalizer"
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

	definition, err := newAPIDefinition(ctx, "syn-api-1", "default", nil)
	require.NoError(t, err)
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

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"id":2}`)}))
	require.NoError(t, steps.assertAPIResponse(ctx, 2))
	require.ErrorContains(t, steps.assertAPIResponse(ctx, 1), "contain post 1")
}

// cloudAPIStub serves handler as the organization's platform API endpoint.
func cloudAPIStub(t *testing.T, handler http.HandlerFunc) *frameworkruntime.Topology {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	inst, err := components.NewExternalInstance(corecatalog.CloudConsole(), 0, 1,
		map[string]string{corecatalog.EndpointAPIPPlatformAPI: server.URL})
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

	publishedResponse(t, steps, ctx, 200, `{"url":"https://echo.example.com/base/posts/1"}`)
	require.NoError(t, steps.assertEchoBackendReceived(ctx, "/posts/1"))
	require.ErrorContains(t, steps.assertEchoBackendReceived(ctx, "/posts/2"), "received path")
	publishedResponse(t, steps, ctx, 200, `{"url":"https://other.example.net/base/posts/1"}`)
	require.ErrorContains(t, steps.assertEchoBackendReceived(ctx, "/posts/1"), "echo came from host")
	publishedResponse(t, steps, ctx, 200, `{"url":"http://ECHO.example.com/base/posts/1"}`)
	require.NoError(t, steps.assertEchoBackendReceived(ctx, "/posts/1"), "the host compares without case and the scheme is not part of the check")
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

func TestHandleLengthHelpers(t *testing.T) {
	require.Equal(t, 28, gatewayHandleBudget("development"))
	require.Equal(t, 1, gatewayHandleBudget(strings.Repeat("e", 60)))

	require.Equal(t, "syn-gw-abc", handleOfLength("syn-gw-abc", 10))
	require.Equal(t, "syn-gw-abcxxxx", handleOfLength("syn-gw-abc", 14))
	require.Equal(t, "gw-abc", handleOfLength("syn-gw-abc", 6))
	require.Equal(t, "agw-abc", handleOfLength("syn-gw-abc", 7), "a handle cannot start with a hyphen")
	require.Equal(t, "", handleOfLength("syn", 0))
	for _, length := range []int{1, 5, 17, 28, 40} {
		require.Len(t, handleOfLength("syn-gw-tyc435bb-3", length), length)
	}
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

func TestCreateManagedGatewayCutsTheHandleToFitTheEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var requests []map[string]string
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var request map[string]string
		_ = json.NewDecoder(r.Body).Decode(&request)
		requests = append(requests, request)
		handle := request["handle"]
		if handle == "" {
			handle = request["displayName"]
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": request["environment"] + "-" + handle})
	})
	id, err := steps.createManagedGatewayIn(ctx, "development")
	require.NoError(t, err)
	require.NotContains(t, requests[0], "handle", "a name that fits is left to the platform to turn into a handle")
	require.LessOrEqual(t, len(id), maxGatewayHandleLength)

	longEnvironment := "syn-env-abcdefgh-123456789012"
	id, err = steps.createManagedGatewayIn(ctx, longEnvironment)
	require.NoError(t, err)
	require.LessOrEqual(t, len(id), maxGatewayHandleLength, "the registration id stays within the platform's limit")
	require.Len(t, requests[1]["handle"], gatewayHandleBudget(longEnvironment))
	require.True(t, strings.HasSuffix(requests[1]["displayName"], requests[1]["handle"]), "the handle keeps the unique tail of the name")
	require.Len(t, registry.Pending(), 2)
}

func TestVerifyManagedGatewayDeletedSaysHowTheDeleteWasAnswered(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.deleteManagedGateway(ctx))
	status, _ := requiredValue(ctx, keyGatewayDeleteStatus)
	require.Equal(t, "204", status)
	require.NoError(t, steps.verifyManagedGatewayDeleted(ctx))
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
	require.NoError(t, steps.deleteAbsentProject(ctx))
	absentAfterDelete := optionalValue(ctx, keyAbsentProjectID)
	require.NotEqual(t, absent, absentAfterDelete, "each probe names a project of its own")
	require.Equal(t, []string{"GET /projects/default", "GET /projects/" + absent, "DELETE /projects/" + absentAfterDelete}, seen)
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
		"managed-gateways/development-syn-gw-abcde234-1",
		"managed-gateways/development-syn-gw-ab",
		"projects/syn-project-abcde234-2",
		"pipelines/syn-pipeline-abcde234-1",
		"environments/syn-env-abcde234-1",
	}, stub.deleted[:7])
	require.Len(t, removed, 7)
	for _, collection := range projectScopedCollections {
		require.ElementsMatch(t, []string{"default", "syn-project-abcde234-2"}, stub.listedProjects[collection],
			collection+" is listed for the default project and the run's projects only")
	}
	for _, id := range stub.deleted {
		require.NotContains(t, id, "default")
		require.NotContains(t, id, "zzzzz222")
		require.NotContains(t, id, "ops-api")
		require.NotContains(t, id, "syn-api-abcde234-9")
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
	stub.failing["/projects/syn-project-abcde234-2"] = http.StatusConflict
	topo := cloudAPIStub(t, stub.handler)
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	removed, err := sweepLeftovers(ctx, base, 2*time.Second)
	require.ErrorContains(t, err, "/projects/syn-project-abcde234-2")
	require.Contains(t, removed, "/managed-gateways/development-syn-gw-abcde234-1")
	require.NotContains(t, removed, "/projects/syn-project-abcde234-2")
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
	}, nil)

	t.Setenv(cloudGatewayPinEnv, " dev-pinned ")
	require.NoError(t, steps.findActiveGateway(ctx))
	id, err := tcontext.ResolveString(ctx, keyGatewayID)
	require.NoError(t, err)
	require.Equal(t, "dev-pinned", id)

	t.Setenv(cloudGatewayPinEnv, "dev-missing")
	err = steps.findActiveGateway(ctx)
	require.ErrorContains(t, err, "not the gateway named by "+cloudGatewayPinEnv)
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
