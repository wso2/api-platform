package fixedwindow

import (
	"context"
	"sync"
	"time"

	"github.com/policy-engine/policies/ratelimit/limiter"
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
