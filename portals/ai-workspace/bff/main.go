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
	"ai-workspace-bff/internal/logger"
	"ai-workspace-bff/internal/server"
	"ai-workspace-bff/internal/tlsutil"
)

// printStartedMarker writes a large, prominent banner for humans watching
// the console, matching the gateway controller's startup banner style. It's
// purely decorative — the structured "AI Workspace BFF started" slog.Info
// line is the source of truth for log parsing.
func printStartedMarker(mode string) {
	fmt.Print("\n\n" +
		"========================================================================\n" +
		"\n" +
		"                    AI Workspace Started mode=" + mode + "\n" +
		"\n" +
		"========================================================================\n" +
		"\n\n")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.SetDefault(logger.NewLogger(logger.Config{Level: "info", Format: "text"}))
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}
	slog.SetDefault(logger.NewLogger(logger.Config{Level: cfg.LogLevel, Format: cfg.LogFormat}))

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

	tlsConfig, err := buildTLS(cfg.TLS, cfg.DemoMode)
	if err != nil {
		slog.Error("failed to set up TLS", "err", err)
		os.Exit(1)
	}
	httpServer.TLSConfig = tlsConfig

	go func() {
		mode := "PRODUCTION"
		if cfg.DemoMode {
			mode = "DEMO"
		}
		slog.Info("AI Workspace BFF started",
			"addr", cfg.Addr,
			"mode", mode,
			"auth_mode", cfg.AuthMode,
			"platform_api", cfg.PlatformAPIURL,
			"oidc_enabled", cfg.OIDC.Enabled,
		)
		printStartedMarker(mode)
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

// buildTLS returns the listener TLS config, or nil for plain HTTP. When TLS is
// disabled no certificate is read, generated, or required. Otherwise the priority
// is: mounted cert/key files (when both exist), then in-memory self-signed. In
// demo mode a missing mounted cert is not fatal — it falls back to a self-signed
// cert; outside demo mode an operator-provided cert is required.
func buildTLS(c config.TLSConfig, demoMode bool) (*tls.Config, error) {
	if !c.Enabled {
		// Plain HTTP is only safe when something upstream terminates TLS.
		if !demoMode {
			slog.Warn("TLS: disabled (BFF_TLS_ENABLED=false) while APIP_DEMO_MODE=false — " +
				"serving plain HTTP. Terminate TLS at an ingress or service-mesh sidecar and " +
				"never expose this listener directly to untrusted networks.")
		} else {
			slog.Info("TLS: disabled (BFF_TLS_ENABLED=false) — serving plain HTTP")
		}
		return nil, nil
	}
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
	// No mounted cert. Auto-generating a self-signed certificate is a dev-only
	// convenience — outside demo mode, require the operator to mount a real cert.
	if !demoMode {
		return nil, fmt.Errorf("APIP_DEMO_MODE=false requires a mounted TLS certificate: "+
			"set BFF_TLS_CERT_FILE (%q) and BFF_TLS_KEY_FILE (%q) to existing files, "+
			"or set BFF_TLS_ENABLED=false to serve plain HTTP behind a TLS-terminating proxy. "+
			"Self-signed certificates are only auto-generated in demo mode", c.CertFile, c.KeyFile)
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
