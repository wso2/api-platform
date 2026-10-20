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
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	failureLine  = regexp.MustCompile(`(?i)health check.*(fail|unhealthy|down|error)`)
	recoveryLine = regexp.MustCompile(`(?i)health check.*(recover|healthy again|back up)`)
)

// logCapture collects the output of a health logger and a debug logger.
type logCapture struct {
	health bytes.Buffer
	debug  bytes.Buffer
}

func (l *logCapture) options() Options {
	opts := testOptions()
	opts.Logger = slog.New(slog.NewTextHandler(&l.health, &slog.HandlerOptions{Level: slog.LevelInfo}))
	opts.DebugLogger = slog.New(slog.NewTextHandler(&l.debug, &slog.HandlerOptions{Level: slog.LevelDebug}))
	opts.BackoffInitial = 10 * time.Second
	opts.BackoffMax = 5 * time.Minute
	return opts
}

func (l *logCapture) healthLines() []string {
	return nonEmptyLines(l.health.String())
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func countContaining(lines []string, sub string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

// probeFor calls Evaluate every 2s for the given duration, starting with a probe at the
// current time.
func probeFor(c *Checker, clk *fakeClock, d time.Duration) {
	for elapsed := time.Duration(0); elapsed < d; elapsed += 2 * time.Second {
		c.Evaluate()
		clk.Advance(2 * time.Second)
	}
}

func TestBackoffScheduleThreeMinuteOutage(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var down error = Reason("database unreachable")
	c.Register("database", func(context.Context) error { return down })

	probeFor(c, clk, 3*time.Minute)

	// Probes arrive every 2s but the 3s cache means the checks only run every 4s, so
	// repeats are logged at the first evaluation at or after each due time: 0s (first
	// failure, next due 10s), 12s (next due 32s), 32s (due 72s), 72s (due 152s) and
	// 152s (due 312s, past the end of the outage). That is exactly five lines.
	lines := logs.healthLines()
	require.Equal(t, 5, countContaining(lines, "health check failed"), strings.Join(lines, "\n"))
	assert.Contains(t, lines[0], "reason=\"database unreachable\"")
	assert.NotContains(t, lines[0], "failing_for")
	assert.Contains(t, lines[1], "failing_for=12s")
	assert.Contains(t, lines[1], "failed_probes=4")
	assert.Contains(t, lines[4], "failing_for=2m32s")

	down = nil
	c.Evaluate()

	lines = logs.healthLines()
	assert.Equal(t, 1, countContaining(lines, "health check recovered"))
	assert.Contains(t, lines[len(lines)-1], "down_for=3m0s")
	assert.Equal(t, 6, len(lines))
}

func TestBackoffScheduleThirtyMinuteOutage(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	c.Register("database", func(context.Context) error { return Reason("database unreachable") })

	probeFor(c, clk, 30*time.Minute)

	// Evaluations run every 4s. Lines at 0, 12, 32, 72 and 152s as in the three-minute
	// case, then the interval reaches 160s (152s -> 312s), and from there it is capped at
	// 300s: 312, 612, 912, 1212 and 1512s. The next is 1812s, after the outage window.
	assert.Equal(t, 10, countContaining(logs.healthLines(), "health check failed"))
}

func TestBackoffChecksHaveIndependentSchedules(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var cacheErr error
	c.Register("database", func(context.Context) error { return Reason("database unreachable") })
	c.Register("cache", func(context.Context) error { return cacheErr })

	probeFor(c, clk, 20*time.Second)
	cacheErr = Reason("cache unreachable")
	probeFor(c, clk, 20*time.Second)

	var dbLines, cacheLines []string
	for _, l := range logs.healthLines() {
		switch {
		case strings.Contains(l, "check=database"):
			dbLines = append(dbLines, l)
		case strings.Contains(l, "check=cache"):
			cacheLines = append(cacheLines, l)
		}
	}
	// The database fails from 0s and logs at 0s and 12s; the cache fails from 20s and logs
	// at 20s (first line, no failing_for) and then waits out its own 10s interval.
	require.Len(t, dbLines, 3)
	require.Len(t, cacheLines, 2)
	assert.NotContains(t, cacheLines[0], "failing_for")
	assert.Contains(t, cacheLines[1], "failing_for=12s")
	assert.Contains(t, dbLines[2], "failing_for=32s")
}

func TestBackoffRecoveryResetsSchedule(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var err error = Reason("database unreachable")
	c.Register("database", func(context.Context) error { return err })

	probeFor(c, clk, 30*time.Second)
	err = nil
	probeFor(c, clk, 4*time.Second)
	before := len(logs.healthLines())

	err = Reason("database unreachable")
	clk.Advance(3 * time.Second) // let the cached answer expire
	c.Evaluate()

	lines := logs.healthLines()
	require.Len(t, lines, before+1)
	last := lines[len(lines)-1]
	assert.Contains(t, last, "health check failed")
	assert.NotContains(t, last, "failing_for")
	assert.Equal(t, 1, countContaining(lines, "health check recovered"))
}

func TestBackoffMessagesMatchOperatorPatterns(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var err error = Reason("database unreachable")
	c.Register("database", func(context.Context) error { return err })

	c.Evaluate()
	clk.Advance(10 * time.Second)
	err = nil
	c.Evaluate()

	lines := logs.healthLines()
	require.Len(t, lines, 2)
	assert.Regexp(t, failureLine, lines[0])
	assert.NotRegexp(t, recoveryLine, lines[0])
	assert.Regexp(t, recoveryLine, lines[1])
	assert.Contains(t, lines[0], "level=ERROR")
	assert.Contains(t, lines[1], "level=INFO")
}

func TestBackoffRawCauseOnlyReachesDebugLogger(t *testing.T) {
	const secret = "postgres://u:p@host"
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	c.Register("database", func(context.Context) error {
		return fmt.Errorf("dial %s: %w", secret, errors.New("connection refused"))
	})

	probeFor(c, clk, 20*time.Second)

	assert.NotContains(t, logs.health.String(), secret)
	assert.Contains(t, logs.health.String(), `reason="check failed"`)
	assert.Contains(t, logs.debug.String(), secret)
	assert.Contains(t, logs.debug.String(), "health check error detail")
	assert.Contains(t, logs.debug.String(), "level=DEBUG")
}

func TestBackoffNilLoggerIsSilent(t *testing.T) {
	opts := testOptions()
	c, clk := newTestChecker(opts)
	var err error = Reason("database unreachable")
	c.Register("database", func(context.Context) error { return err })

	assert.Nil(t, c.logger)
	assert.NotPanics(t, func() {
		probeFor(c, clk, 20*time.Second)
		err = nil
		c.Evaluate()
	})
}

func TestBackoffNilDebugLoggerIsAccepted(t *testing.T) {
	var buf bytes.Buffer
	opts := testOptions()
	opts.Logger = slog.New(slog.NewTextHandler(&buf, nil))
	opts.BackoffInitial = 10 * time.Second
	opts.BackoffMax = time.Minute
	c, _ := newTestChecker(opts)
	c.Register("database", func(context.Context) error { return errors.New("boom") })

	assert.NotPanics(t, func() { c.Evaluate() })
	assert.Contains(t, buf.String(), "health check failed")
}

func TestBackoffCachedAnswersDoNotObserve(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	c.Register("database", func(context.Context) error { return Reason("database unreachable") })

	c.Evaluate()
	clk.Advance(time.Second)
	c.Evaluate()

	assert.Equal(t, 1, c.logger.failures["database"].failedCount)
}

func TestBackoffDropsStateForRemovedChecks(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	b := newBackoffLogger(opts.Logger, opts.DebugLogger, opts.BackoffInitial, opts.BackoffMax)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

	b.observe(Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable", Since: now},
	}}, now)
	require.Contains(t, b.failures, "database")

	b.observe(Result{Checks: map[string]CheckResult{}}, now.Add(time.Second))
	assert.Empty(t, b.failures)
	assert.Equal(t, 1, len(logs.healthLines()), "dropping state must not log a recovery")

	// A reappearing failure starts a fresh schedule and logs immediately.
	b.observe(Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable", Since: now},
	}}, now.Add(2*time.Second))
	assert.Equal(t, 2, len(logs.healthLines()))
}

func TestBackoffFallsBackToNowWhenSinceIsZero(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	b := newBackoffLogger(opts.Logger, opts.DebugLogger, opts.BackoffInitial, opts.BackoffMax)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

	b.observe(Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable"},
	}}, now)

	assert.Equal(t, now, b.failures["database"].failingSince)
}

func TestBackoffUpCheckWithNoStateIsSilent(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	b := newBackoffLogger(opts.Logger, opts.DebugLogger, opts.BackoffInitial, opts.BackoffMax)

	b.observe(Result{Checks: map[string]CheckResult{"database": {Status: StatusUp}}}, time.Now())

	assert.Empty(t, logs.healthLines())
	assert.Empty(t, b.failures)
}

func TestBackoffIntervalIsCapped(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	opts.BackoffInitial = time.Second
	opts.BackoffMax = 4 * time.Second
	b := newBackoffLogger(opts.Logger, opts.DebugLogger, opts.BackoffInitial, opts.BackoffMax)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	res := Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable", Since: now},
	}}

	// Lines at 0s, then intervals 1s, 2s, 4s, 4s: 1s, 3s, 7s, 11s.
	for s := 0; s <= 11; s++ {
		b.observe(res, now.Add(time.Duration(s)*time.Second))
	}

	assert.Equal(t, 5, len(logs.healthLines()))
	assert.Equal(t, 4*time.Second, b.failures["database"].interval)
}

func TestBackoffSkipsUnloggedChecks(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var startErr error = Reason("starting")
	c.RegisterUnlogged("startup", func(context.Context) error { return startErr })
	c.Register("database", func(context.Context) error { return errors.New("connection refused") })

	probeFor(c, clk, 30*time.Second)
	startErr = nil
	clk.Advance(5 * time.Second)
	res := c.Evaluate()

	assert.Equal(t, StatusUp, res.Checks["startup"].Status)
	lines := logs.healthLines()
	assert.Zero(t, countContaining(lines, "check=startup"), "unlogged check must not log: %v", lines)
	assert.NotZero(t, countContaining(lines, "check=database"))
	assert.NotContains(t, logs.debug.String(), "check=startup")
	assert.NotContains(t, c.logger.failures, "startup")
}
