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

package service

import (
	"fmt"
	"log/slog"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"time"
)

// ProjectDeletionGuard is implemented by plugins that need to block project
// deletion when plugin-managed resources still exist under the project.
type ProjectDeletionGuard interface {
	CheckProjectDeletion(orgID, projectID string) error
}

type ProjectService struct {
	projectRepo    repository.ProjectRepository
	orgRepo        repository.OrganizationRepository
	apiRepo        repository.APIRepository
	mcpProxyRepo   repository.MCPProxyRepository
	deletionGuards []ProjectDeletionGuard
	auditRepo      repository.AuditRepository
	slogger        *slog.Logger
}

func NewProjectService(projectRepo repository.ProjectRepository, orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository, mcpProxyRepo repository.MCPProxyRepository,
	auditRepo repository.AuditRepository, slogger *slog.Logger) *ProjectService {
	return &ProjectService{
		projectRepo:  projectRepo,
		orgRepo:      orgRepo,
		apiRepo:      apiRepo,
		mcpProxyRepo: mcpProxyRepo,
		auditRepo:    auditRepo,
		slogger:      slogger,
	}
}

// RegisterDeletionGuard adds a guard that is consulted during DeleteProject.
// Plugins call this to block deletion when they own resources under the project.
func (s *ProjectService) RegisterDeletionGuard(guard ProjectDeletionGuard) {
	s.deletionGuards = append(s.deletionGuards, guard)
}

func (s *ProjectService) CreateProject(req *api.CreateProjectRequest, organizationID, actor string) (*api.Project, error) {
	if req.DisplayName == "" {
		return nil, constants.ErrInvalidProjectName
	}

	org, err := s.orgRepo.GetOrganizationByUUID(organizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	// Determine handle: use provided id or auto-generate from displayName
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
		existing, err := s.projectRepo.GetProjectByHandleAndOrgID(handle, organizationID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, constants.ErrProjectExists
		}
	} else {
		handle, err = utils.GenerateHandle(req.DisplayName, func(h string) bool {
			existing, _ := s.projectRepo.GetProjectByHandleAndOrgID(h, organizationID)
			return existing != nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate project handle: %w", err)
		}
	}

	// Check for duplicate displayName
	existingProjects, err := s.projectRepo.GetProjectsByOrganizationID(organizationID)
	if err != nil {
		return nil, err
	}
	for _, p := range existingProjects {
		if p.Name == req.DisplayName {
			return nil, constants.ErrProjectExists
		}
	}

	projectID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate project ID: %w", err)
	}

	orgUUID, err := utils.ParseOpenAPIUUID(organizationID)
	if err != nil {
		return nil, err
	}

	var description *string
	if req.Description != nil {
		description = req.Description
	}

	project := &api.Project{
		Id:             &handle,
		DisplayName:    req.DisplayName,
		OrganizationId: orgUUID,
		Description:    description,
		CreatedAt:      utils.TimePtrIfNotZero(time.Now()),
		UpdatedAt:      utils.TimePtrIfNotZero(time.Now()),
	}

	projectModel := s.apiToModel(project)
	projectModel.ID = projectID
	projectModel.Handle = handle
	projectModel.CreatedBy = actor
	projectModel.UpdatedBy = actor

	if err = s.projectRepo.CreateProject(projectModel); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", projectModel.ID, "project", organizationID, actor)

	return project, nil
}

func (s *ProjectService) GetProjectByHandle(handle, orgId string) (*api.Project, error) {
	projectModel, err := s.projectRepo.GetProjectByHandleAndOrgID(handle, orgId)
	if err != nil {
		return nil, err
	}
	if projectModel == nil {
		return nil, constants.ErrProjectNotFound
	}

	return s.modelToAPI(projectModel), nil
}

func (s *ProjectService) GetProjectsByOrganization(organizationID string) ([]api.Project, error) {
	org, err := s.orgRepo.GetOrganizationByUUID(organizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	projectModels, err := s.projectRepo.GetProjectsByOrganizationID(organizationID)
	if err != nil {
		return nil, err
	}

	projects := make([]api.Project, 0)
	for _, projectModel := range projectModels {
		apiProj := s.modelToAPI(projectModel)
		if apiProj == nil {
			s.slogger.Warn("Failed to convert project model to API", "organizationId", organizationID)
			continue
		}
		projects = append(projects, *apiProj)
	}
	return projects, nil
}

func (s *ProjectService) UpdateProject(handle string, req *api.UpdateProjectRequest, orgId, actor string) (*api.Project, error) {
	project, err := s.projectRepo.GetProjectByHandleAndOrgID(handle, orgId)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}

	// Validate that the handle in the body matches the path param (immutability check)
	if req.Id != handle {
		return nil, constants.ErrHandleImmutable
	}

	if req.DisplayName != nil && *req.DisplayName != "" && *req.DisplayName != project.Name {
		existingProjects, err := s.projectRepo.GetProjectsByOrganizationID(project.OrganizationID)
		if err != nil {
			return nil, err
		}
		for _, existingProject := range existingProjects {
			if existingProject.Name == *req.DisplayName && existingProject.Handle != handle {
				return nil, constants.ErrProjectExists
			}
		}
		project.Name = *req.DisplayName
	}

	if req.Description != nil {
		project.Description = *req.Description
	}
	project.UpdatedAt = time.Now()
	project.UpdatedBy = actor

	if err = s.projectRepo.UpdateProject(project); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("UPDATE", project.ID, "project", orgId, actor)

	return s.modelToAPI(project), nil
}

func (s *ProjectService) DeleteProject(handle, orgId, actor string) error {
	project, err := s.projectRepo.GetProjectByHandleAndOrgID(handle, orgId)
	if err != nil {
		return err
	}
	if project == nil {
		return constants.ErrProjectNotFound
	}

	projects, err := s.projectRepo.GetProjectsByOrganizationID(project.OrganizationID)
	if err != nil {
		return err
	}
	if len(projects) <= 1 {
		return constants.ErrOrganizationMustHAveAtLeastOneProject
	}

	apis, err := s.apiRepo.GetAPIsByProjectUUID(project.ID, orgId)
	if err != nil {
		return err
	}
	if len(apis) > 0 {
		return constants.ErrProjectHasAssociatedAPIs
	}

	mcpProxiesCount, err := s.mcpProxyRepo.CountByProject(orgId, project.ID)
	if err != nil {
		return err
	}
	if mcpProxiesCount > 0 {
		return constants.ErrProjectHasAssociatedMCPProxies
	}

	for _, guard := range s.deletionGuards {
		if err := guard.CheckProjectDeletion(orgId, project.ID); err != nil {
			return err
		}
	}

	if err := s.projectRepo.DeleteProject(project.ID); err != nil {
		return err
	}
	_ = s.auditRepo.Record("DELETE", project.ID, "project", orgId, actor)
	return nil
}

func (s *ProjectService) apiToModel(project *api.Project) *model.Project {
	if project == nil {
		return nil
	}

	createdAt := time.Now()
	if project.CreatedAt != nil {
		createdAt = *project.CreatedAt
	}

	updatedAt := time.Now()
	if project.UpdatedAt != nil {
		updatedAt = *project.UpdatedAt
	}

	var description string
	if project.Description != nil {
		description = *project.Description
	}

	var handle string
	if project.Id != nil {
		handle = *project.Id
	}

	organizationID := ""
	if project.OrganizationId != nil {
		organizationID = utils.OpenAPIUUIDToString(*project.OrganizationId)
	}

	return &model.Project{
		Handle:         handle,
		Name:           project.DisplayName,
		OrganizationID: organizationID,
		Description:    description,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

func (s *ProjectService) modelToAPI(projectModel *model.Project) *api.Project {
	if projectModel == nil {
		return nil
	}

	projectUUID, err := utils.ParseOpenAPIUUID(projectModel.ID)
	if err != nil {
		return nil
	}

	orgID, err := utils.ParseOpenAPIUUID(projectModel.OrganizationID)
	if err != nil {
		return nil
	}

	var description *string
	if projectModel.Description != "" {
		description = &projectModel.Description
	}

	handle := projectModel.Handle
	return &api.Project{
		Id:             &handle,
		DisplayName:    projectModel.Name,
		Uuid:           projectUUID,
		OrganizationId: orgID,
		Description:    description,
		CreatedAt:      utils.TimePtrIfNotZero(projectModel.CreatedAt),
		UpdatedAt:      utils.TimePtrIfNotZero(projectModel.UpdatedAt),
	}
}
