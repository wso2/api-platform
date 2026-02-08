/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// newTestAPIServer creates a minimal APIServer instance for testing.
// policyDefinitions allow resolving major-only (v1, v2, ...) to full semver for policy ordering tests.
func newTestAPIServer() *APIServer {
	vhosts := &config.VHostsConfig{
		Main:    config.VHostEntry{Default: "localhost"},
		Sandbox: config.VHostEntry{Default: "sandbox-*"},
	}
	defs := map[string]api.PolicyDefinition{
		"auth|v1.0.0": {Name: "auth", Version: "v1.0.0"},
		"auth|v2.0.0": {Name: "auth", Version: "v2.0.0"},
		"auth|v5.0.0": {Name: "auth", Version: "v5.0.0"},
		"rateLimit|v1.0.0": {Name: "rateLimit", Version: "v1.0.0"},
		"rateLimit|v2.0.0": {Name: "rateLimit", Version: "v2.0.0"},
		"rateLimit|v3.0.0": {Name: "rateLimit", Version: "v3.0.0"},
		"rateLimit|v5.0.0": {Name: "rateLimit", Version: "v5.0.0"},
		"logging|v1.0.0": {Name: "logging", Version: "v1.0.0"},
		"logging|v2.0.0": {Name: "logging", Version: "v2.0.0"},
		"logging|v5.0.0": {Name: "logging", Version: "v5.0.0"},
		"cors|v1.0.0": {Name: "cors", Version: "v1.0.0"},
		"validation|v1.0.0": {Name: "validation", Version: "v1.0.0"},
		"caching|v1.0.0": {Name: "caching", Version: "v1.0.0"},
	}
	return &APIServer{
		routerConfig:      &config.RouterConfig{GatewayHost: "localhost", VHosts: *vhosts},
		policyDefinitions: defs,
	}
}

// TestPolicyOrderingDeterministic verifies that policy merging produces deterministic ordering
// by preserving declaration order of API-level policies with operation-level overrides applied
func TestPolicyOrderingDeterministic(t *testing.T) {
	tests := []struct {
		name              string
		apiPolicies       []api.Policy
		operationPolicies []api.Policy
		expectedOrder     []string // expected policy names in order
		description       string
	}{
		{
			name: "API-level policies only",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1"},
				{Name: "rateLimit", Version: "v1"},
				{Name: "logging", Version: "v1"},
			},
			operationPolicies: nil,
			expectedOrder:     []string{"auth", "rateLimit", "logging"},
			description:       "Should preserve API-level policy declaration order",
		},
		{
			name:        "Operation-level policies only",
			apiPolicies: nil,
			operationPolicies: []api.Policy{
				{Name: "cors", Version: "v1"},
				{Name: "validation", Version: "v1"},
			},
			expectedOrder: []string{"cors", "validation"},
			description:   "Should preserve operation-level policy declaration order",
		},
		{
			name: "API policies with operation override",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1"},
				{Name: "rateLimit", Version: "v1"},
				{Name: "logging", Version: "v1"},
			},
			operationPolicies: []api.Policy{
				{Name: "auth", Version: "v2"}, // does not override, appends after API policies
			},
			expectedOrder: []string{"auth", "rateLimit", "logging", "auth"},
			description:   "API policies execute first, then operation policies (no override)",
		},
		{
			name: "API policies with operation override and additional policies",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1"},
				{Name: "rateLimit", Version: "v1"},
				{Name: "logging", Version: "v1"},
			},
			operationPolicies: []api.Policy{
				{Name: "auth", Version: "v2"},       // does not override, appends
				{Name: "cors", Version: "v1"},       // new
				{Name: "validation", Version: "v1"}, // new
			},
			expectedOrder: []string{"auth", "rateLimit", "logging", "auth", "cors", "validation"},
			description:   "API policies execute first, then operation policies in order (allows duplicates)",
		},
		{
			name: "Multiple overrides",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1"},
				{Name: "rateLimit", Version: "v1"},
				{Name: "logging", Version: "v1"},
				{Name: "caching", Version: "v1"},
			},
			operationPolicies: []api.Policy{
				{Name: "rateLimit", Version: "v2"}, // does not override, appends
				{Name: "logging", Version: "v2"},   // does not override, appends
			},
			expectedOrder: []string{"auth", "rateLimit", "logging", "caching", "rateLimit", "logging"},
			description:   "API policies execute first, then operation policies (allows duplicate policy names)",
		},
		{
			name: "Complex scenario - mixed overrides and additions",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1"},
				{Name: "rateLimit", Version: "v1"},
				{Name: "logging", Version: "v1"},
			},
			operationPolicies: []api.Policy{
				{Name: "cors", Version: "v1"},       // new
				{Name: "auth", Version: "v2"},       // does not override, appends
				{Name: "validation", Version: "v1"}, // new
			},
			expectedOrder: []string{"auth", "rateLimit", "logging", "cors", "auth", "validation"},
			description:   "API policies execute first, then operation policies in order",
		},
		{
			name: "Operation reorders API policies",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1"},
				{Name: "rateLimit", Version: "v1"},
				{Name: "logging", Version: "v1"},
			},
			operationPolicies: []api.Policy{
				{Name: "logging", Version: "v2"},   // appends after API policies
				{Name: "auth", Version: "v2"},      // appends after API policies
				{Name: "rateLimit", Version: "v2"}, // appends after API policies
			},
			expectedOrder: []string{"auth", "rateLimit", "logging", "logging", "auth", "rateLimit"},
			description:   "API policies execute first, operation policies append (execution order matters, not declaration)",
		},
	}
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "test-api",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a minimal StoredAPIConfig
			cfg := &models.StoredConfig{
				Configuration: api.APIConfiguration{
					ApiVersion: api.APIConfigurationApiVersion(api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1),
					Kind:       api.APIConfigurationKind(api.RestApi),
					Spec:       specUnion,
				},
			}

			apiData, err := cfg.Configuration.Spec.AsAPIConfigData()
			require.NoError(t, err)

			// Set policies
			if tt.apiPolicies != nil {
				apiData.Policies = &tt.apiPolicies
				cfg.Configuration.Spec.FromAPIConfigData(apiData)
			}
			if tt.operationPolicies != nil {
				apiData.Operations[0].Policies = &tt.operationPolicies
				cfg.Configuration.Spec.FromAPIConfigData(apiData)
			}

			// Call the function
			server := newTestAPIServer()
			result := server.buildStoredPolicyFromAPI(cfg) // Verify result is not nil when policies exist
			if len(tt.expectedOrder) > 0 {
				require.NotNil(t, result, tt.description)
				require.Len(t, result.Configuration.Routes, 1, "Should have one route")

				actualOrder := make([]string, len(result.Configuration.Routes[0].Policies))
				for i, p := range result.Configuration.Routes[0].Policies {
					actualOrder[i] = p.Name
				}

				assert.Equal(t, tt.expectedOrder, actualOrder, tt.description)

				// Verify policy versions for the append behavior test
				if tt.name == "API policies with operation override" {
					// Should have two auth policies: first from API (v1.0.0), second from operation (v2.0.0)
					authPolicies := []string{}
					for _, p := range result.Configuration.Routes[0].Policies {
						if p.Name == "auth" {
							authPolicies = append(authPolicies, p.Version)
						}
					}
					require.Len(t, authPolicies, 2, "Should have two auth policies (one from API, one from operation)")
					assert.Equal(t, "v1.0.0", authPolicies[0], "First auth should be from API level")
					assert.Equal(t, "v2.0.0", authPolicies[1], "Second auth should be from operation level")
				}
			} else {
				assert.Nil(t, result, "Should return nil when no policies")
			}
		})
	}
}

// TestMultipleOperationsIndependentPolicies verifies that each operation's policies
// are independent and don't interfere with each other
func TestMultipleOperationsIndependentPolicies(t *testing.T) {
	apiPolicies := []api.Policy{
		{Name: "auth", Version: "v1"},
		{Name: "rateLimit", Version: "v1"},
		{Name: "logging", Version: "v1"},
	}

	// Operation 1: Reorders API policies
	op1Policies := []api.Policy{
		{Name: "logging", Version: "v2"},
		{Name: "auth", Version: "v2"},
	}

	// Operation 2: Has its own exclusive policy
	op2Policies := []api.Policy{
		{Name: "validation", Version: "v1"},
	}

	// Operation 3: Overrides one and adds new
	op3Policies := []api.Policy{
		{Name: "rateLimit", Version: "v3"},
		{Name: "cors", Version: "v1"},
	}

	// Operation 4: No policies (should use API-level)
	// Operation 5: Different reordering
	op5Policies := []api.Policy{
		{Name: "rateLimit", Version: "v5"},
		{Name: "logging", Version: "v5"},
		{Name: "auth", Version: "v5"},
	}

	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "test-api",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method:   "GET",
				Path:     "/resource1",
				Policies: &op1Policies,
			},
			{
				Method:   "POST",
				Path:     "/resource2",
				Policies: &op2Policies,
			},
			{
				Method:   "PUT",
				Path:     "/resource3",
				Policies: &op3Policies,
			},
			{
				Method: "DELETE",
				Path:   "/resource4",
				// No policies
			},
			{
				Method:   "PATCH",
				Path:     "/resource5",
				Policies: &op5Policies,
			},
		},
		Policies: &apiPolicies,
	})

	cfg := &models.StoredConfig{
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersion(api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1),
			Kind:       api.APIConfigurationKind(api.RestApi),
			Spec:       specUnion,
		},
	}

	server := newTestAPIServer()
	result := server.buildStoredPolicyFromAPI(cfg)
	require.NotNil(t, result)
	require.Len(t, result.Configuration.Routes, 5, "Should have 5 routes")

	// Expected orders for each operation (API policies execute first, then operation policies)
	expectedOrders := map[string][]string{
		"GET|/test/resource1|localhost": {
			// API: [auth(v1), rateLimit(v1), logging(v1)] + op1: [logging(v2), auth(v2)]
			"auth", "rateLimit", "logging", "logging", "auth",
		},
		"POST|/test/resource2|localhost": {
			// API: [auth(v1), rateLimit(v1), logging(v1)] + op2: [validation]
			"auth", "rateLimit", "logging", "validation",
		},
		"PUT|/test/resource3|localhost": {
			// API: [auth(v1), rateLimit(v1), logging(v1)] + op3: [rateLimit(v3), cors]
			"auth", "rateLimit", "logging", "rateLimit", "cors",
		},
		"DELETE|/test/resource4|localhost": {
			// No op policies: API: [auth(v1), rateLimit(v1), logging(v1)]
			"auth", "rateLimit", "logging",
		},
		"PATCH|/test/resource5|localhost": {
			// API: [auth(v1), rateLimit(v1), logging(v1)] + op5: [rateLimit(v5), logging(v5), auth(v5)]
			"auth", "rateLimit", "logging", "rateLimit", "logging", "auth",
		},
	}

	// Verify each route independently
	for _, route := range result.Configuration.Routes {
		expectedOrder, exists := expectedOrders[route.RouteKey]
		require.True(t, exists, "Unexpected route key: %s", route.RouteKey)

		actualOrder := make([]string, len(route.Policies))
		for i, p := range route.Policies {
			actualOrder[i] = p.Name
		}

		assert.Equal(t, expectedOrder, actualOrder,
			"Route %s should have correct policy order", route.RouteKey)

		// Verify policy versions (API policies first with v1.0.0, then operation policies with their versions)
		switch route.RouteKey {
		case "GET|/test/resource1|localhost":
			// Expected: auth(v1), rateLimit(v1), logging(v1), logging(v2), auth(v2)
			require.Len(t, route.Policies, 5)
			assert.Equal(t, "v1.0.0", route.Policies[0].Version, "First auth should be API version")
			assert.Equal(t, "v1.0.0", route.Policies[1].Version, "rateLimit should be API version")
			assert.Equal(t, "v1.0.0", route.Policies[2].Version, "First logging should be API version")
			assert.Equal(t, "v2.0.0", route.Policies[3].Version, "Second logging should be operation version")
			assert.Equal(t, "v2.0.0", route.Policies[4].Version, "Second auth should be operation version")
		case "PUT|/test/resource3|localhost":
			// Expected: auth(v1), rateLimit(v1), logging(v1), rateLimit(v3), cors
			require.Len(t, route.Policies, 5)
			assert.Equal(t, "v1.0.0", route.Policies[0].Version, "auth should be API version")
			assert.Equal(t, "v1.0.0", route.Policies[1].Version, "First rateLimit should be API version")
			assert.Equal(t, "v1.0.0", route.Policies[2].Version, "logging should be API version")
			assert.Equal(t, "v3.0.0", route.Policies[3].Version, "Second rateLimit should be operation version")
		case "DELETE|/test/resource4|localhost":
			// Should use API versions (v1.0.0) for all
			for _, p := range route.Policies {
				assert.Equal(t, "v1.0.0", p.Version,
					"Route DELETE should use API version for %s", p.Name)
			}
		case "PATCH|/test/resource5|localhost":
			// Expected: auth(v1), rateLimit(v1), logging(v1), rateLimit(v5), logging(v5), auth(v5)
			require.Len(t, route.Policies, 6)
			assert.Equal(t, "v1.0.0", route.Policies[0].Version, "First auth should be API version")
			assert.Equal(t, "v1.0.0", route.Policies[1].Version, "First rateLimit should be API version")
			assert.Equal(t, "v1.0.0", route.Policies[2].Version, "First logging should be API version")
			assert.Equal(t, "v5.0.0", route.Policies[3].Version, "Second rateLimit should be operation version")
			assert.Equal(t, "v5.0.0", route.Policies[4].Version, "Second logging should be operation version")
			assert.Equal(t, "v5.0.0", route.Policies[5].Version, "Second auth should be operation version")
		}
	}

	t.Log("All operations have independent policy configurations with correct ordering")
}

// TestPolicyOrderingConsistency runs the same configuration multiple times
// to ensure ordering is deterministic across multiple invocations
func TestPolicyOrderingConsistency(t *testing.T) {
	apiPolicies := []api.Policy{
		{Name: "auth", Version: "v1"},
		{Name: "rateLimit", Version: "v1"},
		{Name: "logging", Version: "v1"},
		{Name: "caching", Version: "v1"},
	}
	operationPolicies := []api.Policy{
		{Name: "cors", Version: "v1"},
		{Name: "auth", Version: "v2"},
		{Name: "validation", Version: "v1"},
	}

	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "test-api",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method:   "GET",
				Path:     "/resource",
				Policies: &operationPolicies,
			},
		},
		Policies: &apiPolicies,
	})

	cfg := &models.StoredConfig{
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersion(api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1),
			Kind:       api.APIConfigurationKind(api.RestApi),
			Spec:       specUnion,
		},
	}

	// Run 100 times to catch any non-deterministic behavior
	var firstOrder []string
	server := newTestAPIServer()
	for i := 0; i < 100; i++ {
		result := server.buildStoredPolicyFromAPI(cfg)
		require.NotNil(t, result)
		require.Len(t, result.Configuration.Routes, 1)

		currentOrder := make([]string, len(result.Configuration.Routes[0].Policies))
		for j, p := range result.Configuration.Routes[0].Policies {
			currentOrder[j] = p.Name
		}

		if i == 0 {
			firstOrder = currentOrder
		} else {
			assert.Equal(t, firstOrder, currentOrder,
				"Policy order should be consistent across invocations (iteration %d)", i)
		}
	}

	// Verify the expected order
	// API policies execute first: auth(v1), rateLimit, logging, caching
	// Then operation policies append: cors, auth(v2), validation
	expectedOrder := []string{"auth", "rateLimit", "logging", "caching", "cors", "auth", "validation"}
	assert.Equal(t, expectedOrder, firstOrder, "Final order should match expected (API policies first, then operation policies)")
}
