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
	"github.com/gin-gonic/gin"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
)

type APIHandler struct {
	apiService *service.APIService
}

func NewAPIHandler(apiService *service.APIService) *APIHandler {
	return &APIHandler{
		apiService: apiService,
	}
}

// CreateAPI creates a new API
func (h *APIHandler) CreateAPI(c *gin.Context) {
	var req service.CreateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API name is required"))
		return
	}
	if req.Context == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API context is required"))
		return
	}
	if req.Version == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API version is required"))
		return
	}
	if req.ProjectID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project UUID is required"))
		return
	}

	api, err := h.apiService.CreateAPI(&req)
	if err != nil {
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "API already exists in the project"))
			return
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API name format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API context format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIVersion) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API version format"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid transport protocol"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create API"))
		return
	}

	c.JSON(http.StatusCreated, api)
}

// GetAPI retrieves an API by UUID
func (h *APIHandler) GetAPI(c *gin.Context) {
	uuid := c.Param("api_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API UUID is required"))
		return
	}

	api, err := h.apiService.GetAPIByUUID(uuid)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API"))
		return
	}

	c.JSON(http.StatusOK, api)
}

// GetAPIsByProject retrieves all APIs for a project
func (h *APIHandler) GetAPIsByProject(c *gin.Context) {
	projectUUID := c.Param("project_uuid")
	if projectUUID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project UUID is required"))
		return
	}

	apis, err := h.apiService.GetAPIsByProjectID(projectUUID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get APIs"))
		return
	}

	response := gin.H{
		"apis": apis,
		"pagination": gin.H{
			"total": len(apis),
		},
	}

	c.JSON(http.StatusOK, response)
}

// UpdateAPI updates an existing API
func (h *APIHandler) UpdateAPI(c *gin.Context) {
	uuid := c.Param("api_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API UUID is required"))
		return
	}

	var req service.UpdateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	api, err := h.apiService.UpdateAPI(uuid, &req)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid lifecycle status"))
			return
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API type"))
			return
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid transport protocol"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update API"))
		return
	}

	c.JSON(http.StatusOK, api)
}

// DeleteAPI deletes an API
func (h *APIHandler) DeleteAPI(c *gin.Context) {
	uuid := c.Param("api_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API UUID is required"))
		return
	}

	err := h.apiService.DeleteAPI(uuid)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete API"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	// API routes
	apiGroup := r.Group("/api/v1/apis")
	{
		apiGroup.POST("", h.CreateAPI)
		apiGroup.GET("/:api_uuid", h.GetAPI)
		apiGroup.PUT("/:api_uuid", h.UpdateAPI)
		apiGroup.DELETE("/:api_uuid", h.DeleteAPI)
	}

	// Project-specific API routes
	projectAPIGroup := r.Group("/api/v1/projects/:project_uuid/apis")
	{
		projectAPIGroup.GET("", h.GetAPIsByProject)
	}
}
