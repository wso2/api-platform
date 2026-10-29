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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/adminserver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage/observe"
)

const (
	// startupRetryInitial is the delay before the database is checked again during the
	// startup wait; it doubles after every failed check up to startupRetryMax.
	startupRetryInitial = time.Second
	// startupRetryMax caps the delay between database checks during the startup wait,
	// which bounds how long readiness lags behind the database coming back.
	startupRetryMax = 10 * time.Second
)

var (
	// errStartupWaitExpired is returned when the database is still unreachable at the
	// end of the startup wait.
	errStartupWaitExpired = errors.New("database not reachable within the startup wait")
	// errStartupInterrupted is returned when a shutdown signal arrives while startup is
	// still waiting for the database.
	errStartupInterrupted = errors.New("shutdown signal received during startup")
)

// errDatabaseUnreachable is the reason reported when the database answers a ping with an
// error. Health answers are served without authentication, so the driver error itself,
// which can name hosts, users or DSN fragments, is kept only as the wrapped cause.
var errDatabaseUnreachable = health.Reason("database unreachable")

// errStarting is the startup check's reason until every server has started.
var errStarting = health.Reason("starting")

// startupGate keeps health down until the controller has started every server, so a
// probe never sees it healthy while its APIs cannot yet be reached.
type startupGate struct {
	started atomic.Bool
}

// check is the startup health check. It is registered unlogged: it fails on every
// start, so its failure is not news.
func (g *startupGate) check(context.Context) error {
	if g.started.Load() {
		return nil
	}
	return errStarting
}

// markStarted ends the startup phase; health then depends on the other checks alone.
func (g *startupGate) markStarted() {
	g.started.Store(true)
}

// databasePool is the part of the database pool the health check needs.
type databasePool interface {
	PingContext(ctx context.Context) error
	Stats() sql.DBStats
}

// errPoolExhausted is the reason reported when the connection pool has stayed saturated
// for longer than the configured limit.
var errPoolExhausted = health.Reason("database connection pool exhausted")

// databaseCheck is the health check of the controller's storage database. It decides
// from the outcomes of the controller's own database operations, recorded in window by
// every pool, so it adds no database calls while traffic flows:
//
//  1. With at least min_attempts operations in the window, the database is down when
//     the share of connection failures reaches failure_ratio.
//  2. With no application activity and no own ping within idle_period, the check pings
//     the database once, bounded by check_timeout, and the ping decides.
//  3. Otherwise the last decided state stands; with none yet, the check pings.
//
// A database that is up is still reported down once the main pool has stayed saturated
// for pool_wait_limit.
type databaseCheck struct {
	pool     databasePool
	window   *observe.Window
	settings config.HealthDatabaseConfig
	now      func() time.Time
	// timeoutReason is reported when the deciding failure was a timeout.
	timeoutReason health.Reason
	// pinging is set while an own ping is in flight. A driver that ignores its context
	// could otherwise leave one ping per evaluation stuck on the database, so probes
	// against a hung database would pile up pooled connections; while an earlier ping is
	// still running, the check reports a timeout without starting another.
	pinging atomic.Bool

	mu sync.Mutex
	// decided is the outcome of the last decision; hasDecided is false until one is made.
	decided    error
	hasDecided bool
	// lastPing is when the check last started a ping of its own.
	lastPing time.Time
	// saturatedSince is when the pool became saturated; zero while it is not.
	saturatedSince time.Time
}

// newDatabaseCheck returns the database health check for pool, deciding from window
// with settings. now supplies the current time.
func newDatabaseCheck(pool databasePool, window *observe.Window, settings config.HealthDatabaseConfig,
	now func() time.Time) health.CheckFunc {
	c := &databaseCheck{
		pool:          pool,
		window:        window,
		settings:      settings,
		now:           now,
		timeoutReason: health.Reason(fmt.Sprintf("timed out after %s", settings.CheckTimeout)),
	}
	return c.check
}

func (c *databaseCheck) check(ctx context.Context) error {
	err := c.decide(ctx)
	if c.poolExhausted() && err == nil {
		return errPoolExhausted
	}
	return err
}

// decide applies the traffic rules and returns nil when the database is up.
func (c *databaseCheck) decide(ctx context.Context) error {
	now := c.now()
	snap := c.window.Snapshot(now)
	s := c.settings

	c.mu.Lock()
	if snap.Attempts >= s.MinAttempts {
		var err error
		if float64(snap.Failures)/float64(snap.Attempts) >= s.FailureRatio {
			err = c.failureReason(snap.LastFailure)
		}
		c.decided, c.hasDecided = err, true
		c.mu.Unlock()
		return err
	}
	lastPing := snap.LastPing
	if c.lastPing.After(lastPing) {
		lastPing = c.lastPing
	}
	idle := !recent(snap.LastActivity, now, s.IdlePeriod) && !recent(lastPing, now, s.IdlePeriod)
	if !idle && c.hasDecided {
		err := c.decided
		c.mu.Unlock()
		return err
	}
	c.lastPing = now
	c.mu.Unlock()

	err := c.ping(ctx)
	c.mu.Lock()
	c.decided, c.hasDecided = err, true
	c.mu.Unlock()
	return err
}

// failureReason returns the reason for a database found down from its traffic.
func (c *databaseCheck) failureReason(kind observe.FailureKind) error {
	if kind == observe.Timeout {
		return c.timeoutReason
	}
	return errDatabaseUnreachable
}

// recent reports whether t lies within d before now.
func recent(t, now time.Time, d time.Duration) bool {
	return !t.IsZero() && now.Sub(t) < d
}

// ping runs one ping of the check's own, marked so it is not taken for application
// activity. A ping cut short by the check deadline is reported as a timeout; any other
// ping error as the database being unreachable, with the driver error kept only as the
// wrapped cause.
func (c *databaseCheck) ping(ctx context.Context) error {
	if !c.pinging.CompareAndSwap(false, true) {
		return context.DeadlineExceeded
	}
	// Buffered, so the ping goroutine never blocks once the check has given up.
	done := make(chan error, 1)
	go func() {
		err := c.pool.PingContext(observe.WithHealthPing(ctx))
		// Cleared before the result is handed over, so the next evaluation after this
		// one can ping again.
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

// poolExhausted reports whether the pool has stayed saturated for at least
// pool_wait_limit. The pool is saturated when at least pool_saturation of its maximum
// connections are in use; pools limited to one connection, or not limited at all, are
// never considered saturated.
func (c *databaseCheck) poolExhausted() bool {
	stats := c.pool.Stats()
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	limit := stats.MaxOpenConnections
	if limit <= 1 || stats.InUse < int(math.Ceil(c.settings.PoolSaturation*float64(limit))) {
		c.saturatedSince = time.Time{}
		return false
	}
	if c.saturatedSince.IsZero() {
		c.saturatedSince = now
	}
	return now.Sub(c.saturatedSince) >= c.settings.PoolWaitLimit
}

// newHealthChecker builds the checker behind the admin health endpoints. It reports check
// failures and recoveries to the controller's log, and answers only the clients in
// controller.health.allowed_clients.
func newHealthChecker(cfg *config.Config, log *slog.Logger) (*health.Checker, error) {
	hc := cfg.Controller.Health
	allowed, err := health.ParseAllowedClients("controller.health", hc.AllowedClients)
	if err != nil {
		return nil, err
	}
	return health.NewChecker(health.Options{
		HealthyStatus:  "healthy",
		CacheTTL:       hc.CacheTTL,
		Logger:         log,
		DebugLogger:    log,
		AllowedClients: allowed,
	}), nil
}

// databaseWait waits at startup for the database health check to pass, retrying with
// backoff, for at most wait. Only the database check counts: the startup check stays
// down until startup has finished, so it cannot be part of the wait.
type databaseWait struct {
	evaluate func() health.Result
	wait     time.Duration
	// stop delivers a shutdown signal; receiving one ends the wait.
	stop  <-chan os.Signal
	log   *slog.Logger
	now   func() time.Time
	after func(time.Duration) <-chan time.Time
}

func newDatabaseWait(evaluate func() health.Result, wait time.Duration, stop <-chan os.Signal,
	log *slog.Logger) *databaseWait {
	return &databaseWait{
		evaluate: evaluate,
		wait:     wait,
		stop:     stop,
		log:      log,
		now:      time.Now,
		after:    time.After,
	}
}

// run returns nil once the database is reachable, errStartupWaitExpired when it is
// still unreachable after the wait, and errStartupInterrupted on a shutdown signal.
// The health checker logs the database check once when it fails and once when it
// recovers, so the retries here add no log lines of their own.
func (w *databaseWait) run() error {
	if w.databaseUp() {
		return nil
	}
	w.log.Info("waiting for the database before completing startup", slog.Duration("startup_wait", w.wait))

	deadline := w.now().Add(w.wait)
	interval := startupRetryInitial
	for {
		remaining := deadline.Sub(w.now())
		if remaining <= 0 {
			w.log.Error(errStartupWaitExpired.Error(), slog.Duration("startup_wait", w.wait))
			return errStartupWaitExpired
		}
		select {
		case <-w.stop:
			return errStartupInterrupted
		case <-w.after(min(interval, remaining)):
		}
		if w.databaseUp() {
			return nil
		}
		interval = min(2*interval, startupRetryMax)
	}
}

// databaseUp evaluates health and reports whether the database check passed.
func (w *databaseWait) databaseUp() bool {
	return w.evaluate().Checks["database"].Status == health.StatusUp
}

// serveHealthWhileWaiting registers the database check on db, deciding from the
// operation outcomes recorded in window, and the startup check on gate, starts the admin server so health is served at once, and waits for the database
// to answer. Health stays down until the caller marks gate started. It returns the
// admin server (nil when disabled) and ok once the database is reachable. When the wait
// ends without the database it has already released db and the admin server, and
// returns the exit code the controller must stop with.
func serveHealthWhileWaiting(cfg *config.Config, log *slog.Logger, checker *health.Checker,
	db storage.Storage, window *observe.Window, gate *startupGate, authMiddleware func(http.Handler) http.Handler,
	quit <-chan os.Signal) (*adminserver.Server, int, bool) {
	settings := cfg.Controller.Health.Database
	checker.Register("database", newDatabaseCheck(db.GetDB(), window, settings, time.Now),
		health.WithTimeout(settings.CheckTimeout))
	checker.RegisterUnlogged("startup", gate.check)
	// Serve health before waiting, so probes get a real answer (503) during the wait
	// instead of a refused connection.
	admin := startAdminServer(cfg, checker.Handler(), authMiddleware, log)
	if err := newDatabaseWait(checker.Evaluate, settings.StartupWait, quit, log).run(); err != nil {
		return nil, endStartupWithoutDatabase(err, admin, cfg.Controller.Server.ShutdownTimeout, log, db), false
	}
	return admin, 0, true
}

// startAdminServer starts the controller admin server, when enabled, serving the health
// probe at once; its debug endpoints become available once SetAPIServer is called. It
// returns nil when the admin server is disabled.
func startAdminServer(cfg *config.Config, healthHandler http.Handler,
	authMiddleware func(http.Handler) http.Handler, log *slog.Logger) *adminserver.Server {
	// Enable block/mutex profiling sampling when pprof is enabled. These are the
	// only profiles that need explicit rate setup; 0 leaves them disabled. Gated so
	// the sampling overhead is never paid unless pprof is deliberately turned on.
	if cfg.Controller.AdminServer.Pprof.Enabled {
		runtime.SetBlockProfileRate(cfg.Controller.AdminServer.Pprof.BlockProfileRate)
		runtime.SetMutexProfileFraction(cfg.Controller.AdminServer.Pprof.MutexProfileFraction)
	}
	if !cfg.Controller.AdminServer.Enabled {
		return nil
	}

	// Gate the admin/debug server behind the same authentication middleware as
	// the management API, plus a deny-by-default admin-role authorization check.
	// The health probe stays public; every other admin
	// endpoint (config_dump, xds_sync_status, pprof) requires an authenticated
	// caller from controller.auth.basic who also holds the "admin" role.
	// Authentication runs first (outermost) to populate the auth context, then
	// authorization consumes it — the same ordering the management API uses.
	adminAuthz := authenticators.AuthorizationMiddleware(
		commonmodels.AuthConfig{ResourceRoles: adminResourceRoles()}, log)
	adminProtect := func(next http.Handler) http.Handler {
		return authMiddleware(adminAuthz(next))
	}
	server := adminserver.NewServer(&cfg.Controller.AdminServer, healthHandler, adminProtect, log)
	go func() {
		if err := server.Start(); err != nil {
			log.Error("Controller admin server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()
	return server
}

// endStartupWithoutDatabase releases what startup acquired before the database wait
// ended without the database, and returns the process exit code: 0 when a shutdown
// signal ended the wait, 1 when the wait expired.
func endStartupWithoutDatabase(waitErr error, admin *adminserver.Server, shutdownTimeout time.Duration,
	log *slog.Logger, closers ...io.Closer) int {
	interrupted := errors.Is(waitErr, errStartupInterrupted)
	if interrupted {
		log.Info("Received shutdown signal during startup")
	}
	if admin != nil {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := admin.Stop(ctx); err != nil {
			log.Error("Failed to stop controller admin server", slog.Any("error", err))
		}
	}
	for _, c := range closers {
		_ = c.Close()
	}
	if interrupted {
		return 0
	}
	return 1
}
