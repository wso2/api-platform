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
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/health"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func textHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	})
}

// freePort returns a TCP port that nothing is listening on.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func httpOnly(port int) config.ServerListeners {
	var l config.ServerListeners
	l.HTTP.Enabled = true
	l.HTTP.Port = port
	return l
}

func bindForTest(t *testing.T, listeners config.ServerListeners, tlsConfig *tls.Config, h http.Handler) *listenerSet {
	t.Helper()
	ls, err := bindListeners(listeners, config.Timeouts{ReadHeader: time.Second}, tlsConfig, h, discardLogger())
	if err != nil {
		t.Fatalf("bindListeners: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ls.shutdown(ctx)
	})
	return ls
}

func getBody(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestPreflightListeners(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name    string
		mutate  func(l *config.ServerListeners)
		wantErr string
	}{
		{"no listener enabled", func(l *config.ServerListeners) {}, "no listeners enabled"},
		{"invalid HTTP port", func(l *config.ServerListeners) { l.HTTP.Enabled = true; l.HTTP.Port = 70000 },
			"server.http.port is invalid"},
		{"invalid HTTPS port", func(l *config.ServerListeners) { l.HTTPS.Enabled = true; l.HTTPS.Port = 0 },
			"server.https.port is invalid"},
		{"HTTPS without certificates", func(l *config.ServerListeners) {
			l.HTTPS.Enabled = true
			l.HTTPS.Port = 9243
			l.HTTPS.CertFile = filepath.Join(missing, "cert.pem")
			l.HTTPS.KeyFile = filepath.Join(missing, "key.pem")
		}, "certificate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l config.ServerListeners
			tt.mutate(&l)
			_, err := testServer().preflightListeners(l)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}

	tlsConfig, err := testServer().preflightListeners(httpOnly(9090))
	if err != nil || tlsConfig != nil {
		t.Fatalf("HTTP only: tlsConfig = %v, err = %v, want no TLS config and no error", tlsConfig, err)
	}
}

func TestBindListenersServesAndSwapsHandler(t *testing.T) {
	ls := bindForTest(t, httpOnly(0), nil, textHandler("starting"))
	url := fmt.Sprintf("http://%s/anything", ls.addrs[0])

	if got := getBody(t, http.DefaultClient, url); got != "starting" {
		t.Fatalf("before the swap: body = %q, want starting", got)
	}
	ls.useHandler(textHandler("ready"))
	if got := getBody(t, http.DefaultClient, url); got != "ready" {
		t.Fatalf("after the swap: body = %q, want ready", got)
	}
}

func TestBindListenersServesTLS(t *testing.T) {
	certDir := t.TempDir()
	writeTestCertPair(t, certDir)
	var l config.ServerListeners
	l.HTTPS.Enabled = true
	l.HTTPS.CertFile = filepath.Join(certDir, "cert.pem")
	l.HTTPS.KeyFile = filepath.Join(certDir, "key.pem")
	l.HTTPS.MinimumProtocolVersion = "TLS1_2"
	l.HTTPS.MaximumProtocolVersion = "TLS1_3"
	l.HTTPS.EcdhCurves = "X25519,P-256"
	tlsConfig, err := testServer().buildTLSConfig(l.HTTPS)
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}

	ls := bindForTest(t, l, tlsConfig, textHandler("secure"))
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	if got := getBody(t, client, fmt.Sprintf("https://%s/", ls.addrs[0])); got != "secure" {
		t.Fatalf("body = %q, want secure", got)
	}
}

func TestBindListenersFailsWhenPortInUse(t *testing.T) {
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	// The HTTP listener binds first; the HTTPS port is taken, so startup must fail and
	// release the HTTP port again.
	httpPort := freePort(t)
	l := httpOnly(httpPort)
	l.HTTPS.Enabled = true
	l.HTTPS.Port = busyPort

	if _, err := bindListeners(l, config.Timeouts{}, &tls.Config{}, textHandler("x"), discardLogger()); err == nil {
		t.Fatal("bindListeners succeeded on a port that is in use")
	}
	again, err := net.Listen("tcp", fmt.Sprintf(":%d", httpPort))
	if err != nil {
		t.Fatalf("HTTP port was not released after the failed start: %v", err)
	}
	again.Close()
}

func TestListenerSetShutdown(t *testing.T) {
	ls, err := bindListeners(httpOnly(0), config.Timeouts{}, nil, textHandler("x"), discardLogger())
	if err != nil {
		t.Fatalf("bindListeners: %v", err)
	}
	addr := ls.addrs[0].String()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ls.shutdown(ctx)

	select {
	case err := <-ls.errs:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("serve returned %v, want http.ErrServerClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not report closing")
	}
	if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("listener still accepts connections after shutdown")
	}
}

func TestStartupHandler(t *testing.T) {
	c := newTestChecker(pingFunc(func(context.Context) error { return errors.New("connection refused") }))
	h := newStartupHandler(c)
	serve := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}

	rec := serve(http.MethodGet, "/health")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"database":{"status":"down"`) {
		t.Errorf("GET /health: got %d %s, want 503 with database down", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodHead, "/health"); rec.Code != http.StatusServiceUnavailable || rec.Body.Len() != 0 {
		t.Errorf("HEAD /health: got %d with %d body bytes, want 503 and no body", rec.Code, rec.Body.Len())
	}
	if rec := serve(http.MethodPost, "/health"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /health: got %d, want 405", rec.Code)
	}
	for _, path := range []string{"/", "/api/internal/v1/gateways", "/api/portal/v0.9/auth/login"} {
		rec := serve(http.MethodGet, path)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":"SERVICE_UNAVAILABLE"`) {
			t.Errorf("GET %s: got %d %s, want 503 SERVICE_UNAVAILABLE", path, rec.Code, rec.Body.String())
		}
	}
}

// fakeWait drives a databaseWait with a scripted health sequence and a fake clock that
// advances by each requested delay.
type fakeWait struct {
	healthy []bool
	calls   int
	clock   time.Time
	delays  []time.Duration
	logs    bytes.Buffer
}

func (f *fakeWait) newWait(wait time.Duration, stop <-chan os.Signal) *databaseWait {
	f.clock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := newDatabaseWait(func() health.Result {
		ok := false
		if f.calls < len(f.healthy) {
			ok = f.healthy[f.calls]
		}
		f.calls++
		return health.Result{Healthy: ok}
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

func TestDatabaseWaitHealthyAtOnce(t *testing.T) {
	f := &fakeWait{healthy: []bool{true}}

	if err := f.newWait(5*time.Minute, nil).run(); err != nil {
		t.Fatalf("run() = %v, want nil", err)
	}
	if len(f.delays) != 0 || f.logs.Len() != 0 {
		t.Fatalf("delays = %v, logs = %q; want no wait and no log line", f.delays, f.logs.String())
	}
}

func TestDatabaseWaitRetriesWithBackoff(t *testing.T) {
	f := &fakeWait{healthy: []bool{false, false, false, true}}

	if err := f.newWait(5*time.Minute, nil).run(); err != nil {
		t.Fatalf("run() = %v, want nil", err)
	}
	if want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}; !reflect.DeepEqual(f.delays, want) {
		t.Fatalf("delays = %v, want %v", f.delays, want)
	}
	if !strings.Contains(f.logs.String(), `msg="waiting for the database before completing startup" startup_wait=5m0s`) {
		t.Fatalf("missing the wait line: %q", f.logs.String())
	}
}

func TestDatabaseWaitExpires(t *testing.T) {
	f := &fakeWait{}

	err := f.newWait(time.Minute, nil).run()

	if !errors.Is(err, errStartupWaitExpired) {
		t.Fatalf("run() = %v, want errStartupWaitExpired", err)
	}
	s := time.Second
	want := []time.Duration{s, 2 * s, 4 * s, 8 * s, 10 * s, 10 * s, 10 * s, 10 * s, 5 * s}
	if !reflect.DeepEqual(f.delays, want) {
		t.Fatalf("delays = %v, want %v (doubling, capped, then the remainder)", f.delays, want)
	}
	if f.calls != len(want)+1 {
		t.Fatalf("health evaluated %d times, want %d (once more after the last delay)", f.calls, len(want)+1)
	}
	if !strings.Contains(f.logs.String(), `level=ERROR msg="database not reachable within the startup wait" startup_wait=1m0s`) {
		t.Fatalf("missing the expiry line: %q", f.logs.String())
	}
}

func TestDatabaseWaitStopsOnSignal(t *testing.T) {
	f := &fakeWait{}
	stop := make(chan os.Signal, 1)
	stop <- syscall.SIGTERM
	w := f.newWait(time.Minute, stop)
	w.after = func(time.Duration) <-chan time.Time { return nil } // never fires

	if err := w.run(); !errors.Is(err, ErrStartupInterrupted) {
		t.Fatalf("run() = %v, want ErrStartupInterrupted", err)
	}
}

func TestDatabaseWaitUsesRealClock(t *testing.T) {
	w := newDatabaseWait(func() health.Result { return health.Result{} }, 30*time.Millisecond, nil, discardLogger())

	start := time.Now()
	err := w.run()

	if !errors.Is(err, errStartupWaitExpired) {
		t.Fatalf("run() = %v, want errStartupWaitExpired", err)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("wait took %v, want about 30ms", elapsed)
	}
}

// servingConfig returns a configuration for startServing with a plain-HTTP listener on
// a free port and the given database.
func servingConfig(t *testing.T, db config.Database) *config.Server {
	t.Helper()
	cfg := &config.Server{Database: db}
	cfg.Listeners = httpOnly(freePort(t))
	cfg.Health = config.Health{
		CheckTimeout:   100 * time.Millisecond,
		CacheTTL:       time.Nanosecond,
		LogMaxSizeMB:   1,
		LogMaxFiles:    1,
		BackoffInitial: 10 * time.Second,
		BackoffMax:     time.Minute,
		StartupWait:    time.Minute,
	}
	return cfg
}

func sqliteDatabase(t *testing.T) config.Database {
	return config.Database{Driver: "sqlite3", Path: filepath.Join(t.TempDir(), "platform.db")}
}

// unreachablePostgres points at a local port nothing listens on.
func unreachablePostgres(t *testing.T) config.Database {
	return config.Database{Driver: "postgres", Host: "127.0.0.1", Port: freePort(t), Name: "platform",
		User: "user", SSLMode: "disable", MaxOpenConns: 2, MaxIdleConns: 1}
}

func assertPortReleased(t *testing.T, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("port %d is still bound: %v", port, err)
	}
	ln.Close()
}

func TestStartServingWithReachableDatabase(t *testing.T) {
	cfg := servingConfig(t, sqliteDatabase(t))
	cfg.Health.LogFile = filepath.Join(t.TempDir(), "logs", "health.log")

	sv, err := testServer().startServing(cfg)
	if err != nil {
		t.Fatalf("startServing: %v", err)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", cfg.Listeners.HTTP.Port)

	if body := getBody(t, http.DefaultClient, base+"/health"); !strings.Contains(body, `"status":"ok"`) ||
		!strings.Contains(body, `"database":{"status":"up"}`) {
		t.Errorf("/health = %s, want ok with database up", body)
	}
	if body := getBody(t, http.DefaultClient, base+"/api/internal/v1/gateways"); !strings.Contains(body, "SERVICE_UNAVAILABLE") {
		t.Errorf("other paths before the full server takes over = %s, want 503 SERVICE_UNAVAILABLE", body)
	}
	if _, err := os.Stat(cfg.Health.LogFile); err != nil {
		t.Errorf("health log file not created: %v", err)
	}
	if err := sv.db.Ping(); err != nil {
		t.Errorf("database pool not usable: %v", err)
	}

	sv.release()
	assertPortReleased(t, cfg.Listeners.HTTP.Port)
	if err := sv.db.Ping(); err == nil {
		t.Error("database pool still open after release")
	}
}

func TestStartServingGivesUpAfterStartupWait(t *testing.T) {
	cfg := servingConfig(t, unreachablePostgres(t))
	cfg.Health.StartupWait = 300 * time.Millisecond

	start := time.Now()
	_, err := testServer().startServing(cfg)

	if !errors.Is(err, errStartupWaitExpired) {
		t.Fatalf("startServing = %v, want errStartupWaitExpired", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("gave up after %v, want about the startup wait", elapsed)
	}
	assertPortReleased(t, cfg.Listeners.HTTP.Port)
}

func TestStartServingAnswersHealthWhileWaiting(t *testing.T) {
	cfg := servingConfig(t, unreachablePostgres(t))
	cfg.Health.StartupWait = 3 * time.Second
	done := make(chan error, 1)
	go func() {
		_, err := testServer().startServing(cfg)
		done <- err
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.Listeners.HTTP.Port)
	var resp *http.Response
	deadline := time.Now().Add(2 * time.Second)
	for {
		var err error
		if resp, err = http.Get(url); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("health endpoint not reachable during the startup wait: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), `"database":{"status":"down"`) {
		t.Errorf("during the wait: got %d %s, want 503 with database down", resp.StatusCode, body)
	}
	if err := <-done; !errors.Is(err, errStartupWaitExpired) {
		t.Fatalf("startServing = %v, want errStartupWaitExpired", err)
	}
}

func TestStartServingStopsOnSignalDuringWait(t *testing.T) {
	cfg := servingConfig(t, unreachablePostgres(t))
	done := make(chan error, 1)
	go func() {
		_, err := testServer().startServing(cfg)
		done <- err
	}()

	// The listeners are bound after the signal handler is installed, so once they
	// answer the signal is caught by startup rather than ending the test process.
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Listeners.HTTP.Port)
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener never came up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, ErrStartupInterrupted) {
			t.Fatalf("startServing = %v, want ErrStartupInterrupted", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("startup did not stop on the shutdown signal")
	}
	assertPortReleased(t, cfg.Listeners.HTTP.Port)
}

func TestStartServingFailsFast(t *testing.T) {
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer busy.Close()

	tests := []struct {
		name   string
		mutate func(cfg *config.Server)
	}{
		{"unsupported database driver", func(cfg *config.Server) { cfg.Database.Driver = "oracle" }},
		{"no listener enabled", func(cfg *config.Server) { cfg.Listeners.HTTP.Enabled = false }},
		{"port in use", func(cfg *config.Server) { cfg.Listeners.HTTP.Port = busy.Addr().(*net.TCPAddr).Port }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := servingConfig(t, sqliteDatabase(t))
			tt.mutate(cfg)

			start := time.Now()
			if _, err := testServer().startServing(cfg); err == nil {
				t.Fatal("startServing succeeded, want an error")
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("failed after %v, want an immediate failure without waiting", elapsed)
			}
		})
	}
}

func TestWaitForStop(t *testing.T) {
	newServer := func() *Server {
		s := testServer()
		s.listeners = &listenerSet{errs: make(chan error, 1)}
		s.quit = make(chan os.Signal, 1)
		return s
	}

	s := newServer()
	s.quit <- syscall.SIGTERM
	if err := s.waitForStop(); err != nil {
		t.Errorf("on a signal: err = %v, want nil", err)
	}

	s = newServer()
	s.listeners.errs <- http.ErrServerClosed
	if err := s.waitForStop(); err != nil {
		t.Errorf("on a graceful close: err = %v, want nil", err)
	}

	s = newServer()
	failure := errors.New("listener failed")
	s.listeners.errs <- failure
	if err := s.waitForStop(); !errors.Is(err, failure) {
		t.Errorf("on a listener failure: err = %v, want %v", err, failure)
	}
}
