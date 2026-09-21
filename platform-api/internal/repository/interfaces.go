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
	GetProjectByUUIDAndOrgID(projectId, orgID string) (*model.Project, error)
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
	// Build methods (immutable rendered snapshots, not bound to a gateway)
	CreateBuildWithLimitEnforcement(build *model.Build, hardLimit int) error
	GetBuild(buildID, artifactUUID, orgUUID string) (*model.Build, error)
	GetBuilds(artifactUUID, orgUUID string, limit int) ([]*model.Build, error)
	// Refuses with ErrBuildInUse when a deployment still holds the build
	DeleteBuild(buildID, artifactUUID, orgUUID string) error

	// Deployment artifact methods (immutable deployments)
	// Atomic: count, cleanup if needed, create. A deployment naming a build it runs
	// is refused if that build has been pruned since it was resolved
	CreateWithLimitEnforcement(deployment *model.Deployment, hardLimit int) error
	// Atomic: stores the build this deployment runs alongside the deployment itself,
	// enforcing both the build and the deployment limits. The build is stored under
	// the deployment's own API and organization
	CreateWithBuild(deployment *model.Deployment, build *model.Build, buildHardLimit, hardLimit int) error
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
	// GetUUIDsByHandles resolves each handle to its subscription_plan_uuid,
	// scoped to the organization. A handle absent from the returned map does
	// not exist in the org's catalog. Used by API Publication draft/publish
	// save to validate and resolve subscriptionPlanIds.
	GetUUIDsByHandles(handles []string, orgUUID string) (map[string]string, error)
	// GetHandlesByIDs is the inverse of GetUUIDsByHandles: subscription_plan_uuid
	// to handle, for reconstructing a subscriptionPlanIds response from stored
	// mapping rows.
	GetHandlesByIDs(planUUIDs []string, orgUUID string) (map[string]string, error)
}

// PublicationRepository defines the interface for api_publications and its
// satellite tables (api_publication_contents, api_publication_doc_mappings,
// api_publication_plan_mappings): draft rows (IsDraft true) and read-only
// access to live rows (IsDraft false).
type PublicationRepository interface {
	// GetDraft returns the draft row for (artifactUUID, apiPortalUUID, orgUUID),
	// plus the raw subscription-plan and document UUIDs its mapping tables
	// store (not yet resolved to handles — the caller does that). Returns
	// (nil, nil, nil, nil) when no draft has been saved.
	GetDraft(artifactUUID, apiPortalUUID, orgUUID string) (pub *model.Publication, planUUIDs []string, docUUIDs []string, err error)
	// GetPublication is GetDraft's counterpart for the live (IsDraft false)
	// row. Returns (nil, nil, nil, nil) when this API is not published to
	// this portal.
	GetPublication(artifactUUID, apiPortalUUID, orgUUID string) (pub *model.Publication, planUUIDs []string, docUUIDs []string, err error)
	// SaveDraftDetails creates the draft row on first save (any artifactUUID +
	// apiPortalUUID pairing with no existing draft), or replaces an existing
	// one in full, together with its plan/document mapping rows (planUUIDs /
	// docUUIDs — already resolved from handles by the caller). Returns the
	// saved row with its resolved UUID and audit timestamps.
	SaveDraftDetails(pub *model.Publication, planUUIDs []string, docUUIDs []string, actor string) (*model.Publication, error)
	// PromoteDraftToPublication makes the draft row for (artifactUUID,
	// apiPortalUUID, orgUUID) live. The uuid of whichever row was first
	// published for this pairing is the durable "anchor" identity and never
	// changes again: on a first publish the draft row itself becomes the
	// anchor (flipped in place, same uuid); on a republish the draft's
	// content is merged into the existing anchor row instead, and the draft
	// row is discarded. Returns (nil, false, nil) if no draft exists to
	// promote. replaced reports whether an existing live row was found
	// (republish) versus this being the first publish. draftUpdatedAt is the
	// draft's updated_at as the caller read it before pushing to the portal;
	// a draft saved since then is not promoted (APIPublicationDraftChanged).
	PromoteDraftToPublication(artifactUUID, apiPortalUUID, orgUUID, actor string, draftUpdatedAt time.Time) (pub *model.Publication, replaced bool, err error)
	// UnpublishPublication is PromoteDraftToPublication's mirror, called
	// after the portal removal succeeds: if no draft
	// exists for (artifactUUID, apiPortalUUID, orgUUID), the live row (the
	// anchor) is demoted into the draft in place (is_draft=1, status
	// cleared) — same row, same uuid, no content copy. If a draft already
	// exists as its own row, that draft's content is merged into the anchor
	// instead of deleting the anchor and leaving the draft's own row as the
	// survivor — the anchor's uuid stays the durable identity for this
	// pairing either way, and the draft's own row is discarded once merged.
	// found reports whether a live row existed to unpublish; false is the
	// same defensive precondition failure the caller already checked before
	// calling the portal.
	UnpublishPublication(artifactUUID, apiPortalUUID, orgUUID, actor string) (found bool, err error)
	// DeprecatePublication sets the live row's status to DEPRECATED if it is PUBLISHED.
	// found is false when no row matched.
	DeprecatePublication(artifactUUID, apiPortalUUID, orgUUID, actor string) (found bool, err error)
	// GetContent returns one content row (definition/landing page/thumbnail)
	// for a publication row, or nil if none is stored.
	GetContent(publicationUUID string, contentType model.PublicationContentType, orgUUID string) (*model.PublicationContent, error)
	// SaveContent replaces the named content row for publicationUUID and bumps
	// the parent api_publications row's updated_at/updated_by in the same
	// transaction — one timestamp covers all four pieces (details,
	// definition, landing page, thumbnail).
	SaveContent(content *model.PublicationContent, actor string) error
	// ListStatusByArtifact returns every api_publications row (draft and/or
	// live) for artifactUUID across all API Portals, reduced to the portal
	// UUID, tier, status and updated_at the GET /api-publications rollup
	// needs — one query instead of a GetDraft/GetPublication call per
	// portal.
	ListStatusByArtifact(artifactUUID, orgUUID string) ([]*model.PublicationStatusRow, error)
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
	CreateImportedVersion(t *model.LLMProviderTemplate) (bool, error)
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
	// ListAPIKeysByUser lists keys created by username, or every user's keys in the org when
	// allUsers is true (callers must have verified constants.ScopeAPIKeyAllManage first).
	// Errors rather than widening when username is empty and allUsers is false.
	ListAPIKeysByUser(orgUUID, username string, allUsers bool, kinds []string) ([]*model.UserAPIKey, error)
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

// APIPortalRepository defines the interface for API Portal persistence.
type APIPortalRepository interface {
	Create(portal *model.APIPortal) error
	GetByUUID(portalID, orgUUID string) (*model.APIPortal, error)
	GetByHandleAndOrgID(handle, orgUUID string) (*model.APIPortal, error)
	ListPaginated(orgUUID string, opts ListOptions) ([]*model.APIPortal, error)
	Count(orgUUID string, search string) (int, error)
	Update(portal *model.APIPortal) error
	Delete(portalID, orgUUID string) error
	Exists(handle, orgUUID string) (bool, error)
	// ListActiveByOrg returns every api_portals row for orgUUID whose status
	// is "active" — used by the API Publication feature's
	// GET /api-publications rollup, which lists only these; a portal still
	// provisioning or failed is absent entirely.
	ListActiveByOrg(orgUUID string) ([]*model.APIPortal, error)
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
	// DeleteCustomPolicyIfUnused atomically deletes the policy if unused, purging
	// orphaned usage rows first; returns the number of orphans purged.
	DeleteCustomPolicyIfUnused(orgUUID, policyUUID string) (int, error)
	// Gateway Custom Policy usage tracking methods.
	GetCustomPolicyUsagesByAPIUUID(apiUUID string) ([]string, error)
	InsertCustomPolicyUsage(policyUUID, apiUUID string) error
	DeleteCustomPolicyUsage(policyUUID, apiUUID string) error
}

// DocumentRepository defines the interface for document persistence.
type DocumentRepository interface {
	CreateDocument(doc *model.Document) error
	GetDocumentByArtifactAndHandle(artifactUUID, handle, orgUUID string) (*model.Document, error)
	GetDocumentByArtifactAndType(artifactUUID, docType, orgUUID string) (*model.Document, error)
	UpsertDocument(doc *model.Document) error
	DeleteDocument(artifactUUID, handle, orgUUID string) error
	DocumentHandleExistsForArtifact(artifactUUID, handle string) (bool, error)
	// GetDocumentUUIDsByHandles resolves each handle to its document uuid,
	// scoped to one artifact (api_documents' real unique index is
	// (artifact_uuid, handle) — a handle is only guaranteed unique per
	// artifact, not per org). A handle absent from the returned map does not
	// exist for this artifact. Used by API Publication draft/publish save to
	// validate and resolve docIds.
	GetDocumentUUIDsByHandles(artifactUUID string, handles []string, orgUUID string) (map[string]string, error)
	// GetDocumentHandlesByUUIDs is the inverse, for reconstructing a docIds
	// response from stored api_publication_doc_mappings rows. Scoped to the
	// organization only (not a single artifact) since a mapping row's
	// doc_uuid already came from that same artifact's own resolved set.
	GetDocumentHandlesByUUIDs(docUUIDs []string, orgUUID string) (map[string]string, error)
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
