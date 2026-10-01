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

// Package correlation implements the in-process handoff of captured analytics
// headers from the ext_proc handler to the ALS (access-log) handler, keyed by the
// request's Envoy-generated x-request-id.
//
// Before this store existed, captured request/response headers made a full round
// trip through Envoy for no reason other than correlating them back to the request
// they belonged to: the analytics system policy JSON-encoded them into ext_proc
// dynamic metadata, Envoy echoed the whole filter_metadata struct back in the
// HTTPAccessLogEntry, and the ALS handler JSON-decoded them again (twice). Since
// the ext_proc handler and the ALS handler are two goroutines in the very same
// process, that round trip bought nothing but CPU (profiled at ~17% of sampled CPU
// with traffic logging on -- see the traffic-logging CPU plan, Step 4). This store
// lets the ext_proc handler stash the already-typed header maps directly, and the
// ALS handler fetch them back by request id instead of decoding Envoy's echo.
//
// A miss is an expected, non-error outcome, not a bug: an HTTPAccessLogEntry is
// produced for requests that never had an ext_proc stream at all (no-route 404s,
// pre-filter rejections), and there is no happens-before relationship between an
// ext_proc stream ending and that request's access-log entry arriving -- they are
// two independent gRPC services on two independent goroutines. Callers must treat
// a miss as "fall back to whatever the access-log entry itself carries", never as
// a reason to drop the log line.
package correlation

import (
	"hash/maphash"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// Payload is the in-process analytics data captured by the ext_proc handler for
// one request and looked up by the ALS handler when that request's access-log
// entry arrives. It intentionally carries only the two fields that used to make
// the Envoy round trip described in the package doc comment -- every other
// analytics field (API identity, auth context, subscription, AI/MCP metadata...)
// is untouched by this change and continues to travel through Envoy dynamic
// metadata exactly as before.
//
// Ownership: Put takes ownership of the maps passed to it -- the caller must treat
// them as immutable afterward. Get hands back the same map values (no defensive
// copy, to avoid an allocation on every access-log entry); callers must likewise
// only read them, never mutate in place. Every caller in this codebase already
// follows that convention (masking always builds a new map -- see
// internal/analytics/publishers/log.go's maskHeaders).
type Payload struct {
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	// RequestBody/ResponseBody are captured payloads (collector.request_body /
	// collector.response_body). Unlike headers they can be large, and every byte
	// left in Envoy dynamic metadata is re-serialized on each later ext_proc message
	// and again in the access-log entry, so they are carried here whenever they fit
	// under the store's per-payload limit (see Store.MaxPayloadBytes).
	RequestBody  string
	ResponseBody string
}

// IsEmpty reports whether payload carries nothing worth storing -- e.g. header
// capture was not enabled for either direction on this request.
func (p Payload) IsEmpty() bool {
	return len(p.RequestHeaders) == 0 && len(p.ResponseHeaders) == 0 &&
		p.RequestBody == "" && p.ResponseBody == ""
}

// bodyBytes is what an entry is charged against its shard's byte budget. Headers
// are small and already bounded by the entry count, so only bodies are counted.
func (p Payload) bodyBytes() int64 {
	return int64(len(p.RequestBody) + len(p.ResponseBody))
}

// entry is one stored record. Fields are only ever read or written while the
// owning shard's mutex is held (see shard) -- in particular, Get copies out the
// fields it needs before releasing the lock, rather than handing back the *entry
// pointer, so a concurrent Put updating an existing entry in place can never race
// a reader.
type entry struct {
	key       string
	payload   Payload
	expiresAt time.Time
	// slot is this entry's index in its shard's ring, so Take can free the slot.
	slot int
}

// shard is one independently-locked partition of the store. entries is the
// lookup index; ring is a fixed-capacity FIFO of the same entries, used to evict
// the oldest write in O(1) once the shard is full -- capacity pressure is handled
// by bounded FIFO eviction, never by growing the map or blocking the writer.
type shard struct {
	mu      sync.Mutex
	entries map[string]*entry
	ring    []*entry
	next    int
	// bodyBytes is the payload bytes currently held; kept at or under maxBodyBytes
	// by evicting the oldest entries (see Put).
	bodyBytes    int64
	maxBodyBytes int64
}

// Store is a bounded, TTL-evicting, sharded key/value store from request id to
// Payload. The zero value is not usable -- construct with NewStore.
type Store struct {
	shards []*shard
	mask   uint64
	ttl    time.Duration
	seed   maphash.Seed
	// maxPayloadBytes is the largest single body the ext_proc side may hand to the
	// store instead of Envoy metadata (0 = bodies are never stored).
	maxPayloadBytes int
}

// NewStore builds a Store with capacity entries spread across numShards
// independently-locked shards (rounded up to the next power of two), each entry
// honored for ttl after it is written. Non-positive inputs fall back to a minimal
// usable value rather than panicking -- config.Validate is the fail-closed gate
// for a genuinely misconfigured deployment; this constructor stays defensive so a
// caller that skipped validation (a unit test, a future call site) still gets a
// working store instead of a divide-by-zero or an unbounded one.
func NewStore(capacity int, ttl time.Duration, numShards int) *Store {
	return NewStoreWithBodyLimits(capacity, ttl, numShards, 0, 0)
}

// NewStoreWithBodyLimits is NewStore that can also carry request/response bodies:
// at most maxPayloadBytes per body, and maxBodyBytes of bodies across the whole
// store (split evenly across shards). Either limit <= 0 disables body storage, and
// bodies then keep travelling through Envoy dynamic metadata as before.
func NewStoreWithBodyLimits(capacity int, ttl time.Duration, numShards, maxPayloadBytes int, maxBodyBytes int64) *Store {
	if capacity <= 0 {
		capacity = 1
	}
	if ttl <= 0 {
		ttl = time.Second
	}
	if numShards <= 0 {
		numShards = 1
	}
	numShards = nextPowerOfTwo(numShards)

	perShard := capacity / numShards
	if perShard < 1 {
		perShard = 1
	}

	perShardBodyBytes := maxBodyBytes / int64(numShards)
	if maxPayloadBytes <= 0 || perShardBodyBytes <= 0 {
		maxPayloadBytes, perShardBodyBytes = 0, 0
	} else if int64(maxPayloadBytes) > perShardBodyBytes {
		// A body larger than one shard's budget could never be held; cap it so the
		// ext_proc side keeps such bodies in metadata instead of losing them.
		maxPayloadBytes = int(perShardBodyBytes)
	}

	shards := make([]*shard, numShards)
	for i := range shards {
		shards[i] = &shard{
			entries:      make(map[string]*entry, perShard),
			ring:         make([]*entry, perShard),
			maxBodyBytes: perShardBodyBytes,
		}
	}

	return &Store{
		shards:          shards,
		mask:            uint64(numShards - 1),
		ttl:             ttl,
		seed:            maphash.MakeSeed(),
		maxPayloadBytes: maxPayloadBytes,
	}
}

// MaxPayloadBytes is the largest body the store accepts; the ext_proc side keeps a
// larger body (or every body, when this is 0) in Envoy dynamic metadata. Safe to
// call on a nil Store, which accepts no bodies.
func (s *Store) MaxPayloadBytes() int {
	if s == nil {
		return 0
	}
	return s.maxPayloadBytes
}

// NewStoreFromConfig builds a Store from the operator-facing
// [analytics.correlation] config (see config.CorrelationStoreConfig).
func NewStoreFromConfig(cfg config.CorrelationStoreConfig) *Store {
	return NewStoreWithBodyLimits(cfg.Capacity, cfg.TTL, cfg.Shards, cfg.MaxPayloadBytes, cfg.MaxBodyBytes)
}

// nextPowerOfTwo returns the smallest power of two >= n (n >= 1).
func nextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// shardFor selects the shard owning key by hashing it. maphash is used instead of
// a general-purpose hash package purely to avoid an extra dependency; the request
// ids being hashed are not attacker-influenced in a way that matters here (a
// skewed distribution across shards degrades to more lock contention, not a
// correctness issue), so maphash's per-process random seed is a bonus, not a
// requirement.
func (s *Store) shardFor(key string) *shard {
	h := maphash.String(s.seed, key)
	return s.shards[h&s.mask]
}

// Put stores payload under key, evicting the shard's oldest entry if it is full.
// A blank key or an empty payload is a silent no-op (nothing to correlate, or
// nothing worth correlating) -- see the CorrelationStoreWritesTotal metric at the
// call site in internal/kernel for why an empty key is counted there instead of
// here: only the caller knows *why* the key was empty (no x-request-id header vs.
// the collector disabled).
func (s *Store) Put(key string, payload Payload) {
	if key == "" || payload.IsEmpty() {
		return
	}

	sh := s.shardFor(key)
	now := time.Now()

	sh.mu.Lock()
	defer sh.mu.Unlock()

	if existing, ok := sh.entries[key]; ok {
		// Same key written twice (should not happen in practice -- one ext_proc
		// stream teardown per request -- but stay correct if it ever does):
		// update in place so the entry's existing ring slot stays valid.
		sh.bodyBytes += payload.bodyBytes() - existing.payload.bodyBytes()
		existing.payload = payload
		existing.expiresAt = now.Add(s.ttl)
		sh.enforceBodyBudget(existing)
		return
	}

	if occupant := sh.ring[sh.next]; occupant != nil {
		// The slot this write lands in already holds an older entry: evict it to
		// make room, regardless of its own remaining TTL. This is the
		// capacity-pressure path -- bounded, O(1), and scoped to this shard only.
		sh.remove(occupant)
		metrics.CorrelationStoreEvictionsTotal.Inc()
	}

	e := &entry{key: key, payload: payload, expiresAt: now.Add(s.ttl), slot: sh.next}
	sh.ring[sh.next] = e
	sh.entries[key] = e
	sh.bodyBytes += payload.bodyBytes()
	sh.next = (sh.next + 1) % len(sh.ring)
	sh.enforceBodyBudget(e)
}

// remove drops e from the shard's index, ring and byte accounting. Caller holds mu.
func (sh *shard) remove(e *entry) {
	if sh.entries[e.key] == e {
		delete(sh.entries, e.key)
	}
	if sh.ring[e.slot] == e {
		sh.ring[e.slot] = nil
	}
	sh.bodyBytes -= e.payload.bodyBytes()
}

// enforceBodyBudget evicts the shard's oldest body-carrying entries, other than
// keep, until its bodies fit in maxBodyBytes. Entries are visited oldest first
// starting at next (the ring's FIFO head), so this is bounded by the ring size.
// Without it, bodies that are never read back -- e.g. requests whose access-log
// entry is filtered by collector.ignore_path_prefixes -- would be pinned until
// the ring wraps, which for large bodies is far more memory than the entry count
// suggests. Caller holds mu.
func (sh *shard) enforceBodyBudget(keep *entry) {
	for i := 0; sh.bodyBytes > sh.maxBodyBytes && i < len(sh.ring); i++ {
		e := sh.ring[(sh.next+i)%len(sh.ring)]
		if e == nil || e == keep || e.payload.bodyBytes() == 0 {
			continue
		}
		sh.remove(e)
		metrics.CorrelationStoreEvictionsTotal.Inc()
	}
}

// Get looks up key and reports whether a live (non-expired) entry was found.
// Expired entries are not proactively swept -- they age out of the ring's FIFO
// order naturally via Put's capacity-pressure eviction, which keeps memory bounded
// regardless of read traffic, at the cost of Get on an expired-but-not-yet-evicted
// key doing one extra map lookup it then discards. Given the entries here are
// small (two string maps) and the ring already bounds their count, that trade is
// deliberate: a background sweep would add a goroutine and a shutdown path for a
// saving that does not matter at this scale.
func (s *Store) Get(key string) (Payload, bool) {
	return s.lookup(key, false)
}

// Take is Get that also removes the entry, releasing its slot and body bytes as
// soon as the access-log entry it was waiting for has been processed. The ALS
// handler reads each request's entry exactly once, so this keeps the store's
// footprint at the in-flight window instead of the full capacity.
func (s *Store) Take(key string) (Payload, bool) {
	return s.lookup(key, true)
}

func (s *Store) lookup(key string, take bool) (Payload, bool) {
	if key == "" {
		metrics.CorrelationStoreReadsTotal.WithLabelValues("miss").Inc()
		return Payload{}, false
	}

	sh := s.shardFor(key)

	sh.mu.Lock()
	e, ok := sh.entries[key]
	var payload Payload
	var live bool
	if ok {
		payload = e.payload
		live = time.Now().Before(e.expiresAt)
		if take {
			sh.remove(e)
		}
	}
	sh.mu.Unlock()

	if !ok || !live {
		metrics.CorrelationStoreReadsTotal.WithLabelValues("miss").Inc()
		return Payload{}, false
	}
	metrics.CorrelationStoreReadsTotal.WithLabelValues("hit").Inc()
	return payload, true
}
