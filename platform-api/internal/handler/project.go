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
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
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
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) error {
	organizationID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("Project displayName is required")
	}

	actor, err := resolveActorErr(r, h.identity, "create project")
	if err != nil {
		return err
	}
	project, err := h.projectService.CreateProject(&req, organizationID, actor)
	if err != nil {
		if errors.Is(err, constants.ErrProjectExists) {
			return apperror.ProjectExists.Wrap(err)
		}
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			return apperror.OrganizationNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrInvalidProjectName) {
			return apperror.ValidationFailed.Wrap(err, "Project displayName is required")
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create project in org %s", organizationID))
	}

	setLocation(w, "projects", strOrEmpty(project.Id))
	httputil.WriteJSON(w, http.StatusCreated, project)
	return nil
}

// GetProject handles GET /api/v0.9/projects/:projectId
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectId := r.PathValue("projectId")
	if projectId == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}

	project, err := h.projectService.GetProjectByHandle(projectId, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			return apperror.ProjectNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get project %s in org %s", projectId, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, project)
	return nil
}

// ListProjects handles GET /api/v0.9/projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projects, err := h.projectService.GetProjectsByOrganization(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			return apperror.OrganizationNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list projects in org %s", orgID))
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
	return nil
}

// UpdateProject handles PUT /api/v0.9/projects/:projectId
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectId := r.PathValue("projectId")
	if projectId == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}

	var req api.Project
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	var reqId string
	if req.Id != nil {
		reqId = *req.Id
	}
	if err := utils.ValidateHandleImmutableRequired(projectId, reqId); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Project id is immutable and cannot be changed")
	}

	actor, err := resolveActorErr(r, h.identity, "update project")
	if err != nil {
		return err
	}
	project, err := h.projectService.UpdateProject(projectId, &req, orgID, actor)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			return apperror.ProjectNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrHandleImmutable) {
			return apperror.ValidationFailed.Wrap(err, "Project id is immutable and cannot be changed")
		}
		if errors.Is(err, constants.ErrProjectExists) {
			return apperror.ProjectExists.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to update project %s in org %s", projectId, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, project)
	return nil
}

// DeleteProject handles DELETE /api/v0.9/projects/:projectId
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectId := r.PathValue("projectId")
	if projectId == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}

	actor, err := resolveActorErr(r, h.identity, "delete project")
	if err != nil {
		return err
	}
	if err := h.projectService.DeleteProject(projectId, orgID, actor); err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			return apperror.ProjectNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrOrganizationMustHAveAtLeastOneProject) {
			return apperror.ValidationFailed.Wrap(err, "Organization must have at least one project")
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedAPIs) {
			return apperror.ValidationFailed.Wrap(err, "Project has associated APIs")
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedMCPProxies) {
			return apperror.ValidationFailed.Wrap(err, "Project has associated MCP proxies")
		}
		if errors.Is(err, constants.ErrProjectHasAssociatedApplications) {
			return apperror.ValidationFailed.Wrap(err, "Project has associated applications")
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete project %s in org %s", projectId, orgID))
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

func (h *ProjectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+constants.APIBasePath+"/projects", middleware.MapErrors(h.slogger, h.ListProjects))
	mux.HandleFunc("POST "+constants.APIBasePath+"/projects", middleware.MapErrors(h.slogger, h.CreateProject))
	mux.HandleFunc("GET "+constants.APIBasePath+"/projects/{projectId}", middleware.MapErrors(h.slogger, h.GetProject))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/projects/{projectId}", middleware.MapErrors(h.slogger, h.UpdateProject))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/projects/{projectId}", middleware.MapErrors(h.slogger, h.DeleteProject))
}
