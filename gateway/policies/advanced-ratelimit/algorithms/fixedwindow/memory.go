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
	"log/slog"
	"sync"
	"time"

	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

// windowEntry stores the request count and window start time
type windowEntry struct {
	count       int64
	windowStart time.Time
	expiration  time.Time
}

// MemoryLimiter implements fixed window rate limiting with in-memory storage
type MemoryLimiter struct {
	data      map[string]*windowEntry
	policy    *Policy
	mu        sync.RWMutex
	clock     limiter.Clock
	cleanup   *time.Ticker
	done      chan struct{}
	closeOnce sync.Once
}

// NewMemoryLimiter creates a new in-memory fixed window rate limiter
// policy: Rate limit policy defining limit and window duration
// cleanupInterval: How often expired entries are removed (0 to disable, recommended: 5 minutes)
func NewMemoryLimiter(policy *Policy, cleanupInterval time.Duration) *MemoryLimiter {
	m := &MemoryLimiter{
		data:   make(map[string]*windowEntry),
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
	windowStart := m.policy.WindowStart(now)
	windowEnd := m.policy.WindowEnd(now)

	slog.Debug("FixedWindow: checking rate limit",
		"key", key,
		"cost", n,
		"windowStart", windowStart,
		"windowEnd", windowEnd)

	// Get current entry or initialize new one
	entry, exists := m.data[key]

	// Reset count if we're in a new window or entry expired
	var currentCount int64
	if !exists || entry.windowStart != windowStart || now.After(entry.expiration) {
		currentCount = 0
	} else {
		currentCount = entry.count
	}

	// Check if request would exceed limit
	newCount := currentCount + n
	allowed := newCount <= m.policy.Limit

	// Calculate remaining capacity
	var remaining int64
	if allowed {
		remaining = m.policy.Limit - newCount
	} else {
		remaining = m.policy.Limit - currentCount
	}
	if remaining < 0 {
		remaining = 0
	}

	// Update entry if allowed and n > 0 (skip mutation for peek operations)
	if allowed && n > 0 {
		m.data[key] = &windowEntry{
			count:       newCount,
			windowStart: windowStart,
			expiration:  windowEnd.Add(time.Minute), // Keep for 1 minute after window ends
		}
	}

	slog.Debug("FixedWindow: rate limit check result",
		"key", key,
		"allowed", allowed,
		"currentCount", currentCount,
		"newCount", newCount,
		"limit", m.policy.Limit,
		"remaining", remaining)

	// Build result
	result := &limiter.Result{
		Allowed:   allowed,
		Limit:     m.policy.Limit,
		Remaining: remaining,
		Reset:     windowEnd,
		Duration:  m.policy.Duration,
		Policy:    m.policy,
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
func (m *MemoryLimiter) ConsumeN(ctx context.Context, key string, n int64) (*limiter.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock.Now()
	windowStart := m.policy.WindowStart(now)
	windowEnd := m.policy.WindowEnd(now)

	slog.Debug("FixedWindow: force consuming tokens",
		"key", key,
		"cost", n,
		"windowStart", windowStart,
		"windowEnd", windowEnd)

	// Get current entry or initialize new one
	entry, exists := m.data[key]

	// Reset count if we're in a new window or entry expired
	var currentCount int64
	if !exists || entry.windowStart != windowStart || now.After(entry.expiration) {
		currentCount = 0
	} else {
		currentCount = entry.count
	}

	// Calculate new count (always consume, even if it exceeds limit)
	newCount := currentCount + n
	allowed := newCount <= m.policy.Limit

	// Calculate remaining capacity (can be negative for overage tracking)
	remaining := m.policy.Limit - newCount
	if remaining < 0 {
		remaining = 0
	}

	// Always update entry (unlike AllowN which only updates when allowed)
	if n > 0 {
		m.data[key] = &windowEntry{
			count:       newCount,
			windowStart: windowStart,
			expiration:  windowEnd.Add(time.Minute), // Keep for 1 minute after window ends
		}
	}

	slog.Debug("FixedWindow: tokens consumed",
		"key", key,
		"allowed", allowed,
		"currentCount", currentCount,
		"newCount", newCount,
		"limit", m.policy.Limit,
		"remaining", remaining)

	// Build result
	result := &limiter.Result{
		Allowed:   allowed,
		Limit:     m.policy.Limit,
		Remaining: remaining,
		Reset:     windowEnd,
		Duration:  m.policy.Duration,
		Policy:    m.policy,
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

// GetAvailable returns the available tokens for the given key without consuming
func (m *MemoryLimiter) GetAvailable(ctx context.Context, key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := m.clock.Now()
	windowStart := m.policy.WindowStart(now)

	// Get current entry or initialize new one
	entry, exists := m.data[key]

	// Reset count if we're in a new window or entry expired
	var currentCount int64
	if !exists || entry.windowStart != windowStart || now.After(entry.expiration) {
		currentCount = 0
	} else {
		currentCount = entry.count
	}

	// Calculate remaining capacity
	remaining := m.policy.Limit - currentCount
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
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
