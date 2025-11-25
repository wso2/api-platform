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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

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
				{Name: "auth", Version: "v1.0.0"},
				{Name: "rateLimit", Version: "v1.0.0"},
				{Name: "logging", Version: "v1.0.0"},
			},
			operationPolicies: nil,
			expectedOrder:     []string{"auth", "rateLimit", "logging"},
			description:       "Should preserve API-level policy declaration order",
		},
		{
			name:        "Operation-level policies only",
			apiPolicies: nil,
			operationPolicies: []api.Policy{
				{Name: "cors", Version: "v1.0.0"},
				{Name: "validation", Version: "v1.0.0"},
			},
			expectedOrder: []string{"cors", "validation"},
			description:   "Should preserve operation-level policy declaration order",
		},
		{
			name: "API policies with operation override",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1.0.0"},
				{Name: "rateLimit", Version: "v1.0.0"},
				{Name: "logging", Version: "v1.0.0"},
			},
			operationPolicies: []api.Policy{
				{Name: "auth", Version: "v2.0.0"}, // override with different version
			},
			expectedOrder: []string{"auth", "rateLimit", "logging"},
			description:   "Single override uses op version, then appends remaining API policies",
		},
		{
			name: "API policies with operation override and additional policies",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1.0.0"},
				{Name: "rateLimit", Version: "v1.0.0"},
				{Name: "logging", Version: "v1.0.0"},
			},
			operationPolicies: []api.Policy{
				{Name: "auth", Version: "v2.0.0"},       // override
				{Name: "cors", Version: "v1.0.0"},       // new
				{Name: "validation", Version: "v1.0.0"}, // new
			},
			expectedOrder: []string{"auth", "cors", "validation", "rateLimit", "logging"},
			description:   "Operation policies first in their order, then remaining API policies",
		},
		{
			name: "Multiple overrides",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1.0.0"},
				{Name: "rateLimit", Version: "v1.0.0"},
				{Name: "logging", Version: "v1.0.0"},
				{Name: "caching", Version: "v1.0.0"},
			},
			operationPolicies: []api.Policy{
				{Name: "rateLimit", Version: "v2.0.0"}, // override second
				{Name: "logging", Version: "v2.0.0"},   // override third
			},
			expectedOrder: []string{"rateLimit", "logging", "auth", "caching"},
			description:   "Operation policy order takes precedence, remaining API policies appended",
		},
		{
			name: "Complex scenario - mixed overrides and additions",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1.0.0"},
				{Name: "rateLimit", Version: "v1.0.0"},
				{Name: "logging", Version: "v1.0.0"},
			},
			operationPolicies: []api.Policy{
				{Name: "cors", Version: "v1.0.0"},       // new
				{Name: "auth", Version: "v2.0.0"},       // override
				{Name: "validation", Version: "v1.0.0"}, // new
			},
			expectedOrder: []string{"cors", "auth", "validation", "rateLimit", "logging"},
			description:   "Operation policies define order, remaining API policies appended",
		},
		{
			name: "Operation reorders API policies",
			apiPolicies: []api.Policy{
				{Name: "auth", Version: "v1.0.0"},
				{Name: "rateLimit", Version: "v1.0.0"},
				{Name: "logging", Version: "v1.0.0"},
			},
			operationPolicies: []api.Policy{
				{Name: "logging", Version: "v2.0.0"},   // third policy first
				{Name: "auth", Version: "v2.0.0"},      // first policy second
				{Name: "rateLimit", Version: "v2.0.0"}, // second policy third
			},
			expectedOrder: []string{"logging", "auth", "rateLimit"},
			description:   "Operation can completely reorder API policies for that specific operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a minimal StoredAPIConfig
			cfg := &models.StoredAPIConfig{
				Configuration: api.APIConfiguration{
					Version: api.APIConfigurationVersion("api-platform.wso2.com/v1"),
					Kind:    api.APIConfigurationKind("http/rest"),
					Data: api.APIConfigData{
						Name:    "test-api",
						Version: "v1.0",
						Context: "/test",
						Upstream: []api.Upstream{
							{Url: "http://backend.example.com"},
						},
						Operations: []api.Operation{
							{
								Method: "GET",
								Path:   "/resource",
							},
						},
					},
				},
			}

			// Set policies
			if tt.apiPolicies != nil {
				cfg.Configuration.Data.Policies = &tt.apiPolicies
			}
			if tt.operationPolicies != nil {
				cfg.Configuration.Data.Operations[0].Policies = &tt.operationPolicies
			}

			// Call the function
			result := buildStoredPolicyFromAPI(cfg)

			// Verify result is not nil when policies exist
			if len(tt.expectedOrder) > 0 {
				require.NotNil(t, result, tt.description)
				require.Len(t, result.Configuration.Routes, 1, "Should have one route")

				actualOrder := make([]string, len(result.Configuration.Routes[0].Policies))
				for i, p := range result.Configuration.Routes[0].Policies {
					actualOrder[i] = p.Name
				}

				assert.Equal(t, tt.expectedOrder, actualOrder, tt.description)

				// Verify the operation override is actually using the operation version
				if tt.name == "API policies with operation override" {
					// Find auth policy and verify it's v2.0.0
					for _, p := range result.Configuration.Routes[0].Policies {
						if p.Name == "auth" {
							assert.Equal(t, "v2.0.0", p.Version, "Override should use operation version")
						}
					}
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
		{Name: "auth", Version: "v1.0.0"},
		{Name: "rateLimit", Version: "v1.0.0"},
		{Name: "logging", Version: "v1.0.0"},
	}

	// Operation 1: Reorders API policies
	op1Policies := []api.Policy{
		{Name: "logging", Version: "v2.0.0"},
		{Name: "auth", Version: "v2.0.0"},
	}

	// Operation 2: Has its own exclusive policy
	op2Policies := []api.Policy{
		{Name: "validation", Version: "v1.0.0"},
	}

	// Operation 3: Overrides one and adds new
	op3Policies := []api.Policy{
		{Name: "rateLimit", Version: "v3.0.0"},
		{Name: "cors", Version: "v1.0.0"},
	}

	// Operation 4: No policies (should use API-level)
	// Operation 5: Different reordering
	op5Policies := []api.Policy{
		{Name: "rateLimit", Version: "v5.0.0"},
		{Name: "logging", Version: "v5.0.0"},
		{Name: "auth", Version: "v5.0.0"},
	}

	cfg := &models.StoredAPIConfig{
		Configuration: api.APIConfiguration{
			Version: api.APIConfigurationVersion("api-platform.wso2.com/v1"),
			Kind:    api.APIConfigurationKind("http/rest"),
			Data: api.APIConfigData{
				Name:    "test-api",
				Version: "v1.0",
				Context: "/test",
				Upstream: []api.Upstream{
					{Url: "http://backend.example.com"},
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
			},
		},
	}

	result := buildStoredPolicyFromAPI(cfg)
	require.NotNil(t, result)
	require.Len(t, result.Configuration.Routes, 5, "Should have 5 routes")

	// Expected orders for each operation
	expectedOrders := map[string][]string{
		"GET|v1.0|/test/resource1": {
			// op1: [logging(v2), auth(v2)] + remaining API [rateLimit]
			"logging", "auth", "rateLimit",
		},
		"POST|v1.0|/test/resource2": {
			// op2: [validation] + all API [auth, rateLimit, logging]
			"validation", "auth", "rateLimit", "logging",
		},
		"PUT|v1.0|/test/resource3": {
			// op3: [rateLimit(v3), cors] + remaining API [auth, logging]
			"rateLimit", "cors", "auth", "logging",
		},
		"DELETE|v1.0|/test/resource4": {
			// No op policies: use API order
			"auth", "rateLimit", "logging",
		},
		"PATCH|v1.0|/test/resource5": {
			// op5: [rateLimit(v5), logging(v5), auth(v5)] - all API policies covered
			"rateLimit", "logging", "auth",
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

		// Verify version overrides for specific routes
		switch route.RouteKey {
		case "GET|v1.0|/test/resource1":
			// Should have v2.0.0 for logging and auth
			for _, p := range route.Policies {
				if p.Name == "logging" || p.Name == "auth" {
					assert.Equal(t, "v2.0.0", p.Version,
						"Route GET should use operation version for %s", p.Name)
				}
			}
		case "PUT|v1.0|/test/resource3":
			// Should have v3.0.0 for rateLimit
			for _, p := range route.Policies {
				if p.Name == "rateLimit" {
					assert.Equal(t, "v3.0.0", p.Version,
						"Route PUT should use operation version for rateLimit")
				}
			}
		case "DELETE|v1.0|/test/resource4":
			// Should use API versions (v1.0.0) for all
			for _, p := range route.Policies {
				assert.Equal(t, "v1.0.0", p.Version,
					"Route DELETE should use API version for %s", p.Name)
			}
		case "PATCH|v1.0|/test/resource5":
			// Should have v5.0.0 for all three
			for _, p := range route.Policies {
				assert.Equal(t, "v5.0.0", p.Version,
					"Route PATCH should use operation version for %s", p.Name)
			}
		}
	}

	t.Log("All operations have independent policy configurations with correct ordering")
}

// TestPolicyOrderingConsistency runs the same configuration multiple times
// to ensure ordering is deterministic across multiple invocations
func TestPolicyOrderingConsistency(t *testing.T) {
	apiPolicies := []api.Policy{
		{Name: "auth", Version: "v1.0.0"},
		{Name: "rateLimit", Version: "v1.0.0"},
		{Name: "logging", Version: "v1.0.0"},
		{Name: "caching", Version: "v1.0.0"},
	}
	operationPolicies := []api.Policy{
		{Name: "cors", Version: "v1.0.0"},
		{Name: "auth", Version: "v2.0.0"},
		{Name: "validation", Version: "v1.0.0"},
	}

	cfg := &models.StoredAPIConfig{
		Configuration: api.APIConfiguration{
			Version: api.APIConfigurationVersion("api-platform.wso2.com/v1"),
			Kind:    api.APIConfigurationKind("http/rest"),
			Data: api.APIConfigData{
				Name:    "test-api",
				Version: "v1.0",
				Context: "/test",
				Upstream: []api.Upstream{
					{Url: "http://backend.example.com"},
				},
				Operations: []api.Operation{
					{
						Method:   "GET",
						Path:     "/resource",
						Policies: &operationPolicies,
					},
				},
				Policies: &apiPolicies,
			},
		},
	}

	// Run 100 times to catch any non-deterministic behavior
	var firstOrder []string
	for i := 0; i < 100; i++ {
		result := buildStoredPolicyFromAPI(cfg)
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
	// Operation policies: cors, auth, validation (in that order)
	// Remaining API policies not in operation: rateLimit, logging, caching
	expectedOrder := []string{"cors", "auth", "validation", "rateLimit", "logging", "caching"}
	assert.Equal(t, expectedOrder, firstOrder, "Final order should match expected")
}
