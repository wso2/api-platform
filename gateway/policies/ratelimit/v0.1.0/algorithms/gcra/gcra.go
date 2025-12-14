package gcra

import (
	"fmt"

	"github.com/policy-engine/policies/ratelimit/v0.1.0/limiter"
)

func init() {
	// Register GCRA algorithm with the factory
	limiter.RegisterAlgorithm("gcra", NewLimiter)
}

// NewLimiter creates a GCRA rate limiter based on the provided configuration
func NewLimiter(config limiter.Config) (limiter.Limiter, error) {
	// Convert generic limit configs to GCRA-specific Policy structs
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

// convertLimits converts generic LimitConfig to GCRA-specific Policy
func convertLimits(limits []limiter.LimitConfig) []*Policy {
	policies := make([]*Policy, len(limits))
	for i, limit := range limits {
		burst := limit.Burst
		if burst == 0 {
			// Default burst to limit if not specified
			burst = limit.Limit
		}
		policies[i] = NewPolicy(limit.Limit, limit.Duration, burst)
	}
	return policies
}
