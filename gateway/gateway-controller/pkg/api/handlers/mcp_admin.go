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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
)

const (
	adminRouteKeyXDSSyncStatus = "GET /xds_sync_status"
	adminRouteKeyConfigDump    = "GET /config_dump"

	toolAdminGatewayStatus = "wso2_apip_gw_get_gateway_status"
	toolAdminConfigDump    = "wso2_apip_gw_get_config_dump"
)

// Returns every route key the admin MCP could ever reach
func AdminMCPRouteKeys() []string {
	return []string{adminRouteKeyConfigDump, adminRouteKeyXDSSyncStatus}
}

// AdminStatusSource is the read-only slice of *APIServer the admin MCP tools need.
// An interface rather than the concrete type, so no mutating method is reachable
// from this endpoint by construction because the admin MCP surface is read-only
type AdminStatusSource interface {
	GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse
	BuildConfigDumpResponse(log *slog.Logger) (*adminapi.ConfigDumpResponse, error)
}

// adminHealthResponse mirrors the payload adminserver.Server.GetHealth writes for GET /health.
func adminHealthResponse() map[string]string {
	return map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
}

// AdminMcpHandler serves the administrative Model Context Protocol endpoint on
// the admin port. Built once at startup and reused for every request.
type AdminMcpHandler struct {
	// protected is the SDK's Streamable HTTP handler wrapped in the per-tool
	// authorization gate. Everything the mux serves goes through this field.
	protected http.Handler

	// both MCPs share the same authorization core and the OAuth challenge
	// and discovery helpers, so the 401/403 step-up behaves identically on both.
	authz  *mcpAuthz
	status AdminStatusSource

	// configDumpEnabled mirrors controller.admin_server.config_dump.enabled.
	// False leaves the config-dump tool unregistered entirely.
	configDumpEnabled bool

	logger *slog.Logger
}

// AdminMcpHandlerParams collects NewAdminMcpHandler's dependencies.
type AdminMcpHandlerParams struct {
	Status              AdminStatusSource
	ResourceRoles       map[string][]string
	RoleMapping         map[string][]string
	ResourceMetadataURL string
	MaxRequestBytes     int64
	ConfigDumpEnabled   bool

	Logger *slog.Logger
}

// NewAdminMcpHandler builds the admin MCP server, registers its tools, and
// wraps the SDK handler in the authorization gate.
func NewAdminMcpHandler(p AdminMcpHandlerParams) *AdminMcpHandler {
	h := &AdminMcpHandler{
		status:            p.Status,
		configDumpEnabled: p.ConfigDumpEnabled,
		logger:            p.Logger,
		authz: newMcpAuthz(mcpAuthzParams{
			ResourceRoles:       p.ResourceRoles,
			RoleMapping:         p.RoleMapping,
			ResourceMetadataURL: p.ResourceMetadataURL,
			MaxRequestBytes:     p.MaxRequestBytes,
			Logger:              p.Logger,
		}),
	}

	// Assigned before the gate is built. Until this line the default resolver
	// maps nothing, so a construction path that skipped it would fail closed.
	h.authz.routeKeysForCall = h.routeKeysForCall

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wso2-api-platform-gateway-controller-admin",
		Version: version.Version,
	}, nil)
	h.registerTools(server)

	streamHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:                    true,
			MaxRequestBodyBytes:          h.authz.maxRequestBytes,
			PropagateRequestCancellation: true,
			Logger:                       p.Logger,
		},
	)

	h.protected = h.authz.ScopeGate(streamHandler)
	return h
}

// ServeHTTP implements http.Handler. Served by adminserver.Server.HandleAdminMcp
// behind the generated router's middleware: the OAuth challenge, authentication
// and baseline authorization, and the IP allowlist.
func (h *AdminMcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.protected.ServeHTTP(w, r)
}

// routeKeysForCall maps an admin tool call to the admin REST route key that
// governs the equivalent operation.
func (h *AdminMcpHandler) routeKeysForCall(tool string, _ json.RawMessage) ([]string, bool) {
	switch tool {
	case toolAdminGatewayStatus:
		return []string{adminRouteKeyXDSSyncStatus}, true
	case toolAdminConfigDump:
		if !h.configDumpEnabled {
			return nil, false
		}
		return []string{adminRouteKeyConfigDump}, true
	default:
		return nil, false
	}
}

// MCPRouteKeys is every admin route key a REGISTERED tool can reach.
func (h *AdminMcpHandler) MCPRouteKeys() []string {
	keys := []string{adminRouteKeyXDSSyncStatus}
	if h.configDumpEnabled {
		keys = append(keys, adminRouteKeyConfigDump)
	}
	sort.Strings(keys)
	return keys
}

// MCPBaselineRoles is the union of every role that can call at least one admin
// tool. Used as the default entry set main.go projects through
// MapRolesToScopes to build the advertised scopes, and as the allow-list its
// fail-closed check validates operator-configured entries against.
// It returns LOCAL ROLES and must keep doing so: it reads the route role map,
// and callers either compare against that map or project explicitly.
func (h *AdminMcpHandler) MCPBaselineRoles() []string {
	seen := map[string]struct{}{}
	for _, key := range h.MCPRouteKeys() {
		if roles, ok := h.authz.rolesFor(key); ok {
			for _, r := range roles {
				seen[r] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// gatewayStatusInput takes no arguments
type gatewayStatusInput struct{}

// configDumpInput takes no arguments
type configDumpInput struct{}

func (h *AdminMcpHandler) registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  toolAdminGatewayStatus,
		Title: "Get gateway runtime status",
		Description: `Report whether this Gateway's controller is healthy and how far its
configuration has propagated.

Returns the controller's liveness status together with the current xDS policy
chain version — the version most recently published to the router and the policy
engine. Use it to answer "is the gateway up" and "has my change been pushed":
read the version before a deploy and again after, and if it has not advanced the
change has not reached the data plane yet.

Takes no arguments and changes nothing.`,
		Annotations: &mcp.ToolAnnotations{
			Title:         "Get gateway runtime status",
			ReadOnlyHint:  true,
			OpenWorldHint: ptr(false),
		},
	}, h.getGatewayStatus)

	if !h.configDumpEnabled {
		return
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:  toolAdminConfigDump,
		Title: "Dump the gateway's current configuration",
		Description: `Return this Gateway's current configuration state: every deployed
resource with its stored manifest and deployment status, every loaded policy
definition, every registered certificate, and summary statistics.

This is the whole configuration in one response and can be large on a gateway
with many resources. When you only need one resource's definition, prefer the
management endpoint's get_resource tool; use this when you need the deployed
state as a whole, or to compare what is deployed against what was intended.

Takes no arguments and changes nothing.`,
		Annotations: &mcp.ToolAnnotations{
			Title:         "Dump the gateway's current configuration",
			ReadOnlyHint:  true,
			OpenWorldHint: ptr(false),
		},
	}, h.getConfigDump)
}

func (h *AdminMcpHandler) getGatewayStatus(
	ctx context.Context, _ *mcp.CallToolRequest, _ gatewayStatusInput,
) (*mcp.CallToolResult, any, error) {
	// The same key the gate authorized. Repeated here so the tool cannot
	// execute if the handler is ever mounted without the gate
	if err := h.authz.authorize(ctx, adminRouteKeyXDSSyncStatus); err != nil {
		return nil, nil, err
	}

	return nil, map[string]any{
		"health":   adminHealthResponse(),
		"xds_sync": h.status.GetXDSSyncStatusResponse(),
	}, nil
}

func (h *AdminMcpHandler) getConfigDump(
	ctx context.Context, _ *mcp.CallToolRequest, _ configDumpInput,
) (*mcp.CallToolResult, any, error) {
	if !h.configDumpEnabled {
		return nil, nil, fmt.Errorf("the configuration dump is not enabled on this Gateway")
	}
	if err := h.authz.authorize(ctx, adminRouteKeyConfigDump); err != nil {
		return nil, nil, err
	}

	dump, err := h.status.BuildConfigDumpResponse(h.logger)
	if err != nil {
		h.logger.Error("MCP "+toolAdminConfigDump+" failed", slog.Any("error", err))
		return nil, nil, fmt.Errorf("the Gateway could not produce a configuration dump")
	}
	return nil, dump, nil
}
