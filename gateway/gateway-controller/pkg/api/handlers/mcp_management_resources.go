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
)

// Tool handlers

// A returned error becomes a tool execution error (isError: true) rather than a
// JSON-RPC protocol error, per the SDK's typed-handler contract and the MCP
// specification's error-handling rules: validation and business failures are
// what the model is expected to read and correct.
func (h *McpHandler) deployAPI(ctx context.Context, _ *mcp.CallToolRequest, in deployInput) (*mcp.CallToolResult, any, error) {
	return h.write(ctx, classRoutable, toolDeployAPI, in)
}

func (h *McpHandler) applyConfig(ctx context.Context, _ *mcp.CallToolRequest, in deployInput) (*mcp.CallToolResult, any, error) {
	return h.write(ctx, classConfig, toolApplyConfig, in)
}

// write is the shared create/update path for wso2_apip_gw_deploy_api and wso2_apip_gw_apply_config. The
// two tools differ only in which class of kind they accept, so the body lives
// here once.
func (h *McpHandler) write(ctx context.Context, class kindClass, tool string, in deployInput) (*mcp.CallToolResult, any, error) {
	if h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; resources are loaded from disk at startup and cannot be changed at runtime")
	}

	ops, err := h.resolveKind(in.Kind, class)
	if err != nil {
		return nil, nil, err
	}

	method := http.MethodPost
	if in.ID != "" {
		method = http.MethodPut
	}
	if err := h.authz.authorize(ctx, routeKey(method, ops, in.ID != "")); err != nil {
		return nil, nil, err
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", tool),
		slog.String("kind", ops.Kind))

	if in.ID != "" {
		resource, err := ops.Update(in.ID, []byte(in.Yaml), correlationID, log)
		if err != nil {
			log.Error("MCP update failed", slog.String("id", in.ID), slog.Any("error", err))
			// Validation details are how the model learns to correct the manifest and retry,
			// So its Mapped exactly as the kind's REST handler maps it
			// while internal failures stay generic to prevent any data leaks
			return nil, nil, mcpWriteError(ops.Kind, mcpWriteOpUpdate, in.ID, err, log)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update",
			"kind": ops.Kind, "id": in.ID, "resource": resource,
		}, nil
	}

	resource, err := ops.Create([]byte(in.Yaml), correlationID, log)
	if err != nil {
		log.Error("MCP create failed", slog.Any("error", err))
		return nil, nil, mcpWriteError(ops.Kind, mcpWriteOpCreate, "", err, log)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create",
		"kind": ops.Kind, "resource": resource,
	}, nil
}

func (h *McpHandler) undeployAPI(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	return h.remove(ctx, classRoutable, toolUndeployAPI, in)
}

func (h *McpHandler) deleteConfig(ctx context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	return h.remove(ctx, classConfig, toolDeleteConfig, in)
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
	if err := h.authz.authorize(ctx, routeKey(http.MethodDelete, ops, true)); err != nil {
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
	if err := h.authz.authorize(ctx, routeKey(http.MethodGet, ops, true)); err != nil {
		return nil, nil, err
	}

	resource, err := ops.Get(in.ID)
	if err != nil {
		h.logger.Warn("MCP "+toolGetResource+" lookup failed",
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
		if err := h.authz.authorize(ctx, routeKey(http.MethodGet, ops, false)); err != nil {
			return nil, nil, err
		}
		rows, err := ops.List()
		if err != nil {
			h.logger.Error("MCP "+toolListResources+" failed",
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
		if err := h.authz.authorize(ctx, routeKey(http.MethodGet, ops, false)); err != nil {
			continue // not readable by this caller — omit rather than fail
		}
		readable++
		rows, err := ops.List()
		if err != nil {
			h.logger.Error("MCP "+toolListResources+" failed for kind",
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
		"hint": "Call " + toolListResources + " again with a kind, or " + toolGetResource + " with a kind and id, " +
			"for full manifests. Certificates and subscriptions are not kinds and do not appear here - " +
			"use " + toolManageCertificates + " and " + toolManageSubscriptions + " for those respectively.",
	}, nil
}
