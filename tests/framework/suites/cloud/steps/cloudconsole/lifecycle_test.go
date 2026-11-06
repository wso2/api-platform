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
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestGatewayDeploymentID(t *testing.T) {
	items := []map[string]any{
		{"gatewayId": "other", "deploymentId": "dep-other"},
		{"gatewayId": "gw-1", "deploymentId": "dep-1"},
	}
	got, err := gatewayDeploymentID(items, "gw-1")
	require.NoError(t, err)
	require.Equal(t, "dep-1", got)

	_, err = gatewayDeploymentID(items, "missing")
	require.ErrorContains(t, err, "no deployment was made")
	_, err = gatewayDeploymentID([]map[string]any{{"gatewayId": "gw-1"}}, "gw-1")
	require.ErrorContains(t, err, "has no deploymentId")
}

func TestCheckAwaitingConnection(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"id": "development-shgw-1", "environment": "development",
			"isActive": false, "endpoints": []any{pendingEndpoint},
		}
	}
	require.NoError(t, checkAwaitingConnection(valid(), "development-shgw-1", "development"))

	for name, mutate := range map[string]func(map[string]any){
		"wrong id":           func(d map[string]any) { d["id"] = "other" },
		"wrong environment":  func(d map[string]any) { d["environment"] = "production" },
		"active":             func(d map[string]any) { d["isActive"] = true },
		"isActive missing":   func(d map[string]any) { delete(d, "isActive") },
		"real endpoint":      func(d map[string]any) { d["endpoints"] = []any{"https://gw.example.com"} },
		"extra endpoint":     func(d map[string]any) { d["endpoints"] = []any{pendingEndpoint, "https://gw.example.com"} },
		"endpoints missing":  func(d map[string]any) { delete(d, "endpoints") },
		"isActive as string": func(d map[string]any) { d["isActive"] = "false" },
	} {
		document := valid()
		mutate(document)
		require.Error(t, checkAwaitingConnection(document, "development-shgw-1", "development"), name)
	}
}

func TestMintedTokenIDNeverQuotesTheBody(t *testing.T) {
	id, err := mintedTokenID([]byte(`{"id":"tok-1","token":"plain-text-secret"}`))
	require.NoError(t, err)
	require.Equal(t, "tok-1", id)

	for _, body := range []string{
		`{"token":"plain-text-secret"}`,
		`{"id":"tok-1","token":""}`,
		`plain-text-secret`,
	} {
		_, err := mintedTokenID([]byte(body))
		require.Error(t, err, body)
		require.NotContains(t, err.Error(), "plain-text-secret")
	}
}

func TestActiveTokenIDsFollowsTheConsoleRule(t *testing.T) {
	require.Equal(t, []string{"active-1", "no-status"}, activeTokenIDs([]map[string]any{
		{"id": "active-1", "status": "active"},
		{"id": "no-status"},
		{"id": "revoked-1", "status": "revoked"},
		{"id": "", "status": "active"},
		{"status": "active"},
	}))
	require.Empty(t, activeTokenIDs(nil))
}

func TestStillActive(t *testing.T) {
	require.Equal(t, []string{"old-2"}, stillActive([]string{"old-1", "old-2"}, []string{"new-1", "old-2"}))
	require.Empty(t, stillActive([]string{"old-1"}, []string{"new-1"}))
}

func TestSelfHostedGatewayPathsEscapeID(t *testing.T) {
	require.Equal(t, "/self-hosted-gateways/id%2Fwith%20separator", selfHostedGatewayPath("id/with separator"))
	require.Equal(t, "/self-hosted-gateways/gw-1/tokens", selfHostedGatewayTokensPath("gw-1"))
}
