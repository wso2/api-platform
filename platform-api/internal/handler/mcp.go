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
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"

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
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies", middleware.MapErrors(h.slogger, h.CreateMCPProxy))
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies", middleware.MapErrors(h.slogger, h.ListMCPProxies))
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/fetch-server-info", middleware.MapErrors(h.slogger, h.FetchMCPProxyServerInfo))
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}", middleware.MapErrors(h.slogger, h.GetMCPProxy))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}", middleware.MapErrors(h.slogger, h.UpdateMCPProxy))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}", middleware.MapErrors(h.slogger, h.DeleteMCPProxy))
}

// CreateMCPProxy handles POST /api/v0.9/mcp-proxies
func (h *MCPProxyHandler) CreateMCPProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.MCPProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid MCP proxy creation request body for org %s", orgID))
	}

	createdBy, err := resolveActorErr(r, h.identity, "create MCP proxy")
	if err != nil {
		return err
	}

	if req.ProjectId == nil {
		h.slogger.Debug("No project ID provided for MCP proxy, proceeding without project association", "mcpProxyId", req.Id)
	}

	resp, err := h.service.Create(orgID, createdBy, &req)
	if err != nil {
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// ListMCPProxies handles GET /api/v0.9/mcp-proxies
func (h *MCPProxyHandler) ListMCPProxies(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		return apperror.ValidationFailed.New("projectId query parameter is required")
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
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// GetMCPProxy handles GET /api/v0.9/mcp-proxies/:id
func (h *MCPProxyHandler) GetMCPProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("mcpProxyId")

	resp, err := h.service.Get(orgID, id)
	if err != nil {
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateMCPProxy handles PUT /api/v0.9/mcp-proxies/:id
func (h *MCPProxyHandler) UpdateMCPProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("mcpProxyId")

	var req api.MCPProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid MCP proxy update request body for proxy %s in org %s", id, orgID))
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update MCP proxy")
	if err != nil {
		return err
	}
	resp, err := h.service.Update(orgID, id, updatedBy, &req)
	if err != nil {
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// DeleteMCPProxy handles DELETE /api/v0.9/mcp-proxies/:id
func (h *MCPProxyHandler) DeleteMCPProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("mcpProxyId")
	deletedBy, err := resolveActorErr(r, h.identity, "delete MCP proxy")
	if err != nil {
		return err
	}

	if err := h.service.Delete(orgID, id, deletedBy); err != nil {
		return h.mapServiceError(err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// FetchMCPProxyServerInfo handles POST /api/v0.9/mcp-proxies/fetch-server-info
func (h *MCPProxyHandler) FetchMCPProxyServerInfo(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.MCPServerInfoFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid MCP server info fetch request body for org %s", orgID))
	}

	resp, err := h.service.FetchServerInfo(orgID, &req)
	if err != nil {
		reqURL := ""
		if req.Url != nil {
			reqURL = *req.Url
		}
		switch {
		case errors.Is(err, constants.ErrInvalidURL):
			return apperror.ValidationFailed.Wrap(err, "Invalid URL provided").
				WithLogMessage(fmt.Sprintf("invalid URL provided for MCP server info fetch: %s", reqURL))
		case errors.Is(err, constants.ErrURLUnreachable):
			return apperror.ValidationFailed.Wrap(err, "URL is unreachable").
				WithLogMessage(fmt.Sprintf("MCP server URL is unreachable: %s", reqURL))
		case errors.Is(err, constants.ErrMCPServerUnauthorized):
			return apperror.ValidationFailed.Wrap(err, "MCP server returned 401 Unauthorized. Check the provided credentials.").
				WithLogMessage(fmt.Sprintf("MCP server returned 401 Unauthorized: %s", reqURL))
		default:
			return h.mapServiceError(err)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// mapServiceError maps service errors to *apperror.Error values for the
// centralized error mapper, preserving the exact status/code/message each
// error produced before the migration (including the read-only /
// deletion-guard mapping previously handled by respondArtifactGuardError).
func (h *MCPProxyHandler) mapServiceError(err error) error {
	switch {
	case errors.Is(err, constants.ErrArtifactReadOnly):
		return apperror.ArtifactReadOnly.Wrap(err, "Artifact is read-only: it originated from a data-plane gateway")
	case errors.Is(err, constants.ErrArtifactRuntimeImmutable):
		return apperror.ArtifactRuntimeImmutable.Wrap(err, "Runtime configuration of this artifact cannot be changed")
	case errors.Is(err, constants.ErrArtifactDeployed):
		return apperror.ArtifactDeployed.Wrap(err, "Artifact is still deployed on a gateway and cannot be deleted")
	case errors.Is(err, constants.ErrHandleImmutable):
		return apperror.ValidationFailed.Wrap(err, "The id is immutable and cannot be changed")
	case errors.Is(err, constants.ErrInvalidInput):
		return apperror.ValidationFailed.Wrap(err, "Invalid input parameters")
	case errors.Is(err, constants.ErrMCPProxyNotFound):
		return apperror.MCPProxyNotFound.Wrap(err)
	case errors.Is(err, constants.ErrMCPProxyExists):
		return apperror.MCPProxyExists.Wrap(err)
	case errors.Is(err, constants.ErrProjectNotFound):
		return apperror.ProjectRefNotFound.Wrap(err)
	case errors.Is(err, constants.ErrMCPProxyLimitReached):
		return apperror.MCPProxyLimitReached.Wrap(err)
	case errors.Is(err, constants.ErrSecretRefMissing):
		return apperror.ValidationFailed.Wrap(err, "One or more referenced secrets do not exist")
	default:
		return apperror.Internal.Wrap(err)
	}
}
