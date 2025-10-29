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
	"errors"
	"log"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

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
		if errors.Is(err, constants.ErrInvalidAPIDeployment) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid API deployment configuration"))
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

// GetAPIDeployedGateways handles GET /api/v1/apis/{apiId}/gateways
func (h *APIHandler) GetAPIDeployedGateways(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	// Get paginated gateways for the API
	gatewayListResponse, err := h.apiService.GetGatewaysForAPI(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API gateways"))
		return
	}

	// Return paginated gateway list
	c.JSON(http.StatusOK, gatewayListResponse)
}

// PublishToDevPortal handles POST /api/v1/apis/:apiId/publish-to-devportal
//
// This endpoint publishes an API to the developer portal with its metadata and OpenAPI definition.
// The API must exist in platform-api and the developer portal integration must be enabled.
//
// T034-T039: Handler implementation with validation and error handling
func (h *APIHandler) PublishToDevPortal(c *gin.Context) {
	// T035: Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// T035: Extract and validate apiId path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// T035: Parse optional request body (devPortalID for updates - reserved for US4)
	var req dto.PublishAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Request body is optional, so ignore binding errors
		log.Printf("[APIHandler] No request body provided for publish, using defaults: %v", err)
	}

	// T030-T033: Call service layer to publish API
	response, err := h.apiService.PublishAPI(apiID, orgID, req.DevPortalID)
	if err != nil {
		// T037: Error response handling
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalSync) {
			// T036: Check if devportal is disabled
			// T037: Devportal unavailable or sync failed
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"Failed to publish API to developer portal. Developer portal may be disabled or unavailable."))
			return
		}
		// T037: Internal server error
		log.Printf("[APIHandler] Failed to publish API %s: %v", apiID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to publish API to developer portal"))
		return
	}

	// T039: Log successful publish
	log.Printf("[APIHandler] API %s published successfully to developer portal (DevPortal ID: %s)",
		apiID, response.DevPortalRefID)

	// Return success response
	c.JSON(http.StatusOK, response)
}

// UnpublishFromDevPortal handles POST /api/v1/apis/:apiId/api-portals/unpublish
//
// This endpoint unpublishes an API from the developer portal by deleting it.
// The API must exist in platform-api and the developer portal integration must be enabled.
// The API ID from the path parameter is used as the devportal API ID by default.
//
// T042-T045: Handler implementation with validation and error handling
func (h *APIHandler) UnpublishFromDevPortal(c *gin.Context) {
	// T043: Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// T043: Extract and validate apiId path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// T043: Parse optional request body (devPortalID if different from apiID)
	var req dto.UnpublishAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Request body is optional, so ignore binding errors
		log.Printf("[APIHandler] No request body provided for unpublish, using defaults: %v", err)
	}

	// Call service layer to unpublish API
	response, err := h.apiService.UnpublishAPI(apiID, orgID, req.DevPortalID)
	if err != nil {
		// T044: Error response handling
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalSync) {
			// T044: Devportal unavailable or sync failed
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"Failed to unpublish API from developer portal. Developer portal may be disabled or unavailable."))
			return
		}
		// T044: Internal server error
		log.Printf("[APIHandler] Failed to unpublish API %s: %v", apiID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to unpublish API from developer portal"))
		return
	}

	// Log successful unpublish
	log.Printf("[APIHandler] API %s unpublished successfully from developer portal", apiID)

	// Return success response
	c.JSON(http.StatusOK, response)
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
		apiGroup.GET("/:apiId/gateways", h.GetAPIDeployedGateways)
		apiGroup.POST("/:apiId/api-portals/publish", h.PublishToDevPortal)
		apiGroup.POST("/:apiId/api-portals/unpublish", h.UnpublishFromDevPortal) // T045: Wire unpublish route
	}
}
