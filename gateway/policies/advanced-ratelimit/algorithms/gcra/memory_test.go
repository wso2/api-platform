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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/gateway-controllers/policies/advanced-ratelimit/limiter"
)

func TestMemoryLimiter_BasicAllow(t *testing.T) {
	policy := NewPolicy(10, time.Minute, 10) // 10 requests per minute, burst 10
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	fixedTime := time.Unix(1000, 0)
	rl.WithClock(limiter.NewFixedClock(fixedTime))

	// First 10 requests at the same instant should be allowed
	for i := 0; i < 10; i++ {
		result, err := rl.Allow(ctx, "user:123")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d should be allowed, but was denied", i)
		}
	}

	// 11th request should be denied (burst exhausted)
	result, err := rl.Allow(ctx, "user:123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("11th request should be denied, but was allowed")
	}
}

func TestMemoryLimiter_AllowN(t *testing.T) {
	// Policy: 10 requests per second, burst of 10
	policy := NewPolicy(10, time.Second, 10)
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
		t.Fatal("5 requests should be allowed from burst capacity")
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
}

func TestMemoryLimiter_BurstRefill(t *testing.T) {
	// 10 req/sec with burst of 10
	policy := NewPolicy(10, time.Second, 10)
	rl := NewMemoryLimiter(policy, 0)
	defer rl.Close()

	ctx := context.Background()
	rl.WithClock(limiter.NewFixedClock(time.Unix(2000, 0)))

	// Exhaust burst
	for i := 0; i < 10; i++ {
		result, err := rl.Allow(ctx, "refill-test")
		if err != nil || !result.Allowed {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// Next request denied
	result, err := rl.Allow(ctx, "refill-test")
	if err != nil || result.Allowed {
		t.Fatal("burst should be exhausted")
	}

	// Advance time by 1 second (all 10 tokens should refill)
	rl.WithClock(limiter.NewFixedClock(time.Unix(2001, 0)))
	result, err = rl.Allow(ctx, "refill-test")
	if err != nil || !result.Allowed {
		t.Fatal("request should be allowed after 1 second")
	}
	if result.Remaining != 9 {
		t.Fatalf("expected 9 remaining after refill, got %d", result.Remaining)
	}
}

func TestMemoryLimiter_Concurrent(t *testing.T) {
	// 100 req/sec, burst 100
	policy := NewPolicy(100, time.Second, 100)
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
	policy := NewPolicy(10, time.Second, 10)
	rl := NewMemoryLimiter(policy, 100*time.Millisecond)
	defer rl.Close()

	ctx := context.Background()
	rl.WithClock(limiter.NewFixedClock(time.Unix(3000, 0)))

	// Create rate limit state for 2 keys
	_, _ = rl.Allow(ctx, "temp-key-1")
	_, _ = rl.Allow(ctx, "temp-key-2")

	// Wait for cleanup cycle
	time.Sleep(150 * time.Millisecond)

	// Advance time beyond expiration (burst allowance + duration)
	rl.WithClock(limiter.NewFixedClock(time.Unix(3002, 0)))
	time.Sleep(150 * time.Millisecond)

	// Verify entries were cleaned up (hard to test directly without exposing internals)
	// Just verify limiter still works
	result, err := rl.Allow(ctx, "new-key")
	if err != nil || !result.Allowed {
		t.Fatal("limiter should still work after cleanup")
	}
}

func TestMemoryLimiter_MultipleKeys(t *testing.T) {
	policy := NewPolicy(5, time.Second, 5)
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
