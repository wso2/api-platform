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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/pdk"
)

type echoInput struct {
	Name  string `json:"name" jsonschema:"A name."`
	Color string `json:"color,omitempty" jsonschema:"A color."`
}

type confirmInput struct {
	Name    string `json:"name" jsonschema:"A name."`
	Confirm bool   `json:"confirm" jsonschema:"Whether to go ahead."`
}

func echoTool(name string, run func(context.Context, pdk.MCPCall) (any, error)) pdk.MCPTool {
	return pdk.MCPTool{
		Name: name, Title: "Echo", Description: "Echoes its input for the test.",
		Risk: pdk.MCPRead, Routes: []string{"GET /projects"}, Input: echoInput{}, Run: run,
	}
}

func newPluginServer(t *testing.T, tools ...pdk.MCPTool) (*Server, error) {
	t.Helper()
	scopes, err := middleware.LoadScopeRegistry(specPath)
	require.NoError(t, err)
	cfg := &config.Server{OpenAPISpecPath: specPath}
	cfg.Auth.Authorization.Enabled = true
	cfg.MCPServer = config.MCPServer{Enabled: true}
	s, err := New(Deps{Config: cfg, Scopes: scopes, PluginTools: tools})
	if s != nil {
		s.backend = &fake{}
	}
	return s, err
}

func TestPluginTool_IsListedAndRuns(t *testing.T) {
	var got pdk.MCPCall
	s, err := newPluginServer(t, echoTool("echo_thing", func(_ context.Context, call pdk.MCPCall) (any, error) {
		got = call
		return map[string]string{"ok": "yes"}, nil
	}))
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	assert.Contains(t, toolNames(t, cs), "echo_thing")
	res, text := call(t, cs, "echo_thing", map[string]any{"name": "a"})
	require.False(t, res.IsError, text)
	assert.JSONEq(t, `{"ok":"yes"}`, text)
	assert.JSONEq(t, `{"name":"a"}`, string(got.Arguments))
	assert.Equal(t, testOrg, got.Org)
	assert.Equal(t, "u1", got.User)
}

// A plugin's middleware hands its tools values through the request's context.
func TestPluginTool_GetsTheRequestContext(t *testing.T) {
	type key struct{}
	var got any
	s, err := newPluginServer(t, echoTool("echo_thing", func(ctx context.Context, _ pdk.MCPCall) (any, error) {
		got = ctx.Value(key{})
		return map[string]string{}, nil
	}))
	require.NoError(t, err)
	h := s.mcpHandler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), key{}, "from middleware"))
		h.ServeHTTP(w, withCaller(r, who(testOrg, "ap:project:read")))
	}))
	t.Cleanup(srv.Close)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, text := call(t, cs, "echo_thing", map[string]any{"name": "a"})
	require.False(t, res.IsError, text)
	assert.Equal(t, "from middleware", got)
}

func TestPluginTool_ReplacesABuiltIn(t *testing.T) {
	s, err := newPluginServer(t, echoTool("list_projects", func(context.Context, pdk.MCPCall) (any, error) {
		return map[string]string{"from": "plugin"}, nil
	}))
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	assert.ElementsMatch(t, []string{"list_projects", "get_api_schema"}, toolNames(t, cs))
	_, text := call(t, cs, "list_projects", map[string]any{"name": "x"})
	assert.JSONEq(t, `{"from":"plugin"}`, text, "the built-in tool does not run")
}

// A plugin's deploy_api names plain routes: no API type, no {c} to expand.
func TestPluginTool_ReplacesAnAPITypeBuiltIn(t *testing.T) {
	tool := echoTool("deploy_api", func(context.Context, pdk.MCPCall) (any, error) { return map[string]string{}, nil })
	tool.Routes = []string{"POST /rest-apis/{id}/deployments"}
	s, err := newPluginServer(t, tool)
	require.NoError(t, err)

	assert.False(t, s.takesAPIType["deploy_api"])
	assert.NotContains(t, toolNames(t, connect(t, s, who(testOrg, "ap:llm_proxy:manage"))), "deploy_api")
	cs := connect(t, s, who(testOrg, "ap:rest_api:manage"))
	assert.Contains(t, toolNames(t, cs), "deploy_api")
	res, text := call(t, cs, "deploy_api", map[string]any{"name": "a"})
	assert.False(t, res.IsError, text)
}

func TestNew_RefusesAMalformedPluginTool(t *testing.T) {
	run := func(context.Context, pdk.MCPCall) (any, error) { return nil, nil }
	with := func(change func(*pdk.MCPTool)) pdk.MCPTool {
		tool := echoTool("echo_thing", run)
		change(&tool)
		return tool
	}
	cases := map[string]struct {
		tool pdk.MCPTool
		want string
	}{
		"no name":              {with(func(tool *pdk.MCPTool) { tool.Name = "" }), "needs a name"},
		"no input":             {with(func(tool *pdk.MCPTool) { tool.Input = nil }), "must be a struct value"},
		"pointer input":        {with(func(tool *pdk.MCPTool) { tool.Input = &echoInput{} }), "must be a struct value"},
		"input not a struct":   {with(func(tool *pdk.MCPTool) { tool.Input = "text" }), "must be a struct value"},
		"no run":               {with(func(tool *pdk.MCPTool) { tool.Run = nil }), "needs a name"},
		"no routes":            {with(func(tool *pdk.MCPTool) { tool.Routes = nil }), "needs a name"},
		"no risk":              {with(func(tool *pdk.MCPTool) { tool.Risk = 0 }), "needs a name"},
		"route with no scopes": {with(func(tool *pdk.MCPTool) { tool.Routes = []string{"GET /no-such-route"} }), "no scopes"},
		"destructive with its own confirm": {with(func(tool *pdk.MCPTool) {
			tool.Risk, tool.Input = pdk.MCPDestructive, confirmInput{}
		}), "gets confirm from the server"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newPluginServer(t, tc.tool)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestPluginTool_ErrorsKeepOnlyWhatIsSafeToShow(t *testing.T) {
	fails := func(name string, err error) pdk.MCPTool {
		return echoTool(name, func(context.Context, pdk.MCPCall) (any, error) { return nil, err })
	}
	s, err := newPluginServer(t,
		fails("refuse_thing", &pdk.ClientError{Status: 409, Code: "conflict", Message: "It already exists."}),
		fails("break_thing", &pdk.ClientError{Status: 500, Code: "internal", Message: "database is down"}),
		fails("empty_thing", &pdk.ClientError{Status: 409}),
		fails("other_thing", assert.AnError))
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	res, text := call(t, cs, "refuse_thing", map[string]any{"name": "a"})
	assert.True(t, res.IsError)
	assert.Equal(t, "It already exists.", text)
	for _, name := range []string{"break_thing", "empty_thing", "other_thing"} {
		res, text = call(t, cs, name, map[string]any{"name": "a"})
		assert.True(t, res.IsError, name)
		assert.Equal(t, "an unexpected error occurred; try again later", text, name)
	}
}

// go-sdk runs each call on its own goroutine with no recover, so an unrecovered
// panic would end the process for every caller.
func TestTool_PanicBecomesAnError(t *testing.T) {
	s, err := newPluginServer(t, echoTool("crash_thing", func(context.Context, pdk.MCPCall) (any, error) {
		var nothing *pdk.MCPCall
		return nothing.Org, nil
	}))
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	for range 2 {
		res, text := call(t, cs, "crash_thing", map[string]any{"name": "a"})
		assert.True(t, res.IsError)
		assert.Equal(t, "an unexpected error occurred; try again later", text)
	}
}

func TestPluginTool_NeedsItsRoutesScope(t *testing.T) {
	ran := false
	s, err := newPluginServer(t, echoTool("echo_thing", func(context.Context, pdk.MCPCall) (any, error) {
		ran = true
		return nil, nil
	}))
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:gateway:read"))

	assert.NotContains(t, toolNames(t, cs), "echo_thing")
	res, text := call(t, cs, "echo_thing", map[string]any{"name": "a"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "missing permission: this needs ap:project:")
	assert.False(t, ran)
}

func TestPluginTool_Schema(t *testing.T) {
	tool := echoTool("echo_thing", func(context.Context, pdk.MCPCall) (any, error) { return nil, nil })
	tool.Titles = map[string]string{"color": "Paint"}
	s, err := newPluginServer(t, tool)
	require.NoError(t, err)
	res, err := connect(t, s, who(testOrg, "ap:project:read")).ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, res.Tools, 3) // echo_thing, list_projects and get_api_schema

	for _, listed := range res.Tools {
		if listed.Name != "echo_thing" {
			continue
		}
		var schema struct {
			Required   []string `json:"required"`
			Properties map[string]struct {
				Title string `json:"title"`
			} `json:"properties"`
		}
		b, _ := json.Marshal(listed.InputSchema)
		require.NoError(t, json.Unmarshal(b, &schema))
		assert.Equal(t, "Paint", schema.Properties["color"].Title)
		assert.Equal(t, "Name", schema.Properties["name"].Title, "the Platform API's own title")
		assert.Equal(t, []string{"name"}, schema.Required)
		assert.Equal(t, read, listed.Annotations)
		return
	}
	t.Fatal("echo_thing is not listed")
}

type apiTypeInput struct {
	APIType string `json:"apiType" jsonschema:"The API type."`
}

// A plugin input with an apiType field lists the same API types as the built-in tools.
func TestPluginTool_APITypeGetsTheEnum(t *testing.T) {
	typed := echoTool("typed_thing", func(context.Context, pdk.MCPCall) (any, error) { return nil, nil })
	typed.Input = apiTypeInput{}
	plain := echoTool("plain_thing", func(context.Context, pdk.MCPCall) (any, error) { return nil, nil })
	s, err := newPluginServer(t, typed, plain)
	require.NoError(t, err)
	res, err := connect(t, s, who(testOrg, "ap:project:read")).ListTools(context.Background(), nil)
	require.NoError(t, err)

	var want []any
	for _, apiKind := range apiTypes {
		want = append(want, apiKind.name)
	}
	found := map[string]bool{}
	for _, listed := range res.Tools {
		var schema struct {
			Properties map[string]struct {
				Enum []any `json:"enum"`
			} `json:"properties"`
		}
		b, _ := json.Marshal(listed.InputSchema)
		require.NoError(t, json.Unmarshal(b, &schema))
		switch listed.Name {
		case "typed_thing":
			found[listed.Name] = true
			assert.Equal(t, want, schema.Properties["apiType"].Enum)
		case "plain_thing":
			found[listed.Name] = true
			assert.NotContains(t, schema.Properties, "apiType")
		}
	}
	assert.Len(t, found, 2, "both tools are listed")
}

type numericAPITypeInput struct {
	APIType int `json:"apiType" jsonschema:"A number."`
}

// An apiType field that is not a string keeps its own schema, so a call with it still runs.
func TestPluginTool_NonStringAPITypeGetsNoEnum(t *testing.T) {
	ran := false
	tool := echoTool("numeric_thing", func(context.Context, pdk.MCPCall) (any, error) {
		ran = true
		return map[string]string{"ok": "yes"}, nil
	})
	tool.Input = numericAPITypeInput{}
	s, err := newPluginServer(t, tool)
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	listed, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, entry := range listed.Tools {
		if entry.Name != "numeric_thing" {
			continue
		}
		var schema struct {
			Properties map[string]struct {
				Type string `json:"type"`
				Enum []any  `json:"enum"`
			} `json:"properties"`
		}
		b, _ := json.Marshal(entry.InputSchema)
		require.NoError(t, json.Unmarshal(b, &schema))
		assert.Equal(t, "integer", schema.Properties["apiType"].Type)
		assert.Empty(t, schema.Properties["apiType"].Enum)
	}

	res, text := call(t, cs, "numeric_thing", map[string]any{"apiType": 7})
	require.False(t, res.IsError, text)
	assert.True(t, ran, "the call reaches Run")
}

// A destructive plugin tool takes confirm too, and runs only when it is true.
func TestPluginTool_DestructiveNeedsConfirm(t *testing.T) {
	ran := false
	tool := echoTool("delete_thing", func(context.Context, pdk.MCPCall) (any, error) {
		ran = true
		return map[string]string{}, nil
	})
	tool.Risk = pdk.MCPDestructive
	s, err := newPluginServer(t, tool)
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	res, text := call(t, cs, "delete_thing", map[string]any{"name": "a", "confirm": false})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "refusing to run delete_thing without confirm=true")
	assert.False(t, ran)

	res, text = call(t, cs, "delete_thing", map[string]any{"name": "a", "confirm": true})
	assert.False(t, res.IsError, text)
	assert.True(t, ran)
}

// An input with no fields has no properties map for confirm to go in.
func TestPluginTool_DestructiveWithNoInput(t *testing.T) {
	tool := echoTool("delete_all", func(context.Context, pdk.MCPCall) (any, error) { return map[string]string{}, nil })
	tool.Risk, tool.Input = pdk.MCPDestructive, struct{}{}
	s, err := newPluginServer(t, tool)
	require.NoError(t, err)

	res, text := call(t, connect(t, s, who(testOrg, "ap:project:read")), "delete_all", map[string]any{"confirm": true})
	assert.False(t, res.IsError, text)
}

func TestPluginTool_ArgumentsAreCheckedFirst(t *testing.T) {
	ran := false
	s, err := newPluginServer(t, echoTool("echo_thing", func(context.Context, pdk.MCPCall) (any, error) {
		ran = true
		return nil, nil
	}))
	require.NoError(t, err)
	cs := connect(t, s, who(testOrg, "ap:project:read"))

	res, text := call(t, cs, "echo_thing", map[string]any{"color": "red"})
	assert.True(t, res.IsError)
	assert.Contains(t, text, "name")
	assert.False(t, ran)
}
