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
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// DevPortalHandler handles HTTP requests related to DevPortal operations
type DevPortalHandler struct {
	devPortalService *service.DevPortalService
	logger           *slog.Logger
}

// NewDevPortalHandler creates a new DevPortalHandler
func NewDevPortalHandler(devPortalService *service.DevPortalService, logger *slog.Logger) *DevPortalHandler {
	return &DevPortalHandler{
		devPortalService: devPortalService,
		logger:           logger,
	}
}

// CreateDevPortal handles POST /api/v1/devportals
func (h *DevPortalHandler) CreateDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Parse request body
	var req api.CreateDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse request body for creating DevPortal", "organizationId", orgID, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	h.logger.Info("Attempting to create DevPortal", "name", req.Name, "organizationId", orgID)

	// Create DevPortal
	response, err := h.devPortalService.CreateDevPortal(orgID, &req)
	if err != nil {
		h.logger.Error("Failed to create DevPortal", "name", req.Name, "organizationId", orgID, "error", err)
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	h.logger.Info("Created DevPortal", "name", response.Name, "organizationId", orgID)
	c.JSON(http.StatusCreated, response)
}

// GetDevPortal handles GET /api/v1/devportals/:devportalId
func (h *DevPortalHandler) GetDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract DevPortal ID from path
	devPortalID := c.Param("devportalId")
	if devPortalID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"DevPortal ID is required"))
		return
	}

	// Get DevPortal
	response, err := h.devPortalService.GetDevPortal(devPortalID, orgID)
	if err != nil {
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListDevPortals handles GET /api/v1/devportals
func (h *DevPortalHandler) ListDevPortals(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Parse filter parameters
	var isDefault, isEnabled *bool
	if defaultStr := c.Query("default"); defaultStr != "" {
		if defaultVal, err := strconv.ParseBool(defaultStr); err == nil {
			isDefault = &defaultVal
		}
	}
	if activeStr := c.Query("active"); activeStr != "" {
		if activeVal, err := strconv.ParseBool(activeStr); err == nil {
			isEnabled = &activeVal
		}
	}
	h.logger.Debug("Listing DevPortals with filters", "isEnabled", isEnabled, "isDefault", isDefault)

	// List DevPortals
	response, err := h.devPortalService.ListDevPortals(orgID, isDefault, isEnabled, limit, offset)
	if err != nil {
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateDevPortal handles PUT /api/v1/devportals/:devportalId
func (h *DevPortalHandler) UpdateDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract DevPortal ID from path
	devPortalID := c.Param("devportalId")
	if devPortalID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"DevPortal ID is required"))
		return
	}

	// Parse request body
	var req api.UpdateDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse request body for updating DevPortal", "devPortalId", devPortalID, "organizationId", orgID, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	h.logger.Info("Attempting to update DevPortal", "devPortalId", devPortalID, "organizationId", orgID)

	// Update DevPortal
	response, err := h.devPortalService.UpdateDevPortal(devPortalID, orgID, &req)
	if err != nil {
		h.logger.Error("Failed to update DevPortal", "devPortalId", devPortalID, "organizationId", orgID, "error", err)
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	h.logger.Info("Updated DevPortal", "devPortalId", devPortalID)
	c.JSON(http.StatusOK, response)
}

// DeleteDevPortal handles DELETE /api/v1/devportals/:devportalId
func (h *DevPortalHandler) DeleteDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract DevPortal ID from path
	devPortalID := c.Param("devportalId")
	if devPortalID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"DevPortal ID is required"))
		return
	}

	h.logger.Info("Attempting to delete DevPortal", "devPortalId", devPortalID, "organizationId", orgID)

	// Delete DevPortal
	if err := h.devPortalService.DeleteDevPortal(devPortalID, orgID); err != nil {
		h.logger.Error("Failed to delete DevPortal", "devPortalId", devPortalID, "organizationId", orgID, "error", err)
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	h.logger.Info("Deleted DevPortal", "devPortalId", devPortalID)
	c.JSON(http.StatusNoContent, nil)
}

// ActivateDevPortal handles POST /api/v1/devportals/:devportalId/activate
func (h *DevPortalHandler) ActivateDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract DevPortal ID from path
	devPortalID := c.Param("devportalId")
	if devPortalID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"DevPortal ID is required"))
		return
	}

	h.logger.Info("Attempting to activate DevPortal", "devPortalId", devPortalID, "organizationId", orgID)

	// Activate DevPortal
	err := h.devPortalService.EnableDevPortal(devPortalID, orgID)
	if err != nil {
		h.logger.Error("Failed to activate DevPortal", "devPortalId", devPortalID, "organizationId", orgID, "error", err)
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	h.logger.Info("Activated DevPortal", "devPortalId", devPortalID)
	c.JSON(http.StatusOK, api.CommonResponse{
		Success:   true,
		Message:   "DevPortal activated successfully",
		Timestamp: time.Now(),
	})
}

// DeactivateDevPortal handles POST /api/v1/devportals/:devportalId/deactivate
func (h *DevPortalHandler) DeactivateDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract DevPortal ID from path
	devPortalID := c.Param("devportalId")
	if devPortalID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"DevPortal ID is required"))
		return
	}

	h.logger.Info("Attempting to deactivate DevPortal", "devPortalId", devPortalID, "organizationId", orgID)

	// Deactivate DevPortal
	err := h.devPortalService.DisableDevPortal(devPortalID, orgID)
	if err != nil {
		h.logger.Error("Failed to deactivate DevPortal", "devPortalId", devPortalID, "organizationId", orgID, "error", err)
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	h.logger.Info("Deactivated DevPortal", "devPortalId", devPortalID)
	c.JSON(http.StatusOK, api.CommonResponse{
		Success:   true,
		Message:   "DevPortal deactivated successfully",
		Timestamp: time.Now(),
	})
}

// SetAsDefault handles POST /api/v1/devportals/:devportalId/set-default
func (h *DevPortalHandler) SetAsDefault(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract DevPortal ID from path
	devPortalID := c.Param("devportalId")
	if devPortalID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"DevPortal ID is required"))
		return
	}

	h.logger.Info("Attempting to set DevPortal as default", "devPortalId", devPortalID, "organizationId", orgID)

	// Set as default
	if err := h.devPortalService.SetAsDefault(devPortalID, orgID); err != nil {
		h.logger.Error("Failed to set DevPortal as default", "devPortalId", devPortalID, "organizationId", orgID, "error", err)
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	h.logger.Info("Set DevPortal as default", "devPortalId", devPortalID)
	c.JSON(http.StatusOK, gin.H{
		"message":       "DevPortal set as default successfully",
		"devPortalUuid": devPortalID,
	})
}

// GetDefaultDevPortal handles GET /api/v1/devportals/default
func (h *DevPortalHandler) GetDefaultDevPortal(c *gin.Context) {
	// Extract organization ID from context
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Get default DevPortal
	response, err := h.devPortalService.GetDefaultDevPortal(orgID)
	if err != nil {
		// Use centralized error handling
		status, errorResp := utils.GetErrorResponse(err)
		c.JSON(status, errorResp)
		return
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers all DevPortal routes
func (h *DevPortalHandler) RegisterRoutes(r *gin.Engine) {
	h.logger.Debug("Registering DevPortal routes")
	v1 := r.Group("/api/v1")
	{
		// DevPortal CRUD operations
		v1.POST("/devportals", h.CreateDevPortal)
		v1.GET("/devportals", h.ListDevPortals)
		v1.GET("/devportals/default", h.GetDefaultDevPortal)
		v1.GET("/devportals/:devportalId", h.GetDevPortal)
		v1.PUT("/devportals/:devportalId", h.UpdateDevPortal)
		v1.DELETE("/devportals/:devportalId", h.DeleteDevPortal)

		// DevPortal actions
		v1.POST("/devportals/:devportalId/activate", h.ActivateDevPortal)
		v1.POST("/devportals/:devportalId/deactivate", h.DeactivateDevPortal)
		v1.POST("/devportals/:devportalId/set-default", h.SetAsDefault)
	}
}
