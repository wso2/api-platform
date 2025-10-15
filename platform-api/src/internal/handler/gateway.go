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
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"platform-api/src/internal/dto"
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
	var req dto.CreateGatewayRequest

	// Bind and validate request body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Call service to register gateway
	response, err := h.gatewayService.RegisterGateway(req.OrganizationID, req.Name, req.DisplayName)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "organization not found") {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "already exists") {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", errMsg))
			return
		}

		if strings.Contains(errMsg, "required") || strings.Contains(errMsg, "invalid") ||
		   strings.Contains(errMsg, "must") || strings.Contains(errMsg, "cannot") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to register gateway"))
		return
	}

	// Return 201 Created with response
	c.JSON(http.StatusCreated, response)
}

// ListGateways handles GET /api/v1/gateways with constitution-compliant response
func (h *GatewayHandler) ListGateways(c *gin.Context) {
	// Extract optional organizationId query parameter (camelCase per constitution)
	orgID := c.Query("organizationId")

	var orgIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}

	// Call service to list gateways (returns structured response)
	listResponse, err := h.gatewayService.ListGateways(orgIDPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list gateways"))
		return
	}

	// Return 200 OK with constitution-compliant envelope structure
	c.JSON(http.StatusOK, listResponse)
}

// GetGateway handles GET /api/v1/gateways/:uuid
func (h *GatewayHandler) GetGateway(c *gin.Context) {
	// Extract UUID path parameter
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Gateway UUID is required"))
		return
	}

	// Call service to get gateway
	gateway, err := h.gatewayService.GetGateway(uuid)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "not found") {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "invalid UUID") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to retrieve gateway"))
		return
	}

	// Return 200 OK with gateway details
	c.JSON(http.StatusOK, gateway)
}

// RotateToken handles POST /api/v1/gateways/:uuid/tokens
func (h *GatewayHandler) RotateToken(c *gin.Context) {
	// Extract UUID path parameter
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Gateway UUID is required"))
		return
	}

	// Call service to rotate token
	response, err := h.gatewayService.RotateToken(uuid)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "gateway not found") {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "maximum") || strings.Contains(errMsg, "Revoke") {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to rotate token"))
		return
	}

	// Return 201 Created with response
	c.JSON(http.StatusCreated, response)
}

// RegisterRoutes registers gateway routes with the router
func (h *GatewayHandler) RegisterRoutes(r *gin.Engine) {
	gatewayGroup := r.Group("/api/v1/gateways")
	{
		gatewayGroup.POST("", h.CreateGateway)
		gatewayGroup.GET("", h.ListGateways)
		gatewayGroup.GET("/:uuid", h.GetGateway)
		gatewayGroup.POST("/:uuid/tokens", h.RotateToken)
	}
}
