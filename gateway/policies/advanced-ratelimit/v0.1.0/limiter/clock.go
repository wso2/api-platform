package limiter

import "time"

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
type FixedClock struct {
	Time time.Time
}

// Now returns the fixed time
func (c *FixedClock) Now() time.Time {
	return c.Time
}
