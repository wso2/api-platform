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
			writeJSON(w, http.StatusOK, map[string]any{"status": "failed"})
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
			writeJSON(w, http.StatusOK, map[string]any{"status": "applying"})
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
}
