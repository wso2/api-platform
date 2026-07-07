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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type ProjectHandler struct {
	projectService *service.ProjectService
	identity       *service.IdentityService
	slogger        *slog.Logger
}

func NewProjectHandler(projectService *service.ProjectService, identity *service.IdentityService, slogger *slog.Logger) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		identity:       identity,
		slogger:        slogger,
	}
}

// CreateProject handles POST /api/v0.9/projects
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	organizationID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	var req api.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}

	if req.DisplayName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Project displayName is required"))
		return
	}

	actor, ok := resolveActor(w, r, h.identity, h.slogger, "create project")
	if !ok {
		return
	}
	project, err := h.projectService.CreateProject(&req, organizationID, actor)
	if err != nil {
		if errors.Is(err, constants.ErrProjectExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeProjectExists, "A project with this name already exists in the organization."))
			return
		}
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeOrganizationNotFound, "The specified organization could not be found."))
			return
		}
		if errors.Is(err, constants.ErrInvalidProjectName) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Project displayName is required"))
			return
		}
		h.slogger.Error("Failed to create project", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to create project"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, project)
}

// GetProject handles GET /api/v0.9/projects/:projectId
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	projectId := r.PathValue("projectId")
	if projectId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Project ID is required"))
		return
	}

	project, err := h.projectService.GetProjectByHandle(projectId, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeProjectNotFound, "The specified project could not be found."))
			return
		}
		h.slogger.Error("Failed to get project", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to get project"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, project)
}

// ListProjects handles GET /api/v0.9/projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	projects, err := h.projectService.GetProjectsByOrganization(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeOrganizationNotFound, "The specified organization could not be found."))
			return
		}
		h.slogger.Error("Failed to list projects", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to get projects"))
		return
	}

	// Return constitution-compliant list response
	httputil.WriteJSON(w, http.StatusOK, api.ProjectListResponse{
		Count: len(projects),
		List:  projects,
		Pagination: api.Pagination{
			Total:  len(projects),
			Offset: 0,
			Limit:  len(projects),
		},
	})
}

// UpdateProject handles PUT /api/v0.9/projects/:projectId
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	projectId := r.PathValue("projectId")
	if projectId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Project ID is required"))
		return
	}

	var req api.Project
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}

	actor, ok := resolveActor(w, r, h.identity, h.slogger, "update project")
	if !ok {
		return
	}
	project, err := h.projectService.UpdateProject(projectId, &req, orgID, actor)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeProjectNotFound, "The specified project could not be found."))
			return
		}
		if errors.Is(err, constants.ErrHandleImmutable) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Project id is immutable and cannot be changed"))
			return
		}
		if errors.Is(err, constants.ErrProjectExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeProjectExists, "A project with this name already exists in the organization."))
			return
		}
		h.slogger.Error("Failed to update project", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to update project"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, project)
}

// DeleteProject handles DELETE /api/v0.9/projects/:projectId
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	projectId := r.PathValue("projectId")
	if projectId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Project ID is required"))
		return
	}

	actor, ok := resolveActor(w, r, h.identity, h.slogger, "delete project")
	if !ok {
		return
	}
	err := h.projectService.DeleteProject(projectId, orgID, actor)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeProjectNotFound, "The specified project could not be found."))
			return
		}
		if errors.Is(err, constants.ErrOrganizationMustHAveAtLeastOneProject) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Organization must have at least one project"))
			return
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedAPIs) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Project has associated APIs"))
			return
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedMCPProxies) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Project has associated MCP proxies"))
			return
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedApplications) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Project has associated applications"))
			return
		}
		h.slogger.Error("Failed to delete project", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to delete project"))
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *ProjectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+constants.APIBasePath+"/projects", h.ListProjects)
	mux.HandleFunc("POST "+constants.APIBasePath+"/projects", h.CreateProject)
	mux.HandleFunc("GET "+constants.APIBasePath+"/projects/{projectId}", h.GetProject)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/projects/{projectId}", h.UpdateProject)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/projects/{projectId}", h.DeleteProject)
}
