package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/policy-engine/policy-engine/internal/config"
	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/registry"
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

// extractClientIP extracts the client IP from the request
func extractClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first (for proxied requests)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Try X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isIPAllowed checks if the given IP is in the allowed list
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	for _, allowedIP := range allowedIPs {
		if clientIP == allowedIP {
			return true
		}
	}
	return false
}
