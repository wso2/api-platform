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
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/utils"

	"platform-api/src/internal/service"

	"github.com/gin-gonic/gin"
)

type OrganizationHandler struct {
	orgService *service.OrganizationService
}

func NewOrganizationHandler(orgService *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
	}
}

// RegisterOrganization handles POST /api/v1/organizations
func (h *OrganizationHandler) RegisterOrganization(c *gin.Context) {
	var req dto.CreateOrganizationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.Handle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Handle is required"))
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Name is required"))
		return
	}
	if req.ID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Organization ID is required"))
		return
	}
	if req.Region == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Region is required"))
		return
	}

	org, err := h.orgService.RegisterOrganization(req.ID, req.Handle, req.Name, req.Region)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Organization already exists"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Organization with the given ID already exists"))
			return
		}
		if errors.Is(err, constants.ErrInvalidHandle) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Organization handle must be URL friendly"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create organization"))
		return
	}

	c.JSON(http.StatusCreated, org)
}

// HeadOrganizationByUuid handles HEAD /api/v1/organizations/{organizationId}
func (h *OrganizationHandler) HeadOrganizationByUuid(c *gin.Context) {
	orgID := c.Param("organizationId")

	_, err := h.orgService.GetOrganizationByUUID(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

// GetOrganization handles GET /api/v1/organizations
func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	org, err := h.orgService.GetOrganizationByUUID(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrMultipleOrganizations) {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Data integrity error: multiple organizations found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get organization"))
		return
	}

	c.JSON(http.StatusOK, org)
}

func (h *OrganizationHandler) RegisterRoutes(r *gin.Engine) {
	orgGroup := r.Group("/api/v1/organizations")
	{
		orgGroup.POST("", h.RegisterOrganization)
		orgGroup.GET("", h.GetOrganization)
		orgGroup.HEAD("/:organizationId", h.HeadOrganizationByUuid)
	}
}
