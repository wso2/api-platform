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

package server

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database/observe"
	"github.com/wso2/api-platform/platform-api/internal/health"
)

// errDatabaseUnreachable is the reason reported when the database cannot be reached.
// Health answers are served without authentication, so the driver error itself, which
// can name hosts, users or DSN fragments, is kept only as the wrapped cause.
var errDatabaseUnreachable = health.Reason("database unreachable")

// errPoolExhausted is the reason reported when the connection pool has stayed
// saturated for too long.
var errPoolExhausted = health.Reason("database connection pool exhausted")

// databasePool is the part of the database pool the health check needs.
type databasePool interface {
	PingContext(ctx context.Context) error
	Stats() sql.DBStats
}

// databaseCheck judges the database from the outcomes of the platform's own database
// operations, recorded in window, and pings it itself only while those are idle:
//
//  1. With at least min_attempts outcomes in the window, the database is down when the
//     share of connection failures reaches failure_ratio.
//  2. When the platform has neither used the database nor pinged it within
//     idle_period, one ping decides.
//  3. Otherwise the last decided state stands; before anything is decided, a ping
//     decides.
//
// Whatever these decide, a pool that has stayed saturated for pool_wait_limit reports
// the database down. Without a window, every evaluation pings.
type databaseCheck struct {
	pool     databasePool
	window   *observe.Window
	settings config.HealthDatabase
	now      func() time.Time

	// pinging is set while a ping is in flight.
	pinging atomic.Bool

	mu sync.Mutex
	// decided is the last decided result, valid once hasDecided is set.
	decided    error
	hasDecided bool
	// pingStarted is when the last ping started. A ping still waiting for a pooled
	// connection has recorded nothing in the window yet, so this, not only the window's
	// last ping time, keeps pings to one per idle period.
	pingStarted time.Time
	// saturatedSince is when the pool became saturated; zero while it is not.
	saturatedSince time.Time
}

// newDatabaseCheck returns the health check for the database pool. now is the clock it
// judges idleness and saturation by.
func newDatabaseCheck(pool databasePool, window *observe.Window, settings config.HealthDatabase,
	now func() time.Time) health.CheckFunc {
	c := &databaseCheck{pool: pool, window: window, settings: settings, now: now}
	return c.check
}

func (c *databaseCheck) check(ctx context.Context) error {
	err := c.reachability(ctx)
	if poolErr := c.poolPressure(c.now()); err == nil {
		err = poolErr
	}
	return err
}

// reachability applies the decision rules for whether the database can be reached.
func (c *databaseCheck) reachability(ctx context.Context) error {
	if c.window == nil {
		return c.ping(ctx)
	}
	now := c.now()
	snap := c.window.Snapshot(now)
	if snap.Attempts >= c.settings.MinAttempts {
		err := c.failureRatio(snap)
		c.decide(err)
		return err
	}

	c.mu.Lock()
	lastPing := snap.LastPing
	if c.pingStarted.After(lastPing) {
		lastPing = c.pingStarted
	}
	idle := c.olderThanIdlePeriod(snap.LastActivity, now) && c.olderThanIdlePeriod(lastPing, now)
	decided, hasDecided := c.decided, c.hasDecided
	c.mu.Unlock()

	if hasDecided && !idle {
		return decided
	}
	return c.ping(ctx)
}

func (c *databaseCheck) olderThanIdlePeriod(t, now time.Time) bool {
	return t.IsZero() || now.Sub(t) >= c.settings.IdlePeriod
}

// failureRatio decides from the window's outcomes. A timeout reason is reported when the
// most recent connection failure was a timeout.
func (c *databaseCheck) failureRatio(snap observe.Snapshot) error {
	if float64(snap.Failures) < c.settings.FailureRatio*float64(snap.Attempts) {
		return nil
	}
	if snap.LastFailureKind == observe.FailureTimeout {
		return fmt.Errorf("%d of %d database operations failed: %w", snap.Failures, snap.Attempts,
			context.DeadlineExceeded)
	}
	return fmt.Errorf("%w: %d of %d database operations failed", errDatabaseUnreachable,
		snap.Failures, snap.Attempts)
}

func (c *databaseCheck) decide(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decided, c.hasDecided = err, true
}

// ping pings the database as the health check's own ping and decides from its result.
// A ping cut short by the check deadline is reported as a timeout; any other ping error
// as the database being unreachable.
//
// At most one ping is in flight at a time. A driver that ignores its context could
// otherwise leave one ping per evaluation stuck on the database, so probes against a
// hung database would pile up pooled connections; while an earlier ping is still
// running, the check reports a timeout without starting another.
func (c *databaseCheck) ping(ctx context.Context) error {
	c.mu.Lock()
	c.pingStarted = c.now()
	c.mu.Unlock()
	err := c.pingOnce(ctx)
	c.decide(err)
	return err
}

func (c *databaseCheck) pingOnce(ctx context.Context) error {
	if !c.pinging.CompareAndSwap(false, true) {
		return context.DeadlineExceeded
	}
	// Buffered, so the ping goroutine never blocks once the check has given up.
	done := make(chan error, 1)
	go func() {
		err := c.pool.PingContext(observe.WithHealthPing(ctx))
		// Cleared before the result is handed over, so the next evaluation after
		// this one can ping again.
		c.pinging.Store(false)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: %w", errDatabaseUnreachable, err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// poolPressure reports the pool as exhausted once at least pool_saturation of its
// connections have been in use for pool_wait_limit. It is skipped for an unlimited or
// single-connection pool, where saturation says nothing about the database.
func (c *databaseCheck) poolPressure(now time.Time) error {
	stats := c.pool.Stats()
	c.mu.Lock()
	defer c.mu.Unlock()
	if stats.MaxOpenConnections <= 1 || stats.InUse < saturationThreshold(c.settings.PoolSaturation, stats.MaxOpenConnections) {
		c.saturatedSince = time.Time{}
		return nil
	}
	if c.saturatedSince.IsZero() {
		c.saturatedSince = now
	}
	if saturated := now.Sub(c.saturatedSince); saturated >= c.settings.PoolWaitLimit {
		return fmt.Errorf("%w: %d of %d connections in use for %s", errPoolExhausted,
			stats.InUse, stats.MaxOpenConnections, saturated)
	}
	return nil
}

// saturationThreshold is the number of connections in use, out of maxOpen, at which the
// pool counts as saturated: ratio of maxOpen, rounded up. A tiny tolerance keeps float
// error such as 0.7 × 10 = 7.000000000000001 from rounding up a whole connection.
func saturationThreshold(ratio float64, maxOpen int) int {
	return int(math.Ceil(ratio*float64(maxOpen) - 1e-9))
}
