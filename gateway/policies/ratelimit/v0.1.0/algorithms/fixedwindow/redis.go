package fixedwindow

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/policy-engine/policies/ratelimit/limiter"
	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements fixed window rate limiting with Redis backend
type RedisLimiter struct {
	client    redis.UniversalClient
	policy    *Policy
	keyPrefix string
	clock     limiter.Clock
	closeOnce sync.Once
}

// NewRedisLimiter creates a new Redis-backed fixed window rate limiter
// client: Redis client for storage
// policy: Rate limit policy defining limit and window duration
// keyPrefix: Prefix for all Redis keys (e.g., "ratelimit:v1:")
func NewRedisLimiter(client redis.UniversalClient, policy *Policy, keyPrefix string) *RedisLimiter {
	if keyPrefix == "" {
		keyPrefix = "ratelimit:v1:"
	}

	return &RedisLimiter{
		client:    client,
		policy:    policy,
		keyPrefix: keyPrefix,
		clock:     &limiter.SystemClock{},
	}
}

// WithClock sets a custom clock (for testing)
func (r *RedisLimiter) WithClock(clock limiter.Clock) *RedisLimiter {
	r.clock = clock
	return r
}

// Allow checks if a single request is allowed for the given key
func (r *RedisLimiter) Allow(ctx context.Context, key string) (*limiter.Result, error) {
	return r.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed for the given key
// Uses atomic INCRBY in Redis - no Lua script needed
func (r *RedisLimiter) AllowN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	now := r.clock.Now()
	windowStart := r.policy.WindowStart(now)
	windowEnd := r.policy.WindowEnd(now)

	// Build window-specific key with timestamp
	// e.g., "ratelimit:v1:user123:1704067200000000000"
	redisKey := fmt.Sprintf("%s%s:%d", r.keyPrefix, key, windowStart.UnixNano())

	// Atomic increment - this is the core of fixed window
	newCount, err := r.client.IncrBy(ctx, redisKey, n).Result()
	if err != nil {
		return nil, fmt.Errorf("redis INCRBY failed: %w", err)
	}

	// Set TTL only on first request in window (when newCount == n)
	// This avoids calling EXPIRE on every request
	if newCount == n {
		// Add jitter (0-5s) to spread expiration load across Redis
		// Prevents "thundering herd" of expirations at window boundaries
		jitter := time.Duration(rand.Int63n(int64(5 * time.Second)))
		ttl := time.Until(windowEnd) + jitter

		// Set expiration - safe to ignore error as key already has data
		r.client.Expire(ctx, redisKey, ttl)
	}

	// Check if allowed
	allowed := newCount <= r.policy.Limit

	// Calculate remaining capacity
	var remaining int64
	if allowed {
		remaining = r.policy.Limit - newCount
	} else {
		remaining = 0
	}

	// Build result
	result := &limiter.Result{
		Allowed:   allowed,
		Limit:     r.policy.Limit,
		Remaining: remaining,
		Reset:     windowEnd,
		Duration:  r.policy.Duration,
		Policy:    r.policy,
	}

	// Set retry-after if denied
	if !allowed {
		result.RetryAfter = time.Until(windowEnd)
		if result.RetryAfter < 0 {
			result.RetryAfter = 0
		}
	}

	return result, nil
}

// Close releases resources (no-op for Redis as connections are managed externally)
// Safe to call multiple times
func (r *RedisLimiter) Close() error {
	r.closeOnce.Do(func() {
		// Redis client is managed externally, so we don't close it
		// This method exists to satisfy the Limiter interface
	})
	return nil
}
