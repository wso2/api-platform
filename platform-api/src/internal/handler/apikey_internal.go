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
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// APIKeyInternalHandler handles internal API key operations for Cloud APIM integration
type APIKeyInternalHandler struct {
	gatewayService *service.GatewayService
	apiKeyService  *service.APIKeyService
}

// NewAPIKeyInternalHandler creates a new API key internal handler
func NewAPIKeyInternalHandler(gatewayService *service.GatewayService, apiKeyService *service.APIKeyService) *APIKeyInternalHandler {
	return &APIKeyInternalHandler{
		gatewayService: gatewayService,
		apiKeyService:  apiKeyService,
	}
}

// CreateAPIKey handles POST /api/internal/v1/apis/{apiId}/api-keys
// This endpoint allows Cloud APIM to inject external API keys to hybrid gateways
func (h *APIKeyInternalHandler) CreateAPIKey(c *gin.Context) {
	// Extract client IP for logging
	clientIP := c.ClientIP()

	// Extract and validate API key from header
	apiKey := c.GetHeader("api-key")
	if apiKey == "" {
		log.Printf("[WARN] Unauthorized API key creation attempt from IP: %s - Missing API key", clientIP)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"API key is required. Provide 'api-key' header."))
		return
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		log.Printf("[WARN] API key creation authentication failed ip: %s - error=%v", clientIP, err)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Invalid or expired API key"))
		return
	}

	// Extract API ID from path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// Parse and validate request body
	var req dto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[WARN] Invalid API key creation request from IP: %s - error=%v", clientIP, err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	// Validate organization matches
	orgID := gateway.OrganizationID

	// Create the API key
	err = h.apiKeyService.CreateAPIKey(c.Request.Context(), apiID, orgID, &req)
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
		if errors.Is(err, constants.ErrAPIKeyHashingFailed) {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to process API key"))
			return
		}

		log.Printf("[ERROR] Failed to create API key: apiId=%s gatewayId=%s keyName=%s error=%v",
			apiID, gateway.ID, req.Name, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API key"))
		return
	}

	log.Printf("[INFO] Successfully created API key: apiId=%s gatewayId=%s keyName=%s orgId=%s",
		apiID, gateway.ID, req.Name, orgID)

	// Return success response
	c.JSON(http.StatusCreated, dto.CreateAPIKeyResponse{
		Status:  "success",
		Message: "API key registered successfully",
		KeyId:   req.Name, // Using name as keyId for external reference
	})
}

// RevokeAPIKey handles DELETE /api/internal/v1/apis/{apiId}/api-keys/{keyName}
// This endpoint allows Cloud APIM to revoke API keys from hybrid gateways
func (h *APIKeyInternalHandler) RevokeAPIKey(c *gin.Context) {
	// Extract client IP for logging
	clientIP := c.ClientIP()

	// Extract and validate API key from header
	apiKey := c.GetHeader("api-key")
	if apiKey == "" {
		log.Printf("[WARN] Unauthorized API key revocation attempt from IP: %s - Missing API key", clientIP)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"API key is required. Provide 'api-key' header."))
		return
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		log.Printf("[WARN] API key revocation authentication failed ip: %s - error=%v", clientIP, err)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Invalid or expired API key"))
		return
	}

	// Extract API ID from path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// Extract key name from path parameter
	keyName := c.Param("keyName")
	if keyName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Key name is required"))
		return
	}

	// Validate organization matches
	orgID := gateway.OrganizationID

	// Revoke the API key
	err = h.apiKeyService.RevokeAPIKey(c.Request.Context(), apiID, orgID, keyName)
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

		log.Printf("[ERROR] Failed to revoke API key: apiId=%s gatewayId=%s keyName=%s error=%v",
			apiID, gateway.ID, keyName, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to revoke API key"))
		return
	}

	log.Printf("[INFO] Successfully revoked API key: apiId=%s gatewayId=%s keyName=%s orgId=%s",
		apiID, gateway.ID, keyName, orgID)

	// Return success response
	c.JSON(http.StatusOK, dto.RevokeAPIKeyResponse{
		Status:  "success",
		Message: "API key revoked successfully",
	})
}

// RegisterRoutes registers the API key internal routes
func (h *APIKeyInternalHandler) RegisterRoutes(r *gin.Engine) {
	apiKeyGroup := r.Group("/api/internal/v1/apis/:apiId/api-keys")
	{
		apiKeyGroup.POST("", h.CreateAPIKey)
		apiKeyGroup.DELETE("/:keyName", h.RevokeAPIKey)
	}
}
