/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package platformgateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestServiceUpstreamURLPreservesServiceBasePath(t *testing.T) {
	definition := &components.Definition{
		Name:  "platform-gateway",
		Alias: "platform-gateway",
		Endpoints: []components.Endpoint{{
			Name: "rest", Port: 9090, Scheme: "http",
		}},
	}
	instance, err := components.NewInstance(definition, 0, 1, "localhost", nil)
	require.NoError(t, err)
	instances := components.NewSet()
	require.NoError(t, instances.Add(instance))

	gateway := &Gateway{topo: &frameworkruntime.Topology{Instances: instances}}
	got, err := gateway.serviceUpstreamURL(context.Background(), "gateway-controller", "/secrets")
	require.NoError(t, err)
	require.Equal(t, "http://platform-gateway:9090/api/management/v1/secrets", got)
}

func TestConfigDumpContainsPolicy(t *testing.T) {
	const routePath = "/orders/v1/test"

	tests := []struct {
		name       string
		version    string
		body       string
		policyName string
		want       bool
	}{
		{
			name:    "gateway 1.2 links a policy chain directly to its route key",
			version: "1.2.0",
			body: `{
				"route_metadata":{"routes":[{"route_key":"GET|/orders/v1/test|localhost"}]},
				"policy_chains":{"policy_chains":[
					{"route_key":"GET|/orders/v1/test|localhost","policies":[{"name":"set-headers"}]}
				]}
			}`,
			policyName: "set-headers",
			want:       true,
		},
		{
			name:    "gateway 1.2 does not match a policy on another route",
			version: "v1.2.0",
			body: `{
				"policy_chains":{"policy_chains":[
					{"route_key":"GET|/orders/v1/other|localhost","policies":[{"name":"set-headers"}]}
				]}
			}`,
			policyName: "set-headers",
			want:       false,
		},
		{
			name:    "gateway 1.2 finds a policy on another method for the same path",
			version: "1.2.0",
			body: `{
				"policy_chains":{"policy_chains":[
					{"route_key":"GET|/orders/v1/test|localhost","policies":[{"name":"set-headers"}]},
					{"route_key":"POST|/orders/v1/test|localhost","policies":[{"name":"prompt-compressor"}]}
				]}
			}`,
			policyName: "prompt-compressor",
			want:       true,
		},
		{
			name:    "current gateway follows chain key from route metadata",
			version: "1.3.0",
			body: `{
				"route_metadata":{"routes":[{"route_key":"GET|/orders/v1/test|localhost","chain_key":"chain-123"}]},
				"policy_chains":{"policy_chains":[
					{"chain_key":"chain-123","policies":[{"name":"prompt-compressor"}]}
				]}
			}`,
			policyName: "prompt-compressor",
			want:       true,
		},
		{
			name:    "current gateway finds a policy on another method for the same path",
			version: "1.3.0",
			body: `{
				"route_metadata":{"routes":[
					{"route_key":"GET|/orders/v1/test|localhost","chain_key":"chain-get"},
					{"route_key":"POST|/orders/v1/test|localhost","chain_key":"chain-post"}
				]},
				"policy_chains":{"policy_chains":[
					{"chain_key":"chain-get","policies":[{"name":"set-headers"}]},
					{"chain_key":"chain-post","policies":[{"name":"prompt-compressor"}]}
				]}
			}`,
			policyName: "prompt-compressor",
			want:       true,
		},
		{
			name:    "gateway source snapshot follows chain key from route metadata",
			version: "1.2.0-SNAPSHOT",
			body: `{
				"route_metadata":{"routes":[{"route_key":"GET|/orders/v1/test|localhost","chain_key":"chain-123"}]},
				"policy_chains":{"policy_chains":[
					{"chain_key":"chain-123","policies":[{"name":"set-headers"}]}
				]}
			}`,
			policyName: "set-headers",
			want:       true,
		},
		{
			name:    "current gateway rejects an absent policy",
			version: "1.3.0",
			body: `{
				"route_metadata":{"routes":[{"route_key":"GET|/orders/v1/test|localhost","chain_key":"chain-123"}]},
				"policy_chains":{"policy_chains":[
					{"chain_key":"chain-123","policies":[{"name":"set-headers"}]}
				]}
			}`,
			policyName: "prompt-compressor",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dump configDump
			require.NoError(t, json.Unmarshal([]byte(tt.body), &dump))
			require.Equal(t, tt.want,
				dump.containsPolicy(configDumpSchemaFor(tt.version), routePath, tt.policyName))
		})
	}
}

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

func TestGatewayMetadataAndParsingHelpers(t *testing.T) {
	definition := "apiVersion: v1\nkind: RestApi\nmetadata: {name: api-name}\n"
	require.Equal(t, "api-name", apiNameFrom(definition))
	require.Equal(t, "RestApi", kindFromDefinition(definition))
	require.Equal(t, "api-name", handleFromDefinition(definition))
	require.Empty(t, apiNameFrom("metadata: [invalid"))
	seconds, err := parseSeconds(" 1.25 ")
	require.NoError(t, err)
	require.Equal(t, 1.25, seconds)
	for _, value := range []string{"", "-1", "NaN", "+Inf"} {
		_, err = parseSeconds(value)
		require.Error(t, err)
	}
}

func TestGatewayHTTPAndMCPParsing(t *testing.T) {
	status, err := parseHTTPStatusLine("HTTP/1.1 408 Request Timeout\r\n")
	require.NoError(t, err)
	require.Equal(t, 408, status)
	_, err = parseHTTPStatusLine("HTTP/1.1 nope")
	require.Error(t, err)
	payload := []byte(`{"jsonrpc":"2.0","id":2}`)
	require.Equal(t, payload, mcpJSONPayload([]byte("event: message\ndata: "+string(payload)+"\n\n")))
	require.Equal(t, []byte("event: message\ndata: not-json\n"), mcpJSONPayload([]byte("event: message\ndata: not-json\n")))
}

func TestGatewayLazyAndAnalyticsHelpers(t *testing.T) {
	body := []byte(`{"lazy_resources":{"resources_by_type":{"LlmProviderTemplate":[{"id":"template-b","resource":{"spec":{"displayName":"Updated"}}}],"ProviderTemplateMapping":[{"id":"provider-b","resource":{"template_handle":"custom"}}]}}}`)
	require.True(t, lazyDisplayNameMatches(body, "template-b", "Updated"))
	require.True(t, providerTemplateMappingMatches(body, "provider-b", "custom"))
	require.True(t, lazyResourceAbsent(body, "missing", "ProviderTemplateMapping"))
	require.Equal(t, "application/json", func() string {
		value, _ := analyticsHeaderValue(map[string][]string{"Content-Type": {"application/json"}}, "content-type")
		return value
	}())
	require.True(t, analyticsEventMatchesPath("/test", "/analytics/v1.0/test"))
}

func TestGatewayTemplatePathAndLiteralHelpers(t *testing.T) {
	root := t.TempDir()
	gateway := &Gateway{featureRoot: root}
	path, err := gateway.templatePath("resources/templates/rest-api.yaml")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "resources/templates/rest-api.yaml"), path)
	for _, name := range []string{"", "/tmp/api.yaml", "../api.yaml", "resources/templates/../../../api.yaml"} {
		_, err = gateway.templatePath(name)
		require.Error(t, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("kind: RestApi\n"), 0o600))
	link := filepath.Join(root, "resources", "templates", "link.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.Symlink(outside, link))
	_, err = gateway.templatePath("resources/templates/link.yaml")
	require.ErrorContains(t, err, "escapes")
	require.True(t, containsLiteralOrJSONEscaped(`Bearer {{ secret "token" }}`, `{{ secret "token" }}`))
}

func TestGatewayElapsedTolerance(t *testing.T) {
	require.Equal(t, 1900*time.Millisecond, time.Duration(2*(1-elapsedTolerance)*float64(time.Second)))
	require.Equal(t, 2100*time.Millisecond, time.Duration(2*(1+elapsedTolerance)*float64(time.Second)))
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
