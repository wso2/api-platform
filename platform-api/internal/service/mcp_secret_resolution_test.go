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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// TestFetchServerInfo_ProxyIdRefetch_ResolvesSecretHandle verifies the exact bug
// fixed in FetchServerInfo's proxyId branch: the stored upstream auth value is a
// {{ secret "handle" }} placeholder, not the plaintext credential, and it must be
// resolved through the secret store before being sent to the target MCP server as
// the actual header value — never sent to the upstream as the literal placeholder
// text, and never appear anywhere in the response returned to the caller.
func TestFetchServerInfo_ProxyIdRefetch_ResolvesSecretHandle(t *testing.T) {
	const orgID = "org-1"
	const plaintextCredential = "Bearer super-secret-live-token"

	// Real SecretService over the lightweight mock vault/repo already used by
	// secret_service_test.go, so Create/Decrypt round-trip through the same
	// encrypt/decrypt code path FetchServerInfo itself calls.
	vault := &mockVault{}
	secretRepo := newMockRepo()
	secretService := NewSecretService(secretRepo, vault, newTestIdentityService())

	created, err := secretService.Create(orgID, "tester", &dto.CreateSecretRequest{
		Handle:      "upstream-auth-handle",
		DisplayName: "Upstream auth",
		Value:       plaintextCredential,
		Type:        model.SecretTypeGeneric,
	})
	require.NoError(t, err)
	require.Equal(t, "upstream-auth-handle", created.Handle)

	var receivedAuthHeader string
	mockMCPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		var body struct {
			Method string `json:"method"`
			ID     *int   `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Captured on every JSON-RPC call — initialize is the first one that
		// carries the custom auth header, so this is where the leak (or lack of
		// one) would show up.
		if h := r.Header.Get("Authorization"); h != "" {
			receivedAuthHeader = h
		}

		w.Header().Set("Content-Type", "application/json")
		switch body.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"test","version":"1.0.0"}}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockMCPServer.Close()

	repo := &mockMCPProxyRepository{getByHandleResult: &model.MCPProxy{
		Handle: "mcp-proxy-1",
		Configuration: model.MCPProxyConfiguration{
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: mockMCPServer.URL,
					Auth: &model.UpstreamAuth{
						Type:   "header",
						Header: "Authorization",
						// The stored value is always a placeholder, never plaintext.
						Value: `{{ secret "upstream-auth-handle" }}`,
					},
				},
			},
		},
	}}

	mcpService := NewMCPProxyService(repo, nil, nil, nil, nil, newTestLogger(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	mcpService.WithSecretService(secretService)

	proxyID := "mcp-proxy-1"
	resp, err := mcpService.FetchServerInfo(orgID, &api.MCPServerInfoFetchRequest{ProxyId: &proxyID})
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, plaintextCredential, receivedAuthHeader,
		"the upstream MCP server must receive the resolved plaintext credential, not the raw secret placeholder")
	require.NotContains(t, receivedAuthHeader, "{{ secret",
		"the placeholder text must never be sent to the upstream server as-is")

	// The resolved plaintext must never appear anywhere in the client-facing response.
	responseJSON, err := json.Marshal(resp)
	require.NoError(t, err)
	require.NotContains(t, string(responseJSON), plaintextCredential,
		"the resolved credential must never be echoed back in the fetch-server-info response")
}
