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

// DeployAPI creates a new immutable deployment artifact and deploys it to a gateway
func (s *DeploymentService) DeployAPI(apiUUID string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("A request body is required.")
	}
	if req.Base == "" {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New("Base is required (use 'current' or a deploymentId).")
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

	var baseDeploymentID *string
	var contentBytes []byte
	var baseDeployment *model.Deployment

	// Determine the source: "current" or existing deployment
	if req.Base != "current" {
		// Use existing deployment as base
		var err error
		baseDeployment, err = s.deploymentRepo.GetWithContent(req.Base, apiUUID, orgUUID)
		if err != nil {
			if apperror.DeploymentNotFound.Is(err) {
				return nil, apperror.DeploymentBaseNotFound.Wrap(err)
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		baseDeploymentID = &req.Base
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	// Declare override variables
	var endpointURL *string
	var needsOverride bool
	var vhostMainOverridden bool
	var vhostSandboxOverridden bool

	// Determine vhost values.
	// For "current" base: default to sentinel so the gateway resolves and persists its defaults.
	// For an existing deployment base: start from the base's stored vhosts, then apply any overrides.
	var vhostMain *string
	var vhostSandbox *string

	if req.Base == "current" {
		// Fresh deployment: default to sentinel so the gateway resolves and persists its defaults.
		mainSentinel := constants.VhostGatewayDefault
		vhostMain = &mainSentinel
		if apiModel.Configuration.Upstream.Sandbox != nil {
			sandboxSentinel := constants.VhostGatewayDefault
			vhostSandbox = &sandboxSentinel
		}
	} else {
		// Base deployment: start from the base's stored vhosts.
		if baseDeployment != nil && baseDeployment.Metadata != nil {
			if m, ok := baseDeployment.Metadata[constants.MetadataKeyVhostMain]; ok {
				if ms, ok := m.(string); ok && ms != "" {
					val := ms
					vhostMain = &val
				}
			}
			if m, ok := baseDeployment.Metadata[constants.MetadataKeyVhostSandbox]; ok {
				if ms, ok := m.(string); ok && ms != "" {
					val := ms
					vhostSandbox = &val
				}
			}
		}
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
				needsOverride = true
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
				needsOverride = true
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
				needsOverride = true
			}
		}
	}

	// Build content bytes with minimal marshal/unmarshal
	if req.Base == "current" {
		// Build struct directly, apply overrides on struct, marshal once
		apiDeployment, err := s.apiUtil.BuildAPIDeploymentYAML(apiModel)
		if err != nil {
			return nil, fmt.Errorf("failed to build API deployment YAML: %w", err)
		}
		applyStructOverrides(apiDeployment, endpointURL, vhostMain, vhostSandbox)
		sourceDataVersion := gatewaytranslator.PlatformDataVersion(apiModel.DataVersion)
		targetDataVersion := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
		if err := gatewaytranslator.Translate(apiModel.Kind, sourceDataVersion, targetDataVersion, apiDeployment); err != nil {
			return nil, fmt.Errorf("failed to transform API deployment for gateway %s: %w", gateway.Version, err)
		}
		contentBytes, err = yaml.Marshal(apiDeployment)
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
	} else {
		// Promote from an existing deployment: start from that deployment's already
		// rendered artifact and NEVER re-read the base API definition. Re-translate it
		// to the target gateway's data version so promoting across gateways on
		// different versions still yields a valid artifact — the source data version
		// is computed from the base artifact's own apiVersion, and only the artifact
		// Kind (an immutable classifier, unchanged by any edit to the API) is read from
		// the API record, never its definition.
		var apiDeployment dto.APIDeploymentYAML
		if err := yaml.Unmarshal(baseDeployment.Content, &apiDeployment); err != nil {
			return nil, fmt.Errorf("failed to parse base deployment YAML: %w", err)
		}
		sourceDataVersion := gatewaytranslator.ComputeDataVersion(apiModel.Kind, apiDeployment.ApiVersion)
		targetDataVersion := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
		if err := gatewaytranslator.Translate(apiModel.Kind, sourceDataVersion, targetDataVersion, &apiDeployment); err != nil {
			return nil, fmt.Errorf("failed to transform base deployment for gateway %s: %w", gateway.Version, err)
		}
		if needsOverride {
			applyBaseStructOverrides(&apiDeployment, endpointURL, vhostMain, vhostSandbox, vhostMainOverridden, vhostSandboxOverridden)
			if endpointURL != nil {
				s.slogger.Debug("Endpoint URL overridden", "endpointURL", *endpointURL, "deploymentID", deploymentID)
			}
			if vhostMainOverridden {
				s.slogger.Debug("Vhost main overridden", "vhostMain", *vhostMain, "deploymentID", deploymentID)
			}
			if vhostSandboxOverridden {
				s.slogger.Debug("Vhost sandbox overridden", "vhostSandbox", *vhostSandbox, "deploymentID", deploymentID)
			}
		}
		contentBytes, err = yaml.Marshal(&apiDeployment)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal promoted deployment YAML: %w", err)
		}
	}

	// Apply the generic override document (customize any field of the config for
	// this deployment) onto the resolved definition, and persist it so it can be
	// read back and carried forward when this deployment is later used as a base.
	//
	// A promoted deployment starts from the base's already-overridden artifact, so
	// the base's override document is inherited (see effectiveOverrideDocument):
	// without it the record would report no overrides while its content carries
	// them.
	if req.Overrides != nil && len(*req.Overrides) > 0 {
		contentBytes, err = mergeGenericOverrides(contentBytes, *req.Overrides)
		if err != nil {
			return nil, fmt.Errorf("failed to apply deployment overrides: %w", err)
		}
		s.slogger.Debug("Generic overrides applied", "deploymentID", deploymentID)
	}
	if effective := effectiveOverrideDocument(baseDeployment, req.Overrides); effective != nil {
		metadata[constants.MetadataKeyOverrides] = effective
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
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ArtifactID:       apiUUID,
		OrganizationID:   orgUUID,
		GatewayID:        gatewayID,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         metadata,
		CreatedBy:        createdBy,
	}

	// Use CreateDeploymentWithLimitEnforcement - handles count, cleanup, insert, and status update atomically
	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit); err != nil {
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

// applyBaseStructOverrides mutates the deployment YAML struct for base-deployment flow.
// It applies endpoint override and selectively updates only overridden vhost fields.
func applyBaseStructOverrides(d *dto.APIDeploymentYAML, endpointURL *string, vhostMain *string, vhostSandbox *string, vhostMainOverridden bool, vhostSandboxOverridden bool) {
	applyEndpointOverride(d, endpointURL)

	if !vhostMainOverridden && !vhostSandboxOverridden {
		return
	}

	if d.Spec.Vhosts == nil {
		d.Spec.Vhosts = &dto.Vhosts{}
		if vhostMain != nil {
			d.Spec.Vhosts.Main = vhostMain
		}
	}

	if vhostMainOverridden && vhostMain != nil {
		d.Spec.Vhosts.Main = vhostMain
	}
	if vhostSandboxOverridden {
		d.Spec.Vhosts.Sandbox = vhostSandbox
	}
}

// protectedOverridePaths are the artifact's immutable identity fields, which an
// override must never change — doing so would repoint or redefine the API rather
// than customize a deployment of it. A customization that targets any of these is
// rejected.
var protectedOverridePaths = [][]string{
	{"apiVersion"},
	{"kind"},
	{"metadata", "name"},
	{"spec", "context"},
	{"spec", "version"},
	{"spec", "operations"},
	{"spec", "channels"},
}

// overrideProtectedPath reports the first protected identity path an override
// document sets (present at or below that path), if any.
func overrideProtectedPath(overrides map[string]interface{}) (string, bool) {
	for _, path := range protectedOverridePaths {
		cur := overrides
		reached := true
		for i, seg := range path {
			v, exists := cur[seg]
			if !exists {
				reached = false
				break
			}
			if i == len(path)-1 {
				break
			}
			m, isMap := asStringKeyedMap(v)
			if !isMap {
				reached = false
				break
			}
			cur = m
		}
		if reached {
			return strings.Join(path, "."), true
		}
	}
	return "", false
}

// effectiveOverrideDocument returns the override document to persist with a new
// deployment: the base deployment's document with the request's deep-merged on
// top. A promoted deployment starts from the base's already-overridden artifact,
// so it inherits that document — otherwise the record would read back as having
// no overrides while its content carries them. Returns nil when neither the base
// nor the request carries one, so no empty document is stored.
func effectiveOverrideDocument(base *model.Deployment, requested *map[string]interface{}) map[string]interface{} {
	effective := map[string]interface{}{}
	if base != nil && base.Metadata != nil {
		if inherited, ok := asStringKeyedMap(base.Metadata[constants.MetadataKeyOverrides]); ok {
			deepMergeMap(effective, inherited)
		}
	}
	if requested != nil {
		deepMergeMap(effective, *requested)
	}
	if len(effective) == 0 {
		return nil
	}
	return effective
}

// mergeGenericOverrides deep-merges a structured override document onto the
// deployment definition YAML, letting a caller customize any field of the API
// config for this deployment without the service needing to know the field. An
// override that targets an immutable identity field is rejected.
func mergeGenericOverrides(contentBytes []byte, overrides map[string]interface{}) ([]byte, error) {
	if path, bad := overrideProtectedPath(overrides); bad {
		return nil, apperror.RESTAPIDeploymentValidationFailed.New(
			fmt.Sprintf("Override targets the immutable field %q, which cannot be customized for a deployment.", path))
	}
	var base map[string]interface{}
	if err := yaml.Unmarshal(contentBytes, &base); err != nil {
		return nil, fmt.Errorf("failed to parse deployment YAML for override: %w", err)
	}
	if base == nil {
		base = map[string]interface{}{}
	}
	out, err := yaml.Marshal(deepMergeMap(base, overrides))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal overridden deployment YAML: %w", err)
	}
	return out, nil
}

// deepMergeMap recursively merges src into dst and returns dst. Nested maps are
// merged key-by-key; every other value in src replaces the value in dst. Keys
// absent from src are left untouched.
func deepMergeMap(dst, src map[string]interface{}) map[string]interface{} {
	for k, sv := range src {
		if svMap, ok := asStringKeyedMap(sv); ok {
			if dvMap, ok := asStringKeyedMap(dst[k]); ok {
				dst[k] = deepMergeMap(dvMap, svMap)
				continue
			}
			dst[k] = svMap
			continue
		}
		dst[k] = sv
	}
	return dst
}

// asStringKeyedMap normalizes the two map shapes YAML/JSON decoding can produce
// (map[string]interface{} and map[interface{}]interface{}) into a string-keyed
// map, reporting whether the value was a map at all.
func asStringKeyedMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprintf("%v", k)] = val
		}
		return out, true
	default:
		return nil, false
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
