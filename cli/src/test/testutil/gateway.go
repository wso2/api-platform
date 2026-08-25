package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewGatewayServer starts an httptest.Server standing in for a gateway-controller
// management API, for use by gateway CLI command tests (e.g. cmd/gateway/...).
// Mirrors NewDevPortalServer's shape for the gateway-facing command tree.
func NewGatewayServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}
