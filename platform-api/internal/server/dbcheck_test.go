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
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database/observe"
	"github.com/wso2/api-platform/platform-api/internal/health"
)

// pingFunc adapts a function to a database pool whose statistics are empty.
type pingFunc func(ctx context.Context) error

func (f pingFunc) PingContext(ctx context.Context) error { return f(ctx) }

func (pingFunc) Stats() sql.DBStats { return sql.DBStats{} }

// testDatabaseSettings are the default database health settings.
func testDatabaseSettings() config.HealthDatabase {
	return config.HealthDatabase{
		CheckTimeout:   200 * time.Millisecond,
		StartupWait:    time.Minute,
		FailureWindow:  30 * time.Second,
		FailureRatio:   0.5,
		MinAttempts:    5,
		IdlePeriod:     10 * time.Second,
		PoolSaturation: 0.9,
		PoolWaitLimit:  5 * time.Second,
	}
}

// newTestChecker returns a platform-api checker with only the database check, without
// observed traffic, a short timeout and no caching, so every evaluation pings.
func newTestChecker(db databasePool) *health.Checker {
	c := health.NewChecker(health.Options{
		HealthyStatus: "ok",
		CacheTTL:      time.Nanosecond,
	})
	c.Register("database", newDatabaseCheck(db, nil, testDatabaseSettings(), time.Now),
		health.WithTimeout(200*time.Millisecond))
	return c
}

type renderedCheck struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Since  string `json:"since"`
}

type renderedBody struct {
	Status string                   `json:"status"`
	Checks map[string]renderedCheck `json:"checks"`
}

func evaluateDatabase(t *testing.T, c *health.Checker) (renderedBody, string) {
	t.Helper()
	raw, err := health.RenderBody(c.Evaluate())
	if err != nil {
		t.Fatalf("RenderBody: %v", err)
	}
	var b renderedBody
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	return b, string(raw)
}

func TestDatabaseCheckUp(t *testing.T) {
	c := newTestChecker(pingFunc(func(context.Context) error { return nil }))

	b, raw := evaluateDatabase(t, c)

	if b.Status != "ok" || b.Checks["database"].Status != "up" {
		t.Fatalf("got %+v, want ok with database up", b)
	}
	if strings.Contains(raw, `"component"`) {
		t.Errorf("body names the component: %s", raw)
	}
}

func TestDatabaseCheckUnreachableHidesDriverError(t *testing.T) {
	driverErr := errors.New(`failed to connect to host=db.internal user=admin password=s3cret: connection refused`)
	c := newTestChecker(pingFunc(func(context.Context) error { return driverErr }))

	b, raw := evaluateDatabase(t, c)

	if b.Status != "unhealthy" {
		t.Errorf("status = %q, want unhealthy", b.Status)
	}
	db := b.Checks["database"]
	if db.Status != "down" || db.Error != "database unreachable" || db.Since == "" {
		t.Errorf("database check = %+v, want down / database unreachable with since", db)
	}
	for _, leak := range []string{"db.internal", "admin", "s3cret", "connection refused"} {
		if strings.Contains(raw, leak) {
			t.Errorf("body leaks %q: %s", leak, raw)
		}
	}
}

func TestDatabaseCheckKeepsDriverErrorAsCause(t *testing.T) {
	driverErr := errors.New("connection refused")
	check := newDatabaseCheck(pingFunc(func(context.Context) error { return driverErr }), nil, testDatabaseSettings(), time.Now)

	err := check(context.Background())

	if !errors.Is(err, errDatabaseUnreachable) || !errors.Is(err, driverErr) {
		t.Fatalf("err = %v, want both the reason and the driver error wrapped", err)
	}
}

func TestDatabaseCheckTimeout(t *testing.T) {
	c := newTestChecker(pingFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return errors.New("driver: canceling query due to user request")
	}))

	b, _ := evaluateDatabase(t, c)

	if got := b.Checks["database"]; got.Status != "down" || got.Error != "timed out after 200ms" {
		t.Fatalf("database check = %+v, want down / timed out after 200ms", got)
	}
}

func TestHealthRouteReflectsDatabase(t *testing.T) {
	var dbErr error
	c := newTestChecker(pingFunc(func(context.Context) error { return dbErr }))
	mux := http.NewServeMux()
	mux.Handle("GET /health", c.Handler())

	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		return rec
	}

	if rec := get(); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthy database: got %d %s", rec.Code, rec.Body.String())
	}
	dbErr = errors.New("connection refused")
	if rec := get(); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"unhealthy"`) {
		t.Fatalf("database down: got %d %s", rec.Code, rec.Body.String())
	}
	dbErr = nil
	if rec := get(); rec.Code != http.StatusOK {
		t.Fatalf("database back: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestDatabaseCheckDoesNotStackPingsOnHungDatabase(t *testing.T) {
	release := make(chan struct{})
	var pings atomic.Int32
	// A driver that ignores its context: the ping blocks until released.
	check := newDatabaseCheck(pingFunc(func(context.Context) error {
		pings.Add(1)
		<-release
		return nil
	}), nil, testDatabaseSettings(), time.Now)
	call := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return check(ctx)
	}

	if err := call(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first call: err = %v, want a timeout", err)
	}
	if err := call(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second call: err = %v, want a timeout", err)
	}
	if n := pings.Load(); n != 1 {
		t.Fatalf("pings = %d while the first is still running, want 1", n)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := call()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("check never recovered after the hung ping returned: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := pings.Load(); n != 2 {
		t.Fatalf("pings = %d, want exactly one new ping after the hung one returned", n)
	}
}

func TestDatabaseCheckReportsHungPingAsTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	c := newTestChecker(pingFunc(func(context.Context) error {
		<-release
		return nil
	}))

	for i := 0; i < 3; i++ {
		b, _ := evaluateDatabase(t, c)
		if got := b.Checks["database"]; got.Status != "down" || got.Error != "timed out after 200ms" {
			t.Fatalf("evaluation %d: database check = %+v, want down / timed out after 200ms", i, got)
		}
	}
}

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

// fakePool is a database pool whose pings record into a window as the observing driver
// would, and whose statistics a test sets.
type fakePool struct {
	window *observe.Window
	pings  atomic.Int32

	mu      sync.Mutex
	pingErr error
	stats   sql.DBStats
}

func (p *fakePool) PingContext(ctx context.Context) error {
	p.pings.Add(1)
	p.mu.Lock()
	err := p.pingErr
	p.mu.Unlock()
	if p.window != nil {
		p.window.Record(ctx, err)
	}
	return err
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

func (p *fakePool) setStats(maxOpen, inUse int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = sql.DBStats{MaxOpenConnections: maxOpen, InUse: inUse}
}

// trafficFixture is a database check fed by a window of observed traffic on a fake
// clock.
type trafficFixture struct {
	clk    *fakeClock
	window *observe.Window
	pool   *fakePool
	check  health.CheckFunc
}

func newTrafficFixture(settings config.HealthDatabase) *trafficFixture {
	clk := newFakeClock()
	window := observe.NewWindow(settings.FailureWindow, clk.Now)
	pool := &fakePool{window: window}
	return &trafficFixture{clk: clk, window: window, pool: pool,
		check: newDatabaseCheck(pool, window, settings, clk.Now)}
}

// traffic records n application operations with the given outcome.
func (f *trafficFixture) traffic(n int, err error) {
	for range n {
		f.window.Record(context.Background(), err)
	}
}

func (f *trafficFixture) evaluate() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return f.check(ctx)
}

// reason renders an error as the health answer would.
func reason(t *testing.T, err error) string {
	t.Helper()
	c := health.NewChecker(health.Options{HealthyStatus: "ok", CacheTTL: time.Nanosecond})
	c.Register("database", func(context.Context) error { return err }, health.WithTimeout(2*time.Second))
	return c.Evaluate().Checks["database"].Error
}

func TestDatabaseCheckSteadyTrafficMakesNoPing(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())

	for range 20 {
		f.traffic(10, nil)
		if err := f.evaluate(); err != nil {
			t.Fatalf("evaluate with healthy traffic = %v, want up", err)
		}
		f.clk.Advance(3 * time.Second)
	}
	if n := f.pool.pings.Load(); n != 0 {
		t.Fatalf("pings = %d under steady traffic, want 0", n)
	}
}

func TestDatabaseCheckFailureRatioDecidesAtMinAttempts(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.pool.window = nil // keep the window to application traffic only

	f.traffic(2, nil)
	f.traffic(2, driver.ErrBadConn) // 4 attempts: below min_attempts
	if err := f.evaluate(); err != nil {
		t.Fatalf("below min_attempts = %v, want up from the deciding ping", err)
	}
	if n := f.pool.pings.Load(); n != 1 {
		t.Fatalf("pings = %d, want 1 to decide while nothing was decided", n)
	}

	f.traffic(1, driver.ErrBadConn) // 5 attempts, 3 failures: ratio 0.6
	err := f.evaluate()
	if !errors.Is(err, errDatabaseUnreachable) {
		t.Fatalf("5 attempts with 3 failures = %v, want unreachable", err)
	}
	if got := reason(t, err); got != "database unreachable" {
		t.Errorf("reason = %q, want database unreachable", got)
	}
	if n := f.pool.pings.Load(); n != 1 {
		t.Errorf("pings = %d, want no ping once traffic decides", n)
	}
}

func TestDatabaseCheckRatioBelowThresholdIsUp(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.traffic(6, nil)
	f.traffic(4, driver.ErrBadConn) // 0.4 < 0.5
	if err := f.evaluate(); err != nil {
		t.Fatalf("ratio 0.4 = %v, want up", err)
	}
	f.traffic(2, driver.ErrBadConn) // 6 of 12: exactly the ratio
	if err := f.evaluate(); err == nil {
		t.Fatal("ratio 0.5 = up, want down at exactly failure_ratio")
	}
}

func TestDatabaseCheckTimeoutFailuresReportTimeout(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.traffic(5, &net.OpError{Op: "read", Err: os.ErrDeadlineExceeded})

	err := f.evaluate()

	if got := reason(t, err); got != "timed out after 2s" {
		t.Fatalf("reason = %q, want the check's own timeout", got)
	}
}

func TestDatabaseCheckRecoversAsFailuresAgeOut(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.traffic(10, driver.ErrBadConn)
	if err := f.evaluate(); err == nil {
		t.Fatal("all operations failing = up, want down")
	}

	f.clk.Advance(20 * time.Second)
	f.traffic(5, nil) // 10 of 15 failed, all within the window
	if err := f.evaluate(); err == nil {
		t.Fatal("10 failures of 15 = up, want down")
	}

	f.clk.Advance(15 * time.Second) // the failures leave the 30s window
	f.traffic(5, nil)
	if err := f.evaluate(); err != nil {
		t.Fatalf("after the failures aged out = %v, want up", err)
	}
	if n := f.pool.pings.Load(); n != 0 {
		t.Errorf("pings = %d, want none while traffic flows", n)
	}
}

func TestDatabaseCheckIdlePingsOncePerIdlePeriod(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())

	for i := range 10 { // 30s of evaluations every 3s, no traffic
		if err := f.evaluate(); err != nil {
			t.Fatalf("evaluation %d = %v, want up", i, err)
		}
		f.clk.Advance(3 * time.Second)
	}
	// Pings at 0s, 12s and 24s: one per started idle period.
	if n := f.pool.pings.Load(); n != 3 {
		t.Fatalf("pings = %d over 30s idle with a 10s idle period, want 3", n)
	}
	if snap := f.window.Snapshot(f.clk.Now()); !snap.LastActivity.IsZero() {
		t.Error("health pings were recorded as activity")
	}
}

func TestDatabaseCheckIdlePingDetectsOutage(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	if err := f.evaluate(); err != nil {
		t.Fatalf("first evaluation = %v, want up", err)
	}
	f.pool.setPingErr(errors.New("connection refused"))

	f.clk.Advance(3 * time.Second)
	if err := f.evaluate(); err != nil {
		t.Fatalf("within the idle period = %v, want the last decided state (up)", err)
	}
	f.clk.Advance(7 * time.Second)
	err := f.evaluate()
	if !errors.Is(err, errDatabaseUnreachable) {
		t.Fatalf("idle ping against a stopped database = %v, want unreachable", err)
	}
	f.clk.Advance(3 * time.Second)
	if err := f.evaluate(); !errors.Is(err, errDatabaseUnreachable) {
		t.Fatalf("after a failed ping = %v, want the decided down state kept", err)
	}
	if n := f.pool.pings.Load(); n != 2 {
		t.Fatalf("pings = %d, want 2", n)
	}
}

// Recent traffic, even below min_attempts, means no ping: the last decided state holds.
func TestDatabaseCheckRecentActivityKeepsDecidedState(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.pool.setPingErr(errors.New("connection refused"))
	if err := f.evaluate(); err == nil {
		t.Fatal("first evaluation = up, want down from the failed ping")
	}
	f.pool.setPingErr(nil)

	f.clk.Advance(11 * time.Second)
	f.traffic(1, nil)
	if err := f.evaluate(); err == nil {
		t.Fatal("recent activity below min_attempts = up, want the decided down state kept")
	}
	f.clk.Advance(11 * time.Second)
	if err := f.evaluate(); err != nil {
		t.Fatalf("idle again = %v, want up from a new ping", err)
	}
	if n := f.pool.pings.Load(); n != 2 {
		t.Fatalf("pings = %d, want 2", n)
	}
}

// With traffic but nothing decided yet, the check pings once to decide.
func TestDatabaseCheckPingsWhenNothingDecided(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.traffic(2, nil)

	if err := f.evaluate(); err != nil {
		t.Fatalf("evaluate = %v, want up", err)
	}
	if n := f.pool.pings.Load(); n != 1 {
		t.Fatalf("pings = %d, want 1", n)
	}
}

func TestDatabaseCheckQueryErrorsNeverCount(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	for range 10 {
		f.traffic(20, &pgconn.PgError{Code: "23505"})
		f.traffic(5, context.DeadlineExceeded)
		f.traffic(5, sql.ErrNoRows)
		if err := f.evaluate(); err != nil {
			t.Fatalf("query errors only = %v, want up", err)
		}
		f.clk.Advance(3 * time.Second)
	}
}

// A ping still waiting for a pooled connection has recorded nothing, yet no second ping
// starts within the idle period.
func TestDatabaseCheckRateLimitsPingsThatRecordNothing(t *testing.T) {
	clk := newFakeClock()
	window := observe.NewWindow(30*time.Second, clk.Now)
	var pings atomic.Int32
	pool := pingFunc(func(ctx context.Context) error {
		pings.Add(1)
		<-ctx.Done()
		return ctx.Err()
	})
	check := newDatabaseCheck(pool, window, testDatabaseSettings(), clk.Now)
	run := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		return check(ctx)
	}

	if err := run(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first evaluation = %v, want a timeout", err)
	}
	clk.Advance(3 * time.Second)
	if err := run(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second evaluation = %v, want the decided timeout kept", err)
	}
	if n := pings.Load(); n != 1 {
		t.Fatalf("pings = %d, want 1 within the idle period", n)
	}
}

// With no traffic at all, the startup wait still pings and completes once the database
// answers.
func TestDatabaseCheckStartupWaitWithoutTraffic(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.pool.setPingErr(errors.New("connection refused"))
	c := health.NewChecker(health.Options{HealthyStatus: "ok", CacheTTL: time.Nanosecond})
	c.Register("database", f.check, health.WithTimeout(time.Second))
	evaluations := 0
	w := newDatabaseWait(func() health.Result {
		evaluations++
		if evaluations == 3 {
			f.pool.setPingErr(nil)
		}
		return c.Evaluate()
	}, time.Minute, nil, discardLogger())
	w.now = f.clk.Now
	w.after = func(d time.Duration) <-chan time.Time {
		f.clk.Advance(d)
		ch := make(chan time.Time, 1)
		ch <- f.clk.Now()
		return ch
	}

	if err := w.run(); err != nil {
		t.Fatalf("startup wait = %v, want nil once the database answers", err)
	}
	if n := f.pool.pings.Load(); n < 2 {
		t.Fatalf("pings = %d, want the wait to keep pinging with no traffic", n)
	}
}

func TestDatabaseCheckPoolExhaustion(t *testing.T) {
	settings := testDatabaseSettings()
	f := newTrafficFixture(settings)
	f.traffic(10, nil)
	f.pool.setStats(10, 9) // ceil(0.9 × 10) = 9: saturated

	if err := f.evaluate(); err != nil {
		t.Fatalf("saturation just began = %v, want up", err)
	}
	f.clk.Advance(3 * time.Second)
	f.traffic(10, nil)
	if err := f.evaluate(); err != nil {
		t.Fatalf("burst shorter than pool_wait_limit = %v, want up", err)
	}
	f.clk.Advance(2 * time.Second)
	f.traffic(10, nil)
	err := f.evaluate()
	if !errors.Is(err, errPoolExhausted) {
		t.Fatalf("saturated for pool_wait_limit = %v, want pool exhausted", err)
	}
	if got := reason(t, err); got != "database connection pool exhausted" {
		t.Errorf("reason = %q", got)
	}

	f.pool.setStats(10, 8)
	f.clk.Advance(time.Second)
	f.traffic(10, nil)
	if err := f.evaluate(); err != nil {
		t.Fatalf("pool below saturation = %v, want up", err)
	}
	f.pool.setStats(10, 10)
	f.clk.Advance(time.Second)
	f.traffic(10, nil)
	if err := f.evaluate(); err != nil {
		t.Fatalf("new saturation = %v, want up until it lasts pool_wait_limit", err)
	}
}

// A short burst that ends before pool_wait_limit restarts the clock.
func TestDatabaseCheckPoolBurstResets(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.traffic(10, nil)
	for i := range 6 {
		inUse := 10
		if i%2 == 1 {
			inUse = 2
		}
		f.pool.setStats(10, inUse)
		f.traffic(10, nil)
		if err := f.evaluate(); err != nil {
			t.Fatalf("evaluation %d = %v, want up for bursts shorter than the limit", i, err)
		}
		f.clk.Advance(3 * time.Second)
	}
}

// Single-connection (SQLite) and unlimited pools skip the pool rule.
func TestDatabaseCheckPoolRuleSkipped(t *testing.T) {
	for _, maxOpen := range []int{0, 1} {
		f := newTrafficFixture(testDatabaseSettings())
		f.pool.setStats(maxOpen, 1)
		for range 5 {
			f.traffic(10, nil)
			if err := f.evaluate(); err != nil {
				t.Fatalf("max open %d = %v, want the pool rule skipped", maxOpen, err)
			}
			f.clk.Advance(10 * time.Second)
		}
	}
}

// A database that is down keeps its own reason even while the pool is saturated.
func TestDatabaseCheckDownReasonWinsOverPool(t *testing.T) {
	f := newTrafficFixture(testDatabaseSettings())
	f.pool.setStats(10, 10)
	f.traffic(10, driver.ErrBadConn)
	f.evaluate()
	f.clk.Advance(6 * time.Second)
	f.traffic(10, driver.ErrBadConn)
	if err := f.evaluate(); !errors.Is(err, errDatabaseUnreachable) {
		t.Fatalf("err = %v, want unreachable", err)
	}
}

func TestSaturationThreshold(t *testing.T) {
	for _, tt := range []struct {
		ratio   float64
		maxOpen int
		want    int
	}{
		{0.9, 10, 9},
		{0.9, 25, 23},
		{0.7, 10, 7},
		{1, 4, 4},
		{0.5, 3, 2},
		{0.1, 2, 1},
	} {
		if got := saturationThreshold(tt.ratio, tt.maxOpen); got != tt.want {
			t.Errorf("saturationThreshold(%v, %d) = %d, want %d", tt.ratio, tt.maxOpen, got, tt.want)
		}
	}
}
