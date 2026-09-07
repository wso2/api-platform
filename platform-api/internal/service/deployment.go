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
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
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

// The two sources a deployment can come from. Every deployment runs a build
// either way: `build` names one prepared earlier, and `current` renders one from
// the API's definition as part of the deploy.
const (
	deployBaseCurrent = "current"
	deployBaseBuild   = "build"
)

// vhostLabelRe matches a single valid DNS label per RFC 1035.
var vhostLabelRe = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// DeploymentService handles business logic for API deployment operations
type DeploymentService struct {
	apiRepo              repository.APIRepository
	artifactRepo         repository.ArtifactRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	auditRepo            repository.AuditRepository
	apiUtil              *utils.APIUtil
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(
	apiRepo repository.APIRepository,
	artifactRepo repository.ArtifactRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	auditRepo repository.AuditRepository,
	apiUtil *utils.APIUtil,
	cfg *config.Server,
	slogger *slog.Logger,
) *DeploymentService {
	return &DeploymentService{
		apiRepo:              apiRepo,
		artifactRepo:         artifactRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		auditRepo:            auditRepo,
		apiUtil:              apiUtil,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// CreateBuild renders the API's current definition into an immutable snapshot and
// stores it, without deploying it anywhere.
//
// Preparing and deploying are separate on purpose: a build fixes WHAT will be
// deployed at a known moment, so a later deploy cannot silently pick up edits made
// since, and the same snapshot can be deployed to any number of gateways and
// promoted onward without being re-rendered. The artifact is stored at the
// platform's own data version — the target gateway is not known yet, so
// translation happens at deploy time.
func (s *DeploymentService) CreateBuild(apiUUID, orgUUID, createdBy string,
	metadata map[string]interface{}) (*api.BuildResponse, error) {
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}
	// DP-originated artifacts are read-only in the control plane, so there is
	// nothing here to snapshot and deploy.
	if err := ensureOriginMutable(apiModel.Origin); err != nil {
		return nil, err
	}

	build, _, err := s.renderBuild(apiModel, apiUUID, orgUUID, createdBy, metadata)
	if err != nil {
		return nil, err
	}
	if err := s.deploymentRepo.CreateBuildWithLimitEnforcement(build, s.cfg.Deployments.MaxBuildsPerAPI); err != nil {
		return nil, err
	}
	s.slogger.Debug("Build created", "buildID", build.BuildID, "apiUUID", apiUUID)
	return toAPIBuildResponse(build), nil
}

// renderBuild renders an API's current definition into a build that has not been
// stored yet, and hands back the struct it was rendered from alongside it.
// Preparing a build stores it on its own; deploying from `current` stores it on the
// transaction that records the deployment. The struct is returned so that path can
// apply its overrides and translate for the target gateway without re-parsing what
// it has just written — and those overrides never reach the build, whose content is
// marshalled here: a build is the definition as it stood, not one deployment's
// customization of it.
func (s *DeploymentService) renderBuild(apiModel *model.API, apiUUID, orgUUID, createdBy string,
	metadata map[string]interface{}) (*model.Build, *dto.APIDeploymentYAML, error) {
	apiDeployment, err := s.apiUtil.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build API deployment YAML: %w", err)
	}
	contentBytes, err := yaml.Marshal(apiDeployment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal API deployment YAML: %w", err)
	}
	return &model.Build{
		ArtifactID:     apiUUID,
		OrganizationID: orgUUID,
		Content:        contentBytes,
		DataVersion:    apiModel.DataVersion,
		Metadata:       metadata,
		CreatedBy:      createdBy,
	}, apiDeployment, nil
}

// GetBuild returns one build of an API.
func (s *DeploymentService) GetBuild(apiUUID, buildID, orgUUID string) (*api.BuildResponse, error) {
	build, err := s.deploymentRepo.GetBuild(buildID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if build == nil {
		return nil, apperror.BuildNotFound.New()
	}
	return toAPIBuildResponse(build), nil
}

// GetBuilds lists an API's builds, newest first.
func (s *DeploymentService) GetBuilds(apiUUID, orgUUID string, limit int) (*api.BuildListResponse, error) {
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}
	builds, err := s.deploymentRepo.GetBuilds(apiUUID, orgUUID, limit)
	if err != nil {
		return nil, err
	}
	list := make([]api.BuildResponse, 0, len(builds))
	for _, build := range builds {
		list = append(list, *toAPIBuildResponse(build))
	}
	return &api.BuildListResponse{Count: len(list), List: list}, nil
}

// toAPIBuildResponse projects a stored build onto the API response.
func toAPIBuildResponse(build *model.Build) *api.BuildResponse {
	out := &api.BuildResponse{
		BuildId:     build.BuildID,
		Uuid:        utils.ParseOpenAPIUUIDOrZero(build.UUID),
		DataVersion: utils.StringPtrIfNotEmpty(build.DataVersion),
		CreatedBy:   utils.StringPtrIfNotEmpty(build.CreatedBy),
		CreatedAt:   build.CreatedAt,
	}
	if len(build.Metadata) > 0 {
		metadata := build.Metadata
		out.Metadata = &metadata
	}
	return out
}

// DeployAPI creates a new immutable deployment artifact and deploys it to a
// gateway. Every deployment runs a build: base "build" deploys one prepared
// earlier, and base "current" renders one from the API's definition and stores it
// with the deployment, so what a gateway is serving is always traceable to a
// snapshot and the next environment always has something to promote.
func (s *DeploymentService) DeployAPI(apiUUID string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("A request body is required.")
	}
	base := strings.TrimSpace(req.Base)
	if base == "" {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("Base is required (use 'current' or 'build').")
	}
	if base != deployBaseCurrent && base != deployBaseBuild {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("Base must be 'current' or 'build'.")
	}
	// base says which of the two this is, so buildId is expected with one and
	// meaningless with the other. Rejecting it where it cannot apply keeps a request
	// from looking like it asked for something it did not get.
	requestedBuild := strings.TrimSpace(utils.ValueOrEmpty(req.BuildId))
	if base == deployBaseBuild && requestedBuild == "" {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("A buildId is required when base is 'build'.")
	}
	if base == deployBaseCurrent && requestedBuild != "" {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("A buildId applies only when base is 'build'.")
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("Gateway ID is required.")
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

	// Get API
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}

	// DP-originated artifacts are read-only in the control plane and cannot be
	// (re)deployed from the CP.
	if err := ensureOriginMutable(apiModel.Origin); err != nil {
		return nil, err
	}

	// Validate deployment name is provided
	if req.Name == "" {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("Deployment name is required.")
	}

	// The artifact this deployment runs comes from a build either way, so each base
	// resolves to one: `build` to the snapshot it names, `current` to a snapshot of
	// the definition taken now, which is stored along with the deployment.
	//
	// apiDeployment is that build's artifact as a struct, ready for this
	// deployment's overrides and for translation to the target gateway below.
	var apiDeployment *dto.APIDeploymentYAML
	var sourceDataVersion gatewaytranslator.PlatformDataVersion
	// newBuild is the build this deploy renders and stores; buildUUID/buildReadableID
	// name a build prepared earlier. Exactly one of the two is set.
	var newBuild *model.Build
	var buildUUID *string
	var buildReadableID *string

	switch base {
	case deployBaseBuild:
		baseBuild, err := s.deploymentRepo.GetBuild(requestedBuild, apiUUID, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get build: %w", err)
		}
		if baseBuild == nil {
			return nil, apperror.BuildNotFound.New()
		}
		apiDeployment = &dto.APIDeploymentYAML{}
		if err := yaml.Unmarshal(baseBuild.Content, apiDeployment); err != nil {
			return nil, fmt.Errorf("failed to parse build YAML: %w", err)
		}
		sourceDataVersion = gatewaytranslator.PlatformDataVersion(baseBuild.DataVersion)
		// Record which build this deployment runs, so it can be traced back to the
		// snapshot it came from.
		buildUUID = &baseBuild.UUID
		buildReadableID = &baseBuild.BuildID
	case deployBaseCurrent:
		var err error
		newBuild, apiDeployment, err = s.renderBuild(apiModel, apiUUID, orgUUID, createdBy, nil)
		if err != nil {
			return nil, err
		}
		sourceDataVersion = gatewaytranslator.PlatformDataVersion(apiModel.DataVersion)
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	// Declare override variables
	var endpointURL *string
	var vhostMainOverridden bool
	var vhostSandboxOverridden bool

	// A build carries no vhost, so default to the sentinel and let the gateway
	// resolve and persist its own.
	mainSentinel := constants.VhostGatewayDefault
	vhostMain := &mainSentinel
	var vhostSandbox *string
	if apiModel.Configuration.Upstream.Sandbox != nil {
		sandboxSentinel := constants.VhostGatewayDefault
		vhostSandbox = &sandboxSentinel
	}

	// Apply overrides from metadata (endpointUrl, vhostMain, vhostSandbox)
	if req.Metadata != nil {
		if v, exists := metadata[constants.MetadataKeyEndpointUrl]; exists {
			eu, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("invalid endpoint URL in metadata: expected string, got %T", v)
			}
			if eu != "" {
				if err := validateEndpointURL(eu); err != nil {
					return nil, fmt.Errorf("invalid endpoint URL in metadata: %w", err)
				}
				endpointURL = &eu
			}
		}

		if v, exists := metadata[constants.MetadataKeyVhostMain]; exists {
			vm, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("invalid vhostMain in metadata: expected string, got %T", v)
			}
			if vm != "" {
				if !isValidVHostOrSentinel(vm) {
					return nil, fmt.Errorf("invalid vhostMain in metadata: %s", vm)
				}
				val := vm
				vhostMain = &val
				vhostMainOverridden = true
			}
		}

		if v, exists := metadata[constants.MetadataKeyVhostSandbox]; exists {
			vs, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("invalid vhostSandbox in metadata: expected string, got %T", v)
			}
			if vs != "" {
				if !isValidVHostOrSentinel(vs) {
					return nil, fmt.Errorf("invalid vhostSandbox in metadata: %s", vs)
				}
				val := vs
				vhostSandbox = &val
				vhostSandboxOverridden = true
			}
		}
	}

	// The build's artifact, customized for this deployment and translated to the
	// target gateway's data version. Builds are stored at a platform data version,
	// so a build prepared before a gateway upgrade still deploys onto it.
	applyStructOverrides(apiDeployment, endpointURL, vhostMain, vhostSandbox)
	targetDataVersion := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
	if err := gatewaytranslator.Translate(apiModel.Kind, sourceDataVersion, targetDataVersion, apiDeployment); err != nil {
		return nil, fmt.Errorf("failed to transform API deployment for gateway %s: %w", gateway.Version, err)
	}
	contentBytes, err := yaml.Marshal(apiDeployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API deployment YAML: %w", err)
	}
	if endpointURL != nil {
		s.slogger.Debug("Endpoint URL overridden", "endpointURL", *endpointURL, "deploymentID", deploymentID)
	}
	if vhostMainOverridden {
		s.slogger.Debug("Vhost main overridden", "vhostMain", *vhostMain, "deploymentID", deploymentID)
	}
	if vhostSandboxOverridden {
		s.slogger.Debug("Vhost sandbox overridden", "vhostSandbox", *vhostSandbox, "deploymentID", deploymentID)
	}

	// Store vhost in metadata so it is returned in the deployment response.
	if vhostMain != nil {
		metadata[constants.MetadataKeyVhostMain] = *vhostMain
	}
	if vhostSandbox != nil {
		metadata[constants.MetadataKeyVhostSandbox] = *vhostSandbox
	}

	// Create new deployment record with limit enforcement.
	// Hard limit = soft limit (configured) + 5 buffer for concurrent deployments
	deployment := &model.Deployment{
		DeploymentID:   deploymentID,
		Name:           req.Name,
		ArtifactID:     apiUUID,
		OrganizationID: orgUUID,
		GatewayID:      gatewayID,
		BuildUUID:      buildUUID,
		BuildID:        buildReadableID,
		Content:        contentBytes,
		Metadata:       metadata,
		CreatedBy:      createdBy,
	}

	// Both writes handle count, cleanup, insert and status update atomically.
	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	// A build rendered for this deploy is stored with the deployment, in one
	// transaction: a deployment that is recorded always has the build it runs, and a
	// deploy that fails leaves no build behind. One prepared earlier is already
	// stored, so recording the deployment only has to confirm it is still there.
	if newBuild != nil {
		err = s.deploymentRepo.CreateWithBuild(deployment, newBuild,
			s.cfg.Deployments.MaxBuildsPerAPI, hardLimit)
	} else {
		err = s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Ensure API-Gateway association exists
	if err := s.ensureAPIGatewayAssociation(apiUUID, gatewayID, orgUUID, createdBy); err != nil {
		s.slogger.Warn("Failed to ensure API-gateway association", "error", err)
	}

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		apiUUID, orgUUID, gatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		s.slogger.Warn("Failed to set deployment status", "error", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.DeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast deployment event", "error", err)
		}

		// Push existing active API keys for this artifact to the (possibly newly
		// associated) gateway so keys created before this association are recognized
		// immediately, rather than only after the controller's next reconnect sync.
		s.backfillAPIKeysToGateway(apiUUID, gatewayID, createdBy)
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
	if err != nil {
		return nil, err
	}
	resp.BuildId = deployment.BuildID
	return resp, nil
}

// RestoreDeployment restores a previous deployment (can be ARCHIVED or UNDEPLOYED)
func (s *DeploymentService) RestoreDeployment(apiUUID, deploymentID, gatewayID, orgUUID, actor string) (*api.DeploymentResponse, error) {
	// DP-originated artifacts are read-only in the control plane; their deployment
	// lifecycle is owned by the data-plane gateway, so restore cannot be CP-initiated.
	if err := ensureArtifactMutableByUUID(s.artifactRepo, apiUUID, orgUUID); err != nil {
		return nil, err
	}

	// Verify target deployment exists and belongs to the API
	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	// Validate that the provided gatewayID matches the deployment's bound gateway
	if targetDeployment.GatewayID != gatewayID {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}

	// Verify target deployment is NOT currently DEPLOYED
	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(apiUUID, orgUUID, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == deploymentID && status.IsDeployedOrDeploying() {
		return nil, apperror.DeploymentRestoreConflict.New()
	}

	// Validate gateway exists and belongs to organization
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
		apiUUID, orgUUID, targetDeployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.DeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast deployment event", "error", err)
		}

		// Push existing active API keys for this artifact to the gateway so a restored
		// deployment recognizes pre-existing keys immediately (see backfillAPIKeysToGateway).
		s.backfillAPIKeysToGateway(apiUUID, targetDeployment.GatewayID, actor)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("RESTORE", deploymentID, "deployment", orgUUID, actor)
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
	if err != nil {
		return nil, err
	}
	resp.BuildId = targetDeployment.BuildID
	return resp, nil
}

// UndeployDeployment undeploys an active deployment
func (s *DeploymentService) UndeployDeployment(apiUUID, deploymentID, gatewayID, orgUUID, actor string) (*api.DeploymentResponse, error) {
	// DP-originated artifacts are read-only in the control plane: their deploy/undeploy
	// lifecycle is owned by the data-plane gateway (driven by the DP->CP push), so the
	// control plane must not initiate an undeployment for them.
	if err := ensureArtifactMutableByUUID(s.artifactRepo, apiUUID, orgUUID); err != nil {
		return nil, err
	}

	// Verify deployment exists and belongs to API
	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	// Validate that the provided gatewayID matches the deployment's bound gateway
	if deployment.GatewayID != gatewayID {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}

	// Verify deployment is currently DEPLOYED (status already populated by GetDeploymentWithState)
	if deployment.Status == nil || !deployment.Status.IsDeployedOrDeploying() {
		return nil, apperror.DeploymentNotActive.New("API")
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}

	// Transitional until the gateway acknowledges the artifact.
	initialStatus := model.DeploymentStatusUndeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	newUpdatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		apiUUID, orgUUID, deployment.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send undeployment event to gateway
	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.APIUndeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast undeployment event", "error", err)
		}
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("UNDEPLOY", deploymentID, "deployment", orgUUID, actor)
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
	if err != nil {
		return nil, err
	}
	resp.BuildId = deployment.BuildID
	return resp, nil
}

// DeleteDeployment permanently deletes an undeployed deployment artifact
func (s *DeploymentService) DeleteDeployment(apiUUID, deploymentID, orgUUID, actor string) error {
	// Verify deployment exists and belongs to the API
	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return apperror.DeploymentNotFound.New()
	}

	// Verify deployment is NOT currently DEPLOYED (status already populated by GetDeploymentWithState)
	if deployment.Status != nil && deployment.Status.IsDeployedOrDeploying() {
		return apperror.DeploymentActive.New()
	}

	// Delete the deployment artifact
	if err := s.deploymentRepo.Delete(deploymentID, apiUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", deploymentID, "deployment", orgUUID, actor)
	}

	return nil
}

// HandleDeploymentAck processes a deployment acknowledgement from the gateway.
// It validates the ack, checks the performed_at concurrency token, and transitions
// the deployment status accordingly.
func (s *DeploymentService) HandleDeploymentAck(gatewayID, orgID string, ack *model.DeploymentAckPayload) error {
	if ack == nil {
		return fmt.Errorf("ack payload is nil")
	}
	if ack.ArtifactID == "" || ack.DeploymentID == "" {
		return fmt.Errorf("ack missing required fields: artifactId=%q, deploymentId=%q", ack.ArtifactID, ack.DeploymentID)
	}

	s.slogger.Info("Processing deployment ack",
		"gatewayID", gatewayID, "artifactID", ack.ArtifactID,
		"deploymentID", ack.DeploymentID, "action", ack.Action,
		"status", ack.Status, "performedAt", ack.PerformedAt)

	if ack.ArtifactID == "" {
		s.slogger.Info("Ack received for unknown deployment, discarding",
			"gatewayID", gatewayID, "deploymentID", ack.DeploymentID)
		return nil
	}

	if ack.Status == "failed" {
		// Failure ack: overwrite any status (DEPLOYING, DEPLOYED, UNDEPLOYING) to FAILED
		// as long as performed_at matches
		rowsAffected, err := s.deploymentRepo.UpdateStatusWithPerformedAtGuard(
			ack.ArtifactID, orgID, gatewayID,
			model.DeploymentStatusFailed, ack.ErrorCode,
			ack.PerformedAt, nil,
		)
		if err != nil {
			return fmt.Errorf("failed to update status for failure ack: %w", err)
		}
		if rowsAffected == 0 {
			s.slogger.Info("Stale failure ack discarded (performed_at mismatch)",
				"gatewayID", gatewayID, "artifactID", ack.ArtifactID)
		}
		return nil
	}

	if ack.Status == "success" {
		var newStatus model.DeploymentStatus
		var requiredStatuses []model.DeploymentStatus

		switch ack.Action {
		case "deploy":
			newStatus = model.DeploymentStatusDeployed
			requiredStatuses = []model.DeploymentStatus{model.DeploymentStatusDeploying}
		case "undeploy":
			newStatus = model.DeploymentStatusUndeployed
			requiredStatuses = []model.DeploymentStatus{model.DeploymentStatusUndeploying}
		default:
			return fmt.Errorf("unknown ack action: %s", ack.Action)
		}

		rowsAffected, err := s.deploymentRepo.UpdateStatusWithPerformedAtGuard(
			ack.ArtifactID, orgID, gatewayID,
			newStatus, "",
			ack.PerformedAt, requiredStatuses,
		)
		if err != nil {
			return fmt.Errorf("failed to update status for success ack: %w", err)
		}
		if rowsAffected == 0 {
			s.slogger.Info("Success ack discarded (stale or status already changed)",
				"gatewayID", gatewayID, "artifactID", ack.ArtifactID,
				"action", ack.Action)
		}
		return nil
	}

	return fmt.Errorf("unknown ack status: %s", ack.Status)
}

// validateEndpointURL validates the format of an endpoint URL
func validateEndpointURL(endpointURL string) error {
	if endpointURL == "" {
		return fmt.Errorf("endpoint URL cannot be empty")
	}

	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate scheme (must be http or https)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got: %s", parsedURL.Scheme)
	}

	// Validate host is present
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	return nil
}

// isValidVHostOrSentinel returns true if vhost is the gateway-default sentinel or a valid RFC 1035 hostname.
func isValidVHostOrSentinel(vhost string) bool {
	if vhost == constants.VhostGatewayDefault {
		return true
	}
	if vhost == "" {
		return false
	}
	labels := strings.Split(vhost, ".")
	for _, label := range labels {
		if !vhostLabelRe.MatchString(label) {
			return false
		}
	}
	return true
}

// applyEndpointOverride mutates upstream URL in deployment YAML and clears ref if URL is set.
func applyEndpointOverride(d *dto.APIDeploymentYAML, endpointURL *string) {
	if endpointURL == nil {
		return
	}
	if d.Spec.Upstream == nil {
		d.Spec.Upstream = &dto.UpstreamYAML{}
	}
	if d.Spec.Upstream.Main == nil {
		d.Spec.Upstream.Main = &dto.UpstreamTarget{}
	}
	d.Spec.Upstream.Main.URL = *endpointURL
	d.Spec.Upstream.Main.Ref = "" // Clear ref if URL is set
}

// applyStructOverrides mutates the deployment YAML struct directly for "current" flow.
// It applies endpoint override and selectively updates vhost fields when values are provided.
func applyStructOverrides(d *dto.APIDeploymentYAML, endpointURL *string, vhostMain *string, vhostSandbox *string) {
	applyEndpointOverride(d, endpointURL)
	if (vhostMain != nil && *vhostMain != "") || (vhostSandbox != nil && *vhostSandbox != "") {
		if d.Spec.Vhosts == nil {
			d.Spec.Vhosts = &dto.Vhosts{}
		}
		if vhostMain != nil && *vhostMain != "" {
			d.Spec.Vhosts.Main = vhostMain
		}
		if vhostSandbox != nil && *vhostSandbox != "" {
			d.Spec.Vhosts.Sandbox = vhostSandbox
		}
	}
}

// GetDeployments retrieves all deployments for an API with optional filters
func (s *DeploymentService) GetDeployments(apiUUID, orgUUID string, gatewayID *string, status *string) (*api.DeploymentListResponse, error) {
	// Verify API exists
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}

	// Validate status parameter
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

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	// Get deployments with state derived via LEFT JOIN
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(apiUUID, orgUUID, gatewayID, status, s.cfg.Deployments.MaxPerAPIGateway)
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

// GetDeployment retrieves a specific deployment by ID
func (s *DeploymentService) GetDeployment(apiUUID, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	// Verify API exists
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}

	// Get deployment with state derived via LEFT JOIN
	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgUUID)
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
	if err != nil {
		return nil, err
	}
	resp.BuildId = deployment.BuildID
	return resp, nil
}

// GetDeploymentContent retrieves the immutable content of a deployment
func (s *DeploymentService) GetDeploymentContent(apiUUID, deploymentID, orgUUID string) ([]byte, error) {
	// Get deployment with content
	deployment, err := s.deploymentRepo.GetWithContent(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	return deployment.Content, nil
}

// ensureAPIGatewayAssociation ensures an association exists between API and gateway
func (s *DeploymentService) ensureAPIGatewayAssociation(apiUUID, gatewayID, orgUUID, createdBy string) error {
	// Check if association already exists
	associations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		return err
	}

	for _, assoc := range associations {
		if assoc.GatewayID == gatewayID {
			// Association already exists
			return nil
		}
	}

	// Create new association
	association := &model.APIAssociation{
		ArtifactID:     apiUUID,
		OrganizationID: orgUUID,
		GatewayID:      gatewayID,
		CreatedBy:      createdBy,
	}

	return s.apiRepo.CreateAPIAssociation(association)
}

// backfillAPIKeysToGateway delegates to the shared BackfillAPIKeysToGateway helper so
// every deploy path (REST, LLM provider/proxy, MCP, WebSub, WebBroker) pushes existing
// keys identically.
func (s *DeploymentService) backfillAPIKeysToGateway(apiUUID, gatewayID, actor string) {
	BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, apiUUID, gatewayID, actor)
}

// CreateBuildByHandle prepares a build of an API identified by its handle.
func (s *DeploymentService) CreateBuildByHandle(apiHandle, orgUUID, createdBy string,
	metadata map[string]interface{}) (*api.BuildResponse, error) {

	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.CreateBuild(apiUUID, orgUUID, createdBy, metadata)
}

// GetBuildByHandle returns one build of an API identified by its handle.
func (s *DeploymentService) GetBuildByHandle(apiHandle, buildID, orgUUID string) (*api.BuildResponse, error) {
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.GetBuild(apiUUID, buildID, orgUUID)
}

// GetBuildsByHandle lists the builds of an API identified by its handle.
func (s *DeploymentService) GetBuildsByHandle(apiHandle, orgUUID string, limit int) (*api.BuildListResponse, error) {
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.GetBuilds(apiUUID, orgUUID, limit)
}

// DeployAPIByHandle creates a new immutable deployment artifact using API handle
func (s *DeploymentService) DeployAPIByHandle(apiHandle string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.DeployAPI(apiUUID, req, orgUUID, createdBy)
}

// RestoreDeploymentByHandle restores a previous deployment using API handle
func (s *DeploymentService) RestoreDeploymentByHandle(apiHandle, deploymentID, gatewayHandle, orgUUID, actor string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	// Resolve gateway handle to UUID (the deployment stores the gateway UUID).
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(strings.TrimSpace(gatewayHandle), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}

	return s.RestoreDeployment(apiUUID, deploymentID, gateway.ID, orgUUID, actor)
}

// getUUIDByHandle retrieves the artifact UUID by its handle from the artifact table
func (s *DeploymentService) getUUIDByHandle(handle, orgUUID string) (string, error) {
	if handle == "" {
		return "", apperror.ValidationFailed.New("artifact handle is required")
	}

	artifact, err := s.artifactRepo.GetByHandle(handle, orgUUID)
	if err != nil {
		return "", err
	}
	if artifact == nil {
		return "", apperror.ArtifactNotFound.New()
	}

	return artifact.UUID, nil
}

// GetDeploymentByHandle retrieves a single deployment using API handle
func (s *DeploymentService) GetDeploymentByHandle(apiHandle, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.GetDeployment(apiUUID, deploymentID, orgUUID)
}

// GetDeploymentsByHandle retrieves deployments for an API using handle
func (s *DeploymentService) GetDeploymentsByHandle(apiHandle, gatewayID, status, orgUUID string) (*api.DeploymentListResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	// Convert empty strings to nil for optional parameters
	var gatewayHandlePtr *string
	var statusPtr *string
	if gatewayID != "" {
		gatewayHandlePtr = &gatewayID
	}
	if status != "" {
		statusPtr = &status
	}

	// The gatewayId filter is a gateway handle (matching deploy/undeploy); resolve it
	// to the internal gateway UUID stored in deployments before filtering.
	gatewayUUID, found, err := resolveGatewayFilter(s.gatewayRepo, gatewayHandlePtr, orgUUID)
	if err != nil {
		return nil, err
	}
	if !found {
		// The filter names a gateway that does not exist in this org: no deployment matches.
		return &api.DeploymentListResponse{Count: 0, List: []api.DeploymentResponse{}}, nil
	}

	return s.GetDeployments(apiUUID, orgUUID, gatewayUUID, statusPtr)
}

// UndeployDeploymentByHandle undeploys a deployment using the API handle and the
// gateway handle. Deploy/attach both identify the gateway by handle, so undeploy
// resolves the handle to the gateway UUID here to keep the contract consistent.
func (s *DeploymentService) UndeployDeploymentByHandle(apiHandle, deploymentID, gatewayHandle, orgUUID, actor string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	// Resolve gateway handle to UUID (the deployment stores the gateway UUID).
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(strings.TrimSpace(gatewayHandle), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}

	return s.UndeployDeployment(apiUUID, deploymentID, gateway.ID, orgUUID, actor)
}

// DeleteDeploymentByHandle deletes a deployment using API handle
func (s *DeploymentService) DeleteDeploymentByHandle(apiHandle, deploymentID, orgUUID, actor string) error {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return err
	}

	return s.DeleteDeployment(apiUUID, deploymentID, orgUUID, actor)
}

// GetDeploymentContentByHandle retrieves deployment artifact content using API handle
func (s *DeploymentService) GetDeploymentContentByHandle(apiHandle, deploymentID, orgUUID string) ([]byte, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.GetDeploymentContent(apiUUID, deploymentID, orgUUID)
}

// resolveGatewayFilter resolves an optional gatewayId filter — supplied by clients
// as a gateway handle — to the internal gateway UUID stored in
// deployments.gateway_uuid. Deploy/undeploy identify the target gateway by handle,
// so the deployment listing must resolve the same way for the gatewayId filter to
// match any rows. Returns (uuidPtr, true, nil) when resolved (uuidPtr is nil when no
// filter was requested), or (nil, false, nil) when a handle was given but no gateway
// with that handle exists in the organization.
func resolveGatewayFilter(gatewayRepo repository.GatewayRepository, gatewayHandle *string, orgUUID string) (*string, bool, error) {
	if gatewayHandle == nil {
		return nil, true, nil
	}
	handle := strings.TrimSpace(*gatewayHandle)
	if handle == "" {
		return nil, true, nil
	}
	gateway, err := gatewayRepo.GetByHandleAndOrgID(handle, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to resolve gateway handle: %w", err)
	}
	if gateway == nil {
		return nil, false, nil
	}
	return &gateway.ID, true, nil
}

func toAPIDeploymentResponse(
	gatewayRepo repository.GatewayRepository,
	deploymentID string,
	name string,
	gatewayID string,
	status model.DeploymentStatus,
	baseDeploymentID *string,
	metadata map[string]interface{},
	createdAt time.Time,
	updatedAt *time.Time,
	statusReason *string,
) (*api.DeploymentResponse, error) {
	deploymentUUID := utils.ParseOpenAPIUUIDOrZero(deploymentID)
	baseUUID := utils.ParseOptionalOpenAPIUUID(baseDeploymentID)

	gateway, err := gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve gateway handle: %w", err)
	}
	gatewayHandle := gatewayID
	if gateway != nil {
		gatewayHandle = gateway.Handle
	}

	resp := &api.DeploymentResponse{
		BaseDeploymentId: baseUUID,
		CreatedAt:        createdAt,
		DeploymentId:     deploymentUUID,
		GatewayId:        gatewayHandle,
		Metadata:         utils.MapPtrIfNotEmpty(metadata),
		Name:             name,
		Status:           api.DeploymentResponseStatus(status),
		StatusReason:     statusReason,
		UpdatedAt:        updatedAt,
	}
	return resp, nil
}
