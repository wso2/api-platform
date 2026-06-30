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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/utils"
	egservice "platform-api/src/plugins/eventgateway/service"

	"github.com/wso2/go-httpkit/httputil"
)

// WebBrokerAPIHandler handles CRUD and auxiliary routes for WebBroker APIs
type WebBrokerAPIHandler struct {
	webbrokerAPIService *egservice.WebBrokerAPIService
	slogger             *slog.Logger
}

// NewWebBrokerAPIHandler creates a new WebBrokerAPIHandler instance
func NewWebBrokerAPIHandler(webbrokerAPIService *egservice.WebBrokerAPIService, slogger *slog.Logger) *WebBrokerAPIHandler {
	return &WebBrokerAPIHandler{
		webbrokerAPIService: webbrokerAPIService,
		slogger:             slogger,
	}
}

// RegisterRoutes registers WebBroker API routes
func (h *WebBrokerAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/webbroker-apis", h.CreateWebBrokerAPI)
	mux.HandleFunc("GET "+constants.APIBasePath+"/webbroker-apis", h.ListWebBrokerAPIs)
	mux.HandleFunc("GET "+constants.APIBasePath+"/webbroker-apis/{id}", h.GetWebBrokerAPI)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/webbroker-apis/{id}", h.UpdateWebBrokerAPI)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/webbroker-apis/{id}", h.DeleteWebBrokerAPI)
}

// CreateWebBrokerAPI handles POST /api/v0.9/webbroker-apis
func (h *WebBrokerAPIHandler) CreateWebBrokerAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.WebBrokerAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("WebBroker API request validation failed", "org_id", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, _ := middleware.GetUserIDFromRequest(r)

	resp, err := h.webbrokerAPIService.Create(orgID, createdBy, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

// ListWebBrokerAPIs handles GET /api/v0.9/webbroker-apis
func (h *WebBrokerAPIHandler) ListWebBrokerAPIs(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "projectId query parameter is required"))
		return
	}

	limitStr := "20"
	if v := r.URL.Query().Get("limit"); v != "" {
		limitStr = v
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offsetStr := "0"
	if v := r.URL.Query().Get("offset"); v != "" {
		offsetStr = v
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.webbrokerAPIService.List(orgID, projectID, limit, offset)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// GetWebBrokerAPI handles GET /api/v0.9/webbroker-apis/:apiId
func (h *WebBrokerAPIHandler) GetWebBrokerAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := r.PathValue("id")
	resp, err := h.webbrokerAPIService.Get(orgID, id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// UpdateWebBrokerAPI handles PUT /api/v0.9/webbroker-apis/:apiId
func (h *WebBrokerAPIHandler) UpdateWebBrokerAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := r.PathValue("id")

	var req api.WebBrokerAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("WebBroker API update validation failed", "org_id", orgID, "api_id", id, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	updatedBy, _ := middleware.GetUserIDFromRequest(r)
	resp, err := h.webbrokerAPIService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// DeleteWebBrokerAPI handles DELETE /api/v0.9/webbroker-apis/:apiId
func (h *WebBrokerAPIHandler) DeleteWebBrokerAPI(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := r.PathValue("id")
	deletedBy, _ := middleware.GetUserIDFromRequest(r)

	if err := h.webbrokerAPIService.Delete(orgID, id, deletedBy); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleServiceError maps service errors to HTTP responses
func (h *WebBrokerAPIHandler) handleServiceError(w http.ResponseWriter, err error) {
	if respondArtifactGuardError(w, err) {
		return
	}
	switch {
	case errors.Is(err, constants.ErrHandleImmutable):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrInvalidInput):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrWebBrokerAPINotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
	case errors.Is(err, constants.ErrWebBrokerAPIExists):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "WebBroker API with this ID already exists"))
	case errors.Is(err, constants.ErrWebBrokerAPILimitReached):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "WebBroker API limit reached for the organization"))
	case errors.Is(err, constants.ErrProjectNotFound):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project not found"))
	case errors.Is(err, constants.ErrDevPortalNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "DevPortal not found"))
	default:
		h.slogger.Error("WebBroker API service error", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
