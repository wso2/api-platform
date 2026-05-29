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
	"fmt"
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// APIKeyHandler handles API key operations for external services (Cloud APIM)
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService
	slogger       *slog.Logger
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(apiKeyService *service.APIKeyService, slogger *slog.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		slogger:       slogger,
	}
}

// CreateAPIKey handles POST /rest-apis/{apiId}/api-keys
// This endpoint allows users to inject external API keys to all the gateways where the API is deployed
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract optional x-user-id header for user identification (empty string if not present)
	userId := c.GetHeader("x-user-id")

	// Extract API handle from path parameter (parameter named apiId for backward compatibility, but contains handle)
	apiHandle := c.Param("apiId")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API handle is required"))
		return
	}

	// Parse and validate request body
	var req api.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("Invalid API key creation request", "userId", userId, "error", err)
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
	if req.Name != nil && *req.Name != "" {
		name = *req.Name
	} else {
		displayName := ""
		if req.DisplayName != nil {
			displayName = *req.DisplayName
		}
		generatedName, err := utils.GenerateHandle(displayName, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Failed to generate API key name"))
			return
		}
		name = generatedName
		req.Name = &name
	}

	if req.DisplayName == nil || *req.DisplayName == "" {
		req.DisplayName = &name
	}

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

		keyName := ""
		if req.Name != nil {
			keyName = *req.Name
		}
		h.slogger.Error("Failed to create API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API key"))
		return
	}

	keyName := ""
	if req.Name != nil {
		keyName = *req.Name
	}
	h.slogger.Info("Successfully created API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	// Return success response
	c.JSON(http.StatusCreated, api.CreateAPIKeyResponse{
		Status:  api.CreateAPIKeyResponseStatusSuccess,
		KeyId:   req.Name,
		Message: "API key created and broadcasted to gateways successfully",
	})
}

// UpdateAPIKey handles PUT /rest-apis/{apiId}/api-keys/{keyName}
// This endpoint allows external platforms to update/regenerate external API keys on hybrid gateways
func (h *APIKeyHandler) UpdateAPIKey(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract optional x-user-id header for user identification (empty string if not present)
	userId := c.GetHeader("x-user-id")

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
	var req api.UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Warn("Invalid API key update request", "userId", userId, "orgId", orgId, "apiHandle", apiHandle, "keyName", keyName, "error", err)
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

	// Validate that the name in the request body (if provided) matches the URL path parameter
	if req.Name != nil && *req.Name != "" && *req.Name != keyName {
		h.slogger.Warn("API key name mismatch", "userId", userId, "orgId", orgId, "apiHandle", apiHandle, "urlKeyName", keyName, "bodyKeyName", *req.Name)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			fmt.Sprintf("API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *req.Name, keyName)))
		return
	}

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

		h.slogger.Error("Failed to update API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update API key"))
		return
	}

	h.slogger.Info("Successfully updated API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	// Return success response
	c.JSON(http.StatusOK, api.UpdateAPIKeyResponse{
		Status:  api.UpdateAPIKeyResponseStatusSuccess,
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   &keyName,
	})
}

// RevokeAPIKey handles DELETE /rest-apis/{apiId}/api-keys/{keyName}
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

	// Extract optional x-user-id header for user identification (empty string if not present)
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

		h.slogger.Error("Failed to revoke API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to revoke API key in one or more gateways"))
		return
	}

	h.slogger.Info("Successfully revoked API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	// Return success response (204 No Content)
	c.Status(http.StatusNoContent)
}

// RegisterRoutes registers API key routes with the router
func (h *APIKeyHandler) RegisterRoutes(r *gin.Engine) {
	h.slogger.Debug("Registering API key routes")
	apiKeyGroup := r.Group("/api/v1/rest-apis/:apiId/api-keys")
	{
		apiKeyGroup.POST("", h.CreateAPIKey)
		apiKeyGroup.PUT("/:keyName", h.UpdateAPIKey)
		apiKeyGroup.DELETE("/:keyName", h.RevokeAPIKey)
	}
}
