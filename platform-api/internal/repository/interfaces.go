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
	"database/sql"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"time"
)

// OrganizationRepository defines the interface for organization data access
type OrganizationRepository interface {
	CreateOrganization(org *model.Organization) error
	GetOrganizationByIdOrHandle(id, handle string) (*model.Organization, error)
	GetOrganizationByUUID(orgId string) (*model.Organization, error)
	GetOrganizationByHandle(handle string) (*model.Organization, error)
	GetOrganizationByIdpOrgRefUUID(idpOrgRefUUID string) (*model.Organization, error)
	UpdateOrganization(org *model.Organization) error
	DeleteOrganization(orgId string) error
	ListOrganizations(limit, offset int) ([]*model.Organization, error)
	CountOrganizations() (int, error)
	ListOrganizationsForUser(userUUID string, limit, offset int) ([]*model.Organization, error)
	CountOrganizationsForUser(userUUID string) (int, error)
}

// ProjectRepository defines the interface for project data access
type ProjectRepository interface {
	CreateProject(project *model.Project) error
	GetProjectByUUID(projectId string) (*model.Project, error)
	GetProjectByNameAndOrgID(name, orgID string) (*model.Project, error)
	GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error)
	GetProjectsByOrganizationID(orgID string) ([]*model.Project, error)
	UpdateProject(project *model.Project) error
	DeleteProject(projectId string) error
	ListProjects(orgID string, opts ListOptions) ([]*model.Project, error)
	CountProjects(orgID, search string) (int, error)
}

type ArtifactRepository interface {
	Create(tx *sql.Tx, artifact *model.Artifact) error
	Delete(tx *sql.Tx, uuid string) error
	Update(tx *sql.Tx, artifact *model.Artifact) error
	Exists(kind, handle, orgUUID string) (bool, error)
	GetByHandle(handle, orgUUID string) (*model.Artifact, error)
	GetByUUID(uuid, orgUUID string) (*model.Artifact, error)
	GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error)
	GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error)
	GetMetadataByUUIDs(uuids []string, orgUUID string) (map[string]*model.APIMetadata, error)
	CountByKindAndOrg(kind, orgUUID string) (int, error)
	ExistsByUUIDs(uuids []string, orgUUID string) ([]string, error)
}

// ApplicationRepository defines the interface for application data access
type ApplicationRepository interface {
	CreateApplication(app *model.Application) error
	GetApplicationByUUID(appID string) (*model.Application, error)
	GetApplicationByIDOrHandle(appIDOrHandle, orgID string) (*model.Application, error)
	GetAssociationTargetByUUID(targetUUID, orgID string) (*model.Artifact, error)
	GetAssociationTargetByIDOrHandle(targetIDOrHandle, orgID string) (*model.Artifact, error)
	GetAssociationTargetByIDOrHandleAndKind(targetIDOrHandle, kind, orgID string) (*model.Artifact, error)
	GetLLMProxyProjectUUID(artifactUUID, orgID string) (string, error)
	GetApplicationsByProjectID(projectID, orgID string) ([]*model.Application, error)
	GetApplicationsByOrganizationID(orgID string) ([]*model.Application, error)
	GetApplicationsByProjectIDPaginated(projectID, orgID string, opts ListOptions) ([]*model.Application, error)
	GetApplicationsByOrganizationIDPaginated(orgID string, limit, offset int) ([]*model.Application, error)
	CountApplicationsByProjectID(projectID, orgID, search string) (int, error)
	CountApplicationsByOrganizationID(orgID string) (int, error)
	GetApplicationByNameInProject(name, projectID, orgID string) (*model.Application, error)
	CheckApplicationHandleExists(handle, orgID string) (bool, error)
	UpdateApplication(app *model.Application) error
	DeleteApplication(appID, orgID string) error

	GetAPIKeyByNameAndArtifactHandle(keyName, artifactHandle, orgID string) (*model.ApplicationAPIKey, error)
	GetDeployedGatewayIDsByArtifactUUID(artifactUUID, orgID string) ([]string, error)
	ListMappedAPIKeys(applicationUUID string) ([]*model.ApplicationAPIKey, error)
	ListApplicationAssociations(applicationUUID string) ([]*model.ApplicationAssociationTarget, error)
	AddApplicationAPIKeys(applicationUUID string, apiKeyIDs []string) error
	AddApplicationAssociations(applicationUUID string, targetUUIDs []string) error
	GetApplicationsByAPIKeyID(apiKeyID, orgID string) ([]*model.Application, error)
	RemoveApplicationAPIKey(applicationUUID, apiKeyID string) error
	RemoveAPIKeyFromAllApplications(apiKeyID string) error
	RemoveApplicationAssociation(applicationUUID, targetUUID string) error
}

// APIRepository defines the interface for API data operations
type APIRepository interface {
	CreateAPI(api *model.API) error
	GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error)
	GetAPIsByUUIDs(uuids []string, orgUUID string) (map[string]string, error)
	GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error)
	GetAPIsByProjectUUID(projectUUID, orgUUID string) ([]*model.API, error)
	GetAPIsByOrganizationUUID(orgUUID string, projectUUID string) ([]*model.API, error)
	GetAPIsByOrganizationUUIDPaginated(orgUUID, projectUUID string, opts ListOptions) ([]*model.API, error)
	CountAPIsByOrganizationUUID(orgUUID, projectUUID, search string) (int, error)
	GetAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error)
	UpdateAPI(api *model.API) error
	DeleteAPI(apiUUID, orgUUID string) error

	// API-Gateway association methods
	GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error)

	// Unified API association methods (supports both gateways and dev portals)
	CreateAPIAssociation(association *model.APIAssociation) error
	GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error)
	UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID, updatedBy string) error

	// API name validation methods
	CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error)
	CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, excludeHandle string) (bool, error)
}

// DeploymentRepository defines the interface for deployment data operations
type DeploymentRepository interface {
	// Deployment artifact methods (immutable deployments)
	CreateWithLimitEnforcement(deployment *model.Deployment, hardLimit int) error // Atomic: count, cleanup if needed, create
	GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error)
	GetWithState(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error)
	GetDeploymentsWithState(artifactUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.Deployment, error)
	Delete(deploymentID, artifactUUID, orgUUID string) error
	GetCurrentByGateway(artifactUUID, gatewayID, orgUUID string) (*model.Deployment, error)

	// Deployment status methods (mutable state tracking)
	SetCurrent(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (updatedAt time.Time, err error)
	SetCurrentWithDetails(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus, statusDesired string, performedAt *time.Time, statusReason string) (updatedAt time.Time, err error)
	GetStatus(artifactUUID, orgUUID, gatewayID string) (deploymentID string, status model.DeploymentStatus, updatedAt *time.Time, err error)
	GetStatusFull(artifactUUID, orgUUID, gatewayID string) (deploymentID string, status model.DeploymentStatus, performedAt *time.Time, statusReason string, err error)
	UpdateStatusWithPerformedAtGuard(artifactUUID, orgUUID, gatewayID string, newStatus model.DeploymentStatus, statusReason string, performedAt time.Time, requireCurrentStatus []model.DeploymentStatus) (rowsAffected int64, err error)
	GetStaleTransitionalStatuses(timeout time.Duration) ([]StaleDeploymentStatus, error)
	DeleteStatus(artifactUUID, orgUUID, gatewayID string) error
	GetDeployedGatewayIDs(artifactUUID, orgUUID string) ([]string, error)
	HasActiveDeployment(artifactUUID, orgUUID string) (bool, error)
	GetLatestDeploymentTime(artifactUUID, orgUUID string) (*time.Time, error)
	GetLatestDeploymentRevision(artifactUUID, gatewayUUID, orgUUID string) (string, error)

	// Gateway deployment methods
	GetControlPlaneDeploymentsByGateway(gatewayID, orgUUID string, since *time.Time) ([]*model.DeploymentInfo, error)
	GetDeploymentContentByIDs(deploymentIDs []string, orgUUID string, gatewayUUID string) (map[string]*model.DeploymentContent, error)
	// GetSecretHandlesByGateway returns the distinct secret handles referenced by all
	// artifacts currently deployed on a gateway, sourced from artifact_secret_refs (gateway_id rows).
	GetSecretHandlesByGateway(gatewayID, orgUUID string) ([]string, error)
}

// GatewayRepository defines the interface for gateway data access
type GatewayRepository interface {
	// Gateway operations
	Create(gateway *model.Gateway) error
	GetByUUID(gatewayId string) (*model.Gateway, error)
	GetByOrganizationID(orgID string) ([]*model.Gateway, error)
	GetByHandleAndOrgID(handle, orgID string) (*model.Gateway, error)
	List() ([]*model.Gateway, error)
	ListPaginated(orgID string, opts ListOptions) ([]*model.Gateway, error)
	CountGateways(orgID, search string) (int, error)
	Delete(gatewayID, organizationID string) error
	UpdateGateway(gateway *model.Gateway) error
	UpdateActiveStatus(gatewayId string, isActive bool) error

	// Gateway association checking operations
	HasGatewayDeployments(gatewayID, organizationID string) (bool, error)
	HasGatewayAssociations(gatewayID, organizationID string) (bool, error)
	HasGatewayAssociationsOrDeployments(gatewayID, organizationID string) (bool, error)

	// Token operations
	CreateToken(token *model.GatewayToken) error
	GetActiveTokensByGatewayUUID(gatewayId string) ([]*model.GatewayToken, error)
	GetActiveTokenByHash(tokenHash string) (*model.GatewayToken, error)
	GetTokenByUUID(tokenId string) (*model.GatewayToken, error)
	RevokeToken(tokenId, revokedBy string) error
	CountActiveTokens(gatewayId string) (int, error)

	// Manifest operations
	UpdateGatewayManifest(gatewayID string, manifest []byte) error
	GetGatewayManifest(gatewayID string) ([]byte, error)

	// Version update — persists the version reported by the gateway controller on connect.
	UpdateGatewayVersion(gatewayID, version string) error
}

// SubscriptionPlanRepository defines the interface for subscription plan data operations
type SubscriptionPlanRepository interface {
	Create(plan *model.SubscriptionPlan) error
	GetByID(planID, orgUUID string) (*model.SubscriptionPlan, error)
	GetByIDs(planIDs []string, orgUUID string) (map[string]string, error)
	GetByHandleAndOrg(handle, orgUUID string) (*model.SubscriptionPlan, error)
	ListByOrganization(orgUUID string, limit, offset int) ([]*model.SubscriptionPlan, error)
	CountByOrganization(orgUUID string) (int, error)
	Update(plan *model.SubscriptionPlan) error
	Delete(planID, orgUUID string) error
	ExistsByHandleAndOrg(handle, orgUUID string) (bool, error)
}

// SubscriptionRepository defines the interface for application-level subscription data operations
type SubscriptionRepository interface {
	Create(sub *model.Subscription) error
	GetByID(subscriptionID, orgUUID string) (*model.Subscription, error)
	// ListByFilters returns subscriptions filtered by API and/or application for an organization.
	// If apiUUID is nil, all APIs are considered. If applicationID is nil, all applications are considered.
	ListByFilters(orgUUID string, apiUUID *string, subscriberID *string, applicationID *string, status *string, limit, offset int) ([]*model.Subscription, error)
	// CountByFilters returns the total count of subscriptions matching the same filters as ListByFilters.
	CountByFilters(orgUUID string, apiUUID *string, subscriberID *string, applicationID *string, status *string) (int, error)
	Update(sub *model.Subscription) error
	UpdateToken(subscriptionID, orgUUID, newToken string) error
	Delete(subscriptionID, orgUUID string) error
	ExistsByAPIAndSubscriber(apiUUID, subscriberID, orgUUID string) (bool, error)
}

// LLMProviderTemplateRepository defines the interface for LLM provider template persistence
type LLMProviderTemplateRepository interface {
	Create(t *model.LLMProviderTemplate) error
	CreateNewVersion(t *model.LLMProviderTemplate) error
	GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error)
	GetByUUID(uuid, orgUUID string) (*model.LLMProviderTemplate, error)
	GetByVersion(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error)
	ListVersions(templateID, orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error)
	CountVersions(templateID, orgUUID string) (int, error)
	List(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error)
	Count(orgUUID string) (int, error)
	ListAllVersions(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error)
	CountAllVersions(orgUUID string) (int, error)
	Update(t *model.LLMProviderTemplate) error
	RenameFamily(baseHandle, orgUUID, name string) error
	SetEnabled(templateID, orgUUID, version string, enabled bool) error
	DeleteVersion(templateID, orgUUID, version string) error
	Exists(templateID, orgUUID string) (bool, error)
	GetGroupID(handle, orgUUID string) (string, error)
	ManagedByForHandle(handle, orgUUID string) (string, error)
	ManagedByForGroupID(groupID, orgUUID string) (string, error)
	CountProvidersUsingTemplate(templateID, orgUUID, version string) (int, error)
}

// LLMProviderRepository defines the interface for LLM provider persistence
type LLMProviderRepository interface {
	Create(p *model.LLMProvider) error
	CreateWithCustomPolicyUsages(p *model.LLMProvider, policyUUIDs []string) error
	GetByID(providerID, orgUUID string) (*model.LLMProvider, error)
	List(orgUUID string, limit, offset int) ([]*model.LLMProvider, error)
	Count(orgUUID string) (int, error)
	Update(p *model.LLMProvider) error
	UpdateWithCustomPolicyUsages(p *model.LLMProvider, policyUUIDs []string) error
	Delete(providerID, orgUUID string) error
	Exists(providerID, orgUUID string) (bool, error)
	// EnsureGatewayAssociation creates a gateway association for the provider if one
	// does not already exist and resolves the metadata to use for the deployment.
	EnsureGatewayAssociation(providerUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error)
}

// APIKeyRepository defines the interface for API key persistence
type APIKeyRepository interface {
	Create(key *model.APIKey) error
	Update(key *model.APIKey) error
	Revoke(artifactUUID, name, updatedBy string) error
	GetByArtifactAndName(artifactUUID, name string) (*model.APIKey, error)
	ListByArtifact(artifactUUID string) ([]*model.APIKey, error)
	ListByGatewayAndKind(gatewayID, orgID, kind, issuer string) ([]*model.APIKey, error)
	Delete(artifactUUID, name string) error
	ListAPIKeysByUser(orgUUID, username string, kinds []string) ([]*model.UserAPIKey, error)
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
	// EnsureGatewayAssociation creates a gateway association for the proxy if one does
	// not already exist and resolves the metadata to use for the deployment.
	EnsureGatewayAssociation(proxyUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error)
}

// MCPProxyRepository defines the interface for MCP proxy persistence
type MCPProxyRepository interface {
	Create(p *model.MCPProxy) error
	GetByHandle(handle, orgUUID string) (*model.MCPProxy, error)
	GetByUUID(uuid, orgUUID string) (*model.MCPProxy, error)
	List(orgUUID string, limit, offset int) ([]*model.MCPProxy, error)
	ListByProject(orgUUID, projectUUID string) ([]*model.MCPProxy, error)
	Count(orgUUID string) (int, error)
	CountByProject(orgUUID, projectUUID string) (int, error)
	Update(p *model.MCPProxy) error
	Delete(handle, orgUUID string) error
	Exists(handle, orgUUID string) (bool, error)
	EnsureGatewayAssociation(proxyUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error)
}

// WebSubAPIHmacSecretRepository defines the interface for WebSub API HMAC secret persistence
type WebSubAPIHmacSecretRepository interface {
	Create(secret *model.WebSubAPIHmacSecret) error
	GetByArtifactAndName(artifactUUID, name string) (*model.WebSubAPIHmacSecret, error)
	ListByArtifact(artifactUUID string) ([]*model.WebSubAPIHmacSecret, error)
	Update(secret *model.WebSubAPIHmacSecret) error
	Delete(artifactUUID, name string) error
}

// WebSubAPIRepository defines the interface for WebSub API persistence
type WebSubAPIRepository interface {
	Create(api *model.WebSubAPI) error
	GetByHandle(handle, orgUUID string) (*model.WebSubAPI, error)
	GetByUUID(uuid, orgUUID string) (*model.WebSubAPI, error)
	List(orgUUID, projectUUID string, limit, offset int) ([]*model.WebSubAPI, error)
	Count(orgUUID string) (int, error)
	CountByProject(orgUUID, projectUUID string) (int, error)
	Update(api *model.WebSubAPI) error
	Delete(handle, orgUUID string) error
	Exists(handle, orgUUID string) (bool, error)
}

// WebBrokerAPIRepository defines the interface for WebBroker API persistence
type WebBrokerAPIRepository interface {
	Create(api *model.WebBrokerAPI) error
	GetByHandle(handle, orgUUID string) (*model.WebBrokerAPI, error)
	GetByUUID(uuid, orgUUID string) (*model.WebBrokerAPI, error)
	List(orgUUID, projectUUID string, limit, offset int) ([]*model.WebBrokerAPI, error)
	Count(orgUUID string) (int, error)
	CountByProject(orgUUID, projectUUID string) (int, error)
	Update(api *model.WebBrokerAPI) error
	Delete(handle, orgUUID string) error
	Exists(handle, orgUUID string) (bool, error)
}

// SecretRepository defines the interface for secret persistence.
type SecretRepository interface {
	Create(s *model.Secret) error
	GetByHandle(orgID, handle string) (*model.Secret, error)
	List(orgID string, limit, offset int, updatedAfter *time.Time) ([]*model.Secret, error)
	ListByHandles(orgID string, handles []string, updatedAfter *time.Time) ([]*model.Secret, error)
	Count(orgID string) (int, error)
	Update(s *model.Secret) error
	FindRefsAndSoftDelete(orgID, handle, updatedBy string) ([]model.SecretReference, error)
	FindRefs(orgID, handle string) ([]model.SecretReference, error)
	Exists(orgID, handle string) (bool, error)
}

// CustomPolicyRepository defines the interface for custom policy persistence
type CustomPolicyRepository interface {
	InsertCustomPolicy(policy *model.CustomPolicy) error
	UpdateCustomPolicy(policy *model.CustomPolicy, oldVersion string) error
	GetCustomPolicyByNameAndVersion(orgUUID, name, version string) (*model.CustomPolicy, error)
	GetCustomPolicyByUUID(orgUUID, policyUUID string) (*model.CustomPolicy, error)
	GetCustomPoliciesByName(orgUUID, name string) ([]*model.CustomPolicy, error)
	ListCustomPolicyByOrganization(orgUUID string) ([]*model.CustomPolicy, error)
	DeleteCustomPolicy(orgUUID, name, version string) error
	CountCustomPolicyUsages(policyUUID string) (int, error)
	// DeleteCustomPolicyIfUnused atomically deletes the policy only when it has no active usages.
	DeleteCustomPolicyIfUnused(orgUUID, policyUUID string) error
	// Gateway Custom Policy usage tracking methods.
	GetCustomPolicyUsagesByAPIUUID(apiUUID string) ([]string, error)
	InsertCustomPolicyUsage(policyUUID, apiUUID string) error
	DeleteCustomPolicyUsage(policyUUID, apiUUID string) error
}

// AuditRepository defines the interface for audit record writes.
type AuditRepository interface {
	Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error
}

// UserIdentityMappingRepository defines the interface for internal-UUID <-> IdP-identity mapping persistence.
type UserIdentityMappingRepository interface {
	GetOrCreateUUID(identity string) (string, error)
	// GetSubByUUID returns the resolved actor identity mapped to uuid, or
	// found=false if uuid has no mapping (a "hanging" UUID).
	GetSubByUUID(uuid string) (identity string, found bool, err error)
	// GetSubsByUUIDs batch-resolves multiple UUIDs to their mapped identity in
	// a single query (avoids N+1 on list endpoints). UUIDs with no mapping are
	// absent from the returned map.
	GetSubsByUUIDs(uuids []string) (map[string]string, error)
}

// UserOrganizationMappingRepository defines the interface for user<->organization
// membership persistence. Both FKs are declared ON DELETE CASCADE in the
// schema; DeleteByUser/DeleteByOrg additionally perform the same deletes in
// application code, in the same transaction as the parent delete, as
// defense-in-depth for pooled SQLite connections that may not enforce FKs.
type UserOrganizationMappingRepository interface {
	// AddMembership records that userUUID has onboarded to orgUUID. Idempotent:
	// a duplicate (userUUID, orgUUID) pair is a no-op, not an error.
	AddMembership(userUUID, orgUUID string) error
	// DeleteByUser removes all membership rows for userUUID, within tx.
	DeleteByUser(tx *sql.Tx, userUUID string) error
	// DeleteByOrg removes all membership rows for orgUUID, within tx.
	DeleteByOrg(tx *sql.Tx, orgUUID string) error
}
