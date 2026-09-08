/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package platformgateway

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestHealthyStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "health JSON", body: `{"status":"healthy"}`, want: true},
		{name: "health JSON case and whitespace", body: `{"status":" HEALTHY "}`, want: true},
		{name: "plain health response", body: "healthy", want: true},
		{name: "unhealthy JSON", body: `{"status":"unhealthy"}`},
		{name: "unrelated JSON", body: `{"message":"healthy eventually"}`},
		{name: "invalid JSON containing marker", body: "not healthy yet"},
		{name: "empty response", body: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, healthyStatus([]byte(tt.body)))
		})
	}
}

func TestAssertAPICreationSucceeded(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		response *httpx.Response
		wantErr  string
	}{
		{
			name:     "current resource status",
			version:  "1.2.0-SNAPSHOT",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":{"id":"api-1","state":"deployed","createdAt":"now","updatedAt":"now"}}`)},
		},
		{
			name:     "legacy status",
			version:  "1.1.0",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":"success"}`)},
		},
		{
			name:     "missing resource status field",
			version:  "1.2.0",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":{"id":"api-1","state":"deployed","createdAt":"now"}}`)},
			wantErr:  "status.updatedAt",
		},
		{
			name:     "wrong resource state",
			version:  "1.2.0",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":{"id":"api-1","state":"pending","createdAt":"now","updatedAt":"now"}}`)},
			wantErr:  "want deployed",
		},
		{
			name:     "wrong status shape for current version",
			version:  "1.2.0",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":"success"}`)},
			wantErr:  "want resource status",
		},
		{
			name:     "unsuccessful response",
			version:  "1.2.0",
			response: &httpx.Response{StatusCode: http.StatusBadRequest, Body: []byte(`{"status":"error"}`)},
			wantErr:  "was not successful",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := assertSuccessfulAPIResponse(tt.response, tt.version, "creation")
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestAssertRetrievedAPIDetails(t *testing.T) {
	response := &httpx.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"kind":"RestApi","metadata":{"name":"api-1"},"spec":{"context":"/api-1"},"status":{"id":"api-1","state":"deployed","createdAt":"now","updatedAt":"now"}}`),
	}
	document, err := assertSuccessfulAPIResponse(response, "1.2.0-SNAPSHOT", "retrieval")
	require.NoError(t, err)
	require.NoError(t, assertRetrievedAPIDetails(document, "1.2.0-SNAPSHOT", "api-1", "/api-1", response))
	require.ErrorContains(t,
		assertRetrievedAPIDetails(document, "1.2.0-SNAPSHOT", "different", "/api-1", response),
		"metadata.name")
	require.ErrorContains(t,
		assertRetrievedAPIDetails(document, "1.2.0-SNAPSHOT", "api-1", "/different", response),
		"spec.context")
}

func TestAssertSuccessfulAPIUpdateResponse(t *testing.T) {
	response := &httpx.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"status":{"id":"api-1","state":"deployed","createdAt":"now","updatedAt":"now"}}`),
	}
	_, err := assertSuccessfulAPIResponse(response, "1.2.0-SNAPSHOT", "update")
	require.NoError(t, err)
}

func TestResponseIsHealthy(t *testing.T) {
	tests := []struct {
		name    string
		service string
		resp    *httpx.Response
		want    bool
		detail  string
	}{
		{name: "controller health payload", service: "gateway-controller", resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"status":"healthy"}`)}, want: true, detail: "status healthy"},
		{name: "policy health payload", service: "policy-engine", resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"status":"healthy"}`)}, want: true, detail: "status healthy"},
		{name: "router status is its contract", service: "router", resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte("ready")}, want: true, detail: "status 200"},
		{name: "unhealthy payload", service: "policy-engine", resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"status":"unhealthy"}`)}, detail: "body does not report status healthy"},
		{name: "malformed payload", service: "gateway-controller", resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte("not-json")}, detail: "body does not report status healthy"},
		{name: "empty payload", service: "gateway-controller", resp: &httpx.Response{StatusCode: http.StatusOK}, detail: "body does not report status healthy"},
		{name: "server failure", service: "policy-engine", resp: &httpx.Response{StatusCode: http.StatusServiceUnavailable}, detail: "status 503"},
		{name: "missing response", service: "policy-engine", detail: "no response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, detail := responseIsHealthy(tt.service, tt.resp)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.detail, detail)
		})
	}
}

func TestAllServicesHealthy(t *testing.T) {
	local := tcontext.NewLocal("health-runner")
	ctx := tcontext.WithLocal(context.Background(), local)
	steps := &Steps{}

	require.ErrorContains(t, steps.allServicesHealthy(ctx), "no health check")

	local.Set(healthResultsKey, map[string]healthResult{
		"router":             {status: http.StatusOK, healthy: true, detail: "status 200"},
		"gateway-controller": {status: http.StatusOK, healthy: true, detail: "status healthy"},
		"policy-engine":      {status: http.StatusOK, healthy: true, detail: "status healthy"},
	})
	require.NoError(t, steps.allServicesHealthy(ctx))

	local.Set(healthResultsKey, map[string]healthResult{
		"router":        {status: http.StatusServiceUnavailable, detail: "status 503"},
		"policy-engine": {err: errors.New("connection refused")},
	})
	err := steps.allServicesHealthy(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "router (status 503)")
	require.ErrorContains(t, err, "policy-engine (connection refused)")

	local.Set(healthResultsKey, map[string]bool{"router": true})
	require.ErrorContains(t, steps.allServicesHealthy(ctx), "stored as map[string]bool")
}

func TestServiceUnhealthy(t *testing.T) {
	local := tcontext.NewLocal("health-failure-runner")
	ctx := tcontext.WithLocal(context.Background(), local)
	steps := &Steps{}

	local.Set(healthResultsKey, map[string]healthResult{
		"policy-engine": {err: errors.New("connection refused")},
	})
	require.NoError(t, steps.serviceUnhealthy(ctx, "policy-engine"))

	local.Set(healthResultsKey, map[string]healthResult{
		"policy-engine": {healthy: true, detail: "status healthy"},
	})
	require.ErrorContains(t, steps.serviceUnhealthy(ctx, "policy-engine"), "is healthy")
	require.ErrorContains(t, steps.serviceUnhealthy(ctx, "router"), "did not include service")

	emptyCtx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("empty-health-runner"))
	require.ErrorContains(t, steps.serviceUnhealthy(emptyCtx, "policy-engine"), "no health check")

	local.Set(healthResultsKey, map[string]bool{"policy-engine": false})
	require.ErrorContains(t, steps.serviceUnhealthy(ctx, "policy-engine"), "stored as map[string]bool")
}
