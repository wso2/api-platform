/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package mcpauthn

import (
	"encoding/json"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

func TestGetPolicy(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, nil)
	if err != nil {
		t.Errorf("GetPolicy returned error: %v", err)
	}
	if p == nil {
		t.Error("GetPolicy returned nil policy")
	}
}

func TestMode(t *testing.T) {
	p := &McpAuthPolicy{}
	mode := p.Mode()
	if mode.RequestHeaderMode != policy.HeaderModeProcess {
		t.Errorf("Expected RequestHeaderMode to be HeaderModeProcess, got %v", mode.RequestHeaderMode)
	}
	if mode.RequestBodyMode != policy.BodyModeSkip {
		t.Errorf("Expected RequestBodyMode to be BodyModeSkip, got %v", mode.RequestBodyMode)
	}
}

func TestOnRequest_WellKnown_Success(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(map[string][]string{
		McpSessionHeader: {"session-123"},
	})
	ctx.Method = "GET"
	ctx.Path = "/.well-known/oauth-protected-resource"

	params := map[string]any{
		"keyManagers": []any{
			map[string]any{
				"name":   "km1",
				"issuer": "https://issuer1.com",
			},
		},
		"requiredScopes": []any{"scope1", "scope2"},
	}

	action := p.OnRequest(ctx, params)

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Headers[McpSessionHeader] != "session-123" {
		t.Errorf("Expected session header 'session-123', got %s", resp.Headers[McpSessionHeader])
	}

	var metadata ProtectedResourceMetadata
	if err := json.Unmarshal(resp.Body, &metadata); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	expectedResource := "http://localhost:8080/mcp"
	if metadata.Resource != expectedResource {
		t.Errorf("Expected resource '%s', got '%s'", expectedResource, metadata.Resource)
	}

	if len(metadata.AuthorizationServers) != 1 || metadata.AuthorizationServers[0] != "https://issuer1.com" {
		t.Errorf("Unexpected authorization servers: %v", metadata.AuthorizationServers)
	}

	if len(metadata.ScopesSupported) != 2 {
		t.Errorf("Unexpected scopes supported: %v", metadata.ScopesSupported)
	}
}

func TestOnRequest_WellKnown_NoKeyManagers(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(nil)
	ctx.Method = "GET"
	ctx.Path = "/.well-known/oauth-protected-resource"

	params := map[string]any{}

	action := p.OnRequest(ctx, params)
	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestOnRequest_WellKnown_FilteredIssuers(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(nil)
	ctx.Method = "GET"
	ctx.Path = "/.well-known/oauth-protected-resource"

	params := map[string]any{
		"keyManagers": []any{
			map[string]any{
				"name":   "km1",
				"issuer": "https://issuer1.com",
			},
			map[string]any{
				"name":   "km2",
				"issuer": "https://issuer2.com",
			},
		},
		"issuers": []any{"km2"}, // Only allow km2
	}

	action := p.OnRequest(ctx, params)

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	var metadata ProtectedResourceMetadata
	if err := json.Unmarshal(resp.Body, &metadata); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	if len(metadata.AuthorizationServers) != 1 || metadata.AuthorizationServers[0] != "https://issuer2.com" {
		t.Errorf("Expected only issuer2, got %v", metadata.AuthorizationServers)
	}
}

func TestOnRequest_WellKnown_WithVhost(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(map[string][]string{
		McpSessionHeader: {"session-456"},
	})
	ctx.Method = "GET"
	ctx.Path = "/.well-known/oauth-protected-resource"
	ctx.Scheme = "https"
	ctx.Authority = "localhost:8443"
	ctx.Vhost = "api.example.com"

	params := map[string]any{
		"keyManagers": []any{
			map[string]any{
				"name":   "km1",
				"issuer": "https://issuer1.com",
			},
		},
	}

	action := p.OnRequest(ctx, params)

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var metadata ProtectedResourceMetadata
	if err := json.Unmarshal(resp.Body, &metadata); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	// Should use vhost (api.example.com) with port from authority (8443)
	expectedResource := "https://api.example.com:8443/mcp"
	if metadata.Resource != expectedResource {
		t.Errorf("Expected resource '%s', got '%s'", expectedResource, metadata.Resource)
	}
}

func TestOnRequest_WellKnown_WithVhost_StandardPort(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(nil)
	ctx.Method = "GET"
	ctx.Path = "/.well-known/oauth-protected-resource"
	ctx.Scheme = "https"
	ctx.Authority = "api.example.com:443"
	ctx.Vhost = "api.example.com"

	params := map[string]any{
		"keyManagers": []any{
			map[string]any{
				"name":   "km1",
				"issuer": "https://issuer1.com",
			},
		},
	}

	action := p.OnRequest(ctx, params)

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	var metadata ProtectedResourceMetadata
	if err := json.Unmarshal(resp.Body, &metadata); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	// Should use vhost without port since 443 is standard for https
	expectedResource := "https://api.example.com/mcp"
	if metadata.Resource != expectedResource {
		t.Errorf("Expected resource '%s', got '%s'", expectedResource, metadata.Resource)
	}
}

func TestOnRequest_WellKnown_WithVhost_AndAPIContext(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(nil)
	ctx.Method = "GET"
	ctx.Path = "/.well-known/oauth-protected-resource"
	ctx.Scheme = "https"
	ctx.Authority = "localhost:8443"
	ctx.Vhost = "api.example.com"
	ctx.APIContext = "/v1/myapi"

	params := map[string]any{
		"keyManagers": []any{
			map[string]any{
				"name":   "km1",
				"issuer": "https://issuer1.com",
			},
		},
	}

	action := p.OnRequest(ctx, params)

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	var metadata ProtectedResourceMetadata
	if err := json.Unmarshal(resp.Body, &metadata); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	// Should include API context in the resource path
	expectedResource := "https://api.example.com:8443/v1/myapi/mcp"
	if metadata.Resource != expectedResource {
		t.Errorf("Expected resource '%s', got '%s'", expectedResource, metadata.Resource)
	}
}

func TestOnRequest_Delegation_Failure(t *testing.T) {
	p := &McpAuthPolicy{}
	ctx := createMockRequestContext(map[string][]string{
		McpSessionHeader: {"session-123"},
	})
	ctx.Method = "GET"
	ctx.Path = "/api/resource"

	// We provide params but no valid JWT token in headers.
	// JWT Auth policy should fail.
	params := map[string]any{
		"gatewayHost": "gateway.com",
	}

	action := p.OnRequest(ctx, params)

	// We expect ImmediateResponse (failure from JWT Auth wrapped)
	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		// If JWT Auth passes (which it shouldn't without token), it would return nil or RequestAction.
		// But JWT Auth usually returns 401 if no token.
		t.Fatalf("Expected ImmediateResponse (auth failure), got %T", action)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}

	authHeader := resp.Headers[WWWAuthenticateHeader]
	if authHeader == "" {
		t.Error("Expected WWW-Authenticate header")
	}

	expectedPrefix := `Bearer resource_metadata="http://gateway.com:8080/.well-known/oauth-protected-resource"`
	if !strings.HasPrefix(authHeader, expectedPrefix) {
		t.Errorf("Unexpected WWW-Authenticate header: %s", authHeader)
	}

	if resp.Headers[McpSessionHeader] != "session-123" {
		t.Errorf("Expected session header 'session-123', got %s", resp.Headers[McpSessionHeader])
	}
}

func createMockRequestContext(headers map[string][]string) *policy.RequestContext {
	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{
			RequestID: "test-request-id",
			Metadata:  make(map[string]any),
		},
		Headers: policy.NewHeaders(headers),
		Body:    nil,
		Path:    "/api/test",
		Method:  "GET",
		Scheme:  "http",
	}
}
