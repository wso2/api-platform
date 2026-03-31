package adminserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

type stubAPIServer struct {
	configDump  adminapi.ConfigDumpResponse
	configErr   error
	xdsResponse adminapi.XDSSyncStatusResponse
}

func (s *stubAPIServer) BuildConfigDumpResponse(_ *slog.Logger) (*adminapi.ConfigDumpResponse, error) {
	if s.configErr != nil {
		return nil, s.configErr
	}
	return &s.configDump, nil
}

func (s *stubAPIServer) GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse {
	return s.xdsResponse
}

func TestAdminServer_ConfigDumpHandler(t *testing.T) {
	status := "ok"
	stub := &stubAPIServer{
		configDump: adminapi.ConfigDumpResponse{Status: &status},
	}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body adminapi.ConfigDumpResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotNil(t, body.Status)
	assert.Equal(t, "ok", *body.Status)
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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/xds_sync_status", nil)
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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/xds_sync_status", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminServer_MethodNotAllowed(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/config_dump", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminServer_HealthHandler(t *testing.T) {
	stub := &stubAPIServer{}
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
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
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"*"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	s.httpSrv.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminServer_HealthHandler_NoIPWhitelist(t *testing.T) {
	stub := &stubAPIServer{}
	// Restrict IPs to only 127.0.0.1 — health should still be accessible from other IPs
	s := NewServer(&config.AdminServerConfig{Port: 9092, AllowedIPs: []string{"127.0.0.1"}}, stub, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
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
