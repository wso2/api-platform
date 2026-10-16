package adminserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

const (
	testAdminUser = "admin"
	testAdminPass = "s3cret"
)

// newBasicAuthMiddleware builds a real basic-auth middleware (single admin user)
// mirroring how the management API wires authenticators.AuthMiddleware, so the
// admin-server auth tests exercise the same code path production uses.
func newBasicAuthMiddleware(t *testing.T) func(http.Handler) http.Handler {
	t.Helper()
	return newBasicAuthMiddlewareWithRoles(t, []string{"admin"})
}

// newBasicAuthMiddlewareWithRoles builds an authentication-only middleware for a
// single user (testAdminUser/testAdminPass) carrying the given roles.
func newBasicAuthMiddlewareWithRoles(t *testing.T, roles []string) func(http.Handler) http.Handler {
	t.Helper()
	mw, err := authenticators.AuthMiddleware(commonmodels.AuthConfig{
		BasicAuth: &commonmodels.BasicAuth{
			Enabled: true,
			Users: []commonmodels.User{
				{Username: testAdminUser, Password: testAdminPass, Roles: roles},
			},
		},
	}, slog.Default())
	require.NoError(t, err)
	return mw
}

// newAdminProtectMiddleware composes authentication with the same deny-by-default
// admin-role authorization the controller wires in production (auth outermost so it
// populates the context that authz consumes), for a single user carrying the given
// roles. It mirrors cmd/controller/main.go's adminProtect + adminResourceRoles().
func newAdminProtectMiddleware(t *testing.T, roles []string) func(http.Handler) http.Handler {
	t.Helper()
	authMW := newBasicAuthMiddlewareWithRoles(t, roles)
	authzMW := authenticators.AuthorizationMiddleware(commonmodels.AuthConfig{
		ResourceRoles: map[string][]string{
			"GET " + AdminAPIBasePath + "/config_dump":     {"admin"},
			"GET " + AdminAPIBasePath + "/xds_sync_status": {"admin"},
		},
	}, slog.Default())
	return func(next http.Handler) http.Handler {
		return authMW(authzMW(next))
	}
}

// newTestServer builds an admin server whose API server is already set, as it is once
// the controller has finished starting.
func newTestServer(cfg *config.AdminServerConfig, api apiServer, health http.Handler,
	authMiddleware func(http.Handler) http.Handler, logger *slog.Logger) *Server {
	s := NewServer(cfg, health, authMiddleware, logger)
	s.SetAPIServer(api)
	return s
}

// okHealth stands in for the health checker's handler: a healthy answer with the
// controller's healthy status value.
var okHealth = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","component":"gateway-controller","timestamp":"2026-01-01T00:00:00Z","checks":{"database":{"status":"up"}}}`))
})

// downHealth stands in for the health checker's handler while the database is down.
var downHealth = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"status":"unhealthy","component":"gateway-controller","timestamp":"2026-01-01T00:00:00Z","checks":{"database":{"status":"down","error":"database unreachable","since":"2026-01-01T00:00:00Z"}}}`))
})

type stubAPIServer struct {
	configDump  adminapi.ConfigDumpResponse
	configErr   error
	xdsResponse adminapi.XDSSyncStatusResponse
}

func (s *stubAPIServer) ConfigDumpJSON(_ *slog.Logger) ([]byte, error) {
	if s.configErr != nil {
		return nil, s.configErr
	}
	return json.Marshal(s.configDump)
}

func (s *stubAPIServer) GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse {
	return s.xdsResponse
}

func TestAdminServer_ConfigDumpHandler(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{
		configDump: adminapi.ConfigDumpResponse{Status: &status},
	}
	s := newTestServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body adminapi.ConfigDumpResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotNil(t, body.Status)
	assert.Equal(t, "ok", *body.Status)
}

func TestAdminServer_ConfigDumpHandler_DisabledByDefault(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{
		configDump: adminapi.ConfigDumpResponse{Status: &status},
	}
	// ConfigDump.Enabled left at its zero value (false) — matches the production default.
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdminServer_XDSSyncStatusHandler(t *testing.T) {
	component := "gateway-controller"
	version := "12"
	now := time.Now()
	stub := &stubAPIServer{
		xdsResponse: adminapi.XDSSyncStatusResponse{
			Component:          &component,
			PolicyChainVersion: &version,
			Timestamp:          &now,
		},
	}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/xds_sync_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body adminapi.XDSSyncStatusResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotNil(t, body.PolicyChainVersion)
	assert.Equal(t, "12", *body.PolicyChainVersion)
}

func TestAdminServer_IPAllowlist(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/xds_sync_status", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminServer_MethodNotAllowed(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodPost, AdminAPIBasePath+"/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminServer_HealthHandler(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])
	assert.NotEmpty(t, body["timestamp"])
}

func TestAdminServer_HealthHandler_MethodNotAllowed(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodPost, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminServer_HealthHandler_NoIPWhitelist(t *testing.T) {
	stub := &stubAPIServer{}
	// Restrict IPs to only 127.0.0.1 — health should still be accessible from other IPs
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestIsIPAllowed(t *testing.T) {
	assert.True(t, isIPAllowed("127.0.0.1", []string{"*"}))
	assert.True(t, isIPAllowed("127.0.0.1", []string{"0.0.0.0/0"}))
	assert.True(t, isIPAllowed("127.0.0.1", []string{"127.0.0.1"}))
	assert.False(t, isIPAllowed("127.0.0.1", []string{"10.0.0.1"}))
}

// Legacy (deprecated) route tests — exercised to ensure backwards
// compatibility while the unprefixed paths remain supported.

func TestAdminServer_LegacyHealthHandler_NoIPWhitelist(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "true", rr.Header().Get("Deprecation"))
	assert.Contains(t, rr.Header().Get("Link"), AdminAPIBasePath+"/health")
}

func TestAdminServer_LegacyConfigDump(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{
		configDump: adminapi.ConfigDumpResponse{Status: &status},
	}
	s := newTestServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "true", rr.Header().Get("Deprecation"))
	assert.Contains(t, rr.Header().Get("Link"), AdminAPIBasePath+"/config_dump")

	var body adminapi.ConfigDumpResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotNil(t, body.Status)
	assert.Equal(t, "ok", *body.Status)
}

func TestAdminServer_LegacyXDSSyncStatus(t *testing.T) {
	component := "gateway-controller"
	version := "12"
	now := time.Now()
	stub := &stubAPIServer{
		xdsResponse: adminapi.XDSSyncStatusResponse{
			Component:          &component,
			PolicyChainVersion: &version,
			Timestamp:          &now,
		},
	}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/xds_sync_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "true", rr.Header().Get("Deprecation"))
}

func TestAdminServer_VersionedPathsHaveNoDeprecationHeader(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Header().Get("Deprecation"))
	assert.Empty(t, rr.Header().Get("Link"))
}

// Authentication tests — the admin server must require basic auth on every
// endpoint except the public health probe (F8 / GO-AUTH-013).

func TestAdminServer_ConfigDump_RequiresAuth(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "config_dump must reject unauthenticated requests")
}

func TestAdminServer_ConfigDump_WrongCredentials(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.SetBasicAuth(testAdminUser, "wrong-password")
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "config_dump must reject wrong credentials")
}

func TestAdminServer_ConfigDump_WithValidAuth(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := newTestServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body adminapi.ConfigDumpResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotNil(t, body.Status)
	assert.Equal(t, "ok", *body.Status)
}

func TestAdminServer_XDSSyncStatus_RequiresAuth(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/xds_sync_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "xds_sync_status must reject unauthenticated requests")
}

func TestAdminServer_Health_PublicWithAuthEnabled(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	// No credentials supplied — the health probe must still succeed so container
	// and kubelet liveness checks keep working.
	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])
}

func TestAdminServer_LegacyHealth_PublicWithAuthEnabled(t *testing.T) {
	stub := &stubAPIServer{}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "legacy health path must stay public")
}

func TestAdminServer_LegacyConfigDump_RequiresAuth(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth, newBasicAuthMiddleware(t), slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "legacy config_dump must require auth too")
}

// Authorization tests — a valid credential is not enough; the caller must also
// hold the "admin" role (deny-by-default, GO-AUTH-007).

func TestAdminServer_ConfigDump_AdminRoleAllowed(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := newTestServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, okHealth,
		newAdminProtectMiddleware(t, []string{"admin"}), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "authenticated admin must be allowed")
}

func TestAdminServer_ConfigDump_NonAdminForbidden(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth,
		newAdminProtectMiddleware(t, []string{"developer"}), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass) // valid credentials, wrong role
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code, "authenticated non-admin must be forbidden")
}

func TestAdminServer_Health_PublicRegardlessOfRole(t *testing.T) {
	stub := &stubAPIServer{}
	// Even a non-admin (in fact, no credentials) must reach the health probe.
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, okHealth,
		newAdminProtectMiddleware(t, []string{"developer"}), slog.Default())

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "health must bypass both authn and authz")
}

func TestAdminServer_HealthServedByHealthHandlerOnBothPaths(t *testing.T) {
	// An allowlist that excludes the caller and an auth middleware that rejects
	// everything: the health probe must bypass both on either path.
	rejectAll := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}},
		&stubAPIServer{}, downHealth, rejectAll, slog.Default())

	for _, path := range []string{AdminAPIBasePath + "/health", "/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.168.1.10:12345"
		rr := httptest.NewRecorder()

		s.httpSrv.Handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code, path)
		assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"), path)
		var body struct {
			Status string `json:"status"`
			Checks map[string]struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"checks"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&body), path)
		assert.Equal(t, "unhealthy", body.Status, path)
		assert.Equal(t, "down", body.Checks["database"].Status, path)
		assert.Equal(t, "database unreachable", body.Checks["database"].Error, path)
	}
}

func TestAdminServer_LegacyHealthKeepsDeprecationHeadersWhenUnhealthy(t *testing.T) {
	s := newTestServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}},
		&stubAPIServer{}, downHealth, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "true", rr.Header().Get("Deprecation"))
	assert.Contains(t, rr.Header().Get("Link"), AdminAPIBasePath+"/health")
	assert.NotEmpty(t, rr.Header().Get("Warning"))
}

func TestAdminServer_DebugEndpointsUnavailableUntilAPIServerSet(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, downHealth, newBasicAuthMiddleware(t), slog.Default())
	get := func(path string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authenticated {
			req.SetBasicAuth(testAdminUser, testAdminPass)
		}
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		s.httpSrv.Handler.ServeHTTP(rr, req)
		return rr
	}
	paths := []string{
		AdminAPIBasePath + "/config_dump", "/config_dump",
		AdminAPIBasePath + "/xds_sync_status", "/xds_sync_status",
	}

	for _, path := range paths {
		assert.Equal(t, http.StatusUnauthorized, get(path, false).Code,
			"%s: authentication must still be checked first while starting", path)
		assert.Equal(t, http.StatusServiceUnavailable, get(path, true).Code,
			"%s: must be unavailable until the controller has started", path)
	}
	assert.Equal(t, http.StatusServiceUnavailable, get(AdminAPIBasePath+"/health", false).Code,
		"health is served while starting")

	s.SetAPIServer(stub)

	for _, path := range paths {
		assert.Equal(t, http.StatusOK, get(path, true).Code, "%s: available once started", path)
	}
}

func TestAdminServer_SetAPIServerWhileServing(t *testing.T) {
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, okHealth, nil, slog.Default())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/xds_sync_status", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rr := httptest.NewRecorder()
			s.httpSrv.Handler.ServeHTTP(rr, req)
			assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, rr.Code)
		}
	}()

	s.SetAPIServer(&stubAPIServer{})
	<-done
}
