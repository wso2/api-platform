/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
)

type stubAdminStatus struct {
	xds        adminapi.XDSSyncStatusResponse
	configDump adminapi.ConfigDumpResponse
	configErr  error
}

func (s *stubAdminStatus) GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse {
	return s.xds
}

func (s *stubAdminStatus) BuildConfigDumpResponse(_ *slog.Logger) (*adminapi.ConfigDumpResponse, error) {
	if s.configErr != nil {
		return nil, s.configErr
	}
	return &s.configDump, nil
}

func newAdminTestHandler(configDumpEnabled bool) *AdminMcpHandler {
	return NewAdminMcpHandler(AdminMcpHandlerParams{
		Status:            &stubAdminStatus{},
		ResourceRoles:     map[string][]string{"GET /config_dump": {"admin"}, "GET /xds_sync_status": {"admin"}},
		ConfigDumpEnabled: configDumpEnabled,
		Logger:            slog.Default(),
	})
}

// The route keys below are written by hand rather than read from the constants,
// so this compares the resolver against the admin routes as spelled in
// cmd/controller/main.go rather than against itself.
func TestAdminRouteKeysForCall(t *testing.T) {
	h := newAdminTestHandler(true)

	tests := []struct {
		tool string
		want []string
		ok   bool
	}{
		{toolAdminGatewayStatus, []string{"GET /xds_sync_status"}, true},
		{toolAdminConfigDump, []string{"GET /config_dump"}, true},
		// A management tool name must not resolve here: the two endpoints are
		// separate servers with disjoint surfaces.
		{"wso2_apip_gw_deploy_api", nil, false},
		{"not_a_tool", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			got, ok := h.routeKeysForCall(tc.tool, nil)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAdminConfigDumpUnmappableWhenDisabled(t *testing.T) {
	h := newAdminTestHandler(false)

	_, ok := h.routeKeysForCall(toolAdminConfigDump, nil)

	assert.False(t, ok, "a disabled config dump must not resolve to a route key")
}

func TestAdminMCPRouteKeysCoversBothTools(t *testing.T) {
	// Listed unconditionally, so the cross-check in cmd/controller asserts the
	// full surface rather than whatever the test config switched on.
	assert.ElementsMatch(t,
		[]string{"GET /config_dump", "GET /xds_sync_status"},
		AdminMCPRouteKeys())
}

func TestAdminMCPRouteKeysReflectsConfiguration(t *testing.T) {
	// MCPRouteKeys, unlike AdminMCPRouteKeys, describes THIS gateway — which is
	// what makes the advertised scope set match the endpoint as configured.
	assert.Equal(t, []string{"GET /xds_sync_status"}, newAdminTestHandler(false).MCPRouteKeys())
	assert.Equal(t,
		[]string{"GET /config_dump", "GET /xds_sync_status"},
		newAdminTestHandler(true).MCPRouteKeys())
}

func TestAdminMCPBaselineRoles(t *testing.T) {
	assert.Equal(t, []string{"admin"}, newAdminTestHandler(true).MCPBaselineRoles())
}

func TestAdminToolsDenyWithoutGate(t *testing.T) {
	h := newAdminTestHandler(true)

	// The absence of a caller in the context is the invariant that proves the
	// gate ran (GO-AUTH-015). Both tools must refuse rather than execute.
	_, _, err := h.getGatewayStatus(t.Context(), nil, gatewayStatusInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")

	_, _, err = h.getConfigDump(t.Context(), nil, configDumpInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestAdminConfigDumpToolRefusesWhenDisabled(t *testing.T) {
	h := newAdminTestHandler(false)
	ctx := withMcpCaller(t.Context(), mcpCaller{Skipped: true})

	// Registration is the primary gate; this is the re-check that keeps the
	// handler safe if it is ever reached anyway.
	_, _, err := h.getConfigDump(ctx, nil, configDumpInput{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestAdminConfigDumpToolReturnsSterileErrorOnFailure(t *testing.T) {
	h := NewAdminMcpHandler(AdminMcpHandlerParams{
		Status:            &stubAdminStatus{configErr: errors.New("sql: connection refused on host db-01")},
		ResourceRoles:     map[string][]string{"GET /config_dump": {"admin"}},
		ConfigDumpEnabled: true,
		Logger:            slog.Default(),
	})
	ctx := withMcpCaller(t.Context(), mcpCaller{Skipped: true})

	_, _, err := h.getConfigDump(ctx, nil, configDumpInput{})

	require.Error(t, err)
	// The real cause is logged internally only — the model must not see the
	// database host or driver error.
	assert.NotContains(t, err.Error(), "db-01")
	assert.NotContains(t, err.Error(), "sql:")
}

func TestAdminGatewayStatusMergesHealthAndXDS(t *testing.T) {
	version := "42"
	h := NewAdminMcpHandler(AdminMcpHandlerParams{
		Status: &stubAdminStatus{
			xds: adminapi.XDSSyncStatusResponse{PolicyChainVersion: &version},
		},
		ResourceRoles: map[string][]string{"GET /xds_sync_status": {"admin"}},
		Logger:        slog.Default(),
	})
	ctx := withMcpCaller(t.Context(), mcpCaller{Skipped: true})

	_, out, err := h.getGatewayStatus(ctx, nil, gatewayStatusInput{})

	require.NoError(t, err)
	result, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, result, "health")
	assert.Contains(t, result, "xds_sync")
}

// callAdminTool drives one JSON-RPC message through the handler's full chain,
// gate included, exactly as the mux would.
func callAdminTool(t *testing.T, h *AdminMcpHandler, body string, roles []string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/mcp", strings.NewReader(body))
	// Streamable HTTP requires both; without them the SDK rejects the request
	// before any tool is reached.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req = req.WithContext(authenticators.WithAuthContext(req.Context(),
		commonmodels.AuthContext{UserID: "u1", Roles: roles}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAdminToolsListOmitsConfigDumpWhenDisabled(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	withDump := callAdminTool(t, newAdminTestHandler(true), body, []string{"admin"})
	assert.Contains(t, withDump.Body.String(), toolAdminGatewayStatus)
	assert.Contains(t, withDump.Body.String(), toolAdminConfigDump)

	// A gateway with the dump switched off should be indistinguishable from one
	// that never implemented it — advertising a tool that always fails teaches
	// the model to keep retrying it.
	withoutDump := callAdminTool(t, newAdminTestHandler(false), body, []string{"admin"})
	assert.Contains(t, withoutDump.Body.String(), toolAdminGatewayStatus)
	assert.NotContains(t, withoutDump.Body.String(), toolAdminConfigDump)
}

func TestAdminGateDeniesCallerWithoutAdminRole(t *testing.T) {
	h := newAdminTestHandler(true)

	rec := callAdminTool(t, h,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+toolAdminConfigDump+`","arguments":{}}}`,
		[]string{"developer"})

	require.Equal(t, http.StatusForbidden, rec.Code)
	// A real HTTP 403 with the challenge is what drives an MCP client back
	// through OAuth; a JSON-RPC error inside a 200 would not.
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `scope="admin"`)
}
