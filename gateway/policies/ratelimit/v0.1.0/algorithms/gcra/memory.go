package gcra

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/policy-engine/policies/ratelimit/limiter"
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

	// GCRA Algorithm Step 3: Calculate the earliest time this request can be allowed
	allowAt := tat.Add(-burstAllowance)

	// GCRA Algorithm Step 4: Check if request is allowed
	// Allow if now >= allowAt (i.e., deny only if now < allowAt)
	if now.Before(allowAt) {
		// Request denied - calculate retry after
		retryAfter := allowAt.Sub(now)
		remaining := m.calculateRemaining(tat, now, emissionInterval, burstAllowance)

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
