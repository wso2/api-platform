package service

import (
	"github.com/google/uuid"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"regexp"
	"strings"
	"time"
)

type OrganizationService struct {
	orgRepo     repository.OrganizationRepository
	projectRepo repository.ProjectRepository
}

func NewOrganizationService(orgRepo repository.OrganizationRepository, projectRepo repository.ProjectRepository) *OrganizationService {
	return &OrganizationService{
		orgRepo:     orgRepo,
		projectRepo: projectRepo,
	}
}

func (s *OrganizationService) CreateOrganization(handle string, name string) (*dto.Organization, error) {
	// Validate handle is URL friendly
	if !s.isURLFriendly(handle) {
		return nil, constants.ErrInvalidHandle
	}

	// Check if handle already exists
	existingOrg, err := s.orgRepo.GetOrganizationByHandle(handle)
	if err != nil {
		return nil, err
	}
	if existingOrg != nil {
		return nil, constants.ErrHandleExists
	}

	if name == "" {
		name = handle // Default name to handle if not provided
	}

	// CreateOrganization organization
	org := &dto.Organization{
		UUID:      uuid.New().String(),
		Handle:    handle,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	orgModel := s.dtoToModel(org)
	err = s.orgRepo.CreateOrganization(orgModel)
	if err != nil {
		return nil, err
	}

	// Create default project for the organization
	defaultProject := &model.Project{
		Name:           "Default",
		OrganizationID: org.UUID,
		IsDefault:      true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.projectRepo.CreateProject(defaultProject)
	if err != nil {
		// If project creation fails, roll back the organization creation
		return org, err
	}

	return org, nil
}

func (s *OrganizationService) GetOrganizationByUUID(uuid string) (*dto.Organization, error) {
	orgModel, err := s.orgRepo.GetOrganizationByUUID(uuid)
	if err != nil {
		return nil, err
	}

	if orgModel == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	org := s.modelToDTO(orgModel)
	return org, nil
}

func (s *OrganizationService) isURLFriendly(handle string) bool {
	// URL friendly: lowercase letters, numbers, hyphens, underscores
	// Must start with letter, no consecutive special chars
	pattern := `^[a-z][a-z0-9_-]*[a-z0-9]$|^[a-z]$`
	matched, _ := regexp.MatchString(pattern, strings.ToLower(handle))
	return matched && handle == strings.ToLower(handle)
}

// Mapping functions
func (s *OrganizationService) dtoToModel(dto *dto.Organization) *model.Organization {
	if dto == nil {
		return nil
	}

	return &model.Organization{
		UUID:      dto.UUID,
		Handle:    dto.Handle,
		Name:      dto.Name,
		CreatedAt: dto.CreatedAt,
		UpdatedAt: dto.UpdatedAt,
	}
}

func (s *OrganizationService) modelToDTO(model *model.Organization) *dto.Organization {
	if model == nil {
		return nil
	}

	return &dto.Organization{
		UUID:      model.UUID,
		Handle:    model.Handle,
		Name:      model.Name,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
