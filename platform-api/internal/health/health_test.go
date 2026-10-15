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

package health

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	return &fakeClock{t: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func testOptions() Options {
	return Options{
		Component:     "platform-api",
		HealthyStatus: "ok",
		CheckTimeout:  200 * time.Millisecond,
		CacheTTL:      3 * time.Second,
	}
}

// newTestChecker returns a Checker driven by a fake clock.
func newTestChecker(opts Options) (*Checker, *fakeClock) {
	clk := newFakeClock()
	c := NewChecker(opts)
	c.now = clk.Now
	return c, clk
}

func passing(context.Context) error { return nil }

func TestEvaluateOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		checks      map[string]CheckFunc
		wantHealthy bool
		wantStatus  string
		wantChecks  map[string]CheckResult
	}{
		{
			name:        "no checks registered",
			checks:      map[string]CheckFunc{},
			wantHealthy: true,
			wantStatus:  "ok",
			wantChecks:  map[string]CheckResult{},
		},
		{
			name:        "all checks pass",
			checks:      map[string]CheckFunc{"database": passing, "cache": passing},
			wantHealthy: true,
			wantStatus:  "ok",
			wantChecks: map[string]CheckResult{
				"database": {Status: StatusUp},
				"cache":    {Status: StatusUp},
			},
		},
		{
			name: "a check fails with its own reason",
			checks: map[string]CheckFunc{
				"database": func(context.Context) error { return Reason("database unreachable") },
				"cache":    passing,
			},
			wantHealthy: false,
			wantStatus:  UnhealthyStatus,
			wantChecks: map[string]CheckResult{
				"database": {Status: StatusDown, Error: "database unreachable"},
				"cache":    {Status: StatusUp},
			},
		},
		{
			name: "a check panics",
			checks: map[string]CheckFunc{
				"database": func(context.Context) error { panic("boom") },
			},
			wantHealthy: false,
			wantStatus:  UnhealthyStatus,
			wantChecks: map[string]CheckResult{
				"database": {Status: StatusDown, Error: genericFailure},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, clk := newTestChecker(testOptions())
			for name, fn := range tt.checks {
				c.Register(name, fn)
			}

			res := c.Evaluate()

			assert.Equal(t, tt.wantHealthy, res.Healthy)
			assert.Equal(t, tt.wantStatus, res.Status)
			assert.Equal(t, "platform-api", res.Component)
			assert.Equal(t, clk.Now(), res.EvaluatedAt)
			require.Len(t, res.Checks, len(tt.wantChecks))
			for name, want := range tt.wantChecks {
				got := res.Checks[name]
				assert.Equal(t, want.Status, got.Status, name)
				assert.Equal(t, want.Error, got.Error, name)
				if want.Status == StatusDown {
					assert.Equal(t, clk.Now(), got.Since, name)
				} else {
					assert.True(t, got.Since.IsZero(), name)
				}
			}
		})
	}
}

func TestEvaluateHangingCheckIsBounded(t *testing.T) {
	opts := testOptions()
	c := NewChecker(opts)
	release := make(chan struct{})
	defer close(release)

	// Ignores its context entirely, so only the evaluation deadline can end the wait.
	c.Register("database", func(context.Context) error {
		<-release
		return nil
	})
	c.Register("cache", passing)

	start := time.Now()
	res := c.Evaluate()
	elapsed := time.Since(start)

	assert.Less(t, elapsed, opts.CheckTimeout+500*time.Millisecond)
	assert.False(t, res.Healthy)
	assert.Equal(t, StatusDown, res.Checks["database"].Status)
	assert.Equal(t, "timed out after 200ms", res.Checks["database"].Error)
	assert.Equal(t, StatusUp, res.Checks["cache"].Status)
}

func TestEvaluateContextAwareCheckTimesOut(t *testing.T) {
	c := NewChecker(testOptions())
	c.Register("database", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	res := c.Evaluate()

	assert.Equal(t, "timed out after 200ms", res.Checks["database"].Error)
}

func TestEvaluateNeverExposesRawErrors(t *testing.T) {
	const secret = "postgres://admin:s3cret@db-internal.example:5432/apip"
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "raw error", err: errors.New("dial " + secret + ": connection refused"), want: genericFailure},
		{name: "raw error wrapping a reason", err: fmt.Errorf("%s: %w", secret, Reason("database unreachable")), want: "database unreachable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestChecker(testOptions())
			c.Register("database", func(context.Context) error { return tt.err })

			res := c.Evaluate()

			got := res.Checks["database"]
			assert.Equal(t, tt.want, got.Error)
			assert.NotContains(t, got.Error, "s3cret")
			assert.NotContains(t, got.Error, "db-internal")
		})
	}
}

func TestEvaluateCachesWithinTTL(t *testing.T) {
	c, clk := newTestChecker(testOptions())
	var runs atomic.Int32
	c.Register("database", func(context.Context) error {
		runs.Add(1)
		return nil
	})

	first := c.Evaluate()
	clk.Advance(2 * time.Second)
	second := c.Evaluate()

	assert.Equal(t, int32(1), runs.Load(), "a result younger than the TTL must be reused")
	assert.Equal(t, first.EvaluatedAt, second.EvaluatedAt)

	clk.Advance(time.Second)
	third := c.Evaluate()

	assert.Equal(t, int32(2), runs.Load(), "a result as old as the TTL must be refreshed")
	assert.True(t, third.EvaluatedAt.After(first.EvaluatedAt))
}

func TestEvaluateConcurrentCallersShareOneRun(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	var runs atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	c.Register("database", func(context.Context) error {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil
	})

	const callers = 20
	results := make([]Result, callers)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		results[0] = c.Evaluate()
	}()
	<-started
	for i := 1; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = c.Evaluate()
		}(i)
	}
	// Give the waiting callers time to queue behind the in-flight run.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), runs.Load())
	for i := 1; i < callers; i++ {
		assert.Equal(t, results[0].EvaluatedAt, results[i].EvaluatedAt)
		assert.True(t, results[i].Healthy)
	}
}

func TestEvaluateKeepsFailureStartAcrossEvaluations(t *testing.T) {
	c, clk := newTestChecker(testOptions())
	var failing atomic.Bool
	failing.Store(true)
	c.Register("database", func(context.Context) error {
		if failing.Load() {
			return Reason("database unreachable")
		}
		return nil
	})

	failedAt := clk.Now()
	first := c.Evaluate()
	clk.Advance(10 * time.Second)
	second := c.Evaluate()

	assert.Equal(t, failedAt, first.Checks["database"].Since)
	assert.Equal(t, failedAt, second.Checks["database"].Since, "the failure start must not move while the check stays down")

	failing.Store(false)
	clk.Advance(10 * time.Second)
	recovered := c.Evaluate()
	assert.Equal(t, StatusUp, recovered.Checks["database"].Status)
	assert.True(t, recovered.Checks["database"].Since.IsZero())

	failing.Store(true)
	clk.Advance(10 * time.Second)
	refailedAt := clk.Now()
	again := c.Evaluate()
	assert.Equal(t, refailedAt, again.Checks["database"].Since, "a new failure starts a new failure window")
}

func TestEvaluateKeepsRawCauseForDebugOnly(t *testing.T) {
	c, _ := newTestChecker(testOptions())
	raw := errors.New("dial tcp 10.0.0.5:5432: connection refused")
	c.Register("database", func(context.Context) error { return raw })

	res := c.Evaluate()

	got := res.Checks["database"]
	assert.ErrorIs(t, got.cause, raw)
	assert.False(t, strings.Contains(got.Error, "10.0.0.5"))
}
