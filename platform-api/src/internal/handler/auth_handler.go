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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
	slogger     *slog.Logger
}

func NewAuthHandler(authService *service.AuthService, slogger *slog.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, slogger: slogger}
}

type tokenExchangeRequest struct {
	OrgID string `json:"orgId" binding:"required"`
}

// ExchangeToken handles POST /api/v1/auth/token.
// The caller must be authenticated (JWT middleware already ran). The handler
// validates that the user is a member of the requested org and issues a new
// platform-signed JWT with that org's UUID in the organization claim.
func (h *AuthHandler) ExchangeToken(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists || userID == "" {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "User ID not found in token"))
		return
	}

	var req tokenExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "orgId is required"))
		return
	}

	email, _ := middleware.GetEmailFromContext(c)
	username, _ := middleware.GetUsernameFromContext(c)
	scope, _ := middleware.GetScopeFromContext(c)
	firstName, _ := middleware.GetFirstNameFromContext(c)
	lastName, _ := middleware.GetLastNameFromContext(c)
	jwtOrgIDs, _ := middleware.GetOrganizationsFromContext(c)
	jwtOrgID, _ := middleware.GetOrganizationFromContext(c)

	resp, err := h.authService.ExchangeToken(userID, email, username, firstName, lastName, req.OrgID, scope, jwtOrgIDs, jwtOrgID)
	if err != nil {
		if errors.Is(err, constants.ErrTokenExchangeDisabled) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "Token exchange is disabled on this server"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", "Access denied for the requested organization"))
			return
		}
		h.slogger.Error("Token exchange failed", "userID", userID, "orgID", req.OrgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Token exchange failed"))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) RegisterRoutes(r *gin.Engine) {
	r.POST("/api/v1/auth/token", h.ExchangeToken)
}
