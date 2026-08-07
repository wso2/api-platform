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
// Config.validate requires at least one of [server.http]/[server.https] enabled
// (the default config enables HTTPS only, matching go-network-service-hardening's
// "default to TLS, plaintext is an explicit opt-out"); both may run at once, each
// on its own port.
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

	servers, err := buildServers(cfg, srv.Handler())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Buffered to len(servers): each listener goroutine below sends exactly
	// once, and a send must never block on a select that already moved on to
	// the ctx.Done() shutdown path.
	errCh := make(chan error, len(servers))
	for _, ls := range servers {
		ls := ls
		go func() { errCh <- ls.listenAndServe() }()
		slog.Info("api-control-plane-bff listening",
			slog.String("protocol", ls.protocol),
			slog.String("addr", ls.httpServer.Addr),
			slog.String("auth_mode", cfg.Auth.Mode),
			slog.Bool("oidc_enabled", cfg.Auth.OIDC.Enabled))
	}

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
		for _, ls := range servers {
			if err := ls.httpServer.Shutdown(shutdownCtx); err != nil {
				slog.Error("graceful shutdown failed", "protocol", ls.protocol, "err", err)
			}
		}
	}
}

// listener pairs a configured http.Server with however it needs to be
// started — ListenAndServe for plain HTTP, ListenAndServeTLS for HTTPS.
type listener struct {
	protocol       string
	httpServer     *http.Server
	listenAndServe func() error
}

// buildServers constructs one *http.Server per enabled listener.
// Config.validate already guarantees at least one of HTTP/HTTPS is enabled,
// with valid, non-colliding ports; it does not check cert_file/key_file are
// set, so buildServers checks that itself before ever calling
// ListenAndServeTLS with an empty path.
//
// WriteTimeout is deliberately 0 (no server-wide deadline) on both: the
// reverse proxy streams long-lived SSE responses (e.g. runtime logs) through
// to the Platform API, and every other handler already bounds itself via
// withWriteDeadline in routes() — a server-wide timeout here would cut off
// the former to bound the latter, when the latter is already bounded.
func buildServers(cfg *config.Config, handler http.Handler) ([]listener, error) {
	var servers []listener

	if cfg.Server.HTTP.Enabled {
		httpServer := &http.Server{
			Addr:           fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
			Handler:        handler,
			ReadTimeout:    15 * time.Second,
			WriteTimeout:   0,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		servers = append(servers, listener{
			protocol:       "http",
			httpServer:     httpServer,
			listenAndServe: httpServer.ListenAndServe,
		})
	}

	if cfg.Server.HTTPS.Enabled {
		if cfg.Server.HTTPS.CertFile == "" || cfg.Server.HTTPS.KeyFile == "" {
			return nil, fmt.Errorf("[server.https] cert_file and key_file are required when enabled = true")
		}
		httpsServer := &http.Server{
			Addr:           fmt.Sprintf(":%d", cfg.Server.HTTPS.Port),
			Handler:        handler,
			ReadTimeout:    15 * time.Second,
			WriteTimeout:   0,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		servers = append(servers, listener{
			protocol:   "https",
			httpServer: httpsServer,
			listenAndServe: func() error {
				return httpsServer.ListenAndServeTLS(cfg.Server.HTTPS.CertFile, cfg.Server.HTTPS.KeyFile)
			},
		})
	}

	return servers, nil
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
