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
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// Server is the admin HTTP server
type Server struct {
	cfg        *config.AdminConfig
	httpServer *http.Server
}

// NewServer creates a new admin server
func NewServer(cfg *config.AdminConfig, k *kernel.Kernel, reg *registry.PolicyRegistry) *Server {
	mux := http.NewServeMux()

	// Register handlers
	configDumpHandler := NewConfigDumpHandler(k, reg)
	mux.Handle("/config_dump", ipWhitelistMiddleware(cfg.AllowedIPs, configDumpHandler))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return &Server{
		cfg:        cfg,
		httpServer: httpServer,
	}
}

// Start starts the admin HTTP server
func (s *Server) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Starting admin HTTP server",
		"port", s.cfg.Port,
		"allowed_ips", s.cfg.AllowedIPs)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("admin server error: %w", err)
	}

	return nil
}

// Stop gracefully stops the admin HTTP server
func (s *Server) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Stopping admin HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// ipWhitelistMiddleware creates a middleware that checks if the request IP is in the allowed list
func ipWhitelistMiddleware(allowedIPs []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP
		clientIP := extractClientIP(r)

		// Check if IP is allowed
		if !isIPAllowed(clientIP, allowedIPs) {
			slog.Warn("Blocked admin request from unauthorized IP",
				"client_ip", clientIP,
				"path", r.URL.Path)
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
