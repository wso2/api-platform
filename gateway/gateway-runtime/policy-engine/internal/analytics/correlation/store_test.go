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
