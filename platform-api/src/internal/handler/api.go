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
	"mime/multipart"
	"net/http"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
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
	if req.Name == "" {
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

	createdBy, _ := middleware.GetUsernameFromRequest(r)
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

	apiId := r.PathValue("apiHandle")
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

	apiId := r.PathValue("apiHandle")
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

	updatedBy, _ := middleware.GetUsernameFromRequest(r)
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

	apiId := r.PathValue("apiHandle")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	deletedBy, _ := middleware.GetUsernameFromRequest(r)
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

	apiId := r.PathValue("apiHandle")
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

	// Extract gateway handles from request
	gatewayHandles := make([]string, len(req))
	for i, gw := range req {
		gatewayHandles[i] = gw.GatewayHandle
	}

	gatewaysResponse, err := h.apiService.AddGatewaysToAPIByHandle(apiId, gatewayHandles, orgId)
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

	apiId := r.PathValue("apiHandle")
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

// ImportAPIProject handles POST /api/v0.9/api-projects/import
func (h *APIHandler) ImportAPIProject(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.ImportAPIProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.RepoUrl == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Repository URL is required"))
		return
	}
	if req.Branch == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Branch is required"))
		return
	}
	if req.Path == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Path is required"))
		return
	}
	if req.Api.Name == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if req.Api.Context == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if req.Api.Version == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if req.Api.ProjectId == (openapi_types.UUID{}) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	// Create Git service
	gitService := service.NewGitService()

	createdBy, _ := middleware.GetUsernameFromRequest(r)
	// Import API project
	apiResponse, err := h.apiService.ImportAPIProject(&req, orgId, createdBy, gitService)
	if err != nil {
		if errors.Is(err, constants.ErrAPIProjectNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "API Project Not Found",
				"API project not found: .api-platform directory not found"))
			return
		}
		if errors.Is(err, constants.ErrMalformedAPIProject) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Malformed API Project",
				"Malformed API project: config.yaml is missing or invalid"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIProject) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Invalid API Project",
				"Invalid API project: referenced files not found"))
			return
		}
		if errors.Is(err, constants.ErrConfigFileNotFound) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Config File Not Found",
				"config.yaml file not found in .api-platform directory"))
			return
		}
		if errors.Is(err, constants.ErrOpenAPIFileNotFound) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "OpenAPI File Not Found",
				"OpenAPI file not found"))
			return
		}
		if errors.Is(err, constants.ErrWSO2ArtifactNotFound) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "WSO2 Artifact Not Found",
				"WSO2 artifact file not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API context format"))
			return
		}

		h.slogger.Error("Failed to import API project", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to import API project"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
}

// ValidateAPIProject handles POST /validate/api-project
func (h *APIHandler) ValidateAPIProject(w http.ResponseWriter, r *http.Request) {
	var req api.ValidateAPIProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.RepoUrl == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Repository URL is required"))
		return
	}
	if req.Branch == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Branch is required"))
		return
	}
	if req.Path == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Path is required"))
		return
	}

	// Create Git service
	gitService := service.NewGitService()

	// Validate API project
	response, err := h.apiService.ValidateAndRetrieveAPIProject(&req, gitService)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to validate API project"))
		return
	}

	// Return validation response (200 OK even if validation fails - errors are in the response body)
	httputil.WriteJSON(w, http.StatusOK, response)
}

// ValidateOpenAPI handles POST /validate/open-api
func (h *APIHandler) ValidateOpenAPI(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Failed to parse multipart form"))
		return
	}

	var req api.ValidateOpenAPIRequest
	var definitionHeader *multipart.FileHeader

	// Get URL from form if provided
	if url := r.FormValue("url"); url != "" {
		req.Url = &url
	}

	// Get definition file from form if provided
	if file, header, err := r.FormFile("definition"); err == nil {
		definitionHeader = header
		var openapiFile openapi_types.File
		openapiFile.InitFromMultipart(header)
		req.Definition = &openapiFile
		defer file.Close()
	}

	// Validate that at least one input is provided
	if req.Url == nil && req.Definition == nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either URL or definition file must be provided"))
		return
	}

	// Validate OpenAPI definition
	response, err := h.apiService.ValidateOpenAPIDefinition(req.Url, definitionHeader)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to validate OpenAPI definition"))
		return
	}

	// Return validation response (200 OK even if validation fails - errors are in the response body)
	httputil.WriteJSON(w, http.StatusOK, response)
}

// ImportOpenAPI handles POST /import/open-api and imports an API from OpenAPI definition
func (h *APIHandler) ImportOpenAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Failed to parse multipart form"))
		return
	}

	var req api.ImportOpenAPIRequest
	var definitionHeader *multipart.FileHeader

	// Get URL from form if provided
	if url := r.FormValue("url"); url != "" {
		req.Url = &url
	}

	// Get definition file from form if provided
	if file, header, err := r.FormFile("definition"); err == nil {
		definitionHeader = header
		var openapiFile openapi_types.File
		openapiFile.InitFromMultipart(header)
		req.Definition = &openapiFile
		defer file.Close()
	}

	// Validate that at least one input is provided
	if req.Url == nil && req.Definition == nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either URL or definition file must be provided"))
		return
	}

	// Get API details from form data (JSON string in 'api' field)
	apiJSON := r.FormValue("api")
	if apiJSON == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API details are required"))
		return
	}
	req.Api = apiJSON

	// Parse API details from JSON string
	var apiDetails api.RESTAPI
	if err := json.Unmarshal([]byte(apiJSON), &apiDetails); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid API details: "+err.Error()))
		return
	}

	if apiDetails.Name == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if apiDetails.Context == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if apiDetails.Version == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if apiDetails.ProjectId == (openapi_types.UUID{}) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	createdBy, _ := middleware.GetUsernameFromRequest(r)
	// Import API from OpenAPI definition
	apiResponse, err := h.apiService.ImportFromOpenAPI(&apiDetails, req.Url, definitionHeader, orgId, createdBy)
	if err != nil {
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API context format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIVersion) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API version format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid transport protocol"))
			return
		}
		// Handle OpenAPI-specific errors
		if strings.Contains(err.Error(), "failed to fetch OpenAPI from URL") {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to fetch OpenAPI definition from URL"))
			return
		}
		if strings.Contains(err.Error(), "failed to open OpenAPI definition file") ||
			strings.Contains(err.Error(), "failed to read OpenAPI definition file") {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to fetch OpenAPI definition from file"))
			return
		}
		if strings.Contains(err.Error(), "failed to validate and parse OpenAPI definition") {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid OpenAPI definition"))
			return
		}
		if strings.Contains(err.Error(), "failed to merge API details") {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to create API from OpenAPI definition: incompatible details"))
			return
		}

		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to import API from OpenAPI definition"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
}

// ValidateAPI handles GET /api/v0.9/rest-apis?name=&version=
func (h *APIHandler) ValidateAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var params dto.ValidateRESTAPIParams
	if v := r.URL.Query().Get("identifier"); v != "" {
		s := v
		params.Identifier = &s
	}
	if v := r.URL.Query().Get("name"); v != "" {
		s := v
		params.Name = &s
	}
	if v := r.URL.Query().Get("version"); v != "" {
		s := v
		params.Version = &s
	}

	identifier := ""
	name := ""
	version := ""
	if params.Identifier != nil {
		identifier = *params.Identifier
	}
	if params.Name != nil {
		name = *params.Name
	}
	if params.Version != nil {
		version = *params.Version
	}
	if identifier == "" && (name == "" || version == "") {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either 'identifier' or both 'name' and 'version' query parameters are required"))
		return
	}

	response, err := h.apiService.ValidateAPI(&params, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to validate API"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering REST API routes")
	base := constants.APIBasePath + "/rest-apis"
	mux.HandleFunc("POST "+base, h.CreateAPI)
	mux.HandleFunc("GET "+base, h.ListAPIs)
	mux.HandleFunc("GET "+base+"/{apiHandle}", h.GetAPI)
	mux.HandleFunc("PUT "+base+"/{apiHandle}", h.UpdateAPI)
	mux.HandleFunc("DELETE "+base+"/{apiHandle}", h.DeleteAPI)
	mux.HandleFunc("POST "+base+"/import-openapi", h.ImportOpenAPI)
	mux.HandleFunc("POST "+base+"/validate-openapi", h.ValidateOpenAPI)
	mux.HandleFunc("GET "+base+"/{apiHandle}/gateways", h.GetAPIGateways)
	mux.HandleFunc("POST "+base+"/{apiHandle}/gateways", h.AddGatewaysToAPI)
	apiProjectsBase := constants.APIBasePath + "/api-projects"
	mux.HandleFunc("POST "+apiProjectsBase+"/import", h.ImportAPIProject)
	mux.HandleFunc("POST "+apiProjectsBase+"/validate", h.ValidateAPIProject)
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
