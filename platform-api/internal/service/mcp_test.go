/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// mockMCPProxyRepository is a minimal mock of repository.MCPProxyRepository.
type mockMCPProxyRepository struct {
	repository.MCPProxyRepository
	getByHandleResult *model.MCPProxy
}

func (m *mockMCPProxyRepository) GetByHandle(handle, orgUUID string) (*model.MCPProxy, error) {
	return m.getByHandleResult, nil
}

func validMCPProxyRequest() *api.MCPProxy {
	url := "https://example.com/mcp"
	return &api.MCPProxy{
		Id:          strPointer("mcp-proxy-1"),
		DisplayName: "Test MCP Proxy",
		Version:     "v1.0",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: &url},
		},
	}
}

func TestMCPProxyServiceCreateRejectsInvalidPolicyVersion(t *testing.T) {
	service := NewMCPProxyService(&mockMCPProxyRepository{}, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validMCPProxyRequest()
	request.Policies = &[]api.Policy{{Name: "api-key-auth", Version: "v1.0.0"}}

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestMCPProxyServiceUpdateRejectsInvalidPolicyVersion(t *testing.T) {
	repo := &mockMCPProxyRepository{getByHandleResult: &model.MCPProxy{Handle: "mcp-proxy-1"}}
	service := NewMCPProxyService(repo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validMCPProxyRequest()
	request.Policies = &[]api.Policy{{Name: "api-key-auth", Version: "1"}}

	_, err := service.Update("org-1", "mcp-proxy-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

// TestFetchServerInfoRefetchUsesStoredURLVerbatim verifies that the refetch flow (proxyId
// provided) contacts the MCP backend at EXACTLY the stored upstream URL path — it must not
// append "/mcp" (or otherwise manipulate the path). This is the regression guard for the
// removed ensureMCPEndpointURL normalization: the stored URL is the full MCP endpoint that
// was validated at creation time, and the gateway forwards to exactly this path.
func TestFetchServerInfoRefetchUsesStoredURLVerbatim(t *testing.T) {
	tests := []struct {
		name       string
		storedPath string // path suffix appended to the test server base URL and stored on the proxy
		wantPath   string // path the backend must actually receive
	}{
		// Upstream already points at the backend's "/mcp" endpoint: contact it as-is, no "/mcp/mcp".
		{"mcp endpoint stored verbatim", "/mcp", "/mcp"},
		// Upstream has no path: contact the root, never rewritten to "/mcp".
		{"root upstream not rewritten to /mcp", "", "/"},
		// Custom-path upstream: contact exactly that path.
		{"custom path stored verbatim", "/api/v1/mcp-server", "/api/v1/mcp-server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var paths []string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths = append(paths, r.URL.Path)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("mcp-session-id", "test-session")
				// Minimal valid MCP initialize/JSON-RPC result so the handshake proceeds.
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"test","version":"1.0"}}}`))
			}))
			defer srv.Close()

			proxy := &model.MCPProxy{
				Handle: "mcp-proxy-1",
				Configuration: model.MCPProxyConfiguration{
					Upstream: model.UpstreamConfig{
						Main: &model.UpstreamEndpoint{URL: srv.URL + tt.storedPath},
					},
				},
			}
			repo := &mockMCPProxyRepository{getByHandleResult: proxy}
			service := NewMCPProxyService(repo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

			proxyID := "mcp-proxy-1"
			_, err := service.FetchServerInfo("org-1", &api.MCPServerInfoFetchRequest{ProxyId: &proxyID})
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()
			require.NotEmpty(t, paths, "backend should have received at least one request")
			for _, p := range paths {
				assert.Equal(t, tt.wantPath, p, "backend received an unexpected path")
				assert.NotContains(t, p, "/mcp/mcp", "the /mcp resource path must not be doubled on the backend")
			}
		})
	}
}
