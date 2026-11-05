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
	"sync"
	"time"
)

// baselineTTL bounds how long a request's baseline is remembered.
//
// An entry is written by the first attempt and read by later ones, so it only has to outlive a
// single request. Nothing deletes it on the last attempt because nothing knows which attempt is
// last — a request that succeeds on attempt 1 never comes back for its entry at all. The TTL is
// therefore the only thing standing between this store and an unbounded leak, and it is set well
// above any plausible route timeout so a slow chain still finds its baseline.
const baselineTTL = 5 * time.Minute

// baselineSweepThreshold is the size at which a write sweeps expired entries.
//
// Swept on write rather than by a background goroutine: the store would otherwise need a
// lifecycle (start, stop, leak-on-shutdown) for work that is cheap and only matters when entries
// are actually accumulating.
const baselineSweepThreshold = 1024

// attemptBaselineStore remembers, per request, the shape of the request as it entered the
// upstream chain on the FIRST attempt.
//
// Envoy hands each retry the request as the PREVIOUS attempt left it — both probes measured
// this, a naive prefix rewrite producing "/ctx-2/ctx-1/chat/completions". Resetting to the first
// attempt's shape is what stops a derived value compounding on every retry (FR-038, FR-038a).
//
// Held server-side rather than round-tripped through Envoy's dynamic metadata. The metadata
// route would rest on an assumption never measured — that an UPSTREAM ext_proc's metadata write
// survives into the next attempt — whereas process memory simply does. Agent Router keeps the
// post-downstream request in its own memory for the same reason.
type attemptBaselineStore struct {
	mu      sync.Mutex
	entries map[string]baselineEntry
	// now is injectable so tests can drive expiry without sleeping.
	now func() time.Time
}

type baselineEntry struct {
	baseline requestBaseline
	expires  time.Time
}

func newAttemptBaselineStore() *attemptBaselineStore {
	return &attemptBaselineStore{
		entries: make(map[string]baselineEntry),
		now:     time.Now,
	}
}

// Remember records a request's baseline if one is not already held.
//
// First write wins: the FIRST attempt's shape is the baseline by definition, so a later attempt
// must never overwrite it with its own already-rewritten state. That is the whole point, and
// getting it backwards would silently restore the compounding this store exists to prevent.
func (s *attemptBaselineStore) Remember(requestID string, baseline requestBaseline) {
	if requestID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.entries[requestID]; ok && existing.expires.After(s.now()) {
		return
	}
	if len(s.entries) >= baselineSweepThreshold {
		s.sweepLocked()
	}
	s.entries[requestID] = baselineEntry{baseline: baseline, expires: s.now().Add(baselineTTL)}
}

// RememberTouchedHeaders merges the headers a per-attempt chain changed into this request's
// baseline, keeping the FIRST value recorded for each name.
//
// Separate from Remember because the two are written at different moments: the request line is
// known before the chain runs, the touched headers only after it has run. First-touch-wins for
// the same reason Remember is first-write-wins — the earliest attempt to touch a header is the
// one holding its post-downstream value, and a later attempt would record its own predecessor's
// leftover instead.
//
// A no-op when the request has no entry: the entry is written by the same attempt that would
// call this, so its absence means the request ID is empty or the entry expired mid-request, and
// inventing one here would record a polluted value as if it were the baseline.
func (s *attemptBaselineStore) RememberTouchedHeaders(requestID string, touched map[string]headerBaseline) {
	if requestID == "" || len(touched) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[requestID]
	if !ok || !entry.expires.After(s.now()) {
		return
	}
	if entry.baseline.Headers == nil {
		entry.baseline.Headers = make(map[string]headerBaseline, len(touched))
	}
	for name, base := range touched {
		if _, seen := entry.baseline.Headers[name]; seen {
			continue
		}
		entry.baseline.Headers[name] = base
	}
	s.entries[requestID] = entry
}

// Lookup returns the baseline recorded by this request's first attempt. ok=false means this IS
// the first attempt (or the entry has expired, which is treated the same way: the attempt
// becomes its own baseline rather than failing).
func (s *attemptBaselineStore) Lookup(requestID string) (requestBaseline, bool) {
	if requestID == "" {
		return requestBaseline{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[requestID]
	if !ok || !entry.expires.After(s.now()) {
		return requestBaseline{}, false
	}
	return entry.baseline, true
}

// Forget drops a request's entry. Best-effort tidying for the paths that know a request is
// finished; correctness does not depend on it being called, which is why the TTL exists.
func (s *attemptBaselineStore) Forget(requestID string) {
	if requestID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, requestID)
}

func (s *attemptBaselineStore) sweepLocked() {
	now := s.now()
	for id, entry := range s.entries {
		if !entry.expires.After(now) {
			delete(s.entries, id)
		}
	}
}
