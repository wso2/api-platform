/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package kernel

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// THE invariant. The FIRST attempt's shape is the baseline by definition; a later attempt must
// never overwrite it with its own already-rewritten state. Getting this backwards silently
// restores the compounding the store exists to prevent — attempt 2 would record
// "/ctx-1/chat" as the baseline and attempt 3 would build on that.
func TestAttemptBaselineStore_FirstWriteWins(t *testing.T) {
	store := newAttemptBaselineStore()

	store.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	store.Remember("req-1", requestBaseline{Path: "/ctx-1/v1/chat"}) // a later attempt

	got, ok := store.Lookup("req-1")
	if !ok {
		t.Fatal("expected a baseline")
	}
	if got.Path != "/v1/chat" {
		t.Fatalf("path = %q; a later attempt must not overwrite the first attempt's shape", got.Path)
	}
}

func TestAttemptBaselineStore_SeparatesRequests(t *testing.T) {
	store := newAttemptBaselineStore()
	store.Remember("req-1", requestBaseline{Path: "/one"})
	store.Remember("req-2", requestBaseline{Path: "/two"})

	one, _ := store.Lookup("req-1")
	two, _ := store.Lookup("req-2")
	if one.Path != "/one" || two.Path != "/two" {
		t.Fatalf("entries crossed: %q / %q", one.Path, two.Path)
	}
}

// An unknown request is the first attempt, not an error.
func TestAttemptBaselineStore_UnknownRequest(t *testing.T) {
	store := newAttemptBaselineStore()
	if _, ok := store.Lookup("never-seen"); ok {
		t.Fatal("expected no baseline for an unseen request")
	}
	// An empty id cannot correlate attempts; it must not become a shared bucket every
	// request writes to and reads from.
	store.Remember("", requestBaseline{Path: "/x"})
	if _, ok := store.Lookup(""); ok {
		t.Fatal("an empty request id must never resolve to a baseline")
	}
}

// Nothing deletes an entry on the last attempt, because nothing knows which attempt is last —
// a request that succeeds first time never returns for its entry. The TTL is the only thing
// preventing an unbounded leak.
func TestAttemptBaselineStore_ExpiresEntries(t *testing.T) {
	clock := time.Now()
	store := newAttemptBaselineStore()
	store.now = func() time.Time { return clock }

	store.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	if _, ok := store.Lookup("req-1"); !ok {
		t.Fatal("expected the entry to be live")
	}

	clock = clock.Add(baselineTTL + time.Second)
	if _, ok := store.Lookup("req-1"); ok {
		t.Fatal("expected the entry to have expired")
	}
}

// An expired entry must not block a fresh one: the id could legitimately recur.
func TestAttemptBaselineStore_ExpiredEntryIsReplaceable(t *testing.T) {
	clock := time.Now()
	store := newAttemptBaselineStore()
	store.now = func() time.Time { return clock }

	store.Remember("req-1", requestBaseline{Path: "/old"})
	clock = clock.Add(baselineTTL + time.Second)
	store.Remember("req-1", requestBaseline{Path: "/new"})

	got, ok := store.Lookup("req-1")
	if !ok || got.Path != "/new" {
		t.Fatalf("baseline = %+v ok=%v, want the fresh one", got, ok)
	}
}

// Expired entries are reclaimed on write once the map grows, so a long-running process does not
// accumulate one entry per request ever seen.
func TestAttemptBaselineStore_SweepsOnGrowth(t *testing.T) {
	clock := time.Now()
	store := newAttemptBaselineStore()
	store.now = func() time.Time { return clock }

	for i := 0; i < baselineSweepThreshold; i++ {
		store.Remember(fmt.Sprintf("old-%d", i), requestBaseline{Path: "/old"})
	}
	clock = clock.Add(baselineTTL + time.Second)
	store.Remember("fresh", requestBaseline{Path: "/fresh"})

	store.mu.Lock()
	size := len(store.entries)
	store.mu.Unlock()
	if size != 1 {
		t.Fatalf("entries = %d, want 1 — expired entries must be reclaimed on growth", size)
	}
}

func TestAttemptBaselineStore_Forget(t *testing.T) {
	store := newAttemptBaselineStore()
	store.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	store.Forget("req-1")
	if _, ok := store.Lookup("req-1"); ok {
		t.Fatal("expected the entry to be gone")
	}
}

// Attempts of different requests are concurrent by nature.
func TestAttemptBaselineStore_ConcurrentAccess(t *testing.T) {
	store := newAttemptBaselineStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("req-%d", n)
			store.Remember(id, requestBaseline{Path: fmt.Sprintf("/p-%d", n)})
			if got, ok := store.Lookup(id); !ok || got.Path != fmt.Sprintf("/p-%d", n) {
				t.Errorf("request %d read back %+v", n, got)
			}
		}(i)
	}
	wg.Wait()
}

// FR-037h. The serving cluster is LAST write wins, where the baseline is FIRST write wins.
// The asymmetry is the whole point: the baseline is attempt 1's shape, while the serving cluster
// is whichever attempt ended up answering — Envoy stops retrying once one is committed, so the
// last recorded is the one whose response the client receives.
func TestAttemptBaselineStore_ServingClusterLastWriteWins(t *testing.T) {
	store := newAttemptBaselineStore()
	store.Remember("req-1", requestBaseline{Path: "/v1/chat"})

	store.RememberServingCluster("req-1", "cluster_openai")
	store.RememberServingCluster("req-1", "cluster_anthropic") // the retry that served

	got, ok := store.ServingCluster("req-1")
	if !ok || got != "cluster_anthropic" {
		t.Fatalf("ServingCluster = %q (ok=%v), want the LAST attempt's cluster", got, ok)
	}

	// And the baseline is untouched by it — different fact, different write policy.
	baseline, known := store.Lookup("req-1")
	if !known || baseline.Path != "/v1/chat" {
		t.Fatalf("baseline Path = %q, want attempt 1's", baseline.Path)
	}
}

// A request that never reached the upstream position has no serving cluster, so the response
// phase unwinds nothing — the ordinary case for every API without per-attempt work.
func TestAttemptBaselineStore_NoServingClusterWithoutAnAttempt(t *testing.T) {
	store := newAttemptBaselineStore()
	store.Remember("req-1", requestBaseline{Path: "/v1/chat"})

	if got, ok := store.ServingCluster("req-1"); ok {
		t.Fatalf("ServingCluster = %q, want none recorded", got)
	}
	if _, ok := store.ServingCluster("unknown"); ok {
		t.Fatal("an unknown request must report no serving cluster")
	}
}

// Writing a serving cluster for a request with no entry is a no-op rather than an invention:
// the entry is written by the same attempt that would record this, so its absence means the
// request id is empty or the entry expired mid-request.
func TestAttemptBaselineStore_ServingClusterNeedsAnEntry(t *testing.T) {
	store := newAttemptBaselineStore()

	store.RememberServingCluster("req-1", "cluster_openai")

	if _, ok := store.ServingCluster("req-1"); ok {
		t.Fatal("recorded a serving cluster for a request with no baseline entry")
	}
}
