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
	_ "embed"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
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
	if keyPrefix == "" {
		keyPrefix = "ratelimit:v1:"
	}

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

	slog.Debug("GCRA(Redis): checking rate limit",
		"key", key,
		"fullKey", fullKey,
		"cost", n,
		"now", now)

	emissionInterval := r.policy.EmissionInterval()
	burstAllowance := r.policy.BurstAllowance()
	expirationSeconds := int64((r.policy.Duration + burstAllowance).Seconds())

	slog.Debug("GCRA(Redis): executing Lua script",
		"key", key,
		"fullKey", fullKey,
		"emissionInterval", emissionInterval,
		"burstAllowance", burstAllowance,
		"burst", r.policy.Burst)

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

	slog.Debug("GCRA(Redis): script execution result",
		"key", key,
		"fullKey", fullKey,
		"allowed", allowed,
		"remaining", remaining,
		"reset", time.Unix(0, resetNanos))

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

// ConsumeN always consumes N tokens for the given key, regardless of whether
// it would exceed the limit. This is used for post-response cost extraction
// where the upstream has already processed the request.
// Note: For Redis GCRA, we can reuse AllowN since the Lua script already
// handles consumption atomically (tokens are always consumed on success).
func (r *RedisLimiter) ConsumeN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	// For GCRA Redis, AllowN already consumes atomically
	// TODO: Consider adding a force-consume Lua script variant for true overage handling
	return r.AllowN(ctx, key, n)
}

// GetAvailable returns the available tokens for the given key without consuming
// For GCRA, we use a Lua script to compute remaining without updating state
func (r *RedisLimiter) GetAvailable(ctx context.Context, key string) (int64, error) {
	now := r.clock.Now()
	emissionInterval := r.policy.EmissionInterval()
	burstAllowance := r.policy.BurstAllowance()

	fullKey := r.keyPrefix + key

	// Get current TAT from Redis
	tatBytes, err := r.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		// No previous request - full burst capacity available
		return r.policy.Burst, nil
	} else if err != nil {
		return 0, fmt.Errorf("redis get failed: %w", err)
	}

	tatNanos, err := strconv.ParseInt(string(tatBytes), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse TAT: %w", err)
	}

	tat := time.Unix(0, tatNanos)

	// Calculate remaining capacity without modifying TAT
	remaining := calculateRemainingGCRA(tat, now, emissionInterval, burstAllowance, r.policy.Burst)
	return remaining, nil
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

// calculateRemainingGCRA computes how many requests can still be made
// Formula: remaining = burst - ceil((tat - now) / emissionInterval)
func calculateRemainingGCRA(tat, now time.Time, emissionInterval, burstAllowance time.Duration, burst int64) int64 {
	if tat.Before(now) || tat.Equal(now) {
		// All burst capacity available
		return burst
	}

	usedBurst := tat.Sub(now)
	if usedBurst > burstAllowance {
		return 0
	}

	remaining := burst - int64(math.Ceil(float64(usedBurst)/float64(emissionInterval)))
	if remaining < 0 {
		return 0
	}

	return remaining
}
