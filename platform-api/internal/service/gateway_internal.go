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

package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// GatewayInternalAPIService handles internal gateway API operations
type GatewayInternalAPIService struct {
	apiRepo              repository.APIRepository
	subscriptionRepo     repository.SubscriptionRepository
	subscriptionPlanRepo repository.SubscriptionPlanRepository
	providerRepo         repository.LLMProviderRepository
	proxyRepo            repository.LLMProxyRepository
	mcpProxyRepo         repository.MCPProxyRepository
	websubAPIRepo        repository.WebSubAPIRepository
	webbrokerAPIRepo     repository.WebBrokerAPIRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	projectRepo          repository.ProjectRepository
	apiKeyRepo           repository.APIKeyRepository
	artifactRepo         repository.ArtifactRepository
	secretRepo           repository.SecretRepository
	apiUtil              *utils.APIUtil
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewGatewayInternalAPIService creates a new gateway internal API service.
// websubAPIRepo and webbrokerAPIRepo are nil in OSS builds and injected by the
// event-gateway plugin in experimental builds via SetEventArtifactRepos.
func NewGatewayInternalAPIService(apiRepo repository.APIRepository, subscriptionRepo repository.SubscriptionRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository, providerRepo repository.LLMProviderRepository,
	proxyRepo repository.LLMProxyRepository, mcpProxyRepo repository.MCPProxyRepository,
	deploymentRepo repository.DeploymentRepository, gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository, projectRepo repository.ProjectRepository, apiKeyRepo repository.APIKeyRepository,
	artifactRepo repository.ArtifactRepository, secretRepo repository.SecretRepository, cfg *config.Server, slogger *slog.Logger) *GatewayInternalAPIService {
	return &GatewayInternalAPIService{
		apiRepo:              apiRepo,
		subscriptionRepo:     subscriptionRepo,
		subscriptionPlanRepo: subscriptionPlanRepo,
		providerRepo:         providerRepo,
		proxyRepo:            proxyRepo,
		mcpProxyRepo:         mcpProxyRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		projectRepo:          projectRepo,
		apiKeyRepo:           apiKeyRepo,
		artifactRepo:         artifactRepo,
		secretRepo:           secretRepo,
		apiUtil:              &utils.APIUtil{},
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// SetEventArtifactRepos injects the WebSub/WebBroker repositories after construction.
// Called by the server when the event-gateway plugin is loaded (experimental builds).
func (s *GatewayInternalAPIService) SetEventArtifactRepos(
	websubRepo repository.WebSubAPIRepository,
	webbrokerRepo repository.WebBrokerAPIRepository,
) {
	s.websubAPIRepo = websubRepo
	s.webbrokerAPIRepo = webbrokerRepo
}

// GetSecretsByGateway returns secrets referenced by artifacts deployed on this gateway.
// Handles are sourced from artifact_secret_refs (gateway_id rows, maintained at deploy time) so no
// config blob JOIN or application-level regex is needed here.
// If updatedAfter is set, only secrets updated after that time are included.
func (s *GatewayInternalAPIService) GetSecretsByGateway(orgID, gatewayID string, updatedAfter *time.Time) ([]*model.Secret, error) {
	handles, err := s.deploymentRepo.GetSecretHandlesByGateway(gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret handles for gateway: %w", err)
	}
	if len(handles) == 0 {
		return nil, nil
	}
	return s.secretRepo.ListByHandles(orgID, handles, updatedAfter)
}

// IsSecretDeployedOnGateway reports whether a secret handle is referenced by any
// artifact currently deployed on the given gateway.
func (s *GatewayInternalAPIService) IsSecretDeployedOnGateway(orgID, gatewayID, handle string) (bool, error) {
	handles, err := s.deploymentRepo.GetSecretHandlesByGateway(gatewayID, orgID)
	if err != nil {
		return false, fmt.Errorf("failed to get secret handles for gateway: %w", err)
	}
	for _, h := range handles {
		if h == handle {
			return true, nil
		}
	}
	return false, nil
}

// GetAPIByUUID retrieves an API by its ID
func (s *GatewayInternalAPIService) GetAPIByUUID(apiId, orgId string) (map[string]string, error) {
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}
	if apiModel.OrganizationID != orgId {
		return nil, apperror.RESTAPINotFound.New()
	}

	apiYaml, err := s.apiUtil.GenerateAPIDeploymentYAML(apiModel)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API YAML: %w", err)
	}
	apiYamlMap := map[string]string{
		apiModel.Handle: apiYaml,
	}
	return apiYamlMap, nil
}

// GetActiveDeploymentByGateway retrieves the currently deployed API artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveDeploymentByGateway(apiID, orgID, gatewayID string) (map[string]string, error) {
	// Get the active deployment for this API on this gateway
	deployment, err := s.deploymentRepo.GetCurrentByGateway(apiID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotActive.New("API")
	}

	// Deployment content is already stored as YAML, so return it directly
	apiYaml := string(deployment.Content)

	apiYamlMap := map[string]string{
		apiID: apiYaml,
	}
	return apiYamlMap, nil
}

// GetActiveLLMProviderDeploymentByGateway retrieves the currently deployed LLM provider artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveLLMProviderDeploymentByGateway(providerID, orgID, gatewayID string) (map[string]string, error) {

	deployment, err := s.deploymentRepo.GetCurrentByGateway(providerID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotActive.New("LLM provider")
	}

	providerYaml := string(deployment.Content)
	providerYamlMap := map[string]string{
		providerID: providerYaml,
	}
	return providerYamlMap, nil
}

// GetActiveLLMProxyDeploymentByGateway retrieves the currently deployed LLM proxy artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveLLMProxyDeploymentByGateway(proxyID, orgID, gatewayID string) (map[string]string, error) {

	deployment, err := s.deploymentRepo.GetCurrentByGateway(proxyID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotActive.New("LLM proxy")
	}

	proxyYaml := string(deployment.Content)
	proxyYamlMap := map[string]string{
		proxyID: proxyYaml,
	}
	return proxyYamlMap, nil
}

// IsAPIDeployedOnGateway returns nil if the API has an active deployment_status row on the gateway
// (DEPLOYED or UNDEPLOYED), REST_API_NOT_FOUND if the API does not exist, or DEPLOYMENT_NOT_ACTIVE
// if no active deployment_status exists for the API+gateway.
func (s *GatewayInternalAPIService) IsAPIDeployedOnGateway(apiID, gatewayID, orgID string) error {
	api, err := s.apiRepo.GetAPIByUUID(apiID, orgID)
	if err != nil {
		if apperror.RESTAPINotFound.Is(err) {
			return err
		}
		return fmt.Errorf("failed to get api: %w", err)
	}
	if api == nil || api.OrganizationID != orgID {
		return apperror.RESTAPINotFound.New()
	}

	deploymentID, status, _, err := s.deploymentRepo.GetStatus(apiID, orgID, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to get deployment status: %w", err)
	}
	if deploymentID == "" {
		return apperror.DeploymentNotActive.New("API")
	}
	if status != model.DeploymentStatusDeployed && status != model.DeploymentStatusUndeployed {
		return apperror.DeploymentNotActive.New("API")
	}
	return nil
}

// ListSubscriptionsForAPI lists subscriptions for a given API within an organization.
func (s *GatewayInternalAPIService) ListSubscriptionsForAPI(apiID, orgID string) ([]dto.GatewaySubscriptionInfo, error) {
	if apiID == "" || orgID == "" {
		return nil, apperror.ValidationFailed.New("The API id and organization id are required.")
	}
	// apiID here is the API UUID (rest_apis.uuid) used as deployment id for REST APIs.
	// First, ensure the API exists and belongs to the organization so callers can map
	// unknown APIs to a 404 instead of silently returning an empty list.
	apiModel, err := s.apiRepo.GetAPIByUUID(apiID, orgID)
	if err != nil {
		// Preserve explicit not-found signaling from the repository so callers
		// can translate it to a 404, while still wrapping unexpected failures.
		if apperror.RESTAPINotFound.Is(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get api for listing subscriptions: %w", err)
	}
	if apiModel == nil || apiModel.OrganizationID != orgID {
		return nil, apperror.RESTAPINotFound.New()
	}

	// Internal sync: fetch all subscriptions for the API via pagination so reconciliation
	// never performs destructive deletes due to a cutoff.
	const pageSize = 1000
	var subs []*model.Subscription
	for offset := 0; ; offset += pageSize {
		page, err := s.subscriptionRepo.ListByFilters(orgID, &apiID, nil, nil, nil, pageSize, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions for API %s: %w", apiID, err)
		}
		subs = append(subs, page...)
		if len(page) < pageSize {
			break
		}
	}

	items := make([]dto.GatewaySubscriptionInfo, len(subs))
	for i, sub := range subs {
		items[i] = dto.GatewaySubscriptionInfo{
			ID:                 sub.UUID,
			APIID:              sub.ArtifactUUID,
			ApplicationID:      sub.ApplicationID,
			SubscriptionToken:  sub.SubscriptionToken,
			SubscriptionPlanID: sub.SubscriptionPlanID,
			Status:             string(sub.Status),
			CreatedAt:          sub.CreatedAt,
			UpdatedAt:          sub.UpdatedAt,
			Etag:               utils.GenerateDeterministicUUIDv7(sub.UUID, sub.UpdatedAt),
		}
	}
	return items, nil
}

// ListSubscriptionPlansForOrg lists all subscription plans for an organization.
func (s *GatewayInternalAPIService) ListSubscriptionPlansForOrg(orgID string) ([]dto.GatewaySubscriptionPlanInfo, error) {
	if orgID == "" {
		return nil, apperror.ValidationFailed.New("The organization id is required.")
	}
	// Internal sync: fetch all plans for the organization via pagination so reconciliation
	// never performs destructive deletes due to a cutoff.
	const pageSize = 1000
	var plans []*model.SubscriptionPlan
	for offset := 0; ; offset += pageSize {
		page, err := s.subscriptionPlanRepo.ListByOrganization(orgID, pageSize, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscription plans for org %s: %w", orgID, err)
		}
		plans = append(plans, page...)
		if len(page) < pageSize {
			break
		}
	}

	items := make([]dto.GatewaySubscriptionPlanInfo, len(plans))
	for i, plan := range plans {
		items[i] = dto.GatewaySubscriptionPlanInfo{
			ID:                 plan.UUID,
			Handle:             plan.Handle,
			PlanName:           plan.Name,
			StopOnQuotaReach:   plan.StopOnQuotaReach,
			ThrottleLimitCount: plan.ThrottleLimitCount,
			ThrottleLimitUnit:  plan.ThrottleLimitUnit,
			ExpiryTime:         plan.ExpiryTime,
			Status:             string(plan.Status),
			CreatedAt:          plan.CreatedAt,
			UpdatedAt:          plan.UpdatedAt,
			Etag:               utils.GenerateDeterministicUUIDv7(plan.UUID, plan.UpdatedAt),
		}
	}
	return items, nil
}

// GetActiveMCPProxyDeploymentByGateway retrieves the currently deployed MCP proxy artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveMCPProxyDeploymentByGateway(proxyID, orgID, gatewayID string) (map[string]string, error) {
	proxy, err := s.mcpProxyRepo.GetByUUID(proxyID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if proxy == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	deployment, err := s.deploymentRepo.GetCurrentByGateway(proxy.UUID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotActive.New("MCP proxy")
	}

	proxyYaml := string(deployment.Content)
	proxyYamlMap := map[string]string{
		proxyID: proxyYaml,
	}
	return proxyYamlMap, nil
}

// GetActiveWebSubAPIDeploymentByGateway retrieves the currently deployed WebSub API artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveWebSubAPIDeploymentByGateway(apiID, orgID, gatewayID string) (map[string]string, error) {
	if s.websubAPIRepo == nil {
		return nil, apperror.WebSubAPINotFound.New().
			WithLogMessage("WebSub API repository unavailable: the event-gateway plugin is not enabled")
	}
	websubAPI, err := s.websubAPIRepo.GetByUUID(apiID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebSub API: %w", err)
	}
	if websubAPI == nil {
		return nil, apperror.WebSubAPINotFound.New()
	}

	deployment, err := s.deploymentRepo.GetCurrentByGateway(websubAPI.UUID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotActive.New("WebSub API")
	}

	apiYaml := string(deployment.Content)
	apiYamlMap := map[string]string{
		apiID: apiYaml,
	}
	return apiYamlMap, nil
}

// GetActiveWebBrokerAPIDeploymentByGateway retrieves the currently deployed WebBroker API artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveWebBrokerAPIDeploymentByGateway(apiID, orgID, gatewayID string) (map[string]string, error) {
	if s.webbrokerAPIRepo == nil {
		return nil, apperror.WebBrokerAPINotFound.New().
			WithLogMessage("WebBroker API repository unavailable: the event-gateway plugin is not enabled")
	}
	webbrokerAPI, err := s.webbrokerAPIRepo.GetByUUID(apiID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebBroker API: %w", err)
	}
	if webbrokerAPI == nil {
		return nil, apperror.WebBrokerAPINotFound.New()
	}

	deployment, err := s.deploymentRepo.GetCurrentByGateway(webbrokerAPI.UUID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotActive.New("WebBroker API")
	}

	apiYaml := string(deployment.Content)
	apiYamlMap := map[string]string{
		apiID: apiYaml,
	}
	return apiYamlMap, nil
}

// GetDeploymentsByGateway retrieves all deployments for a specific gateway
// Used to compare local gateway state with platform-api state
// If since is provided, only returns deployments updated after that timestamp
func (s *GatewayInternalAPIService) GetDeploymentsByGateway(orgID, gatewayID string, since *time.Time) (*dto.GatewayDeploymentsResponse, error) {
	// Get all deployments for this gateway (optionally filtered by timestamp)
	deployments, err := s.deploymentRepo.GetControlPlaneDeploymentsByGateway(gatewayID, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployments: %w", err)
	}

	// Convert to response DTO
	items := make([]dto.GatewayDeploymentInfo, len(deployments))
	for i, dep := range deployments {
		deployedAt := dep.PerformedAt
		items[i] = dto.GatewayDeploymentInfo{
			ArtifactID:   dep.ArtifactID,
			DeploymentID: dep.DeploymentID,
			Kind:         dep.Type,
			State:        string(dep.Status),
			DeployedAt:   deployedAt,
			Etag:         utils.GenerateDeterministicUUIDv7(dep.DeploymentID, deployedAt),
		}
	}

	return &dto.GatewayDeploymentsResponse{
		Deployments: items,
	}, nil
}

// GetDeploymentContentBatch retrieves deployment content for multiple deployment IDs
// Returns a map of deploymentID -> DeploymentContent (artifact ID + YAML bytes)
func (s *GatewayInternalAPIService) GetDeploymentContentBatch(orgID, gatewayID string, deploymentIDs []string) (map[string]*model.DeploymentContent, error) {
	contentMap, err := s.deploymentRepo.GetDeploymentContentByIDs(deploymentIDs, orgID, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment content: %w", err)
	}
	return contentMap, nil
}

// GetAPIKeysByKind returns all API keys for artifacts of the given kind associated with the gateway
// via deployment_status (DEPLOYED or UNDEPLOYED).
// When issuer is non-empty only keys whose issuer matches are returned.
// Each item carries a stable correlationId derived from (artifactUuid, name, updatedAt).
// source is always "external" and externalRefId is always null.
func (s *GatewayInternalAPIService) GetAPIKeysByKind(gatewayID, orgID, kind, issuer string) ([]model.InternalAPIKeyItem, error) {
	keys, err := s.apiKeyRepo.ListByGatewayAndKind(gatewayID, orgID, kind, issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys by kind: %w", err)
	}

	items := make([]model.InternalAPIKeyItem, 0, len(keys))
	for _, k := range keys {
		var hashes map[string]string
		if k.APIKeyHashes != "" {
			if err := json.Unmarshal([]byte(k.APIKeyHashes), &hashes); err != nil {
				s.slogger.Warn("Failed to unmarshal API key hashes, skipping key",
					"keyUUID", k.UUID, "kind", kind, "error", err)
				continue
			}
		}
		items = append(items, model.InternalAPIKeyItem{
			ETag:          utils.APIKeyETag(k.ArtifactUUID, k.Name, k.UpdatedAt),
			UUID:          k.UUID,
			Name:          k.Name,
			MaskedAPIKey:  k.MaskedAPIKey,
			APIKeyHashes:  hashes,
			ArtifactUUID:  k.ArtifactUUID,
			Status:        k.Status,
			CreatedAt:     k.CreatedAt,
			CreatedBy:     k.CreatedBy,
			UpdatedAt:     k.UpdatedAt,
			ExpiresAt:     k.ExpiresAt,
			Source:        "external",
			ExternalRefId: nil,
			Issuer:        k.Issuer,
		})
	}
	return items, nil
}

// CheckArtifactsExist returns the subset of provided artifact UUIDs that still exist on the platform.
// Used by the gateway during sync to distinguish orphaned artifacts (deleted on platform)
// from artifacts that exist but have no active deployment.
func (s *GatewayInternalAPIService) CheckArtifactsExist(orgID string, artifactIDs []string) ([]string, error) {
	if len(artifactIDs) == 0 {
		return nil, nil
	}
	return s.artifactRepo.ExistsByUUIDs(artifactIDs, orgID)
}
