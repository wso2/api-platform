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

// This file is an external test package because it drives the Agent transformer
// through the Envoy xDS translator, and the translator's own package imports
// transform. Route ordering is not a property of the RDC — it is decided when
// the RDC becomes Envoy routes — so this is the only layer at which it can be
// asserted end to end.
package transform_test

import (
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

func routingTestRouterConfig() *config.RouterConfig {
	return &config.RouterConfig{
		ListenerPort: 8080,
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "agents.example.com"},
			Sandbox: config.VHostEntry{Default: "sandbox.example.com"},
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
		AccessLogs: config.AccessLogsConfig{Enabled: false},
		HTTPListener: config.HTTPListenerConfig{
			ServerHeaderTransformation:    commonconstants.OVERWRITE,
			PerConnectionBufferLimitBytes: 1048576,
			PathWithEscapedSlashesAction:  commonconstants.KEEP_UNCHANGED,
		},
		LuaScriptPath: "../../lua/request_transformation.lua",
	}
}

// routingTestPolicyDefinitions is the catalogue these tests translate against:
// the A2A system policy and nothing else. A managed card cannot be served
// without it, and every fixture here starts with one.
func routingTestPolicyDefinitions() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		constants.A2A_SYSTEM_POLICY_NAME + "_v1.0.0": {
			Name:    constants.A2A_SYSTEM_POLICY_NAME,
			Version: "v1.0.0",
		},
	}
}

// routingTestAgent is an Agent exposing HTTP+JSON at the context root, which is
// the layout where the 1.0 binding table's overlapping paths all appear at once.
func routingTestAgent(options ...func(*api.AgentConfiguration)) *models.StoredConfig {
	upstreamURL := "https://weather.internal"
	context := "/weather"
	cardContent := api.A2AAgentCardDocument{"name": "Weather Agent", "protocolVersion": "1.0"}
	stored := &models.StoredConfig{
		UUID:   "agent-routing-1",
		Kind:   models.KindAgent,
		Handle: "weather-agent",
		Configuration: api.AgentConfiguration{
			ApiVersion: api.AgentConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.AgentConfigurationKindAgent,
			Metadata:   api.Metadata{Name: "weather-agent-v1-0"},
			Spec: api.AgentConfigData{
				DisplayName: "Weather Agent",
				Version:     "v1.0",
				Context:     &context,
				Upstream:    api.AgentConfigData_Upstream{Url: &upstreamURL},
				A2a: api.A2AConfig{
					ProtocolVersion: "1.0",
					OperationConfigs: api.A2AOperationConfigs{
						Transports: []api.A2ATransport{
							{ProtocolBinding: api.HTTPJSON},
						},
					},
					AgentCard: api.A2AAgentCard{
						Public: api.A2APublicAgentCard{
							Mode:    api.A2APublicAgentCardModeManaged,
							Content: &cardContent,
						},
					},
				},
			},
		},
	}
	cfg := stored.Configuration.(api.AgentConfiguration)
	for _, option := range options {
		option(&cfg)
	}
	stored.Configuration = cfg
	return stored
}

// agentEnvoyRoutes translates the Agent through the real xDS path and returns
// the routes of its own virtual host, in the order Envoy will evaluate them.
func agentEnvoyRoutes(t *testing.T, stored *models.StoredConfig) []*route.Route {
	t.Helper()

	routerCfg := routingTestRouterConfig()
	systemCfg := &config.Config{Router: *routerCfg}
	translator := xds.NewTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)), routerCfg, nil, systemCfg)
	registry := transform.NewRegistry(
		transform.NewRestAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}),
		nil,
		transform.NewAgentTransformer(routerCfg, systemCfg, routingTestPolicyDefinitions()),
	)
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindAgent: registry})

	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err)

	for _, res := range resources[resource.RouteType] {
		routeConfig, ok := res.(*route.RouteConfiguration)
		require.True(t, ok)
		for _, virtualHost := range routeConfig.VirtualHosts {
			if virtualHost.Name == "agents.example.com" {
				return virtualHost.Routes
			}
		}
	}
	t.Fatal("the agent's virtual host is missing from the translated routes")
	return nil
}

// indexOfRoute returns the position of a route by name, failing the test if it
// is absent — an absent route makes an ordering assertion vacuously pass.
func indexOfRoute(t *testing.T, routes []*route.Route, name string) int {
	t.Helper()
	for i, r := range routes {
		if r.GetName() == name {
			return i
		}
	}
	names := make([]string, 0, len(routes))
	for _, r := range routes {
		names = append(names, r.GetName())
	}
	t.Fatalf("route %q not found; have %v", name, names)
	return -1
}

// A2A's 1.0 binding table contains paths that overlap, and Envoy takes the first
// route that matches. Getting the order wrong does not fail a request: it hands
// GetTaskPushNotificationConfig to ListTaskPushNotificationConfigs' route, which
// then runs that operation's chain — its authentication, its rate limits — with
// nothing anywhere reporting a problem.
//
// The route sorter is supposed to make this fall out (Exact before Regex, then
// left-to-right segment specificity), so each pair below is asserted explicitly
// rather than trusted to.
func TestAgentOverlappingRoutesAreOrderedMostSpecificFirst(t *testing.T) {
	routes := agentEnvoyRoutes(t, routingTestAgent())

	tests := []struct {
		name   string
		first  string
		second string
		why    string
	}{
		{
			name:   "a literal collection path outranks the templated member path",
			first:  "GET|/weather/tasks|agents.example.com",
			second: "GET|/weather/tasks/{id}|agents.example.com",
			why:    "GetTask's regex must not be reached before ListTasks' exact match",
		},
		{
			name:   "the deeper templated path outranks its parent collection",
			first:  "GET|/weather/tasks/{id}/pushNotificationConfigs/{configId}|agents.example.com",
			second: "GET|/weather/tasks/{id}/pushNotificationConfigs|agents.example.com",
			why:    "GetTaskPushNotificationConfig would otherwise run ListTaskPushNotificationConfigs' chain",
		},
		{
			name:   "the card's exact path outranks the templated task member",
			first:  "GET|/weather/.well-known/agent-card.json|agents.example.com",
			second: "GET|/weather/tasks/{id}|agents.example.com",
			why:    "an exact match must never be shadowed by a regex",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			firstIdx := indexOfRoute(t, routes, tc.first)
			secondIdx := indexOfRoute(t, routes, tc.second)
			assert.Less(t, firstIdx, secondIdx, tc.why)
		})
	}
}

// Two operations that differ only by method share a path, so ordering cannot
// separate them — the :method header matcher must. Without it whichever route
// sorted first would answer both, and one operation would silently run the
// other's policy chain.
func TestAgentSamePathOperationsAreSeparatedByMethod(t *testing.T) {
	routes := agentEnvoyRoutes(t, routingTestAgent())

	methodOf := func(name string) string {
		r := routes[indexOfRoute(t, routes, name)]
		for _, header := range r.GetMatch().GetHeaders() {
			if header.GetName() == ":method" {
				return header.GetStringMatch().GetExact()
			}
		}
		t.Fatalf("route %q carries no :method matcher", name)
		return ""
	}

	assert.Equal(t, "POST",
		methodOf("POST|/weather/tasks/{id}/pushNotificationConfigs|agents.example.com"))
	assert.Equal(t, "GET",
		methodOf("GET|/weather/tasks/{id}/pushNotificationConfigs|agents.example.com"))
}

// The path a request arrives on is not the path the upstream expects: the
// gateway strips its own context and transport prefix and hangs the operation
// path off the upstream's base path. A rewrite that dropped the operation path,
// or kept the gateway prefix, would send every A2A request to the wrong place.
// rewrittenUpstreamPath returns the :path the backend actually receives for a
// request, by applying the route's regex rewrite the way Envoy does. Asserting
// on the resulting path rather than on the pattern and substitution strings
// keeps these tests about behaviour: a rewrite can be spelled several ways and
// only the result is the contract.
//
// It also checks the route matches the request at all — an assertion against a
// route that would never have been selected proves nothing.
func rewrittenUpstreamPath(t *testing.T, r *route.Route, requestPath string) string {
	t.Helper()

	match := r.GetMatch()
	switch {
	case match.GetPath() != "":
		require.Equal(t, match.GetPath(), requestPath,
			"route %q does not match %q", r.GetName(), requestPath)
	case match.GetSafeRegex().GetRegex() != "":
		require.Regexp(t, match.GetSafeRegex().GetRegex(), requestPath,
			"route %q does not match %q", r.GetName(), requestPath)
	default:
		t.Fatalf("route %q has no path matcher", r.GetName())
	}

	rewrite := r.GetRoute().GetRegexRewrite()
	require.NotNil(t, rewrite, "route %q has no regex rewrite", r.GetName())
	// Envoy uses RE2 with \1-style references; Go's regexp wants ${1}.
	substitution := strings.ReplaceAll(rewrite.GetSubstitution(), "\\1", "${1}")
	return regexp.MustCompile(rewrite.GetPattern().GetRegex()).
		ReplaceAllString(requestPath, substitution)
}

// spec.context is the only gateway-local part of a path. A transport's
// pathPrefix describes where the agent serves that protocol binding and the
// gateway mirrors it, so the prefix travels upstream — both transports of one
// Agent land under the same upstream base with their own prefixes intact.
//
// Getting this backwards in either direction is silent: strip too much and the
// backend sees a path it does not serve, strip too little and the gateway's own
// context leaks into the upstream request.
func TestAgentForwardsTheTransportPrefixButNotTheContext(t *testing.T) {
	stored := routingTestAgent()
	agentCfg := stored.Configuration.(api.AgentConfiguration)
	upstream := "https://weather.internal/a2a/v1"
	agentCfg.Spec.Upstream.Url = &upstream
	jsonrpcPrefix, httpjsonPrefix := "/rpc", "/http"
	agentCfg.Spec.A2a.OperationConfigs.Transports = []api.A2ATransport{
		{ProtocolBinding: api.JSONRPC, PathPrefix: &jsonrpcPrefix},
		{ProtocolBinding: api.HTTPJSON, PathPrefix: &httpjsonPrefix},
	}
	stored.Configuration = agentCfg

	routes := agentEnvoyRoutes(t, stored)

	tests := []struct {
		name    string
		route   string
		request string
		want    string
	}{
		{
			name:    "the JSON-RPC endpoint keeps its prefix",
			route:   "POST|/weather/rpc|agents.example.com",
			request: "/weather/rpc",
			want:    "/a2a/v1/rpc",
		},
		{
			name:    "an HTTP+JSON operation keeps its prefix and its protocol path",
			route:   "POST|/weather/http/message:send|agents.example.com",
			request: "/weather/http/message:send",
			want:    "/a2a/v1/http/message:send",
		},
		{
			name:    "a templated HTTP+JSON operation carries its path parameter through",
			route:   "GET|/weather/http/tasks/{id}|agents.example.com",
			request: "/weather/http/tasks/abc",
			want:    "/a2a/v1/http/tasks/abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := routes[indexOfRoute(t, routes, tc.route)]
			assert.Equal(t, tc.want, rewrittenUpstreamPath(t, r, tc.request))
		})
	}
}

// agentCard.public.path chooses where the *gateway* serves the card. It says
// nothing about the upstream, which serves its card at the standard well-known
// location whatever the gateway calls it — so a proxied card request is rewritten
// onto that location rather than forwarded verbatim.
//
// Forwarding the gateway-facing path instead is silent in the ordinary case,
// where the two happen to be the same string, and a 404 from the agent the moment
// an operator configures a custom path.
func TestAgentPassthroughCardIsFetchedFromTheWellKnownUpstreamPath(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		cardPath string
		route    string
		request  string
		want     string
	}{
		{
			name:     "the default path at a root upstream",
			upstream: "https://weather.internal",
			route:    "GET|/weather/.well-known/agent-card.json|agents.example.com",
			request:  "/weather/.well-known/agent-card.json",
			want:     "/.well-known/agent-card.json",
		},
		{
			name:     "a custom gateway path still fetches the well-known one",
			upstream: "https://weather.internal",
			cardPath: "/card",
			route:    "GET|/weather/card|agents.example.com",
			request:  "/weather/card",
			want:     "/.well-known/agent-card.json",
		},
		{
			name:     "the upstream base path is preserved beneath it",
			upstream: "https://weather.internal/a2a/v1",
			cardPath: "/card",
			route:    "GET|/weather/card|agents.example.com",
			request:  "/weather/card",
			want:     "/a2a/v1/.well-known/agent-card.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			routes := agentEnvoyRoutes(t, routingTestAgent(func(cfg *api.AgentConfiguration) {
				cfg.Spec.Upstream.Url = &tc.upstream
				cfg.Spec.A2a.AgentCard.Public.Mode = api.A2APublicAgentCardModePassthrough
				cfg.Spec.A2a.AgentCard.Public.Content = nil
				if tc.cardPath != "" {
					cfg.Spec.A2a.AgentCard.Public.Path = &tc.cardPath
				}
			}))

			r := routes[indexOfRoute(t, routes, tc.route)]
			assert.Equal(t, tc.want, rewrittenUpstreamPath(t, r, tc.request))
		})
	}
}

// A custom card path replaces the default route rather than adding an alias: an
// Agent that also answered the well-known path would be advertising one location
// while remaining discoverable at another, and only one of them can be the
// signed card's own URL.
func TestAgentCustomCardPathReplacesTheDefaultRoute(t *testing.T) {
	cardPath := "/card"
	routes := agentEnvoyRoutes(t, routingTestAgent(func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.AgentCard.Public.Path = &cardPath
	}))

	indexOfRoute(t, routes, "GET|/weather/card|agents.example.com")
	for _, r := range routes {
		assert.NotEqual(t, "GET|/weather/.well-known/agent-card.json|agents.example.com", r.GetName(),
			"the default card path is still served alongside the custom one")
	}
}

// The card route must remain an ordinary proxying route with a real policy chain.
// Rendering it as a DirectResponse would answer without ext_proc, which would
// take the CORS preflight and the system observability policies down with it —
// and, in passthrough mode, would answer from the gateway a card the gateway is
// not supposed to hold.
func TestAgentCardRouteIsAProxyingRoute(t *testing.T) {
	for _, mode := range []api.A2APublicAgentCardMode{
		api.A2APublicAgentCardModeManaged,
		api.A2APublicAgentCardModePassthrough,
	} {
		t.Run(string(mode), func(t *testing.T) {
			routes := agentEnvoyRoutes(t, routingTestAgent(func(cfg *api.AgentConfiguration) {
				cfg.Spec.A2a.AgentCard.Public.Mode = mode
				if mode == api.A2APublicAgentCardModePassthrough {
					cfg.Spec.A2a.AgentCard.Public.Content = nil
				}
			}))

			r := routes[indexOfRoute(t, routes, "GET|/weather/.well-known/agent-card.json|agents.example.com")]
			assert.Nil(t, r.GetDirectResponse(), "the card route must not be a direct response")
			require.NotNil(t, r.GetRoute(), "the card route must have a route action")
			assert.NotEmpty(t, r.GetRoute().GetCluster()+r.GetRoute().GetClusterHeader(),
				"the card route must resolve to an upstream cluster")
		})
	}
}
