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
}

func NewProjectService(projectRepo repository.ProjectRepository, orgRepo repository.OrganizationRepository) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
	}
}

func (s *ProjectService) CreateProject(name, organizationID string, isDefault bool) (*dto.Project, error) {
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
	existingProjects, err := s.projectRepo.GetProjectByOrganizationID(organizationID)
	if err != nil {
		return nil, err
	}

	for _, existingProject := range existingProjects {
		if existingProject.Name == name {
			return nil, constants.ErrProjectNameExists
		}
	}

	// If this is set as default check if there is an existing default and if there is throw an error
	if isDefault {
		defaultProject, err := s.projectRepo.GetDefaultProjectByOrganizationID(organizationID)
		if err != nil {
			return nil, err
		}
		if defaultProject != nil {
			return nil, constants.ErrDefaultProjectAlreadyExists
		}
	}

	// CreateOrganization project
	project := &dto.Project{
		UUID:           uuid.New().String(),
		Name:           name,
		OrganizationID: organizationID,
		IsDefault:      isDefault,
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

	projectModels, err := s.projectRepo.GetProjectByOrganizationID(organizationID)
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
		existingProjects, err := s.projectRepo.GetProjectByOrganizationID(project.OrganizationID)
		if err != nil {
			return nil, err
		}

		for _, existingProject := range existingProjects {
			if existingProject.Name == name && existingProject.UUID != uuid {
				return nil, constants.ErrProjectNameExists
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
	if project.IsDefault {
		return constants.ErrCannotDeleteDefaultProject
	}

	return s.projectRepo.DeleteProject(uuid)
}

// Mapping functions
func (s *ProjectService) DtoToModel(dto *dto.Project) *model.Project {
	if dto == nil {
		return nil
	}

	return &model.Project{
		UUID:           dto.UUID,
		Name:           dto.Name,
		OrganizationID: dto.OrganizationID,
		IsDefault:      dto.IsDefault,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
	}
}

func (s *ProjectService) ModelToDTO(model *model.Project) *dto.Project {
	if model == nil {
		return nil
	}

	return &dto.Project{
		UUID:           model.UUID,
		Name:           model.Name,
		OrganizationID: model.OrganizationID,
		IsDefault:      model.IsDefault,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
