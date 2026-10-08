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

package correlation

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// TestMain initializes the metrics registry before any test runs -- the store
// records counters on every Put/Get, and the metric variables are nil until
// Init() is called, so without this the first call would panic on a nil interface.
func TestMain(m *testing.M) {
	metrics.Enabled = true
	metrics.Init()
	os.Exit(m.Run())
}

func TestStore_PutTake_Hit(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	p := Payload{RequestHeaders: map[string]string{"host": "example.com"}}
	s.Put("req-1", p)

	got, ok := s.Take("req-1")
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.RequestHeaders["host"] != "example.com" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestStore_Take_MissForUnknownKey(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	if s.Has("never-written") {
		t.Fatal("expected a miss for a key never written")
	}
}

func TestStore_Take_MissForEmptyKey(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	// An empty key models the ext_proc side's uuid fallback when x-request-id was
	// absent -- the write path must never call Put with it (see the kernel-side
	// skip logic), but Take must also refuse to "match" on it defensively.
	if s.Has("") {
		t.Fatal("expected a miss for an empty key")
	}
}

func TestStore_Put_EmptyPayloadIsNoop(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	s.Put("req-1", Payload{})
	if s.Has("req-1") {
		t.Fatal("expected an empty payload to never be stored")
	}
}

func TestStore_Put_EmptyKeyIsNoop(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	s.Put("", Payload{RequestHeaders: map[string]string{"a": "b"}})
	if s.Has("") {
		t.Fatal("expected an empty key to never be stored")
	}
}

// The TTL never hides an entry: a completed entry stays readable until it is
// taken or its slot is reclaimed for a new request.
func TestStore_TTLMakesCompletedEntryReclaimableNotInvisible(t *testing.T) {
	s := NewStore(1, 10*time.Millisecond, 1)
	s.Put("req-1", Payload{RequestHeaders: map[string]string{"host": "example.com"}})

	time.Sleep(30 * time.Millisecond)
	if !s.Has("req-1") {
		t.Fatal("past the TTL but not reclaimed: must still be readable")
	}

	if !s.Merge("req-2", Payload{RequestHeaders: map[string]string{"host": "b"}}) {
		t.Fatal("expected the stale completed entry's slot to be reclaimed")
	}
	if s.Has("req-1") {
		t.Fatal("expected req-1 to have been reclaimed")
	}
}

// A full shard refuses new entries rather than evicting unread ones; the caller
// then keeps those fields in Envoy metadata.
func TestStore_FullShardRefusesInsteadOfEvicting(t *testing.T) {
	const capacity = 8
	s := NewStore(capacity, time.Hour, 1)

	for i := 0; i < capacity; i++ {
		if !s.Merge(fmt.Sprintf("req-%d", i), Payload{RequestHeaders: map[string]string{"i": fmt.Sprintf("%d", i)}}) {
			t.Fatalf("expected req-%d to be accepted", i)
		}
	}
	if s.Merge("req-overflow", Payload{RequestHeaders: map[string]string{"i": "overflow"}}) {
		t.Fatal("expected a full shard to refuse a new entry")
	}
	for i := 0; i < capacity; i++ {
		if !s.Has(fmt.Sprintf("req-%d", i)) {
			t.Fatalf("expected unread req-%d to survive", i)
		}
	}

	// Completed entries within the TTL are not reclaimable either.
	s.Complete("req-0")
	if s.Merge("req-overflow", Payload{RequestHeaders: map[string]string{"i": "overflow"}}) {
		t.Fatal("expected an entry completed within the TTL to be kept")
	}

	// Taking an entry frees its slot for a new one.
	if _, ok := s.Take("req-3"); !ok {
		t.Fatal("expected req-3")
	}
	if !s.Merge("req-overflow", Payload{RequestHeaders: map[string]string{"i": "overflow"}}) {
		t.Fatal("expected the freed slot to be reused")
	}
}

// Merging into an existing entry needs no new slot, so it succeeds on a full shard.
func TestStore_MergeIntoExistingEntryOnFullShard(t *testing.T) {
	s := NewStore(1, time.Hour, 1)
	if !s.Merge("req-1", Payload{RequestHeaders: map[string]string{"a": "1"}}) {
		t.Fatal("expected first merge to succeed")
	}
	if !s.Merge("req-1", Payload{ResponseHeaders: map[string]string{"b": "2"}}) {
		t.Fatal("expected merge into the existing entry to succeed")
	}
	got, ok := s.Take("req-1")
	if !ok || got.RequestHeaders["a"] != "1" || got.ResponseHeaders["b"] != "2" {
		t.Fatalf("expected both phases merged, got %+v (ok=%v)", got, ok)
	}
}

func TestStore_PutTwiceUpdatesInPlace(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	s.Put("req-1", Payload{RequestHeaders: map[string]string{"v": "1"}})
	s.Put("req-1", Payload{RequestHeaders: map[string]string{"v": "2"}})

	got, ok := s.Take("req-1")
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.RequestHeaders["v"] != "2" {
		t.Fatalf("expected the second write to win, got %+v", got)
	}
}

// TestStore_ConcurrentPutHas exercises concurrent writers and readers across many
// shards under -race: each writer only ever writes (and each reader only ever
// reads) its own key, so no assertion about content is made beyond "no race and
// no panic" -- the correctness of individual Put/Get pairs is covered above.
func TestStore_ConcurrentPutHas(t *testing.T) {
	s := NewStore(1000, 200*time.Millisecond, 16)

	const goroutines = 50
	const opsPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("g%d-req-%d", g, i)
				s.Put(key, Payload{
					RequestHeaders:  map[string]string{"host": "example.com"},
					ResponseHeaders: map[string]string{"content-type": "application/json"},
				})
				s.Has(key)
			}
		}(g)
	}
	wg.Wait()
}

func TestStore_Take_RemovesEntryAndFreesSlot(t *testing.T) {
	s := NewStore(1, time.Minute, 1)
	s.Put("a", Payload{RequestHeaders: map[string]string{"h": "1"}})

	got, ok := s.Take("a")
	require.True(t, ok)
	assert.Equal(t, "1", got.RequestHeaders["h"])

	assert.False(t, s.Has("a"), "taken entry must be gone")

	// The single ring slot was freed by Take, so this write must not count as an
	// eviction of "a" and must not disturb a different, live key.
	s.Put("b", Payload{RequestHeaders: map[string]string{"h": "2"}})
	got, ok = s.Take("b")
	require.True(t, ok)
	assert.Equal(t, "2", got.RequestHeaders["h"])
}

func TestStore_Has_DoesNotRemove(t *testing.T) {
	s := NewStoreWithBodyLimits(10, time.Minute, 1, 10, 100)
	s.Put("a", Payload{RequestBody: "x"})
	assert.True(t, s.Has("a"))
	assert.True(t, s.Has("a"), "Has only reports presence")
	got, ok := s.Take("a")
	require.True(t, ok)
	assert.Equal(t, "x", got.RequestBody)
	assert.False(t, s.Has("a"))
	assert.False(t, s.Has(""))
}

// Over the byte budget, a body is refused unless stale completed bodies can be
// reclaimed to make room; unread in-flight bodies are never evicted.
func TestStore_BodyBudget_RefusesOrReclaimsStale(t *testing.T) {
	s := NewStoreWithBodyLimits(100, 10*time.Millisecond, 1, 10, 10)
	require.True(t, s.Merge("in-flight", Payload{RequestBody: "123456"}))
	assert.False(t, s.Merge("new", Payload{RequestBody: "abcdef"}), "no room, and the existing body is in flight")
	_, ok := s.Take("new")
	assert.False(t, ok, "a refused merge stores nothing")
	assert.True(t, s.Merge("headers-only", Payload{RequestHeaders: map[string]string{"h": "v"}}), "headers are not charged")

	s.Complete("in-flight")
	time.Sleep(30 * time.Millisecond)
	assert.True(t, s.Merge("new", Payload{RequestBody: "abcdef"}), "stale completed body reclaimed")
	assert.False(t, s.Has("in-flight"))
	assert.Equal(t, int64(6), s.shards[0].bodyBytes)
}

// A body over the per-body limit is refused outright.
func TestStore_BodyOverLimitRefused(t *testing.T) {
	s := NewStoreWithBodyLimits(100, time.Minute, 1, 4, 100)
	assert.False(t, s.Merge("a", Payload{RequestBody: "12345"}))
	assert.False(t, NewStore(100, time.Minute, 1).Merge("a", Payload{RequestBody: "x"}), "no body limits: bodies refused")
}

func TestStore_BodyBudget_TakeReleasesBytes(t *testing.T) {
	s := NewStoreWithBodyLimits(100, time.Minute, 1, 10, 10)
	s.Put("a", Payload{RequestBody: "123456"})
	_, ok := s.Take("a")
	require.True(t, ok)
	assert.Equal(t, int64(0), s.shards[0].bodyBytes)
	s.Put("b", Payload{RequestBody: "abcdef"})
	assert.True(t, s.Has("b"))
}

func TestStore_BodyBudget_UpdateInPlaceAccounting(t *testing.T) {
	s := NewStoreWithBodyLimits(2, time.Minute, 1, 100, 100)
	require.True(t, s.Merge("a", Payload{RequestBody: "1234"}))
	require.True(t, s.Merge("a", Payload{RequestBody: "12"})) // replaces the request body
	require.True(t, s.Merge("a", Payload{ResponseBody: "123"}))
	assert.Equal(t, int64(5), s.shards[0].bodyBytes)
	_, ok := s.Take("a")
	require.True(t, ok)
	assert.Equal(t, int64(0), s.shards[0].bodyBytes)
}

func TestStore_MaxPayloadBytes(t *testing.T) {
	assert.Equal(t, 0, NewStore(10, time.Minute, 1).MaxPayloadBytes(), "plain store carries no bodies")
	assert.Equal(t, 0, (*Store)(nil).MaxPayloadBytes())
	assert.Equal(t, 0, NewStoreWithBodyLimits(10, time.Minute, 1, 100, 0).MaxPayloadBytes(), "no budget, no bodies")
	assert.Equal(t, 100, NewStoreWithBodyLimits(10, time.Minute, 1, 100, 1000).MaxPayloadBytes())
	// 4 shards x 50 bytes: a body larger than one shard's budget is capped.
	assert.Equal(t, 50, NewStoreWithBodyLimits(10, time.Minute, 4, 100, 200).MaxPayloadBytes())
}

// An entry that never completes is not reclaimable by the TTL, only by the hard
// cap -- the access-log entry of a long streamed response must still find it.
func TestStore_IncompleteEntryOnlyReclaimedAfterHardCap(t *testing.T) {
	s := NewStore(1, time.Nanosecond, 1)
	require.True(t, s.Merge("streaming", Payload{RequestHeaders: map[string]string{"h": "1"}}))
	time.Sleep(time.Millisecond)
	assert.False(t, s.Merge("next", Payload{RequestHeaders: map[string]string{"h": "2"}}),
		"past the TTL but never completed: not reclaimable")

	s.maxIncompleteAge = time.Millisecond
	time.Sleep(2 * time.Millisecond)
	assert.True(t, s.Merge("next", Payload{RequestHeaders: map[string]string{"h": "2"}}), "past the hard cap: reclaimed")
	assert.False(t, s.Has("streaming"))
}

func TestStore_HardCapIsAtLeastTTL(t *testing.T) {
	assert.Equal(t, incompleteEntryMaxAge, NewStore(10, time.Minute, 1).maxIncompleteAge)
	assert.Equal(t, 2*time.Hour, NewStore(10, 2*time.Hour, 1).maxIncompleteAge)
}

func totalSlots(s *Store) int {
	n := 0
	for _, sh := range s.shards {
		n += len(sh.ring)
	}
	return n
}

// More shards than capacity must not silently raise the capacity to one slot per
// shard; the shard count is reduced instead.
func TestStore_ShardsNeverExceedCapacity(t *testing.T) {
	s := NewStore(4, time.Minute, 64)
	assert.Equal(t, 4, len(s.shards))
	assert.Equal(t, 4, totalSlots(s))

	s = NewStore(5, time.Minute, 64)
	assert.Equal(t, 4, len(s.shards), "largest power of two within capacity")
	assert.LessOrEqual(t, totalSlots(s), 5)
}

func TestStore_ShardCountIsCapped(t *testing.T) {
	s := NewStore(10_000_000, time.Minute, 1<<40)
	assert.Equal(t, config.MaxCorrelationStoreShards, len(s.shards))
}

func TestNextPowerOfTwo(t *testing.T) {
	assert.Equal(t, 1, nextPowerOfTwo(1))
	assert.Equal(t, 32, nextPowerOfTwo(17))
	assert.Equal(t, 64, nextPowerOfTwo(64))
	done := make(chan int, 1)
	go func() { done <- nextPowerOfTwo(1<<62 + 1) }()
	select {
	case got := <-done:
		assert.Equal(t, config.MaxCorrelationStoreShards, got, "huge inputs are capped, never loop")
	case <-time.After(time.Second):
		t.Fatal("nextPowerOfTwo did not return for a huge input")
	}
}

func TestStore_IgnoresPath(t *testing.T) {
	s := NewStoreFromConfig(config.CollectorConfig{
		CorrelationStore:   config.CorrelationStoreConfig{Capacity: 10, TTL: time.Minute, Shards: 1},
		IgnorePathPrefixes: []string{"/health", "  ", " /metrics "},
	})
	assert.True(t, s.IgnoresPath("/health"))
	assert.True(t, s.IgnoresPath("/health/live?x=1"))
	assert.True(t, s.IgnoresPath("/metrics"))
	assert.False(t, s.IgnoresPath("/api/health"))
	assert.False(t, s.IgnoresPath(""), "blank prefixes are dropped, never matching everything")
	var nilStore *Store
	assert.False(t, nilStore.IgnoresPath("/health"))
}
