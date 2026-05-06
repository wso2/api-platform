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

// WebSubAPIDeploymentHandler handles deployment routes for WebSub APIs
type WebSubAPIDeploymentHandler struct {
	websubAPIDeploymentService *service.WebSubAPIDeploymentService
	slogger                    *slog.Logger
}

// NewWebSubAPIDeploymentHandler creates a new WebSubAPIDeploymentHandler
func NewWebSubAPIDeploymentHandler(websubAPIDeploymentService *service.WebSubAPIDeploymentService, slogger *slog.Logger) *WebSubAPIDeploymentHandler {
	return &WebSubAPIDeploymentHandler{
		websubAPIDeploymentService: websubAPIDeploymentService,
		slogger:                    slogger,
	}
}

// RegisterRoutes registers WebSub API deployment routes
func (h *WebSubAPIDeploymentHandler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/websub-apis/:apiId")
	{
		g.POST("/deployments", h.DeployWebSubAPI)
		g.POST("/deployments/undeploy", h.UndeployDeployment)
		g.POST("/deployments/restore", h.RestoreDeployment)
		g.GET("/deployments", h.GetDeployments)
		g.GET("/deployments/:deploymentId", h.GetDeployment)
		g.DELETE("/deployments/:deploymentId", h.DeleteDeployment)
	}
}

// DeployWebSubAPI handles POST /api/v1/websub-apis/:apiId/deployments
func (h *WebSubAPIDeploymentHandler) DeployWebSubAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	var req api.DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "name is required"))
		return
	}
	if req.Base == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "base is required"))
		return
	}
	if req.GatewayId == (openapi_types.UUID{}) {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.websubAPIDeploymentService.DeployWebSubAPIByHandle(apiId, &req, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusCreated, deployment)
}

// UndeployDeployment handles POST /api/v1/websub-apis/:apiId/deployments/undeploy
func (h *WebSubAPIDeploymentHandler) UndeployDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	var params api.UndeployDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.websubAPIDeploymentService.UndeployWebSubAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreDeployment handles POST /api/v1/websub-apis/:apiId/deployments/restore
func (h *WebSubAPIDeploymentHandler) RestoreDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	var params api.RestoreDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	deployment, err := h.websubAPIDeploymentService.RestoreWebSubAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// GetDeployments handles GET /api/v1/websub-apis/:apiId/deployments
func (h *WebSubAPIDeploymentHandler) GetDeployments(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	var params api.GetDeploymentsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	var gatewayId, status string
	if params.GatewayId != nil {
		gatewayId = string(*params.GatewayId)
	}
	if params.Status != nil {
		status = string(*params.Status)
	}

	deployments, err := h.websubAPIDeploymentService.GetWebSubAPIDeploymentsByHandle(apiId, gatewayId, status, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployments)
}

// GetDeployment handles GET /api/v1/websub-apis/:apiId/deployments/:deploymentId
func (h *WebSubAPIDeploymentHandler) GetDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")

	deployment, err := h.websubAPIDeploymentService.GetWebSubAPIDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteDeployment handles DELETE /api/v1/websub-apis/:apiId/deployments/:deploymentId
func (h *WebSubAPIDeploymentHandler) DeleteDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")

	if err := h.websubAPIDeploymentService.DeleteWebSubAPIDeploymentByHandle(apiId, deploymentId, orgId); err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *WebSubAPIDeploymentHandler) handleDeploymentError(c *gin.Context, err error, apiId string) {
	switch {
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	case errors.Is(err, constants.ErrGatewayNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Gateway not found"))
	case errors.Is(err, constants.ErrDeploymentNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Deployment not found"))
	case errors.Is(err, constants.ErrBaseDeploymentNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Base deployment not found"))
	case errors.Is(err, constants.ErrDeploymentNotActive):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "No active deployment found for this API on the gateway"))
	case errors.Is(err, constants.ErrDeploymentIsDeployed):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Cannot delete an active deployment - undeploy it first"))
	case errors.Is(err, constants.ErrDeploymentAlreadyDeployed):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Cannot restore currently deployed deployment"))
	case errors.Is(err, constants.ErrInvalidDeploymentRestoreState):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Deployment cannot be restored: only ARCHIVED or UNDEPLOYED deployments are eligible"))
	case errors.Is(err, constants.ErrGatewayIDMismatch):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Deployment is bound to a different gateway"))
	case errors.Is(err, constants.ErrAPINoBackendServices):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API must have at least one backend service configured"))
	default:
		h.slogger.Error("WebSub API deployment error", "apiId", apiId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
