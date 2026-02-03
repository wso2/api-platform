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
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

// tatEntry stores TAT with expiration time
type tatEntry struct {
	tat        time.Time
	expiration time.Time
}

// MemoryLimiter implements GCRA rate limiting with in-memory storage
type MemoryLimiter struct {
	data      map[string]*tatEntry
	policy    *Policy
	mu        sync.RWMutex
	clock     limiter.Clock
	cleanup   *time.Ticker
	done      chan struct{}
	closeOnce sync.Once
}

// NewMemoryLimiter creates a new in-memory GCRA rate limiter
// policy: Rate limit policy defining limits and burst capacity
// cleanupInterval: How often expired entries are removed (0 to disable, recommended: 1 minute)
func NewMemoryLimiter(policy *Policy, cleanupInterval time.Duration) *MemoryLimiter {
	m := &MemoryLimiter{
		data:   make(map[string]*tatEntry),
		policy: policy,
		clock:  &limiter.SystemClock{},
		done:   make(chan struct{}),
	}

	// Start cleanup goroutine if cleanup interval is specified
	if cleanupInterval > 0 {
		m.cleanup = time.NewTicker(cleanupInterval)
		go m.cleanupLoop()
	}

	return m
}

// WithClock sets a custom clock (for testing)
func (m *MemoryLimiter) WithClock(clock limiter.Clock) *MemoryLimiter {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clock = clock
	return m
}

// Allow checks if a single request is allowed for the given key
func (m *MemoryLimiter) Allow(ctx context.Context, key string) (*limiter.Result, error) {
	return m.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed for the given key
// Atomically consumes N request tokens if allowed
func (m *MemoryLimiter) AllowN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock.Now()

	slog.Debug("GCRA: checking rate limit",
		"key", key,
		"cost", n,
		"now", now,
		"limit", m.policy.Limit,
		"burst", m.policy.Burst)

	// Get current TAT (Theoretical Arrival Time) from map
	var tat time.Time
	entry, exists := m.data[key]
	if exists && now.Before(entry.expiration) {
		tat = entry.tat
	} else {
		tat = now
	}

	// GCRA Algorithm Step 1: TAT = max(TAT, now)
	if tat.Before(now) {
		tat = now
	}

	// GCRA Algorithm Step 2: Calculate emission interval and burst allowance
	emissionInterval := m.policy.EmissionInterval()
	burstAllowance := m.policy.BurstAllowance()

	slog.Debug("GCRA: calculated parameters",
		"key", key,
		"tat", tat,
		"emissionInterval", emissionInterval,
		"burstAllowance", burstAllowance)

	// GCRA Algorithm Step 3: Calculate the earliest time this request can be allowed
	allowAt := tat.Add(-burstAllowance)

	// GCRA Algorithm Step 4: Check if request is allowed
	// Allow if now >= allowAt (i.e., deny only if now < allowAt)
	if now.Before(allowAt) {
		// Request denied - calculate retry after
		retryAfter := allowAt.Sub(now)
		remaining := m.calculateRemaining(tat, now, emissionInterval, burstAllowance)

		slog.Debug("GCRA: request denied",
			"key", key,
			"now", now,
			"allowAt", allowAt,
			"retryAfter", retryAfter,
			"remaining", remaining)

		// Full quota available when TAT <= now
		fullQuotaAt := tat
		if tat.Before(now) {
			fullQuotaAt = now
		}

		return &limiter.Result{
			Allowed:     false,
			Limit:       m.policy.Limit,
			Remaining:   remaining,
			Reset:       tat,
			RetryAfter:  retryAfter,
			FullQuotaAt: fullQuotaAt,
			Duration:    m.policy.Duration,
			Policy:      m.policy,
		}, nil
	}

	// Additional check: ensure we have enough capacity for N requests
	remaining := m.calculateRemaining(tat, now, emissionInterval, burstAllowance)
	if n > remaining {
		// Not enough capacity
		fullQuotaAt := tat
		if tat.Before(now) {
			fullQuotaAt = now
		}
		return &limiter.Result{
			Allowed:     false,
			Limit:       m.policy.Limit,
			Remaining:   remaining,
			Reset:       tat,
			RetryAfter:  0, // Can't provide meaningful retry time for batch requests
			FullQuotaAt: fullQuotaAt,
			Duration:    m.policy.Duration,
			Policy:      m.policy,
		}, nil
	}

	// GCRA Algorithm Step 5: Request allowed - update TAT
	newTAT := tat.Add(emissionInterval * time.Duration(n))

	// Store new TAT with expiration (skip for peek operations where n=0)
	if n > 0 {
		expiration := m.policy.Duration + burstAllowance
		m.data[key] = &tatEntry{
			tat:        newTAT,
			expiration: now.Add(expiration),
		}
	}

	// GCRA Algorithm Step 6: Calculate remaining requests
	remaining = m.calculateRemaining(newTAT, now, emissionInterval, burstAllowance)

	slog.Debug("GCRA: request allowed",
		"key", key,
		"cost", n,
		"newTAT", newTAT,
		"remaining", remaining)

	// Full quota available when newTAT <= now
	fullQuotaAt := newTAT
	if newTAT.Before(now) {
		fullQuotaAt = now
	}

	return &limiter.Result{
		Allowed:     true,
		Limit:       m.policy.Limit,
		Remaining:   remaining,
		Reset:       newTAT,
		RetryAfter:  0,
		FullQuotaAt: fullQuotaAt,
		Duration:    m.policy.Duration,
		Policy:      m.policy,
	}, nil
}

// ConsumeN always consumes N tokens for the given key, regardless of whether
// it would exceed the limit. This is used for post-response cost extraction
// where the upstream has already processed the request.
func (m *MemoryLimiter) ConsumeN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock.Now()

	slog.Debug("GCRA: force consuming tokens",
		"key", key,
		"cost", n,
		"now", now,
		"limit", m.policy.Limit,
		"burst", m.policy.Burst)

	// Get current TAT (Theoretical Arrival Time) from map
	var tat time.Time
	entry, exists := m.data[key]
	if exists && now.Before(entry.expiration) {
		tat = entry.tat
	} else {
		tat = now
	}

	// GCRA Algorithm: TAT = max(TAT, now)
	if tat.Before(now) {
		tat = now
	}

	emissionInterval := m.policy.EmissionInterval()
	burstAllowance := m.policy.BurstAllowance()

	// Calculate new TAT (always advance, regardless of limits)
	newTAT := tat.Add(emissionInterval * time.Duration(n))

	// Always store new TAT (unlike AllowN which only updates when allowed)
	if n > 0 {
		expiration := m.policy.Duration + burstAllowance
		m.data[key] = &tatEntry{
			tat:        newTAT,
			expiration: now.Add(expiration),
		}
	}

	// Calculate remaining and check if allowed
	remaining := m.calculateRemaining(newTAT, now, emissionInterval, burstAllowance)
	allowAt := tat.Add(-burstAllowance)
	allowed := !now.Before(allowAt) && n <= m.policy.Burst

	slog.Debug("GCRA: tokens consumed",
		"key", key,
		"cost", n,
		"allowed", allowed,
		"newTAT", newTAT,
		"remaining", remaining)

	// Full quota available when newTAT <= now
	fullQuotaAt := newTAT
	if newTAT.Before(now) {
		fullQuotaAt = now
	}

	result := &limiter.Result{
		Allowed:     allowed,
		Limit:       m.policy.Limit,
		Remaining:   remaining,
		Reset:       newTAT,
		FullQuotaAt: fullQuotaAt,
		Duration:    m.policy.Duration,
		Policy:      m.policy,
	}

	if !allowed {
		result.RetryAfter = allowAt.Sub(now)
		if result.RetryAfter < 0 {
			result.RetryAfter = 0
		}
	}

	return result, nil
}

// GetAvailable returns the available tokens for the given key without consuming
func (m *MemoryLimiter) GetAvailable(ctx context.Context, key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := m.clock.Now()
	emissionInterval := m.policy.EmissionInterval()
	burstAllowance := m.policy.BurstAllowance()

	// Get the Theoretical Arrival Time (TAT) for this key
	tat, exists := m.data[key]
	if !exists || now.After(tat.expiration) {
		// No previous request or expired - full burst capacity available
		return m.policy.Burst, nil
	}

	// Calculate remaining based on current TAT
	remaining := m.calculateRemaining(tat.tat, now, emissionInterval, burstAllowance)
	return remaining, nil
}

// calculateRemaining computes how many requests can still be made
// Formula: remaining = burst - ceil((tat - now) / emissionInterval)
func (m *MemoryLimiter) calculateRemaining(tat, now time.Time, emissionInterval, burstAllowance time.Duration) int64 {
	if tat.Before(now) || tat.Equal(now) {
		// All burst capacity available
		return m.policy.Burst
	}

	usedBurst := tat.Sub(now)
	if usedBurst > burstAllowance {
		return 0
	}

	remaining := m.policy.Burst - int64(math.Ceil(float64(usedBurst)/float64(emissionInterval)))
	if remaining < 0 {
		return 0
	}

	return remaining
}

// cleanupLoop removes expired entries periodically
func (m *MemoryLimiter) cleanupLoop() {
	for {
		select {
		case <-m.cleanup.C:
			m.removeExpired()
		case <-m.done:
			return
		}
	}
}

// removeExpired deletes expired entries
func (m *MemoryLimiter) removeExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock.Now()
	for key, entry := range m.data {
		if now.After(entry.expiration) {
			delete(m.data, key)
		}
	}
}

// Close stops the cleanup goroutine and releases resources
// Safe to call multiple times
func (m *MemoryLimiter) Close() error {
	m.closeOnce.Do(func() {
		close(m.done)
		if m.cleanup != nil {
			m.cleanup.Stop()
		}
	})
	return nil
}
