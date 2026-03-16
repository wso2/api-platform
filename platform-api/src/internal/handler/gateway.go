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
	"log/slog"
	"net/http"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"strings"

	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// GatewayHandler handles HTTP requests for gateway operations
type GatewayHandler struct {
	gatewayService *service.GatewayService
	slogger        *slog.Logger
}

// NewGatewayHandler creates a new gateway handler
func NewGatewayHandler(gatewayService *service.GatewayService, slogger *slog.Logger) *GatewayHandler {
	return &GatewayHandler{
		gatewayService: gatewayService,
		slogger:        slogger,
	}
}

// manifestSyncResponse is the response body for manifest-sync endpoints
type manifestSyncResponse struct {
	Status   string                           `json:"status"`
	Policies []service.GatewayPolicyDefinition `json:"policies,omitempty"`
}

// CreateGateway handles POST /api/v1/gateways
func (h *GatewayHandler) CreateGateway(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.CreateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Convert functionality type to string
	functionalityType := string(req.FunctionalityType)

	// Extract values from pointers
	var description string
	if req.Description != nil {
		description = *req.Description
	}

	var isCritical bool
	if req.IsCritical != nil {
		isCritical = *req.IsCritical
	}

	var properties map[string]interface{}
	if req.Properties != nil {
		properties = *req.Properties
	}

	gateway, err := h.gatewayService.RegisterGateway(orgId, req.Name, req.DisplayName, description, req.Vhost,
		isCritical, functionalityType, properties)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "organization not found") {
			h.slogger.Error("Organization not found during gateway creation", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "already exists") {
			h.slogger.Error("Gateway already exists", "error", err)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", errMsg))
			return
		}

		if strings.Contains(errMsg, "required") || strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "must") || strings.Contains(errMsg, "cannot") {
			h.slogger.Error("Invalid gateway creation request", "error", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to register gateway", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to register gateway"))
		return
	}

	// Return 201 Created with response
	c.JSON(http.StatusCreated, gateway)
}

// ListGateways handles GET /api/v1/gateways with constitution-compliant response
func (h *GatewayHandler) ListGateways(c *gin.Context) {
	organizationID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gateways, err := h.gatewayService.ListGateways(&organizationID)
	if err != nil {
		h.slogger.Error("Failed to list gateways", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list gateways"))
		return
	}

	// Return 200 OK with constitution-compliant envelope structure
	c.JSON(http.StatusOK, gateways)
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
			h.slogger.Error("Gateway not found", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "invalid UUID") {
			h.slogger.Error("Invalid gateway UUID", "error", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to retrieve gateway", "error", err)
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

	var req api.UpdateGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	response, err := h.gatewayService.UpdateGateway(gatewayId, orgId, req.Description, req.DisplayName, req.IsCritical, req.Properties)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			h.slogger.Error("Gateway not found during update", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		h.slogger.Error("Failed to update gateway", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update gateway"))
		return
	}

	c.JSON(http.StatusOK, response)
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
			h.slogger.Error("Gateway not found during deletion", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"The specified resource does not exist"))
			return
		}
		if errors.Is(err, constants.ErrGatewayHasAssociatedAPIs) {
			h.slogger.Error("Gateway has associated APIs during deletion", "error", err)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"The gateway has associated APIs. Please remove all API associations before deleting the gateway"))
			return
		}

		if strings.Contains(err.Error(), "invalid UUID") {
			h.slogger.Error("Invalid UUID during gateway deletion", "error", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid gateway ID format"))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to delete gateway", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"The server encountered an internal error. Please contact administrator."))
		return
	}

	// Return 204 No Content on successful deletion
	c.Status(http.StatusNoContent)
}

// ListTokens handles GET /api/v1/gateways/:gatewayId/tokens
func (h *GatewayHandler) ListTokens(c *gin.Context) {
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

	tokens, err := h.gatewayService.ListTokens(gatewayId, orgId)
	if err != nil {
		errMsg := err.Error()

		if strings.Contains(errMsg, "gateway not found") {
			h.slogger.Error("Gateway not found during token listing", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		h.slogger.Error("Failed to list tokens", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list tokens"))
		return
	}

	c.JSON(http.StatusOK, tokens)
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
			h.slogger.Error("Gateway not found during token rotation", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "maximum") || strings.Contains(errMsg, "Revoke") {
			h.slogger.Error("Token rotation request validation failed", "error", err)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to rotate token", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to rotate token"))
		return
	}

	// Return 201 Created with response
	c.JSON(http.StatusCreated, response)
}

// RevokeToken handles DELETE /api/v1/gateways/:gatewayId/tokens/:tokenId
func (h *GatewayHandler) RevokeToken(c *gin.Context) {
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

	tokenId := c.Param("tokenId")
	if tokenId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Token ID is required"))
		return
	}

	err := h.gatewayService.RevokeToken(gatewayId, tokenId, orgId)
	if err != nil {
		errMsg := err.Error()

		if strings.Contains(errMsg, "not found") {
			h.slogger.Error("Resource not found during token revocation", "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		h.slogger.Error("Failed to revoke token", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to revoke token"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Token revoked successfully"})
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

// GetManifestSync handles GET /api/v1/gateways/{gatewayId}/manifest-sync
// Called by APIM on behalf of the UI (which polls until status is "ready").
// On the first call (or after a failure) it triggers the manifest request to the gateway controller.
// Subsequent calls while pending simply return the current status.
// When ready, returns the filtered custom policies.
func (h *GatewayHandler) GetManifestSync(c *gin.Context) {
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

	job, err := h.gatewayService.SyncManifest(gatewayId, orgId)
	if err != nil {
		if strings.Contains(err.Error(), "gateway not found") {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to sync gateway manifest"))
		return
	}

	c.JSON(http.StatusOK, manifestSyncResponse{
		Status:   job.Status,
		Policies: job.Policies,
	})
}

// RegisterRoutes registers gateway routes with the router
func (h *GatewayHandler) RegisterRoutes(r *gin.Engine) {
	h.slogger.Debug("Registering gateway routes")
	gatewayGroup := r.Group("/api/v1/gateways")
	{
		gatewayGroup.POST("", h.CreateGateway)
		gatewayGroup.GET("", h.ListGateways)
		gatewayGroup.GET("/:gatewayId", h.GetGateway)
		gatewayGroup.PUT("/:gatewayId", h.UpdateGateway)
		gatewayGroup.DELETE("/:gatewayId", h.DeleteGateway)
		gatewayGroup.GET("/:gatewayId/tokens", h.ListTokens)
		gatewayGroup.POST("/:gatewayId/tokens", h.RotateToken)
		gatewayGroup.DELETE("/:gatewayId/tokens/:tokenId", h.RevokeToken)
		gatewayGroup.GET("/:gatewayId/live-proxy-artifacts", h.GetGatewayArtifacts)
		gatewayGroup.GET("/:gatewayId/manifest-sync", h.GetManifestSync)
	}

	gatewayStatusGroup := r.Group("/api/v1/status")
	{
		gatewayStatusGroup.GET("/gateways", h.GetGatewayStatus)
	}
}
