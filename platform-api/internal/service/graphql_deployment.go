/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"fmt"
	"log/slog"
	"strings"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

// GraphQLAPIDeploymentService handles business logic for GraphQL API deployment
// operations, using the shared deployments table and status model.
//
// This is a dedicated per-kind deployment service, following the precedent set
// by LLMProviderDeploymentService/LLMProxyDeploymentService (llm_deployment.go)
// rather than generalizing the REST-only DeploymentService (deployment.go).
// DeploymentService's core deploy logic is genuinely REST-typed — it calls
// s.apiRepo.GetAPIByUUID (returns *model.API, reads the REST-only `apis` table)
// and s.apiUtil.BuildAPIDeploymentYAML(*model.API) — so a GraphQL artifact UUID
// would 404 against it today. The generic pieces (DeploymentRepository,
// GatewayRepository, APIKeyRepository, the deployments/deployment_status
// tables) are reused as-is; only the REST-specific artifact lookup and YAML
// builder are kind-specific, exactly as they are for LLM Provider/Proxy.
type GraphQLAPIDeploymentService struct {
	graphqlRepo          repository.GraphQLAPIRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewGraphQLAPIDeploymentService creates a new GraphQL API deployment service.
func NewGraphQLAPIDeploymentService(
	graphqlRepo repository.GraphQLAPIRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	cfg *config.Server,
	slogger *slog.Logger,
) *GraphQLAPIDeploymentService {
	return &GraphQLAPIDeploymentService{
		graphqlRepo:          graphqlRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// generateGraphQLAPIDeploymentYAML builds the deployment YAML struct for a
// GraphQL API. Mirrors APIUtil.BuildAPIDeploymentYAML (internal/utils/api.go)
// in shape — REST's simple struct-building approach, not LLM's
// policy-transformation pipeline, since GraphQL's configuration shape
// (policies + subscriptionPlans + a single upstream) is much closer to REST's
// than to LLM's rate-limit/guardrail model.
func generateGraphQLAPIDeploymentYAML(apiModel *model.GraphQLAPI) (dto.GraphQLAPIDeploymentYAML, error) {
	if apiModel == nil {
		return dto.GraphQLAPIDeploymentYAML{}, apperror.Internal.New().WithLogMessage("generateGraphQLAPIDeploymentYAML: apiModel is nil")
	}

	var upstream *dto.GraphQLUpstream
	if apiModel.Configuration.Upstream.Main != nil {
		main := apiModel.Configuration.Upstream.Main
		upstream = &dto.GraphQLUpstream{
			Main: &dto.GraphQLUpstreamTarget{
				URL:  main.URL,
				Ref:  main.Ref,
				Auth: main.Auth, // raw model.UpstreamAuth — the gateway needs the real credential, unlike API read responses
			},
		}
	}

	contextValue := ""
	if apiModel.Configuration.Context != nil {
		contextValue = *apiModel.Configuration.Context
	}

	policies := make([]dto.Policy, 0, len(apiModel.Configuration.Policies))
	for _, p := range apiModel.Configuration.Policies {
		policies = append(policies, dto.Policy{
			Name:               p.Name,
			Version:            p.Version,
			Params:             p.Params,
			ExecutionCondition: p.ExecutionCondition,
		})
	}

	return dto.GraphQLAPIDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.GraphQLApi,
		Metadata: dto.DeploymentMetadata{
			Name: apiModel.Handle,
			Annotations: map[string]string{
				commonconstants.AnnotationProjectID: apiModel.ProjectID,
			},
			Labels: map[string]string{
				commonconstants.DeprecatedLabelProjectID: apiModel.ProjectID,
			},
		},
		Spec: dto.GraphQLAPIYAMLData{
			DisplayName:       apiModel.Name,
			Version:           apiModel.Version,
			Context:           contextValue,
			SubscriptionPlans: apiModel.Configuration.SubscriptionPlans,
			Upstream:          upstream,
			Policies:          policies,
		},
	}, nil
}

// DeployGraphQLAPI creates a new immutable deployment artifact and deploys it to a
// gateway. Mirrors LLMProviderDeploymentService.DeployLLMProvider.
func (s *GraphQLAPIDeploymentService) DeployGraphQLAPI(apiID string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	if req == nil {
		return nil, apperror.GraphQLAPIDeploymentValidationFailed.New("A request body is required.")
	}
	if req.Base == "" {
		return nil, apperror.GraphQLAPIDeploymentValidationFailed.New("Base is required (use 'current' or a deploymentId).")
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, apperror.GraphQLAPIDeploymentValidationFailed.New("Gateway ID is required.")
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}
	gatewayID := gateway.ID

	apiModel, err := s.graphqlRepo.GetByHandle(apiID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	// DP-originated artifacts are read-only in the control plane and cannot be
	// (re)deployed from the CP.
	if err := ensureOriginMutable(apiModel.Origin); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, apperror.GraphQLAPIDeploymentValidationFailed.New("Deployment name is required.")
	}

	// Ensure a gateway association exists for the target gateway before deploying, and
	// resolve the deployment metadata — see APIService/LLMProviderDeploymentService for
	// the full semantics of this pattern.
	metadataProvided := req.Metadata != nil
	deployMetaJSON, err := marshalDeploymentMetadata(metadata)
	if err != nil {
		return nil, err
	}
	effectiveMetaJSON, err := s.graphqlRepo.EnsureGatewayAssociation(apiModel.ID, gatewayID, orgUUID, createdBy, deployMetaJSON, metadataProvided)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure gateway association: %w", err)
	}
	if metadata, err = unmarshalDeploymentMetadata(effectiveMetaJSON); err != nil {
		return nil, err
	}

	var baseDeploymentID *string
	var contentBytes []byte

	if req.Base == "current" {
		apiDeployment, err := generateGraphQLAPIDeploymentYAML(apiModel)
		if err != nil {
			return nil, fmt.Errorf("failed to generate GraphQL API deployment YAML: %w", err)
		}
		sourceDataVersion := gatewaytranslator.PlatformDataVersion(apiModel.DataVersion)
		targetDataVersion := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
		if err := gatewaytranslator.Translate(constants.GraphQLApi, sourceDataVersion, targetDataVersion, &apiDeployment); err != nil {
			return nil, fmt.Errorf("failed to transform GraphQL API deployment for gateway %s: %w", gateway.Version, err)
		}
		yamlBytes, marshalErr := yaml.Marshal(apiDeployment)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal GraphQL API deployment YAML: %w", marshalErr)
		}
		contentBytes = yamlBytes
	} else {
		baseDeployment, err := s.deploymentRepo.GetWithContent(req.Base, apiModel.ID, orgUUID)
		if err != nil {
			if apperror.DeploymentNotFound.Is(err) {
				return nil, apperror.DeploymentBaseNotFound.Wrap(err)
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base
	}

	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}
	deployed := model.DeploymentStatusDeployed

	deployment := &model.Deployment{
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ArtifactID:       apiModel.ID,
		OrganizationID:   orgUUID,
		GatewayID:        gatewayID,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         metadata,
		Status:           &deployed,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		apiModel.ID, orgUUID, gatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		return nil, fmt.Errorf("failed to set deployment status for GraphQL API: %w", err)
	}

	if s.gatewayEventsService != nil {
		deploymentEvent := &model.GraphQLAPIDeploymentEvent{
			ApiId:        apiModel.ID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastGraphQLAPIDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast GraphQL API deployment event", "error", err)
		}

		// Push existing active API keys for this artifact to the (possibly newly
		// associated) gateway — see BackfillAPIKeysToGateway.
		BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, apiModel.ID, gatewayID, createdBy)
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		initialStatus,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
		nil,
	)
}

// RestoreGraphQLAPIDeployment restores a previous deployment (ARCHIVED or
// UNDEPLOYED). Mirrors LLMProviderDeploymentService.RestoreLLMProviderDeployment.
func (s *GraphQLAPIDeploymentService) RestoreGraphQLAPIDeployment(apiID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	apiModel, err := s.graphqlRepo.GetByHandle(apiID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}
	if err := ensureOriginMutable(apiModel.Origin); err != nil {
		return nil, err
	}

	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, apiModel.ID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	resolvedGateway, err := s.gatewayRepo.GetByHandleAndOrgID(strings.TrimSpace(gatewayID), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if resolvedGateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}
	if targetDeployment.GatewayID != resolvedGateway.ID {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(apiModel.ID, orgUUID, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == deploymentID && status.IsDeployedOrDeploying() {
		return nil, apperror.DeploymentRestoreConflict.New()
	}

	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, apperror.GatewayNotFound.New()
	}

	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		apiModel.ID, orgUUID, targetDeployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	if s.gatewayEventsService != nil {
		deploymentEvent := &model.GraphQLAPIDeploymentEvent{
			ApiId:        apiModel.ID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastGraphQLAPIDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast GraphQL API deployment event", "error", err)
		}
		BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, apiModel.ID, targetDeployment.GatewayID, "")
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		targetDeployment.DeploymentID,
		targetDeployment.Name,
		targetDeployment.GatewayID,
		initialStatus,
		targetDeployment.BaseDeploymentID,
		targetDeployment.Metadata,
		targetDeployment.CreatedAt,
		&updatedAt,
		nil,
	)
}

// UndeployGraphQLAPIDeployment undeploys an active deployment. Mirrors
// LLMProviderDeploymentService.UndeployLLMProviderDeployment.
func (s *GraphQLAPIDeploymentService) UndeployGraphQLAPIDeployment(apiID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	apiModel, err := s.graphqlRepo.GetByHandle(apiID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}
	if err := ensureOriginMutable(apiModel.Origin); err != nil {
		return nil, err
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiModel.ID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	resolvedGateway, err := s.gatewayRepo.GetByHandleAndOrgID(strings.TrimSpace(gatewayID), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if resolvedGateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}
	if deployment.GatewayID != resolvedGateway.ID {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}
	if deployment.Status == nil || !deployment.Status.IsDeployedOrDeploying() {
		return nil, apperror.DeploymentNotActive.New("GraphQL API")
	}

	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, apperror.GatewayNotFound.New()
	}

	initialStatus := model.DeploymentStatusUndeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	newUpdatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		apiModel.ID, orgUUID, deployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.GraphQLAPIUndeploymentEvent{
			ApiId:        apiModel.ID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastGraphQLAPIUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast GraphQL API undeployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		initialStatus,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		&newUpdatedAt,
		nil,
	)
}

// DeleteGraphQLAPIDeployment permanently deletes an undeployed deployment
// artifact. Mirrors LLMProviderDeploymentService.DeleteLLMProviderDeployment.
func (s *GraphQLAPIDeploymentService) DeleteGraphQLAPIDeployment(apiID, deploymentID, orgUUID string) error {
	apiModel, err := s.graphqlRepo.GetByHandle(apiID, orgUUID)
	if err != nil {
		return err
	}
	if apiModel == nil {
		return apperror.GraphQLAPINotFound.New()
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiModel.ID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return apperror.DeploymentNotFound.New()
	}
	if deployment.Status != nil && deployment.Status.IsDeployedOrDeploying() {
		return apperror.DeploymentActive.New()
	}

	if err := s.deploymentRepo.Delete(deploymentID, apiModel.ID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// GetGraphQLAPIDeployments retrieves all deployments for a GraphQL API with
// optional filters. Mirrors LLMProviderDeploymentService.GetLLMProviderDeployments.
func (s *GraphQLAPIDeploymentService) GetGraphQLAPIDeployments(apiID, orgUUID string, gatewayID *string, status *string) (*api.DeploymentListResponse, error) {
	apiModel, err := s.graphqlRepo.GetByHandle(apiID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):    true,
			string(model.DeploymentStatusUndeployed):  true,
			string(model.DeploymentStatusDeploying):   true,
			string(model.DeploymentStatusUndeploying): true,
			string(model.DeploymentStatusFailed):      true,
			string(model.DeploymentStatusArchived):    true,
		}
		if !validStatuses[*status] {
			return nil, apperror.DeploymentInvalidStatus.New()
		}
	}

	gatewayUUID, found, err := resolveGatewayFilter(s.gatewayRepo, gatewayID, orgUUID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &api.DeploymentListResponse{Count: 0, List: []api.DeploymentResponse{}}, nil
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(apiModel.ID, orgUUID, gatewayUUID, status, s.cfg.Deployments.MaxPerAPIGateway)
	if err != nil {
		return nil, err
	}

	items := make([]api.DeploymentResponse, 0, len(deployments))
	for _, d := range deployments {
		mapped, err := toAPIDeploymentResponse(
			s.gatewayRepo,
			d.DeploymentID,
			d.Name,
			d.GatewayID,
			*d.Status,
			d.BaseDeploymentID,
			d.Metadata,
			d.CreatedAt,
			d.UpdatedAt,
			d.StatusReason,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, *mapped)
	}

	return &api.DeploymentListResponse{
		Count: len(items),
		List:  items,
	}, nil
}

// GetGraphQLAPIDeployment retrieves a specific deployment by ID. Mirrors
// LLMProviderDeploymentService.GetLLMProviderDeployment.
func (s *GraphQLAPIDeploymentService) GetGraphQLAPIDeployment(apiID, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	apiModel, err := s.graphqlRepo.GetByHandle(apiID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiModel.ID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		*deployment.Status,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
		deployment.StatusReason,
	)
}
