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
	"strconv"

	"platform-api/src/api"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type MCPProxyHandler struct {
	slogger *slog.Logger
}

func NewMCPProxyHandler(slogger *slog.Logger) *MCPProxyHandler {
	return &MCPProxyHandler{
		slogger: slogger,
	}
}

func (h *MCPProxyHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		v1.POST("/mcp-proxies", h.CreateMCPProxy)
		v1.GET("/mcp-proxies", h.ListMCPProxies)
		v1.GET("/mcp-proxies/:id", h.GetMCPProxy)
		v1.PUT("/mcp-proxies/:id", h.UpdateMCPProxy)
		v1.DELETE("/mcp-proxies/:id", h.DeleteMCPProxy)
	}
}

// CreateMCPProxy handles POST /api/v1/mcp-proxies
func (h *MCPProxyHandler) CreateMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.MCPProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, _ := middleware.GetUsernameFromContext(c)
	h.slogger.Info("CreateMCPProxy called", "organizationId", orgID, "createdBy", createdBy, "mcpProxyId", req.Id)

	// TODO: Implement service layer call when available
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented", "MCP proxy creation is not yet implemented"))
}

// ListMCPProxies handles GET /api/v1/mcp-proxies
func (h *MCPProxyHandler) ListMCPProxies(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := c.Query("projectId")
	var projectIDPtr *string
	if projectID != "" {
		projectIDPtr = &projectID
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		h.slogger.Warn("Invalid limit query parameter, defaulting to 20", "input", limitStr)
		limit = 20
	}
	if limit > 100 {
		h.slogger.Warn("Limit query parameter exceeds maximum, capping to 100", "input", limit)
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	h.slogger.Info("ListMCPProxies called", "organizationId", orgID, "projectId", projectIDPtr, "limit", limit, "offset", offset)

	// TODO: Implement service layer call when available
	// For now, return empty list
	resp := api.MCPProxyListResponse{
		Count:      0,
		List:       []api.MCPProxyListItem{},
		Pagination: api.Pagination{Limit: limit, Offset: offset},
	}
	c.JSON(http.StatusOK, resp)
}

// GetMCPProxy handles GET /api/v1/mcp-proxies/:id
func (h *MCPProxyHandler) GetMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	h.slogger.Info("GetMCPProxy called", "organizationId", orgID, "mcpProxyId", id)

	// TODO: Implement service layer call when available
	// For now, return not found
	c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "MCP proxy not found"))
}

// UpdateMCPProxy handles PUT /api/v1/mcp-proxies/:id
func (h *MCPProxyHandler) UpdateMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	var req api.MCPProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	h.slogger.Info("UpdateMCPProxy called", "organizationId", orgID, "mcpProxyId", id)

	// TODO: Implement service layer call when available
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented", "MCP proxy update is not yet implemented"))
}

// DeleteMCPProxy handles DELETE /api/v1/mcp-proxies/:id
func (h *MCPProxyHandler) DeleteMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	h.slogger.Info("DeleteMCPProxy called", "organizationId", orgID, "mcpProxyId", id)

	// TODO: Implement service layer call when available
	// For now, return not found (simulating not found for delete)
	c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "MCP proxy not found"))
}
