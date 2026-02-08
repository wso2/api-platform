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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// DumpConfig Tests
// =============================================================================

func TestDumpConfig_Empty(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	result := DumpConfig(k, reg)

	require.NotNil(t, result)
	assert.False(t, result.Timestamp.IsZero())
	assert.Empty(t, result.PolicyRegistry.Policies)
	assert.Equal(t, 0, result.PolicyRegistry.TotalPolicies)
	assert.Empty(t, result.Routes.RouteConfigs)
	assert.Equal(t, 0, result.Routes.TotalRoutes)
}

func TestDumpConfig_WithPolicies(t *testing.T) {
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: map[string]*policy.PolicyDefinition{
			"test-policy:v1.0.0": {
				Name:    "test-policy",
				Version: "v1.0.0",
			},
			"another-policy:v2.0.0": {
				Name:    "another-policy",
				Version: "v2.0.0",
			},
		},
		Factories: make(map[string]policy.PolicyFactory),
	}

	result := DumpConfig(k, reg)

	require.NotNil(t, result)
	assert.Equal(t, 2, result.PolicyRegistry.TotalPolicies)
	assert.Len(t, result.PolicyRegistry.Policies, 2)
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
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	result := DumpConfig(k, reg)

	require.NotNil(t, result)
	assert.Equal(t, 1, result.Routes.TotalRoutes)
	require.Len(t, result.Routes.RouteConfigs, 1)
	
	routeConfig := result.Routes.RouteConfigs[0]
	assert.Equal(t, "test-route", routeConfig.RouteKey)
	assert.True(t, routeConfig.RequiresRequestBody)
	assert.False(t, routeConfig.RequiresResponseBody)
	assert.Equal(t, 1, routeConfig.TotalPolicies)
}

// =============================================================================
// dumpPolicyRegistry Tests
// =============================================================================

func TestDumpPolicyRegistry_Empty(t *testing.T) {
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
	}

	result := dumpPolicyRegistry(reg)

	assert.Equal(t, 0, result.TotalPolicies)
	assert.Empty(t, result.Policies)
}

func TestDumpPolicyRegistry_Multiple(t *testing.T) {
	reg := &registry.PolicyRegistry{
		Definitions: map[string]*policy.PolicyDefinition{
			"auth-policy:v1.0.0": {
				Name:    "auth-policy",
				Version: "v1.0.0",
			},
			"rate-limit:v2.0.0": {
				Name:    "rate-limit",
				Version: "v2.0.0",
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
// dumpRoutes Tests
// =============================================================================

func TestDumpRoutes_Empty(t *testing.T) {
	k := kernel.NewKernel()

	result := dumpRoutes(k)

	assert.Equal(t, 0, result.TotalRoutes)
	assert.Empty(t, result.RouteConfigs)
}

func TestDumpRoutes_SingleRoute(t *testing.T) {
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

	result := dumpRoutes(k)

	assert.Equal(t, 1, result.TotalRoutes)
	require.Len(t, result.RouteConfigs, 1)

	routeConfig := result.RouteConfigs[0]
	assert.Equal(t, "api-route-1", routeConfig.RouteKey)
	assert.True(t, routeConfig.RequiresRequestBody)
	assert.True(t, routeConfig.RequiresResponseBody)
	assert.Equal(t, 1, routeConfig.TotalPolicies)
	require.Len(t, routeConfig.Policies, 1)

	policySpec := routeConfig.Policies[0]
	assert.Equal(t, "test-policy", policySpec.Name)
	assert.Equal(t, "v1.0.0", policySpec.Version)
	assert.True(t, policySpec.Enabled)
	require.NotNil(t, policySpec.ExecutionCondition)
	assert.Equal(t, "request.method == 'GET'", *policySpec.ExecutionCondition)
}

func TestDumpRoutes_MultipleRoutes(t *testing.T) {
	k := kernel.NewKernel()
	
	chain1 := &registry.PolicyChain{
		PolicySpecs: []policy.PolicySpec{{Name: "p1", Version: "v1"}},
	}
	chain2 := &registry.PolicyChain{
		PolicySpecs: []policy.PolicySpec{{Name: "p2", Version: "v2"}, {Name: "p3", Version: "v3"}},
	}
	
	k.RegisterRoute("route-1", chain1)
	k.RegisterRoute("route-2", chain2)

	result := dumpRoutes(k)

	assert.Equal(t, 2, result.TotalRoutes)
	assert.Len(t, result.RouteConfigs, 2)
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
