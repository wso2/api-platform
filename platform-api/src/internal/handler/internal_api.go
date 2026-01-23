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
	"net/http"

	"github.com/gin-gonic/gin"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
)

type introspectTokenRequest struct {
	Token string `form:"token" binding:"required"`
}

type introspectTokenResponse struct {
	Active bool     `json:"active"`
	Scope  string   `json:"scope,omitempty"`
	Aud    []string `json:"aud"`
}

// InternalAPIHandler handles internal API endpoints.
type InternalAPIHandler struct {
	gatewayService *service.GatewayService
}

// NewInternalAPIHandler creates a new InternalAPIHandler.
func NewInternalAPIHandler(gatewayService *service.GatewayService) *InternalAPIHandler {
	return &InternalAPIHandler{
		gatewayService: gatewayService,
	}
}

// IntrospectToken handles POST /internal/v1/token/introspect.
func (h *InternalAPIHandler) IntrospectToken(c *gin.Context) {
	var req introspectTokenRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(
			http.StatusBadRequest,
			"Bad Request",
			"Token is required",
		))
		return
	}

	gateway, err := h.gatewayService.VerifyToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(
			http.StatusUnauthorized,
			"Unauthorized",
			"Invalid or missing token",
		))
		return
	}

	c.JSON(http.StatusOK, introspectTokenResponse{
		Active: true,
		Aud:    []string{gateway.OrganizationID + "/" + gateway.Name},
	})
}

// RegisterRoutes registers internal API routes with the router.
func (h *InternalAPIHandler) RegisterRoutes(r *gin.Engine) {
	internalGroup := r.Group("/internal/v1")
	{
		internalGroup.POST("/token/introspect", h.IntrospectToken)
	}
}
