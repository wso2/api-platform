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

func TestTransitionLogsOneLinePerOutage(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var down error = Reason("database unreachable")
	c.Register("database", func(context.Context) error { return down })

	// Each evaluation runs past the 3s cache, so all 90 run the checks.
	for range 90 {
		c.Evaluate()
		clk.Advance(4 * time.Second)
	}

	lines := logs.healthLines()
	require.Len(t, lines, 1, strings.Join(lines, "\n"))
	assert.Contains(t, lines[0], "health check failed")
	assert.Contains(t, lines[0], "check=database")
	assert.Contains(t, lines[0], `reason="database unreachable"`)

	down = nil
	c.Evaluate()

	lines = logs.healthLines()
	require.Len(t, lines, 2, strings.Join(lines, "\n"))
	assert.Contains(t, lines[1], "health check recovered")
	assert.Contains(t, lines[1], "check=database")
	assert.Contains(t, lines[1], "down_for=6m0s")
	for _, l := range lines {
		assert.NotContains(t, l, "failing_for")
		assert.NotContains(t, l, "failed_probes")
	}
}

func TestTransitionStaysSilentWhileUp(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	c.Register("database", passing)

	probeFor(c, clk, time.Minute)

	assert.Empty(t, logs.healthLines())
}

func TestTransitionChecksAreIndependent(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var cacheErr error
	c.Register("database", func(context.Context) error { return Reason("database unreachable") })
	c.Register("cache", func(context.Context) error { return cacheErr })

	probeFor(c, clk, 20*time.Second)
	cacheErr = Reason("cache unreachable")
	probeFor(c, clk, 20*time.Second)
	cacheErr = nil
	probeFor(c, clk, 20*time.Second)

	lines := logs.healthLines()
	require.Len(t, lines, 3, strings.Join(lines, "\n"))
	assert.Contains(t, lines[0], "check=database")
	assert.Contains(t, lines[0], "health check failed")
	assert.Contains(t, lines[1], "check=cache")
	assert.Contains(t, lines[1], "health check failed")
	assert.Contains(t, lines[2], "check=cache")
	assert.Contains(t, lines[2], "health check recovered")
	assert.Contains(t, lines[2], "down_for=20s")
}

func TestTransitionLogsEachNewOutage(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var err error = Reason("database unreachable")
	c.Register("database", func(context.Context) error { return err })

	probeFor(c, clk, 30*time.Second)
	err = nil
	probeFor(c, clk, 4*time.Second)
	err = Reason("database unreachable")
	probeFor(c, clk, 30*time.Second)

	lines := logs.healthLines()
	assert.Equal(t, 2, countContaining(lines, "health check failed"), strings.Join(lines, "\n"))
	assert.Equal(t, 1, countContaining(lines, "health check recovered"))
	assert.Len(t, lines, 3)
}

func TestTransitionRecoveryMeasuresFromReportedStart(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	start := clk.Now().Add(-5 * time.Minute)
	var err error = FailingSince(Reason("disconnected from the controller"), start)
	c.Register("xds_connection", func(context.Context) error { return err })

	c.Evaluate()
	clk.Advance(10 * time.Second)
	err = nil
	c.Evaluate()

	lines := logs.healthLines()
	require.Len(t, lines, 2)
	assert.Contains(t, lines[1], "down_for=5m10s")
}

func TestTransitionMessagesMatchOperatorPatterns(t *testing.T) {
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

func TestTransitionRawCauseOnlyReachesDebugLogger(t *testing.T) {
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

func TestTransitionNilLoggerIsSilent(t *testing.T) {
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

func TestTransitionNilDebugLoggerIsAccepted(t *testing.T) {
	var buf bytes.Buffer
	opts := testOptions()
	opts.Logger = slog.New(slog.NewTextHandler(&buf, nil))
	c, _ := newTestChecker(opts)
	c.Register("database", func(context.Context) error { return errors.New("boom") })

	assert.NotPanics(t, func() { c.Evaluate() })
	assert.Contains(t, buf.String(), "health check failed")
}

func TestTransitionDropsStateForRemovedChecks(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	tl := newTransitionLogger(opts.Logger, opts.DebugLogger)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

	tl.observe(Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable", Since: now},
	}}, now)
	require.Contains(t, tl.failingSince, "database")

	tl.observe(Result{Checks: map[string]CheckResult{}}, now.Add(time.Second))
	assert.Empty(t, tl.failingSince)
	assert.Equal(t, 1, len(logs.healthLines()), "dropping state must not log a recovery")

	// A reappearing failure is a new outage and is logged again.
	tl.observe(Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable", Since: now},
	}}, now.Add(2*time.Second))
	assert.Equal(t, 2, len(logs.healthLines()))
}

func TestTransitionFallsBackToNowWhenSinceIsZero(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	tl := newTransitionLogger(opts.Logger, opts.DebugLogger)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

	tl.observe(Result{Checks: map[string]CheckResult{
		"database": {Status: StatusDown, Error: "database unreachable"},
	}}, now)

	assert.Equal(t, now, tl.failingSince["database"])
}

func TestTransitionUpCheckWithNoStateIsSilent(t *testing.T) {
	var logs logCapture
	opts := logs.options()
	tl := newTransitionLogger(opts.Logger, opts.DebugLogger)

	tl.observe(Result{Checks: map[string]CheckResult{"database": {Status: StatusUp}}}, time.Now())

	assert.Empty(t, logs.healthLines())
	assert.Empty(t, tl.failingSince)
}

func TestTransitionSkipsUnloggedChecks(t *testing.T) {
	var logs logCapture
	c, clk := newTestChecker(logs.options())
	var loadErr error = Reason("policy configuration not loaded")
	c.RegisterUnlogged("policy_engine", func(context.Context) error { return loadErr })
	c.Register("xds_connection", func(context.Context) error { return errors.New("connection refused") })

	probeFor(c, clk, 30*time.Second)
	loadErr = nil
	clk.Advance(5 * time.Second)
	res := c.Evaluate()

	assert.Equal(t, StatusUp, res.Checks["policy_engine"].Status)
	lines := logs.healthLines()
	assert.Zero(t, countContaining(lines, "check=policy_engine"), "unlogged check must not log: %v", lines)
	assert.Equal(t, 1, countContaining(lines, "check=xds_connection"))
	assert.NotContains(t, logs.debug.String(), "check=policy_engine")
	assert.NotContains(t, c.logger.failingSince, "policy_engine")
}
