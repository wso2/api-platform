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

// WebBrokerAPIDeploymentHandler handles deployment routes for WebBroker APIs
type WebBrokerAPIDeploymentHandler struct {
	webbrokerAPIDeploymentService *service.WebBrokerAPIDeploymentService
	slogger                       *slog.Logger
}

// NewWebBrokerAPIDeploymentHandler creates a new WebBrokerAPIDeploymentHandler
func NewWebBrokerAPIDeploymentHandler(webbrokerAPIDeploymentService *service.WebBrokerAPIDeploymentService, slogger *slog.Logger) *WebBrokerAPIDeploymentHandler {
	return &WebBrokerAPIDeploymentHandler{
		webbrokerAPIDeploymentService: webbrokerAPIDeploymentService,
		slogger:                       slogger,
	}
}

// RegisterRoutes registers WebBroker API deployment routes
func (h *WebBrokerAPIDeploymentHandler) RegisterRoutes(r *gin.Engine) {
	g := r.Group(constants.APIBasePath + "/webbroker-apis/:apiId")
	{
		g.POST("/deployments", h.DeployWebBrokerAPI)
		g.POST("/deployments/:deploymentId/undeploy", h.UndeployDeployment)
		g.POST("/deployments/:deploymentId/restore", h.RestoreDeployment)
		g.GET("/deployments", h.GetDeployments)
		g.GET("/deployments/:deploymentId", h.GetDeployment)
		g.DELETE("/deployments/:deploymentId", h.DeleteDeployment)
	}
}

// DeployWebBrokerAPI handles POST /api/v0.9/webbroker-apis/:apiId/deployments
func (h *WebBrokerAPIDeploymentHandler) DeployWebBrokerAPI(c *gin.Context) {
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

	deployment, err := h.webbrokerAPIDeploymentService.DeployWebBrokerAPIByHandle(apiId, &req, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusCreated, deployment)
}

// UndeployDeployment handles POST /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId/undeploy
func (h *WebBrokerAPIDeploymentHandler) UndeployDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")
	gatewayId := c.Query("gatewayId")
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.webbrokerAPIDeploymentService.UndeployWebBrokerAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreDeployment handles POST /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId/restore
func (h *WebBrokerAPIDeploymentHandler) RestoreDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")
	gatewayId := c.Query("gatewayId")
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.webbrokerAPIDeploymentService.RestoreWebBrokerAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// GetDeployments handles GET /api/v0.9/webbroker-apis/:apiId/deployments
func (h *WebBrokerAPIDeploymentHandler) GetDeployments(c *gin.Context) {
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

	deployments, err := h.webbrokerAPIDeploymentService.GetWebBrokerAPIDeploymentsByHandle(apiId, gatewayId, status, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployments)
}

// GetDeployment handles GET /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId
func (h *WebBrokerAPIDeploymentHandler) GetDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")

	deployment, err := h.webbrokerAPIDeploymentService.GetWebBrokerAPIDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteDeployment handles DELETE /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId
func (h *WebBrokerAPIDeploymentHandler) DeleteDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")

	if err := h.webbrokerAPIDeploymentService.DeleteWebBrokerAPIDeploymentByHandle(apiId, deploymentId, orgId); err != nil {
		h.handleDeploymentError(c, err, apiId)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *WebBrokerAPIDeploymentHandler) handleDeploymentError(c *gin.Context, err error, apiId string) {
	switch {
	case errors.Is(err, constants.ErrWebBrokerAPINotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
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
		h.slogger.Error("WebBroker API deployment error", "apiId", apiId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
