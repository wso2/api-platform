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

package transform

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/agentproto"
	"github.com/wso2/api-platform/common/chainkey"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// testAgentUUID is the artifact id every fixture uses; composed chain keys are
// built from it, so tests assert against it directly.
const testAgentUUID = "agent-uuid-1"

// agentOption spoils one field of the fixture. Tests compose them rather than
// restating a whole Agent, so what a test is about is the only thing it says.
type agentOption func(*api.AgentConfiguration)

// testAgent returns a deployable Agent: context /weather, both transports, a
// managed public card at the default path, no policies.
func testAgent(options ...agentOption) *models.StoredConfig {
	upstreamURL := "https://weather.internal/a2a"
	cfg := api.AgentConfiguration{
		ApiVersion: api.AgentConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.AgentConfigurationKindAgent,
		Metadata:   api.Metadata{Name: "weather-agent-v1-0"},
		Spec: api.AgentConfigData{
			DisplayName: "Weather Agent",
			Version:     "v1.0",
			Context:     ptrStr("/weather"),
			Upstream:    api.AgentConfigData_Upstream{Url: &upstreamURL},
			A2a: api.A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: api.A2AOperationConfigs{
					Transports: []api.A2ATransport{
						{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr("/rpc")},
						{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")},
					},
				},
				AgentCard: api.A2AAgentCard{
					Public: api.A2APublicAgentCard{Mode: api.A2APublicAgentCardModeManaged},
				},
			},
		},
	}
	for _, option := range options {
		option(&cfg)
	}
	return &models.StoredConfig{
		UUID:          testAgentUUID,
		Kind:          models.KindAgent,
		Handle:        "weather-agent",
		Configuration: cfg,
	}
}

func withTransports(transports ...api.A2ATransport) agentOption {
	return func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.OperationConfigs.Transports = transports
	}
}

func withVhost(vhost string) agentOption {
	return func(cfg *api.AgentConfiguration) { cfg.Spec.Vhost = &vhost }
}

func withContext(context *string) agentOption {
	return func(cfg *api.AgentConfiguration) { cfg.Spec.Context = context }
}

func withOperationPolicies(policies ...api.Policy) agentOption {
	return func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.OperationConfigs.Policies = &policies
	}
}

func withOperationConfig(operation agentproto.Operation, policies ...api.Policy) agentOption {
	return func(cfg *api.AgentConfiguration) {
		entry := api.A2AOperationConfig{Name: api.A2AOperationName(operation)}
		if len(policies) > 0 {
			entry.Policies = &policies
		}
		existing := []api.A2AOperationConfig{}
		if cfg.Spec.A2a.OperationConfigs.Operations != nil {
			existing = *cfg.Spec.A2a.OperationConfigs.Operations
		}
		existing = append(existing, entry)
		cfg.Spec.A2a.OperationConfigs.Operations = &existing
	}
}

func withCardPolicies(policies ...api.Policy) agentOption {
	return func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.AgentCard.Public.Policies = &policies
	}
}

// agentTransformer builds a transformer with no policy definitions loaded, which
// is what most of these tests want: route topology, chain keys and timeouts are
// independent of which policies exist, and an empty catalogue keeps chains empty
// so an assertion on chain contents reads as exactly what was attached.
func agentTransformer() *AgentTransformer {
	return NewAgentTransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
}

// agentTransformerWithPolicies builds a transformer whose catalogue knows the
// named policies at v1.0.0, so attaching them survives version resolution.
func agentTransformerWithPolicies(names ...string) *AgentTransformer {
	definitions := make(map[string]models.PolicyDefinition, len(names))
	for _, name := range names {
		definitions[name+"_v1.0.0"] = models.PolicyDefinition{Name: name, Version: "v1.0.0"}
	}
	return NewAgentTransformer(testRouterCfg(), &config.Config{}, definitions)
}

// routeKeysOf returns the RDC's route keys, sorted, for readable diffs.
func routeKeysOf(rdc *models.RuntimeDeployConfig) []string {
	keys := make([]string, 0, len(rdc.Routes))
	for key := range rdc.Routes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// operationChainKeys returns the composed (operation) chain keys, sorted. The
// route-key chains — public card, preflights — are excluded, so a count assertion
// is about operations and nothing else.
func operationChainKeys(rdc *models.RuntimeDeployConfig) []string {
	keys := make([]string, 0, len(rdc.PolicyChains))
	for key := range rdc.PolicyChains {
		if chainkey.IsComposed(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func policyNames(chain *models.PolicyChain) []string {
	require := make([]string, 0, len(chain.Policies))
	for _, policy := range chain.Policies {
		require = append(require, policy.Name)
	}
	return require
}

// ─── The headline invariant ─────────────────────────────────────────────────

// One canonical chain per operation per routing partition, always — whichever
// transports are configured. This is the property the whole design rests on: two
// transports of one operation must run the *same* policies, which is only true if
// the chain belongs to the operation rather than to the route that carried it.
func TestAgentChainCountIsOperationsTimesPartitions(t *testing.T) {
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)
	require.Len(t, operations, 11, "the 1.0 fixture below is written against eleven operations")

	tests := []struct {
		name       string
		options    []agentOption
		partitions int
	}{
		{
			name:       "both transports, one vhost",
			partitions: 1,
		},
		{
			name:       "JSONRPC only",
			options:    []agentOption{withTransports(api.A2ATransport{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr("/rpc")})},
			partitions: 1,
		},
		{
			name:       "HTTP+JSON only",
			options:    []agentOption{withTransports(api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")})},
			partitions: 1,
		},
		{
			name:       "both transports, three vhosts",
			options:    []agentOption{withVhost("a.example.com;b.example.com;c.example.com")},
			partitions: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rdc, err := agentTransformer().Transform(testAgent(tc.options...))
			require.NoError(t, err)

			assert.Len(t, operationChainKeys(rdc), len(operations)*tc.partitions,
				"one chain per operation per routing partition, invariant under transports")
		})
	}
}

// The chain key must be the one common/chainkey composes, because the policy
// engine composes the same key at request time from the same helper. A chain
// emitted under any other spelling is one it will never find.
func TestAgentChainKeysAreComposedFromTheSharedHelper(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(withVhost("agents.example.com")))
	require.NoError(t, err)

	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)
	for _, operation := range operations {
		key := chainkey.For(testAgentUUID, "agents.example.com", string(operation))
		assert.Contains(t, rdc.PolicyChains, key, "no chain for operation %s", operation)
	}
}

// ─── Route topology ─────────────────────────────────────────────────────────

func TestAgentRouteCounts(t *testing.T) {
	jsonrpcOnly := api.A2ATransport{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr("/rpc")}
	httpjsonOnly := api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")}

	// Eleven 1.0 operations, each with exactly one HTTP+JSON binding today.
	httpJSONRoutes := 0
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)
	for _, operation := range operations {
		bindings, ok := agentproto.HTTPJSONBindings(agentproto.V1_0, operation)
		require.True(t, ok)
		httpJSONRoutes += len(bindings)
	}

	tests := []struct {
		name      string
		transport []api.A2ATransport
		want      int
	}{
		{"JSONRPC only", []api.A2ATransport{jsonrpcOnly}, 1 + 1},
		{"HTTP+JSON only", []api.A2ATransport{httpjsonOnly}, httpJSONRoutes + 1},
		{"both", []api.A2ATransport{jsonrpcOnly, httpjsonOnly}, 1 + httpJSONRoutes + 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rdc, err := agentTransformer().Transform(testAgent(withTransports(tc.transport...)))
			require.NoError(t, err)
			assert.Len(t, rdc.Routes, tc.want, "routes: %v", routeKeysOf(rdc))
		})
	}
}

// A wrong verb is not a test failure somewhere else — it is an operation that no
// request ever reaches, with Envoy returning 404 and no chain ever running. So
// every generated route is checked against the protocol's own binding table.
func TestAgentHTTPJSONVerbsAndPathsMatchTheBindingTable(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(
		withTransports(api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")}),
	))
	require.NoError(t, err)

	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)
	for _, operation := range operations {
		bindings, ok := agentproto.HTTPJSONBindings(agentproto.V1_0, operation)
		require.True(t, ok)
		for _, binding := range bindings {
			key := xds.GenerateRouteNameWithDiscriminator(
				binding.Method, "", "", "/weather"+binding.PathTemplate, "main.local", "")
			route, exists := rdc.Routes[key]
			require.True(t, exists, "no route for %s (%s %s); have %v",
				operation, binding.Method, binding.PathTemplate, routeKeysOf(rdc))
			assert.Equal(t, binding.Method, route.Method)
			assert.Equal(t, binding.PathTemplate, route.OperationPath,
				"with a root prefix the operation path is the binding template itself")
		}
	}
}

// OperationPath is the path with spec.context removed — and nothing else
// removed. The transport prefix stays in it, because the prefix is where the
// agent serves that binding and the gateway mirrors that layout; only
// spec.context is gateway-local. This is the field the translator subtracts from
// Path to decide what to strip, so getting it wrong silently rewrites every A2A
// request to a path the backend does not serve.
func TestAgentOperationPathKeepsTheTransportPrefix(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(withTransports(
		api.A2ATransport{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr("/rpc")},
		api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/http")},
	)))
	require.NoError(t, err)

	tests := []struct {
		routeKey          string
		wantOperationPath string
	}{
		{"POST|/weather/rpc|main.local", "/rpc"},
		{"POST|/weather/http/message:send|main.local", "/http/message:send"},
		{"GET|/weather/http/tasks/{id}|main.local", "/http/tasks/{id}"},
		// The card path is relative to the context and belongs to no transport.
		{"GET|/weather/.well-known/agent-card.json|main.local", "/.well-known/agent-card.json"},
	}

	for _, tc := range tests {
		t.Run(tc.routeKey, func(t *testing.T) {
			route, exists := rdc.Routes[tc.routeKey]
			require.True(t, exists, "have %v", routeKeysOf(rdc))
			assert.Equal(t, tc.wantOperationPath, route.OperationPath)
			assert.Equal(t, "/weather"+tc.wantOperationPath, route.Path,
				"Path minus OperationPath must be exactly spec.context")
		})
	}
}

// A2A's spelled-out 1.0 layout, pinned. The generic assertions above derive
// everything from the registry, which means they would keep passing if the
// registry itself were wrong; this one states the paths a client actually calls.
func TestAgentRouteKeysForV1_0(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	assert.Equal(t, []string{
		"DELETE|/weather/tasks/{id}/pushNotificationConfigs/{configId}|main.local",
		"GET|/weather/.well-known/agent-card.json|main.local",
		"GET|/weather/extendedAgentCard|main.local",
		"GET|/weather/tasks/{id}/pushNotificationConfigs/{configId}|main.local",
		"GET|/weather/tasks/{id}/pushNotificationConfigs|main.local",
		"GET|/weather/tasks/{id}|main.local",
		"GET|/weather/tasks|main.local",
		"POST|/weather/message:send|main.local",
		"POST|/weather/message:stream|main.local",
		"POST|/weather/rpc|main.local",
		"POST|/weather/tasks/{id}/pushNotificationConfigs|main.local",
		"POST|/weather/tasks/{id}:cancel|main.local",
		"POST|/weather/tasks/{id}:subscribe|main.local",
	}, routeKeysOf(rdc))
}

// An Agent may omit spec.context entirely, which is the layout cold discovery
// needs: the card sits at the virtual host root, where an A2A client probes.
func TestAgentWithoutContextServesAtTheVirtualHostRoot(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(
		withContext(nil),
		withTransports(
			api.A2ATransport{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr("/")},
			api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")},
		),
	))
	require.NoError(t, err)

	assert.Empty(t, rdc.Context)
	assert.Contains(t, rdc.Routes, "GET|/.well-known/agent-card.json|main.local")
	assert.Contains(t, rdc.Routes, "POST|/message:send|main.local")

	jsonrpc, exists := rdc.Routes["POST|/|main.local"]
	require.True(t, exists, "the JSON-RPC endpoint is the root itself; have %v", routeKeysOf(rdc))
	assert.Equal(t, "/", jsonrpc.Path)
}

// The route paths this transformer generates have to be the ones the validator
// checked for collisions. Two spellings of the same arithmetic is how a route
// that was proven unique ships as one that shadows another.
func TestAgentRoutePathsMatchTheValidatedPaths(t *testing.T) {
	stored := testAgent(withContext(ptrStr("/weather")))
	agentCfg := stored.Configuration.(api.AgentConfiguration)

	rdc, err := agentTransformer().Transform(stored)
	require.NoError(t, err)

	context := config.AgentContextPath(agentCfg.Spec.Context)
	assert.Equal(t, config.JoinAgentPath(context, "/rpc"), rdc.Routes["POST|/weather/rpc|main.local"].Path)
	assert.Equal(t,
		config.JoinAgentPath(context, config.DefaultAgentCardPath),
		rdc.Routes["GET|/weather/.well-known/agent-card.json|main.local"].Path)
}

// Vhost is parsed back out of a route key by position (index 2). A route name
// that does not round-trip makes every request on it fail to find its chain.
func TestAgentRouteNamesRoundTripTheVhost(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(withVhost("agents.example.com")))
	require.NoError(t, err)

	for key, route := range rdc.Routes {
		parts := strings.Split(key, "|")
		require.GreaterOrEqual(t, len(parts), 3, "route key %q", key)
		assert.Equal(t, route.Method, parts[0])
		assert.Equal(t, route.Path, parts[1])
		assert.Equal(t, "agents.example.com", parts[2])
		assert.Equal(t, route.Vhost, parts[2])
	}
}

// ─── Resolver wiring ────────────────────────────────────────────────────────

// Every operation route names the a2a resolver and carries the protocol version;
// no route carries both a resolver and a canonical chain key. The card route is
// the other half: it is not an operation, so it stays directly resolved.
func TestAgentResolverWiring(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	cardKey := "GET|/weather/.well-known/agent-card.json|main.local"
	jsonrpcKey := "POST|/weather/rpc|main.local"

	for key, route := range rdc.Routes {
		if key == cardKey {
			assert.Empty(t, route.ResolverName, "the public card is not an A2A operation")
			assert.Empty(t, route.ResolverConfig)
			assert.True(t, models.IsDirectlyResolved(rdc.EffectiveResolverName(route)))
			continue
		}

		assert.Equal(t, agentproto.ResolverName, route.ResolverName, "route %q", key)
		assert.Empty(t, route.CanonicalChainKey,
			"route %q: a resolver-bearing route composes its key per request", key)

		var resolverConfig agentproto.ResolverConfig
		require.NoError(t, json.Unmarshal(route.ResolverConfig, &resolverConfig), "route %q", key)
		assert.Equal(t, agentproto.V1_0, resolverConfig.ProtocolVersion,
			"route %q: the version selects the operation table the engine resolves against", key)

		if key == jsonrpcKey {
			assert.Equal(t, agentproto.TransportJSONRPC, resolverConfig.Transport)
			assert.Empty(t, resolverConfig.Operation, "the JSON-RPC operation comes from the request body")
			continue
		}
		assert.Equal(t, agentproto.TransportHTTPJSON, resolverConfig.Transport)
		assert.NotEmpty(t, resolverConfig.Operation,
			"route %q: an HTTP+JSON route knows its operation at ingest", key)
		assert.True(t, agentproto.IsOperation(agentproto.V1_0, string(resolverConfig.Operation)),
			"route %q resolves to %q, which is not a 1.0 operation", key, resolverConfig.Operation)
	}
}

// Both transports of one operation must carry the same operation identity, since
// that identity is what composes the chain key. Asserted on the one operation
// whose two transports are most obviously different requests.
func TestAgentTransportsShareOneOperationChain(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	var httpJSON agentproto.ResolverConfig
	require.NoError(t, json.Unmarshal(
		rdc.Routes["POST|/weather/message:send|main.local"].ResolverConfig, &httpJSON))
	assert.Equal(t, agentproto.SendMessage, httpJSON.Operation)

	// The JSON-RPC route resolves the same name from the body at request time,
	// and both compose this one key.
	assert.Contains(t, rdc.PolicyChains,
		chainkey.For(testAgentUUID, "main.local", string(agentproto.SendMessage)))
}

// The JSON-RPC route buffers its body before any policy runs, so it is the one
// route with an explicit acceptance ceiling on it.
func TestAgentJSONRPCRouteBoundsTheRequestBody(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	assert.Equal(t, int64(maxAgentJSONRPCRequestBodyBytes),
		rdc.Routes["POST|/weather/rpc|main.local"].MaxRequestBodyBytes)
	assert.Zero(t, rdc.Routes["POST|/weather/message:send|main.local"].MaxRequestBodyBytes,
		"an HTTP+JSON route buffers nothing for resolution")
}

func TestAgentRejectsUnsupportedProtocolVersion(t *testing.T) {
	stored := testAgent(func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.ProtocolVersion = "9.9"
	})

	_, err := agentTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "9.9")
}

// ─── Chain composition ──────────────────────────────────────────────────────

// System policies first, then the operation-common policies, then the selected
// operation's own — plain concatenation, so a policy attached at both levels
// runs twice as two instances rather than being deduplicated.
func TestAgentChainOrderIsSystemThenCommonThenOperation(t *testing.T) {
	transformer := agentTransformerWithPolicies("common-policy", "op-policy")
	// A system policy is injected only when the gateway config enables one;
	// with none enabled the chain is exactly what the Agent attached, which is
	// what makes the ordering assertion below unambiguous.
	rdc, err := transformer.Transform(testAgent(
		withOperationPolicies(api.Policy{Name: "common-policy"}),
		withOperationConfig(agentproto.GetTask, api.Policy{Name: "op-policy"}, api.Policy{Name: "common-policy"}),
	))
	require.NoError(t, err)

	getTask := rdc.PolicyChains[chainkey.For(testAgentUUID, "main.local", string(agentproto.GetTask))]
	require.NotNil(t, getTask)
	assert.Equal(t, []string{"common-policy", "op-policy", "common-policy"}, policyNames(getTask),
		"no dedup: a doubly-attached policy is two instances")

	// An operation with no entry of its own gets the common policies only.
	listTasks := rdc.PolicyChains[chainkey.For(testAgentUUID, "main.local", string(agentproto.ListTasks))]
	require.NotNil(t, listTasks)
	assert.Equal(t, []string{"common-policy"}, policyNames(listTasks))
}

// operationConfigs.operations is configuration, not an allowlist: an operation
// nobody configured is still exposed and still gets the common policies.
func TestAgentEmitsEveryOperationChainRegardlessOfConfiguration(t *testing.T) {
	transformer := agentTransformerWithPolicies("common-policy")
	rdc, err := transformer.Transform(testAgent(
		withOperationPolicies(api.Policy{Name: "common-policy"}),
		withOperationConfig(agentproto.SendMessage),
	))
	require.NoError(t, err)

	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)
	for _, operation := range operations {
		chain := rdc.PolicyChains[chainkey.For(testAgentUUID, "main.local", string(operation))]
		require.NotNil(t, chain, "operation %s has no chain", operation)
		assert.Equal(t, []string{"common-policy"}, policyNames(chain), "operation %s", operation)
	}
}

// The public card has its own policy scope. Operation policies must not reach it
// — the card is a discovery document, and running an operation's authentication
// against it would make the Agent undiscoverable.
func TestAgentCardChainUsesOnlyCardPolicies(t *testing.T) {
	transformer := agentTransformerWithPolicies("common-policy", "card-policy")
	rdc, err := transformer.Transform(testAgent(
		withOperationPolicies(api.Policy{Name: "common-policy"}),
		withCardPolicies(api.Policy{Name: "card-policy"}),
	))
	require.NoError(t, err)

	cardChain := rdc.PolicyChains["GET|/weather/.well-known/agent-card.json|main.local"]
	require.NotNil(t, cardChain)
	assert.Equal(t, []string{"card-policy"}, policyNames(cardChain))
}

// The upstream credential travels in the chains whose requests reach the
// upstream, and nowhere else: a managed card is answered by the gateway, so its
// chain has no business carrying the credential.
func TestAgentUpstreamAuthReachesOperationChainsOnly(t *testing.T) {
	transformer := agentTransformerWithPolicies("set-headers")
	withAuth := func(cfg *api.AgentConfiguration) {
		cfg.Spec.Upstream.Auth = &struct {
			Header *string                             `json:"header,omitempty" yaml:"header,omitempty"`
			Type   api.AgentConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value  *string                             `json:"value,omitempty" yaml:"value,omitempty"`
		}{
			Header: ptrStr("Authorization"),
			Type:   api.AgentConfigDataUpstreamAuthTypeApiKey,
			Value:  ptrStr("Bearer secret-token"),
		}
	}

	managed, err := transformer.Transform(testAgent(withAuth))
	require.NoError(t, err)
	assert.Equal(t, []string{"set-headers"},
		policyNames(managed.PolicyChains[chainkey.For(testAgentUUID, "main.local", string(agentproto.GetTask))]))
	assert.Empty(t,
		policyNames(managed.PolicyChains["GET|/weather/.well-known/agent-card.json|main.local"]),
		"a managed card is served by the gateway and never reaches the upstream")

	passthrough, err := transformer.Transform(testAgent(withAuth, func(cfg *api.AgentConfiguration) {
		cfg.Spec.A2a.AgentCard.Public.Mode = api.A2APublicAgentCardModePassthrough
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"set-headers"},
		policyNames(passthrough.PolicyChains["GET|/weather/.well-known/agent-card.json|main.local"]),
		"a proxied card is fetched from the upstream, which may require the credential")
}

// ─── CORS preflight ─────────────────────────────────────────────────────────

// A preflight route exists only when a cors policy is attached, because without
// one nothing would answer it and Envoy would forward an OPTIONS upstream.
func TestAgentPreflightRoutesOnlyWithCORS(t *testing.T) {
	withoutCORS, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)
	for key := range withoutCORS.Routes {
		assert.False(t, strings.HasPrefix(key, "OPTIONS|"), "unexpected preflight route %q", key)
	}

	transformer := agentTransformerWithPolicies("cors")
	withCORS, err := transformer.Transform(testAgent(withOperationPolicies(api.Policy{Name: "cors"})))
	require.NoError(t, err)

	preflights := map[string]bool{}
	for key := range withCORS.Routes {
		if strings.HasPrefix(key, "OPTIONS|") {
			preflights[key] = true
		}
	}
	// One per distinct operation path: the JSON-RPC endpoint plus the nine
	// distinct HTTP+JSON paths (two pairs of operations share a path). The card
	// has no cors policy of its own, so it gets none.
	assert.Len(t, preflights, 10, "preflights: %v", preflights)
	assert.True(t, preflights["OPTIONS|/weather/rpc|main.local"])
	assert.True(t, preflights["OPTIONS|/weather/tasks/{id}/pushNotificationConfigs|main.local"],
		"POST and GET on one path share a single preflight")
	assert.False(t, preflights["OPTIONS|/weather/.well-known/agent-card.json|main.local"],
		"the card scope attached no cors policy")
}

// The invariant that makes preflight generation meaningful: a preflight route
// exists only where its own chain can answer it.
//
// Route existence alone proves nothing. A route that matches OPTIONS but whose
// chain has no cors policy is worse than no route: Envoy stops 404ing the
// preflight and proxies it to the upstream instead, so the browser's failure
// becomes whatever the backend does with an OPTIONS it never expected.
func TestAgentEveryPreflightChainCanAnswerIt(t *testing.T) {
	transformer := agentTransformerWithPolicies("cors", "auth")

	for _, tc := range []struct {
		name    string
		options []agentOption
	}{
		{"no cors anywhere", nil},
		{"cors on the common scope", []agentOption{withOperationPolicies(api.Policy{Name: "cors"})}},
		{"cors on one operation", []agentOption{withOperationConfig(agentproto.GetTask, api.Policy{Name: "cors"})}},
		{"cors on the card only", []agentOption{withCardPolicies(api.Policy{Name: "cors"})}},
		{
			name: "cors on an operation, another policy on the common scope",
			options: []agentOption{
				withOperationPolicies(api.Policy{Name: "auth"}),
				withOperationConfig(agentproto.SendMessage, api.Policy{Name: "cors"}),
			},
		},
		{
			// A cors attachment naming a policy the gateway does not have is
			// dropped from the chain, so it must not conjure a preflight either.
			name:    "cors attached but not a loaded policy",
			options: []agentOption{withOperationPolicies(api.Policy{Name: "cors", Version: "v9"})},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rdc, err := transformer.Transform(testAgent(tc.options...))
			require.NoError(t, err)

			for key, route := range rdc.Routes {
				if route.Method != "OPTIONS" {
					continue
				}
				chain := rdc.PolicyChains[key]
				require.NotNil(t, chain, "preflight %q has no chain", key)
				assert.Contains(t, policyNames(chain), "cors",
					"preflight %q cannot answer the request it matches", key)
			}
		})
	}
}

// A cors policy attached to a single operation must produce a preflight for the
// paths that operation is reachable at — and only those. Attaching it to one
// operation previously enabled a preflight on every path while contributing its
// policy to none of them.
func TestAgentOperationScopedCORSPreflights(t *testing.T) {
	transformer := agentTransformerWithPolicies("cors", "auth")
	rdc, err := transformer.Transform(testAgent(
		withOperationPolicies(api.Policy{Name: "auth"}),
		withOperationConfig(agentproto.GetTask, api.Policy{Name: "cors"}),
	))
	require.NoError(t, err)

	preflights := map[string][]string{}
	for key, route := range rdc.Routes {
		if route.Method == "OPTIONS" {
			preflights[key] = policyNames(rdc.PolicyChains[key])
		}
	}

	// GetTask's own path, plus the JSON-RPC endpoint — every operation is
	// reachable there, so an operation-level cors covers it too.
	assert.Equal(t, map[string][]string{
		"OPTIONS|/weather/tasks/{id}|main.local": {"auth", "cors"},
		"OPTIONS|/weather/rpc|main.local":        {"auth", "cors"},
	}, preflights)

	// The operation's other policies stay out: a preflight carries no
	// credentials, so running the operation's authentication against it would
	// reject the very request the cors policy is there to answer. Only cors is
	// borrowed — auth here comes from the common scope, which every preflight
	// gets.
	getTaskChain := rdc.PolicyChains[chainkey.For(testAgentUUID, "main.local", string(agentproto.GetTask))]
	assert.Equal(t, []string{"auth", "cors"}, policyNames(getTaskChain),
		"the operation's real chain is unaffected")
}

// Two operations sharing one path each contribute their cors policy to the
// single preflight that covers both, in operation order — the same plain
// concatenation the operation chains use, with no dedup and no override.
func TestAgentSharedPathPreflightMergesContributingOperations(t *testing.T) {
	transformer := agentTransformerWithPolicies("cors")
	rdc, err := transformer.Transform(testAgent(
		withTransports(api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")}),
		withOperationConfig(agentproto.CreateTaskPushNotificationConfig, api.Policy{Name: "cors"}),
		withOperationConfig(agentproto.ListTaskPushNotificationConfigs, api.Policy{Name: "cors"}),
	))
	require.NoError(t, err)

	key := "OPTIONS|/weather/tasks/{id}/pushNotificationConfigs|main.local"
	require.Contains(t, rdc.Routes, key, "have %v", routeKeysOf(rdc))
	assert.Equal(t, []string{"cors", "cors"}, policyNames(rdc.PolicyChains[key]),
		"POST and GET on this path both attached cors; both instances run")
}

// A preflight must strip exactly what the route it guards strips. Re-deriving
// its operation path from the gateway path instead of carrying it from that
// route gets it wrong at both ends of the range: a context of "/" leaves a
// relative "rpc", and a transport mounted at the context itself leaves "".
// Either one rewrites the upstream path differently from the real route.
func TestAgentPreflightSharesItsRouteOperationPath(t *testing.T) {
	transformer := agentTransformerWithPolicies("cors")

	tests := []struct {
		name     string
		context  *string
		prefix   string
		routeKey string
	}{
		{"root context", ptrStr("/"), "/rpc", "/rpc"},
		{"no context", nil, "/rpc", "/rpc"},
		{"nested context", ptrStr("/weather"), "/rpc", "/weather/rpc"},
		{"transport at the context itself", ptrStr("/weather"), "/", "/weather"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rdc, err := transformer.Transform(testAgent(
				withContext(tc.context),
				withTransports(api.A2ATransport{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr(tc.prefix)}),
				withOperationPolicies(api.Policy{Name: "cors"}),
			))
			require.NoError(t, err)

			endpoint := rdc.Routes["POST|"+tc.routeKey+"|main.local"]
			require.NotNil(t, endpoint, "have %v", routeKeysOf(rdc))
			preflight := rdc.Routes["OPTIONS|"+tc.routeKey+"|main.local"]
			require.NotNil(t, preflight, "have %v", routeKeysOf(rdc))

			assert.Equal(t, endpoint.OperationPath, preflight.OperationPath,
				"the preflight must rewrite the upstream path exactly as its route does")
			assert.True(t, strings.HasPrefix(preflight.OperationPath, "/"),
				"an operation path is absolute; got %q", preflight.OperationPath)
		})
	}
}

// The card's cors policy is attached in its own scope, so it produces its own
// preflight and takes its own chain.
func TestAgentCardPreflightUsesTheCardScope(t *testing.T) {
	transformer := agentTransformerWithPolicies("cors")
	rdc, err := transformer.Transform(testAgent(withCardPolicies(api.Policy{Name: "cors"})))
	require.NoError(t, err)

	key := "OPTIONS|/weather/.well-known/agent-card.json|main.local"
	require.Contains(t, rdc.Routes, key)
	assert.Equal(t, []string{"cors"}, policyNames(rdc.PolicyChains[key]))
	assert.Empty(t, rdc.Routes[key].ResolverName, "a preflight is not an A2A operation")
}

// ─── Timeouts ───────────────────────────────────────────────────────────────

// A finite route timeout would sever a healthy stream, so the routes that can
// carry one default to a disabled timeout; the idle timeout stays the liveness
// guard, and an explicit resilience.timeout always wins.
func TestAgentStreamingRoutesDisableTheRouteTimeoutByDefault(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	disabled := []string{
		"POST|/weather/rpc|main.local",                  // every operation shares it, streaming included
		"POST|/weather/message:stream|main.local",       // SendStreamingMessage
		"POST|/weather/tasks/{id}:subscribe|main.local", // SubscribeToTask
	}
	for _, key := range disabled {
		route := rdc.Routes[key]
		require.NotNil(t, route, "route %q", key)
		require.NotNil(t, route.Timeout, "route %q", key)
		require.NotNil(t, route.Timeout.Timeout, "route %q", key)
		assert.Zero(t, *route.Timeout.Timeout, "route %q must disable the route timeout", key)
	}

	unary := rdc.Routes["POST|/weather/message:send|main.local"]
	require.NotNil(t, unary)
	assert.Nil(t, unary.Timeout, "a unary route falls back to the gateway's global default")
}

func TestAgentExplicitTimeoutOverridesTheStreamingDefault(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(func(cfg *api.AgentConfiguration) {
		cfg.Spec.Resilience = &api.Resilience{Timeout: ptrStr("45s"), IdleTimeout: ptrStr("90s")}
	}))
	require.NoError(t, err)

	for _, key := range []string{
		"POST|/weather/rpc|main.local",
		"POST|/weather/message:stream|main.local",
		"POST|/weather/message:send|main.local",
	} {
		route := rdc.Routes[key]
		require.NotNil(t, route.Timeout, "route %q", key)
		assert.Equal(t, "45s", route.Timeout.Timeout.String(), "route %q", key)
		assert.Equal(t, "1m30s", route.Timeout.IdleTimeout.String(), "route %q", key)
	}
}

// Timeout and idleTimeout resolve independently. An operation that configures
// only one of them must not discard the Agent's setting for the other — and on a
// streaming route, where an unset route timeout defaults to disabled, that
// discarded value would be replaced by "no timeout at all" rather than merely
// falling back.
func TestAgentTimeoutPrecedenceIsPerField(t *testing.T) {
	tests := []struct {
		name      string
		operation agentproto.Operation
		routeKey  string
		wantRoute string
		wantIdle  string
	}{
		{
			name:      "streaming operation setting only idleTimeout keeps the agent timeout",
			operation: agentproto.SendStreamingMessage,
			routeKey:  "POST|/weather/message:stream|main.local",
			wantRoute: "45s",
			wantIdle:  "1m30s",
		},
		{
			name:      "unary operation setting only idleTimeout keeps the agent timeout",
			operation: agentproto.GetTask,
			routeKey:  "GET|/weather/tasks/{id}|main.local",
			wantRoute: "45s",
			wantIdle:  "1m30s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rdc, err := agentTransformer().Transform(testAgent(
				func(cfg *api.AgentConfiguration) {
					cfg.Spec.Resilience = &api.Resilience{Timeout: ptrStr("45s")}
					operations := []api.A2AOperationConfig{{
						Name:       api.A2AOperationName(tc.operation),
						Resilience: &api.Resilience{IdleTimeout: ptrStr("90s")},
					}}
					cfg.Spec.A2a.OperationConfigs.Operations = &operations
				},
			))
			require.NoError(t, err)

			route := rdc.Routes[tc.routeKey]
			require.NotNil(t, route)
			require.NotNil(t, route.Timeout)
			require.NotNil(t, route.Timeout.Timeout)
			assert.Equal(t, tc.wantRoute, route.Timeout.Timeout.String())
			require.NotNil(t, route.Timeout.IdleTimeout)
			assert.Equal(t, tc.wantIdle, route.Timeout.IdleTimeout.String())
		})
	}
}

// The streaming default applies only when no timeout is configured at either
// level — it is a default, not an override.
func TestAgentStreamingDefaultAppliesOnlyWhenNoTimeoutIsConfigured(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(func(cfg *api.AgentConfiguration) {
		operations := []api.A2AOperationConfig{{
			Name:       api.A2AOperationName(agentproto.SendStreamingMessage),
			Resilience: &api.Resilience{IdleTimeout: ptrStr("90s")},
		}}
		cfg.Spec.A2a.OperationConfigs.Operations = &operations
	}))
	require.NoError(t, err)

	route := rdc.Routes["POST|/weather/message:stream|main.local"]
	require.NotNil(t, route.Timeout)
	require.NotNil(t, route.Timeout.Timeout)
	assert.Zero(t, *route.Timeout.Timeout,
		"nothing set a route timeout, so a streaming route disables it")
	assert.Equal(t, "1m30s", route.Timeout.IdleTimeout.String(),
		"the idle timeout is the liveness guard a disabled route timeout relies on")
}

func TestAgentOperationResilienceOverridesAgentLevel(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent(
		func(cfg *api.AgentConfiguration) {
			cfg.Spec.Resilience = &api.Resilience{Timeout: ptrStr("45s")}
		},
		func(cfg *api.AgentConfiguration) {
			operations := []api.A2AOperationConfig{{
				Name:       api.A2AOperationName(agentproto.GetTask),
				Resilience: &api.Resilience{Timeout: ptrStr("5s")},
			}}
			cfg.Spec.A2a.OperationConfigs.Operations = &operations
		},
	))
	require.NoError(t, err)

	assert.Equal(t, "5s", rdc.Routes["GET|/weather/tasks/{id}|main.local"].Timeout.Timeout.String())
	assert.Equal(t, "45s", rdc.Routes["GET|/weather/tasks|main.local"].Timeout.Timeout.String())
}

// ─── Upstream and metadata ──────────────────────────────────────────────────

func TestAgentUpstreamAndMetadata(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	assert.Equal(t, models.KindAgent, rdc.Metadata.Kind)
	assert.Equal(t, testAgentUUID, rdc.Metadata.UUID)
	assert.Equal(t, "weather-agent", rdc.Metadata.Handle)
	assert.Equal(t, "v1.0", rdc.Metadata.Version)
	assert.Equal(t, "Weather Agent", rdc.Metadata.DisplayName)
	assert.Equal(t, "/weather", rdc.Context)
	assert.Equal(t, models.RouteKeyResolverName, rdc.PolicyChainResolver)

	require.Len(t, rdc.UpstreamClusters, 1)
	for key, cluster := range rdc.UpstreamClusters {
		assert.Equal(t, "upstream_main_weather.internal_443", key)
		assert.Equal(t, "/a2a", cluster.BasePath)
		assert.True(t, cluster.TLS.Enabled)
		require.Len(t, cluster.Endpoints, 1)
		assert.Equal(t, "weather.internal", cluster.Endpoints[0].Host)
		assert.Equal(t, 443, cluster.Endpoints[0].Port)
	}

	for key, route := range rdc.Routes {
		require.NotNil(t, route.Upstream.Default, "route %q", key)
		assert.Equal(t, "upstream_main_weather.internal_443", route.Upstream.ClusterKey, "route %q", key)
		assert.False(t, route.Upstream.UseClusterHeader,
			"an Agent has no sandbox slot and no upstream definitions here")
	}
}

// ─── Resolution validation ──────────────────────────────────────────────────

// Every generated RDC must satisfy the model's own resolution rules, since both
// xDS streams are fed from it independently and a mismatch surfaces only at
// request time.
func TestAgentGeneratedConfigPassesResolutionValidation(t *testing.T) {
	for _, options := range [][]agentOption{
		{},
		{withTransports(api.A2ATransport{ProtocolBinding: api.JSONRPC, PathPrefix: ptrStr("/rpc")})},
		{withTransports(api.A2ATransport{ProtocolBinding: api.HTTPJSON, PathPrefix: ptrStr("/")})},
		{withContext(nil)},
		{withVhost("a.example.com;b.example.com")},
	} {
		rdc, err := agentTransformer().Transform(testAgent(options...))
		require.NoError(t, err)
		assert.NoError(t, rdc.ValidateResolution())
	}
}

// The counterpart: a deliberately broken RDC is rejected. Removing an operation
// chain leaves the resolver-bearing routes in that partition with nothing to
// resolve to, which is exactly the deployment error ValidateResolution exists to
// name.
func TestAgentResolutionValidationRejectsAMissingOperationChain(t *testing.T) {
	rdc, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	for _, key := range operationChainKeys(rdc) {
		delete(rdc.PolicyChains, key)
	}
	require.Error(t, rdc.ValidateResolution())
}

// ─── Kind dispatch ──────────────────────────────────────────────────────────

// R3: the registry's dispatch and the Envoy translator's kind map must agree.
//
// main.go no longer hand-writes that map — it ranges over EnvoyTranslatorKinds()
// — so this asserts the derivation is right rather than that someone remembered
// to update a literal. A test that built its own map (as the routing tests must,
// to construct a translator) would keep passing if production dropped Agent, so
// it cannot stand in for this one.
func TestEnvoyTranslatorKindsCoverAgent(t *testing.T) {
	kinds := EnvoyTranslatorKinds()

	assert.Contains(t, kinds, models.KindAgent,
		"Agent must reach the Envoy translator through the RDC path, or its routes are named "+
			"by the legacy path while its policy chains are keyed from the RDC")

	// Every kind wired into the translator must be one the registry can actually
	// transform, or the translator falls back to the legacy path at runtime with
	// only a log line.
	for _, kind := range kinds {
		assert.Contains(t, Kinds(), kind, "kind %q is wired into the translator but the registry cannot transform it", kind)
	}

	// And the only kind held back is the one held back on purpose. This is what
	// makes a newly added kind fail here instead of being quietly omitted.
	var excluded []string
	for _, kind := range Kinds() {
		if !slices.Contains(kinds, kind) {
			excluded = append(excluded, kind)
		}
	}
	assert.Equal(t, []string{models.KindWebSubApi}, excluded,
		"WebSubApi keeps the legacy async translation path; nothing else may be excluded silently")
}

// registryKinds is a list beside a switch, so it can drift from the switch it
// describes. Every kind it names must dispatch, and a kind it does not name must
// not — otherwise EnvoyTranslatorKinds() above is derived from a fiction.
func TestRegistryKindsMatchDispatch(t *testing.T) {
	registry := NewRegistry(
		NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{}),
		nil,
		agentTransformer(),
	)

	for _, kind := range Kinds() {
		// An empty Configuration fails the transformer's own type assertion, which
		// is a different error from "no transformer for this kind" — that
		// distinction is exactly what is being checked.
		_, err := registry.Transform(&models.StoredConfig{UUID: "u", Kind: kind})
		assert.NotErrorIs(t, err, ErrUnsupportedKind, "Kinds() names %q but Transform does not dispatch it", kind)
	}

	_, err := registry.Transform(&models.StoredConfig{UUID: "u", Kind: "NotAKind"})
	assert.ErrorIs(t, err, ErrUnsupportedKind)
}

func TestAgentRegistryDispatch(t *testing.T) {
	registry := NewRegistry(
		NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{}),
		nil,
		agentTransformer(),
	)

	viaRegistry, err := registry.Transform(testAgent())
	require.NoError(t, err)
	direct, err := agentTransformer().Transform(testAgent())
	require.NoError(t, err)

	assert.Equal(t, routeKeysOf(direct), routeKeysOf(viaRegistry))
	assert.Equal(t, operationChainKeys(direct), operationChainKeys(viaRegistry))
}

func TestAgentTransformerRejectsForeignConfiguration(t *testing.T) {
	_, err := agentTransformer().Transform(&models.StoredConfig{
		UUID:          "not-an-agent",
		Kind:          models.KindAgent,
		Configuration: api.RestAPI{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an Agent")
}
