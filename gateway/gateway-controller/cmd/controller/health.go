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
	"net"
	"net/http"
	"os"
	"regexp"
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
	// errStartupDatabaseRejected is returned when the database answers during the
	// startup wait but rejects the connection in a way retrying cannot fix, such as
	// wrong credentials or a database that does not exist.
	errStartupDatabaseRejected = errors.New("database rejected the connection; check the database settings")
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

// errStopping is the startup check's reason once shutdown has begun.
var errStopping = health.Reason("stopping")

// startupGate keeps health down until the controller has started every server, so a
// probe never sees it healthy while its APIs cannot yet be reached, and again once
// shutdown has begun, so load balancers stop sending it requests while it stops.
type startupGate struct {
	started  atomic.Bool
	stopping atomic.Bool
	// invalidate drops the health checker's cached result. A result cached during the
	// startup phase reports startup as down, and would otherwise outlive it by up to the
	// cache TTL.
	invalidate func()
}

// check is the startup health check. It is registered unlogged: it fails on every
// start, so its failure is not news.
func (g *startupGate) check(context.Context) error {
	if g.stopping.Load() {
		return errStopping
	}
	if g.started.Load() {
		return nil
	}
	return errStarting
}

// markStarted ends the startup phase; health then depends on the other checks alone.
func (g *startupGate) markStarted() {
	g.started.Store(true)
	if g.invalidate != nil {
		g.invalidate()
	}
}

// markStopping reports the controller down from now on, without waiting out the
// health checker's cached result.
func (g *startupGate) markStopping() {
	g.stopping.Store(true)
	if g.invalidate != nil {
		g.invalidate()
	}
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
//  2. With fewer operations than that, the check pings the database once per
//     idle_period, bounded by check_timeout, and the ping decides. It pings even while
//     the controller uses the database, since traffic too thin for rule 1 cannot show
//     an outage or a recovery, and a decision left standing would never expire.
//  3. Between those pings the last decided state stands; with none yet, the check pings.
//
// A database that is up is still reported down once the main pool has stayed saturated,
// with callers waiting for a connection, for pool_wait_limit.
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
	// lastPingErr is the error of the check's latest own ping, nil when it succeeded.
	// The health answer never carries it; the startup wait logs it.
	lastPingErr error
	// saturatedSince is when the pool became saturated; zero while it is not.
	saturatedSince time.Time
	// waitCount is the pool's WaitCount at the previous evaluation, the baseline for
	// telling whether callers have waited since; hasWaitCount is false until it is set.
	waitCount    int64
	hasWaitCount bool
}

// newDatabaseCheck returns the database health check for pool, deciding from window
// with settings. now supplies the current time.
func newDatabaseCheck(pool databasePool, window *observe.Window, settings config.HealthDatabaseConfig,
	now func() time.Time) health.CheckFunc {
	return buildDatabaseCheck(pool, window, settings, now).check
}

// buildDatabaseCheck returns the database check itself, for callers that also need to
// make it decide afresh.
func buildDatabaseCheck(pool databasePool, window *observe.Window, settings config.HealthDatabaseConfig,
	now func() time.Time) *databaseCheck {
	c := &databaseCheck{
		pool:          pool,
		window:        window,
		settings:      settings,
		now:           now,
		timeoutReason: health.Reason(fmt.Sprintf("timed out after %s", settings.CheckTimeout)),
	}
	return c
}

// forget drops the last decided state, so the next evaluation pings the database
// instead of repeating it.
func (c *databaseCheck) forget() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decided, c.hasDecided = nil, false
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
	if recent(lastPing, now, s.IdlePeriod) && c.hasDecided {
		err := c.decided
		c.mu.Unlock()
		return err
	}
	c.lastPing = now
	c.mu.Unlock()

	err := c.ping(ctx)
	c.mu.Lock()
	c.decided, c.hasDecided = err, true
	c.lastPingErr = err
	c.mu.Unlock()
	return err
}

// lastPingError returns the error of the check's latest own ping, nil when it
// succeeded or none has run.
func (c *databaseCheck) lastPingError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastPingErr
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
// connections are in use and callers have waited for a connection since the previous
// evaluation; a busy pool that still serves every caller at once is not. Pools limited
// to one connection, or not limited at all, are never considered saturated.
func (c *databaseCheck) poolExhausted() bool {
	stats := c.pool.Stats()
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	waited := c.hasWaitCount && stats.WaitCount > c.waitCount
	c.waitCount, c.hasWaitCount = stats.WaitCount, true
	limit := stats.MaxOpenConnections
	if limit <= 1 || stats.InUse < saturationThreshold(c.settings.PoolSaturation, limit) || !waited {
		c.saturatedSince = time.Time{}
		return false
	}
	if c.saturatedSince.IsZero() {
		c.saturatedSince = now
	}
	return now.Sub(c.saturatedSince) >= c.settings.PoolWaitLimit
}

// saturationThreshold is the number of connections in use, out of maxOpen, at which the
// pool counts as saturated: ratio of maxOpen, rounded up. A tiny tolerance keeps float
// error such as 0.7 × 10 = 7.000000000000001 from rounding up a whole connection.
func saturationThreshold(ratio float64, maxOpen int) int {
	return int(math.Ceil(ratio*float64(maxOpen) - 1e-9))
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
	// refresh, when set, runs before each retry so the retry checks the database
	// afresh instead of reading a cached or still-standing earlier decision.
	refresh func()
	// cause, when set, returns the driver error behind the latest failed check. It is
	// logged, redacted, so an operator sees why the database is not reachable, and it
	// decides whether retrying can help at all.
	cause func() error
	// fatal reports whether a cause is one retrying cannot fix.
	fatal func(error) bool
	// loggedCause is the last cause text logged, so a retry failing the same way adds
	// no log line.
	loggedCause string
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
		fatal:    observe.IsMisconfiguration,
	}
}

// run returns nil once the database is reachable, errStartupWaitExpired when it is
// still unreachable after the wait, errStartupDatabaseRejected as soon as the database
// rejects the connection in a way retrying cannot fix, and errStartupInterrupted on a
// shutdown signal. The health checker logs the database check once when it fails and
// once when it recovers; the wait itself logs the driver's cause, redacted, when it
// starts waiting and whenever the cause changes, so the reason the database is not
// reachable is visible without debug logging.
func (w *databaseWait) run() error {
	if w.databaseUp() {
		return w.interrupted()
	}
	if err := w.rejected(); err != nil {
		return err
	}
	w.log.Warn("waiting for the database before completing startup",
		slog.Duration("startup_wait", w.wait), slog.String("cause", w.noteCause()))

	deadline := w.now().Add(w.wait)
	interval := startupRetryInitial
	for {
		remaining := deadline.Sub(w.now())
		if remaining <= 0 {
			w.log.Error(errStartupWaitExpired.Error(), slog.Duration("startup_wait", w.wait),
				slog.String("cause", w.causeText()))
			return errStartupWaitExpired
		}
		select {
		case <-w.stop:
			return errStartupInterrupted
		case <-w.after(min(interval, remaining)):
		}
		if w.refresh != nil {
			w.refresh()
		}
		if w.databaseUp() {
			return w.interrupted()
		}
		if err := w.rejected(); err != nil {
			return err
		}
		if text := w.causeText(); text != w.loggedCause {
			w.log.Warn("database still not reachable", slog.String("cause", w.noteCause()))
		}
		interval = min(2*interval, startupRetryMax)
	}
}

// interrupted returns errStartupInterrupted when a shutdown signal arrived while the database
// was being checked, so a signal that comes during a successful check stops startup
// instead of waiting until the whole server has started; otherwise nil.
func (w *databaseWait) interrupted() error {
	select {
	case <-w.stop:
		return errStartupInterrupted
	default:
		return nil
	}
}

// rejected returns errStartupDatabaseRejected, after logging the cause, when the latest
// failure is one retrying cannot fix.
func (w *databaseWait) rejected() error {
	if w.cause == nil || w.fatal == nil {
		return nil
	}
	if err := w.cause(); err != nil && w.fatal(err) {
		w.log.Error(errStartupDatabaseRejected.Error(), slog.String("cause", w.causeText()))
		return errStartupDatabaseRejected
	}
	return nil
}

// causeText returns the redacted text of the latest failure's cause, or "unknown".
func (w *databaseWait) causeText() string {
	if w.cause == nil {
		return "unknown"
	}
	err := w.cause()
	if err == nil {
		return "unknown"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "database ping timed out"
	}
	return redactDatabaseCause(err.Error())
}

// noteCause returns causeText and remembers it as logged.
func (w *databaseWait) noteCause() string {
	w.loggedCause = w.causeText()
	return w.loggedCause
}

var (
	// passwordSetting matches a password key-value in a connection string. A quoted
	// value may contain backslash-escaped quotes (libpq) or doubled quotes (SQL Server),
	// and SQL Server also accepts a {braced} value with doubled closing braces; each is
	// matched to its real end so no part of the password is left behind.
	passwordSetting = regexp.MustCompile(`(?i)(password|pwd)(\s*=\s*)('(?:[^'\\]|\\.|'')*'|"(?:[^"\\]|\\.|"")*"|\{(?:[^}]|\}\})*\}|[^\s;]+)`)
	// urlUserinfoPassword matches the password in a URL's userinfo.
	urlUserinfoPassword = regexp.MustCompile(`(://[^:/@\s]*:)[^@\s]*@`)
)

// redactDatabaseCause removes passwords from a driver error, which can echo the
// connection string it failed to use.
func redactDatabaseCause(msg string) string {
	msg = passwordSetting.ReplaceAllString(msg, "${1}${2}xxxxx")
	return urlUserinfoPassword.ReplaceAllString(msg, "${1}xxxxx@")
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
	dbCheck := buildDatabaseCheck(db.GetDB(), window, settings, time.Now)
	checker.Register("database", dbCheck.check, health.WithTimeout(settings.CheckTimeout))
	checker.RegisterUnlogged("startup", gate.check)
	gate.invalidate = checker.Invalidate
	// Serve health before waiting, so probes get a real answer (503) during the wait
	// instead of a refused connection.
	admin := startAdminServer(cfg, checker.Handler(), authMiddleware, log)
	wait := newDatabaseWait(checker.Evaluate, settings.StartupWait, quit, log)
	wait.refresh = func() {
		dbCheck.forget()
		checker.Invalidate()
	}
	wait.cause = dbCheck.lastPingError
	if err := wait.run(); err != nil {
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
// signal ended the wait, 1 when the wait expired or the database rejected the
// connection.
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

// listenOrExit binds addr for the named server, exiting the controller when the address
// cannot be bound, the same way a failed ListenAndServe does.
func listenOrExit(addr, name string, log *slog.Logger) net.Listener {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("Failed to start "+name, slog.String("addr", addr), slog.Any("error", err))
		os.Exit(1)
	}
	return ln
}
