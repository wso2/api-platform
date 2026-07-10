/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	egservice "github.com/wso2/api-platform/platform-api/plugins/eventgateway/service"

	"github.com/wso2/go-httpkit/httputil"
)

// WebSubAPIHandler handles CRUD and auxiliary routes for WebSub APIs
type WebSubAPIHandler struct {
	websubAPIService *egservice.WebSubAPIService
	identity         *service.IdentityService
	slogger          *slog.Logger
}

// NewWebSubAPIHandler creates a new WebSubAPIHandler instance
func NewWebSubAPIHandler(websubAPIService *egservice.WebSubAPIService, identity *service.IdentityService, slogger *slog.Logger) *WebSubAPIHandler {
	return &WebSubAPIHandler{
		websubAPIService: websubAPIService,
		identity:         identity,
		slogger:          slogger,
	}
}

// RegisterRoutes registers WebSub API routes
func (h *WebSubAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/websub-apis", h.CreateWebSubAPI)
	mux.HandleFunc("GET "+constants.APIBasePath+"/websub-apis", h.ListWebSubAPIs)
	mux.HandleFunc("GET "+constants.APIBasePath+"/websub-apis/{webSubApiId}", h.GetWebSubAPI)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/websub-apis/{webSubApiId}", h.UpdateWebSubAPI)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/websub-apis/{webSubApiId}", h.DeleteWebSubAPI)
}

// CreateWebSubAPI handles POST /api/v0.9/websub-apis
func (h *WebSubAPIHandler) CreateWebSubAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.WebSubAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("WebSub API request validation failed", "org_id", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "create WebSub API")
	if !ok {
		return
	}

	resp, err := h.websubAPIService.Create(orgID, createdBy, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

// ListWebSubAPIs handles GET /api/v0.9/websub-apis
func (h *WebSubAPIHandler) ListWebSubAPIs(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "projectId query parameter is required"))
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.websubAPIService.List(orgID, projectID, limit, offset)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// GetWebSubAPI handles GET /api/v0.9/websub-apis/:apiId
func (h *WebSubAPIHandler) GetWebSubAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := r.PathValue("webSubApiId")
	resp, err := h.websubAPIService.Get(orgID, id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// UpdateWebSubAPI handles PUT /api/v0.9/websub-apis/:apiId
func (h *WebSubAPIHandler) UpdateWebSubAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := r.PathValue("webSubApiId")

	var req api.WebSubAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("WebSub API update validation failed", "org_id", orgID, "api_id", id, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if err := utils.ValidateHandleImmutable(id, req.Id); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request",
			"WebSub API id is immutable and cannot be changed"))
		return
	}

	updatedBy, ok := resolveActor(w, r, h.identity, h.slogger, "update WebSub API")
	if !ok {
		return
	}
	resp, err := h.websubAPIService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// DeleteWebSubAPI handles DELETE /api/v0.9/websub-apis/:apiId
func (h *WebSubAPIHandler) DeleteWebSubAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := r.PathValue("webSubApiId")
	deletedBy, ok := resolveActor(w, r, h.identity, h.slogger, "delete WebSub API")
	if !ok {
		return
	}

	if err := h.websubAPIService.Delete(orgID, id, deletedBy); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleServiceError maps service errors to HTTP responses
func (h *WebSubAPIHandler) handleServiceError(w http.ResponseWriter, err error) {
	if respondArtifactGuardError(w, err) {
		return
	}
	switch {
	case errors.Is(err, constants.ErrHandleImmutable):
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrInvalidInput):
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		httputil.WriteJSON(w, http.StatusNotFound, apperror.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	case errors.Is(err, constants.ErrWebSubAPIExists):
		httputil.WriteJSON(w, http.StatusConflict, apperror.NewErrorResponse(409, "Conflict", "WebSub API with this ID already exists"))
	case errors.Is(err, constants.ErrWebSubAPILimitReached):
		httputil.WriteJSON(w, http.StatusConflict, apperror.NewErrorResponse(409, "Conflict", "WebSub API limit reached for the organization"))
	case errors.Is(err, constants.ErrProjectNotFound):
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "Project not found"))
	case errors.Is(err, constants.ErrDevPortalNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, apperror.NewErrorResponse(404, "Not Found", "DevPortal not found"))
	default:
		h.slogger.Error("WebSub API service error", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, apperror.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
