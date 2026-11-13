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

// Package counters is the controller's side of the shared counter service
// (sdk/core/counters): the store the policy engines' adds are applied to, and the
// gRPC stream they report on.
package counters

import (
	"context"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

// DefaultRetention is how long a counter is kept after its last add when the
// configuration gives none.
const DefaultRetention = 40 * 24 * time.Hour

// appliedReportTTL is how long a ReportID is remembered so a resent report is not
// applied twice. Engines resend within seconds.
const appliedReportTTL = 10 * time.Minute

// Store applies reported adds and returns totals.
type Store interface {
	// Sync applies adds once per (source, reportID) and returns the totals of keys.
	Sync(ctx context.Context, source, reportID string, adds []counterproto.Add, keys []counterproto.Key) ([]counterproto.Total, error)
	Close() error
}

// Memory is a Store in this controller's memory: shared by its own policy engines and
// lost on restart.
type Memory struct {
	mu        sync.Mutex
	totals    map[counterproto.Key]memoryCounter
	applied   map[string]time.Time
	retention time.Duration
	lastSweep time.Time
	now       func() time.Time
}

type memoryCounter struct {
	total   float64
	touched time.Time
}

// NewMemory returns an empty in-memory Store.
func NewMemory(retention time.Duration) *Memory {
	if retention <= 0 {
		retention = DefaultRetention
	}
	return &Memory{
		totals:    map[counterproto.Key]memoryCounter{},
		applied:   map[string]time.Time{},
		retention: retention,
		now:       time.Now,
	}
}

// Sync applies adds once per (source, reportID) and returns the totals of keys.
func (m *Memory) Sync(_ context.Context, source, reportID string, adds []counterproto.Add, keys []counterproto.Key) ([]counterproto.Total, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweepLocked(now)
	if len(adds) > 0 {
		marker := source + "\x00" + reportID
		if _, done := m.applied[marker]; !done {
			m.applied[marker] = now
			for _, add := range adds {
				c := m.totals[add.Key]
				c.total += add.Delta
				c.touched = now
				m.totals[add.Key] = c
			}
		}
	}
	totals := make([]counterproto.Total, 0, len(keys))
	for _, key := range keys {
		totals = append(totals, counterproto.Total{Key: key, Total: m.totals[key].total})
	}
	return totals, nil
}

// Close does nothing.
func (m *Memory) Close() error { return nil }

// sweepLocked forgets old report markers and idle counters, at most once a minute.
func (m *Memory) sweepLocked(now time.Time) {
	if now.Sub(m.lastSweep) < time.Minute {
		return
	}
	m.lastSweep = now
	for marker, at := range m.applied {
		if now.Sub(at) > appliedReportTTL {
			delete(m.applied, marker)
		}
	}
	for key, c := range m.totals {
		if now.Sub(c.touched) > m.retention {
			delete(m.totals, key)
		}
	}
}
