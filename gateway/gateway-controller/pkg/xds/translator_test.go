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

package xds

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func TestResolveUpstreamDefinition_Found(t *testing.T) {
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "test-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	def, err := resolveUpstreamDefinition("test-upstream", definitions)

	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, "test-upstream", def.Name)
}

func TestResolveUpstreamDefinition_NotFound(t *testing.T) {
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "existing-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	def, err := resolveUpstreamDefinition("0000-non-existent-0000-000000000000", definitions)

	assert.Error(t, err)
	assert.Nil(t, def)
	assert.Contains(t, err.Error(), "upstream definition '0000-non-existent-0000-000000000000' not found")
}

func TestResolveUpstreamDefinition_NoDefinitions(t *testing.T) {
	def, err := resolveUpstreamDefinition("test-upstream", nil)

	assert.Error(t, err)
	assert.Nil(t, def)
	assert.Contains(t, err.Error(), "no definitions provided")
}

func TestParseTimeout_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "seconds",
			input:    "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "minutes",
			input:    "2m",
			expected: 2 * time.Minute,
		},
		{
			name:     "milliseconds",
			input:    "500ms",
			expected: 500 * time.Millisecond,
		},
		{
			name:     "hours",
			input:    "1h",
			expected: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := parseTimeout(&tt.input)

			require.NoError(t, err)
			require.NotNil(t, duration)
			assert.Equal(t, tt.expected, *duration)
		})
	}
}

func TestParseTimeout_Invalid(t *testing.T) {
	invalid := "invalid"
	duration, err := parseTimeout(&invalid)

	assert.Error(t, err)
	assert.Nil(t, duration)
	assert.Contains(t, err.Error(), "invalid timeout format")
}

func TestParseTimeout_Nil(t *testing.T) {
	duration, err := parseTimeout(nil)

	assert.NoError(t, err)
	assert.Nil(t, duration)
}

func TestParseTimeout_Empty(t *testing.T) {
	empty := ""
	duration, err := parseTimeout(&empty)

	assert.NoError(t, err)
	assert.Nil(t, duration)
}

func TestResolveUpstreamCluster_WithDirectURL(t *testing.T) {
	translator := &Translator{}
	url := "http://backend:8080/api"
	upstream := &api.Upstream{
		Url: &url,
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("main", upstream, nil)

	require.NoError(t, err)
	assert.Equal(t, "cluster_http_backend_8080", clusterName)
	assert.NotNil(t, parsedURL)
	assert.Equal(t, "http", parsedURL.Scheme)
	assert.Equal(t, "backend:8080", parsedURL.Host)
	assert.Equal(t, "/api", parsedURL.Path)
	assert.Nil(t, timeout, "Direct URL should not have timeout override")
}

func TestResolveUpstreamCluster_WithRef_WithTimeout(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	timeoutStr := "45s"
	basePath := "/v2"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name:     "my-upstream",
			BasePath: &basePath,
			Timeout: &api.UpstreamTimeout{
				Connect: &timeoutStr,
			},
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend-1:9000",
				},
			},
		},
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	require.NoError(t, err)
	assert.Equal(t, "cluster_http_backend-1_9000", clusterName)
	assert.NotNil(t, parsedURL)
	assert.Equal(t, "http", parsedURL.Scheme)
	assert.Equal(t, "backend-1:9000", parsedURL.Host)
	assert.Equal(t, "/v2", parsedURL.Path)
	require.NotNil(t, timeout)
	require.NotNil(t, timeout.Connect)
	assert.Equal(t, 45*time.Second, *timeout.Connect)
}

func TestResolveUpstreamCluster_WithRef_NoTimeout(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	require.NoError(t, err)
	assert.Equal(t, "cluster_http_backend_8080", clusterName)
	assert.NotNil(t, parsedURL)
	assert.Nil(t, timeout, "No timeout in definition should result in nil timeout")
}

func TestResolveUpstreamCluster_WithRef_NotFound(t *testing.T) {
	translator := &Translator{}
	ref := "0000-non-existent-0000-000000000000"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "other-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve main upstream ref")
	assert.Contains(t, err.Error(), "upstream definition '0000-non-existent-0000-000000000000' not found")
}

func TestResolveUpstreamCluster_WithRef_InvalidTimeout(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	invalidTimeout := "invalid"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Timeout: &api.UpstreamTimeout{
				Connect: &invalidTimeout,
			},
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout in upstream definition")
}

func TestResolveUpstreamCluster_WithRef_NoURLs(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{},
		},
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no URLs configured")
}

func TestResolveUpstreamCluster_NoURLOrRef(t *testing.T) {
	translator := &Translator{}
	upstream := &api.Upstream{}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no main upstream configured")
}

func TestResolveUpstreamCluster_InvalidURL(t *testing.T) {
	translator := &Translator{}
	invalidURL := "not a valid url"
	upstream := &api.Upstream{
		Url: &invalidURL,
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid main upstream URL")
}

// testRouterConfig creates a minimal valid router config for testing
func testRouterConfig() *config.RouterConfig {
	return &config.RouterConfig{
		ListenerPort: 8080,
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "localhost"},
			Sandbox: config.VHostEntry{Default: "sandbox.localhost"},
		},
		Upstream: config.RouterUpstream{
			TLS: config.UpstreamTLS{
				MinimumProtocolVersion: constants.TLSVersion12,
				MaximumProtocolVersion: constants.TLSVersion13,
				DisableSslVerification: true,
			},
			Timeouts: config.UpstreamTimeouts{
				RouteTimeoutMs:     60000,
				RouteIdleTimeoutMs: 300000,
				ConnectTimeoutMs:   5000,
			},
		},
		PolicyEngine: config.PolicyEngineConfig{},
		AccessLogs: config.AccessLogsConfig{
			Enabled: false,
		},
		HTTPListener: config.HTTPListenerConfig{
			ServerHeaderTransformation: commonconstants.OVERWRITE,
		},
		LuaScriptPath: "../../lua/request_transformation.lua",
	}
}

// testConfig creates a minimal valid config for testing
func testConfig() *config.Config {
	return &config.Config{
		Controller: config.Controller{
			ControlPlane: config.ControlPlaneConfig{
				Host:             "localhost",
				ReconnectInitial: time.Second,
				ReconnectMax:     30 * time.Second,
				PollingInterval:  5 * time.Second,
			},
		},
		Router: *testRouterConfig(),
	}
}

func TestTranslator_CreateTLSProtocolVersion(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		version  string
		expected tlsv3.TlsParameters_TlsProtocol
	}{
		{name: "TLS 1.0", version: constants.TLSVersion10, expected: tlsv3.TlsParameters_TLSv1_0},
		{name: "TLS 1.1", version: constants.TLSVersion11, expected: tlsv3.TlsParameters_TLSv1_1},
		{name: "TLS 1.2", version: constants.TLSVersion12, expected: tlsv3.TlsParameters_TLSv1_2},
		{name: "TLS 1.3", version: constants.TLSVersion13, expected: tlsv3.TlsParameters_TLSv1_3},
		{name: "Unknown version", version: "TLSv2.0", expected: tlsv3.TlsParameters_TLS_AUTO},
		{name: "Empty version", version: "", expected: tlsv3.TlsParameters_TLS_AUTO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.createTLSProtocolVersion(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslator_ParseCipherSuites(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		ciphers  string
		expected []string
	}{
		{
			name:     "Single cipher",
			ciphers:  "ECDHE-RSA-AES256-GCM-SHA384",
			expected: []string{"ECDHE-RSA-AES256-GCM-SHA384"},
		},
		{
			name:     "Multiple ciphers",
			ciphers:  "ECDHE-RSA-AES256-GCM-SHA384,ECDHE-RSA-AES128-GCM-SHA256",
			expected: []string{"ECDHE-RSA-AES256-GCM-SHA384", "ECDHE-RSA-AES128-GCM-SHA256"},
		},
		{
			name:     "Ciphers with spaces",
			ciphers:  "CIPHER1 , CIPHER2 , CIPHER3",
			expected: []string{"CIPHER1", "CIPHER2", "CIPHER3"},
		},
		{
			name:     "Empty string",
			ciphers:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.parseCipherSuites(tt.ciphers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslator_PathToRegex(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Simple path",
			path:     "/api/users",
			expected: "^/api/users$",
		},
		{
			name:     "Path with parameter",
			path:     "/api/users/{id}",
			expected: "^/api/users/[^/]+$",
		},
		{
			name:     "Path with multiple parameters",
			path:     "/api/{resource}/{id}",
			expected: "^/api/[^/]+/[^/]+$",
		},
		{
			name:     "Path with dots (version)",
			path:     "/api/v1.0/users",
			expected: "^/api/v1\\.0/users$",
		},
		{
			name:     "Root path",
			path:     "/",
			expected: "^/$",
		},
		{
			name:     "Path with special chars",
			path:     "/api/data.json",
			expected: "^/api/data\\.json$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.pathToRegex(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslator_CreateRoute_PathSpecifier(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name          string
		context       string
		apiVersion    string
		path          string
		expectedRegex string
	}{
		{
			name:          "Wildcard /* uses boundary-aware regex",
			context:       "/weather/$version",
			apiVersion:    "v1.0",
			path:          "/*",
			expectedRegex: `^/weather/v1\.0(?:/.*)?$`,
		},
		{
			name:          "Wildcard /* on plain context uses boundary-aware regex",
			context:       "/api",
			apiVersion:    "v1",
			path:          "/*",
			expectedRegex: `^/api(?:/.*)?$`,
		},
		{
			name:          "Root path / matches with and without trailing slash",
			context:       "/weather/$version",
			apiVersion:    "v1.0",
			path:          "/",
			expectedRegex: `^/weather/v1\.0/?$`,
		},
		{
			name:          "Root path / on plain context",
			context:       "/api",
			apiVersion:    "v1",
			path:          "/",
			expectedRegex: `^/api/?$`,
		},
		{
			name:          "Exact path accepts optional trailing slash",
			context:       "/weather/$version",
			apiVersion:    "v1.0",
			path:          "/forecast",
			expectedRegex: `^/weather/v1\.0/forecast/?$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := translator.createRoute(
				"test-id", "TestAPI", tt.apiVersion, tt.context,
				"GET", tt.path, "test-cluster", "/",
				"localhost", "http/rest", "", "", nil, "", nil,
				false, nil,
			)
			require.NotNil(t, r)
			{
				regex, ok := r.Match.PathSpecifier.(*route.RouteMatch_SafeRegex)
				require.True(t, ok, "expected RouteMatch_SafeRegex specifier")
				assert.Equal(t, tt.expectedRegex, regex.SafeRegex.Regex)
			}
			// Method header matcher must always be present
			require.Len(t, r.Match.Headers, 1)
			assert.Equal(t, ":method", r.Match.Headers[0].Name)
		})
	}
}

func TestTranslator_WildcardRegexBoundary(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	type wildcardCase struct {
		context        string
		apiVersion     string
		path           string
		shouldMatch    []string
		shouldNotMatch []string
	}

	cases := []wildcardCase{
		{
			context:        "/weather/$version",
			apiVersion:     "v1.0",
			path:           "/*",
			shouldMatch:    []string{"/weather/v1.0", "/weather/v1.0/", "/weather/v1.0/forecast", "/weather/v1.0/a/b/c"},
			shouldNotMatch: []string{"/weather/v1.0beta", "/weather/v1.0extra"},
		},
		{
			context:        "/api",
			apiVersion:     "v1",
			path:           "/*",
			shouldMatch:    []string{"/api", "/api/", "/api/users", "/api/v2/items"},
			shouldNotMatch: []string{"/api2", "/apixyz"},
		},
	}

	for _, tc := range cases {
		r := translator.createRoute(
			"test-id", "TestAPI", tc.apiVersion, tc.context,
			"GET", tc.path, "test-cluster", "/",
			"localhost", "http/rest", "", "", nil, "", nil,
			false, nil,
		)
		require.NotNil(t, r)
		regexSpec, ok := r.Match.PathSpecifier.(*route.RouteMatch_SafeRegex)
		require.True(t, ok)
		re := regexp.MustCompile(regexSpec.SafeRegex.Regex)

		for _, p := range tc.shouldMatch {
			assert.True(t, re.MatchString(p), "regex %q should match %q", regexSpec.SafeRegex.Regex, p)
		}
		for _, p := range tc.shouldNotMatch {
			assert.False(t, re.MatchString(p), "regex %q should NOT match %q", regexSpec.SafeRegex.Regex, p)
		}
	}
}

// applyEnvoyRewrite emulates how Envoy applies a route's RegexRewrite to a request path:
// the request must first be matched by the route's path specifier, then the rewrite regex
// substitution is applied. Envoy uses "\1" substitution syntax; Go's regexp uses "$1".
func applyEnvoyRewrite(t *testing.T, r *route.Route, requestPath string) string {
	t.Helper()
	spec, ok := r.Match.PathSpecifier.(*route.RouteMatch_SafeRegex)
	require.True(t, ok, "expected SafeRegex path specifier")
	require.True(t, regexp.MustCompile(spec.SafeRegex.Regex).MatchString(requestPath),
		"match regex %q should match request %q", spec.SafeRegex.Regex, requestPath)

	rw := r.GetRoute().GetRegexRewrite()
	require.NotNil(t, rw, "route should have a RegexRewrite")
	pattern := regexp.MustCompile(rw.GetPattern().GetRegex())
	goSub := strings.ReplaceAll(rw.GetSubstitution(), `\1`, `${1}`)
	return pattern.ReplaceAllString(requestPath, goSub)
}

// TestTranslator_WildcardUpstreamRewrite verifies that a non-root wildcard operation path
// ("/foo/*") preserves the matched literal prefix ("/foo") on the upstream — consistent with
// exact paths — while the bare "/*" catch-all and base-path upstreams behave as before.
// Regression test for issue #2071 (PathPrefix-derived routes forwarded the wrong upstream path).
func TestTranslator_WildcardUpstreamRewrite(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name         string
		context      string
		path         string
		upstreamPath string
		request      string
		wantUpstream string
	}{
		// Non-root wildcard: the matched literal prefix "/forecast" must be preserved (the bug).
		{"wildcard subpath, root upstream, bare prefix", "/route/$version", "/forecast/*", "/", "/route/v1.0/forecast", "/forecast"},
		{"wildcard subpath, root upstream, with subpath", "/route/$version", "/forecast/*", "/", "/route/v1.0/forecast/today", "/forecast/today"},
		{"wildcard subpath, base-path upstream, bare prefix", "/route/$version", "/forecast/*", "/api/v2", "/route/v1.0/forecast", "/api/v2/forecast"},
		{"wildcard subpath, base-path upstream, with subpath", "/route/$version", "/forecast/*", "/api/v2", "/route/v1.0/forecast/today", "/api/v2/forecast/today"},
		// Bare /* catch-all: unchanged — the whole context is the stripped prefix.
		{"bare wildcard, root upstream, subpath", "/api/$version", "/*", "/", "/api/v1.0/users", "/users"},
		{"bare wildcard, root upstream, bare context", "/api/$version", "/*", "/", "/api/v1.0", "/"},
		{"bare wildcard, base-path upstream, subpath", "/api/$version", "/*", "/svc", "/api/v1.0/users", "/svc/users"},
		// Exact path: unchanged — operation path preserved on the upstream.
		{"exact path, root upstream", "/route/$version", "/weather", "/", "/route/v1.0/weather", "/weather"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := translator.createRoute(
				"test-id", "TestAPI", "v1.0", tt.context,
				"GET", tt.path, "test-cluster", tt.upstreamPath,
				"localhost", "http/rest", "", "", nil, "", nil,
				false, nil,
			)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantUpstream, applyEnvoyRewrite(t, r, tt.request))
		})
	}
}

// TestTranslator_MCPUpstreamRewrite verifies that for MCP proxies the gateway-facing "/mcp"
// resource is forwarded to EXACTLY the configured upstream URL path — the "/mcp" segment is
// not appended to the backend. The upstream is expected to be the full MCP endpoint URL, and
// some backends don't serve a "/mcp" sub-path. Regression test for the double-"/mcp" bug.
func TestTranslator_MCPUpstreamRewrite(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	mcpKind := string(models.KindMcp)
	mcpPath := constants.MCP_RESOURCE_PATH

	tests := []struct {
		name         string
		apiKind      string
		context      string
		path         string
		upstreamPath string
		request      string
		wantUpstream string
	}{
		// Upstream already points at the backend's "/mcp" endpoint: forward there as-is,
		// do NOT produce "/mcp/mcp".
		{"mcp endpoint upstream", mcpKind, "/mcpauth", mcpPath, "/mcp", "/mcpauth/mcp", "/mcp"},
		// Upstream has no path (e.g. http://backend:3001): forward to root, not "/mcp".
		{"root upstream", mcpKind, "/mcpauth", mcpPath, "", "/mcpauth/mcp", "/"},
		// Upstream serves MCP at a custom path: forward to exactly that path.
		{"custom path upstream", mcpKind, "/mcpauth", mcpPath, "/api/v1/mcp-server", "/mcpauth/mcp", "/api/v1/mcp-server"},
		// Trailing slash on the gateway-facing request is accepted and rewrites the same way.
		{"trailing slash request", mcpKind, "/mcpauth", mcpPath, "/mcp", "/mcpauth/mcp/", "/mcp"},
		// Non-MCP kind with a "/mcp" operation path keeps the standard behavior (path preserved
		// on the upstream) — the special-casing is scoped to MCP proxies only.
		{"non-mcp kind unaffected", "http/rest", "/mcpauth", mcpPath, "/base", "/mcpauth/mcp", "/base/mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := translator.createRoute(
				"test-id", "TestMCP", "v1.0", tt.context,
				"POST", tt.path, "test-cluster", tt.upstreamPath,
				"localhost", tt.apiKind, "", "", nil, "", nil,
				false, nil,
			)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantUpstream, applyEnvoyRewrite(t, r, tt.request))
		})
	}
}

// TestTranslator_WildcardUpstreamRewriteFromRDC verifies the same prefix-preserving behavior on
// the RuntimeDeployConfig path (createRouteFromRDC), which the policy/runtime xDS pipeline uses.
func TestTranslator_WildcardUpstreamRewriteFromRDC(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name          string
		fullPath      string
		operationPath string
		basePath      string
		request       string
		wantUpstream  string
	}{
		{"wildcard subpath, root upstream, bare prefix", "/route/v1.0/forecast/*", "/forecast/*", "", "/route/v1.0/forecast", "/forecast"},
		{"wildcard subpath, root upstream, with subpath", "/route/v1.0/forecast/*", "/forecast/*", "", "/route/v1.0/forecast/today", "/forecast/today"},
		{"wildcard subpath, base-path upstream", "/route/v1.0/forecast/*", "/forecast/*", "/api/v2", "/route/v1.0/forecast/today", "/api/v2/forecast/today"},
		{"bare wildcard, root upstream, subpath", "/api/v1.0/*", "/*", "", "/api/v1.0/users", "/users"},
		{"bare wildcard, root upstream, bare context", "/api/v1.0/*", "/*", "", "/api/v1.0", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc := &models.RuntimeDeployConfig{
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"main": {BasePath: tt.basePath, Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
				},
			}
			rdcRoute := &models.Route{
				Method:          "GET",
				Path:            tt.fullPath,
				OperationPath:   tt.operationPath,
				AutoHostRewrite: true,
				Upstream:        models.RouteUpstream{ClusterKey: "main"},
			}
			r := translator.createRouteFromRDC("GET|"+tt.fullPath+"|", rdcRoute, rdc)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantUpstream, applyEnvoyRewrite(t, r, tt.request))
		})
	}
}

// TestTranslator_RouteResilienceTimeoutsFromRDC verifies that per-route resilience
// timeouts on a models.Route flow into the Envoy RouteAction, with fallback to the
// global defaults (60s / 300s from testRouterConfig) when unset, and that an explicit
// 0s is preserved (disables the timeout).
func TestTranslator_RouteResilienceTimeoutsFromRDC(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	dur := func(d time.Duration) *time.Duration { return &d }

	tests := []struct {
		name        string
		timeout     *models.RouteTimeout
		wantTimeout time.Duration
		wantIdle    time.Duration
	}{
		{name: "nil timeout uses global defaults", timeout: nil, wantTimeout: 60 * time.Second, wantIdle: 300 * time.Second},
		{name: "configured values applied", timeout: &models.RouteTimeout{Timeout: dur(2 * time.Second), IdleTimeout: dur(10 * time.Second)}, wantTimeout: 2 * time.Second, wantIdle: 10 * time.Second},
		{name: "timeout set, idle falls back", timeout: &models.RouteTimeout{Timeout: dur(3 * time.Second)}, wantTimeout: 3 * time.Second, wantIdle: 300 * time.Second},
		{name: "explicit 0s disables route timeout", timeout: &models.RouteTimeout{Timeout: dur(0)}, wantTimeout: 0, wantIdle: 300 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc := &models.RuntimeDeployConfig{
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"main": {Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
				},
			}
			rdcRoute := &models.Route{
				Method:          "GET",
				Path:            "/api/v1.0/items",
				OperationPath:   "/items",
				AutoHostRewrite: true,
				Timeout:         tt.timeout,
				Upstream:        models.RouteUpstream{ClusterKey: "main"},
			}
			r := translator.createRouteFromRDC("GET|/api/v1.0/items|", rdcRoute, rdc)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantTimeout, r.GetRoute().GetTimeout().AsDuration(), "route timeout")
			assert.Equal(t, tt.wantIdle, r.GetRoute().GetIdleTimeout().AsDuration(), "route idle timeout")
		})
	}
}

// TestTranslator_MCPUpstreamRewriteFromRDC verifies the MCP "/mcp"-not-appended behavior on the
// RuntimeDeployConfig path (createRouteFromRDC), which the policy/runtime xDS pipeline uses.
func TestTranslator_MCPUpstreamRewriteFromRDC(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	mcpPath := constants.MCP_RESOURCE_PATH

	tests := []struct {
		name         string
		kind         string
		fullPath     string
		basePath     string
		request      string
		wantUpstream string
	}{
		{"mcp endpoint upstream", string(models.KindMcp), "/mcpauth" + mcpPath, "/mcp", "/mcpauth/mcp", "/mcp"},
		{"root upstream", string(models.KindMcp), "/mcpauth" + mcpPath, "", "/mcpauth/mcp", "/"},
		{"custom path upstream", string(models.KindMcp), "/mcpauth" + mcpPath, "/api/v1/mcp-server", "/mcpauth/mcp", "/api/v1/mcp-server"},
		{"trailing slash request", string(models.KindMcp), "/mcpauth" + mcpPath, "/mcp", "/mcpauth/mcp/", "/mcp"},
		// Non-MCP kind keeps the standard behavior (operation path preserved on the upstream).
		{"non-mcp kind unaffected", string(models.KindRestApi), "/mcpauth" + mcpPath, "/base", "/mcpauth/mcp", "/base/mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc := &models.RuntimeDeployConfig{
				Metadata: models.Metadata{Kind: tt.kind},
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"main": {BasePath: tt.basePath, Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
				},
			}
			rdcRoute := &models.Route{
				Method:          "POST",
				Path:            tt.fullPath,
				OperationPath:   mcpPath,
				AutoHostRewrite: true,
				Upstream:        models.RouteUpstream{ClusterKey: "main"},
			}
			r := translator.createRouteFromRDC("POST|"+tt.fullPath+"|", rdcRoute, rdc)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantUpstream, applyEnvoyRewrite(t, r, tt.request))
		})
	}
}

// TestTranslator_MCPAppendResourcePathToBackend verifies that when
// mcp.append_resource_path_to_backend is enabled, MCP "/mcp" routes fall back to the
// legacy behaviour of appending "/mcp" to the backend upstream path. This preserves
// compatibility for MCP API definitions authored against the previous gateway version.
func TestTranslator_MCPAppendResourcePathToBackend(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.MCP.AppendResourcePathToBackend = true
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	mcpKind := string(models.KindMcp)
	mcpPath := constants.MCP_RESOURCE_PATH

	tests := []struct {
		name         string
		context      string
		upstreamPath string
		request      string
		wantUpstream string
	}{
		// Legacy behaviour: "/mcp" IS appended to the configured upstream path.
		{"root upstream", "/mcpauth", "", "/mcpauth/mcp", "/mcp"},
		{"base-path upstream", "/mcpauth", "/api/v2", "/mcpauth/mcp", "/api/v2/mcp"},
		{"trailing slash request", "/mcpauth", "/api/v2", "/mcpauth/mcp/", "/api/v2/mcp/"},
	}

	for _, tt := range tests {
		t.Run("createRoute/"+tt.name, func(t *testing.T) {
			r := translator.createRoute(
				"test-id", "TestMCP", "v1.0", tt.context,
				"POST", mcpPath, "test-cluster", tt.upstreamPath,
				"localhost", mcpKind, "", "", nil, "", nil,
				false, nil,
			)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantUpstream, applyEnvoyRewrite(t, r, tt.request))
		})

		t.Run("createRouteFromRDC/"+tt.name, func(t *testing.T) {
			rdc := &models.RuntimeDeployConfig{
				Metadata: models.Metadata{Kind: mcpKind},
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"main": {BasePath: tt.upstreamPath, Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
				},
			}
			rdcRoute := &models.Route{
				Method:          "POST",
				Path:            tt.context + mcpPath,
				OperationPath:   mcpPath,
				AutoHostRewrite: true,
				Upstream:        models.RouteUpstream{ClusterKey: "main"},
			}
			r := translator.createRouteFromRDC("POST|"+tt.context+mcpPath+"|", rdcRoute, rdc)
			require.NotNil(t, r)
			assert.Equal(t, tt.wantUpstream, applyEnvoyRewrite(t, r, tt.request))
		})
	}
}

// TestTranslator_ExactPathUsesNativeMatcher guards the fix for HTTPRoutePathMatchOrder:
// an Exact path match must be emitted as Envoy's native exact matcher (RouteMatch_Path),
// NOT as a safe_regex. Rendering it as a regex made SortRoutesByPriority treat every route
// as a Regex, so it fell back to regex-string length and let a longer prefix regex
// (^/match(?:/.*)?$) outrank a shorter exact (^/match/exact$).
func TestTranslator_ExactPathUsesNativeMatcher(t *testing.T) {
	logger := createTestLogger()
	translator := NewTranslator(logger, testRouterConfig(), nil, testConfig())

	rdc := &models.RuntimeDeployConfig{
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"main": {BasePath: "", Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
		},
	}
	rdcRoute := &models.Route{
		Method:        "GET",
		Path:          "/match/exact",
		OperationPath: "/match/exact",
		PathMatchType: "Exact",
		Upstream:      models.RouteUpstream{ClusterKey: "main"},
	}
	r := translator.createRouteFromRDC("GET|/match/exact|", rdcRoute, rdc)
	require.NotNil(t, r)
	pathSpec, ok := r.GetMatch().GetPathSpecifier().(*route.RouteMatch_Path)
	require.True(t, ok, "exact path should use RouteMatch_Path, got %T", r.GetMatch().GetPathSpecifier())
	assert.Equal(t, "/match/exact", pathSpec.Path)
	assert.Equal(t, pathMatchTypeExact, getPathMatchType(r.GetMatch()),
		"exact route must rank as Exact for SortRoutesByPriority")
}

// TestSortRoutesByPriority_ExactBeatsLongerPrefixRegex reproduces the HTTPRoutePathMatchOrder
// conformance shape: an exact /match must outrank the /match/ prefix even though the prefix's
// regex string is longer. Before the fix the exact route was a safe_regex and lost on length.
func TestSortRoutesByPriority_ExactBeatsLongerPrefixRegex(t *testing.T) {
	exactMatch := &route.Route{
		Name:  "exact-match",
		Match: &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: "/match"}},
	}
	exactMatchExact := &route.Route{
		Name:  "exact-match-exact",
		Match: &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: "/match/exact"}},
	}
	prefixMatch := &route.Route{
		Name: "prefix-match",
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_SafeRegex{
				SafeRegex: &matcher.RegexMatcher{Regex: "^/match(?:/.*)?$"},
			},
		},
	}

	sorted := SortRoutesByPriority([]*route.Route{prefixMatch, exactMatch, exactMatchExact})

	// Both exacts must precede the prefix regex.
	assert.Equal(t, "exact-match-exact", sorted[0].Name)
	assert.Equal(t, "exact-match", sorted[1].Name)
	assert.Equal(t, "prefix-match", sorted[2].Name)
}

func TestTranslator_SanitizeClusterName(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		hostname string
		scheme   string
		expected string
	}{
		{
			name:     "Simple hostname HTTP",
			hostname: "localhost",
			scheme:   "http",
			expected: "cluster_http_localhost",
		},
		{
			name:     "Dotted hostname HTTPS",
			hostname: "api.example.com",
			scheme:   "https",
			expected: "cluster_https_api_example_com",
		},
		{
			name:     "Hostname with port",
			hostname: "localhost:8080",
			scheme:   "http",
			expected: "cluster_http_localhost_8080",
		},
		{
			name:     "Complex hostname",
			hostname: "api.v1.prod.example.com:443",
			scheme:   "https",
			expected: "cluster_https_api_v1_prod_example_com_443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.sanitizeClusterName(tt.hostname, tt.scheme)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValueFromSourceConfig(t *testing.T) {
	tests := []struct {
		name         string
		sourceConfig any
		key          string
		expected     any
		expectError  bool
	}{
		{
			name: "Simple key",
			sourceConfig: map[string]interface{}{
				"0000-key1-0000-000000000000": "value1",
			},
			key:         "0000-key1-0000-000000000000",
			expected:    "value1",
			expectError: false,
		},
		{
			name: "Nested key",
			sourceConfig: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "nested_value",
				},
			},
			key:         "outer.inner",
			expected:    "nested_value",
			expectError: false,
		},
		{
			name: "Deeply nested key",
			sourceConfig: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "deep_value",
					},
				},
			},
			key:         "a.b.c",
			expected:    "deep_value",
			expectError: false,
		},
		{
			name:         "Nil sourceConfig",
			sourceConfig: nil,
			key:          "key",
			expected:     nil,
			expectError:  true,
		},
		{
			name: "Key not found",
			sourceConfig: map[string]interface{}{
				"0000-key1-0000-000000000000": "value1",
			},
			key:         "nonexistent",
			expected:    nil,
			expectError: true,
		},
		{
			name: "Invalid nested path",
			sourceConfig: map[string]interface{}{
				"0000-key1-0000-000000000000": "value1",
			},
			key:         "key1.nested",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getValueFromSourceConfig(tt.sourceConfig, tt.key)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]interface{}
	}{
		{
			name:     "Empty map",
			input:    map[string]string{},
			expected: map[string]interface{}{},
		},
		{
			name: "Single entry",
			input: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "Multiple entries",
			input: map[string]string{
				"status":     "%RESPONSE_CODE%",
				"duration":   "%DURATION%",
				"user_agent": "%REQ(User-Agent)%",
			},
			expected: map[string]interface{}{
				"status":     "%RESPONSE_CODE%",
				"duration":   "%DURATION%",
				"user_agent": "%REQ(User-Agent)%",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToInterface(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewTranslator_WithoutCerts(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()

	translator := NewTranslator(logger, routerCfg, nil, cfg)
	assert.NotNil(t, translator)
	assert.Nil(t, translator.GetCertStore())
}

func TestTranslator_ExtractTemplateHandle_NilSourceConfig(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: nil,
		Origin:              models.OriginGatewayAPI,
	}

	result := translator.extractTemplateHandle(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_ExtractProviderName_NilSourceConfig(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: nil,
		Origin:              models.OriginGatewayAPI,
	}

	result := translator.extractProviderName(storedCfg, nil)
	assert.Equal(t, "", result)
}

// extractHCM pulls the HttpConnectionManager out of the listener's first filter chain.
func extractHCM(t *testing.T, lis *listener.Listener) *hcm.HttpConnectionManager {
	t.Helper()
	require.NotEmpty(t, lis.GetFilterChains())
	require.NotEmpty(t, lis.GetFilterChains()[0].GetFilters())
	typedConfig := lis.GetFilterChains()[0].GetFilters()[0].GetTypedConfig()
	require.NotNil(t, typedConfig)
	manager := &hcm.HttpConnectionManager{}
	require.NoError(t, typedConfig.UnmarshalTo(manager))
	return manager
}

func TestTranslator_CreateListener_HCMTimeouts(t *testing.T) {
	tests := []struct {
		name     string
		timeouts config.HCMTimeouts
	}{
		{
			name:     "configured values",
			timeouts: config.HCMTimeouts{RequestTimeout: 30 * time.Second, RequestHeadersTimeout: 10 * time.Second, StreamIdleTimeout: 2 * time.Minute, IdleTimeout: 30 * time.Minute},
		},
		{
			name:     "envoy defaults flow through unchanged",
			timeouts: config.HCMTimeouts{RequestTimeout: 0, RequestHeadersTimeout: 0, StreamIdleTimeout: 5 * time.Minute, IdleTimeout: time.Hour},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			routerCfg := testRouterConfig()
			routerCfg.HTTPListener.Timeouts = tt.timeouts
			cfg := testConfig()
			cfg.Router = *routerCfg
			translator := NewTranslator(logger, routerCfg, nil, cfg)

			lis, _, err := translator.createListener(nil, false)
			require.NoError(t, err)

			manager := extractHCM(t, lis)
			assert.Equal(t, tt.timeouts.RequestTimeout, manager.GetRequestTimeout().AsDuration(), "request_timeout")
			assert.Equal(t, tt.timeouts.RequestHeadersTimeout, manager.GetRequestHeadersTimeout().AsDuration(), "request_headers_timeout")
			assert.Equal(t, tt.timeouts.StreamIdleTimeout, manager.GetStreamIdleTimeout().AsDuration(), "stream_idle_timeout")
			require.NotNil(t, manager.GetCommonHttpProtocolOptions(), "common_http_protocol_options must be set")
			assert.Equal(t, tt.timeouts.IdleTimeout, manager.GetCommonHttpProtocolOptions().GetIdleTimeout().AsDuration(), "idle_timeout")
		})
	}
}

func TestTranslator_CreateAccessLogConfig_Disabled(t *testing.T) {
	// Note: createAccessLogConfig should only be called when access logs are enabled.
	// The check for enabled is done at the caller level. When called directly with disabled
	// access logs (format defaults to empty, which falls through to text format check),
	// it should return an error about missing text_format.
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs.Enabled = false
	// When format is empty, it falls through to text format check
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	// Without format configured, it returns error (this is expected behavior)
	assert.Error(t, err)
	assert.Nil(t, logs)
}

func TestTranslator_CreateAccessLogConfig_JSON(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled: true,
		Format:  "json",
		JSONFields: map[string]string{
			"status":   "%RESPONSE_CODE%",
			"duration": "%DURATION%",
		},
	}
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, logs)
}

func TestTranslator_CreateAccessLogConfig_Text(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled:    true,
		Format:     "text",
		TextFormat: "[%START_TIME%] %REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL% %RESPONSE_CODE%",
	}
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, logs)
}

func TestTranslator_CreateAccessLogConfig_JSONMissingFields(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled:    true,
		Format:     "json",
		JSONFields: nil,
	}
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	assert.Error(t, err)
	assert.Nil(t, logs)
	assert.Contains(t, err.Error(), "json_fields not configured")
}

func TestTranslator_CreatePolicyEngineCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.PolicyEngine = config.PolicyEngineConfig{
		Host:      "localhost",
		Port:      50051,
		TimeoutMs: 1000,
		TLS: config.PolicyEngineTLS{
			Enabled: false,
		},
	}
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createPolicyEngineCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, constants.PolicyEngineClusterName, cluster.Name)
}

func TestTranslator_CreatePolicyEngineCluster_UDS(t *testing.T) {
	logger := createTestLogger()

	t.Run("UDS mode (default)", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.PolicyEngine = config.PolicyEngineConfig{
			Mode:             "uds",
			TimeoutMs:        1000,
			MessageTimeoutMs: 500,
		}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createPolicyEngineCluster()
		assert.NotNil(t, c)
		assert.Equal(t, constants.PolicyEngineClusterName, c.Name)

		// Verify cluster type is STATIC for UDS
		assert.Equal(t, cluster.Cluster_STATIC, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a Pipe (UDS) with constant path
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		pipe := addr.GetPipe()
		assert.NotNil(t, pipe, "Expected Pipe address for UDS mode")
		assert.Equal(t, constants.DefaultPolicyEngineSocketPath, pipe.Path)
	})

	t.Run("TCP mode with host:port", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.PolicyEngine = config.PolicyEngineConfig{
			Mode:             "tcp",
			Host:             "policy-engine",
			Port:             9001,
			TimeoutMs:        1000,
			MessageTimeoutMs: 500,
		}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createPolicyEngineCluster()
		assert.NotNil(t, c)
		assert.Equal(t, constants.PolicyEngineClusterName, c.Name)

		// Verify cluster type is STRICT_DNS for TCP
		assert.Equal(t, cluster.Cluster_STRICT_DNS, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a SocketAddress (TCP)
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		socketAddr := addr.GetSocketAddress()
		assert.NotNil(t, socketAddr, "Expected SocketAddress for TCP mode")
		assert.Equal(t, "policy-engine", socketAddr.Address)
		assert.Equal(t, uint32(9001), socketAddr.GetPortValue())
	})
}

func TestTranslator_CreateExtProcFilter(t *testing.T) {
	logger := createTestLogger()

	t.Run("Creates ext_proc filter with DEFAULT route cache action", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.PolicyEngine = config.PolicyEngineConfig{
			Host:             "localhost",
			Port:             50051,
			TimeoutMs:        1000,
			MessageTimeoutMs: 500,
		}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		filter, err := translator.createExtProcFilter()
		assert.NoError(t, err)
		assert.NotNil(t, filter)
		assert.Equal(t, constants.ExtProcFilterName, filter.Name)
	})
}

func TestTranslator_CreateRouteConfiguration(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	// Test with nil virtual hosts
	routeConfig := translator.createRouteConfiguration(nil)
	assert.NotNil(t, routeConfig)
	assert.Equal(t, SharedRouteConfigName, routeConfig.Name)
}

func TestTranslator_TranslateConfigs_EmptyConfigs(t *testing.T) {
	logger := createTestLogger()

	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	// Test with empty configs
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test-correlation-id")
	require.NoError(t, err)
	assert.NotNil(t, resources)
}

// Every API virtual host must strip any client-supplied x-envoy-original-path so it
// cannot survive to the collector.ignore_path_prefixes access-log filter on a route
// that never rewrites :path (see the comment on this field in TranslateConfigs).
// vhostMap is pre-seeded with the wildcard "*" vhost, so this is exercised even with
// no APIs deployed.
func TestTranslator_TranslateConfigs_StripsClientOriginalPathHeader(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test-correlation-id")
	require.NoError(t, err)

	routeConfigs := resources[resource.RouteType]
	require.NotEmpty(t, routeConfigs)

	found := false
	for _, res := range routeConfigs {
		rc, ok := res.(*route.RouteConfiguration)
		require.True(t, ok)
		for _, vh := range rc.VirtualHosts {
			found = true
			assert.Contains(t, vh.RequestHeadersToRemove, envoyOriginalPathHeader,
				"virtual host %q must strip client-supplied x-envoy-original-path", vh.Name)
		}
	}
	assert.True(t, found, "expected at least one virtual host in the shared route config")
}

func TestTranslator_GetVHostDomains(t *testing.T) {
	logger := createTestLogger()

	t.Run("fallback domains when explicit domain lists are empty", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("api.example.com")
		assert.Equal(t, []string{"api.example.com", "api.example.com:*"}, domains)
	})

	t.Run("expands configured main domains when vhost equals main default", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "*.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"*.wso2.com", "*.foo.com"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("*.wso2.com")
		assert.Equal(t, []string{"*.wso2.com", "*.wso2.com:*", "*.foo.com", "*.foo.com:*"}, domains)
	})

	t.Run("expands configured sandbox domains when vhost equals sandbox default", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Sandbox.Default = "*-sandbox.wso2.com"
		routerCfg.VHosts.Sandbox.Domains = []string{"*-sandbox.wso2.com", "*-sandbox.foo.com"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("*-sandbox.wso2.com")
		assert.Equal(t, []string{"*-sandbox.wso2.com", "*-sandbox.wso2.com:*", "*-sandbox.foo.com", "*-sandbox.foo.com:*"}, domains)
	})

	t.Run("api-level vhost override uses fallback pair only", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "*.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"*.wso2.com", "*.foo.com"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("custom.wso2.com")
		assert.Equal(t, []string{"custom.wso2.com", "custom.wso2.com:*"}, domains)
	})

	t.Run("port-qualified domain is not expanded with :*", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("api.example.com:8443")
		assert.Equal(t, []string{"api.example.com:8443"}, domains)
	})

	t.Run("whitespace-only domain list falls back to effective vhost", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "*.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"   ", "  "}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("*.wso2.com")
		assert.Equal(t, []string{"*.wso2.com", "*.wso2.com:*"}, domains)
	})

	t.Run("port-qualified domain in configured list is not expanded with :*", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "api.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"api.wso2.com", "api.wso2.com:8443"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		domains := translator.getVHostDomains("api.wso2.com")
		assert.Equal(t, []string{"api.wso2.com", "api.wso2.com:*", "api.wso2.com:8443"}, domains)
	})
}

func TestTranslator_GetCertStore_Nil(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	assert.Nil(t, translator.GetCertStore())
}

func TestTranslator_ExtractTemplateHandle_InvalidKind(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: map[string]interface{}{
			"kind": 123, // Invalid type
		},
		Origin: models.OriginGatewayAPI,
	}

	result := translator.extractTemplateHandle(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_ExtractProviderName_InvalidKind(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: map[string]interface{}{
			"kind": 123, // Invalid type
		},
		Origin: models.OriginGatewayAPI,
	}

	result := translator.extractProviderName(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_CreateTracingConfig_Disabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = false
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tracingCfg, err := translator.createTracingConfig()
	assert.NoError(t, err)
	assert.Nil(t, tracingCfg)
}

func TestTranslator_CreateTracingConfig_Enabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	cfg.TracingConfig.SamplingRate = 0.5
	cfg.Router.TracingServiceName = "test-service"
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tracingCfg, err := translator.createTracingConfig()
	assert.NoError(t, err)
	assert.NotNil(t, tracingCfg)
}

func TestTranslator_CreateOTELCollectorCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createOTELCollectorCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, OTELCollectorClusterName, cluster.Name)
}

func TestTranslator_CreateOTELCollectorCluster_Disabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = false
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createOTELCollectorCluster()
	assert.Nil(t, cluster)
}

func TestTranslator_CreateALSCluster(t *testing.T) {
	logger := createTestLogger()

	t.Run("UDS mode (default)", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		cfg.Analytics.Enabled = true
		cfg.Collector.Server = config.GRPCEventServerConfig{
			Mode:                "uds",
			BufferFlushInterval: 1000000000,
			BufferSizeBytes:     16384,
			GRPCRequestTimeout:  20000000000,
		}
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createALSCluster()
		assert.NotNil(t, c)
		assert.Equal(t, constants.GRPCAccessLogClusterName, c.Name)

		// Verify cluster type is STATIC for UDS
		assert.Equal(t, cluster.Cluster_STATIC, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a Pipe (UDS) with constant path
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		pipe := addr.GetPipe()
		assert.NotNil(t, pipe, "Expected Pipe address for UDS mode")
		assert.Equal(t, constants.DefaultALSSocketPath, pipe.Path)
	})

	t.Run("UDS mode (empty string defaults to UDS)", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		cfg.Analytics.Enabled = true
		cfg.Collector.Server = config.GRPCEventServerConfig{
			Mode:                "",
			BufferFlushInterval: 1000000000,
			BufferSizeBytes:     16384,
			GRPCRequestTimeout:  20000000000,
		}
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createALSCluster()
		assert.NotNil(t, c)

		// Verify cluster type is STATIC for UDS
		assert.Equal(t, cluster.Cluster_STATIC, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a Pipe (UDS)
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		pipe := addr.GetPipe()
		assert.NotNil(t, pipe, "Expected Pipe address for default (empty) mode")
		assert.Equal(t, constants.DefaultALSSocketPath, pipe.Path)
	})

	t.Run("TCP mode with host:port", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		cfg.Analytics.Enabled = true
		cfg.Collector.Server = config.GRPCEventServerConfig{
			Mode:                "tcp",
			BufferFlushInterval: 1000000000,
			BufferSizeBytes:     16384,
			GRPCRequestTimeout:  20000000000,
		}
		// Set policy engine host - ALS uses the same host in TCP mode
		cfg.Router.PolicyEngine.Host = "policy-engine"
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createALSCluster()
		assert.NotNil(t, c)
		assert.Equal(t, constants.GRPCAccessLogClusterName, c.Name)

		// Verify cluster type is STRICT_DNS for TCP
		assert.Equal(t, cluster.Cluster_STRICT_DNS, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a SocketAddress (TCP)
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		socketAddr := addr.GetSocketAddress()
		assert.NotNil(t, socketAddr, "Expected SocketAddress for TCP mode")
		assert.Equal(t, "policy-engine", socketAddr.Address)
		assert.Equal(t, uint32(18090), socketAddr.GetPortValue())
	})

	t.Run("TCP mode honors deprecated port override (backward compat)", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		cfg.Analytics.Enabled = true
		cfg.Collector.Server = config.GRPCEventServerConfig{
			Mode:                "tcp",
			Port:                9099,
			BufferFlushInterval: 1000000000,
			BufferSizeBytes:     16384,
			GRPCRequestTimeout:  20000000000,
		}
		cfg.Router.PolicyEngine.Host = "policy-engine"
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createALSCluster()
		assert.NotNil(t, c)

		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		socketAddr := lbEndpoint.GetEndpoint().Address.GetSocketAddress()
		assert.NotNil(t, socketAddr)
		assert.Equal(t, uint32(9099), socketAddr.GetPortValue())
	})
}

func TestTranslator_CreateGRPCAccessLog(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Collector.Server = config.GRPCEventServerConfig{
		Mode:                "tcp",
		BufferFlushInterval: 1000,
		BufferSizeBytes:     16384,
		GRPCRequestTimeout:  5000,
	}
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	accessLog, err := translator.createGRPCAccessLog()
	assert.NoError(t, err)
	assert.NotNil(t, accessLog)
	assert.Nil(t, accessLog.Filter, "no ignore_path_prefixes configured -> no filter")
}

func TestTranslator_CreateGRPCAccessLog_WithIgnorePathPrefixes(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Collector.Server = config.GRPCEventServerConfig{
		Mode:                "tcp",
		BufferFlushInterval: 1000,
		BufferSizeBytes:     16384,
		GRPCRequestTimeout:  5000,
	}
	cfg.Collector.IgnorePathPrefixes = []string{"/health"}
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	accessLog, err := translator.createGRPCAccessLog()
	assert.NoError(t, err)
	assert.NotNil(t, accessLog)
	assert.NotNil(t, accessLog.Filter, "ignore_path_prefixes configured -> filter attached")
}

func TestTranslator_CreateGRPCAccessLog_BufferSizeOverflow(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Collector.Server = config.GRPCEventServerConfig{
		Mode:                "tcp",
		BufferFlushInterval: 1000,
		BufferSizeBytes:     math.MaxInt,
		GRPCRequestTimeout:  5000,
	}
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	accessLog, err := translator.createGRPCAccessLog()
	assert.Error(t, err)
	assert.Nil(t, accessLog)
	assert.Contains(t, err.Error(), "buffer_size_bytes")
}

// evalAccessLogFilter walks a constructed AccessLogFilter tree and evaluates it
// against a synthetic header set, mirroring how Envoy itself would evaluate the
// filter. This proves actual matching behavior, not just proto shape.
func evalAccessLogFilter(t *testing.T, filter *accesslog.AccessLogFilter, headers map[string]string) bool {
	t.Helper()
	switch fs := filter.FilterSpecifier.(type) {
	case *accesslog.AccessLogFilter_HeaderFilter:
		return evalHeaderMatcher(t, fs.HeaderFilter.Header, headers)
	case *accesslog.AccessLogFilter_AndFilter:
		for _, f := range fs.AndFilter.Filters {
			if !evalAccessLogFilter(t, f, headers) {
				return false
			}
		}
		return true
	case *accesslog.AccessLogFilter_OrFilter:
		for _, f := range fs.OrFilter.Filters {
			if evalAccessLogFilter(t, f, headers) {
				return true
			}
		}
		return false
	default:
		t.Fatalf("evalAccessLogFilter: unsupported filter specifier %T", fs)
		return false
	}
}

func evalHeaderMatcher(t *testing.T, m *route.HeaderMatcher, headers map[string]string) bool {
	t.Helper()
	val, present := headers[m.Name]
	var result bool
	switch spec := m.HeaderMatchSpecifier.(type) {
	case *route.HeaderMatcher_PresentMatch:
		result = present == spec.PresentMatch
	case *route.HeaderMatcher_PrefixMatch:
		result = present && strings.HasPrefix(val, spec.PrefixMatch)
	default:
		t.Fatalf("evalHeaderMatcher: unsupported header match specifier %T", spec)
	}
	if m.InvertMatch {
		result = !result
	}
	return result
}

func TestBuildIgnorePathsAccessLogFilter(t *testing.T) {
	t.Run("nil prefixes -> nil filter", func(t *testing.T) {
		assert.Nil(t, buildIgnorePathsAccessLogFilter(nil))
	})

	t.Run("empty prefixes -> nil filter", func(t *testing.T) {
		assert.Nil(t, buildIgnorePathsAccessLogFilter([]string{}))
	})

	t.Run("whitespace-only entries -> nil filter", func(t *testing.T) {
		assert.Nil(t, buildIgnorePathsAccessLogFilter([]string{"", "   "}))
	})

	t.Run("single prefix -> unwrapped per-prefix filter", func(t *testing.T) {
		filter := buildIgnorePathsAccessLogFilter([]string{"/health"})
		require.NotNil(t, filter)
		_, isAnd := filter.FilterSpecifier.(*accesslog.AccessLogFilter_AndFilter)
		assert.False(t, isAnd, "single prefix should not be wrapped in an outer AndFilter")

		assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/health/live",
		}), "matching original path -> suppressed")
		assert.True(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/orders",
		}), "non-matching original path -> logged")
		assert.True(t, evalAccessLogFilter(t, filter, map[string]string{
			":path": "/health/live",
		}), "no original-path header -> logged regardless of :path")
	})

	t.Run("multiple prefixes -> outer AndFilter", func(t *testing.T) {
		filter := buildIgnorePathsAccessLogFilter([]string{"/health", "/metrics", ""})
		require.NotNil(t, filter)
		andFilter, isAnd := filter.FilterSpecifier.(*accesslog.AccessLogFilter_AndFilter)
		require.True(t, isAnd, "multiple prefixes should be wrapped in an outer AndFilter")
		assert.Len(t, andFilter.AndFilter.Filters, 2, "blank entry must be dropped")

		assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/health/live",
		}), "matches first prefix -> suppressed")
		assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/metrics/scrape",
		}), "matches second prefix -> suppressed")
		assert.True(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/orders",
		}), "matches neither prefix -> logged")
	})
}

func TestNotEffectivelyMatchesPrefix(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantLog bool
	}{
		{
			name:    "original present and has prefix -> suppress even if :path (rewritten backend path) differs",
			headers: map[string]string{envoyOriginalPathHeader: "/health/live", ":path": "/some/rewritten/backend/path"},
			wantLog: false,
		},
		{
			name:    "original present and does not have prefix -> log, original is authoritative",
			headers: map[string]string{envoyOriginalPathHeader: "/orders", ":path": "/health"},
			wantLog: true,
		},
		{
			name:    "original absent, :path happens to have prefix -> log anyway, no :path fallback",
			headers: map[string]string{":path": "/health/live"},
			wantLog: true,
		},
		{
			name:    "original absent, :path does not have prefix -> log",
			headers: map[string]string{":path": "/orders"},
			wantLog: true,
		},
		{
			name:    "no headers at all -> log",
			headers: map[string]string{},
			wantLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := notEffectivelyMatchesPrefix("/health")
			assert.Equal(t, tt.wantLog, evalAccessLogFilter(t, filter, tt.headers))
		})
	}
}

func TestTranslator_CreateDynamicForwardProxyCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createDynamicForwardProxyCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, DynamicForwardProxyClusterName, cluster.Name)
}

func TestTranslator_CreateSDSCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createSDSCluster()
	assert.NotNil(t, cluster)
}

func TestTranslator_CreateUpstreamTLSContext(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	// Test with no certificate
	tlsContext := translator.createUpstreamTLSContext(nil, "example.com")
	assert.NotNil(t, tlsContext)
	assert.Equal(t, "example.com", tlsContext.Sni)

	// Test with certificate
	certPem := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	tlsContextWithCert := translator.createUpstreamTLSContext(certPem, "secure.example.com")
	assert.NotNil(t, tlsContextWithCert)
	assert.Equal(t, "secure.example.com", tlsContextWithCert.Sni)
}

func TestTranslator_ResolveUpstreamCluster_SimpleURL(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	urlStr := "http://backend:8080"
	upstream := &api.Upstream{
		Url: &urlStr,
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("test-upstream", upstream, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, clusterName)
	assert.NotNil(t, parsedURL)
	assert.Nil(t, timeout)
	assert.Equal(t, "backend", parsedURL.Hostname())
}

func TestTranslator_ResolveUpstreamCluster_HTTPSUrl(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	urlStr := "https://secure-backend:443/api"
	upstream := &api.Upstream{
		Url: &urlStr,
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("secure-upstream", upstream, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, clusterName)
	assert.NotNil(t, parsedURL)
	assert.Nil(t, timeout)
	assert.Equal(t, "https", parsedURL.Scheme)
}

func TestTranslator_ResolveUpstreamCluster_MissingURL(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	upstream := &api.Upstream{
		Url: nil, // No URL
	}

	_, _, _, err := translator.resolveUpstreamCluster("no-url-upstream", upstream, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no no-url-upstream upstream configured")
}

func strPtr(s string) *string {
	return &s
}

func TestTranslator_CreateCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name       string
		clusterNm  string
		urlStr     string
		certs      map[string][]byte
		hasCluster bool
	}{
		{name: "HTTP cluster", clusterNm: "http-cluster", urlStr: "http://localhost:8080", certs: nil, hasCluster: true},
		{name: "HTTPS cluster", clusterNm: "https-cluster", urlStr: "https://secure.example.com:443", certs: nil, hasCluster: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := parseURL(tt.urlStr)
			require.NoError(t, err)
			cluster := translator.createCluster(tt.clusterNm, parsedURL, tt.certs, nil)
			if tt.hasCluster {
				assert.NotNil(t, cluster)
				assert.Equal(t, tt.clusterNm, cluster.Name)
			}
		})
	}
}

func parseURL(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}

func TestTranslator_CreateListener_HTTP(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.ListenerPort = 8080
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	listener, routeConfig, err := translator.createListener(nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, listener)
	assert.NotNil(t, routeConfig)
	assert.Contains(t, listener.Name, "8080")
}

func TestTranslator_CreateDownstreamTLSContext_NoCert(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tlsContext, err := translator.createDownstreamTLSContext()
	// Should fail because no certs are configured
	assert.Error(t, err)
	assert.Nil(t, tlsContext)
}

func TestTranslator_CreateRoute_Basic(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	route := translator.createRoute(
		"api-123",                         // apiId
		"0000-test-api-0000-000000000000", // apiName
		"v1",                              // apiVersion
		"/api",                            // context
		"GET",                             // method
		"/users",                          // path
		"test-cluster",                    // clusterName
		"",                                // upstreamPath
		"localhost",                       // vhost
		"API",                             // apiKind
		"",                                // templateHandle
		"",                                // providerName
		nil,                               // hostRewrite
		"proj-001",                        // projectID
		nil,                               // timeoutCfg
		false,                             // useClusterHeader
		nil,                               // upstreamDefPaths
	)

	assert.NotNil(t, route)
	assert.Contains(t, route.Name, "GET")
	assert.Contains(t, route.Name, "/api/users")
}

// TestTranslator_CreateRoute_DynamicRouting pins the cluster specifier createRoute emits:
// a static cluster when useClusterHeader is false, and cluster_header routing (with the
// x-target-upstream header stripped before forwarding) when it is true. This is the
// legacy-xDS half of the sandbox dynamic-endpoint fix.
func TestTranslator_CreateRoute_DynamicRouting(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	t.Run("static cluster when useClusterHeader is false", func(t *testing.T) {
		r := translator.createRoute(
			"api-123", "0000-test-api-0000-000000000000", "v1", "/api", "GET", "/users",
			"static-cluster", "", "localhost", "API", "", "", nil, "proj-001", nil,
			false, nil,
		)
		require.NotNil(t, r)
		routeAction, ok := r.Action.(*route.Route_Route)
		require.True(t, ok)
		clusterSpec, ok := routeAction.Route.ClusterSpecifier.(*route.RouteAction_Cluster)
		require.True(t, ok, "expected a static cluster specifier")
		assert.Equal(t, "static-cluster", clusterSpec.Cluster)
		assert.NotContains(t, r.RequestHeadersToRemove, constants.TargetUpstreamHeader)
	})

	t.Run("cluster_header routing when useClusterHeader is true", func(t *testing.T) {
		r := translator.createRoute(
			"api-123", "0000-test-api-0000-000000000000", "v1", "/api", "GET", "/users",
			"static-cluster", "", "localhost", "API", "", "", nil, "proj-001", nil,
			true, nil,
		)
		require.NotNil(t, r)
		routeAction, ok := r.Action.(*route.Route_Route)
		require.True(t, ok)
		clusterSpec, ok := routeAction.Route.ClusterSpecifier.(*route.RouteAction_ClusterHeader)
		require.True(t, ok, "expected a cluster_header specifier for dynamic selection")
		assert.Equal(t, constants.TargetUpstreamHeader, clusterSpec.ClusterHeader)
		assert.Contains(t, r.RequestHeadersToRemove, constants.TargetUpstreamHeader)
	})
}

func TestTranslator_ExtractTemplateHandle_ValidLLMProvider(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		Kind: string(api.LLMProviderConfigurationKindLlmProvider),
		SourceConfiguration: map[string]interface{}{
			"kind": string(api.LLMProviderConfigurationKindLlmProvider),
			"spec": map[string]interface{}{
				"template": "openai-template",
			},
		},
		Origin: models.OriginGatewayAPI,
	}

	result := translator.extractTemplateHandle(storedCfg, nil)
	assert.Equal(t, "openai-template", result)
}

func TestTranslator_ExtractProviderName_ValidLLMProvider(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		Kind: string(api.LLMProviderConfigurationKindLlmProvider),
		SourceConfiguration: map[string]interface{}{
			"kind": string(api.LLMProviderConfigurationKindLlmProvider),
			"metadata": map[string]interface{}{
				"name": "openai-provider",
			},
		},
		Origin: models.OriginGatewayAPI,
	}

	result := translator.extractProviderName(storedCfg, nil)
	assert.Equal(t, "openai-provider", result)
}

// Tests for lines 184-200: WebSub API translation error handling
func TestTranslator_TranslateConfigs_WebSubAPIError(t *testing.T) {
	translator := createTestTranslator()

	// Create invalid WebSub API config that will cause translation error
	invalidConfig := &models.StoredConfig{
		UUID:   "0000-test-websub-invalid-0000-000000000000",
		Kind:   "WebSubApi",
		Origin: models.OriginGatewayAPI,
		// Use a non-WebSubAPI type so the type assertion in translateAsyncAPIConfig fails
		Configuration: "invalid-configuration",
	}

	result, err := translator.TranslateConfigs([]*models.StoredConfig{invalidConfig}, "test-correlation")

	// Should handle the error gracefully and continue
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// Tests for lines 1439-1493: createRoutePerTopic method
func TestTranslator_CreateRoutePerTopic(t *testing.T) {
	t.Run("Create route with all parameters", func(t *testing.T) {
		translator := createTestTranslator()

		route := translator.createRoutePerTopic(
			"api-123",
			"Test API",
			"v1.0.0",
			"/test",
			"POST",
			"/channel1",
			"test-cluster",
			"localhost",
			"WebSubApi",
			"project-123",
		)

		assert.NotNil(t, route)
		assert.NotEmpty(t, route.Name)
		assert.Equal(t, "/test/channel1", route.GetMatch().GetPath())
		assert.Equal(t, "/hub", route.GetRoute().PrefixRewrite)
		assert.Equal(t, "test-cluster", route.GetRoute().GetCluster())

		// Verify metadata contains project ID
		assert.NotNil(t, route.Metadata)
		metadata := route.Metadata.FilterMetadata["wso2.route"]
		assert.NotNil(t, metadata)
	})

	t.Run("Create route with version placeholder in context", func(t *testing.T) {
		translator := createTestTranslator()

		route := translator.createRoutePerTopic(
			"api-123",
			"Test API",
			"v1.0.0",
			"/test/$version", // Context with version placeholder
			"POST",
			"/channel1",
			"test-cluster",
			"localhost",
			"WebSubApi",
			"project-123",
		)

		assert.NotNil(t, route)
		// ConstructFullPath replaces $version with actual version
		assert.Equal(t, "/test/v1.0.0/channel1", route.GetMatch().GetPath())
	})
}

// Tests for lines 1568-1629: TLS context creation for policy engine
func TestTranslator_CreatePolicyEngineCluster_TLS(t *testing.T) {
	t.Run("TLS with client certificates", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.TLS.Enabled = true
		translator.routerConfig.PolicyEngine.TLS.CertPath = "/path/to/client.crt"
		translator.routerConfig.PolicyEngine.TLS.KeyPath = "/path/to/client.key"
		translator.routerConfig.PolicyEngine.TLS.CAPath = "/path/to/ca.crt"
		translator.routerConfig.PolicyEngine.TLS.ServerName = "policy-engine.example.com"
		translator.routerConfig.PolicyEngine.TLS.SkipVerify = false

		cluster := translator.createPolicyEngineCluster()
		assert.NotNil(t, cluster)
		assert.NotNil(t, cluster.TransportSocket)
		assert.Equal(t, "envoy.transport_sockets.tls", cluster.TransportSocket.Name)
	})

	t.Run("TLS without client certificates", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.TLS.Enabled = true
		translator.routerConfig.PolicyEngine.TLS.CertPath = ""
		translator.routerConfig.PolicyEngine.TLS.KeyPath = ""
		translator.routerConfig.PolicyEngine.TLS.CAPath = "/path/to/ca.crt"
		translator.routerConfig.PolicyEngine.TLS.SkipVerify = false

		cluster := translator.createPolicyEngineCluster()
		assert.NotNil(t, cluster)
		assert.NotNil(t, cluster.TransportSocket)
	})
}

func createTestTranslator() *Translator {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	return NewTranslator(logger, routerCfg, nil, cfg)
}

// Tests for lines 310-351: Event gateway WebSub hub configuration
func TestTranslator_TranslateConfigs_WebSubHub_Enabled(t *testing.T) {
	t.Run("Event gateway enabled creates WebSub listeners and clusters", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.EventGateway.WebSubHubPort = 8080
		translator.routerConfig.HTTPSEnabled = false

		// Empty config list to just test WebSub infrastructure
		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)
		assert.NotNil(t, resources)

		// Verify that WebSub clusters and listeners were created
		clusters := resources[resource.ClusterType]
		listeners := resources[resource.ListenerType]

		// Should contain WebSub internal cluster and dynamic forward proxy cluster
		clusterNames := make([]string, 0)
		for _, c := range clusters {
			clusterNames = append(clusterNames, c.(*cluster.Cluster).GetName())
		}
		assert.Contains(t, clusterNames, constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME)
		assert.Contains(t, clusterNames, DynamicForwardProxyClusterName)

		// Should contain listeners for WebSub
		listenerNames := make([]string, 0)
		for _, l := range listeners {
			listenerNames = append(listenerNames, l.(*listener.Listener).GetName())
		}
		// Check for internal listener
		assert.Contains(t, listenerNames, fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT))
	})

	t.Run("Event gateway with HTTPS enabled creates HTTPS listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "https://websub.example.com"
		translator.routerConfig.EventGateway.WebSubHubPort = 8443
		translator.routerConfig.HTTPSEnabled = false // Set to false to avoid TLS cert errors

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)

		listeners := resources[resource.ListenerType]
		listenerNames := make([]string, 0)
		for _, l := range listeners {
			listenerNames = append(listenerNames, l.(*listener.Listener).GetName())
		}

		// Should have HTTP listener for WebSub (HTTPS is disabled)
		assert.Contains(t, listenerNames, fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT))
	})

	t.Run("Event gateway URL parsing with missing port", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com"
		translator.routerConfig.EventGateway.WebSubHubPort = 9090

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)
		assert.NotNil(t, resources)
	})

	t.Run("Event gateway URL parsing with missing scheme", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "websub.example.com:8080"
		translator.routerConfig.EventGateway.WebSubHubPort = 8080

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)
		assert.NotNil(t, resources)
	})

	t.Run("Event gateway with invalid URL", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "://invalid-url"

		_, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid upstream URL")
	})
}

// Tests for lines 400-447: translateAsyncAPIConfig method
func TestTranslator_TranslateAsyncAPIConfig(t *testing.T) {
	t.Run("Translate valid WebSub API config", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"

		webhookConfig := &models.StoredConfig{
			UUID: "0000-websub-api-1-0000-000000000000",
			Kind: "WebSubApi",
			Configuration: api.WebSubAPI{
				Metadata: api.Metadata{
					Name:        "websub-test",
					Annotations: &map[string]string{"gateway.api-platform.wso2.com/project-id": "proj-123"},
				},
				Kind:       api.WebSubAPIKindWebSubApi,
				ApiVersion: api.WebSubAPIApiVersionGatewayApiPlatformWso2Comv1,
				Spec: api.WebhookAPIData{
					DisplayName: "WebSub Test API",
					Version:     "v1.0",
					Context:     "/webhook",
					Channels: &map[string]api.WebSubChannel{
						"/topic1": {},
						"topic2":  {},
					},
				},
			},
			Origin: models.OriginGatewayAPI,
		}

		routes, clusters, err := translator.translateAsyncAPIConfig(webhookConfig, []*models.StoredConfig{})
		require.NoError(t, err)
		assert.NotNil(t, routes)
		assert.NotNil(t, clusters)

		// Should create route for each channel plus the main route
		assert.GreaterOrEqual(t, len(routes), 2)

		// Verify routes are created correctly
		for _, r := range routes {
			assert.NotNil(t, r.GetMatch())
			assert.Equal(t, constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME, r.GetRoute().GetCluster())
		}
	})

	t.Run("WebSub API with invalid URL", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "://invalid"

		webhookConfig := &models.StoredConfig{
			UUID: "0000-websub-api-2-0000-000000000000",
			Kind: "WebSubApi",
			Configuration: api.WebSubAPI{
				Metadata:   api.Metadata{Name: "websub-invalid"},
				Kind:       api.WebSubAPIKindWebSubApi,
				ApiVersion: api.WebSubAPIApiVersionGatewayApiPlatformWso2Comv1,
				Spec: api.WebhookAPIData{
					DisplayName: "WebSub Invalid",
					Version:     "v1.0",
					Context:     "/webhook",
					Channels: &map[string]api.WebSubChannel{
						"/test": {},
					},
				},
			},
			Origin: models.OriginGatewayAPI,
		}

		_, _, err := translator.translateAsyncAPIConfig(webhookConfig, []*models.StoredConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid upstream URL")
	})
}

// Tests for lines 697-834: createInternalListenerForWebSubHub method
func TestTranslator_CreateInternalListenerForWebSubHub(t *testing.T) {
	t.Run("Create HTTP internal listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.AccessLogs.Enabled = false

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener name and port
		expectedName := fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT)
		assert.Equal(t, expectedName, listener.GetName())
		assert.Equal(t, uint32(constants.WEBSUB_HUB_INTERNAL_HTTP_PORT), listener.GetAddress().GetSocketAddress().GetPortValue())

		// Verify filter chain exists
		assert.NotEmpty(t, listener.GetFilterChains())
		filterChain := listener.GetFilterChains()[0]
		assert.NotNil(t, filterChain)

		// Should not have TLS for HTTP
		assert.Nil(t, filterChain.GetTransportSocket())
	})

	t.Run("Create HTTPS internal listener with TLS", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.AccessLogs.Enabled = false

		// This test will fail without proper TLS certs, so we expect an error
		_, err := translator.createInternalListenerForWebSubHub(true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create downstream TLS context")
	})

	t.Run("Create listener with policy engine enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.Host = "policy-engine"
		translator.routerConfig.PolicyEngine.Port = 9002
		translator.routerConfig.LuaScriptPath = "../../lua/request_transformation.lua"

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener was created successfully with ext_proc filter
		assert.NotEmpty(t, listener.GetFilterChains())
	})

	t.Run("Create listener with access logs enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.AccessLogs.Enabled = true
		translator.routerConfig.AccessLogs.Format = "json"
		translator.routerConfig.AccessLogs.JSONFields = map[string]string{
			"start_time": "%START_TIME%",
			"method":     "%REQ(:METHOD)%",
		}

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})

	t.Run("Create listener with tracing enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.config.TracingConfig.Enabled = true
		translator.config.TracingConfig.Endpoint = "otel-collector:4317"

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})
}

// Tests for lines 913-1108: createDynamicFwdListenerForWebSubHub method
func TestTranslator_CreateDynamicFwdListenerForWebSubHub(t *testing.T) {
	t.Run("Create HTTP dynamic forward proxy listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.AccessLogs.Enabled = false

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener name and port
		expectedName := fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT)
		assert.Equal(t, expectedName, listener.GetName())
		assert.Equal(t, uint32(constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT), listener.GetAddress().GetSocketAddress().GetPortValue())

		// Verify filter chain
		assert.NotEmpty(t, listener.GetFilterChains())
		filterChain := listener.GetFilterChains()[0]
		assert.NotNil(t, filterChain)
		assert.NotEmpty(t, filterChain.GetFilters())
	})

	t.Run("Create HTTPS dynamic forward proxy listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "https://websub.example.com"
		translator.routerConfig.AccessLogs.Enabled = false

		listener, err := translator.createDynamicFwdListenerForWebSubHub(true)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener name and port
		expectedName := fmt.Sprintf("listener_https_%d", constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT)
		assert.Equal(t, expectedName, listener.GetName())
		assert.Equal(t, uint32(constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT), listener.GetAddress().GetSocketAddress().GetPortValue())
	})

	t.Run("Create dynamic listener with policy engine enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.PolicyEngine.Host = "policy-engine"
		translator.routerConfig.PolicyEngine.Port = 9002
		translator.routerConfig.LuaScriptPath = "../../lua/request_transformation.lua"

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify HTTP filters include ext_proc when policy engine is enabled
		assert.NotEmpty(t, listener.GetFilterChains())
	})

	t.Run("Create dynamic listener with access logs enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.AccessLogs.Enabled = true
		translator.routerConfig.AccessLogs.Format = "json"
		translator.routerConfig.AccessLogs.JSONFields = map[string]string{
			"start_time": "%START_TIME%",
		}

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})

	t.Run("Create dynamic listener with tracing enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.config.TracingConfig.Enabled = true
		translator.config.TracingConfig.Endpoint = "otel-collector:4317"

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})

	t.Run("Verify dynamic forward proxy configuration", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify the listener has the correct configuration
		assert.Equal(t, "0.0.0.0", listener.GetAddress().GetSocketAddress().GetAddress())
		assert.Equal(t, core.SocketAddress_TCP, listener.GetAddress().GetSocketAddress().GetProtocol())
	})
}

// TestBuildRedirectAction guards HTTPRouteRedirectHostAndStatus: a 3xx direct response with a
// Location header becomes an Envoy RedirectAction (single, path-preserving Location) instead of
// a direct_response + manual Location header (which Envoy comma-joins with its own auto Location).
func TestBuildRedirectAction(t *testing.T) {
	loc := func(v string) []models.RouteResponseHeader {
		return []models.RouteResponseHeader{{Name: "Location", Value: v}}
	}

	// 302 hostname redirect: host set, scheme set, path preserved (unset), code FOUND.
	ra := buildRedirectAction(302, loc("http://example.org"))
	require.NotNil(t, ra)
	assert.Equal(t, "example.org", ra.GetHostRedirect())
	assert.Equal(t, "http", ra.GetSchemeRedirect())
	assert.Equal(t, route.RedirectAction_FOUND, ra.GetResponseCode())
	assert.Nil(t, ra.GetPathRewriteSpecifier(), "hostname-only redirect must preserve the original path")

	// 301 maps to MOVED_PERMANENTLY.
	ra = buildRedirectAction(301, loc("http://example.org"))
	require.NotNil(t, ra)
	assert.Equal(t, route.RedirectAction_MOVED_PERMANENTLY, ra.GetResponseCode())

	// Location with an explicit port and path.
	ra = buildRedirectAction(302, loc("https://example.org:8443/foo"))
	require.NotNil(t, ra)
	assert.Equal(t, "example.org", ra.GetHostRedirect())
	assert.Equal(t, "https", ra.GetSchemeRedirect())
	assert.Equal(t, uint32(8443), ra.GetPortRedirect())
	assert.Equal(t, "/foo", ra.GetPathRedirect())

	// Not a redirect: non-3xx status returns nil (stays a direct_response).
	assert.Nil(t, buildRedirectAction(404, loc("http://example.org")))
	// 3xx without a Location header is not a representable redirect.
	assert.Nil(t, buildRedirectAction(302, nil))
}

// parseDurationAllowZero must accept exactly what the CRD admission controller accepts
// (constants.ResilienceDurationPattern): single-unit durations including "0s" to disable, while
// rejecting compound, negative, and unitless values.
func TestParseDurationAllowZero_MatchesCRDPattern(t *testing.T) {
	ptr := func(s string) *string { return &s }

	t.Run("accepts single-unit and zero", func(t *testing.T) {
		for _, in := range []string{"30s", "500ms", "1m", "2h", "1.5s", "0s", "0ms"} {
			d, err := parseDurationAllowZero(ptr(in))
			if err != nil {
				t.Errorf("expected %q to be accepted, got error: %v", in, err)
				continue
			}
			if d == nil {
				t.Errorf("expected %q to yield a non-nil duration", in)
			}
		}
	})

	t.Run("nil and empty yield nil without error", func(t *testing.T) {
		for _, in := range []*string{nil, ptr(""), ptr("  ")} {
			d, err := parseDurationAllowZero(in)
			if err != nil || d != nil {
				t.Errorf("expected nil,nil for empty input, got %v,%v", d, err)
			}
		}
	})

	t.Run("rejects compound, negative, and unitless", func(t *testing.T) {
		for _, in := range []string{"1h30m", "1m30s", "-30s", "-5s", "30", "0", "15seconds", "abc"} {
			if _, err := parseDurationAllowZero(ptr(in)); err == nil {
				t.Errorf("expected %q to be rejected, but it was accepted", in)
			}
		}
	})
}
