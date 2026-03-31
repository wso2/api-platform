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
	"errors"
	"fmt"
	"log/slog"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"time"

	"gopkg.in/yaml.v3"
)

// GatewayInternalAPIService handles internal gateway API operations
type GatewayInternalAPIService struct {
	apiRepo              repository.APIRepository
	subscriptionRepo     repository.SubscriptionRepository
	subscriptionPlanRepo repository.SubscriptionPlanRepository
	providerRepo         repository.LLMProviderRepository
	proxyRepo            repository.LLMProxyRepository
	mcpProxyRepo         repository.MCPProxyRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	projectRepo          repository.ProjectRepository
	apiKeyRepo           repository.APIKeyRepository
	artifactRepo         repository.ArtifactRepository
	apiUtil              *utils.APIUtil
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewGatewayInternalAPIService creates a new gateway internal API service
func NewGatewayInternalAPIService(apiRepo repository.APIRepository, subscriptionRepo repository.SubscriptionRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository, providerRepo repository.LLMProviderRepository,
	proxyRepo repository.LLMProxyRepository, mcpProxyRepo repository.MCPProxyRepository, deploymentRepo repository.DeploymentRepository, gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository, projectRepo repository.ProjectRepository, apiKeyRepo repository.APIKeyRepository,
	artifactRepo repository.ArtifactRepository, cfg *config.Server, slogger *slog.Logger) *GatewayInternalAPIService {
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
		apiUtil:              &utils.APIUtil{},
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// GetAPIByUUID retrieves an API by its ID
func (s *GatewayInternalAPIService) GetAPIByUUID(apiId, orgId string) (map[string]string, error) {
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
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
		return nil, constants.ErrDeploymentNotActive
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
		return nil, constants.ErrDeploymentNotActive
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
		return nil, constants.ErrDeploymentNotActive
	}

	proxyYaml := string(deployment.Content)
	proxyYamlMap := map[string]string{
		proxyID: proxyYaml,
	}
	return proxyYamlMap, nil
}

// IsAPIDeployedOnGateway returns nil if the API has an active deployment_status row on the gateway
// (DEPLOYED or UNDEPLOYED), ErrAPINotFound if the API does not exist, or ErrDeploymentNotActive
// if no active deployment_status exists for the API+gateway.
func (s *GatewayInternalAPIService) IsAPIDeployedOnGateway(apiID, gatewayID, orgID string) error {
	api, err := s.apiRepo.GetAPIByUUID(apiID, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return err
		}
		return fmt.Errorf("failed to get api: %w", err)
	}
	if api == nil || api.OrganizationID != orgID {
		return constants.ErrAPINotFound
	}

	deploymentID, status, _, err := s.deploymentRepo.GetStatus(apiID, orgID, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to get deployment status: %w", err)
	}
	if deploymentID == "" {
		return constants.ErrDeploymentNotActive
	}
	if status != model.DeploymentStatusDeployed && status != model.DeploymentStatusUndeployed {
		return constants.ErrDeploymentNotActive
	}
	return nil
}

// ListSubscriptionsForAPI lists subscriptions for a given API within an organization.
func (s *GatewayInternalAPIService) ListSubscriptionsForAPI(apiID, orgID string) ([]dto.GatewaySubscriptionInfo, error) {
	if apiID == "" || orgID == "" {
		return nil, constants.ErrInvalidInput
	}
	// apiID here is the API UUID (rest_apis.uuid) used as deployment id for REST APIs.
	// First, ensure the API exists and belongs to the organization so callers can map
	// unknown APIs to a 404 instead of silently returning an empty list.
	apiModel, err := s.apiRepo.GetAPIByUUID(apiID, orgID)
	if err != nil {
		// Preserve explicit not-found signaling from the repository so callers
		// can translate it to a 404, while still wrapping unexpected failures.
		if errors.Is(err, constants.ErrAPINotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get api for listing subscriptions: %w", err)
	}
	if apiModel == nil || apiModel.OrganizationID != orgID {
		return nil, constants.ErrAPINotFound
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
			APIID:              sub.APIUUID,
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
		return nil, constants.ErrInvalidInput
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
			PlanName:           plan.PlanName,
			BillingPlan:        plan.BillingPlan,
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
		return nil, constants.ErrMCPProxyNotFound
	}

	deployment, err := s.deploymentRepo.GetCurrentByGateway(proxy.UUID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotActive
	}

	proxyYaml := string(deployment.Content)
	proxyYamlMap := map[string]string{
		proxyID: proxyYaml,
	}
	return proxyYamlMap, nil
}

// CreateGatewayDeployment handles the registration of an API deployment from a gateway
func (s *GatewayInternalAPIService) CreateGatewayDeployment(apiHandle, orgID, gatewayID string,
	notification dto.DeploymentNotification, deploymentID *string) (*dto.GatewayDeploymentResponse, error) {
	// Note: deploymentID parameter is reserved for future use
	_ = deploymentID

	// Validate input
	if apiHandle == "" || orgID == "" || gatewayID == "" {
		return nil, constants.ErrInvalidInput
	}

	// Check if the gateway exists and belongs to the organization
	gatewayModel, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gatewayModel == nil {
		return nil, constants.ErrGatewayNotFound
	}
	if gatewayModel.OrganizationID != orgID {
		return nil, constants.ErrGatewayNotFound
	}

	// Find the project by name within the organization
	projectName := notification.ProjectIdentifier
	project, err := s.projectRepo.GetProjectByNameAndOrgID(projectName, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project by name: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectName)
	}
	projectID := project.ID

	// Check if API already exists by getting metadata
	existingAPIMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing API: %w", err)
	}

	apiCreated := false
	apiUUID := ""
	now := time.Now()
	if existingAPIMetadata == nil {
		// Convert operations from notification to API operations
		operations := make([]model.Operation, 0, len(notification.Configuration.Spec.Operations))
		for _, op := range notification.Configuration.Spec.Operations {
			operation := model.Operation{
				Name:        fmt.Sprintf("%s %s", op.Method, op.Path),
				Description: fmt.Sprintf("Operation for %s %s", op.Method, op.Path),
				Request: &model.OperationRequest{
					Method: op.Method,
					Path:   op.Path,
				},
			}
			operations = append(operations, operation)
		}

		// Create new API from notification
		newAPI := &model.API{
			Handle:          apiHandle, // Use provided apiID as handle
			Name:            notification.Configuration.Spec.Name,
			Version:         notification.Configuration.Spec.Version,
			ProjectID:       projectID,
			OrganizationID:  orgID,
			CreatedBy:       "admin", // Default provider
			LifeCycleStatus: "CREATED",
			Kind:            notification.Configuration.Kind,
			Transport:       []string{"http", "https"},
			Configuration: model.RestAPIConfig{
				Context:    &notification.Configuration.Spec.Context,
				Operations: operations,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		err = s.apiRepo.CreateAPI(newAPI)
		if err != nil {
			return nil, fmt.Errorf("failed to create API: %w", err)
		}

		// CreateAPI sets the UUID on the model
		apiUUID = newAPI.ID

		apiCreated = true
	} else {
		// Validate that existing API belongs to the same organization
		if existingAPIMetadata.OrganizationID != orgID {
			return nil, constants.ErrAPINotFound
		}
		apiUUID = existingAPIMetadata.ID
	}

	// Check if deployment already exists
	existingDeploymentID, status, _, err := s.deploymentRepo.GetStatus(apiUUID, orgID, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to check deployment status of gateway: %w", err)
	}

	// Check if this gateway already has this API deployed or undeployed
	if existingDeploymentID != "" && (status == model.DeploymentStatusDeployed || status == model.DeploymentStatusUndeployed) {
		switch status {
		case model.DeploymentStatusDeployed:
			// An active deployment already exists for this API-gateway combination
			return nil, fmt.Errorf("API already deployed to this gateway")
		case model.DeploymentStatusUndeployed:
			// A deployment record exists, but it is currently undeployed
			return nil, fmt.Errorf("a deployment already exists for this API-gateway combination with status %s", status)
		default:
			// Fallback in case new statuses are introduced in the future
			return nil, fmt.Errorf("a deployment already exists for this API-gateway combination with status %s", status)
		}
	}

	// Check if API-gateway association exists, create if not
	existingAssociations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing API-gateway associations: %w", err)
	}

	// Check if gateway is already associated with the API
	isAssociated := false
	for _, assoc := range existingAssociations {
		if assoc.ResourceID == gatewayID {
			isAssociated = true
			break
		}
	}

	// If gateway is not associated with the API, create the association
	if !isAssociated {
		association := &model.APIAssociation{
			ArtifactID:      apiUUID,
			OrganizationID:  orgID,
			ResourceID:      gatewayID,
			AssociationType: constants.AssociationTypeGateway,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
			return nil, fmt.Errorf("failed to create API-gateway association: %w", err)
		}
	}

	// Create deployment record
	deploymentName := fmt.Sprintf("deployment-%d", now.Unix())
	deployed := model.DeploymentStatusDeployed

	// Generate deployment content YAML from notification configuration
	deploymentContent, err := yaml.Marshal(notification.Configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize deployment content: %w", err)
	}

	deployment := &model.Deployment{
		Name:           deploymentName,
		ArtifactID:     apiUUID,
		GatewayID:      gatewayID,
		OrganizationID: orgID,
		Content:        deploymentContent,
		Status:         &deployed,
		CreatedAt:      now,
	}

	// Use same limit computation as DeploymentService: MaxPerAPIGateway + buffer
	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	err = s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment record: %w", err)
	}

	return &dto.GatewayDeploymentResponse{
		APIId:        apiUUID,
		DeploymentId: 0, // Legacy field, no longer used with new deployment model
		Message:      "API deployment registered successfully",
		Created:      apiCreated,
	}, nil
}

// GetDeploymentsByGateway retrieves all deployments for a specific gateway
// Used to compare local gateway state with platform-api state
// If since is provided, only returns deployments updated after that timestamp
func (s *GatewayInternalAPIService) GetDeploymentsByGateway(orgID, gatewayID string, since *time.Time) (*dto.GatewayDeploymentsResponse, error) {
	// Get all deployments for this gateway (optionally filtered by timestamp)
	deployments, err := s.deploymentRepo.GetAllDeploymentsByGateway(gatewayID, orgID, since)
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
			Kind:         dep.Kind,
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
			ETag: utils.APIKeyETag(k.ArtifactUUID, k.Name, k.UpdatedAt),
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
