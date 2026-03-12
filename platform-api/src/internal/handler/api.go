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
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
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

// CreateAPI handles POST /api/v1/rest-apis and creates a new API
func (h *APIHandler) CreateAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.CreateRESTAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.NewValidationErrorResponse(c, err)
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if req.Context == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if req.Version == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if req.ProjectId == (openapi_types.UUID{}) {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		h.slogger.Error("Validation failed: No upstream endpoints provided", "organizationId", orgId)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one upstream endpoint (main or sandbox) is required"))
		return
	}

	apiResponse, err := h.apiService.CreateAPI(&req, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			h.slogger.Error("API handle already exists", "organizationId", orgId)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API handle already exists"))
			return
		}
		if errors.Is(err, constants.ErrAPINameVersionAlreadyExists) {
			h.slogger.Error("API with same name and version already exists", "organizationId", orgId)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API with same name and version already exists in the organization"))
			return
		}
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			h.slogger.Error("API already exists in the project", "organizationId", orgId)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			h.slogger.Error("Project not found", "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			h.slogger.Error("Invalid API name format", "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			h.slogger.Error("Invalid API context format", "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API context format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIVersion) {
			h.slogger.Error("Invalid API version format", "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API version format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			h.slogger.Error("Invalid lifecycle status", "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			h.slogger.Error("Invalid API type", "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			h.slogger.Error("Invalid transport protocol", "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid transport protocol"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive) {
			h.slogger.Error("Subscription plan not found or not active", "organizationId", orgId, "error", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				err.Error()))
			return
		}
		h.slogger.Error("Failed to create API", "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API"))
		return
	}

	c.JSON(http.StatusCreated, apiResponse)
}

// GetAPI handles GET /api/v1/rest-apis/:apiId and retrieves an API by its handle
func (h *APIHandler) GetAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	apiResponse, err := h.apiService.GetAPIByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to get API", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API"))
		return
	}

	c.JSON(http.StatusOK, apiResponse)
}

// ListAPIs handles GET /api/v1/rest-apis and lists APIs for an organization with optional project filter
func (h *APIHandler) ListAPIs(c *gin.Context) {
	// Get organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var params api.ListRESTAPIsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	var projectId string
	if params.ProjectId != nil {
		projectId = string(*params.ProjectId)
	}

	apis, err := h.apiService.GetAPIsByOrganization(orgId, projectId)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			h.slogger.Error("Project not found", "organizationId", orgId, "projectId", projectId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		h.slogger.Error("Failed to get APIs", "organizationId", orgId, "projectId", projectId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
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

	c.JSON(http.StatusOK, response)
}

// UpdateAPI updates an existing API identified by handle
func (h *APIHandler) UpdateAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var req api.UpdateRESTAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			err.Error()))
		return
	}

	// Validate upstream configuration if provided
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one upstream endpoint (main or sandbox) is required"))
		return
	}

	apiResponse, err := h.apiService.UpdateAPIByHandle(apiId, &req, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			h.slogger.Error("Invalid lifecycle status", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			h.slogger.Error("Invalid API type", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			h.slogger.Error("Invalid transport protocol", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid transport protocol"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive) {
			h.slogger.Error("Subscription plan not found or not active", "apiId", apiId, "organizationId", orgId, "error", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				err.Error()))
			return
		}
		h.slogger.Error("Failed to update API", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update API"))
		return
	}

	c.JSON(http.StatusOK, apiResponse)
}

// DeleteAPI handles DELETE /api/v1/rest-apis/:apiId and deletes an API by its handle
func (h *APIHandler) DeleteAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	err := h.apiService.DeleteAPIByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to delete API", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete API"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// AddGatewaysToAPI handles POST /api/v1/rest-apis/:apiId/gateways to associate gateways with an API
func (h *APIHandler) AddGatewaysToAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var req []api.AddGatewayToRESTAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if len(req) == 0 {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
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
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			h.slogger.Error("One or more gateways not found", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"One or more gateways not found"))
			return
		}
		h.slogger.Error("Failed to associate gateways with API", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to associate gateways with API"))
		return
	}

	c.JSON(http.StatusOK, gatewaysResponse)
}

// GetAPIGateways handles GET /api/v1/rest-apis/:apiId/gateways to get gateways associated with an API including deployment details
func (h *APIHandler) GetAPIGateways(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	gatewaysResponse, err := h.apiService.GetAPIGatewaysByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiId", apiId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to get API gateways", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API gateways"))
		return
	}

	c.JSON(http.StatusOK, gatewaysResponse)
}

// PublishToDevPortal handles POST /api/v1/rest-apis/:apiId/devportals/publish
//
// This endpoint publishes an API to a specific DevPortal with its metadata and OpenAPI definition.
// The API must exist in platform-api and the specified DevPortal must be active.
func (h *APIHandler) PublishToDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract and validate apiId path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// Parse request body
	var req api.PublishToDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	// Publish API to DevPortal through service layer
	err := h.apiService.PublishAPIToDevPortalByHandle(apiID, &req, orgID)
	if err != nil {
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	// Log successful publish
	h.slogger.Info("API published successfully to DevPortal", "apiID", apiID, "devPortalUUID", utils.OpenAPIUUIDToString(req.DevPortalUuid))

	// Return success response
	c.JSON(http.StatusOK, api.CommonResponse{
		Success:   true,
		Message:   "API published successfully to DevPortal",
		Timestamp: time.Now(),
	})
}

// UnpublishFromDevPortal handles POST /api/v1/rest-apis/:apiId/devportals/unpublish
//
// This endpoint unpublishes an API from a specific DevPortal by deleting it.
// The API must exist in platform-api and the specified DevPortal must exist.
func (h *APIHandler) UnpublishFromDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract and validate apiId path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// Parse request body
	var req api.UnpublishFromDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	// Unpublish API from DevPortal through service layer
	err := h.apiService.UnpublishAPIFromDevPortalByHandle(apiID, utils.OpenAPIUUIDToString(req.DevPortalUuid), orgID)
	if err != nil {
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	// Log successful unpublish
	h.slogger.Info("API unpublished successfully from DevPortal", "apiID", apiID, "devPortalUUID", utils.OpenAPIUUIDToString(req.DevPortalUuid))

	// Return success response
	c.JSON(http.StatusOK, api.CommonResponse{
		Success:   true,
		Message:   "API unpublished successfully from DevPortal",
		Timestamp: time.Now(),
	})
}

// GetAPIPublications handles GET /api/v1/rest-apis/:apiId/publications
//
// This endpoint retrieves all DevPortals associated with an API including publication details.
func (h *APIHandler) GetAPIPublications(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract and validate apiId path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	// Get publications through service layer
	response, err := h.apiService.GetAPIPublicationsByHandle(apiID, orgID)
	if err != nil {
		// Handle specific errors
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found", "apiID", apiID, "organizationId", orgID)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to get publications for API", "apiID", apiID, "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve API publications"))
		return
	}

	// Return success response
	c.JSON(http.StatusOK, response)
}

// ImportAPIProject handles POST /api/v1/import/api-project
func (h *APIHandler) ImportAPIProject(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.ImportAPIProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.RepoUrl == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Repository URL is required"))
		return
	}
	if req.Branch == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Branch is required"))
		return
	}
	if req.Path == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Path is required"))
		return
	}
	if req.Api.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if req.Api.Context == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if req.Api.Version == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if req.Api.ProjectId == (openapi_types.UUID{}) {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	// Create Git service
	gitService := service.NewGitService()

	// Import API project
	apiResponse, err := h.apiService.ImportAPIProject(&req, orgId, gitService)
	if err != nil {
		if errors.Is(err, constants.ErrAPIProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "API Project Not Found",
				"API project not found: .api-platform directory not found"))
			return
		}
		if errors.Is(err, constants.ErrMalformedAPIProject) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Malformed API Project",
				"Malformed API project: config.yaml is missing or invalid"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIProject) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Invalid API Project",
				"Invalid API project: referenced files not found"))
			return
		}
		if errors.Is(err, constants.ErrConfigFileNotFound) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Config File Not Found",
				"config.yaml file not found in .api-platform directory"))
			return
		}
		if errors.Is(err, constants.ErrOpenAPIFileNotFound) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "OpenAPI File Not Found",
				"OpenAPI file not found"))
			return
		}
		if errors.Is(err, constants.ErrWSO2ArtifactNotFound) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "WSO2 Artifact Not Found",
				"WSO2 artifact file not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API context format"))
			return
		}

		h.slogger.Error("Failed to import API project", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to import API project"))
		return
	}

	c.JSON(http.StatusCreated, apiResponse)
}

// ValidateAPIProject handles POST /validate/api-project
func (h *APIHandler) ValidateAPIProject(c *gin.Context) {
	var req api.ValidateAPIProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.RepoUrl == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Repository URL is required"))
		return
	}
	if req.Branch == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Branch is required"))
		return
	}
	if req.Path == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Path is required"))
		return
	}

	// Create Git service
	gitService := service.NewGitService()

	// Validate API project
	response, err := h.apiService.ValidateAndRetrieveAPIProject(&req, gitService)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to validate API project"))
		return
	}

	// Return validation response (200 OK even if validation fails - errors are in the response body)
	c.JSON(http.StatusOK, response)
}

// ValidateOpenAPI handles POST /validate/open-api
func (h *APIHandler) ValidateOpenAPI(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Failed to parse multipart form"))
		return
	}

	var req api.ValidateOpenAPIRequest
	var definitionHeader *multipart.FileHeader

	// Get URL from form if provided
	if url := c.PostForm("url"); url != "" {
		req.Url = &url
	}

	// Get definition file from form if provided
	if file, header, err := c.Request.FormFile("definition"); err == nil {
		definitionHeader = header
		var openapiFile openapi_types.File
		openapiFile.InitFromMultipart(header)
		req.Definition = &openapiFile
		defer file.Close()
	}

	// Validate that at least one input is provided
	if req.Url == nil && req.Definition == nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either URL or definition file must be provided"))
		return
	}

	// Validate OpenAPI definition
	response, err := h.apiService.ValidateOpenAPIDefinition(req.Url, definitionHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to validate OpenAPI definition"))
		return
	}

	// Return validation response (200 OK even if validation fails - errors are in the response body)
	c.JSON(http.StatusOK, response)
}

// ImportOpenAPI handles POST /import/open-api and imports an API from OpenAPI definition
func (h *APIHandler) ImportOpenAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Parse multipart form
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Failed to parse multipart form"))
		return
	}

	var req api.ImportOpenAPIRequest
	var definitionHeader *multipart.FileHeader

	// Get URL from form if provided
	if url := c.PostForm("url"); url != "" {
		req.Url = &url
	}

	// Get definition file from form if provided
	if file, header, err := c.Request.FormFile("definition"); err == nil {
		definitionHeader = header
		var openapiFile openapi_types.File
		openapiFile.InitFromMultipart(header)
		req.Definition = &openapiFile
		defer file.Close()
	}

	// Validate that at least one input is provided
	if req.Url == nil && req.Definition == nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either URL or definition file must be provided"))
		return
	}

	// Get API details from form data (JSON string in 'api' field)
	apiJSON := c.PostForm("api")
	if apiJSON == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API details are required"))
		return
	}
	req.Api = apiJSON

	// Parse API details from JSON string
	var apiDetails api.RESTAPI
	if err := json.Unmarshal([]byte(apiJSON), &apiDetails); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid API details: "+err.Error()))
		return
	}

	if apiDetails.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if apiDetails.Context == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if apiDetails.Version == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if apiDetails.ProjectId == (openapi_types.UUID{}) {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	// Import API from OpenAPI definition
	apiResponse, err := h.apiService.ImportFromOpenAPI(&apiDetails, req.Url, definitionHeader, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API context format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIVersion) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API version format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid transport protocol"))
			return
		}
		// Handle OpenAPI-specific errors
		if strings.Contains(err.Error(), "failed to fetch OpenAPI from URL") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to fetch OpenAPI definition from URL"))
			return
		}
		if strings.Contains(err.Error(), "failed to open OpenAPI definition file") ||
			strings.Contains(err.Error(), "failed to read OpenAPI definition file") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to fetch OpenAPI definition from file"))
			return
		}
		if strings.Contains(err.Error(), "failed to validate and parse OpenAPI definition") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid OpenAPI definition"))
			return
		}
		if strings.Contains(err.Error(), "failed to merge API details") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to create API from OpenAPI definition: incompatible details"))
			return
		}

		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to import API from OpenAPI definition"))
		return
	}

	c.JSON(http.StatusCreated, apiResponse)
}

// ValidateAPI handles GET /api/v1/rest-apis/validate
func (h *APIHandler) ValidateAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var params api.ValidateRESTAPIParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate that either identifier OR both name and version are provided
	identifier := ""
	name := ""
	version := ""
	if params.Identifier != nil {
		identifier = string(*params.Identifier)
	}
	if params.Name != nil {
		name = string(*params.Name)
	}
	if params.Version != nil {
		version = string(*params.Version)
	}
	if identifier == "" && (name == "" || version == "") {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either 'identifier' or both 'name' and 'version' query parameters are required"))
		return
	}

	response, err := h.apiService.ValidateAPI(&params, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to validate API"))
		return
	}

	// Always return 200 OK with the validation result
	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	h.slogger.Debug("Registering REST API routes")
	// API routes
	apiGroup := r.Group("/api/v1/rest-apis")
	{
		apiGroup.POST("", h.CreateAPI)
		apiGroup.GET("", h.ListAPIs)
		apiGroup.GET("/:apiId", h.GetAPI)
		apiGroup.PUT("/:apiId", h.UpdateAPI)
		apiGroup.DELETE("/:apiId", h.DeleteAPI)
		apiGroup.GET("/validate", h.ValidateAPI)
		apiGroup.GET("/:apiId/gateways", h.GetAPIGateways)
		apiGroup.POST("/:apiId/gateways", h.AddGatewaysToAPI)
		apiGroup.POST("/:apiId/devportals/publish", h.PublishToDevPortal)
		apiGroup.POST("/:apiId/devportals/unpublish", h.UnpublishFromDevPortal)
		apiGroup.GET("/:apiId/publications", h.GetAPIPublications)
	}
	importGroup := r.Group("/api/v1/import")
	{
		importGroup.POST("/api-project", h.ImportAPIProject)
		importGroup.POST("/openapi", h.ImportOpenAPI)
	}
	validateGroup := r.Group("/api/v1/validate")
	{
		validateGroup.POST("/api-project", h.ValidateAPIProject)
		validateGroup.POST("/openapi", h.ValidateOpenAPI)
	}
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
