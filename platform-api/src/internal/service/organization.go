package service

import (
	"errors"
	"github.com/google/uuid"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"regexp"
	"strings"
	"time"
)

var (
	ErrHandleExists          = errors.New("handle already exists")
	ErrInvalidHandle         = errors.New("invalid handle format")
	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrMultipleOrganizations = errors.New("multiple organizations found")
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

func (s *OrganizationService) CreateOrganization(handle string, name string) (*model.Organization, error) {
	// Validate handle is URL friendly
	if !s.isURLFriendly(handle) {
		return nil, ErrInvalidHandle
	}

	// Check if handle already exists
	existingOrg, err := s.orgRepo.GetByHandle(handle)
	if err != nil {
		return nil, err
	}
	if existingOrg != nil {
		return nil, ErrHandleExists
	}

	if name == "" {
		name = handle // Default name to handle if not provided
	}

	// Create organization
	org := &model.Organization{
		UUID:      uuid.New().String(),
		Handle:    handle,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.orgRepo.Create(org)
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

	err = s.projectRepo.Create(defaultProject)
	if err != nil {
		// If project creation fails, roll back the organization creation
		return org, err
	}

	return org, nil
}

func (s *OrganizationService) GetOrganizationByUUID(uuid string) (*model.Organization, error) {
	org, err := s.orgRepo.GetByUUID(uuid)
	if err != nil {
		return nil, err
	}

	if org == nil {
		return nil, ErrOrganizationNotFound
	}

	return org, nil
}

func (s *OrganizationService) isURLFriendly(handle string) bool {
	// URL friendly: lowercase letters, numbers, hyphens, underscores
	// Must start with letter, no consecutive special chars
	pattern := `^[a-z][a-z0-9_-]*[a-z0-9]$|^[a-z]$`
	matched, _ := regexp.MatchString(pattern, strings.ToLower(handle))
	return matched && handle == strings.ToLower(handle)
}
