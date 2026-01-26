package limiter

import (
	"sync"
	"time"
)

// Clock provides an abstraction for time operations (useful for testing)
type Clock interface {
	Now() time.Time
}

// SystemClock uses the system time
type SystemClock struct{}

// Now returns the current system time
func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// FixedClock returns a fixed time (for testing)
// Thread-safe for concurrent reads and writes
type FixedClock struct {
	mu   sync.RWMutex
	time time.Time
}

// NewFixedClock creates a new FixedClock with the given time
func NewFixedClock(t time.Time) *FixedClock {
	return &FixedClock{time: t}
}

// Now returns the fixed time (thread-safe)
func (c *FixedClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.time
}

// Set updates the fixed time (thread-safe)
func (c *FixedClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time = t
}
