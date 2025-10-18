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
	"github.com/google/uuid"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"time"
)

type ProjectService struct {
	projectRepo repository.ProjectRepository
	orgRepo     repository.OrganizationRepository
	apiRepo     repository.APIRepository
}

func NewProjectService(projectRepo repository.ProjectRepository, orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
		apiRepo:     apiRepo,
	}
}

func (s *ProjectService) CreateProject(name, organizationID string) (*dto.Project, error) {
	// Validate project name
	if name == "" {
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

	for _, existingProject := range existingProjects {
		if existingProject.Name == name {
			return nil, constants.ErrProjectExists
		}
	}

	// CreateOrganization project
	project := &dto.Project{
		ID:             uuid.New().String(),
		Name:           name,
		OrganizationID: organizationID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	projectModel := s.DtoToModel(project)
	err = s.projectRepo.CreateProject(projectModel)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) GetProjectByID(uuid string) (*dto.Project, error) {
	projectModel, err := s.projectRepo.GetProjectByUUID(uuid)
	if err != nil {
		return nil, err
	}

	if projectModel == nil {
		return nil, constants.ErrProjectNotFound
	}

	project := s.ModelToDTO(projectModel)
	return project, nil
}

func (s *ProjectService) GetProjectsByOrganization(organizationID string) ([]*dto.Project, error) {
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

	projects := make([]*dto.Project, 0)
	for _, projectModel := range projectModels {
		projects = append(projects, s.ModelToDTO(projectModel))
	}
	return projects, nil
}

func (s *ProjectService) UpdateProject(uuid string, name string) (*dto.Project, error) {
	// Get existing project
	project, err := s.projectRepo.GetProjectByUUID(uuid)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}

	// If name is being changed, check for duplicates in the organization
	if name != "" && name != project.Name {
		existingProjects, err := s.projectRepo.GetProjectsByOrganizationID(project.OrganizationID)
		if err != nil {
			return nil, err
		}

		for _, existingProject := range existingProjects {
			if existingProject.Name == name && existingProject.ID != uuid {
				return nil, constants.ErrProjectExists
			}
		}
		project.Name = name
	}

	project.UpdatedAt = time.Now()

	err = s.projectRepo.UpdateProject(project)
	if err != nil {
		return nil, err
	}

	updatedProject := s.ModelToDTO(project)
	return updatedProject, nil
}

func (s *ProjectService) DeleteProject(uuid string) error {
	// Check if project exists
	project, err := s.projectRepo.GetProjectByUUID(uuid)
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

	// check if there are any APIs associated with the project
	apis, err := s.apiRepo.GetAPIsByProjectID(uuid)
	if err != nil {
		return err
	}
	if len(apis) > 0 {
		return constants.ErrProjectHasAssociatedAPIs
	}

	return s.projectRepo.DeleteProject(uuid)
}

// Mapping functions
func (s *ProjectService) DtoToModel(dto *dto.Project) *model.Project {
	if dto == nil {
		return nil
	}

	return &model.Project{
		ID:             dto.ID,
		Name:           dto.Name,
		OrganizationID: dto.OrganizationID,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
	}
}

func (s *ProjectService) ModelToDTO(model *model.Project) *dto.Project {
	if model == nil {
		return nil
	}

	return &dto.Project{
		ID:             model.ID,
		Name:           model.Name,
		OrganizationID: model.OrganizationID,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
