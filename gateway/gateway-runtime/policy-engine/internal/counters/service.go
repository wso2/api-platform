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

// Package counters is the policy engine's shared counter service (sdk/core/counters).
// Adds and reads are in memory; Sync reports the adds to the gateway-controller about
// once a second and takes back the shared totals of the counters this engine uses.
package counters

import (
	"strconv"
	"sync"
	"time"

	sdkcounters "github.com/wso2/api-platform/sdk/core/counters"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

// DefaultIdleTimeout is how long a counter keeps being synced after this engine last
// used it, when no other is configured.
const DefaultIdleTimeout = 10 * time.Minute

// entry is what this engine keeps for one counter it uses.
type entry struct {
	// total is the shared total from the last sync; synced is false until one arrives.
	total  float64
	synced bool
	// pending holds adds not yet in a report.
	pending float64
	// expiresAt is pushed forward on every use; past it, with nothing left to report,
	// the counter is no longer synced and is forgotten.
	expiresAt time.Time
	// labels describe the counter; sent with its adds (sdk/core/counters.Describe).
	labels map[string]string
}

// Service implements sdk/core/counters.Service.
type Service struct {
	mu sync.Mutex
	// source names this engine to the controller for the life of the process.
	source      string
	idleTimeout time.Duration
	seq         uint64
	entries     map[sdkcounters.Key]*entry
	// inflight is the report sent and not yet acknowledged; resent as is until it is.
	inflight   map[sdkcounters.Key]float64
	inflightID string
	now        func() time.Time
	// used is closed when a policy first uses a counter: until then there is nothing to
	// sync and the engine does not connect to its controller.
	used     chan struct{}
	usedOnce sync.Once
}

// NewService returns an empty Service. source must be unique per engine process;
// idleTimeout is how long a counter keeps being synced after its last use
// (DefaultIdleTimeout when not positive).
func NewService(source string, idleTimeout time.Duration) *Service {
	if idleTimeout <= 0 {
		idleTimeout = DefaultIdleTimeout
	}
	return &Service{
		source:      source,
		idleTimeout: idleTimeout,
		entries:     map[sdkcounters.Key]*entry{},
		now:         time.Now,
		used:        make(chan struct{}),
	}
}

// Used is closed once a policy has used a counter.
func (s *Service) Used() <-chan struct{} {
	return s.used
}

// use returns the counter's entry, creating it, and pushes its expiry forward.
// Callers hold mu.
func (s *Service) use(key sdkcounters.Key) *entry {
	e := s.entries[key]
	if e == nil {
		e = &entry{}
		s.entries[key] = e
		s.usedOnce.Do(func() { close(s.used) })
	}
	e.expiresAt = s.now().Add(s.idleTimeout)
	return e
}

// Add adds delta to the counter; it is reported on the next sync.
func (s *Service) Add(key sdkcounters.Key, delta float64) {
	key = normalize(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.use(key).pending += delta
}

// Describe sets the labels sent with the counter's adds.
func (s *Service) Describe(key sdkcounters.Key, labels map[string]string) {
	key = normalize(key)
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.use(key).labels = copied
}

// Total returns the last shared total plus this engine's adds not yet in it. ok is
// false until a shared total has come back for the counter.
func (s *Service) Total(key sdkcounters.Key) (float64, bool) {
	key = normalize(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.use(key)
	return e.total + s.inflight[key] + e.pending, e.synced
}

// nextRequest builds the request for this sync: the report in flight, or a new one
// from the pending adds, and every counter still in use. Counters past their expiry
// with nothing left to report are forgotten.
func (s *Service) nextRequest() *counterproto.SyncRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.inflight == nil {
		report := map[sdkcounters.Key]float64{}
		for key, e := range s.entries {
			if e.pending != 0 {
				report[key], e.pending = e.pending, 0
			}
		}
		if len(report) > 0 {
			s.seq++
			s.inflight, s.inflightID = report, strconv.FormatUint(s.seq, 10)
		}
	}
	req := &counterproto.SyncRequest{Source: s.source}
	if s.inflight != nil {
		req.ReportID = s.inflightID
		for key, delta := range s.inflight {
			add := counterproto.Add{Key: wire(key), Delta: delta}
			if e := s.entries[key]; e != nil {
				add.Labels = e.labels
			}
			req.Adds = append(req.Adds, add)
		}
	}
	for key, e := range s.entries {
		if now.After(e.expiresAt) && e.pending == 0 && s.inflight[key] == 0 {
			delete(s.entries, key)
			continue
		}
		req.Keys = append(req.Keys, wire(key))
	}
	return req
}

// apply takes in a response: totals of the counters still in use, and an acknowledged
// report, now part of those totals.
func (s *Service) apply(_ *counterproto.SyncRequest, resp *counterproto.SyncResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !resp.OK {
		return // the store failed; the report stays in flight and is sent again
	}
	// Totals apply on every sync, with adds or without, so an engine that has nothing
	// to report still sees what other gateways spend.
	if resp.ReportID != "" && resp.ReportID == s.inflightID {
		s.inflight, s.inflightID = nil, ""
	}
	for _, t := range resp.Totals {
		if e := s.entries[local(t.Key)]; e != nil {
			e.total, e.synced = t.Total, true
		}
	}
}

// normalize makes keys that name the same window equal.
func normalize(key sdkcounters.Key) sdkcounters.Key {
	if !key.Window.IsZero() {
		key.Window = time.Unix(key.Window.Unix(), 0).UTC()
	}
	return key
}

func wire(key sdkcounters.Key) counterproto.Key {
	return counterproto.KeyFor(key.Namespace, key.ID, key.Window)
}

func local(key counterproto.Key) sdkcounters.Key {
	return sdkcounters.Key{Namespace: key.Namespace, ID: key.ID, Window: key.WindowTime()}
}
