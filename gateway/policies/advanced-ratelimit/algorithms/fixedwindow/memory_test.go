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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

func TestMemoryLimiter_BasicAllow(t *testing.T) {
	policy := NewPolicy(10, time.Minute) // 10 requests per minute
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	fixedTime := time.Unix(1000, 0)
	rl.WithClock(limiter.NewFixedClock(fixedTime))

	// First 10 requests in the same window should be allowed
	for i := 0; i < 10; i++ {
		result, err := rl.Allow(ctx, "user:123")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d should be allowed, but was denied", i)
		}
	}

	// 11th request should be denied (limit reached)
	result, err := rl.Allow(ctx, "user:123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("11th request should be denied, but was allowed")
	}
	if result.Remaining != 0 {
		t.Fatalf("expected 0 remaining, got %d", result.Remaining)
	}
}

func TestMemoryLimiter_AllowN(t *testing.T) {
	// Policy: 10 requests per second
	policy := NewPolicy(10, time.Second)
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	rl.WithClock(limiter.NewFixedClock(time.Unix(1000, 0)))

	// Consume 5 requests at once
	result, err := rl.AllowN(ctx, "user:456", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatal("5 requests should be allowed")
	}
	if result.Remaining != 5 {
		t.Fatalf("expected 5 remaining, got %d", result.Remaining)
	}

	// Try to consume 6 more (should fail, only 5 remaining)
	result, err = rl.AllowN(ctx, "user:456", 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("6 requests should be denied (only 5 remaining)")
	}

	// Consume exactly 5 more (should succeed)
	result, err = rl.AllowN(ctx, "user:456", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatal("5 requests should be allowed (exactly remaining)")
	}
	if result.Remaining != 0 {
		t.Fatalf("expected 0 remaining, got %d", result.Remaining)
	}
}

func TestMemoryLimiter_WindowReset(t *testing.T) {
	// 10 req/second
	policy := NewPolicy(10, time.Second)
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	// Start at exactly 2000 seconds (window boundary)
	rl.WithClock(limiter.NewFixedClock(time.Unix(2000, 0)))

	// Exhaust limit in first window
	for i := 0; i < 10; i++ {
		result, err := rl.Allow(ctx, "reset-test")
		if err != nil || !result.Allowed {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// Next request denied in same window
	result, err := rl.Allow(ctx, "reset-test")
	if err != nil || result.Allowed {
		t.Fatal("limit should be exhausted in current window")
	}

	// Advance to next window boundary (2001 seconds)
	rl.WithClock(limiter.NewFixedClock(time.Unix(2001, 0)))

	// Counter should reset - all 10 requests available again
	for i := 0; i < 10; i++ {
		result, err := rl.Allow(ctx, "reset-test")
		if err != nil || !result.Allowed {
			t.Fatalf("request %d should be allowed after window reset", i)
		}
	}

	// Verify we're at limit again
	result, err = rl.Allow(ctx, "reset-test")
	if err != nil || result.Allowed {
		t.Fatal("limit should be exhausted in new window")
	}
}

func TestMemoryLimiter_Concurrent(t *testing.T) {
	// 100 req/sec
	policy := NewPolicy(100, time.Second)
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	var allowed, denied atomic.Int64
	var wg sync.WaitGroup

	// 200 concurrent requests (100 should succeed, 100 should fail)
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := rl.Allow(ctx, "concurrent-test")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result.Allowed {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}

	wg.Wait()

	if allowed.Load() != 100 {
		t.Fatalf("expected 100 allowed, got %d", allowed.Load())
	}
	if denied.Load() != 100 {
		t.Fatalf("expected 100 denied, got %d", denied.Load())
	}
}

func TestMemoryLimiter_CleanupExpired(t *testing.T) {
	policy := NewPolicy(10, time.Second)
	rl := NewMemoryLimiter(policy, 100*time.Millisecond)
	defer rl.Close()

	ctx := context.Background()
	rl.WithClock(limiter.NewFixedClock(time.Unix(3000, 0)))

	// Create rate limit state for 2 keys
	_, _ = rl.Allow(ctx, "temp-key-1")
	_, _ = rl.Allow(ctx, "temp-key-2")

	// Wait for cleanup cycle
	time.Sleep(150 * time.Millisecond)

	// Advance time beyond expiration (window + 1 minute buffer)
	rl.WithClock(limiter.NewFixedClock(time.Unix(3062, 0)))
	time.Sleep(150 * time.Millisecond)

	// Verify limiter still works (entries should be cleaned up)
	result, err := rl.Allow(ctx, "new-key")
	if err != nil || !result.Allowed {
		t.Fatal("limiter should still work after cleanup")
	}
}

func TestMemoryLimiter_MultipleKeys(t *testing.T) {
	policy := NewPolicy(5, time.Second)
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()

	// Each key should have independent quota
	for _, key := range []string{"user:1", "user:2", "user:3"} {
		for i := 0; i < 5; i++ {
			result, err := rl.Allow(ctx, key)
			if err != nil || !result.Allowed {
				t.Fatalf("key %s request %d should be allowed", key, i)
			}
		}
		// 6th request should be denied
		result, err := rl.Allow(ctx, key)
		if err != nil || result.Allowed {
			t.Fatalf("key %s 6th request should be denied", key)
		}
	}
}

func TestMemoryLimiter_PartialWindow(t *testing.T) {
	// Test that requests in middle of window work correctly
	policy := NewPolicy(10, time.Minute)
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	// Start at 30 seconds into a minute (not at window boundary)
	rl.WithClock(limiter.NewFixedClock(time.Unix(1030, 0)))

	// Use 7 requests
	for i := 0; i < 7; i++ {
		result, err := rl.Allow(ctx, "partial-test")
		if err != nil || !result.Allowed {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// Move to 50 seconds (still same window: 1000-1059)
	rl.WithClock(limiter.NewFixedClock(time.Unix(1050, 0)))

	// Should only have 3 remaining
	result, err := rl.AllowN(ctx, "partial-test", 3)
	if err != nil || !result.Allowed {
		t.Fatal("3 requests should be allowed (remaining quota)")
	}

	// Next request denied
	result, err = rl.Allow(ctx, "partial-test")
	if err != nil || result.Allowed {
		t.Fatal("should be denied (quota exhausted)")
	}

	// Move to next window (1080 = start of new minute: 1080-1139)
	rl.WithClock(limiter.NewFixedClock(time.Unix(1080, 0)))

	// Full quota available again
	result, err = rl.AllowN(ctx, "partial-test", 10)
	if err != nil || !result.Allowed {
		t.Fatal("full quota should be available in new window")
	}
}
