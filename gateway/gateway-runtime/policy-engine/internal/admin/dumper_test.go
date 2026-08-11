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

package admin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/chainkey"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// =============================================================================
// DumpConfig Tests
// =============================================================================

func TestDumpConfig_Empty(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	routes, _ := k.DumpRoutesAndSensitiveValues()
	result := DumpConfig(routes, k, reg, "pc-v1")

	require.NotNil(t, result)
	assert.False(t, result.Timestamp.IsZero())
	assert.Empty(t, result.PolicyRegistry.Policies)
	assert.Equal(t, 0, result.PolicyRegistry.TotalPolicies)
	assert.Empty(t, result.PolicyChains.PolicyChains)
	assert.Equal(t, 0, result.PolicyChains.TotalPolicyChains)
	assert.Equal(t, "pc-v1", result.XDSSync.PolicyChainVersion)
}

func TestDumpConfig_WithPolicies(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: map[string]*registry.PolicyEntry{
			"test-policy:v1": {
				Definition: &policy.PolicyDefinition{
					Name:    "test-policy",
					Version: "v1.0.0",
				},
			},
			"another-policy:v2": {
				Definition: &policy.PolicyDefinition{
					Name:    "another-policy",
					Version: "v2.0.0",
				},
			},
		},
	}

	routes, _ := k.DumpRoutesAndSensitiveValues()
	result := DumpConfig(routes, k, reg, "pc-v2")

	require.NotNil(t, result)
	assert.Equal(t, 2, result.PolicyRegistry.TotalPolicies)
	assert.Len(t, result.PolicyRegistry.Policies, 2)
	assert.Equal(t, "pc-v2", result.XDSSync.PolicyChainVersion)
}

func TestDumpConfig_WithRoutes(t *testing.T) {
	k := kernel.NewKernel()

	chain := &registry.PolicyChain{
		RequiresRequestBody:  true,
		RequiresResponseBody: false,
		PolicySpecs: []policy.PolicySpec{
			{Name: "policy-1", Version: "v1.0.0", Enabled: true},
		},
	}
	k.RegisterRoute("test-route", chain)

	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	routes, _ := k.DumpRoutesAndSensitiveValues()
	result := DumpConfig(routes, k, reg, "pc-v3")

	require.NotNil(t, result)
	assert.Equal(t, 1, result.PolicyChains.TotalPolicyChains)
	require.Len(t, result.PolicyChains.PolicyChains, 1)

	routeConfig := result.PolicyChains.PolicyChains[0]
	assert.Equal(t, "test-route", routeConfig.RouteKey)
	assert.True(t, routeConfig.RequiresRequestBody)
	assert.False(t, routeConfig.RequiresResponseBody)
	assert.Equal(t, 1, routeConfig.TotalPolicies)
	assert.Equal(t, "pc-v3", result.XDSSync.PolicyChainVersion)
}

// =============================================================================
// dumpPolicyRegistry Tests
// =============================================================================

func TestDumpPolicyRegistry_Empty(t *testing.T) {
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	result := dumpPolicyRegistry(reg)

	assert.Equal(t, 0, result.TotalPolicies)
	assert.Empty(t, result.Policies)
}

func TestDumpPolicyRegistry_Multiple(t *testing.T) {
	reg := &registry.PolicyRegistry{
		Policies: map[string]*registry.PolicyEntry{
			"auth-policy:v1": {
				Definition: &policy.PolicyDefinition{
					Name:    "auth-policy",
					Version: "v1.0.0",
				},
			},
			"rate-limit:v2": {
				Definition: &policy.PolicyDefinition{
					Name:    "rate-limit",
					Version: "v2.0.0",
				},
			},
		},
	}

	result := dumpPolicyRegistry(reg)

	assert.Equal(t, 2, result.TotalPolicies)
	assert.Len(t, result.Policies, 2)

	// Check that policy info is correct
	policyNames := make([]string, 0, 2)
	for _, p := range result.Policies {
		policyNames = append(policyNames, p.Name)
	}
	assert.Contains(t, policyNames, "auth-policy")
	assert.Contains(t, policyNames, "rate-limit")
}

// =============================================================================
// dumpPolicyChains Tests
// =============================================================================

func TestDumpPolicyChains_Empty(t *testing.T) {
	k := kernel.NewKernel()

	routes, _ := k.DumpRoutesAndSensitiveValues()
	result := dumpPolicyChains(routes)

	assert.Equal(t, 0, result.TotalPolicyChains)
	assert.Empty(t, result.PolicyChains)
}

func TestDumpPolicyChains_SingleRoute(t *testing.T) {
	k := kernel.NewKernel()

	condition := "request.method == 'GET'"
	chain := &registry.PolicyChain{
		RequiresRequestBody:  true,
		RequiresResponseBody: true,
		PolicySpecs: []policy.PolicySpec{
			{
				Name:               "test-policy",
				Version:            "v1.0.0",
				Enabled:            true,
				ExecutionCondition: &condition,
				Parameters:         policy.PolicyParameters{Raw: map[string]interface{}{"key": "value"}},
			},
		},
	}
	k.RegisterRoute("api-route-1", chain)

	routes, _ := k.DumpRoutesAndSensitiveValues()
	result := dumpPolicyChains(routes)

	assert.Equal(t, 1, result.TotalPolicyChains)
	require.Len(t, result.PolicyChains, 1)

	entry := result.PolicyChains[0]
	assert.Equal(t, "api-route-1", entry.RouteKey)
	assert.True(t, entry.RequiresRequestBody)
	assert.True(t, entry.RequiresResponseBody)
	assert.Equal(t, 1, entry.TotalPolicies)
	require.Len(t, entry.Policies, 1)

	policySpec := entry.Policies[0]
	assert.Equal(t, "test-policy", policySpec.Name)
	assert.Equal(t, "v1.0.0", policySpec.Version)
	assert.True(t, policySpec.Enabled)
	require.NotNil(t, policySpec.ExecutionCondition)
	assert.Equal(t, "request.method == 'GET'", *policySpec.ExecutionCondition)
}

func TestDumpPolicyChains_MultipleRoutes(t *testing.T) {
	k := kernel.NewKernel()

	chain1 := &registry.PolicyChain{
		PolicySpecs: []policy.PolicySpec{{Name: "p1", Version: "v1"}},
	}
	chain2 := &registry.PolicyChain{
		PolicySpecs: []policy.PolicySpec{{Name: "p2", Version: "v2"}, {Name: "p3", Version: "v3"}},
	}

	k.RegisterRoute("route-1", chain1)
	k.RegisterRoute("route-2", chain2)

	routes, _ := k.DumpRoutesAndSensitiveValues()
	result := dumpPolicyChains(routes)

	assert.Equal(t, 2, result.TotalPolicyChains)
	assert.Len(t, result.PolicyChains, 2)
}

// =============================================================================
// dumpRouteMetadata Tests
// =============================================================================

func TestDumpRouteMetadata_Empty(t *testing.T) {
	k := kernel.NewKernel()

	result := dumpRouteMetadata(k)

	assert.Equal(t, 0, result.TotalRoutes)
	assert.Empty(t, result.Routes)
}

func TestDumpRouteMetadata_WithRoutes(t *testing.T) {
	k := kernel.NewKernel()

	configs := map[string]*kernel.RouteConfig{
		"petstore|/pets|GET": {
			Metadata: kernel.RouteMetadata{
				APIId:                  "uuid-1",
				APIName:                "PetStore",
				APIVersion:             "v1",
				Context:                "/pets",
				OperationPath:          "/pets",
				Vhost:                  "default",
				APIKind:                "http/rest",
				DefaultUpstreamCluster: "petstore_cluster",
				UpstreamBasePath:       "/",
				UpstreamDefinitionPaths: map[string]string{
					"default": "/openapi.yaml",
				},
			},
		},
	}
	k.ApplyWholeRouteConfigs(configs)

	result := dumpRouteMetadata(k)

	assert.Equal(t, 1, result.TotalRoutes)
	require.Len(t, result.Routes, 1)

	entry := result.Routes[0]
	assert.Equal(t, "petstore|/pets|GET", entry.RouteKey)
	assert.Equal(t, "uuid-1", entry.APIId)
	assert.Equal(t, "PetStore", entry.APIName)
	assert.Equal(t, "v1", entry.APIVersion)
	assert.Equal(t, "/pets", entry.Context)
	assert.Equal(t, "/pets", entry.OperationPath)
	assert.Equal(t, "default", entry.Vhost)
	assert.Equal(t, "http/rest", entry.APIKind)
	assert.Equal(t, "petstore_cluster", entry.DefaultUpstreamCluster)
	assert.Equal(t, "/", entry.UpstreamBasePath)
	assert.Equal(t, map[string]string{"default": "/openapi.yaml"}, entry.UpstreamDefinitionPaths)
}

// =============================================================================
// dumpPolicySpecs Tests
// =============================================================================

func TestDumpPolicySpecs_Empty(t *testing.T) {
	result := dumpPolicySpecs([]policy.PolicySpec{})

	assert.Empty(t, result)
}

func TestDumpPolicySpecs_SingleSpec(t *testing.T) {
	condition := "true"
	specs := []policy.PolicySpec{
		{
			Name:               "auth-policy",
			Version:            "v1.0.0",
			Enabled:            true,
			ExecutionCondition: &condition,
			Parameters:         policy.PolicyParameters{Raw: map[string]interface{}{"audience": "api"}},
		},
	}

	result := dumpPolicySpecs(specs)

	require.Len(t, result, 1)
	assert.Equal(t, "auth-policy", result[0].Name)
	assert.Equal(t, "v1.0.0", result[0].Version)
	assert.True(t, result[0].Enabled)
	require.NotNil(t, result[0].ExecutionCondition)
	assert.Equal(t, "true", *result[0].ExecutionCondition)
	assert.Equal(t, map[string]interface{}{"audience": "api"}, result[0].Parameters)
}

func TestDumpPolicySpecs_DisabledPolicy(t *testing.T) {
	specs := []policy.PolicySpec{
		{
			Name:    "disabled-policy",
			Version: "v1.0.0",
			Enabled: false,
		},
	}

	result := dumpPolicySpecs(specs)

	require.Len(t, result, 1)
	assert.False(t, result[0].Enabled)
}

// =============================================================================
// dumpLazyResources Tests
// =============================================================================

func TestDumpLazyResources(t *testing.T) {
	result := dumpLazyResources()

	require.NotNil(t, result)
	// The lazy resource store is a singleton, so we just verify structure
	assert.NotNil(t, result.ResourcesByType)
}

// =============================================================================
// Policy chain resolution in the route dump
// =============================================================================

// An identity route — every kind shipping today — echoes its chain key and reports
// nothing else, so the dump is effectively unchanged for existing deployments.
func TestDumpRouteMetadata_IdentityRouteReportsChainKeyOnly(t *testing.T) {
	k := kernel.NewKernel()
	k.ApplyWholeRouteConfigs(map[string]*kernel.RouteConfig{
		"GET|/pets|localhost": {
			Metadata: kernel.RouteMetadata{APIName: "PetStore"},
			RouteResolution: resolver.RouteResolution{
				RouteKey:          "GET|/pets|localhost",
				CanonicalChainKey: "GET|/pets|localhost",
				ResolverName:      resolver.RouteKeyResolverName,
			},
		},
	})

	entry := dumpRouteMetadata(k).Routes[0]
	assert.Equal(t, "GET|/pets|localhost", entry.CanonicalChainKey)
	assert.Equal(t, resolver.RouteKeyResolverName, entry.ResolverName)
	assert.Empty(t, entry.ChainKeyPrefix, "an identity route composes nothing")
	assert.Zero(t, entry.MaxRequestBodyBytes,
		"the buffer limit only governs bodies buffered before the chain is known, which identity routes never do")
	assert.False(t, entry.ResolverPrepared)
}

// On a multiplexed route the dump is the only way to answer "why did this request get
// that chain?" from outside the process. Under composed keys there is no per-route
// mapping to show, so what an operator needs is the prefix the engine joins a resolved
// operation onto — enough to match a dumped chain key back to the route that reaches it.
func TestDumpRouteMetadata_MultiplexedRouteReportsChainKeyPrefix(t *testing.T) {
	k := kernel.NewKernel()
	k.ApplyWholeRouteConfigs(map[string]*kernel.RouteConfig{
		"POST|/rpc|localhost": {
			Metadata: kernel.RouteMetadata{APIName: "Assistant", APIId: "agent-1", Vhost: "localhost"},
			RouteResolution: resolver.RouteResolution{
				RouteKey:     "POST|/rpc|localhost",
				ResolverName: "a2a-jsonrpc",
				RouteState:   "compiled",
			},
		},
	})

	entry := dumpRouteMetadata(k).Routes[0]
	assert.Equal(t, "a2a-jsonrpc", entry.ResolverName)
	assert.True(t, entry.ResolverPrepared)

	// Built from the same helper as the keys themselves, so the dump cannot drift from
	// what is actually probed: a composed key for any operation starts with this.
	assert.Equal(t, chainkey.For("agent-1", "localhost", ""), entry.ChainKeyPrefix)
	assert.True(t, strings.HasPrefix(
		chainkey.For("agent-1", "localhost", "SendMessage"), entry.ChainKeyPrefix))

	// The default is resolved rather than reported as 0: an operator needs the bound
	// that actually applies, not the raw configured value.
	assert.Equal(t, kernel.DefaultMaxResolverRequestBodyBytes, entry.MaxRequestBodyBytes)
}

func TestDumpRouteMetadata_ExplicitBufferLimitIsReported(t *testing.T) {
	k := kernel.NewKernel()
	k.ApplyWholeRouteConfigs(map[string]*kernel.RouteConfig{
		"POST|/rpc|localhost": {
			RouteResolution: resolver.RouteResolution{
				RouteKey:     "POST|/rpc|localhost",
				ResolverName: "a2a-jsonrpc",
			},
			MaxRequestBodyBytes: 4096,
		},
	})

	assert.Equal(t, int64(4096), dumpRouteMetadata(k).Routes[0].MaxRequestBodyBytes)
}
