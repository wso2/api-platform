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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/adminserver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
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

// pinger is the part of the database pool the health check needs.
type pinger interface {
	PingContext(ctx context.Context) error
}

// newDatabaseCheck returns the health check that pings the controller's storage
// database. A ping cut short by the check deadline is reported as a timeout; any other
// ping error as the database being unreachable.
//
// At most one ping is in flight at a time. A driver that ignores its context could
// otherwise leave one ping per evaluation stuck on the database, so probes against a
// hung database would pile up pooled connections; while an earlier ping is still
// running, the check reports a timeout without starting another.
func newDatabaseCheck(db pinger) health.CheckFunc {
	var running atomic.Bool
	return func(ctx context.Context) error {
		if !running.CompareAndSwap(false, true) {
			return context.DeadlineExceeded
		}
		// Buffered, so the ping goroutine never blocks once the check has given up.
		done := make(chan error, 1)
		go func() {
			err := db.PingContext(ctx)
			// Cleared before the result is handed over, so the next evaluation after
			// this one can ping again.
			running.Store(false)
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
}

// newHealthChecker builds the checker behind the admin health endpoints and the logger
// it reports check failures and recoveries to: the main log plus, when configured, the
// health log file in the same format. The returned closer closes that file.
func newHealthChecker(cfg *config.Config, log *slog.Logger) (*health.Checker, io.Closer) {
	logCfg := logger.Config{Level: cfg.Controller.Logging.Level, Format: cfg.Controller.Logging.Format}
	hc := cfg.Controller.Health
	healthLogger, closer := health.NewLogger(log,
		func(w io.Writer) slog.Handler { return logger.NewHandler(logCfg, w) },
		health.FileLogConfig{Path: hc.LogFile, MaxSizeMB: hc.LogMaxSizeMB, MaxFiles: hc.LogMaxFiles})
	checker := health.NewChecker(health.Options{
		Component:      "gateway-controller",
		HealthyStatus:  "healthy",
		CheckTimeout:   hc.CheckTimeout,
		CacheTTL:       hc.CacheTTL,
		Logger:         healthLogger,
		DebugLogger:    log,
		BackoffInitial: hc.BackoffInitial,
		BackoffMax:     hc.BackoffMax,
	})
	return checker, closer
}

// databaseWait waits at startup for the database health check to pass, retrying with
// backoff, for at most wait.
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
// Check failures and the recovery are logged by the health checker, on its own
// backoff schedule, so the retries here add no log lines of their own.
func (w *databaseWait) run() error {
	if w.evaluate().Healthy {
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
		if w.evaluate().Healthy {
			return nil
		}
		interval = min(2*interval, startupRetryMax)
	}
}

// serveHealthWhileWaiting registers the database check on db, starts the admin server
// so health is served at once, and waits for the database to answer. It returns the
// admin server (nil when disabled) and ok once the database is reachable. When the wait
// ends without the database it has already released db, the admin server and the
// health log, and returns the exit code the controller must stop with.
func serveHealthWhileWaiting(cfg *config.Config, log *slog.Logger, checker *health.Checker, healthLog io.Closer,
	db storage.Storage, authMiddleware func(http.Handler) http.Handler, quit <-chan os.Signal) (*adminserver.Server, int, bool) {
	checker.Register("database", newDatabaseCheck(db.GetDB()))
	// Serve health before waiting, so probes get a real answer (503, database down)
	// during the wait instead of a refused connection.
	admin := startAdminServer(cfg, checker.Handler(), authMiddleware, log)
	if err := newDatabaseWait(checker.Evaluate, cfg.Controller.Health.StartupWait, quit, log).run(); err != nil {
		return nil, endStartupWithoutDatabase(err, admin, cfg.Controller.Server.ShutdownTimeout, log, db, healthLog), false
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
