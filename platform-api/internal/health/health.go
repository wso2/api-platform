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

// Package health runs platform-api's dependency checks and turns them into the health
// answer served to probes and load balancers: every check runs in parallel under a
// deadline, the overall result is cached briefly, and a failure never exposes raw error
// text.
package health

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// CheckStatus is the state of a single check.
type CheckStatus string

const (
	// StatusUp means the check passed.
	StatusUp CheckStatus = "up"
	// StatusDown means the check failed, timed out or panicked.
	StatusDown CheckStatus = "down"
)

// UnhealthyStatus is the overall status value reported when any check is down.
const UnhealthyStatus = "unhealthy"

// errCheckPanicked stands in for a check that panicked. It carries no Reason, so it is
// reported with the generic failure text.
var errCheckPanicked = errors.New("health check panicked")

// CheckFunc tests one dependency. It must return promptly once ctx is done; a nil error
// means the dependency is usable.
type CheckFunc func(ctx context.Context) error

// Options configures a Checker.
type Options struct {
	// Component names the reporting component in every result.
	Component string
	// HealthyStatus is the overall status value reported when every check passes.
	HealthyStatus string
	// CheckTimeout bounds each check run.
	CheckTimeout time.Duration
	// CacheTTL is how long an evaluated result is reused before the checks run again.
	CacheTTL time.Duration
	// Logger receives health check failure and recovery lines. Nil disables health
	// logging.
	Logger *slog.Logger
	// DebugLogger receives raw check error details at debug level. Nil disables them.
	DebugLogger *slog.Logger
	// BackoffInitial is the interval before a still-failing check is logged again; it
	// doubles after each repeat.
	BackoffInitial time.Duration
	// BackoffMax caps the repeat interval.
	BackoffMax time.Duration
}

// CheckResult is the outcome of one check in an evaluation.
type CheckResult struct {
	Status CheckStatus
	// Error is the safe failure reason. Empty when the check is up.
	Error string
	// Since is when the current failure started. Zero when the check is up.
	Since time.Time
	// cause is the raw check error, kept for debug logging only. It is never rendered.
	cause error
	// unlogged marks a check registered with RegisterUnlogged; the health logger skips it.
	unlogged bool
}

// Result is the outcome of one evaluation of every registered check. A Result is shared
// by every caller served from the same evaluation and must not be modified.
type Result struct {
	Healthy     bool
	Status      string
	Component   string
	EvaluatedAt time.Time
	Checks      map[string]CheckResult
}

type namedCheck struct {
	name string
	fn   CheckFunc
}

// evaluation is one run of the checks. done is closed once result is set, so callers
// that arrived while it was running can share its result.
type evaluation struct {
	done   chan struct{}
	result Result
}

// Checker runs the registered checks and caches the overall result.
type Checker struct {
	opts Options
	now  func() time.Time

	mu       sync.Mutex
	checks   []namedCheck
	cached   *evaluation
	cachedAt time.Time
	inflight *evaluation
	// since records when each currently failing check started failing, so the start
	// time survives across evaluations for as long as the check stays down.
	since map[string]time.Time
	// unlogged holds the names of checks whose failures and recoveries are not logged.
	unlogged map[string]bool
	// logger writes state-change lines; nil when health logging is disabled.
	logger *backoffLogger
}

// NewChecker returns a Checker with no checks registered.
func NewChecker(opts Options) *Checker {
	c := &Checker{
		opts:     opts,
		now:      time.Now,
		since:    make(map[string]time.Time),
		unlogged: make(map[string]bool),
	}
	if opts.Logger != nil {
		debug := opts.DebugLogger
		if debug == nil {
			debug = slog.New(slog.DiscardHandler)
		}
		c.logger = newBackoffLogger(opts.Logger, debug, opts.BackoffInitial, opts.BackoffMax)
	}
	return c
}

// Register adds a named check. Checks are normally registered during startup, before
// the health endpoint is served.
func (c *Checker) Register(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, namedCheck{name: name, fn: fn})
}

// RegisterUnlogged adds a named check that appears in the answer and decides health like
// any other check, but whose failures and recoveries are not logged. It suits a check
// that is expected to fail briefly on every start.
func (c *Checker) RegisterUnlogged(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, namedCheck{name: name, fn: fn})
	c.unlogged[name] = true
}

// Evaluate returns the current health result. A result younger than the cache TTL is
// returned as is; otherwise the checks run, and callers arriving while they run wait for
// and share that single evaluation rather than starting their own.
func (c *Checker) Evaluate() Result {
	c.mu.Lock()
	if c.cached != nil && c.now().Sub(c.cachedAt) < c.opts.CacheTTL {
		r := c.cached.result
		c.mu.Unlock()
		return r
	}
	if ev := c.inflight; ev != nil {
		c.mu.Unlock()
		<-ev.done
		return ev.result
	}
	ev := &evaluation{done: make(chan struct{})}
	c.inflight = ev
	checks := append([]namedCheck(nil), c.checks...)
	c.mu.Unlock()

	started := c.now()
	outcomes := c.run(checks)

	c.mu.Lock()
	ev.result = c.buildResult(started, outcomes)
	c.cached = ev
	c.cachedAt = c.now()
	if c.logger != nil {
		c.logger.observe(ev.result, c.cachedAt)
	}
	c.inflight = nil
	c.mu.Unlock()
	close(ev.done)
	return ev.result
}

type outcome struct {
	name string
	err  error
}

// run executes every check in parallel under one shared deadline and returns each
// check's error (nil when it passed). A check that has not returned by the deadline is
// reported as timed out; its goroutine is left to finish on its own and its late result
// is discarded, so a check that ignores its context cannot stall the health answer.
func (c *Checker) run(checks []namedCheck) map[string]error {
	errs := make(map[string]error, len(checks))
	if len(checks) == 0 {
		return errs
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.opts.CheckTimeout)
	defer cancel()

	// Buffered for every check, so late senders never block after the collector stops.
	results := make(chan outcome, len(checks))
	for _, chk := range checks {
		go func(chk namedCheck) {
			defer func() {
				if r := recover(); r != nil {
					results <- outcome{name: chk.name, err: errCheckPanicked}
				}
			}()
			results <- outcome{name: chk.name, err: chk.fn(ctx)}
		}(chk)
	}

collect:
	for range checks {
		select {
		case o := <-results:
			errs[o.name] = o.err
		case <-ctx.Done():
			break collect
		}
	}
	for _, chk := range checks {
		if _, ok := errs[chk.name]; !ok {
			errs[chk.name] = context.DeadlineExceeded
		}
	}
	return errs
}

// buildResult turns raw check errors into a Result and maintains each check's failure
// start time. The caller must hold c.mu.
func (c *Checker) buildResult(started time.Time, errs map[string]error) Result {
	res := Result{
		Healthy:     true,
		Status:      c.opts.HealthyStatus,
		Component:   c.opts.Component,
		EvaluatedAt: started,
		Checks:      make(map[string]CheckResult, len(errs)),
	}
	for name, err := range errs {
		if err == nil {
			delete(c.since, name)
			res.Checks[name] = CheckResult{Status: StatusUp, unlogged: c.unlogged[name]}
			continue
		}
		since, failing := c.since[name]
		if !failing {
			since = started
			c.since[name] = since
		}
		res.Healthy = false
		res.Checks[name] = CheckResult{
			Status:   StatusDown,
			Error:    safeReason(err, c.opts.CheckTimeout),
			Since:    since,
			cause:    err,
			unlogged: c.unlogged[name],
		}
	}
	if !res.Healthy {
		res.Status = UnhealthyStatus
	}
	return res
}
