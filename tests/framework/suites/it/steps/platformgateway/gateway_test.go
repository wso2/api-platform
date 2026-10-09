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
	"fmt"
	"io"
	"iter"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/a2ax"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/testbench/services/capture"
)

func TestAwaitMappedTestbenchServiceProbesMappedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/testbench/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	port := server.Listener.Addr().(*net.TCPAddr).Port
	definition := &components.Definition{
		Name:  "testbench",
		Alias: "testbench",
		Endpoints: []components.Endpoint{{
			Name: "capture", Port: capture.Port, Scheme: "http",
		}},
	}
	instance, err := components.NewInstance(definition, 0, 1, "127.0.0.1", map[int]int{capture.Port: port})
	require.NoError(t, err)
	instances := components.NewSet()
	require.NoError(t, instances.Add(instance))

	gateway := &Gateway{
		topo:   &frameworkruntime.Topology{Instances: instances},
		funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: time.Second}), 0, 0),
	}
	require.NoError(t, gateway.awaitMappedTestbenchService(context.Background(), "capture"))
}

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
				dump.containsPolicy(configDumpSchemaFor(tt.version), routePath, policyNamed(tt.policyName)))
		})
	}
}

func TestConfigDumpContainsPolicyWithParameter(t *testing.T) {
	const routePath = "/orders/v1/test"
	const current = `{
		"route_metadata":{"routes":[{"route_key":"GET|/orders/v1/test|localhost","chain_key":"chain-123"}]},
		"policy_chains":{"policy_chains":[
			{"chain_key":"chain-123","policies":[
				{"name":"set-headers","parameters":{"request":{"mode":"allow"}}},
				{"name":"analytics-header-filter","parameters":{"request":{"mode":"allow","headers":["x-first"]}}}
			]}
		]}
	}`
	const legacy = `{
		"policy_chains":{"policy_chains":[
			{"route_key":"GET|/orders/v1/test|localhost","policies":[
				{"name":"analytics-header-filter","parameters":{"request":{"mode":"deny","headers":["x-first"]}}}
			]}
		]}
	}`
	tests := []struct {
		name      string
		version   string
		body      string
		policy    string
		paramPath string
		want      string
		expected  bool
	}{
		{name: "current gateway matches a scalar parameter", version: "1.3.0", body: current,
			policy: "analytics-header-filter", paramPath: "request.mode", want: "allow", expected: true},
		{name: "current gateway matches an array element", version: "1.3.0", body: current,
			policy: "analytics-header-filter", paramPath: "request.headers.0", want: "x-first", expected: true},
		{name: "current gateway rejects a stale parameter value", version: "1.3.0", body: current,
			policy: "analytics-header-filter", paramPath: "request.headers.0", want: "x-second"},
		{name: "parameter on another policy does not match", version: "1.3.0", body: current,
			policy: "prompt-compressor", paramPath: "request.mode", want: "allow"},
		{name: "empty parameter path matches by name", version: "1.3.0", body: current,
			policy: "analytics-header-filter", expected: true},
		{name: "gateway 1.2 matches a parameter on the route-keyed chain", version: "1.2.0", body: legacy,
			policy: "analytics-header-filter", paramPath: "request.mode", want: "deny", expected: true},
		{name: "gateway 1.1 rejects a different parameter value", version: "1.1.0", body: legacy,
			policy: "analytics-header-filter", paramPath: "request.mode", want: "allow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dump configDump
			require.NoError(t, json.Unmarshal([]byte(tt.body), &dump))
			match := policyNamedWithParameter(tt.policy, tt.paramPath, tt.want)
			require.Equal(t, tt.expected, dump.containsPolicy(configDumpSchemaFor(tt.version), routePath, match))
		})
	}
}

func TestPolicyParameterValue(t *testing.T) {
	var parameters map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{
		"request":{"mode":"deny","headers":["x-first","x-second"],"empty":null},
		"limits":[{"requests":3,"enabled":true}]
	}`), &parameters))
	tests := []struct {
		name  string
		path  string
		want  string
		found bool
	}{
		{name: "nested string", path: "request.mode", want: "deny", found: true},
		{name: "array element", path: "request.headers.1", want: "x-second", found: true},
		{name: "number inside an array object", path: "limits.0.requests", want: "3", found: true},
		{name: "boolean", path: "limits.0.enabled", want: "true", found: true},
		{name: "missing key", path: "response.mode"},
		{name: "index out of range", path: "request.headers.2"},
		{name: "negative index", path: "request.headers.-1"},
		{name: "non-numeric index", path: "request.headers.first"},
		{name: "path ends at an object", path: "request"},
		{name: "path ends at an array", path: "request.headers"},
		{name: "null value", path: "request.empty"},
		{name: "path descends through a scalar", path: "request.mode.value"},
		{name: "empty segment", path: "request..mode"},
		{name: "empty path", path: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := policyParameterValue(parameters, tt.path)
			require.Equal(t, tt.found, found)
			require.Equal(t, tt.want, got)
		})
	}
	_, found := policyParameterValue(nil, "request.mode")
	require.False(t, found)
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
	require.Equal(t, "/mcp", gatewayMCPUpstreamPathForVersion("2026.09.24"))
}

func TestUsesResourceStatus(t *testing.T) {
	require.False(t, usesResourceStatus("0.9.0"))
	require.False(t, usesResourceStatus("1.0.0"))
	require.False(t, usesResourceStatus("v1.0.3"))
	require.False(t, usesResourceStatus("1.0.0-SNAPSHOT"))
	require.True(t, usesResourceStatus("1.1.0"))
	require.True(t, usesResourceStatus("v1.1.0"))
	require.True(t, usesResourceStatus("1.2.0"))
	require.True(t, usesResourceStatus("1.2.0-SNAPSHOT"))
	require.True(t, usesResourceStatus("2.0.0"))
	require.True(t, usesResourceStatus("2026.09.24"))
	require.True(t, usesResourceStatus("1.0"))
	require.True(t, usesResourceStatus("1.0.-1"))
	require.True(t, usesResourceStatus("invalid"))
	require.True(t, usesResourceStatus(""))
}

func TestGatewayContractsForCalendarVersion(t *testing.T) {
	require.False(t, usesLegacyGatewayContract("2026.09.24"))
	require.Equal(t, ManagementBasePath, ManagementBasePathForVersion("2026.09.24"))
	require.Equal(t, gatewaySpecVersion, gatewaySpecVersionForVersion("2026.09.24"))
	require.Equal(t, "spec.operationPolicies", llmPolicyFieldForVersion("2026.09.24"))
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

func TestAnalyticsHeaderValuesMatch(t *testing.T) {
	headers := map[string][]string{
		"X-Multi-Response": {"first", "second"},
		"X-Empty":          {},
		"Content-Type":     {"application/json"},
	}
	tests := []struct {
		name    string
		header  string
		want    string
		wantErr string
	}{
		{name: "exact values", header: "X-Multi-Response", want: "first,second"},
		{name: "case-insensitive name and trimmed values", header: "x-multi-response", want: " first , second "},
		{name: "single value", header: "content-type", want: "application/json"},
		{name: "wrong order", header: "X-Multi-Response", want: "second,first", wantErr: "has values"},
		{name: "missing value", header: "X-Multi-Response", want: "first", wantErr: "has values"},
		{name: "extra value", header: "X-Multi-Response", want: "first,second,third", wantErr: "has values"},
		{name: "joined value is not split", header: "X-Multi-Response", want: "first, second, extra", wantErr: "has values"},
		{name: "empty recorded values", header: "X-Empty", want: "first", wantErr: "has values"},
		{name: "absent header", header: "X-Absent", want: "first", wantErr: "is absent"},
		{name: "no expected values", header: "X-Multi-Response", want: " , ", wantErr: "no expected values"},
		{name: "empty expectation", header: "X-Multi-Response", want: "", wantErr: "no expected values"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyticsHeaderValuesMatch(headers, tt.header, tt.want)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
	require.ErrorContains(t, analyticsHeaderValuesMatch(nil, "X-Multi-Response", "first"), "is absent")
}

func analyticsTable(t *testing.T, rows ...[]string) *godog.Table {
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
		r := row{}
		for _, v := range values {
			r.Cells = append(r.Cells, cell{Value: v})
		}
		payload.Rows = append(payload.Rows, r)
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	table := &godog.Table{}
	require.NoError(t, json.Unmarshal(raw, table))
	return table
}

func TestPerRequestAnalyticsHeaders(t *testing.T) {
	local := tcontext.NewLocal("runner")
	local.Set("uid", "run")
	ctx := tcontext.WithLocal(context.Background(), local)

	got, err := perRequestAnalyticsHeaders(ctx, analyticsTable(t,
		[]string{"request", "x-tenant-data", "contain", "${CTX:uid}-data"},
		[]string{"response", "x-denied-response", "not contain", ""}))
	require.NoError(t, err)
	require.Equal(t, []perRequestAnalyticsHeader{
		{plane: "request", header: "x-tenant-data", present: true, prefix: "run-data"},
		{plane: "response", header: "x-denied-response"},
	}, got)

	tests := []struct {
		name  string
		table *godog.Table
		want  string
	}{
		{name: "nil table", want: "table is required"},
		{name: "empty table", table: &godog.Table{}, want: "table is required"},
		{name: "wrong cell count", table: analyticsTable(t, []string{"request", "x-a", "contain"}), want: "must contain"},
		{name: "bad plane", table: analyticsTable(t, []string{"body", "x-a", "contain", "p"}), want: "plane must be"},
		{name: "empty header", table: analyticsTable(t, []string{"request", " ", "contain", "p"}), want: "header name is empty"},
		{name: "bad presence", table: analyticsTable(t, []string{"request", "x-a", "have", "p"}), want: "presence must be"},
		{name: "contain without prefix", table: analyticsTable(t, []string{"request", "x-a", "contain", ""}), want: "needs a value prefix"},
		{name: "absent with prefix", table: analyticsTable(t, []string{"request", "x-a", "not contain", "p"}), want: "takes no value prefix"},
		{name: "missing context", table: analyticsTable(t, []string{"request", "x-a", "contain", "${CTX:missing}"}), want: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := perRequestAnalyticsHeaders(ctx, tt.table)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func perRequestEvent(uri string, index int, secret bool) analyticsEvent {
	event := analyticsEvent{}
	event.Request.URI = uri
	event.Request.Headers = map[string][]string{
		"X-Correlation-Id": {fmt.Sprintf("run-%d", index)},
		"X-Tenant-Data":    {fmt.Sprintf("data-%d", index)},
	}
	if secret {
		event.Request.Headers["X-Secret-Token"] = []string{"leaked"}
	}
	event.Response.Headers = map[string][]string{"X-Correlation-Response": {fmt.Sprintf("run-%d", index)}}
	return event
}

func TestVerifyPerRequestAnalyticsEvents(t *testing.T) {
	expectations := []perRequestAnalyticsHeader{
		{plane: "request", header: "x-tenant-data", present: true, prefix: "data"},
		{plane: "request", header: "x-secret-token"},
		{plane: "response", header: "x-correlation-response", present: true, prefix: "run"},
	}
	valid := func() []analyticsEvent {
		return []analyticsEvent{perRequestEvent("/p", 2, false), perRequestEvent("/p", 1, false)}
	}
	require.NoError(t, verifyPerRequestAnalyticsEvents(valid(), 2, "X-Correlation-Id", "run", expectations))

	leaked := valid()
	leaked[0].Request.Headers["X-Tenant-Data"] = []string{"data-1"}
	missing := valid()
	delete(missing[1].Response.Headers, "X-Correlation-Response")
	noKey := valid()
	delete(noKey[0].Request.Headers, "X-Correlation-Id")
	tests := []struct {
		name   string
		events []analyticsEvent
		n      int
		want   string
	}{
		{name: "too few events", events: valid()[:1], n: 2, want: "expected 2 analytics events, got 1"},
		{name: "too many events", events: valid(), n: 1, want: "expected 1 analytics events, got 2"},
		{name: "zero count", events: nil, n: 0, want: "must be positive"},
		{name: "duplicate request", events: []analyticsEvent{perRequestEvent("/p", 1, false), perRequestEvent("/p", 1, false)}, n: 2, want: "more than one"},
		{name: "out of range key", events: []analyticsEvent{perRequestEvent("/p", 3, false), perRequestEvent("/p", 1, false)}, n: 2, want: "unexpected value"},
		{name: "missing key", events: noKey, n: 2, want: "has no request header"},
		{name: "cross-request value", events: leaked, n: 2, want: `value "data-1", want "data-2"`},
		{name: "denied header present", events: []analyticsEvent{perRequestEvent("/p", 1, true)}, n: 1, want: "contains denied request header"},
		{name: "expected header absent", events: missing, n: 2, want: "does not contain response header"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyPerRequestAnalyticsEvents(tt.events, tt.n, "X-Correlation-Id", "run", expectations)
			require.ErrorContains(t, err, tt.want)
		})
	}

	badPrefix := []analyticsEvent{perRequestEvent("/p", 1, false)}
	require.ErrorContains(t, verifyPerRequestAnalyticsEvents(badPrefix, 1, "X-Correlation-Id", "other", expectations),
		"unexpected value")
}

func TestAnalyticsEventsForPath(t *testing.T) {
	events := []analyticsEvent{perRequestEvent("/a/v1/p", 1, false), perRequestEvent("/other", 2, false), perRequestEvent("/a/v1/p", 3, false)}
	matched := analyticsEventsForPath(events, "/a/v1/p")
	require.Len(t, matched, 2)
	require.Empty(t, analyticsEventsForPath(nil, "/a/v1/p"))
	require.Empty(t, analyticsEventsForPath(events, ""))
}

// Overriding a mirrored header is how the negative MCP scenarios make a header disagree with its
// body, so the scenario's value has to win outright. Without canonicalising, two spellings of one
// header survive the merge as two map entries and collapse later inside Header.Set, leaving map
// iteration order to choose - which reads as flakiness rather than as a wrong answer.
func TestScenarioHeadersWinWhateverTheirSpelling(t *testing.T) {
	stepLayer := map[string]string{
		"MCP-Protocol-Version": "2026-07-28",
		"Mcp-Name":             "add",
	}
	// The spellings a feature would plausibly write: Go's canonical form of the first, and a
	// lowercase second.
	scenarioLayer := map[string]string{
		"Mcp-Protocol-Version": "2025-06-18",
		"mcp-name":             "echo",
	}

	merged := mergeCanonicalHeaders(stepLayer, scenarioLayer)

	require.Len(t, merged, 2, "one entry per header, whatever spelling reached it")
	require.Equal(t, "2025-06-18", merged["Mcp-Protocol-Version"])
	require.Equal(t, "echo", merged["Mcp-Name"])
}

// A 2026-07-28 server answers -32602 when any of the three _meta members is missing, so the
// envelope is asserted here rather than discovered as a puzzling failure inside a scenario.
func TestMcpModernBodyCarriesTheRequiredMetaMembers(t *testing.T) {
	var named map[string]any
	require.NoError(t, json.Unmarshal([]byte(mcpModernBody("tools/call", "echo", "2026-07-28")), &named))

	params := named["params"].(map[string]any)
	require.Equal(t, "echo", params["name"])
	require.Equal(t, "Hello, World!", params["arguments"].(map[string]any)["message"])

	meta := params["_meta"].(map[string]any)
	require.Equal(t, "2026-07-28", meta["io.modelcontextprotocol/protocolVersion"])
	require.Contains(t, meta, "io.modelcontextprotocol/clientInfo")
	require.Contains(t, meta, "io.modelcontextprotocol/clientCapabilities")

	// A resource is identified by uri, never by name, and that member is what a conformant
	// server compares the mirrored Mcp-Name against.
	var resource map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(mcpModernBody("resources/read", "file:///a.txt", "2026-07-28")), &resource))
	resourceParams := resource["params"].(map[string]any)
	require.Equal(t, "file:///a.txt", resourceParams["uri"])
	require.NotContains(t, resourceParams, "name")

	// A method that names no capability carries the same _meta and no name, which is what keeps
	// Mcp-Name off the request as well.
	var unnamed map[string]any
	require.NoError(t, json.Unmarshal([]byte(mcpModernBody("tools/list", "", "2026-07-28")), &unnamed))
	unnamedParams := unnamed["params"].(map[string]any)
	require.NotContains(t, unnamedParams, "name")
	require.Contains(t, unnamedParams["_meta"], "io.modelcontextprotocol/protocolVersion")
}

// MCP analytics nests its fields under mcpAnalytics, so the metadata step resolves a dotted path.
// A name with no dot must keep working, since every other event's fields are flat.
func TestAnalyticsMetadataResolvesNestedPaths(t *testing.T) {
	metadata := map[string]any{
		"apiName": "mcp-proxy",
		"mcpAnalytics": map[string]any{
			"jsonRpcMethod":  "tools/call",
			"capabilityName": "add",
			"capability":     "TOOL",
		},
	}

	value, ok := traverseJSON(metadata, "mcpAnalytics.jsonRpcMethod")
	require.True(t, ok)
	require.Equal(t, "tools/call", value)

	value, ok = traverseJSON(metadata, "apiName")
	require.True(t, ok)
	require.Equal(t, "mcp-proxy", value)

	_, ok = traverseJSON(metadata, "mcpAnalytics.missing")
	require.False(t, ok)
	_, ok = traverseJSON(metadata, "apiName.jsonRpcMethod")
	require.False(t, ok)
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

// Every controller collection a step can create into has a cleanup kind, so a created resource
// is always registered; the Agent collection maps to the gateway's own Agent kind.
func TestControllerResourceKindsHaveCleanupKinds(t *testing.T) {
	for stepKind, spec := range resourceKinds {
		if spec.collection == "" {
			continue // the API handlers register and deregister their own cleanup
		}
		_, ok := cleanupKindForCollection(spec.collection)
		require.Truef(t, ok, "resource kind %q (collection %q) has no cleanup kind", stepKind, spec.collection)
	}

	agent, ok := resourceKinds["Agent"]
	require.True(t, ok, "the Agent step kind is registered")
	require.Equal(t, "Agent", agent.declared)
	require.Equal(t, "/agents", agent.collection)
	kind, ok := cleanupKindForCollection(agent.collection)
	require.True(t, ok)
	require.Equal(t, cleanup.KindAgent, kind)

	_, ok = cleanupKindForCollection("/not-a-collection")
	require.False(t, ok)
}

// agentTestBase is a Base whose data plane is one test server.
type agentTestBase struct {
	url     string
	headers map[string]string
}

func (b *agentTestBase) FeatureRoot() string                    { return "" }
func (b *agentTestBase) GatewayURL(path string) (string, error) { return b.url + path, nil }
func (b *agentTestBase) GatewayURLAt(_, path string) (string, error) {
	return b.url + path, nil
}
func (b *agentTestBase) ScenarioHeaders(context.Context) map[string]string {
	out := map[string]string{}
	for k, v := range b.headers {
		out[k] = v
	}
	return out
}
func (b *agentTestBase) InvokeWith(context.Context, string, string, map[string]string, []byte) error {
	return nil
}
func (b *agentTestBase) RequestHost(context.Context) string { return "" }
func (b *agentTestBase) ResetRequest(context.Context) error { return nil }
func (b *agentTestBase) SendUntilHeader(context.Context, string, string, string, string) error {
	return nil
}

func agentTestContext() context.Context {
	return tcontext.WithLocal(context.Background(), tcontext.NewLocal("agent-runner"))
}

func agentTestGateway(url string, headers map[string]string) *Gateway {
	return &Gateway{
		base:   &agentTestBase{url: url, headers: headers},
		funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0),
	}
}

func TestA2ASSEData(t *testing.T) {
	for line, want := range map[string]string{"data: {\"a\":1}": `{"a":1}`, "data:x": "x"} {
		data, ok := a2aSSEData(line)
		require.True(t, ok, line)
		require.Equal(t, want, data)
	}
	for _, framing := range []string{"", ": ping", "event: message", "id: 3", "data:   ", "retry: 10"} {
		_, ok := a2aSSEData(framing)
		require.False(t, ok, framing)
	}
}

func TestOpenA2AStreamCapturesPacedEventsAndPublishes(t *testing.T) {
	var gotAccept, gotType, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept, gotType, gotAuth = r.Header.Get("Accept"), r.Header.Get("Content-Type"), r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i, state := range []string{"TASK_STATE_WORKING", "TASK_STATE_WORKING", "TASK_STATE_COMPLETED"} {
			if i > 0 {
				time.Sleep(60 * time.Millisecond)
			}
			_, _ = w.Write([]byte(": ping\ndata: {\"state\":\"" + state + "\"}\n\n"))
			flusher.Flush()
		}
	}))
	t.Cleanup(server.Close)

	ctx := agentTestContext()
	require.NoError(t, tcontext.Set(ctx, "task", "t-1"))
	g := agentTestGateway(server.URL, map[string]string{"Authorization": "Bearer x"})

	require.NoError(t, g.openA2AStream(ctx, "", "post", "/agent/v1/tasks/${CTX:task}:subscribe",
		&godog.DocString{Content: `{"id":"${CTX:task}"}`}))
	require.Equal(t, "text/event-stream", gotAccept)
	require.Equal(t, "application/json", gotType)
	require.Equal(t, "Bearer x", gotAuth)
	require.Equal(t, `{"id":"t-1"}`, gotBody)

	require.NoError(t, g.a2aStreamAtLeast(ctx, 3))
	require.ErrorContains(t, g.a2aStreamAtLeast(ctx, 4), "expected at least 4")
	require.NoError(t, g.a2aStreamFirstBeforeLast(ctx))
	require.NoError(t, g.a2aStreamLastContains(ctx, "COMPLETED"))
	require.ErrorContains(t, g.a2aStreamLastContains(ctx, "WORKING"), "last stream event did not contain")

	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode)
	require.Contains(t, published.Headers.Get("Transfer-Encoding"), "chunked")
	require.Contains(t, published.Text(), "TASK_STATE_COMPLETED")

	require.NoError(t, g.openA2AStream(ctx, "1", "GET", "/agent/v1/tasks", nil))
	require.Empty(t, gotType, "a stream without a body sends no content type")
	stream, err := a2aStreamOf(ctx)
	require.NoError(t, err)
	require.Len(t, stream.Events, 1, "a bounded read stops at the requested count")
	require.ErrorContains(t, g.a2aStreamFirstBeforeLast(ctx), "need at least 2 events")
}

func TestOpenA2AStreamRejectsBadInputAndFailures(t *testing.T) {
	ctx := agentTestContext()
	g := agentTestGateway("http://127.0.0.1:1", nil)
	for _, count := range []string{"0", "-2", "many"} {
		require.ErrorContains(t, g.openA2AStream(ctx, count, "GET", "/x", nil), "positive integer")
	}
	require.Error(t, g.openA2AStream(ctx, "", "GET", "/x", nil))
	_, err := a2aStreamOf(ctx)
	require.ErrorContains(t, err, "no A2A stream has been opened")
	require.Error(t, g.a2aStreamAtLeast(ctx, 1))
	require.Error(t, g.a2aStreamFirstBeforeLast(ctx))
	require.Error(t, g.a2aStreamLastContains(ctx, "x"))
	require.Error(t, g.openA2AStream(ctx, "", "GET", "${CTX:missing}", nil))
}

func TestA2AStreamDetectsABufferedResponse(t *testing.T) {
	ctx := agentTestContext()
	g := &Gateway{}
	require.NoError(t, tcontext.Set(ctx, keyA2AStream, &a2aStream{Events: []a2aStreamEvent{
		{Data: "a", Offset: time.Second}, {Data: "b", Offset: time.Second},
	}}))
	require.ErrorContains(t, g.a2aStreamFirstBeforeLast(ctx), "one buffered unit")
	require.NoError(t, tcontext.Set(ctx, keyA2AStream, &a2aStream{}))
	require.ErrorContains(t, g.a2aStreamLastContains(ctx, "a"), "no events")
	require.Equal(t, "stream delivered no events", (&a2aStream{}).summary())
	require.NoError(t, tcontext.Set(ctx, keyA2AStream, "not a stream"))
	_, err := a2aStreamOf(ctx)
	require.ErrorContains(t, err, "not a stream")
}

func TestA2AAnalyticsAssertions(t *testing.T) {
	block := map[string]any{
		"operation": "SendMessage", "transport": "JSONRPC", "agent_id": "a", "agent_name": "n",
		"response": map[string]any{"task_state": "TASK_STATE_COMPLETED", "task_id": "t-1", "empty": ""},
		"request":  map[string]any{"input_part_count": float64(2)},
	}
	require.NoError(t, a2aAnalyticsAssert(block, "have", "operation", "SendMessage"))
	require.NoError(t, a2aAnalyticsAssert(block, "have", "request.input_part_count", "2"))
	require.ErrorContains(t, a2aAnalyticsAssert(block, "have", "operation", "GetTask"), `to be "GetTask"`)
	require.ErrorContains(t, a2aAnalyticsAssert(block, "have", "response.missing", "x"), "not found")
	require.ErrorContains(t, a2aAnalyticsAssert(block, "have", "nope.task_state", "x"), `no "nope" sub-block`)
	require.NoError(t, a2aAnalyticsAssert(block, "not have", "request.history_length", ""))
	require.ErrorContains(t, a2aAnalyticsAssert(block, "not have", "operation", ""), "present with value")
	require.NoError(t, a2aAnalyticsAssert(block, "have a non-empty", "response.task_id", ""))
	require.ErrorContains(t, a2aAnalyticsAssert(block, "have a non-empty", "response.empty", ""), "present but empty")
	require.Error(t, a2aAnalyticsAssert(block, "have a non-empty", "response.absent", ""))
	require.ErrorContains(t, a2aAnalyticsAssert(block, "count", "operation", ""), "unsupported")

	card := map[string]any{"operation": "Unknown", "transport": "UNKNOWN", "agent_id": "a", "agent_name": "n", "request_type": "agentCard"}
	require.NoError(t, a2aAnalyticsAssert(card, "carry only", "request_type", ""))
	require.ErrorContains(t, a2aAnalyticsAssert(block, "carry only", "request_type", ""), "catch-all")
	leaky := map[string]any{"operation": "Unknown", "transport": "UNKNOWN", "agent_id": "a", "agent_name": "n",
		"request_type": "agentCard", "outcome": "SUCCESS"}
	require.ErrorContains(t, a2aAnalyticsAssert(leaky, "carry only", "request_type", ""), "carry only")
	anonymous := map[string]any{"operation": "Unknown", "transport": "UNKNOWN", "agent_id": "", "agent_name": "n", "request_type": "x"}
	require.ErrorContains(t, a2aAnalyticsAssert(anonymous, "carry only", "request_type", ""), "agent identity")
	missing := map[string]any{"operation": "Unknown", "transport": "UNKNOWN", "agent_id": "a", "agent_name": "n"}
	require.Error(t, a2aAnalyticsAssert(missing, "carry only", "request_type", ""))
}

func TestA2AAnalyticsBlockRejectsRetiredShapes(t *testing.T) {
	_, err := a2aAnalyticsBlock(&analyticsEvent{Metadata: map[string]any{"agentAnalytics": map[string]any{}}, A2A: map[string]any{}})
	require.ErrorContains(t, err, "retired metadata key")
	_, err = a2aAnalyticsBlock(&analyticsEvent{Metadata: map[string]any{"apiName": "x"}})
	require.ErrorContains(t, err, "no a2a block (metadata keys: apiName)")
	block, err := a2aAnalyticsBlock(&analyticsEvent{A2A: map[string]any{"operation": "GetTask"}})
	require.NoError(t, err)
	require.Equal(t, "GetTask", block["operation"])
	require.Equal(t, "none", sortedAnyKeys(nil))
}

func TestAnalyticsA2AFieldValidatesTheValueClause(t *testing.T) {
	g := &Gateway{}
	ctx := agentTestContext()
	require.ErrorContains(t, g.analyticsA2AField(ctx, "/p", "have", "operation", ""), "requires a value")
	require.ErrorContains(t, g.analyticsA2AField(ctx, "/p", "not have", "operation", "x"), "takes no value")
}

func TestA2ACredentialsCarryOnlyCredentialHeaders(t *testing.T) {
	got := a2aCredentials(map[string]string{
		"authorization": "Bearer t", "API-KEY": "k", "Accept": "application/json",
		"A2A-Version": "0.3", "X-API-Key": "", "Content-Type": "text/plain",
	})
	require.Equal(t, a2ax.Headers{"Authorization": "Bearer t", "API-Key": "k"}, got)
	require.Empty(t, a2aCredentials(nil))
}

func TestA2ASessionBookkeeping(t *testing.T) {
	session := newA2ASession()
	_, err := session.client("rpc")
	require.ErrorContains(t, err, "created: none")
	_, err = session.requireTaskID()
	require.ErrorContains(t, err, "no A2A task")

	first, err := a2ax.NewClient(context.Background(), a2ax.BindingJSONRPC, "http://127.0.0.1:1/a", a2ax.Options{ProtocolVersion: "1.0"})
	require.NoError(t, err)
	require.NoError(t, session.put("rpc", first))
	second, err := a2ax.NewClient(context.Background(), a2ax.BindingHTTPJSON, "http://127.0.0.1:1/a/v1", a2ax.Options{ProtocolVersion: "1.0"})
	require.NoError(t, err)
	require.NoError(t, session.put("rpc", second), "a reused name replaces the earlier client")
	require.Equal(t, []string{"rpc"}, session.names())

	client, err := session.client("rpc")
	require.NoError(t, err)
	_, _, err = session.outcome("rpc")
	require.ErrorContains(t, err, "has not made a call")

	session.record(client, &a2ax.Outcome{Method: "SendMessage", Task: &a2ax.Task{ID: "t-1"}})
	taskID, err := session.requireTaskID()
	require.NoError(t, err)
	require.Equal(t, "t-1", taskID)
	session.record(client, &a2ax.Outcome{Method: "ListTasks", Tasks: []a2ax.Task{}})
	taskID, _ = session.requireTaskID()
	require.Equal(t, "t-1", taskID, "a call naming no task keeps the scenario's task")

	session.record(client, &a2ax.Outcome{Method: "GetTask", Err: errors.New("refused")})
	_, err = session.succeeded("rpc")
	require.ErrorContains(t, err, "GetTask over HTTP+JSON through http://127.0.0.1:1/a/v1 failed: refused")

	require.NoError(t, session.close())
	require.Empty(t, session.names())
	_, err = session.requireTaskID()
	require.Error(t, err)

	ctx := agentTestContext()
	_, err = a2aSessionOf(ctx)
	require.ErrorContains(t, err, "no A2A session")
	require.NoError(t, tcontext.Set(ctx, keyA2ASession, "wrong"))
	_, err = a2aSessionOf(ctx)
	require.ErrorContains(t, err, "not an A2A session")
}

func TestA2AGatewayEndpointAcceptsOnlyGatewayPaths(t *testing.T) {
	ctx := agentTestContext()
	require.NoError(t, tcontext.Set(ctx, "agentContext", "/agent-x"))
	g := agentTestGateway("http://gateway:8080", nil)
	url, err := g.a2aGatewayEndpoint(ctx, "${CTX:agentContext}/v1")
	require.NoError(t, err)
	require.Equal(t, "http://gateway:8080/agent-x/v1", url)
	_, err = g.a2aGatewayEndpoint(ctx, "http://a2a-trip-planner:9099")
	require.ErrorContains(t, err, "must be a gateway path")
	_, err = g.a2aGatewayEndpoint(ctx, "${CTX:missing}")
	require.Error(t, err)
}

// tripAgent is a minimal in-process agent on the reference server SDK: every message completes
// with an artifact echoing its text, and every call is made by one authenticated user.
func tripAgent(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	executor := a2asrv.AgentExecutorFunc(func(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
		return func(yield func(a2a.Event, error) bool) {
			if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
				return
			}
			text := ""
			for _, part := range execCtx.Message.Parts {
				text += part.Text()
			}
			if !yield(a2a.NewArtifactEvent(execCtx, a2a.NewTextPart("plan: "+text)), nil) {
				return
			}
			yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil), nil)
		}
	})
	handler := a2asrv.NewHandler(executor,
		a2asrv.WithExtendedAgentCard(&a2a.AgentCard{Name: "Trip Planner", Skills: []a2a.AgentSkill{{ID: "plan_trip"}}}))
	mux := http.NewServeMux()
	mux.Handle("/agent", a2asrv.NewJSONRPCHandler(handler))
	mux.Handle("/agent/v1/", http.StripPrefix("/agent/v1", a2asrv.NewRESTHandler(handler)))
	var auth []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agent/.well-known/agent-card.json" {
			_, _ = w.Write([]byte(`{"name":"Trip Planner","supportedInterfaces":[` +
				`{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"` + "http://" + r.Host + `/agent"}]}`))
			return
		}
		mu.Lock()
		auth = append(auth, r.Header.Get("Authorization"))
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &auth
}

func TestA2AClientStepsDriveBothBindingsWithoutPublishingResponses(t *testing.T) {
	server, auth := tripAgent(t)
	ctx := agentTestContext()
	require.NoError(t, tcontext.Set(ctx, keyA2ASession, newA2ASession()))
	require.NoError(t, tcontext.Set(ctx, "agentContext", "/agent"))
	g := agentTestGateway(server.URL, map[string]string{"Authorization": "Bearer jwt", "A2A-Version": "0.3"})

	require.NoError(t, g.createA2AClient(ctx, "rpc", a2ax.BindingJSONRPC, "${CTX:agentContext}"))
	require.NoError(t, g.createA2AClient(ctx, "rest", a2ax.BindingHTTPJSON, "${CTX:agentContext}/v1"))
	require.ErrorContains(t, g.createA2AClient(ctx, " ", a2ax.BindingJSONRPC, "/agent"), "needs a name")
	require.NoError(t, g.a2aClientTalksTo(ctx, "rest", "${CTX:agentContext}/v1"))
	require.ErrorContains(t, g.a2aClientTalksTo(ctx, "rest", "/elsewhere"), "is talking to")

	require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{StatusCode: 200}))
	for _, name := range []string{"rpc", "rest"} {
		require.NoError(t, g.a2aSendMessage(ctx, name, "Kandy", ""))
		require.NoError(t, g.a2aCallResult(ctx, name, "succeeded"))
		require.ErrorContains(t, g.a2aCallResult(ctx, name, "failed"), "to be refused")
		require.NoError(t, g.a2aTaskState(ctx, name, "TASK_STATE_COMPLETED"))
		require.ErrorContains(t, g.a2aTaskState(ctx, name, "TASK_STATE_WORKING"), "to be in state")
		require.ErrorContains(t, g.a2aTaskRunning(ctx, name), "terminal state")
		require.NoError(t, g.a2aArtifactContains(ctx, name, "plan: Kandy"))
		require.ErrorContains(t, g.a2aArtifactContains(ctx, name, "Galle"), "expected an artifact")
	}
	_, err := httpx.Published(ctx)
	require.Error(t, err, "an SDK call must clear the published response")
	require.NoError(t, g.a2aSameArtifact(ctx, "rpc", "rest"))
	require.Contains(t, *auth, "Bearer jwt")

	require.NoError(t, g.a2aActOnTask(ctx, "rest", "gets"))
	require.NoError(t, g.a2aTaskState(ctx, "rest", "TASK_STATE_COMPLETED"))
	require.NoError(t, g.a2aStreamMessage(ctx, "rpc", "Ella"))
	require.NoError(t, g.a2aStreamEventCount(ctx, "rpc", 2))
	require.ErrorContains(t, g.a2aStreamEventCount(ctx, "rpc", 50), "expected at least 50")
	require.NoError(t, g.a2aStreamEndState(ctx, "rpc", "TASK_STATE_COMPLETED"))
	require.ErrorContains(t, g.a2aStreamEndState(ctx, "rpc", "TASK_STATE_FAILED"), "to end in state")
	require.NoError(t, g.a2aStreamContains(ctx, "rpc", "plan: Ella"))
	require.ErrorContains(t, g.a2aStreamContains(ctx, "rpc", "Galle"), "no stream event contained")
	require.ErrorContains(t, g.a2aTaskState(ctx, "rpc", "x"), "returned no task")

	require.NoError(t, g.a2aExtendedCard(ctx, "rest"))
	require.NoError(t, g.a2aCardName(ctx, "rest", "Trip Planner"))
	require.ErrorContains(t, g.a2aCardName(ctx, "rest", "Other"), "named")
	require.NoError(t, g.a2aCardSkill(ctx, "rest", "with", "plan_trip"))
	require.NoError(t, g.a2aCardSkill(ctx, "rest", "without", "extended_only"))
	require.ErrorContains(t, g.a2aCardSkill(ctx, "rest", "with", "extended_only"), "to declare skill")
	require.ErrorContains(t, g.a2aCardSkill(ctx, "rest", "without", "plan_trip"), "must not")

	require.NoError(t, g.a2aPushConfig(ctx, "rest", "creates", "push-1"))
	require.NoError(t, g.a2aCallResult(ctx, "rest", "failed"), "the agent was built without push support")
	require.ErrorContains(t, g.a2aSubscribe(ctx, "rest", 0), "must be positive")
	require.ErrorContains(t, g.a2aSendMessage(ctx, "missing", "x", ""), "no A2A client named")
	require.ErrorContains(t, g.a2aPushConfigIs(ctx, "rpc", "push-1"), "no push notification config")
	require.ErrorContains(t, g.a2aPushConfigCount(ctx, "rpc", 0), "no push notification config list")
	require.ErrorContains(t, g.a2aCardName(ctx, "rpc", "Trip Planner"), "no Agent Card")
	require.ErrorContains(t, g.a2aStreamPaced(ctx, "rest"), "push") // the failed call is reported first

	require.NoError(t, g.createA2AClientFromCard(ctx, "card", a2ax.BindingJSONRPC, "${CTX:agentContext}/.well-known/agent-card.json"))
	require.NoError(t, g.a2aClientTalksTo(ctx, "card", "/agent"))
	require.Error(t, g.createA2AClientFromCard(ctx, "card", a2ax.BindingHTTPJSON, "/agent/.well-known/agent-card.json"))
	require.Error(t, g.createA2AClientFromCard(ctx, "card", a2ax.BindingJSONRPC, "/missing"))

	session, err := a2aSessionOf(ctx)
	require.NoError(t, err)
	require.NoError(t, session.close())
}

func TestA2ATaskListedAndPushConfigAssertions(t *testing.T) {
	ctx := agentTestContext()
	session := newA2ASession()
	require.NoError(t, tcontext.Set(ctx, keyA2ASession, session))
	client, err := a2ax.NewClient(context.Background(), a2ax.BindingJSONRPC, "http://127.0.0.1:1", a2ax.Options{ProtocolVersion: "1.0"})
	require.NoError(t, err)
	require.NoError(t, session.put("rpc", client))
	t.Cleanup(func() { _ = session.close() })
	entry, _ := session.client("rpc")
	g := &Gateway{}

	session.record(entry, &a2ax.Outcome{Method: "SendMessage", Task: &a2ax.Task{ID: "t-2", State: "TASK_STATE_WORKING"}})
	require.NoError(t, g.a2aTaskRunning(ctx, "rpc"))
	session.record(entry, &a2ax.Outcome{Method: "ListTasks", Tasks: []a2ax.Task{{ID: "t-1"}, {ID: "t-2"}}})
	require.NoError(t, g.a2aTaskListed(ctx, "rpc"))
	session.record(entry, &a2ax.Outcome{Method: "ListTasks", Tasks: []a2ax.Task{{ID: "t-1"}}})
	require.ErrorContains(t, g.a2aTaskListed(ctx, "rpc"), "absent from the 1 listed")

	session.record(entry, &a2ax.Outcome{Method: "GetTaskPushNotificationConfig", PushConfig: &a2ax.PushConfig{ID: "p-1"}})
	require.NoError(t, g.a2aPushConfigIs(ctx, "rpc", "p-1"))
	require.ErrorContains(t, g.a2aPushConfigIs(ctx, "rpc", "p-2"), `expected push notification config "p-2"`)
	session.record(entry, &a2ax.Outcome{Method: "ListTaskPushNotificationConfigs", PushConfigs: []a2ax.PushConfig{{ID: "p-1"}}})
	require.NoError(t, g.a2aPushConfigCount(ctx, "rpc", 1))
	require.ErrorContains(t, g.a2aPushConfigCount(ctx, "rpc", 0), "expected 0")

	session.record(entry, &a2ax.Outcome{Method: "SubscribeToTask", Events: []a2ax.Event{{Offset: time.Second}, {Offset: time.Second}}})
	require.ErrorContains(t, g.a2aStreamPaced(ctx, "rpc"), "one buffered unit")
	session.record(entry, &a2ax.Outcome{Method: "SubscribeToTask", Events: []a2ax.Event{{Offset: 0}, {Offset: time.Second}}})
	require.NoError(t, g.a2aStreamPaced(ctx, "rpc"))
	session.record(entry, &a2ax.Outcome{Method: "SubscribeToTask", Events: []a2ax.Event{{Offset: 0}}})
	require.ErrorContains(t, g.a2aStreamPaced(ctx, "rpc"), "need at least 2")
}
