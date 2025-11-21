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
	"log"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	apiService *service.APIService
}

func NewAPIHandler(apiService *service.APIService) *APIHandler {
	return &APIHandler{
		apiService: apiService,
	}
}

// CreateAPI handles POST /api/v1/apis and creates a new API
func (h *APIHandler) CreateAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req service.CreateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
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
	if req.ProjectID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	api, err := h.apiService.CreateAPI(&req, orgId)
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
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API"))
		return
	}

	c.JSON(http.StatusCreated, api)
}

// GetAPI handles GET /api/v1/apis/:apiId and retrieves an API by its ID
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

	api, err := h.apiService.GetAPIByUUID(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API"))
		return
	}

	c.JSON(http.StatusOK, api)
}

// ListAPIs handles GET /api/v1/apis and lists APIs for an organization with optional project filter
func (h *APIHandler) ListAPIs(c *gin.Context) {
	// Get organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Get optional project filter from query parameter
	projectId := c.Query("projectId")
	var projectIdPtr *string
	if projectId != "" {
		projectIdPtr = &projectId
	}

	apis, err := h.apiService.GetAPIsByOrganization(orgId, projectIdPtr)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get APIs"))
		return
	}

	// Return constitution-compliant list response
	c.JSON(http.StatusOK, dto.APIListResponse{
		Count: len(apis),
		List:  apis,
		Pagination: dto.Pagination{
			Total:  len(apis),
			Offset: 0,
			Limit:  len(apis),
		},
	})
}

// UpdateAPI updates an existing API
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

	var req service.UpdateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			err.Error()))
		return
	}

	api, err := h.apiService.UpdateAPI(apiId, &req, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
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
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update API"))
		return
	}

	c.JSON(http.StatusOK, api)
}

// DeleteAPI handles DELETE /api/v1/apis/:apiId and deletes an API by its ID
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

	err := h.apiService.DeleteAPI(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete API"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// AddGatewaysToAPI handles POST /api/v1/apis/:apiId/gateways to associate gateways with an API
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

	var req []dto.AddGatewayToAPIRequest
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
		gatewayIds[i] = gw.GatewayID
	}

	gateways, err := h.apiService.AddGatewaysToAPI(apiId, gatewayIds, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"One or more gateways not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to associate gateways with API"))
		return
	}

	c.JSON(http.StatusOK, gateways)
}

// GetAPIGateways handles GET /api/v1/apis/:apiId/gateways to get gateways associated with an API including deployment details
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

	gateways, err := h.apiService.GetAPIGateways(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API gateways"))
		return
	}

	c.JSON(http.StatusOK, gateways)
}

// DeployAPIRevision handles POST /api/v1/apis/:apiId/deploy-revision to deploy an API revision
func (h *APIHandler) DeployAPIRevision(c *gin.Context) {
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

	// Get optional revision ID from query parameter
	revisionID := c.Query("revisionId")

	// Parse deployment request body
	var deploymentRequests []dto.APIRevisionDeployment
	if err := c.ShouldBindJSON(&deploymentRequests); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			err.Error()))
		return
	}

	// Validate that we have at least one deployment request
	if len(deploymentRequests) == 0 {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one deployment configuration is required"))
		return
	}

	// Call service to deploy the API
	deployments, err := h.apiService.DeployAPIRevision(apiId, revisionID, deploymentRequests, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if strings.Contains(err.Error(), "invalid api deployment") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API deployment configuration: "+err.Error()))
			return
		}
		log.Printf("[ERROR] Failed to deploy API revision: apiUUID=%s revisionID=%s error=%v",
			apiId, revisionID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to deploy API revision"))
		return
	}

	c.JSON(http.StatusOK, deployments)
}

// PublishToDevPortal handles POST /api/v1/apis/:apiId/devportals/publish
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
	var req dto.PublishToDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	// Publish API to DevPortal through service layer
	response, err := h.apiService.PublishAPIToDevPortal(apiID, &req, orgID)
	if err != nil {
		// Handle specific errors
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIPublicationInProgress) {
			// Publication already in progress
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API publication is already in progress. Please wait for the current operation to complete or try again in a few minutes."))
			return
		}
		if errors.Is(err, constants.ErrAPIAlreadyPublished) {
			// API already published
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"API is already published to this DevPortal"))
			return
		}
		if errors.Is(err, constants.ErrApiPortalSync) {
			// Devportal unavailable or sync failed
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"Failed to publish API to DevPortal. DevPortal may be unavailable."))
			return
		}

		// Internal server error
		log.Printf("[APIHandler] Failed to publish API %s to DevPortal %s: %v", apiID, req.DevPortalUUID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to publish API to DevPortal"))
		return
	}

	// Log successful publish
	log.Printf("[APIHandler] API %s published successfully to DevPortal %s", apiID, req.DevPortalUUID)

	// Return success response
	c.JSON(http.StatusOK, response)
}

// UnpublishFromDevPortal handles POST /api/v1/apis/:apiId/devportals/unpublish
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
	var req dto.UnpublishFromDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	// Unpublish API from DevPortal through service layer
	response, err := h.apiService.UnpublishAPIFromDevPortal(apiID, req.DevPortalUUID, orgID)
	if err != nil {
		// Handle specific errors
		if errors.Is(err, constants.ErrAPIPublicationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API publication not found"))
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}
		if errors.Is(err, constants.ErrApiPortalSync) {
			// Devportal unavailable or sync failed
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"Failed to unpublish API from DevPortal. DevPortal may be unavailable."))
			return
		}

		// Internal server error
		log.Printf("[APIHandler] Failed to unpublish API %s from DevPortal %s: %v", apiID, req.DevPortalUUID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to unpublish API from DevPortal"))
		return
	}

	// Log successful unpublish
	log.Printf("[APIHandler] API %s unpublished successfully from DevPortal %s", apiID, req.DevPortalUUID)

	// Return success response
	c.JSON(http.StatusOK, response)
}

// GetAPIPublications handles GET /api/v1/apis/:apiId/publications
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
	response, err := h.apiService.GetAPIPublications(apiID, orgID)
	if err != nil {
		// Handle specific errors
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}

		// Internal server error
		log.Printf("[APIHandler] Failed to get publications for API %s: %v", apiID, err)
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

	var req dto.ImportAPIProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.RepoURL == "" {
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
	if req.API.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if req.API.Context == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if req.API.Version == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if req.API.ProjectID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	// Create Git service
	gitService := service.NewGitService()

	// Import API project
	api, err := h.apiService.ImportAPIProject(&req, orgId, gitService)
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

		log.Printf("Failed to import API project: %v", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to import API project"))
		return
	}

	c.JSON(http.StatusCreated, api)
}

// ValidateAPIProject handles POST /validate/api-project
func (h *APIHandler) ValidateAPIProject(c *gin.Context) {
	var req dto.ValidateAPIProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.RepoURL == "" {
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

	var req dto.ValidateOpenAPIRequest

	// Get URL from form if provided
	if url := c.PostForm("url"); url != "" {
		req.URL = url
	}

	// Get definition file from form if provided
	if file, header, err := c.Request.FormFile("definition"); err == nil {
		req.Definition = header
		defer file.Close()
	}

	// Validate that at least one input is provided
	if req.URL == "" && req.Definition == nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Either URL or definition file must be provided"))
		return
	}

	// Validate OpenAPI definition
	response, err := h.apiService.ValidateOpenAPIDefinition(&req)
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

	var req dto.ImportOpenAPIRequest

	// Get URL from form if provided
	if url := c.PostForm("url"); url != "" {
		req.URL = url
	}

	// Get definition file from form if provided
	if file, header, err := c.Request.FormFile("definition"); err == nil {
		req.Definition = header
		defer file.Close()
	}

	// Validate that at least one input is provided
	if req.URL == "" && req.Definition == nil {
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

	// Parse API details from JSON string
	if err := json.Unmarshal([]byte(apiJSON), &req.API); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid API details: "+err.Error()))
		return
	}

	if req.API.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API name is required"))
		return
	}
	if req.API.Context == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API context is required"))
		return
	}
	if req.API.Version == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API version is required"))
		return
	}
	if req.API.ProjectID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	// Import API from OpenAPI definition
	api, err := h.apiService.ImportFromOpenAPI(&req, orgId)
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

	c.JSON(http.StatusCreated, api)
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	// API routes
	apiGroup := r.Group("/api/v1/apis")
	{
		apiGroup.POST("", h.CreateAPI)
		apiGroup.GET("", h.ListAPIs)
		apiGroup.GET("/:apiId", h.GetAPI)
		apiGroup.PUT("/:apiId", h.UpdateAPI)
		apiGroup.DELETE("/:apiId", h.DeleteAPI)
		apiGroup.POST("/:apiId/deploy-revision", h.DeployAPIRevision)
		apiGroup.GET("/:apiId/gateways", h.GetAPIGateways)
		apiGroup.POST("/:apiId/gateways", h.AddGatewaysToAPI)
		apiGroup.POST("/:apiId/devportals/publish", h.PublishToDevPortal)
		apiGroup.POST("/:apiId/devportals/unpublish", h.UnpublishFromDevPortal)
		apiGroup.GET("/:apiId/publications", h.GetAPIPublications)
	}
	importGroup := r.Group("/api/v1/import")
	{
		importGroup.POST("/api-project", h.ImportAPIProject)
		importGroup.POST("/open-api", h.ImportOpenAPI)
	}
	validateGroup := r.Group("/api/v1/validate")
	{
		validateGroup.POST("/api-project", h.ValidateAPIProject)
		validateGroup.POST("/open-api", h.ValidateOpenAPI)
	}
}
