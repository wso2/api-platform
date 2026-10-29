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

package observe

import (
	"context"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock is a manually advanced clock, safe for concurrent use.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

var errRefused = errors.New("connection refused")

func TestWindowCountsAttemptsAndConnectionFailures(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(30*time.Second, clk.Now)
	ctx := context.Background()

	w.Record(ctx, nil)
	w.Record(ctx, driver.ErrBadConn)
	w.Record(ctx, errors.New("duplicate key value violates unique constraint"))

	s := w.Snapshot(clk.Now())
	assert.Equal(t, 3, s.Attempts)
	assert.Equal(t, 1, s.Failures)
	assert.Equal(t, FailureUnreachable, s.LastFailureKind)
	assert.Equal(t, clk.Now(), s.LastActivity)
	assert.True(t, s.LastPing.IsZero())
}

func TestWindowCanceledOperationIsActivityButNoAttempt(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(30*time.Second, clk.Now)

	w.Record(context.Background(), context.Canceled)
	w.recordConnection(context.Background(), context.Canceled)

	s := w.Snapshot(clk.Now())
	assert.Zero(t, s.Attempts)
	assert.Zero(t, s.Failures)
	assert.Equal(t, clk.Now(), s.LastActivity)
}

func TestWindowConnectionOutcomesAlwaysCount(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(30*time.Second, clk.Now)
	ctx := context.Background()

	w.recordConnection(ctx, errors.New("password authentication failed"))
	s := w.Snapshot(clk.Now())
	assert.Equal(t, 1, s.Failures)
	assert.Equal(t, FailureUnreachable, s.LastFailureKind)

	w.recordConnection(ctx, context.DeadlineExceeded)
	s = w.Snapshot(clk.Now())
	assert.Equal(t, 2, s.Failures)
	assert.Equal(t, FailureTimeout, s.LastFailureKind)

	w.recordConnection(ctx, nil)
	s = w.Snapshot(clk.Now())
	assert.Equal(t, 3, s.Attempts)
	assert.Equal(t, 2, s.Failures)
	assert.Equal(t, FailureTimeout, s.LastFailureKind, "a success does not clear the last failure kind")
}

func TestWindowHealthPingUpdatesLastPingOnly(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(30*time.Second, clk.Now)

	w.recordConnection(WithHealthPing(context.Background()), errRefused)

	s := w.Snapshot(clk.Now())
	assert.Equal(t, 1, s.Attempts)
	assert.Equal(t, 1, s.Failures)
	assert.Equal(t, clk.Now(), s.LastPing)
	assert.True(t, s.LastActivity.IsZero(), "a health ping is not activity")
}

func TestWindowOutcomesAgeOut(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(3*time.Second, clk.Now)
	ctx := context.Background()

	w.Record(ctx, driver.ErrBadConn) // second 0
	clk.Advance(time.Second)
	w.Record(ctx, nil) // second 1
	clk.Advance(time.Second)
	w.Record(ctx, nil) // second 2

	s := w.Snapshot(clk.Now())
	assert.Equal(t, 3, s.Attempts)
	assert.Equal(t, 1, s.Failures)

	clk.Advance(time.Second) // second 3: second 0 leaves the window
	s = w.Snapshot(clk.Now())
	assert.Equal(t, 2, s.Attempts)
	assert.Zero(t, s.Failures)

	w.Record(ctx, nil) // reuses second 0's bucket
	s = w.Snapshot(clk.Now())
	assert.Equal(t, 3, s.Attempts)
	assert.Zero(t, s.Failures)

	clk.Advance(10 * time.Second)
	s = w.Snapshot(clk.Now())
	assert.Zero(t, s.Attempts)
	assert.Equal(t, FailureUnreachable, s.LastFailureKind, "the last failure kind outlives the window")
}

func TestWindowSnapshotIgnoresFutureAndPastInstants(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(5*time.Second, clk.Now)
	clk.Advance(2 * time.Second)
	w.Record(context.Background(), nil)

	assert.Zero(t, w.Snapshot(clk.Now().Add(-time.Second)).Attempts, "an outcome after the instant is not counted")
	assert.Equal(t, 1, w.Snapshot(clk.Now().Add(4*time.Second)).Attempts)
	assert.Zero(t, w.Snapshot(clk.Now().Add(5*time.Second)).Attempts)
}

func TestWindowLengthRoundsUpToWholeSeconds(t *testing.T) {
	for _, tt := range []struct {
		length time.Duration
		want   int
	}{
		{0, 1},
		{-time.Second, 1},
		{500 * time.Millisecond, 1},
		{time.Second, 1},
		{1500 * time.Millisecond, 2},
		{30 * time.Second, 30},
	} {
		assert.Len(t, NewWindow(tt.length, nil).buckets, tt.want, tt.length)
	}
}

func TestWindowSecondFloorsBeforeOrigin(t *testing.T) {
	w := NewWindow(time.Second, nil)
	assert.Equal(t, int64(0), w.second(w.origin))
	assert.Equal(t, int64(0), w.second(w.origin.Add(999*time.Millisecond)))
	assert.Equal(t, int64(-1), w.second(w.origin.Add(-time.Millisecond)))
	assert.Equal(t, int64(-1), w.second(w.origin.Add(-time.Second)))
	assert.Equal(t, int64(-2), w.second(w.origin.Add(-1001*time.Millisecond)))
}

func TestWindowDefaultClock(t *testing.T) {
	w := NewWindow(time.Minute, nil)
	w.Record(context.Background(), nil)
	s := w.Snapshot(time.Now())
	assert.Equal(t, 1, s.Attempts)
	assert.WithinDuration(t, time.Now(), s.LastActivity, time.Second)
}

func TestWindowConcurrentRecording(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(30*time.Second, clk.Now)
	const workers, perWorker = 8, 500
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			if i%2 == 0 {
				ctx = WithHealthPing(ctx)
			}
			for j := range perWorker {
				if j%5 == 0 {
					w.Record(ctx, driver.ErrBadConn)
				} else {
					w.Record(ctx, nil)
				}
				if j%100 == 0 {
					w.Snapshot(clk.Now())
				}
			}
		}(i)
	}
	wg.Wait()

	s := w.Snapshot(clk.Now())
	require.Equal(t, workers*perWorker, s.Attempts)
	assert.Equal(t, workers*perWorker/5, s.Failures)
	assert.False(t, s.LastActivity.IsZero())
	assert.False(t, s.LastPing.IsZero())
}
