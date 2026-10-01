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
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/testbench/services/tlsbackend"
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

// fakeMTLSGateway answers the controller, policy engine and Envoy admin reads the mutual TLS
// steps make, from state a test sets.
type fakeMTLSGateway struct {
	mu                sync.Mutex
	pool              map[string][]string // controller and policy engine: name -> certificate PEMs
	engineOmits       string
	attachesMTLSAuth  bool
	controllerVersion string
	engineVersion     string
	listenerNamesCA   bool
	listenerWarming   bool
	secret            []byte
	clusterWarming    bool
	uploadStatus      int
	uploads           []map[string]any
	extraCerts        []map[string]any
	deletedIDs        []string
	deleteStatus      int
	engineCountDelta  int
	controllerStatus  int
}

func (f *fakeMTLSGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != BasicAuthHeader("admin", "secret") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch {
	case r.URL.Path == "/api/management/v1/certificates" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.uploads = append(f.uploads, body)
		w.WriteHeader(f.uploadStatus)
		write(map[string]string{"id": "cert-1"})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/management/v1/certificates/"):
		f.deletedIDs = append(f.deletedIDs, strings.TrimPrefix(r.URL.Path, "/api/management/v1/certificates/"))
		if f.deleteStatus != 0 {
			w.WriteHeader(f.deleteStatus)
		}
		write(map[string]string{"status": "success"})
	case r.URL.Path == "/api/management/v1/certificates" && f.controllerStatus != 0:
		w.WriteHeader(f.controllerStatus)
	case r.URL.Path == "/api/management/v1/certificates":
		var certs []map[string]any
		for _, name := range sortedKeys(f.pool) {
			certs = append(certs, map[string]any{"name": name, "role": "client", "count": len(f.pool[name])})
		}
		certs = append(certs, f.extraCerts...)
		write(map[string]any{"certificates": certs})
	case r.URL.Path == "/api/admin/v1/xds_sync_status":
		write(map[string]string{"policy_chain_version": f.controllerVersion})
	case r.URL.Path == "/api/admin/v1/config_dump":
		var policies []map[string]string
		if f.attachesMTLSAuth {
			policies = []map[string]string{{"name": "mtls-auth"}}
		}
		write(map[string]any{"apis": []any{map[string]any{"configuration": map[string]any{
			"spec": map[string]any{"operations": []any{map[string]any{"policies": policies}}},
		}}}})
	case r.URL.Path == "/xds_sync_status":
		write(map[string]string{"policy_chain_version": f.engineVersion})
	case r.URL.Path == "/config_dump" && r.URL.Query().Get("resource") == "":
		var resources []any
		for name, certs := range f.pool {
			if name != f.engineOmits {
				held := append([]string(nil), certs...)
				for i := 0; i < f.engineCountDelta; i++ {
					held = append(held, certs[0])
				}
				resources = append(resources, map[string]any{"id": name, "resource": map[string]any{"certificates": held, "role": "client"}})
			}
		}
		write(map[string]any{"lazy_resources": map[string]any{"resources_by_type": map[string]any{clientAuthorityResourceType: resources}}})
	case r.URL.Query().Get("resource") == "dynamic_listeners":
		listener := map[string]any{"name": "https", "active_state": map[string]any{"listener": map[string]any{}}}
		if f.listenerNamesCA {
			listener["active_state"] = map[string]any{"listener": map[string]any{"name": downstreamClientCASecret}}
		}
		if f.listenerWarming {
			listener["warming_state"] = map[string]any{"version_info": "2"}
		}
		write(map[string]any{"configs": []any{listener}})
	case r.URL.Query().Get("resource") == "dynamic_active_secrets":
		var configs []any
		if f.secret != nil {
			configs = append(configs, map[string]any{"name": downstreamClientCASecret, "secret": map[string]any{
				"validation_context": map[string]any{"trusted_ca": map[string]any{"inline_bytes": f.secret}},
			}})
		}
		write(map[string]any{"configs": configs})
	case r.URL.Query().Get("resource") == "dynamic_warming_clusters":
		var configs []any
		if f.clusterWarming {
			configs = append(configs, map[string]any{"cluster": map[string]string{"name": "backend"}})
		}
		write(map[string]any{"configs": configs})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// consistentMTLSGateway holds one pooled authority, attached by an API, applied everywhere.
func consistentMTLSGateway(t *testing.T) *fakeMTLSGateway {
	t.Helper()
	caA := mtlsFixture(t, "ca-a")
	return &fakeMTLSGateway{
		pool:              map[string][]string{"pool-a": {string(caA.CertPEM)}},
		attachesMTLSAuth:  true,
		controllerVersion: "7", engineVersion: "7",
		listenerNamesCA: true,
		secret:          caA.CertPEM,
		uploadStatus:    http.StatusCreated,
	}
}

func mtlsFixture(t *testing.T, name string) *testpki.Fixture {
	t.Helper()
	set, err := testpki.Default()
	require.NoError(t, err)
	fixture, err := set.Get(name)
	require.NoError(t, err)
	return fixture
}

// mtlsGatewayUnderTest serves fake on every plain endpoint and https on the HTTPS endpoint.
func mtlsGatewayUnderTest(t *testing.T, fake http.Handler, https *httptest.Server) (*Gateway, context.Context) {
	t.Helper()
	plain := httptest.NewServer(fake)
	t.Cleanup(plain.Close)
	port := func(raw string) int {
		parsed, err := url.Parse(raw)
		require.NoError(t, err)
		p, err := strconv.Atoi(parsed.Port())
		require.NoError(t, err)
		return p
	}
	mapped := map[int]int{9090: port(plain.URL), 9092: port(plain.URL), 9002: port(plain.URL), 9901: port(plain.URL)}
	if https != nil {
		mapped[8443] = port(https.URL)
	}
	definition := &components.Definition{
		Name: "platform-gateway", Alias: "platform-gateway",
		Endpoints: []components.Endpoint{
			{Name: "rest", Port: 9090, Scheme: "http"},
			{Name: "admin", Port: 9092, Scheme: "http"},
			{Name: "policy-admin", Port: 9002, Scheme: "http"},
			{Name: "envoy-admin", Port: 9901, Scheme: "http"},
			{Name: "https", Port: 8443, Scheme: "https"},
		},
	}
	instance, err := components.NewInstance(definition, 0, 1, "127.0.0.1", mapped)
	require.NoError(t, err)
	instances := components.NewSet()
	require.NoError(t, instances.Add(instance))

	shared := tcontext.NewShared("block")
	shared.Set(frameworkruntime.KeyAdminUser, "admin")
	shared.Set(frameworkruntime.KeyAdminPass, "secret")
	ctx := tcontext.WithLocal(tcontext.WithShared(context.Background(), shared), tcontext.NewLocal("runner"))
	require.NoError(t, cleanup.Install(ctx, cleanup.NewRegistry(nil)))
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})
	return &Gateway{topo: &frameworkruntime.Topology{Instances: instances}, funnel: httpx.NewFunnel(client, 0, time.Millisecond), base: stubBase{}}, ctx
}

// stubBase supplies admin credentials as the scenario's headers, no Host override, and a
// data plane at dataPlane.
type stubBase struct {
	Base
	dataPlane string
}

func (b stubBase) GatewayURL(path string) (string, error) { return b.dataPlane + path, nil }

func (stubBase) ScenarioHeaders(context.Context) map[string]string {
	return map[string]string{"Authorization": BasicAuthHeader("admin", "secret")}
}

func (stubBase) RequestHost(context.Context) string { return "" }

func TestParsePresentation(t *testing.T) {
	for phrase, want := range map[string]presentation{
		`with no client certificate`:                                         {},
		`with client certificate "client-valid"`:                             {fixture: "client-valid"},
		`with client certificate "client-via-intermediate" and its chain`:    {fixture: "client-via-intermediate", withChain: true},
		`with client certificate "client-valid" on a resumable TLS session`:  {fixture: "client-valid", resumable: true},
		`on a new connection from the same TLS session cache`:                {reuseSession: true},
		`with client certificate "client-valid" and bearer token "${CTX:t}"`: {fixture: "client-valid", bearer: "${CTX:t}"},
		`with no client certificate and bearer token "abc"`:                  {bearer: "abc"},
		`with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"`: {
			fixture: "edge-lb",
			relayed: relayedCertificate{header: "X-WSO2-CLIENT-CERTIFICATE", fixture: "client-valid"},
		},
		`with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "base64"`: {
			fixture: "edge-lb",
			relayed: relayedCertificate{header: "X-WSO2-CLIENT-CERTIFICATE", fixture: "client-valid", encoding: "base64"},
		},
		`with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "pem" and bearer token "abc"`: {
			bearer:  "abc",
			relayed: relayedCertificate{header: "X-WSO2-CLIENT-CERTIFICATE", fixture: "client-valid", encoding: "pem"},
		},
	} {
		got, err := parsePresentation(phrase)
		require.NoError(t, err, phrase)
		require.Equal(t, want, got, phrase)
	}
	for _, phrase := range []string{``, `with client certificate ""`, `with a certificate "x"`, `with no client certificate and its chain`} {
		_, err := parsePresentation(phrase)
		require.Error(t, err, phrase)
	}
}

// outcome classifies an observation: "" satisfied, "tolerated" polled through, "fail" fatal.
func outcome(err error) string {
	switch {
	case err == nil:
		return ""
	case retry.IsTransient(err):
		return "tolerated"
	default:
		return "fail"
	}
}

func TestObserveGatewayApplied(t *testing.T) {
	for _, tc := range []struct {
		name    string
		shape   func(*fakeMTLSGateway)
		outcome string
		message string
	}{
		{name: "applied everywhere", shape: func(*fakeMTLSGateway) {}},
		{name: "empty pool and a listener that does not ask", shape: func(f *fakeMTLSGateway) {
			f.pool, f.listenerNamesCA, f.secret = map[string][]string{}, false, nil
		}},
		{name: "policy chain versions are left to the policy snapshot wait", shape: func(f *fakeMTLSGateway) { f.engineVersion = "6" }},
		{name: "policy engine lags the pool", shape: func(f *fakeMTLSGateway) { f.engineOmits = "pool-a" },
			outcome: "tolerated", message: `the policy engine does not hold client authority "pool-a" yet`},
		{name: "listener still warming", shape: func(f *fakeMTLSGateway) { f.listenerWarming = true },
			outcome: "tolerated", message: "Envoy listeners [https] are still warming"},
		{name: "listener asks although no API attaches mtls-auth", shape: func(f *fakeMTLSGateway) { f.attachesMTLSAuth = false },
			outcome: "tolerated", message: "the HTTPS listener requests a client certificate: true, the controller's configuration requires false"},
		{name: "listener does not ask yet", shape: func(f *fakeMTLSGateway) { f.listenerNamesCA = false },
			outcome: "tolerated", message: "the HTTPS listener requests a client certificate: false, the controller's configuration requires true"},
		{name: "secret not served", shape: func(f *fakeMTLSGateway) { f.secret = nil },
			outcome: "tolerated", message: "Envoy serves no downstream_client_ca secret yet"},
		{name: "secret holds another authority", shape: func(f *fakeMTLSGateway) { f.secret = mtlsFixture(t, "ca-b").CertPEM },
			outcome: "tolerated", message: "Envoy's downstream_client_ca holds"},
		{name: "cluster still warming", shape: func(f *fakeMTLSGateway) { f.clusterWarming = true },
			outcome: "tolerated", message: "Envoy clusters [backend] are still warming"},
		{name: "policy engine holds another certificate count", shape: func(f *fakeMTLSGateway) { f.engineCountDelta = 1 },
			outcome: "fail", message: `the policy engine holds "pool-a" with role "client" and 2 certificates, the controller with role "client" and 1`},
		{name: "controller rejects the admin", shape: func(f *fakeMTLSGateway) { f.controllerStatus = http.StatusForbidden },
			outcome: "fail", message: "-> 403"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := consistentMTLSGateway(t)
			tc.shape(fake)
			g, ctx := mtlsGatewayUnderTest(t, fake, nil)
			err := g.observeGatewayApplied(ctx)
			require.Equal(t, tc.outcome, outcome(err), "%v", err)
			if tc.message != "" {
				require.ErrorContains(t, err, tc.message)
			}
		})
	}
}

func TestAwaitGatewayAppliedFailsAtOnceOnAContradiction(t *testing.T) {
	fake := consistentMTLSGateway(t)
	fake.engineCountDelta = 1
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	started := time.Now()
	require.ErrorContains(t, g.awaitGatewayApplied(ctx), "non-retryable")
	require.Less(t, time.Since(started), 5*time.Second)
}

func TestReadJSONToleratesOnlyAnUnreadyComponent(t *testing.T) {
	for status, want := range map[int]string{http.StatusServiceUnavailable: "tolerated", http.StatusUnauthorized: "fail", http.StatusInternalServerError: "fail", http.StatusNotFound: "fail"} {
		g, ctx := mtlsGatewayUnderTest(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"x"}`))
		}), nil)
		var out map[string]any
		err := g.readJSON(ctx, "policy-engine", "/config_dump", &out)
		require.Equal(t, want, outcome(err), "status %d: %v", status, err)
		require.ErrorContains(t, err, strconv.Itoa(status))
	}
	g, ctx := mtlsGatewayUnderTest(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}), nil)
	var out map[string]any
	require.Equal(t, "fail", outcome(g.readJSON(ctx, "policy-engine", "/config_dump", &out)))
}

func TestPolicySyncObserve(t *testing.T) {
	for _, tc := range []struct {
		sync     policySync
		baseline string
		want     string
	}{
		{sync: policySync{controller: "8", engine: "8"}, baseline: "7", want: ""},
		{sync: policySync{controller: "8", engine: "8"}, baseline: "", want: ""},
		{sync: policySync{controller: "7", engine: "7"}, baseline: "7", want: "tolerated"},
		{sync: policySync{controller: "8", engine: "7"}, baseline: "7", want: "tolerated"},
		{sync: policySync{controller: "8", engine: "9"}, baseline: "7", want: "fail"},
		{sync: policySync{controller: "", engine: "9"}, baseline: "", want: "fail"},
		{sync: policySync{controller: "b", engine: "a"}, baseline: "", want: "tolerated"},
	} {
		require.Equal(t, tc.want, outcome(tc.sync.observe(tc.baseline)), "%s past %q", tc.sync, tc.baseline)
	}
}

func TestRequireCleanGatewayNamesALeftoverAuthority(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	err := g.requireCleanGateway(ctx)
	require.ErrorContains(t, err, "the gateway still holds client authorities [pool-a] from an earlier scenario")
}

func TestPolicySnapshotSyncMovesPastTheBaselineAndForgetsIt(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, tcontext.Set(ctx, keyMTLSScenario, true))
	require.NoError(t, g.beforeAPIMutation(ctx))
	baseline, _ := tcontext.Get(ctx, keyPolicyBaseline)
	require.Equal(t, "7", baseline)

	fake.mu.Lock()
	fake.controllerVersion, fake.engineVersion = "8", "8"
	fake.mu.Unlock()
	require.NoError(t, g.awaitPolicySnapshotSync(ctx))
	require.False(t, tcontext.Contains(ctx, keyPolicyBaseline))

	require.NoError(t, tcontext.Set(ctx, keyMTLSScenario, false))
	require.NoError(t, g.beforeAPIMutation(ctx))
	require.False(t, tcontext.Contains(ctx, keyPolicyBaseline))
}

func TestUploadFixturesRegistersAnAcceptedCertificate(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "pool-a", "poolA"))

	require.ErrorContains(t, g.poolCertificateFixtures(ctx, "ca-a", "literal-name", "downstream"), "is not generated")
	require.ErrorContains(t, g.poolCertificateFixtures(ctx, "no-such-fixture", "${CTX:poolA}", "downstream"), "unknown fixture")
	require.NoError(t, g.poolCertificateFixtures(ctx, "ca-a-intermediate,ca-a", "${CTX:poolA}", "downstream"))

	name, err := stepscommon.StoredValue(ctx, "poolA")
	require.NoError(t, err)
	require.Len(t, fake.uploads, 1)
	require.Equal(t, name, fake.uploads[0]["name"])
	require.Equal(t, "downstream", fake.uploads[0]["usage"])
	require.Equal(t, string(mtlsFixture(t, "ca-a-intermediate").CertPEM)+"\n"+string(mtlsFixture(t, "ca-a").CertPEM),
		fake.uploads[0]["certificate"])
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	pending := registry.Pending()
	require.Len(t, pending, 1)
	require.Equal(t, cleanup.KindCertificate.Name, pending[0].Kind.Name)
	require.Equal(t, "cert-1", pending[0].ID)
}

func TestARejectedUploadIsPublishedAndNotRegistered(t *testing.T) {
	fake := consistentMTLSGateway(t)
	fake.uploadStatus = http.StatusConflict
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "pool-a", "poolA"))

	require.NoError(t, g.uploadCertificateFixtures(ctx, "ca-a", "${CTX:poolA}", ""))
	_, hasUsage := fake.uploads[0]["usage"]
	require.False(t, hasUsage)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, published.StatusCode)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Empty(t, registry.Pending())
	require.ErrorContains(t, g.poolCertificateFixtures(ctx, "ca-a", "${CTX:poolA}", "downstream"), "status 201")
}

func TestSendHTTPSPresentsTheFixtureAndPublishesTheHandshake(t *testing.T) {
	var authorization atomic.Value
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization.Store(r.Header.Get("Authorization"))
		subject := "none"
		if len(r.TLS.PeerCertificates) > 0 {
			subject = r.TLS.PeerCertificates[0].Subject.CommonName + "/" + strconv.Itoa(len(r.TLS.PeerCertificates))
		}
		_, _ = w.Write([]byte(subject))
	}))
	https.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	https.StartTLS()
	t.Cleanup(https.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), https)
	require.NoError(t, tcontext.Set(ctx, "token", "abc"))

	require.NoError(t, g.sendHTTPS(ctx, "get", "/anything", `with client certificate "client-via-intermediate" and its chain and bearer token "${CTX:token}"`))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, "client-via-intermediate/2", published.Text())
	require.Equal(t, "Bearer abc", authorization.Load())
	require.True(t, published.TLS.ClientCertificateRequested)
	require.Equal(t, gatewaySNI, published.TLS.ServerName)
	require.NoError(t, g.fullTLSHandshake(ctx))

	require.NoError(t, g.sendHTTPS(ctx, "GET", "anything", "with no client certificate"))
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, "none", published.Text())
	require.Equal(t, BasicAuthHeader("admin", "secret"), authorization.Load())

	require.ErrorContains(t, g.sendHTTPS(ctx, "GET", "/anything", "on a new connection from the same TLS session cache"),
		"send a request on a resumable TLS session first")
	require.NoError(t, g.sendHTTPS(ctx, "GET", "/anything", `with client certificate "client-valid" on a resumable TLS session`))
	require.NoError(t, g.sendHTTPS(ctx, "GET", "/anything", "on a new connection from the same TLS session cache"))
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, "client-valid/1", published.Text())
	require.ErrorContains(t, g.sendHTTPS(ctx, "GET", "/anything", "with a certificate"), "unknown HTTPS request phrasing")
}

func TestFullTLSHandshakeRejectsResumedAndPlainResponses(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, time.Millisecond)
	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: 200, TLS: &httpx.TLSState{DidResume: true}}))
	require.ErrorContains(t, g.fullTLSHandshake(ctx), "resumed the cached TLS session")
	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: 200}))
	require.ErrorContains(t, g.fullTLSHandshake(ctx), "did not arrive over an HTTPS request step")
}

func TestMTLSHelpers(t *testing.T) {
	require.True(t, hasTag([]string{"@a", "@mtls"}, tagMTLS))
	require.False(t, hasTag(nil, tagMTLS))

	caA, caB := mtlsFixture(t, "ca-a"), mtlsFixture(t, "ca-b")
	bundle := append(append(append([]byte{}, caA.CertPEM...), mtlsFixture(t, "client-valid").KeyPEM...), caB.CertPEM...)
	require.Equal(t, []string{caA.Thumbprint, caB.Thumbprint}, pemThumbprints(bundle))
	require.Empty(t, pemThumbprints(nil))

	require.True(t, sameSet(map[string]bool{"a": true}, map[string]bool{"a": true}))
	require.False(t, sameSet(map[string]bool{"a": true}, map[string]bool{"b": true}))
	require.False(t, sameSet(map[string]bool{"a": true}, map[string]bool{}))
	require.Equal(t, []string{"a", "b"}, sortedKeys(map[string]int{"b": 1, "a": 2}))

	want := map[string]clientAuthority{"a": {role: "client", count: 1}}
	require.NoError(t, compareClientAuthorities(want, map[string]clientAuthority{"a": {role: "client", count: 1}}))
	relay := compareClientAuthorities(want, map[string]clientAuthority{"a": {role: "relay", count: 1}})
	require.Equal(t, "fail", outcome(relay))
	require.ErrorContains(t, relay, `with role "relay"`)
	removed := compareClientAuthorities(map[string]clientAuthority{}, want)
	require.Equal(t, "tolerated", outcome(removed))
	require.ErrorContains(t, removed, `still holds removed client authority "a"`)
	require.Equal(t, "fixture.client-valid.thumbprint", fixtureThumbprintKey("client-valid"))
}

// scriptedDataPlane answers each request with the next reply, repeating the last one.
type scriptedDataPlane struct {
	mu      sync.Mutex
	replies []scriptedReply
	served  int
}

type scriptedReply struct {
	status   int
	upstream bool
	body     string
}

func (d *scriptedDataPlane) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	reply := d.replies[min(d.served, len(d.replies)-1)]
	d.served++
	d.mu.Unlock()
	if reply.upstream {
		w.Header().Set(headerUpstreamServiceTime, "3")
	}
	w.WriteHeader(reply.status)
	_, _ = w.Write([]byte(reply.body))
}

func TestSendUntilRouteAnswers(t *testing.T) {
	noRoute := scriptedReply{status: http.StatusNotFound, body: noRouteBody}
	warming := scriptedReply{status: http.StatusServiceUnavailable, body: "no healthy upstream"}
	policyChainMissing := scriptedReply{status: http.StatusInternalServerError, body: policyChainMissingBody}
	for _, tc := range []struct {
		name    string
		want    int
		replies []scriptedReply
		err     string
		served  int
	}{
		{name: "200 after no route and a warming upstream", want: 200,
			replies: []scriptedReply{noRoute, warming, {status: 200, upstream: true}}, served: 3},
		{name: "401 after no route", want: 401, replies: []scriptedReply{noRoute, {status: 401}}, served: 2},
		{name: "a 200 while waiting for a rejection fails at once", want: 401,
			replies: []scriptedReply{noRoute, {status: 200, upstream: true}}, err: "expected 401 once the route is live", served: 2},
		{name: "a warming upstream is no rejection", want: 401, replies: []scriptedReply{warming}, err: "got GET", served: 1},
		{name: "a backend 404 fails at once", want: 200,
			replies: []scriptedReply{{status: 404, upstream: true, body: "not here"}}, err: "not here", served: 1},
		{name: "a 404 with another body fails at once", want: 401,
			replies: []scriptedReply{{status: 404, body: "gone"}}, err: "gone", served: 1},
		{name: "a 403 fails at once", want: 200, replies: []scriptedReply{{status: 403}}, err: "-> 403", served: 1},
		{name: "503 after no route", want: 503, replies: []scriptedReply{noRoute, warming}, served: 2},
		{name: "a 200 while waiting for an unreachable upstream fails at once", want: 503,
			replies: []scriptedReply{noRoute, {status: 200, upstream: true}}, err: "expected 503 once the route is live", served: 2},
		{name: "200 after the policy chain is missing", want: 200,
			replies: []scriptedReply{noRoute, policyChainMissing, {status: 200, upstream: true}}, served: 3},
		{name: "401 after the policy chain is missing", want: 401, replies: []scriptedReply{policyChainMissing, {status: 401}}, served: 2},
		{name: "a 500 with another body fails at once", want: 200,
			replies: []scriptedReply{{status: 500, body: `{"error":"boom"}`}}, err: "boom", served: 1},
		{name: "a 500 from the upstream fails at once", want: 200,
			replies: []scriptedReply{{status: 500, upstream: true, body: policyChainMissingBody}}, err: "-> 500", served: 1},
		{name: "a 500 fails at once", want: 200, replies: []scriptedReply{{status: 500, upstream: true}}, err: "-> 500", served: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataPlane := &scriptedDataPlane{replies: tc.replies}
			server := httptest.NewServer(dataPlane)
			t.Cleanup(server.Close)
			g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
			g.base = stubBase{dataPlane: server.URL}

			err := g.sendUntilRouteAnswers(ctx, "get", "/anything", tc.want)
			if tc.err == "" {
				require.NoError(t, err)
				published, pubErr := httpx.Published(ctx)
				require.NoError(t, pubErr)
				require.Equal(t, tc.want, published.StatusCode)
			} else {
				require.ErrorContains(t, err, tc.err)
				require.ErrorContains(t, err, "non-retryable")
			}
			require.Equal(t, tc.served, dataPlane.served)
		})
	}
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	require.ErrorContains(t, g.sendUntilRouteAnswers(ctx, "GET", "/x", 404), "supports status 200, 401 or 503")
}

func TestRelayedCertificateValue(t *testing.T) {
	fixture := mtlsFixture(t, "client-valid")
	urlValue, err := (&relayedCertificate{fixture: "client-valid"}).value()
	require.NoError(t, err)
	require.Equal(t, url.PathEscape(string(fixture.CertPEM)), urlValue)

	pemValue, err := (&relayedCertificate{fixture: "client-valid", encoding: relayEncodingPEM}).value()
	require.NoError(t, err)
	require.Equal(t, strings.ReplaceAll(string(fixture.CertPEM), "\n", " "), pemValue)
	require.NotContains(t, pemValue, "\n")

	encoded, err := (&relayedCertificate{fixture: "client-valid", encoding: relayEncodingBase64}).value()
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	require.Equal(t, fixture.Certificate.Raw, raw)

	_, err = (&relayedCertificate{fixture: "client-valid", encoding: "der"}).value()
	require.ErrorContains(t, err, `unknown certificate header encoding "der"`)
	_, err = (&relayedCertificate{fixture: "missing"}).value()
	require.ErrorContains(t, err, "unknown fixture")
}

func TestSendHTTPSRelaysACertificateHeader(t *testing.T) {
	var got atomic.Value
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("X-WSO2-CLIENT-CERTIFICATE"))
		w.WriteHeader(http.StatusOK)
	}))
	https.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	https.StartTLS()
	t.Cleanup(https.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), https)

	phrase := `with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "pem"`
	require.NoError(t, g.sendHTTPS(ctx, "GET", "/anything", phrase))
	header, _ := got.Load().(string)
	require.Contains(t, header, "BEGIN CERTIFICATE")
	require.NotContains(t, header, "\n")

	require.ErrorContains(t, g.sendHTTPS(ctx, "GET", "/anything",
		`with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "der"`),
		`unknown certificate header encoding "der"`)
}

func TestForwardedCertificateNames(t *testing.T) {
	valid := mtlsFixture(t, "client-valid")
	other := mtlsFixture(t, "client-wrong-ca")
	xfcc := fmt.Sprintf(`Hash=%s;Subject="CN=client-valid",Hash=%s;Subject="CN=client-wrong-ca"`, valid.Thumbprint, other.Thumbprint)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	publish := func(value any) {
		t.Helper()
		body, err := json.Marshal(map[string]any{"headers": map[string]any{"X-Forwarded-Client-Cert": value}})
		require.NoError(t, err)
		require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: 200, Body: body}))
	}

	publish(xfcc)
	require.NoError(t, g.forwardedCertificateNames(ctx, "name", "client-valid"))
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "not name", "client-wrong-ca"), "not to name")
	require.NoError(t, g.forwardedCertificateNames(ctx, "not name", "edge-lb"))

	secondOnly := fmt.Sprintf(`Hash=%s;Subject="CN=client-wrong-ca",Hash=%s;Subject="CN=client-valid"`, other.Thumbprint, valid.Thumbprint)
	publish([]any{secondOnly})
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), "Hash")
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "not name", "client-valid"), "not to name")

	publish("")
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), "got none")
	require.NoError(t, g.forwardedCertificateNames(ctx, "not name", "client-valid"))
}

func TestCertificateUploadOptions(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "relay-edge-lb", "edgeLB"))

	require.NoError(t, g.uploadFixtureWithRole(ctx, "edge-lb-ca", "${CTX:edgeLB}", "downstream", "relay", ""))
	require.Equal(t, "relay", fake.uploads[0]["role"])
	require.Equal(t, "downstream", fake.uploads[0]["usage"])
	_, hasMatch := fake.uploads[0]["match"]
	require.False(t, hasMatch)
	_, hasKey := fake.uploads[0]["privateKey"]
	require.False(t, hasKey)

	require.NoError(t, g.uploadFixtureWithRole(ctx, "corp-ca", "${CTX:edgeLB}", "downstream", "relay", "lb.corp.test"))
	match, ok := fake.uploads[1]["match"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"lb.corp.test"}, match["dnsSANs"])

	require.NoError(t, g.storeIdentity(ctx, "gw-identity-a", "${CTX:edgeLB}"))
	require.Equal(t, "identity", fake.uploads[2]["usage"])
	require.Contains(t, fake.uploads[2]["privateKey"], "PRIVATE KEY")
	require.Contains(t, fake.uploads[2]["certificate"], "BEGIN CERTIFICATE")
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Len(t, registry.Pending(), 1)
	require.Equal(t, "cert-1", registry.Pending()[0].ID)

	fake.uploadStatus = http.StatusBadRequest
	require.ErrorContains(t, g.storeIdentity(ctx, "gw-identity-a", "${CTX:edgeLB}"), "status 201")
}

func TestDeleteCertificateNamed(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "relay-edge-lb", "edgeLB"))
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "missing-cert", "missing"))
	name, err := stepscommon.StoredValue(ctx, "edgeLB")
	require.NoError(t, err)
	fake.extraCerts = []map[string]any{{"id": "cert-9", "name": name}}

	require.NoError(t, g.deleteCertificateNamed(ctx, "${CTX:edgeLB}"))
	require.Equal(t, []string{"cert-9"}, fake.deletedIDs)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode)

	require.NoError(t, g.poolCertificateFixtures(ctx, "edge-lb-ca", "${CTX:edgeLB}", "downstream"))
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Equal(t, "cert-1", registry.Pending()[0].ID)
	fake.deleteStatus = http.StatusConflict
	fake.extraCerts = []map[string]any{{"id": "cert-1", "name": name}}
	require.NoError(t, g.deleteCertificateNamed(ctx, "${CTX:edgeLB}"))
	require.Equal(t, http.StatusConflict, mustPublished(t, ctx).StatusCode)
	require.Equal(t, "cert-1", registry.Pending()[0].ID)

	started := time.Now()
	err = g.deleteCertificateNamed(ctx, "${CTX:missing}")
	require.ErrorContains(t, err, "no certificate named")
	require.Less(t, time.Since(started), 5*time.Second)
}

func mustPublished(t *testing.T, ctx context.Context) *httpx.Response {
	t.Helper()
	resp, err := httpx.Published(ctx)
	require.NoError(t, err)
	return resp
}

func TestValidationErrorAndMetricText(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "relay-edge-lb", "edgeLB"))
	name, err := stepscommon.StoredValue(ctx, "edgeLB")
	require.NoError(t, err)
	body := `{"errors":[{"field":"spec.policies[0].params.accept[0].ca","message":"` + name + ` is a relay (front proxy) entry and cannot be accepted as a client"}]}`
	require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: 400, Body: []byte(body)}))
	require.NoError(t, validationErrorListed(ctx, "spec.policies[0].params.accept[0].ca", "with message",
		"${CTX:edgeLB} is a relay (front proxy) entry and cannot be accepted as a client"))
	require.ErrorContains(t, validationErrorListed(ctx, "spec.policies[0].params.accept[0].ca", "with message", "other"), "no validation error")

	require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: 200, Body: []byte("policy_executions_total 1\n")}))
	require.NoError(t, g.responseOmitsMetric(ctx, "mtls_auth_"))
	require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: 200, Body: []byte("# TYPE mtls_auth_total counter\n")}))
	require.ErrorContains(t, g.responseOmitsMetric(ctx, `mtls_auth_`), `has the metric "mtls_auth_total"`)
}

func TestObservabilityHelpers(t *testing.T) {
	valid := mtlsFixture(t, "client-valid")
	require.Equal(t, "urn:partner-a:payments", certificateSubjectIdentity(valid.Certificate))
	require.Equal(t, "edge-lb.internal", certificateSubjectIdentity(mtlsFixture(t, "edge-lb").Certificate))
	require.Contains(t, certificateSubjectIdentity(mtlsFixture(t, "client-no-san").Certificate), "CN=client-no-san")

	labels, err := parseLabels(`{policy_name="mtls-auth",status="denied"}`)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"policy_name": "mtls-auth", "status": "denied"}, labels)
	_, err = parseLabels(`{usage=downstream}`)
	require.Error(t, err)
	_, err = parseLabels(`{usage="downstream",usage="upstream"}`)
	require.ErrorContains(t, err, "appears twice")

	samples := parseSamples("# HELP gateway_controller_certificates_total\ngateway_controller_certificates_total{usage=\"downstream\"} 2\n")
	require.Len(t, samples, 1)
	require.Equal(t, "gateway_controller_certificates_total", samples[0].name)
	require.Equal(t, "downstream", samples[0].labels["usage"])
	require.Equal(t, "2", samples[0].value)

	line := `{"path":"/obs/v1/anything","peerSubj":"CN=client-valid","peerFp":"` + valid.Thumbprint + `"}`
	expectations := []logExpectation{{describe: "subject", shownIn: func(s string) bool { return strings.Contains(s, `"peerSubj":"CN=client-valid"`) }}}
	require.NoError(t, observeAccessLogLine("noise\n"+line, "/obs/v1/anything", expectations))
	require.Equal(t, "tolerated", outcome(observeAccessLogLine("no line yet", "/obs/v1/anything", expectations)))
	wrong := observeAccessLogLine(line, "/obs/v1/anything", []logExpectation{{describe: "tls", shownIn: func(string) bool { return false }}})
	require.Equal(t, "fail", outcome(wrong))
	require.ErrorContains(t, wrong, "does not show tls")

	require.NoError(t, observeCounterGrowth("policy_executions_total", `{status="denied"}`, 1, 2, 1))
	require.Equal(t, "tolerated", outcome(observeCounterGrowth("policy_executions_total", `{status="denied"}`, 1, 1, 1)))
	fell := observeCounterGrowth("policy_executions_total", `{status="denied"}`, 4, 3, 1)
	require.Equal(t, "fail", outcome(fell))
	require.ErrorContains(t, fell, "was reset")
}

func TestParsePresentationCarriesARelayedCertificate(t *testing.T) {
	for phrase, want := range map[string]presentation{
		`with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"`: {
			relayed: relayedCertificate{header: "X-WSO2-CLIENT-CERTIFICATE", fixture: "client-valid"},
		},
		`with client certificate "client-wrong-ca" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "base64"`: {
			fixture: "client-wrong-ca",
			relayed: relayedCertificate{header: "X-WSO2-CLIENT-CERTIFICATE", fixture: "client-valid", encoding: "base64"},
		},
		`with no client certificate and header "X-Client-Cert" carrying certificate "client-valid" encoded as "pem" and bearer token "abc"`: {
			relayed: relayedCertificate{header: "X-Client-Cert", fixture: "client-valid", encoding: "pem"},
			bearer:  "abc",
		},
		`with client certificate "client-valid" and its chain and header "X-Client-Cert" carrying certificate "ca-a" encoded as "url"`: {
			fixture:   "client-valid",
			withChain: true,
			relayed:   relayedCertificate{header: "X-Client-Cert", fixture: "ca-a", encoding: "url"},
		},
	} {
		got, err := parsePresentation(phrase)
		require.NoError(t, err, phrase)
		require.Equal(t, want, got, phrase)
	}
	plain, err := parsePresentation(`with no client certificate`)
	require.NoError(t, err)
	require.Zero(t, plain.relayed)
}

func TestEncodeRelayedCertificate(t *testing.T) {
	fixture := mtlsFixture(t, "client-valid")
	urlEncoded, err := encodeRelayedCertificate("client-valid", "")
	require.NoError(t, err)
	require.Equal(t, url.PathEscape(string(fixture.CertPEM)), urlEncoded)
	named, err := encodeRelayedCertificate("client-valid", "url")
	require.NoError(t, err)
	require.Equal(t, urlEncoded, named)

	pemText, err := encodeRelayedCertificate("client-valid", "pem")
	require.NoError(t, err)
	require.Equal(t, strings.ReplaceAll(string(fixture.CertPEM), "\n", " "), pemText)
	require.NotContains(t, pemText, "\n")

	der, err := encodeRelayedCertificate("client-valid", "base64")
	require.NoError(t, err)
	require.Equal(t, base64.StdEncoding.EncodeToString(fixture.Certificate.Raw), der)

	_, err = encodeRelayedCertificate("client-valid", "der")
	require.ErrorContains(t, err, "unknown certificate header encoding")
	_, err = encodeRelayedCertificate("no-such-fixture", "")
	require.ErrorContains(t, err, "unknown fixture")
}

func TestSendRelayingCertificateSetsTheHeaderOnPlainHTTP(t *testing.T) {
	var got atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Nil(t, r.TLS)
		got.Store(r.Header.Get("X-WSO2-CLIENT-CERTIFICATE"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	g.base = stubBase{dataPlane: server.URL}

	require.NoError(t, g.sendRelayingCertificate(ctx, "get", "/anything", "X-WSO2-CLIENT-CERTIFICATE", "client-valid", ""))
	want, err := encodeRelayedCertificate("client-valid", "url")
	require.NoError(t, err)
	require.Equal(t, want, got.Load())
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode)
	require.Nil(t, published.TLS)

	require.NoError(t, g.sendRelayingCertificate(ctx, "GET", "/anything", "X-WSO2-CLIENT-CERTIFICATE", "client-valid", "pem"))
	wantPEM, err := encodeRelayedCertificate("client-valid", "pem")
	require.NoError(t, err)
	// An HTTP parser drops the trailing space that the PEM's final newline becomes.
	require.Equal(t, strings.TrimRight(wantPEM, " "), got.Load())
	require.NotContains(t, got.Load(), "\n")
	require.ErrorContains(t, g.sendRelayingCertificate(ctx, "GET", "/anything", "X-WSO2-CLIENT-CERTIFICATE", "client-valid", "der"),
		"unknown certificate header encoding")
}

func TestSendHTTPSAppliesTheRelayedHeaderBesideTheClientCertificate(t *testing.T) {
	var gotHeader, gotSubject atomic.Value
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader.Store(r.Header.Get("X-WSO2-CLIENT-CERTIFICATE"))
		subject := "none"
		if len(r.TLS.PeerCertificates) > 0 {
			subject = r.TLS.PeerCertificates[0].Subject.CommonName
		}
		gotSubject.Store(subject)
		w.WriteHeader(http.StatusOK)
	}))
	https.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	https.StartTLS()
	t.Cleanup(https.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), https)

	phrase := `with client certificate "client-wrong-ca" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "base64"`
	require.NoError(t, g.sendHTTPS(ctx, "GET", "/anything", phrase))
	want, err := encodeRelayedCertificate("client-valid", "base64")
	require.NoError(t, err)
	require.Equal(t, want, gotHeader.Load())
	require.Equal(t, "client-wrong-ca", gotSubject.Load())
}

func TestForwardedCertificateNamesFromEcho(t *testing.T) {
	fixture := mtlsFixture(t, "client-valid")
	cn := fixture.Certificate.Subject.CommonName
	named := "Hash=" + fixture.Thumbprint + `;Subject="CN=` + cn + `"`
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	publishEcho := func(headers map[string]any) {
		t.Helper()
		body, err := json.Marshal(map[string]any{"headers": headers})
		require.NoError(t, err)
		require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: body}))
	}

	publishEcho(map[string]any{"X-Forwarded-Client-Cert": named})
	require.NoError(t, g.forwardedCertificateNames(ctx, "name", "client-valid"))
	publishEcho(map[string]any{"x-forwarded-client-cert": []any{named}})
	require.NoError(t, g.forwardedCertificateNames(ctx, "name", "client-valid"))

	publishEcho(map[string]any{"X-Forwarded-Client-Cert": "Hash=" + strings.Repeat("ab", 32) + `;Subject="CN=` + cn + `"`})
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), fixture.Thumbprint)
	publishEcho(map[string]any{"X-Forwarded-Client-Cert": "Hash=" + fixture.Thumbprint})
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), "CN="+cn)
	publishEcho(map[string]any{})
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), "got none")
	publishEcho(map[string]any{"X-Forwarded-Client-Cert": 7})
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), "not a string or an array")
	require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: []byte("not json")}))
	require.ErrorContains(t, g.forwardedCertificateNames(ctx, "name", "client-valid"), "no echoed headers")
}

func TestUploadWithRoleAndPoolWithoutUsage(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "listener-edge-lb", "edgeLb"))
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "listener-backend-trust", "backendTrust"))

	require.NoError(t, g.uploadFixtureWithRole(ctx, "edge-lb-ca", "${CTX:edgeLb}", "downstream", "relay", ""))
	require.Equal(t, "relay", fake.uploads[0]["role"])
	require.Equal(t, "downstream", fake.uploads[0]["usage"])
	require.Contains(t, fake.uploads[0]["certificate"], string(mtlsFixture(t, "edge-lb-ca").CertPEM))

	require.NoError(t, g.poolCertificateFixturesWithoutUsage(ctx, "backend-ca", "${CTX:backendTrust}"))
	_, hasUsage := fake.uploads[1]["usage"]
	require.False(t, hasUsage)
	require.Equal(t, string(mtlsFixture(t, "backend-ca").CertPEM), fake.uploads[1]["certificate"])
	require.NotContains(t, fake.uploads[1], "role")
}

func TestValidationErrorMatchesFieldAndMessage(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "listener-partner-b", "partnerB"))
	name, err := stepscommon.StoredValue(ctx, "partnerB")
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{"errors": []map[string]string{{
		"field":   "spec.policies[0]",
		"message": "no client-CA authority named " + name + " exists on this gateway",
	}}})
	require.NoError(t, err)
	require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusBadRequest, Body: body}))

	require.NoError(t, validationErrorListed(ctx, "spec.policies[0]", "with message", "no client-CA authority named ${CTX:partnerB} exists on this gateway"))
	require.NoError(t, validationErrorListed(ctx, "spec.policies[0]", "containing", "no client-CA authority named"))
	require.ErrorContains(t, validationErrorListed(ctx, "spec.policies[0]", "with message", "no client-CA authority named"), "with message")
	require.ErrorContains(t, validationErrorListed(ctx, "spec.policies[1]", "containing", "no client-CA authority named"), "containing")
	require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusBadRequest, Body: []byte("not json")}))
	require.ErrorContains(t, validationErrorListed(ctx, "spec.policies[0]", "with message", "x"), "not a JSON error response")
}

func TestResponseWarningsPreferTheStatusList(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	publish := func(body string) {
		t.Helper()
		require.NoError(t, g.funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(body)}))
	}
	statusWarning := `{"code":"MTLS_ACCEPT_INHERITS_POOL","field":"spec.policies[0].params.accept","message":"inherited"}`
	topWarning := `{"code":"HEADER_CERT_BYPASS_ACTIVE","message":"bypass"}`

	publish(`{"status":{"warnings":[` + statusWarning + `]},"warnings":[` + topWarning + `]}`)
	require.NoError(t, responseWarnsForField(ctx, "MTLS_ACCEPT_INHERITS_POOL", "spec.policies[0].params.accept"))
	require.ErrorContains(t, responseWarnsForField(ctx, "HEADER_CERT_BYPASS_ACTIVE", ""), "no warning with code")

	publish(`{"status":{"warnings":[]},"warnings":[` + topWarning + `]}`)
	require.NoError(t, responseHasNoWarnings(ctx))

	publish(`{"warnings":[` + topWarning + `]}`)
	require.NoError(t, responseWarnsForField(ctx, "HEADER_CERT_BYPASS_ACTIVE", ""))
	publish(`{"status":{"warnings":null},"warnings":[` + topWarning + `]}`)
	require.NoError(t, responseWarnsForField(ctx, "HEADER_CERT_BYPASS_ACTIVE", ""))
	publish(`{"status":"success","warnings":[` + topWarning + `]}`)
	require.NoError(t, responseWarnsForField(ctx, "HEADER_CERT_BYPASS_ACTIVE", ""))
	publish(`{"status":"success"}`)
	require.NoError(t, responseHasNoWarnings(ctx))

	publish(`{"status":{"warnings":{"code":"MTLS_ACCEPT_UNNARROWED"}}}`)
	_, err := responseWarnings(ctx)
	require.ErrorContains(t, err, "status.warnings is not a list")
	publish(`{"warnings":{"code":"HEADER_CERT_BYPASS_ACTIVE"}}`)
	_, err = responseWarnings(ctx)
	require.ErrorContains(t, err, "the response warnings are not a list")
	publish(`not json`)
	_, err = responseWarnings(ctx)
	require.ErrorContains(t, err, "not JSON")
	publish(`{}`)
	require.NoError(t, responseHasNoWarnings(ctx))
}

func TestRepositoryFileStaysInsideTheCheckout(t *testing.T) {
	const relative = "gateway/gateway-controller/listener-certs/default-listener.crt"
	path, err := repositoryFile(relative)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	for _, bad := range []string{"", "/etc/passwd", "../outside", "gateway/../../outside", "a\x00b"} {
		_, err := repositoryFile(bad)
		require.Error(t, err, bad)
	}
}

func TestListenerProbeAndHold(t *testing.T) {
	requesting := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	requesting.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	requesting.StartTLS()
	t.Cleanup(requesting.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), requesting)
	started := time.Now()
	require.NoError(t, g.listenerRequestsClientCertificate(ctx))
	require.Less(t, time.Since(started), 2*time.Second)
	started = time.Now()
	err := g.listenerDoesNotRequestClientCertificate(ctx)
	require.ErrorContains(t, err, "was expected to not request a client certificate")
	require.ErrorContains(t, err, "invariant was violated")
	require.NotContains(t, err.Error(), "requested a client certificate, expected none")
	require.Less(t, time.Since(started), 2*time.Second)

	quiet := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	quiet.TLS = &tls.Config{ClientAuth: tls.NoClientCert}
	quiet.StartTLS()
	t.Cleanup(quiet.Close)
	g, ctx = mtlsGatewayUnderTest(t, consistentMTLSGateway(t), quiet)
	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	started = time.Now()
	err = g.listenerRequestsClientCertificate(short)
	require.Error(t, err)
	require.ErrorContains(t, err, "cancel")
	require.Less(t, time.Since(started), 2*time.Second)

	started = time.Now()
	require.NoError(t, g.listenerDoesNotRequestClientCertificate(ctx))
	require.GreaterOrEqual(t, time.Since(started), listenerHoldWindow)
	require.Less(t, time.Since(started), listenerHoldWindow+3*time.Second)

	unmapped, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	started = time.Now()
	err = unmapped.listenerDoesNotRequestClientCertificate(ctx)
	require.ErrorContains(t, err, "was expected to not request a client certificate")
	require.NotContains(t, err.Error(), "requested a client certificate, expected none")
	require.Less(t, time.Since(started), time.Second)
}

func TestListenerPresentsTheCertificateFile(t *testing.T) {
	const relative = "gateway/gateway-controller/listener-certs/default-listener.crt"
	path, err := repositoryFile(relative)
	require.NoError(t, err)
	pair, err := tls.LoadX509KeyPair(path, filepath.Join(filepath.Dir(path), "default-listener.key"))
	require.NoError(t, err)
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	https.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	https.StartTLS()
	t.Cleanup(https.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), https)
	require.NoError(t, g.listenerPresentsCertificateFile(ctx, relative))

	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(other.Close)
	g, ctx = mtlsGatewayUnderTest(t, consistentMTLSGateway(t), other)
	require.ErrorContains(t, g.listenerPresentsCertificateFile(ctx, relative), relative)
}

func TestListenerDeployDocumentsKeepTheirFields(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	template, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "resources", "templates", "rest-api.yaml"))
	require.NoError(t, err)
	render := func(pairs ...string) map[string]any {
		t.Helper()
		definition, renderErr := stepscommon.RenderResourceTemplate(context.Background(),
			"resources/templates/rest-api.yaml", template, valueTable(t, pairs...))
		require.NoError(t, renderErr)
		var document map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(definition), &document))
		return document
	}
	specOf := func(document map[string]any) map[string]any {
		t.Helper()
		require.Equal(t, "RestApi", document["kind"])
		require.Equal(t, "gateway.api-platform.wso2.com/v1", document["apiVersion"])
		metadata, ok := document["metadata"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "mtls-listener-api", metadata["name"])
		spec, ok := document["spec"].(map[string]any)
		require.True(t, ok)
		return spec
	}

	protected := specOf(render(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "mtls-listener-api",
		"spec.displayName", "mTLS Listener API",
		"spec.version", "v1.0",
		"spec.context", "/mtls-listener/v1.0/$version",
		"spec.upstream.main.url", "http://testbench:3000/api/v1",
		"spec.operations", `[{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"mtls-auth","version":"v1"}]}]`,
	))
	upstream := protected["upstream"].(map[string]any)["main"].(map[string]any)
	require.Equal(t, "http://testbench:3000/api/v1", upstream["url"])
	operations := protected["operations"].([]any)
	require.Len(t, operations, 2)
	require.Empty(t, operations[0].(map[string]any)["policies"])
	policies := operations[1].(map[string]any)["policies"].([]any)
	require.Equal(t, "mtls-auth", policies[0].(map[string]any)["name"])

	conditional := specOf(render(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "mtls-listener-api",
		"spec.policies", `[{"name":"mtls-auth","version":"v1","executionCondition":"request.Method == \"POST\""}]`,
		"spec.operations", `[{"method":"POST","path":"/echo"}]`,
		"spec.upstream.main.url", "http://testbench:3002",
	))
	require.Equal(t, `request.Method == "POST"`, conditional["policies"].([]any)[0].(map[string]any)["executionCondition"])

	optOut := specOf(render(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "mtls-listener-api",
		"spec.policies", `[{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"partner-a"}],"forwardCertificate":false}}]`,
		"spec.operations", `[{"method":"GET","path":"/anything"}]`,
		"spec.upstream.main.url", "http://testbench:3002",
	))
	params := optOut["policies"].([]any)[0].(map[string]any)["params"].(map[string]any)
	require.Equal(t, false, params["forwardCertificate"])
	require.Equal(t, "partner-a", params["accept"].([]any)[0].(map[string]any)["ca"])

	inherited := specOf(render(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "mtls-listener-api",
		"spec.policies", `[{"name":"mtls-auth","version":"v1","params":{"accept":[]}}]`,
		"spec.operations", `[{"method":"GET","path":"/health"}]`,
		"spec.upstream.main.url", "http://testbench:3000/api/v1",
	))
	accept := inherited["policies"].([]any)[0].(map[string]any)["params"].(map[string]any)["accept"]
	require.Empty(t, accept)
}

func valueTable(t *testing.T, pairs ...string) *godog.Table {
	t.Helper()
	require.Zero(t, len(pairs)%2)
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []row `json:"rows"`
	}{}
	for i := 0; i < len(pairs); i += 2 {
		payload.Rows = append(payload.Rows, row{Cells: []cell{{Value: pairs[i]}, {Value: pairs[i+1]}}})
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	var table godog.Table
	require.NoError(t, json.Unmarshal(raw, &table))
	return &table
}

// twoControllerGatewayUnderTest serves management on one fake and the xDS controller's admin
// API on another, so a test sees which controller each read reaches.
func twoControllerGatewayUnderTest(t *testing.T, management, xds *fakeMTLSGateway) (*Gateway, context.Context) {
	t.Helper()
	managementServer := httptest.NewServer(management)
	t.Cleanup(managementServer.Close)
	xdsServer := httptest.NewServer(xds)
	t.Cleanup(xdsServer.Close)
	port := func(raw string) int {
		parsed, err := url.Parse(raw)
		require.NoError(t, err)
		p, err := strconv.Atoi(parsed.Port())
		require.NoError(t, err)
		return p
	}
	mapped := map[int]int{
		9090: port(managementServer.URL), 9092: port(managementServer.URL),
		9002: port(managementServer.URL), 9901: port(managementServer.URL),
		9094: port(xdsServer.URL),
	}
	definition := &components.Definition{
		Name: "platform-gateway", Alias: "platform-gateway",
		Endpoints: []components.Endpoint{
			{Name: "rest", Port: 9090, Scheme: "http"},
			{Name: "admin", Port: 9092, Scheme: "http"},
			{Name: "policy-admin", Port: 9002, Scheme: "http"},
			{Name: "envoy-admin", Port: 9901, Scheme: "http"},
			{Name: xdsControllerAdminEndpoint, Port: 9094, Scheme: "http"},
		},
	}
	instance, err := components.NewInstance(definition, 0, 1, "127.0.0.1", mapped)
	require.NoError(t, err)
	instances := components.NewSet()
	require.NoError(t, instances.Add(instance))

	shared := tcontext.NewShared("block")
	shared.Set(frameworkruntime.KeyAdminUser, "admin")
	shared.Set(frameworkruntime.KeyAdminPass, "secret")
	ctx := tcontext.WithLocal(tcontext.WithShared(context.Background(), shared), tcontext.NewLocal("runner"))
	require.NoError(t, cleanup.Install(ctx, cleanup.NewRegistry(nil)))
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})
	return &Gateway{topo: &frameworkruntime.Topology{Instances: instances}, funnel: httpx.NewFunnel(client, 0, time.Millisecond), base: stubBase{}}, ctx
}

func TestRuntimeControllerAdminIsTheManagementControllerOnOneController(t *testing.T) {
	g, _ := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	service, separate, err := g.runtimeControllerAdmin()
	require.NoError(t, err)
	require.False(t, separate)
	require.Equal(t, managementControllerAdmin, service)
}

func TestRuntimeControllerAdminIsTheXDSControllerOnTwoControllers(t *testing.T) {
	g, _ := twoControllerGatewayUnderTest(t, consistentMTLSGateway(t), consistentMTLSGateway(t))
	service, separate, err := g.runtimeControllerAdmin()
	require.NoError(t, err)
	require.True(t, separate)
	require.Equal(t, xdsControllerAdmin, service)
}

func TestPolicySnapshotVersionsReadTheControllerThatFeedsTheRuntime(t *testing.T) {
	management, xds := consistentMTLSGateway(t), consistentMTLSGateway(t)
	management.controllerVersion, management.engineVersion = "9", "4"
	xds.controllerVersion = "4"
	g, ctx := twoControllerGatewayUnderTest(t, management, xds)
	got, err := g.policySnapshotVersions(ctx)
	require.NoError(t, err)
	require.Equal(t, policySync{controller: "4", engine: "4"}, got)
}

func TestRuntimeControllerAgreementWaitsForTheEventPath(t *testing.T) {
	management, xds := consistentMTLSGateway(t), consistentMTLSGateway(t)
	xds.attachesMTLSAuth = false
	g, ctx := twoControllerGatewayUnderTest(t, management, xds)

	require.Equal(t, "tolerated", outcome(g.requireRuntimeControllerAgrees(ctx, true)))
	require.NoError(t, g.requireRuntimeControllerAgrees(ctx, false))

	xds.mu.Lock()
	xds.attachesMTLSAuth = true
	xds.mu.Unlock()
	require.NoError(t, g.requireRuntimeControllerAgrees(ctx, true))
	require.Equal(t, "tolerated", outcome(g.requireRuntimeControllerAgrees(ctx, false)))
}

func TestRuntimeControllerAgreementIsNothingToWaitForOnOneController(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	require.NoError(t, g.requireRuntimeControllerAgrees(ctx, true))
	require.NoError(t, g.requireRuntimeControllerAgrees(ctx, false))
}

func noListener() (string, error) { return "", errors.New("no listener") }

func TestParseBackendView(t *testing.T) {
	gatewayA := mtlsFixture(t, "gw-identity-a").Certificate.Subject.String()
	require.Equal(t, "CN=gateway-a", gatewayA)
	listener := func() (string, error) { return "CN=localhost", nil }

	for phrase, want := range map[string]backendView{
		`no client certificate`: {kind: viewNone},
		`a refusal`:             {kind: viewRefused},
		`the client certificate of "gw-identity-a"`:     {kind: viewSubject, subject: gatewayA},
		`the gateway listener certificate`:              {kind: viewSubject, subject: "CN=localhost"},
		`  the client certificate of "gw-identity-a"  `: {kind: viewSubject, subject: gatewayA},
	} {
		got, err := parseBackendView(phrase, listener)
		require.NoError(t, err, phrase)
		require.Equal(t, want, got, phrase)
	}

	_, err := parseBackendView("a certificate", listener)
	require.ErrorContains(t, err, `unknown backend view "a certificate"`)
	_, err = parseBackendView(`the client certificate of "no-such-fixture"`, listener)
	require.ErrorContains(t, err, "unknown fixture")
	_, err = parseBackendView("the gateway listener certificate", noListener)
	require.ErrorContains(t, err, "no listener")
}

func TestBackendViewString(t *testing.T) {
	require.Equal(t, "no client certificate", backendView{kind: viewNone}.String())
	require.Equal(t, "a refusal", backendView{kind: viewRefused}.String())
	require.Equal(t, `the client certificate "CN=a"`, backendView{kind: viewSubject, subject: "CN=a"}.String())
}

func echoResponse(status int, upstream bool, subject string) *httpx.Response {
	headers := http.Header{}
	if upstream {
		headers.Set(headerUpstreamServiceTime, "2")
	}
	if subject != "" {
		headers.Set(tlsbackend.HeaderClientSubject, subject)
	}
	return &httpx.Response{StatusCode: status, Headers: headers, Method: http.MethodGet, URL: "http://gw/anything"}
}

func TestJudgeBackendResponse(t *testing.T) {
	listener := backendView{kind: viewSubject, subject: "CN=localhost"}
	gatewayA := backendView{kind: viewSubject, subject: "CN=gateway-a"}
	none := backendView{kind: viewNone}
	refused := backendView{kind: viewRefused}

	noRoute := &httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(noRouteBody), Headers: http.Header{}}
	warming := &httpx.Response{StatusCode: http.StatusServiceUnavailable, Body: []byte("no healthy upstream"), Headers: http.Header{}}

	for _, tc := range []struct {
		name      string
		resp      *httpx.Response
		want      backendView
		inBetween []backendView
		outcome   string
		message   string
	}{
		{name: "the expected subject", resp: echoResponse(200, true, "CN=gateway-a"), want: gatewayA},
		{name: "an empty subject is no certificate", resp: echoResponse(200, true, ""), want: none},
		{name: "a refusal", resp: echoResponse(400, true, ""), want: refused},
		{name: "no route yet", resp: noRoute, want: gatewayA, outcome: "tolerated", message: "Envoy has no route yet"},
		{name: "a warming upstream", resp: warming, want: gatewayA, outcome: "tolerated", message: "still warming"},
		{name: "the previous subject while it propagates", resp: echoResponse(200, true, "CN=localhost"),
			want: gatewayA, inBetween: []backendView{listener}, outcome: "tolerated", message: "expected the client certificate"},
		{name: "a subject nobody listed", resp: echoResponse(200, true, "CN=intruder"),
			want: gatewayA, inBetween: []backendView{listener}, outcome: "fail", message: `sees the client certificate "CN=intruder"`},
		{name: "the previous subject without a tolerance", resp: echoResponse(200, true, "CN=localhost"),
			want: gatewayA, outcome: "fail", message: "expected the client certificate"},
		{name: "no certificate where one is expected", resp: echoResponse(200, true, ""),
			want: gatewayA, inBetween: []backendView{listener}, outcome: "fail", message: "sees no client certificate"},
		{name: "a refusal where a subject is expected", resp: echoResponse(400, true, ""),
			want: gatewayA, outcome: "fail", message: "sees a refusal"},
		{name: "a subject where a refusal is expected", resp: echoResponse(200, true, "CN=gateway-a"),
			want: refused, outcome: "fail", message: `sees the client certificate "CN=gateway-a"`},
		{name: "a backend error", resp: echoResponse(500, true, ""), want: gatewayA, outcome: "fail", message: "answered 500"},
		{name: "a reply of the gateway that is no in-between state", resp: echoResponse(502, false, ""),
			want: gatewayA, outcome: "fail", message: "expected the request to reach the backend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := judgeBackendResponse(tc.resp, tc.want, tc.inBetween)
			require.Equal(t, tc.outcome, outcome(err))
			if tc.message != "" {
				require.ErrorContains(t, err, tc.message)
			}
		})
	}
}

// subjectDataPlane answers each request with the next scripted reply, repeating the last one.
type subjectDataPlane struct {
	mu      sync.Mutex
	replies []subjectReply
	served  int
}

type subjectReply struct {
	status   int
	upstream bool
	subject  string
	body     string
}

func (d *subjectDataPlane) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	reply := d.replies[min(d.served, len(d.replies)-1)]
	d.served++
	d.mu.Unlock()
	if reply.upstream {
		w.Header().Set(headerUpstreamServiceTime, "3")
	}
	if reply.subject != "" {
		w.Header().Set(tlsbackend.HeaderClientSubject, reply.subject)
	}
	w.WriteHeader(reply.status)
	_, _ = w.Write([]byte(reply.body))
}

func TestSendUntilBackendSees(t *testing.T) {
	const gatewayA = "CN=gateway-a"
	noRoute := subjectReply{status: 404, body: noRouteBody}
	warming := subjectReply{status: 503, body: "no healthy upstream"}
	for _, tc := range []struct {
		name    string
		phrase  string
		replies []subjectReply
		status  int
		err     string
		served  int
	}{
		{name: "polls through the route coming up and the old view", phrase: `the client certificate of "gw-identity-a" while tolerating no client certificate`,
			replies: []subjectReply{noRoute, warming, {status: 200, upstream: true}, {status: 200, upstream: true, subject: gatewayA}},
			status:  200, served: 4},
		{name: "two tolerated views", phrase: `the client certificate of "gw-identity-a" while tolerating no client certificate or a refusal`,
			replies: []subjectReply{{status: 400, upstream: true}, {status: 200, upstream: true}, {status: 200, upstream: true, subject: gatewayA}},
			status:  200, served: 3},
		{name: "a subject that was not listed ends the wait", phrase: `the client certificate of "gw-identity-a" while tolerating a refusal`,
			replies: []subjectReply{{status: 200, upstream: true, subject: "CN=intruder"}}, err: `sees the client certificate "CN=intruder"`, served: 1},
		{name: "the old view is fatal without a tolerance", phrase: `the client certificate of "gw-identity-a"`,
			replies: []subjectReply{{status: 200, upstream: true}}, err: "sees no client certificate", served: 1},
		{name: "a refusal is awaited", phrase: `a refusal`,
			replies: []subjectReply{noRoute, {status: 400, upstream: true}}, status: 400, served: 2},
		{name: "an unknown view", phrase: `somebody`, err: "unknown backend view"},
		{name: "an unknown tolerated view", phrase: `a refusal while tolerating somebody`, err: "unknown backend view"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataPlane := &subjectDataPlane{replies: tc.replies}
			if len(tc.replies) == 0 {
				dataPlane.replies = []subjectReply{{status: 500}}
			}
			server := httptest.NewServer(dataPlane)
			t.Cleanup(server.Close)
			g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
			g.base = stubBase{dataPlane: server.URL}

			err := g.sendUntilBackendSees(ctx, "get", "/anything", tc.phrase)
			if tc.err == "" {
				require.NoError(t, err)
				published, pubErr := httpx.Published(ctx)
				require.NoError(t, pubErr)
				require.Equal(t, tc.status, published.StatusCode)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
			require.Equal(t, tc.served, dataPlane.served)
		})
	}
}

func TestListenerSubjectIsReadFromTheHTTPSListener(t *testing.T) {
	https := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(https.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), https)

	subject, err := g.listenerSubject(ctx)
	require.NoError(t, err)
	require.Equal(t, https.Certificate().Subject.String(), subject)

	dataPlane := &subjectDataPlane{replies: []subjectReply{{status: 200, upstream: true, subject: subject}}}
	server := httptest.NewServer(dataPlane)
	t.Cleanup(server.Close)
	g.base = stubBase{dataPlane: server.URL}
	require.NoError(t, g.sendUntilBackendSees(ctx, "GET", "/anything", "the gateway listener certificate"))

	stopped := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed, closedCtx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), stopped)
	stopped.Close()
	_, err = closed.listenerSubject(closedCtx)
	require.ErrorContains(t, err, "reading the HTTPS listener certificate")
}

func TestStoreEchoBackendURL(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	require.NoError(t, g.storeEchoBackendURL(ctx, "optional", "optionalBackend"))
	require.NoError(t, g.storeEchoBackendURL(ctx, "required", "requiredBackend"))
	optional, err := stepscommon.StoredValue(ctx, "optionalBackend")
	require.NoError(t, err)
	require.Equal(t, "https://tls-backend:8446", optional)
	required, err := stepscommon.StoredValue(ctx, "requiredBackend")
	require.NoError(t, err)
	require.Equal(t, "https://tls-backend:8443", required)
	require.ErrorContains(t, g.storeEchoBackendURL(ctx, "other", "x"), `unknown TLS backend kind "other"`)
	require.ErrorContains(t, g.storeEchoBackendURL(context.Background(), "optional", "x"), "without runner context")
}

// fakeIdentityController serves the certificate endpoints the identity steps use.
type fakeIdentityController struct {
	mu       sync.Mutex
	identity []identityEntry
	status   int
	posted   []map[string]string
	put      map[string]map[string]string
	deleted  []string
	message  string
}

func (f *fakeIdentityController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	write := func(status int, v any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	switch {
	case r.URL.Path == "/api/management/v1/certificates" && r.Method == http.MethodPost:
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.posted = append(f.posted, body)
		if f.status != http.StatusCreated {
			write(f.status, map[string]string{"message": f.message})
			return
		}
		write(http.StatusCreated, map[string]string{"id": "cert-" + body["name"]})
	case r.URL.Path == "/api/management/v1/certificates" && r.URL.Query().Get("usage") == "identity":
		write(http.StatusOK, map[string]any{"certificates": f.identity})
	case strings.HasPrefix(r.URL.Path, "/api/management/v1/certificates/") && r.Method == http.MethodPut:
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if f.put == nil {
			f.put = map[string]map[string]string{}
		}
		f.put[strings.TrimPrefix(r.URL.Path, "/api/management/v1/certificates/")] = body
		write(http.StatusOK, map[string]string{"status": "success"})
	case strings.HasPrefix(r.URL.Path, "/api/management/v1/certificates/") && r.Method == http.MethodDelete:
		f.deleted = append(f.deleted, strings.TrimPrefix(r.URL.Path, "/api/management/v1/certificates/"))
		write(http.StatusOK, map[string]string{"status": "success"})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestUploadingAGatewayIdentityCarriesKeyAndRole(t *testing.T) {
	fake := &fakeIdentityController{status: http.StatusCreated}
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "default-identity", "defaultIdentity"))
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "other-identity", "otherIdentity"))
	gatewayA := mtlsFixture(t, "gw-identity-a")

	require.NoError(t, g.storeGatewayIdentityWithRole(ctx, "${CTX:defaultIdentity}", "gw-identity-a", " with role default"))
	require.NoError(t, g.uploadGatewayIdentity(ctx, "${CTX:otherIdentity}", "gw-identity-a", ""))
	require.Len(t, fake.posted, 2)

	name, err := stepscommon.StoredValue(ctx, "defaultIdentity")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"name": name, "usage": "identity", "role": "default",
		"certificate": string(gatewayA.CertPEM), "privateKey": string(gatewayA.KeyPEM),
	}, fake.posted[0])
	require.NotContains(t, fake.posted[1], "role")
	require.Equal(t, string(gatewayA.KeyPEM), fake.posted[1]["privateKey"])

	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Len(t, registry.Pending(), 2)

	require.ErrorContains(t, g.uploadGatewayIdentity(ctx, "literal-name", "gw-identity-a", ""), "is not generated")
	require.ErrorContains(t, g.uploadGatewayIdentity(ctx, "${CTX:otherIdentity}", "no-such-fixture", ""), "unknown fixture")
}

func TestUploadingAnotherUsageCarriesTheRoleWithoutAKey(t *testing.T) {
	fake := &fakeIdentityController{status: http.StatusBadRequest, message: "invalid"}
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "pool", "pool"))

	require.NoError(t, g.uploadFixtureWithRole(ctx, "ca-b", "${CTX:pool}", "upstream", "default", ""))
	require.Len(t, fake.posted, 1)
	require.Equal(t, "upstream", fake.posted[0]["usage"])
	require.Equal(t, "default", fake.posted[0]["role"])
	require.NotContains(t, fake.posted[0], "privateKey")

	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Empty(t, registry.Pending(), "a refused upload leaves nothing to clean up")
}

func TestARefusedGatewayIdentityIsReportedWithItsAnswer(t *testing.T) {
	fake := &fakeIdentityController{status: http.StatusConflict, message: "already has role: default"}
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "default-identity", "defaultIdentity"))

	err := g.storeGatewayIdentityWithRole(ctx, "${CTX:defaultIdentity}", "gw-identity-a", " with role default")
	require.ErrorContains(t, err, "status 201")
	require.ErrorContains(t, err, "already has role: default")
	require.NoError(t, g.uploadGatewayIdentity(ctx, "${CTX:defaultIdentity}", "gw-identity-a", " with role default"))
	published, pubErr := httpx.Published(ctx)
	require.NoError(t, pubErr)
	require.Equal(t, http.StatusConflict, published.StatusCode)
}

func TestRotatingAndRemovingAGatewayIdentityAddressItByName(t *testing.T) {
	fake := &fakeIdentityController{identity: []identityEntry{
		{ID: "id-1", Name: "first", Role: "default"}, {ID: "id-2", Name: "second"},
	}}
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	gatewayB := mtlsFixture(t, "gw-identity-b")

	require.NoError(t, g.rotateGatewayIdentity(ctx, "first", "gw-identity-b"))
	require.Equal(t, map[string]string{
		"certificate": string(gatewayB.CertPEM), "privateKey": string(gatewayB.KeyPEM),
	}, fake.put["id-1"])
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode)

	require.NoError(t, g.deleteGatewayIdentity(ctx, "second"))
	require.Equal(t, []string{"id-2"}, fake.deleted)

	require.ErrorContains(t, g.rotateGatewayIdentity(ctx, "missing", "gw-identity-b"), `no identity named "missing"`)
	require.ErrorContains(t, g.deleteGatewayIdentity(ctx, "missing"), `no identity named "missing"`)
	require.ErrorContains(t, g.rotateGatewayIdentity(ctx, "first", "no-such-fixture"), "unknown fixture")
}

func TestListingShowsDefaultOnly(t *testing.T) {
	for _, tc := range []struct {
		name     string
		identity []identityEntry
		err      string
	}{
		{name: "the default alone", identity: []identityEntry{{Name: "first", Role: "default"}, {Name: "second"}}},
		{name: "the named identity has no role", identity: []identityEntry{{Name: "first"}, {Name: "second"}}, err: `"first" has role ""`},
		{name: "another identity is the default too", identity: []identityEntry{{Name: "first", Role: "default"}, {Name: "second", Role: "default"}},
			err: `"second" has role "default", only "first" should have one`},
		{name: "the identity is missing", identity: []identityEntry{{Name: "second"}}, err: `no identity named "first"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, ctx := mtlsGatewayUnderTest(t, &fakeIdentityController{identity: tc.identity}, nil)
			err := g.listingShowsDefaultOnly(ctx, "first")
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestRoleFromPhrase(t *testing.T) {
	require.Equal(t, "", roleFromPhrase(""))
	require.Equal(t, "default", roleFromPhrase(" with role default"))
}

const askedServerName = "asked.example"

// listenerAskingOn serves HTTPS and asks for a client certificate only on connections that send
// askedServerName, as the gateway's listener does for the hostname of an mtls-auth API. It
// records the server name of the last handshake.
func listenerAskingOn(t *testing.T) (*httptest.Server, *atomic.Value) {
	t.Helper()
	var sni atomic.Value
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello " + r.Host))
	}))
	server.TLS = &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			sni.Store(hello.ServerName)
			if strings.EqualFold(hello.ServerName, askedServerName) {
				return &tls.Config{Certificates: server.TLS.Certificates, ClientAuth: tls.RequestClientCert}, nil
			}
			return nil, nil
		},
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, &sni
}

func TestServerNameChoiceAppliesToTheClientHandshake(t *testing.T) {
	opts := &httpx.ClientTLS{ServerName: gatewaySNI}
	serverNameChoice{}.apply(opts)
	require.Equal(t, gatewaySNI, opts.ServerName)
	require.False(t, opts.OmitServerName)

	serverNameChoice{name: "api.example"}.apply(opts)
	require.Equal(t, "api.example", opts.ServerName)

	serverNameChoice{none: true}.apply(opts)
	require.Empty(t, opts.ServerName)
	require.True(t, opts.OmitServerName)
}

func TestServerNameChoiceOfExpandsAndRejectsNothing(t *testing.T) {
	_, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "orders", "label"))
	label, err := stepscommon.StoredValue(ctx, "label")
	require.NoError(t, err)

	choice, err := serverNameChoiceOf(ctx, "${CTX:label}.example", "")
	require.NoError(t, err)
	require.Equal(t, label+".example", choice.name)

	choice, err = serverNameChoiceOf(ctx, "${CTX:label}.example", " in upper case")
	require.NoError(t, err)
	require.Equal(t, strings.ToUpper(label)+".EXAMPLE", choice.name)

	_, err = serverNameChoiceOf(ctx, " ", "")
	require.ErrorContains(t, err, "expands to nothing")
	_, err = serverNameChoiceOf(ctx, "${CTX:missing}.example", "")
	require.ErrorContains(t, err, `no value in context for key "missing"`)
}

func TestHTTPSRequestsSendTheChosenServerNameUntilTheScenarioEnds(t *testing.T) {
	server, sni := listenerAskingOn(t)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), server)
	send := func() *httpx.TLSState {
		require.NoError(t, g.sendHTTPS(ctx, "GET", "/anything", "with no client certificate"))
		published, err := httpx.Published(ctx)
		require.NoError(t, err)
		return published.TLS
	}

	require.Equal(t, gatewaySNI, send().ServerName)
	require.Equal(t, gatewaySNI, sni.Load())

	require.NoError(t, sendServerName(ctx, askedServerName, " in upper case"))
	state := send()
	require.Equal(t, strings.ToUpper(askedServerName), state.ServerName)
	require.True(t, state.ClientCertificateRequested)

	require.NoError(t, sendNoServerName(ctx))
	state = send()
	require.Empty(t, state.ServerName)
	require.Empty(t, sni.Load())
	require.False(t, state.ClientCertificateRequested)

	_, err := resetHostnameState(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, gatewaySNI, send().ServerName)
}

func TestObserveAskedReportsTheListenersCertificateRequest(t *testing.T) {
	server, _ := listenerAskingOn(t)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), server)

	asked, err := g.observeAsked(ctx, serverNameChoice{name: askedServerName})
	require.NoError(t, err)
	require.True(t, asked)
	asked, err = g.observeAsked(ctx, serverNameChoice{name: strings.ToUpper(askedServerName)})
	require.NoError(t, err)
	require.True(t, asked)
	asked, err = g.observeAsked(ctx, serverNameChoice{name: "public.example"})
	require.NoError(t, err)
	require.False(t, asked)
	asked, err = g.observeAsked(ctx, serverNameChoice{none: true})
	require.NoError(t, err)
	require.False(t, asked)

	server.Close()
	_, err = g.observeAsked(ctx, serverNameChoice{name: askedServerName})
	require.Equal(t, "tolerated", outcome(err), "%v", err)
	require.ErrorContains(t, err, "did not complete a handshake")
}

func TestHandshakeAskedWaitsForTheStateTheStepNames(t *testing.T) {
	server, _ := listenerAskingOn(t)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), server)

	require.NoError(t, g.handshakeAsked(ctx, `with server name "asked.example"`, askedServerName, "", ""))
	require.NoError(t, g.handshakeAsked(ctx, `with server name "asked.example" in upper case`, askedServerName, " in upper case", ""))
	require.NoError(t, g.awaitNotAsked(ctx, serverNameChoice{name: "public.example"}, 300*time.Millisecond))
	require.NoError(t, g.awaitNotAsked(ctx, serverNameChoice{none: true}, 300*time.Millisecond))
	require.ErrorContains(t, g.handshakeAsked(ctx, `with server name "${CTX:missing}"`, "${CTX:missing}", "", "not "),
		`no value in context for key "missing"`)
}

func TestNotAskedHoldFailsTheMomentTheListenerAsks(t *testing.T) {
	var asking atomic.Bool
	flipped := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	flipped.TLS = &tls.Config{
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			if asking.Load() {
				return &tls.Config{Certificates: flipped.TLS.Certificates, ClientAuth: tls.RequestClientCert}, nil
			}
			return nil, nil
		},
	}
	flipped.StartTLS()
	t.Cleanup(flipped.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), flipped)
	time.AfterFunc(700*time.Millisecond, func() { asking.Store(true) })
	err := g.awaitNotAsked(ctx, serverNameChoice{name: "x.example"}, 5*time.Second)
	require.ErrorContains(t, err, "the invariant was violated")
}

func TestConnectionAskedAndSessionResumedReadThePublishedHandshake(t *testing.T) {
	_, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, time.Millisecond)

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: 200}))
	require.ErrorContains(t, connectionAsked(ctx, ""), "did not arrive over an HTTPS request step")
	require.ErrorContains(t, sessionResumed(ctx), "did not arrive over an HTTPS request step")

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: 200, TLS: &httpx.TLSState{ServerName: "a.example", ClientCertificateRequested: true}}))
	require.NoError(t, connectionAsked(ctx, ""))
	require.ErrorContains(t, connectionAsked(ctx, "not "), `asked the connection for a client certificate (server name "a.example")`)
	require.ErrorContains(t, sessionResumed(ctx), "full TLS handshake")

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: 200, TLS: &httpx.TLSState{DidResume: true}}))
	require.NoError(t, connectionAsked(ctx, "not "))
	require.ErrorContains(t, connectionAsked(ctx, ""), "did not ask the connection")
	require.NoError(t, sessionResumed(ctx))
}

func TestPoolRelayFixtureUploadsTheRelayRoleAndRegistersItForCleanup(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "relay", "relayName"))

	require.ErrorContains(t, g.poolRelayFixture(ctx, "edge-lb-ca", "literal-name"), "is not generated")
	require.ErrorContains(t, g.poolRelayFixture(ctx, "no-such-fixture", "${CTX:relayName}"), "unknown fixture")
	require.NoError(t, g.poolRelayFixture(ctx, "edge-lb-ca", "${CTX:relayName}"))

	name, err := stepscommon.StoredValue(ctx, "relayName")
	require.NoError(t, err)
	require.Len(t, fake.uploads, 1)
	require.Equal(t, name, fake.uploads[0]["name"])
	require.Equal(t, "downstream", fake.uploads[0]["usage"])
	require.Equal(t, "relay", fake.uploads[0]["role"])
	require.Equal(t, string(mtlsFixture(t, "edge-lb-ca").CertPEM), fake.uploads[0]["certificate"])
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Contains(t, published.Text(), "cert-1")
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	pending := registry.Pending()
	require.Len(t, pending, 1)
	require.Equal(t, cleanup.KindCertificate.Name, pending[0].Kind.Name)
	require.Equal(t, "cert-1", pending[0].ID)

	fake.uploadStatus = http.StatusConflict
	require.ErrorContains(t, g.poolRelayFixture(ctx, "edge-lb-ca", "${CTX:relayName}"), "status 201")
	require.Len(t, registry.Pending(), 1)
}

func TestKeptAliveConnectionCarriesSeveralRequestsAndNamesAClosure(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("host=" + r.Host + " path=" + r.URL.Path))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), server)

	require.ErrorContains(t, g.sendOnKeptAliveConnection(ctx, "/second"), "open one first")

	require.NoError(t, g.openKeptAliveConnection(ctx, "public.example", "anything"))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode)
	require.Equal(t, "host=public.example path=/anything", published.Text())

	require.NoError(t, g.sendOnKeptAliveConnection(ctx, "/second"))
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, "host=public.example path=/second", published.Text())
	require.EqualValues(t, 1, connections.Load())
	require.ErrorContains(t, g.keptAliveConnectionStillOpen(ctx, 3), "carried 2")
	require.NoError(t, g.keptAliveConnectionStillOpen(ctx, 2))

	server.CloseClientConnections()
	err = g.sendOnKeptAliveConnection(ctx, "/third")
	require.Error(t, err)
	require.Contains(t, err.Error(), "kept-alive HTTPS connection")
	_, published2 := httpx.Published(ctx)
	require.Error(t, published2, "a failed request leaves no stale response behind")

	_, err = closeKeptAliveConnection(ctx, nil, nil)
	require.NoError(t, err)
	require.False(t, tcontext.Contains(ctx, keyKeptAliveConnection))
	require.ErrorContains(t, g.sendOnKeptAliveConnection(ctx, "/fourth"), "open one first")
}

// fakeCertificates is the controller's certificates endpoint: it keeps what is uploaded and
// records every request.
type fakeCertificates struct {
	mu        sync.Mutex
	status    int // answer to an upload; 0 means 201
	listed    []map[string]string
	requests  []string
	bodies    [][]byte
	deleteAns int // answer to a delete; 0 means 204
}

func (f *fakeCertificates) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.bodies = append(f.bodies, body)
	switch {
	case r.Method == http.MethodPost:
		status := f.status
		if status == 0 {
			status = http.StatusCreated
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":"cert-1","status":"success"}`))
	case r.Method == http.MethodGet:
		out := []map[string]string{}
		for _, c := range f.listed {
			if usage := r.URL.Query().Get("usage"); usage == "" || usage == c["usage"] {
				out = append(out, c)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"certificates": out})
	case r.Method == http.MethodDelete:
		status := f.deleteAns
		if status == 0 {
			status = http.StatusNoContent
		}
		w.WriteHeader(status)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func certificatesUnderTest(t *testing.T) (*fakeCertificates, *Gateway, context.Context) {
	t.Helper()
	fake := &fakeCertificates{listed: []map[string]string{
		{"id": "id-identity", "name": "identity-1", "usage": "identity"},
		{"id": "id-trust", "name": "trust-1", "usage": "upstream"},
	}}
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "identity", "identity"))
	return fake, g, ctx
}

func (f *fakeCertificates) last() (string, map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]string
	_ = json.Unmarshal(f.bodies[len(f.bodies)-1], &body)
	return f.requests[len(f.requests)-1], body
}

func TestIdentityMaterial(t *testing.T) {
	leaf := mtlsFixture(t, "gw-identity-a")
	certificate, key, err := identityMaterial(leaf, false)
	require.NoError(t, err)
	require.Equal(t, string(leaf.CertPEM), certificate)
	require.Equal(t, string(leaf.KeyPEM), key)

	chained := mtlsFixture(t, "gw-identity-via-intermediate")
	certificate, _, err = identityMaterial(chained, true)
	require.NoError(t, err)
	require.Equal(t, string(chained.CertPEM)+string(chained.ChainPEM), certificate)
	require.Equal(t, 2, strings.Count(certificate, "BEGIN CERTIFICATE"))

	_, _, err = identityMaterial(leaf, true)
	require.ErrorContains(t, err, "has no issuing chain")
}

func TestPublishedFixtureMaterialIsJSONReady(t *testing.T) {
	_, _, ctx := certificatesUnderTest(t)
	require.NoError(t, func() error { _, err := publishFixtureMaterial(ctx, taggedScenario(t, tagMTLS)); return err }())

	for _, form := range []string{"pem", "key", "encryptedKey"} {
		value, err := tcontext.ResolveString(ctx, "fixture.gw-identity-a."+form)
		require.NoError(t, err, form)
		var decoded string
		require.NoError(t, json.Unmarshal([]byte(`"`+value+`"`), &decoded), form)
		require.Contains(t, decoded, "-----BEGIN", form)
		require.NotContains(t, value, "\n", form)
	}
	pem, err := tcontext.ResolveString(ctx, "fixture.gw-identity-a.pem")
	require.NoError(t, err)
	require.Equal(t, jsonEscaped(mtlsFixture(t, "gw-identity-a").CertPEM), pem)
	encrypted, err := tcontext.ResolveString(ctx, "fixture.gw-identity-a.encryptedKey")
	require.NoError(t, err)
	require.Contains(t, encrypted, "ENCRYPTED PRIVATE KEY")

	expanded, err := stepscommon.Expand(ctx, `{"privateKey":"${CTX:fixture.key-mismatch.key}"}`)
	require.NoError(t, err)
	require.Contains(t, expanded, "PRIVATE KEY")
}

func TestUploadIdentityPostsTheIdentityAndRegistersIt(t *testing.T) {
	fake, g, ctx := certificatesUnderTest(t)
	name, err := stepscommon.StoredValue(ctx, "identity")
	require.NoError(t, err)

	require.ErrorContains(t, g.uploadIdentity(ctx, "gw-identity-a", "literal"), "is not generated")
	require.ErrorContains(t, g.uploadIdentity(ctx, "no-such-fixture", "${CTX:identity}"), "unknown fixture")

	require.NoError(t, g.uploadIdentity(ctx, "gw-identity-a", "${CTX:identity}"))
	request, body := fake.last()
	require.Equal(t, "POST /api/management/v1/certificates", request)
	require.Equal(t, map[string]string{
		"name": name, "usage": "identity",
		"certificate": string(mtlsFixture(t, "gw-identity-a").CertPEM),
		"privateKey":  string(mtlsFixture(t, "gw-identity-a").KeyPEM),
	}, body)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Len(t, registry.Pending(), 1)
	require.Equal(t, "cert-1", registry.Pending()[0].ID)

	require.NoError(t, g.uploadIdentityWithChain(ctx, "gw-identity-via-intermediate", "${CTX:identity}"))
	_, body = fake.last()
	require.Equal(t, 2, strings.Count(body["certificate"], "BEGIN CERTIFICATE"))
	require.ErrorContains(t, g.uploadIdentityWithChain(ctx, "gw-identity-a", "${CTX:identity}"), "has no issuing chain")
}

func TestStoreIdentityRequiresACreatedIdentity(t *testing.T) {
	fake, g, ctx := certificatesUnderTest(t)
	require.NoError(t, g.storeIdentity(ctx, "gw-identity-a", "${CTX:identity}"))

	fake.status = http.StatusConflict
	require.ErrorContains(t, g.storeIdentity(ctx, "gw-identity-a", "${CTX:identity}"), "status 201")
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Len(t, registry.Pending(), 1, "a refused upload registers nothing")
}

func TestUploadIdentityBodyExpandsFixtureMaterialAndRegistersACreatedCertificate(t *testing.T) {
	fake, g, ctx := certificatesUnderTest(t)
	require.NoError(t, func() error { _, err := publishFixtureMaterial(ctx, taggedScenario(t, tagMTLS)); return err }())
	body := &godog.DocString{Content: `{"name":"${CTX:identity}","usage":"identity","certificate":"${CTX:fixture.gw-identity-a.pem}","privateKey":"${CTX:fixture.gw-identity-a.key}"}`}

	require.NoError(t, g.uploadIdentityBody(ctx, body))
	_, sent := fake.last()
	require.Equal(t, string(mtlsFixture(t, "gw-identity-a").CertPEM), sent["certificate"])
	require.Equal(t, string(mtlsFixture(t, "gw-identity-a").KeyPEM), sent["privateKey"])
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Len(t, registry.Pending(), 1)

	fake.status = http.StatusBadRequest
	require.NoError(t, g.uploadIdentityBody(ctx, body))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)
	require.Len(t, registry.Pending(), 1)

	require.NoError(t, g.uploadIdentityBody(ctx, &godog.DocString{Content: "not json"}))
	require.ErrorContains(t, g.uploadIdentityBody(ctx, &godog.DocString{Content: "${CTX:missing}"}), "no value in context")
}

func TestDeleteResolvesTheNameAndKeepsRefusedDeletesRegistered(t *testing.T) {
	fake, g, ctx := certificatesUnderTest(t)
	fake.listed[0]["name"] = mustStored(t, ctx, "identity")
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindCertificate, ID: "id-identity", Actor: "admin"}))

	require.NoError(t, g.deleteCertificateNamed(ctx, "${CTX:identity}"))
	request, _ := fake.last()
	require.Equal(t, "DELETE /api/management/v1/certificates/id-identity", request)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.Empty(t, registry.Pending(), "a deleted identity leaves the cleanup list")

	require.NoError(t, g.deleteCertificateNamed(ctx, "trust-1"))
	request, _ = fake.last()
	require.Equal(t, "DELETE /api/management/v1/certificates/id-trust", request)

	fake.deleteAns = http.StatusConflict
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindCertificate, ID: "id-trust", Actor: "admin"}))
	require.NoError(t, g.deleteCertificateNamed(ctx, "trust-1"))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, published.StatusCode)
	require.Len(t, registry.Pending(), 1, "a refused deletion leaves the certificate registered")
}

func mustStored(t *testing.T, ctx context.Context, key string) string {
	t.Helper()
	value, err := stepscommon.StoredValue(ctx, key)
	require.NoError(t, err)
	return value
}

func TestIdentityUpdates(t *testing.T) {
	fake, g, ctx := certificatesUnderTest(t)
	fake.listed[0]["name"] = mustStored(t, ctx, "identity")

	require.NoError(t, g.rotateIdentity(ctx, "${CTX:identity}", "gw-identity-via-intermediate"))
	request, body := fake.last()
	require.Equal(t, "PUT /api/management/v1/certificates/id-identity", request)
	require.Equal(t, 2, strings.Count(body["certificate"], "BEGIN CERTIFICATE"))
	require.Equal(t, string(mtlsFixture(t, "gw-identity-via-intermediate").KeyPEM), body["privateKey"])
	require.Len(t, body, 2, "an update names neither the certificate nor its usage")

	require.NoError(t, g.updateCertificateWithIdentity(ctx, "trust-1", "gw-identity-a"))
	request, body = fake.last()
	require.Equal(t, "PUT /api/management/v1/certificates/id-trust", request)
	require.Equal(t, string(mtlsFixture(t, "gw-identity-a").CertPEM), body["certificate"])

	require.ErrorContains(t, g.rotateIdentity(ctx, "${CTX:identity}", "gw-identity-a"), "has no issuing chain")
	require.ErrorContains(t, g.updateCertificateWithIdentity(ctx, "trust-1", "no-such-fixture"), "unknown fixture")
}

func publishStatus(t *testing.T, ctx context.Context, status int, body string) {
	t.Helper()
	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, time.Millisecond)
	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: status, Body: []byte(body)}))
}

func upstreamReply(status int, body string) *httpx.Response {
	headers := http.Header{}
	headers.Set(headerUpstreamServiceTime, "3")
	return &httpx.Response{StatusCode: status, Headers: headers, Body: []byte(body)}
}

func envoyReply(status int, body string) *httpx.Response {
	return &httpx.Response{StatusCode: status, Headers: http.Header{}, Body: []byte(body)}
}

const connectFailureBody = "upstream connect error or disconnect/reset before headers. reset reason: remote connection failure, transport failure reason: TLS_error"

func TestJudgeBackendRefusal(t *testing.T) {
	for _, tc := range []struct {
		name string
		resp *httpx.Response
		want string
	}{
		{"the backend's 400", upstreamReply(400, `{"backend":"a"}`), ""},
		{"no route yet", envoyReply(404, noRouteBody), "tolerated"},
		{"cluster warming", envoyReply(503, "no healthy upstream"), "tolerated"},
		{"an upstream connect failure is not a warming cluster", envoyReply(503, connectFailureBody), "fail"},
		{"the backend accepted the request", upstreamReply(200, "{}"), "fail"},
		{"a 400 Envoy produced itself", envoyReply(400, ""), "fail"},
		{"a backend 404", upstreamReply(404, "{}"), "fail"},
		{"a backend 503", upstreamReply(503, "down"), "fail"},
	} {
		require.Equal(t, tc.want, outcome(judgeBackendRefusal(tc.resp)), tc.name)
	}
	require.ErrorContains(t, judgeBackendRefusal(envoyReply(503, connectFailureBody)), "upstream TLS connection failed")
}

func TestJudgeUpstreamTLSFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		resp *httpx.Response
		want string
	}{
		{"the connect failure", envoyReply(503, connectFailureBody), ""},
		{"no route yet", envoyReply(404, noRouteBody), "tolerated"},
		{"cluster warming", envoyReply(503, ""), "tolerated"},
		{"the backend answered, so the connection succeeded", upstreamReply(200, "{}"), "fail"},
		{"the backend refused", upstreamReply(400, "{}"), "fail"},
		{"a backend 503 carrying the marker", upstreamReply(503, connectFailureBody), "fail"},
		{"another Envoy status", envoyReply(502, ""), "fail"},
	} {
		require.Equal(t, tc.want, outcome(judgeUpstreamTLSFailure(tc.resp)), tc.name)
	}
}

func TestSendUntilJudgedPollsThroughInBetweenStatesAndSettles(t *testing.T) {
	var mu sync.Mutex
	answers := []func(http.ResponseWriter){
		func(w http.ResponseWriter) { w.WriteHeader(404); _, _ = w.Write([]byte(noRouteBody)) },
		func(w http.ResponseWriter) { w.WriteHeader(503) },
		func(w http.ResponseWriter) { w.Header().Set(headerUpstreamServiceTime, "1"); w.WriteHeader(400) },
	}
	calls := 0
	dataPlane := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		answers[min(calls, len(answers)-1)](w)
		calls++
	}))
	t.Cleanup(dataPlane.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	g.base = stubBase{dataPlane: dataPlane.URL}

	require.NoError(t, g.sendUntilBackendRefuses(ctx, "get", "/out/anything"))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, 400, published.StatusCode)
	require.Equal(t, 3, calls)
}

func TestSendUntilJudgedFailsAtOnceOnAContradiction(t *testing.T) {
	calls := 0
	dataPlane := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set(headerUpstreamServiceTime, "1")
		_, _ = w.Write([]byte("accepted"))
	}))
	t.Cleanup(dataPlane.Close)
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	g.base = stubBase{dataPlane: dataPlane.URL}

	err := g.sendUntilUpstreamTLSFails(ctx, "GET", "/out/anything")
	require.ErrorContains(t, err, "expected Envoy's 503")
	require.ErrorContains(t, err, "200")
	require.Equal(t, 1, calls)
	require.ErrorContains(t, g.sendUntilUpstreamTLSFails(ctx, "GET", "${CTX:missing}"), "no value in context")
}

func TestPoolWithoutUsageOmitsTheUsage(t *testing.T) {
	fake, g, ctx := certificatesUnderTest(t)
	require.NoError(t, g.poolCertificateFixturesWithoutUsage(ctx, "backend-ca", "${CTX:identity}"))
	_, body := fake.last()
	_, hasUsage := body["usage"]
	require.False(t, hasUsage)
	require.Equal(t, string(mtlsFixture(t, "backend-ca").CertPEM), body["certificate"])
}

// fakeCertificateStore serves the controller's certificate collection.
type fakeCertificateStore struct {
	mu          sync.Mutex
	listing     []map[string]any
	references  []int // referencedByApis served on successive listings; the last repeats
	listings    int
	postStatus  int
	postBodies  [][]byte
	deleteCode  int
	deletedIDs  []string
	deleteAuths []string
}

func (f *fakeCertificateStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	const collection = "/api/management/v1/certificates"
	switch {
	case r.URL.Path == collection && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		f.postBodies = append(f.postBodies, body)
		w.WriteHeader(f.postStatus)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": fmt.Sprintf("cert-%d", len(f.postBodies))})
	case r.URL.Path == collection:
		listing := f.listing
		if len(f.references) > 0 {
			i := min(f.listings, len(f.references)-1)
			listing = []map[string]any{{"id": "cert-1", "name": "named", "referencedByApis": f.references[i]}}
		}
		f.listings++
		_ = json.NewEncoder(w).Encode(map[string]any{"certificates": listing})
	case strings.HasPrefix(r.URL.Path, collection+"/") && r.Method == http.MethodDelete:
		f.deletedIDs = append(f.deletedIDs, strings.TrimPrefix(r.URL.Path, collection+"/"))
		f.deleteAuths = append(f.deleteAuths, r.Header.Get("Authorization"))
		w.WriteHeader(f.deleteCode)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// adminOnly rejects reads that are not the admin's, as the lookup must always be one.
func adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.Header.Get("Authorization") != BasicAuthHeader("admin", "secret") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func publish(t *testing.T, ctx context.Context, body string) {
	t.Helper()
	require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(body)}))
}

// taggedScenario builds a scenario carrying the given tags.
func taggedScenario(t *testing.T, tags ...string) *godog.Scenario {
	t.Helper()
	var doc struct {
		Tags []map[string]string `json:"tags"`
	}
	for _, tag := range tags {
		doc.Tags = append(doc.Tags, map[string]string{"name": tag})
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	var sc godog.Scenario
	require.NoError(t, json.Unmarshal(raw, &sc))
	return &sc
}

func pendingCertificates(t *testing.T, ctx context.Context) []string {
	t.Helper()
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	var ids []string
	for _, r := range registry.Pending() {
		if r.Kind.Name == cleanup.KindCertificate.Name {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func TestPublishFixtureMaterialOnlyForMTLSScenarios(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	_, err := publishFixtureMaterial(ctx, taggedScenario(t, "@other"))
	require.NoError(t, err)
	_, ok := tcontext.Get(ctx, fixturePEMKey("ca-a"))
	require.False(t, ok)
	_, err = publishFixtureMaterial(ctx, nil)
	require.NoError(t, err)

	_, err = publishFixtureMaterial(ctx, taggedScenario(t, "@other", tagMTLS))
	require.NoError(t, err)
	caA := mtlsFixture(t, "ca-a")
	body, err := stepscommon.Expand(ctx, `{"certificate":"${CTX:fixture.ca-a.pem}\n${CTX:fixture.ca-a.key}"}`)
	require.NoError(t, err)
	var decoded struct{ Certificate string }
	require.NoError(t, json.Unmarshal([]byte(body), &decoded))
	require.Equal(t, string(caA.CertPEM)+"\n"+string(caA.KeyPEM), decoded.Certificate)
	subject, err := stepscommon.Expand(ctx, "${CTX:fixture.ca-a-intermediate.subject}")
	require.NoError(t, err)
	require.Equal(t, "CN=Partner A Issuing CA 1,O=Partner A", subject)
	_, err = stepscommon.Expand(ctx, "${CTX:fixture.backend-ca.pem}")
	require.NoError(t, err)
}

func TestJSONEscaped(t *testing.T) {
	require.Equal(t, `a\nb\"c\\`, jsonEscaped([]byte("a\nb\"c\\")))
	require.Equal(t, "", jsonEscaped(nil))
}

func TestUploadFixtureSendsUsageRoleAndNarrowing(t *testing.T) {
	store := &fakeCertificateStore{postStatus: http.StatusCreated}
	g, ctx := mtlsGatewayUnderTest(t, store, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "relay", "relay"))

	require.ErrorContains(t, g.uploadFixtureWithRole(ctx, "edge-lb-ca", "literal", "downstream", "relay", ""), "is not generated")
	require.NoError(t, g.uploadFixtureWithRole(ctx, "edge-lb-ca", "${CTX:relay}", "downstream", "relay", ""))
	require.NoError(t, g.uploadFixtureWithRole(ctx, "edge-lb-ca", "${CTX:relay}", "downstream", "relay", "lb.corp.test"))
	require.NoError(t, g.uploadUpstreamFixture(ctx, "backend-ca", "${CTX:relay}"))

	var plain, narrowed, upstream map[string]any
	require.NoError(t, json.Unmarshal(store.postBodies[0], &plain))
	require.NoError(t, json.Unmarshal(store.postBodies[1], &narrowed))
	require.NoError(t, json.Unmarshal(store.postBodies[2], &upstream))
	require.Equal(t, "relay", plain["role"])
	require.Equal(t, "downstream", plain["usage"])
	require.NotContains(t, plain, "match")
	require.Equal(t, map[string]any{"dnsSANs": []any{"lb.corp.test"}}, narrowed["match"])
	require.NotContains(t, upstream, "usage")
	require.NotContains(t, upstream, "role")
	require.Equal(t, string(mtlsFixture(t, "backend-ca").CertPEM), upstream["certificate"])
	require.ElementsMatch(t, []string{"cert-1", "cert-2", "cert-3"}, pendingCertificates(t, ctx))
}

func TestPoolUpstreamFixtureRequiresCreated(t *testing.T) {
	store := &fakeCertificateStore{postStatus: http.StatusConflict}
	g, ctx := mtlsGatewayUnderTest(t, store, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "trust", "trust"))
	require.ErrorContains(t, g.poolCertificateFixturesWithoutUsage(ctx, "ca-a", "${CTX:trust}"), "status 201")
	require.ErrorContains(t, g.poolCertificateFixturesWithoutUsage(ctx, "no-such-fixture", "${CTX:trust}"), "unknown fixture")
	require.Empty(t, pendingCertificates(t, ctx))
}

func TestUploadCertificateBodyRegistersOnlyAnAcceptedUpload(t *testing.T) {
	store := &fakeCertificateStore{postStatus: http.StatusBadRequest}
	g, ctx := mtlsGatewayUnderTest(t, store, nil)
	require.NoError(t, tcontext.Set(ctx, "certName", "body-cert"))

	require.NoError(t, g.uploadCertificateBody(ctx, &godog.DocString{Content: `{"name":"${CTX:certName}","usage":"downstream"}`}))
	require.JSONEq(t, `{"name":"body-cert","usage":"downstream"}`, string(store.postBodies[0]))
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, published.StatusCode)
	require.Empty(t, pendingCertificates(t, ctx))

	store.postStatus = http.StatusCreated
	require.NoError(t, g.uploadCertificateBody(ctx, &godog.DocString{Content: `not json`}))
	require.Equal(t, "not json", string(store.postBodies[1]))
	require.Equal(t, []string{"cert-2"}, pendingCertificates(t, ctx))
	require.Error(t, g.uploadCertificateBody(ctx, &godog.DocString{Content: `${CTX:missing}`}))
}

func TestUploadOversizedCertificateExceedsTheLimitByATenth(t *testing.T) {
	store := &fakeCertificateStore{postStatus: http.StatusRequestEntityTooLarge}
	g, ctx := mtlsGatewayUnderTest(t, store, nil)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "big", "big"))
	require.NoError(t, g.uploadOversizedCertificate(ctx, "${CTX:big}", "downstream"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(store.postBodies[0], &body))
	require.Len(t, body["certificate"], certificateUploadLimitBytes+certificateUploadLimitBytes/10)
	require.Equal(t, "downstream", body["usage"])
	require.ErrorContains(t, g.uploadOversizedCertificate(ctx, "literal", "downstream"), "is not generated")
}

func TestDeleteCertificateNamedLooksUpAsAdminAndDeletesAsTheCaller(t *testing.T) {
	store := &fakeCertificateStore{
		listing:    []map[string]any{{"id": "id-a", "name": "pool-a"}, {"id": "id-b", "name": "pool-b"}},
		deleteCode: http.StatusForbidden,
	}
	g, ctx := mtlsGatewayUnderTest(t, adminOnly(store), nil)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.NoError(t, registry.Register(cleanup.Resource{Kind: cleanup.KindCertificate, ID: "id-b", Actor: "admin"}))
	require.NoError(t, tcontext.Set(ctx, "b", "pool-b"))

	require.NoError(t, g.deleteCertificateNamed(ctx, "${CTX:b}"))
	require.Equal(t, []string{"id-b"}, store.deletedIDs)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, published.StatusCode)
	require.Equal(t, []string{"id-b"}, pendingCertificates(t, ctx), "a refused delete stays registered")

	store.deleteCode = http.StatusNoContent
	require.NoError(t, g.deleteCertificateNamed(ctx, "pool-b"))
	require.Empty(t, pendingCertificates(t, ctx))
	require.ErrorContains(t, g.deleteCertificateNamed(ctx, "pool-c"), `no certificate named "pool-c"`)
}

func TestDeleteCertificateOnceUnreferenced(t *testing.T) {
	store := &fakeCertificateStore{references: []int{1, 0}, deleteCode: http.StatusOK}
	g, ctx := mtlsGatewayUnderTest(t, store, nil)
	require.NoError(t, g.deleteCertificateOnceUnreferenced(ctx, "named"))
	require.Equal(t, 2, store.listings)
	require.Equal(t, []string{"cert-1"}, store.deletedIDs)

	uncounted := &fakeCertificateStore{listing: []map[string]any{{"id": "x", "name": "named"}}}
	g, ctx = mtlsGatewayUnderTest(t, uncounted, nil)
	require.ErrorContains(t, g.deleteCertificateOnceUnreferenced(ctx, "named"), "without a referencedByApis count")
	require.Equal(t, 1, uncounted.listings, "a listing without a count fails at once")
	require.ErrorContains(t, g.deleteCertificateOnceUnreferenced(ctx, "absent"), `no certificate named "absent"`)
	require.Empty(t, uncounted.deletedIDs)
}

func TestCertificateListAssertions(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.NoError(t, tcontext.Set(ctx, "a", "pool-a"))
	publish(t, ctx, `{"certificates":[
		{"name":"pool-a","usage":"downstream","referencedByApis":0,"isLeaf":true,
		 "warnings":[{"code":"CERT_EXPIRES_SOON","field":"notAfter"}]},
		{"name":"pool-b","usage":"upstream","warnings":[]}],"totalCount":2}`)

	require.NoError(t, certificateListContains(ctx, "contain", "${CTX:a}"))
	require.NoError(t, certificateListContains(ctx, "not contain", "pool-c"))
	require.Error(t, certificateListContains(ctx, "contain", "pool-c"))
	require.Error(t, certificateListContains(ctx, "not contain", "pool-a"))

	require.NoError(t, listedCertificateFieldEquals(ctx, "${CTX:a}", "usage", `"downstream"`))
	require.NoError(t, listedCertificateFieldEquals(ctx, "pool-a", "referencedByApis", "0"))
	require.NoError(t, listedCertificateFieldEquals(ctx, "pool-a", "isLeaf", "true"))
	require.ErrorContains(t, listedCertificateFieldEquals(ctx, "pool-a", "referencedByApis", `"0"`), "expected 0 (string)")
	require.Error(t, listedCertificateFieldEquals(ctx, "pool-a", "isLeaf", "false"))
	require.Error(t, listedCertificateFieldEquals(ctx, "pool-a", "isLeaf", `"true"`))
	require.ErrorContains(t, listedCertificateFieldEquals(ctx, "pool-b", "referencedByApis", "0"), "without field")
	require.ErrorContains(t, listedCertificateFieldEquals(ctx, "pool-c", "usage", `"upstream"`), "not in the listing")

	require.NoError(t, listedCertificateLacksField(ctx, "pool-b", "referencedByApis"))
	require.Error(t, listedCertificateLacksField(ctx, "pool-a", "referencedByApis"))

	require.NoError(t, listedCertificateHasWarning(ctx, "pool-a", "code", "CERT_EXPIRES_SOON"))
	require.NoError(t, listedCertificateHasWarning(ctx, "pool-a", "field", "notAfter"))
	require.Error(t, listedCertificateHasWarning(ctx, "pool-a", "code", "notAfter"))
	require.Error(t, listedCertificateHasWarning(ctx, "pool-b", "code", "CERT_EXPIRES_SOON"))
	require.NoError(t, listedCertificateHasNoWarnings(ctx, "pool-b"))
	require.Error(t, listedCertificateHasNoWarnings(ctx, "pool-a"))

	publish(t, ctx, `{"status":"error"}`)
	require.ErrorContains(t, certificateListContains(ctx, "not contain", "pool-a"), "not a certificate listing")
	httpx.ClearPublished(ctx)
	require.Error(t, certificateListContains(ctx, "not contain", "pool-a"))
}

func TestValidationErrorListed(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.NoError(t, tcontext.Set(ctx, "api", "ref-api-1"))
	publish(t, ctx, `{"status":"error","errors":[
		{"field":"name","message":"name may contain only letters"},
		{"field":"spec.policies[0]","message":"named by ref-api-1"}]}`)

	require.NoError(t, validationErrorListed(ctx, "name", "with message", "name may contain only letters"))
	require.Error(t, validationErrorListed(ctx, "name", "with message", "name may contain"))
	require.NoError(t, validationErrorListed(ctx, "name", "containing", "may contain"))
	require.NoError(t, validationErrorListed(ctx, "spec.policies[0]", "containing", "${CTX:api}"))
	require.NoError(t, validationErrorListed(ctx, "name", "", ""))
	require.Error(t, validationErrorListed(ctx, "role", "", ""))
	require.Error(t, validationErrorListed(ctx, "spec.policies[0]", "with message", "name may contain only letters"))

	publish(t, ctx, `not json`)
	require.ErrorContains(t, validationErrorListed(ctx, "name", "", ""), "not a JSON error response")
}

// settleCounter counts the controller config dumps an observation of the gateway reads.
type settleCounter struct {
	http.Handler
	dumps atomic.Int32
}

func (c *settleCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/config_dump" {
		c.dumps.Add(1)
	}
	c.Handler.ServeHTTP(w, r)
}

func TestGatewayChangePendingState(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.False(t, gatewayChangePending(ctx), "nothing changed before a scenario starts")
	markGatewayChanged(ctx)
	require.True(t, gatewayChangePending(ctx))
	markGatewayChanged(ctx)
	require.True(t, gatewayChangePending(ctx))
	clearGatewayChange(ctx)
	require.False(t, gatewayChangePending(ctx))
}

func TestSettleAfterChangeSettlesOnlyWhenSomethingChanged(t *testing.T) {
	fake := &settleCounter{Handler: consistentMTLSGateway(t)}
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)

	require.NoError(t, g.settleAfterChange(ctx))
	require.Zero(t, fake.dumps.Load(), "no change, no settle")

	markGatewayChanged(ctx)
	require.NoError(t, g.settleAfterChange(ctx))
	settled := fake.dumps.Load()
	require.Positive(t, settled, "a change is settled")
	require.False(t, gatewayChangePending(ctx), "a settle clears the change")

	require.NoError(t, g.settleAfterChange(ctx))
	require.Equal(t, settled, fake.dumps.Load(), "a second request settles nothing")
}

func TestExplicitSettleClearsThePendingChange(t *testing.T) {
	g, ctx := mtlsGatewayUnderTest(t, consistentMTLSGateway(t), nil)
	markGatewayChanged(ctx)
	require.NoError(t, g.awaitGatewayApplied(ctx))
	require.False(t, gatewayChangePending(ctx))
}

func TestAFailedSettleKeepsTheChangePending(t *testing.T) {
	fake := consistentMTLSGateway(t)
	fake.controllerStatus = http.StatusForbidden
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	markGatewayChanged(ctx)
	require.Error(t, g.awaitGatewayApplied(ctx))
	require.True(t, gatewayChangePending(ctx))
}

func TestMTLSScenarioStartMarksTheGatewayChanged(t *testing.T) {
	fake := consistentMTLSGateway(t)
	fake.pool, fake.listenerNamesCA, fake.secret = map[string][]string{}, false, nil
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)

	_, err := g.beginMTLSScenario(ctx, taggedScenario(t, tagMTLS))
	require.NoError(t, err)
	require.True(t, gatewayChangePending(ctx), "the first request of a scenario settles")
	require.NoError(t, g.settleAfterChange(ctx))
	require.False(t, gatewayChangePending(ctx))
}

func TestStepsThatChangeTheGatewayMarkIt(t *testing.T) {
	fake := consistentMTLSGateway(t)
	g, ctx := mtlsGatewayUnderTest(t, fake, nil)
	require.False(t, gatewayChangePending(ctx))

	require.NoError(t, g.beforeAPIMutation(ctx))
	require.True(t, gatewayChangePending(ctx), "creating, updating or deleting an API")

	clearGatewayChange(ctx)
	require.NoError(t, stepscommon.GenerateResourceAndStore(ctx, "settle-cert", "cert"))
	require.NoError(t, g.poolCertificateFixtures(ctx, "ca-a", "${CTX:cert}", "downstream"))
	require.True(t, gatewayChangePending(ctx), "uploading a certificate")

	clearGatewayChange(ctx)
	require.NoError(t, g.deleteCertificate(ctx, storedCertificate{ID: "cert-1", Name: "x"}))
	require.True(t, gatewayChangePending(ctx), "deleting a certificate")

	clearGatewayChange(ctx)
	fake.extraCerts = []map[string]any{{"id": "cert-9", "name": "named-identity"}}
	require.NoError(t, g.rotateIdentity(ctx, "named-identity", "gw-identity-via-intermediate"))
	require.True(t, gatewayChangePending(ctx), "rotating an identity")
}

func TestPublishedCertificatesRequiresAnArrayAndATotalCount(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
		ok   bool
	}{
		{name: "null certificates", body: `{"certificates":null,"totalCount":0,"totalBytes":0,"status":"success"}`},
		{name: "no totalCount", body: `{"certificates":[]}`},
		{name: "empty array", body: `{"certificates":[],"totalCount":0}`, ok: true},
		{name: "one certificate", body: `{"certificates":[{"name":"a"}],"totalCount":1}`, want: 1, ok: true},
		{name: "error body", body: `{"status":"error","message":"nope"}`},
		{name: "not JSON", body: `nope`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
			require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(tt.body)}))
			certs, _, err := publishedCertificates(ctx)
			require.Equal(t, tt.ok, err == nil, "%v", err)
			require.Len(t, certs, tt.want)
		})
	}
}

func TestMetricNamedWithPrefixIgnoresLabelValues(t *testing.T) {
	exposition := "# HELP policy_executions_total Executions.\n" +
		"# TYPE policy_executions_total counter\n" +
		"policy_executions_total{api_name=\"mtls_auth_abc\",policy_name=\"mtls-auth\"} 3\n"
	_, found := metricNamedWithPrefix(exposition, "mtls_auth_")
	require.False(t, found)

	for _, withName := range []string{
		exposition + "mtls_auth_denied_total 1\n",
		exposition + "# TYPE mtls_auth_denied_total counter\n",
		exposition + "mtls_auth_denied_total{reason=\"x\"} 1\n",
	} {
		name, found := metricNamedWithPrefix(withName, "mtls_auth_")
		require.True(t, found)
		require.Equal(t, "mtls_auth_denied_total", name)
	}
}
