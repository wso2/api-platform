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

// watchFor is how long a counter keeps being synced after this engine last used it.
const watchFor = 10 * time.Minute

// Service implements sdk/core/counters.Service.
type Service struct {
	mu sync.Mutex
	// source names this engine to the controller for the life of the process.
	source string
	seq    uint64
	// pending holds adds not yet in a report.
	pending map[sdkcounters.Key]float64
	// inflight is the report sent and not yet acknowledged; resent as is until it is.
	inflight   map[sdkcounters.Key]float64
	inflightID string
	totals     map[sdkcounters.Key]float64
	known      map[sdkcounters.Key]bool
	lastUsed   map[sdkcounters.Key]time.Time
	now        func() time.Time
}

// NewService returns an empty Service. source must be unique per engine process.
func NewService(source string) *Service {
	return &Service{
		source:   source,
		pending:  map[sdkcounters.Key]float64{},
		totals:   map[sdkcounters.Key]float64{},
		known:    map[sdkcounters.Key]bool{},
		lastUsed: map[sdkcounters.Key]time.Time{},
		now:      time.Now,
	}
}

// Add adds delta to the counter; it is reported on the next sync.
func (s *Service) Add(key sdkcounters.Key, delta float64) {
	key = normalize(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[key] += delta
	s.lastUsed[key] = s.now()
}

// Total returns the last shared total plus this engine's adds not yet in it. ok is
// false until a shared total has come back for the counter.
func (s *Service) Total(key sdkcounters.Key) (float64, bool) {
	key = normalize(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUsed[key] = s.now()
	return s.totals[key] + s.inflight[key] + s.pending[key], s.known[key]
}

// nextRequest builds the request for this sync: the report in flight, or a new one
// from the pending adds, and every counter used recently.
func (s *Service) nextRequest() *counterproto.SyncRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.inflight == nil && len(s.pending) > 0 {
		s.seq++
		s.inflight, s.pending = s.pending, map[sdkcounters.Key]float64{}
		s.inflightID = strconv.FormatUint(s.seq, 10)
	}
	req := &counterproto.SyncRequest{Source: s.source}
	if s.inflight != nil {
		req.ReportID = s.inflightID
		for key, delta := range s.inflight {
			req.Adds = append(req.Adds, counterproto.Add{Key: wire(key), Delta: delta})
		}
	}
	for key, used := range s.lastUsed {
		if now.Sub(used) > watchFor && s.inflight[key] == 0 && s.pending[key] == 0 {
			delete(s.lastUsed, key)
			delete(s.totals, key)
			delete(s.known, key)
			continue
		}
		req.Keys = append(req.Keys, wire(key))
	}
	return req
}

// apply takes in a response: an acknowledged report is now part of the totals.
func (s *Service) apply(req *counterproto.SyncRequest, resp *counterproto.SyncResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if resp.ReportID == "" {
		return // not applied; the report stays in flight and is sent again
	}
	if resp.ReportID == s.inflightID {
		s.inflight, s.inflightID = nil, ""
	}
	for _, t := range resp.Totals {
		key := local(t.Key)
		s.totals[key] = t.Total
		s.known[key] = true
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
