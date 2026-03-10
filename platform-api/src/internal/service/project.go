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

type ProjectService struct {
	projectRepo repository.ProjectRepository
	orgRepo     repository.OrganizationRepository
	apiRepo     repository.APIRepository
	slogger     *slog.Logger
}

func NewProjectService(projectRepo repository.ProjectRepository, orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository, slogger *slog.Logger) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
		apiRepo:     apiRepo,
		slogger:     slogger,
	}
}

func (s *ProjectService) CreateProject(req *api.CreateProjectRequest, organizationID string) (*api.Project, error) {
	// Validate project name
	if req.Name == "" {
		return nil, constants.ErrInvalidProjectName
	}

	// Check if organization exists
	org, err := s.orgRepo.GetOrganizationByUUID(organizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	// Check if project name already exists in the organization
	existingProjects, err := s.projectRepo.GetProjectsByOrganizationID(organizationID)
	if err != nil {
		return nil, err
	}

	// Generate new project ID or use provided one
	var projectID string
	if req.Id != nil {
		projectID = utils.OpenAPIUUIDToString(*req.Id)
	} else {
		projectID, err = utils.GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate project ID: %w", err)
		}
	}

	for _, existingProject := range existingProjects {
		if existingProject.Name == req.Name || (req.Id != nil && existingProject.ID == projectID) {
			return nil, constants.ErrProjectExists
		}
	}

	orgUUID, err := utils.ParseOpenAPIUUID(organizationID)
	if err != nil {
		return nil, err
	}

	projectUUID, err := utils.ParseOpenAPIUUID(projectID)
	if err != nil {
		return nil, err
	}

	project := &api.Project{
		Id:             projectUUID,
		Name:           req.Name,
		OrganizationId: orgUUID,
		Description:    req.Description,
		CreatedAt:      utils.TimePtrIfNotZero(time.Now()),
		UpdatedAt:      utils.TimePtrIfNotZero(time.Now()),
	}

	projectModel := s.apiToModel(project)
	err = s.projectRepo.CreateProject(projectModel)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) GetProjectByID(projectId, orgId string) (*api.Project, error) {
	projectModel, err := s.projectRepo.GetProjectByUUID(projectId)
	if err != nil {
		return nil, err
	}

	if projectModel == nil {
		return nil, constants.ErrProjectNotFound
	}
	if projectModel.OrganizationID != orgId {
		return nil, constants.ErrProjectNotFound
	}

	project := s.modelToAPI(projectModel)
	return project, nil
}

func (s *ProjectService) GetProjectsByOrganization(organizationID string) ([]api.Project, error) {
	// Check if organization exists
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

func (s *ProjectService) UpdateProject(projectId string, req *api.UpdateProjectRequest, orgId string) (*api.Project, error) {
	// Get existing project
	project, err := s.projectRepo.GetProjectByUUID(projectId)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}
	if project.OrganizationID != orgId {
		return nil, constants.ErrProjectNotFound
	}

	// If name is being changed, check for duplicates in the organization
	if req.Name != nil && *req.Name != "" && *req.Name != project.Name {
		existingProjects, err := s.projectRepo.GetProjectsByOrganizationID(project.OrganizationID)
		if err != nil {
			return nil, err
		}

		for _, existingProject := range existingProjects {
			if existingProject.Name == *req.Name && existingProject.ID != projectId {
				return nil, constants.ErrProjectExists
			}
		}
		project.Name = *req.Name
	}

	if req.Description != nil {
		project.Description = *req.Description
	}
	project.UpdatedAt = time.Now()

	err = s.projectRepo.UpdateProject(project)
	if err != nil {
		return nil, err
	}

	updatedProject := s.modelToAPI(project)
	return updatedProject, nil
}

func (s *ProjectService) DeleteProject(projectId, orgId string) error {
	// Check if project exists
	project, err := s.projectRepo.GetProjectByUUID(projectId)
	if err != nil {
		return err
	}
	if project == nil {
		return constants.ErrProjectNotFound
	}
	if project.OrganizationID != orgId {
		return constants.ErrProjectNotFound
	}

	projects, err := s.projectRepo.GetProjectsByOrganizationID(project.OrganizationID)
	if err != nil {
		return err
	}
	if len(projects) <= 1 {
		return constants.ErrOrganizationMustHAveAtLeastOneProject
	}

	// check if there are any APIs associated with the project
	apis, err := s.apiRepo.GetAPIsByProjectUUID(projectId, orgId)
	if err != nil {
		return err
	}
	if len(apis) > 0 {
		return constants.ErrProjectHasAssociatedAPIs
	}

	return s.projectRepo.DeleteProject(projectId)
}

// Mapping functions
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

	projectID := ""
	if project.Id != nil {
		projectID = utils.OpenAPIUUIDToString(*project.Id)
	}

	organizationID := ""
	if project.OrganizationId != nil {
		organizationID = utils.OpenAPIUUIDToString(*project.OrganizationId)
	}

	return &model.Project{
		ID:             projectID,
		Name:           project.Name,
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

	projectID, err := utils.ParseOpenAPIUUID(projectModel.ID)
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

	return &api.Project{
		Id:             projectID,
		Name:           projectModel.Name,
		OrganizationId: orgID,
		Description:    description,
		CreatedAt:      utils.TimePtrIfNotZero(projectModel.CreatedAt),
		UpdatedAt:      utils.TimePtrIfNotZero(projectModel.UpdatedAt),
	}
}
