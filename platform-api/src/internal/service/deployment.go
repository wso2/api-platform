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
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
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
		return nil, errors.New("base is required")
	}
	if req.GatewayID == "" {
		return nil, errors.New("gatewayId is required")
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

	// Validate API has backend services attached (do this early before deployment limits)
	backendServices, err := s.backendServiceRepo.GetBackendServicesByAPIID(apiUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend services: %w", err)
	}
	if len(backendServices) == 0 {
		return nil, errors.New("API must have at least one backend service attached before deployment")
	}

	// Check if there's an existing active deployment on this gateway
	existingDeployment, err := s.apiRepo.GetActiveDeploymentByGateway(apiUUID, req.GatewayID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing deployments: %w", err)
	}

	// If exists, mark it as UNDEPLOYED (but don't send undeployment event yet)
	// TODO:// The gateway will receive the new deployment event and handle the transition
	if existingDeployment != nil {
		if err := s.apiRepo.UpdateDeploymentStatus(existingDeployment.DeploymentID, apiUUID, string(model.DeploymentStatusUndeployed), orgUUID); err != nil {
			return nil, fmt.Errorf("failed to undeploy existing deployment %s: %w", existingDeployment.DeploymentID, err)
		}
	}
	// TODO:// Transaction handling for deployment creation and existing deployment update
	// Check deployment limits
	apiDeploymentCount, err := s.apiRepo.CountDeploymentsByAPIAndGateway(apiUUID, req.GatewayID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check deployment count: %w", err)
	}
	if apiDeploymentCount >= s.cfg.Deployments.MaxPerAPIGateway {
		// Delete oldest deployment in UNDEPLOYED state to make room
		oldestDeployment, err := s.apiRepo.GetOldestUndeployedDeploymentByGateway(apiUUID, req.GatewayID, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get oldest undeployed deployment: %w", err)
		}
		if oldestDeployment != nil {
			if err := s.apiRepo.DeleteDeployment(oldestDeployment.DeploymentID, apiUUID, orgUUID); err != nil {
				return nil, fmt.Errorf("failed to delete oldest undeployed deployment: %w", err)
			}
			log.Printf("[INFO] Deleted oldest undeployed deployment %s to make room for new deployment", oldestDeployment.DeploymentID)
		}
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
		baseDeployment, err := s.apiRepo.GetDeploymentByID(req.Base, apiUUID, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		if baseDeployment == nil {
			return nil, constants.ErrDeploymentNotFound
		}

		// Deployment content is already stored as YAML, reuse it directly
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base
	}

	// Generate deployment ID
	deploymentID := uuid.New().String()

	// Create new deployment record
	deployment := &model.APIDeployment{
		DeploymentID:     deploymentID,
		ApiID:            apiUUID,
		OrganizationID:   orgUUID,
		GatewayID:        req.GatewayID,
		Status:           model.DeploymentStatusDeployed,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         req.Metadata,
		CreatedAt:        time.Now(),
	}

	if err := s.apiRepo.CreateDeployment(deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Ensure API-Gateway association exists
	// TODO:// Handle error properly (maybe rollback deployment?)
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

	// Return deployment response
	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		GatewayID:        deployment.GatewayID,
		Status:           string(deployment.Status),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
	}, nil
}

// RedeployDeployment re-deploys an existing undeployed deployment artifact
func (s *DeploymentService) RedeployDeployment(apiUUID, deploymentID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Get the deployment
	deployment, err := s.apiRepo.GetDeploymentByID(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}
	if deployment.Status == model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentAlreadyActive
	}

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}

	// Check if there's an existing active deployment on this gateway
	existingDeployment, err := s.apiRepo.GetActiveDeploymentByGateway(apiUUID, deployment.GatewayID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing deployments: %w", err)
	}

	// If exists, mark it as UNDEPLOYED (but don't send undeployment event yet)
	// TODO:// The gateway will receive the new deployment event and update the deployment
	if existingDeployment != nil {
		if err := s.apiRepo.UpdateDeploymentStatus(existingDeployment.DeploymentID, apiUUID, string(model.DeploymentStatusUndeployed), orgUUID); err != nil {
			return nil, fmt.Errorf("failed to undeploy existing deployment %s: %w", existingDeployment.DeploymentID, err)

		}
	}

	// Update status to DEPLOYED
	if err := s.apiRepo.UpdateDeploymentStatus(deploymentID, apiUUID, string(model.DeploymentStatusDeployed), orgUUID); err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := gateway.Vhost
		deploymentEvent := &model.APIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			Vhost:        vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastDeploymentEvent(deployment.GatewayID, deploymentEvent); err != nil {
			log.Printf("[WARN] Failed to broadcast deployment event: %v", err)
		}
	}

	deployment.Status = model.DeploymentStatusDeployed

	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		GatewayID:        deployment.GatewayID,
		Status:           string(deployment.Status),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
	}, nil
}

// UndeployDeployment undeploys an active deployment
func (s *DeploymentService) UndeployDeployment(apiUUID, deploymentID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Get the deployment
	deployment, err := s.apiRepo.GetDeploymentByID(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}
	if deployment.Status != model.DeploymentStatusDeployed {
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

	// Update status to UNDEPLOYED
	if err := s.apiRepo.UpdateDeploymentStatus(deploymentID, apiUUID, string(model.DeploymentStatusUndeployed), orgUUID); err != nil {
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

	deployment.Status = model.DeploymentStatusUndeployed

	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		GatewayID:        deployment.GatewayID,
		Status:           string(deployment.Status),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
	}, nil
}

// DeleteDeployment permanently deletes an undeployed deployment artifact
func (s *DeploymentService) DeleteDeployment(apiUUID, deploymentID, orgUUID string) error {
	// Get the deployment
	deployment, err := s.apiRepo.GetDeploymentByID(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return constants.ErrDeploymentNotFound
	}
	if deployment.Status == model.DeploymentStatusDeployed {
		return constants.ErrDeploymentIsDeployed
	}

	// Delete the deployment
	if err := s.apiRepo.DeleteDeployment(deploymentID, apiUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
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

	// Get deployments
	deployments, err := s.apiRepo.GetDeploymentsByAPIUUID(apiUUID, orgUUID, gatewayID, status)
	if err != nil {
		return nil, err
	}

	// Convert to DTOs
	var deploymentDTOs = []*dto.DeploymentResponse{}
	for _, d := range deployments {
		deploymentDTOs = append(deploymentDTOs, &dto.DeploymentResponse{
			DeploymentID:     d.DeploymentID,
			GatewayID:        d.GatewayID,
			Status:           string(d.Status),
			BaseDeploymentID: d.BaseDeploymentID,
			Metadata:         d.Metadata,
			CreatedAt:        d.CreatedAt,
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

	// Get deployment - apiId is validated at repository level
	deployment, err := s.apiRepo.GetDeploymentByID(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Convert to DTO
	return &dto.DeploymentResponse{
		DeploymentID:     deployment.DeploymentID,
		GatewayID:        deployment.GatewayID,
		Status:           string(deployment.Status),
		BaseDeploymentID: deployment.BaseDeploymentID,
		Metadata:         deployment.Metadata,
		CreatedAt:        deployment.CreatedAt,
	}, nil
}

// GetDeploymentContent retrieves the immutable content of a deployment
func (s *DeploymentService) GetDeploymentContent(apiUUID, deploymentID, orgUUID string) ([]byte, error) {
	// Verify deployment exists and belongs to the API
	deployment, err := s.apiRepo.GetDeploymentByID(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Get content - apiId is validated at repository level
	content, err := s.apiRepo.GetDeploymentContent(deploymentID, apiUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment content: %w", err)
	}

	return content, nil
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

// RedeployDeploymentByHandle redeploys an existing deployment using API handle
func (s *DeploymentService) RedeployDeploymentByHandle(apiHandle, deploymentID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.RedeployDeployment(apiUUID, deploymentID, orgUUID)
}

// UndeployDeploymentByHandle undeploys a deployment using API handle
func (s *DeploymentService) UndeployDeploymentByHandle(apiHandle, deploymentID, orgUUID string) (*dto.DeploymentResponse, error) {
	// Convert API handle to UUID
	apiUUID, err := s.getAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	return s.UndeployDeployment(apiUUID, deploymentID, orgUUID)
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
