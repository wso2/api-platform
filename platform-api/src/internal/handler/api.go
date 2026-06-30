/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	"strings"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/wso2/go-httpkit/httputil"
)

type APIHandler struct {
	apiService *service.APIService
	slogger    *slog.Logger
}

func NewAPIHandler(apiService *service.APIService, slogger *slog.Logger) *APIHandler {
	return &APIHandler{
		apiService: apiService,
		slogger:    slogger,
	}
}

// CreateAPI handles POST /api/v0.9/rest-apis and creates a new API
func (h *APIHandler) CreateAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.CreateRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}

	// Validate required fields
	if req.DisplayName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if req.Context == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if req.Version == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if req.ProjectId == (openapi_types.UUID{}) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		h.slogger.Error("Validation failed: No upstream endpoints provided", "organizationId", orgId)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one upstream endpoint (main or sandbox) is required"))
		return
	}

	createdBy, _ := middleware.GetUserIDFromRequest(r)
	apiResponse, err := h.apiService.CreateAPI(&req, orgId, createdBy)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			h.slogger.Error("API handle already exists", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API handle already exists"))
			return
		}
		if errors.Is(err, constants.ErrAPINameVersionAlreadyExists) {
			h.slogger.Error("API with same name and version already exists", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API with same name and version already exists in the organization"))
			return
		}
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			h.slogger.Error("API already exists in the project", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			h.slogger.Error("Project not found", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			h.slogger.Error("Invalid API name format", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			h.slogger.Error("Invalid API context format", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API context format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIVersion) {
			h.slogger.Error("Invalid API version format", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API version format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			h.slogger.Error("Invalid lifecycle status", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			h.slogger.Error("Invalid API type", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			h.slogger.Error("Invalid transport protocol", "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid transport protocol"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive) {
			h.slogger.Error("Subscription plan not found or not active", "organizationId", orgId, "error", err)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				err.Error()))
			return
		}
		h.slogger.Error("Failed to create API", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
}

// GetAPI handles GET /api/v0.9/rest-apis/:apiId and retrieves an API by its handle
func (h *APIHandler) GetAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	apiResponse, err := h.apiService.GetAPIByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to get API", "apiId", apiId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
}

// ListAPIs handles GET /api/v0.9/rest-apis and lists APIs for an organization filtered by project
func (h *APIHandler) ListAPIs(w http.ResponseWriter, r *http.Request) {
	// Get organization from JWT token
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	projectId := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "projectId query parameter is required"))
		return
	}
	if _, err := uuid.Parse(projectId); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "invalid projectId"))
		return
	}

	apis, err := h.apiService.GetAPIsByOrganization(orgId, projectId)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			h.slogger.Error("Project not found", "organizationId", orgId, "projectId", projectId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		h.slogger.Error("Failed to get APIs", "organizationId", orgId, "projectId", projectId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get APIs"))
		return
	}

	response := api.RESTAPIListResponse{
		Count: len(apis),
		List:  apis,
		Pagination: api.Pagination{
			Total:  len(apis),
			Offset: 0,
			Limit:  len(apis),
		},
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// UpdateAPI updates an existing API identified by handle
func (h *APIHandler) UpdateAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var req api.UpdateRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			err.Error()))
		return
	}

	// Validate upstream configuration if provided
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one upstream endpoint (main or sandbox) is required"))
		return
	}

	updatedBy, _ := middleware.GetUserIDFromRequest(r)
	apiResponse, err := h.apiService.UpdateAPIByHandle(apiId, &req, orgId, updatedBy)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			h.slogger.Error("Invalid lifecycle status", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			h.slogger.Error("Invalid API type", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			h.slogger.Error("Invalid transport protocol", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid transport protocol"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive) {
			h.slogger.Error("Subscription plan not found or not active", "apiId", apiId, "organizationId", orgId, "error", err)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				err.Error()))
			return
		}
		h.slogger.Error("Failed to update API", "apiId", apiId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update API"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
}

// DeleteAPI handles DELETE /api/v0.9/rest-apis/:apiId and deletes an API by its handle
func (h *APIHandler) DeleteAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	deletedBy, _ := middleware.GetUserIDFromRequest(r)
	err := h.apiService.DeleteAPIByHandle(apiId, orgId, deletedBy)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to delete API", "apiId", apiId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete API"))
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

// AddGatewaysToAPI handles POST /api/v0.9/rest-apis/:apiId/gateways to associate gateways with an API
func (h *APIHandler) AddGatewaysToAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var req []api.AddGatewayToRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if len(req) == 0 {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one gateway ID is required"))
		return
	}

	// Extract gateway IDs from request
	gatewayIds := make([]string, len(req))
	for i, gw := range req {
		gatewayIds[i] = utils.OpenAPIUUIDToString(gw.GatewayId)
	}

	gatewaysResponse, err := h.apiService.AddGatewaysToAPIByHandle(apiId, gatewayIds, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			h.slogger.Error("One or more gateways not found", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"One or more gateways not found"))
			return
		}
		h.slogger.Error("Failed to associate gateways with API", "apiId", apiId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to associate gateways with API"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
}

// GetAPIGateways handles GET /api/v0.9/rest-apis/:apiId/gateways to get gateways associated with an API including deployment details
func (h *APIHandler) GetAPIGateways(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	gatewaysResponse, err := h.apiService.GetAPIGatewaysByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to get API gateways", "apiId", apiId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API gateways"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering REST API routes")
	base := constants.APIBasePath + "/rest-apis"
	mux.HandleFunc("POST "+base, h.CreateAPI)
	mux.HandleFunc("GET "+base, h.ListAPIs)
	mux.HandleFunc("GET "+base+"/{apiId}", h.GetAPI)
	mux.HandleFunc("PUT "+base+"/{apiId}", h.UpdateAPI)
	mux.HandleFunc("DELETE "+base+"/{apiId}", h.DeleteAPI)
	mux.HandleFunc("GET "+base+"/{apiId}/gateways", h.GetAPIGateways)
	mux.HandleFunc("POST "+base+"/{apiId}/gateways", h.AddGatewaysToAPI)
}

func isEmptyUpstreamDefinition(definition api.UpstreamDefinition) bool {
	if definition.Url != nil && *definition.Url != "" {
		return false
	}
	if definition.Ref != nil && *definition.Ref != "" {
		return false
	}
	return true
}
