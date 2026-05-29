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

// WebSubAPIHandler handles CRUD and auxiliary routes for WebSub APIs
type WebSubAPIHandler struct {
	websubAPIService *service.WebSubAPIService
	slogger          *slog.Logger
}

// NewWebSubAPIHandler creates a new WebSubAPIHandler instance
func NewWebSubAPIHandler(websubAPIService *service.WebSubAPIService, slogger *slog.Logger) *WebSubAPIHandler {
	return &WebSubAPIHandler{
		websubAPIService: websubAPIService,
		slogger:          slogger,
	}
}

// RegisterRoutes registers WebSub API routes
func (h *WebSubAPIHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		v1.POST("/websub-apis", h.CreateWebSubAPI)
		v1.GET("/websub-apis", h.ListWebSubAPIs)
		v1.GET("/websub-apis/:apiId", h.GetWebSubAPI)
		v1.PUT("/websub-apis/:apiId", h.UpdateWebSubAPI)
		v1.DELETE("/websub-apis/:apiId", h.DeleteWebSubAPI)
		v1.POST("/websub-apis/:apiId/devportals/publish", h.PublishToDevPortal)
		v1.POST("/websub-apis/:apiId/devportals/unpublish", h.UnpublishFromDevPortal)
	}
}

// CreateWebSubAPI handles POST /api/v1/websub-apis
func (h *WebSubAPIHandler) CreateWebSubAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.WebSubAPI
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("WebSub API request validation failed", "org_id", orgID, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	createdBy, _ := middleware.GetUsernameFromContext(c)

	resp, err := h.websubAPIService.Create(orgID, createdBy, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListWebSubAPIs handles GET /api/v1/websub-apis
func (h *WebSubAPIHandler) ListWebSubAPIs(c *gin.Context) {
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

	resp, err := h.websubAPIService.List(orgID, projectID, limit, offset)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetWebSubAPI handles GET /api/v1/websub-apis/:apiId
func (h *WebSubAPIHandler) GetWebSubAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := c.Param("apiId")
	resp, err := h.websubAPIService.Get(orgID, id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateWebSubAPI handles PUT /api/v1/websub-apis/:apiId
func (h *WebSubAPIHandler) UpdateWebSubAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := c.Param("apiId")

	var req api.WebSubAPI
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("WebSub API update validation failed", "org_id", orgID, "api_id", id, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.websubAPIService.Update(orgID, id, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteWebSubAPI handles DELETE /api/v1/websub-apis/:apiId
func (h *WebSubAPIHandler) DeleteWebSubAPI(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	id := c.Param("apiId")

	if err := h.websubAPIService.Delete(orgID, id); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// PublishToDevPortal handles POST /api/v1/websub-apis/:apiId/devportals/publish
func (h *WebSubAPIHandler) PublishToDevPortal(c *gin.Context) {
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

	if err := h.websubAPIService.PublishToDevPortal(orgID, apiHandle, &req); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "WebSub API published successfully to DevPortal"})
}

// UnpublishFromDevPortal handles POST /api/v1/websub-apis/:apiId/devportals/unpublish
func (h *WebSubAPIHandler) UnpublishFromDevPortal(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiId")

	var req api.UnpublishFromDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	devPortalUUID := req.DevPortalUuid.String()

	if err := h.websubAPIService.UnpublishFromDevPortal(orgID, apiHandle, devPortalUUID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "WebSub API unpublished successfully from DevPortal"})
}

// handleServiceError maps service errors to HTTP responses
func (h *WebSubAPIHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, constants.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	case errors.Is(err, constants.ErrWebSubAPIExists):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "WebSub API with this ID already exists"))
	case errors.Is(err, constants.ErrWebSubAPILimitReached):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "WebSub API limit reached for the organization"))
	case errors.Is(err, constants.ErrProjectNotFound):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project not found"))
	case errors.Is(err, constants.ErrDevPortalNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "DevPortal not found"))
	default:
		h.slogger.Error("WebSub API service error", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
