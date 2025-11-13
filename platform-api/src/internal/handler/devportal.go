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
	"strconv"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// DevPortalHandler handles HTTP requests related to DevPortal operations
type DevPortalHandler struct {
	devPortalService *service.DevPortalService
}

// NewDevPortalHandler creates a new DevPortalHandler
func NewDevPortalHandler(devPortalService *service.DevPortalService) *DevPortalHandler {
	return &DevPortalHandler{
		devPortalService: devPortalService,
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
	var req dto.CreateDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	// Create DevPortal
	response, err := h.devPortalService.CreateDevPortal(orgID, &req)
	if err != nil {
		// Handle duplicate DevPortal errors
		if errors.Is(err, constants.ErrDevPortalAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"DevPortal with these attributes already exists"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalAPIUrlExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"DevPortal with this API URL already exists in the organization"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalIdentifierExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"DevPortal with this identifier already exists in the organization"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalHostnameExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"DevPortal with this hostname already exists in the organization"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalDefaultAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Default DevPortal already exists for this organization"))
			return
		}

		// Handle validation errors
		if errors.Is(err, constants.ErrDevPortalNameRequired) ||
			errors.Is(err, constants.ErrDevPortalIdentifierRequired) ||
			errors.Is(err, constants.ErrDevPortalAPIUrlRequired) ||
			errors.Is(err, constants.ErrDevPortalHostnameRequired) ||
			errors.Is(err, constants.ErrDevPortalAPIKeyRequired) ||
			errors.Is(err, constants.ErrDevPortalHeaderKeyNameRequired) ||
			errors.Is(err, constants.ErrDevPortalInvalidVisibility) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
			return
		}

		// Internal server error
		log.Printf("[DevPortalHandler] Failed to create DevPortal: %v", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create DevPortal"))
		return
	}

	log.Printf("[DevPortalHandler] Created DevPortal %s for organization %s", response.Name, orgID)
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
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}

		log.Printf("[DevPortalHandler] Failed to get DevPortal %s: %v", devPortalID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get DevPortal"))
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
	log.Println("isEnabled:", isEnabled, "isDefault:", isDefault)

	// List DevPortals
	response, err := h.devPortalService.ListDevPortals(orgID, isDefault, isEnabled, limit, offset)
	if err != nil {
		log.Printf("[DevPortalHandler] Failed to list DevPortals for organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list DevPortals"))
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
	var req dto.UpdateDevPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	// Update DevPortal
	response, err := h.devPortalService.UpdateDevPortal(devPortalID, orgID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}

		// Handle validation errors
		if errors.Is(err, constants.ErrDevPortalNameRequired) ||
			errors.Is(err, constants.ErrDevPortalAPIUrlRequired) ||
			errors.Is(err, constants.ErrDevPortalHostnameRequired) ||
			errors.Is(err, constants.ErrDevPortalAPIKeyRequired) ||
			errors.Is(err, constants.ErrDevPortalHeaderKeyNameRequired) ||
			errors.Is(err, constants.ErrDevPortalInvalidVisibility) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
			return
		}

		log.Printf("[DevPortalHandler] Failed to update DevPortal %s: %v", devPortalID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update DevPortal"))
		return
	}

	log.Printf("[DevPortalHandler] Updated DevPortal %s", devPortalID)
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

	// Delete DevPortal
	if err := h.devPortalService.DeleteDevPortal(devPortalID, orgID); err != nil {
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalCannotDeleteDefault) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Cannot delete default DevPortal"))
			return
		}

		log.Printf("[DevPortalHandler] Failed to delete DevPortal %s: %v", devPortalID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete DevPortal"))
		return
	}

	log.Printf("[DevPortalHandler] Deleted DevPortal %s", devPortalID)
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

	// Activate DevPortal
	response, err := h.devPortalService.EnableDevPortal(devPortalID, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}

		if errors.Is(err, constants.ErrDevPortalBackendUnreachable) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"DevPortal backend is currently unreachable. Please try again later or contact administrator."))
			return
		}

		if errors.Is(err, constants.ErrDevPortalSyncFailed) {
			c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"Failed to sync organization to DevPortal. Please check DevPortal configuration."))
			return
		}

		log.Printf("[DevPortalHandler] Failed to activate DevPortal %s: %v", devPortalID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to activate DevPortal"))
		return
	}

	log.Printf("[DevPortalHandler] Activated DevPortal %s", devPortalID)
	c.JSON(http.StatusOK, response)
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

	// Deactivate DevPortal
	response, err := h.devPortalService.DisableDevPortal(devPortalID, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}
		if errors.Is(err, constants.ErrDevPortalCannotDeactivateDefault) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Cannot deactivate default DevPortal"))
			return
		}

		log.Printf("[DevPortalHandler] Failed to deactivate DevPortal %s: %v", devPortalID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to deactivate DevPortal"))
		return
	}

	log.Printf("[DevPortalHandler] Deactivated DevPortal %s", devPortalID)
	c.JSON(http.StatusOK, response)
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

	// Set as default
	if err := h.devPortalService.SetAsDefault(devPortalID, orgID); err != nil {
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"DevPortal not found"))
			return
		}

		log.Printf("[DevPortalHandler] Failed to set DevPortal %s as default: %v", devPortalID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to set DevPortal as default"))
		return
	}

	log.Printf("[DevPortalHandler] Set DevPortal %s as default", devPortalID)
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
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No default DevPortal found for organization"))
			return
		}

		log.Printf("[DevPortalHandler] Failed to get default DevPortal for organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get default DevPortal"))
		return
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers all DevPortal routes
func (h *DevPortalHandler) RegisterRoutes(r *gin.Engine) {
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
