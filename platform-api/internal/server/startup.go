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
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/health"
	"github.com/wso2/api-platform/platform-api/internal/logger"
)

const (
	// startupRetryInitial is the delay before the database is checked again during the
	// startup wait; it doubles after every failed check up to startupRetryMax.
	startupRetryInitial = time.Second
	// startupRetryMax caps the delay between database checks during the startup wait,
	// which bounds how long readiness lags behind the database coming back.
	startupRetryMax = 10 * time.Second
)

// ErrStartupInterrupted is returned when a shutdown signal arrives while startup is
// still waiting for the database. It is a clean stop, not a failure.
var ErrStartupInterrupted = errors.New("shutdown signal received during startup")

// errStartupWaitExpired is returned when the database is still unreachable at the end
// of the startup wait.
var errStartupWaitExpired = errors.New("database not reachable within the startup wait")

// errStarting is the startup check's reason until the full API serves.
var errStarting = health.Reason("starting")

// serving is what startup acquires before the full server is built: the health
// checker and its log file, the database pool, the bound listeners, the channel that
// receives shutdown signals, and the flag that ends the health check's startup phase.
type serving struct {
	health    *health.Checker
	healthLog io.Closer
	db        *database.DB
	listeners *listenerSet
	quit      chan os.Signal
	// started is set once the full API serves on the listeners; until then the startup
	// check keeps health down.
	started *atomic.Bool
}

// startServing opens the database pool, binds the listeners and waits for the database
// to answer. The listeners are bound before the wait, so health probes get a real
// answer (503) while it lasts instead of a refused connection; every other request gets
// 503 until the full server takes over. Health stays down, through the startup check,
// until the caller sets started. On any failure, everything acquired so far is released
// again.
func (s *Server) startServing(cfg *config.Server) (*serving, error) {
	sv := &serving{quit: make(chan os.Signal, 1), started: new(atomic.Bool)}
	sv.health, sv.healthLog = newHealthChecker(cfg, s.logger)
	sv.health.RegisterUnlogged("startup", newStartupCheck(sv.started))
	// Shutdown signals are caught from here on, so one arriving during the wait stops
	// startup cleanly; Start keeps receiving them afterwards.
	signal.Notify(sv.quit, syscall.SIGINT, syscall.SIGTERM)

	started := false
	defer func() {
		if !started {
			sv.release()
		}
	}()

	// Opening the pool does not connect; whether the database answers is decided by
	// the health check.
	db, err := database.Open(&cfg.Database, s.logger)
	if err != nil {
		s.logger.Error("Failed to open database", "error", err)
		return nil, err
	}
	sv.db = db
	sv.health.Register("database", newDatabaseCheck(db))

	tlsConfig, err := s.preflightListeners(cfg.Listeners)
	if err != nil {
		return nil, err
	}
	sv.listeners, err = bindListeners(cfg.Listeners, cfg.Listeners.Timeouts, tlsConfig,
		newStartupHandler(sv.health), s.logger)
	if err != nil {
		s.logger.Error("Failed to start listeners", "error", err)
		return nil, err
	}

	if err := newDatabaseWait(sv.health.Evaluate, cfg.Health.StartupWait, sv.quit, s.logger).run(); err != nil {
		if errors.Is(err, ErrStartupInterrupted) {
			s.logger.Info("Received shutdown signal during startup")
		}
		return nil, err
	}
	if err := db.VerifyConnection(context.Background()); err != nil {
		s.logger.Error("Failed to verify database connection", "error", err)
		return nil, err
	}

	started = true
	return sv, nil
}

// release stops everything startServing acquired, for a startup that fails after it.
func (sv *serving) release() {
	signal.Stop(sv.quit)
	if sv.listeners != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		sv.listeners.shutdown(shutdownCtx)
	}
	if sv.db != nil {
		_ = sv.db.Close()
	}
	_ = sv.healthLog.Close()
}

// newStartupCheck reports the platform as starting until started is set. It is
// registered unlogged: it fails on every start, so its failure is not news.
func newStartupCheck(started *atomic.Bool) health.CheckFunc {
	return func(context.Context) error {
		if started.Load() {
			return nil
		}
		return errStarting
	}
}

// newHealthChecker builds the checker behind /health and the logger it reports check
// failures and recoveries to: the main log plus, when configured, the health log file
// in the same format. The returned closer closes that file.
func newHealthChecker(cfg *config.Server, slogger *slog.Logger) (*health.Checker, io.Closer) {
	logCfg := logger.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format}
	healthLogger, closer := health.NewLogger(slogger,
		func(w io.Writer) slog.Handler { return logger.NewHandler(logCfg, w) },
		health.FileLogConfig{
			Path:      cfg.Health.LogFile,
			MaxSizeMB: cfg.Health.LogMaxSizeMB,
			MaxFiles:  cfg.Health.LogMaxFiles,
		})
	checker := health.NewChecker(health.Options{
		Component:      "platform-api",
		HealthyStatus:  "ok",
		CheckTimeout:   cfg.Health.CheckTimeout,
		CacheTTL:       cfg.Health.CacheTTL,
		Logger:         healthLogger,
		DebugLogger:    slogger,
		BackoffInitial: cfg.Health.BackoffInitial,
		BackoffMax:     cfg.Health.BackoffMax,
	})
	return checker, closer
}

// waitForStop blocks until a listener fails or a shutdown signal arrives. It returns
// the listener's error, or nil for a signal.
func (s *Server) waitForStop() error {
	select {
	case err := <-s.listeners.errs:
		// A graceful Shutdown surfaces http.ErrServerClosed on the listener
		// goroutine; that is a clean exit, not a startup failure.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case sig := <-s.quit:
		s.logger.Info("Received shutdown signal", "signal", sig)
		return nil
	}
}

// preflightListeners validates the listener configuration and builds the TLS config
// before anything is bound, so a misconfiguration fails startup at once.
func (s *Server) preflightListeners(listeners config.ServerListeners) (*tls.Config, error) {
	httpCfg := listeners.HTTP
	httpsCfg := listeners.HTTPS
	if !httpCfg.Enabled && !httpsCfg.Enabled {
		s.logger.Error("No listeners enabled")
		return nil, fmt.Errorf("no listeners enabled: set server.http.enabled=true and/or server.https.enabled=true in config")
	}
	if httpCfg.Enabled && (httpCfg.Port <= 0 || httpCfg.Port > 65535) {
		return nil, fmt.Errorf("HTTP listener enabled but server.http.port is invalid (got %d)", httpCfg.Port)
	}
	if !httpsCfg.Enabled {
		return nil, nil
	}
	if httpsCfg.Port <= 0 || httpsCfg.Port > 65535 {
		return nil, fmt.Errorf("HTTPS listener enabled but server.https.port is invalid (got %d)", httpsCfg.Port)
	}
	return s.buildTLSConfig(httpsCfg)
}

// listenerSet is the running HTTP and HTTPS listeners. Every listener serves whichever
// handler was set last, so the same ports can answer during startup and then switch
// to the full API without being rebound.
type listenerSet struct {
	logger  *slog.Logger
	handler atomic.Pointer[http.Handler]
	servers []*http.Server
	addrs   []net.Addr
	// errs receives what each listener's Serve returns; it is buffered for every
	// listener so a failing listener never blocks.
	errs chan error
}

// bindListeners binds every enabled listener and starts serving handler on it. The
// plain-HTTP and TLS listeners are independent: either or both may run, each on its own
// port, mirroring the gateway router's http/https listener split. Binding happens before
// this returns, so a port that is already in use fails here rather than later from a
// background goroutine.
//
// timeouts bounds connection lifetime on both listeners so a slow or idle peer cannot
// hold one open indefinitely (Slowloris). It is validated at config load.
func bindListeners(listeners config.ServerListeners, timeouts config.Timeouts, tlsConfig *tls.Config,
	handler http.Handler, logger *slog.Logger) (*listenerSet, error) {
	ls := &listenerSet{logger: logger}
	ls.useHandler(handler)

	type binding struct {
		ln   net.Listener
		srv  *http.Server
		tls  bool
		name string
	}
	var bindings []binding
	closeAll := func() {
		for _, b := range bindings {
			_ = b.ln.Close()
		}
	}
	bind := func(port int, useTLS bool, name string) error {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			return fmt.Errorf("failed to bind the %s listener on port %d: %w", name, port, err)
		}
		srv := &http.Server{
			Addr:              ":" + strconv.Itoa(port),
			Handler:           ls,
			ReadHeaderTimeout: timeouts.ReadHeader,
			ReadTimeout:       timeouts.Read,
			WriteTimeout:      timeouts.Write,
			IdleTimeout:       timeouts.Idle,
		}
		if useTLS {
			srv.TLSConfig = tlsConfig
		}
		bindings = append(bindings, binding{ln: ln, srv: srv, tls: useTLS, name: name})
		return nil
	}

	// Plain-HTTP listener.
	if listeners.HTTP.Enabled {
		// Plain HTTP is only safe when something upstream terminates TLS, or for
		// internal traffic. Say so loudly.
		logger.Warn("Plain-HTTP listener is enabled (server.http.enabled=true); " +
			"terminate TLS at an ingress or service-mesh sidecar and never expose this listener " +
			"directly to untrusted networks.")
		if err := bind(listeners.HTTP.Port, false, "HTTP"); err != nil {
			closeAll()
			return nil, err
		}
	}
	// TLS listener.
	if listeners.HTTPS.Enabled {
		if err := bind(listeners.HTTPS.Port, true, "HTTPS"); err != nil {
			closeAll()
			return nil, err
		}
	}

	ls.errs = make(chan error, len(bindings))
	for _, b := range bindings {
		port := strconv.Itoa(b.ln.Addr().(*net.TCPAddr).Port)
		ls.servers = append(ls.servers, b.srv)
		ls.addrs = append(ls.addrs, b.ln.Addr())
		if b.tls {
			logger.Info("Starting HTTPS listener", "address", "https://localhost:"+port)
			go func(srv *http.Server, ln net.Listener) {
				ls.errs <- srv.ServeTLS(ln, "", "")
			}(b.srv, b.ln)
		} else {
			logger.Info("Starting HTTP listener", "address", "http://localhost:"+port)
			go func(srv *http.Server, ln net.Listener) {
				ls.errs <- srv.Serve(ln)
			}(b.srv, b.ln)
		}
	}
	return ls, nil
}

// ServeHTTP passes the request to the current handler.
func (ls *listenerSet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*ls.handler.Load()).ServeHTTP(w, r)
}

// useHandler switches every listener to h. Requests already being served finish on
// the handler they started with.
func (ls *listenerSet) useHandler(h http.Handler) {
	ls.handler.Store(&h)
}

// shutdown gracefully stops every listener, waiting for in-flight requests until ctx
// ends.
func (ls *listenerSet) shutdown(ctx context.Context) {
	for _, srv := range ls.servers {
		if err := srv.Shutdown(ctx); err != nil {
			ls.logger.Error("HTTP server shutdown error", "addr", srv.Addr, "error", err)
		}
	}
}

// newStartupHandler is what the listeners serve while startup waits for the database:
// the health answer on /health, and 503 for everything else, so no request is served
// as if the platform were ready.
func newStartupHandler(checker *health.Checker) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", checker.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		apperror.WriteHTTP(w, apperror.ServiceUnavailable.New(), "")
	})
	return mux
}

// databaseWait waits at startup for the database health check to pass, retrying with
// backoff, for at most wait. Only the database check counts: the startup check stays
// down until startup has finished, so it cannot be part of the wait.
type databaseWait struct {
	evaluate func() health.Result
	wait     time.Duration
	// stop delivers a shutdown signal; receiving one ends the wait.
	stop   <-chan os.Signal
	logger *slog.Logger
	now    func() time.Time
	after  func(time.Duration) <-chan time.Time
}

func newDatabaseWait(evaluate func() health.Result, wait time.Duration, stop <-chan os.Signal,
	logger *slog.Logger) *databaseWait {
	return &databaseWait{
		evaluate: evaluate,
		wait:     wait,
		stop:     stop,
		logger:   logger,
		now:      time.Now,
		after:    time.After,
	}
}

// run returns nil once the database is reachable, errStartupWaitExpired when it is
// still unreachable after the wait, and ErrStartupInterrupted on a shutdown signal.
// Check failures and the recovery are logged by the health checker, on its own
// backoff schedule, so the retries here add no log lines of their own.
func (w *databaseWait) run() error {
	if w.databaseUp() {
		return nil
	}
	w.logger.Info("waiting for the database before completing startup", slog.Duration("startup_wait", w.wait))

	deadline := w.now().Add(w.wait)
	interval := startupRetryInitial
	for {
		remaining := deadline.Sub(w.now())
		if remaining <= 0 {
			w.logger.Error(errStartupWaitExpired.Error(), slog.Duration("startup_wait", w.wait))
			return errStartupWaitExpired
		}
		select {
		case <-w.stop:
			return ErrStartupInterrupted
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
