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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const testMCPAPIID = "mcp-api-1"

// makeMCPStoredConfig builds an MCP StoredConfig the way the deployment service does —
// desugaring through the real MCPTransformer — so these tests exercise the actual
// synthesised routes rather than a hand-written approximation of them.
func makeMCPStoredConfig(t *testing.T, specVersion string) *models.StoredConfig {
	t.Helper()

	upstreamURL := "http://weather:3001"
	spec := api.MCPProxyConfigData{
		DisplayName: "Weather",
		Version:     "v1.0",
		Context:     api.Ptr("/weather"),
		Upstream:    api.MCPProxyConfigData_Upstream{Url: &upstreamURL},
	}
	if specVersion != "" {
		spec.SpecVersion = &specVersion
	}

	mcpConfig := api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "weather-mcp"},
		Spec:       spec,
	}

	var restAPI api.RestAPI
	// A nil policy version resolver is supported: it only validates an
	// upstream.auth.policyVersion override, and this fixture's upstream carries no auth
	// block, so nothing in the transform path calls it.
	converted, err := utils.NewMCPTransformer(nil).Transform(&mcpConfig, &restAPI)
	require.NoError(t, err, "the MCP proxy must desugar into a RestAPI first")

	return &models.StoredConfig{
		UUID:                testMCPAPIID,
		Kind:                string(api.MCPProxyConfigurationKindMcp),
		Handle:              "weather-mcp",
		DisplayName:         "Weather",
		Version:             "v1.0",
		Configuration:       *converted,
		SourceConfiguration: mcpConfig,
	}
}

// transformMCP runs the real transformer with no registered policy definitions. That is
// enough here because these tests assert on route wiring — which route carries a
// resolver, and which key its chain is filed under — not on chain contents. A test that
// needs a policy to survive chain building would have to register its definition.
func transformMCP(t *testing.T, cfg *models.StoredConfig) *models.RuntimeDeployConfig {
	t.Helper()
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	return rdc
}

// findRoute returns the route for one method on a path suffix, without depending on how
// route keys are spelled.
func findRoute(t *testing.T, rdc *models.RuntimeDeployConfig, method, pathSuffix string) (string, *models.Route) {
	t.Helper()
	for key, route := range rdc.Routes {
		if route.Method == method && strings.HasSuffix(route.Path, pathSuffix) {
			return key, route
		}
	}
	t.Fatalf("no %s route ending %q among %d routes", method, pathSuffix, len(rdc.Routes))
	return "", nil
}

// ─── The multiplexed route ───────────────────────────────────────────────────

// The POST route is the one endpoint every MCP operation arrives on, so it is the only
// one whose chain a resolver selects.
func TestMCPResolution_POSTRouteIsResolverBearing(t *testing.T) {
	rdc := transformMCP(t, makeMCPStoredConfig(t, "2025-06-18"))

	_, post := findRoute(t, rdc, "POST", "/mcp")
	assert.Equal(t, MCPResolverName, post.ResolverName)
	// No resolver config: the MCP resolver reads none, so anything emitted here would be
	// written and never read. Asserted rather than assumed, because a field that is
	// silently ignored is exactly the kind that grows back.
	assert.Empty(t, post.ResolverConfig)
	// Left unset on purpose: the engine applies its own 64 KiB ceiling to any
	// body-resolved route, and that default is where the bound should be tuned. Pinning
	// the same number here would exempt MCP from a future change to it.
	assert.Zero(t, post.MaxRequestBodyBytes)

	// A resolver-bearing route composes its key per request, so a canonical key on it
	// would be read by nothing — ValidateResolution rejects one outright.
	assert.Empty(t, post.CanonicalChainKey)

	// The chain must be filed under the key the resolver will actually return. If these
	// two disagree the lookup finds nothing and every request to the route fails.
	composed := rdc.ChainKeyFor(post.Vhost, mcpResolverOperation)
	assert.Contains(t, rdc.PolicyChains, composed,
		"the POST chain must be filed under the composed operation key")
}

// Only the multiplexed route changes. Its siblings each mean exactly one thing, so they
// keep resolving by route identity — a mix the runtime supports per route.
func TestMCPResolution_SiblingRoutesStayRouteKeyed(t *testing.T) {
	rdc := transformMCP(t, makeMCPStoredConfig(t, "2025-06-18"))

	for _, method := range []string{"GET", "DELETE"} {
		routeKey, route := findRoute(t, rdc, method, "/mcp")
		assert.Empty(t, route.ResolverName, "%s /mcp means one thing and needs no resolver", method)
		assert.Contains(t, rdc.PolicyChains, routeKey,
			"%s keeps a route-keyed chain", method)
	}
}

// The chain's *contents* are unchanged by this wiring — only the key the POST chain is
// filed under. Same policies, in the same order, as its route-keyed siblings.
func TestMCPResolution_ChainContentsAreUnchanged(t *testing.T) {
	rdc := transformMCP(t, makeMCPStoredConfig(t, "2025-06-18"))

	_, post := findRoute(t, rdc, "POST", "/mcp")
	getKey, _ := findRoute(t, rdc, "GET", "/mcp")

	postChain := rdc.PolicyChains[rdc.ChainKeyFor(post.Vhost, mcpResolverOperation)]
	getChain := rdc.PolicyChains[getKey]
	require.NotNil(t, postChain)
	require.NotNil(t, getChain)

	assert.Equal(t, getChain.Policies, postChain.Policies,
		"re-keying a chain must not rebuild it")
}

// ─── The declared era changes nothing the controller emits ───────────────────

// The proxy's specVersion once decided both what the resolver was told and whether the
// route buffered. It now decides neither: the resolver reads no config and every MCP
// route reads the body. Asserted across the eras because this is the property that was
// deliberately given up, and re-deriving anything from specVersion here would undo it.
func TestMCPResolution_DeclaredEraDoesNotChangeTheEmittedRoute(t *testing.T) {
	for _, specVersion := range []string{"2025-06-18", "2026-07-28", ""} {
		t.Run("specVersion="+specVersion, func(t *testing.T) {
			rdc := transformMCP(t, makeMCPStoredConfig(t, specVersion))

			_, post := findRoute(t, rdc, "POST", "/mcp")
			assert.Equal(t, MCPResolverName, post.ResolverName)
			assert.Empty(t, post.ResolverConfig)
		})
	}
}

// ─── The contract the controller holds itself to ─────────────────────────────

// ValidateResolution is the controller's own check that what it is about to push is
// coherent: a resolver-bearing route must carry no canonical key, and must have an
// operation chain in its routing partition. Running it here proves the emitted config is
// acceptable before it reaches the wire, rather than after a gateway starts 500ing.
func TestMCPResolution_EmittedConfigPassesValidateResolution(t *testing.T) {
	rdc := transformMCP(t, makeMCPStoredConfig(t, "2025-06-18"))
	assert.NoError(t, rdc.ValidateResolution())
}

// ─── Everything else is untouched ────────────────────────────────────────────

func TestMCPResolution_NonMCPAPIIsUnaffected(t *testing.T) {
	rdc := transformMCP(t, makeRestAPIStoredConfig(nil, nil))

	for routeKey, route := range rdc.Routes {
		assert.Empty(t, route.ResolverName, "route %q", routeKey)
		assert.Contains(t, rdc.PolicyChains, routeKey,
			"a non-MCP route keeps its route-keyed chain")
	}
	assert.NoError(t, rdc.ValidateResolution())
}

// A config whose kind says MCP but whose original configuration is missing must fail
// loudly. Falling back to no resolver would deploy an MCP proxy whose tool policies never
// learn which tool was invoked — traffic that looks healthy until someone audits it.
func TestMCPResolution_MCPKindWithoutSourceConfigIsRefused(t *testing.T) {
	cfg := makeMCPStoredConfig(t, "2025-06-18")
	cfg.SourceConfiguration = nil

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	_, err := transformer.Transform(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MCPProxyConfiguration")
}

// ─── Only the multiplexed route is resolver-bearing ──────────────────────────

// Only the multiplexed POST route resolves a body, so only its chain is filed under the composed
// key. GET and DELETE /mcp stay route-keyed, and a policy on them reads no resolver off the
// request and parses the body for itself.
func TestMCPResolution_OnlyTheMultiplexedRouteIsResolverBearing(t *testing.T) {
	rdc := transformMCP(t, makeMCPStoredConfig(t, "2025-06-18"))

	_, post := findRoute(t, rdc, "POST", "/mcp")
	getKey, get := findRoute(t, rdc, "GET", "/mcp")

	assert.Equal(t, MCPResolverName, post.ResolverName, "the multiplexed POST route carries the resolver")
	assert.Empty(t, get.ResolverName, "GET /mcp is route-keyed")

	require.NotNil(t, rdc.PolicyChains[rdc.ChainKeyFor(post.Vhost, mcpResolverOperation)],
		"the POST chain is filed under the composed operation key")
	require.NotNil(t, rdc.PolicyChains[getKey], "GET /mcp keeps its route key")
}
