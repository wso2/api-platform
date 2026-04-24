package adminserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// AdminAPIBasePath is the URL prefix under which the gateway-controller admin API
// is served. It must stay in sync with `servers.url` in api/admin-openapi.yaml.
const AdminAPIBasePath = "/api/admin/v0.9"

type apiServer interface {
	BuildConfigDumpResponse(log *slog.Logger) (*adminapi.ConfigDumpResponse, error)
	GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse
}

// Server is the controller admin HTTP server for debug endpoints.
type Server struct {
	cfg       *config.AdminServerConfig
	apiServer apiServer
	httpSrv   *http.Server
	logger    *slog.Logger
}

// NewServer creates a new admin HTTP server.
func NewServer(cfg *config.AdminServerConfig, apiServer apiServer, logger *slog.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		apiServer: apiServer,
		logger:    logger,
	}

	// Share a single mux so both registrations populate the same router.
	mux := http.NewServeMux()

	// Versioned admin API routes — the current, non-deprecated form.
	// BaseURL must match the `servers.url` prefix in api/admin-openapi.yaml.
	adminapi.HandlerWithOptions(s, adminapi.StdHTTPServerOptions{
		BaseURL:    AdminAPIBasePath,
		BaseRouter: mux,
		Middlewares: []adminapi.MiddlewareFunc{
			createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs),
		},
	})

	// Legacy unprefixed admin routes for backwards compatibility. These are
	// deprecated; responses carry RFC 8594 headers pointing at the versioned
	// paths. Remove once all clients (docker-compose healthchecks, older
	// kubelet probes, etc.) have been migrated.
	adminapi.HandlerWithOptions(s, adminapi.StdHTTPServerOptions{
		BaseURL:    "",
		BaseRouter: mux,
		Middlewares: []adminapi.MiddlewareFunc{
			createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs),
			deprecatedAdminPathMiddleware(AdminAPIBasePath),
		},
	})

	s.httpSrv = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	return s
}

// Start starts the admin HTTP server in a blocking manner.
func (s *Server) Start() error {
	s.logger.Info("Starting controller admin HTTP server",
		slog.Int("port", s.cfg.Port),
		slog.Any("allowed_ips", s.cfg.AllowedIPs))
	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("admin server error: %w", err)
	}
	return nil
}

// Stop gracefully stops the admin HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping controller admin HTTP server")
	return s.httpSrv.Shutdown(ctx)
}

// GetConfigDump implements adminapi.ServerInterface.
func (s *Server) GetConfigDump(w http.ResponseWriter, r *http.Request) {
	resp, err := s.apiServer.BuildConfigDumpResponse(s.logger)
	if err != nil {
		http.Error(w, "Failed to retrieve configuration dump", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetXDSSyncStatus implements adminapi.ServerInterface.
func (s *Server) GetXDSSyncStatus(w http.ResponseWriter, r *http.Request) {
	resp := s.apiServer.GetXDSSyncStatusResponse()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetHealth implements adminapi.ServerInterface.
func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// createSelectiveIPWhitelistMiddleware creates a middleware that applies IP whitelist
// to all endpoints except /health (which must be accessible for Docker/k8s health probes).
// Both the versioned (AdminAPIBasePath+"/health") and the deprecated legacy
// ("/health") variants are exempt while legacy support is retained.
func createSelectiveIPWhitelistMiddleware(allowedIPs []string) adminapi.MiddlewareFunc {
	healthPath := AdminAPIBasePath + "/health"
	const legacyHealthPath = "/health"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip IP whitelist for health endpoints (versioned and legacy).
			if r.URL.Path == healthPath || r.URL.Path == legacyHealthPath {
				next.ServeHTTP(w, r)
				return
			}

			// Apply IP whitelist for all other endpoints
			clientIP := extractClientIP(r)
			if !isIPAllowed(clientIP, allowedIPs) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// deprecatedAdminPathMiddleware marks responses served on the legacy unprefixed
// admin API paths as deprecated per RFC 8594 (`Deprecation` header) and points
// clients at the versioned successor via a `Link` header. It should be attached
// only to the legacy registration; versioned requests bypass it.
func deprecatedAdminPathMiddleware(newBasePath string) adminapi.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			successor := newBasePath + r.URL.Path
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", successor))
			w.Header().Set("Warning",
				fmt.Sprintf("299 - \"Deprecated API: migrate to %s prefix\"", newBasePath))
			next.ServeHTTP(w, r)
		})
	}
}

func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isIPAllowed(clientIP string, allowedIPs []string) bool {
	for _, allowedIP := range allowedIPs {
		if allowedIP == "*" || allowedIP == "0.0.0.0/0" {
			return true
		}
		if clientIP == allowedIP {
			return true
		}
	}
	return false
}
