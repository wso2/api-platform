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
	"strconv"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type MCPProxyHandler struct {
	service *service.MCPProxyService
	slogger *slog.Logger
}

func NewMCPProxyHandler(service *service.MCPProxyService, slogger *slog.Logger) *MCPProxyHandler {
	return &MCPProxyHandler{
		service: service,
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
		v1.POST("/mcp-proxies/fetch-server-info", h.FetchMCPProxyServerInfo)
	}
}

// CreateMCPProxy handles POST /api/v1/mcp-proxies
func (h *MCPProxyHandler) CreateMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.MCPProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("MCP request validation failed", "org_id", orgID, "reason", "Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, _ := middleware.GetUsernameFromContext(c)

	if req.ProjectId == nil {
		h.slogger.Debug("No project ID provided for MCP proxy, proceeding without project association", "mcpProxyId", req.Id)
	}

	resp, err := h.service.Create(orgID, createdBy, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListMCPProxies handles GET /api/v1/mcp-proxies
func (h *MCPProxyHandler) ListMCPProxies(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
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

	var resp *api.MCPProxyListResponse
	if projectIDPtr != nil {
		resp, err = h.service.ListByProject(orgID, *projectIDPtr, limit, offset)
	} else {
		resp, err = h.service.List(orgID, limit, offset)
	}

	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetMCPProxy handles GET /api/v1/mcp-proxies/:id
func (h *MCPProxyHandler) GetMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	resp, err := h.service.Get(orgID, id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateMCPProxy handles PUT /api/v1/mcp-proxies/:id
func (h *MCPProxyHandler) UpdateMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	var req api.MCPProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("MCP request validation failed", "org_id", orgID, "proxy_id", id, "reason", "Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.service.Update(orgID, id, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteMCPProxy handles DELETE /api/v1/mcp-proxies/:id
func (h *MCPProxyHandler) DeleteMCPProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	if err := h.service.Delete(orgID, id); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// FetchMCPProxyServerInfo handles POST /api/v1/mcp-proxies/fetch-server-info
func (h *MCPProxyHandler) FetchMCPProxyServerInfo(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		h.slogger.Error("MCP request validation failed", "reason", "Organization claim not found in token")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.MCPServerInfoFetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("MCP request validation failed", "org_id", orgID, "reason", "Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.service.FetchServerInfo(orgID, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrInvalidURL):
			h.slogger.Error("Invalid URL provided for MCP server info fetch", "error", err, "inputUrl", req.Url)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
			return
		case errors.Is(err, constants.ErrURLUnreachable):
			h.slogger.Error("MCP server URL is unreachable", "error", err, "inputUrl", req.Url)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", strings.Split(err.Error(), ":")[0]))
			return
		case errors.Is(err, constants.ErrMCPServerUnauthorized):
			h.slogger.Error("MCP server returned 401 Unauthorized", "error", err, "inputUrl", req.Url)
			c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(400, "Bad Request", "MCP server returned 401 Unauthorized. Check the provided credentials."))
			return
		default:
			h.handleServiceError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

// handleServiceError maps service errors to HTTP responses
func (h *MCPProxyHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, constants.ErrInvalidInput):
		h.slogger.Error("MCP request validation failed", "reason", err.Error())
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrMCPProxyNotFound):
		h.slogger.Error("MCP proxy not found", "reason", err.Error())
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "MCP proxy not found"))
	case errors.Is(err, constants.ErrMCPProxyExists):
		h.slogger.Error("MCP proxy conflict", "reason", err.Error())
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "MCP proxy with this ID already exists"))
	case errors.Is(err, constants.ErrProjectNotFound):
		h.slogger.Error("MCP request validation failed", "reason", "Project not found")
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project not found"))
	case errors.Is(err, constants.ErrMCPProxyLimitReached):
		h.slogger.Error("MCP proxy limit reached", "reason", err.Error())
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "MCP proxy limit reached for the organization"))
	default:
		h.slogger.Error("MCP proxy service error", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
