/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package service

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

type MCPDeploymentService struct {
	artifactRepo         repository.ArtifactRepository
	mcpRepo              repository.MCPProxyRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	utils                *utils.MCPUtils
	slogger              *slog.Logger
}

func NewMCPDeploymentService(mcpRepo repository.MCPProxyRepository, deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository, orgRepo repository.OrganizationRepository, artifactRepo repository.ArtifactRepository,
	gatewayEventsService *GatewayEventsService, cfg *config.Server, slogger *slog.Logger) *MCPDeploymentService {
	return &MCPDeploymentService{
		mcpRepo:              mcpRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		artifactRepo:         artifactRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		utils:                &utils.MCPUtils{},
		slogger:              slogger,
	}
}

// DeployMCPProxyByHandle creates a new immutable deployment artifact using MCP proxy handle
func (s *MCPDeploymentService) DeployMCPProxyByHandle(proxyHandle string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.deployMCPProxy(proxyUUID, req, orgUUID, createdBy)
}

// RestoreMCPDeploymentByHandle restores a previous deployment using the MCP proxy
// handle and the gateway handle. Deploy resolves the gateway by handle, so restore
// resolves the handle to the gateway UUID here to keep the contract consistent.
func (s *MCPDeploymentService) RestoreMCPDeploymentByHandle(proxyHandle, deploymentID, gatewayHandle, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
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
	gatewayUUID := gateway.ID

	return s.restoreMCPProxyDeployment(proxyUUID, &deploymentID, &gatewayUUID, orgUUID)
}

// UndeployDeploymentByHandle undeploys a deployment using the MCP proxy handle and
// the gateway handle. Deploy resolves the gateway by handle, so undeploy resolves
// the handle to the gateway UUID here to keep the contract consistent.
func (s *MCPDeploymentService) UndeployDeploymentByHandle(proxyHandle, deploymentID, gatewayHandle, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
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
	gatewayUUID := gateway.ID

	return s.undeployMCPProxyDeployment(proxyUUID, &deploymentID, &gatewayUUID, orgUUID)
}

// DeleteDeploymentByHandle deletes a deployment using MCP proxy handle
func (s *MCPDeploymentService) DeleteDeploymentByHandle(proxyHandle, deploymentID, orgUUID string) error {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
	if err != nil {
		return err
	}

	return s.deleteMCPProxyDeployment(proxyUUID, deploymentID, orgUUID)
}

// GetDeploymentByHandle retrieves a single deployment using MCP proxy handle
func (s *MCPDeploymentService) GetDeploymentByHandle(proxyHandle, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.getMCPProxyDeployment(proxyUUID, deploymentID, orgUUID)
}

// GetDeploymentsByHandle retrieves deployments for an MCP proxy using handle
func (s *MCPDeploymentService) GetDeploymentsByHandle(proxyHandle, gatewayID, status, orgUUID string) (*api.DeploymentListResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
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

	return s.getMCPProxyDeployments(proxyUUID, orgUUID, gatewayUUID, statusPtr)
}

// deployMCPProxy deploys an MCP proxy to a gateway
func (s *MCPDeploymentService) deployMCPProxy(proxyUUID string, req *api.DeployRequest, orgId, createdBy string) (*api.DeploymentResponse, error) {
	if req == nil {
		return nil, apperror.MCPProxyDeploymentValidationFailed.New("A request body is required.")
	}
	if req.Base == "" {
		return nil, apperror.MCPProxyDeploymentValidationFailed.New("Base is required.")
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, apperror.MCPProxyDeploymentValidationFailed.New("Gateway ID is required.")
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}
	gatewayID := gateway.ID

	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	// DP-originated artifacts are read-only in the control plane and cannot be
	// (re)deployed from the CP.
	if err := ensureOriginMutable(mcpProxy.Origin); err != nil {
		return nil, err
	}

	// Validate the request-supplied metadata (e.g. endpointUrl) before it is used to seed
	// a new association's metadata, so an invalid override is rejected up-front rather
	// than persisted onto the association.
	if _, err := extractEndpointURLOverride(metadata); err != nil {
		return nil, err
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
	effectiveMetaJSON, err := s.mcpRepo.EnsureGatewayAssociation(mcpProxy.UUID, gatewayID, orgId, deployMetaJSON, metadataProvided)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure gateway association: %w", err)
	}
	if metadata, err = unmarshalDeploymentMetadata(effectiveMetaJSON); err != nil {
		return nil, err
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	// Collect endpoint URL override from the effective metadata so a value stored on the
	// association (when the request omits metadata) is honored, not just request values.
	endpointURL, err := extractEndpointURLOverride(metadata)
	if err != nil {
		return nil, err
	}

	var baseDeploymentID *string
	var contentBytes []byte

	if req.Base == "current" {
		// Build struct directly, apply overrides on struct, marshal once
		d, err := s.utils.BuildMCPDeploymentYAML(mcpProxy)
		if err != nil {
			return nil, fmt.Errorf("failed to build MCP deployment YAML: %w", err)
		}
		if endpointURL != nil {
			d.Spec.Upstream.URL = *endpointURL
			s.slogger.Debug("Endpoint URL overridden", "endpointURL", *endpointURL, "deploymentID", deploymentID)
		}
		contentBytes, err = yaml.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal MCP deployment YAML: %w", err)
		}
	} else {
		// Use existing deployment as base
		baseDeployment, err := s.deploymentRepo.GetWithContent(req.Base, proxyUUID, orgId)
		if err != nil {
			if apperror.DeploymentNotFound.Is(err) {
				return nil, apperror.DeploymentBaseNotFound.Wrap(err)
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base

		if endpointURL != nil {
			// Unmarshal into the correct MCP type, apply override, marshal back
			var mcpDeployment model.MCPProxyDeploymentYAML
			if err := yaml.Unmarshal(contentBytes, &mcpDeployment); err != nil {
				return nil, fmt.Errorf("failed to parse MCP deployment YAML: %w", err)
			}
			mcpDeployment.Spec.Upstream.URL = *endpointURL
			contentBytes, err = yaml.Marshal(&mcpDeployment)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal modified MCP deployment YAML: %w", err)
			}
			s.slogger.Debug("Endpoint URL overridden", "endpointURL", *endpointURL, "deploymentID", deploymentID)
		}
	}

	// Create new deployment record with limit enforcement
	// Hard limit = soft limit (configured) + 5 buffer for concurrent deployments
	deployment := &model.Deployment{
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ArtifactID:       proxyUUID,
		OrganizationID:   orgId,
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

	// Set initial status based on config; transitional (DEPLOYING) only when enabled
	initialStatus := model.DeploymentStatusDeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusDeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		proxyUUID, orgId, gatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		return nil, fmt.Errorf("failed to set deployment status for MCP proxy: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.MCPProxyDeploymentEvent{
			ProxyId:      proxyUUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastMCPProxyDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast MCP proxy deployment event", "error", err)
		}
	}

	// Return deployment response
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

// extractEndpointURLOverride reads and validates the optional "endpointUrl" override from
// a deployment metadata map. It returns nil (no override) when the key is absent or empty,
// and ErrInvalidInput when the value is not a string or is not a valid endpoint URL.
func extractEndpointURLOverride(metadata map[string]interface{}) (*string, error) {
	v, exists := metadata["endpointUrl"]
	if !exists {
		return nil, nil
	}
	eu, ok := v.(string)
	if !ok {
		return nil, apperror.MCPProxyDeploymentValidationFailed.New(
			fmt.Sprintf("The endpointUrl in metadata must be a string, got %T.", v))
	}
	if eu == "" {
		return nil, nil
	}
	if err := validateEndpointURL(eu); err != nil {
		return nil, apperror.MCPProxyDeploymentValidationFailed.Wrap(err,
			"The endpointUrl in metadata is not a valid URL.")
	}
	return &eu, nil
}

// UndeployMCPProxyDeployment undeploys an MCP proxy from a gateway
func (s *MCPDeploymentService) undeployMCPProxyDeployment(proxyUUID string, deploymentId *string, gatewayId *string, orgId string) (*api.DeploymentResponse, error) {
	// DP-originated artifacts are read-only in the control plane; their deployment
	// lifecycle is owned by the data-plane gateway, so undeploy cannot be CP-initiated.
	if err := ensureArtifactMutableByUUID(s.artifactRepo, proxyUUID, orgId); err != nil {
		return nil, err
	}

	// Verify MCP proxy exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	// Resolve deployment using provided deploymentId or gatewayId
	var deployment *model.Deployment
	if deploymentId != nil {
		// Get specific deployment
		deployment, err = s.deploymentRepo.GetWithState(*deploymentId, proxyUUID, orgId)
		if err != nil {
			return nil, err
		}
		if deployment == nil {
			return nil, apperror.DeploymentNotFound.New()
		}
	} else if gatewayId != nil {
		// Find current deployment for this gateway
		deployment, err = s.deploymentRepo.GetCurrentByGateway(proxyUUID, *gatewayId, orgId)
		if err != nil {
			return nil, err
		}
		if deployment == nil {
			return nil, apperror.DeploymentNotFound.New()
		}
	} else {
		return nil, apperror.MCPProxyDeploymentValidationFailed.New(
			"Either a deploymentId or a gatewayId is required.")
	}

	// Validate that the provided gatewayId matches the deployment's bound gateway
	if gatewayId != nil && deployment.GatewayID != *gatewayId {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}

	// Verify deployment is currently DEPLOYED (status already populated by GetWithState)
	if deployment.Status == nil || *deployment.Status != model.DeploymentStatusDeployed {
		return nil, apperror.DeploymentNotActive.New("MCP proxy")
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
		proxyUUID, orgId, deployment.GatewayID, deployment.DeploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send undeployment event to gateway
	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.MCPProxyUndeploymentEvent{
			ProxyId:      proxyUUID,
			DeploymentID: deployment.DeploymentID,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastMCPProxyUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast MCP proxy undeployment event", "error", err)
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

// restoreMCPProxyDeployment restores a previously undeployed MCP proxy deployment
func (s *MCPDeploymentService) restoreMCPProxyDeployment(proxyUUID string, deploymentId *string, gatewayId *string, orgId string) (*api.DeploymentResponse, error) {
	// DP-originated artifacts are read-only in the control plane; their deployment
	// lifecycle is owned by the data-plane gateway, so restore cannot be CP-initiated.
	if err := ensureArtifactMutableByUUID(s.artifactRepo, proxyUUID, orgId); err != nil {
		return nil, err
	}

	// Verify target deployment exists and belongs to the API
	targetDeployment, err := s.deploymentRepo.GetWithContent(*deploymentId, proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	// Validate that the provided gatewayID matches the deployment's bound gateway
	if targetDeployment.GatewayID != *gatewayId {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}

	// Verify target deployment is NOT currently DEPLOYED
	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(proxyUUID, orgId, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == *deploymentId && status == model.DeploymentStatusDeployed {
		return nil, apperror.DeploymentRestoreConflict.New()
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgId {
		return nil, apperror.GatewayNotFound.New()
	}

	// Set initial status based on config; transitional (DEPLOYING) only when enabled
	initialStatus := model.DeploymentStatusDeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusDeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		proxyUUID, orgId, targetDeployment.GatewayID, *deploymentId,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.MCPProxyDeploymentEvent{
			ProxyId:      proxyUUID,
			DeploymentID: *deploymentId,
			PerformedAt:  performedAt,
		}

		if err := s.gatewayEventsService.BroadcastMCPProxyDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast MCP proxy deployment event", "error", err)
		}
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

// getMCPProxyDeployment retrieves a specific MCP proxy deployment
func (s *MCPDeploymentService) getMCPProxyDeployment(proxyUUID string, deploymentId string, orgId string) (*api.DeploymentResponse, error) {
	// Verify API exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	// Get deployment with state derived via LEFT JOIN
	deployment, err := s.deploymentRepo.GetWithState(deploymentId, proxyUUID, orgId)
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

// getMCPProxyDeployments retrieves all deployments for an MCP proxy
func (s *MCPDeploymentService) getMCPProxyDeployments(proxyUUID string, orgId string, gatewayId *string, status *string) (*api.DeploymentListResponse, error) {
	// Verify MCP proxy exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	// Validate status parameter
	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):    true,
			string(model.DeploymentStatusUndeployed):  true,
			string(model.DeploymentStatusArchived):    true,
			string(model.DeploymentStatusDeploying):   true,
			string(model.DeploymentStatusUndeploying): true,
			string(model.DeploymentStatusFailed):      true,
		}
		if !validStatuses[*status] {
			return nil, apperror.DeploymentInvalidStatus.New()
		}
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}

	// Get deployments with state derived via LEFT JOIN
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(proxyUUID, orgId, gatewayId, status, s.cfg.Deployments.MaxPerAPIGateway)
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

// deleteMCPProxyDeployment deletes an MCP proxy deployment
func (s *MCPDeploymentService) deleteMCPProxyDeployment(proxyUUID string, deploymentId string, orgId string) error {
	// Verify MCP proxy exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return err
	}
	if mcpProxy == nil {
		return apperror.MCPProxyNotFound.New()
	}

	// Verify deployment exists and belongs to the MCP proxy
	deployment, err := s.deploymentRepo.GetWithState(deploymentId, proxyUUID, orgId)
	if err != nil {
		return err
	}
	if deployment == nil {
		return apperror.DeploymentNotFound.New()
	}

	// Verify deployment is NOT currently DEPLOYED (status already populated by GetWithState)
	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
		return apperror.DeploymentActive.New()
	}

	// Delete the deployment artifact
	if err := s.deploymentRepo.Delete(deploymentId, proxyUUID, orgId); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// getMCPProxyUUIDByHandle retrieves the artifact UUID by its handle from the artifact table
func (s *MCPDeploymentService) getMCPProxyUUIDByHandle(handle, orgUUID string) (string, error) {
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
