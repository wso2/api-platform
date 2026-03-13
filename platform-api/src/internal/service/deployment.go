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
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

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
		apiUtil:              apiUtil,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// DeployAPI creates a new immutable deployment artifact and deploys it to a gateway
func (s *DeploymentService) DeployAPI(apiUUID string, req *api.DeployRequest, orgUUID string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Base == "" {
		return nil, constants.ErrDeploymentBaseRequired
	}
	gatewayID := utils.OpenAPIUUIDToString(req.GatewayId)
	if gatewayID == "" {
		return nil, constants.ErrDeploymentGatewayIDRequired
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	// Get API
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}

	// Validate deployment name is provided
	if req.Name == "" {
		return nil, constants.ErrDeploymentNameRequired
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
			if errors.Is(err, constants.ErrDeploymentNotFound) {
				return nil, constants.ErrBaseDeploymentNotFound
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

	// Collect endpoint URL override from metadata
	var endpointURL *string
	var needsOverride bool
	var vhostMainOverridden bool
	var vhostSandboxOverridden bool
	if req.Metadata != nil {
		if v, exists := metadata["endpointUrl"]; exists {
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
	}

	// Determine vhost values.
	// For "current" base: default to sentinel so the gateway resolves and persists its defaults.
	// For an existing deployment base: start from the base's stored vhosts, then apply any overrides.
	var vhostMain string
	var vhostSandbox *string

	if req.Base == "current" {
		// Fresh deployment: default to sentinel so the gateway resolves and persists its defaults.
		vhostMain = constants.VhostGatewayDefault
		if apiModel.Configuration.Upstream.Sandbox != nil {
			sandboxSentinel := constants.VhostGatewayDefault
			vhostSandbox = &sandboxSentinel
		}
	} else {
		// Base deployment: start from the base's stored vhosts.
		if baseDeployment != nil && baseDeployment.Metadata != nil {
			if m, ok := baseDeployment.Metadata[constants.MetadataKeyVhostMain]; ok {
				if ms, ok := m.(string); ok {
					vhostMain = ms
				}
			}
			if m, ok := baseDeployment.Metadata[constants.MetadataKeyVhostSandbox]; ok {
				if ms, ok := m.(string); ok {
					vhostSandbox = &ms
				}
			}
		}
	}

	if req.Vhost != nil {
		// Apply only the fields explicitly provided; leave others at their current value.
		if req.Vhost.Main != nil && *req.Vhost.Main != "" {
			if !isValidVHostOrSentinel(*req.Vhost.Main) {
				return nil, fmt.Errorf("invalid vhost.main value: %s", *req.Vhost.Main)
			}
			vhostMain = *req.Vhost.Main
			vhostMainOverridden = true
			needsOverride = true
		}
		if req.Vhost.Sandbox != nil && *req.Vhost.Sandbox != "" {
			if !isValidVHostOrSentinel(*req.Vhost.Sandbox) {
				return nil, fmt.Errorf("invalid vhost.sandbox value: %s", *req.Vhost.Sandbox)
			}
			vhostSandbox = req.Vhost.Sandbox
			vhostSandboxOverridden = true
			needsOverride = true
		}
	}

	// Build content bytes with minimal marshal/unmarshal
	if req.Base == "current" {
		// Build struct directly, apply overrides on struct, marshal once
		apiDeployment, err := s.apiUtil.BuildAPIDeploymentYAML(apiModel)
		if err != nil {
			return nil, fmt.Errorf("failed to build API deployment YAML: %w", err)
		}
		applyStructOverrides(apiDeployment, endpointURL, &vhostMain, vhostSandbox)
		contentBytes, err = yaml.Marshal(apiDeployment)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal API deployment YAML: %w", err)
		}
		if endpointURL != nil {
			s.slogger.Debug("Endpoint URL overridden", "endpointURL", *endpointURL, "deploymentID", deploymentID)
		}
		if vhostMainOverridden {
			s.slogger.Debug("Vhost main overridden", "vhostMain", vhostMain, "deploymentID", deploymentID)
		}
		if vhostSandboxOverridden {
			s.slogger.Debug("Vhost sandbox overridden", "vhostSandbox", *vhostSandbox, "deploymentID", deploymentID)
		}
	} else {
		// Start from base deployment bytes
		contentBytes = baseDeployment.Content
		if needsOverride {
			// Single unmarshal -> apply overrides -> single marshal
			contentBytes, err = applyDeploymentOverrides(contentBytes, endpointURL, &vhostMain, vhostSandbox, vhostMainOverridden, vhostSandboxOverridden)
			if err != nil {
				return nil, fmt.Errorf("failed to apply deployment overrides: %w", err)
			}
			if endpointURL != nil {
				s.slogger.Debug("Endpoint URL overridden", "endpointURL", *endpointURL, "deploymentID", deploymentID)
			}
			if vhostMainOverridden {
				s.slogger.Debug("Vhost main overridden", "vhostMain", vhostMain, "deploymentID", deploymentID)
			}
			if vhostSandboxOverridden {
				s.slogger.Debug("Vhost sandbox overridden", "vhostSandbox", *vhostSandbox, "deploymentID", deploymentID)
			}
		}
	}
	// If base: <deploymentId> and no overrides, contentBytes passes through unchanged.

	// Store vhost in metadata so it is returned in the deployment response.
	if vhostMain != "" {
		metadata[constants.MetadataKeyVhostMain] = vhostMain
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

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.DeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			Vhost:        gateway.Vhost,
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast deployment event", "error", err)
		}
	}

	// Return deployment response (status and updatedAt are set by CreateDeploymentWithLimitEnforcement)
	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		model.DeploymentStatusDeployed,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
	)
}

// RestoreDeployment restores a previous deployment (can be ARCHIVED or UNDEPLOYED)
func (s *DeploymentService) RestoreDeployment(apiUUID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	// Verify target deployment exists and belongs to the API
	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Validate that the provided gatewayID matches the deployment's bound gateway
	if targetDeployment.GatewayID != gatewayID {
		return nil, constants.ErrGatewayIDMismatch
	}

	// Verify target deployment is NOT currently DEPLOYED
	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(apiUUID, orgUUID, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == deploymentID && status == model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentAlreadyDeployed
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	// Use SetCurrentDeployment to activate the target deployment with status='DEPLOYED'
	updatedAt, err := s.deploymentRepo.SetCurrent(apiUUID, orgUUID, targetDeployment.GatewayID, deploymentID, model.DeploymentStatusDeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.DeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			Vhost:        gateway.Vhost,
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		targetDeployment.DeploymentID,
		targetDeployment.Name,
		targetDeployment.GatewayID,
		model.DeploymentStatusDeployed,
		targetDeployment.BaseDeploymentID,
		targetDeployment.Metadata,
		targetDeployment.CreatedAt,
		&updatedAt,
	)
}

// UndeployDeployment undeploys an active deployment
func (s *DeploymentService) UndeployDeployment(apiUUID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	// Verify deployment exists and belongs to API
	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Validate that the provided gatewayID matches the deployment's bound gateway
	if deployment.GatewayID != gatewayID {
		return nil, constants.ErrGatewayIDMismatch
	}

	// Verify deployment is currently DEPLOYED (status already populated by GetDeploymentWithState)
	if deployment.Status == nil || *deployment.Status != model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentNotActive
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}

	// Update status to UNDEPLOYED using SetCurrentDeployment
	newUpdatedAt, err := s.deploymentRepo.SetCurrent(apiUUID, orgUUID, deployment.GatewayID, deploymentID, model.DeploymentStatusUndeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send undeployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := gateway.Vhost
		undeploymentEvent := &model.APIUndeploymentEvent{
			ApiId: apiUUID,
			Vhost: vhost,
		}

		if err := s.gatewayEventsService.BroadcastUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast undeployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		model.DeploymentStatusUndeployed,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		&newUpdatedAt,
	)
}

// DeleteDeployment permanently deletes an undeployed deployment artifact
func (s *DeploymentService) DeleteDeployment(apiUUID, deploymentID, orgUUID string) error {
	// Verify deployment exists and belongs to the API
	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return constants.ErrDeploymentNotFound
	}

	// Verify deployment is NOT currently DEPLOYED (status already populated by GetDeploymentWithState)
	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
		return constants.ErrDeploymentIsDeployed
	}

	// Delete the deployment artifact
	if err := s.deploymentRepo.Delete(deploymentID, apiUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
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
// It applies endpoint override and sets the full vhosts object when vhostMain is provided.
func applyStructOverrides(d *dto.APIDeploymentYAML, endpointURL *string, vhostMain *string, vhostSandbox *string) {
	applyEndpointOverride(d, endpointURL)
	if vhostMain != nil {
		d.Spec.Vhosts = &dto.Vhosts{
			Main:    *vhostMain,
			Sandbox: vhostSandbox,
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
			d.Spec.Vhosts.Main = *vhostMain
		}
	}

	if vhostMainOverridden && vhostMain != nil {
		d.Spec.Vhosts.Main = *vhostMain
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
		return nil, constants.ErrAPINotFound
	}

	// Validate status parameter
	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):   true,
			string(model.DeploymentStatusUndeployed): true,
			string(model.DeploymentStatusArchived):   true,
		}
		if !validStatuses[*status] {
			return nil, constants.ErrInvalidDeploymentStatus
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
			d.DeploymentID,
			d.Name,
			d.GatewayID,
			*d.Status,
			d.BaseDeploymentID,
			d.Metadata,
			d.CreatedAt,
			d.UpdatedAt,
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
		return nil, constants.ErrAPINotFound
	}

	// Get deployment with state derived via LEFT JOIN
	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		*deployment.Status,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
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
		return nil, constants.ErrDeploymentNotFound
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
		if assoc.ResourceID == gatewayID {
			// Association already exists
			return nil
		}
	}

	// Create new association
	association := &model.APIAssociation{
		ArtifactID:      apiUUID,
		OrganizationID:  orgUUID,
		ResourceID:      gatewayID,
		AssociationType: constants.AssociationTypeGateway,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	return s.apiRepo.CreateAPIAssociation(association)
}

// DeployAPIByHandle creates a new immutable deployment artifact using API handle
func (s *DeploymentService) DeployAPIByHandle(apiHandle string, req *api.DeployRequest, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.DeployAPI(apiUUID, req, orgUUID)
}

// RestoreDeploymentByHandle restores a previous deployment using API handle
func (s *DeploymentService) RestoreDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.RestoreDeployment(apiUUID, deploymentID, gatewayID, orgUUID)
}

// getUUIDByHandle retrieves the artifact UUID by its handle from the artifact table
func (s *DeploymentService) getUUIDByHandle(handle, orgUUID string) (string, error) {
	if handle == "" {
		return "", errors.New("artifact handle is required")
	}

	artifact, err := s.artifactRepo.GetByHandle(handle, orgUUID)
	if err != nil {
		return "", err
	}
	if artifact == nil {
		return "", constants.ErrArtifactNotFound
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
	var gatewayIdPtr *string
	var statusPtr *string
	if gatewayID != "" {
		gatewayIdPtr = &gatewayID
	}
	if status != "" {
		statusPtr = &status
	}

	return s.GetDeployments(apiUUID, orgUUID, gatewayIdPtr, statusPtr)
}

// UndeployDeploymentByHandle undeploys a deployment using API handle
func (s *DeploymentService) UndeployDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.UndeployDeployment(apiUUID, deploymentID, gatewayID, orgUUID)
}

// DeleteDeploymentByHandle deletes a deployment using API handle
func (s *DeploymentService) DeleteDeploymentByHandle(apiHandle, deploymentID, orgUUID string) error {
	// Convert API handle to UUID
	apiUUID, err := s.getUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return err
	}

	return s.DeleteDeployment(apiUUID, deploymentID, orgUUID)
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

func toAPIDeploymentResponse(
	deploymentID string,
	name string,
	gatewayID string,
	status model.DeploymentStatus,
	baseDeploymentID *string,
	metadata map[string]interface{},
	createdAt time.Time,
	updatedAt *time.Time,
) (*api.DeploymentResponse, error) {
	deploymentUUID := utils.ParseOpenAPIUUIDOrZero(deploymentID)
	gatewayUUID := utils.ParseOpenAPIUUIDOrZero(gatewayID)
	baseUUID := utils.ParseOptionalOpenAPIUUID(baseDeploymentID)

	// Extract vhost from metadata and populate the response field.
	var vhost *api.APIVhost
	if vhostMainRaw, ok := metadata[constants.MetadataKeyVhostMain]; ok {
		if vhostMainStr, ok := vhostMainRaw.(string); ok {
			vhost = &api.APIVhost{Main: &vhostMainStr}
			if vhostSandboxRaw, ok := metadata[constants.MetadataKeyVhostSandbox]; ok {
				if vhostSandboxStr, ok := vhostSandboxRaw.(string); ok {
					vhost.Sandbox = &vhostSandboxStr
				} else {
					slog.Warn("unexpected type for vhost_sandbox metadata", "type", fmt.Sprintf("%T", vhostSandboxRaw))
				}
			}
		} else {
			slog.Warn("unexpected type for vhost_main metadata", "type", fmt.Sprintf("%T", vhostMainRaw))
		}
	}

	return &api.DeploymentResponse{
		BaseDeploymentId: baseUUID,
		CreatedAt:        createdAt,
		DeploymentId:     deploymentUUID,
		GatewayId:        gatewayUUID,
		Metadata:         utils.MapPtrIfNotEmpty(metadata),
		Name:             name,
		Status:           api.DeploymentResponseStatus(status),
		UpdatedAt:        updatedAt,
		Vhost:            vhost,
	}, nil
}
