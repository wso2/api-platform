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

// In-process display cache for the stored-handle Agent Card fetch.
//
// The public Agent Card of a passthrough Agent proxy is not stored anywhere, so
// showing it means fetching it from the upstream — on a routine page view, for
// every viewer. This cache is what keeps that from becoming one outbound
// request per view. It follows APIPortalAuthRegistry, the house pattern for
// exactly this shape: an organization-scoped composed key and a generation
// counter so a fill that was already in flight cannot repopulate an entry an
// invalidate has just dropped.
//
// What it is not: durable. Nothing is persisted, nothing survives a restart,
// and no table is involved. It is per replica, so two replicas may report
// different Age values for the same Agent proxy — accepted for a display cache,
// and not to be presented as a cluster-wide view. It is display-only and never
// a source of truth: builders, importers and the deployment path read stored
// configuration, never a cached card.

package service

import (
	"container/list"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

// agentCardCacheKey identifies one cached display fetch.
//
// It is keyed on the Agent proxy's UUID within its organization — never on the
// upstream URL. Two organizations may point at the same URL with different
// stored credentials, and a URL-keyed entry would serve one organization's card
// to the other.
type agentCardCacheKey struct {
	orgUUID   string
	proxyUUID string
}

// agentCardCacheEntry is one cached outcome. Exactly one of card and failure is
// set: successes and failures are both cached, at their own TTLs, because
// caching only successes would leave a down upstream re-contacted on every
// single page view — the exact load this cache exists to remove.
type agentCardCacheEntry struct {
	// card is the upstream's response body, held verbatim. Nothing re-encodes
	// it on the way in or out, so a cache hit returns byte-for-byte what a miss
	// would have.
	card    []byte
	failure *agentCardFailure

	storedAt  time.Time
	expiresAt time.Time
	// bytes is the entry's charge against the cache's byte ceiling.
	bytes int64
}

// agentCardFailure is a cached fetch failure, reduced to what a repeat caller
// needs: the sterile client-facing reason. The underlying cause is logged when
// the failure is first observed and is not retained.
type agentCardFailure struct {
	message string
}

// agentCardCache is a bounded, TTL'd LRU over agentCardCacheEntry values.
type agentCardCache struct {
	mu          sync.Mutex
	entries     map[agentCardCacheKey]*list.Element
	order       *list.List // front is most recently used
	generations map[agentCardCacheKey]agentCardGeneration
	totalBytes  int64

	cfg config.AgentCardCache
	// now is injectable so TTL and Age behaviour can be tested without sleeping.
	now func() time.Time
}

// agentCardGeneration records that a key was invalidated, and when. The counter
// is what a store compares its snapshot against; the timestamp is what lets the
// entry be forgotten once nothing can still be racing it.
type agentCardGeneration struct {
	counter     uint64
	invalidated time.Time
}

const (
	// agentCardGenerationRetention is how long an invalidation is remembered.
	// It has to comfortably exceed the longest a single display fetch can take
	// (the fetch carries its own timeout, an order of magnitude below this), so
	// that anything older is settled beyond doubt.
	agentCardGenerationRetention = 2 * time.Minute

	// agentCardGenerationSweepFloor keeps a small deployment from walking the
	// generation map on every invalidation just to find nothing to drop.
	agentCardGenerationSweepFloor = 64
)

// agentCardCacheNode is what the LRU list holds.
type agentCardCacheNode struct {
	key   agentCardCacheKey
	entry *agentCardCacheEntry
}

// newAgentCardCache builds the cache from its configuration. A configuration
// that disables every TTL, or leaves no room for an entry, still yields a usable
// cache object — it simply never retains anything, so callers need no nil check
// and no "is caching on" branch of their own.
func newAgentCardCache(cfg config.AgentCardCache) *agentCardCache {
	return &agentCardCache{
		entries:     make(map[agentCardCacheKey]*list.Element),
		order:       list.New(),
		generations: make(map[agentCardCacheKey]agentCardGeneration),
		cfg:         cfg,
		now:         time.Now,
	}
}

// positiveTTL is how long a fetched card stays fresh. Zero disables caching of
// successes and is what the response reports as max-age=0.
func (c *agentCardCache) positiveTTL() time.Duration {
	if c == nil || c.cfg.PositiveTTL < 0 {
		return 0
	}
	return c.cfg.PositiveTTL
}

// negativeTTL is how long a fetch failure stays cached.
func (c *agentCardCache) negativeTTL() time.Duration {
	if c == nil || c.cfg.NegativeTTL < 0 {
		return 0
	}
	return c.cfg.NegativeTTL
}

// bounded reports whether the configured ceilings leave room for anything at
// all. Both are hard caps rather than hints, so a non-positive value means the
// cache holds nothing rather than meaning "unlimited" — an unbounded map keyed
// per Agent proxy, each entry up to 1 MiB, is a memory-exhaustion vector.
func (c *agentCardCache) bounded() bool {
	return c != nil && c.cfg.MaxEntries > 0 && c.cfg.MaxBytes > 0
}

// begin snapshots the invalidation generation for a key. A caller passes the
// snapshot back to store; a store whose snapshot is stale is dropped, so a fetch
// that was already in flight when the Agent proxy changed cannot refill the
// entry that change dropped.
func (c *agentCardCache) begin(key agentCardCacheKey) uint64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generations[key].counter
}

// get returns a live entry and the time since it was actually fetched. An
// expired entry is dropped and reported as a miss.
func (c *agentCardCache) get(key agentCardCacheKey) (*agentCardCacheEntry, time.Duration, bool) {
	if c == nil {
		return nil, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.entries[key]
	if !ok {
		return nil, 0, false
	}
	node := element.Value.(*agentCardCacheNode)
	now := c.now()
	if !now.Before(node.entry.expiresAt) {
		c.removeElement(element)
		return nil, 0, false
	}
	c.order.MoveToFront(element)

	return node.entry, max(now.Sub(node.entry.storedAt), 0), true
}

// storeCard caches a successful fetch. The card's own length is its charge
// against the byte ceiling.
func (c *agentCardCache) storeCard(key agentCardCacheKey, gen uint64, card []byte) {
	c.store(key, gen, &agentCardCacheEntry{card: card, bytes: int64(len(card))}, c.positiveTTL())
}

// storeFailure caches a fetch failure under the shorter negative TTL. A failure
// entry still charges the byte ceiling — for its message — so a flood of failing
// Agent proxies cannot grow the cache without bound either.
func (c *agentCardCache) storeFailure(key agentCardCacheKey, gen uint64, message string) {
	entry := &agentCardCacheEntry{
		failure: &agentCardFailure{message: message},
		bytes:   int64(len(message)),
	}
	c.store(key, gen, entry, c.negativeTTL())
}

func (c *agentCardCache) store(key agentCardCacheKey, gen uint64, entry *agentCardCacheEntry, ttl time.Duration) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.generations[key].counter != gen {
		// Invalidate ran while this fetch was in flight; the result describes the
		// Agent proxy as it was before the change, so it is discarded rather than
		// written over the entry that change dropped — and it must not drop that
		// entry either, which is why this returns ahead of the supersession below.
		return
	}

	// Whatever is held for this key describes an outcome this fetch has just
	// superseded, so it goes now — *before* any decision about whether the new
	// outcome can itself be retained. Otherwise a replacement that cannot be
	// cached (a disabled TTL, a document past the byte ceiling) would leave the
	// old one in place and serve it on: with negative caching off, a no-cache
	// refresh that returns 503 would answer the next ordinary request with the
	// successful card it had just superseded.
	if existing, ok := c.entries[key]; ok {
		c.removeElement(existing)
	}

	// A single entry larger than the whole ceiling can never be held without
	// evicting itself, so it is not admitted at all — evicting every other entry
	// to make room for one that then exceeds the cap serves nobody.
	if ttl <= 0 || !c.bounded() || entry.bytes > c.cfg.MaxBytes {
		return
	}

	now := c.now()
	entry.storedAt = now
	entry.expiresAt = now.Add(ttl)

	element := c.order.PushFront(&agentCardCacheNode{key: key, entry: entry})
	c.entries[key] = element
	c.totalBytes += entry.bytes

	// Eviction is a miss for whoever comes back for the evicted key, never an
	// error — the next call simply fetches again.
	for c.order.Len() > c.cfg.MaxEntries || c.totalBytes > c.cfg.MaxBytes {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.removeElement(oldest)
	}
}

// invalidate drops an entry and bumps its generation. Called whenever the Agent
// proxy's upstream URL, upstream credentials or card mode may have changed, and
// on delete — without it a stale failure would outlive the fix by a full TTL.
func (c *agentCardCache) invalidate(orgUUID, proxyUUID string) {
	if c == nil || !c.bounded() {
		// A cache that retains nothing has no entry to drop and no fill to guard
		// against, so recording the invalidation would be bookkeeping that
		// protects nothing — and, since create/delete cycles never repeat a UUID,
		// grows for as long as the process runs.
		return
	}
	key := agentCardCacheKey{orgUUID: orgUUID, proxyUUID: proxyUUID}

	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.removeElement(element)
	}
	c.generations[key] = agentCardGeneration{
		counter:     c.generations[key].counter + 1,
		invalidated: c.now(),
	}
	c.forgetSettledGenerations()
}

// forgetSettledGenerations drops invalidations old enough that nothing can still
// be racing them. The caller holds the lock.
//
// A generation entry exists only to out-live a fetch that was already in flight
// when the Agent proxy changed, and a fetch is bounded by its own timeout — so
// an invalidation older than the retention window cannot still be suppressing
// anything. Without this the map holds one entry per Agent proxy ever
// invalidated, which repeated create/delete cycles grow without bound even
// though the cached results themselves are capped.
//
// Forgetting is only safe in one direction, which is why the cutoff is not
// negotiable: a *recent* invalidation dropped early would reset the counter to
// the value an in-flight fetch snapshotted before the change, and that fetch
// would then be admitted — reinstating a pre-change result. Past the cutoff no
// such fetch can exist, and the worst case inverts: a fetch that somehow
// outlived the window disagrees with the reset counter and discards its own
// result, costing a cache miss and never admitting a stale card. So when a burst
// of invalidations inside the window leaves the map over its cap, the entries
// stay — a bound that can briefly be exceeded is worth more than one bought by
// serving a superseded card.
func (c *agentCardCache) forgetSettledGenerations() {
	// The sweep is a whole-map walk, so it waits until there is enough to be
	// worth walking. The cap is a multiple of the entry cap because a key with a
	// live entry is exactly the kind that gets invalidated.
	limit := max(4*c.cfg.MaxEntries, agentCardGenerationSweepFloor)
	if len(c.generations) <= limit {
		return
	}
	cutoff := c.now().Add(-agentCardGenerationRetention)
	for key, generation := range c.generations {
		if generation.invalidated.Before(cutoff) {
			delete(c.generations, key)
		}
	}
}

// removeElement drops one element from both the index and the LRU order. The
// caller holds the lock.
func (c *agentCardCache) removeElement(element *list.Element) {
	node := element.Value.(*agentCardCacheNode)
	c.order.Remove(element)
	delete(c.entries, node.key)
	c.totalBytes -= node.entry.bytes
	if c.totalBytes < 0 {
		c.totalBytes = 0
	}
}
