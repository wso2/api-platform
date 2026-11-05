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

package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage/observe"
)

// testClock is a manually advanced clock, safe for concurrent use.
type testClock struct {
	mu    sync.Mutex
	start time.Time
	t     time.Time
}

func newTestClock() *testClock {
	start := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	return &testClock{start: start, t: start}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// Elapsed is how far the clock has been advanced.
func (c *testClock) Elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t.Sub(c.start)
}

// fakePool is a database pool whose ping result and statistics the test controls.
type fakePool struct {
	mu      sync.Mutex
	pingErr error
	stats   sql.DBStats
	pings   atomic.Int32
}

func (p *fakePool) PingContext(context.Context) error {
	p.pings.Add(1)
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pingErr
}

func (p *fakePool) Stats() sql.DBStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

func (p *fakePool) setPingErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pingErr = err
}

func (p *fakePool) setUsage(inUse, maxOpen int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.InUse, p.stats.MaxOpenConnections = inUse, maxOpen
}

// wait records n more callers having waited for a connection.
func (p *fakePool) wait(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.WaitCount += n
}

// checkFixture is a database check over a fake pool and a window driven by a test clock.
type checkFixture struct {
	clk    *testClock
	pool   *fakePool
	window *observe.Window
	check  health.CheckFunc
}

func newCheckFixture() *checkFixture {
	clk := newTestClock()
	settings := databaseSettings(2 * time.Second)
	f := &checkFixture{clk: clk, pool: &fakePool{}, window: observe.NewWindow(settings.FailureWindow, clk.Now)}
	f.check = newDatabaseCheck(f.pool, f.window, settings, clk.Now)
	return f
}

// traffic records n application operations ending with err.
func (f *checkFixture) traffic(n int, err error) {
	for i := 0; i < n; i++ {
		f.window.Record(context.Background(), err)
	}
}

func (f *checkFixture) evaluate() error {
	return f.check(context.Background())
}

func TestDatabaseCheckBusyInstanceNeverPings(t *testing.T) {
	f := newCheckFixture()
	for i := 0; i < 40; i++ {
		f.traffic(10, nil)
		require.NoError(t, f.evaluate())
		f.clk.Advance(3 * time.Second)
	}
	assert.Zero(t, f.pool.pings.Load(), "health must add no database calls while traffic flows")
}

func TestDatabaseCheckFailureRatio(t *testing.T) {
	f := newCheckFixture()
	f.traffic(2, driver.ErrBadConn)
	f.traffic(2, nil)
	require.NoError(t, f.evaluate(), "below min_attempts with no decision yet, the check pings")
	assert.Equal(t, int32(1), f.pool.pings.Load())

	f.clk.Advance(time.Second)
	f.traffic(1, nil)
	assert.NoError(t, f.evaluate(), "two failures in five attempts is below the ratio")

	f.clk.Advance(time.Second)
	f.traffic(1, driver.ErrBadConn)
	assert.Equal(t, errDatabaseUnreachable, f.evaluate(), "three failures in six attempts reaches the ratio")
	assert.Equal(t, int32(1), f.pool.pings.Load(), "decided from traffic alone")
}

func TestDatabaseCheckTimeoutReason(t *testing.T) {
	f := newCheckFixture()
	f.traffic(5, &pgconn.ConnectError{Config: &pgconn.Config{}})
	f.traffic(1, &netTimeout{})

	err := f.evaluate()

	assert.EqualError(t, err, "timed out after 2s")
	var reason health.Reason
	assert.ErrorAs(t, err, &reason)
}

func TestDatabaseCheckRecoversAsFailuresAgeOut(t *testing.T) {
	f := newCheckFixture()
	f.traffic(10, driver.ErrBadConn)
	require.Error(t, f.evaluate())

	for i := 0; i < 11; i++ {
		f.clk.Advance(3 * time.Second)
		f.traffic(5, nil)
	}
	assert.NoError(t, f.evaluate())
	assert.Zero(t, f.pool.pings.Load())
}

func TestDatabaseCheckQueryErrorsNeverCount(t *testing.T) {
	f := newCheckFixture()
	f.traffic(20, &pgconn.PgError{Code: "23505"})
	f.traffic(20, sql.ErrNoRows)
	f.traffic(20, context.DeadlineExceeded)

	assert.NoError(t, f.evaluate())
}

func TestDatabaseCheckIdlePingsOncePerIdlePeriod(t *testing.T) {
	f := newCheckFixture()
	f.pool.setPingErr(errors.New("connection refused"))

	assert.ErrorIs(t, f.evaluate(), errDatabaseUnreachable)
	for i := 0; i < 3; i++ {
		f.clk.Advance(3 * time.Second)
		assert.ErrorIs(t, f.evaluate(), errDatabaseUnreachable, "the last decision stands between pings")
	}
	assert.Equal(t, int32(1), f.pool.pings.Load())

	f.pool.setPingErr(nil)
	f.clk.Advance(time.Second)
	assert.NoError(t, f.evaluate(), "the next idle period pings again")
	assert.Equal(t, int32(2), f.pool.pings.Load())
}

// Traffic too thin to decide from does not keep a decision standing: the check still
// pings once per idle period, so it sees both an outage and a recovery.
func TestDatabaseCheckThinTrafficStillPingsEachIdlePeriod(t *testing.T) {
	f := newCheckFixture()
	require.NoError(t, f.evaluate(), "the first ping decides up")
	require.Equal(t, int32(1), f.pool.pings.Load())

	// The database dies; one failing query every 8 s never reaches min_attempts.
	f.pool.setPingErr(driver.ErrBadConn)
	for i := 0; i < 4; i++ {
		f.clk.Advance(8 * time.Second)
		f.traffic(1, driver.ErrBadConn)
		f.evaluate()
	}
	assert.ErrorIs(t, f.evaluate(), errDatabaseUnreachable)
	pings := f.pool.pings.Load()
	assert.Equal(t, int32(3), pings, "one ping per idle period, not one per evaluation")

	// It recovers; one successful query every 8 s still lets a ping see it.
	f.pool.setPingErr(nil)
	for i := 0; i < 2; i++ {
		f.clk.Advance(8 * time.Second)
		f.traffic(1, nil)
		f.evaluate()
	}
	assert.NoError(t, f.evaluate())
}

// With enough traffic to decide from, the check adds no pings.
func TestDatabaseCheckBusyTrafficAddsNoPings(t *testing.T) {
	f := newCheckFixture()
	for i := 0; i < 4; i++ {
		f.traffic(5, nil)
		require.NoError(t, f.evaluate())
		f.clk.Advance(11 * time.Second)
	}
	assert.Zero(t, f.pool.pings.Load())
}

func TestDatabaseCheckPoolExhaustion(t *testing.T) {
	f := newCheckFixture()
	f.traffic(10, nil)
	require.NoError(t, f.evaluate(), "sets the wait baseline")
	f.pool.setUsage(9, 10)
	f.pool.wait(1)

	assert.NoError(t, f.evaluate(), "saturation has only just started")
	f.clk.Advance(4 * time.Second)
	f.traffic(10, nil)
	f.pool.wait(1)
	assert.NoError(t, f.evaluate(), "a burst shorter than the wait limit")
	f.clk.Advance(time.Second)
	f.pool.wait(1)
	assert.Equal(t, errPoolExhausted, f.evaluate())
	f.pool.wait(1)
	assert.EqualError(t, f.evaluate(), "database connection pool exhausted")

	f.pool.setUsage(8, 10)
	f.pool.wait(1)
	assert.NoError(t, f.evaluate(), "recovered once usage drops below the threshold")
	f.pool.setUsage(10, 10)
	f.pool.wait(1)
	require.NoError(t, f.evaluate())
	f.clk.Advance(4 * time.Second)
	f.pool.wait(1)
	assert.NoError(t, f.evaluate(), "a new saturation starts a new wait")
}

// A busy pool that still serves every caller at once is not exhausted, however long it
// stays busy; once callers stop waiting, the wait starts over.
func TestDatabaseCheckPoolSaturationNeedsWaiters(t *testing.T) {
	f := newCheckFixture()
	f.traffic(10, nil)
	f.pool.setUsage(10, 10)
	f.pool.wait(7) // waits from before the first evaluation are only the baseline
	require.NoError(t, f.evaluate())
	for i := 0; i < 3; i++ {
		f.clk.Advance(3 * time.Second)
		f.traffic(10, nil)
		assert.NoError(t, f.evaluate(), "no caller is waiting")
	}

	f.pool.wait(1)
	require.NoError(t, f.evaluate())
	f.clk.Advance(3 * time.Second)
	f.pool.wait(1)
	require.NoError(t, f.evaluate())
	f.clk.Advance(3 * time.Second)
	assert.NoError(t, f.evaluate(), "no new waiters, so the saturation timer resets")
	f.pool.wait(1)
	assert.NoError(t, f.evaluate(), "a new saturation starts a new wait")
}

func TestSaturationThresholdToleratesFloatError(t *testing.T) {
	assert.Equal(t, 7, saturationThreshold(0.7, 10))
	assert.Equal(t, 8, saturationThreshold(0.71, 10))
	assert.Equal(t, 23, saturationThreshold(0.9, 25))
}

func TestDatabaseCheckPoolRuleSkippedForSingleAndUnlimitedPools(t *testing.T) {
	for _, maxOpen := range []int{0, 1} {
		f := newCheckFixture()
		f.traffic(10, nil)
		f.pool.setUsage(maxOpen, maxOpen)
		require.NoError(t, f.evaluate())
		f.clk.Advance(time.Minute)
		f.traffic(10, nil)
		assert.NoError(t, f.evaluate(), "max open %d", maxOpen)
	}
}

func TestDatabaseCheckPoolRuleKeepsConnectionReason(t *testing.T) {
	f := newCheckFixture()
	f.pool.setUsage(10, 10)
	require.NoError(t, f.evaluate())
	f.pool.wait(1)
	require.NoError(t, f.evaluate())
	f.clk.Advance(6 * time.Second)
	f.pool.wait(1)
	f.traffic(10, driver.ErrBadConn)

	assert.Equal(t, errDatabaseUnreachable, f.evaluate())
}

// With a real pool, the check's own ping marks the shared window's last ping time
// without looking like application traffic or counting as an attempt.
func TestDatabaseCheckOwnPingIsNotTraffic(t *testing.T) {
	clk := newTestClock()
	settings := databaseSettings(2 * time.Second)
	window := observe.NewWindow(settings.FailureWindow, clk.Now)
	db, err := storage.Open(storage.BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"),
		Window: window}, discardLogger())
	require.NoError(t, err)
	defer db.Close()
	clk.Advance(time.Minute)
	check := newDatabaseCheck(db.GetDB(), window, settings, clk.Now)

	before := window.Snapshot(clk.Now())
	require.NoError(t, check(context.Background()))
	after := window.Snapshot(clk.Now())

	assert.Equal(t, before.Attempts, after.Attempts)
	assert.Equal(t, before.LastActivity, after.LastActivity)
	assert.Equal(t, clk.Now(), after.LastPing)

	clk.Advance(3 * time.Second)
	require.NoError(t, check(context.Background()))
	assert.Equal(t, after.LastPing, window.Snapshot(clk.Now()).LastPing, "no second ping within the idle period")
}

// netTimeout is a network error that reports a timeout.
type netTimeout struct{}

func (*netTimeout) Error() string   { return "i/o timeout" }
func (*netTimeout) Timeout() bool   { return true }
func (*netTimeout) Temporary() bool { return true }
