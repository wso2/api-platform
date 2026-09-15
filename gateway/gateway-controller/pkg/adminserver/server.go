package adminserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// AdminAPIBasePath is the URL prefix under which the gateway-controller admin API
// is served. It must stay in sync with `servers.url` in api/admin-openapi.yaml.
const AdminAPIBasePath = "/api/admin/v1"

type apiServer interface {
	// ConfigDumpJSON returns the configuration dump with resolved secret values
	// redacted. The admin server must not marshal the response struct itself:
	// the dump carries rendered configuration, so a plaintext upstream
	// credential would be served verbatim.
	ConfigDumpJSON(log *slog.Logger) ([]byte, error)
	GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse
}

// adminMcpRelPath is the MCP operation's path in api/admin-openapi.yaml
const adminMcpRelPath = "/mcp"

// MCPConfig carries everything the administrative MCP endpoint needs, expressed
// purely as http.Handler and middleware values. Passed to NewServer; nil means
// the MCP endpoint is disabled and the generated /mcp route answers 404.
type MCPConfig struct {
	// Handler serves POST <AdminAPIBasePath>/mcp
	Handler http.Handler

	// Challenge attaches the RFC 6750 WWW-Authenticate challenge that lets an
	// MCP client start, or step up, an OAuth flow. It arrives already scoped
	// to the MCP route, and is appended to the shared middleware list.
	Challenge func(http.Handler) http.Handler

	// Metadata serves the RFC 9728 protected resource document. Registered
	// outside the authentication chain, a client with no token has to read it
	// to begin the OAuth flow
	Metadata http.HandlerFunc
}

// Server is the controller admin HTTP server for debug endpoints.
type Server struct {
	cfg        *config.AdminServerConfig
	apiServer  apiServer
	httpSrv    *http.Server
	logger     *slog.Logger
	mcpHandler http.Handler
}

// NewServer creates a new admin HTTP server.
//
// authMiddleware is the protection chain applied to every admin endpoint except
// the public health probe. In production it composes the shared authentication
// middleware (basic auth / IDP) with a deny-by-default admin-role authorization
// check, so callers must both present valid credentials and hold the "admin" role.
// A nil authMiddleware disables that protection entirely and is intended only for
// tests that exercise the handlers or IP allowlist in isolation — production
// callers must always pass a configured chain.
// mcp configures the administrative MCP endpoint; nil leaves it disabled.
func NewServer(cfg *config.AdminServerConfig, apiServer apiServer, authMiddleware func(http.Handler) http.Handler, logger *slog.Logger, mcp *MCPConfig) *Server {
	s := &Server{
		cfg:       cfg,
		apiServer: apiServer,
		logger:    logger,
	}

	// Authentication is the primary gate; the IP allowlist is defense-in-depth.
	// The health probe bypasses both so Docker/k8s liveness checks
	// keep working without credentials.
	if authMiddleware == nil {
		logger.Warn("admin server starting WITHOUT authentication — every non-health endpoint " +
			"(config_dump, xds_sync_status, pprof) is reachable without credentials; this is only " +
			"safe in tests. Configure controller.auth.basic and pass the auth middleware.")
	}
	authMW := createSelectiveAuthMiddleware(authMiddleware)

	// Middleware shared by every generated route. The MCP challenge, when
	// present, is appended last so it is outermost and can decorate the 401
	// the authenticator writes. It arrives from main.go already scoped to the
	// MCP route patterns, so REST 401s are untouched.
	versionedMW := []adminapi.MiddlewareFunc{
		createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs),
		authMW,
	}
	legacyMW := []adminapi.MiddlewareFunc{
		createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs),
		deprecatedAdminPathMiddleware(AdminAPIBasePath),
		authMW,
	}

	if mcp != nil && mcp.Handler != nil {
		s.mcpHandler = mcp.Handler
		if mcp.Challenge != nil {
			versionedMW = append(versionedMW, mcp.Challenge)
			legacyMW = append(legacyMW, mcp.Challenge)
		}
	}

	// Share a single mux so both registrations populate the same router.
	mux := http.NewServeMux()

	// Versioned admin API routes — the current, non-deprecated form.
	// BaseURL must match the `servers.url` prefix in api/admin-openapi.yaml.
	// The auth middleware is placed after the IP check so it is the outer
	// wrapper and runs first — an unauthenticated request is rejected before
	// the IP check.
	adminapi.HandlerWithOptions(s, adminapi.StdHTTPServerOptions{
		BaseURL:     AdminAPIBasePath,
		BaseRouter:  mux,
		Middlewares: versionedMW,
	})

	// Legacy unprefixed admin routes for backwards compatibility. These are
	// deprecated; responses carry RFC 8594 headers pointing at the versioned
	// paths. Remove once all clients (docker-compose healthchecks, older
	// kubelet probes, etc.) have been migrated.
	adminapi.HandlerWithOptions(s, adminapi.StdHTTPServerOptions{
		BaseURL:     "",
		BaseRouter:  mux,
		Middlewares: legacyMW,
	})

	// Go runtime profiling endpoints, registered only when explicitly enabled.
	// They are wrapped in the same auth + IP whitelist as the other admin routes —
	// the middleware here (not on the mux itself) is what protects them, so
	// registering directly on the mux without it would leave pprof unauthenticated.
	if cfg.Pprof.Enabled {
		ipmw := createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs)
		protect := func(h http.Handler) http.Handler { return authMW(ipmw(h)) }
		mux.Handle("/debug/pprof/", protect(http.HandlerFunc(pprof.Index)))
		mux.Handle("/debug/pprof/cmdline", protect(http.HandlerFunc(pprof.Cmdline)))
		mux.Handle("/debug/pprof/profile", protect(http.HandlerFunc(pprof.Profile)))
		mux.Handle("/debug/pprof/symbol", protect(http.HandlerFunc(pprof.Symbol)))
		mux.Handle("/debug/pprof/trace", protect(http.HandlerFunc(pprof.Trace)))
	}

	// The OAuth discovery document. A client with no token reads this to learn
	// where to log in, so it is registered WITHOUT the auth middleware. The IP
	// allowlist still applies. The MCP route itself is registered by the
	// generated router above.
	if mcp != nil && mcp.Metadata != nil {
		ipmw := createSelectiveIPWhitelistMiddleware(cfg.AllowedIPs)
		mux.Handle("GET /.well-known/oauth-protected-resource"+AdminAPIBasePath+adminMcpRelPath,
			ipmw(mcp.Metadata))
	}

	if s.mcpHandler != nil {
		logger.Info("Administrative MCP endpoint enabled",
			slog.String("path", AdminAPIBasePath+adminMcpRelPath))
	}

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
	if !s.cfg.ConfigDump.Enabled {
		http.NotFound(w, r)
		return
	}

	body, err := s.apiServer.ConfigDumpJSON(s.logger)
	if err != nil {
		http.Error(w, "Failed to retrieve configuration dump", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// GetXDSSyncStatus implements adminapi.ServerInterface.
func (s *Server) GetXDSSyncStatus(w http.ResponseWriter, r *http.Request) {
	resp := s.apiServer.GetXDSSyncStatusResponse()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleAdminMcp implements adminapi.ServerInterface. It answers 404 when the
// MCP endpoint is disabled, so a gateway with MCP switched off is
// indistinguishable from one that does not implement it.
func (s *Server) HandleAdminMcp(w http.ResponseWriter, r *http.Request) {
	if s.mcpHandler == nil {
		http.NotFound(w, r)
		return
	}
	s.mcpHandler.ServeHTTP(w, r)
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

// createSelectiveAuthMiddleware wraps the shared authentication middleware so the
// public health endpoints (versioned and legacy) bypass authentication — matching
// the exemption applied by createSelectiveIPWhitelistMiddleware — while every other
// admin endpoint (config_dump, xds_sync_status, pprof) requires valid credentials.
//
// The health paths are matched exactly (not by prefix) so an unrelated path can
// never be mistaken for the public probe. When
// authMiddleware is nil the returned wrapper is a passthrough; production callers
// must supply a configured middleware (see NewServer).
func createSelectiveAuthMiddleware(authMiddleware func(http.Handler) http.Handler) adminapi.MiddlewareFunc {
	healthPath := AdminAPIBasePath + "/health"
	const legacyHealthPath = "/health"
	return func(next http.Handler) http.Handler {
		// Build the authenticated handler once per wrapped route. When no
		// authenticator is wired, fall through to the unauthenticated handler.
		authenticated := next
		if authMiddleware != nil {
			authenticated = authMiddleware(next)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Health probes must stay reachable without credentials.
			if r.URL.Path == healthPath || r.URL.Path == legacyHealthPath {
				next.ServeHTTP(w, r)
				return
			}
			authenticated.ServeHTTP(w, r)
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
