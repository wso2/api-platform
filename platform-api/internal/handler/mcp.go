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

package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type MCPProxyHandler struct {
	service  *service.MCPProxyService
	identity *service.IdentityService
	slogger  *slog.Logger
}

func NewMCPProxyHandler(service *service.MCPProxyService, identity *service.IdentityService, slogger *slog.Logger) *MCPProxyHandler {
	return &MCPProxyHandler{
		service:  service,
		identity: identity,
		slogger:  slogger,
	}
}

func (h *MCPProxyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies", h.CreateMCPProxy)
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies", h.ListMCPProxies)
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/fetch-server-info", h.FetchMCPProxyServerInfo)
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}", h.GetMCPProxy)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}", h.UpdateMCPProxy)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}", h.DeleteMCPProxy)
}

// CreateMCPProxy handles POST /api/v0.9/mcp-proxies
func (h *MCPProxyHandler) CreateMCPProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.MCPProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("MCP request validation failed", "org_id", orgID, "reason", "Invalid request body", "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "create MCP proxy")
	if !ok {
		return
	}

	if req.ProjectId == nil {
		h.slogger.Debug("No project ID provided for MCP proxy, proceeding without project association", "mcpProxyId", req.Id)
	}

	resp, err := h.service.Create(orgID, createdBy, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

// ListMCPProxies handles GET /api/v0.9/mcp-proxies
func (h *MCPProxyHandler) ListMCPProxies(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "projectId query parameter is required"))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "20"
	}
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		offsetStr = "0"
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		h.slogger.Warn("Invalid limit query parameter, defaulting to 20", "input", limitStr)
		limit = 20
	}
	if limit > 100 {
		h.slogger.Warn("Limit query parameter exceeds maximum, capping to 100", "input", limit)
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.service.ListByProject(orgID, projectID, limit, offset)

	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// GetMCPProxy handles GET /api/v0.9/mcp-proxies/:id
func (h *MCPProxyHandler) GetMCPProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := r.PathValue("mcpProxyId")

	resp, err := h.service.Get(orgID, id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// UpdateMCPProxy handles PUT /api/v0.9/mcp-proxies/:id
func (h *MCPProxyHandler) UpdateMCPProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := r.PathValue("mcpProxyId")

	var req api.MCPProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("MCP request validation failed", "org_id", orgID, "proxy_id", id, "reason", "Invalid request body", "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	updatedBy, ok := resolveActor(w, r, h.identity, h.slogger, "update MCP proxy")
	if !ok {
		return
	}
	resp, err := h.service.Update(orgID, id, updatedBy, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// DeleteMCPProxy handles DELETE /api/v0.9/mcp-proxies/:id
func (h *MCPProxyHandler) DeleteMCPProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := r.PathValue("mcpProxyId")
	deletedBy, ok := resolveActor(w, r, h.identity, h.slogger, "delete MCP proxy")
	if !ok {
		return
	}

	if err := h.service.Delete(orgID, id, deletedBy); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// FetchMCPProxyServerInfo handles POST /api/v0.9/mcp-proxies/fetch-server-info
func (h *MCPProxyHandler) FetchMCPProxyServerInfo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.MCPServerInfoFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("MCP request validation failed", "org_id", orgID, "reason", "Invalid request body", "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.service.FetchServerInfo(orgID, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrInvalidURL):
			h.slogger.Error("Invalid URL provided for MCP server info fetch", "error", err, "inputUrl", req.Url)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
			return
		case errors.Is(err, constants.ErrURLUnreachable):
			h.slogger.Error("MCP server URL is unreachable", "error", err, "inputUrl", req.Url)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", strings.Split(err.Error(), ":")[0]))
			return
		case errors.Is(err, constants.ErrMCPServerUnauthorized):
			h.slogger.Error("MCP server returned 401 Unauthorized", "error", err, "inputUrl", req.Url)
			httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(400, "Bad Request", "MCP server returned 401 Unauthorized. Check the provided credentials."))
			return
		default:
			h.handleServiceError(w, err)
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleServiceError maps service errors to HTTP responses
func (h *MCPProxyHandler) handleServiceError(w http.ResponseWriter, err error) {
	if respondArtifactGuardError(w, err) {
		return
	}
	switch {
	case errors.Is(err, constants.ErrHandleImmutable):
		h.slogger.Error("MCP handle immutability violation", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrInvalidInput):
		h.slogger.Error("MCP request validation failed", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrMCPProxyNotFound):
		h.slogger.Error("MCP proxy not found", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "MCP proxy not found"))
	case errors.Is(err, constants.ErrMCPProxyExists):
		h.slogger.Error("MCP proxy conflict", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "MCP proxy with this ID already exists"))
	case errors.Is(err, constants.ErrProjectNotFound):
		h.slogger.Error("MCP request validation failed", "reason", "Project not found")
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project not found"))
	case errors.Is(err, constants.ErrMCPProxyLimitReached):
		h.slogger.Error("MCP proxy limit reached", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "MCP proxy limit reached for the organization"))
	case errors.Is(err, constants.ErrSecretRefMissing):
		h.slogger.Error("MCP proxy secret ref missing", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	default:
		h.slogger.Error("MCP proxy service error", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
