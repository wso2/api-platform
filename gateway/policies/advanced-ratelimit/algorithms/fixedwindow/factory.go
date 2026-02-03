/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package fixedwindow

import (
	"fmt"

	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

func init() {
	// Register fixed-window algorithm with the factory
	limiter.RegisterAlgorithm("fixed-window", NewLimiter)
}

// NewLimiter creates a fixed window rate limiter based on the provided configuration
func NewLimiter(config limiter.Config) (limiter.Limiter, error) {
	// Convert generic limit configs to fixed-window-specific Policy structs
	policies := convertLimits(config.Limits)

	if len(policies) == 0 {
		return nil, fmt.Errorf("at least one limit must be specified")
	}

	// Create limiter based on backend
	if config.Backend == "redis" {
		if config.RedisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis backend")
		}

		if len(policies) == 1 {
			// Single limiter
			return NewRedisLimiter(config.RedisClient, policies[0], config.KeyPrefix), nil
		}

		// Multi-limiter for Redis
		limiters := make([]limiter.Limiter, len(policies))
		for i, policy := range policies {
			// Use different key prefix for each policy
			policyPrefix := fmt.Sprintf("%sp%d:", config.KeyPrefix, i)
			limiters[i] = NewRedisLimiter(config.RedisClient, policy, policyPrefix)
		}
		return NewMultiLimiter(limiters...), nil
	}

	// Memory backend
	if len(policies) == 1 {
		// Single limiter
		return NewMemoryLimiter(policies[0], config.CleanupInterval), nil
	}

	// Multi-limiter for memory
	limiters := make([]limiter.Limiter, len(policies))
	for i, policy := range policies {
		limiters[i] = NewMemoryLimiter(policy, config.CleanupInterval)
	}
	return NewMultiLimiter(limiters...), nil
}

// convertLimits converts generic LimitConfig to fixed-window-specific Policy
func convertLimits(limits []limiter.LimitConfig) []*Policy {
	policies := make([]*Policy, len(limits))
	for i, limit := range limits {
		// For fixed window, burst parameter is not used
		// The limit itself is the maximum per window
		policies[i] = NewPolicy(limit.Limit, limit.Duration)
	}
	return policies
}
