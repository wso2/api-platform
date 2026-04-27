/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
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

// WebSubAPIKeyHandler handles API key operations for WebSub APIs
type WebSubAPIKeyHandler struct {
	websubAPIService *service.WebSubAPIService
	apiKeyService    *service.APIKeyService
	slogger          *slog.Logger
}

// NewWebSubAPIKeyHandler creates a new WebSubAPIKeyHandler instance
func NewWebSubAPIKeyHandler(websubAPIService *service.WebSubAPIService, apiKeyService *service.APIKeyService, slogger *slog.Logger) *WebSubAPIKeyHandler {
	return &WebSubAPIKeyHandler{
		websubAPIService: websubAPIService,
		apiKeyService:    apiKeyService,
		slogger:          slogger,
	}
}

// RegisterRoutes registers WebSub API key routes
func (h *WebSubAPIKeyHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1/websub-apis/:apiId/api-keys")
	{
		v1.POST("", h.CreateAPIKey)
		v1.PUT("/:keyName", h.UpdateAPIKey)
		v1.DELETE("/:keyName", h.DeleteAPIKey)
	}
}

// CreateAPIKey handles POST /api/v1/websub-apis/:apiId/api-keys
func (h *WebSubAPIKeyHandler) CreateAPIKey(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiId")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	// Verify it's a WebSub API
	if _, err := h.websubAPIService.Get(orgID, apiHandle); err != nil {
		h.handleServiceError(c, err)
		return
	}

	userId := c.GetHeader("x-user-id")

	var req api.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.ApiKey == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API key value is required"))
		return
	}

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
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Failed to generate API key name"))
			return
		}
		name = generatedName
		req.Name = &name
	}
	if req.DisplayName == nil || *req.DisplayName == "" {
		req.DisplayName = &name
	}

	if err := h.apiKeyService.CreateAPIKey(c.Request.Context(), apiHandle, orgID, userId, &req); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "No gateway connections available"))
			return
		}
		h.slogger.Error("Failed to create API key for WebSub API", "apiHandle", apiHandle, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create API key"))
		return
	}

	c.JSON(http.StatusCreated, api.CreateAPIKeyResponse{
		Status:  api.CreateAPIKeyResponseStatusSuccess,
		KeyId:   req.Name,
		Message: "API key created and broadcasted to gateways successfully",
	})
}

// UpdateAPIKey handles PUT /api/v1/websub-apis/:apiId/api-keys/:keyName
func (h *WebSubAPIKeyHandler) UpdateAPIKey(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiId")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	keyName := c.Param("keyName")
	if keyName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Key name is required"))
		return
	}

	// Verify it's a WebSub API
	if _, err := h.websubAPIService.Get(orgID, apiHandle); err != nil {
		h.handleServiceError(c, err)
		return
	}

	userId := c.GetHeader("x-user-id")

	var req api.UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Warn("Invalid API key update request", "orgId", orgID, "apiHandle", apiHandle, "keyName", keyName, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body: "+err.Error()))
		return
	}

	if req.ApiKey == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API key value is required"))
		return
	}

	// Validate that the name in the request body (if provided) matches the URL path parameter
	if req.Name != nil && *req.Name != "" && *req.Name != keyName {
		h.slogger.Warn("API key name mismatch", "orgId", orgID, "apiHandle", apiHandle, "urlKeyName", keyName, "bodyKeyName", *req.Name)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			fmt.Sprintf("API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *req.Name, keyName)))
		return
	}

	if err := h.apiKeyService.UpdateAPIKey(c.Request.Context(), apiHandle, orgID, keyName, userId, &req); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "No gateway connections available"))
			return
		}
		h.slogger.Error("Failed to update API key for WebSub API", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update API key"))
		return
	}

	h.slogger.Info("Successfully updated API key for WebSub API", "apiHandle", apiHandle, "orgId", orgID, "keyName", keyName)

	c.JSON(http.StatusOK, api.UpdateAPIKeyResponse{
		Status:  api.UpdateAPIKeyResponseStatusSuccess,
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   &keyName,
	})
}

// DeleteAPIKey handles DELETE /api/v1/websub-apis/:apiId/api-keys/:keyName
func (h *WebSubAPIKeyHandler) DeleteAPIKey(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiId")
	keyName := c.Param("keyName")

	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}
	if keyName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Key name is required"))
		return
	}

	userId := c.GetHeader("x-user-id")

	if err := h.apiKeyService.RevokeAPIKey(c.Request.Context(), apiHandle, orgID, keyName, userId); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API key not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "No gateway connections available"))
			return
		}
		h.slogger.Error("Failed to delete API key for WebSub API", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete API key"))
		return
	}

	c.Status(http.StatusNoContent)
}

// handleServiceError maps service errors to HTTP responses
func (h *WebSubAPIKeyHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, constants.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	default:
		h.slogger.Error("WebSub API key service error", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
