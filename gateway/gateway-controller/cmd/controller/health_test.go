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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// pingFunc adapts a function to the pinger interface.
type pingFunc func(ctx context.Context) error

func (f pingFunc) PingContext(ctx context.Context) error { return f(ctx) }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestChecker returns a controller checker with only the database check, a short
// timeout and no caching, so every evaluation pings.
func newTestChecker(db pinger) *health.Checker {
	c := health.NewChecker(health.Options{
		Component:     "gateway-controller",
		HealthyStatus: "healthy",
		CheckTimeout:  200 * time.Millisecond,
		CacheTTL:      time.Nanosecond,
	})
	c.Register("database", newDatabaseCheck(db))
	return c
}

type renderedCheck struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Since  string `json:"since"`
}

type renderedBody struct {
	Status    string                   `json:"status"`
	Component string                   `json:"component"`
	Checks    map[string]renderedCheck `json:"checks"`
}

func evaluate(t *testing.T, c *health.Checker) (renderedBody, string) {
	t.Helper()
	raw, err := health.RenderBody(c.Evaluate())
	require.NoError(t, err)
	var b renderedBody
	require.NoError(t, json.Unmarshal(raw, &b))
	return b, string(raw)
}

func TestDatabaseCheckUp(t *testing.T) {
	b, _ := evaluate(t, newTestChecker(pingFunc(func(context.Context) error { return nil })))

	assert.Equal(t, "healthy", b.Status)
	assert.Equal(t, "gateway-controller", b.Component)
	assert.Equal(t, "up", b.Checks["database"].Status)
}

func TestDatabaseCheckUnreachableHidesDriverError(t *testing.T) {
	driverErr := errors.New("failed to connect to host=gw-db.internal user=gateway password=s3cret: connection refused")

	b, raw := evaluate(t, newTestChecker(pingFunc(func(context.Context) error { return driverErr })))

	assert.Equal(t, "unhealthy", b.Status)
	db := b.Checks["database"]
	assert.Equal(t, "down", db.Status)
	assert.Equal(t, "database unreachable", db.Error)
	assert.NotEmpty(t, db.Since)
	for _, leak := range []string{"gw-db.internal", "gateway password", "s3cret", "connection refused"} {
		assert.NotContains(t, raw, leak)
	}
}

func TestDatabaseCheckKeepsDriverErrorAsCause(t *testing.T) {
	driverErr := errors.New("connection refused")

	err := newDatabaseCheck(pingFunc(func(context.Context) error { return driverErr }))(context.Background())

	assert.ErrorIs(t, err, errDatabaseUnreachable)
	assert.ErrorIs(t, err, driverErr)
}

func TestDatabaseCheckTimeout(t *testing.T) {
	b, _ := evaluate(t, newTestChecker(pingFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return errors.New("driver: canceling query due to user request")
	})))

	assert.Equal(t, renderedCheck{Status: "down", Error: "timed out after 200ms", Since: b.Checks["database"].Since},
		b.Checks["database"])
}

func TestNewHealthChecker(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "logs", "health.log")
	cfg := &config.Config{}
	cfg.Controller.Logging = config.LoggingConfig{Level: "info", Format: "json"}
	cfg.Controller.Health = config.HealthConfig{
		CheckTimeout:   time.Second,
		CacheTTL:       time.Second,
		LogFile:        logFile,
		LogMaxSizeMB:   1,
		LogMaxFiles:    1,
		BackoffInitial: 10 * time.Second,
		BackoffMax:     time.Minute,
	}

	checker, closer := newHealthChecker(cfg, discardLogger())
	defer closer.Close()
	checker.Register("database", newDatabaseCheck(pingFunc(func(context.Context) error {
		return errors.New("connection refused")
	})))

	b, _ := evaluate(t, checker)
	assert.Equal(t, "gateway-controller", b.Component)
	assert.Equal(t, "unhealthy", b.Status)
	require.NoError(t, closer.Close())
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(content), "health check failed"),
		"the failure must be written to the health log file: %q", content)
}

func TestDatabaseCheckDoesNotStackPingsOnHungDatabase(t *testing.T) {
	release := make(chan struct{})
	var pings atomic.Int32
	// A driver that ignores its context: the ping blocks until released.
	check := newDatabaseCheck(pingFunc(func(context.Context) error {
		pings.Add(1)
		<-release
		return nil
	}))
	call := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return check(ctx)
	}

	assert.ErrorIs(t, call(), context.DeadlineExceeded, "first call")
	assert.ErrorIs(t, call(), context.DeadlineExceeded, "second call while the first ping hangs")
	assert.Equal(t, int32(1), pings.Load(), "no second ping while the first is still running")

	close(release)
	require.Eventually(t, func() bool { return call() == nil }, 2*time.Second, 10*time.Millisecond,
		"the check must recover once the hung ping returns")
	assert.Equal(t, int32(2), pings.Load(), "exactly one new ping after the hung one returned")
}

func TestDatabaseCheckReportsHungPingAsTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	c := newTestChecker(pingFunc(func(context.Context) error {
		<-release
		return nil
	}))

	for i := 0; i < 3; i++ {
		b, _ := evaluate(t, c)
		assert.Equal(t, "down", b.Checks["database"].Status, "evaluation %d", i)
		assert.Equal(t, "timed out after 200ms", b.Checks["database"].Error, "evaluation %d", i)
	}
}

// fakeWait drives a databaseWait with a scripted health sequence and a fake clock that
// advances by each requested delay.
type fakeWait struct {
	healthy []bool
	calls   int
	clock   time.Time
	delays  []time.Duration
	logs    strings.Builder
}

func (f *fakeWait) newWait(wait time.Duration, stop <-chan os.Signal) *databaseWait {
	f.clock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := newDatabaseWait(func() health.Result {
		ok := f.calls < len(f.healthy) && f.healthy[f.calls]
		f.calls++
		return databaseResult(ok)
	}, wait, stop, slog.New(slog.NewTextHandler(&f.logs, nil)))
	w.now = func() time.Time { return f.clock }
	w.after = func(d time.Duration) <-chan time.Time {
		f.delays = append(f.delays, d)
		f.clock = f.clock.Add(d)
		ch := make(chan time.Time, 1)
		ch <- f.clock
		return ch
	}
	return w
}

// databaseResult is a health result whose database check is up or down, with the
// startup check always down, as during a real startup wait.
func databaseResult(up bool) health.Result {
	db := health.CheckResult{Status: health.StatusDown}
	if up {
		db = health.CheckResult{Status: health.StatusUp}
	}
	return health.Result{Checks: map[string]health.CheckResult{
		"database": db,
		"startup":  {Status: health.StatusDown, Error: "starting"},
	}}
}

func TestStartupGate(t *testing.T) {
	gate := &startupGate{}

	assert.ErrorIs(t, gate.check(context.Background()), errStarting)
	gate.markStarted()
	assert.NoError(t, gate.check(context.Background()))
}

func TestDatabaseWaitHealthyAtOnce(t *testing.T) {
	f := &fakeWait{healthy: []bool{true}}

	require.NoError(t, f.newWait(5*time.Minute, nil).run())
	assert.Empty(t, f.delays)
	assert.Empty(t, f.logs.String(), "no wait, so no log line")
}

func TestDatabaseWaitRetriesWithBackoff(t *testing.T) {
	f := &fakeWait{healthy: []bool{false, false, false, true}}

	require.NoError(t, f.newWait(5*time.Minute, nil).run())
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, f.delays)
	assert.Contains(t, f.logs.String(), `msg="waiting for the database before completing startup" startup_wait=5m0s`)
}

func TestDatabaseWaitExpires(t *testing.T) {
	f := &fakeWait{}

	err := f.newWait(time.Minute, nil).run()

	assert.ErrorIs(t, err, errStartupWaitExpired)
	s := time.Second
	want := []time.Duration{s, 2 * s, 4 * s, 8 * s, 10 * s, 10 * s, 10 * s, 10 * s, 5 * s}
	assert.Equal(t, want, f.delays, "doubling, capped, then the remainder of the wait")
	assert.Equal(t, len(want)+1, f.calls, "evaluated once more after the last delay")
	assert.Contains(t, f.logs.String(), `level=ERROR msg="database not reachable within the startup wait" startup_wait=1m0s`)
}

func TestDatabaseWaitStopsOnSignal(t *testing.T) {
	f := &fakeWait{}
	stop := make(chan os.Signal, 1)
	stop <- os.Interrupt
	w := f.newWait(time.Minute, stop)
	w.after = func(time.Duration) <-chan time.Time { return nil } // never fires

	assert.ErrorIs(t, w.run(), errStartupInterrupted)
}

func TestDatabaseWaitUsesRealClock(t *testing.T) {
	w := newDatabaseWait(func() health.Result { return health.Result{} }, 30*time.Millisecond, nil, discardLogger())

	start := time.Now()
	err := w.run()

	assert.ErrorIs(t, err, errStartupWaitExpired)
	assert.WithinDuration(t, start.Add(30*time.Millisecond), time.Now(), 2*time.Second)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

// rejectAll stands in for the authentication middleware: no request is authenticated.
func rejectAll(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
}

func TestStartAdminServerServesHealthBeforeStartupCompletes(t *testing.T) {
	cfg := &config.Config{}
	cfg.Controller.AdminServer = config.AdminServerConfig{Enabled: true, Port: freePort(t), AllowedIPs: []string{"*"}}
	checker := newTestChecker(pingFunc(func(context.Context) error { return errors.New("connection refused") }))

	server := startAdminServer(cfg, checker.Handler(), rejectAll, discardLogger())
	require.NotNil(t, server)
	defer endStartupWithoutDatabase(errStartupInterrupted, server, time.Second, discardLogger())

	base := fmt.Sprintf("http://127.0.0.1:%d", cfg.Controller.AdminServer.Port)
	var resp *http.Response
	require.Eventually(t, func() bool {
		var err error
		resp, err = http.Get(base + "/api/admin/v1/health")
		return err == nil
	}, 2*time.Second, 20*time.Millisecond, "admin server must come up")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Contains(t, string(body), `"database":{"status":"down"`)

	resp, err := http.Get(base + "/api/admin/v1/xds_sync_status")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "debug endpoints stay protected while starting")
}

func TestStartAdminServerDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Controller.AdminServer = config.AdminServerConfig{Enabled: false}

	assert.Nil(t, startAdminServer(cfg, http.NotFoundHandler(), rejectAll, discardLogger()))
}

type recordingCloser struct{ closed bool }

func (c *recordingCloser) Close() error {
	c.closed = true
	return nil
}

func TestEndStartupWithoutDatabase(t *testing.T) {
	for _, tt := range []struct {
		name     string
		err      error
		wantCode int
		wantLog  string
	}{
		{"shutdown signal", errStartupInterrupted, 0, "Received shutdown signal during startup"},
		{"wait expired", errStartupWaitExpired, 1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs strings.Builder
			db, healthLog := &recordingCloser{}, &recordingCloser{}

			code := endStartupWithoutDatabase(tt.err, nil, time.Second, slog.New(slog.NewTextHandler(&logs, nil)), db, healthLog)

			assert.Equal(t, tt.wantCode, code)
			assert.True(t, db.closed && healthLog.closed, "everything acquired before the wait must be released")
			if tt.wantLog != "" {
				assert.Contains(t, logs.String(), tt.wantLog)
			}
		})
	}
}

// waitConfig returns a configuration with the admin server on a free port and short
// health timings, so a wait for an unreachable database ends quickly.
func waitConfig(t *testing.T, startupWait time.Duration) *config.Config {
	cfg := &config.Config{}
	cfg.Controller.AdminServer = config.AdminServerConfig{Enabled: true, Port: freePort(t), AllowedIPs: []string{"*"}}
	cfg.Controller.Server.ShutdownTimeout = time.Second
	cfg.Controller.Health = config.HealthConfig{CheckTimeout: 100 * time.Millisecond, CacheTTL: time.Nanosecond,
		LogMaxSizeMB: 1, LogMaxFiles: 1, BackoffInitial: 10 * time.Second, BackoffMax: time.Minute, StartupWait: startupWait}
	return cfg
}

func newWaitChecker(cfg *config.Config) *health.Checker {
	return health.NewChecker(health.Options{
		Component: "gateway-controller", HealthyStatus: "healthy",
		CheckTimeout: cfg.Controller.Health.CheckTimeout, CacheTTL: cfg.Controller.Health.CacheTTL,
	})
}

func TestServeHealthWhileWaitingWithReachableDatabase(t *testing.T) {
	cfg := waitConfig(t, time.Minute)
	db, err := storage.Open(storage.BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db")}, discardLogger())
	require.NoError(t, err)
	defer db.Close()
	var healthLogs strings.Builder
	checker := health.NewChecker(health.Options{
		Component: "gateway-controller", HealthyStatus: "healthy",
		CheckTimeout: cfg.Controller.Health.CheckTimeout, CacheTTL: cfg.Controller.Health.CacheTTL,
		Logger:         slog.New(slog.NewTextHandler(&healthLogs, nil)),
		BackoffInitial: cfg.Controller.Health.BackoffInitial, BackoffMax: cfg.Controller.Health.BackoffMax,
	})
	gate := &startupGate{}

	admin, code, ok := serveHealthWhileWaiting(cfg, discardLogger(), checker, &recordingCloser{}, db, gate, rejectAll, nil)

	require.True(t, ok, "the wait ends on the database alone, while the startup check is still down")
	assert.Equal(t, 0, code)
	require.NotNil(t, admin)
	defer endStartupWithoutDatabase(errStartupInterrupted, admin, time.Second, discardLogger())

	url := fmt.Sprintf("http://127.0.0.1:%d/api/admin/v1/health", cfg.Controller.AdminServer.Port)
	var resp *http.Response
	require.Eventually(t, func() bool {
		var err error
		resp, err = http.Get(url)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond, "admin server must come up")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "not healthy before every server has started")
	assert.Contains(t, string(body), `"database":{"status":"up"}`)
	assert.Contains(t, string(body), `"startup":{"status":"down","error":"starting"`)

	gate.markStarted()
	resp, err = http.Get(url)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `"startup":{"status":"up"}`)
	assert.NotContains(t, healthLogs.String(), "startup", "the startup check is never logged")
}

func TestServeHealthWhileWaitingGivesUp(t *testing.T) {
	cfg := waitConfig(t, 300*time.Millisecond)
	db, err := storage.Open(storage.BackendConfig{Type: "postgres", Postgres: storage.PostgresConnectionConfig{
		Host: "127.0.0.1", Port: freePort(t), Database: "gateway", User: "gateway", SSLMode: "disable",
	}}, discardLogger())
	require.NoError(t, err)
	healthLog := &recordingCloser{}

	admin, code, ok := serveHealthWhileWaiting(cfg, discardLogger(), newWaitChecker(cfg), healthLog, db, &startupGate{},
		rejectAll, nil)

	assert.False(t, ok)
	assert.Equal(t, 1, code)
	assert.Nil(t, admin)
	assert.True(t, healthLog.closed)
	assert.Error(t, db.GetDB().Ping(), "the database pool must be closed")
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Controller.AdminServer.Port))
	require.NoError(t, err, "the admin server port must be released")
	ln.Close()
}
