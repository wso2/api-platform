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
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// LLMProxyAPIKeyHandler handles API key operations for LLM proxies
type LLMProxyAPIKeyHandler struct {
	apiKeyService *service.LLMProxyAPIKeyService
	slogger       *slog.Logger
}

// NewLLMProxyAPIKeyHandler creates a new LLM proxy API key handler
func NewLLMProxyAPIKeyHandler(apiKeyService *service.LLMProxyAPIKeyService, slogger *slog.Logger) *LLMProxyAPIKeyHandler {
	return &LLMProxyAPIKeyHandler{
		apiKeyService: apiKeyService,
		slogger:       slogger,
	}
}

// ListAPIKeys handles GET /api/v1/llm-proxies/{id}/api-keys
func (h *LLMProxyAPIKeyHandler) ListAPIKeys(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyID := c.Param("id")
	if proxyID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}

	callerUserID := c.GetHeader("x-user-id")

	response, err := h.apiKeyService.ListLLMProxyAPIKeys(c.Request.Context(), proxyID, orgID, callerUserID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		}
		h.slogger.Error("Failed to list LLM proxy API keys", "proxyId", proxyID, "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list API keys"))
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteAPIKey handles DELETE /api/v1/llm-proxies/{id}/api-keys/{keyName}
func (h *LLMProxyAPIKeyHandler) DeleteAPIKey(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyID := c.Param("id")
	if proxyID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}

	keyName := c.Param("keyName")
	if keyName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API key name is required"))
		return
	}

	callerUserID := c.GetHeader("x-user-id")

	err := h.apiKeyService.DeleteLLMProxyAPIKey(c.Request.Context(), proxyID, orgID, callerUserID, keyName)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API key not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyForbidden) {
			c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
				"Only the key creator can delete this API key"))
			return
		}
		h.slogger.Error("Failed to delete LLM proxy API key", "proxyId", proxyID, "keyName", keyName, "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete API key"))
		return
	}

	h.slogger.Info("Successfully deleted LLM proxy API key", "proxyId", proxyID, "keyName", keyName, "organizationId", orgID)
	c.Status(http.StatusNoContent)
}

// CreateAPIKey handles POST /api/v1/llm-proxies/{id}/api-keys
func (h *LLMProxyAPIKeyHandler) CreateAPIKey(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyID := c.Param("id")
	if proxyID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}

	var req api.CreateLLMProxyAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("Invalid LLM proxy API key creation request", "proxyId", proxyID, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body"))
		return
	}

	// Validate that at least one of name or displayName is provided
	nameProvided := req.Name != nil && *req.Name != ""
	displayNameProvided := req.DisplayName != nil && *req.DisplayName != ""
	if !nameProvided && !displayNameProvided {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one of 'name' or 'displayName' must be provided"))
		return
	}

	userID := c.GetHeader("x-user-id")

	response, err := h.apiKeyService.CreateLLMProxyAPIKey(c.Request.Context(), proxyID, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"No gateway connections available"))
			return
		}

		h.slogger.Error("Failed to create LLM proxy API key", "proxyId", proxyID, "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API key"))
		return
	}

	h.slogger.Info("Successfully created LLM proxy API key", "proxyId", proxyID, "organizationId", orgID, "keyId", response.KeyId)

	c.JSON(http.StatusCreated, response)
}

// RegisterRoutes registers LLM proxy API key routes with the router
func (h *LLMProxyAPIKeyHandler) RegisterRoutes(r *gin.Engine) {
	apiKeyGroup := r.Group("/api/v1/llm-proxies/:id/api-keys")
	{
		apiKeyGroup.POST("", h.CreateAPIKey)
		apiKeyGroup.GET("", h.ListAPIKeys)
		apiKeyGroup.DELETE("/:keyName", h.DeleteAPIKey)
	}
}
