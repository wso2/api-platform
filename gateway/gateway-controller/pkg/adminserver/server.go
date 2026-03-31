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

const adminAPIBasePath = "/api/v1"

// NewServer creates a new admin HTTP server.
func NewServer(cfg *config.AdminServerConfig, apiServer apiServer, logger *slog.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		apiServer: apiServer,
		logger:    logger,
	}

	// Use generated handler with IP whitelist middleware for protected endpoints
	handler := adminapi.HandlerWithOptions(s, adminapi.StdHTTPServerOptions{
		BaseURL: adminAPIBasePath,
		Middlewares: []adminapi.MiddlewareFunc{
			createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs),
		},
	})

	s.httpSrv = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
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
// to all endpoints except /api/v1/health (which must be accessible for Docker/k8s health probes).
func createSelectiveIPWhitelistMiddleware(allowedIPs []string) adminapi.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip IP whitelist for health endpoint
			if r.URL.Path == adminAPIBasePath+"/health" {
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
