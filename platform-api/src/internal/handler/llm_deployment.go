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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// LLMProviderDeploymentHandler handles LLM provider deployment endpoints
// using the shared deployment model.
type LLMProviderDeploymentHandler struct {
	deploymentService *service.LLMProviderDeploymentService
	slogger           *slog.Logger
}

// LLMProxyDeploymentHandler handles LLM proxy deployment endpoints
// using the shared deployment model.
type LLMProxyDeploymentHandler struct {
	deploymentService *service.LLMProxyDeploymentService
	slogger           *slog.Logger
}

func NewLLMProviderDeploymentHandler(deploymentService *service.LLMProviderDeploymentService, slogger *slog.Logger) *LLMProviderDeploymentHandler {
	return &LLMProviderDeploymentHandler{deploymentService: deploymentService, slogger: slogger}
}

func NewLLMProxyDeploymentHandler(deploymentService *service.LLMProxyDeploymentService, slogger *slog.Logger) *LLMProxyDeploymentHandler {
	return &LLMProxyDeploymentHandler{deploymentService: deploymentService, slogger: slogger}
}

// DeployLLMProvider handles POST /api/v1/llm-providers/:id/deployments
func (h *LLMProviderDeploymentHandler) DeployLLMProvider(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("id")
	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
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

	deployment, err := h.deploymentService.DeployLLMProvider(providerId, &req, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
				"Base is required (use 'current' or a deploymentId)"))
			return
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Gateway ID is required"))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Referenced template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid input"))
			return
		default:
			h.slogger.Error("Failed to deploy LLM provider", "providerId", providerId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to deploy LLM provider"))
			return
		}
	}

	c.JSON(http.StatusCreated, deployment)
}

// UndeployLLMProviderDeployment handles POST /api/v1/llm-providers/:id/deployments/undeploy
func (h *LLMProviderDeploymentHandler) UndeployLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("id")
	var params api.UndeployLLMProviderDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	deployment, err := h.deploymentService.UndeployLLMProviderDeployment(providerId, deploymentId, gatewayId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
				"No active deployment found for this LLM provider on the gateway"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to undeploy LLM provider", "providerId", providerId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreLLMProviderDeployment handles POST /api/v1/llm-providers/:id/deployments/restore
func (h *LLMProviderDeploymentHandler) RestoreLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("id")
	var params api.RestoreLLMProviderDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	deployment, err := h.deploymentService.RestoreLLMProviderDeployment(providerId, deploymentId, gatewayId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
			h.slogger.Error("Failed to restore LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteLLMProviderDeployment handles DELETE /api/v1/llm-providers/:id/deployments/:deploymentId
func (h *LLMProviderDeploymentHandler) DeleteLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteLLMProviderDeployment(providerId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
			h.slogger.Error("Failed to delete LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
			return
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetLLMProviderDeployment handles GET /api/v1/llm-providers/:id/deployments/:deploymentId
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetLLMProviderDeployment(providerId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// GetLLMProviderDeployments handles GET /api/v1/llm-providers/:id/deployments
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployments(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("id")
	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}

	var params api.GetLLMProviderDeploymentsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	var gatewayId, status *string
	if params.GatewayId != nil {
		value := string(*params.GatewayId)
		gatewayId = &value
	}
	if params.Status != nil {
		value := string(*params.Status)
		status = &value
	}

	deployments, err := h.deploymentService.GetLLMProviderDeployments(providerId, orgId, gatewayId, status)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider deployments", "providerId", providerId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployments"))
			return
		}
	}

	c.JSON(http.StatusOK, deployments)
}

// RegisterRoutes registers all LLM provider deployment-related routes
func (h *LLMProviderDeploymentHandler) RegisterRoutes(r *gin.Engine) {
	providerGroup := r.Group("/api/v1/llm-providers/:id")
	{
		providerGroup.POST("/deployments", h.DeployLLMProvider)
		providerGroup.POST("/deployments/undeploy", h.UndeployLLMProviderDeployment)
		providerGroup.POST("/deployments/restore", h.RestoreLLMProviderDeployment)
		providerGroup.GET("/deployments", h.GetLLMProviderDeployments)
		providerGroup.GET("/deployments/:deploymentId", h.GetLLMProviderDeployment)
		providerGroup.DELETE("/deployments/:deploymentId", h.DeleteLLMProviderDeployment)
	}
}

// DeployLLMProxy handles POST /api/v1/llm-proxies/:id/deployments
func (h *LLMProxyDeploymentHandler) DeployLLMProxy(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
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

	deployment, err := h.deploymentService.DeployLLMProxy(proxyId, &req, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
				"Base is required (use 'current' or a deploymentId)"))
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
			h.slogger.Error("Failed to deploy LLM proxy", "proxyId", proxyId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to deploy LLM proxy"))
			return
		}
	}

	c.JSON(http.StatusCreated, deployment)
}

// UndeployLLMProxyDeployment handles POST /api/v1/llm-proxies/:id/deployments/undeploy
func (h *LLMProxyDeploymentHandler) UndeployLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	var params api.UndeployLLMProxyDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	deployment, err := h.deploymentService.UndeployLLMProxyDeployment(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
				"No active deployment found for this LLM proxy on the gateway"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to undeploy LLM proxy", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreLLMProxyDeployment handles POST /api/v1/llm-proxies/:id/deployments/restore
func (h *LLMProxyDeploymentHandler) RestoreLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	var params api.RestoreLLMProxyDeploymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	deploymentId := params.DeploymentId
	gatewayId := params.GatewayId

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	deployment, err := h.deploymentService.RestoreLLMProxyDeployment(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
			h.slogger.Error("Failed to restore LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteLLMProxyDeployment handles DELETE /api/v1/llm-proxies/:id/deployments/:deploymentId
func (h *LLMProxyDeploymentHandler) DeleteLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteLLMProxyDeployment(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
			h.slogger.Error("Failed to delete LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
			return
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetLLMProxyDeployment handles GET /api/v1/llm-proxies/:id/deployments/:deploymentId
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetLLMProxyDeployment(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		default:
			h.slogger.Error("Failed to get LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// GetLLMProxyDeployments handles GET /api/v1/llm-proxies/:id/deployments
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployments(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("id")
	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}

	var params api.GetLLMProxyDeploymentsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	var gatewayId, status *string
	if params.GatewayId != nil {
		value := string(*params.GatewayId)
		gatewayId = &value
	}
	if params.Status != nil {
		value := string(*params.Status)
		status = &value
	}

	deployments, err := h.deploymentService.GetLLMProxyDeployments(proxyId, orgId, gatewayId, status)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		default:
			h.slogger.Error("Failed to get LLM proxy deployments", "proxyId", proxyId, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployments"))
			return
		}
	}

	c.JSON(http.StatusOK, deployments)
}

// RegisterRoutes registers all LLM proxy deployment-related routes
func (h *LLMProxyDeploymentHandler) RegisterRoutes(r *gin.Engine) {
	proxyGroup := r.Group("/api/v1/llm-proxies/:id")
	{
		proxyGroup.POST("/deployments", h.DeployLLMProxy)
		proxyGroup.POST("/deployments/undeploy", h.UndeployLLMProxyDeployment)
		proxyGroup.POST("/deployments/restore", h.RestoreLLMProxyDeployment)
		proxyGroup.GET("/deployments", h.GetLLMProxyDeployments)
		proxyGroup.GET("/deployments/:deploymentId", h.GetLLMProxyDeployment)
		proxyGroup.DELETE("/deployments/:deploymentId", h.DeleteLLMProxyDeployment)
	}
}
