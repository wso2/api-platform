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
	"errors"
	"fmt"
	"log/slog"
	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
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
func (s *MCPDeploymentService) DeployMCPProxyByHandle(proxyHandle string, req *api.DeployRequest, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.deployMCPProxy(proxyUUID, req, orgUUID)
}

// RestoreMCPDeploymentByHandle restores a previous deployment using MCP proxy handle
func (s *MCPDeploymentService) RestoreMCPDeploymentByHandle(proxyHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.restoreMCPProxyDeployment(proxyUUID, &deploymentID, &gatewayID, orgUUID)
}

// UndeployDeploymentByHandle undeploys a deployment using MCP proxy handle
func (s *MCPDeploymentService) UndeployDeploymentByHandle(proxyHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	// Convert MCP proxy handle to UUID
	proxyUUID, err := s.getMCPProxyUUIDByHandle(proxyHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.undeployMCPProxyDeployment(proxyUUID, &deploymentID, &gatewayID, orgUUID)
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
	var gatewayIdPtr *string
	var statusPtr *string
	if gatewayID != "" {
		gatewayIdPtr = &gatewayID
	}
	if status != "" {
		statusPtr = &status
	}

	return s.getMCPProxyDeployments(proxyUUID, orgUUID, gatewayIdPtr, statusPtr)
}

// deployMCPProxy deploys an MCP proxy to a gateway
func (s *MCPDeploymentService) deployMCPProxy(proxyUUID string, req *api.DeployRequest, orgId string) (*api.DeploymentResponse, error) {
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
	if gateway == nil || gateway.OrganizationID != orgId {
		return nil, constants.ErrGatewayNotFound
	}

	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, constants.ErrMCPProxyNotFound
	}

	var baseDeploymentID *string
	var contentBytes []byte

	// Determine the source: "current" or existing deployment
	if req.Base == "current" {
		// Generate MCP deployment YAML for storage using the model directly
		mcpYaml, err := s.utils.GenerateMCPDeploymentYAML(mcpProxy)
		if err != nil {
			return nil, fmt.Errorf("failed to generate MCP deployment YAML: %w", err)
		}

		// Create immutable deployment artifact content (store as YAML bytes)
		contentBytes = []byte(mcpYaml)
	} else {
		// Use existing deployment as base
		baseDeployment, err := s.deploymentRepo.GetWithContent(req.Base, proxyUUID, orgId)
		if err != nil {
			if errors.Is(err, constants.ErrDeploymentNotFound) {
				return nil, constants.ErrBaseDeploymentNotFound
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}

		// Deployment content is already stored as YAML, reuse it directly
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	// Handle endpoint URL override from metadata (Phase 5)
	if req.Metadata != nil {
		if v, exists := metadata["endpointUrl"]; exists {
			endpointURL, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%w: invalid endpoint URL in metadata: expected string, got %T", constants.ErrInvalidInput, v)
			}
			if endpointURL != "" {
				// Validate endpoint URL format
				if err := validateEndpointURL(endpointURL); err != nil {
					return nil, fmt.Errorf("%w: invalid endpoint URL in metadata: %v", constants.ErrInvalidInput, err)
				}

				// Override endpoint URL in deployment content
				modifiedContent, err := overrideEndpointURL(contentBytes, endpointURL)
				if err != nil {
					return nil, fmt.Errorf("failed to override endpoint URL: %w", err)
				}
				contentBytes = modifiedContent
				s.slogger.Info("Endpoint URL overridden", "endpointURL", endpointURL, "deploymentID", deploymentID)
			}
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
	}

	// Use CreateDeploymentWithLimitEnforcement - handles count, cleanup, insert, and status update atomically
	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.MCPProxyDeploymentEvent{
			ProxyId:      proxyUUID,
			DeploymentID: deploymentID,
			Vhost:        gateway.Vhost,
		}

		if err := s.gatewayEventsService.BroadcastMCPProxyDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast MCP proxy deployment event", "error", err)
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

// UndeployMCPProxyDeployment undeploys an MCP proxy from a gateway
func (s *MCPDeploymentService) undeployMCPProxyDeployment(proxyUUID string, deploymentId *string, gatewayId *string, orgId string) (*api.DeploymentResponse, error) {
	// Verify MCP proxy exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, constants.ErrMCPProxyNotFound
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
			return nil, constants.ErrDeploymentNotFound
		}
	} else if gatewayId != nil {
		// Find current deployment for this gateway
		deployment, err = s.deploymentRepo.GetCurrentByGateway(proxyUUID, *gatewayId, orgId)
		if err != nil {
			return nil, err
		}
		if deployment == nil {
			return nil, constants.ErrDeploymentNotFound
		}
	} else {
		return nil, constants.ErrInvalidInput
	}

	// Validate that the provided gatewayId matches the deployment's bound gateway
	if gatewayId != nil && deployment.GatewayID != *gatewayId {
		return nil, constants.ErrGatewayIDMismatch
	}

	// Verify deployment is currently DEPLOYED (status already populated by GetWithState)
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

	// Update status to UNDEPLOYED using SetCurrent
	newUpdatedAt, err := s.deploymentRepo.SetCurrent(proxyUUID, orgId, deployment.GatewayID, deployment.DeploymentID, model.DeploymentStatusUndeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send undeployment event to gateway
	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.MCPProxyUndeploymentEvent{
			ProxyId: proxyUUID,
			Vhost:   gateway.Vhost,
		}

		if err := s.gatewayEventsService.BroadcastMCPProxyUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast MCP proxy undeployment event", "error", err)
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

// restoreMCPProxyDeployment restores a previously undeployed MCP proxy deployment
func (s *MCPDeploymentService) restoreMCPProxyDeployment(proxyUUID string, deploymentId *string, gatewayId *string, orgId string) (*api.DeploymentResponse, error) {
	// Verify target deployment exists and belongs to the API
	targetDeployment, err := s.deploymentRepo.GetWithContent(*deploymentId, proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Validate that the provided gatewayID matches the deployment's bound gateway
	if targetDeployment.GatewayID != *gatewayId {
		return nil, constants.ErrGatewayIDMismatch
	}

	// Verify target deployment is NOT currently DEPLOYED
	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(proxyUUID, orgId, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == *deploymentId && status == model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentAlreadyDeployed
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgId {
		return nil, constants.ErrGatewayNotFound
	}

	// Use SetCurrentDeployment to activate the target deployment with status='DEPLOYED'
	updatedAt, err := s.deploymentRepo.SetCurrent(proxyUUID, orgId, targetDeployment.GatewayID, *deploymentId, model.DeploymentStatusDeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.MCPProxyDeploymentEvent{
			ProxyId:      proxyUUID,
			DeploymentID: *deploymentId,
			Vhost:        gateway.Vhost,
		}

		if err := s.gatewayEventsService.BroadcastMCPProxyDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast MCP proxy deployment event", "error", err)
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

// getMCPProxyDeployment retrieves a specific MCP proxy deployment
func (s *MCPDeploymentService) getMCPProxyDeployment(proxyUUID string, deploymentId string, orgId string) (*api.DeploymentResponse, error) {
	// Verify API exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, constants.ErrMCPProxyNotFound
	}

	// Get deployment with state derived via LEFT JOIN
	deployment, err := s.deploymentRepo.GetWithState(deploymentId, proxyUUID, orgId)
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

// getMCPProxyDeployments retrieves all deployments for an MCP proxy
func (s *MCPDeploymentService) getMCPProxyDeployments(proxyUUID string, orgId string, gatewayId *string, status *string) (*api.DeploymentListResponse, error) {
	// Verify MCP proxy exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return nil, err
	}
	if mcpProxy == nil {
		return nil, constants.ErrMCPProxyNotFound
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
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(proxyUUID, orgId, gatewayId, status, s.cfg.Deployments.MaxPerAPIGateway)
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

// deleteMCPProxyDeployment deletes an MCP proxy deployment
func (s *MCPDeploymentService) deleteMCPProxyDeployment(proxyUUID string, deploymentId string, orgId string) error {
	// Verify MCP proxy exists
	mcpProxy, err := s.mcpRepo.GetByUUID(proxyUUID, orgId)
	if err != nil {
		return err
	}
	if mcpProxy == nil {
		return constants.ErrMCPProxyNotFound
	}

	// Verify deployment exists and belongs to the MCP proxy
	deployment, err := s.deploymentRepo.GetWithState(deploymentId, proxyUUID, orgId)
	if err != nil {
		return err
	}
	if deployment == nil {
		return constants.ErrDeploymentNotFound
	}

	// Verify deployment is NOT currently DEPLOYED (status already populated by GetWithState)
	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
		return constants.ErrDeploymentIsDeployed
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
