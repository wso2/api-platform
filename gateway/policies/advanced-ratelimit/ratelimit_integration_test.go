/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ratelimit

import (
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// TestSharedQuotaLimiterCleanup tests that API-scoped quota limiters are not
// incorrectly deleted when one of multiple routes sharing the limiter is reconfigured.
// This reproduces the bug where cleanup of one route's quota would delete a shared
// limiter still in use by another route.
func TestSharedQuotaLimiterCleanup(t *testing.T) {
	// Clear caches before test to ensure clean state
	clearCaches()

	// Common API name for API-scoped quotas
	apiName := "test-api"

	// Create Route 1 with API-scoped quota (apiname key extraction without routename)
	metadata1 := policy.PolicyMetadata{
		RouteName:  "route-1",
		APIName:    apiName,
		APIVersion: "v1",
	}

	params1 := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "api-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(10),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "apiname",
					},
				},
			},
		},
	}

	// Create policy for Route 1
	policy1, err := GetPolicy(metadata1, params1)
	if err != nil {
		t.Fatalf("Failed to create policy for route-1: %v", err)
	}

	rlPolicy1, ok := policy1.(*RateLimitPolicy)
	if !ok {
		t.Fatalf("Expected *RateLimitPolicy, got %T", policy1)
	}

	if len(rlPolicy1.quotas) != 1 {
		t.Fatalf("Expected 1 quota, got %d", len(rlPolicy1.quotas))
	}

	// Get the limiter from route 1
	limiter1 := rlPolicy1.quotas[0].Limiter
	if limiter1 == nil {
		t.Fatal("Expected non-nil limiter for route-1")
	}

	// Create Route 2 with the SAME API-scoped quota configuration
	// This should reuse the same limiter due to API-scoped caching
	metadata2 := policy.PolicyMetadata{
		RouteName:  "route-2",
		APIName:    apiName,
		APIVersion: "v1",
	}

	params2 := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "api-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(10),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "apiname",
					},
				},
			},
		},
	}

	// Create policy for Route 2
	policy2, err := GetPolicy(metadata2, params2)
	if err != nil {
		t.Fatalf("Failed to create policy for route-2: %v", err)
	}

	rlPolicy2, ok := policy2.(*RateLimitPolicy)
	if !ok {
		t.Fatalf("Expected *RateLimitPolicy, got %T", policy2)
	}

	if len(rlPolicy2.quotas) != 1 {
		t.Fatalf("Expected 1 quota, got %d", len(rlPolicy2.quotas))
	}

	// Get the limiter from route 2
	limiter2 := rlPolicy2.quotas[0].Limiter
	if limiter2 == nil {
		t.Fatal("Expected non-nil limiter for route-2")
	}

	// Verify both routes share the same limiter (same pointer)
	if limiter1 != limiter2 {
		t.Error("Expected route-1 and route-2 to share the same limiter for API-scoped quota")
	}

	// Now simulate Route 1 being reconfigured WITHOUT the shared api-quota
	// (but with a different route-scoped quota instead)
	// This should NOT delete the shared limiter since Route 2 still uses it
	params1Updated := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "route-specific-quota", // Different quota, not shared
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(5),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename", // Route-scoped, not API-scoped
					},
				},
			},
		},
	}

	// This will trigger cleanup for route-1's old quotas (removing api-quota)
	_, err = GetPolicy(metadata1, params1Updated)
	if err != nil {
		t.Fatalf("Failed to update policy for route-1: %v", err)
	}

	// Verify the shared limiter still exists (not deleted)
	// We can check this by creating a new route that should reuse the same limiter
	metadata3 := policy.PolicyMetadata{
		RouteName:  "route-3",
		APIName:    apiName,
		APIVersion: "v1",
	}

	params3 := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "api-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(10),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "apiname",
					},
				},
			},
		},
	}

	policy3, err := GetPolicy(metadata3, params3)
	if err != nil {
		t.Fatalf("Failed to create policy for route-3: %v", err)
	}

	rlPolicy3, ok := policy3.(*RateLimitPolicy)
	if !ok {
		t.Fatalf("Expected *RateLimitPolicy, got %T", policy3)
	}

	// Route 3 should still get the SAME limiter (not a new one)
	// This proves the limiter wasn't deleted when route-1 was reconfigured
	limiter3 := rlPolicy3.quotas[0].Limiter
	if limiter3 != limiter1 {
		t.Error("Shared limiter was incorrectly deleted when route-1 was reconfigured. " +
			"Route-3 should have reused the same limiter that route-2 is still using.")
	}

	// Now remove the shared quota from route-2 as well (replace with route-scoped quota)
	params2Updated := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "route-2-specific-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(5),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename",
					},
				},
			},
		},
	}

	_, err = GetPolicy(metadata2, params2Updated)
	if err != nil {
		t.Fatalf("Failed to update policy for route-2: %v", err)
	}

	// Finally remove the shared quota from route-3 as well
	// This should delete the limiter since no routes are using it anymore
	params3Updated := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "route-3-specific-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(5),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename",
					},
				},
			},
		},
	}

	_, err = GetPolicy(metadata3, params3Updated)
	if err != nil {
		t.Fatalf("Failed to update policy for route-3: %v", err)
	}
}

// TestRouteScopedQuotaCleanup tests that route-scoped quotas are properly cleaned up
// when a route is reconfigured (no sharing between routes).
func TestRouteScopedQuotaCleanup(t *testing.T) {
	// Clear caches before test
	clearCaches()

	apiName := "test-api"

	// Create Route 1 with route-scoped quota (uses routename in key extraction)
	metadata1 := policy.PolicyMetadata{
		RouteName:  "route-1",
		APIName:    apiName,
		APIVersion: "v1",
	}

	params1 := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "route-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(10),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename", // Route-scoped
					},
				},
			},
		},
	}

	policy1, err := GetPolicy(metadata1, params1)
	if err != nil {
		t.Fatalf("Failed to create policy for route-1: %v", err)
	}

	rlPolicy1, ok := policy1.(*RateLimitPolicy)
	if !ok {
		t.Fatalf("Expected *RateLimitPolicy, got %T", policy1)
	}

	limiter1 := rlPolicy1.quotas[0].Limiter

	// Create Route 2 with route-scoped quota (different route = different limiter)
	metadata2 := policy.PolicyMetadata{
		RouteName:  "route-2",
		APIName:    apiName,
		APIVersion: "v1",
	}

	params2 := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "route-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(10),
						"duration": "1m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename", // Route-scoped
					},
				},
			},
		},
	}

	policy2, err := GetPolicy(metadata2, params2)
	if err != nil {
		t.Fatalf("Failed to create policy for route-2: %v", err)
	}

	rlPolicy2, ok := policy2.(*RateLimitPolicy)
	if !ok {
		t.Fatalf("Expected *RateLimitPolicy, got %T", policy2)
	}

	limiter2 := rlPolicy2.quotas[0].Limiter

	// Route-scoped quotas should NOT share limiters
	if limiter1 == limiter2 {
		t.Error("Route-scoped quotas should not share limiters between different routes")
	}

	// Remove the route-scoped quota from route-1 (replace with different quota)
	// This should delete its limiter since ref count will be 0
	params1Updated := map[string]interface{}{
		"backend":   "memory",
		"algorithm": "fixed-window",
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "route-1-other-quota",
				"limits": []interface{}{
					map[string]interface{}{
						"limit":    float64(20),
						"duration": "2m",
					},
				},
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename",
					},
				},
			},
		},
	}

	_, err = GetPolicy(metadata1, params1Updated)
	if err != nil {
		t.Fatalf("Failed to update policy for route-1: %v", err)
	}

}

// clearCaches resets all global caches for test isolation
func clearCaches() {
	globalLimiterCache.mu.Lock()
	defer globalLimiterCache.mu.Unlock()
	globalLimiterCache.byQuotaKey = make(map[string]*limiterEntry)
	globalLimiterCache.quotaKeysByBaseKey = make(map[string]map[string]struct{})
}

// getSharedQuotaRefCount retrieves the total reference count across all cached limiters.
// This is useful for verifying that shared limiters are correctly reference-counted.
func getSharedQuotaRefCount(apiName, quotaName string, limit int64, duration string) int {
	globalLimiterCache.mu.Lock()
	defer globalLimiterCache.mu.Unlock()

	var totalRefCount int
	for _, entry := range globalLimiterCache.byQuotaKey {
		if entry.refCount > 0 {
			totalRefCount += entry.refCount
		}
	}
	return totalRefCount
}

// getLimiterRefCountByInstance checks how many routes are referencing a specific limiter instance.
// This verifies that the same limiter pointer is being shared correctly.
func getLimiterRefCountByInstance(targetLimiter interface{}) int {
	globalLimiterCache.mu.Lock()
	defer globalLimiterCache.mu.Unlock()

	count := 0
	for _, entry := range globalLimiterCache.byQuotaKey {
		if entry.lim == targetLimiter {
			count += entry.refCount
		}
	}
	return count
}
