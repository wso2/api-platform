package limiter

import (
	"context"
	"time"
)

// Limiter is the main rate limiter interface (common to all algorithms)
type Limiter interface {
	// Allow checks if a request is allowed for the given key
	// Returns a Result with rate limit information
	Allow(ctx context.Context, key string) (*Result, error)

	// AllowN checks if N requests are allowed for the given key
	AllowN(ctx context.Context, key string, n int64) (*Result, error)

	// Close cleans up limiter resources
	Close() error
}

// LimitConfig is algorithm-agnostic limit configuration
type LimitConfig struct {
	Limit    int64
	Duration time.Duration
	Burst    int64 // Optional, interpretation depends on algorithm
}
