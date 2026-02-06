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
	"net/http"
	"platform-api/src/internal/constants"
	"strings"

	"github.com/gin-gonic/gin"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
)

// GatewayHandler handles HTTP requests for gateway operations
type GatewayHandler struct {
	gatewayService *service.GatewayService
}

// NewGatewayHandler creates a new gateway handler
func NewGatewayHandler(gatewayService *service.GatewayService) *GatewayHandler {
	return &GatewayHandler{
		gatewayService: gatewayService,
	}
}

// CreateGateway handles POST /api/v1/gateways
func (h *GatewayHandler) CreateGateway(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req dto.CreateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	response, err := h.gatewayService.RegisterGateway(orgId, req.Name, req.DisplayName, req.Description, req.Vhost,
		req.IsCritical, req.FunctionalityType, req.Properties)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "organization not found") {
			utils.LogError("Organization not found during gateway creation", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "already exists") {
			utils.LogError("Gateway already exists", err)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", errMsg))
			return
		}

		if strings.Contains(errMsg, "required") || strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "must") || strings.Contains(errMsg, "cannot") {
			utils.LogError("Invalid gateway creation request", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		utils.LogError("Failed to register gateway", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to register gateway"))
		return
	}

	// Return 201 Created with response
	c.JSON(http.StatusCreated, response)
}

// ListGateways handles GET /api/v1/gateways with constitution-compliant response
func (h *GatewayHandler) ListGateways(c *gin.Context) {
	organizationID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	listResponse, err := h.gatewayService.ListGateways(&organizationID)
	if err != nil {
		utils.LogError("Failed to list gateways", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list gateways"))
		return
	}

	// Return 200 OK with constitution-compliant envelope structure
	c.JSON(http.StatusOK, listResponse)
}

// GetGateway handles GET /api/v1/gateways/:gatewayId
func (h *GatewayHandler) GetGateway(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract UUID path parameter
	gatewayId := c.Param("gatewayId")
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	gateway, err := h.gatewayService.GetGateway(gatewayId, orgId)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "not found") {
			utils.LogError("Gateway not found", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "invalid UUID") {
			utils.LogError("Invalid gateway UUID", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		utils.LogError("Failed to retrieve gateway", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve gateway"))
		return
	}

	// Return 200 OK with gateway details
	c.JSON(http.StatusOK, gateway)
}

// GetGatewayStatus handles GET /api/v1/status/gateways
func (h *GatewayHandler) GetGatewayStatus(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Get optional gatewayId filter from query parameter
	gatewayId := c.Query("gatewayId")
	var gatewayIdPtr *string
	if gatewayId != "" {
		gatewayIdPtr = &gatewayId
	}

	// Get gateway status from service
	status, err := h.gatewayService.GetGatewayStatus(orgId, gatewayIdPtr)
	if err != nil {
		if strings.Contains(err.Error(), "gateway not found") {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get gateway status"))
		return
	}

	c.JSON(http.StatusOK, status)
}

// UpdateGateway handles PUT /api/v1/gateways/:gatewayId
func (h *GatewayHandler) UpdateGateway(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract UUID path parameter
	gatewayId := c.Param("gatewayId")
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	var req dto.UpdateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	gateway, err := h.gatewayService.UpdateGateway(gatewayId, orgId, req.Description, req.DisplayName, req.IsCritical, req.Properties)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			utils.LogError("Gateway not found during update", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		utils.LogError("Failed to update gateway", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update gateway"))
		return
	}

	c.JSON(http.StatusOK, gateway)
}

// DeleteGateway handles DELETE /api/v1/gateways/:gatewayId
func (h *GatewayHandler) DeleteGateway(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract UUID path parameter
	gatewayId := c.Param("gatewayId")
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	err := h.gatewayService.DeleteGateway(gatewayId, orgId)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, constants.ErrGatewayNotFound) {
			utils.LogError("Gateway not found during deletion", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"The specified resource does not exist"))
			return
		}
		if errors.Is(err, constants.ErrGatewayHasAssociatedAPIs) {
			utils.LogError("Gateway has associated APIs during deletion", err)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"The gateway has associated APIs. Please remove all API associations before deleting the gateway"))
			return
		}

		if strings.Contains(err.Error(), "invalid UUID") {
			utils.LogError("Invalid UUID during gateway deletion", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid gateway ID format"))
			return
		}

		// Internal server error
		utils.LogError("Failed to delete gateway", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"The server encountered an internal error. Please contact administrator."))
		return
	}

	// Return 204 No Content on successful deletion
	c.Status(http.StatusNoContent)
}

// RotateToken handles POST /api/v1/gateways/:gatewayId/tokens
func (h *GatewayHandler) RotateToken(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract ID path parameter
	gatewayId := c.Param("gatewayId")
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	response, err := h.gatewayService.RotateToken(gatewayId, orgId)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "gateway not found") {
			utils.LogError("Gateway not found during token rotation", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "maximum") || strings.Contains(errMsg, "Revoke") {
			utils.LogError("Token rotation request validation failed", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		utils.LogError("Failed to rotate token", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to rotate token"))
		return
	}

	// Return 201 Created with response
	c.JSON(http.StatusCreated, response)
}

// GetGatewayArtifacts handles GET /api/v1/gateways/{gatewayId}/live-proxy-artifacts
func (h *GatewayHandler) GetGatewayArtifacts(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := c.Param("gatewayId")
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	// Parse artifact type filter parameter
	artifactType := c.Query("artifactType")
	// Validate artifactType if provided
	if artifactType != "" {
		if !constants.ValidArtifactTypes[artifactType] {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid artifact type. Valid values are: "+constants.ArtifactTypeAPI+", "+constants.ArtifactTypeMCP+
					", "+constants.ArtifactTypeAPIProduct))
			return
		}
	}

	// Get paginated artifacts for the gateway
	artifactListResponse, err := h.gatewayService.GetGatewayArtifacts(gatewayId, orgId, artifactType)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get gateway artifacts"))
		return
	}

	// Return paginated artifact list
	c.JSON(http.StatusOK, artifactListResponse)
}

// RegisterRoutes registers gateway routes with the router
func (h *GatewayHandler) RegisterRoutes(r *gin.Engine) {
	gatewayGroup := r.Group("/api/v1/gateways")
	{
		gatewayGroup.POST("", h.CreateGateway)
		gatewayGroup.GET("", h.ListGateways)
		gatewayGroup.GET("/:gatewayId", h.GetGateway)
		gatewayGroup.PUT("/:gatewayId", h.UpdateGateway)
		gatewayGroup.DELETE("/:gatewayId", h.DeleteGateway)
		gatewayGroup.POST("/:gatewayId/tokens", h.RotateToken)
		gatewayGroup.GET("/:gatewayId/live-proxy-artifacts", h.GetGatewayArtifacts)
	}

	gatewayStatusGroup := r.Group("/api/v1/status")
	{
		gatewayStatusGroup.GET("/gateways", h.GetGatewayStatus)
	}
}
