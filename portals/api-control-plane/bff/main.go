/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Command api-control-plane-bff is the Backend-for-Frontend for the api-control-plane
// SPA. It owns authentication (Platform API's file-based login by default, or a
// confidential/PKCE OIDC flow against a configured external IdP): tokens live
// server-side, and the browser only ever holds an opaque HttpOnly session cookie.
//
// The same-origin reverse proxy to the Platform API and static SPA serving, plus the
// HTTPS listener and graceful dual-listener shutdown, land in a follow-up commit —
// today only the plain-HTTP listener is wired so local development can exercise the
// auth flows end to end.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/server"
)

func main() {
	var configPaths multiFlag
	flag.Var(&configPaths, "config", "path to a config.toml file; may be repeated to merge multiple files")
	flag.Parse()

	if len(configPaths) == 0 {
		fmt.Fprintln(os.Stderr, "at least one -config path is required")
		os.Exit(1)
	}

	cfg, err := config.Load(configPaths...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	setupLogging(cfg.Logging.Level, cfg.Logging.Format)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize server", "err", err)
		os.Exit(1)
	}
	defer srv.Close()

	addr, err := listen(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	httpServer := &http.Server{
		Addr:           addr,
		Handler:        srv.Handler(),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   0, // streamed/proxied responses (SSE) need no write deadline
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()
	slog.Info("api-control-plane-bff listening",
		slog.String("addr", addr),
		slog.String("auth_mode", cfg.Auth.Mode),
		slog.Bool("oidc_enabled", cfg.Auth.OIDC.Enabled))

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server exited: %v\n", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
	}
}

// listen picks the listener address from config. The HTTPS listener (cert/key
// loading, dual-listener shutdown) arrives with the proxy/deployment work; today
// only the plain-HTTP listener is wired so local development can start end to end.
func listen(cfg *config.Config) (string, error) {
	if cfg.Server.HTTP.Enabled {
		return fmt.Sprintf(":%d", cfg.Server.HTTP.Port), nil
	}
	return "", fmt.Errorf("only [server.http] is wired up so far; enable it for now (set enabled = true)")
}

func setupLogging(level, format string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: lvl}
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// multiFlag collects repeated -config flags into an ordered slice.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
