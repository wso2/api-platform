package service

import (
	"errors"
	"github.com/google/uuid"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"time"
)

var (
	ErrProjectNameExists    = errors.New("project name already exists in organization")
	ErrProjectNotFound      = errors.New("project not found")
	ErrInvalidProjectName   = errors.New("invalid project name")
	ErrOrganizationRequired = errors.New("organization is required")
)

type ProjectService struct {
	projectRepo repository.ProjectRepository
	orgRepo     repository.OrganizationRepository
}

func NewProjectService(projectRepo repository.ProjectRepository, orgRepo repository.OrganizationRepository) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
	}
}

func (s *ProjectService) CreateProject(name, organizationID string, isDefault bool) (*model.Project, error) {
	// Validate project name
	if name == "" {
		return nil, ErrInvalidProjectName
	}

	// Check if organization exists
	org, err := s.orgRepo.GetByUUID(organizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}

	// Check if project name already exists in the organization
	existingProjects, err := s.projectRepo.GetByOrganizationID(organizationID)
	if err != nil {
		return nil, err
	}

	for _, existingProject := range existingProjects {
		if existingProject.Name == name {
			return nil, ErrProjectNameExists
		}
	}

	// If this is set as default, unset other defaults in the organization
	if isDefault {
		defaultProject, err := s.projectRepo.GetDefaultByOrganizationID(organizationID)
		if err != nil {
			return nil, err
		}
		if defaultProject != nil {
			defaultProject.IsDefault = false
			if err := s.projectRepo.Update(defaultProject); err != nil {
				return nil, err
			}
		}
	}

	// Create project
	project := &model.Project{
		UUID:           uuid.New().String(),
		Name:           name,
		OrganizationID: organizationID,
		IsDefault:      isDefault,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.projectRepo.Create(project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) GetProjectByID(uuid string) (*model.Project, error) {
	project, err := s.projectRepo.GetByUUID(uuid)
	if err != nil {
		return nil, err
	}

	if project == nil {
		return nil, ErrProjectNotFound
	}

	return project, nil
}

func (s *ProjectService) GetProjectsByOrganization(organizationID string) ([]*model.Project, error) {
	// Check if organization exists
	org, err := s.orgRepo.GetByUUID(organizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}

	projects, err := s.projectRepo.GetByOrganizationID(organizationID)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (s *ProjectService) UpdateProject(uuid string, name string, isDefault bool) (*model.Project, error) {
	// Get existing project
	project, err := s.projectRepo.GetByUUID(uuid)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, ErrProjectNotFound
	}

	// If name is being changed, check for duplicates in the organization
	if name != "" && name != project.Name {
		existingProjects, err := s.projectRepo.GetByOrganizationID(project.OrganizationID)
		if err != nil {
			return nil, err
		}

		for _, existingProject := range existingProjects {
			if existingProject.Name == name && existingProject.UUID != uuid {
				return nil, ErrProjectNameExists
			}
		}
		project.Name = name
	}

	// If this is being set as default, unset other defaults in the organization
	if isDefault && !project.IsDefault {
		defaultProject, err := s.projectRepo.GetDefaultByOrganizationID(project.OrganizationID)
		if err != nil {
			return nil, err
		}
		if defaultProject != nil && defaultProject.UUID != uuid {
			defaultProject.IsDefault = false
			if err := s.projectRepo.Update(defaultProject); err != nil {
				return nil, err
			}
		}
	}

	project.IsDefault = isDefault
	project.UpdatedAt = time.Now()

	err = s.projectRepo.Update(project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) DeleteProject(uuid string) error {
	// Check if project exists
	project, err := s.projectRepo.GetByUUID(uuid)
	if err != nil {
		return err
	}
	if project == nil {
		return ErrProjectNotFound
	}

	return s.projectRepo.Delete(uuid)
}
