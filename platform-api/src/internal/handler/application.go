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
	"strings"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type ApplicationHandler struct {
	applicationService *service.ApplicationService
	slogger            *slog.Logger
}

func NewApplicationHandler(applicationService *service.ApplicationService, slogger *slog.Logger) *ApplicationHandler {
	return &ApplicationHandler{applicationService: applicationService, slogger: slogger}
}

func (h *ApplicationHandler) CreateApplication(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req dto.CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.NewValidationErrorResponse(c, err)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application name is required"))
		return
	}
	if strings.TrimSpace(req.Type) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application type is required"))
		return
	}

	app, err := h.applicationService.CreateApplication(&req, orgID)
	if err != nil {
		h.writeApplicationError(c, err, "Failed to create application")
		return
	}

	c.JSON(http.StatusCreated, app)
}

func (h *ApplicationHandler) GetApplication(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := c.Param("appId")
	if strings.TrimSpace(appID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	app, err := h.applicationService.GetApplicationByID(appID, orgID)
	if err != nil {
		h.writeApplicationError(c, err, "Failed to get application")
		return
	}

	c.JSON(http.StatusOK, app)
}

func (h *ApplicationHandler) ListApplications(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := strings.TrimSpace(c.Query("projectId"))
	apps, err := h.applicationService.GetApplicationsByOrganization(orgID, projectID)
	if err != nil {
		h.writeApplicationError(c, err, "Failed to list applications")
		return
	}

	c.JSON(http.StatusOK, apps)
}

func (h *ApplicationHandler) UpdateApplication(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := c.Param("appId")
	if strings.TrimSpace(appID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	userID := h.resolveRequesterUserID(c)

	var req dto.UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.NewValidationErrorResponse(c, err)
		return
	}

	app, err := h.applicationService.UpdateApplication(appID, &req, orgID, userID)
	if err != nil {
		h.writeApplicationError(c, err, "Failed to update application")
		return
	}

	c.JSON(http.StatusOK, app)
}

func (h *ApplicationHandler) DeleteApplication(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := c.Param("appId")
	if strings.TrimSpace(appID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	if err := h.applicationService.DeleteApplication(appID, orgID); err != nil {
		h.writeApplicationError(c, err, "Failed to delete application")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *ApplicationHandler) ListApplicationAPIKeys(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := c.Param("appId")
	if strings.TrimSpace(appID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	keys, err := h.applicationService.ListMappedAPIKeys(appID, orgID)
	if err != nil {
		h.writeApplicationError(c, err, "Failed to list mapped API keys")
		return
	}

	c.JSON(http.StatusOK, keys)
}

func (h *ApplicationHandler) AddApplicationAPIKeys(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := c.Param("appId")
	userID := h.resolveRequesterUserID(c)
	if strings.TrimSpace(appID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	var req dto.AddApplicationAPIKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.NewValidationErrorResponse(c, err)
		return
	}
	if len(req.ApiKeyIds) == 0 {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "At least one API key id is required"))
		return
	}

	keys, err := h.applicationService.AddMappedAPIKeys(appID, &req, orgID, userID)
	if err != nil {
		h.writeApplicationError(c, err, "Failed to add mapped API keys")
		return
	}

	c.JSON(http.StatusOK, keys)
}

func (h *ApplicationHandler) RemoveApplicationAPIKey(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := c.Param("appId")
	keyID := c.Param("keyId")
	userID := h.resolveRequesterUserID(c)
	if strings.TrimSpace(appID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	if strings.TrimSpace(keyID) == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API key id is required"))
		return
	}

	if err := h.applicationService.RemoveMappedAPIKey(appID, keyID, orgID, userID); err != nil {
		h.writeApplicationError(c, err, "Failed to remove mapped API key")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *ApplicationHandler) RegisterRoutes(r *gin.Engine) {
	applicationGroup := r.Group("/api/v1/applications")
	{
		applicationGroup.GET("", h.ListApplications)
		applicationGroup.POST("", h.CreateApplication)
		applicationGroup.GET("/:appId", h.GetApplication)
		applicationGroup.PUT("/:appId", h.UpdateApplication)
		applicationGroup.DELETE("/:appId", h.DeleteApplication)

		applicationGroup.GET("/:appId/api-keys", h.ListApplicationAPIKeys)
		applicationGroup.POST("/:appId/api-keys", h.AddApplicationAPIKeys)
		applicationGroup.DELETE("/:appId/api-keys/:keyId", h.RemoveApplicationAPIKey)
	}
}

func (h *ApplicationHandler) resolveRequesterUserID(c *gin.Context) string {
	userID := strings.TrimSpace(c.GetHeader("x-user-id"))
	if userID != "" {
		return userID
	}

	if ctxUserID, ok := middleware.GetUserIDFromContext(c); ok {
		return strings.TrimSpace(ctxUserID)
	}

	return ""
}

func (h *ApplicationHandler) writeApplicationError(c *gin.Context, err error, fallback string) {
	if h.slogger != nil {
		h.slogger.Error(fallback,
			"error", err,
			"path", c.FullPath(),
			"method", c.Request.Method,
			"appId", c.Param("appId"),
		)
	}

	switch {
	case errors.Is(err, constants.ErrApplicationNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Application not found"))
	case errors.Is(err, constants.ErrProjectNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
	case errors.Is(err, constants.ErrOrganizationNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Organization not found"))
	case errors.Is(err, constants.ErrApplicationExists):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Application already exists in project"))
	case errors.Is(err, constants.ErrHandleExists):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Application handle already exists in organization"))
	case errors.Is(err, constants.ErrAPIKeyNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API key not found"))
	case errors.Is(err, constants.ErrAPIKeyForbidden):
		c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", "Only the key creator can perform this action"))
	case errors.Is(err, constants.ErrInvalidApplicationName):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application name is required"))
	case errors.Is(err, constants.ErrInvalidApplicationType):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application type is required"))
	case errors.Is(err, constants.ErrUnsupportedApplicationType):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid application type. Only 'genai' is supported"))
	case errors.Is(err, constants.ErrInvalidHandle):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid application handle format"))
	case errors.Is(err, constants.ErrInvalidAPIKey):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API key id"))
	default:
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", fallback))
	}
}
