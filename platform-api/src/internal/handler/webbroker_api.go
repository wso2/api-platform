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
	"strconv"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// WebBrokerAPIHandler handles CRUD and auxiliary routes for WebBroker APIs
type WebBrokerAPIHandler struct {
	webbrokerAPIService *service.WebBrokerAPIService
	slogger             *slog.Logger
}

// NewWebBrokerAPIHandler creates a new WebBrokerAPIHandler instance
func NewWebBrokerAPIHandler(webbrokerAPIService *service.WebBrokerAPIService, slogger *slog.Logger) *WebBrokerAPIHandler {
	return &WebBrokerAPIHandler{
		webbrokerAPIService: webbrokerAPIService,
		slogger:             slogger,
	}
}

// RegisterRoutes registers WebBroker API routes
func (h *WebBrokerAPIHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group(constants.APIBasePath)
	{
		v1.POST("/webbroker-apis", h.CreateWebBrokerAPI)
		v1.GET("/webbroker-apis", h.ListWebBrokerAPIs)
		v1.GET("/webbroker-apis/:apiId", h.GetWebBrokerAPI)
		v1.PUT("/webbroker-apis/:apiId", h.UpdateWebBrokerAPI)
		v1.DELETE("/webbroker-apis/:apiId", h.DeleteWebBrokerAPI)
		v1.POST("/webbroker-apis/:apiId/publications", h.PublishToDevPortal)
		v1.DELETE("/webbroker-apis/:apiId/publications/:devportalId", h.UnpublishFromDevPortal)
	}
}

// CreateWebBrokerAPI handles POST /api/v0.9/webbroker-apis
func (h *WebBrokerAPIHandler) CreateWebBrokerAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.WebBrokerAPI
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("WebBroker API request validation failed", "org_id", orgID, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, _ := middleware.GetUsernameFromContext(c)

	resp, err := h.webbrokerAPIService.Create(orgID, createdBy, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListWebBrokerAPIs handles GET /api/v0.9/webbroker-apis
func (h *WebBrokerAPIHandler) ListWebBrokerAPIs(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := strings.TrimSpace(c.Query("projectId"))
	if projectID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "projectId query parameter is required"))
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.webbrokerAPIService.List(orgID, projectID, limit, offset)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetWebBrokerAPI handles GET /api/v0.9/webbroker-apis/:apiId
func (h *WebBrokerAPIHandler) GetWebBrokerAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := c.Param("apiId")
	resp, err := h.webbrokerAPIService.Get(orgID, id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateWebBrokerAPI handles PUT /api/v0.9/webbroker-apis/:apiId
func (h *WebBrokerAPIHandler) UpdateWebBrokerAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := c.Param("apiId")

	var req api.WebBrokerAPI
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("WebBroker API update validation failed", "org_id", orgID, "api_id", id, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.webbrokerAPIService.Update(orgID, id, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteWebBrokerAPI handles DELETE /api/v0.9/webbroker-apis/:apiId
func (h *WebBrokerAPIHandler) DeleteWebBrokerAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := c.Param("apiId")

	if err := h.webbrokerAPIService.Delete(orgID, id); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// PublishToDevPortal publishes a WebBroker API to a DevPortal.
func (h *WebBrokerAPIHandler) PublishToDevPortal(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiId")

	var req api.PublishToDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if err := h.webbrokerAPIService.PublishToDevPortal(orgID, apiHandle, &req); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "WebBroker API published successfully to DevPortal"})
}

// UnpublishFromDevPortal unpublishes a WebBroker API from a DevPortal.
func (h *WebBrokerAPIHandler) UnpublishFromDevPortal(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiId")
	devPortalID := c.Param("devportalId")

	if err := h.webbrokerAPIService.UnpublishFromDevPortal(orgID, apiHandle, devPortalID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "WebBroker API unpublished successfully from DevPortal"})
}

// handleServiceError maps service errors to HTTP responses
func (h *WebBrokerAPIHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, constants.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrWebBrokerAPINotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
	case errors.Is(err, constants.ErrWebBrokerAPIExists):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "WebBroker API with this ID already exists"))
	case errors.Is(err, constants.ErrWebBrokerAPILimitReached):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "WebBroker API limit reached for the organization"))
	case errors.Is(err, constants.ErrProjectNotFound):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project not found"))
	case errors.Is(err, constants.ErrDevPortalNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "DevPortal not found"))
	default:
		h.slogger.Error("WebBroker API service error", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
