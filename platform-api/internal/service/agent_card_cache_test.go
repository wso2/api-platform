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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

// newTestAgentCardCache builds a cache whose clock the test drives, so TTL and
// Age behaviour can be asserted without sleeping.
func newTestAgentCardCache(t *testing.T, cfg config.AgentCardCache) (*agentCardCache, func(time.Duration)) {
	t.Helper()

	cache := newAgentCardCache(cfg)
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	cache.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(d)
	}
	return cache, advance
}

func defaultTestCardCacheConfig() config.AgentCardCache {
	return config.AgentCardCache{
		PositiveTTL: 60 * time.Second,
		NegativeTTL: 10 * time.Second,
		MaxEntries:  4,
		MaxBytes:    1024,
	}
}

func testCardCacheKey(org, proxy string) agentCardCacheKey {
	return agentCardCacheKey{orgUUID: org, proxyUUID: proxy}
}

func TestAgentCardCache_ServesAStoredCardWithItsAge(t *testing.T) {
	t.Parallel()

	cache, advance := newTestAgentCardCache(t, defaultTestCardCacheConfig())
	key := testCardCacheKey("org-a", "proxy-1")
	card := []byte(`{"name":"Weather Agent"}`)

	cache.storeCard(key, cache.begin(key), card)

	advance(12 * time.Second)
	entry, age, ok := cache.get(key)
	if !ok {
		t.Fatal("get() reported a miss inside the positive TTL")
	}
	if string(entry.card) != string(card) {
		t.Errorf("cached card = %s, want %s", entry.card, card)
	}
	if age != 12*time.Second {
		t.Errorf("age = %v, want %v", age, 12*time.Second)
	}
}

// A lapsed entry is a miss, not a stale hit — and the miss drops it rather than
// leaving it to accumulate.
func TestAgentCardCache_EntriesExpireAtTheirOwnTTL(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cache, advance := newTestAgentCardCache(t, cfg)
	success := testCardCacheKey("org-a", "proxy-success")
	failure := testCardCacheKey("org-a", "proxy-failure")

	cache.storeCard(success, cache.begin(success), []byte(`{"name":"a"}`))
	cache.storeFailure(failure, cache.begin(failure), "upstream unreachable")

	// Past the negative TTL but still inside the positive one: the failure has
	// lapsed and the success has not. Caching failures for as long as successes
	// would leave a recovered upstream unreachable for a full minute.
	advance(cfg.NegativeTTL + time.Second)
	if _, _, ok := cache.get(failure); ok {
		t.Error("a failure survived past the negative TTL")
	}
	if _, _, ok := cache.get(success); !ok {
		t.Error("a success expired at the negative TTL rather than the positive one")
	}

	advance(cfg.PositiveTTL)
	if _, _, ok := cache.get(success); ok {
		t.Error("a success survived past the positive TTL")
	}
}

func TestAgentCardCache_CachesFailuresWithTheirMessage(t *testing.T) {
	t.Parallel()

	cache, advance := newTestAgentCardCache(t, defaultTestCardCacheConfig())
	key := testCardCacheKey("org-a", "proxy-1")

	cache.storeFailure(key, cache.begin(key), "the upstream could not be reached")

	advance(3 * time.Second)
	entry, age, ok := cache.get(key)
	if !ok {
		t.Fatal("get() reported a miss inside the negative TTL")
	}
	if entry.failure == nil {
		t.Fatalf("entry = %+v, want a cached failure", entry)
	}
	if entry.failure.message != "the upstream could not be reached" {
		t.Errorf("failure message = %q, want the stored one", entry.failure.message)
	}
	if entry.card != nil {
		t.Errorf("a cached failure carries a card: %s", entry.card)
	}
	if age != 3*time.Second {
		t.Errorf("age = %v, want %v", age, 3*time.Second)
	}
}

// Two organizations pointing at the same upstream must never see each other's
// card: they may hold different credentials for it, so the key is the Agent
// proxy within its organization, never the URL.
func TestAgentCardCache_IsolatesOrganizations(t *testing.T) {
	t.Parallel()

	cache, _ := newTestAgentCardCache(t, defaultTestCardCacheConfig())
	a := testCardCacheKey("org-a", "proxy-1")
	b := testCardCacheKey("org-b", "proxy-1")

	cache.storeCard(a, cache.begin(a), []byte(`{"name":"org-a card"}`))

	if _, _, ok := cache.get(b); ok {
		t.Fatal("an entry stored for one organization was served to another")
	}

	cache.storeCard(b, cache.begin(b), []byte(`{"name":"org-b card"}`))
	entryA, _, _ := cache.get(a)
	entryB, _, _ := cache.get(b)
	if string(entryA.card) == string(entryB.card) {
		t.Error("the two organizations' entries collided on one key")
	}
}

// Invalidation is immediate rather than TTL-deferred: the upstream URL, its
// credentials or the card mode may have just changed, and a stale failure would
// otherwise outlive the fix by a full TTL.
func TestAgentCardCache_InvalidateDropsTheEntryImmediately(t *testing.T) {
	t.Parallel()

	cache, _ := newTestAgentCardCache(t, defaultTestCardCacheConfig())
	key := testCardCacheKey("org-a", "proxy-1")

	cache.storeCard(key, cache.begin(key), []byte(`{"name":"a"}`))
	cache.invalidate(key.orgUUID, key.proxyUUID)

	if _, _, ok := cache.get(key); ok {
		t.Error("get() reported a hit after invalidate()")
	}
}

// The generation guard: a fetch already in flight when the Agent proxy changed
// describes the old configuration, so its result must not reinstate the entry
// the change dropped.
func TestAgentCardCache_AnInFlightFetchCannotRefillAnInvalidatedEntry(t *testing.T) {
	t.Parallel()

	cache, _ := newTestAgentCardCache(t, defaultTestCardCacheConfig())
	key := testCardCacheKey("org-a", "proxy-1")

	generation := cache.begin(key) // fetch starts
	cache.invalidate(key.orgUUID, key.proxyUUID)
	cache.storeCard(key, generation, []byte(`{"name":"stale"}`)) // fetch returns

	if _, _, ok := cache.get(key); ok {
		t.Error("a fetch in flight across an invalidation repopulated the dropped entry")
	}

	// A fetch that starts after the invalidation stores normally, so the guard
	// suppresses the stale write only and does not disable the cache.
	cache.storeCard(key, cache.begin(key), []byte(`{"name":"fresh"}`))
	if _, _, ok := cache.get(key); !ok {
		t.Error("a fetch started after the invalidation failed to populate the entry")
	}
}

// A fetch supersedes whatever was held for its key, whether or not its own
// result can be retained.
//
// The case that motivates it: with negative caching off, an explicit no-cache
// refresh that comes back 503 would otherwise leave the previous successful
// card in place — and the next ordinary request would be handed a card the
// refresh had already established was no longer what the upstream serves.
func TestAgentCardCache_SupersededEntriesGoEvenWhenTheReplacementCannotBeHeld(t *testing.T) {
	t.Parallel()

	t.Run("replacement has a disabled TTL", func(t *testing.T) {
		t.Parallel()

		cfg := defaultTestCardCacheConfig()
		cfg.NegativeTTL = 0
		cache, _ := newTestAgentCardCache(t, cfg)
		key := testCardCacheKey("org-a", "proxy-1")

		cache.storeCard(key, cache.begin(key), []byte(`{"name":"the card as it was"}`))
		cache.storeFailure(key, cache.begin(key), "the upstream could not be reached")

		if entry, _, ok := cache.get(key); ok {
			t.Errorf("the superseded card survived a failure that could not be cached: %s", entry.card)
		}
	})

	t.Run("replacement exceeds the byte ceiling", func(t *testing.T) {
		t.Parallel()

		cfg := defaultTestCardCacheConfig()
		cfg.MaxBytes = 64
		cache, _ := newTestAgentCardCache(t, cfg)
		key := testCardCacheKey("org-a", "proxy-1")

		cache.storeCard(key, cache.begin(key), make([]byte, 32))
		cache.storeCard(key, cache.begin(key), make([]byte, 65))

		if _, _, ok := cache.get(key); ok {
			t.Error("the superseded card survived a replacement too large to hold")
		}
		if cache.totalBytes != 0 {
			t.Errorf("totalBytes = %d, want 0 once the key holds nothing", cache.totalBytes)
		}
	})

	t.Run("a stale-generation result supersedes nothing", func(t *testing.T) {
		t.Parallel()

		cache, _ := newTestAgentCardCache(t, defaultTestCardCacheConfig())
		key := testCardCacheKey("org-a", "proxy-1")

		generation := cache.begin(key) // fetch starts
		cache.invalidate(key.orgUUID, key.proxyUUID)
		cache.storeCard(key, cache.begin(key), []byte(`{"name":"post-change"}`))

		// The in-flight fetch returns last, describing the Agent proxy as it was
		// before the change. It is discarded — and must not take the newer entry
		// with it on the way out.
		cache.storeCard(key, generation, []byte(`{"name":"pre-change"}`))

		entry, _, ok := cache.get(key)
		if !ok {
			t.Fatal("a discarded stale result dropped the newer entry")
		}
		if string(entry.card) != `{"name":"post-change"}` {
			t.Errorf("cached card = %s, want the post-change one", entry.card)
		}
	})
}

// The generation map is bookkeeping, not a cache, so nothing evicts it — it has
// to forget on its own or it grows for as long as the process runs.
func TestAgentCardCache_ForgetsSettledInvalidations(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cache, advance := newTestAgentCardCache(t, cfg)
	limit := max(4*cfg.MaxEntries, agentCardGenerationSweepFloor)

	// Every create/delete cycle uses a fresh UUID, so these keys never repeat.
	for i := range limit + 1 {
		cache.invalidate("org-a", fmt.Sprintf("proxy-%d", i))
	}
	// The sweep runs — the map is over its cap — but every invalidation is still
	// inside the retention window, so it correctly drops none of them.
	if len(cache.generations) != limit+1 {
		t.Fatalf("generations = %d immediately after the burst, want all %d retained", len(cache.generations), limit+1)
	}

	advance(agentCardGenerationRetention + time.Second)
	for i := range limit + 1 {
		cache.invalidate("org-a", fmt.Sprintf("later-proxy-%d", i))
	}

	// Only the second burst survives: the first has settled and is forgotten, so
	// the map tracks the recent write rate rather than every Agent proxy the
	// process has ever seen.
	if got, want := len(cache.generations), limit+1; got != want {
		t.Errorf("generations = %d, want %d — the settled invalidations should have been forgotten", got, want)
	}
	for i := range limit + 1 {
		if _, held := cache.generations[testCardCacheKey("org-a", fmt.Sprintf("proxy-%d", i))]; held {
			t.Fatalf("a settled invalidation was retained: proxy-%d", i)
		}
	}
}

// Forgetting is safe in one direction only. An invalidation still inside the
// retention window may be racing a fetch that snapshotted the counter before it,
// so dropping it would reinstate that fetch's pre-change result.
func TestAgentCardCache_KeepsInvalidationsThatCouldStillBeRacingAFetch(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cache, advance := newTestAgentCardCache(t, cfg)
	limit := max(4*cfg.MaxEntries, agentCardGenerationSweepFloor)

	contended := testCardCacheKey("org-a", "proxy-contended")
	generation := cache.begin(contended) // a fetch starts against the old configuration
	cache.invalidate(contended.orgUUID, contended.proxyUUID)

	// A burst of unrelated invalidations pushes the map well past its cap, but
	// all of them — including the contended one — are younger than the cutoff.
	advance(agentCardGenerationRetention / 2)
	for i := range 4 * limit {
		cache.invalidate("org-a", fmt.Sprintf("proxy-%d", i))
	}

	if _, held := cache.generations[contended]; !held {
		t.Fatal("an invalidation inside the retention window was forgotten")
	}
	cache.storeCard(contended, generation, []byte(`{"name":"pre-change"}`))
	if entry, _, ok := cache.get(contended); ok {
		t.Errorf("a pre-change result was reinstated after its invalidation was forgotten: %s", entry.card)
	}
}

// A cache that retains nothing has nothing to protect, so it records nothing
// either — otherwise create/delete cycles would grow memory on a deployment
// that had switched caching off precisely to avoid holding anything.
func TestAgentCardCache_DisabledCacheRecordsNoInvalidations(t *testing.T) {
	t.Parallel()

	cache, _ := newTestAgentCardCache(t, config.AgentCardCache{})
	for i := range 500 {
		cache.invalidate("org-a", fmt.Sprintf("proxy-%d", i))
	}

	if len(cache.generations) != 0 {
		t.Errorf("generations = %d on a disabled cache, want 0", len(cache.generations))
	}
}

// Eviction is a miss for whoever comes back for the evicted key, never an error.
func TestAgentCardCache_EvictsLeastRecentlyUsedAtTheEntryCap(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cfg.MaxEntries = 2
	cache, _ := newTestAgentCardCache(t, cfg)

	first := testCardCacheKey("org-a", "proxy-1")
	second := testCardCacheKey("org-a", "proxy-2")
	third := testCardCacheKey("org-a", "proxy-3")

	cache.storeCard(first, cache.begin(first), []byte(`{"name":"1"}`))
	cache.storeCard(second, cache.begin(second), []byte(`{"name":"2"}`))

	// Touching the first makes the second the least recently used.
	if _, _, ok := cache.get(first); !ok {
		t.Fatal("the first entry was not cached")
	}
	cache.storeCard(third, cache.begin(third), []byte(`{"name":"3"}`))

	if _, _, ok := cache.get(second); ok {
		t.Error("the least recently used entry survived the entry cap")
	}
	if _, _, ok := cache.get(first); !ok {
		t.Error("a recently used entry was evicted instead")
	}
	if _, _, ok := cache.get(third); !ok {
		t.Error("the newest entry was not cached")
	}
}

func TestAgentCardCache_EvictsAtTheByteCeiling(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cfg.MaxEntries = 100
	cfg.MaxBytes = 200
	cache, _ := newTestAgentCardCache(t, cfg)

	card := make([]byte, 80)
	var keys []agentCardCacheKey
	for i := range 3 {
		key := testCardCacheKey("org-a", fmt.Sprintf("proxy-%d", i))
		keys = append(keys, key)
		cache.storeCard(key, cache.begin(key), card)
	}

	if _, _, ok := cache.get(keys[0]); ok {
		t.Error("the oldest entry survived the byte ceiling")
	}
	if cache.totalBytes > cfg.MaxBytes {
		t.Errorf("totalBytes = %d, want at most %d", cache.totalBytes, cfg.MaxBytes)
	}
}

// A card bigger than the whole ceiling is not admitted: evicting every other
// entry to make room for one that still exceeds the cap serves nobody.
func TestAgentCardCache_RefusesAnEntryLargerThanTheCeiling(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cfg.MaxBytes = 64
	cache, _ := newTestAgentCardCache(t, cfg)

	keep := testCardCacheKey("org-a", "proxy-keep")
	huge := testCardCacheKey("org-a", "proxy-huge")

	cache.storeCard(keep, cache.begin(keep), make([]byte, 32))
	cache.storeCard(huge, cache.begin(huge), make([]byte, 65))

	if _, _, ok := cache.get(huge); ok {
		t.Error("an entry larger than the whole ceiling was admitted")
	}
	if _, _, ok := cache.get(keep); !ok {
		t.Error("admitting an oversized entry evicted the entries already held")
	}
}

// Every zeroed knob is an off switch, not an "unlimited" setting: an unbounded
// map keyed per Agent proxy, each entry up to 1 MiB, is a memory-exhaustion
// vector rather than a nicety.
func TestAgentCardCache_DisabledConfigurationsRetainNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  config.AgentCardCache
	}{
		{name: "zero positive ttl", cfg: config.AgentCardCache{PositiveTTL: 0, NegativeTTL: 10 * time.Second, MaxEntries: 4, MaxBytes: 1024}},
		{name: "zero max entries", cfg: config.AgentCardCache{PositiveTTL: time.Minute, NegativeTTL: 10 * time.Second, MaxBytes: 1024}},
		{name: "zero max bytes", cfg: config.AgentCardCache{PositiveTTL: time.Minute, NegativeTTL: 10 * time.Second, MaxEntries: 4}},
		{name: "wholly unconfigured", cfg: config.AgentCardCache{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cache, _ := newTestAgentCardCache(t, tc.cfg)
			key := testCardCacheKey("org-a", "proxy-1")
			cache.storeCard(key, cache.begin(key), []byte(`{"name":"a"}`))

			if _, _, ok := cache.get(key); ok {
				t.Error("a disabled cache retained an entry")
			}
		})
	}
}

// A zero negative TTL disables failure caching alone, leaving successes cached —
// the two knobs are independent.
func TestAgentCardCache_ZeroNegativeTTLLeavesSuccessesCached(t *testing.T) {
	t.Parallel()

	cfg := defaultTestCardCacheConfig()
	cfg.NegativeTTL = 0
	cache, _ := newTestAgentCardCache(t, cfg)

	success := testCardCacheKey("org-a", "proxy-success")
	failure := testCardCacheKey("org-a", "proxy-failure")

	cache.storeCard(success, cache.begin(success), []byte(`{"name":"a"}`))
	cache.storeFailure(failure, cache.begin(failure), "unreachable")

	if _, _, ok := cache.get(failure); ok {
		t.Error("a failure was cached under a zero negative TTL")
	}
	if _, _, ok := cache.get(success); !ok {
		t.Error("a zero negative TTL also disabled success caching")
	}
}

// A nil cache is a usable no-op, so callers need no nil check of their own.
func TestAgentCardCache_NilReceiverIsANoOp(t *testing.T) {
	t.Parallel()

	var cache *agentCardCache
	key := testCardCacheKey("org-a", "proxy-1")

	cache.storeCard(key, cache.begin(key), []byte(`{"name":"a"}`))
	cache.storeFailure(key, 0, "unreachable")
	cache.invalidate(key.orgUUID, key.proxyUUID)

	if _, _, ok := cache.get(key); ok {
		t.Error("a nil cache reported a hit")
	}
	if cache.positiveTTL() != 0 || cache.negativeTTL() != 0 {
		t.Error("a nil cache reported a non-zero TTL")
	}
}

// Concurrent traffic against one key must not corrupt the index, the LRU order
// or the byte accounting.
func TestAgentCardCache_IsSafeUnderConcurrentUse(t *testing.T) {
	t.Parallel()

	cache, _ := newTestAgentCardCache(t, defaultTestCardCacheConfig())

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := testCardCacheKey("org-a", fmt.Sprintf("proxy-%d", i%3))
			for range 50 {
				generation := cache.begin(key)
				cache.storeCard(key, generation, []byte(`{"name":"a"}`))
				cache.get(key)
				cache.storeFailure(key, cache.begin(key), "unreachable")
				cache.invalidate(key.orgUUID, key.proxyUUID)
			}
		}(i)
	}
	wg.Wait()

	if cache.totalBytes < 0 {
		t.Errorf("totalBytes = %d, want it never to go negative", cache.totalBytes)
	}
	if len(cache.entries) != cache.order.Len() {
		t.Errorf("index holds %d entries but the LRU order holds %d", len(cache.entries), cache.order.Len())
	}
}
