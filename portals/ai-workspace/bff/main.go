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
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/logger"
	"ai-workspace-bff/internal/server"
	"ai-workspace-bff/internal/tlsutil"
)

// bannerWidth is the character width of the startup banner's rule lines.
const bannerWidth = 72

// centerInBanner left-pads s so it sits centered between the banner rules.
func centerInBanner(s string) string {
	if len(s) >= bannerWidth {
		return s
	}
	return strings.Repeat(" ", (bannerWidth-len(s))/2) + s
}

// printStartedMarker writes a large, prominent banner for humans watching
// the console, matching the gateway controller's startup banner style. It's
// purely decorative — the structured "AI Workspace BFF started" slog.Info
// line is the source of truth for log parsing.
func printStartedMarker(url string) {
	rule := strings.Repeat("=", bannerWidth)
	fmt.Print("\n\n" +
		rule + "\n" +
		"\n" +
		centerInBanner("AI Workspace Started") + "\n\n" +
		centerInBanner("Visit "+url) + "\n" +
		"\n" +
		rule + "\n" +
		"\n\n")
}

// portalURL renders the browser-visitable address of the portal. A wildcard or
// empty listen host is reported as localhost, since "https://:8081" and
// "https://0.0.0.0:8081" are not addresses a human can click.
func portalURL(addr string, tlsEnabled bool) string {
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return scheme + "://" + addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func main() {
	// The container reads its mounted config.toml, so -config is only needed to run
	// the BFF outside one (see `make bff-run`).
	configFile := flag.String("config", config.DefaultConfigFile, "path to config.toml")
	// The shipped config.toml has no {{ env }} token for static_dir — the container
	// always serves the SPA baked in at /app, so there's nothing for an operator to
	// override. `make bff-run` is the one caller that needs a different directory
	// (the locally-built ../dist), so it's a dev-only CLI flag rather than a config
	// key, keeping the shipped config free of a token no deployment ever sets.
	staticDir := flag.String("static-dir", "", "override the directory serving the built SPA (local dev only, e.g. `make bff-run`)")
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		slog.SetDefault(logger.NewLogger(logger.Config{Level: "info", Format: "text"}))
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}
	if *staticDir != "" {
		cfg.Server.StaticDir = *staticDir
	}
	slog.SetDefault(logger.NewLogger(logger.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize server", "err", err)
		os.Exit(1)
	}
	defer srv.Close()

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0, // 0 = no write timeout; needed for SSE / streamed responses
		IdleTimeout:       120 * time.Second,
	}

	tlsConfig, err := buildTLS(cfg.Server.HTTPS)
	if err != nil {
		slog.Error("failed to set up TLS", "err", err)
		os.Exit(1)
	}
	httpServer.TLSConfig = tlsConfig

	go func() {
		url := portalURL(cfg.Addr(), tlsConfig != nil)
		slog.Info("AI Workspace BFF started",
			"addr", cfg.Addr(),
			"url", url,
			"auth_mode", cfg.Auth.Mode,
			"control_plane", cfg.ControlPlane.URL,
			"oidc_enabled", cfg.Auth.OIDC.Enabled,
		)
		printStartedMarker(url)
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
// disabled no certificate is read or required. Otherwise a mounted cert/key pair
// is required — there is no self-signed fallback; use the quickstart setup
// script (or your own tooling) to generate a pair and mount it.
func buildTLS(c config.HTTPSConfig) (*tls.Config, error) {
	if !c.Enabled {
		// Plain HTTP is only safe when something upstream terminates TLS.
		slog.Warn("TLS: disabled ([server.https] enabled = false) — serving plain HTTP. " +
			"Terminate TLS at an ingress or service-mesh sidecar and " +
			"never expose this listener directly to untrusted networks.")
		return nil, nil
	}
	// A partial mount (exactly one of cert/key present) is a misconfiguration, not
	// a request for plain HTTP — fail loudly instead of silently downgrading.
	if fileExists(c.CertFile) != fileExists(c.KeyFile) {
		return nil, fmt.Errorf("incomplete TLS mount: exactly one of cert (%q) and key (%q) is present", c.CertFile, c.KeyFile)
	}
	if !fileExists(c.CertFile) {
		return nil, fmt.Errorf("TLS is enabled but no certificate is mounted: "+
			"set [server.https] cert_file (%q) and key_file (%q) to existing files, "+
			"or set [server.https] enabled = false to serve plain HTTP behind a TLS-terminating proxy", c.CertFile, c.KeyFile)
	}
	cert, err := tlsutil.CertFromFiles(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	slog.Info("TLS: using mounted certificate", "cert", c.CertFile)
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
