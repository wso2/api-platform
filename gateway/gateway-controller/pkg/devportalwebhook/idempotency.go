/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package devportalwebhook

import (
	"sort"
	"sync"
	"time"
)

// IdempotencyCache is an in-memory TTL cache that deduplicates devportal event IDs.
// It prevents duplicate side effects when the devportal retries failed deliveries.
//
// Memory cost: ~48 bytes per entry (string header + time.Time + map overhead).
// At the default maxSize of 10 000 entries that is approximately 480 KB.
type IdempotencyCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
	maxSize int
	stop    chan struct{}
}

// NewIdempotencyCache creates a cache and starts a background goroutine that evicts
// expired entries every ttl/2 (minimum 30 s). Call Close() to stop it.
func NewIdempotencyCache(ttl time.Duration, maxSize int) *IdempotencyCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if maxSize <= 0 {
		maxSize = 10_000
	}

	c := &IdempotencyCache{
		entries: make(map[string]time.Time, maxSize),
		ttl:     ttl,
		maxSize: maxSize,
		stop:    make(chan struct{}),
	}

	sweepInterval := ttl / 2
	if sweepInterval < 30*time.Second {
		sweepInterval = 30 * time.Second
	}

	go c.sweeper(sweepInterval)
	return c
}

// CheckAndSet atomically checks whether eventID has been seen.
// Returns true if it was already seen (duplicate); records it and returns false otherwise.
func (c *IdempotencyCache) CheckAndSet(eventID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, seen := c.entries[eventID]; seen {
		return true
	}
	c.entries[eventID] = time.Now()
	return false
}

// Close stops the background sweeper goroutine.
func (c *IdempotencyCache) Close() {
	close(c.stop)
}

func (c *IdempotencyCache) sweeper(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sweep()
		case <-c.stop:
			return
		}
	}
}

func (c *IdempotencyCache) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-c.ttl)
	for id, insertedAt := range c.entries {
		if insertedAt.Before(cutoff) {
			delete(c.entries, id)
		}
	}

	// If still over capacity after TTL eviction, drop the oldest entries.
	if len(c.entries) > c.maxSize {
		type entry struct {
			id         string
			insertedAt time.Time
		}
		all := make([]entry, 0, len(c.entries))
		for id, t := range c.entries {
			all = append(all, entry{id, t})
		}
		sort.Slice(all, func(i, j int) bool {
			return all[i].insertedAt.Before(all[j].insertedAt)
		})
		for i := 0; i < len(all)-c.maxSize; i++ {
			delete(c.entries, all[i].id)
		}
	}
}
