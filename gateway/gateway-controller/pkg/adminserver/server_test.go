package adminserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
			"POST " + AdminAPIBasePath + "/mcp":            {"admin"},
			"POST /mcp":                                    {"admin"},
		},
	}, slog.Default())
	return func(next http.Handler) http.Handler {
		return authMW(authzMW(next))
	}
}

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
	s := NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, nil, slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/xds_sync_status", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminServer_MethodNotAllowed(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

	req := httptest.NewRequest(http.MethodPost, AdminAPIBasePath+"/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminServer_HealthHandler(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])
	assert.NotEmpty(t, body["timestamp"])
}

func TestAdminServer_HealthHandler_MethodNotAllowed(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

	req := httptest.NewRequest(http.MethodPost, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminServer_HealthHandler_NoIPWhitelist(t *testing.T) {
	stub := &stubAPIServer{}
	// Restrict IPs to only 127.0.0.1 — health should still be accessible from other IPs
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/xds_sync_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "true", rr.Header().Get("Deprecation"))
}

func TestAdminServer_VersionedPathsHaveNoDeprecationHeader(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, nil, slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "config_dump must reject unauthenticated requests")
}

func TestAdminServer_ConfigDump_WrongCredentials(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/xds_sync_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "xds_sync_status must reject unauthenticated requests")
}

func TestAdminServer_Health_PublicWithAuthEnabled(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

	// No credentials supplied — the health probe must still succeed so container
	// and kubelet liveness checks keep working.
	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])
}

func TestAdminServer_LegacyHealth_PublicWithAuthEnabled(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "legacy health path must stay public")
}

func TestAdminServer_LegacyConfigDump_RequiresAuth(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{configDump: adminapi.ConfigDumpResponse{Status: &status}}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, newBasicAuthMiddleware(t), slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}, stub,
		newAdminProtectMiddleware(t, []string{"admin"}), slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub,
		newAdminProtectMiddleware(t, []string{"developer"}), slog.Default(), nil)

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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub,
		newAdminProtectMiddleware(t, []string{"developer"}), slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "health must bypass both authn and authz")
}

// Administrative MCP endpoint

// stubMCPHandler stands in for the real MCP handler: these tests exercise the
// transport chain the admin server wraps it in (IP allowlist, OAuth challenge,
// authentication, authorization), not the MCP protocol itself.
func stubMCPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})
}

// challengeMiddleware mirrors handlers.MCPChallengeMiddleware closely enough to
// pin the ordering: it decorates any 401/403 with a Bearer challenge, and only
// when the header is not already set.
func challengeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&challengeRecorder{ResponseWriter: w}, r)
	})
}

type challengeRecorder struct {
	http.ResponseWriter
	written bool
}

func (c *challengeRecorder) WriteHeader(status int) {
	if !c.written {
		c.written = true
		if (status == http.StatusUnauthorized || status == http.StatusForbidden) &&
			c.Header().Get("WWW-Authenticate") == "" {
			c.Header().Set("WWW-Authenticate",
				`Bearer resource_metadata="https://gw.example.com/.well-known/oauth-protected-resource"`)
		}
	}
	c.ResponseWriter.WriteHeader(status)
}

func (c *challengeRecorder) Write(b []byte) (int, error) {
	if !c.written {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(b)
}

func newMCPTestServer(t *testing.T, allowedIPs []string, roles []string) *Server {
	t.Helper()
	return NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: allowedIPs,
	}, &stubAPIServer{}, newAdminProtectMiddleware(t, roles), slog.Default(),
		&MCPConfig{
			Handler:   stubMCPHandler(),
			Challenge: challengeMiddleware,
			Metadata: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"resource":"https://gw.example.com/api/admin/v1/mcp",` +
					`"authorization_servers":["https://idp.example.com"]}`))
			},
		})
}

func mcpRequest(t *testing.T, s *Server, remoteAddr string, withCreds bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, AdminAPIBasePath+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	if withCreds {
		req.SetBasicAuth(testAdminUser, testAdminPass)
	}
	rec := httptest.NewRecorder()
	s.httpSrv.Handler.ServeHTTP(rec, req)
	return rec
}

func TestAdminServer_MCP_DisabledReturns404(t *testing.T) {
	s := NewServer(&config.AdminServerConfig{
		Port:       9092,
		AllowedIPs: []string{"*"},
	}, &stubAPIServer{}, newAdminProtectMiddleware(t, []string{"admin"}), slog.Default(), nil)

	rec := mcpRequest(t, s, "", true)

	// With a nil MCPConfig the generated route exists but HandleAdminMcp has no
	// handler, so it answers 404 — a gateway with the endpoint switched off is
	// indistinguishable from one that never implemented it.
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminServer_MCP_RequiresAuth(t *testing.T) {
	s := newMCPTestServer(t, []string{"*"}, []string{"admin"})

	rec := mcpRequest(t, s, "", false)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	challenge := rec.Header().Get("WWW-Authenticate")
	assert.True(t, strings.HasPrefix(challenge, "Bearer "), "got %q", challenge)
	assert.Contains(t, challenge, "resource_metadata=")
}

func TestAdminServer_MCP_NonAdminForbidden(t *testing.T) {
	s := newMCPTestServer(t, []string{"*"}, []string{"developer"})

	rec := mcpRequest(t, s, "", true)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Bearer ")
}

func TestAdminServer_MCP_AdminAllowed(t *testing.T) {
	s := newMCPTestServer(t, []string{"*"}, []string{"admin"})

	rec := mcpRequest(t, s, "", true)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "jsonrpc")
}

func TestAdminServer_MCP_MetadataIsUnauthenticated(t *testing.T) {
	s := newMCPTestServer(t, []string{"*"}, []string{"admin"})

	req := httptest.NewRequest(http.MethodGet,
		"/.well-known/oauth-protected-resource"+AdminAPIBasePath+"/mcp", nil)
	rec := httptest.NewRecorder()
	s.httpSrv.Handler.ServeHTTP(rec, req)

	// A client with no token has to read this to begin the OAuth flow.
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "authorization_servers")
}

func TestAdminServer_MCP_MetadataStillIPGated(t *testing.T) {
	s := newMCPTestServer(t, []string{"10.0.0.1"}, []string{"admin"})

	req := httptest.NewRequest(http.MethodGet,
		"/.well-known/oauth-protected-resource"+AdminAPIBasePath+"/mcp", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	s.httpSrv.Handler.ServeHTTP(rec, req)

	// Unauthenticated is not unrestricted: a client that cannot reach the
	// endpoint gains nothing from reading how to authenticate to it.
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestAdminServer_MCP_LegacyAliasDeprecated pins that the generated router's
// unprefixed alias works and carries the RFC 8594 deprecation header, exactly
// as the other legacy admin routes do.
func TestAdminServer_MCP_LegacyAliasDeprecated(t *testing.T) {
	s := newMCPTestServer(t, []string{"*"}, []string{"admin"})

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	rec := httptest.NewRecorder()
	s.httpSrv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "true", rec.Header().Get("Deprecation"))
}

func TestAdminServer_MCP_MethodNotAllowed(t *testing.T) {
	s := newMCPTestServer(t, []string{"*"}, []string{"admin"})

	req := httptest.NewRequest(http.MethodGet, AdminAPIBasePath+"/mcp", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	rec := httptest.NewRecorder()
	s.httpSrv.Handler.ServeHTTP(rec, req)

	// Only POST is registered on the mux.
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
