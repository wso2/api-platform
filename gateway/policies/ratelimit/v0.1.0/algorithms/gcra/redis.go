package gcra

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/policy-engine/policies/ratelimit/v0.1.0/limiter"
	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements GCRA rate limiting with Redis backend
type RedisLimiter struct {
	client    redis.UniversalClient
	policy    *Policy
	script    *redis.Script
	keyPrefix string
	clock     limiter.Clock
	closeOnce sync.Once
}

//go:embed gcra.lua
var gcraLuaScript string

// NewRedisLimiter creates a new Redis-backed GCRA rate limiter
// client: Redis client (supports both redis.Client and redis.ClusterClient)
// policy: Rate limit policy defining limits and burst capacity
// keyPrefix: Prefix prepended to all keys (e.g., "ratelimit:v1:")
func NewRedisLimiter(client redis.UniversalClient, policy *Policy, keyPrefix string) *RedisLimiter {
	return &RedisLimiter{
		client:    client,
		policy:    policy,
		keyPrefix: keyPrefix,
		script:    redis.NewScript(gcraLuaScript),
		clock:     &limiter.SystemClock{},
	}
}

// Allow checks if a single request is allowed for the given key
func (r *RedisLimiter) Allow(ctx context.Context, key string) (*limiter.Result, error) {
	return r.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed for the given key
// Atomically consumes N request tokens if allowed
func (r *RedisLimiter) AllowN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	now := r.clock.Now()
	fullKey := r.keyPrefix + key

	emissionInterval := r.policy.EmissionInterval()
	burstAllowance := r.policy.BurstAllowance()
	expirationSeconds := int64((r.policy.Duration + burstAllowance).Seconds())

	// Execute Lua script atomically
	result, err := r.script.Run(ctx, r.client,
		[]string{fullKey},
		now.UnixNano(),                 // ARGV[1]: current time in nanoseconds
		emissionInterval.Nanoseconds(), // ARGV[2]: emission interval in nanoseconds
		burstAllowance.Nanoseconds(),   // ARGV[3]: burst allowance in nanoseconds
		r.policy.Burst,                 // ARGV[4]: burst capacity
		expirationSeconds,              // ARGV[5]: expiration in seconds
		n,                              // ARGV[6]: count (number of requests)
	).Result()

	if err != nil {
		// Handle NOSCRIPT error - script not loaded in Redis
		if strings.Contains(err.Error(), "NOSCRIPT") {
			// Load script and retry once
			_, loadErr := r.script.Load(ctx, r.client).Result()
			if loadErr != nil {
				return nil, fmt.Errorf("failed to load Lua script: %w", loadErr)
			}

			// Retry execution
			result, err = r.script.Run(ctx, r.client,
				[]string{fullKey},
				now.UnixNano(),
				emissionInterval.Nanoseconds(),
				burstAllowance.Nanoseconds(),
				r.policy.Burst,
				expirationSeconds,
				n,
			).Result()

			if err != nil {
				return nil, fmt.Errorf("script execution failed after load: %w", err)
			}
		} else {
			return nil, fmt.Errorf("script execution failed: %w", err)
		}
	}

	// Parse result from Lua script
	// Returns: {allowed, remaining, reset_nanos, retry_after_nanos, full_quota_at_nanos}
	values := result.([]interface{})

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)
	resetNanos := values[2].(int64)
	retryAfterNanos := values[3].(int64)
	fullQuotaAtNanos := values[4].(int64)

	return &limiter.Result{
		Allowed:     allowed,
		Limit:       r.policy.Limit,
		Remaining:   remaining,
		Reset:       time.Unix(0, resetNanos),
		RetryAfter:  time.Duration(retryAfterNanos),
		FullQuotaAt: time.Unix(0, fullQuotaAtNanos),
		Duration:    r.policy.Duration,
		Policy:      r.policy,
	}, nil
}

// Close closes the Redis connection
// Safe to call multiple times
func (r *RedisLimiter) Close() error {
	var err error
	r.closeOnce.Do(func() {
		err = r.client.Close()
	})
	return err
}
