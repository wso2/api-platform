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
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// MCPProxyDeploymentHandler handles MCP proxy deployment endpoints
type MCPProxyDeploymentHandler struct {
	deploymentService *service.MCPDeploymentService
	slogger           *slog.Logger
}

// NewMCPProxyDeploymentHandler creates a new MCP proxy deployment handler
func NewMCPProxyDeploymentHandler(deploymentService *service.MCPDeploymentService, slogger *slog.Logger) *MCPProxyDeploymentHandler {
	return &MCPProxyDeploymentHandler{
		deploymentService: deploymentService,
		slogger:           slogger,
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
	orgId, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	var req api.DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"name is required"))
		return
	}
	if req.Base == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"base is required (use 'current' or a deploymentId)"))
		return
	}
	if req.GatewayId == (openapi_types.UUID{}) {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}

	deployment, err := h.deploymentService.DeployMCPProxyByHandle(proxyId, &req, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrGatewayNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		case errors.Is(err, constants.ErrBaseDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Base deployment not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNameRequired):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment name is required"))
			return
		case errors.Is(err, constants.ErrDeploymentBaseRequired):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Base is required"))
			return
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Gateway ID is required"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid input"))
			return
		default:
			h.slogger.Error("Failed to deploy MCP proxy", "proxyId", proxyId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to deploy MCP proxy"))
			return
		}
	}

	c.JSON(http.StatusCreated, deployment)
}

// UndeployMCPProxyDeployment handles POST /api/v1/mcp-proxies/:id/deployments/undeploy
func (h *MCPProxyDeploymentHandler) UndeployMCPProxyDeployment(c *gin.Context) {
	orgId, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	var params api.UndeployMCPProxyDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	deployment, err := h.deploymentService.UndeployDeploymentByHandle(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		case errors.Is(err, constants.ErrGatewayNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotActive):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"No active deployment found for this MCP proxy on the gateway"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to undeploy MCP proxy", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreMCPProxyDeployment handles POST /api/v1/mcp-proxies/:id/deployments/restore
func (h *MCPProxyDeploymentHandler) RestoreMCPProxyDeployment(c *gin.Context) {
	orgId, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	var params api.RestoreMCPProxyDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	deployment, err := h.deploymentService.RestoreMCPDeploymentByHandle(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		case errors.Is(err, constants.ErrGatewayNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		case errors.Is(err, constants.ErrDeploymentAlreadyDeployed):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot restore currently deployed deployment"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to restore MCP proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteMCPProxyDeployment handles DELETE /api/v1/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) DeleteMCPProxyDeployment(c *gin.Context) {
	orgId, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteDeploymentByHandle(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		case errors.Is(err, constants.ErrDeploymentIsDeployed):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot delete an active deployment - undeploy it first"))
			return
		default:
			h.slogger.Error("Failed to delete MCP proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// GetMCPProxyDeployment handles GET /api/v1/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployment(c *gin.Context) {
	orgId, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetDeploymentByHandle(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		default:
			h.slogger.Error("Failed to get MCP proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// GetMCPProxyDeployments handles GET /api/v1/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployments(c *gin.Context) {
	orgId, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	var params api.GetMCPProxyDeploymentsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	gatewayVal := ""
	if params.GatewayId != nil {
		gatewayVal = string(*params.GatewayId)
	}

	statusVal := ""
	if params.Status != nil {
		statusVal = string(*params.Status)
	}

	deployments, err := h.deploymentService.GetDeploymentsByHandle(proxyId, gatewayVal, statusVal, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		default:
			h.slogger.Error("Failed to get MCP proxy deployments", "proxyId", proxyId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployments"))
			return
		}
	}

	c.JSON(http.StatusOK, deployments)
}
