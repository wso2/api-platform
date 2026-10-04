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
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

// Policy names referenced during LLM deployment artifact generation, migration,
// and version-aware downconversion.
const (
	tokenBasedRateLimitPolicyName   = "token-based-ratelimit"
	advancedRateLimitPolicyName     = "advanced-ratelimit"
	basicRateLimitPolicyName        = "basic-ratelimit"
	apiKeyAuthPolicyName            = "api-key-auth"
	llmCostPolicyName               = "llm-cost"
	llmCostBasedRateLimitPolicyName = "llm-cost-based-ratelimit"
)

// LLMProviderDeploymentService handles business logic for LLM provider deployment operations
// using the shared deployments table and status model.
type LLMProviderDeploymentService struct {
	providerRepo repository.LLMProviderRepository
	templateRepo repository.LLMProviderTemplateRepository
	// builds is the shared build store every artifact kind uses.
	builds               *BuildService
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	// secretService resolves the {{ secret "handle" }} reference a per-deployment
	// upstream credential is given as. Injected after construction (SetSecretService).
	secretService *SecretService
	cfg           *config.Server
	slogger       *slog.Logger
}

// LLMProxyDeploymentService handles business logic for LLM proxy deployment operations
// using the shared deployments table and status model.
type LLMProxyDeploymentService struct {
	proxyRepo repository.LLMProxyRepository
	// builds is the shared build store every artifact kind uses.
	builds               *BuildService
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewLLMProviderDeploymentService creates a new LLM provider deployment service
func NewLLMProviderDeploymentService(
	providerRepo repository.LLMProviderRepository,
	templateRepo repository.LLMProviderTemplateRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	artifactRepo repository.ArtifactRepository,
	definitions ArtifactDefinitions,
	cfg *config.Server,
	slogger *slog.Logger,
) *LLMProviderDeploymentService {
	return &LLMProviderDeploymentService{
		builds:               NewBuildService(artifactRepo, deploymentRepo, definitions, cfg, slogger),
		providerRepo:         providerRepo,
		templateRepo:         templateRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// SetSecretService injects the SecretService used to check that a per-deployment
// upstream credential names a secret this organization actually has. Called after
// both services are constructed, to avoid a circular dependency.
func (s *LLMProviderDeploymentService) SetSecretService(ss *SecretService) {
	s.secretService = ss
}

// NewLLMProxyDeploymentService creates a new LLM proxy deployment service
func NewLLMProxyDeploymentService(
	proxyRepo repository.LLMProxyRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	artifactRepo repository.ArtifactRepository,
	definitions ArtifactDefinitions,
	cfg *config.Server,
	slogger *slog.Logger,
) *LLMProxyDeploymentService {
	return &LLMProxyDeploymentService{
		builds:               NewBuildService(artifactRepo, deploymentRepo, definitions, cfg, slogger),
		proxyRepo:            proxyRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// providerUUID resolves an LLM provider's identifier to its artifact UUID, which is
// what builds are keyed by. Resolving here keeps this kind's own not-found.
func (s *LLMProviderDeploymentService) providerUUID(providerID, orgUUID string) (string, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return "", err
	}
	if provider == nil {
		return "", apperror.LLMProviderNotFound.New()
	}
	return provider.UUID, nil
}

// CreateBuildByHandle prepares a build of an LLM provider without deploying it.
//
// Builds are the same thing for every artifact kind, so these four delegate to the
// shared store; only resolving the identifier is this kind's own.
func (s *LLMProviderDeploymentService) CreateBuildByHandle(providerID, orgUUID, createdBy, description string,
	metadata map[string]interface{}) (*api.BuildResponse, error) {
	providerUUID, err := s.providerUUID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.builds.Create(providerUUID, orgUUID, constants.LLMProvider, createdBy, description, metadata)
}

// GetBuildByHandle returns one of an LLM provider's builds.
func (s *LLMProviderDeploymentService) GetBuildByHandle(providerID, buildID, orgUUID string) (*api.BuildResponse, error) {
	providerUUID, err := s.providerUUID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.builds.Get(providerUUID, buildID, orgUUID, constants.LLMProvider)
}

// GetBuildsByHandle lists an LLM provider's builds, newest first.
func (s *LLMProviderDeploymentService) GetBuildsByHandle(providerID, orgUUID string, limit int) (*api.BuildListResponse, error) {
	providerUUID, err := s.providerUUID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.builds.List(providerUUID, orgUUID, constants.LLMProvider, limit)
}

// DeleteBuildByHandle removes one of an LLM provider's builds.
func (s *LLMProviderDeploymentService) DeleteBuildByHandle(providerID, buildID, orgUUID string) error {
	providerUUID, err := s.providerUUID(providerID, orgUUID)
	if err != nil {
		return err
	}
	return s.builds.Delete(providerUUID, buildID, orgUUID, constants.LLMProvider)
}

// proxyUUID resolves an LLM proxy's identifier to its artifact UUID.
func (s *LLMProxyDeploymentService) proxyUUID(proxyID, orgUUID string) (string, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return "", err
	}
	if proxy == nil {
		return "", apperror.LLMProxyNotFound.New()
	}
	return proxy.UUID, nil
}

// CreateBuildByHandle prepares a build of an LLM proxy without deploying it.
func (s *LLMProxyDeploymentService) CreateBuildByHandle(proxyID, orgUUID, createdBy, description string,
	metadata map[string]interface{}) (*api.BuildResponse, error) {
	proxyUUID, err := s.proxyUUID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.builds.Create(proxyUUID, orgUUID, constants.LLMProxy, createdBy, description, metadata)
}

// GetBuildByHandle returns one of an LLM proxy's builds.
func (s *LLMProxyDeploymentService) GetBuildByHandle(proxyID, buildID, orgUUID string) (*api.BuildResponse, error) {
	proxyUUID, err := s.proxyUUID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.builds.Get(proxyUUID, buildID, orgUUID, constants.LLMProxy)
}

// GetBuildsByHandle lists an LLM proxy's builds, newest first.
func (s *LLMProxyDeploymentService) GetBuildsByHandle(proxyID, orgUUID string, limit int) (*api.BuildListResponse, error) {
	proxyUUID, err := s.proxyUUID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.builds.List(proxyUUID, orgUUID, constants.LLMProxy, limit)
}

// DeleteBuildByHandle removes one of an LLM proxy's builds.
func (s *LLMProxyDeploymentService) DeleteBuildByHandle(proxyID, buildID, orgUUID string) error {
	proxyUUID, err := s.proxyUUID(proxyID, orgUUID)
	if err != nil {
		return err
	}
	return s.builds.Delete(proxyUUID, buildID, orgUUID, constants.LLMProxy)
}

// DeployLLMProvider creates a new immutable deployment artifact and deploys it to a gateway
func (s *LLMProviderDeploymentService) DeployLLMProvider(providerID string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, apperror.LLMProviderDeploymentValidationFailed.New("A request body is required.")
	}
	base, requestedBuild, err := ValidateDeployBase(req.Base, req.BuildId,
		apperror.LLMProviderDeploymentValidationFailed)
	if err != nil {
		return nil, err
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, apperror.LLMProviderDeploymentValidationFailed.New("Gateway ID is required.")
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}
	gatewayID := gateway.ID

	// Get LLM provider
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, apperror.LLMProviderNotFound.New()
	}

	// DP-originated artifacts are read-only in the control plane and cannot be
	// (re)deployed from the CP.
	if err := ensureOriginMutable(provider.Origin); err != nil {
		return nil, err
	}

	// Validate deployment name is provided
	if req.Name == "" {
		return nil, apperror.LLMProviderDeploymentValidationFailed.New("Deployment name is required.")
	}

	// metadataProvided distinguishes an omitted metadata field from one explicitly set
	// (even to empty), so a deploy can request empty metadata while the association
	// keeps its creation-time value.
	metadataProvided := req.Metadata != nil
	requestMetadata := metadata
	deployMetaJSON, err := marshalDeploymentMetadata(requestMetadata)
	if err != nil {
		return nil, err
	}

	// The credential this gateway is running now, read before the new deployment
	// replaces it, so a rotation can release the secret it rotated away from.
	previousCredential := s.currentDeploymentCredential(provider.UUID, gatewayID, orgUUID)

	// What this deploy ships: a build prepared earlier, or a snapshot of the
	// provider as it stands now. A snapshot comes back unstored so it commits with
	// the deployment below.
	source, err := s.builds.SourceForDeploy(provider.UUID, orgUUID, constants.LLMProvider, createdBy, base, requestedBuild)
	if err != nil {
		return nil, err
	}
	providerDeployment, ok := source.Definition.(*dto.LLMProviderDeploymentYAML)
	if !ok {
		return nil, fmt.Errorf("artifact %s did not render as an LLM provider definition", provider.UUID)
	}
	// Validate what the request itself carries, before the association can be seeded
	// with it below. A first deployment seeds the association from this request and an
	// association is never rewritten, so metadata that seeds and is then rejected stays
	// on the gateway for good, and every later deploy that omits metadata inherits it
	// and fails the same way. A request that omitted the field carries nothing, so this
	// does nothing; when it did carry something, it is exactly what the effective
	// metadata below resolves to, and re-applying it is setting the same values twice.
	if err := s.applyUpstreamOverrides(providerDeployment, requestMetadata, orgUUID); err != nil {
		return nil, err
	}

	// Ensure a gateway association exists for the target gateway, and resolve the
	// metadata this deployment runs with. The first deployment to a gateway creates the
	// association and seeds its metadata from this deployment. For an existing
	// association the deploy request value overrides for this deployment; when the
	// metadata field is omitted, the association's stored metadata is used. An existing
	// association's metadata is never modified at deploy time.
	//
	// One call does both, so a deployment racing another onto the same gateway renders
	// with whatever metadata the association actually ended up holding.
	effectiveMetaJSON, err := s.providerRepo.EnsureGatewayAssociation(
		provider.UUID, gatewayID, orgUUID, createdBy, deployMetaJSON, metadataProvided)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure gateway association: %w", err)
	}
	if metadata, err = unmarshalDeploymentMetadata(effectiveMetaJSON); err != nil {
		return nil, err
	}

	// Customized for this deployment before it is translated, exactly as an API's
	// endpoint and vhosts are: the build is a snapshot of the provider's definition,
	// and what a single gateway authenticates with is a property of the deployment
	// rather than of that snapshot.
	if err := s.applyUpstreamOverrides(providerDeployment, metadata, orgUUID); err != nil {
		return nil, err
	}

	sourceDataVersion := gatewaytranslator.PlatformDataVersion(source.DataVersion)
	targetDataVersion := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
	if err := gatewaytranslator.Translate(
		constants.LLMProvider,
		sourceDataVersion,
		targetDataVersion,
		providerDeployment,
	); err != nil {
		return nil, fmt.Errorf("failed to transform LLM provider deployment for gateway %s: %w", gateway.Version, err)
	}
	contentBytes, err := yaml.Marshal(providerDeployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLM provider deployment YAML: %w", err)
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}
	deployed := model.DeploymentStatusDeployed

	deployment := &model.Deployment{
		DeploymentID:   deploymentID,
		Name:           req.Name,
		ArtifactID:     provider.UUID,
		OrganizationID: orgUUID,
		GatewayID:      gatewayID,
		BuildUUID:      source.BuildUUID,
		BuildID:        source.BuildID,
		Content:        contentBytes,
		Metadata:       metadata,
		Status:         &deployed,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	// A build rendered for this deploy is stored with the deployment, in one
	// transaction, so a recorded deployment always has the build it runs.
	if source.NewBuild != nil {
		err = s.deploymentRepo.CreateWithBuild(deployment, source.NewBuild,
			s.cfg.Deployments.MaxBuildsPerAPI, hardLimit)
	} else {
		err = s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit)
	}
	if err != nil {
		if limitErr := s.builds.LimitError(err); limitErr != err {
			return nil, limitErr
		}
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		provider.UUID, orgUUID, gatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		return nil, fmt.Errorf("failed to set deployment status for LLM provider: %w", err)
	}

	// Best-effort: release the secret this gateway's credential was rotated away from,
	// as an update to the provider's own credential does. It runs after the status is
	// set above, because that is what rewrites this gateway's secret references — until
	// it has, the old handle still looks in use by this very gateway and the delete
	// would be refused.
	s.cleanupRotatedCredential(orgUUID, previousCredential, metadata, createdBy)

	// Broadcast LLM provider deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.LLMProviderDeploymentEvent{
			ProviderId:   provider.UUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastLLMProviderDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM provider deployment event", "error", err)
		}

		// Push existing active API keys for this provider to the (possibly newly
		// associated) gateway so keys created before this association are recognized
		// immediately, rather than only after the controller's next reconnect sync.
		BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, provider.UUID, gatewayID, "")
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

// RestoreLLMProviderDeployment restores a previous deployment (ARCHIVED or UNDEPLOYED)
func (s *LLMProviderDeploymentService) RestoreLLMProviderDeployment(providerID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, apperror.LLMProviderNotFound.New()
	}
	// DP-originated artifacts are read-only in the control plane; restore cannot be CP-initiated.
	if err := ensureOriginMutable(provider.Origin); err != nil {
		return nil, err
	}

	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	// gatewayID is a gateway handle (matching deploy); resolve it to the internal
	// gateway UUID stored on the deployment before comparing.
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

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(provider.UUID, orgUUID, targetDeployment.GatewayID)
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

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		provider.UUID, orgUUID, targetDeployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Broadcast LLM provider deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.LLMProviderDeploymentEvent{
			ProviderId:   provider.UUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastLLMProviderDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM provider deployment event", "error", err)
		}

		// Backfill existing active API keys to the gateway (see BackfillAPIKeysToGateway).
		BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, provider.UUID, targetDeployment.GatewayID, "")
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, targetDeployment.BuildID)
}

// UndeployLLMProviderDeployment undeploys an active deployment
func (s *LLMProviderDeploymentService) UndeployLLMProviderDeployment(providerID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, apperror.LLMProviderNotFound.New()
	}
	// DP-originated artifacts are read-only in the control plane: their deploy/undeploy
	// lifecycle is owned by the data-plane gateway, so undeployment cannot be initiated
	// from the control plane.
	if err := ensureOriginMutable(provider.Origin); err != nil {
		return nil, err
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	// gatewayID is a gateway handle (matching deploy); resolve it to the internal
	// gateway UUID stored on the deployment before comparing.
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
		return nil, apperror.DeploymentNotActive.New("LLM provider")
	}

	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, apperror.GatewayNotFound.New()
	}

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusUndeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	newUpdatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		provider.UUID, orgUUID, deployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Broadcast LLM provider undeployment event to gateway
	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.LLMProviderUndeploymentEvent{
			ProviderId:   provider.UUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastLLMProviderUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM provider undeployment event", "error", err)
		}
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

// DeleteLLMProviderDeployment permanently deletes an undeployed deployment artifact
func (s *LLMProviderDeploymentService) DeleteLLMProviderDeployment(providerID, deploymentID, orgUUID string) error {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return err
	}
	if provider == nil {
		return apperror.LLMProviderNotFound.New()
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return apperror.DeploymentNotFound.New()
	}
	if deployment.Status != nil && deployment.Status.IsDeployedOrDeploying() {
		return apperror.DeploymentActive.New()
	}

	if err := s.deploymentRepo.Delete(deploymentID, provider.UUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// GetLLMProviderDeployments retrieves all deployments for a provider with optional filters
func (s *LLMProviderDeploymentService) GetLLMProviderDeployments(providerID, orgUUID string, gatewayID *string, status *string) (*api.DeploymentListResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, apperror.LLMProviderNotFound.New()
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

	// The gatewayId filter is a gateway handle (matching deploy/undeploy); resolve it
	// to the internal gateway UUID stored in deployments.gateway_uuid before filtering.
	gatewayUUID, found, err := resolveGatewayFilter(s.gatewayRepo, gatewayID, orgUUID)
	if err != nil {
		return nil, err
	}
	if !found {
		// The filter names a gateway that does not exist in this org: no deployment matches.
		return &api.DeploymentListResponse{Count: 0, List: []api.DeploymentResponse{}}, nil
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(provider.UUID, orgUUID, gatewayUUID, status, s.cfg.Deployments.MaxPerAPIGateway)
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
		mapped.BuildId = d.BuildID
		items = append(items, *mapped)
	}

	return &api.DeploymentListResponse{
		Count: len(items),
		List:  items,
	}, nil
}

// GetLLMProviderDeployment retrieves a specific deployment by ID
func (s *LLMProviderDeploymentService) GetLLMProviderDeployment(providerID, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, apperror.LLMProviderNotFound.New()
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

func (s *LLMProviderDeploymentService) getTemplateHandle(templateUUID, orgUUID string) (string, error) {
	if templateUUID == "" {
		return "", apperror.LLMProviderTemplateNotFound.Wrap(apperror.LLMProviderDeploymentValidationFailed.New("The referenced LLM provider template could not be found."))
	}
	tpl, err := s.templateRepo.GetByUUID(templateUUID, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve template: %w", err)
	}
	if tpl == nil {
		return "", apperror.LLMProviderTemplateNotFound.Wrap(apperror.LLMProviderDeploymentValidationFailed.New("The referenced LLM provider template could not be found."))
	}
	return tpl.ID, nil
}

// currentDeploymentCredential reads the per-deployment credential the gateway is
// running now, or "" when it is running nothing, has no credential of its own, or
// cannot be read. A failure to read it only means no secret is released, which is why
// it is not surfaced: it must never be the reason a deploy fails.
func (s *LLMProviderDeploymentService) currentDeploymentCredential(artifactUUID, gatewayID, orgUUID string) string {
	current, err := s.deploymentRepo.GetCurrentByGateway(artifactUUID, gatewayID, orgUUID)
	if err != nil || current == nil {
		return ""
	}
	value, _ := current.Metadata[constants.MetadataKeyUpstreamAuthValue].(string)
	return value
}

// cleanupRotatedCredential releases the secret a gateway's credential was rotated away
// from, mirroring what an update to the provider's own credential does
// (LLMProviderService.Update). Deleting is soft and is refused outright while anything
// still references the handle, so a secret another gateway — or this provider itself —
// is still using survives.
//
// Best-effort by design: the deployment is already live by the time this runs, and a
// secret left behind is not a reason to report the deploy as failed.
func (s *LLMProviderDeploymentService) cleanupRotatedCredential(
	orgUUID, previousCredential string, metadata map[string]interface{}, actor string) {

	if s.secretService == nil || previousCredential == "" {
		return
	}
	current, _ := metadata[constants.MetadataKeyUpstreamAuthValue].(string)
	if strings.TrimSpace(current) == "" {
		// Nothing replaced it, so nothing was rotated. A deploy that carries no
		// credential of its own is not a removal: it runs on the provider's own, and a
		// client that knows nothing about per-deployment credentials sends metadata
		// like that on every redeploy. Releasing here would destroy a secret nobody
		// asked to remove, and because the value is write-only no client could resend
		// it to get it back. Left in place it is merely unreferenced, which the
		// organization's secrets page can clear.
		return
	}
	s.secretService.cleanupRotatedSecret(orgUUID, previousCredential, current, actor, s.slogger)
}

// namingBuild reports the build a deployment runs alongside the rest of its response.
//
// LLM providers, LLM proxies and MCP proxies are all built the way a REST API is — `base:
// build` runs the build it names and `base: current` stores what it renders — so each can
// say which build it is running. The shared response builder does not set it, because a
// kind that has no builds shares that builder too.
func namingBuild(resp *api.DeploymentResponse, err error, buildID *string) (*api.DeploymentResponse, error) {
	if err != nil || resp == nil {
		return resp, err
	}
	resp.BuildId = buildID
	return resp, nil
}

// applyUpstreamOverrides customizes the upstream this deployment talks to: which backend,
// and what it authenticates with. The provider's own definition is untouched, so a build
// stays what it was.
func (s *LLMProviderDeploymentService) applyUpstreamOverrides(
	providerDeployment *dto.LLMProviderDeploymentYAML, metadata map[string]interface{}, orgUUID string) error {

	if err := applyUpstreamURLOverride(providerDeployment, metadata); err != nil {
		return err
	}
	return s.applyUpstreamAuthOverride(providerDeployment, metadata, orgUUID)
}

// applyUpstreamURLOverride replaces the backend this deployment routes to, under the same
// metadata key a REST API's endpoint uses, so one gateway can be pointed at a regional or
// proxied endpoint without changing the provider.
//
// It clears any `ref`: the platform requires exactly one of url and ref, and a deployment
// that names a URL has chosen the url form.
func applyUpstreamURLOverride(
	providerDeployment *dto.LLMProviderDeploymentYAML, metadata map[string]interface{}) error {

	raw, given := metadata[constants.MetadataKeyEndpointUrl]
	if !given {
		return nil
	}
	endpoint, ok := raw.(string)
	if !ok {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q must be a string, got %T.", constants.MetadataKeyEndpointUrl, raw))
	}
	if endpoint = strings.TrimSpace(endpoint); endpoint == "" {
		return nil
	}
	if err := validateEndpointURL(endpoint); err != nil {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q is not a valid endpoint URL: %s.", constants.MetadataKeyEndpointUrl, err))
	}
	providerDeployment.Spec.Upstream.URL = endpoint
	providerDeployment.Spec.Upstream.Ref = ""
	return nil
}

// applyUpstreamAuthOverride replaces the credential this deployment authenticates to
// the provider's upstream with, leaving the provider's own definition untouched. It is
// what lets one provider be deployed to several gateways, each holding a different
// account with the same LLM vendor.
//
// The value is a {{ secret "handle" }} reference, never the credential itself, and is
// refused otherwise. Deployment metadata is returned with every read of a deployment,
// so a literal here would be a credential readable by anyone who can list deployments —
// whereas a reference names a secret the platform already guards. It also costs nothing
// to carry: the rendered content's references are recorded per gateway on deploy
// (upsertDeploymentSecretRefs), which is what syncs the secret and protects it from
// being deleted while a gateway is serving it.
//
// An absent or empty value leaves the provider's own credential in place, matching how
// the endpoint and vhost overrides treat one.
func (s *LLMProviderDeploymentService) applyUpstreamAuthOverride(
	providerDeployment *dto.LLMProviderDeploymentYAML, metadata map[string]interface{}, orgUUID string) error {

	raw, given := metadata[constants.MetadataKeyUpstreamAuthValue]
	if !given {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q must be a string, got %T.", constants.MetadataKeyUpstreamAuthValue, raw))
	}
	if value = strings.TrimSpace(value); value == "" {
		return nil
	}
	if !isSecretReference(value) {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q must be a secret reference of the form {{ secret \"handle\" }}, so the credential itself is not stored on the deployment.",
			constants.MetadataKeyUpstreamAuthValue))
	}
	if s.secretService != nil {
		if err := s.secretService.ValidateSecretRefs(orgUUID, value); err != nil {
			// The validator names the handles it could not resolve, which is how it
			// reports a whole config at once. Here there is exactly one, and the caller
			// supplied it, so naming it back buys nothing and puts a secret handle into
			// the response body and the request log. Say only that it did not resolve.
			if apperror.ValidationFailed.Is(err) {
				return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
					"Metadata %q references a secret that does not exist in this organization.",
					constants.MetadataKeyUpstreamAuthValue))
			}
			return err
		}
	}

	// Only an upstream that authenticates at all can have its credential replaced.
	// Setting one on a provider whose upstream takes none would ship an auth block the
	// gateway has no use for, and would read as though the deployment were authenticating
	// when it is not.
	auth := providerDeployment.Spec.Upstream.Auth
	if auth == nil || auth.Type == nil || isCredentialLessUpstreamAuthType(string(*auth.Type)) {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"This provider's upstream takes no credential, so %q cannot be set for a deployment of it.",
			constants.MetadataKeyUpstreamAuthValue))
	}
	auth.Value = &value

	return s.applyUpstreamAuthHeaderOverride(auth, metadata)
}

// applyUpstreamAuthHeaderOverride replaces the header this deployment sends its upstream
// credential in.
//
// It applies only to an api-key upstream, because that is the only type whose header is a
// choice: basic and bearer both send Authorization by definition, and changing it would
// produce a request the vendor does not recognise. It is also only read alongside a
// credential — a header on its own would name where to put a key this deployment does not
// have.
func (s *LLMProviderDeploymentService) applyUpstreamAuthHeaderOverride(
	auth *api.UpstreamAuth, metadata map[string]interface{}) error {

	raw, given := metadata[constants.MetadataKeyUpstreamAuthHeader]
	if !given {
		return nil
	}
	header, ok := raw.(string)
	if !ok {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q must be a string, got %T.", constants.MetadataKeyUpstreamAuthHeader, raw))
	}
	if header = strings.TrimSpace(header); header == "" {
		return nil
	}
	if normalizeUpstreamAuthType(string(*auth.Type)) != string(api.ApiKey) {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q applies only to an upstream that authenticates with an api-key.",
			constants.MetadataKeyUpstreamAuthHeader))
	}
	if !isHTTPHeaderName(header) {
		return apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf(
			"Metadata %q must be a valid HTTP header name.", constants.MetadataKeyUpstreamAuthHeader))
	}
	auth.Header = &header
	return nil
}

// isHTTPHeaderName reports whether s is a valid HTTP field name (RFC 9110 token), so a
// header override cannot inject a second header or a request line.
func isHTTPHeaderName(s string) bool {
	if s == "" || len(s) > 256 {
		return false
	}
	for _, c := range s {
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if isAlphaNum || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return false
	}
	return true
}

// isSecretReference reports whether s is exactly a {{ secret "handle" }} placeholder,
// rather than merely containing one — a credential with a placeholder appended to it
// must not pass as a reference.
func isSecretReference(s string) bool {
	loc := constants.SecretPlaceholderRe.FindStringIndex(s)
	return loc != nil && loc[0] == 0 && loc[1] == len(s)
}

func generateLLMProviderDeploymentYAML(provider *model.LLMProvider, templateHandle string) (dto.LLMProviderDeploymentYAML, error) {
	if provider == nil {
		return dto.LLMProviderDeploymentYAML{}, apperror.Internal.New().WithLogMessage("generateLLMProviderDeploymentYAML: provider is nil")
	}
	if templateHandle == "" {
		return dto.LLMProviderDeploymentYAML{}, apperror.Internal.New().WithLogMessage("generateLLMProviderDeploymentYAML: template handle is empty")
	}
	if provider.Configuration.Upstream == nil || provider.Configuration.Upstream.Main == nil {
		return dto.LLMProviderDeploymentYAML{}, apperror.LLMProviderDeploymentValidationFailed.New("The LLM provider must define an upstream main configuration.")
	}
	main := provider.Configuration.Upstream.Main
	if main.URL == "" && main.Ref == "" {
		return dto.LLMProviderDeploymentYAML{}, apperror.LLMProviderDeploymentValidationFailed.New("The LLM provider upstream main must specify either a url or a ref.")
	}

	contextValue := "/"
	if provider.Configuration.Context != nil && *provider.Configuration.Context != "" {
		contextValue = *provider.Configuration.Context
	}
	vhostValue := ""
	if provider.Configuration.VHost != nil {
		vhostValue = *provider.Configuration.VHost
	}

	accessControl := api.LLMAccessControl{Mode: api.DenyAll}
	if provider.Configuration.AccessControl != nil {
		accessControl.Mode = api.LLMAccessControlMode(provider.Configuration.AccessControl.Mode)
		if len(provider.Configuration.AccessControl.Exceptions) > 0 {
			exceptions := make([]api.RouteException, 0, len(provider.Configuration.AccessControl.Exceptions))
			for _, e := range provider.Configuration.AccessControl.Exceptions {
				methods := make([]api.RouteExceptionMethods, 0, len(e.Methods))
				for _, m := range e.Methods {
					methods = append(methods, api.RouteExceptionMethods(m))
				}
				exceptions = append(exceptions, api.RouteException{Path: e.Path, Methods: methods})
			}
			accessControl.Exceptions = &exceptions
		}
	}

	var globalPolicies []api.Policy
	var operationPolicies []api.OperationPolicy
	policies := make([]api.LLMPolicy, 0)

	// Transform security config
	security := provider.Configuration.Security
	if security != nil && isBoolTrue(security.Enabled) {
		if security.APIKey != nil && isBoolTrue(security.APIKey.Enabled) {
			key := strings.TrimSpace(security.APIKey.Key)
			if key == "" {
				return dto.LLMProviderDeploymentYAML{}, apperror.LLMProviderDeploymentValidationFailed.New("invalid api key security configuration: key is required")
			}

			in := strings.ToLower(strings.TrimSpace(security.APIKey.In))
			if in != "header" && in != "query" {
				return dto.LLMProviderDeploymentYAML{}, apperror.LLMProviderDeploymentValidationFailed.New(fmt.Sprintf("invalid api key security configuration: in must be 'header' or 'query', got %q", security.APIKey.In))
			}

			params := map[string]interface{}{"key": key, "in": in}
			if prefix := strings.TrimSpace(security.APIKey.ValuePrefix); prefix != "" {
				params["valuePrefix"] = prefix
			}
			globalPolicies = append(globalPolicies, api.Policy{
				Name:   apiKeyAuthPolicyName,
				Params: &params,
			})
		}
	}

	// Transform rate limit config
	// Step 1: Convert rate limit config to policy format
	rateLimit := provider.Configuration.RateLimiting
	if rateLimit != nil {
		// Step 2: Provider level rate limit
		providerLevel := rateLimit.ProviderLevel
		if providerLevel != nil {
			// Priority to global rate limit configuration if both global and resource-wise are present
			if providerLevel.Global != nil {
				// Step 2.1 Handle global rate limiting — emits to globalPolicies (shared api-level bucket)
				if providerLevel.Global.Token != nil && providerLevel.Global.Token.Enabled {
					tokenLimit := providerLevel.Global.Token
					duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid token reset window: %w", err)
					}
					params := map[string]interface{}{
						"totalTokenLimits": []map[string]interface{}{
							{"count": tokenLimit.Count, "duration": duration},
						},
					}
					globalPolicies = append(globalPolicies, api.Policy{Name: tokenBasedRateLimitPolicyName, Version: "", Params: &params})
				}
				if providerLevel.Global.Request != nil && providerLevel.Global.Request.Enabled {
					requestLimit := providerLevel.Global.Request
					duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid request reset window: %w", err)
					}
					params := map[string]interface{}{
						"limits": []map[string]interface{}{
							{"requests": requestLimit.Count, "duration": duration},
						},
					}
					globalPolicies = append(globalPolicies, api.Policy{Name: basicRateLimitPolicyName, Version: "", Params: &params})
				}
				if providerLevel.Global.Cost != nil && providerLevel.Global.Cost.Enabled {
					costLimit := providerLevel.Global.Cost
					duration, err := formatRateLimitDuration(costLimit.Reset.Duration, costLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid cost reset window: %w", err)
					}
					costParams := map[string]interface{}{
						"budgetLimits": []map[string]interface{}{
							{"amount": costLimit.Amount, "duration": duration},
						},
					}
					globalPolicies = append(globalPolicies, api.Policy{Name: llmCostBasedRateLimitPolicyName, Version: "", Params: &costParams})
					if !hasGlobalPolicy(globalPolicies, llmCostPolicyName) {
						emptyParams := map[string]interface{}{}
						globalPolicies = append(globalPolicies, api.Policy{Name: llmCostPolicyName, Version: "", Params: &emptyParams})
					}
				}
			} else if providerLevel.ResourceWise != nil {
				// Step 2.2 Handle resource-wise rate limiting — emits to operationPolicies (per-path buckets)
				defaultLimit := &providerLevel.ResourceWise.Default

				// Step 2.2.1 Default resource-wise rate limit (path: /* catches all unmatched paths)
				if defaultLimit.Token != nil && defaultLimit.Token.Enabled {
					tokenLimit := defaultLimit.Token
					duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid token reset window: %w", err)
					}
					addOrAppendOperationPolicyPath(&operationPolicies, tokenBasedRateLimitPolicyName, "", api.OperationPolicyPath{
						Path:    "/*",
						Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk},
						Params: map[string]interface{}{
							"totalTokenLimits": []map[string]interface{}{
								{"count": tokenLimit.Count, "duration": duration},
							},
						},
					})
				}
				if defaultLimit.Request != nil && defaultLimit.Request.Enabled {
					requestLimit := defaultLimit.Request
					duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid request reset window: %w", err)
					}
					addOrAppendOperationPolicyPath(&operationPolicies, basicRateLimitPolicyName, "", api.OperationPolicyPath{
						Path:    "/*",
						Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk},
						Params: map[string]interface{}{
							"limits": []map[string]interface{}{
								{"requests": requestLimit.Count, "duration": duration},
							},
						},
					})
				}
				if defaultLimit.Cost != nil && defaultLimit.Cost.Enabled {
					costLimit := defaultLimit.Cost
					duration, err := formatRateLimitDuration(costLimit.Reset.Duration, costLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid cost reset window: %w", err)
					}
					addOrAppendOperationPolicyPath(&operationPolicies, llmCostBasedRateLimitPolicyName, "", api.OperationPolicyPath{
						Path:    "/*",
						Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk},
						Params: map[string]interface{}{
							"budgetLimits": []map[string]interface{}{
								{"amount": costLimit.Amount, "duration": duration},
							},
						},
					})
					if !hasOperationPolicy(operationPolicies, llmCostPolicyName) {
						operationPolicies = append(operationPolicies, api.OperationPolicy{
							Name:  llmCostPolicyName,
							Paths: []api.OperationPolicyPath{{Path: "/*", Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk}, Params: map[string]interface{}{}}},
						})
					}
				}

				// Step 2.2.2 Resource-wise rate limit (per specific resource path)
				for _, r := range providerLevel.ResourceWise.Resources {
					if r.Limit.Token != nil && r.Limit.Token.Enabled {
						tokenLimit := r.Limit.Token
						duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
						if err != nil {
							return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid token reset window for resource %s: %w", r.Resource, err)
						}
						addOrAppendOperationPolicyPath(&operationPolicies, tokenBasedRateLimitPolicyName, "", api.OperationPolicyPath{
							Path:    r.Resource,
							Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk},
							Params: map[string]interface{}{
								"totalTokenLimits": []map[string]interface{}{
									{"count": tokenLimit.Count, "duration": duration},
								},
							},
						})
					}
					if r.Limit.Request != nil && r.Limit.Request.Enabled {
						requestLimit := r.Limit.Request
						duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
						if err != nil {
							return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid request reset window for resource %s: %w", r.Resource, err)
						}
						addOrAppendOperationPolicyPath(&operationPolicies, basicRateLimitPolicyName, "", api.OperationPolicyPath{
							Path:    r.Resource,
							Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk},
							Params: map[string]interface{}{
								"limits": []map[string]interface{}{
									{"requests": requestLimit.Count, "duration": duration},
								},
							},
						})
					}
					if r.Limit.Cost != nil && r.Limit.Cost.Enabled {
						costLimit := r.Limit.Cost
						duration, err := formatRateLimitDuration(costLimit.Reset.Duration, costLimit.Reset.Unit)
						if err != nil {
							return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid cost reset window for resource %s: %w", r.Resource, err)
						}
						addOrAppendOperationPolicyPath(&operationPolicies, llmCostBasedRateLimitPolicyName, "", api.OperationPolicyPath{
							Path:    r.Resource,
							Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk},
							Params: map[string]interface{}{
								"budgetLimits": []map[string]interface{}{
									{"amount": costLimit.Amount, "duration": duration},
								},
							},
						})
						if !hasOperationPolicy(operationPolicies, llmCostPolicyName) {
							operationPolicies = append(operationPolicies, api.OperationPolicy{
								Name:  llmCostPolicyName,
								Paths: []api.OperationPolicyPath{{Path: "/*", Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsAsterisk}, Params: map[string]interface{}{}}},
							})
						}
					}
				}
			}
		}

		// Step 3: Consumer level rate limit
		consumerLevel := rateLimit.ConsumerLevel
		if consumerLevel != nil {
			if consumerLevel.Global != nil {
				if consumerLevel.Global.Token != nil && consumerLevel.Global.Token.Enabled {
					tokenLimit := consumerLevel.Global.Token
					duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid consumer token reset window: %w", err)
					}
					policies = append(policies, api.LLMPolicy{
						Name:    tokenBasedRateLimitPolicyName,
						Version: "",
						Paths: []api.LLMPolicyPath{
							{
								Path:    "/*",
								Methods: []api.LLMPolicyPathMethods{"*"},
								Params: map[string]interface{}{
									"totalTokenLimits": []map[string]interface{}{
										{
											"count":    tokenLimit.Count,
											"duration": duration,
										},
									},
									"consumerBased": true,
								},
							},
						},
					})
				}
				if consumerLevel.Global.Request != nil && consumerLevel.Global.Request.Enabled {
					requestLimit := consumerLevel.Global.Request
					duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid consumer request reset window: %w", err)
					}
					params := map[string]interface{}{
						"quotas": []map[string]interface{}{
							{
								"name": "consumer-request-limit",
								"limits": []map[string]interface{}{
									{"limit": requestLimit.Count, "duration": duration},
								},
								"keyExtraction": []map[string]interface{}{
									{"type": "apiname"},
									{"type": "metadata", "key": "x-wso2-application-id"},
								},
							},
						},
					}
					globalPolicies = append(globalPolicies, api.Policy{Name: advancedRateLimitPolicyName, Version: "", Params: &params})
				}
				if consumerLevel.Global.Cost != nil && consumerLevel.Global.Cost.Enabled {
					costLimit := consumerLevel.Global.Cost
					duration, err := formatRateLimitDuration(costLimit.Reset.Duration, costLimit.Reset.Unit)
					if err != nil {
						return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid consumer cost reset window: %w", err)
					}
					policies = append(policies, api.LLMPolicy{
						Name:    llmCostBasedRateLimitPolicyName,
						Version: "",
						Paths: []api.LLMPolicyPath{
							{
								Path:    "/*",
								Methods: []api.LLMPolicyPathMethods{"*"},
								Params: map[string]interface{}{
									"budgetLimits": []map[string]interface{}{
										{"amount": costLimit.Amount, "duration": duration},
									},
									"consumerBased": true,
								},
							},
						},
					})
					if !hasPolicy(policies, llmCostPolicyName) && !hasGlobalPolicy(globalPolicies, llmCostPolicyName) {
						policies = append(policies, api.LLMPolicy{
							Name:    llmCostPolicyName,
							Version: "",
							Paths: []api.LLMPolicyPath{
								{
									Path:    "/*",
									Methods: []api.LLMPolicyPathMethods{"*"},
									Params:  map[string]interface{}{},
								},
							},
						})
					}
				}
			} else if consumerLevel.ResourceWise != nil {
				for _, r := range consumerLevel.ResourceWise.Resources {
					if r.Limit.Token != nil && r.Limit.Token.Enabled {
						tokenLimit := r.Limit.Token
						duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
						if err != nil {
							return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid consumer token reset window for resource %s: %w", r.Resource, err)
						}
						addOrAppendPolicyPath(&policies, tokenBasedRateLimitPolicyName, "", api.LLMPolicyPath{
							Path:    r.Resource,
							Methods: []api.LLMPolicyPathMethods{"*"},
							Params: map[string]interface{}{
								"totalTokenLimits": []map[string]interface{}{
									{
										"count":    tokenLimit.Count,
										"duration": duration,
									},
								},
								"consumerBased": true,
							},
						})
					}
					if r.Limit.Request != nil && r.Limit.Request.Enabled {
						requestLimit := r.Limit.Request
						duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
						if err != nil {
							return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid consumer request reset window for resource %s: %w", r.Resource, err)
						}
						addOrAppendPolicyPath(&policies, advancedRateLimitPolicyName, "", api.LLMPolicyPath{
							Path:    r.Resource,
							Methods: []api.LLMPolicyPathMethods{"*"},
							Params: map[string]interface{}{
								"quotas": []map[string]interface{}{
									{
										"name": "consumer-request-limit",
										"limits": []map[string]interface{}{
											{
												"limit":    requestLimit.Count,
												"duration": duration,
											},
										},
										"keyExtraction": []map[string]interface{}{
											{"type": "routename"},
											{"type": "metadata", "key": "x-wso2-application-id"},
										},
									},
								},
							},
						})
					}
					if r.Limit.Cost != nil && r.Limit.Cost.Enabled {
						costLimit := r.Limit.Cost
						duration, err := formatRateLimitDuration(costLimit.Reset.Duration, costLimit.Reset.Unit)
						if err != nil {
							return dto.LLMProviderDeploymentYAML{}, fmt.Errorf("invalid consumer cost reset window for resource %s: %w", r.Resource, err)
						}
						addOrAppendPolicyPath(&policies, llmCostBasedRateLimitPolicyName, "", api.LLMPolicyPath{
							Path:    r.Resource,
							Methods: []api.LLMPolicyPathMethods{"*"},
							Params: map[string]interface{}{
								"budgetLimits": []map[string]interface{}{
									{"amount": costLimit.Amount, "duration": duration},
								},
								"consumerBased": true,
							},
						})
						if !hasPolicy(policies, llmCostPolicyName) && !hasGlobalPolicy(globalPolicies, llmCostPolicyName) {
							policies = append(policies, api.LLMPolicy{
								Name:    llmCostPolicyName,
								Version: "",
								Paths: []api.LLMPolicyPath{
									{
										Path:    "/*",
										Methods: []api.LLMPolicyPathMethods{"*"},
										Params:  map[string]interface{}{},
									},
								},
							})
						}
					}
				}
			}
		}
	}

	// Carry through user-set globalPolicies from the model (e.g. guardrails set via UI/API).
	// Snapshot the system-generated api-level policy names added above (rate limiting, llm-cost,
	// etc.) BEFORE this loop: a user policy that collides with one of those defers to the system
	// policy, but duplicates *among* user-set policies (e.g. two set-headers guardrails) must all
	// be preserved. Checking hasGlobalPolicy against the growing accumulator would drop the second
	// user duplicate, since the first was just appended to it.
	systemGlobalNames := make(map[string]bool, len(globalPolicies))
	for _, p := range globalPolicies {
		systemGlobalNames[p.Name] = true
	}
	for _, p := range provider.Configuration.GlobalPolicies {
		if systemGlobalNames[p.Name] {
			continue
		}
		params := p.Params
		entry := api.Policy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version)}
		if p.ExecutionCondition != "" {
			entry.ExecutionCondition = &p.ExecutionCondition
		}
		if params != nil {
			entry.Params = &params
		}
		globalPolicies = append(globalPolicies, entry)
	}

	// Carry through user-set operationPolicies from the model
	for _, p := range provider.Configuration.OperationPolicies {
		paths := make([]api.OperationPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.OperationPolicyPathMethods, 0, len(pp.Methods))
			for _, mm := range pp.Methods {
				methods = append(methods, api.OperationPolicyPathMethods(mm))
			}
			paths = append(paths, api.OperationPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		entry := api.OperationPolicy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version), Paths: paths}
		if p.ExecutionCondition != "" {
			entry.ExecutionCondition = &p.ExecutionCondition
		}
		operationPolicies = append(operationPolicies, entry)
	}

	// Carry through deprecated policies field
	for _, p := range provider.Configuration.Policies {
		// llm-cost is a parameterless tracker policy that the frontend attaches by default.
		// Skip it here if the rate limiting block already added it to avoid duplication.
		if p.Name == llmCostPolicyName && (hasGlobalPolicy(globalPolicies, llmCostPolicyName) || hasPolicy(policies, llmCostPolicyName)) {
			continue
		}
		paths := make([]api.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.LLMPolicyPathMethods(m))
			}
			paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		policies = append(policies, api.LLMPolicy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version), Paths: paths})
	}

	globalPolicies = orderLLMGlobalPolicies(globalPolicies)
	operationPolicies = orderLLMOperationPolicies(operationPolicies)
	policies = orderLLMPolicies(policies)

	upstream := dto.LLMUpstreamYAML{URL: main.URL, Ref: main.Ref}
	// Auth type. "none"/"other" carry only the type (no credentials);
	// "api-key" carries the header and value. Absent auth => "none".
	upstream.Auth = mapModelAuthToAPI(main.Auth)

	providerDeployment := dto.LLMProviderDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProvider,
		Metadata: dto.DeploymentMetadata{
			Name: provider.ID,
		},
		Spec: dto.LLMProviderDeploymentSpec{
			DisplayName:       provider.Name,
			Version:           provider.Version,
			Context:           contextValue,
			VHost:             vhostValue,
			Template:          templateHandle,
			Upstream:          upstream,
			AccessControl:     accessControl,
			GlobalPolicies:    globalPolicies,
			OperationPolicies: operationPolicies,
			Policies:          policies,
		},
	}

	// Promote any legacy policies assembled by the generator (security api-key-auth,
	// consumer-level rate limits) into operationPolicies — the canonical latest format.
	// The deploy orchestration layer applies version-aware transformation afterward.
	for _, p := range providerDeployment.Spec.Policies {
		paths := make([]api.OperationPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.OperationPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.OperationPolicyPathMethods(m))
			}
			paths = append(paths, api.OperationPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		providerDeployment.Spec.OperationPolicies = append(providerDeployment.Spec.OperationPolicies, api.OperationPolicy{
			Name:    p.Name,
			Version: p.Version,
			Paths:   paths,
		})
	}
	providerDeployment.Spec.Policies = nil

	return providerDeployment, nil
}

func formatRateLimitDuration(duration int, unit string) (string, error) {
	if duration <= 0 {
		return "", fmt.Errorf("duration must be positive, got %d", duration)
	}

	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "minute":
		return fmt.Sprintf("%dm", duration), nil
	case "hour":
		return fmt.Sprintf("%dh", duration), nil
	case "day":
		return fmt.Sprintf("%dh", duration*24), nil
	case "week":
		return fmt.Sprintf("%dh", duration*24*7), nil
	case "month":
		// policy accepts Go duration units; month is represented as 30 days.
		return fmt.Sprintf("%dh", duration*24*30), nil
	default:
		return "", fmt.Errorf("unsupported reset unit: %q", unit)
	}
}

func normalizePolicyVersionToMajor(version string) string {
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return trimmedVersion
	}

	versionWithoutPrefix := trimmedVersion
	if strings.HasPrefix(strings.ToLower(versionWithoutPrefix), "v") {
		versionWithoutPrefix = versionWithoutPrefix[1:]
	}
	if versionWithoutPrefix == "" {
		return trimmedVersion
	}

	majorVersion := versionWithoutPrefix
	if idx := strings.Index(majorVersion, "."); idx >= 0 {
		majorVersion = majorVersion[:idx]
	}
	if idx := strings.Index(majorVersion, "-"); idx >= 0 {
		majorVersion = majorVersion[:idx]
	}
	majorVersion = strings.TrimSpace(majorVersion)
	if majorVersion == "" {
		return trimmedVersion
	}

	if _, err := strconv.Atoi(majorVersion); err != nil {
		return trimmedVersion
	}

	return "v" + majorVersion
}

func addOrAppendPolicyPath(policies *[]api.LLMPolicy, name, version string, path api.LLMPolicyPath) {
	newConsumerBased, _ := path.Params["consumerBased"].(bool)

	for i := range *policies {
		if (*policies)[i].Name == name && (*policies)[i].Version == version {
			// Only merge entries that share the same scope (backend vs consumer)
			if len((*policies)[i].Paths) > 0 {
				existingConsumerBased, _ := (*policies)[i].Paths[0].Params["consumerBased"].(bool)
				if existingConsumerBased != newConsumerBased {
					continue // different scope — skip, look for another entry
				}
			}
			for _, existingPath := range (*policies)[i].Paths {
				if existingPath.Path == path.Path {
					// Keep first occurrence and avoid duplicates.
					return
				}
			}
			(*policies)[i].Paths = append((*policies)[i].Paths, path)
			return
		}
	}

	*policies = append(*policies, api.LLMPolicy{
		Name:    name,
		Version: version,
		Paths:   []api.LLMPolicyPath{path},
	})
}

func hasPolicy(policies []api.LLMPolicy, name string) bool {
	for _, p := range policies {
		if p.Name == name {
			return true
		}
	}
	return false
}

func isBoolTrue(v *bool) bool {
	return v != nil && *v
}

// marshalDeploymentMetadata serializes the deployment metadata map to the JSON form
// stored in the metadata column shared with artifact_gateway_mappings. An empty or
// nil map yields an empty string ("no metadata").
func marshalDeploymentMetadata(m map[string]any) (string, error) {
	if len(m) == 0 {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal deployment metadata: %w", err)
	}
	return string(b), nil
}

// unmarshalDeploymentMetadata parses the JSON metadata form back into a map for the
// deployment record. An empty string yields an empty (non-nil) map.
func unmarshalDeploymentMetadata(s string) (map[string]any, error) {
	if strings.TrimSpace(s) == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
	}
	return m, nil
}

// DeployLLMProxy creates a new immutable deployment artifact and deploys it to a gateway
func (s *LLMProxyDeploymentService) DeployLLMProxy(proxyID string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, apperror.LLMProxyDeploymentValidationFailed.New("A request body is required.")
	}
	base, requestedBuild, err := ValidateDeployBase(req.Base, req.BuildId,
		apperror.LLMProxyDeploymentValidationFailed)
	if err != nil {
		return nil, err
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, apperror.LLMProxyDeploymentValidationFailed.New("Gateway ID is required.")
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}
	gatewayID := gateway.ID

	// Get LLM proxy
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.LLMProxyNotFound.New()
	}

	// DP-originated artifacts are read-only in the control plane and cannot be
	// (re)deployed from the CP.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return nil, err
	}

	// Validate deployment name is provided
	if req.Name == "" {
		return nil, apperror.LLMProxyDeploymentValidationFailed.New("Deployment name is required.")
	}

	// Ensure a gateway association exists for the target gateway before deploying, and
	// resolve the deployment metadata. The first deployment to a gateway creates the
	// association and seeds its metadata from this deployment. For an existing
	// association the deploy request value overrides for this deployment; when the
	// metadata field is omitted, the association's stored metadata is used. An existing
	// association's metadata is never modified at deploy time.
	metadataProvided := req.Metadata != nil
	deployMetaJSON, err := marshalDeploymentMetadata(metadata)
	if err != nil {
		return nil, err
	}
	effectiveMetaJSON, err := s.proxyRepo.EnsureGatewayAssociation(proxy.UUID, gatewayID, orgUUID, createdBy, deployMetaJSON, metadataProvided)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure gateway association: %w", err)
	}
	if metadata, err = unmarshalDeploymentMetadata(effectiveMetaJSON); err != nil {
		return nil, err
	}

	// What this deploy ships: a build prepared earlier, or a snapshot of the proxy
	// as it stands now. A snapshot comes back unstored so it commits with the
	// deployment below.
	source, err := s.builds.SourceForDeploy(proxy.UUID, orgUUID, constants.LLMProxy, createdBy, base, requestedBuild)
	if err != nil {
		return nil, err
	}
	proxyDeployment, ok := source.Definition.(*dto.LLMProxyDeploymentYAML)
	if !ok {
		return nil, fmt.Errorf("artifact %s did not render as an LLM proxy definition", proxy.UUID)
	}
	sourceDataVersion := gatewaytranslator.PlatformDataVersion(source.DataVersion)
	targetDataVersion := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
	if err := gatewaytranslator.Translate(
		constants.LLMProxy,
		sourceDataVersion,
		targetDataVersion,
		proxyDeployment,
	); err != nil {
		return nil, fmt.Errorf("failed to transform LLM proxy deployment for gateway %s: %w", gateway.Version, err)
	}
	contentBytes, err := yaml.Marshal(proxyDeployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLM proxy deployment YAML: %w", err)
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}
	deployed := model.DeploymentStatusDeployed

	deployment := &model.Deployment{
		DeploymentID:   deploymentID,
		Name:           req.Name,
		ArtifactID:     proxy.UUID,
		OrganizationID: orgUUID,
		GatewayID:      gatewayID,
		BuildUUID:      source.BuildUUID,
		BuildID:        source.BuildID,
		Content:        contentBytes,
		Metadata:       metadata,
		Status:         &deployed,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	// A build rendered for this deploy is stored with the deployment, in one
	// transaction, so a recorded deployment always has the build it runs.
	if source.NewBuild != nil {
		err = s.deploymentRepo.CreateWithBuild(deployment, source.NewBuild,
			s.cfg.Deployments.MaxBuildsPerAPI, hardLimit)
	} else {
		err = s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit)
	}
	if err != nil {
		if limitErr := s.builds.LimitError(err); limitErr != err {
			return nil, limitErr
		}
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		proxy.UUID, orgUUID, gatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		return nil, fmt.Errorf("failed to set deployment status for LLM proxy: %w", err)
	}

	// Broadcast LLM proxy deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.LLMProxyDeploymentEvent{
			ProxyId:      proxy.UUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastLLMProxyDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM proxy deployment event", "error", err)
		}

		// Push existing active API keys for this proxy to the gateway (see BackfillAPIKeysToGateway).
		BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, proxy.UUID, gatewayID, "")
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

// RestoreLLMProxyDeployment restores a previous deployment (ARCHIVED or UNDEPLOYED)
func (s *LLMProxyDeploymentService) RestoreLLMProxyDeployment(proxyID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.LLMProxyNotFound.New()
	}
	// DP-originated artifacts are read-only in the control plane; restore cannot be CP-initiated.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return nil, err
	}

	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	// gatewayID is a gateway handle (matching deploy); resolve it to the internal
	// gateway UUID stored on the deployment before comparing.
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

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(proxy.UUID, orgUUID, targetDeployment.GatewayID)
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

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		proxy.UUID, orgUUID, targetDeployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Broadcast LLM proxy deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.LLMProxyDeploymentEvent{
			ProxyId:      proxy.UUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastLLMProxyDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM proxy deployment event", "error", err)
		}

		// Backfill existing active API keys to the gateway (see BackfillAPIKeysToGateway).
		BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, proxy.UUID, targetDeployment.GatewayID, "")
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, targetDeployment.BuildID)
}

// UndeployLLMProxyDeployment undeploys an active deployment
func (s *LLMProxyDeploymentService) UndeployLLMProxyDeployment(proxyID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.LLMProxyNotFound.New()
	}
	// DP-originated artifacts are read-only in the control plane: their deploy/undeploy
	// lifecycle is owned by the data-plane gateway, so undeployment cannot be initiated
	// from the control plane.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return nil, err
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	// gatewayID is a gateway handle (matching deploy); resolve it to the internal
	// gateway UUID stored on the deployment before comparing.
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
		return nil, apperror.DeploymentNotActive.New("LLM proxy")
	}

	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, apperror.GatewayNotFound.New()
	}

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusUndeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	newUpdatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		proxy.UUID, orgUUID, deployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Broadcast LLM proxy undeployment event to gateway
	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.LLMProxyUndeploymentEvent{
			ProxyId:      proxy.UUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastLLMProxyUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM proxy undeployment event", "error", err)
		}
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

// DeleteLLMProxyDeployment permanently deletes an undeployed deployment artifact
func (s *LLMProxyDeploymentService) DeleteLLMProxyDeployment(proxyID, deploymentID, orgUUID string) error {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return err
	}
	if proxy == nil {
		return apperror.LLMProxyNotFound.New()
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return apperror.DeploymentNotFound.New()
	}
	if deployment.Status != nil && deployment.Status.IsDeployedOrDeploying() {
		return apperror.DeploymentActive.New()
	}

	if err := s.deploymentRepo.Delete(deploymentID, proxy.UUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// GetLLMProxyDeployments retrieves all deployments for a proxy with optional filters
func (s *LLMProxyDeploymentService) GetLLMProxyDeployments(proxyID, orgUUID string, gatewayID *string, status *string) (*api.DeploymentListResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.LLMProxyNotFound.New()
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

	// The gatewayId filter is a gateway handle (matching deploy/undeploy); resolve it
	// to the internal gateway UUID stored in deployments.gateway_uuid before filtering.
	gatewayUUID, found, err := resolveGatewayFilter(s.gatewayRepo, gatewayID, orgUUID)
	if err != nil {
		return nil, err
	}
	if !found {
		// The filter names a gateway that does not exist in this org: no deployment matches.
		return &api.DeploymentListResponse{Count: 0, List: []api.DeploymentResponse{}}, nil
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(proxy.UUID, orgUUID, gatewayUUID, status, s.cfg.Deployments.MaxPerAPIGateway)
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
		if err == nil && mapped != nil {
			mapped.BuildId = d.BuildID
		}
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

// GetLLMProxyDeployment retrieves a specific deployment by ID
func (s *LLMProxyDeploymentService) GetLLMProxyDeployment(proxyID, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.LLMProxyNotFound.New()
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

func generateLLMProxyDeploymentYAML(proxy *model.LLMProxy) (dto.LLMProxyDeploymentYAML, error) {
	if proxy == nil {
		return dto.LLMProxyDeploymentYAML{}, apperror.Internal.New().WithLogMessage("generateLLMProxyDeploymentYAML: proxy is nil")
	}
	if proxy.Configuration.Provider == "" {
		return dto.LLMProxyDeploymentYAML{}, apperror.LLMProxyDeploymentValidationFailed.New("The LLM proxy must reference a provider.")
	}

	contextValue := "/"
	if proxy.Configuration.Context != nil && *proxy.Configuration.Context != "" {
		contextValue = *proxy.Configuration.Context
	}
	vhostValue := ""
	if proxy.Configuration.Vhost != nil {
		vhostValue = *proxy.Configuration.Vhost
	}

	var proxyGlobalPolicies []api.Policy
	var proxyOperationPolicies []api.OperationPolicy
	proxyPolicies := make([]api.LLMPolicy, 0)

	// Transform security config
	security := proxy.Configuration.Security
	if security != nil && isBoolTrue(security.Enabled) {
		if security.APIKey != nil && isBoolTrue(security.APIKey.Enabled) {
			key := strings.TrimSpace(security.APIKey.Key)
			if key == "" {
				return dto.LLMProxyDeploymentYAML{}, apperror.LLMProxyDeploymentValidationFailed.New("invalid api key security configuration: key is required")
			}

			in := strings.ToLower(strings.TrimSpace(security.APIKey.In))
			if in != "header" && in != "query" {
				return dto.LLMProxyDeploymentYAML{}, apperror.LLMProxyDeploymentValidationFailed.New(fmt.Sprintf("invalid api key security configuration: in must be 'header' or 'query', got %q", security.APIKey.In))
			}

			params := map[string]interface{}{"key": key, "in": in}
			if prefix := strings.TrimSpace(security.APIKey.ValuePrefix); prefix != "" {
				params["valuePrefix"] = prefix
			}
			proxyGlobalPolicies = append(proxyGlobalPolicies, api.Policy{
				Name:   apiKeyAuthPolicyName,
				Params: &params,
			})
		}
	}

	// Carry through user-set globalPolicies from the model. Snapshot the system-generated
	// api-level policy names added above BEFORE this loop so a user policy only defers to a
	// system policy of the same name — duplicates among user-set policies (e.g. two set-headers
	// guardrails) must all be preserved. See the provider path above for the full rationale.
	systemProxyGlobalNames := make(map[string]bool, len(proxyGlobalPolicies))
	for _, p := range proxyGlobalPolicies {
		systemProxyGlobalNames[p.Name] = true
	}
	for _, p := range proxy.Configuration.GlobalPolicies {
		if systemProxyGlobalNames[p.Name] {
			continue
		}
		params := p.Params
		entry := api.Policy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version)}
		if p.ExecutionCondition != "" {
			entry.ExecutionCondition = &p.ExecutionCondition
		}
		if params != nil {
			entry.Params = &params
		}
		proxyGlobalPolicies = append(proxyGlobalPolicies, entry)
	}

	// Carry through user-set operationPolicies from the model
	for _, p := range proxy.Configuration.OperationPolicies {
		paths := make([]api.OperationPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.OperationPolicyPathMethods, 0, len(pp.Methods))
			for _, mm := range pp.Methods {
				methods = append(methods, api.OperationPolicyPathMethods(mm))
			}
			paths = append(paths, api.OperationPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		entry := api.OperationPolicy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version), Paths: paths}
		if p.ExecutionCondition != "" {
			entry.ExecutionCondition = &p.ExecutionCondition
		}
		proxyOperationPolicies = append(proxyOperationPolicies, entry)
	}

	// Carry through deprecated policies field
	for _, p := range proxy.Configuration.Policies {
		paths := make([]api.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.LLMPolicyPathMethods(m))
			}
			paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		proxyPolicies = append(proxyPolicies, api.LLMPolicy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version), Paths: paths})
	}

	proxyPolicies = orderLLMPolicies(proxyPolicies)

	proxyDeployment := dto.LLMProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProxy,
		Metadata: dto.DeploymentMetadata{
			Name: proxy.ID,
		},
		Spec: dto.LLMProxyDeploymentSpec{
			DisplayName: proxy.Name,
			Version:     proxy.Version,
			Context:     contextValue,
			VHost:       vhostValue,
			Provider: dto.LLMProxyDeploymentProvider{
				ID: proxy.Configuration.Provider,
			},
			GlobalPolicies:    proxyGlobalPolicies,
			OperationPolicies: proxyOperationPolicies,
			Policies:          proxyPolicies,
		},
	}

	// Auth type. "none"/"other" carry only the type (no credentials);
	// "api-key" carries the header and value. Absent auth => "none".
	proxyDeployment.Spec.Provider.Auth = mapModelAuthToAPI(proxy.Configuration.UpstreamAuth)

	// Carry additional providers (multi-provider proxies) into the deployment
	// artifact so the gateway-controller can expose each as a selectable upstream.
	if len(proxy.Configuration.AdditionalProviders) > 0 {
		additional := make([]dto.LLMProxyDeploymentAdditionalProvider, 0, len(proxy.Configuration.AdditionalProviders))
		for _, ap := range proxy.Configuration.AdditionalProviders {
			entry := dto.LLMProxyDeploymentAdditionalProvider{
				ID: ap.ID,
				As: ap.As,
			}
			if ap.Transformer != nil {
				entry.Transformer = &api.LLMProxyTransformer{
					Type:    ap.Transformer.Type,
					Version: ap.Transformer.Version,
				}
				if len(ap.Transformer.Params) > 0 {
					params := ap.Transformer.Params
					entry.Transformer.Params = &params
				}
			}
			additional = append(additional, entry)
		}
		proxyDeployment.Spec.AdditionalProviders = additional
	}

	// Promote any legacy policies assembled by the generator into operationPolicies.
	for _, p := range proxyDeployment.Spec.Policies {
		paths := make([]api.OperationPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.OperationPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.OperationPolicyPathMethods(m))
			}
			paths = append(paths, api.OperationPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		proxyDeployment.Spec.OperationPolicies = append(proxyDeployment.Spec.OperationPolicies, api.OperationPolicy{
			Name:    p.Name,
			Version: p.Version,
			Paths:   paths,
		})
	}
	proxyDeployment.Spec.Policies = nil

	return proxyDeployment, nil
}

// mapModelAuthToAPI converts a stored model.UpstreamAuth into the api.UpstreamAuth
// The gateway accepts an explicit type of "api-key", "other", or "none"
// absent/empty auth defaults to "none", and the credential-less types ("none"/"other") carry only
// the type - no header/value. "api-key" (and legacy basic/bearer) carry the header and value.
func mapModelAuthToAPI(auth *model.UpstreamAuth) *api.UpstreamAuth {
	if auth == nil {
		t := api.None
		return &api.UpstreamAuth{Type: &t}
	}
	authType := string(api.None)
	if normalized := normalizeUpstreamAuthType(auth.Type); normalized != "" {
		authType = normalized
	}
	t := api.UpstreamAuthType(authType)
	if isCredentialLessUpstreamAuthType(authType) {
		return &api.UpstreamAuth{Type: &t}
	}
	return &api.UpstreamAuth{
		Type:   &t,
		Header: utils.StringPtrIfNotEmpty(auth.Header),
		Value:  utils.StringPtrIfNotEmpty(auth.Value),
	}
}

// orderLLMPolicies ensures llm-cost-based-ratelimit always precedes llm-cost in the policy list.
// All other policies retain their relative positions.
func orderLLMPolicies(policies []api.LLMPolicy) []api.LLMPolicy {
	costIdx := -1
	rateLimitIdx := -1
	for i, p := range policies {
		switch p.Name {
		case llmCostPolicyName:
			costIdx = i
		case llmCostBasedRateLimitPolicyName:
			rateLimitIdx = i
		}
	}
	if costIdx != -1 && rateLimitIdx != -1 && costIdx < rateLimitIdx {
		policies[costIdx], policies[rateLimitIdx] = policies[rateLimitIdx], policies[costIdx]
	}
	return policies
}

// orderLLMGlobalPolicies ensures llm-cost-based-ratelimit precedes llm-cost in the global policy list.
func orderLLMGlobalPolicies(policies []api.Policy) []api.Policy {
	costIdx := -1
	rateLimitIdx := -1
	for i, p := range policies {
		switch p.Name {
		case llmCostPolicyName:
			costIdx = i
		case llmCostBasedRateLimitPolicyName:
			rateLimitIdx = i
		}
	}
	if costIdx != -1 && rateLimitIdx != -1 && costIdx < rateLimitIdx {
		policies[costIdx], policies[rateLimitIdx] = policies[rateLimitIdx], policies[costIdx]
	}
	return policies
}

// orderLLMOperationPolicies ensures llm-cost-based-ratelimit precedes llm-cost in the operation policy list.
func orderLLMOperationPolicies(policies []api.OperationPolicy) []api.OperationPolicy {
	costIdx := -1
	rateLimitIdx := -1
	for i, p := range policies {
		switch p.Name {
		case llmCostPolicyName:
			costIdx = i
		case llmCostBasedRateLimitPolicyName:
			rateLimitIdx = i
		}
	}
	if costIdx != -1 && rateLimitIdx != -1 && costIdx < rateLimitIdx {
		policies[costIdx], policies[rateLimitIdx] = policies[rateLimitIdx], policies[costIdx]
	}
	return policies
}

func hasGlobalPolicy(policies []api.Policy, name string) bool {
	for _, p := range policies {
		if p.Name == name {
			return true
		}
	}
	return false
}

func hasOperationPolicy(policies []api.OperationPolicy, name string) bool {
	for _, p := range policies {
		if p.Name == name {
			return true
		}
	}
	return false
}

// addOrAppendOperationPolicyPath adds a path to an existing OperationPolicy entry with the
// given name, or appends a new entry if none exists.
func addOrAppendOperationPolicyPath(policies *[]api.OperationPolicy, name, version string, path api.OperationPolicyPath) {
	for i := range *policies {
		if (*policies)[i].Name == name && (*policies)[i].Version == version {
			for _, existing := range (*policies)[i].Paths {
				if existing.Path == path.Path {
					return // already present, skip
				}
			}
			(*policies)[i].Paths = append((*policies)[i].Paths, path)
			return
		}
	}
	*policies = append(*policies, api.OperationPolicy{Name: name, Version: version, Paths: []api.OperationPolicyPath{path}})
}
