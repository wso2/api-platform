package repository

import (
	"platform-api/src/internal/model"
)

// OrganizationRepository defines the interface for organization data access
type OrganizationRepository interface {
	Create(org *model.Organization) error
	GetByUUID(uuid string) (*model.Organization, error)
	GetByHandle(handle string) (*model.Organization, error)
	Update(org *model.Organization) error
	Delete(uuid string) error
	List(limit, offset int) ([]*model.Organization, error)
}

// ProjectRepository defines the interface for project data access
type ProjectRepository interface {
	Create(project *model.Project) error
	GetByUUID(uuid string) (*model.Project, error)
	GetByOrganizationID(orgID string) ([]*model.Project, error)
	GetDefaultByOrganizationID(orgID string) (*model.Project, error)
	Update(project *model.Project) error
	Delete(uuid string) error
	List(limit, offset int) ([]*model.Project, error)
}
