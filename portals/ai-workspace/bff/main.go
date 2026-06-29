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

// Command ai-workspace-bff is the Backend-for-Frontend for the AI Workspace SPA.
// It serves the SPA, proxies all browser→backend traffic same-origin, and owns
// authentication (file-based credentials and the OIDC confidential-client flow).
// Tokens live server-side; the browser only ever holds an opaque session cookie.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/server"
	"ai-workspace-bff/internal/tlsutil"
)

const startupBanner = `
========================================

         AI Workspace started

========================================
`

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize server", "err", err)
		os.Exit(1)
	}
	defer srv.Close()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0, // 0 = no write timeout; needed for SSE / streamed responses
		IdleTimeout:       120 * time.Second,
	}

	tlsConfig, err := buildTLS(cfg.TLS)
	if err != nil {
		slog.Error("failed to set up TLS", "err", err)
		os.Exit(1)
	}
	httpServer.TLSConfig = tlsConfig

	go func() {
		fmt.Print(startupBanner)
		slog.Info("AI Workspace BFF started",
			"addr", cfg.Addr,
			"auth_mode", cfg.AuthMode,
			"platform_api", cfg.PlatformAPIURL,
			"oidc_enabled", cfg.OIDC.Enabled,
		)
		var serveErr error
		if tlsConfig != nil {
			serveErr = httpServer.ListenAndServeTLS("", "")
		} else {
			serveErr = httpServer.ListenAndServe()
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("server error", "err", serveErr)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sig
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}

// buildTLS returns the listener TLS config, or nil for plain HTTP. Priority:
// mounted cert/key files (when both exist), then in-memory self-signed, then
// disabled. A missing mounted cert is not fatal — it falls back to self-signed.
func buildTLS(c config.TLSConfig) (*tls.Config, error) {
	// A partial mount (exactly one of cert/key present) is a misconfiguration, not
	// a request for plain HTTP — fail loudly instead of silently downgrading.
	if fileExists(c.CertFile) != fileExists(c.KeyFile) {
		return nil, fmt.Errorf("incomplete TLS mount: exactly one of cert (%q) and key (%q) is present", c.CertFile, c.KeyFile)
	}
	if fileExists(c.CertFile) && fileExists(c.KeyFile) {
		cert, err := tlsutil.CertFromFiles(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		slog.Info("TLS: using mounted certificate", "cert", c.CertFile)
		return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
	}
	if c.SelfSigned {
		cert, err := tlsutil.SelfSigned()
		if err != nil {
			return nil, err
		}
		slog.Warn("TLS: using in-memory self-signed certificate (browsers will warn)")
		return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
	}
	return nil, nil
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
