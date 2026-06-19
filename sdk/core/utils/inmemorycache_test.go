/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import (
	"context"
	"testing"
	"time"
)

const testValue = "testValue"

func TestNewInMemoryCache(t *testing.T) {
	testCases := []struct {
		name        string
		size        int
		ttl         time.Duration
		policy      string
		expectedMax int
	}{
		{"LRUCache", 100, 60 * time.Second, LRUEvictionPolicy, 100},
		{"LFUCache", 100, 60 * time.Second, LFUEvictionPolicy, 100},
		{"ZeroSize", 0, 60 * time.Second, LRUEvictionPolicy, 0},
		{"ZeroTTL", 100, 0, LRUEvictionPolicy, 100},
		{"UnknownPolicyDefaultsToLRU", 100, 60 * time.Second, "UNKNOWN", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewInMemoryCache[string](tc.name, tc.size, tc.ttl, tc.policy)
			if c == nil {
				t.Fatal("expected non-nil cache")
			}
			if !c.IsEnabled() {
				t.Error("expected cache to be enabled")
			}
			if c.GetName() != tc.name {
				t.Errorf("expected name %q, got %q", tc.name, c.GetName())
			}
			stats := c.GetStats()
			if !stats.Enabled {
				t.Error("expected stats.Enabled to be true")
			}
			if stats.MaxSize != tc.expectedMax {
				t.Errorf("expected MaxSize %d, got %d", tc.expectedMax, stats.MaxSize)
			}
			if stats.Size != 0 {
				t.Errorf("expected empty cache, got size %d", stats.Size)
			}
		})
	}
}

func TestSetAndGet(t *testing.T) {
	for _, policy := range []string{LRUEvictionPolicy, LFUEvictionPolicy} {
		t.Run(policy, func(t *testing.T) {
			c := NewInMemoryCache[string](policy, 100, 60*time.Second, policy)
			ctx := context.Background()
			key := CacheKey{Key: "testKey"}

			if err := c.Set(ctx, key, testValue); err != nil {
				t.Fatalf("Set failed: %v", err)
			}

			val, found := c.Get(ctx, key)
			if !found {
				t.Fatal("expected to find key after Set")
			}
			if val != testValue {
				t.Errorf("expected %q, got %q", testValue, val)
			}

			stats := c.GetStats()
			if stats.HitCount != 1 {
				t.Errorf("expected HitCount 1, got %d", stats.HitCount)
			}
			if stats.Size != 1 {
				t.Errorf("expected Size 1, got %d", stats.Size)
			}
			if stats.HitRate != 1.0 {
				t.Errorf("expected HitRate 1.0, got %f", stats.HitRate)
			}

			_, found = c.Get(ctx, CacheKey{Key: "missing"})
			if found {
				t.Error("expected miss for non-existent key")
			}

			stats = c.GetStats()
			if stats.MissCount != 1 {
				t.Errorf("expected MissCount 1, got %d", stats.MissCount)
			}
			if stats.HitRate != 0.5 {
				t.Errorf("expected HitRate 0.5, got %f", stats.HitRate)
			}
		})
	}
}

func TestUpdateExistingEntry(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 60*time.Second, LRUEvictionPolicy)
	ctx := context.Background()
	key := CacheKey{Key: "k"}

	_ = c.Set(ctx, key, "first")
	_ = c.Set(ctx, key, "second")

	val, found := c.Get(ctx, key)
	if !found {
		t.Fatal("expected to find key")
	}
	if val != "second" {
		t.Errorf("expected updated value %q, got %q", "second", val)
	}
	if c.GetStats().Size != 1 {
		t.Error("expected size to remain 1 after updating existing key")
	}
}

func TestDelete(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 60*time.Second, LRUEvictionPolicy)
	ctx := context.Background()
	key := CacheKey{Key: "testKey"}

	_ = c.Set(ctx, key, testValue)

	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, found := c.Get(ctx, key)
	if found {
		t.Error("expected key to be absent after Delete")
	}
	if c.GetStats().Size != 0 {
		t.Errorf("expected Size 0, got %d", c.GetStats().Size)
	}
}

func TestDeleteMissingKey(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 60*time.Second, LRUEvictionPolicy)
	if err := c.Delete(context.Background(), CacheKey{Key: "ghost"}); err != nil {
		t.Errorf("expected no error deleting missing key, got %v", err)
	}
}

func TestClear(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 60*time.Second, LRUEvictionPolicy)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = c.Set(ctx, CacheKey{Key: "k" + string(rune('0'+i))}, testValue)
	}
	_, _ = c.Get(ctx, CacheKey{Key: "k0"})
	_, _ = c.Get(ctx, CacheKey{Key: "missing"})

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	stats := c.GetStats()
	if stats.Size != 0 {
		t.Errorf("expected Size 0 after Clear, got %d", stats.Size)
	}
	if stats.HitCount != 0 || stats.MissCount != 0 || stats.EvictCount != 0 {
		t.Error("expected statistics reset after Clear")
	}
}

func TestExpiry(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 50*time.Millisecond, LRUEvictionPolicy)
	ctx := context.Background()
	key := CacheKey{Key: "k"}

	_ = c.Set(ctx, key, testValue)

	val, found := c.Get(ctx, key)
	if !found || val != testValue {
		t.Fatal("expected to find key immediately after Set")
	}

	time.Sleep(100 * time.Millisecond)

	_, found = c.Get(ctx, key)
	if found {
		t.Error("expected entry to be expired after TTL elapsed")
	}
}

func TestZeroTTLNeverExpires(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 0, LRUEvictionPolicy)
	ctx := context.Background()
	key := CacheKey{Key: "k"}

	_ = c.Set(ctx, key, testValue)

	// Brief delay; zero-TTL entries must remain accessible.
	time.Sleep(10 * time.Millisecond)

	_, found := c.Get(ctx, key)
	if !found {
		t.Error("expected zero-TTL entry to remain accessible")
	}
}

func TestCleanupExpired(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 50*time.Millisecond, LRUEvictionPolicy)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = c.Set(ctx, CacheKey{Key: "k" + string(rune('0'+i))}, testValue)
	}
	if c.GetStats().Size != 5 {
		t.Fatalf("expected 5 entries before sleep")
	}

	time.Sleep(100 * time.Millisecond)
	c.CleanupExpired()

	if c.GetStats().Size != 0 {
		t.Errorf("expected 0 entries after CleanupExpired, got %d", c.GetStats().Size)
	}
}

func TestCleanupExpiredPreservesNonExpired(t *testing.T) {
	c := NewInMemoryCache[string]("cache", 100, 50*time.Millisecond, LRUEvictionPolicy)
	ctx := context.Background()

	_ = c.Set(ctx, CacheKey{Key: "expires"}, "a")
	time.Sleep(100 * time.Millisecond)

	// Add a no-expiry entry after the TTL window.
	noExpiry := NewInMemoryCache[string]("cache2", 100, 0, LRUEvictionPolicy)
	_ = noExpiry.Set(ctx, CacheKey{Key: "forever"}, "b")
	noExpiry.CleanupExpired()
	if noExpiry.GetStats().Size != 1 {
		t.Error("expected zero-TTL entry to survive CleanupExpired")
	}

	c.CleanupExpired()
	if c.GetStats().Size != 0 {
		t.Errorf("expected expired entry to be removed, got size %d", c.GetStats().Size)
	}
}

func TestLRUEviction(t *testing.T) {
	// Cache of size 3. After filling, access key0 so key1 becomes LRU.
	// Adding key3 should evict key1.
	c := NewInMemoryCache[string]("lru", 3, 60*time.Second, LRUEvictionPolicy)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = c.Set(ctx, CacheKey{Key: "key" + string(rune('0'+i))}, testValue)
	}

	// Make key0 the most recently used.
	_, _ = c.Get(ctx, CacheKey{Key: "key0"})

	// Adding key3 evicts key1 (the LRU entry).
	_ = c.Set(ctx, CacheKey{Key: "key3"}, testValue)

	stats := c.GetStats()
	if stats.Size != 3 {
		t.Errorf("expected size 3, got %d", stats.Size)
	}
	if stats.EvictCount != 1 {
		t.Errorf("expected EvictCount 1, got %d", stats.EvictCount)
	}

	_, found := c.Get(ctx, CacheKey{Key: "key1"})
	if found {
		t.Error("expected key1 to be evicted (LRU)")
	}
	_, found = c.Get(ctx, CacheKey{Key: "key0"})
	if !found {
		t.Error("expected key0 to remain (most recently used)")
	}
	_, found = c.Get(ctx, CacheKey{Key: "key3"})
	if !found {
		t.Error("expected key3 to exist")
	}
}

func TestLFUEviction(t *testing.T) {
	// Cache of size 3. Access key0 multiple times so key1/key2 are the least frequent.
	// Adding key3 should evict the least-frequent entry (key1 or key2).
	c := NewInMemoryCache[string]("lfu", 3, 60*time.Second, LFUEvictionPolicy)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = c.Set(ctx, CacheKey{Key: "key" + string(rune('0'+i))}, testValue)
	}

	// Boost key0's frequency.
	for i := 0; i < 3; i++ {
		_, _ = c.Get(ctx, CacheKey{Key: "key0"})
	}

	// Adding key3 must evict the entry with the lowest access count.
	_ = c.Set(ctx, CacheKey{Key: "key3"}, testValue)

	stats := c.GetStats()
	if stats.Size != 3 {
		t.Errorf("expected size 3, got %d", stats.Size)
	}
	if stats.EvictCount != 1 {
		t.Errorf("expected EvictCount 1, got %d", stats.EvictCount)
	}

	_, found := c.Get(ctx, CacheKey{Key: "key0"})
	if !found {
		t.Error("expected high-frequency key0 to survive eviction")
	}
}

func TestGetStats(t *testing.T) {
	c := NewInMemoryCache[string]("stats", 100, 60*time.Second, LRUEvictionPolicy)
	ctx := context.Background()

	initial := c.GetStats()
	if !initial.Enabled || initial.Size != 0 || initial.MaxSize != 100 ||
		initial.HitCount != 0 || initial.MissCount != 0 ||
		initial.HitRate != 0 || initial.EvictCount != 0 {
		t.Error("unexpected non-zero initial stats")
	}

	for i := 0; i < 5; i++ {
		_ = c.Set(ctx, CacheKey{Key: "k" + string(rune('0'+i))}, "v")
	}
	_, _ = c.Get(ctx, CacheKey{Key: "k0"})
	_, _ = c.Get(ctx, CacheKey{Key: "k1"})
	_, _ = c.Get(ctx, CacheKey{Key: "k2"})
	_, _ = c.Get(ctx, CacheKey{Key: "miss1"})
	_, _ = c.Get(ctx, CacheKey{Key: "miss2"})

	s := c.GetStats()
	if s.HitCount != 3 {
		t.Errorf("expected HitCount 3, got %d", s.HitCount)
	}
	if s.MissCount != 2 {
		t.Errorf("expected MissCount 2, got %d", s.MissCount)
	}
	if s.HitRate != 0.6 {
		t.Errorf("expected HitRate 0.6, got %f", s.HitRate)
	}

	// Trigger evictions by overfilling.
	for i := 0; i < 100; i++ {
		_ = c.Set(ctx, CacheKey{Key: "evict" + string(rune('0'+i))}, "v")
	}
	if c.GetStats().EvictCount == 0 {
		t.Error("expected EvictCount > 0 after overfilling cache")
	}
}

func TestGetName(t *testing.T) {
	testCases := []struct{ name string }{
		{"simple"},
		{""},
		{"cache-name_123:special"},
	}
	for _, tc := range testCases {
		c := NewInMemoryCache[string](tc.name, 10, time.Second, LRUEvictionPolicy)
		if c.GetName() != tc.name {
			t.Errorf("expected name %q, got %q", tc.name, c.GetName())
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := NewInMemoryCache[int]("concurrent", 50, 60*time.Second, LRUEvictionPolicy)
	ctx := context.Background()
	done := make(chan struct{})

	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 20; i++ {
				key := CacheKey{Key: "key" + string(rune('0'+id))}
				_ = c.Set(ctx, key, id*100+i)
				_, _ = c.Get(ctx, key)
				_ = c.GetStats()
			}
			done <- struct{}{}
		}(g)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
