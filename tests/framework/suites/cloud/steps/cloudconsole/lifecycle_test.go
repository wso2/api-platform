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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
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
