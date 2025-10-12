package repository

import (
	"platform-api/src/internal/model"
)

// OrganizationRepository defines the interface for organization data access
type OrganizationRepository interface {
	CreateOrganization(org *model.Organization) error
	GetOrganizationByUUID(uuid string) (*model.Organization, error)
	GetOrganizationByHandle(handle string) (*model.Organization, error)
	UpdateOrganization(org *model.Organization) error
	DeleteOrganization(uuid string) error
	ListOrganizations(limit, offset int) ([]*model.Organization, error)
}

// ProjectRepository defines the interface for project data access
type ProjectRepository interface {
	CreateProject(project *model.Project) error
	GetProjectByUUID(uuid string) (*model.Project, error)
	GetProjectByOrganizationID(orgID string) ([]*model.Project, error)
	GetDefaultProjectByOrganizationID(orgID string) (*model.Project, error)
	UpdateProject(project *model.Project) error
	DeleteProject(uuid string) error
	ListProjects(orgID string, limit, offset int) ([]*model.Project, error)
}

// APIRepository defines the interface for API data operations
type APIRepository interface {
	CreateAPI(api *model.API) error
	GetAPIByUUID(uuid string) (*model.API, error)
	GetAPIsByProjectID(projectID string) ([]*model.API, error)
	UpdateAPI(api *model.API) error
	DeleteAPI(uuid string) error
}
