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
	"strconv"
	"strings"
	"testing"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tracev3 "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	otelresourcedetectorsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/tracers/opentelemetry/resource_detectors/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	metadatav3 "github.com/envoyproxy/go-control-plane/envoy/type/metadata/v3"
	tracingv3 "github.com/envoyproxy/go-control-plane/envoy/type/tracing/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certstore"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
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
			ServerHeaderTransformation:    commonconstants.OVERWRITE,
			PerConnectionBufferLimitBytes: 1048576,
			PathWithEscapedSlashesAction:  commonconstants.KEEP_UNCHANGED,
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	// Either matcher kind is accepted: an Exact path match is emitted as Envoy's
	// native matcher rather than a regex (see TestTranslator_ExactPathUsesNativeMatcher),
	// and a rewrite assertion against a route that would never have been selected
	// proves nothing either way.
	switch spec := r.Match.PathSpecifier.(type) {
	case *route.RouteMatch_SafeRegex:
		require.True(t, regexp.MustCompile(spec.SafeRegex.Regex).MatchString(requestPath),
			"match regex %q should match request %q", spec.SafeRegex.Regex, requestPath)
	case *route.RouteMatch_Path:
		require.Equal(t, spec.Path, requestPath,
			"exact matcher %q should match request %q", spec.Path, requestPath)
	default:
		t.Fatalf("route %q has no path matcher", r.GetName())
	}

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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

// A route carrying UpstreamPathOverride forwards to exactly that path under its
// upstream's base path, whatever it matched downstream. It is the shape a route
// whose gateway-facing path is configurable but whose upstream path is fixed by a
// protocol needs — a proxied A2A Agent Card being the case it exists for.
//
// Getting this wrong is silent whenever the two paths happen to be equal, so each
// case below configures a gateway path that differs from the upstream one.
func TestTranslator_UpstreamPathOverride(t *testing.T) {
	logger := createTestLogger()
	translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
	require.NoError(t, err)

	const override = "/.well-known/agent-card.json"

	tests := []struct {
		name          string
		context       string
		operationPath string
		upstreamPath  string
		request       string
		wantUpstream  string
	}{
		{
			name:          "root upstream",
			context:       "/weather",
			operationPath: "/card",
			request:       "/weather/card",
			wantUpstream:  override,
		},
		{
			name:          "base-path upstream",
			context:       "/weather",
			operationPath: "/card",
			upstreamPath:  "/a2a/v1",
			request:       "/weather/card",
			wantUpstream:  "/a2a/v1" + override,
		},
		{
			name:          "the gateway path already being the upstream one changes nothing",
			context:       "/weather",
			operationPath: override,
			request:       "/weather" + override,
			wantUpstream:  override,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc := &models.RuntimeDeployConfig{
				UpstreamClusters: map[string]*models.UpstreamCluster{
					"main": {BasePath: tt.upstreamPath, Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
				},
			}
			rdcRoute := &models.Route{
				Method:               "GET",
				Path:                 tt.context + tt.operationPath,
				OperationPath:        tt.operationPath,
				PathMatchType:        "Exact",
				UpstreamPathOverride: override,
				Upstream:             models.RouteUpstream{ClusterKey: "main"},
			}
			r := translator.createRouteFromRDC("GET|"+tt.context+tt.operationPath+"|", rdcRoute, rdc)
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
	translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
	require.NoError(t, err)

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

// TestSortRoutesByPriority_LegacyExactBeatsWildcardRegex covers simple-form operations used by
// LLM-generated routes. Both routes intentionally remain safe_regex matchers to preserve the
// legacy optional-trailing-slash behavior; sorting must use path semantics rather than the raw
// regex length so the wildcard's (?:/.*)? syntax cannot shadow the exact route's policy chain.
func TestSortRoutesByPriority_LegacyExactBeatsWildcardRegex(t *testing.T) {
	logger := createTestLogger()
	translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
	require.NoError(t, err)

	const (
		exactKey    = "POST|/llm/a/b|"
		wildcardKey = "POST|/llm/a/*|"
	)
	rdc := &models.RuntimeDeployConfig{
		Routes: map[string]*models.Route{
			exactKey: {
				Method:        "POST",
				Path:          "/llm/a/b",
				OperationPath: "/a/b",
				Upstream:      models.RouteUpstream{ClusterKey: "main"},
			},
			wildcardKey: {
				Method:        "POST",
				Path:          "/llm/a/*",
				OperationPath: "/a/*",
				Upstream:      models.RouteUpstream{ClusterKey: "main"},
			},
		},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"main": {Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
		},
	}

	routes, _, err := translator.translateRuntimeConfig(rdc)
	require.NoError(t, err)
	require.Len(t, routes, 2)

	byName := map[string]*route.Route{}
	for _, translatedRoute := range routes {
		byName[translatedRoute.GetName()] = translatedRoute
	}
	require.IsType(t, &route.RouteMatch_SafeRegex{}, byName[exactKey].GetMatch().GetPathSpecifier())
	require.IsType(t, &route.RouteMatch_SafeRegex{}, byName[wildcardKey].GetMatch().GetPathSpecifier())
	assert.Equal(t, "^/llm/a/b/?$", byName[exactKey].GetMatch().GetSafeRegex().GetRegex())
	assert.Equal(t, "^/llm/a(?:/.*)?$", byName[wildcardKey].GetMatch().GetSafeRegex().GetRegex())

	sorted := SortRoutesByPriority(routes)
	assert.Equal(t, exactKey, sorted[0].GetName())
	assert.Equal(t, wildcardKey, sorted[1].GetName())
}

func TestTranslator_SanitizeClusterName(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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

func TestNewTranslator_WithoutCerts_StoreExistsWithEmptyBundle(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()

	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)
	assert.NotNil(t, translator)
	require.NotNil(t, translator.GetCertStore(), "the certificate store backs SDS and must exist without a custom certs path")
	assert.Empty(t, translator.GetCertStore().GetCombinedCertificates())
}

func TestTranslator_ExtractTemplateHandle_NilSourceConfig(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
			translator, err := NewTranslator(logger, routerCfg, nil, cfg)
			require.NoError(t, err)

			lis, _, err := translator.createListener(nil, false, false)
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

// The generated HttpConnectionManager must canonicalize the request path
// (NormalizePath, MergeSlashes) before routing/policy/authz matching — an
// un-normalized path could desynchronize the route Envoy selects from what
// validateNotReservedHealthPath (pkg/config/validator.go) reasoned about at
// config time. See go-control-plane-xds-security.md directive 6.
func TestTranslator_CreateListener_HCMPathNormalization(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	lis, _, err := translator.createListener(nil, false, false)
	require.NoError(t, err)

	manager := extractHCM(t, lis)
	require.NotNil(t, manager.GetNormalizePath(), "normalize_path must be explicitly set")
	assert.True(t, manager.GetNormalizePath().GetValue(), "normalize_path")
	assert.True(t, manager.GetMergeSlashes(), "merge_slashes")
	// PathWithEscapedSlashesAction is fully config-driven — see
	// TestTranslator_CreateListener_PathWithEscapedSlashesAction for its default
	// and every configurable value.
}

func TestTranslator_CreateListener_PathNormalization(t *testing.T) {
	tests := []struct {
		name                     string
		disablePathNormalization bool
		wantNormalizePath        bool
		wantMergeSlashes         bool
	}{
		{
			name:                     "enabled by default",
			disablePathNormalization: false,
			wantNormalizePath:        true,
			wantMergeSlashes:         true,
		},
		{
			name:                     "disabled via config",
			disablePathNormalization: true,
			wantNormalizePath:        false,
			wantMergeSlashes:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			routerCfg := testRouterConfig()
			routerCfg.HTTPListener.DisablePathNormalization = tt.disablePathNormalization
			cfg := testConfig()
			cfg.Router = *routerCfg
			translator, err := NewTranslator(logger, routerCfg, nil, cfg)
			require.NoError(t, err)

			lis, _, err := translator.createListener(nil, false, false)
			require.NoError(t, err)

			manager := extractHCM(t, lis)
			require.NotNil(t, manager.GetNormalizePath(), "normalize_path must be explicitly set")
			assert.Equal(t, tt.wantNormalizePath, manager.GetNormalizePath().GetValue(), "normalize_path")
			assert.Equal(t, tt.wantMergeSlashes, manager.GetMergeSlashes(), "merge_slashes")
		})
	}
}

func TestTranslator_CreateListener_PathWithEscapedSlashesAction(t *testing.T) {
	tests := []struct {
		name                         string
		pathWithEscapedSlashesAction string
		want                         hcm.HttpConnectionManager_PathWithEscapedSlashesAction
	}{
		{
			name:                         "keeps unchanged by default",
			pathWithEscapedSlashesAction: commonconstants.KEEP_UNCHANGED,
			want:                         hcm.HttpConnectionManager_KEEP_UNCHANGED,
		},
		{
			name:                         "configurable to reject",
			pathWithEscapedSlashesAction: commonconstants.REJECT_REQUEST,
			want:                         hcm.HttpConnectionManager_REJECT_REQUEST,
		},
		{
			name:                         "configurable to unescape and forward",
			pathWithEscapedSlashesAction: commonconstants.UNESCAPE_AND_FORWARD,
			want:                         hcm.HttpConnectionManager_UNESCAPE_AND_FORWARD,
		},
		{
			name:                         "configurable to unescape and redirect",
			pathWithEscapedSlashesAction: commonconstants.UNESCAPE_AND_REDIRECT,
			want:                         hcm.HttpConnectionManager_UNESCAPE_AND_REDIRECT,
		},
		{
			name:                         "unknown value falls back to keep unchanged",
			pathWithEscapedSlashesAction: "SOMETHING_UNKNOWN",
			want:                         hcm.HttpConnectionManager_KEEP_UNCHANGED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			routerCfg := testRouterConfig()
			routerCfg.HTTPListener.PathWithEscapedSlashesAction = tt.pathWithEscapedSlashesAction
			cfg := testConfig()
			cfg.Router = *routerCfg
			translator, err := NewTranslator(logger, routerCfg, nil, cfg)
			require.NoError(t, err)

			lis, _, err := translator.createListener(nil, false, false)
			require.NoError(t, err)

			manager := extractHCM(t, lis)
			assert.Equal(t, tt.want, manager.GetPathWithEscapedSlashesAction(), "path_with_escaped_slashes_action")
		})
	}
}

func TestTranslator_CreateAccessLogConfig_Disabled(t *testing.T) {
	// Both consumers off: no stdout sink, and no ALS sink either. The stdout
	// format fields are left unset on purpose — with access logs disabled they
	// are never read, so an empty format must not surface as an error.
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs.Enabled = false
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	logs, err := translator.createAccessLogConfig()
	assert.NoError(t, err)
	assert.Empty(t, logs)
}

// TestTranslator_AccessLogSinks_DecoupledFromStdoutToggle pins the invariant that
// router.access_logs.enabled governs only the operator-facing stdout log line,
// never the gRPC ALS sink. ALS is the sole data source for traffic logging and
// analytics (the policy-engine publishes exclusively on receipt of an ALS entry),
// so gating it on the stdout toggle silently disables both with no error anywhere.
func TestTranslator_AccessLogSinks_DecoupledFromStdoutToggle(t *testing.T) {
	const (
		fileSink = "envoy.access_loggers.file"
		grpcSink = "envoy.access_loggers.http_grpc"
	)

	tests := []struct {
		name             string
		stdoutEnabled    bool
		trafficLogging   bool
		analytics        bool
		wantSinkNames    []string
		wantManagerSinks bool
	}{
		{
			name:             "stdout off, traffic logging on -> ALS sink only",
			stdoutEnabled:    false,
			trafficLogging:   true,
			wantSinkNames:    []string{grpcSink},
			wantManagerSinks: true,
		},
		{
			name:             "stdout off, analytics on -> ALS sink only",
			stdoutEnabled:    false,
			analytics:        true,
			wantSinkNames:    []string{grpcSink},
			wantManagerSinks: true,
		},
		{
			name:             "stdout on, traffic logging on -> both sinks",
			stdoutEnabled:    true,
			trafficLogging:   true,
			wantSinkNames:    []string{fileSink, grpcSink},
			wantManagerSinks: true,
		},
		{
			name:             "stdout on, collector off -> stdout sink only",
			stdoutEnabled:    true,
			wantSinkNames:    []string{fileSink},
			wantManagerSinks: true,
		},
		{
			name:             "both off -> no sinks at all",
			stdoutEnabled:    false,
			wantSinkNames:    nil,
			wantManagerSinks: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			routerCfg := testRouterConfig()
			routerCfg.AccessLogs = config.AccessLogsConfig{
				Enabled:    tt.stdoutEnabled,
				Format:     "text",
				TextFormat: "[%START_TIME%] %RESPONSE_CODE%",
			}
			cfg := testConfig()
			cfg.Router = *routerCfg
			cfg.TrafficLogging.Enabled = tt.trafficLogging
			cfg.Analytics.Enabled = tt.analytics
			cfg.Collector.Server = config.GRPCEventServerConfig{
				Mode:                "tcp",
				BufferFlushInterval: 1000,
				BufferSizeBytes:     16384,
				GRPCRequestTimeout:  5000,
			}
			translator, err := NewTranslator(logger, routerCfg, nil, cfg)
			require.NoError(t, err)

			logs, err := translator.createAccessLogConfig()
			require.NoError(t, err)

			gotNames := make([]string, 0, len(logs))
			for _, l := range logs {
				gotNames = append(gotNames, l.Name)
				assert.NotNil(t, l.Filter, "sink %q must carry the reserved health-path suppression filter", l.Name)
			}
			assert.Equal(t, tt.wantSinkNames, nilIfEmpty(gotNames), "access log sinks")

			// The sinks must actually reach the HCM — the caller's gate is half the fix.
			lis, _, err := translator.createListener(nil, false, false)
			require.NoError(t, err)
			manager := extractHCM(t, lis)
			if !tt.wantManagerSinks {
				assert.Empty(t, manager.GetAccessLog(), "no sink should be attached to the HCM")
				return
			}
			managerNames := make([]string, 0, len(manager.GetAccessLog()))
			for _, l := range manager.GetAccessLog() {
				managerNames = append(managerNames, l.Name)
			}
			assert.Equal(t, tt.wantSinkNames, managerNames, "access log sinks attached to the HCM")
		})
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	logs, err := translator.createAccessLogConfig()
	assert.Error(t, err)
	assert.Nil(t, logs)
	assert.Contains(t, err.Error(), "json_fields not configured")
}

// TestTranslator_CreateFileAccessLog_SuppressesHealthProbes pins the invariant
// that the stdout access log sink suppresses the reserved /_gateway-health
// prefix, mirroring the ALS sink's suppression (TestTranslator_CreateGRPCAccessLog),
// so kubernetes readiness/liveness probes never reach the operator's log.
func TestTranslator_CreateFileAccessLog_SuppressesHealthProbes(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled:    true,
		Format:     "text",
		TextFormat: "[%START_TIME%] %RESPONSE_CODE%",
	}
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	accessLog, err := translator.createFileAccessLog()
	assert.NoError(t, err)
	require.NotNil(t, accessLog)
	require.NotNil(t, accessLog.Filter, "reserved health-path suppression filter is always attached")
	assert.False(t, evalAccessLogFilter(t, accessLog.Filter, map[string]string{
		":path": constants.GatewayHealthyPath,
	}), "gateway healthy-check path is suppressed")
	assert.False(t, evalAccessLogFilter(t, accessLog.Filter, map[string]string{
		":path": constants.GatewayReadyPath,
	}), "gateway ready-check path is suppressed")
	assert.True(t, evalAccessLogFilter(t, accessLog.Filter, map[string]string{
		":path": "/orders",
	}), "non-health path is still logged")
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		filter, err := translator.createExtProcFilter()
		assert.NoError(t, err)
		assert.NotNil(t, filter)
		assert.Equal(t, constants.ExtProcFilterName, filter.Name)
	})

	// mtls-auth needs every connection.* attribute; a missing one silently
	// starves the policy engine of a fact.
	t.Run("RequestAttributes carries every connection.* fact plus the route name", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		filter, err := translator.createExtProcFilter()
		require.NoError(t, err)

		var extProcConfig extproc.ExternalProcessor
		require.NoError(t, filter.GetTypedConfig().UnmarshalTo(&extProcConfig))

		want := []string{
			constants.ExtProcRequestAttributeRouteName,
			constants.ExtProcRequestAttributeConnectionMTLS,
			constants.ExtProcRequestAttributeConnectionPeerCertificate,
			constants.ExtProcRequestAttributeConnectionPeerCertificateDigest,
			constants.ExtProcRequestAttributeConnectionSubjectPeerCertificate,
			constants.ExtProcRequestAttributeConnectionURISANPeerCertificate,
			constants.ExtProcRequestAttributeConnectionDNSSANPeerCertificate,
			constants.ExtProcRequestAttributeConnectionTLSVersion,
			constants.ExtProcRequestAttributeConnectionRequestedServerName,
			constants.ExtProcRequestAttributeConnectionPeerCertificateValid,
		}
		assert.ElementsMatch(t, want, extProcConfig.RequestAttributes)
	})
}

func TestTranslator_CreateRouteConfiguration(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	// Test with nil virtual hosts
	routeConfig := translator.createRouteConfiguration(nil)
	assert.NotNil(t, routeConfig)
	assert.Equal(t, SharedRouteConfigName, routeConfig.Name)
}

func TestTranslator_TranslateConfigs_EmptyConfigs(t *testing.T) {
	logger := createTestLogger()

	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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

// The gateway's own /ready and /healthy direct-response routes must be present in
// every virtual host — including the pre-seeded "*" wildcard vhost when zero
// APIs/LLMProviders/LLMProxies are deployed, and every API-specific vhost once
// resources are deployed — and must always be evaluated before the "no-api-found"
// catch-all, which matches Prefix "/" and would otherwise shadow them.
func TestTranslator_TranslateConfigs_GatewayHealthRoutes(t *testing.T) {
	assertHealthRoutes := func(t *testing.T, vh *route.VirtualHost) {
		t.Helper()
		require.GreaterOrEqual(t, len(vh.Routes), 3,
			"virtual host %q must contain at least the 2 health routes plus the catch-all", vh.Name)

		byName := make(map[string]*route.Route, len(vh.Routes))
		var readyIdx, healthyIdx, catchAllIdx = -1, -1, -1
		for i, r := range vh.Routes {
			byName[r.Name] = r
			switch r.Name {
			case "gateway-ready":
				readyIdx = i
			case "gateway-healthy":
				healthyIdx = i
			case "no-api-found":
				catchAllIdx = i
			}
		}

		readyRoute, ok := byName["gateway-ready"]
		require.True(t, ok, "virtual host %q missing gateway-ready route", vh.Name)
		assert.Equal(t, constants.GatewayReadyPath, readyRoute.GetMatch().GetPath())
		assert.Equal(t, uint32(200), readyRoute.GetDirectResponse().GetStatus())

		healthyRoute, ok := byName["gateway-healthy"]
		require.True(t, ok, "virtual host %q missing gateway-healthy route", vh.Name)
		assert.Equal(t, constants.GatewayHealthyPath, healthyRoute.GetMatch().GetPath())
		assert.Equal(t, uint32(200), healthyRoute.GetDirectResponse().GetStatus())

		assert.Equal(t, uint32(0), readyRoute.GetTracing().GetOverallSampling().GetNumerator(),
			"gateway-ready must force tracing sampling to zero")
		assert.Equal(t, uint32(0), healthyRoute.GetTracing().GetOverallSampling().GetNumerator(),
			"gateway-healthy must force tracing sampling to zero")

		require.NotEqual(t, -1, catchAllIdx, "virtual host %q missing no-api-found catch-all", vh.Name)
		assert.Less(t, readyIdx, catchAllIdx,
			"gateway-ready must be evaluated before the Prefix:\"/\" catch-all or it will be shadowed")
		assert.Less(t, healthyIdx, catchAllIdx,
			"gateway-healthy must be evaluated before the Prefix:\"/\" catch-all or it will be shadowed")

		catchAllRoute := vh.Routes[catchAllIdx]
		assert.Nil(t, catchAllRoute.GetTracing(),
			"tracing suppression must be scoped to the health routes only, not the no-api-found catch-all")
	}

	t.Run("present on the wildcard vhost with zero deployed artifacts", func(t *testing.T) {
		logger := createTestLogger()
		translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
		require.NoError(t, err)

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test-correlation-id")
		require.NoError(t, err)

		found := false
		for _, res := range resources[resource.RouteType] {
			rc, ok := res.(*route.RouteConfiguration)
			require.True(t, ok)
			for _, vh := range rc.VirtualHosts {
				found = true
				assertHealthRoutes(t, vh)
			}
		}
		require.True(t, found, "expected the wildcard vhost even with zero deployed artifacts")
	})

	t.Run("present on every vhost once APIs are deployed", func(t *testing.T) {
		logger := createTestLogger()
		translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
		require.NoError(t, err)

		configs := []*models.StoredConfig{makeRestAPI("uuid-api-1", "api-one", "/api-one")}
		resources, err := translator.TranslateConfigs(configs, "test-correlation-id")
		require.NoError(t, err)

		vhostsSeen := map[string]bool{}
		for _, res := range resources[resource.RouteType] {
			rc, ok := res.(*route.RouteConfiguration)
			require.True(t, ok)
			for _, vh := range rc.VirtualHosts {
				vhostsSeen[vh.Name] = true
				assertHealthRoutes(t, vh)
			}
		}
		require.True(t, vhostsSeen["localhost"], "expected the API's own vhost to carry the health routes too")
		require.True(t, vhostsSeen["*"], "expected the wildcard vhost to still be present")
	})
}

func TestTranslator_GetVHostDomains(t *testing.T) {
	logger := createTestLogger()

	t.Run("fallback domains when explicit domain lists are empty", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("api.example.com")
		assert.Equal(t, []string{"api.example.com", "api.example.com:*"}, domains)
	})

	t.Run("expands configured main domains when vhost equals main default", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "*.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"*.wso2.com", "*.foo.com"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("*.wso2.com")
		assert.Equal(t, []string{"*.wso2.com", "*.wso2.com:*", "*.foo.com", "*.foo.com:*"}, domains)
	})

	t.Run("expands configured sandbox domains when vhost equals sandbox default", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Sandbox.Default = "*-sandbox.wso2.com"
		routerCfg.VHosts.Sandbox.Domains = []string{"*-sandbox.wso2.com", "*-sandbox.foo.com"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("*-sandbox.wso2.com")
		assert.Equal(t, []string{"*-sandbox.wso2.com", "*-sandbox.wso2.com:*", "*-sandbox.foo.com", "*-sandbox.foo.com:*"}, domains)
	})

	t.Run("api-level vhost override uses fallback pair only", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "*.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"*.wso2.com", "*.foo.com"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("custom.wso2.com")
		assert.Equal(t, []string{"custom.wso2.com", "custom.wso2.com:*"}, domains)
	})

	t.Run("port-qualified domain is not expanded with :*", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("api.example.com:8443")
		assert.Equal(t, []string{"api.example.com:8443"}, domains)
	})

	t.Run("whitespace-only domain list falls back to effective vhost", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "*.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"   ", "  "}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("*.wso2.com")
		assert.Equal(t, []string{"*.wso2.com", "*.wso2.com:*"}, domains)
	})

	t.Run("port-qualified domain in configured list is not expanded with :*", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.VHosts.Main.Default = "api.wso2.com"
		routerCfg.VHosts.Main.Domains = []string{"api.wso2.com", "api.wso2.com:8443"}
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		domains := translator.getVHostDomains("api.wso2.com")
		assert.Equal(t, []string{"api.wso2.com", "api.wso2.com:*", "api.wso2.com:8443"}, domains)
	})
}

func TestTranslator_ExtractTemplateHandle_InvalidKind(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	tracingCfg, err := translator.createTracingConfig()
	assert.NoError(t, err)
	require.NotNil(t, tracingCfg)
	assert.True(t, tracingCfg.GetSpawnUpstreamSpan().GetValue())

	otelConfig := &tracev3.OpenTelemetryConfig{}
	err = tracingCfg.GetProvider().GetTypedConfig().UnmarshalTo(otelConfig)
	require.NoError(t, err)
	assert.Equal(t, "test-service", otelConfig.GetServiceName())

	// With no [tracing.resource_attributes] configured, only the environment
	// detector is attached — it contributes nothing when OTEL_RESOURCE_ATTRIBUTES
	// is unset, so tracing still works without any resource attributes.
	require.Len(t, otelConfig.GetResourceDetectors(), 1)
	assert.Equal(t, "envoy.tracers.opentelemetry.resource_detectors.environment",
		otelConfig.GetResourceDetectors()[0].GetName())

	envDetector := &otelresourcedetectorsv3.EnvironmentResourceDetectorConfig{}
	err = otelConfig.GetResourceDetectors()[0].GetTypedConfig().UnmarshalTo(envDetector)
	require.NoError(t, err)
}

func TestTranslator_CreateTracingConfig_ResourceAttributes(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	cfg.TracingConfig.ResourceAttributes = map[string]string{
		"deployment.environment": "prod",
		"service.namespace":      "api-gw",
	}
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	tracingCfg, err := translator.createTracingConfig()
	require.NoError(t, err)
	require.NotNil(t, tracingCfg)

	otelConfig := &tracev3.OpenTelemetryConfig{}
	err = tracingCfg.GetProvider().GetTypedConfig().UnmarshalTo(otelConfig)
	require.NoError(t, err)

	// Order is load-bearing: Envoy merges detectors in sequence and later ones win,
	// so the static detector must come after the environment detector for
	// [tracing.resource_attributes] to override OTEL_RESOURCE_ATTRIBUTES.
	detectors := otelConfig.GetResourceDetectors()
	require.Len(t, detectors, 2)
	assert.Equal(t, "envoy.tracers.opentelemetry.resource_detectors.environment", detectors[0].GetName())
	assert.Equal(t, "envoy.tracers.opentelemetry.resource_detectors.static_config", detectors[1].GetName())

	staticDetector := &otelresourcedetectorsv3.StaticConfigResourceDetectorConfig{}
	err = detectors[1].GetTypedConfig().UnmarshalTo(staticDetector)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"deployment.environment": "prod",
		"service.namespace":      "api-gw",
	}, staticDetector.GetAttributes())
}

// TestTranslator_CreateTracingConfig_PeerServiceCustomTag guards the fix for GH issue #2883:
// with tracing enabled, the HCM tracing config must carry a "peer.service" custom tag, sourced
// from host metadata under the envoy.lb/hostname key set by setEndpointPeerHostname — without
// this, APM backends have no peer attribute on upstream CLIENT spans to build a service
// dependency map from. This pins the wire shape so a go-control-plane version bump can't
// silently reshape it without a test failure.
//
// Also guards http.route (the OTel semantic-convention attribute for the matched route
// template, see setRouteHTTPRoute), sourced from ROUTE metadata under the
// wso2.route/http.route key. Tags are looked up by name rather than asserted by
// count/order — new tags may be added alongside these two over time.
func TestTranslator_CreateTracingConfig_PeerServiceCustomTag(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	tracingCfg, err := translator.createTracingConfig()
	require.NoError(t, err)
	require.NotNil(t, tracingCfg)

	tagsByName := make(map[string]*tracingv3.CustomTag)
	for _, tag := range tracingCfg.GetCustomTags() {
		tagsByName[tag.GetTag()] = tag
	}

	peerServiceTag, ok := tagsByName["peer.service"]
	require.True(t, ok, "expected a peer.service custom tag")

	metaTag, ok := peerServiceTag.GetType().(*tracingv3.CustomTag_Metadata_)
	require.True(t, ok, "peer.service must be sourced from metadata, got %T", peerServiceTag.GetType())

	hostKind, ok := metaTag.Metadata.GetKind().GetKind().(*metadatav3.MetadataKind_Host_)
	require.True(t, ok, "peer.service must read HOST metadata (the selected upstream endpoint), got %T",
		metaTag.Metadata.GetKind().GetKind())
	assert.NotNil(t, hostKind)

	key := metaTag.Metadata.GetMetadataKey()
	assert.Equal(t, "envoy.lb", key.GetKey())
	require.Len(t, key.GetPath(), 1)
	assert.Equal(t, "hostname", key.GetPath()[0].GetKey())

	httpRouteTag, ok := tagsByName["http.route"]
	require.True(t, ok, "expected an http.route custom tag")

	routeMetaTag, ok := httpRouteTag.GetType().(*tracingv3.CustomTag_Metadata_)
	require.True(t, ok, "http.route must be sourced from metadata, got %T", httpRouteTag.GetType())

	routeKind, ok := routeMetaTag.Metadata.GetKind().GetKind().(*metadatav3.MetadataKind_Route_)
	require.True(t, ok, "http.route must read ROUTE metadata (the matched route), got %T",
		routeMetaTag.Metadata.GetKind().GetKind())
	assert.NotNil(t, routeKind)

	routeKey := routeMetaTag.Metadata.GetMetadataKey()
	assert.Equal(t, "wso2.route", routeKey.GetKey())
	require.Len(t, routeKey.GetPath(), 1)
	assert.Equal(t, "http.route", routeKey.GetPath()[0].GetKey())
}

func TestTranslator_CreateOTELCollectorCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	cluster := translator.createOTELCollectorCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, OTELCollectorClusterName, cluster.Name)
}

func TestTranslator_CreateOTELCollectorCluster_Disabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = false
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

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
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	accessLog, err := translator.createGRPCAccessLog()
	assert.NoError(t, err)
	assert.NotNil(t, accessLog)
	require.NotNil(t, accessLog.Filter, "reserved health-path suppression filter is always attached")
	assert.False(t, evalAccessLogFilter(t, accessLog.Filter, map[string]string{
		":path": constants.GatewayHealthyPath,
	}), "gateway health-check path is suppressed even with no ignore_path_prefixes configured")
	assert.True(t, evalAccessLogFilter(t, accessLog.Filter, map[string]string{
		":path": "/orders",
	}), "non-health path is still logged")
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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

func TestBuildReservedHealthPathAccessLogFilter(t *testing.T) {
	filter := buildReservedHealthPathAccessLogFilter()
	require.NotNil(t, filter)

	assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
		":path": constants.GatewayHealthyPath,
	}), "healthy probe path is suppressed")
	assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
		":path": constants.GatewayReadyPath,
	}), "ready probe path is suppressed")
	assert.True(t, evalAccessLogFilter(t, filter, map[string]string{
		":path": "/orders",
	}), "unrelated path is logged")
	assert.True(t, evalAccessLogFilter(t, filter, map[string]string{}),
		"no :path header at all is logged (fails open, never suppresses by default)")
}

func TestBuildAccessLogFilter(t *testing.T) {
	t.Run("no ignore prefixes -> health-only suppression", func(t *testing.T) {
		filter := buildAccessLogFilter(nil)
		require.NotNil(t, filter, "reserved health-path filter is always attached")
		assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
			":path": constants.GatewayHealthyPath,
		}))
		assert.True(t, evalAccessLogFilter(t, filter, map[string]string{
			":path": "/orders",
		}))
	})

	t.Run("with ignore prefixes -> both health path and configured prefix suppressed", func(t *testing.T) {
		filter := buildAccessLogFilter([]string{"/metrics"})
		require.NotNil(t, filter)
		assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
			":path": constants.GatewayHealthyPath,
		}), "reserved health path still suppressed alongside a configured prefix")
		assert.False(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/metrics/scrape",
			":path":                 "/orders",
		}), "configured ignore prefix still suppressed")
		assert.True(t, evalAccessLogFilter(t, filter, map[string]string{
			"x-envoy-original-path": "/orders",
			":path":                 "/orders",
		}), "unrelated path with neither match is logged")
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

// TestTranslator_CreateUpstreamTLSContext_SDSViaADS verifies that, when a
// cert store is configured, the upstream validation context's SDS reference
// rides the existing ADS stream (ConfigSource_Ads) rather than naming a
// dedicated cluster. This means gateway-controller never needs to construct
// a TLS transport socket pointing at cert/key/CA file paths that only exist
// on gateway-runtime's filesystem -- that connection's TLS is entirely
// gateway-runtime's own concern (its bootstrap xds_cluster).
func TestTranslator_CreateUpstreamTLSContext_SDSViaADS(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.DisableSslVerification = false
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)
	// Only t.certStore != nil matters for this code path -- construct one
	// directly rather than routing through NewTranslator's CustomCertsPath
	// init, which calls LoadCertificates against a real db.Storage.
	translator.certStore = certstore.NewCertStore(logger, nil, "", "")

	tlsContext, err := translator.createUpstreamTLSContext(nil, "example.com", nil, "")
	require.NoError(t, err)
	require.NotNil(t, tlsContext)

	combinedCtx := tlsContext.CommonTlsContext.GetCombinedValidationContext()
	require.NotNil(t, combinedCtx)
	sdsConfig := combinedCtx.GetValidationContextSdsSecretConfig().GetSdsConfig()
	require.NotNil(t, sdsConfig)

	ads := sdsConfig.GetAds()
	assert.NotNil(t, ads, "SDS config should ride the ADS stream rather than naming a dedicated cluster")
	assert.Nil(t, sdsConfig.GetApiConfigSource(), "SDS config should not name a dedicated grpc cluster")
}

func TestTranslator_CreateUpstreamTLSContext(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.EcdhCurves = "X25519,P-256"
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	// Test with no certificate
	tlsContext, err := translator.createUpstreamTLSContext(nil, "example.com", nil, "")
	require.NoError(t, err)
	assert.NotNil(t, tlsContext)
	assert.Equal(t, "example.com", tlsContext.Sni)
	assert.Equal(t, []string{"X25519", "P-256"}, tlsContext.CommonTlsContext.TlsParams.EcdhCurves)

	// Test with certificate
	certPem := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	tlsContextWithCert, err := translator.createUpstreamTLSContext(certPem, "secure.example.com", nil, "")
	require.NoError(t, err)
	assert.NotNil(t, tlsContextWithCert)
	assert.Equal(t, "secure.example.com", tlsContextWithCert.Sni)
}

// ============================================================================
// Outbound mTLS: gateway identity + per-upstream trust
// ============================================================================

// assertNoInlineBytesAnywhere asserts no DataSource in the TLS context is
// inline bytes: identity and trust material must arrive via SDS.
func assertNoInlineBytesAnywhere(t *testing.T, tlsCtx *tlsv3.UpstreamTlsContext) {
	t.Helper()
	common := tlsCtx.GetCommonTlsContext()
	for _, tc := range common.GetTlsCertificates() {
		_, isInline := tc.GetCertificateChain().GetSpecifier().(*core.DataSource_InlineBytes)
		assert.False(t, isInline, "TlsCertificates must never carry inline_bytes")
	}
	if vc := common.GetValidationContext(); vc != nil {
		_, isInline := vc.GetTrustedCa().GetSpecifier().(*core.DataSource_InlineBytes)
		assert.False(t, isInline, "ValidationContext.TrustedCa must never carry inline_bytes when a tls block is configured")
	}
	if combined := common.GetCombinedValidationContext(); combined != nil {
		_, isInline := combined.GetDefaultValidationContext().GetTrustedCa().GetSpecifier().(*core.DataSource_InlineBytes)
		assert.False(t, isInline, "CombinedValidationContext.DefaultValidationContext.TrustedCa must never carry inline_bytes")
	}
}

func TestTranslator_CreateUpstreamTLSContext_IdentityAndTrust_VerifyHostNameTrue(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.DisableSslVerification = false // the SAN-matching/validation-context branch is gated on this
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	tlsOpts := &models.UpstreamTLS{
		HasTLSBlock:    true,
		IdentityName:   "out-identity-a",
		TrustedCANames: []string{"out-backend-ca"},
		VerifyHostName: true,
	}
	validationSecretName := UpstreamCAValidationContextSecretName("out-partner-api", "partner-a")

	tlsCtx, err := translator.createUpstreamTLSContext(nil, "mtls-backend-a", tlsOpts, validationSecretName)
	require.NoError(t, err)
	require.NotNil(t, tlsCtx)

	assert.Equal(t, "mtls-backend-a", tlsCtx.Sni, "SNI must be set to the target host")

	// Identity presented via SDS, named gateway_identity:<name>.
	sdsConfigs := tlsCtx.CommonTlsContext.GetTlsCertificateSdsSecretConfigs()
	require.Len(t, sdsConfigs, 1)
	assert.Equal(t, GatewayIdentitySecretName("out-identity-a"), sdsConfigs[0].GetName())

	// Per-upstream trust via the CombinedValidationContext's SDS secret,
	// named upstream_ca:<handle>:<definition>.
	combined := tlsCtx.CommonTlsContext.GetCombinedValidationContext()
	require.NotNil(t, combined)
	assert.Equal(t, validationSecretName, combined.GetValidationContextSdsSecretConfig().GetName())

	// verifyHostName true -> a DNS SAN matcher on the target host.
	sanMatchers := combined.GetDefaultValidationContext().GetMatchTypedSubjectAltNames()
	require.Len(t, sanMatchers, 1)
	assert.Equal(t, tlsv3.SubjectAltNameMatcher_DNS, sanMatchers[0].GetSanType())
	assert.Equal(t, "mtls-backend-a", sanMatchers[0].GetMatcher().GetExact())

	assertNoInlineBytesAnywhere(t, tlsCtx)
}

// An IP-literal target gets an IP_ADDRESS SAN matcher and no SNI.
func TestTranslator_CreateUpstreamTLSContext_IPAddressTarget_UsesIPMatcher(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.DisableSslVerification = false
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)
	// A cert store gives the context a validation context to hold the matcher.
	translator.certStore = certstore.NewCertStore(logger, nil, "", "")

	tlsOpts := &models.UpstreamTLS{HasTLSBlock: true, VerifyHostName: true, TrustedCANames: []string{"partner-ca"}}
	tlsCtx, err := translator.createUpstreamTLSContext(nil, "10.0.0.5", tlsOpts, "upstream_ca:test:partner")
	require.NoError(t, err)

	assert.Empty(t, tlsCtx.Sni, "SNI is not meaningful for an IP-literal target")
	combined := tlsCtx.CommonTlsContext.GetCombinedValidationContext()
	sanMatchers := combined.GetDefaultValidationContext().GetMatchTypedSubjectAltNames()
	require.Len(t, sanMatchers, 1)
	assert.Equal(t, tlsv3.SubjectAltNameMatcher_IP_ADDRESS, sanMatchers[0].GetSanType())
}

// Only a tls block that sets identity or trustedCAs yields a secret ref.
func TestTranslator_CollectUpstreamTLSSecretRefs(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{Handle: "out-partner-api"},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"with-tls": {
				Name: "partner-a",
				TLS: &models.UpstreamTLS{
					Enabled: true, HasTLSBlock: true,
					IdentityName: "out-identity-a", TrustedCANames: []string{"out-backend-ca"}, VerifyHostName: true,
				},
			},
			"no-tls-block": {
				Name: "partner-b",
				TLS:  &models.UpstreamTLS{Enabled: true, HasTLSBlock: false},
			},
			"nil-tls": {
				Name: "partner-c",
			},
			"empty-tls-block": {
				Name: "partner-d",
				TLS:  &models.UpstreamTLS{Enabled: true, HasTLSBlock: true}, // tls: {} — no identity, no trustedCAs
			},
		},
	}

	translator.collectUpstreamTLSSecretRefs(rdc)
	refs := translator.GetUpstreamTLSSecretRefs()

	require.Len(t, refs, 1, "only the definition with an actual identity/trustedCAs should produce a ref, got %+v", refs)
	assert.Equal(t, "out-identity-a", refs[0].IdentityName)
	assert.Equal(t, "out-partner-api", refs[0].APIHandle)
	assert.Equal(t, "partner-a", refs[0].DefinitionName)
	assert.Equal(t, []string{"out-backend-ca"}, refs[0].TrustedCANames)
}

// A certificate store load failure is returned as an error with no
// Translator.
func TestNewTranslator_CertStoreInitFailure_SurfacesAsError(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.CustomCertsPath = t.TempDir()
	routerCfg.Upstream.TLS.TrustedCertPath = "" // no system-cert fallback either
	cfg := testConfig()

	db := &fakeSDSStorage{listErr: fmt.Errorf("boom")}
	translator, err := NewTranslator(logger, routerCfg, db, cfg)

	require.Error(t, err)
	assert.Nil(t, translator, "no translator should be returned alongside a construction error")
}

func TestTranslator_ResolveUpstreamCluster_SimpleURL(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	upstream := &api.Upstream{
		Url: nil, // No URL
	}

	_, _, _, err = translator.resolveUpstreamCluster("no-url-upstream", upstream, nil)
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
			cluster, err := translator.createCluster(tt.clusterNm, parsedURL, tt.certs, nil, nil, "")
			require.NoError(t, err)
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	listener, routeConfig, err := translator.createListener(nil, false, false)
	assert.NoError(t, err)
	assert.NotNil(t, listener)
	assert.NotNil(t, routeConfig)
	assert.Contains(t, listener.Name, "8080")
	assert.Equal(t, uint32(1048576), listener.GetPerConnectionBufferLimitBytes().GetValue())
}

func TestTranslator_CreateListener_PerConnectionBufferLimitBytes(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.HTTPListener.PerConnectionBufferLimitBytes = 2097152
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	listener, _, err := translator.createListener(nil, false, false)
	assert.NoError(t, err)
	assert.NotNil(t, listener)
	assert.Equal(t, uint32(2097152), listener.GetPerConnectionBufferLimitBytes().GetValue())
}

// The listener certificate is referenced via SDS and never inlined.
func TestTranslator_CreateDownstreamTLSContext_ListenerCertViaSDS(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	tlsContext, err := translator.createDownstreamTLSContext(false)
	require.NoError(t, err)
	require.NotNil(t, tlsContext)

	sdsConfigs := tlsContext.CommonTlsContext.GetTlsCertificateSdsSecretConfigs()
	require.Len(t, sdsConfigs, 1)
	assert.Equal(t, SecretNameDownstreamListenerCert, sdsConfigs[0].GetName())
	assert.Empty(t, tlsContext.CommonTlsContext.GetTlsCertificates(),
		"the listener certificate/key must never be inlined into the LDS resource")
}

// A listener that requests a client certificate never resumes a session, so
// every connection presents its certificate to mtls-auth; one that does not
// request a certificate keeps resumption.
func TestTranslator_CreateDownstreamTLSContext_SessionResumptionOnlyWithoutClientCertificate(t *testing.T) {
	translator, err := NewTranslator(createTestLogger(), testRouterConfig(), nil, testConfig())
	require.NoError(t, err)

	requesting, err := translator.createDownstreamTLSContext(true)
	require.NoError(t, err)
	assert.True(t, requesting.GetDisableStatelessSessionResumption())
	assert.True(t, requesting.GetDisableStatefulSessionResumption())

	plain, err := translator.createDownstreamTLSContext(false)
	require.NoError(t, err)
	assert.False(t, plain.GetDisableStatelessSessionResumption())
	assert.False(t, plain.GetDisableStatefulSessionResumption())
}

func TestTranslator_CreateRoute_Basic(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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

	// Guards the http.route tracing fix: the route's path template must be recorded as
	// route metadata so createTracingCustomTags' http.route tag can read it (see
	// setRouteHTTPRoute).
	require.NotNil(t, route.Metadata)
	assert.Equal(t, "/api/users",
		route.Metadata.FilterMetadata["wso2.route"].Fields["http.route"].GetStringValue())
}

// TestTranslator_CreateRouteFromRDC_HTTPRouteMetadata guards the http.route tracing fix:
// createRouteFromRDC must record the route's full path *template* (not a concrete matched
// path) as wso2.route/http.route metadata, so the HCM tracing http.route custom tag
// (createTracingCustomTags) can surface it. Without this, APM backends that derive a
// span's resource/display name from "<http.method> <http.route>" fall back to the bare
// HTTP method.
func TestTranslator_CreateRouteFromRDC_HTTPRouteMetadata(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	rdc := &models.RuntimeDeployConfig{
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"main": {Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
		},
	}
	rdcRoute := &models.Route{
		Method:          "GET",
		Path:            "/pets/v1.0/{id}",
		OperationPath:   "/{id}",
		AutoHostRewrite: true,
		Upstream:        models.RouteUpstream{ClusterKey: "main"},
	}

	r := translator.createRouteFromRDC("GET|/pets/v1.0/{id}|", rdcRoute, rdc)
	require.NotNil(t, r)
	require.NotNil(t, r.Metadata)

	// The templated path, not a concrete request path, must be stored.
	assert.Equal(t, "/pets/v1.0/{id}",
		r.Metadata.FilterMetadata["wso2.route"].Fields["http.route"].GetStringValue())
}

// TestTranslator_CreateRoute_DynamicRouting pins the cluster specifier createRoute emits:
// a static cluster when useClusterHeader is false, and cluster_header routing (with the
// x-target-upstream header stripped before forwarding) when it is true. This is the
// legacy-xDS half of the sandbox dynamic-endpoint fix.
func TestTranslator_CreateRoute_DynamicRouting(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

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

		// setRouteHTTPRoute must merge into the same wso2.route namespace rather than
		// overwrite it — pre-existing keys and the new http.route key must both survive.
		assert.Equal(t, "api-123", metadata.Fields["api_id"].GetStringValue())
		assert.Equal(t, "project-123", metadata.Fields["project_id"].GetStringValue())
		assert.Equal(t, "/test/channel1", metadata.Fields["http.route"].GetStringValue())
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
		assert.Equal(t, "/test/v1.0.0/channel1",
			route.Metadata.FilterMetadata["wso2.route"].Fields["http.route"].GetStringValue())
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
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	if err != nil {
		panic(err)
	}
	return translator
}

// TestProcessEndpoint_PeerHostnameMetadata guards the fix for GH issue #2883 (Datadog
// Dependencies view needs a peer.service tag on upstream CLIENT spans). processEndpoint must
// stamp the upstream hostname under the envoy.lb/hostname metadata key so the peer.service
// custom tag (createTracingCustomTags) can read it, for both plaintext and HTTPS
// endpoints — and, for HTTPS, alongside (not instead of) the existing
// envoy.transport_socket_match/lb_id metadata that TLS transport-socket matching depends on.
func TestProcessEndpoint_PeerHostnameMetadata(t *testing.T) {
	translator := createTestTranslator()

	t.Run("http endpoint gets peer hostname metadata, no transport socket metadata", func(t *testing.T) {
		u, err := url.Parse("http://backend.default.svc.cluster.local:8080/v1")
		require.NoError(t, err)

		endpoints, tsm, err := translator.processEndpoint(u, nil, nil, "")
		require.NoError(t, err)
		require.Len(t, endpoints, 1)
		lb := endpoints[0].GetLbEndpoints()[0]

		assert.Nil(t, tsm, "plaintext endpoint must not produce a transport socket match")
		md := lb.GetMetadata().GetFilterMetadata()
		assert.Equal(t, "backend.default.svc.cluster.local", md["envoy.lb"].GetFields()["hostname"].GetStringValue())
		assert.Nil(t, md["envoy.transport_socket_match"], "plaintext endpoint must not carry transport-socket metadata")
	})

	t.Run("https endpoint gets peer hostname metadata alongside transport socket metadata", func(t *testing.T) {
		u, err := url.Parse("https://api.example.com/v1")
		require.NoError(t, err)

		endpoints, tsm, err := translator.processEndpoint(u, nil, nil, "")
		require.NoError(t, err)
		require.Len(t, endpoints, 1)
		lb := endpoints[0].GetLbEndpoints()[0]

		require.NotNil(t, tsm, "https endpoint must produce a transport socket match")
		md := lb.GetMetadata().GetFilterMetadata()
		assert.Equal(t, "api.example.com", md["envoy.lb"].GetFields()["hostname"].GetStringValue())
		require.NotNil(t, md["envoy.transport_socket_match"],
			"https endpoint must still carry transport-socket metadata alongside peer hostname metadata")
		assert.Equal(t, constants.DefaultMatchID, md["envoy.transport_socket_match"].GetFields()["lb_id"].GetStringValue())
	})
}

// TestCreateWeightedCluster_TLS guards the fix for the reviewer concern
// "Configure TLS for weighted HTTPS upstreams." A multi-endpoint (weighted) upstream
// definition whose endpoints are HTTPS must be dialed over TLS, mirroring the single-endpoint
// createCluster path. Before the fix createWeightedCluster discarded the scheme and produced a
// plain cluster with no transport socket, silently downgrading HTTPS weighted upstreams to
// plaintext.
func TestCreateWeightedCluster_TLS(t *testing.T) {
	logger := createTestLogger()
	translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
	require.NoError(t, err)

	w := func(n int) *int { return &n }
	endpoints := []models.Endpoint{
		{Host: "a.example.com", Port: 443, Weight: w(70)},
		{Host: "b.example.com", Port: 443, Weight: w(30)},
	}

	t.Run("https weighted upstream gets per-endpoint TLS transport sockets", func(t *testing.T) {
		c, err := translator.createWeightedCluster("upstream_secure", endpoints, &models.UpstreamTLS{Enabled: true}, nil, "")
		require.NoError(t, err)
		require.NotNil(t, c)

		// One transport socket match per endpoint, each carrying a TLS transport socket.
		require.Len(t, c.GetTransportSocketMatches(), len(endpoints),
			"each HTTPS endpoint must get its own transport socket match")
		for i, tsm := range c.GetTransportSocketMatches() {
			matchID := strconv.Itoa(i)
			assert.Equal(t, "ts"+matchID, tsm.GetName())
			assert.Equal(t, matchID, tsm.GetMatch().GetFields()["lb_id"].GetStringValue())
			require.NotNil(t, tsm.GetTransportSocket())
			assert.Equal(t, "envoy.transport_sockets.tls", tsm.GetTransportSocket().GetName())

			// The transport socket must hold an UpstreamTlsContext with the endpoint's host as SNI.
			tc := &tlsv3.UpstreamTlsContext{}
			require.NoError(t, tsm.GetTransportSocket().GetTypedConfig().UnmarshalTo(tc))
			assert.Equal(t, endpoints[i].Host, tc.GetSni(),
				"each endpoint's TLS context must use its own hostname as SNI")
		}

		// Every LbEndpoint must be tagged with the matching lb_id so Envoy selects its socket.
		lbs := c.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints()
		require.Len(t, lbs, len(endpoints))
		for i, lb := range lbs {
			md := lb.GetMetadata().GetFilterMetadata()["envoy.transport_socket_match"]
			require.NotNil(t, md, "HTTPS endpoint must carry transport_socket_match metadata")
			assert.Equal(t, strconv.Itoa(i), md.GetFields()["lb_id"].GetStringValue())
		}
	})

	t.Run("plaintext weighted upstream is unchanged (no transport socket)", func(t *testing.T) {
		plain := []models.Endpoint{
			{Host: "a.internal", Port: 8080, Weight: w(1)},
			{Host: "b.internal", Port: 8080, Weight: w(1)},
		}
		// Both nil TLS and explicitly-disabled TLS must produce a plain cluster.
		for _, tls := range []*models.UpstreamTLS{nil, {Enabled: false}} {
			c, err := translator.createWeightedCluster("upstream_plain", plain, tls, nil, "")
			require.NoError(t, err)
			require.NotNil(t, c)
			assert.Empty(t, c.GetTransportSocketMatches(),
				"plaintext weighted upstream must not get transport socket matches")
			for i, lb := range c.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints() {
				assert.Nil(t, lb.GetMetadata().GetFilterMetadata()["envoy.transport_socket_match"],
					"plaintext endpoint must not carry transport-socket metadata")
				// peer.service hostname metadata is independent of TLS and must still be present.
				assert.Equal(t, plain[i].Host,
					lb.GetMetadata().GetFilterMetadata()["envoy.lb"].GetFields()["hostname"].GetStringValue())
			}
		}
	})
}

// TestCreateWeightedCluster_PeerHostnameMetadata guards the multi-endpoint half of the fix for
// GH issue #2883: a weighted (multi-endpoint) upstream cluster is exactly as common as a
// single-endpoint one (round-robin/failover across distinct backends), so every LbEndpoint
// createWeightedCluster produces must carry its own envoy.lb/hostname metadata — not just the
// single-endpoint processEndpoint path. Before this fix, any upstream with more than one
// endpoint had no peer.service tracing tag whatsoever.
func TestCreateWeightedCluster_PeerHostnameMetadata(t *testing.T) {
	translator := createTestTranslator()
	w := func(n int) *int { return &n }
	endpoints := []models.Endpoint{
		{Host: "a.example.com", Port: 443, Weight: w(70)},
		{Host: "b.example.com", Port: 443, Weight: w(30)},
	}

	t.Run("each endpoint carries its own hostname, distinct from its siblings", func(t *testing.T) {
		c, err := translator.createWeightedCluster("upstream_secure", endpoints, &models.UpstreamTLS{Enabled: true}, nil, "")
		require.NoError(t, err)
		require.NotNil(t, c)

		lbs := c.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints()
		require.Len(t, lbs, len(endpoints))
		for i, lb := range lbs {
			md := lb.GetMetadata().GetFilterMetadata()
			assert.Equal(t, endpoints[i].Host, md["envoy.lb"].GetFields()["hostname"].GetStringValue(),
				"endpoint %d must carry its own hostname as peer.service metadata", i)
			require.NotNil(t, md["envoy.transport_socket_match"])
			assert.Equal(t, strconv.Itoa(i), md["envoy.transport_socket_match"].GetFields()["lb_id"].GetStringValue())
		}
	})

	t.Run("plaintext weighted endpoints still get hostname metadata", func(t *testing.T) {
		c, err := translator.createWeightedCluster("upstream_plain", endpoints, nil, nil, "")
		require.NoError(t, err)
		require.NotNil(t, c)

		lbs := c.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints()
		require.Len(t, lbs, len(endpoints))
		for i, lb := range lbs {
			assert.Equal(t, endpoints[i].Host,
				lb.GetMetadata().GetFilterMetadata()["envoy.lb"].GetFields()["hostname"].GetStringValue())
		}
	})
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

// TestBuildMatchHeaders_HeaderMatchersRendered guards that configured header matches are rendered
// as Envoy header matchers on the route (the mechanism that makes header-based route selection and
// cross-HTTPRoute precedence work), and that a RegularExpression match becomes a safe_regex rather
// than being downgraded to an exact match.
func TestBuildMatchHeaders_HeaderMatchersRendered(t *testing.T) {
	logger := createTestLogger()
	translator, err := NewTranslator(logger, testRouterConfig(), nil, testConfig())
	require.NoError(t, err)
	rdc := &models.RuntimeDeployConfig{
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"main": {BasePath: "", Endpoints: []models.Endpoint{{Host: "echo", Port: 80}}},
		},
	}

	route1 := &models.Route{
		Method: "GET", Path: "/svc/v1/things", OperationPath: "/things",
		Upstream: models.RouteUpstream{ClusterKey: "main"},
		MatchHeaders: []models.RouteHeaderMatch{
			{Name: "Version", Type: "Exact", Value: "two"},
			{Name: "X-Flavor", Type: "RegularExpression", Value: "red|blue"},
		},
	}
	r := translator.createRouteFromRDC("GET|/svc/v1/things|main.local|abc123", route1, rdc)
	require.NotNil(t, r)

	var version, flavor *route.HeaderMatcher
	for _, h := range r.GetMatch().GetHeaders() {
		switch h.GetName() {
		case "version":
			version = h
		case "x-flavor":
			flavor = h
		}
	}
	require.NotNil(t, version, "expected a lower-cased 'version' header matcher")
	_, exactOK := version.GetHeaderMatchSpecifier().(*route.HeaderMatcher_StringMatch)
	require.True(t, exactOK, "Exact header match must be a string_match, got %T", version.GetHeaderMatchSpecifier())

	require.NotNil(t, flavor, "expected an 'x-flavor' header matcher")
	rx, ok := flavor.GetHeaderMatchSpecifier().(*route.HeaderMatcher_SafeRegexMatch)
	require.True(t, ok, "RegularExpression header match must stay a safe_regex, got %T", flavor.GetHeaderMatchSpecifier())
	assert.Equal(t, "red|blue", rx.SafeRegexMatch.GetRegex())
}

// TestTranslator_TranslateRuntimeConfig_AppliesConnectTimeout is the translator half of the
// connect-timeout regression guard: translateRuntimeConfig must apply an UpstreamCluster's
// ConnectTimeout to the emitted Envoy cluster, and fall back to the router's global default
// (5s here) when it is nil — rather than dropping the per-upstream value.
func TestTranslator_TranslateRuntimeConfig_AppliesConnectTimeout(t *testing.T) {
	translator := createTestTranslator()
	d := 8 * time.Second
	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: "u", Kind: "RestApi"},
		Routes:   map[string]*models.Route{},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"with_timeout": {
				BasePath:       "/",
				Endpoints:      []models.Endpoint{{Host: "backend", Port: 8080}},
				TLS:            &models.UpstreamTLS{},
				ConnectTimeout: &d,
			},
			"without_timeout": {
				BasePath:  "/",
				Endpoints: []models.Endpoint{{Host: "backend2", Port: 8080}},
				TLS:       &models.UpstreamTLS{},
			},
		},
	}

	_, clusters, err := translator.translateRuntimeConfig(rdc)
	require.NoError(t, err)

	got := map[string]time.Duration{}
	for _, c := range clusters {
		got[c.GetName()] = c.GetConnectTimeout().AsDuration()
	}
	assert.Equal(t, 8*time.Second, got["with_timeout"],
		"an explicit per-upstream connect timeout must be applied to the Envoy cluster")
	assert.Equal(t, 5*time.Second, got["without_timeout"],
		"a nil connect timeout must fall back to the router global default (5s), not be dropped to zero")
}

// TestTranslateRuntimeConfig_PeerHostnameOnEveryEndpoint is the end-to-end invariant guard for
// GH issue #2883: translateRuntimeConfig builds clusters via two different endpoint-construction
// paths depending on endpoint count (createCluster/processEndpoint for one endpoint,
// createWeightedCluster for more than one) — every LbEndpoint produced by either path must carry
// non-empty envoy.lb/hostname metadata. This is the test that keeps gap #1 (multi-endpoint
// upstreams silently getting no peer.service tag) from being reintroduced if a third
// endpoint-building path is ever added without also calling setEndpointPeerHostname.
func TestTranslateRuntimeConfig_PeerHostnameOnEveryEndpoint(t *testing.T) {
	translator := createTestTranslator()
	w := func(n int) *int { return &n }
	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: "u", Kind: "RestApi"},
		Routes:   map[string]*models.Route{},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"single": {
				BasePath:  "/",
				Endpoints: []models.Endpoint{{Host: "solo.example.com", Port: 8080}},
			},
			"weighted": {
				BasePath: "/",
				Endpoints: []models.Endpoint{
					{Host: "primary.example.com", Port: 8080, Weight: w(80)},
					{Host: "secondary.example.com", Port: 8080, Weight: w(15)},
					{Host: "tertiary.example.com", Port: 8080, Weight: w(5)},
				},
			},
		},
	}

	_, clusters, err := translator.translateRuntimeConfig(rdc)
	require.NoError(t, err)
	require.Len(t, clusters, 2)

	checked := 0
	for _, c := range clusters {
		for _, localityLb := range c.GetLoadAssignment().GetEndpoints() {
			for _, lb := range localityLb.GetLbEndpoints() {
				hostname := lb.GetMetadata().GetFilterMetadata()["envoy.lb"].GetFields()["hostname"].GetStringValue()
				assert.NotEmpty(t, hostname,
					"cluster %q: every LbEndpoint must carry a non-empty peer.service hostname", c.GetName())
				checked++
			}
		}
	}
	assert.Equal(t, 4, checked, "expected to have checked all 4 endpoints across both clusters (1 + 3)")
}

// makeRestAPIWithOperationLevelMTLSAuth is makeRestAPI with mtls-auth on its
// one operation.
func makeRestAPIWithOperationLevelMTLSAuth(uuid, name, ctx string) *models.StoredConfig {
	cfg := api.RestAPI{
		Kind:     api.RestAPIKindRestApi,
		Metadata: api.Metadata{Name: name},
		Spec: api.APIConfigData{
			DisplayName: name,
			Version:     "v1.0",
			Context:     ctx,
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: api.Ptr("http://backend:8080")},
			},
			Operations: []api.Operation{
				{
					Method: api.Ptr(api.OperationMethodGET),
					Path:   api.Ptr("/resource"),
					Policies: &[]api.Policy{
						{Name: "mtls-auth", Version: "v1"},
					},
				},
			},
		},
	}
	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                models.KindRestApi,
		Handle:              name,
		DisplayName:         name,
		Version:             "v1.0",
		DesiredState:        models.StateDeployed,
		Configuration:       cfg,
		SourceConfiguration: cfg,
	}
}

func findListenerByPort(t *testing.T, resources []types.Resource, port int) *listener.Listener {
	t.Helper()
	for _, res := range resources {
		l, ok := res.(*listener.Listener)
		if !ok {
			continue
		}
		if l.GetAddress().GetSocketAddress().GetPortValue() == uint32(port) {
			return l
		}
	}
	t.Fatalf("no listener found bound to port %d", port)
	return nil
}

func extractDownstreamTLSContext(t *testing.T, l *listener.Listener) *tlsv3.DownstreamTlsContext {
	t.Helper()
	require.Len(t, l.FilterChains, 1)
	typedConfig := l.FilterChains[0].GetTransportSocket().GetTypedConfig()
	require.NotNil(t, typedConfig, "listener %q has no transport socket configured", l.GetName())
	var tlsCtx tlsv3.DownstreamTlsContext
	require.NoError(t, typedConfig.UnmarshalTo(&tlsCtx))
	return &tlsCtx
}

// An API with operation-level mtls-auth and a non-empty client-CA pool make
// the HTTPS listener request a client certificate against that pool.
func TestTranslator_TranslateConfigs_HTTPSListener_MTLSAuthAttached_RequiresClientCA(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.HTTPSEnabled = true
	routerCfg.HTTPSPort = 8443
	cfg := testConfig()
	cfg.Router = *routerCfg
	clientCA := pki.NewRootCA(t, "Listener Client CA")
	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{UUID: "client-1", Name: "client-ca", Certificate: clientCA.PEM(), Usage: models.CertificateUsageDownstream},
	}}
	translator, err := NewTranslator(logger, routerCfg, db, cfg)
	require.NoError(t, err)

	configs := []*models.StoredConfig{makeRestAPIWithOperationLevelMTLSAuth("uuid-mtls-1", "mtls-api", "/mtls-api")}
	resources, err := translator.TranslateConfigs(configs, "test-correlation-id")
	require.NoError(t, err)

	httpsListener := findListenerByPort(t, resources[resource.ListenerType], routerCfg.HTTPSPort)
	tlsCtx := extractDownstreamTLSContext(t, httpsListener)

	require.NotNil(t, tlsCtx.CommonTlsContext.GetValidationContextSdsSecretConfig())
	assert.Equal(t, SecretNameDownstreamClientCA, tlsCtx.CommonTlsContext.GetValidationContextSdsSecretConfig().GetName())
	require.NotNil(t, tlsCtx.RequireClientCertificate)
	assert.False(t, tlsCtx.RequireClientCertificate.GetValue())
}

// With mtls-auth attached but an empty client-CA pool, the HTTPS listener
// names no downstream_client_ca secret, so it never waits on a secret that is
// not served; mtls-auth then denies for lack of a certificate.
func TestTranslator_TranslateConfigs_HTTPSListener_MTLSAuthAttached_EmptyPool_NoClientCA(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.HTTPSEnabled = true
	routerCfg.HTTPSPort = 8443
	cfg := testConfig()
	cfg.Router = *routerCfg
	upstreamCA := pki.NewRootCA(t, "Listener Upstream CA")
	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{UUID: "upstream-1", Name: "upstream-ca", Certificate: upstreamCA.PEM(), Usage: models.CertificateUsageUpstream},
	}}
	translator, err := NewTranslator(logger, routerCfg, db, cfg)
	require.NoError(t, err)

	configs := []*models.StoredConfig{makeRestAPIWithOperationLevelMTLSAuth("uuid-mtls-1", "mtls-api", "/mtls-api")}
	resources, err := translator.TranslateConfigs(configs, "test-correlation-id")
	require.NoError(t, err)

	httpsListener := findListenerByPort(t, resources[resource.ListenerType], routerCfg.HTTPSPort)
	tlsCtx := extractDownstreamTLSContext(t, httpsListener)

	assert.Nil(t, tlsCtx.CommonTlsContext.GetValidationContextType())
	assert.Nil(t, tlsCtx.RequireClientCertificate)
	assert.False(t, SnapshotReferencesSDSSecret(nil, []types.Resource{httpsListener}, SecretNameDownstreamClientCA))
}

// With no mtls-auth deployed, the HTTPS listener carries no client-CA
// validation context.
func TestTranslator_TranslateConfigs_HTTPSListener_NoMTLSAuth_NoClientCA(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.HTTPSEnabled = true
	routerCfg.HTTPSPort = 8443
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	configs := []*models.StoredConfig{makeRestAPI("uuid-plain-1", "plain-api", "/plain-api")}
	resources, err := translator.TranslateConfigs(configs, "test-correlation-id")
	require.NoError(t, err)

	httpsListener := findListenerByPort(t, resources[resource.ListenerType], routerCfg.HTTPSPort)
	tlsCtx := extractDownstreamTLSContext(t, httpsListener)

	assert.Nil(t, tlsCtx.CommonTlsContext.GetValidationContextType())
	assert.Nil(t, tlsCtx.RequireClientCertificate)
}

// TestSnapshotReferencesSDSSecret covers listener validation context,
// listener certificate and cluster trust references, and an unreferenced
// name.
func TestSnapshotReferencesSDSSecret(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.HTTPSEnabled = true
	routerCfg.HTTPSPort = 8443
	routerCfg.Upstream.TLS.DisableSslVerification = false
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)
	// A non-nil cert store is all the SDS path needs.
	translator.certStore = certstore.NewCertStore(logger, nil, "", "")

	httpsListener, _, err := translator.createListener(nil, true, true)
	require.NoError(t, err)
	listeners := []types.Resource{httpsListener}

	weightedCluster, err := translator.createWeightedCluster(
		"upstream-cluster",
		[]models.Endpoint{{Host: "backend.example.com", Port: 8443}},
		&models.UpstreamTLS{Enabled: true},
		nil,
		"",
	)
	require.NoError(t, err)
	clusters := []types.Resource{weightedCluster}

	assert.True(t, SnapshotReferencesSDSSecret(nil, listeners, SecretNameDownstreamClientCA),
		"a listener's ValidationContextSdsSecretConfig must be recognised")
	assert.True(t, SnapshotReferencesSDSSecret(nil, listeners, SecretNameDownstreamListenerCert),
		"a listener's TlsCertificateSdsSecretConfigs entry must be recognised")
	assert.True(t, SnapshotReferencesSDSSecret(clusters, nil, SecretNameUpstreamCA),
		"a cluster's CombinedValidationContext must be recognised")
	assert.False(t, SnapshotReferencesSDSSecret(clusters, listeners, "some-unreferenced-secret"),
		"a secret name referenced by nothing in the snapshot must report false")
}

// The HCM uses SANITIZE_SET, so a forged x-forwarded-client-cert never
// survives, and sets every SetCurrentClientCertDetails flag.
func TestTranslator_CreateListener_ForwardClientCertDetails(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	lis, _, err := translator.createListener(nil, false, false)
	require.NoError(t, err)

	manager := extractHCM(t, lis)
	assert.Equal(t, hcm.HttpConnectionManager_SANITIZE_SET, manager.GetForwardClientCertDetails())

	details := manager.GetSetCurrentClientCertDetails()
	require.NotNil(t, details, "set_current_client_cert_details must be explicitly configured")
	require.NotNil(t, details.GetSubject(), "subject must be explicitly set (not left as an unset wrapper)")
	assert.True(t, details.GetSubject().GetValue())
	assert.True(t, details.GetCert())
	assert.True(t, details.GetChain())
	assert.True(t, details.GetUri())
	assert.True(t, details.GetDns())
}

// A route without mtls-auth strips x-forwarded-client-cert before its
// backend; a route with it keeps the header.
func TestTranslator_CreateRouteFromRDC_StripsXFCCHeader_UnlessChainAttachesMTLSAuth(t *testing.T) {
	translator := createTestTranslator()

	// Hand-built because importing the transform package here would be an
	// import cycle.
	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: "u", Kind: "RestApi"},
		Routes: map[string]*models.Route{
			"plain-route": {
				Method: "GET", Path: "/plain-api/resource", OperationPath: "/resource",
				Upstream: models.RouteUpstream{ClusterKey: "backend"},
			},
			"mtls-route": {
				Method: "GET", Path: "/mtls-api/resource", OperationPath: "/resource",
				Upstream: models.RouteUpstream{ClusterKey: "backend"},
			},
		},
		PolicyChains: map[string]*models.PolicyChain{
			"mtls-route": {Policies: []models.Policy{{Name: "mtls-auth", Version: "v1.0.0"}}},
			// "plain-route" has no chain at all.
		},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"backend": {BasePath: "/", Endpoints: []models.Endpoint{{Host: "backend.example.com", Port: 8080}}},
		},
	}

	routes, _, err := translator.translateRuntimeConfig(rdc)
	require.NoError(t, err)

	var plainRoute, mtlsRoute *route.Route
	for _, r := range routes {
		switch r.GetName() {
		case "plain-route":
			plainRoute = r
		case "mtls-route":
			mtlsRoute = r
		}
	}
	require.NotNil(t, plainRoute, "expected to find the plain (no mtls-auth) route")
	require.NotNil(t, mtlsRoute, "expected to find the mtls-auth route")

	assert.Contains(t, plainRoute.RequestHeadersToRemove, xfccHeaderName,
		"a route whose chain lacks mtls-auth must strip x-forwarded-client-cert before its backend")
	assert.NotContains(t, mtlsRoute.RequestHeadersToRemove, xfccHeaderName,
		"a route whose chain attaches mtls-auth must not strip x-forwarded-client-cert")
}

// mtlsHeaderStrippingRDC builds a plain route and an mtls-auth route.
func mtlsHeaderStrippingRDC() *models.RuntimeDeployConfig {
	return &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: "u", Kind: "RestApi"},
		Routes: map[string]*models.Route{
			"plain-route": {
				Method: "GET", Path: "/plain-api/resource", OperationPath: "/resource",
				Upstream: models.RouteUpstream{ClusterKey: "backend"},
			},
			"mtls-route": {
				Method: "GET", Path: "/mtls-api/resource", OperationPath: "/resource",
				Upstream: models.RouteUpstream{ClusterKey: "backend"},
			},
		},
		PolicyChains: map[string]*models.PolicyChain{
			"mtls-route": {Policies: []models.Policy{{Name: "mtls-auth", Version: "v1.0.0"}}},
		},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"backend": {BasePath: "/", Endpoints: []models.Endpoint{{Host: "backend.example.com", Port: 8080}}},
		},
	}
}

func findRoutesByName(t *testing.T, routes []*route.Route) (plainRoute, mtlsRoute *route.Route) {
	t.Helper()
	for _, r := range routes {
		switch r.GetName() {
		case "plain-route":
			plainRoute = r
		case "mtls-route":
			mtlsRoute = r
		}
	}
	require.NotNil(t, plainRoute, "expected to find the plain (no mtls-auth) route")
	require.NotNil(t, mtlsRoute, "expected to find the mtls-auth route")
	return plainRoute, mtlsRoute
}

// A route without mtls-auth always strips the relayed header; on a route
// with it the policy decides, so the router leaves it in place.
func TestTranslator_CreateRouteFromRDC_StripsRelayedCertificateHeader_UnlessChainAttachesMTLSAuth(t *testing.T) {
	t.Run("stripped on the plain route only", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.DownstreamTLS.ClientCertificateHeader = config.ClientCertificateHeader{Name: "X-WSO2-CLIENT-CERTIFICATE"}

		routes, _, err := translator.translateRuntimeConfig(mtlsHeaderStrippingRDC())
		require.NoError(t, err)
		plainRoute, mtlsRoute := findRoutesByName(t, routes)

		assert.Contains(t, plainRoute.RequestHeadersToRemove, "x-wso2-client-certificate",
			"a route whose chain lacks mtls-auth must strip the relayed-certificate header before its backend")
		assert.NotContains(t, mtlsRoute.RequestHeadersToRemove, "x-wso2-client-certificate",
			"a route whose chain attaches mtls-auth leaves the relayed-certificate header to the policy")
	})

	t.Run("header name is lower-cased before being added to RequestHeadersToRemove", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.DownstreamTLS.ClientCertificateHeader = config.ClientCertificateHeader{Name: "X-Amzn-Mtls-Clientcert"}

		routes, _, err := translator.translateRuntimeConfig(mtlsHeaderStrippingRDC())
		require.NoError(t, err)
		plainRoute, _ := findRoutesByName(t, routes)

		assert.Contains(t, plainRoute.RequestHeadersToRemove, "x-amzn-mtls-clientcert")
		assert.NotContains(t, plainRoute.RequestHeadersToRemove, "X-Amzn-Mtls-Clientcert",
			"the configured header name must be lower-cased, not added as-authored")
	})
}

// Routes that never carry a policy chain evaluating the client-certificate
// headers strip both headers before their backend.
func TestTranslator_RoutesWithoutPolicyChain_StripClientCertificateHeaders(t *testing.T) {
	translator := createTestTranslator()
	translator.routerConfig.DownstreamTLS.ClientCertificateHeader = config.ClientCertificateHeader{Name: "X-WSO2-CLIENT-CERTIFICATE"}

	topicRoute := translator.createRoutePerTopic("api-123", "Test API", "v1.0.0", "/test", "POST", "/channel1", "test-cluster", "localhost", "WebSubApi", "project-123")
	legacyRoute := translator.createRoute(
		"test-id", "TestAPI", "v1.0", "/weather/v1.0",
		"GET", "/forecast", "test-cluster", "/",
		"localhost", "http/rest", "", "", nil, "", nil,
		false, nil,
	)

	for name, r := range map[string]*route.Route{"websub topic route": topicRoute, "legacy route": legacyRoute} {
		require.NotNil(t, r, name)
		assert.Contains(t, r.RequestHeadersToRemove, xfccHeaderName, "%s must strip the forwarded-certificate header", name)
		assert.Contains(t, r.RequestHeadersToRemove, "x-wso2-client-certificate", "%s must strip the relayed-certificate header", name)
	}
}
