package fixedwindow

import (
	"context"
	"fmt"

	"github.com/policy-engine/policies/ratelimit/limiter"
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
		// Create policy-specific key to separate window tracking
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
