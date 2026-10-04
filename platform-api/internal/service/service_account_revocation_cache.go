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

package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// RevocationCache holds every live revocation watermark in memory. It is
// rebuilt from the database on a jittered poll, because replicas have no other
// way to hear about a revoke another replica served.
type RevocationCache struct {
	mu         sync.RWMutex
	watermarks map[string]watermark
	loaded     bool
	repo       repository.ServiceAccountRevocationRepository
	slogger    *slog.Logger
}

type watermark struct {
	minVersion int64
	expiresAt  time.Time
}

// NewRevocationCache creates an empty, unloaded cache.
func NewRevocationCache(repo repository.ServiceAccountRevocationRepository, slogger *slog.Logger) *RevocationCache {
	return &RevocationCache{watermarks: map[string]watermark{}, repo: repo, slogger: slogger}
}

// Load fills the cache. The server calls it before serving and refuses to
// start on error: a cold cache must never read as "nothing is revoked".
func (c *RevocationCache) Load() error {
	return c.refresh()
}

// IsRevoked reports whether a token carrying tokenVersion for accountUUID is
// revoked. An unloaded cache revokes everything.
func (c *RevocationCache) IsRevoked(accountUUID string, tokenVersion int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return true
	}
	wm, ok := c.watermarks[accountUUID]
	return ok && tokenVersion < wm.minVersion
}

// Remember applies a revoke this replica just wrote, without waiting for the poll.
func (c *RevocationCache) Remember(rev *model.ServiceAccountRevocation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.raise(rev.AccountUUID, watermark{minVersion: rev.MinTokenVersion, expiresAt: rev.ExpiresAt})
}

// raise stores wm unless a higher watermark is already held. Callers hold mu.
func (c *RevocationCache) raise(accountUUID string, wm watermark) {
	if cur, ok := c.watermarks[accountUUID]; !ok || wm.minVersion > cur.minVersion {
		c.watermarks[accountUUID] = wm
	}
}

// Run polls until ctx ends. The jitter is waited before every fetch, the first
// included, so replicas restarting together do not hit the database in step.
func (c *RevocationCache) Run(ctx context.Context, interval time.Duration) {
	for {
		delay := interval
		if half := interval / 2; half > 0 {
			delay += rand.N(half)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		// A failed refresh keeps the old contents; emptying them would fail open.
		if err := c.refresh(); err != nil {
			c.slogger.Error("failed to refresh service account revocations", "error", err)
			continue
		}
		if n, err := c.repo.PruneExpired(time.Now()); err != nil {
			c.slogger.Warn("failed to prune service account revocations", "error", err)
		} else if n > 0 {
			c.slogger.Debug("pruned service account revocations", "count", n)
		}
	}
}

// refresh merges the database's watermarks into the cache rather than swapping
// the map: a Remember that lands between the read and the lock must survive.
// Watermarks only rise and only disappear by expiring, so an entry is dropped
// once it expires and never because a snapshot lacked it.
func (c *RevocationCache) refresh() error {
	now := time.Now()
	revs, err := c.repo.ListActive(now)
	if err != nil {
		return fmt.Errorf("failed to load service account revocations: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, wm := range c.watermarks {
		if !wm.expiresAt.After(now) {
			delete(c.watermarks, id)
		}
	}
	for _, r := range revs {
		c.raise(r.AccountUUID, watermark{minVersion: r.MinTokenVersion, expiresAt: r.ExpiresAt})
	}
	c.loaded = true
	return nil
}
