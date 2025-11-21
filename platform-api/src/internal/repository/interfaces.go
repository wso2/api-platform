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

package repository

import (
	"platform-api/src/internal/model"
)

// OrganizationRepository defines the interface for organization data access
type OrganizationRepository interface {
	CreateOrganization(org *model.Organization) error
	GetOrganizationByIdOrHandle(id, handle string) (*model.Organization, error)
	GetOrganizationByUUID(orgId string) (*model.Organization, error)
	GetOrganizationByHandle(handle string) (*model.Organization, error)
	UpdateOrganization(org *model.Organization) error
	DeleteOrganization(orgId string) error
	ListOrganizations(limit, offset int) ([]*model.Organization, error)
}

// ProjectRepository defines the interface for project data access
type ProjectRepository interface {
	CreateProject(project *model.Project) error
	GetProjectByUUID(projectId string) (*model.Project, error)
	GetProjectByNameAndOrgID(name, orgID string) (*model.Project, error)
	GetProjectsByOrganizationID(orgID string) ([]*model.Project, error)
	UpdateProject(project *model.Project) error
	DeleteProject(projectId string) error
	ListProjects(orgID string, limit, offset int) ([]*model.Project, error)
}

// APIRepository defines the interface for API data operations
type APIRepository interface {
	CreateAPI(api *model.API) error
	GetAPIByUUID(apiId string) (*model.API, error)
	GetAPIsByProjectID(projectID string) ([]*model.API, error)
	GetAPIsByOrganizationID(orgID string, projectID *string) ([]*model.API, error)
	GetAPIsByGatewayID(gatewayID, organizationID string) ([]*model.API, error)
	GetDeployedAPIsByGatewayID(gatewayID, organizationID string) ([]*model.API, error)
	UpdateAPI(api *model.API) error
	DeleteAPI(apiId string) error
	CreateDeployment(deployment *model.APIDeployment) error
	GetDeploymentsByAPIUUID(apiId string) ([]*model.APIDeployment, error)

	// API-Gateway association methods
	GetAPIGatewaysWithDetails(apiId, organizationId string) ([]*model.APIGatewayWithDetails, error)

	// Unified API association methods (supports both gateways and dev portals)
	CreateAPIAssociation(association *model.APIAssociation) error
	GetAPIAssociations(apiId, associationType, orgId string) ([]*model.APIAssociation, error)
	UpdateAPIAssociation(apiId, resourceId, associationType, orgId string) error
}

// BackendServiceRepository defines the interface for backend service data operations
type BackendServiceRepository interface {
	CreateBackendService(service *model.BackendService) error
	GetBackendServiceByUUID(serviceId string) (*model.BackendService, error)
	GetBackendServicesByOrganizationID(orgID string) ([]*model.BackendService, error)
	GetBackendServiceByNameAndOrgID(name, orgID string) (*model.BackendService, error)
	UpdateBackendService(service *model.BackendService) error
	DeleteBackendService(serviceId string) error

	// API-Backend Service associations
	AssociateBackendServiceWithAPI(apiId, backendServiceId string, isDefault bool) error
	DisassociateBackendServiceFromAPI(apiId, backendServiceId string) error
	GetBackendServicesByAPIID(apiId string) ([]*model.BackendService, error)
	GetAPIsByBackendServiceID(backendServiceId string) ([]string, error)
}

// GatewayRepository defines the interface for gateway data access
type GatewayRepository interface {
	// Gateway operations
	Create(gateway *model.Gateway) error
	GetByUUID(gatewayId string) (*model.Gateway, error)
	GetByOrganizationID(orgID string) ([]*model.Gateway, error)
	GetByNameAndOrgID(name, orgID string) (*model.Gateway, error)
	List() ([]*model.Gateway, error)
	Delete(gatewayID, organizationID string) error
	UpdateGateway(gateway *model.Gateway) error
	UpdateActiveStatus(gatewayId string, isActive bool) error

	// Gateway association checking operations
	HasGatewayAPIDeployments(gatewayID, organizationID string) (bool, error)
	HasGatewayAPIAssociations(gatewayID, organizationID string) (bool, error)
	HasGatewayAssociations(gatewayID, organizationID string) (bool, error)

	// Token operations
	CreateToken(token *model.GatewayToken) error
	GetActiveTokensByGatewayUUID(gatewayId string) ([]*model.GatewayToken, error)
	GetTokenByUUID(tokenId string) (*model.GatewayToken, error)
	RevokeToken(tokenId string) error
	CountActiveTokens(gatewayId string) (int, error)
}

// DevPortalRepository interface for DevPortal-related database operations
type DevPortalRepository interface {
	// Basic CRUD operations
	Create(devPortal *model.DevPortal) error
	GetByUUID(uuid, orgUUID string) (*model.DevPortal, error)
	GetByOrganizationUUID(orgUUID string, isDefault, isActive *bool, limit, offset int) ([]*model.DevPortal, error)
	Update(devPortal *model.DevPortal, orgUUID string) error
	Delete(uuid, orgUUID string) error

	// Special operations
	GetDefaultByOrganizationUUID(orgUUID string) (*model.DevPortal, error)
	CountByOrganizationUUID(orgUUID string, isDefault, isActive *bool) (int, error)
	UpdateEnabledStatus(uuid, orgUUID string, isEnabled bool) error
	SetAsDefault(uuid, orgUUID string) error
}

// APIPublicationRepository interface defines operations for API publication tracking
type APIPublicationRepository interface {
	// Basic CRUD operations
	Create(publication *model.APIPublication) error
	GetByAPIAndDevPortal(apiUUID, devPortalUUID, orgUUID string) (*model.APIPublication, error)
	GetByAPIUUID(apiUUID, orgUUID string) ([]*model.APIPublication, error)
	Update(publication *model.APIPublication) error
	Delete(apiUUID, devPortalUUID, orgUUID string) error
	UpsertPublication(publication *model.APIPublication) error
	GetAPIDevPortalsWithDetails(apiUUID, orgUUID string) ([]*model.APIDevPortalWithDetails, error)
}
