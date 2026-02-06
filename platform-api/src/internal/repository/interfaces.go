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
	"time"
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
	GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error)
	GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error)
	GetAPIsByProjectUUID(projectUUID, orgUUID string) ([]*model.API, error)
	GetAPIsByOrganizationUUID(orgUUID string, projectUUID *string) ([]*model.API, error)
	GetAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error)
	GetDeployedAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error)
	UpdateAPI(api *model.API) error
	DeleteAPI(apiUUID, orgUUID string) error

	// Deployment artifact methods (immutable deployments)
	CreateDeploymentWithLimitEnforcement(deployment *model.APIDeployment, hardLimit int) error    // Atomic: count, cleanup if needed, create
	GetDeploymentWithContent(deploymentID, apiUUID, orgUUID string) (*model.APIDeployment, error) // With content (for rollback/base deployment)
	GetDeploymentWithState(deploymentID, apiUUID, orgUUID string) (*model.APIDeployment, error)   // With status derived (without content - lightweight)
	GetDeploymentsWithState(apiUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.APIDeployment, error)
	DeleteDeployment(deploymentID, apiUUID, orgUUID string) error
	GetCurrentDeploymentByGateway(apiUUID, gatewayID, orgUUID string) (*model.APIDeployment, error)

	// Deployment status methods (mutable state tracking)
	SetCurrentDeployment(apiUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (updatedAt time.Time, err error)
	GetDeploymentStatus(apiUUID, orgUUID, gatewayID string) (deploymentID string, status model.DeploymentStatus, updatedAt *time.Time, err error)
	DeleteDeploymentStatus(apiUUID, orgUUID, gatewayID string) error

	// API-Gateway association methods
	GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error)

	// Unified API association methods (supports both gateways and dev portals)
	CreateAPIAssociation(association *model.APIAssociation) error
	GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error)
	UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID string) error

	// API name validation methods
	CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error)
	CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, excludeHandle string) (bool, error)
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

// LLMProviderTemplateRepository defines the interface for LLM provider template persistence
type LLMProviderTemplateRepository interface {
	Create(t *model.LLMProviderTemplate) error
	GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error)
	List(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error)
	Count(orgUUID string) (int, error)
	Update(t *model.LLMProviderTemplate) error
	Delete(templateID, orgUUID string) error
	Exists(templateID, orgUUID string) (bool, error)
}

// LLMProviderRepository defines the interface for LLM provider persistence
type LLMProviderRepository interface {
	Create(p *model.LLMProvider) error
	GetByID(providerID, orgUUID string) (*model.LLMProvider, error)
	List(orgUUID string, limit, offset int) ([]*model.LLMProvider, error)
	Count(orgUUID string) (int, error)
	Update(p *model.LLMProvider) error
	Delete(providerID, orgUUID string) error
	Exists(providerID, orgUUID string) (bool, error)
}

// LLMProxyRepository defines the interface for LLM proxy persistence
type LLMProxyRepository interface {
	Create(p *model.LLMProxy) error
	GetByID(proxyID, orgUUID string) (*model.LLMProxy, error)
	List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error)
	ListByProject(orgUUID, projectUUID string, limit, offset int) ([]*model.LLMProxy, error)
	ListByProvider(orgUUID, providerID string, limit, offset int) ([]*model.LLMProxy, error)
	Count(orgUUID string) (int, error)
	CountByProject(orgUUID, projectUUID string) (int, error)
	CountByProvider(orgUUID, providerID string) (int, error)
	Update(p *model.LLMProxy) error
	Delete(proxyID, orgUUID string) error
	Exists(proxyID, orgUUID string) (bool, error)
}
