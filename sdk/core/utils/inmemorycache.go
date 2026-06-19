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
	"container/heap"
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type evictionPolicy string

const (
	evictionPolicyLRU evictionPolicy = "LRU"
	evictionPolicyLFU evictionPolicy = "LFU"
)

// lfuHeapItem is a node in the LFU min-heap.
type lfuHeapItem struct {
	key         CacheKey
	accessCount int64
	lastAccess  time.Time
	index       int // position in the heap; -1 when not in heap
}

// lfuHeap implements heap.Interface for LFU eviction.
type lfuHeap []*lfuHeapItem

func (h lfuHeap) Len() int { return len(h) }

func (h lfuHeap) Less(i, j int) bool {
	if h[i].accessCount != h[j].accessCount {
		return h[i].accessCount < h[j].accessCount
	}
	return h[i].lastAccess.Before(h[j].lastAccess)
}

func (h lfuHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *lfuHeap) Push(x any) {
	n := len(*h)
	item := x.(*lfuHeapItem)
	item.index = n
	*h = append(*h, item)
}

func (h *lfuHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // marks as removed
	*h = old[0 : n-1]
	return item
}

// inMemoryCacheEntry wraps a CacheEntry with LRU list and LFU heap metadata.
type inMemoryCacheEntry[T any] struct {
	*CacheEntry[T]
	listElement *list.Element
	heapItem    *lfuHeapItem
	lastAccess  time.Time
	accessCount int64
}

// InMemoryCache is a thread-safe, generic in-memory cache with LRU or LFU eviction.
// Create one with NewInMemoryCache; the zero value is not usable.
type InMemoryCache[T any] struct {
	enabled        bool
	name           string
	cache          map[CacheKey]*inMemoryCacheEntry[T]
	accessOrder    *list.List
	lfuHeap        *lfuHeap
	mu             sync.RWMutex
	size           int
	ttl            time.Duration
	evictionPolicy evictionPolicy
	hitCount       atomic.Int64
	missCount      atomic.Int64
	evictCount     atomic.Int64
}

// NewInMemoryCache creates an enabled InMemoryCache.
// ttl=0 means entries never expire. An unrecognised policy defaults to LRU.
func NewInMemoryCache[T any](name string, size int, ttl time.Duration, policy string) *InMemoryCache[T] {
	ep := evictionPolicyLRU
	if policy == string(evictionPolicyLFU) {
		ep = evictionPolicyLFU
	}

	h := &lfuHeap{}
	heap.Init(h)

	return &InMemoryCache[T]{
		enabled:        true,
		name:           name,
		cache:          make(map[CacheKey]*inMemoryCacheEntry[T]),
		accessOrder:    list.New(),
		lfuHeap:        h,
		size:           size,
		ttl:            ttl,
		evictionPolicy: ep,
	}
}

// GetName returns the cache name.
func (c *InMemoryCache[T]) GetName() string { return c.name }

// IsEnabled reports whether the cache is enabled.
func (c *InMemoryCache[T]) IsEnabled() bool { return c.enabled }

// Set adds or updates an entry. If the cache is at capacity a victim is evicted first.
func (c *InMemoryCache[T]) Set(_ context.Context, key CacheKey, value T) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var expiryTime time.Time
	if c.ttl > 0 {
		expiryTime = now.Add(c.ttl)
	}

	if existing, exists := c.cache[key]; exists {
		existing.Value = value
		existing.ExpiryTime = expiryTime
		existing.lastAccess = now
		existing.accessCount++
		c.accessOrder.MoveToFront(existing.listElement)

		if c.evictionPolicy == evictionPolicyLFU && existing.heapItem != nil {
			existing.heapItem.accessCount = existing.accessCount
			existing.heapItem.lastAccess = existing.lastAccess
			heap.Fix(c.lfuHeap, existing.heapItem.index)
		}
		return nil
	}

	if c.size <= 0 {
		return nil
	}

	// Evict before inserting so the new entry is never a candidate for immediate removal.
	// Under LFU, a post-insert evict could self-evict the just-added entry (accessCount=1)
	// if all existing entries have higher counts.
	if len(c.cache) >= c.size {
		c.evict()
	}

	listElement := c.accessOrder.PushFront(key)

	var heapItem *lfuHeapItem
	if c.evictionPolicy == evictionPolicyLFU {
		heapItem = &lfuHeapItem{key: key, accessCount: 1, lastAccess: now}
		heap.Push(c.lfuHeap, heapItem)
	}

	c.cache[key] = &inMemoryCacheEntry[T]{
		CacheEntry:  &CacheEntry[T]{Value: value, ExpiryTime: expiryTime},
		listElement: listElement,
		heapItem:    heapItem,
		lastAccess:  now,
		accessCount: 1,
	}

	return nil
}

// Get retrieves a value. Expired entries are lazily removed and reported as misses.
func (c *InMemoryCache[T]) Get(_ context.Context, key CacheKey) (T, bool) {
	if !c.enabled {
		var zero T
		return zero, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		c.missCount.Add(1)
		var zero T
		return zero, false
	}

	if !entry.ExpiryTime.IsZero() && time.Now().After(entry.ExpiryTime) {
		c.deleteEntry(key, entry)
		c.missCount.Add(1)
		var zero T
		return zero, false
	}

	entry.lastAccess = time.Now()
	entry.accessCount++
	c.accessOrder.MoveToFront(entry.listElement)
	c.hitCount.Add(1)

	if c.evictionPolicy == evictionPolicyLFU && entry.heapItem != nil {
		entry.heapItem.accessCount = entry.accessCount
		entry.heapItem.lastAccess = entry.lastAccess
		heap.Fix(c.lfuHeap, entry.heapItem.index)
	}

	return entry.Value, true
}

// Delete removes a single entry. A missing key is a no-op.
func (c *InMemoryCache[T]) Delete(_ context.Context, key CacheKey) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.cache[key]; exists {
		c.deleteEntry(key, entry)
	}
	return nil
}

// Clear removes all entries and resets statistics.
func (c *InMemoryCache[T]) Clear(_ context.Context) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[CacheKey]*inMemoryCacheEntry[T])
	c.accessOrder.Init()
	c.lfuHeap = &lfuHeap{}
	heap.Init(c.lfuHeap)
	c.hitCount.Store(0)
	c.missCount.Store(0)
	c.evictCount.Store(0)
	return nil
}

// GetStats returns a snapshot of cache statistics.
func (c *InMemoryCache[T]) GetStats() CacheStat {
	if !c.enabled {
		return CacheStat{Enabled: false}
	}

	c.mu.RLock()
	size := len(c.cache)
	c.mu.RUnlock()

	hits := c.hitCount.Load()
	misses := c.missCount.Load()
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CacheStat{
		Enabled:    true,
		Size:       size,
		MaxSize:    c.size,
		HitCount:   hits,
		MissCount:  misses,
		HitRate:    hitRate,
		EvictCount: c.evictCount.Load(),
	}
}

// CleanupExpired removes all entries whose TTL has elapsed.
func (c *InMemoryCache[T]) CleanupExpired() {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if !entry.ExpiryTime.IsZero() && now.After(entry.ExpiryTime) {
			c.deleteEntry(key, entry)
		}
	}
}

// evict removes one entry according to the configured eviction policy.
// Must be called with the write lock held.
func (c *InMemoryCache[T]) evict() {
	if c.evictionPolicy == evictionPolicyLFU {
		c.evictLeastFrequent()
	} else {
		c.evictOldest()
	}
}

// evictOldest removes the least recently used entry (LRU).
func (c *InMemoryCache[T]) evictOldest() {
	back := c.accessOrder.Back()
	if back == nil {
		return
	}
	key := back.Value.(CacheKey)
	if entry, exists := c.cache[key]; exists {
		c.deleteEntry(key, entry)
		c.evictCount.Add(1)
	}
}

// evictLeastFrequent removes the least frequently used entry (LFU).
func (c *InMemoryCache[T]) evictLeastFrequent() {
	if c.lfuHeap.Len() == 0 {
		return
	}
	item := heap.Pop(c.lfuHeap).(*lfuHeapItem)
	if entry, exists := c.cache[item.key]; exists {
		c.deleteEntry(item.key, entry)
		c.evictCount.Add(1)
	}
}

// deleteEntry removes an entry from the map, LRU list, and LFU heap atomically.
// Must be called with the write lock held.
func (c *InMemoryCache[T]) deleteEntry(key CacheKey, entry *inMemoryCacheEntry[T]) {
	delete(c.cache, key)
	c.accessOrder.Remove(entry.listElement)
	// heap.Pop already sets index=-1; only call heap.Remove for non-popped items.
	if c.evictionPolicy == evictionPolicyLFU && entry.heapItem != nil && entry.heapItem.index >= 0 {
		heap.Remove(c.lfuHeap, entry.heapItem.index)
	}
}
