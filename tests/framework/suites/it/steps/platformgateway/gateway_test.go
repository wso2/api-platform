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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
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

func TestAdminBasePathForVersion(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    string
	}{
		{version: "1.0.0", want: adminBasePathV11},
		{version: "1.1.0", want: adminBasePathV11},
		{version: "v1.1.0", want: adminBasePathV11},
		{version: "1.2.0", want: adminBasePath},
		{version: "1.2.0-SNAPSHOT", want: adminBasePath},
		{version: "", want: adminBasePath},
	} {
		t.Run(tt.version, func(t *testing.T) {
			require.Equal(t, tt.want, adminBasePathForVersion(tt.version))
		})
	}
}

func TestManagementBasePathForVersion(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    string
	}{
		{version: "1.0.0", want: managementBasePathV11},
		{version: "1.1.0", want: managementBasePathV11},
		{version: "v1.1.0", want: managementBasePathV11},
		{version: "1.2.0", want: ManagementBasePath},
		{version: "1.2.0-SNAPSHOT", want: ManagementBasePath},
		{version: "", want: ManagementBasePath},
	} {
		t.Run(tt.version, func(t *testing.T) {
			require.Equal(t, tt.want, ManagementBasePathForVersion(tt.version))
		})
	}
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
			name:    "gateway 1.0 links a policy chain directly to its route key",
			version: "1.0.0",
			body: `{
				"policy_chains":{"policy_chains":[
					{"route_key":"GET|/orders/v1/test|localhost","policies":[{"name":"set-headers"}]}
				]}
			}`,
			policyName: "set-headers",
			want:       true,
		},
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
			name:    "gateway 1.1 links a policy chain directly to its route key",
			version: "1.1.0",
			body: `{
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

func TestGatewaySpecVersionForVersion(t *testing.T) {
	require.Equal(t, gatewaySpecVersionV11, gatewaySpecVersionForVersion("1.0.0"))
	require.Equal(t, gatewaySpecVersionV11, gatewaySpecVersionForVersion("1.1.0"))
	require.Equal(t, gatewaySpecVersionV11, gatewaySpecVersionForVersion("v1.1.0"))
	require.Equal(t, gatewaySpecVersion, gatewaySpecVersionForVersion("1.2.0"))
	require.Equal(t, gatewaySpecVersion, gatewaySpecVersionForVersion("1.2.0-SNAPSHOT"))
	require.Equal(t, gatewaySpecVersion, gatewaySpecVersionForVersion(""))
}

func TestLegacyRestAPIUpstreamDefinitionMovesBasePathIntoURLs(t *testing.T) {
	definition := `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: sandbox-api
spec:
  upstreamDefinitions:
    - name: sandbox-upstream
      basePath: /sandbox
      upstreams:
        - url: http://testbench:3000
        - url: http://testbench:3001/first
`

	got, err := legacyRestAPIUpstreamDefinition(definition)
	require.NoError(t, err)
	require.NotContains(t, got, "basePath:")
	require.Contains(t, got, "url: http://testbench:3000/sandbox")
	require.Contains(t, got, "url: http://testbench:3001/sandbox/first")
}

func TestUsesLegacyUpstreamPath(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    bool
	}{
		{version: "1.0.0", want: true},
		{version: "v1.1.0", want: true},
		{version: "1.1.1", want: false},
		{version: "1.2.0", want: false},
		{version: "", want: false},
	} {
		t.Run(tt.version, func(t *testing.T) {
			require.Equal(t, tt.want, usesLegacyUpstreamPath(tt.version))
		})
	}
}

func TestLegacyLLMProviderPolicyDefinition(t *testing.T) {
	const modernDefinition = `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: provider
spec:
  operationPolicies:
    - name: set-headers
      version: v1
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              headers:
                - name: x-custom-header
                  value: test-value
`

	definition, err := legacyLLMPolicyDefinition(modernDefinition)
	require.NoError(t, err)
	require.Contains(t, definition, "policies:")
	require.NotContains(t, definition, "operationPolicies:")
	require.Contains(t, definition, "x-custom-header")

	_, err = legacyLLMPolicyDefinition(strings.Replace(modernDefinition,
		"version: v1", "version: v1\n      executionCondition: request.headers['x-test'] == 'enabled'", 1))
	require.ErrorContains(t, err, "does not support spec.operationPolicies[].executionCondition")

	_, err = legacyLLMPolicyDefinition(strings.Replace(modernDefinition,
		"spec:\n", "spec:\n  policies: []\n", 1))
	require.ErrorContains(t, err, "cannot set both spec.operationPolicies and spec.policies")
}

func TestLegacySemanticCacheDefinitionRemovesUnsupportedParameter(t *testing.T) {
	definition := `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: cache
spec:
  operations:
    - method: POST
      path: /chat
      policies:
        - name: semantic-cache
          version: v1
          params:
            similarityThreshold: 0.9
            cacheUnauthenticated: true
            jsonPath: $.messages[0].content
        - name: set-headers
          version: v1
          params:
            header: value
`

	got, err := legacySemanticCacheDefinition(definition)
	require.NoError(t, err)
	require.NotContains(t, got, "cacheUnauthenticated")
	require.Contains(t, got, "similarityThreshold: 0.9")
	require.Contains(t, got, "jsonPath: $.messages[0].content")
	require.Contains(t, got, "name: set-headers")
}

func TestLegacySemanticCacheDefinitionPreservesDefinitionWithoutUnsupportedParameter(t *testing.T) {
	definition := `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: cache
spec:
  operations:
    - method: POST
      path: /chat
      policies:
        - name: semantic-cache
          version: v1
          params:
            similarityThreshold: 0.9
`

	got, err := legacySemanticCacheDefinition(definition)
	require.NoError(t, err)
	require.Equal(t, definition, got)
}

func TestLLMPolicyContractForVersion(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    string
	}{
		{version: "1.0.0", want: "spec.policies"},
		{version: "v1.1.0", want: "spec.policies"},
		{version: "1.1.1", want: "spec.operationPolicies"},
		{version: "1.2.0", want: "spec.operationPolicies"},
		{version: "invalid", want: "spec.operationPolicies"},
	} {
		t.Run(tt.version, func(t *testing.T) {
			require.Equal(t, tt.want, llmPolicyFieldForVersion(tt.version))
		})
	}
}

func TestLegacyLLMPolicyDefinitionSupportsProviderAndProxy(t *testing.T) {
	for _, kind := range []struct {
		name string
		want bool
	}{
		{name: "LLM provider", want: true},
		{name: "LLM proxy", want: true},
		{name: "REST API", want: false},
	} {
		t.Run(kind.name, func(t *testing.T) {
			require.Equal(t, kind.want, usesLegacyLLMResource(kind.name, "1.1.0"))
			require.False(t, usesLegacyLLMResource(kind.name, "1.2.0"))
		})
	}
}

func TestGatewayMCPUpstreamPathForVersion(t *testing.T) {
	require.Empty(t, gatewayMCPUpstreamPathForVersion("1.0.0"))
	require.Empty(t, gatewayMCPUpstreamPathForVersion("1.1.0"))
	require.Empty(t, gatewayMCPUpstreamPathForVersion("v1.1.0"))
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion("1.1.1"))
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion("1.2.0"))
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion("1.2.0-SNAPSHOT"))
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion("1.1.-1"))
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion("invalid"))
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion(""))
}

func TestGatewaySpecVersionExpandsFromScenarioContext(t *testing.T) {
	ctx := tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("block")),
		tcontext.NewLocal("runner"),
	)
	require.NoError(t, tcontext.Set(ctx, keyGatewaySpecVersion, gatewaySpecVersionForVersion("1.1.0")))
	actual, err := stepscommon.Expand(ctx, "${CTX:gatewaySpecVersion}")
	require.NoError(t, err)
	require.Equal(t, gatewaySpecVersionV11, actual)
}

func TestGatewayMCPUpstreamPathExpandsFromScenarioContext(t *testing.T) {
	ctx := tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("block")),
		tcontext.NewLocal("runner"),
	)
	require.NoError(t, tcontext.Set(ctx, keyGatewayMCPUpstreamPath, gatewayMCPUpstreamPathForVersion("1.1.0")))
	actual, err := stepscommon.Expand(ctx, "http://testbench:3009${CTX:gatewayMCPUpstreamPath}")
	require.NoError(t, err)
	require.Equal(t, "http://testbench:3009", actual)
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
			name:     "pre-resource-status legacy response",
			version:  "1.0.0",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":"success"}`)},
		},
		{
			name:     "1.1 resource status",
			version:  "1.1.0",
			response: &httpx.Response{StatusCode: http.StatusCreated, Body: []byte(`{"status":{"id":"api-1","state":"deployed","createdAt":"now","updatedAt":"now"}}`)},
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
