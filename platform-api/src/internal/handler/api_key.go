/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// APIKeyHandler handles API key operations for external services (Cloud APIM)
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(apiKeyService *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKey handles POST /api/v1/apis/{apiId}/api-keys
// This endpoint allows users to inject external API keys to all the gateways where the API is deployed
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract API handle from path parameter (parameter named apiId for backward compatibility, but contains handle)
	apiHandle := c.Param("apiId")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API handle is required"))
		return
	}

	// Parse and validate request body
	var req dto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid API key creation request", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body"))
		return
	}

	if req.ApiKey == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API key value is required"))
		return
	}

	// If user has provided a name, use it. Otherwise, generate a name from the display name.
	var name string
	if (req.Name != "") {
		name = req.Name
	} else {
		name, err := utils.GenerateHandle(req.DisplayName, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to generate API key name"))
			return
		}
		req.Name = name
	}

	if req.DisplayName == "" {
		req.DisplayName = name
	}

	// Extract optional x-user-id header for temporary user identification
	userId := c.GetHeader("x-user-id")

	// Create the API key and broadcast to gateways
	err := h.apiKeyService.CreateAPIKey(c.Request.Context(), apiHandle, orgId, userId, &req)
	if err != nil {
		// Handle specific error cases
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"No gateway connections available for API"))
			return
		}

		log.Printf("[ERROR] Failed to create API key: apiHandle=%s orgId=%s keyName=%s error=%v",
			apiHandle, orgId, req.Name, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API key"))
		return
	}

	log.Printf("[INFO] Successfully created API key: apiHandle=%s orgId=%s keyName=%s",
		apiHandle, orgId, req.Name)

	// Return success response
	c.JSON(http.StatusCreated, dto.CreateAPIKeyResponse{
		Status:  "success",
		KeyId:   req.Name,
		Message: "API key created and broadcasted to gateways successfully",
	})
}
// UpdateAPIKey handles PUT /api/v1/apis/{apiId}/api-keys/{keyName}
// This endpoint allows external platforms to update/regenerate external API keys on hybrid gateways
func (h *APIKeyHandler) UpdateAPIKey(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract API ID and key name from path parameters
	apiHandle := c.Param("apiId")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API handle is required"))
		return
	}

	keyName := c.Param("keyName")
	if keyName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API key name is required"))
		return
	}

	// Parse and validate request body
	var req dto.UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[WARN] Invalid API key update request: orgId=%s apiHandle=%s keyName=%s error=%v",
			orgId, apiHandle, keyName, err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	// Validate new API key value
	if req.ApiKey == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API key value is required"))
		return
	}

	// Extract optional x-user-id header for temporary user identification
	userId := c.GetHeader("x-user-id")

	// Update the API key and broadcast to gateways
	err := h.apiKeyService.UpdateAPIKey(c.Request.Context(), apiHandle, orgId, keyName, userId, &req)
	if err != nil {
		// Handle specific error cases
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"No gateway connections available for API"))
			return
		}

		log.Printf("[ERROR] Failed to update API key: apiHandle=%s orgId=%s keyName=%s error=%v",
			apiHandle, orgId, keyName, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update API key"))
		return
	}

	log.Printf("[INFO] Successfully updated API key: apiId=%s orgId=%s keyName=%s",
		apiHandle, orgId, keyName)

	// Return success response
	c.JSON(http.StatusOK, dto.UpdateAPIKeyResponse{
		Status:  "success",
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   keyName,
	})
}

// RevokeAPIKey handles DELETE /api/v1/apis/{apiId}/api-keys/{keyName}
// This endpoint allows Cloud APIM to revoke external API keys on hybrid gateways
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract API ID and key name from path parameters
	apiHandle := c.Param("apiId")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API handle is required"))
		return
	}

	keyName := c.Param("keyName")
	if keyName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API key name is required"))
		return
	}

	// Extract optional x-user-id header for temporary user identification
	userId := c.GetHeader("x-user-id")

	// Revoke the API key and broadcast to gateways
	err := h.apiKeyService.RevokeAPIKey(c.Request.Context(), apiHandle, orgId, keyName, userId)
	if err != nil {
		// Handle specific error cases
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"No gateway connections available for API"))
			return
		}

		log.Printf("[ERROR] Failed to revoke API key: apiId=%s orgId=%s keyName=%s error=%v",
			apiHandle, orgId, keyName, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to revoke API key in one or more gateways"))
		return
	}

	log.Printf("[INFO] Successfully revoked API key: apiHandle=%s orgId=%s keyName=%s",
		apiHandle, orgId, keyName)

	// Return success response (204 No Content)
	c.Status(http.StatusNoContent)
}

// RegisterRoutes registers API key routes with the router
func (h *APIKeyHandler) RegisterRoutes(r *gin.Engine) {
	apiKeyGroup := r.Group("/api/v1/apis/:apiId/api-keys")
	{
		apiKeyGroup.POST("", h.CreateAPIKey)
		apiKeyGroup.PUT("/:keyName", h.UpdateAPIKey)
		apiKeyGroup.DELETE("/:keyName", h.RevokeAPIKey)
	}
}
