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

// Package utils provides shared utilities for SDK policy implementations.
package utils

import (
	"context"
	"time"
)

// Eviction policy constants for use with NewInMemoryCache.
const (
	LRUEvictionPolicy = "LRU"
	LFUEvictionPolicy = "LFU"
)

// Cache defines the common interface for cache operations.
type Cache[T any] interface {
	GetName() string
	Set(ctx context.Context, key CacheKey, value T) error
	Get(ctx context.Context, key CacheKey) (T, bool)
	Delete(ctx context.Context, key CacheKey) error
	Clear(ctx context.Context) error
	IsEnabled() bool
	GetStats() CacheStat
	CleanupExpired()
}

// CacheKey represents a key for the cache.
type CacheKey struct {
	Key string
}

// ToString returns the string representation of the CacheKey.
func (k CacheKey) ToString() string {
	return k.Key
}

// CacheEntry represents a single cache entry.
// A zero ExpiryTime means the entry never expires.
type CacheEntry[T any] struct {
	Value      T
	ExpiryTime time.Time
}

// CacheStat holds cache statistics.
type CacheStat struct {
	Enabled    bool
	Size       int
	MaxSize    int
	HitCount   int64
	MissCount  int64
	HitRate    float64
	EvictCount int64
}
