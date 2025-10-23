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
	"log"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
	"platform-api/src/internal/service"
)

type GatewayInternalAPIHandler struct {
	gatewayService         *service.GatewayService
	gatewayInternalService *service.GatewayInternalAPIService
}

func NewGatewayInternalAPIHandler(gatewayService *service.GatewayService,
	gatewayInternalService *service.GatewayInternalAPIService) *GatewayInternalAPIHandler {
	return &GatewayInternalAPIHandler{
		gatewayService:         gatewayService,
		gatewayInternalService: gatewayInternalService,
	}
}

// GetAPIsByOrganization handles GET /api/internal/v1/apis
func (h *GatewayInternalAPIHandler) GetAPIsByOrganization(c *gin.Context) {
	// Extract client IP for rate limiting
	clientIP := c.ClientIP()

	// Extract and validate API key from header
	apiKey := c.GetHeader("api-key")
	if apiKey == "" {
		log.Printf("[WARN] Unauthorized access attempt from IP: %s - Missing API key", clientIP)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"API key is required. Provide 'api-key' header."))
		return
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		log.Printf("[WARN] Authentication failed ip: %s - error=%v", clientIP, err)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Invalid or expired API key"))
		return
	}

	orgID := gateway.OrganizationID
	apis, err := h.gatewayInternalService.GetAPIsByOrganization(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get apis"))
		return
	}

	// Create ZIP file from API YAML file
	zipData, err := utils.CreateAPIYamlZip(apis)
	if err != nil {
		log.Printf("[ERROR] Failed to create ZIP file for org %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API package"))
		return
	}

	// Set headers for ZIP file download
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"apis-org-%s.zip\"", orgID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	c.Data(http.StatusOK, "application/zip", zipData)
}

// GetAPI handles GET /api/internal/v1/apis/:apiId
func (h *GatewayInternalAPIHandler) GetAPI(c *gin.Context) {
	// Extract client IP for rate limiting
	clientIP := c.ClientIP()

	// Extract and validate API key from header
	apiKey := c.GetHeader("api-key")
	if apiKey == "" {
		log.Printf("[WARN] Unauthorized access attempt from IP: %s - Missing API key", clientIP)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"API key is required. Provide 'api-key' header."))
		return
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		log.Printf("[WARN] Authentication failed ip: %s - error=%v", clientIP, err)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Invalid or expired API key"))
		return
	}

	orgID := gateway.OrganizationID
	apiID := c.Param("apiId")
	if apiID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	api, err := h.gatewayInternalService.GetAPIByUUID(apiID, orgID)
	if err != nil {
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
		log.Printf("[ERROR] Failed to create ZIP file for API %s: %v", apiID, err)
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

func (h *GatewayInternalAPIHandler) RegisterRoutes(r *gin.Engine) {
	orgGroup := r.Group("/api/internal/v1/apis")
	{
		orgGroup.GET("", h.GetAPIsByOrganization)
		orgGroup.GET("/:apiId", h.GetAPI)
	}
}
