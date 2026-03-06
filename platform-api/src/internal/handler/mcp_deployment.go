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
	"log/slog"
	"net/http"

	"platform-api/src/internal/middleware"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// MCPProxyDeploymentHandler handles MCP proxy deployment endpoints
type MCPProxyDeploymentHandler struct {
	slogger *slog.Logger
}

// NewMCPProxyDeploymentHandler creates a new MCP proxy deployment handler
func NewMCPProxyDeploymentHandler(slogger *slog.Logger) *MCPProxyDeploymentHandler {
	return &MCPProxyDeploymentHandler{
		slogger: slogger,
	}
}

// RegisterRoutes registers all MCP proxy deployment-related routes
func (h *MCPProxyDeploymentHandler) RegisterRoutes(r *gin.Engine) {
	proxyGroup := r.Group("/api/v1/mcp-proxies/:id")
	{
		proxyGroup.POST("/deployments", h.DeployMCPProxy)
		proxyGroup.POST("/deployments/undeploy", h.UndeployMCPProxyDeployment)
		proxyGroup.POST("/deployments/restore", h.RestoreMCPProxyDeployment)
		proxyGroup.GET("/deployments", h.GetMCPProxyDeployments)
		proxyGroup.GET("/deployments/:deploymentId", h.GetMCPProxyDeployment)
		proxyGroup.DELETE("/deployments/:deploymentId", h.DeleteMCPProxyDeployment)
	}
}

// DeployMCPProxy handles POST /api/v1/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) DeployMCPProxy(c *gin.Context) {
	_, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented",
		"MCP proxy deployment is not implemented yet"))
}

// UndeployMCPProxyDeployment handles POST /api/v1/mcp-proxies/:id/deployments/undeploy
func (h *MCPProxyDeploymentHandler) UndeployMCPProxyDeployment(c *gin.Context) {
	_, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented",
		"MCP proxy undeployment is not implemented yet"))
}

// RestoreMCPProxyDeployment handles POST /api/v1/mcp-proxies/:id/deployments/restore
func (h *MCPProxyDeploymentHandler) RestoreMCPProxyDeployment(c *gin.Context) {
	_, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented",
		"MCP proxy deployment restore is not implemented yet"))
}

// DeleteMCPProxyDeployment handles DELETE /api/v1/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) DeleteMCPProxyDeployment(c *gin.Context) {
	_, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented",
		"MCP proxy deployment deletion is not implemented yet"))
}

// GetMCPProxyDeployment handles GET /api/v1/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployment(c *gin.Context) {
	_, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented",
		"MCP proxy deployment retrieval is not implemented yet"))
}

// GetMCPProxyDeployments handles GET /api/v1/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployments(c *gin.Context) {
	_, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	c.JSON(http.StatusNotImplemented, utils.NewErrorResponse(501, "Not Implemented",
		"MCP proxy deployments list is not implemented yet"))
}
