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

// DeployLLMProvider handles POST /api/v0.9/llm-providers/:providerHandle/deployments
func (h *LLMProviderDeploymentHandler) DeployLLMProvider(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("providerHandle")
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
	if req.GatewayHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayHandle is required"))
		return
	}

	deployment, err := h.deploymentService.DeployLLMProvider(providerId, &req, orgId)
	if err != nil {
		if respondArtifactGuardError(c, err) {
			return
		}
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

// UndeployLLMProviderDeployment handles POST /api/v0.9/llm-providers/:providerHandle/deployments/:deploymentId/undeploy
func (h *LLMProviderDeploymentHandler) UndeployLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("providerHandle")
	deploymentId := c.Param("deploymentId")
	gatewayHandle := c.Query("gatewayHandle")

	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	deployment, err := h.deploymentService.UndeployLLMProviderDeployment(providerId, deploymentId, gatewayHandle, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if respondArtifactGuardError(c, err) {
			return
		}
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
			h.slogger.Error("Failed to undeploy LLM provider", "providerId", providerId, "deploymentId", deploymentId, "gatewayHandle", gatewayHandle, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreLLMProviderDeployment handles POST /api/v0.9/llm-providers/:providerHandle/deployments/:deploymentId/restore
func (h *LLMProviderDeploymentHandler) RestoreLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("providerHandle")
	deploymentId := c.Param("deploymentId")
	gatewayHandle := c.Query("gatewayHandle")

	if providerId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	deployment, err := h.deploymentService.RestoreLLMProviderDeployment(providerId, deploymentId, gatewayHandle, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if respondArtifactGuardError(c, err) {
			return
		}
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
			h.slogger.Error("Failed to restore LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "gatewayHandle", gatewayHandle, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteLLMProviderDeployment handles DELETE /api/v0.9/llm-providers/:providerHandle/deployments/:deploymentId
func (h *LLMProviderDeploymentHandler) DeleteLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("providerHandle")
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

// GetLLMProviderDeployment handles GET /api/v0.9/llm-providers/:providerHandle/deployments/:deploymentId
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("providerHandle")
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

// GetLLMProviderDeployments handles GET /api/v0.9/llm-providers/:providerHandle/deployments
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployments(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := c.Param("providerHandle")
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

	var gatewayHandle, status *string
	if params.GatewayHandle != nil {
		value := string(*params.GatewayHandle)
		gatewayHandle = &value
	}
	if params.Status != nil {
		value := string(*params.Status)
		status = &value
	}

	deployments, err := h.deploymentService.GetLLMProviderDeployments(providerId, orgId, gatewayHandle, status)
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
	providerGroup := r.Group(constants.APIBasePath + "/llm-providers/:providerHandle")
	{
		providerGroup.POST("/deployments", h.DeployLLMProvider)
		providerGroup.POST("/deployments/:deploymentId/undeploy", h.UndeployLLMProviderDeployment)
		providerGroup.POST("/deployments/:deploymentId/restore", h.RestoreLLMProviderDeployment)
		providerGroup.GET("/deployments", h.GetLLMProviderDeployments)
		providerGroup.GET("/deployments/:deploymentId", h.GetLLMProviderDeployment)
		providerGroup.DELETE("/deployments/:deploymentId", h.DeleteLLMProviderDeployment)
	}
}

// DeployLLMProxy handles POST /api/v0.9/llm-proxies/:proxyHandle/deployments
func (h *LLMProxyDeploymentHandler) DeployLLMProxy(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("proxyHandle")
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
	if req.GatewayHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayHandle is required"))
		return
	}

	deployment, err := h.deploymentService.DeployLLMProxy(proxyId, &req, orgId)
	if err != nil {
		if respondArtifactGuardError(c, err) {
			return
		}
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

// UndeployLLMProxyDeployment handles POST /api/v0.9/llm-proxies/:proxyHandle/deployments/:deploymentId/undeploy
func (h *LLMProxyDeploymentHandler) UndeployLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("proxyHandle")
	deploymentId := c.Param("deploymentId")
	gatewayHandle := c.Query("gatewayHandle")

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	deployment, err := h.deploymentService.UndeployLLMProxyDeployment(proxyId, deploymentId, gatewayHandle, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if respondArtifactGuardError(c, err) {
			return
		}
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
			h.slogger.Error("Failed to undeploy LLM proxy", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayHandle", gatewayHandle, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreLLMProxyDeployment handles POST /api/v0.9/llm-proxies/:proxyHandle/deployments/:deploymentId/restore
func (h *LLMProxyDeploymentHandler) RestoreLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("proxyHandle")
	deploymentId := c.Param("deploymentId")
	gatewayHandle := c.Query("gatewayHandle")

	if proxyId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	deployment, err := h.deploymentService.RestoreLLMProxyDeployment(proxyId, deploymentId, gatewayHandle, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if respondArtifactGuardError(c, err) {
			return
		}
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
			h.slogger.Error("Failed to restore LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayHandle", gatewayHandle, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteLLMProxyDeployment handles DELETE /api/v0.9/llm-proxies/:proxyHandle/deployments/:deploymentId
func (h *LLMProxyDeploymentHandler) DeleteLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("proxyHandle")
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

// GetLLMProxyDeployment handles GET /api/v0.9/llm-proxies/:proxyHandle/deployments/:deploymentId
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("proxyHandle")
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

// GetLLMProxyDeployments handles GET /api/v0.9/llm-proxies/:proxyHandle/deployments
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployments(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := c.Param("proxyHandle")
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

	var gatewayHandle, status *string
	if params.GatewayHandle != nil {
		value := string(*params.GatewayHandle)
		gatewayHandle = &value
	}
	if params.Status != nil {
		value := string(*params.Status)
		status = &value
	}

	deployments, err := h.deploymentService.GetLLMProxyDeployments(proxyId, orgId, gatewayHandle, status)
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
	proxyGroup := r.Group(constants.APIBasePath + "/llm-proxies/:proxyHandle")
	{
		proxyGroup.POST("/deployments", h.DeployLLMProxy)
		proxyGroup.POST("/deployments/:deploymentId/undeploy", h.UndeployLLMProxyDeployment)
		proxyGroup.POST("/deployments/:deploymentId/restore", h.RestoreLLMProxyDeployment)
		proxyGroup.GET("/deployments", h.GetLLMProxyDeployments)
		proxyGroup.GET("/deployments/:deploymentId", h.GetLLMProxyDeployment)
		proxyGroup.DELETE("/deployments/:deploymentId", h.DeleteLLMProxyDeployment)
	}
}
