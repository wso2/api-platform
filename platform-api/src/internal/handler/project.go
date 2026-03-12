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
	"log/slog"
	"net/http"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	projectService *service.ProjectService
	slogger        *slog.Logger
}

func NewProjectHandler(projectService *service.ProjectService, slogger *slog.Logger) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		slogger:        slogger,
	}
}

// CreateProject handles POST /api/v1/projects
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	organizationID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.NewValidationErrorResponse(c, err)
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project name is required"))
		return
	}

	project, err := h.projectService.CreateProject(&req, organizationID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Project already exists in organization"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidProjectName) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Project name is required"))
			return
		}
		h.slogger.Error("Failed to create project", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create project"))
		return
	}

	c.JSON(http.StatusCreated, project)
}

// GetProject handles GET /api/v1/projects/:projectId
func (h *ProjectHandler) GetProject(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	projectId := c.Param("projectId")
	if projectId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	project, err := h.projectService.GetProjectByID(projectId, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		h.slogger.Error("Failed to get project", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get project"))
		return
	}

	c.JSON(http.StatusOK, project)
}

// ListProjects handles GET /api/v1/projects
func (h *ProjectHandler) ListProjects(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	projects, err := h.projectService.GetProjectsByOrganization(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		h.slogger.Error("Failed to list projects", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get projects"))
		return
	}

	// Return constitution-compliant list response
	c.JSON(http.StatusOK, api.ProjectListResponse{
		Count: len(projects),
		List:  projects,
		Pagination: api.Pagination{
			Total:  len(projects),
			Offset: 0,
			Limit:  len(projects),
		},
	})
}

// UpdateProject handles PUT /api/v1/projects/:projectId
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	projectId := c.Param("projectId")
	if projectId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	var req api.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.NewValidationErrorResponse(c, err)
		return
	}

	project, err := h.projectService.UpdateProject(projectId, &req, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrProjectExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Project already exists in organization"))
			return
		}
		h.slogger.Error("Failed to update project", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update project"))
		return
	}

	c.JSON(http.StatusOK, project)
}

// DeleteProject handles DELETE /api/v1/projects/:projectId
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	projectId := c.Param("projectId")
	if projectId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Project ID is required"))
		return
	}

	err := h.projectService.DeleteProject(projectId, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Project not found"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationMustHAveAtLeastOneProject) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Organization must have at least one project"))
			return
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedAPIs) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Project has associated APIs"))
			return
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedMCPProxies) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Project has associated MCP proxies"))
			return
		}
		h.slogger.Error("Failed to delete project", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete project"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *ProjectHandler) RegisterRoutes(r *gin.Engine) {
	projectGroup := r.Group("/api/v1/projects")
	{
		projectGroup.GET("", h.ListProjects)
		projectGroup.POST("", h.CreateProject)
		projectGroup.GET("/:projectId", h.GetProject)
		projectGroup.PUT("/:projectId", h.UpdateProject)
		projectGroup.DELETE("/:projectId", h.DeleteProject)
	}
}
