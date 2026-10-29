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

// Package observe records the outcomes of database operations so that database health
// can be judged from real traffic. Wrap places an observing layer around a
// database/sql driver connector; every connect, ping, prepare, exec, query, begin,
// commit, rollback and session reset then lands in a Window, which keeps a sliding
// count of attempts and connection-class failures together with the time of the last
// application activity.
package observe

import (
	"context"
	"sync"
	"time"
)

// DefaultWindowLength is the window length used when a component does not supply its
// own window.
const DefaultWindowLength = 30 * time.Second

// bucketWidth is the time span counted by one bucket of a Window.
const bucketWidth = time.Second

// FailureKind describes a counted connection failure.
type FailureKind int

const (
	// NoFailure means no connection failure has been counted.
	NoFailure FailureKind = iota
	// Unreachable is a failure to reach or keep a connection to the database.
	Unreachable
	// Timeout is a connection failure caused by a deadline or a network timeout.
	Timeout
)

// healthPingKey marks a context whose operations belong to the health check's own ping.
type healthPingKey struct{}

// WithHealthPing marks ctx as belonging to the health check's own ping. Operations run
// under it are counted in the window and update the last ping time, but are not
// application activity.
func WithHealthPing(ctx context.Context) context.Context {
	return context.WithValue(ctx, healthPingKey{}, true)
}

func isHealthPing(ctx context.Context) bool {
	tagged, _ := ctx.Value(healthPingKey{}).(bool)
	return tagged
}

type bucket struct {
	second   int64
	attempts int
	failures int
}

// Snapshot is the state of a Window at one point in time.
type Snapshot struct {
	// Attempts is the number of operations recorded within the window.
	Attempts int
	// Failures is the number of those operations that failed with a connection failure.
	Failures int
	// LastActivity is when the last application operation was recorded. Zero if none.
	LastActivity time.Time
	// LastPing is when the last health ping operation was recorded. Zero if none.
	LastPing time.Time
	// LastFailure is the kind of the most recent counted failure, at any time.
	LastFailure FailureKind
}

// Window counts operation outcomes over a sliding time window made of one-second
// buckets. It is safe for concurrent use, and a nil Window ignores every call.
type Window struct {
	now func() time.Time

	mu           sync.Mutex
	buckets      []bucket
	lastActivity time.Time
	lastPing     time.Time
	lastFailure  FailureKind
}

// NewWindow returns an empty window that covers length, rounded up to whole seconds.
// now supplies the current time; nil means time.Now.
func NewWindow(length time.Duration, now func() time.Time) *Window {
	n := int((length + bucketWidth - 1) / bucketWidth)
	if n < 1 {
		n = 1
	}
	if now == nil {
		now = time.Now
	}
	return &Window{now: now, buckets: make([]bucket, n)}
}

// Record counts one operation whose outcome is err. Only connection failures count as
// failures; any other error, such as a constraint violation, is a successful attempt.
func (w *Window) Record(ctx context.Context, err error) {
	failed, kind := IsConnectionFailure(err)
	w.add(ctx, failed, kind)
}

// recordConnect counts the outcome of a connect or ping, where every error other than
// the caller's own cancellation counts as a connection failure.
func (w *Window) recordConnect(ctx context.Context, err error) {
	failed, kind := connectFailure(err)
	w.add(ctx, failed, kind)
}

func (w *Window) add(ctx context.Context, failed bool, kind FailureKind) {
	if w == nil {
		return
	}
	now := w.now()
	w.mu.Lock()
	defer w.mu.Unlock()
	b := w.bucketAt(now)
	b.attempts++
	if failed {
		b.failures++
		w.lastFailure = kind
	}
	if isHealthPing(ctx) {
		w.lastPing = now
	} else {
		w.lastActivity = now
	}
}

// bucketAt returns the bucket for the second containing t, clearing it if it last
// counted an older second. The caller must hold w.mu.
func (w *Window) bucketAt(t time.Time) *bucket {
	sec := t.Unix()
	b := &w.buckets[int(uint64(sec)%uint64(len(w.buckets)))]
	if b.second != sec {
		*b = bucket{second: sec}
	}
	return b
}

// Snapshot returns the counts recorded within the window ending at now, and the
// activity, ping and failure state.
func (w *Window) Snapshot(now time.Time) Snapshot {
	if w == nil {
		return Snapshot{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	s := Snapshot{
		LastActivity: w.lastActivity,
		LastPing:     w.lastPing,
		LastFailure:  w.lastFailure,
	}
	sec := now.Unix()
	n := int64(len(w.buckets))
	for _, b := range w.buckets {
		if age := sec - b.second; age >= 0 && age < n {
			s.Attempts += b.attempts
			s.Failures += b.failures
		}
	}
	return s
}
