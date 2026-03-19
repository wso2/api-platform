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
	"fmt"
	"log/slog"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
	"time"

	"platform-api/src/internal/service"

	"github.com/gin-gonic/gin"
)

type GatewayInternalAPIHandler struct {
	gatewayService         *service.GatewayService
	gatewayInternalService *service.GatewayInternalAPIService
	slogger                *slog.Logger
}

func NewGatewayInternalAPIHandler(gatewayService *service.GatewayService,
	gatewayInternalService *service.GatewayInternalAPIService, slogger *slog.Logger) *GatewayInternalAPIHandler {
	return &GatewayInternalAPIHandler{
		gatewayService:         gatewayService,
		gatewayInternalService: gatewayInternalService,
		slogger:                slogger,
	}
}

// authenticateGateway validates the API key and returns the authenticated gateway.
func (h *GatewayInternalAPIHandler) authenticateGateway(apiKey string) (*model.Gateway, error) {
	if apiKey == "" {
		return nil, constants.ErrMissingAPIKey
	}
	return h.gatewayService.VerifyToken(apiKey)
}

// authenticateRequest extracts the API key from headers and authenticates the gateway.
func (h *GatewayInternalAPIHandler) authenticateRequest(c *gin.Context) (orgID, gatewayID string, ok bool) {
	clientIP := c.ClientIP()
	apiKey := c.GetHeader("api-key")

	gateway, err := h.authenticateGateway(apiKey)
	if err != nil {
		if errors.Is(err, constants.ErrMissingAPIKey) {
			h.slogger.Warn("Unauthorized access attempt - Missing API key", "clientIP", clientIP)
			c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
				"API key is required. Provide 'api-key' header."))
		} else if errors.Is(err, constants.ErrInvalidAPIToken) {
			h.slogger.Warn("Authentication failed - Invalid API key", "clientIP", clientIP)
			c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
				"Invalid or expired API key"))
		} else {
			h.slogger.Error("Authentication failed", "clientIP", clientIP, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Error while validating API key"))
		}
		return "", "", false
	}
	return gateway.OrganizationID, gateway.ID, true
}

// GetAPI handles GET /api/internal/v1/apis/:apiId
func (h *GatewayInternalAPIHandler) GetAPI(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	api, err := h.gatewayInternalService.GetActiveDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this API on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API"))
		return
	}

	// Create ZIP file from API YAML file
	zipData, err := utils.CreateAPIYamlZip(api)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "apiID", apiID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API package"))
		return
	}

	// Set headers for ZIP file download
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"api-%s.zip\"", apiID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	c.Data(http.StatusOK, "application/zip", zipData)
}

// CreateGatewayDeployment handles POST /api/internal/v1/apis/{apiId}/gateway-deployments
func (h *GatewayInternalAPIHandler) CreateGatewayDeployment(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	// Extract API ID from path parameter
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	// Extract optional deployment ID from query parameter
	deploymentID := c.Query("deploymentId")
	var deploymentIDPtr *string
	if deploymentID != "" {
		deploymentIDPtr = &deploymentID
	}

	// Parse and validate request body
	var notification dto.DeploymentNotification
	if err := c.ShouldBindJSON(&notification); err != nil {
		clientIP := c.ClientIP()
		h.slogger.Warn("Invalid request body", "clientIP", clientIP, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	response, err := h.gatewayInternalService.CreateGatewayDeployment(
		apiID, orgID, gatewayID, notification, deploymentIDPtr)
	if err != nil {
		if errors.Is(err, constants.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid input data"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to create gateway API deployment", "apiID", apiID, "gatewayID", gatewayID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API deployment"))
		return
	}

	h.slogger.Info("Successfully created gateway API deployment", "apiID", apiID, "gatewayID", gatewayID, "created", response.Created)

	// Return success response
	c.JSON(http.StatusCreated, map[string]interface{}{
		"message": response.Message,
	})
}

// GetLLMProvider handles GET /api/internal/v1/llm-providers/:providerId
func (h *GatewayInternalAPIHandler) GetLLMProvider(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	providerID := c.Param("providerId")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Provider ID is required"))
		return
	}

	provider, err := h.gatewayInternalService.GetActiveLLMProviderDeploymentByGateway(providerID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this LLM provider on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrLLMProviderNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get LLM provider"))
		return
	}

	// Create ZIP file from LLM provider YAML file
	zipData, err := utils.CreateLLMProviderYamlZip(provider)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "providerID", providerID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create LLM provider package"))
		return
	}

	// Set headers for ZIP file download
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"llm-provider-%s.zip\"", providerID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	c.Data(http.StatusOK, "application/zip", zipData)
}

// GetLLMProxy handles GET /api/internal/v1/llm-proxies/:proxyId
func (h *GatewayInternalAPIHandler) GetLLMProxy(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	proxyID := c.Param("proxyId")
	if proxyID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Proxy ID is required"))
		return
	}

	proxy, err := h.gatewayInternalService.GetActiveLLMProxyDeploymentByGateway(proxyID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this LLM proxy on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrLLMProxyNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get LLM proxy"))
		return
	}

	// Create ZIP file from LLM proxy YAML file
	zipData, err := utils.CreateLLMProxyYamlZip(proxy)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "proxyID", proxyID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create LLM proxy package"))
		return
	}

	// Set headers for ZIP file download
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"llm-proxy-%s.zip\"", proxyID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	c.Data(http.StatusOK, "application/zip", zipData)
}

// GetGatewayDeployments handles GET /api/internal/v1/deployments
// Returns the list of deployments that should be active on a gateway for startup sync
func (h *GatewayInternalAPIHandler) GetGatewayDeployments(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	// Parse optional "since" query parameter for incremental sync
	var since *time.Time
	sinceStr := c.Query("since")
	if sinceStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid 'since' parameter. Expected ISO 8601 format (e.g., 2026-03-04T10:00:00Z)"))
			return
		}
		since = &parsedTime
	}

	deployments, err := h.gatewayInternalService.GetDeploymentsByGateway(orgID, gatewayID, since)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		h.slogger.Error("Failed to get gateway deployments", "gatewayID", gatewayID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get deployments"))
		return
	}

	c.JSON(http.StatusOK, deployments)
}

// BatchFetchDeployments handles POST /api/internal/v1/deployments/fetch-batch
// Fetches multiple deployment artifacts in a single request for gateway startup sync
func (h *GatewayInternalAPIHandler) BatchFetchDeployments(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	// Enforce Accept header - only application/x-tar+gzip is supported
	if accept := c.GetHeader("Accept"); accept != "application/x-tar+gzip" {
		c.JSON(http.StatusNotAcceptable, utils.NewErrorResponse(406, "Not Acceptable",
			"This endpoint only supports Accept: application/x-tar+gzip"))
		return
	}

	// Parse request body
	var req dto.DeploymentsBatchFetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	if len(req.DeploymentIDs) == 0 {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one deployment ID is required"))
		return
	}

	// Fetch deployment content
	contentMap, err := h.gatewayInternalService.GetDeploymentContentBatch(orgID, gatewayID, req.DeploymentIDs)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		h.slogger.Error("Failed to get deployment content batch", "gatewayID", gatewayID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get deployment content"))
		return
	}

	// Create TAR.GZ archive from deployment content
	tarGzData, err := utils.CreateBatchDeploymentTarGz(contentMap)
	if err != nil {
		h.slogger.Error("Failed to create batch TAR.GZ archive", "gatewayID", gatewayID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create deployment package"))
		return
	}

	// Set headers for TAR.GZ download
	c.Header("Content-Type", "application/x-tar+gzip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"deployments-batch-%s.tar.gz\"", gatewayID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(tarGzData)))

	// Return TAR.GZ archive
	c.Data(http.StatusOK, "application/x-tar+gzip", tarGzData)
}

// GetSubscriptions handles GET /api/internal/v1/apis/:apiId/subscriptions
func (h *GatewayInternalAPIHandler) GetSubscriptions(c *gin.Context) {
	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	apiID := c.Param("apiId")
	if apiID == "" {
		h.slogger.Error("API ID is required for subscriptions request",
			"clientIP", c.ClientIP(),
			"organizationId", orgID,
			"apiId", apiID)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	if err := h.gatewayInternalService.IsAPIDeployedOnGateway(apiID, gatewayID, orgID); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found when listing subscriptions",
				"apiId", apiID,
				"organizationId", orgID,
				"gatewayId", gatewayID,
				"error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			h.slogger.Error("Subscription list denied - API not deployed on gateway",
				"apiId", apiID,
				"organizationId", orgID,
				"gatewayId", gatewayID)
			c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
				"API is not deployed on this gateway"))
			return
		}
		h.slogger.Error("Failed to verify API deployment for subscriptions",
			"apiId", apiID,
			"organizationId", orgID,
			"gatewayId", gatewayID,
			"error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to verify API deployment"))
		return
	}

	subs, err := h.gatewayInternalService.ListSubscriptionsForAPI(apiID, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found when listing subscriptions",
				"apiId", apiID,
				"organizationId", orgID,
				"error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to list subscriptions for API",
			"apiId", apiID,
			"organizationId", orgID,
			"error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get subscriptions"))
		return
	}

	c.JSON(http.StatusOK, subs)
}

// GetSubscriptionPlans handles GET /api/internal/v1/subscription-plans
func (h *GatewayInternalAPIHandler) GetSubscriptionPlans(c *gin.Context) {
	orgID, _, ok := h.authenticateRequest(c)
	if !ok {
		return
	}

	plans, err := h.gatewayInternalService.ListSubscriptionPlansForOrg(orgID)
	if err != nil {
		h.slogger.Error("Failed to list subscription plans",
			"organizationId", orgID,
			"error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get subscription plans"))
		return
	}

	c.JSON(http.StatusOK, plans)
}

// GetMCPProxy handles GET /api/internal/v1/mcp-proxies/:proxyId
func (h *GatewayInternalAPIHandler) GetMCPProxy(c *gin.Context) {

	orgID, gatewayID, ok := h.authenticateRequest(c)
	if !ok {
		return
	}
	proxyID := c.Param("proxyId")
	if proxyID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Proxy ID is required"))
		return
	}

	proxy, err := h.gatewayInternalService.GetActiveMCPProxyDeploymentByGateway(proxyID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			h.slogger.Error("No active deployment found for MCP proxy", "clientIP", c.ClientIP(), "proxyID", proxyID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this MCP proxy on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrMCPProxyNotFound) {
			h.slogger.Error("MCP proxy not found", "clientIP", c.ClientIP(), "proxyID", proxyID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		}
		h.slogger.Error("Failed to get MCP proxy", "clientIP", c.ClientIP(), "proxyID", proxyID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get MCP proxy"))
		return
	}

	// Create ZIP file from MCP proxy YAML file
	zipData, err := utils.CreateMCPProxyYamlZip(proxy)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "proxyID", proxyID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create MCP proxy package"))
		return
	}

	// Set headers for ZIP file download
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"mcp-proxy-%s.zip\"", proxyID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	c.Data(http.StatusOK, "application/zip", zipData)
}

func (h *GatewayInternalAPIHandler) RegisterRoutes(r *gin.Engine) {
	orgGroup := r.Group("/api/internal/v1/apis")
	{
		orgGroup.GET("/:apiId", h.GetAPI)
		orgGroup.POST("/:apiId/gateway-deployments", h.CreateGatewayDeployment)
		orgGroup.GET("/:apiId/subscriptions", h.GetSubscriptions)
	}

	subPlanGroup := r.Group("/api/internal/v1")
	{
		subPlanGroup.GET("/subscription-plans", h.GetSubscriptionPlans)
	}

	llmGroup := r.Group("/api/internal/v1/llm-providers")
	{
		llmGroup.GET("/:providerId", h.GetLLMProvider)
	}

	llmProxyGroup := r.Group("/api/internal/v1/llm-proxies")
	{
		llmProxyGroup.GET("/:proxyId", h.GetLLMProxy)
	}

	deploymentGroup := r.Group("/api/internal/v1/deployments")
	{
		deploymentGroup.GET("", h.GetGatewayDeployments)
		deploymentGroup.POST("/fetch-batch", h.BatchFetchDeployments)
	}

	mcpProxyGroup := r.Group("/api/internal/v1/mcp-proxies")
	{
		mcpProxyGroup.GET("/:proxyId", h.GetMCPProxy)
	}
}
