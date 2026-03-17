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
	"log/slog"
	"net/http"
	"strings"

	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// APIKeyUserHandler handles listing API keys for a user across artifact types.
type APIKeyUserHandler struct {
	apiKeyUserService *service.APIKeyUserService
	slogger           *slog.Logger
}

// NewAPIKeyUserHandler creates a new APIKeyUserHandler.
func NewAPIKeyUserHandler(apiKeyUserService *service.APIKeyUserService, slogger *slog.Logger) *APIKeyUserHandler {
	return &APIKeyUserHandler{
		apiKeyUserService: apiKeyUserService,
		slogger:           slogger,
	}
}

// ListUserAPIKeys handles GET /api/v1/me/api-keys
func (h *APIKeyUserHandler) ListUserAPIKeys(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	callerUserID := c.GetHeader("x-user-id")

	var types []string
	if typeParam := c.Query("type"); typeParam != "" {
		types = strings.Split(typeParam, ",")
	}

	response, err := h.apiKeyUserService.ListAPIKeysByUser(c.Request.Context(), orgID, callerUserID, types)
	if err != nil {
		h.slogger.Error("Failed to list API keys for user", "orgId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list API keys"))
		return
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers the user API key routes.
func (h *APIKeyUserHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/v1/me/api-keys", h.ListUserAPIKeys)
}
