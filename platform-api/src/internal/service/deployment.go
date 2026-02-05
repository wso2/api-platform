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
	"log"
	"net/url"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// DeploymentService handles business logic for API deployment operations
type DeploymentService struct {
	apiRepo              repository.APIRepository
	gatewayRepo          repository.GatewayRepository
	backendServiceRepo   repository.BackendServiceRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *GatewayEventsService
	apiUtil              *utils.APIUtil
	cfg                  *config.Server
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(
	apiRepo repository.APIRepository,
	gatewayRepo repository.GatewayRepository,
	backendServiceRepo repository.BackendServiceRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEventsService *GatewayEventsService,
	apiUtil *utils.APIUtil,
	cfg *config.Server,
) *DeploymentService {
	return &DeploymentService{
		apiRepo:              apiRepo,
		gatewayRepo:          gatewayRepo,
		backendServiceRepo:   backendServiceRepo,
		orgRepo:              orgRepo,
		gatewayEventsService: gatewayEventsService,
		apiUtil:              apiUtil,
		cfg:                  cfg,
	}
}

// DeployAPI creates a new immutable deployment artifact and deploys it to a gateway
func (s *DeploymentService) DeployAPI(apiUUID string, req *dto.DeployAPIRequest, orgUUID string) (*dto.DeploymentResponse, error) {
	// Validate request
	if req.Base == "" {
		return nil, constants.ErrDeploymentBaseRequired
	}
	if req.GatewayID == "" {
		return nil, constants.ErrDeploymentGatewayIDRequired
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(req.GatewayID)
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

	// Validate API has backend services attached (do this early before deployment limits)
	backendServices, err := s.backendServiceRepo.GetBackendServicesByAPIID(apiUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend services: %w", err)
	}
	if len(backendServices) == 0 {
		return nil, constants.ErrAPINoBackendServices
	}

	var baseDeploymentID *string
	var apiContent *dto.API
	var contentBytes []byte

	// Determine the source: "current" or existing deployment
	if req.Base == "current" {
		// Use current API state
		apiContent = s.apiUtil.ModelToDTO(apiModel)

		// Generate API deployment YAML for storage
		apiYaml, err := s.apiUtil.GenerateAPIDeploymentYAML(apiContent)
		if err != nil {
			return nil, fmt.Errorf("failed to generate API deployment YAML: %w", err)
		}

		// Create immutable deployment artifact content (store as YAML bytes)
		contentBytes = []byte(apiYaml)
	} else {
		// Use existing deployment as base
		baseDeployment, err := s.apiRepo.GetDeploymentWithContent(req.Base, apiUUID, orgUUID)
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
	deploymentID := uuid.New().String()

	// Handle endpoint URL override from metadata (Phase 5)
	if req.Metadata != nil {
		if v, exists := req.Metadata["endpointUrl"]; exists {
			endpointURL, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("invalid endpoint URL in metadata: expected string, got %T", v)
			}
			if endpointURL != "" {
				// Validate endpoint URL format
				if err := validateEndpointURL(endpointURL); err != nil {
					return nil, fmt.Errorf("invalid endpoint URL in metadata: %w", err)
				}

				// Override endpoint URL in deployment content
				modifiedContent, err := overrideEndpointURL(contentBytes, endpointURL)
				if err != nil {
					return nil, fmt.Errorf("failed to override endpoint URL: %w", err)
				}
				contentBytes = modifiedContent
				log.Printf("[INFO] Endpoint URL overridden to: %s for deployment %s", endpointURL, deploymentID)
			}
		}
	}

	// Create new deployment record with limit enforcement
	// Hard limit = soft limit (configured) + 5 buffer for concurrent deployments
	deployment := &model.APIDeployment{
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ApiID:            apiUUID,
		OrganizationID:   orgUUID,
		GatewayID:        req.GatewayID,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         req.Metadata,
	}

	// Use CreateDeploymentWithLimitEnforcement - handles count, cleanup, insert, and status update atomically
	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.apiRepo.CreateDeploymentWithLimitEnforcement(deployment, hardLimit); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Ensure API-Gateway association exists
	if err := s.ensureAPIGatewayAssociation(apiUUID, req.GatewayID, orgUUID); err != nil {
		log.Printf("[WARN] Failed to ensure API-gateway association: %v", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.APIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			Vhost:        gateway.Vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(req.GatewayID, deploymentEvent); err != nil {
			log.Printf("[WARN] Failed to broadcast deployment event: %v", err)
		}
	}

	// Return deployment response (status and updatedAt are set by CreateDeploymentWithLimitEnforcement)
	deployedStatus := model.DeploymentStatusDeployed
	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		Name:             deployment.Name,
		GatewayID:        deployment.GatewayID,
		Status:           string(deployedStatus),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
		UpdatedAt:        deployment.UpdatedAt,
	}, nil
}

// RestoreDeployment restores a previous deployment (can be ARCHIVED or UNDEPLOYED)
func (s *DeploymentService) RestoreDeployment(apiUUID, deploymentID, gatewayID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Verify target deployment exists and belongs to the API
	targetDeployment, err := s.apiRepo.GetDeploymentWithContent(deploymentID, apiUUID, orgUUID)
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
	currentDeploymentID, status, _, err := s.apiRepo.GetDeploymentStatus(apiUUID, orgUUID, targetDeployment.GatewayID)
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
	updatedAt, err := s.apiRepo.SetCurrentDeployment(apiUUID, orgUUID, targetDeployment.GatewayID, deploymentID, model.DeploymentStatusDeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.APIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			Vhost:        gateway.Vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			log.Printf("[WARN] Failed to broadcast deployment event: %v", err)
		}
	}

	deployedStatus := model.DeploymentStatusDeployed
	return &dto.DeploymentResponse{
		DeploymentID:     targetDeployment.DeploymentID,
		Name:             targetDeployment.Name,
		GatewayID:        targetDeployment.GatewayID,
		Status:           string(deployedStatus),
		BaseDeploymentID: targetDeployment.BaseDeploymentID,
		Metadata:         targetDeployment.Metadata,
		CreatedAt:        targetDeployment.CreatedAt,
		UpdatedAt:        &updatedAt,
	}, nil
}

// UndeployDeployment undeploys an active deployment
func (s *DeploymentService) UndeployDeployment(apiUUID, deploymentID, gatewayID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Verify deployment exists and belongs to API
	deployment, err := s.apiRepo.GetDeploymentWithState(deploymentID, apiUUID, orgUUID)
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
	newUpdatedAt, err := s.apiRepo.SetCurrentDeployment(apiUUID, orgUUID, deployment.GatewayID, deploymentID, model.DeploymentStatusUndeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send undeployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := gateway.Vhost
		undeploymentEvent := &model.APIUndeploymentEvent{
			ApiId:       apiUUID,
			Vhost:       vhost,
			Environment: "production",
		}

		if err := s.gatewayEventsService.BroadcastUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			log.Printf("[WARN] Failed to broadcast undeployment event: %v", err)
		}
	}

	undeployedStatus := model.DeploymentStatusUndeployed
	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		Name:             deployment.Name,
		GatewayID:        deployment.GatewayID,
		Status:           string(undeployedStatus),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
		UpdatedAt:        &newUpdatedAt,
	}, nil
}

// DeleteDeployment permanently deletes an undeployed deployment artifact
func (s *DeploymentService) DeleteDeployment(apiUUID, deploymentID, orgUUID string) error {
	// Verify deployment exists and belongs to the API
	deployment, err := s.apiRepo.GetDeploymentWithState(deploymentID, apiUUID, orgUUID)
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
	if err := s.apiRepo.DeleteDeployment(deploymentID, apiUUID, orgUUID); err != nil {
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

// overrideEndpointURL parses the deployment YAML, overrides the upstream URL, and returns modified bytes
func overrideEndpointURL(contentBytes []byte, newURL string) ([]byte, error) {
	var apiDeployment dto.APIDeploymentYAML

	// Parse existing YAML
	if err := yaml.Unmarshal(contentBytes, &apiDeployment); err != nil {
		return nil, fmt.Errorf("failed to parse deployment YAML: %w", err)
	}

	// Ensure upstream section exists
	if apiDeployment.Spec.Upstream == nil {
		apiDeployment.Spec.Upstream = &dto.UpstreamYAML{}
	}

	// Override main upstream URL (production endpoint)
	if apiDeployment.Spec.Upstream.Main == nil {
		apiDeployment.Spec.Upstream.Main = &dto.UpstreamTarget{}
	}
	apiDeployment.Spec.Upstream.Main.URL = newURL
	apiDeployment.Spec.Upstream.Main.Ref = "" // Clear ref if URL is set

	// Serialize back to YAML
	modifiedBytes, err := yaml.Marshal(&apiDeployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified YAML: %w", err)
	}

	return modifiedBytes, nil
}

// GetDeployments retrieves all deployments for an API with optional filters
func (s *DeploymentService) GetDeployments(apiUUID, orgUUID string, gatewayID *string, status *string) (*dto.DeploymentListResponse, error) {
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
	deployments, err := s.apiRepo.GetDeploymentsWithState(apiUUID, orgUUID, gatewayID, status, s.cfg.Deployments.MaxPerAPIGateway)
	if err != nil {
		return nil, err
	}

	// Convert to DTOs
	var deploymentDTOs = []*dto.DeploymentResponse{}
	for _, d := range deployments {
		// Status is guaranteed non-nil by repository (set to ARCHIVED if no status row exists)
		deploymentDTOs = append(deploymentDTOs, &dto.DeploymentResponse{
			DeploymentID:     d.DeploymentID,
			Name:             d.Name,
			GatewayID:        d.GatewayID,
			Status:           string(*d.Status),
			BaseDeploymentID: d.BaseDeploymentID,
			Metadata:         d.Metadata,
			CreatedAt:        d.CreatedAt,
			UpdatedAt:        d.UpdatedAt,
		})
	}

	return &dto.DeploymentListResponse{
		Count: len(deploymentDTOs),
		List:  deploymentDTOs,
	}, nil
}

// GetDeployment retrieves a specific deployment by ID
func (s *DeploymentService) GetDeployment(apiUUID, deploymentID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Verify API exists
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}

	// Get deployment with state derived via LEFT JOIN
	deployment, err := s.apiRepo.GetDeploymentWithState(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Convert to DTO
	// Status is guaranteed non-nil by repository (set to ARCHIVED if no status row exists)
	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		Name:             deployment.Name,
		GatewayID:        deployment.GatewayID,
		Status:           string(*deployment.Status),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
		UpdatedAt:        deployment.UpdatedAt,
	}, nil
}

// GetDeploymentContent retrieves the immutable content of a deployment
func (s *DeploymentService) GetDeploymentContent(apiUUID, deploymentID, orgUUID string) ([]byte, error) {
	// Get deployment with content
	deployment, err := s.apiRepo.GetDeploymentWithContent(deploymentID, apiUUID, orgUUID)
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
		ApiID:           apiUUID,
		OrganizationID:  orgUUID,
		ResourceID:      gatewayID,
		AssociationType: constants.AssociationTypeGateway,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	return s.apiRepo.CreateAPIAssociation(association)
}

// DeployAPIByHandle creates a new immutable deployment artifact using API handle
func (s *DeploymentService) DeployAPIByHandle(apiHandle string, req *dto.DeployAPIRequest, orgUUID string) (*dto.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.DeployAPI(apiUUID, req, orgUUID)
}

// RestoreDeploymentByHandle restores a previous deployment using API handle
func (s *DeploymentService) RestoreDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.RestoreDeployment(apiUUID, deploymentID, gatewayID, orgUUID)
}

// getAPIUUIDByHandle retrieves the internal UUID for an API by its handle
func (s *DeploymentService) getAPIUUIDByHandle(handle, orgUUID string) (string, error) {
	if handle == "" {
		return "", errors.New("API handle is required")
	}

	metadata, err := s.apiRepo.GetAPIMetadataByHandle(handle, orgUUID)
	if err != nil {
		return "", err
	}
	if metadata == nil {
		return "", constants.ErrAPINotFound
	}

	return metadata.ID, nil
}

// GetDeploymentByHandle retrieves a single deployment using API handle
func (s *DeploymentService) GetDeploymentByHandle(apiHandle, deploymentID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.GetDeployment(apiUUID, deploymentID, orgUUID)
}

// GetDeploymentsByHandle retrieves deployments for an API using handle
func (s *DeploymentService) GetDeploymentsByHandle(apiHandle, gatewayID, status, orgUUID string) (*dto.DeploymentListResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
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
func (s *DeploymentService) UndeployDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.UndeployDeployment(apiUUID, deploymentID, gatewayID, orgUUID)
}

// DeleteDeploymentByHandle deletes a deployment using API handle
func (s *DeploymentService) DeleteDeploymentByHandle(apiHandle, deploymentID, orgUUID string) error {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return err
	}

	return s.DeleteDeployment(apiUUID, deploymentID, orgUUID)
}

// GetDeploymentContentByHandle retrieves deployment artifact content using API handle
func (s *DeploymentService) GetDeploymentContentByHandle(apiHandle, deploymentID, orgUUID string) ([]byte, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.GetDeploymentContent(apiUUID, deploymentID, orgUUID)
}
