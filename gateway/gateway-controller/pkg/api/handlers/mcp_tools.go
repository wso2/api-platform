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
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
)

// McpHandler serves the Model Context Protocol endpoint. Built once at startup and reused for every request.
type McpHandler struct {
	// protected is the SDK's Streamable HTTP handler wrapped in the per-tool
	// authorization gate. Everything the mux serves goes through this field.
	protected http.Handler

	restAPIService       *restapi.RestAPIService
	mcpDeploymentService *utils.MCPDeploymentService
	llmDeploymentService *utils.LLMDeploymentService

	kinds map[string]*kindOps // kind -&gt; service adapters; see mcp_kinds.go

	// resourceRoles is the management API's route-&gt;roles map, exactly as built
	// by generateAuthConfig in cmd/controller/main.go. MCP calls are authorized
	// against it by relative key ("POST /llm-providers"), so the REST API and
	// the MCP endpoint can never drift apart. See mcp_authz.go.
	resourceRoles map[string][]string

	// resourceMetadataURL is the RFC 9728 document URL advertised in
	// WWW-Authenticate challenges. Empty disables the pointer.
	resourceMetadataURL string

	// immutable mirrors ImmutableGateway.Enabled. The MCP route is deliberately
	// NOT wrapped in the immutable middleware (that middleware rejects every
	// POST, which would disable reads too, since every MCP call is a POST), so
	// the write tools enforce the invariant themselves.
	immutable bool

	maxRequestBytes int64
	logger          *slog.Logger

	// pushArtifactUndeploy is the shared control-plane (DP-&gt;CP) push hook,
	// wired from APIServer so an artifact deleted through an MCP tool takes the
	// same path as one deleted through the REST handlers. Nil when no control
	// plane is configured (e.g. in unit tests).
	pushArtifactUndeploy func(cfg *models.StoredConfig, log *slog.Logger)
}

// McpHandlerParams collects newMcpHandler's dependencies. A struct rather than
// a positional list because the handler needs services, policy data and config
// from three different layers.
type McpHandlerParams struct {
	RestAPIService       *restapi.RestAPIService
	MCPDeploymentService *utils.MCPDeploymentService
	LLMDeploymentService *utils.LLMDeploymentService
	ResourceRoles        map[string][]string
	ResourceMetadataURL  string
	Immutable            bool
	MaxRequestBytes      int64
	Logger               *slog.Logger
}

// newMcpHandler builds the MCP server, registers the six tools, and wraps the
// SDK handler in the authorization gate.
//
// It returns an error rather than panicking when a registered tool has no
// authorization mapping: a tool that cannot be authorized must never be served,
// and finding that at boot is the whole point of the check.
func newMcpHandler(p McpHandlerParams) (*McpHandler, error) {
	h := &McpHandler{
		restAPIService:       p.RestAPIService,
		mcpDeploymentService: p.MCPDeploymentService,
		llmDeploymentService: p.LLMDeploymentService,
		resourceRoles:        p.ResourceRoles,
		resourceMetadataURL:  p.ResourceMetadataURL,
		immutable:            p.Immutable,
		maxRequestBytes:      p.MaxRequestBytes,
		logger:               p.Logger,
	}
	if h.maxRequestBytes <= 0 {
		h.maxRequestBytes = mcp.DefaultMaxRequestBodyBytes
	}

	// Built after h exists because each kindOps closes over h's services.
	h.kinds = h.buildKindRegistry()

	if err := h.validateAuthorizationCoverage(); err != nil {
		return nil, err
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wso2-api-platform-gateway-controller",
		Version: version.Version,
	}, nil)
	h.registerTools(server)

	streamHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			// MCP 2026-07-28 removed protocol-level sessions and the GET
			// stream. Stateless mode implements exactly that: no
			// Mcp-Session-Id is read or minted, and GET/DELETE answer 405.
			Stateless: true,
			// The gate reads the body first; keep the SDK's ceiling identical
			// so the two layers reject oversized bodies at the same threshold.
			MaxRequestBodyBytes: h.maxRequestBytes,
			// A closed HTTP request means the response can no longer be
			// delivered, so cancelling the in-flight handler is safe and stops
			// work on an abandoned deploy.
			PropagateRequestCancellation: true,
			Logger:                       p.Logger,
		},
	)

	h.protected = h.ScopeGate(streamHandler)
	return h, nil
}

// ServeHTTP implements http.Handler. Registered on the mux in main.go behind
// the authentication and baseline authorization middleware.
func (h *McpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.protected.ServeHTTP(w, r)
}

// -------------------------------------------------------------------------
// Tool inputs
//
// Typed inputs, not hand-parsed maps: the SDK generates the input schema from
// these structs and validates arguments against it before the handler runs, so
// a malformed call never reaches a service. Fields without `omitempty` are
// required in the generated schema.
// -------------------------------------------------------------------------

type deployInput struct {
	Yaml string `json:"yaml" jsonschema:"complete resource manifest in YAML or JSON; must declare apiVersion, kind, metadata and spec"`
	ID   string `json:"id,omitempty" jsonschema:"handle (metadata.name) of an existing resource; present means update, absent means create"`
}

type deleteInput struct {
	Kind    string `json:"kind" jsonschema:"resource kind"`
	ID      string `json:"id" jsonschema:"resource handle (metadata.name)"`
	Confirm bool   `json:"confirm" jsonschema:"must be true; guards against accidental deletion"`
}

type getInput struct {
	Kind string `json:"kind" jsonschema:"resource kind"`
	ID   string `json:"id" jsonschema:"resource handle (metadata.name)"`
}

type listInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"restrict to one kind; omit for a cross-kind inventory"`
}

// -------------------------------------------------------------------------
// Tool registration
//
// One tool per intent, not per kind. The kind comes from the manifest for the
// create/update tools and from an explicit argument for the rest, so
// supporting a new resource kind means one entry in mcp_kinds.go and no tool
// changes at all.
// -------------------------------------------------------------------------

func (h *McpHandler) registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "deploy_api",
		Title: "Deploy or update a routable resource",
		Description: `Create or update any routable resource on this Gateway.

The kind is read from the manifest's "kind" field — do not guess it and do not
pass it separately. Routable kinds: RestApi, Mcp, LlmProxy, LlmProvider.

For supporting configuration (LlmProviderTemplate) use apply_config instead;
this tool rejects it.

Omit "id" to create. Pass "id" (the existing resource's handle, i.e. its
metadata.name) to update — read the current state with get_resource first, then
submit the full updated manifest.

Authorization follows the equivalent management REST operation: LlmProvider
writes require the admin scope, other kinds accept the developer scope. A call
you lack the scope for is refused at the HTTP layer with a step-up challenge,
not by this tool.

The Gateway validates the manifest and returns the validation error when it is
rejected. Read that error, correct the manifest, and retry. Ask the user for any
value you do not have rather than inventing one.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Deploy or update a routable resource",
			DestructiveHint: ptr(false),
			IdempotentHint:  false,
			OpenWorldHint:   ptr(false),
		},
	}, h.deployAPI)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "undeploy_api",
		Title: "Undeploy a routable resource",
		Description: `Take a routable resource offline and remove it from this Gateway.

Destructive. Confirm the exact kind and id with the user, then call with
confirm=true. Supported kinds: RestApi, Mcp, LlmProxy, LlmProvider.

For supporting configuration (LlmProviderTemplate) use delete_config instead.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Undeploy a routable resource",
			DestructiveHint: ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   ptr(false),
		},
	}, h.undeployAPI)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "apply_config",
		Title: "Create or update a supporting configuration",
		Description: `Create or update a supporting configuration on this Gateway.

Supporting configurations are referenced by routable resources but do not take
traffic themselves. Config kinds: LlmProviderTemplate.

For routable resources (RestApi, Mcp, LlmProxy, LlmProvider) use deploy_api
instead; this tool rejects them.

The kind is read from the manifest's "kind" field. Omit "id" to create; pass it
to update.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create or update a supporting configuration",
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		},
	}, h.applyConfig)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "delete_config",
		Title: "Delete a supporting configuration",
		Description: `Delete a supporting configuration from this Gateway.

Destructive. Confirm the exact kind and id with the user, then call with
confirm=true. Config kinds: LlmProviderTemplate.

Routable resources may still reference the configuration; check with
list_resources before deleting.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete a supporting configuration",
			DestructiveHint: ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   ptr(false),
		},
	}, h.deleteConfig)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_resources",
		Title: "List deployed resources",
		Description: `List resources deployed on this Gateway.

With "kind": returns the full manifest of every resource of that kind.
Without "kind": returns a cross-kind inventory — per-kind counts and a compact
summary (id, displayName, version, state) — limited to the kinds your scope
permits reading. Call again with a kind to get full manifests.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List deployed resources",
			ReadOnlyHint: true,
		},
	}, h.listResources)

	mcp.AddTool(server, &mcp.Tool{
		Name:  "get_resource",
		Title: "Get one resource",
		Description: `Fetch the full manifest of one resource by kind and handle.

Call this before updating anything with deploy_api or apply_config so the
manifest you submit is based on the resource's current state.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get one resource",
			ReadOnlyHint: true,
		},
	}, h.getResource)
}

// Tool handlers
// A returned error becomes a tool execution error (isError: true) rather than a
// JSON-RPC protocol error, per the SDK's typed-handler contract and the MCP
// specification's error-handling rules: validation and business failures are
// what the model is expected to read and correct.
func (h *McpHandler) deployAPI(ctx context.Context, _ *mcp.CallToolRequest, in deployInput) (*mcp.CallToolResult, any, error) {
	return h.write(ctx, classRoutable, "deploy_api", in)
}

func (h *McpHandler) applyConfig(ctx context.Context, _ *mcp.CallToolRequest, in deployInput) (*mcp.CallToolResult, any, error) {
	return h.write(ctx, classConfig, "apply_config", in)
}

// write is the shared create/update path for deploy_api and apply_config. The
// two tools differ only in which class of kind they accept, so the body lives
// here once.
func (h *McpHandler) write(ctx context.Context, class kindClass, tool string, in deployInput) (*mcp.CallToolResult, any, error) {
	if h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; resources are loaded from disk at startup and cannot be changed at runtime")
	}

	env, err := readManifestEnvelope([]byte(in.Yaml))
	if err != nil {
		return nil, nil, err
	}
	ops, err := h.resolveKind(env.Kind, class)
	if err != nil {
		return nil, nil, err
	}

	method := http.MethodPost
	if in.ID != "" {
		method = http.MethodPut
	}
	if err := h.authorize(ctx, routeKey(method, ops, in.ID != "")); err != nil {
		return nil, nil, err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	// A bare manifest is always a create. An explicit id makes it an update and
	// must agree with metadata.name, so a mistyped id cannot overwrite the
	// wrong resource.
	if in.ID != "" {
		if env.Metadata.Name != "" && env.Metadata.Name != in.ID {
			return nil, nil, fmt.Errorf(
				"id %q does not match metadata.name %q in the manifest", in.ID, env.Metadata.Name)
		}
		resource, err := ops.Update(in.ID, []byte(in.Yaml), correlationID, log)
		if err != nil {
			log.Error("MCP update failed", slog.String("id", in.ID), slog.Any("error", err))
			// The validation detail is surfaced deliberately: it is how the
			// model learns to correct the manifest and retry.
			return nil, nil, fmt.Errorf("failed to update %s %q: %w", ops.Kind, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update",
			"kind": ops.Kind, "id": in.ID, "resource": resource,
		}, nil
	}

	resource, err := ops.Create([]byte(in.Yaml), correlationID, log)
	if err != nil {
		log.Error("MCP create failed", slog.Any("error", err))
		return nil, nil, fmt.Errorf("failed to create %s: %w", ops.Kind, err)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create",
		"kind": ops.Kind, "resource": resource,
	}, nil
}

func (h *McpHandler) undeployAPI(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	return h.remove(ctx, classRoutable, "undeploy_api", in)
}

func (h *McpHandler) deleteConfig(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	return h.remove(ctx, classConfig, "delete_config", in)
}

func (h *McpHandler) remove(ctx context.Context, class kindClass, tool string, in deleteInput) (*mcp.CallToolResult, any, error) {
	if h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; resources are loaded from disk at startup and cannot be changed at runtime")
	}
	if !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to delete %s %q: confirm the kind and id with the user, then retry with confirm=true",
			in.Kind, in.ID)
	}

	ops, err := h.resolveKind(in.Kind, class)
	if err != nil {
		return nil, nil, err
	}
	if err := h.authorize(ctx, routeKey(http.MethodDelete, ops, true)); err != nil {
		return nil, nil, err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	if err := ops.Delete(in.ID, correlationID, log); err != nil {
		log.Error("MCP delete failed", slog.String("id", in.ID), slog.Any("error", err))
		// Deliberately generic: a delete failure is usually "not found", and
		// echoing the storage error would confirm what does and does not exist.
		return nil, nil, fmt.Errorf("failed to delete %s %q", ops.Kind, in.ID)
	}

	return nil, map[string]any{
		"status": "success", "kind": ops.Kind, "id": in.ID,
		"message": fmt.Sprintf("%s %q deleted successfully", ops.Kind, in.ID),
	}, nil
}

func (h *McpHandler) getResource(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
	ops, err := h.resolveKind(in.Kind, classAny)
	if err != nil {
		return nil, nil, err
	}
	if err := h.authorize(ctx, routeKey(http.MethodGet, ops, true)); err != nil {
		return nil, nil, err
	}

	resource, err := ops.Get(in.ID)
	if err != nil {
		h.logger.Warn("MCP get_resource lookup failed",
			slog.String("kind", ops.Kind), slog.String("id", in.ID), slog.Any("error", err))
		return nil, nil, fmt.Errorf("%s with handle %q not found", ops.Kind, in.ID)
	}
	return nil, map[string]any{"kind": ops.Kind, "id": in.ID, "resource": resource}, nil
}

// listResources returns full manifests for one kind, or a cross-kind inventory.
// The cross-kind form lists only the kinds the caller may read, which the MCP
// specification explicitly permits ("The set MAY vary by the authorization
// presented on the request"). Silently omitting a kind the caller cannot read
// is better than failing the whole call over one of five kinds.
func (h *McpHandler) listResources(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	if in.Kind != "" {
		ops, err := h.resolveKind(in.Kind, classAny)
		if err != nil {
			return nil, nil, err
		}
		if err := h.authorize(ctx, routeKey(http.MethodGet, ops, false)); err != nil {
			return nil, nil, err
		}
		rows, err := ops.List()
		if err != nil {
			h.logger.Error("MCP list_resources failed",
				slog.String("kind", ops.Kind), slog.Any("error", err))
			return nil, nil, fmt.Errorf("failed to list resources of kind %s", ops.Kind)
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, row.Resource)
		}
		return nil, map[string]any{"kind": ops.Kind, "count": len(items), "items": items}, nil
	}

	breakdown := make(map[string]any, len(h.kinds))
	total, readable := 0, 0
	for _, kind := range canonicalKinds() {
		ops, ok := h.kinds[kind]
		if !ok {
			continue
		}
		if err := h.authorize(ctx, routeKey(http.MethodGet, ops, false)); err != nil {
			continue // not readable by this caller — omit rather than fail
		}
		readable++
		rows, err := ops.List()
		if err != nil {
			h.logger.Error("MCP list_resources failed for kind",
				slog.String("kind", kind), slog.Any("error", err))
			breakdown[kind] = map[string]any{"error": "failed to list this kind"}
			continue
		}
		total += len(rows)
		breakdown[kind] = map[string]any{
			"count":     len(rows),
			"resources": rows, // Resource is json:"-" — summaries only here
		}
	}
	if readable == 0 {
		return nil, nil, fmt.Errorf("your credentials do not permit reading any resource kind on this Gateway")
	}

	return nil, map[string]any{
		"total": total,
		"kinds": breakdown,
		"hint":  "Call list_resources again with a kind, or get_resource with a kind and id, for full manifests.",
	}, nil
}
