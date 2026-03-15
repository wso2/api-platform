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

	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// LLMAPIKeyUserHandler handles listing LLM API keys for a user across providers and proxies.
type LLMAPIKeyUserHandler struct {
	apiKeyUserService *service.LLMAPIKeyUserService
	slogger           *slog.Logger
}

// NewLLMAPIKeyUserHandler creates a new LLMAPIKeyUserHandler.
func NewLLMAPIKeyUserHandler(apiKeyUserService *service.LLMAPIKeyUserService, slogger *slog.Logger) *LLMAPIKeyUserHandler {
	return &LLMAPIKeyUserHandler{
		apiKeyUserService: apiKeyUserService,
		slogger:           slogger,
	}
}

// ListUserLLMAPIKeys handles GET /api/v1/user/llm-api-keys
func (h *LLMAPIKeyUserHandler) ListUserLLMAPIKeys(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	callerUserID := c.GetHeader("x-user-id")

	response, err := h.apiKeyUserService.ListLLMAPIKeysByUser(c.Request.Context(), orgID, callerUserID)
	if err != nil {
		h.slogger.Error("Failed to list LLM API keys for user", "orgId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list API keys"))
		return
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers the user LLM API key routes.
func (h *LLMAPIKeyUserHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/v1/user/llm-api-keys", h.ListUserLLMAPIKeys)
}
