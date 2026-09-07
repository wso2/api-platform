/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package admin

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// Server is the admin HTTP server
type Server struct {
	cfg        *config.AdminConfig
	httpServer *http.Server
	tlsServer  *http.Server // nil unless cfg.TLS.Enabled
}

// Resource limits for both admin listeners (plaintext and, when enabled,
// TLS) — go-network-service-hardening.md directive 1: a bare http.Server
// with no timeouts/body cap lets a slow or oversized request hold a
// goroutine indefinitely. Admin requests are small (no handler here reads a
// body today), so maxAdminRequestBytes is a defensive ceiling rather than a
// tuned limit.
const (
	adminReadTimeout     = 30 * time.Second
	adminWriteTimeout    = 60 * time.Second
	adminIdleTimeout     = 120 * time.Second
	adminMaxHeaderBytes  = 1 << 20 // 1 MiB
	maxAdminRequestBytes = 1 << 20 // 1 MiB
)

// maxBytesMiddleware bounds every request body reaching next via
// http.MaxBytesReader, so a caller cannot hold a handler open by streaming
// an unbounded body.
func maxBytesMiddleware(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

// NewServer creates a new admin server
func NewServer(cfg *config.AdminConfig, k *kernel.Kernel, reg *registry.PolicyRegistry, xds XDSSyncStatusProvider, health HealthProvider, pythonHealth PythonHealthChecker) *Server {
	mux := http.NewServeMux()

	// Register handlers
	configDumpHandler := NewConfigDumpHandler(k, reg, xds)
	xdsSyncHandler := NewXDSSyncStatusHandler(xds)
	healthHandler := NewHealthHandler(health, pythonHealth)
	mux.Handle("/config_dump", configDumpEnabledMiddleware(cfg.ConfigDump.Enabled,
		ipWhitelistMiddleware(cfg.AllowedIPs, configDumpHandler)))
	mux.Handle("/xds_sync_status", ipWhitelistMiddleware(cfg.AllowedIPs, xdsSyncHandler))
	// Health endpoint is registered without IP whitelist so Docker/k8s health probes can reach it
	mux.Handle("/health", healthHandler)

	// Go runtime profiling endpoints, registered only when explicitly enabled and
	// wrapped in the same IP whitelist as the other admin routes.
	if cfg.Pprof.Enabled {
		mux.Handle("/debug/pprof/", ipWhitelistMiddleware(cfg.AllowedIPs, http.HandlerFunc(pprof.Index)))
		mux.Handle("/debug/pprof/cmdline", ipWhitelistMiddleware(cfg.AllowedIPs, http.HandlerFunc(pprof.Cmdline)))
		mux.Handle("/debug/pprof/profile", ipWhitelistMiddleware(cfg.AllowedIPs, http.HandlerFunc(pprof.Profile)))
		mux.Handle("/debug/pprof/symbol", ipWhitelistMiddleware(cfg.AllowedIPs, http.HandlerFunc(pprof.Symbol)))
		mux.Handle("/debug/pprof/trace", ipWhitelistMiddleware(cfg.AllowedIPs, http.HandlerFunc(pprof.Trace)))
	}

	protectedMux := maxBytesMiddleware(maxAdminRequestBytes, mux)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           protectedMux,
		ReadTimeout:       adminReadTimeout,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      adminWriteTimeout,
		IdleTimeout:       adminIdleTimeout,
		MaxHeaderBytes:    adminMaxHeaderBytes,
	}

	// TLS listener is additive: served alongside, not instead of, the
	// plaintext listener above, on the same mux — every route keeps the same
	// IP-allowlist/config_dump gating regardless of which listener it's
	// reached through. Config validation (Config.Validate) already rejects a
	// bad EcdhCurves/Ciphers/protocol-version value before this ever runs in
	// production, so a parse failure here can only come from a caller that
	// bypassed validation — fail safe by leaving the TLS listener disabled
	// rather than panicking.
	var tlsServer *http.Server
	if cfg.TLS.Enabled {
		tlsConfig, err := buildAdminTLSConfig(&cfg.TLS)
		if err != nil {
			slog.Error("invalid admin.tls config, admin TLS listener disabled", "error", err)
		} else {
			tlsServer = &http.Server{
				Addr:              fmt.Sprintf(":%d", cfg.TLS.Port),
				Handler:           protectedMux,
				ReadTimeout:       adminReadTimeout,
				ReadHeaderTimeout: 30 * time.Second,
				WriteTimeout:      adminWriteTimeout,
				IdleTimeout:       adminIdleTimeout,
				MaxHeaderBytes:    adminMaxHeaderBytes,
				TLSConfig:         tlsConfig,
			}
		}
	}

	return &Server{
		cfg:        cfg,
		httpServer: httpServer,
		tlsServer:  tlsServer,
	}
}

// buildAdminTLSConfig translates an AdminTLSConfig into a tls.Config: bounded
// protocol version range, an optional cipher-suite restriction (TLS 1.2 and
// below only — TLS 1.3 suite selection isn't configurable in Go's
// crypto/tls), and the ECDH/group preference list, PQC hybrid group included
// when the operator has opted in.
func buildAdminTLSConfig(cfg *config.AdminTLSConfig) (*tls.Config, error) {
	if err := config.ValidateAdminTLSVersions(cfg.MinimumProtocolVersion, cfg.MaximumProtocolVersion); err != nil {
		return nil, err
	}
	minVersion, _ := config.ParseAdminTLSVersion(cfg.MinimumProtocolVersion)
	maxVersion, _ := config.ParseAdminTLSVersion(cfg.MaximumProtocolVersion)

	cipherSuites, err := config.ParseAdminCiphers(cfg.Ciphers)
	if err != nil {
		return nil, err
	}

	curves, err := config.ParseAdminEcdhCurves(cfg.EcdhCurves)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:       minVersion,
		MaxVersion:       maxVersion,
		CipherSuites:     cipherSuites, // nil == Go's own secure default set/order
		CurvePreferences: curves,
	}, nil
}

// Start starts the admin HTTP server(s): the TLS listener (when configured)
// is bound and its certificate loaded synchronously, up front — a bad
// cert/key or a port conflict fails this call before the plaintext listener
// ever starts accepting connections, rather than silently downgrading the
// admin API to plaintext-only with only a background log line. Once bound,
// the TLS accept loop runs in the background alongside the plaintext
// listener, which Start still blocks on, matching the previous
// single-listener behavior callers already depend on.
func (s *Server) Start(ctx context.Context) error {
	var tlsListener net.Listener
	if s.tlsServer != nil {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLS.CertPath, s.cfg.TLS.KeyPath)
		if err != nil {
			return fmt.Errorf("admin TLS server: failed to load certificate/key: %w", err)
		}
		s.tlsServer.TLSConfig.Certificates = []tls.Certificate{cert}

		tlsListener, err = net.Listen("tcp", s.tlsServer.Addr)
		if err != nil {
			return fmt.Errorf("admin TLS server: failed to bind %s: %w", s.tlsServer.Addr, err)
		}

		go func() {
			slog.InfoContext(ctx, "Starting admin TLS HTTP server", "port", s.cfg.TLS.Port)
			// Certificates are already populated above, so the empty cert/key
			// path args here are not used to (re)load from disk.
			if err := s.tlsServer.ServeTLS(tlsListener, "", ""); err != nil && err != http.ErrServerClosed {
				slog.ErrorContext(ctx, "Admin TLS server error", "error", err)
			}
		}()
	}

	slog.InfoContext(ctx, "Starting admin HTTP server",
		"port", s.cfg.Port,
		"allowed_ips", s.cfg.AllowedIPs)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("admin server error: %w", err)
	}

	return nil
}

// Stop gracefully stops the admin HTTP server(s)
func (s *Server) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Stopping admin HTTP server")
	err := s.httpServer.Shutdown(ctx)
	if s.tlsServer != nil {
		if tlsErr := s.tlsServer.Shutdown(ctx); tlsErr != nil && err == nil {
			err = tlsErr
		}
	}
	return err
}

// configDumpEnabledMiddleware gates /config_dump behind an explicit enable flag
// that is independent of the admin server's own Enabled flag, so /health (relied
// on by Docker/k8s health probes) keeps working even when config_dump is off.
// Disabled by default; returns 404 rather than a payload when disabled.
func configDumpEnabledMiddleware(enabled bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ipWhitelistMiddleware creates a middleware that checks if the request IP is in the allowed list
func ipWhitelistMiddleware(allowedIPs []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP
		clientIP := extractClientIP(r)

		// Check if IP is allowed
		if !isIPAllowed(clientIP, allowedIPs) {
			slog.Warn("Blocked admin request from unauthorized IP",
				"client_ip", sanitizeLogValue(clientIP),
				"path", sanitizeLogValue(r.URL.Path))
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// IP is allowed, proceed to the handler
		next.ServeHTTP(w, r)
	})
}

// extractClientIP extracts the client IP from the request.
// For security, prefer RemoteAddr for direct connections.
// Proxy headers should only be trusted in controlled environments.
func extractClientIP(r *http.Request) string {
	// Use RemoteAddr as the authoritative source for admin endpoints
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isIPAllowed checks if the given IP is in the allowed list
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	for _, allowedIP := range allowedIPs {
		// Support wildcard to allow any IP
		if allowedIP == "*" || allowedIP == "0.0.0.0/0" {
			return true
		}
		if clientIP == allowedIP {
			return true
		}
	}
	return false
}

func sanitizeLogValue(value string) string {
	return strings.NewReplacer(
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	).Replace(value)
}
