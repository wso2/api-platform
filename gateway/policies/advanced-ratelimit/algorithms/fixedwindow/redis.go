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
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

// RedisLimiter implements fixed window rate limiting with Redis backend
type RedisLimiter struct {
	client    redis.UniversalClient
	policy    *Policy
	keyPrefix string
	clock     limiter.Clock
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

	slog.Debug("FixedWindow(Redis): checking rate limit",
		"key", key,
		"redisKey", redisKey,
		"cost", n,
		"windowStart", windowStart,
		"windowEnd", windowEnd)

	var newCount int64
	var err error

	// For peek operations (n=0), use GET to avoid creating keys or resetting TTL
	if n == 0 {
		val, getErr := r.client.Get(ctx, redisKey).Int64()
		if getErr == redis.Nil {
			// Key doesn't exist - no requests in this window yet
			newCount = 0
		} else if getErr != nil {
			return nil, fmt.Errorf("redis GET failed: %w", getErr)
		} else {
			newCount = val
		}
	} else {
		// Atomic increment - this is the core of fixed window
		newCount, err = r.client.IncrBy(ctx, redisKey, n).Result()
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

			// Set expiration and handle potential error to avoid keys without TTL
			if err := r.client.Expire(ctx, redisKey, ttl).Err(); err != nil {
				// Log with context; surface error so callers can decide (fail-open/closed)
				slog.Error("redis EXPIRE failed for rate limit key", "redisKey", redisKey, "ttl", ttl, "error", err)
				return nil, fmt.Errorf("redis EXPIRE failed for key %s ttl %s: %w", redisKey, ttl.String(), err)
			}
		}
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

	slog.Debug("FixedWindow(Redis): rate limit check result",
		"key", key,
		"redisKey", redisKey,
		"allowed", allowed,
		"newCount", newCount,
		"limit", r.policy.Limit,
		"remaining", remaining)

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

// ConsumeN always consumes N tokens for the given key, regardless of whether
// it would exceed the limit. This is used for post-response cost extraction
// where the upstream has already processed the request.
// Note: For Redis, INCRBY already atomically increments unconditionally,
// so this is the same as AllowN in terms of consumption behavior.
func (r *RedisLimiter) ConsumeN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	// Redis INCRBY already always increments, so we can reuse AllowN
	// The only difference is semantic: we're explicitly consuming even on overage
	return r.AllowN(ctx, key, n)
}

// GetAvailable returns the available tokens for the given key without consuming
func (r *RedisLimiter) GetAvailable(ctx context.Context, key string) (int64, error) {
	now := r.clock.Now()
	windowStart := r.policy.WindowStart(now)

	// Use Redis key with window start
	redisKey := fmt.Sprintf("%s%s:%d", r.keyPrefix, key, windowStart.UnixNano())

	// Get current count from Redis
	count, err := r.client.Get(ctx, redisKey).Int64()
	if err == redis.Nil {
		count = 0
	} else if err != nil {
		return 0, fmt.Errorf("redis get failed: %w", err)
	}

	// Calculate remaining
	remaining := r.policy.Limit - count
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// Close releases resources (no-op for Redis as connections are managed externally)
// Safe to call multiple times
func (r *RedisLimiter) Close() error {
	// Redis client is managed externally, so we don't close it
	// This method exists to satisfy the Limiter interface
	return nil
}
