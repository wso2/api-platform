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
// headers and bodies from the ext_proc handler to the ALS (access-log) handler,
// keyed by a token unique to each ext_proc stream (a client-supplied x-request-id
// can repeat across concurrent requests, so it is not used as the key).
//
// Before this store existed, captured request/response headers made a full round
// trip through Envoy for no reason other than correlating them back to the request
// they belonged to: the analytics system policy JSON-encoded them into ext_proc
// dynamic metadata, Envoy echoed the whole filter_metadata struct back in the
// HTTPAccessLogEntry, and the ALS handler JSON-decoded them again (twice). Since
// the ext_proc handler and the ALS handler are two goroutines in the very same
// process, that round trip bought nothing but CPU. This store lets the ext_proc
// handler stash the already-typed values directly, and the ALS handler fetch them
// back by that token instead of decoding Envoy's echo.
//
// Ordering: a field is left out of Envoy metadata only after Merge has accepted
// it, and Merge runs before the ext_proc response that would otherwise have
// carried the field is sent. Envoy cannot log the request before it has that
// response, so the access-log entry can never arrive ahead of the data. When the
// store cannot accept a field (no free slot, a body over the per-body limit, or
// headers and bodies over the byte budget), Merge refuses it and the caller keeps
// it in metadata, the pre-store path. When a newer value of a field that is already
// stored is refused, the caller clears the stored copy (Clear), so the newer value
// in metadata is the one logged.
//
// Retention: an unread entry is never dropped while it is fresh. It becomes
// reclaimable, and only when its slot or bytes are needed for another request,
// once (a) the ext_proc side saw the response finish (Complete) more than the TTL
// ago, or (b) the response was never seen to finish and the entry is older than
// max_entry_age. A completed entry follows the TTL only, however old it is. The
// ext_proc stream can end before the response does (when body processing is
// skipped, Envoy closes it after the response headers while the body is still
// streaming), so stream end alone never starts the TTL. Fields of a reclaimed
// entry are gone: they were already left out of metadata. That happens only under
// store pressure combined with an access-log entry that is late by more than the
// TTL (a stalled ALS consumer) or a response that streams for longer than
// max_entry_age. The evictions metric counts each case by reason.
//
// Scope: the store only works when the ext_proc stream and the ALS stream for a
// request reach the same policy-engine process. The policy engine enables it only
// when that is guaranteed (see config.CorrelationStoreConfig.Mode). Paths in
// collector.ignore_path_prefixes, whose access-log entry Envoy never sends, are not
// stored at all (see IgnoresPath).
//
// A lookup miss is an expected, non-error outcome: an HTTPAccessLogEntry is also
// produced for requests that never had an ext_proc stream (no-route 404s,
// pre-filter rejections), and for requests whose fields all stayed in metadata.
// Callers must treat a miss as "use whatever the access-log entry itself carries",
// never as a reason to drop the log line.
package correlation

import (
	"hash/maphash"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// Payload is the in-process analytics data captured by the ext_proc handler for
// one request and looked up by the ALS handler when that request's access-log
// entry arrives. Only the fields that used to make the Envoy round trip described
// in the package doc are carried here; every other analytics field (API identity,
// auth context, subscription, AI/MCP metadata...) still travels through Envoy
// dynamic metadata.
//
// Ownership: Merge takes ownership of the maps passed to it -- the caller must
// treat them as immutable afterward. Take hands back the same map values (no
// defensive copy, to avoid an allocation on every access-log entry); callers must
// likewise only read them, never mutate in place.
type Payload struct {
	// RequestHeaders/ResponseHeaders hold captured headers in the shape the
	// analytics data carried them: map[string]string, or map[string][]string when
	// an analytics header filter kept repeated values separate. Consumers decode
	// either shape with internal/analytics/headers. nil means not captured; an
	// empty map is a deliberate "no headers" (for example, every header filtered
	// out in a later phase) and replaces any earlier value.
	RequestHeaders  any
	ResponseHeaders any
	// RequestBody/ResponseBody are captured payloads (collector.request_body /
	// collector.response_body). Every byte left in Envoy dynamic metadata is
	// re-serialized on each later ext_proc message and again in the access-log
	// entry, so they are carried here whenever they fit under the store's per-body
	// limit and byte budget.
	RequestBody  string
	ResponseBody string
}

// IsEmpty reports whether payload carries nothing to record.
func (p Payload) IsEmpty() bool {
	return p.RequestHeaders == nil && p.ResponseHeaders == nil &&
		p.RequestBody == "" && p.ResponseBody == ""
}

// size is what an entry is charged against its shard's byte budget: the captured
// header names and values and both bodies.
func (p Payload) size() int64 {
	return headerBytes(p.RequestHeaders) + headerBytes(p.ResponseHeaders) +
		int64(len(p.RequestBody)+len(p.ResponseBody))
}

// headerBytes is the total length of a captured header map's names and values.
func headerBytes(h any) int64 {
	var n int
	switch m := h.(type) {
	case map[string]string:
		for k, v := range m {
			n += len(k) + len(v)
		}
	case map[string][]string:
		for k, vs := range m {
			n += len(k)
			for _, v := range vs {
				n += len(v)
			}
		}
	}
	return int64(n)
}

// mergeInto overlays p's set fields onto dst, field by field, so the request and
// response phases of one request can each contribute their own. A set header map
// replaces the earlier one even when empty, so a later phase can narrow or clear
// headers an earlier phase captured.
func (p Payload) mergeInto(dst *Payload) {
	if p.RequestHeaders != nil {
		dst.RequestHeaders = p.RequestHeaders
	}
	if p.ResponseHeaders != nil {
		dst.ResponseHeaders = p.ResponseHeaders
	}
	if p.RequestBody != "" {
		dst.RequestBody = p.RequestBody
	}
	if p.ResponseBody != "" {
		dst.ResponseBody = p.ResponseBody
	}
}

// Field names one captured field of a Payload, for Clear.
type Field int

const (
	FieldRequestHeaders Field = iota
	FieldResponseHeaders
	FieldRequestBody
	FieldResponseBody
)

// clear unsets field f.
func (p *Payload) clear(f Field) {
	switch f {
	case FieldRequestHeaders:
		p.RequestHeaders = nil
	case FieldResponseHeaders:
		p.ResponseHeaders = nil
	case FieldRequestBody:
		p.RequestBody = ""
	case FieldResponseBody:
		p.ResponseBody = ""
	}
}

// entry is one stored record. Fields are only ever read or written while the
// owning shard's mutex is held (see shard).
type entry struct {
	key     string
	payload Payload
	// size is payload.size(), kept so the shard's byte accounting never recomputes it.
	size int64
	// createdAt is when the entry was first written. An entry that never completes
	// becomes reclaimable once it is older than the store's max entry age.
	createdAt time.Time
	// completedAt is when the ext_proc side saw the response finish (Complete).
	// Zero until then; the TTL only counts from this point.
	completedAt time.Time
	// slot is this entry's index in its shard's ring.
	slot int
}

// reclaimAt is when e, still unread, may first be dropped to make room: the TTL
// after its response finished, or, for a response never seen to finish, the max
// entry age after it was created. A completed entry follows the TTL only.
func (e *entry) reclaimAt(s *Store) time.Time {
	if !e.completedAt.IsZero() {
		return e.completedAt.Add(s.ttl)
	}
	return e.createdAt.Add(s.maxIncompleteAge)
}

// shard is one independently-locked partition of the store. entries is the
// lookup index; ring holds the same entries in a fixed number of slots, so the
// shard's entry count is bounded without ever growing the map.
type shard struct {
	mu      sync.Mutex
	entries map[string]*entry
	ring    []*entry
	// free lists the empty ring slots, so finding one never scans the ring.
	free []int
	// next is where the next reclaim scan starts.
	next int
	// bytes is the header and body bytes currently held; never above maxBytes.
	bytes    int64
	maxBytes int64
	// noSlotBefore / noBytesBefore: a reclaim scan that found nothing to drop
	// records when the earliest candidate becomes reclaimable, so writes that
	// cannot fit fail immediately until then instead of rescanning the full shard
	// each time. Complete resets them, since completing an entry can move its
	// reclaim time earlier.
	noSlotBefore  time.Time
	noBytesBefore time.Time
}

// Store is a bounded, sharded key/value store from correlation token to Payload. The
// zero value is not usable -- construct with NewStore or NewStoreWithBodyLimits.
type Store struct {
	shards []*shard
	mask   uint64
	// ttl is how long a completed entry waits for its access-log entry before its
	// slot may be reclaimed.
	ttl  time.Duration
	seed maphash.Seed
	// maxIncompleteAge is how long an entry whose response was never seen to finish
	// may hold its slot: max(collector.correlation_store.max_entry_age, ttl).
	maxIncompleteAge time.Duration
	// maxPayloadBytes is the largest single body the store accepts (0 = bodies are
	// never stored and always stay in Envoy metadata).
	maxPayloadBytes int
	// ignoredPathPrefixes are the collector.ignore_path_prefixes; requests on these
	// paths never get an access-log entry, so their fields are not stored.
	ignoredPathPrefixes []string

	// Metric children resolved once, so the hot path does no label lookups.
	storedTotal, rejectedFullTotal, rejectedBudgetTotal metrics.Counter
	hitTotal, missTotal                                 metrics.Counter
	evictedTTLTotal, evictedMaxAgeTotal                 metrics.Counter
}

// defaultMaxEntryAge is the max entry age for a store built without configuration
// (config.CorrelationStoreConfig.MaxEntryAge's default).
const defaultMaxEntryAge = 5 * time.Minute

// NewStore builds a Store with capacity entries spread across numShards
// independently-locked shards (rounded up to the next power of two), holding
// headers only (bodies stay in Envoy metadata). Non-positive inputs fall back to a
// minimal usable value rather than panicking -- config.Validate is the
// fail-closed gate for a genuinely misconfigured deployment; this constructor
// stays defensive so a caller that skipped validation (a unit test, a future call
// site) still gets a working, bounded store.
func NewStore(capacity int, ttl time.Duration, numShards int) *Store {
	return NewStoreWithBodyLimits(capacity, ttl, numShards, 0, 0)
}

// NewStoreWithBodyLimits is NewStore that can also carry request/response bodies:
// at most maxPayloadBytes per body, and maxBodyBytes of captured headers and bodies
// across the whole store (split evenly across shards). Either limit <= 0 disables
// body storage; headers then share config.DefaultCorrelationStoreHeaderBytes.
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
	if numShards > config.MaxCorrelationStoreShards {
		numShards = config.MaxCorrelationStoreShards
	}
	numShards = nextPowerOfTwo(numShards)
	// Every shard holds at least one slot, so more shards than capacity would
	// silently raise the real capacity to one slot per shard. Use the largest power
	// of two that keeps the total within capacity instead.
	for numShards > 1 && numShards > capacity {
		numShards >>= 1
	}

	perShard := capacity / numShards
	if perShard < 1 {
		perShard = 1
	}

	perShardBytes := maxBodyBytes / int64(numShards)
	if maxPayloadBytes <= 0 || perShardBytes <= 0 {
		maxPayloadBytes = 0
		perShardBytes = config.DefaultCorrelationStoreHeaderBytes / int64(numShards)
	} else if int64(maxPayloadBytes) > perShardBytes {
		// A body larger than one shard's budget could never be held; cap the limit so
		// such bodies are refused up front and stay in metadata.
		maxPayloadBytes = int(perShardBytes)
	}

	shards := make([]*shard, numShards)
	for i := range shards {
		free := make([]int, perShard)
		for j := range free {
			free[j] = perShard - 1 - j // popped from the end: slot 0 first
		}
		shards[i] = &shard{
			entries:  make(map[string]*entry, perShard),
			ring:     make([]*entry, perShard),
			free:     free,
			maxBytes: perShardBytes,
		}
	}

	s := &Store{
		shards:              shards,
		mask:                uint64(numShards - 1),
		ttl:                 ttl,
		seed:                maphash.MakeSeed(),
		maxPayloadBytes:     maxPayloadBytes,
		storedTotal:         metrics.CorrelationStoreWritesTotal.WithLabelValues("stored"),
		rejectedFullTotal:   metrics.CorrelationStoreWritesTotal.WithLabelValues("rejected_full"),
		rejectedBudgetTotal: metrics.CorrelationStoreWritesTotal.WithLabelValues("rejected_budget"),
		hitTotal:            metrics.CorrelationStoreReadsTotal.WithLabelValues("hit"),
		missTotal:           metrics.CorrelationStoreReadsTotal.WithLabelValues("miss"),
		evictedTTLTotal:     metrics.CorrelationStoreEvictionsTotal.WithLabelValues("ttl"),
		evictedMaxAgeTotal:  metrics.CorrelationStoreEvictionsTotal.WithLabelValues("max_age"),
	}
	s.setMaxEntryAge(defaultMaxEntryAge)
	return s
}

// setMaxEntryAge sets how long an entry whose response was never seen to finish
// may hold its slot; never below the TTL.
func (s *Store) setMaxEntryAge(age time.Duration) {
	if age < s.ttl {
		age = s.ttl
	}
	s.maxIncompleteAge = age
}

// MaxPayloadBytes is the largest body the store accepts. Safe to call on a nil
// Store, which accepts no bodies.
func (s *Store) MaxPayloadBytes() int {
	if s == nil {
		return 0
	}
	return s.maxPayloadBytes
}

// NewStoreFromConfig builds a Store from the operator-facing [collector] config:
// the [collector.correlation_store] tuning (see config.CorrelationStoreConfig) and
// collector.ignore_path_prefixes.
func NewStoreFromConfig(cfg config.CollectorConfig) *Store {
	c := cfg.CorrelationStore
	s := NewStoreWithBodyLimits(c.Capacity, c.TTL, c.Shards, c.MaxPayloadBytes, c.MaxBodyBytes)
	if c.MaxEntryAge > 0 {
		s.setMaxEntryAge(c.MaxEntryAge)
	}
	for _, prefix := range cfg.IgnorePathPrefixes {
		if prefix = strings.TrimSpace(prefix); prefix != "" {
			s.ignoredPathPrefixes = append(s.ignoredPathPrefixes, prefix)
		}
	}
	return s
}

// IgnoresPath reports whether path is under one of collector.ignore_path_prefixes.
// Envoy never sends an access-log entry for those requests, so nothing would ever
// take their entry; the caller keeps such fields in Envoy metadata instead (where
// they are simply dropped with the unsent entry). Safe to call on a nil Store.
func (s *Store) IgnoresPath(path string) bool {
	if s == nil {
		return false
	}
	for _, prefix := range s.ignoredPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// nextPowerOfTwo returns the smallest power of two >= n (n >= 1), capped at
// config.MaxCorrelationStoreShards so the loop always ends.
func nextPowerOfTwo(n int) int {
	p := 1
	for p < n && p < config.MaxCorrelationStoreShards {
		p <<= 1
	}
	return p
}

// shardFor selects the shard owning key by hashing it. A skewed distribution
// across shards only degrades to more lock contention, not a correctness issue.
func (s *Store) shardFor(key string) *shard {
	h := maphash.String(s.seed, key)
	return s.shards[h&s.mask]
}

// Merge records p's set fields under key, creating the entry if needed, and
// reports whether it did. It refuses -- storing nothing from p -- when a body is
// over the per-body limit or body storage is disabled, when the fields do not fit
// the shard's byte budget, or when a new entry is needed and the shard has no free
// or reclaimable slot. On false the caller must keep those fields in Envoy
// metadata.
//
// Merge never evicts an entry the ALS handler has not read, other than a
// reclaimable one (see the package doc).
func (s *Store) Merge(key string, p Payload) bool {
	return s.merge(key, p, true)
}

// Update is Merge for a key whose entry must already exist: it never creates one.
// The ext_proc side uses it once a stream's entry has been created, so a phase that
// runs after the ALS handler already took the entry (for example after an Envoy
// message timeout) cannot leave behind an entry nobody will read. On false the
// caller keeps the fields in Envoy metadata.
func (s *Store) Update(key string, p Payload) bool {
	return s.merge(key, p, false)
}

func (s *Store) merge(key string, p Payload, create bool) bool {
	if key == "" || p.IsEmpty() {
		return false
	}
	if p.RequestBody != "" || p.ResponseBody != "" {
		if s.maxPayloadBytes == 0 {
			// Body storage is disabled (max_payload_bytes or max_body_bytes is 0):
			// bodies stay in metadata by configuration, which is not a rejection.
			return false
		}
		if len(p.RequestBody) > s.maxPayloadBytes || len(p.ResponseBody) > s.maxPayloadBytes {
			s.rejectedBudgetTotal.Inc()
			return false
		}
	}

	sh := s.shardFor(key)
	now := time.Now()

	sh.mu.Lock()
	defer sh.mu.Unlock()

	e := sh.entries[key]
	if e == nil && !create {
		return false
	}
	var updated Payload
	var oldSize int64
	if e != nil {
		updated, oldSize = e.payload, e.size
	}
	p.mergeInto(&updated)
	newSize := updated.size()
	if delta := newSize - oldSize; delta > 0 && !sh.makeRoom(s, delta, e, now) {
		s.rejectedBudgetTotal.Inc()
		return false
	}

	if e == nil {
		slot, ok := sh.freeSlot(s, now)
		if !ok {
			s.rejectedFullTotal.Inc()
			return false
		}
		e = &entry{key: key, slot: slot, createdAt: now}
		sh.ring[slot] = e
		sh.entries[key] = e
	}
	e.payload = updated
	sh.bytes += newSize - e.size
	e.size = newSize
	s.storedTotal.Inc()
	return true
}

// Clear unsets one field of key's entry, if the entry exists. The ext_proc side
// calls it when a newer value of a field it already stored is refused, so the
// stored, older copy is not logged in place of the newer one left in metadata.
func (s *Store) Clear(key string, f Field) {
	if key == "" {
		return
	}
	sh := s.shardFor(key)
	sh.mu.Lock()
	if e, ok := sh.entries[key]; ok {
		e.payload.clear(f)
		size := e.payload.size()
		sh.bytes += size - e.size
		e.size = size
	}
	sh.mu.Unlock()
}

// Complete marks key's response as finished, as seen by the ext_proc side. From
// then on its entry, if still unread after the TTL, may be reclaimed to make room.
// Callers must only complete once the response has actually ended, not merely
// when the ext_proc stream closes (see the package doc). A key with no entry is a
// no-op.
func (s *Store) Complete(key string) {
	if key == "" {
		return
	}
	sh := s.shardFor(key)
	sh.mu.Lock()
	if e, ok := sh.entries[key]; ok && e.completedAt.IsZero() {
		e.completedAt = time.Now()
		// Completion can move this entry's reclaim time earlier.
		sh.noSlotBefore, sh.noBytesBefore = time.Time{}, time.Time{}
	}
	sh.mu.Unlock()
}

// evict drops a reclaimable entry and counts it by reason. Caller holds mu.
func (sh *shard) evict(s *Store, e *entry) {
	if e.completedAt.IsZero() {
		s.evictedMaxAgeTotal.Inc()
	} else {
		s.evictedTTLTotal.Inc()
	}
	sh.remove(e)
}

// freeSlot returns an empty ring slot, reclaiming one reclaimable entry if that is
// the only way to get one. A scan that finds nothing to reclaim records when the
// earliest entry becomes reclaimable, and later calls fail immediately until then.
// Caller holds mu.
func (sh *shard) freeSlot(s *Store, now time.Time) (int, bool) {
	if n := len(sh.free); n > 0 {
		slot := sh.free[n-1]
		sh.free = sh.free[:n-1]
		return slot, true
	}
	if now.Before(sh.noSlotBefore) {
		return 0, false
	}
	var earliest time.Time
	for i := 0; i < len(sh.ring); i++ {
		idx := (sh.next + i) % len(sh.ring)
		e := sh.ring[idx]
		at := e.reclaimAt(s)
		if !now.Before(at) {
			sh.evict(s, e)
			sh.next = (idx + 1) % len(sh.ring)
			slot := sh.free[len(sh.free)-1]
			sh.free = sh.free[:len(sh.free)-1]
			return slot, true
		}
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
	}
	sh.noSlotBefore = earliest
	return 0, false
}

// makeRoom ensures need more bytes fit in the shard's budget, reclaiming
// reclaimable entries that hold bytes (never keep) if necessary. It changes nothing
// and returns false when the room cannot be made; it then records when the earliest
// other candidate becomes reclaimable, and later calls fail immediately until then.
// Caller holds mu.
func (sh *shard) makeRoom(s *Store, need int64, keep *entry, now time.Time) bool {
	if sh.bytes+need <= sh.maxBytes {
		return true
	}
	if now.Before(sh.noBytesBefore) {
		return false
	}
	var free int64
	var victims []*entry
	var earliest time.Time
	for i := 0; i < len(sh.ring) && sh.bytes-free+need > sh.maxBytes; i++ {
		e := sh.ring[(sh.next+i)%len(sh.ring)]
		if e == nil || e == keep || e.size == 0 {
			continue
		}
		if at := e.reclaimAt(s); now.Before(at) {
			if earliest.IsZero() || at.Before(earliest) {
				earliest = at
			}
			continue
		}
		victims = append(victims, e)
		free += e.size
	}
	if sh.bytes-free+need > sh.maxBytes {
		if len(victims) == 0 {
			sh.noBytesBefore = earliest
		}
		return false
	}
	for _, e := range victims {
		sh.evict(s, e)
	}
	return true
}

// remove drops e from the shard's index, ring and byte accounting, and returns its
// slot to the free list. Caller holds mu.
func (sh *shard) remove(e *entry) {
	if sh.entries[e.key] == e {
		delete(sh.entries, e.key)
	}
	if sh.ring[e.slot] == e {
		sh.ring[e.slot] = nil
		sh.free = append(sh.free, e.slot)
	}
	sh.bytes -= e.size
}

// Take looks up key and removes its entry, releasing its slot and bytes as soon as
// the access-log entry it was waiting for has been processed. The ALS handler
// reads each request's entry exactly once, so this keeps the store's footprint at
// the in-flight window instead of the full capacity. There is deliberately no
// non-consuming read of the payload: an entry that is read but left in place would
// keep its slot until reclaimed.
func (s *Store) Take(key string) (Payload, bool) {
	if key == "" {
		s.missTotal.Inc()
		return Payload{}, false
	}

	sh := s.shardFor(key)

	sh.mu.Lock()
	e, ok := sh.entries[key]
	var payload Payload
	if ok {
		payload = e.payload
		sh.remove(e)
	}
	sh.mu.Unlock()

	if !ok {
		s.missTotal.Inc()
		return Payload{}, false
	}
	s.hitTotal.Inc()
	return payload, true
}

// Discard removes key's entry, if any, without reading it and without touching the
// read metrics. For an entry the ALS handler can never read.
func (s *Store) Discard(key string) {
	if key == "" {
		return
	}
	sh := s.shardFor(key)
	sh.mu.Lock()
	if e, ok := sh.entries[key]; ok {
		sh.remove(e)
	}
	sh.mu.Unlock()
}

// Has reports whether key currently has an entry, without reading or removing its
// payload and without touching the read metrics. For diagnostics and tests.
func (s *Store) Has(key string) bool {
	if key == "" {
		return false
	}
	sh := s.shardFor(key)
	sh.mu.Lock()
	_, ok := sh.entries[key]
	sh.mu.Unlock()
	return ok
}
