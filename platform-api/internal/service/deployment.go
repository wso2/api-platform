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
		targetDataVersion := gatewaytranslator.TargetGatewayDataVersion(gatewaytranslator.ParseVersion(gateway.Version))
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
		// Start from base deployment bytes
		contentBytes = baseDeployment.Content
		if needsOverride {
			// Single unmarshal -> apply overrides -> single marshal
			contentBytes, err = applyDeploymentOverrides(contentBytes, endpointURL, vhostMain, vhostSandbox, vhostMainOverridden, vhostSandboxOverridden)
			if err != nil {
				return nil, fmt.Errorf("failed to apply deployment overrides: %w", err)
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
		}
	}
	// If base: <deploymentId> and no overrides, contentBytes passes through unchanged.

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
	if err := s.ensureAPIGatewayAssociation(apiUUID, gatewayID, orgUUID); err != nil {
		s.slogger.Warn("Failed to ensure API-gateway association", "error", err)
	}

	// Set initial status based on config; transitional (DEPLOYING) only when enabled
	initialStatus := model.DeploymentStatusDeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusDeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
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
	if currentDeploymentID == deploymentID && status == model.DeploymentStatusDeployed {
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

	// Set initial status based on config; transitional (DEPLOYING) only when enabled
	initialStatus := model.DeploymentStatusDeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusDeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
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
	if deployment.Status == nil || *deployment.Status != model.DeploymentStatusDeployed {
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

	// Set initial status based on config; transitional (UNDEPLOYING) only when enabled
	initialStatus := model.DeploymentStatusUndeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusUndeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
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
	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
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

// applyDeploymentOverrides unmarshals deployment YAML bytes, applies endpoint URL and/or vhost
// overrides, and marshals back. Used for the base-deployment path when overrides are needed.
func applyDeploymentOverrides(contentBytes []byte, endpointURL *string, vhostMain *string, vhostSandbox *string, vhostMainOverridden bool, vhostSandboxOverridden bool) ([]byte, error) {
	var apiDeployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal(contentBytes, &apiDeployment); err != nil {
		return nil, fmt.Errorf("failed to parse deployment YAML: %w", err)
	}
	applyBaseStructOverrides(&apiDeployment, endpointURL, vhostMain, vhostSandbox, vhostMainOverridden, vhostSandboxOverridden)
	modifiedBytes, err := yaml.Marshal(&apiDeployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified deployment YAML: %w", err)
	}
	return modifiedBytes, nil
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
func (s *DeploymentService) ensureAPIGatewayAssociation(apiUUID, gatewayID, orgUUID string) error {
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
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return s.apiRepo.CreateAPIAssociation(association)
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
