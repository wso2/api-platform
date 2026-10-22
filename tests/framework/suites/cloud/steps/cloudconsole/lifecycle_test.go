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

	expired, err := forgeToken(forgeExpired, realHeader, realClaims, now)
	require.NoError(t, err)
	header, claims, err := decodeJWT(expired)
	require.NoError(t, err)
	require.Equal(t, "RS256", header["alg"])
	require.Equal(t, "real-key", header["kid"])
	require.Equal(t, "real-org", claims["ouId"])
	require.Equal(t, float64(now.Add(-time.Hour).Unix()), claims["exp"])
	require.Equal(t, "real-org", realClaims["ouId"], "the real claims are copied, not modified")
	require.Equal(t, float64(now.Add(time.Hour).Unix()), realClaims["exp"])

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
	for _, kind := range []string{forgeExpired, forgeUnsigned, forgeUnknownKey, forgeUnknownSecret} {
		require.NoError(t, steps.sendWithForgedToken(ctx, "GET", "/projects", kind), kind)
		require.True(t, strings.HasPrefix(sent, "Bearer "), kind)
		require.NotEqual(t, "Bearer "+real, sent, kind)
		_, _, err := decodeJWT(strings.TrimPrefix(sent, "Bearer "))
		require.NoError(t, err, kind)
	}
}

func TestSendWithForgedTokenNeedsTheConsoleTokenOnlyForTheExpiredKind(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	require.NoError(t, steps.sendWithForgedToken(ctx, "GET", "/projects", forgeUnknownKey))
	require.ErrorContains(t, steps.sendWithForgedToken(ctx, "GET", "/projects", forgeExpired), cloudConsoleAccessTokenEnv)
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
	for id, ok := range map[string]bool{"2026-01-31-1": true, "2026-01-31-12": true, "2026-01-31-0": false, "build-1": false, "2026-1-31-1": false} {
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

	require.NoError(t, steps.assertBuildCount(ctx, 2))
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
	require.NoError(t, steps.assertDeploymentCount(ctx, 2))
	require.Error(t, steps.assertDeploymentCount(ctx, 1))
	require.NoError(t, steps.assertDeployedBuildIsPrepared(ctx))

	deployment, err := requiredValue(ctx, keyDeploymentID)
	require.NoError(t, err)
	require.Equal(t, "dep-1", deployment)
	targets, err := recordedTargets(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"dev-default": "dep-1", "dev-extra": "dep-2"}, targets)

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

func TestWaitDeployedOnTargetsReadsEveryRequestedDeployment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var path string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]string{
			{"deploymentId": "dep-1", "status": "DEPLOYED"}, {"deploymentId": "dep-2", "status": "DEPLOYED"},
			{"deploymentId": "older", "status": "ARCHIVED"},
		}})
	})
	local.Set(keyAPIID, "syn-api-1")
	require.Error(t, steps.waitDeployedOnTargets(ctx), "no deployment was requested")
	local.Set(keyDeployedTargets, `{"gw-1":"dep-1","gw-2":"dep-2"}`)
	require.NoError(t, steps.waitDeployedOnTargets(ctx))
	require.Equal(t, "/rest-apis/syn-api-1/deployments", path)
	local.Set(keyDeployedTargets, `{}`)
	require.Error(t, steps.waitDeployedOnTargets(ctx))
}

func TestCreateGatewaysInActiveEnvironmentRegistersEachForCleanup(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var created []map[string]string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var request map[string]string
		_ = json.NewDecoder(r.Body).Decode(&request)
		created = append(created, request)
		writeJSON(w, http.StatusCreated, map[string]string{"id": request["environment"] + "-" + request["displayName"]})
	})
	require.Error(t, steps.createGatewaysInActiveEnvironment(ctx, 1), "the active gateway's environment is required")
	local.Set(keyGatewayEnv, "development")
	require.Error(t, steps.createGatewaysInActiveEnvironment(ctx, 0))

	require.NoError(t, steps.createGatewaysInActiveEnvironment(ctx, 2))
	ids := syntheticGatewayIDs(ctx)
	require.Len(t, ids, 2)
	pending := registry.Pending()
	require.Len(t, pending, 2)
	for index, request := range created {
		require.Equal(t, "development", request["environment"])
		require.Equal(t, ids[index], "development-"+request["displayName"])
		require.Equal(t, cleanup.KindGateway, pending[index].Kind)
	}
	last, _ := requiredValue(ctx, keyManagedGatewayID)
	require.Equal(t, ids[1], last)

	// A later scenario's gateways replace the earlier scenario's, which are already deleted.
	previous := ids
	require.NoError(t, steps.createGatewaysInActiveEnvironment(ctx, 1))
	ids = syntheticGatewayIDs(ctx)
	require.Len(t, ids, 1)
	require.NotContains(t, previous, ids[0])
	require.Len(t, created, 3)
	require.Len(t, registry.Pending(), 3, "every gateway created stays registered for cleanup")
}

func TestWaitSyntheticGatewaysActive(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var requested []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		writeJSON(w, http.StatusOK, map[string]any{"id": "gw", "isActive": true})
	})
	require.Error(t, steps.waitSyntheticGatewaysActive(ctx), "no synthetic gateway exists")
	local.Set(keySyntheticGateways, "gw-a,gw-b")
	require.NoError(t, steps.waitSyntheticGatewaysActive(ctx))
	require.Equal(t, []string{"/managed-gateways/gw-a", "/managed-gateways/gw-b"}, requested)
}

func TestWaitSyntheticGatewaysStopsWhenAGatewayFails(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			writeJSON(w, http.StatusOK, map[string]any{"status": map[string]any{"phase": "failed"}})
		case strings.HasSuffix(r.URL.Path, "gw-b"):
			writeJSON(w, http.StatusOK, map[string]any{"isActive": false, "status": "FAILED", "statusReason": "The gateway could not be released to its environment."})
		default:
			writeJSON(w, http.StatusOK, map[string]any{"isActive": false, "status": "PROVISIONING"})
		}
	})
	local.Set(keySyntheticGateways, "gw-a,gw-b")
	started := time.Now()
	err := steps.waitSyntheticGatewaysActive(ctx)
	require.ErrorContains(t, err, "gw-b: FAILED (The gateway could not be released to its environment.), release failed")
	require.Less(t, time.Since(started), 5*time.Second, "a failed gateway ends the wait at once")
}

func TestGatewayProgressReportsStatusAndReleasePhase(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	status, code := "PROVISIONING", http.StatusOK
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/configuration") {
			writeJSON(w, http.StatusOK, map[string]any{"status": map[string]any{"phase": "applying"}})
			return
		}
		if code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"isActive": status == "ACTIVE", "status": status})
	})
	progress, err := steps.gatewayProgressOf(ctx, "gw-a")
	require.NoError(t, err)
	require.False(t, progress.Active)
	require.Equal(t, "gw-a: PROVISIONING, release applying", progress.String())

	status = "ACTIVE"
	progress, err = steps.gatewayProgressOf(ctx, "gw-a")
	require.NoError(t, err)
	require.True(t, progress.Active)
	require.Equal(t, "gw-a: ACTIVE", progress.String(), "the release phase is read only while the gateway is not active")

	code = http.StatusNotFound
	_, err = steps.gatewayProgressOf(ctx, "gw-a")
	require.ErrorContains(t, err, "no longer exists")
	require.False(t, retry.IsTransient(err), "a vanished gateway is not worth waiting for")
	code = http.StatusBadGateway
	_, err = steps.gatewayProgressOf(ctx, "gw-a")
	require.True(t, retry.IsTransient(err))
}

func TestBoolField(t *testing.T) {
	active, err := boolField(&httpx.Response{Body: []byte(`{"isActive":true}`)}, "isActive")
	require.NoError(t, err)
	require.True(t, active)
	for _, body := range []string{`{}`, `{"isActive":"yes"}`, `nope`} {
		_, err := boolField(&httpx.Response{Body: []byte(body)}, "isActive")
		require.Error(t, err, body)
	}
}

func TestDeployBuildPerGatewayUsesTheNativeRouteOncePerGateway(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var requests []map[string]string
	status := http.StatusCreated
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest-apis/syn-api-1/deployments", r.URL.Path)
		var request map[string]string
		_ = json.NewDecoder(r.Body).Decode(&request)
		requests = append(requests, request)
		writeJSON(w, status, map[string]string{"deploymentId": "d"})
	})
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keyGatewayID, "dev-default")
	local.Set(keySyntheticGateways, "gw-a,gw-b")

	require.NoError(t, steps.deployBuildPerGateway(ctx))
	require.Len(t, requests, 3)
	names := map[string]bool{}
	for index, want := range []string{"dev-default", "gw-a", "gw-b"} {
		require.Equal(t, want, requests[index]["gatewayId"])
		require.Equal(t, "current", requests[index]["base"], "every gateway gets a build of its own")
		names[requests[index]["name"]] = true
	}
	require.Len(t, names, 3, "deployment names are distinct")

	status = http.StatusInternalServerError
	require.ErrorContains(t, steps.deployBuildPerGateway(ctx), "dev-default")
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

func TestChooseUnusedAPIHandleGeneratesAUniqueHandle(t *testing.T) {
	steps, ctx, _, registry := cloudSteps(t, nil)
	require.NoError(t, steps.chooseUnusedAPIHandle(ctx))
	first, err := requiredValue(ctx, keyAPIID)
	require.NoError(t, err)
	require.NoError(t, steps.chooseUnusedAPIHandle(ctx))
	second, _ := requiredValue(ctx, keyAPIID)
	require.NotEqual(t, first, second)
	require.Empty(t, registry.Pending(), "an API that is never created needs no cleanup")
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

func TestGatewayProgressSaysWhyTheReleasePhaseIsMissing(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	configStatus, configBody := http.StatusForbidden, `{}`
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/configuration") {
			w.WriteHeader(configStatus)
			_, _ = w.Write([]byte(configBody))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"isActive": false, "status": "DISCONNECTED"})
	})
	progress, err := steps.gatewayProgressOf(ctx, "gw-a")
	require.NoError(t, err)
	require.Equal(t, "gw-a: DISCONNECTED, release unreadable (HTTP 403)", progress.String())

	configStatus = http.StatusOK
	progress, err = steps.gatewayProgressOf(ctx, "gw-a")
	require.NoError(t, err)
	require.Equal(t, "gw-a: DISCONNECTED, release not reported", progress.String())

	configBody = `{"status":"applying"}`
	progress, err = steps.gatewayProgressOf(ctx, "gw-a")
	require.NoError(t, err)
	require.Equal(t, "gw-a: DISCONNECTED, release not reported", progress.String(), "the phase is read from status.phase")

	configBody = `{"status":{"phase":"applying","message":"waiting for 1 pod"}}`
	progress, err = steps.gatewayProgressOf(ctx, "gw-a")
	require.NoError(t, err)
	require.Equal(t, "gw-a: DISCONNECTED, release applying (waiting for 1 pod)", progress.String())
}

func listedGateway(id, environment, status string, active bool) map[string]any {
	return map[string]any{
		"id": id, "environment": environment, "status": status, "isActive": active,
		"isCritical": false, "isDefault": false,
	}
}

func TestValidateGatewayListing(t *testing.T) {
	good := []map[string]any{
		listedGateway("dev-default", "dev", "ACTIVE", true),
		listedGateway("dev-new", "dev", "PROVISIONING", false),
		listedGateway("dev-built", "dev", "DISCONNECTED", false),
	}
	good[1]["endpoints"] = []any{"https://gw.example.com"}
	good[1]["functionalityType"] = "ai"
	require.NoError(t, validateGatewayListing(good))

	failed := listedGateway("dev-failed", "dev", "FAILED", false)
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
		gateway := listedGateway("dev-x", "dev", "DISCONNECTED", false)
		mutate(gateway)
		require.Error(t, validateGatewayListing([]map[string]any{gateway}), name)
	}
}

func TestGatewayKindTreatsAMissingKindAsRegular(t *testing.T) {
	require.Equal(t, "regular", gatewayKind(map[string]any{}))
	require.Equal(t, "regular", gatewayKind(map[string]any{"functionalityType": " "}))
	require.Equal(t, "ai", gatewayKind(map[string]any{"functionalityType": "ai"}))
}

func TestPartitionByKind(t *testing.T) {
	regular := map[string]any{"id": "a", "functionalityType": "regular"}
	unset := map[string]any{"id": "b"}
	ai := map[string]any{"id": "c", "functionalityType": "ai"}
	all := []map[string]any{regular, unset, ai}
	byKind := map[string][]map[string]any{"regular": {regular, unset}, "ai": {ai}, "event": nil}
	require.NoError(t, partitionByKind(all, byKind))

	byKind["event"] = []map[string]any{ai}
	require.ErrorContains(t, partitionByKind(all, byKind), "a gateway of type")
	byKind["event"] = nil
	byKind["ai"] = []map[string]any{ai, ai}
	require.ErrorContains(t, partitionByKind(all, byKind), "listed for both")
	byKind["ai"] = []map[string]any{regular}
	require.ErrorContains(t, partitionByKind(all, byKind), "a gateway of type")
	byKind["ai"] = nil
	require.ErrorContains(t, partitionByKind(all, byKind), "in no typed list")
	byKind["ai"] = []map[string]any{ai}
	byKind["regular"] = []map[string]any{regular, unset, {"id": "d"}}
	require.ErrorContains(t, partitionByKind(all, byKind), "lack")
}

func TestDefaultGatewayOf(t *testing.T) {
	items := []map[string]any{
		{"id": "e-default", "environment": "e", "isDefault": true},
		{"id": "e-ai", "environment": "e", "isDefault": true, "functionalityType": "ai"},
		{"id": "e-two", "environment": "e", "isDefault": false},
		{"id": "other-default", "environment": "other", "isDefault": true},
	}
	id, err := defaultGatewayOf(items, "e", "regular")
	require.NoError(t, err)
	require.Equal(t, "e-default", id)
	id, err = defaultGatewayOf(items, "e", "ai")
	require.NoError(t, err)
	require.Equal(t, "e-ai", id)
	_, err = defaultGatewayOf(items, "e", "event")
	require.ErrorContains(t, err, "no default event gateway")
	items = append(items, map[string]any{"id": "e-extra", "environment": "e", "isDefault": true})
	_, err = defaultGatewayOf(items, "e", "regular")
	require.ErrorContains(t, err, "2 default regular gateways")
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

func TestExpectedGatewayID(t *testing.T) {
	require.Equal(t, "dev-my-gateway", expectedGatewayID("dev", " My Gateway ", ""))
	require.Equal(t, "dev-explicit", expectedGatewayID("dev", "My Gateway", " explicit "))
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
		"properties": map[string]any{"owner": "it"}, "version": "1.0",
	}, body)

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

func TestChooseGatewayNameForgetsEarlierScenarios(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keySyntheticGateways, "old-a,old-b")
	local.Set(keyManagedGatewayID, "old-b")
	local.Set(keyDefaultGatewayID, "old-a")
	require.NoError(t, steps.chooseGatewayName(ctx))
	name, err := requiredValue(ctx, keyManagedGatewayName)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(name, "syn-gw"))
	require.Empty(t, syntheticGatewayIDs(ctx))
	require.Empty(t, optionalValue(ctx, keyManagedGatewayID))
	require.Empty(t, optionalValue(ctx, keyDefaultGatewayID))

	other, err := freshGatewayName(ctx)
	require.NoError(t, err)
	require.NotEqual(t, name, other)
	again, _ := requiredValue(ctx, keyManagedGatewayName)
	require.Equal(t, name, again, "drawing another name leaves the scenario's own name alone")
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
	require.Equal(t, []string{"dev-syn-gw-1"}, syntheticGatewayIDs(ctx))
	require.Len(t, registry.Pending(), 1)
	id, _ := requiredValue(ctx, keyManagedGatewayID)
	require.Equal(t, "dev-syn-gw-1", id)
	env, _ := requiredValue(ctx, keyManagedGatewayEnv)
	require.Equal(t, "dev", env)

	// A repeat that is refused must not drop the registration the first request made.
	status = http.StatusConflict
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1, "the first gateway stays registered")
	require.Equal(t, []string{"dev-syn-gw-1"}, syntheticGatewayIDs(ctx))

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
	require.Equal(t, []string{"dev-syn-gw-1"}, syntheticGatewayIDs(ctx))

	// Once the gateway was deleted and deregistered, creating the name again registers it again.
	registry.Deregister(cleanup.KindGateway, "dev-syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)
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

func TestCreateGatewayBelowHandleLimitUsesTheBudget(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var handles []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		handles = append(handles, body["handle"].(string))
		writeJSON(w, http.StatusCreated, map[string]string{"id": "development-" + body["handle"].(string)})
	})
	require.Error(t, steps.createGatewayBelowHandleLimit(ctx, 0), "the active gateway's environment is required")
	local.Set(keyGatewayEnv, "development")
	for _, below := range []int{2, 1, 0} {
		require.NoError(t, steps.createGatewayBelowHandleLimit(ctx, below))
	}
	require.Len(t, handles, 3)
	for index, want := range []int{26, 27, 28} {
		require.Len(t, handles[index], want)
	}
	require.Len(t, syntheticGatewayIDs(ctx), 3)
	require.Error(t, steps.createGatewayBelowHandleLimit(ctx, 28), "no handle fits in zero characters")
}

func TestGatewayListingSteps(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var queries []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		kind := r.URL.Query().Get("functionalityType")
		items := []map[string]any{
			listedGateway("dev-default", "dev", "ACTIVE", true),
			listedGateway("dev-ai", "dev", "DISCONNECTED", false),
		}
		items[1]["functionalityType"] = "ai"
		var out []map[string]any
		for _, item := range items {
			if kind == "" || gatewayKind(item) == kind {
				out = append(out, item)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": out})
	})
	local.Set(keyGatewayID, "dev-default")

	require.NoError(t, steps.listManagedGateways(ctx))
	require.NoError(t, steps.assertListingIsValid(ctx))
	require.NoError(t, steps.assertListingIncludesActiveGateway(ctx))
	require.Error(t, steps.assertListingOnlyOfType(ctx, "regular"))

	require.NoError(t, steps.listManagedGatewaysOfType(ctx, "ai"))
	require.NoError(t, steps.assertListingOnlyOfType(ctx, "ai"))
	require.Error(t, steps.assertListingIncludesActiveGateway(ctx))
	require.Contains(t, queries, "functionalityType=ai")

	require.NoError(t, steps.assertTypesPartitionTheListing(ctx))
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

func TestUpdateManagedGatewayAndItsAssertions(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	gateway := map[string]any{
		"id": "dev-gw", "environment": "dev", "host": "gw.example.com", "displayName": "old",
		"isCritical": false, "functionalityType": "regular",
	}
	var put map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&put)
			for key, value := range put {
				gateway[key] = value
			}
		}
		writeJSON(w, http.StatusOK, gateway)
	})
	local.Set(keyManagedGatewayID, "dev-gw")
	table := valuesTable(t, []string{"displayName", "renamed"}, []string{"isCritical", "true"}, []string{"properties", `{"owner":"it"}`})
	require.NoError(t, steps.updateManagedGateway(ctx, table))
	require.Equal(t, map[string]any{"displayName": "renamed", "isCritical": true, "properties": map[string]any{"owner": "it"}}, put)

	require.NoError(t, steps.assertManagedGatewayReports(ctx,
		valuesTable(t, []string{"displayName", "renamed"}, []string{"isCritical", "true"}, []string{"properties.owner", "it"})))
	require.NoError(t, steps.assertIdentityUnchanged(ctx))

	gateway["host"] = "elsewhere.example.com"
	require.ErrorContains(t, steps.assertIdentityUnchanged(ctx), "changed host")

	require.Error(t, steps.updateManagedGateway(ctx, valuesTable(t, []string{"environment", "other"})), "identity cannot be updated")
	require.Error(t, steps.updateManagedGateway(ctx, valuesTable(t, []string{"isCritical", "perhaps"})))
}

func TestAssertBoundToEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	gateway := map[string]any{
		"id": "dev-gw", "environment": "dev", "host": "gw.example.com", "isDefault": false,
		"endpoints": []any{"https://gw.example.com"},
	}
	listedEnvironment := "dev"
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" {
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "dev-gw", "environment": listedEnvironment}}})
			return
		}
		writeJSON(w, http.StatusOK, gateway)
	})
	local.Set(keyManagedGatewayID, "dev-gw")
	local.Set(keyManagedGatewayEnv, "dev")
	require.NoError(t, steps.assertBoundToEnvironment(ctx))

	gateway["host"] = "https://gw.example.com"
	require.ErrorContains(t, steps.assertBoundToEnvironment(ctx), "without a scheme")
	gateway["host"] = "gw.example.com"
	gateway["environment"] = "prod"
	require.ErrorContains(t, steps.assertBoundToEnvironment(ctx), `environment "prod"`)
}

func TestDeleteDefaultGatewayOfSyntheticEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var deleted []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			deleted = append(deleted, r.URL.Path)
			writeJSON(w, http.StatusConflict, map[string]string{"code": "gateway_is_default", "message": "mark another"})
		case r.URL.Path == "/managed-gateways":
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
				{"id": "syn-env-default", "environment": "syn-env", "isDefault": true},
				{"id": "syn-env-gw", "environment": "syn-env", "isDefault": false},
			}})
		default:
			writeJSON(w, http.StatusOK, map[string]any{"id": "syn-env-default", "isDefault": true})
		}
	})
	require.Error(t, steps.deleteDefaultGatewayOfSyntheticEnvironment(ctx), "no synthetic environment")
	local.Set(keyEnvironmentName, "syn-env")
	require.NoError(t, steps.deleteDefaultGatewayOfSyntheticEnvironment(ctx))
	require.Equal(t, []string{"/managed-gateways/syn-env-default"}, deleted)
	require.NoError(t, steps.assertRefusedDefaultStillExists(ctx))
	require.NoError(t, steps.assertCloudErrorCode(ctx, "gateway_is_default"))
}

func TestFirstReadStatusAndProgression(t *testing.T) {
	require.NoError(t, firstReadStatus(map[string]any{"id": "g", "status": "PROVISIONING"}))
	require.NoError(t, firstReadStatus(map[string]any{"id": "g", "status": "DISCONNECTED"}))
	require.ErrorContains(t, firstReadStatus(map[string]any{"id": "g", "status": ""}), "recorded provisioning state")
	require.ErrorContains(t, firstReadStatus(map[string]any{"id": "g", "status": "FAILED", "statusReason": "boom"}), "boom")

	var progress statusProgression
	for _, status := range []string{"PROVISIONING", "PROVISIONING", "DISCONNECTED", "ACTIVE"} {
		progress.add(status)
	}
	require.Equal(t, statusProgression{"PROVISIONING", "DISCONNECTED", "ACTIVE"}, progress)
	require.NoError(t, progress.check())

	progress.add("PROVISIONING")
	require.ErrorContains(t, progress.check(), "went back to PROVISIONING")
	require.ErrorContains(t, statusProgression{"PROVISIONING", "FAILED"}.check(), "FAILED")
	require.NoError(t, statusProgression{"DISCONNECTED"}.check())
}

func TestAssertStatusProgressesToBuilt(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	statuses := []string{"PROVISIONING", "DISCONNECTED"}
	var reads int
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		status := statuses[min(reads, len(statuses)-1)]
		reads++
		writeJSON(w, http.StatusOK, map[string]any{"id": "g", "status": status})
	})
	local.Set(keyManagedGatewayID, "g")
	require.NoError(t, steps.assertStatusProgressesToBuilt(ctx))
	require.Equal(t, 2, reads)

	statuses, reads = []string{"PROVISIONING", "FAILED"}, 0
	err := steps.assertStatusProgressesToBuilt(ctx)
	require.ErrorContains(t, err, "statuses seen: PROVISIONING -> FAILED")
}

func TestGatewayConnectionAssertions(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	gateway := map[string]any{"id": "g", "isActive": false, "status": "DISCONNECTED"}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, gateway) })
	local.Set(keyManagedGatewayID, "g")

	require.NoError(t, steps.assertGatewayNotConnected(ctx))
	require.Error(t, steps.assertGatewayConnected(ctx))
	require.NoError(t, steps.assertGatewayVersion(ctx, "no"))
	require.Error(t, steps.assertGatewayVersion(ctx, "a"))

	gateway["isActive"], gateway["status"], gateway["version"] = true, "ACTIVE", "1.2.0"
	require.Error(t, steps.assertGatewayNotConnected(ctx))
	require.NoError(t, steps.assertGatewayConnected(ctx))
	require.NoError(t, steps.assertGatewayVersion(ctx, "a"))
	require.Error(t, steps.assertGatewayVersion(ctx, "no"))

	gateway["status"] = "DISCONNECTED"
	require.ErrorContains(t, steps.assertGatewayConnected(ctx), "connected but reports")
}

func TestGatewayEndpointAnswers(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	data := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	t.Cleanup(data.Close)
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": "g", "endpoints": []any{data.URL}})
	})
	steps.dataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	require.Error(t, steps.assertGatewayEndpointAnswers(ctx), "no gateway was created")
	local.Set(keySyntheticGateways, "g")
	require.NoError(t, steps.assertGatewayEndpointAnswers(ctx))

	steps.dataPlaneURLResolver = func(context.Context, string) (string, error) { return "", errors.New("unapproved") }
	require.ErrorContains(t, steps.assertGatewayEndpointAnswers(ctx), "data-plane host check")
}

func TestSyntheticGatewayDeploymentSteps(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	deployedOn := map[string]bool{}
	var deletedGateway bool
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest-apis/api-1/deployments":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			deployedOn[body["gatewayId"]] = true
			require.Equal(t, "current", body["base"])
			writeJSON(w, http.StatusCreated, map[string]string{"deploymentId": "d"})
		case r.Method == http.MethodGet && r.URL.Path == "/rest-apis/api-1/deployments":
			if deployedOn[r.URL.Query().Get("gatewayId")] {
				writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"deploymentId": "d"}}})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{}})
		case r.Method == http.MethodDelete && r.URL.Path == "/managed-gateways/gw-b":
			deletedGateway = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/managed-gateways/gw-b":
			if deletedGateway {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "gw-b"})
		case r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]string{"id": "gw-a"})
		}
	})
	local.Set(keyAPIID, "api-1")
	local.Set(keySyntheticGateways, "gw-a,gw-b")
	require.NoError(t, registry.Register(cleanup.Resource{Kind: cleanup.KindGateway, ID: "gw-b", Actor: resourceActor}))

	require.NoError(t, steps.deployBuildToSyntheticGateway(ctx, 1))
	require.NoError(t, steps.assertDeployedOnSyntheticGateway(ctx, 1, true))
	require.NoError(t, steps.assertDeployedOnSyntheticGateway(ctx, 2, false))
	require.Error(t, steps.deployBuildToSyntheticGateway(ctx, 3), "only two gateways exist")
	require.Error(t, steps.deployBuildToSyntheticGateway(ctx, 0))

	require.NoError(t, steps.assertSyntheticGatewayRetrievable(ctx, 2))
	require.NoError(t, steps.deleteSyntheticGateway(ctx, 2))
	require.True(t, deletedGateway)
	require.Empty(t, registry.Pending(), "a gateway deleted by the scenario is no longer waiting for cleanup")
	require.Error(t, steps.assertSyntheticGatewayRetrievable(ctx, 2))
	require.NoError(t, steps.assertSyntheticGatewayRetrievable(ctx, 1))
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

func TestConfigValuesFromTable(t *testing.T) {
	values, err := configValuesFromTable(valuesTable(t, []string{"a.b", "true"}, []string{"c", "text"}, []string{"d", "null"}))
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a.b": true, "c": "text", "d": nil}, values)
	_, err = configValuesFromTable(valuesTable(t, []string{"a", "1"}, []string{"a", "2"}))
	require.Error(t, err)
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
	require.Error(t, steps.readGatewayConfiguration(ctx, "active"), "no active gateway was found")
	local.Set(keyGatewayID, "dev-gw")
	require.NoError(t, steps.readGatewayConfiguration(ctx, "active"))
	require.NoError(t, steps.assertConfigurationDocument(ctx))
	require.NoError(t, steps.assertConfigurationPhase(ctx, "healthy"))
	require.ErrorContains(t, steps.assertConfigurationPhase(ctx, "applying"), "healthy")

	local.Set(keyConfigGatewayID, "another")
	require.ErrorContains(t, steps.assertConfigurationDocument(ctx), "another")
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.readGatewayConfiguration(ctx, "synthetic managed"))

	status = http.StatusNotFound
	require.NoError(t, steps.readGatewayConfiguration(ctx, "active"))
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
	require.Error(t, steps.awaitConfigurationReadable(ctx), "no gateway was created")
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.awaitConfigurationReadable(ctx))
	require.Equal(t, 2, reads, "a gateway whose release does not exist yet is waited for")
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
	local.Set(keyManagedGatewayID, "dev-gw")
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

func TestSaveGatewayConfigurationRetriesConflictsOnly(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	answers := []int{http.StatusConflict, http.StatusOK}
	var requests int
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		status := answers[min(requests, len(answers)-1)]
		requests++
		writeJSON(w, status, map[string]any{"code": "conflict", "status": map[string]string{"phase": "applying"}})
	})
	local.Set(keyManagedGatewayID, "dev-gw")
	table := valuesTable(t, []string{"gateway.config.controller.logging.level", "debug"})
	require.NoError(t, steps.saveGatewayConfiguration(ctx, table))
	require.Equal(t, 2, requests, "a conflict with the provisioning of the gateway is retried")

	answers, requests = []int{http.StatusBadRequest}, 0
	require.Error(t, steps.saveGatewayConfiguration(ctx, table))
	require.Equal(t, 1, requests, "a refusal is not retried")
}

func TestWriteGatewayConfigurationAndLongString(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var bodies []map[string]map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "no"})
	})
	require.Error(t, steps.writeGatewayConfiguration(ctx, valuesTable(t, []string{"a", "1"})), "no gateway was created")
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.writeGatewayConfiguration(ctx, valuesTable(t, []string{"a", "1"}, []string{"b", "null"})))
	require.Equal(t, map[string]any{"a": float64(1), "b": nil}, bodies[0]["values"])
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusBadRequest))

	require.NoError(t, steps.writeLongString(ctx, "gateway.config_toml", 8193))
	require.Len(t, bodies[1]["values"]["gateway.config_toml"], 8193)
	require.Error(t, steps.writeLongString(ctx, "gateway.config_toml", 0))
}

func TestConfigurationReadBackAssertions(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	values := map[string]any{"gateway.config.controller.logging.level": "debug", "gateway.config.router.access_logs.enabled": false}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": "dev-gw", "values": values})
	})
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.assertConfigurationReports(ctx, valuesTable(t,
		[]string{"gateway.config.controller.logging.level", "debug"}, []string{"gateway.config.router.access_logs.enabled", "false"})))
	require.NoError(t, steps.assertConfigurationOmits(ctx, "gateway.config.policy_engine.logging.level"))

	delete(values, "gateway.config.controller.logging.level")
	require.NoError(t, steps.assertConfigurationOmits(ctx, "gateway.config.controller.logging.level"))
}

func TestJudgeRound(t *testing.T) {
	conflict := &httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"conflict"}`)}
	ok := &httpx.Response{StatusCode: http.StatusOK}
	round := judgeRound([]*httpx.Response{ok, conflict, ok})
	require.Equal(t, []int{200, 409, 200}, round.statuses)
	require.Same(t, conflict, round.conflict)
	require.Empty(t, round.unwanted)

	round = judgeRound([]*httpx.Response{ok, ok})
	require.Nil(t, round.conflict)

	odd := &httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"other"}`)}
	round = judgeRound([]*httpx.Response{odd, {StatusCode: http.StatusInternalServerError, Method: "PUT", URL: "https://x"}})
	require.Len(t, round.unwanted, 2)
}

func TestWriteConcurrentlyStopsAtTheFirstConflict(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var mu sync.Mutex
	var values []float64
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]float64
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		values = append(values, body["values"][concurrentWritePath])
		count := len(values)
		mu.Unlock()
		if count == 3 {
			writeJSON(w, http.StatusConflict, map[string]string{"code": "conflict", "message": "re-read"})
			return
		}
		writeJSON(w, status, map[string]string{})
	})
	require.Error(t, steps.writeConcurrently(ctx, 4), "no gateway was created")
	local.Set(keyManagedGatewayID, "dev-gw")
	require.Error(t, steps.writeConcurrently(ctx, 1), "one writer cannot collide")

	require.NoError(t, steps.writeConcurrently(ctx, 4))
	require.Len(t, values, 4, "one round of four writers was enough")
	distinct := map[float64]bool{}
	for _, value := range values {
		distinct[value] = true
		require.GreaterOrEqual(t, value, float64(1000))
		require.LessOrEqual(t, value, float64(1000000), "every value is one the setting accepts")
	}
	require.Len(t, distinct, 4, "every writer sends a value of its own")
	require.NoError(t, steps.assertCloudResponseStatus(ctx, http.StatusConflict))
	require.NoError(t, steps.assertCloudErrorCode(ctx, "conflict"))

	values, status = nil, http.StatusInternalServerError
	err := steps.writeConcurrently(ctx, 2)
	require.ErrorContains(t, err, "round 1", "a fault other than a conflict ends the wait")
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
	require.Equal(t, []string{"dev-syn-gw-1", "dev-syn-gw-1-ow9j"}, syntheticGatewayIDs(ctx))
}

func TestAssertGatewayListedOnceCountsByNameInTheEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	items := []map[string]any{
		{"id": "dev-syn-gw-1", "displayName": "syn-gw-1", "environment": "dev"},
		{"id": "other-syn-gw-1", "displayName": "syn-gw-1", "environment": "other"},
	}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"list": items})
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	local.Set(keyManagedGatewayEnv, "dev")
	require.NoError(t, steps.assertGatewayListedOnce(ctx), "the same name in another environment is not a duplicate")

	items = append(items, map[string]any{"id": "dev-syn-gw-1-ow9j", "displayName": "syn-gw-1", "environment": "dev"})
	err := steps.assertGatewayListedOnce(ctx)
	require.ErrorContains(t, err, "2 managed gateways")
	require.ErrorContains(t, err, "dev-syn-gw-1-ow9j")

	items = nil
	require.ErrorContains(t, steps.assertGatewayListedOnce(ctx), "0 managed gateways")
}

func TestAssertGatewayHasItsNameAsID(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	local.Set(keyManagedGatewayName, "syn-gw-1")
	local.Set(keyManagedGatewayEnv, "dev")
	local.Set(keyManagedGatewayID, "dev-syn-gw-1")
	require.NoError(t, steps.assertGatewayHasItsNameAsID(ctx))
	local.Set(keyManagedGatewayID, "dev-syn-gw-1-ow9j")
	require.ErrorContains(t, steps.assertGatewayHasItsNameAsID(ctx), "name was not free")
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

func TestValidateGatewayListingShowsTheItemWithoutAStatus(t *testing.T) {
	err := validateGatewayListing([]map[string]any{{
		"id": "dev-orphan", "environment": "dev", "isActive": false, "isCritical": false, "isDefault": false, "status": "",
	}})
	require.ErrorContains(t, err, "the listing gives it as")
	require.ErrorContains(t, err, "dev-orphan")
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

func TestCreateSyntheticEnvironmentsNumbersThemAndClearsEarlierState(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/environments", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		writeJSON(w, http.StatusCreated, map[string]any{"id": body["name"], "name": body["name"]})
	})
	local.Set(numberedKey(keySyntheticEnvironmentPrefix, 3), "left-by-an-earlier-scenario")
	local.Set(keyDeploymentID, "left-by-an-earlier-scenario")

	require.NoError(t, steps.createSyntheticEnvironments(ctx, 2))

	first, second := optionalValue(ctx, numberedKey(keySyntheticEnvironmentPrefix, 1)), optionalValue(ctx, numberedKey(keySyntheticEnvironmentPrefix, 2))
	require.NotEmpty(t, first)
	require.NotEmpty(t, second)
	require.NotEqual(t, first, second)
	require.Equal(t, first, optionalValue(ctx, keyEnvironmentName), "the first environment is the scenario's environment")
	require.Equal(t, first+","+second, optionalValue(ctx, keySyntheticEnvironments))
	require.Empty(t, optionalValue(ctx, numberedKey(keySyntheticEnvironmentPrefix, 3)))
	require.Empty(t, optionalValue(ctx, keyDeploymentID))
	require.Len(t, registry.Pending(), 2)

	environment, err := syntheticEnvironmentAt(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, second, environment)
	_, err = syntheticEnvironmentAt(ctx, 3)
	require.ErrorContains(t, err, "created 2")

	for _, count := range []int{0, maxPromotionEnvironments + 1} {
		require.Error(t, steps.createSyntheticEnvironments(ctx, count))
	}
}

func TestCreateGatewayInEachSyntheticEnvironment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		writeJSON(w, http.StatusCreated, map[string]any{"id": body["environment"].(string) + "-" + body["displayName"].(string)})
	})
	require.ErrorContains(t, steps.createGatewayInEachSyntheticEnvironment(ctx), "no synthetic environment")

	local.Set(keySyntheticEnvironments, "env-a,env-b")
	require.NoError(t, steps.createGatewayInEachSyntheticEnvironment(ctx))
	require.Len(t, syntheticGatewayIDs(ctx), 2)
	for n, environment := range []string{"env-a", "env-b"} {
		id, err := syntheticEnvironmentGatewayAt(ctx, n+1)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(id, environment+"-"), id)
	}
	_, err := syntheticEnvironmentGatewayAt(ctx, 3)
	require.ErrorContains(t, err, "no gateway in synthetic APIP environment 3")
	require.Len(t, registry.Pending(), 2)
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

func TestReplacePipelinePathsPublishesTheAnswer(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var method, path string
	var sent map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeJSON(w, http.StatusOK, sent)
	})
	require.ErrorContains(t, steps.replacePipelinePaths(ctx, valuesTable(t, []string{"source", "targets"}, []string{"a", "b"})),
		"no deployment pipeline")

	local.Set(keyPipelineID, "syn-pipeline-1")
	require.NoError(t, steps.replacePipelinePaths(ctx, valuesTable(t, []string{"source", "targets"}, []string{"b", "a"})))
	require.Equal(t, http.MethodPut, method)
	require.Equal(t, "/pipelines/syn-pipeline-1", path)
	require.Equal(t, "syn-pipeline-1", sent["name"])
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode)
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

func TestCreateAPIInSyntheticProjectRefusesTheDefaultProject(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request may be sent")
	})
	require.ErrorContains(t, steps.createAPIInSyntheticProject(ctx, nil), "no synthetic project")
	local.Set(keyProjectID, "default")
	require.ErrorContains(t, steps.createAPIInSyntheticProject(ctx, nil), `"default"`)
}

func TestRequestProviderDeploymentUsesTheOrganisationRoute(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var path string
	var sent map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "no deployment pipeline"})
	})
	local.Set(keyGatewayEnv, "development")
	require.NoError(t, steps.requestProviderDeployment(ctx, valuesTable(t,
		[]string{"environment", "${CTX:cloudGatewayEnvironment}"}, []string{"fromEnvironment", "upstream-env"})))

	provider, err := requiredValue(ctx, keyProviderID)
	require.NoError(t, err)
	require.Equal(t, "/llm-providers/"+provider+"/environment-deployments", path)
	require.Equal(t, map[string]any{"environment": "development", "fromEnvironment": "upstream-env"}, sent)
	require.Empty(t, registry.Pending(), "the request creates nothing, so nothing is registered")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)
}

const promotionStages = `{"list":[
	{"environment":"dev","gateways":[{"gatewayId":"gw-dev","status":"DEPLOYED","buildId":"2026-01-31-1"}]},
	{"environment":"env-1","gateways":[{"gatewayId":"gw-1","status":"NOT_DEPLOYED"}]},
	{"environment":"env-2","gateways":[]}]}`

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

func TestStageLookups(t *testing.T) {
	var document struct {
		List []map[string]any `json:"list"`
	}
	require.NoError(t, json.Unmarshal([]byte(promotionStages), &document))

	entry, err := stageGateway(document.List, "dev", "gw-dev")
	require.NoError(t, err)
	require.Equal(t, "DEPLOYED", entry["status"])
	_, err = stageGateway(document.List, "dev", "gw-other")
	require.ErrorContains(t, err, `does not list gateway "gw-other"`)
	_, err = stageGateway(document.List, "missing", "gw-dev")
	require.ErrorContains(t, err, `no stage for environment "missing"`)

	gateways, err := stageOf(document.List, "env-2")
	require.NoError(t, err)
	require.Empty(t, gateways)
	_, err = stageOf(document.List, "missing")
	require.Error(t, err)
}

func TestAssertStageRunsRenderedBuild(t *testing.T) {
	steps, ctx, local := stagesStub(t, promotionStages)
	local.Set(keyBuildIDs, "2026-01-31-1")
	require.NoError(t, steps.assertStageRunsRenderedBuild(ctx, 1))
	require.ErrorContains(t, steps.assertStageRunsRenderedBuild(ctx, 4), "does not exist")
	require.ErrorContains(t, steps.assertStageRunsRenderedBuild(ctx, 3), "created no gateway in synthetic APIP environment 3")
}

func TestAssertStageRunsRenderedBuildStopsWhenTheStageIsNotListed(t *testing.T) {
	steps, ctx, local := stagesStub(t, `{"list":[{"environment":"other","gateways":[]}]}`)
	local.Set(keyBuildIDs, "2026-01-31-1")
	require.ErrorContains(t, steps.assertStageRunsRenderedBuild(ctx, 1), `no stage for environment "dev"`)
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

func TestAssertStageNoLongerDeployed(t *testing.T) {
	steps, ctx, _ := stagesStub(t, promotionStages)
	require.NoError(t, steps.assertStageNoLongerDeployed(ctx, 2))
	require.ErrorContains(t, steps.assertStageNoLongerDeployed(ctx, 3), "created no gateway")
}

func TestUndeployFromSyntheticEnvironmentUsesTheTargetDeployment(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	var method, path, environment, gateway string
	status := http.StatusNoContent
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		environment, gateway = r.URL.Query().Get("environment"), r.URL.Query().Get("gatewayId")
		w.WriteHeader(status)
	})
	local.Set(keyProjectID, "syn-project-1")
	local.Set(keyAPIID, "syn-api-1")
	local.Set(keySyntheticEnvironments, "dev,env-1")
	local.Set(numberedKey(keySyntheticGatewayPrefix, 2), "gw-1")
	require.ErrorContains(t, steps.undeployFromSyntheticEnvironment(ctx, 2), "no deployment has been requested")

	local.Set(keyDeployedTargets, `{"gw-dev":"dep-source"}`)
	require.ErrorContains(t, steps.undeployFromSyntheticEnvironment(ctx, 2), `no deployment on gateway "gw-1"`)

	local.Set(keyDeployedTargets, `{"gw-1":"dep-target"}`)
	require.NoError(t, steps.undeployFromSyntheticEnvironment(ctx, 2))
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/projects/syn-project-1/apis/syn-api-1/deployments/dep-target/undeploy", path)
	require.Equal(t, "env-1", environment)
	require.Equal(t, "gw-1", gateway)

	status = http.StatusNotFound
	require.Error(t, steps.undeployFromSyntheticEnvironment(ctx, 2), "only 204 is the documented success")
}

func TestAssertDeploymentListsGatewayOf(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"list": []map[string]any{{"gatewayId": "gw-1", "deploymentId": "dep-1"}}})
	})
	local.Set(numberedKey(keySyntheticGatewayPrefix, 1), "gw-1")
	local.Set(numberedKey(keySyntheticGatewayPrefix, 2), "gw-2")
	_, err := steps.doAuthenticated(ctx, http.MethodPost, "/anything", nil)
	require.NoError(t, err)
	require.NoError(t, steps.assertDeploymentListsGatewayOf(ctx, 1))
	require.ErrorContains(t, steps.assertDeploymentListsGatewayOf(ctx, 2), "does not list gateway")
}

func TestBodyBytesAndSizedJSONBody(t *testing.T) {
	size, err := bodyBytes(2, "MiB")
	require.NoError(t, err)
	require.Equal(t, 2<<20, size)
	size, err = bodyBytes(3, "KiB")
	require.NoError(t, err)
	require.Equal(t, 3<<10, size)
	for _, bad := range []struct {
		count int
		unit  string
	}{{0, "KiB"}, {-1, "MiB"}, {1, "GiB"}} {
		_, err := bodyBytes(bad.count, bad.unit)
		require.Error(t, err, "%d %s", bad.count, bad.unit)
	}

	for _, want := range []int{11, 1 << 10, 2 << 20} {
		body, err := sizedJSONBody(want)
		require.NoError(t, err)
		require.Len(t, body, want)
		require.True(t, json.Valid(body), "a body of %d bytes is still JSON", want)
	}
	_, err = sizedJSONBody(10)
	require.ErrorContains(t, err, "at least")
}

func TestConnectionCut(t *testing.T) {
	for _, cut := range []error{
		io.EOF, io.ErrUnexpectedEOF, errors.New("write tcp: connection reset by peer"),
		errors.New("write: broken pipe"), errors.New(`http2: stream error: stream ID 3; CANCEL`),
	} {
		require.True(t, connectionCut(cut), cut.Error())
	}
	for _, other := range []error{errors.New("no such host"), errors.New("x509: certificate signed by unknown authority"), context.DeadlineExceeded} {
		require.False(t, connectionCut(other), other.Error())
	}
}

func TestJudgeOversizedRequest(t *testing.T) {
	respond := func(status int) *httpx.Response {
		return &httpx.Response{StatusCode: status, Method: "POST", URL: "https://gw/x"}
	}
	require.NoError(t, judgeOversizedRequest(respond(http.StatusRequestEntityTooLarge), nil, 100))
	require.NoError(t, judgeOversizedRequest(respond(http.StatusBadRequest), nil, 100))
	require.NoError(t, judgeOversizedRequest(nil, io.ErrUnexpectedEOF, 100), "a connection ended before the body was accepted is a refusal")

	for _, accepted := range []int{http.StatusOK, http.StatusCreated, http.StatusFound} {
		require.ErrorContains(t, judgeOversizedRequest(respond(accepted), nil, 2<<20), "passed it on to the upstream")
	}
	for _, other := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusServiceUnavailable} {
		require.ErrorContains(t, judgeOversizedRequest(respond(other), nil, 100), "want a refusal")
	}
	require.ErrorContains(t, judgeOversizedRequest(nil, errors.New("no such host"), 100), "without the gateway refusing")
	require.ErrorContains(t, judgeOversizedRequest(nil, nil, 100), "no response")
}

// dataPlaneSteps returns steps whose data-plane requests may go to the given local server.
func dataPlaneSteps(t *testing.T, controlPlane http.HandlerFunc) (*Steps, context.Context, *tcontext.Local) {
	t.Helper()
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, controlPlane)
	steps.dataPlaneURLResolver = func(_ context.Context, target string) (string, error) { return target, nil }
	return steps, ctx, local
}

func TestAssertOversizedRefusedSendsTheBodyWholeOrChunked(t *testing.T) {
	type seen struct {
		contentLength int64
		encoding      []string
		bytes         int
	}
	received := make(chan seen, 4)
	status := http.StatusRequestEntityTooLarge
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		received <- seen{contentLength: r.ContentLength, encoding: r.TransferEncoding, bytes: len(raw)}
		w.WriteHeader(status)
	}))
	defer server.Close()
	steps, ctx, _ := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})

	require.NoError(t, steps.assertOversizedRefused(ctx, "", "POST", server.URL+"/posts", 64, "KiB"))
	whole := <-received
	require.Equal(t, int64(64<<10), whole.contentLength)
	require.Empty(t, whole.encoding)
	require.Equal(t, 64<<10, whole.bytes)

	require.NoError(t, steps.assertOversizedRefused(ctx, "chunked ", "POST", server.URL+"/posts", 64, "KiB"))
	chunked := <-received
	require.Equal(t, int64(-1), chunked.contentLength)
	require.Equal(t, []string{"chunked"}, chunked.encoding)
	require.Equal(t, 64<<10, chunked.bytes)

	status = http.StatusCreated
	require.ErrorContains(t, steps.assertOversizedRefused(ctx, "", "POST", server.URL+"/posts", 64, "KiB"), "passed it on")
	require.ErrorContains(t, steps.assertOversizedRefused(ctx, "", "POST", server.URL+"/posts", 64, "GiB"), "unsupported body size unit")
}

func TestSendSizedUntilStatusPollsWithABody(t *testing.T) {
	var mu sync.Mutex
	calls, lastBytes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls++
		attempt := calls
		lastBytes = len(raw)
		mu.Unlock()
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		if attempt == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	steps, ctx, _ := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})

	require.NoError(t, steps.sendSizedUntilStatus(ctx, "POST", server.URL, 1, "KiB", http.StatusCreated))
	require.Equal(t, 2, calls)
	require.Equal(t, 1<<10, lastBytes)
	require.Error(t, steps.sendSizedUntilStatus(ctx, "POST", server.URL, 1, "KiB", 99))
}

func TestJudgeRouterFailure(t *testing.T) {
	respond := func(status int, body string) *httpx.Response {
		return &httpx.Response{StatusCode: status, Body: []byte(body), Method: "GET", URL: "https://gw/x"}
	}
	require.NoError(t, judgeRouterFailure(respond(http.StatusServiceUnavailable, "no healthy upstream"), "backend-unavailable.invalid"))
	require.NoError(t, judgeRouterFailure(respond(http.StatusGatewayTimeout, "upstream request timeout"), ""))

	for _, status := range []int{http.StatusBadRequest, http.StatusBadGateway, http.StatusInternalServerError, http.StatusOK} {
		require.ErrorContains(t, judgeRouterFailure(respond(status, "x"), ""), "want the router's 503 or 504")
	}
	require.ErrorContains(t, judgeRouterFailure(respond(503, "goroutine 1 [running]"), ""), "exposes server detail")
	require.ErrorContains(t, judgeRouterFailure(respond(503, "java.lang.NullPointerException at x"), ""), "exposes server detail")
	require.ErrorContains(t, judgeRouterFailure(respond(503, "cannot reach Backend-Unavailable.invalid"), "backend-unavailable.invalid"), "names the backend host")
}

func TestAssertRouterFailureReadsTheScenarioBackend(t *testing.T) {
	status := http.StatusServiceUnavailable
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("no healthy upstream"))
	}))
	defer server.Close()
	steps, ctx, _ := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})
	require.NoError(t, steps.useUnreachableBackend(ctx))
	require.Equal(t, unreachableBackend, optionalValue(ctx, keyUnreachableBackendURL))

	require.NoError(t, steps.sendUntilRouted(ctx, "GET", server.URL))
	require.NoError(t, steps.assertRouterFailure(ctx))

	status = http.StatusBadRequest
	require.NoError(t, steps.sendUntilRouted(ctx, "GET", server.URL))
	require.ErrorContains(t, steps.assertRouterFailure(ctx), "want the router's 503 or 504")
}

func TestSendUntilRoutedWaitsPastNotFound(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()
		if attempt < 3 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	steps, ctx, _ := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})
	require.NoError(t, steps.sendUntilRouted(ctx, "GET", server.URL))
	require.Equal(t, 3, calls)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, published.StatusCode)
}

func TestPodsInAccessLog(t *testing.T) {
	pods, err := podsInAccessLog([]byte(`{"list":[
		{"kind":"access","podName":"runtime-b"},{"kind":"access","podName":"runtime-a"},{"kind":"access","podName":"runtime-a"},
		{"kind":"operational","podName":"controller-0"},{"kind":"access"}]}`))
	require.NoError(t, err)
	require.Equal(t, []string{"runtime-a", "runtime-b"}, pods, "only the pods named by access lines, once each")
	_, err = podsInAccessLog([]byte(`not json`))
	require.Error(t, err)
}

func TestAssertServedByManyPods(t *testing.T) {
	var mu sync.Mutex
	connections := map[string]bool{}
	body := `{"id":1}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connections[r.RemoteAddr] = true
		mu.Unlock()
		require.Equal(t, "close", r.Header.Get("Connection"), "every probe asks for a connection of its own")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	pods := `[{"kind":"access","podName":"runtime-a"},{"kind":"access","podName":"runtime-b"}]`
	logStatus := http.StatusOK
	var query string
	steps, ctx, local := dataPlaneSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/logs", r.URL.Path)
		query = r.URL.RawQuery
		if logStatus != http.StatusOK {
			writeJSON(w, logStatus, map[string]string{"code": "not_found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"list":` + pods + `}`))
	})
	local.Set(keyAPIID, "syn-api-1")

	require.NoError(t, steps.assertServedByManyPods(ctx, server.URL+"/syn-api-1/v1/posts/1", 2))
	require.Contains(t, query, "searchPhrase=syn-api-1")
	require.Contains(t, query, "kind=access")
	require.Len(t, connections, requestBurst, "each request of a burst uses a connection of its own")

	logStatus = http.StatusNotFound
	require.ErrorContains(t, steps.assertServedByManyPods(ctx, server.URL+"/x", 2), "gateway logs cannot be read")
}

func TestProbeRoutingConsistencyFailsOnAChangedAnswer(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(map[bool]string{true: `{"id":1}`, false: `{"id":2}`}[attempt < 3]))
	}))
	defer server.Close()
	steps, ctx, _ := dataPlaneSteps(t, func(http.ResponseWriter, *http.Request) {})
	request := httpx.Request{Method: http.MethodGet, URL: server.URL}

	require.ErrorContains(t, steps.probeRoutingConsistency(ctx, request, http.StatusOK), "different body")
	status = http.StatusBadGateway
	require.ErrorContains(t, steps.probeRoutingConsistency(ctx, request, http.StatusOK), "routing is inconsistent")
}

func TestAwaitConfigurationPhase(t *testing.T) {
	var mu sync.Mutex
	reads := 0
	steps, ctx, local := dataPlaneSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways/dev-gw/configuration", r.URL.Path)
		mu.Lock()
		reads++
		attempt := reads
		mu.Unlock()
		phase := "applying"
		if attempt > 1 {
			phase = "healthy"
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": map[string]string{"phase": phase}})
	})
	require.Error(t, steps.awaitConfigurationPhase(ctx, "healthy"), "no gateway has been created")
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.awaitConfigurationPhase(ctx, "healthy"))
	require.Equal(t, 2, reads)
}

func TestRecordManagedGatewayEndpoint(t *testing.T) {
	steps, ctx, local := dataPlaneSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways/dev-gw", r.URL.Path)
		writeJSON(w, http.StatusOK, map[string]any{"id": "dev-gw", "endpoints": []string{"https://gw.example.com/"}})
	})
	require.Error(t, steps.recordManagedGatewayEndpoint(ctx))
	local.Set(keyManagedGatewayID, "dev-gw")
	require.NoError(t, steps.recordManagedGatewayEndpoint(ctx))
	require.Equal(t, "https://gw.example.com", optionalValue(ctx, keyManagedGatewayEndpoint))

	local.Set(numberedKey(keySyntheticGatewayPrefix, 1), "dev-gw")
	require.NoError(t, steps.recordSyntheticEnvironmentEndpoint(ctx, 1))
	require.Equal(t, "https://gw.example.com", optionalValue(ctx, numberedKey(keySyntheticEndpointPrefix, 1)))
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

func TestDefaultGatewayEnvironmentNeedsNoActiveGateway(t *testing.T) {
	items := []map[string]any{
		{"id": "syn-env-1-default", "environment": "syn-env-1", "isActive": true},
		{"id": "development-default-ai", "environment": "development", "functionalityType": "ai"},
		{"id": "development-default", "environment": "development", "isActive": false},
		{"id": "development-syn-gw-1", "environment": "development", "isActive": true},
	}
	environment, err := defaultGatewayEnvironment(items)
	require.NoError(t, err)
	require.Equal(t, "development", environment)

	_, err = defaultGatewayEnvironment(items[:2])
	require.ErrorContains(t, err, "organisation default gateway")
}

func TestFindDefaultGatewayEnvironmentRecordsIt(t *testing.T) {
	t.Setenv(cloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
			{"id": "development-default", "environment": "development", "isActive": false, "status": "DISCONNECTED"},
		}})
	})
	require.NoError(t, steps.findDefaultGatewayEnvironment(ctx))
	require.Equal(t, "development", optionalValue(ctx, keyGatewayEnv))
}
