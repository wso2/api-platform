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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

var toolsDiscard = slog.New(slog.NewTextHandler(io.Discard, nil))

// toolsCalls records every call a stub kind receives and carries the canned
// results and errors a test wants the stub to return.
type toolsCalls struct {
	create, update, delete, get, list int
	handle, manifest                  string
	rows                              []listedResource

	createErr, updateErr, deleteErr, getErr, listErr error

	keyCalls  map[string]int
	keyParent string
	keyName   string
	caller    *commonmodels.AuthContext
	issueReq  *api.APIKeyCreationRequest
	updateReq *api.APIKeyCreationRequest
	rotateReq *api.APIKeyRegenerationRequest
	keyErr    error
}

func (c *toolsCalls) recordKey(op, parent, keyName string, caller *commonmodels.AuthContext) error {
	c.keyCalls[op]++
	c.keyParent, c.keyName, c.caller = parent, keyName, caller
	return c.keyErr
}

func toolsStubKind(kind, collection string, routable, withKeys bool, c *toolsCalls) *kindOps {
	c.keyCalls = map[string]int{}
	ops := &kindOps{
		Kind:       kind,
		Routable:   routable,
		Collection: collection,
		Create: func(manifest []byte, _ string, _ *slog.Logger) (any, error) {
			c.create++
			c.manifest = string(manifest)
			if c.createErr != nil {
				return nil, c.createErr
			}
			return map[string]any{"created": kind}, nil
		},
		Update: func(handle string, manifest []byte, _ string, _ *slog.Logger) (any, error) {
			c.update++
			c.handle, c.manifest = handle, string(manifest)
			if c.updateErr != nil {
				return nil, c.updateErr
			}
			return map[string]any{"updated": handle}, nil
		},
		Delete: func(handle, _ string, _ *slog.Logger) error {
			c.delete++
			c.handle = handle
			return c.deleteErr
		},
		Get: func(handle string) (any, error) {
			c.get++
			c.handle = handle
			if c.getErr != nil {
				return nil, c.getErr
			}
			return map[string]any{"handle": handle}, nil
		},
		List: func() ([]listedResource, error) {
			c.list++
			if c.listErr != nil {
				return nil, c.listErr
			}
			return c.rows, nil
		},
	}
	if !withKeys {
		return ops
	}
	ops.Keys = &keyOps{
		Issue: func(parent string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			c.issueReq = &req
			if err := c.recordKey("issue", parent, "", caller); err != nil {
				return nil, err
			}
			return map[string]any{"issued": true}, nil
		},
		List: func(parent string, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			if err := c.recordKey("list", parent, "", caller); err != nil {
				return nil, err
			}
			return map[string]any{"listed": true}, nil
		},
		Rotate: func(parent, keyName string, req api.APIKeyRegenerationRequest, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			c.rotateReq = &req
			if err := c.recordKey("rotate", parent, keyName, caller); err != nil {
				return nil, err
			}
			return map[string]any{"rotated": keyName}, nil
		},
		Update: func(parent, keyName string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			c.updateReq = &req
			if err := c.recordKey("update", parent, keyName, caller); err != nil {
				return nil, err
			}
			return map[string]any{"updated": keyName}, nil
		},
		Revoke: func(parent, keyName string, caller *commonmodels.AuthContext, _ string, _ *slog.Logger) (any, error) {
			if err := c.recordKey("revoke", parent, keyName, caller); err != nil {
				return nil, err
			}
			return map[string]any{"revoked": keyName}, nil
		},
	}
	return ops
}

// toolsRoleMap grants developer the RestApi and Mcp routes, and admin the Secret
// and key-injection routes.
func toolsRoleMap() map[string][]string {
	both := []string{"developer", "admin"}
	admin := []string{"admin"}
	return map[string][]string{
		"POST /rest-apis":        both,
		"GET /rest-apis":         both,
		"GET /rest-apis/{id}":    both,
		"PUT /rest-apis/{id}":    both,
		"DELETE /rest-apis/{id}": both,

		"POST /mcp-proxies":        both,
		"GET /mcp-proxies":         both,
		"GET /mcp-proxies/{id}":    both,
		"PUT /mcp-proxies/{id}":    both,
		"DELETE /mcp-proxies/{id}": both,

		"POST /secrets":        admin,
		"GET /secrets":         admin,
		"GET /secrets/{id}":    admin,
		"PUT /secrets/{id}":    admin,
		"DELETE /secrets/{id}": admin,

		"POST /rest-apis/{id}/api-keys":                         both,
		"GET /rest-apis/{id}/api-keys":                          both,
		"POST /rest-apis/{id}/api-keys/{apiKeyName}/regenerate": both,
		"PUT /rest-apis/{id}/api-keys/{apiKeyName}":             admin,
		"DELETE /rest-apis/{id}/api-keys/{apiKeyName}":          both,
	}
}

type toolsFixture struct {
	h                  *McpHandler
	rest, mcp, secrets *toolsCalls
}

// newToolsHandler builds an McpHandler over three stub kinds: RestApi
// (routable, key-bearing), Mcp (routable, no keys) and Secret (config).
func newToolsHandler(immutable bool) *toolsFixture {
	f := &toolsFixture{rest: &toolsCalls{}, mcp: &toolsCalls{}, secrets: &toolsCalls{}}
	f.h = &McpHandler{
		immutable: immutable,
		logger:    toolsDiscard,
		authz:     newMcpAuthz(mcpAuthzParams{ResourceRoles: toolsRoleMap(), Logger: toolsDiscard}),
		kinds: map[string]*kindOps{
			models.KindRestApi: toolsStubKind(models.KindRestApi, "/rest-apis", true, true, f.rest),
			models.KindMcp:     toolsStubKind(models.KindMcp, "/mcp-proxies", true, false, f.mcp),
			models.KindSecret:  toolsStubKind(models.KindSecret, "/secrets", false, false, f.secrets),
		},
	}
	return f
}

// toolsCtx is a request that cleared the HTTP gate as user "alice" holding roles.
func toolsCtx(roles ...string) context.Context {
	return withMcpCaller(context.Background(), mcpCaller{
		Auth: commonmodels.AuthContext{UserID: "alice", Roles: roles},
	})
}

func resultMap(t *testing.T, out any) map[string]any {
	t.Helper()
	m, ok := out.(map[string]any)
	require.Truef(t, ok, "tool output is %T, not map[string]any", out)
	return m
}

// toolsAs asserts v's dynamic type and fails the test with a readable message,
// rather than panicking, when a tool returns a different shape.
func toolsAs[T any](t *testing.T, v any) T {
	t.Helper()
	got, ok := v.(T)
	require.Truef(t, ok, "got %T, want %T", v, *new(T))
	return got
}

var toolsSkippedCtx = withMcpCaller(context.Background(), mcpCaller{Skipped: true})

// mcpPost sends one JSON-RPC message through the real handler, as an MCP
// client would, and returns the recorder.
func mcpPost(t *testing.T, h http.Handler, auth *commonmodels.AuthContext, method string, params map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "tools-test", "version": "1"},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/management/v1/mcp", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", method)
	if name, ok := params["name"].(string); ok {
		req.Header.Set("Mcp-Name", name)
	}
	if auth != nil {
		req = req.WithContext(authenticators.WithAuthContext(req.Context(), *auth))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeMCPResponse reads the JSON-RPC response from either a JSON body or an
// SSE stream, whichever the SDK chose to answer with.
func decodeMCPResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	raw := rec.Body.String()
	if strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(strings.NewReader(raw))
		scanner.Buffer(make([]byte, 0, 1<<20), 1<<24)
		raw = ""
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data:") {
				raw = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
	}
	var msg map[string]any
	require.NoErrorf(t, json.Unmarshal([]byte(raw), &msg), "response was not JSON-RPC: %q", rec.Body.String())
	return msg
}

func registeredToolNames(t *testing.T, h http.Handler) []string {
	t.Helper()
	rec := mcpPost(t, h, nil, "tools/list", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	result, ok := decodeMCPResponse(t, rec)["result"].(map[string]any)
	require.True(t, ok, "tools/list returned no result: %s", rec.Body.String())
	var names []string
	for _, tool := range toolsAs[[]any](t, result["tools"]) {
		names = append(names, toolsAs[string](t, toolsAs[map[string]any](t, tool)["name"]))
	}
	return names
}

var baseMCPTools = []string{
	"wso2_apip_gw_deploy_api", "wso2_apip_gw_undeploy_api",
	"wso2_apip_gw_apply_config", "wso2_apip_gw_delete_config",
	"wso2_apip_gw_list_resources", "wso2_apip_gw_get_resource",
	"wso2_apip_gw_issue_api_key", "wso2_apip_gw_list_api_keys",
	"wso2_apip_gw_rotate_api_key", "wso2_apip_gw_revoke_api_key",
}

// The certificate and subscription tools are only registered when their service is set.
func TestNewMcpHandlerRegistersToolsForTheWiredServices(t *testing.T) {
	bare := newMcpHandler(McpHandlerParams{Logger: toolsDiscard})
	assert.ElementsMatch(t, baseMCPTools, registeredToolNames(t, bare))

	db := NewMockStorage()
	full := newMcpHandler(McpHandlerParams{
		Logger:              toolsDiscard,
		CertificateService:  toolsCertService(db, &stubCertStore{}, &stubSnapshot{}),
		SubscriptionService: toolsSubscriptionService(db),
	})
	assert.ElementsMatch(t,
		append(append([]string{}, baseMCPTools...), "wso2_apip_gw_manage_certificates", "wso2_apip_gw_manage_subscriptions"),
		registeredToolNames(t, full))
}

// A call the caller has no role for is refused at the HTTP layer with a 403 challenge.
func TestNewMcpHandlerGateStepsUpBeforeTheToolRuns(t *testing.T) {
	h := newMcpHandler(McpHandlerParams{
		Logger: toolsDiscard,
		ResourceRoles: map[string][]string{
			"POST /rest-apis":     {"developer", "admin"},
			"POST /llm-providers": {"admin"},
		},
	})
	developer := &commonmodels.AuthContext{UserID: "alice", Roles: []string{"developer"}}

	rec := mcpPost(t, h, developer, "tools/call", map[string]any{
		"name":      "wso2_apip_gw_deploy_api",
		"arguments": map[string]any{"kind": "LlmProvider", "yaml": "kind: LlmProvider\n"},
	})

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`)
}

// A tool's own refusal is a tool execution error (isError) inside a 200, not a
// JSON-RPC protocol error: it is what the model is meant to read and correct.
func TestNewMcpHandlerReturnsToolRefusalsAsToolErrors(t *testing.T) {
	h := newMcpHandler(McpHandlerParams{
		Logger:        toolsDiscard,
		Immutable:     true,
		ResourceRoles: map[string][]string{"POST /rest-apis": {"developer"}},
	})
	// No RestAPIService is wired, so record calls to Create instead of reaching a nil service.
	var createCalled bool
	h.kinds[models.KindRestApi].Create = func([]byte, string, *slog.Logger) (any, error) {
		createCalled = true
		return nil, errors.New("create reached")
	}
	developer := &commonmodels.AuthContext{UserID: "alice", Roles: []string{"developer"}}

	rec := mcpPost(t, h, developer, "tools/call", map[string]any{
		"name":      "wso2_apip_gw_deploy_api",
		"arguments": map[string]any{"kind": "RestApi", "yaml": "kind: RestApi\n"},
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msg := decodeMCPResponse(t, rec)
	assert.NotContains(t, msg, "error", "a tool refusal is not a protocol error")
	result := toolsAs[map[string]any](t, msg["result"])
	assert.Equal(t, true, result["isError"])
	encoded, err := json.Marshal(result["content"])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "immutable mode")
	assert.False(t, createCalled, "an immutable gateway must refuse before the service is reached")
}
