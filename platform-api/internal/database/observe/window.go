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

// Package observe records the outcomes of the platform's own database operations, so
// database health can be judged from real traffic instead of from pings of its own. A
// connector returned by Wrap passes every operation through to the real driver
// unchanged and records whether it reached the database into a Window.
package observe

import (
	"context"
	"errors"
	"sync"
	"time"
)

// FailureKind tells what kind of connection failure was counted.
type FailureKind int

const (
	// FailureNone means no connection failure has been counted.
	FailureNone FailureKind = iota
	// FailureUnreachable is a connection failure other than a timeout.
	FailureUnreachable
	// FailureTimeout is a connection attempt or network operation that timed out.
	FailureTimeout
)

// healthPingKey is the context key that marks an operation as the health check's own
// ping.
type healthPingKey struct{}

// WithHealthPing returns a context that marks the operations run with it as the health
// check's own ping. Their outcomes are recorded like any other, but they update the
// window's last ping time instead of its last activity time, so health's own pings
// never make an idle platform look busy.
func WithHealthPing(ctx context.Context) context.Context {
	return context.WithValue(ctx, healthPingKey{}, true)
}

func isHealthPing(ctx context.Context) bool {
	ping, _ := ctx.Value(healthPingKey{}).(bool)
	return ping
}

// bucket holds the outcomes recorded in one second of the window.
type bucket struct {
	second   int64
	attempts int
	failures int
}

// Window holds the outcomes of recent database operations in one-second buckets, along
// with when the platform last used the database and when health last pinged it. It is
// safe for concurrent use; recording an outcome takes constant time.
type Window struct {
	now    func() time.Time
	origin time.Time

	mu           sync.Mutex
	buckets      []bucket
	lastActivity time.Time
	lastPing     time.Time
	lastKind     FailureKind
}

// Snapshot is the state of a window at one instant.
type Snapshot struct {
	// Attempts is the number of operations recorded within the window.
	Attempts int
	// Failures is how many of those operations were connection failures.
	Failures int
	// LastActivity is when the platform's own last database operation finished; zero
	// when there has been none.
	LastActivity time.Time
	// LastPing is when health's own last ping finished; zero when there has been none.
	LastPing time.Time
	// LastFailureKind is the kind of the most recent connection failure, at any time.
	LastFailureKind FailureKind
}

// NewWindow returns an empty window covering length, rounded up to whole seconds. now
// is the clock it records with; nil means time.Now.
func NewWindow(length time.Duration, now func() time.Time) *Window {
	if now == nil {
		now = time.Now
	}
	n := max(1, int((length+time.Second-1)/time.Second))
	return &Window{now: now, origin: now(), buckets: make([]bucket, n)}
}

// Record records the outcome of one database operation; err is nil when it succeeded.
// Only a connection failure, as decided by IsConnectionFailure, counts as a failure:
// any other error still means the database answered. An operation its caller canceled
// counts as activity but not as an attempt, since it says nothing about the database.
func (w *Window) Record(ctx context.Context, err error) {
	failed, kind := IsConnectionFailure(err)
	w.record(ctx, err, failed, kind)
}

// recordConnection records the outcome of opening a connection or pinging over one,
// where every error but the caller's own cancellation is a connection failure.
func (w *Window) recordConnection(ctx context.Context, err error) {
	failed := err != nil && !errors.Is(err, context.Canceled)
	kind := FailureNone
	if failed {
		kind = failureKind(err)
	}
	w.record(ctx, err, failed, kind)
}

func (w *Window) record(ctx context.Context, err error, failed bool, kind FailureKind) {
	now := w.now()
	w.mu.Lock()
	defer w.mu.Unlock()
	if isHealthPing(ctx) {
		w.lastPing = now
	} else {
		w.lastActivity = now
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	sec := w.second(now)
	b := &w.buckets[mod(sec, int64(len(w.buckets)))]
	if b.second != sec {
		*b = bucket{second: sec}
	}
	b.attempts++
	if failed {
		b.failures++
		w.lastKind = kind
	}
}

// Snapshot returns the window's state at now: the attempts and failures recorded in the
// last window length up to and including now's second, and the latest timestamps.
func (w *Window) Snapshot(now time.Time) Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := Snapshot{LastActivity: w.lastActivity, LastPing: w.lastPing, LastFailureKind: w.lastKind}
	sec := w.second(now)
	n := int64(len(w.buckets))
	for _, b := range w.buckets {
		if b.second <= sec && sec-b.second < n {
			s.Attempts += b.attempts
			s.Failures += b.failures
		}
	}
	return s
}

// second returns the whole second t falls in, counted from the window's creation.
// It floors, so instants before the origin fall in negative seconds.
func (w *Window) second(t time.Time) int64 {
	d := t.Sub(w.origin)
	sec := int64(d / time.Second)
	if d < 0 && d%time.Second != 0 {
		sec--
	}
	return sec
}

// mod returns a modulo n in [0, n).
func mod(a, n int64) int64 {
	return ((a % n) + n) % n
}
