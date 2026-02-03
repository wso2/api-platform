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

package gcra

import (
	"context"
	"fmt"

	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

// MultiLimiter supports multiple concurrent rate limit policies
// It checks all limiters and returns the most restrictive result
type MultiLimiter struct {
	limiters []limiter.Limiter
}

// NewMultiLimiter creates a limiter that enforces multiple policies
// Each policy is checked independently, and the most restrictive result is returned
// Example: Combine a short-term (10/second) and long-term (1000/hour) rate limit
func NewMultiLimiter(limiters ...limiter.Limiter) *MultiLimiter {
	return &MultiLimiter{limiters: limiters}
}

// Allow checks if a single request is allowed against all policies
// Returns the most restrictive result (fail-fast on first denial)
func (m *MultiLimiter) Allow(ctx context.Context, key string) (*limiter.Result, error) {
	return m.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed against all policies
// Returns the most restrictive result (fail-fast on first denial)
func (m *MultiLimiter) AllowN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	if len(m.limiters) == 0 {
		return nil, fmt.Errorf("no limiters configured")
	}

	var mostRestrictive *limiter.Result

	for i, limiter := range m.limiters {
		// Create policy-specific key to separate TAT tracking
		policyKey := fmt.Sprintf("%s:p%d", key, i)

		result, err := limiter.AllowN(ctx, policyKey, n)
		if err != nil {
			return nil, fmt.Errorf("limiter %d failed: %w", i, err)
		}

		// Track the most restrictive result
		if mostRestrictive == nil {
			mostRestrictive = result
		} else if !result.Allowed {
			// If denied, this is more restrictive
			mostRestrictive = result
		} else if result.Remaining < mostRestrictive.Remaining {
			// If fewer remaining, this is more restrictive
			mostRestrictive = result
		}

		// Fail-fast: if any policy denies, return immediately
		if !result.Allowed {
			return result, nil
		}
	}

	return mostRestrictive, nil
}

// ConsumeN always consumes N tokens against all policies, regardless of limits.
// This is used for post-response cost extraction where the upstream has already
// processed the request. Returns the most restrictive result.
func (m *MultiLimiter) ConsumeN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	if len(m.limiters) == 0 {
		return nil, fmt.Errorf("no limiters configured")
	}

	var mostRestrictive *limiter.Result

	for i, lim := range m.limiters {
		// Create policy-specific key to separate TAT tracking
		policyKey := fmt.Sprintf("%s:p%d", key, i)

		result, err := lim.ConsumeN(ctx, policyKey, n)
		if err != nil {
			return nil, fmt.Errorf("limiter %d failed: %w", i, err)
		}

		// Track the most restrictive result
		if mostRestrictive == nil {
			mostRestrictive = result
		} else if !result.Allowed {
			// If denied, this is more restrictive
			mostRestrictive = result
		} else if result.Remaining < mostRestrictive.Remaining {
			// If fewer remaining, this is more restrictive
			mostRestrictive = result
		}
	}

	return mostRestrictive, nil
}

// GetAvailable returns the minimum available tokens across all policies
func (m *MultiLimiter) GetAvailable(ctx context.Context, key string) (int64, error) {
	if len(m.limiters) == 0 {
		return 0, fmt.Errorf("no limiters configured")
	}

	var minAvailable int64 = -1

	for i, limiter := range m.limiters {
		// Create policy-specific key to separate TAT tracking
		policyKey := fmt.Sprintf("%s:p%d", key, i)

		available, err := limiter.GetAvailable(ctx, policyKey)
		if err != nil {
			return 0, fmt.Errorf("limiter %d failed: %w", i, err)
		}

		if minAvailable == -1 || available < minAvailable {
			minAvailable = available
		}
	}

	return minAvailable, nil
}

// Close closes all limiters
// Safe to call multiple times
func (m *MultiLimiter) Close() error {
	var firstErr error
	for i, limiter := range m.limiters {
		if err := limiter.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close limiter %d: %w", i, err)
		}
	}
	return firstErr
}
