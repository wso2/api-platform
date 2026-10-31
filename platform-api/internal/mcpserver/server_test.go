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

package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

const specPath = "../../resources/openapi.yaml"

const testOrg = "11111111-1111-1111-1111-111111111111"

// fake is a backend that records what the tools ask of it.
type fake struct {
	backend    // unimplemented methods panic
	api        object
	listed     []object
	updated    object
	created    object
	failOn     string // undeploy fails for this deployment ID
	running    []api.DeploymentResponse
	undone     []string
	deleted    []string // IDs passed to deleteAPI and deleteProject
	failDelete bool     // deleteAPI and deleteProject fail
}

func (f *fake) getAPI(context.Context, caller, *apiType, string) (object, error) {
	if f.api == nil {
		return nil, apperror.RESTAPINotFound.New()
	}
	return f.api, nil
}

func (f *fake) listAPIs(context.Context, caller, *apiType, string, page) ([]object, int, error) {
	return f.listed, len(f.listed), nil
}

func (f *fake) createAPI(_ context.Context, _ caller, _ *apiType, p object) (object, error) {
	f.created = p
	return p, nil
}

func (f *fake) updateAPI(_ context.Context, _ caller, _ *apiType, _ string, whole object) (object, error) {
	f.updated = whole
	return whole, nil
}

func (f *fake) deleteAPI(_ context.Context, _ caller, _ *apiType, id string) error {
	if f.failDelete {
		return apperror.RESTAPINotFound.New()
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fake) deleteProject(_ context.Context, _ caller, id string) error {
	if f.failDelete {
		return apperror.ValidationFailed.New("project still has APIs")
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fake) listProjects(context.Context, caller, page) ([]api.Project, int, error) {
	id := "default"
	return []api.Project{{Id: &id, DisplayName: "Default"}}, 1, nil
}

func (f *fake) listDeployments(_ context.Context, _ caller, _ *apiType, _, _, status string, _ page) ([]api.DeploymentResponse, int, error) {
	var out []api.DeploymentResponse
	for _, d := range f.running {
		if status == "" || string(d.Status) == status {
			out = append(out, d)
		}
	}
	return out, len(out), nil
}

func (f *fake) undeploy(_ context.Context, _ caller, _ *apiType, _, id, gw string) (*api.DeploymentResponse, error) {
	if id == f.failOn {
		return nil, apperror.ValidationFailed.New("gateway is not connected")
	}
	f.undone = append(f.undone, id)
	return &api.DeploymentResponse{DeploymentId: openapi_types.UUID(uuidOf(id)), GatewayId: gw, Status: api.DeploymentResponseStatusUNDEPLOYED}, nil
}

func uuidOf(s string) [16]byte {
	var u openapi_types.UUID
	_ = u.UnmarshalText([]byte(s))
	return u
}

func newTestServer(t *testing.T) (*Server, *fake) {
	t.Helper()
	scopes, err := middleware.LoadScopeRegistry(specPath)
	require.NoError(t, err)
	cfg := &config.Server{OpenAPISpecPath: specPath}
	cfg.Auth.Authorization.Enabled = true // the Platform API's default
	cfg.MCPServer = config.MCPServer{Enabled: true}
	s, err := New(Deps{Config: cfg, Scopes: scopes})
	require.NoError(t, err)
	f := &fake{}
	s.backend = f
	return s, f
}

// connect starts the MCP handler with a fixed caller and returns a client session.
func connect(t *testing.T, s *Server, c caller) *mcp.ClientSession {
	t.Helper()
	h := s.mcpHandler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, withCaller(r, c))
	}))
	t.Cleanup(srv.Close)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func who(org string, scopes ...string) caller {
	return caller{org: org, user: "u1", scopes: toSet(scopes)}
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return res, text.String()
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestToolsList_ShowsOnlyWhatTheCallerMayUse(t *testing.T) {
	s, _ := newTestServer(t)
	names := toolNames(t, connect(t, s, who(testOrg, "ap:project:read")))
	assert.ElementsMatch(t, []string{"list_projects", "get_api_schema"}, names)

	names = toolNames(t, connect(t, s, who(testOrg, "ap:api:rest:manage", "ap:project:manage", "ap:gateway:manage")))
	assert.Len(t, names, 14)
}

// TestToolsList_ListsEveryAPIType checks that an API tool shows all API types
// to a caller who may use some, and is hidden from one who may use none.
func TestToolsList_ListsEveryAPIType(t *testing.T) {
	s, _ := newTestServer(t)
	res, err := connect(t, s, who(testOrg, "ap:api:llm:read")).ListTools(context.Background(), nil)
	require.NoError(t, err)
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Name != "list_apis" {
			continue
		}
		var in struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		b, _ := json.Marshal(tool.InputSchema)
		require.NoError(t, json.Unmarshal(b, &in))
		assert.Len(t, in.Properties["apiType"].Enum, len(apiTypes))
	}
	assert.Contains(t, names, "list_apis")
	assert.NotContains(t, names, "create_api")
}

func TestAdd_MissingScopeIsAToolError(t *testing.T) {
	s, _ := newTestServer(t)
	res, text := call(t, connect(t, s, who(testOrg, "ap:project:read")), "list_apis",
		map[string]any{"apiType": "rest-api", "projectId": "default"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "missing permission: this needs ap:api:rest:")
}

// TestScopeChecksOff_EveryToolIsOpen: with the Platform API's scope checks off,
// /mcp checks no scopes either, as the REST API does.
func TestScopeChecksOff_EveryToolIsOpen(t *testing.T) {
	s, _ := newTestServer(t)
	s.auth.Authorization.Enabled = false
	cs := connect(t, s, who(testOrg))
	assert.Len(t, toolNames(t, cs), 14)
	res, text := call(t, cs, "list_projects", nil)
	assert.False(t, res.IsError, text)
	assert.Contains(t, text, `"id":"default"`)
}

// TestToolRiskLevels pins each tool's risk level.
func TestToolRiskLevels(t *testing.T) {
	s, _ := newTestServer(t)
	scopes := []string{"ap:api:rest:manage", "ap:project:manage", "ap:gateway:manage"}
	expected := map[string]*mcp.ToolAnnotations{
		"list_apis": read, "get_api": read, "get_api_schema": read, "list_api_deployments": read,
		"list_projects": read, "list_gateways": read, "get_gateway": read,
		"create_api": write, "create_project": write,
		"update_api": writeRepeat, "deploy_api": writeRepeat,
		"undeploy_api": destructiveRepeat, "delete_api": destructiveRepeat, "delete_project": destructiveRepeat,
	}
	listed, err := connect(t, s, who(testOrg, scopes...)).ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, listed.Tools, len(expected))
	for _, tool := range listed.Tools {
		want, ok := expected[tool.Name]
		require.True(t, ok, "%s is not in the table", tool.Name)
		require.Equal(t, want, tool.Annotations, tool.Name)
	}
}

func TestAdd_NoCallerIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.mcpHandler()
	srv := httptest.NewServer(h) // no caller in the context
	defer srv.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()
	assert.Empty(t, toolNames(t, cs))
	res, text := call(t, cs, "list_projects", nil)
	assert.True(t, res.IsError)
	assert.Contains(t, text, "not signed in")
}

func TestCreateAPI_PayloadRules(t *testing.T) {
	s, f := newTestServer(t)
	cs := connect(t, s, who(testOrg, "ap:api:rest:manage"))
	payload := func(auth string) map[string]any {
		return map[string]any{"displayName": "Orders", "version": "v1", "context": "/orders", "projectId": "default",
			"upstream": map[string]any{"main": map[string]any{"url": "http://orders", "auth": map[string]any{"type": "bearer", "value": auth}}}}
	}

	res, text := call(t, cs, "create_api", map[string]any{"apiType": "rest-api", "payload": payload("sk-live-123")})
	require.False(t, res.IsError, text)
	assert.Equal(t, "sk-live-123", f.created["upstream"].(map[string]any)["main"].(map[string]any)["auth"].(map[string]any)["value"])

	res, text = call(t, cs, "create_api", map[string]any{"apiType": "rest-api", "payload": payload(`{{ secret "orders-key" }}`)})
	assert.False(t, res.IsError, text)
	assert.Equal(t, "Orders", f.created["displayName"])

	res, _ = call(t, cs, "create_api", map[string]any{"apiType": "graphql-api", "payload": payload("")})
	assert.True(t, res.IsError, "a type that is not switched on is refused by the schema")
}

func TestUpdateAPI_Merges(t *testing.T) {
	s, f := newTestServer(t)
	f.api = object{"id": "orders-v1", "context": "/orders", "version": "v1", "projectId": "default",
		"description": "old", "policies": []any{map[string]any{"name": "cors"}},
		"upstream": map[string]any{"main": map[string]any{"url": "http://orders"}}}
	cs := connect(t, s, who(testOrg, "ap:api:rest:read", "ap:api:rest:update"))

	res, text := call(t, cs, "update_api", map[string]any{"apiType": "rest-api", "apiId": "orders-v1",
		"payload": map[string]any{"description": "new"}})
	require.False(t, res.IsError, text)
	assert.Equal(t, "new", f.updated["description"])
	assert.Equal(t, f.api["policies"], f.updated["policies"], "fields left out survive the PUT")

	res, text = call(t, connect(t, s, who(testOrg, "ap:api:rest:update")), "update_api",
		map[string]any{"apiType": "rest-api", "apiId": "orders-v1", "payload": map[string]any{"description": "x"}})
	assert.True(t, res.IsError, "update reads first, so it needs a read scope too")
	assert.Contains(t, text, "missing permission")
}

func TestUndeployAPI_StopsAtFirstFailureAndSaysWhatFinished(t *testing.T) {
	s, f := newTestServer(t)
	a, b, c := "aaaaaaaa-0000-0000-0000-000000000001", "aaaaaaaa-0000-0000-0000-000000000002", "aaaaaaaa-0000-0000-0000-000000000003"
	for _, id := range []string{a, b, c} {
		f.running = append(f.running, api.DeploymentResponse{DeploymentId: uuidOf(id), GatewayId: "gw1", Status: api.DeploymentResponseStatusDEPLOYED})
	}
	f.running[1].Status = api.DeploymentResponseStatusDEPLOYING // still starting: undeployed too
	f.failOn = b
	res, text := call(t, connect(t, s, who(testOrg, "ap:api:rest:manage")), "undeploy_api",
		map[string]any{"apiType": "rest-api", "apiId": "orders-v1", "gatewayId": "gw1", "confirm": true})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "undeployed ["+a+", "+c+"], then deployment "+b+" failed: gateway is not connected")
	assert.Equal(t, []string{a, c}, f.undone)
}

func TestUndeployAPI_NothingRunningSaysSo(t *testing.T) {
	s, f := newTestServer(t)
	f.running = []api.DeploymentResponse{{DeploymentId: uuidOf("aaaaaaaa-0000-0000-0000-000000000009"), GatewayId: "gw1", Status: api.DeploymentResponseStatusUNDEPLOYED}}
	res, text := call(t, connect(t, s, who(testOrg, "ap:api:rest:manage")), "undeploy_api",
		map[string]any{"apiType": "rest-api", "apiId": "orders-v1", "gatewayId": "gw-typo", "confirm": true})
	assert.False(t, res.IsError, "a repeat must stay safe")
	assert.Contains(t, text, "Nothing to undeploy: orders-v1 has no running deployment on gateway gw-typo")
	assert.Empty(t, f.undone)
}

func TestUndeployAPI_EmptyGatewayIsRefused(t *testing.T) {
	s, f := newTestServer(t)
	f.running = []api.DeploymentResponse{{DeploymentId: uuidOf("aaaaaaaa-0000-0000-0000-000000000001"), GatewayId: "gw1", Status: api.DeploymentResponseStatusDEPLOYED}}
	cs := connect(t, s, who(testOrg, "ap:api:rest:manage"))
	for _, gateway := range []string{"", "  "} {
		res, text := call(t, cs, "undeploy_api", map[string]any{"apiType": "rest-api", "apiId": "orders-v1", "gatewayId": gateway, "confirm": true})
		assert.True(t, res.IsError)
		assert.Contains(t, text, "gatewayId is required; get one from list_gateways")
	}
	assert.Empty(t, f.undone)
}

// Only the destructive tools take confirm, and it is required.
func TestDestructiveTools_TakeConfirm(t *testing.T) {
	s, _ := newTestServer(t)
	res, err := connect(t, s, who(testOrg, "ap:api:rest:manage", "ap:project:manage", "ap:gateway:manage")).
		ListTools(context.Background(), nil)
	require.NoError(t, err)
	var withConfirm []string
	for _, tool := range res.Tools {
		var schema struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		b, _ := json.Marshal(tool.InputSchema)
		require.NoError(t, json.Unmarshal(b, &schema))
		if _, ok := schema.Properties["confirm"]; ok {
			assert.Contains(t, schema.Required, "confirm", tool.Name)
			withConfirm = append(withConfirm, tool.Name)
		}
	}
	assert.ElementsMatch(t, []string{"undeploy_api", "delete_api", "delete_project"}, withConfirm)
}

func TestUndeployAPI_RefusedWithoutConfirm(t *testing.T) {
	s, f := newTestServer(t)
	f.running = []api.DeploymentResponse{{DeploymentId: uuidOf("aaaaaaaa-0000-0000-0000-000000000001"), GatewayId: "gw1", Status: api.DeploymentResponseStatusDEPLOYED}}
	var logs bytes.Buffer
	s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	cs := connect(t, s, who(testOrg, "ap:api:rest:manage"))
	args := map[string]any{"apiType": "rest-api", "apiId": "orders-v1", "gatewayId": "gw1"}

	res, text := call(t, cs, "undeploy_api", args)
	assert.True(t, res.IsError)
	assert.Contains(t, text, "missing properties", "the schema requires confirm")

	args["confirm"] = false
	res, text = call(t, cs, "undeploy_api", args)
	assert.True(t, res.IsError)
	assert.Contains(t, text, "refusing to run undeploy_api without confirm=true. Set it only for the exact target the user asked for, then call again")
	assert.Contains(t, logs.String(), `tool=undeploy_api outcome="refused: not confirmed"`)
	assert.Empty(t, f.undone)
}

func TestListAPIs_ProjectRule(t *testing.T) {
	s, f := newTestServer(t)
	f.listed = []object{{"id": "agent-1", "displayName": "Agent", "status": "CREATED"}}
	cs := connect(t, s, who(testOrg, "ap:api:rest:read", "ap:llm_provider:read", "ap:api:mcp:read", "ap:api:agent:read"))

	res, text := call(t, cs, "list_apis", map[string]any{"apiType": "rest-api"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "projectId is required for rest-api")

	res, text = call(t, cs, "list_apis", map[string]any{"apiType": "mcp-proxy"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "projectId is required for mcp-proxy")

	res, text = call(t, cs, "list_apis", map[string]any{"apiType": "llm-provider", "projectId": "default"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "llm-provider has no project; leave projectId out")

	res, text = call(t, cs, "list_apis", map[string]any{"apiType": "agent-proxy"})
	require.False(t, res.IsError, "agent-proxy's project is optional: %s", text)
	assert.Contains(t, text, `"status":"CREATED"`, "the other types call it status, not lifeCycleStatus")
}

func TestCheckRoutes_RefusesARouteWithNoScopes(t *testing.T) {
	s, _ := newTestServer(t)
	s.routes["broken"] = []string{"GET /no-such-route"}
	assert.ErrorContains(t, s.checkRoutes(), `tool broken names route "GET /no-such-route"`)

	delete(s.routes, "broken")
	s.routes["broken_typed"], s.takesAPIType["broken_typed"] = []string{"GET {c}/{id}/nothing"}, true
	assert.ErrorContains(t, s.checkRoutes(), `route "GET /rest-apis/{id}/nothing"`, "{c} is expanded per API type")
}

// TestToolContract follows the console agent's main lint rules.
func TestToolContract(t *testing.T) {
	s, _ := newTestServer(t)
	res, err := connect(t, s, who(testOrg, "ap:api:rest:manage", "ap:project:manage", "ap:gateway:manage")).
		ListTools(context.Background(), nil)
	require.NoError(t, err)
	verbNoun := regexp.MustCompile(`^(list|get|create|update|delete|deploy|undeploy)_[a-z_]+$`)
	for _, tool := range res.Tools {
		assert.Regexp(t, verbNoun, tool.Name)
		assert.LessOrEqual(t, len(tool.Title), 40, tool.Name)
		assert.GreaterOrEqual(t, len(tool.Description), 20, tool.Name)
		assert.LessOrEqual(t, len(tool.Description), 400, tool.Name)
		require.NotNil(t, tool.Annotations, tool.Name)
		assert.NotNil(t, tool.OutputSchema, tool.Name)

		var in struct {
			Properties map[string]struct {
				Title string `json:"title"`
			} `json:"properties"`
		}
		b, _ := json.Marshal(tool.InputSchema)
		require.NoError(t, json.Unmarshal(b, &in))
		for name, p := range in.Properties {
			assert.NotEqual(t, "id", name, tool.Name)
			assert.NotEmpty(t, p.Title, "%s.%s has no title", tool.Name, name)
		}
	}
}

func TestLogCall_RecordsRefusedCalls(t *testing.T) {
	s, _ := newTestServer(t)
	var logs bytes.Buffer
	s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	call(t, connect(t, s, who(testOrg, "ap:project:read")), "create_project", map[string]any{"name": "p1"})
	assert.Contains(t, logs.String(), `tool=create_project outcome="denied: missing scope" user=u1 org=`+testOrg)
}

// TestMCPHandler_RefusesALoopbackCallWithAnotherHost keeps go-sdk's guard
// against DNS rebinding on: a call that reaches a local Platform API under
// another host name gets 403.
func TestMCPHandler_RefusesALoopbackCallWithAnotherHost(t *testing.T) {
	s, _ := newTestServer(t)
	srv := httptest.NewServer(s.mcpHandler())
	defer srv.Close()
	post := func(host string) int {
		r, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
		require.NoError(t, err)
		r.Host = host
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		res, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		_ = res.Body.Close()
		return res.StatusCode
	}
	assert.Equal(t, http.StatusForbidden, post("api.example.com"))
	assert.Equal(t, http.StatusOK, post("localhost"))
}

func TestDeleteAPI_ReportsWhatWasDeleted(t *testing.T) {
	s, f := newTestServer(t)
	cs := connect(t, s, who(testOrg, "ap:api:llm:manage"))
	args := map[string]any{"apiType": "llm-proxy", "apiId": "chat-proxy", "confirm": true}

	res, text := call(t, cs, "delete_api", args)
	require.False(t, res.IsError, text)
	assert.JSONEq(t, `{"apiType":"llm-proxy","deleted":"chat-proxy"}`, text)
	assert.Equal(t, []string{"chat-proxy"}, f.deleted)

	f.failDelete = true
	res, text = call(t, cs, "delete_api", args)
	assert.True(t, res.IsError, "a backend error is a tool error")
	assert.NotEmpty(t, text)
	assert.Equal(t, []string{"chat-proxy"}, f.deleted)
}

func TestDeleteProject_ReportsWhatWasDeleted(t *testing.T) {
	s, f := newTestServer(t)
	cs := connect(t, s, who(testOrg, "ap:project:manage"))
	args := map[string]any{"projectId": "p1", "confirm": true}

	res, text := call(t, cs, "delete_project", args)
	require.False(t, res.IsError, text)
	assert.JSONEq(t, `{"deleted":"p1"}`, text)
	assert.Equal(t, []string{"p1"}, f.deleted)

	f.failDelete = true
	res, text = call(t, cs, "delete_project", args)
	assert.True(t, res.IsError)
	assert.Contains(t, text, "project still has APIs")
}

// shortPage answers a deployments list with one item out of three, as a
// backend does when the page is smaller than the total.
type shortPage struct{ *fake }

func (shortPage) listDeployments(_ context.Context, _ caller, _ *apiType, _, _, _ string, p page) ([]api.DeploymentResponse, int, error) {
	id := fmt.Sprintf("aaaaaaaa-0000-0000-0000-00000000000%d", p.offset+1)
	return []api.DeploymentResponse{{DeploymentId: uuidOf(id), GatewayId: "gw1", Status: api.DeploymentResponseStatusDEPLOYED}}, 3, nil
}

func TestListAPIDeployments_ShapesItems(t *testing.T) {
	s, f := newTestServer(t)
	reason := "DEPLOYMENT_TIMEOUT"
	failed := api.DeploymentResponse{DeploymentId: uuidOf("aaaaaaaa-0000-0000-0000-000000000002"), Name: "d", GatewayId: "gw1",
		Status: api.DeploymentResponseStatusFAILED, StatusReason: &reason}
	f.running = []api.DeploymentResponse{
		{DeploymentId: uuidOf("aaaaaaaa-0000-0000-0000-000000000001"), Name: "d", GatewayId: "gw1", Status: api.DeploymentResponseStatusDEPLOYED},
		failed,
	}
	cs := connect(t, s, who(testOrg, "ap:api:rest:manage"))

	res, text := call(t, cs, "list_api_deployments", map[string]any{"apiType": "rest-api", "apiId": "orders-v1"})
	require.False(t, res.IsError, text)
	var out listDeploymentsOut
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.Equal(t, 2, out.Total)
	require.Len(t, out.Items, 2)
	assert.Empty(t, out.NextCursor, "everything fits on one page")
	assert.Equal(t, reason, out.Items[1].StatusReason)
	assert.Equal(t, "DEPLOYED", out.Items[0].Status)
}

func TestListAPIDeployments_PagesWithACursor(t *testing.T) {
	s, f := newTestServer(t)
	s.backend = shortPage{f}
	cs := connect(t, s, who(testOrg, "ap:api:rest:manage"))
	args := map[string]any{"apiType": "rest-api", "apiId": "orders-v1", "limit": 1}

	_, text := call(t, cs, "list_api_deployments", args)
	var first listDeploymentsOut
	require.NoError(t, json.Unmarshal([]byte(text), &first))
	assert.Equal(t, 3, first.Total)
	require.NotEmpty(t, first.NextCursor)

	args["cursor"] = first.NextCursor
	_, text = call(t, cs, "list_api_deployments", args)
	var second listDeploymentsOut
	require.NoError(t, json.Unmarshal([]byte(text), &second))
	assert.NotEqual(t, first.Items[0].ID, second.Items[0].ID, "the cursor moved the offset")

	args["cursor"] = "!"
	res, text := call(t, cs, "list_api_deployments", args)
	assert.True(t, res.IsError)
	assert.Contains(t, text, "the cursor is not valid")
}

// failingDeployments fails every deployments list.
type failingDeployments struct{ *fake }

func (failingDeployments) listDeployments(context.Context, caller, *apiType, string, string, string, page) ([]api.DeploymentResponse, int, error) {
	return nil, 0, apperror.RESTAPINotFound.New()
}

func TestListAPIDeployments_BackendErrorIsAToolError(t *testing.T) {
	s, _ := newTestServer(t)
	s.backend = failingDeployments{&fake{}}
	res, _ := call(t, connect(t, s, who(testOrg, "ap:api:rest:manage")), "list_api_deployments",
		map[string]any{"apiType": "rest-api", "apiId": "orders-v1"})
	assert.True(t, res.IsError)
}
