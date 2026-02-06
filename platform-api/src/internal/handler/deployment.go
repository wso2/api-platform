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
	"log"
	"net/http"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type DeploymentHandler struct {
	deploymentService *service.DeploymentService
}

func NewDeploymentHandler(deploymentService *service.DeploymentService) *DeploymentHandler {
	return &DeploymentHandler{
		deploymentService: deploymentService,
	}
}

// DeployAPI handles POST /api/v1/apis/:apiId/deploy
// Creates a new immutable deployment artifact and deploys it to a gateway
func (h *DeploymentHandler) DeployAPI(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var req dto.DeployAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
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
	if req.GatewayID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}

	deployment, err := h.deploymentService.DeployAPIByHandle(apiId, &req, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrBaseDeploymentNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Base deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNameRequired) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment name is required"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentBaseRequired) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Base is required (use 'current' or a deploymentId)"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentGatewayIDRequired) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Gateway ID is required"))
			return
		}
		if errors.Is(err, constants.ErrAPINoBackendServices) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"API must have at least one backend service attached before deployment"))
			return
		}
		log.Printf("[ERROR] Failed to deploy API: apiId=%s error=%v", apiId, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to deploy API"))
		return
	}

	c.JSON(http.StatusCreated, deployment)
}

// UndeployDeployment handles POST /api/v1/apis/:apiId/deployments/undeploy
// Undeploys an active deployment by changing its status to UNDEPLOYED
func (h *DeploymentHandler) UndeployDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Query("deploymentId")
	gatewayId := c.Query("gatewayId")

	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId query parameter is required"))
		return
	}
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId query parameter is required"))
		return
	}

	deployment, err := h.deploymentService.UndeployDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"No active deployment found for this API on the gateway"))
			return
		}
		if errors.Is(err, constants.ErrGatewayIDMismatch) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		}
		log.Printf("[ERROR] Failed to undeploy: apiId=%s deploymentId=%s gatewayId=%s error=%v", apiId, deploymentId, gatewayId, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// RestoreDeployment handles POST /api/v1/apis/:apiId/deployments/restore
// Restores a previous deployment (ARCHIVED or UNDEPLOYED)
func (h *DeploymentHandler) RestoreDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Query("deploymentId")
	gatewayId := c.Query("gatewayId")

	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId query parameter is required"))
		return
	}
	if gatewayId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId query parameter is required"))
		return
	}

	deployment, err := h.deploymentService.RestoreDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentAlreadyDeployed) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot restore currently deployed deployment"))
			return
		}
		if errors.Is(err, constants.ErrGatewayIDMismatch) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		}
		log.Printf("[ERROR] Failed to restore deployment: apiId=%s deploymentId=%s gatewayId=%s error=%v", apiId, deploymentId, gatewayId, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// DeleteDeployment handles DELETE /api/v1/apis/:apiId/deployments/:deploymentId
// Permanently deletes an undeployed deployment artifact
func (h *DeploymentHandler) DeleteDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")

	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentIsDeployed) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot delete an active deployment - undeploy it first"))
			return
		}
		log.Printf("[ERROR] Failed to delete deployment: apiId=%s deploymentId=%s error=%v", apiId, deploymentId, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetDeployment handles GET /api/v1/apis/:apiId/deployments/:deploymentId
// Retrieves metadata for a specific deployment artifact
func (h *DeploymentHandler) GetDeployment(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	deploymentId := c.Param("deploymentId")

	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		log.Printf("[ERROR] Failed to get deployment: apiId=%s deploymentId=%s error=%v", apiId, deploymentId, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve deployment"))
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// GetDeployments handles GET /api/v1/apis/:apiId/deployments
// Retrieves all deployment records for an API with optional filters
func (h *DeploymentHandler) GetDeployments(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := c.Param("apiId")
	if apiId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// Get optional query parameters
	gatewayId := c.Query("gatewayId")
	status := c.Query("status")

	deployments, err := h.deploymentService.GetDeploymentsByHandle(apiId, gatewayId, status, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidDeploymentStatus) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		}
		log.Printf("[ERROR] Failed to get deployments: apiId=%s error=%v", apiId, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve deployments"))
		return
	}

	c.JSON(http.StatusOK, deployments)
}

// RegisterRoutes registers all deployment-related routes
func (h *DeploymentHandler) RegisterRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api/v1/apis/:apiId")
	{
		apiGroup.POST("/deployments", h.DeployAPI)
		apiGroup.POST("/deployments/undeploy", h.UndeployDeployment)
		apiGroup.POST("/deployments/restore", h.RestoreDeployment)
		apiGroup.GET("/deployments", h.GetDeployments)
		apiGroup.GET("/deployments/:deploymentId", h.GetDeployment)
		apiGroup.DELETE("/deployments/:deploymentId", h.DeleteDeployment)
	}
}
