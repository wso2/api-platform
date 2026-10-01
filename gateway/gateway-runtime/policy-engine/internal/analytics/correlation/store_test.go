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

func TestStore_PutGet_Hit(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	p := Payload{RequestHeaders: map[string]string{"host": "example.com"}}
	s.Put("req-1", p)

	got, ok := s.Get("req-1")
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.RequestHeaders["host"] != "example.com" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestStore_Get_MissForUnknownKey(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	if _, ok := s.Get("never-written"); ok {
		t.Fatal("expected a miss for a key never written")
	}
}

func TestStore_Get_MissForEmptyKey(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	// An empty key models the ext_proc side's uuid fallback when x-request-id was
	// absent -- the write path must never call Put with it (see the kernel-side
	// skip logic), but Get must also refuse to "match" on it defensively.
	if _, ok := s.Get(""); ok {
		t.Fatal("expected a miss for an empty key")
	}
}

func TestStore_Put_EmptyPayloadIsNoop(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	s.Put("req-1", Payload{})
	if _, ok := s.Get("req-1"); ok {
		t.Fatal("expected an empty payload to never be stored")
	}
}

func TestStore_Put_EmptyKeyIsNoop(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	s.Put("", Payload{RequestHeaders: map[string]string{"a": "b"}})
	if _, ok := s.Get(""); ok {
		t.Fatal("expected an empty key to never be stored")
	}
}

func TestStore_TTLExpiry(t *testing.T) {
	s := NewStore(100, 10*time.Millisecond, 1)
	s.Put("req-1", Payload{RequestHeaders: map[string]string{"host": "example.com"}})

	if _, ok := s.Get("req-1"); !ok {
		t.Fatal("expected a hit immediately after Put")
	}

	time.Sleep(30 * time.Millisecond)

	if _, ok := s.Get("req-1"); ok {
		t.Fatal("expected the entry to have expired")
	}
}

// TestStore_EvictionUnderCapacityPressure fills a single-shard store to capacity
// and writes one more entry, and asserts the oldest (first-written) entry was
// evicted (FIFO) while capacity is never exceeded.
func TestStore_EvictionUnderCapacityPressure(t *testing.T) {
	const capacity = 8
	s := NewStore(capacity, time.Hour, 1) // 1 shard: deterministic FIFO order

	for i := 0; i < capacity; i++ {
		s.Put(fmt.Sprintf("req-%d", i), Payload{RequestHeaders: map[string]string{"i": fmt.Sprintf("%d", i)}})
	}
	// Every entry should still be present -- the ring isn't over capacity yet.
	for i := 0; i < capacity; i++ {
		if _, ok := s.Get(fmt.Sprintf("req-%d", i)); !ok {
			t.Fatalf("expected req-%d to still be present before overflow", i)
		}
	}

	// One more write should evict the oldest entry (req-0).
	s.Put("req-overflow", Payload{RequestHeaders: map[string]string{"i": "overflow"}})

	if _, ok := s.Get("req-0"); ok {
		t.Fatal("expected the oldest entry to have been evicted under capacity pressure")
	}
	if _, ok := s.Get("req-overflow"); !ok {
		t.Fatal("expected the new entry to be present")
	}
	// The rest of the window survives.
	for i := 1; i < capacity; i++ {
		if _, ok := s.Get(fmt.Sprintf("req-%d", i)); !ok {
			t.Fatalf("expected req-%d to survive the eviction", i)
		}
	}
}

func TestStore_PutTwiceUpdatesInPlace(t *testing.T) {
	s := NewStore(100, time.Minute, 4)
	s.Put("req-1", Payload{RequestHeaders: map[string]string{"v": "1"}})
	s.Put("req-1", Payload{RequestHeaders: map[string]string{"v": "2"}})

	got, ok := s.Get("req-1")
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.RequestHeaders["v"] != "2" {
		t.Fatalf("expected the second write to win, got %+v", got)
	}
}

// TestStore_ConcurrentPutGet exercises concurrent writers and readers across many
// shards under -race: each writer only ever writes (and each reader only ever
// reads) its own key, so no assertion about content is made beyond "no race and
// no panic" -- the correctness of individual Put/Get pairs is covered above.
func TestStore_ConcurrentPutGet(t *testing.T) {
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
				s.Get(key)
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

	_, ok = s.Get("a")
	assert.False(t, ok, "taken entry must be gone")

	// The single ring slot was freed by Take, so this write must not count as an
	// eviction of "a" and must not disturb a different, live key.
	s.Put("b", Payload{RequestHeaders: map[string]string{"h": "2"}})
	got, ok = s.Get("b")
	require.True(t, ok)
	assert.Equal(t, "2", got.RequestHeaders["h"])
}

func TestStore_Get_DoesNotRemove(t *testing.T) {
	s := NewStore(10, time.Minute, 1)
	s.Put("a", Payload{RequestBody: "x"})
	_, ok := s.Get("a")
	require.True(t, ok)
	_, ok = s.Get("a")
	assert.True(t, ok, "Get only peeks")
}

func TestStore_BodyBudget_EvictsOldestBodies(t *testing.T) {
	// One shard, room for 100 entries but only 10 body bytes.
	s := NewStoreWithBodyLimits(100, time.Minute, 1, 10, 10)
	s.Put("old", Payload{RequestBody: "123456"})
	s.Put("headers-only", Payload{RequestHeaders: map[string]string{"h": "v"}})
	s.Put("new", Payload{RequestBody: "abcdef"})

	_, ok := s.Get("old")
	assert.False(t, ok, "oldest body evicted to fit the budget")
	got, ok := s.Get("new")
	require.True(t, ok)
	assert.Equal(t, "abcdef", got.RequestBody)
	_, ok = s.Get("headers-only")
	assert.True(t, ok, "entries without bodies are not charged and not evicted for bytes")

	sh := s.shards[0]
	assert.Equal(t, int64(6), sh.bodyBytes)
}

func TestStore_BodyBudget_TakeReleasesBytes(t *testing.T) {
	s := NewStoreWithBodyLimits(100, time.Minute, 1, 10, 10)
	s.Put("a", Payload{RequestBody: "123456"})
	_, ok := s.Take("a")
	require.True(t, ok)
	assert.Equal(t, int64(0), s.shards[0].bodyBytes)
	s.Put("b", Payload{RequestBody: "abcdef"})
	_, ok = s.Get("b")
	assert.True(t, ok)
}

func TestStore_BodyBudget_UpdateInPlaceAndRingEvictionAccounting(t *testing.T) {
	s := NewStoreWithBodyLimits(2, time.Minute, 1, 100, 100)
	s.Put("a", Payload{RequestBody: "1234"})
	s.Put("a", Payload{RequestBody: "12"}) // update in place
	assert.Equal(t, int64(2), s.shards[0].bodyBytes)
	s.Put("b", Payload{RequestBody: "123"})
	s.Put("c", Payload{RequestBody: "1"}) // ring of 2 wraps and evicts "a"
	assert.Equal(t, int64(4), s.shards[0].bodyBytes)
	_, ok := s.Get("a")
	assert.False(t, ok)
}

func TestStore_MaxPayloadBytes(t *testing.T) {
	assert.Equal(t, 0, NewStore(10, time.Minute, 1).MaxPayloadBytes(), "plain store carries no bodies")
	assert.Equal(t, 0, (*Store)(nil).MaxPayloadBytes())
	assert.Equal(t, 0, NewStoreWithBodyLimits(10, time.Minute, 1, 100, 0).MaxPayloadBytes(), "no budget, no bodies")
	assert.Equal(t, 100, NewStoreWithBodyLimits(10, time.Minute, 1, 100, 1000).MaxPayloadBytes())
	// 4 shards x 50 bytes: a body larger than one shard's budget is capped.
	assert.Equal(t, 50, NewStoreWithBodyLimits(10, time.Minute, 4, 100, 200).MaxPayloadBytes())
}
