package policyv1alpha

import (
    "sync"
    "sync/atomic"
    "time"
)

// RequestCountEntry stores count data for a specific key
type RequestCountEntry struct {
    Count       int64
    FirstSeen   time.Time
    LastUpdated time.Time
}

// RequestCountStore is a global singleton for storing request counts
type RequestCountStore struct {
    mu     sync.RWMutex
    counts map[string]*RequestCountEntry
}

// Singleton instance for RequestCountStore
var (
    requestCountInstance *RequestCountStore
    requestCountOnce     sync.Once
)

// GetRequestCountStoreInstance returns the global singleton instance
func GetRequestCountStoreInstance() *RequestCountStore {
    requestCountOnce.Do(func() {
        requestCountInstance = &RequestCountStore{
            counts: make(map[string]*RequestCountEntry),
        }
    })
    return requestCountInstance
}

// Increment atomically increments the count for a key and returns the new value
func (rcs *RequestCountStore) Increment(key string) int64 {
    rcs.mu.Lock()
    defer rcs.mu.Unlock()

    entry, exists := rcs.counts[key]
    if !exists {
        entry = &RequestCountEntry{
            Count:       0,
            FirstSeen:   time.Now(),
            LastUpdated: time.Now(),
        }
        rcs.counts[key] = entry
    }

    entry.Count++
    entry.LastUpdated = time.Now()
    return entry.Count
}

// Get returns the current count for a key
func (rcs *RequestCountStore) Get(key string) int64 {
    rcs.mu.RLock()
    defer rcs.mu.RUnlock()

    entry, exists := rcs.counts[key]
    if !exists {
        return 0
    }
    return atomic.LoadInt64(&entry.Count)
}

// Reset resets the count for a key to zero
func (rcs *RequestCountStore) Reset(key string) {
    rcs.mu.Lock()
    defer rcs.mu.Unlock()

    if entry, exists := rcs.counts[key]; exists {
        atomic.StoreInt64(&entry.Count, 0)
        entry.LastUpdated = time.Now()
    }
}

// Delete removes a key from the store
func (rcs *RequestCountStore) Delete(key string) {
    rcs.mu.Lock()
    defer rcs.mu.Unlock()
    delete(rcs.counts, key)
}