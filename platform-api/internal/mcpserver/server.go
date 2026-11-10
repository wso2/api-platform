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

// Package mcpserver serves /api/v0.9/mcp: an MCP server whose tools manage
// APIs, projects and gateways for the caller. It is not the MCP proxy feature.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/pdk"
)

const (
	// Path is the MCP endpoint.
	Path = constants.APIBasePath + "/mcp"
	// DiscoveryPath is the RFC 9728 protected-resource metadata for Path.
	DiscoveryPath = "/.well-known/oauth-protected-resource" + Path
)

// Deps is what the MCP server needs from the Platform API.
type Deps struct {
	Config      *config.Server
	Scopes      *middleware.ScopeRegistry
	Projects    *service.ProjectService
	APIs        *service.APIService
	Deployments *service.DeploymentService
	Gateways    *service.GatewayService
	Identity    *service.IdentityService

	LLMProviders           *service.LLMProviderService
	LLMProviderDeployments *service.LLMProviderDeploymentService
	LLMProxies             *service.LLMProxyService
	LLMProxyDeployments    *service.LLMProxyDeploymentService
	MCPProxies             *service.MCPProxyService
	MCPProxyDeployments    *service.MCPDeploymentService
	AgentProxies           *service.AgentProxyService
	AgentProxyDeployments  *service.AgentDeploymentService
	Logger                 *slog.Logger

	// PluginTools are the tools plugins add, or use to replace built-in ones.
	PluginTools []pdk.MCPTool
}

// Server holds the tools and the checks every tool call goes through.
type Server struct {
	cfg      config.MCPServer
	auth     config.Auth
	scopes   *middleware.ScopeRegistry
	identity *service.IdentityService
	backend  backend
	logger   *slog.Logger

	// routes holds each tool's REST routes, "METHOD /path". In an API tool's
	// routes, {c} stands for the API type's collection.
	routes       map[string][]string
	takesAPIType map[string]bool
	schemas      map[string]apiSchemaOut // by API type, for get_api_schema
	sdk          *mcp.Server
}

// New builds the MCP server. It fails when a tool names a route that has no
// scopes in the Platform API's scope table, or when the OpenAPI spec lacks an
// API type's schema. A tool with no routes, like get_api_schema, needs no scope.
func New(d Deps) (*Server, error) {
	s := &Server{
		cfg:          d.Config.MCPServer,
		auth:         d.Config.Auth,
		scopes:       d.Scopes,
		identity:     d.Identity,
		logger:       d.Logger,
		routes:       map[string][]string{},
		takesAPIType: map[string]bool{},
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}

	direct, err := newDirect(d)
	if err != nil {
		return nil, err
	}
	s.backend = direct
	if s.schemas, err = loadAPISchemas(d.Config.OpenAPISpecPath); err != nil {
		return nil, err
	}

	s.sdk = mcp.NewServer(&mcp.Implementation{Name: "api-platform", Version: constants.APIVersion}, nil)
	s.addTools(s.sdk)
	if err := s.addPluginTools(s.sdk, d.PluginTools); err != nil {
		return nil, err
	}
	if err := s.checkRoutes(); err != nil {
		return nil, err
	}
	s.sdk.AddReceivingMiddleware(s.filterToolsList)
	return s, nil
}

// Register adds the MCP routes to mux and returns the paths the Platform API's
// sign-in must skip: only the discovery route. /mcp signs in like any route.
func (s *Server) Register(mux *http.ServeMux) []string {
	mux.Handle("POST "+Path, s.authenticate(s.mcpHandler()))
	var skip []string
	if s.discoveryEnabled() {
		mux.HandleFunc("GET "+DiscoveryPath, s.discovery)
		skip = append(skip, DiscoveryPath)
	}
	return skip
}

// mcpHandler is the SDK's Streamable HTTP handler, stateless, before auth.
func (s *Server) mcpHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.sdk }, &mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: s.cfg.MaxRequestBytes,
	})
}

// add registers one tool. Every tool goes through here, so the checks live in
// one place. routes are the REST routes the tool stands for; a tool with none
// needs no scope. A destructive tool also takes confirm, and runs only when it
// is true.
func add[In, Out any](s *Server, srv *mcp.Server, t *mcp.Tool, routes []string,
	run func(ctx context.Context, c caller, in In) (Out, error)) {

	s.routes[t.Name] = routes
	_, s.takesAPIType[t.Name] = any(*new(In)).(typedInput)
	schema, _ := t.InputSchema.(*jsonschema.Schema)
	if schema == nil { // a plugin tool comes with its own
		schema = s.inputSchema(inputSchemaFor[In](), s.takesAPIType[t.Name])
	}
	destructive := t.Annotations != nil && t.Annotations.DestructiveHint != nil && *t.Annotations.DestructiveHint
	if destructive {
		addConfirm(schema)
	}
	t.InputSchema = schema

	mcp.AddTool(srv, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var none Out
		c, ok := callerFrom(ctx)
		if !ok {
			return nil, none, errors.New("not signed in")
		}
		toolRoutes, err := s.routesFor(t.Name, in)
		if err != nil {
			return nil, none, s.toolError(t.Name, err)
		}
		if s.auth.Authorization.Enabled { // as in the REST API's scope check
			if need := s.missingScope(c, toolRoutes); need != "" {
				s.logCall(t.Name, c, "denied: missing scope")
				return nil, none, fmt.Errorf("missing permission: this needs %s", need)
			}
		}
		if destructive && !confirmed(req) {
			s.logCall(t.Name, c, "refused: not confirmed")
			return nil, none, fmt.Errorf("refusing to run %s without confirm=true. Set it only for the exact target the user asked for, then call again", t.Name)
		}
		out, err := recovered(s.logger, t.Name, func() (Out, error) { return run(ctx, c, in) })
		if err != nil {
			s.logCall(t.Name, c, "error")
			return nil, none, s.toolError(t.Name, err)
		}
		s.logCall(t.Name, c, "ok")
		return nil, out, nil
	})
}

// addConfirm adds the required confirm input. It is for clients that run calls
// without asking the user.
func addConfirm(schema *jsonschema.Schema) {
	if schema.Properties == nil { // an input struct with no fields
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	schema.Properties["confirm"] = &jsonschema.Schema{
		Type:        "boolean",
		Title:       "Confirm",
		Description: "Must be true. Set it only for the exact target the user asked for. If your client does not ask the user to approve each call, ask them first.",
	}
	schema.PropertyOrder = append(schema.PropertyOrder, "confirm")
	schema.Required = append(schema.Required, "confirm")
}

// confirmed reports whether the call sent confirm=true. The schema has already
// checked that confirm is there and is a boolean.
func confirmed(req *mcp.CallToolRequest) bool {
	var args struct {
		Confirm bool `json:"confirm"`
	}
	return json.Unmarshal(req.Params.Arguments, &args) == nil && args.Confirm
}

// recovered runs a tool, turning a panic into an error. go-sdk runs each call on
// its own goroutine with no recover, so a panic would end the process.
func recovered[Out any](logger *slog.Logger, tool string, run func() (Out, error)) (out Out, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("MCP tool panicked", "tool", tool, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			err = fmt.Errorf("tool %s panicked", tool)
		}
	}()
	return run()
}

// logCall records every tool call, refused ones included.
func (s *Server) logCall(tool string, c caller, outcome string) {
	s.logger.Info("MCP tool call", "tool", tool, "outcome", outcome, "user", c.user, "org", c.org)
}

// routesFor returns the routes one call of the tool uses.
func (s *Server) routesFor(tool string, in any) ([]string, error) {
	ti, ok := in.(typedInput)
	if !ok {
		return s.routes[tool], nil
	}
	t, err := apiTypeNamed(ti.typeName())
	if err != nil {
		return nil, err
	}
	return t.expand(s.routes[tool]), nil
}

// missingScope returns the scopes a route needs when the caller holds none of
// them, as "a or b". Any one scope per route is enough, as in the REST API.
func (s *Server) missingScope(c caller, routes []string) string {
	for _, r := range routes {
		need := s.routeScopes(r)
		if len(need) == 0 {
			return "a route with no scopes: " + r
		}
		if !slices.ContainsFunc(need, func(sc string) bool { return c.scopes[sc] }) {
			return strings.Join(need, " or ")
		}
	}
	return ""
}

func (s *Server) routeScopes(route string) []string {
	method, path, _ := strings.Cut(route, " ")
	need, _ := s.scopes.Lookup(method, constants.APIBasePath+path)
	return need
}

// canUseAnyType reports whether the caller may use an API tool for at least
// one API type.
func (s *Server) canUseAnyType(c caller, tool string) bool {
	return slices.ContainsFunc(apiTypes, func(t apiType) bool {
		return s.missingScope(c, t.expand(s.routes[tool])) == ""
	})
}

// filterToolsList shows a caller only the tools they may use: all of them when
// the Platform API's scope checks are off.
// An API tool lists every API type; a call for one the caller cannot use gets
// the usual missing-permission error.
func (s *Server) filterToolsList(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		list, ok := res.(*mcp.ListToolsResult)
		if err != nil || !ok {
			return res, err
		}
		c, signedIn := callerFrom(ctx)
		kept := make([]*mcp.Tool, 0, len(list.Tools))
		for _, t := range list.Tools {
			if !signedIn {
				break
			}
			if !s.auth.Authorization.Enabled ||
				s.takesAPIType[t.Name] && s.canUseAnyType(c, t.Name) ||
				!s.takesAPIType[t.Name] && s.missingScope(c, s.routes[t.Name]) == "" {
				kept = append(kept, t)
			}
		}
		list.Tools = kept
		list.CacheScope = "private" // the list differs per caller
		return list, nil
	}
}

// allRoutes is every route a tool can call, across all API types it serves.
func (s *Server) allRoutes(tool string) []string {
	if !s.takesAPIType[tool] {
		return s.routes[tool]
	}
	var all []string
	for _, t := range apiTypes {
		all = append(all, t.expand(s.routes[tool])...)
	}
	return all
}

// checkRoutes refuses to start when a tool names a route that has no scopes.
func (s *Server) checkRoutes() error {
	for tool := range s.routes {
		for _, r := range s.allRoutes(tool) {
			if len(s.routeScopes(r)) == 0 {
				return fmt.Errorf("mcp server: tool %s names route %q, which has no scopes in the OpenAPI spec", tool, r)
			}
		}
	}
	return nil
}

// allScopes is every scope any tool can need, for login discovery.
func (s *Server) allScopes() []string {
	set := map[string]bool{}
	for tool := range s.routes {
		for _, r := range s.allRoutes(tool) {
			for _, sc := range s.routeScopes(r) {
				set[sc] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for sc := range set {
		out = append(out, sc)
	}
	slices.Sort(out)
	return out
}

// inputSchemaFor builds a tool's input schema from its Go type.
func inputSchemaFor[In any]() *jsonschema.Schema {
	sch, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err) // a bad input type is a programming error; fail at startup
	}
	return sch
}

// inputTitles are the labels the console agent's approval card shows.
var inputTitles = map[string]string{
	"apiType":     "API Type",
	"apiId":       "API",
	"projectId":   "Project",
	"gatewayId":   "Gateway",
	"payload":     "Payload",
	"name":        "Name",
	"description": "Description",
	"cursor":      "Cursor",
	"limit":       "Limit",
}

// inputSchema adds titles to every input and, for an API tool, the list of API
// types that are switched on.
func (s *Server) inputSchema(sch *jsonschema.Schema, takesAPIType bool) *jsonschema.Schema {
	for name, p := range sch.Properties {
		p.Title = inputTitles[name]
	}
	if takesAPIType {
		enum := make([]any, 0, len(apiTypes))
		for _, t := range apiTypes {
			enum = append(enum, t.name)
		}
		sch.Properties["apiType"].Enum = enum
	}
	return sch
}

// toolError turns an error into one sentence the model can act on.
func (s *Server) toolError(tool string, err error) error {
	var ae *apperror.Error
	if errors.As(err, &ae) && ae.HTTPStatus != http.StatusInternalServerError {
		msg := ae.Message
		for _, fe := range ae.FieldErrors {
			msg += fmt.Sprintf(" %s: %s.", fe.Field, fe.Message)
		}
		return errors.New(msg)
	}
	var ce *pdk.ClientError
	if errors.As(err, &ce) && ce != nil && ce.Message != "" && ce.Status != http.StatusInternalServerError {
		return errors.New(ce.Message)
	}
	s.logger.Error("MCP tool failed", "tool", tool, "error", err)
	return errors.New("an unexpected error occurred; try again later")
}

// fail is an input error the model can fix.
func fail(format string, args ...any) error {
	return apperror.ValidationFailed.New(fmt.Sprintf(format, args...))
}
