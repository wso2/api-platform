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
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"time"

	"gopkg.in/yaml.v3"
)

// GatewayInternalAPIService handles internal gateway API operations
type GatewayInternalAPIService struct {
	apiRepo         repository.APIRepository
	gatewayRepo     repository.GatewayRepository
	orgRepo         repository.OrganizationRepository
	projectRepo     repository.ProjectRepository
	upstreamService *UpstreamService
	apiUtil         *utils.APIUtil
	cfg             *config.Server
}

// NewGatewayInternalAPIService creates a new gateway internal API service
func NewGatewayInternalAPIService(apiRepo repository.APIRepository, gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository, projectRepo repository.ProjectRepository,
	upstreamSvc *UpstreamService, cfg *config.Server) *GatewayInternalAPIService {
	return &GatewayInternalAPIService{
		apiRepo:         apiRepo,
		gatewayRepo:     gatewayRepo,
		orgRepo:         orgRepo,
		projectRepo:     projectRepo,
		upstreamService: upstreamSvc,
		apiUtil:         &utils.APIUtil{},
		cfg:             cfg,
	}
}

// GetAPIsByOrganization retrieves all APIs for a specific organization (used by gateways)
func (s *GatewayInternalAPIService) GetAPIsByOrganization(orgID string) (map[string]string, error) {
	// Get all APIs for the organization
	apis, err := s.apiRepo.GetAPIsByOrganizationUUID(orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve APIs: %w", err)
	}

	apiYamlMap := make(map[string]string)
	for _, api := range apis {
		apiDTO := s.apiUtil.ModelToDTO(api)
		apiYaml, err := s.apiUtil.GenerateAPIDeploymentYAML(apiDTO)
		if err != nil {
			return nil, fmt.Errorf("failed to generate API YAML: %w", err)
		}
		apiYamlMap[api.ID] = apiYaml
	}
	return apiYamlMap, nil
}

// GetAPIByUUID retrieves an API by its ID
func (s *GatewayInternalAPIService) GetAPIByUUID(apiId, orgId string) (map[string]string, error) {
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	apiDTO := s.apiUtil.ModelToDTO(apiModel)
	apiYaml, err := s.apiUtil.GenerateAPIDeploymentYAML(apiDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API YAML: %w", err)
	}
	apiYamlMap := map[string]string{
		apiDTO.ID: apiYaml,
	}
	return apiYamlMap, nil
}

// GetActiveDeploymentByGateway retrieves the currently deployed API artifact for a specific gateway
func (s *GatewayInternalAPIService) GetActiveDeploymentByGateway(apiID, orgID, gatewayID string) (map[string]string, error) {
	// Get the active deployment for this API on this gateway
	deployment, err := s.apiRepo.GetCurrentDeploymentByGateway(apiID, gatewayID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotActive
	}

	// Deployment content is already stored as YAML, so return it directly
	apiYaml := string(deployment.Content)

	apiYamlMap := map[string]string{
		apiID: apiYaml,
	}
	return apiYamlMap, nil
}

// CreateGatewayAPIDeployment handles the registration of an API deployment from a gateway
func (s *GatewayInternalAPIService) CreateGatewayAPIDeployment(apiHandle, orgID, gatewayID string,
	notification dto.APIDeploymentNotification, revisionID *string) (*dto.GatewayAPIDeploymentResponse, error) {
	// Note: revisionID parameter is reserved for future use
	_ = revisionID

	// Validate input
	if apiHandle == "" || orgID == "" || gatewayID == "" {
		return nil, constants.ErrInvalidInput
	}

	// Check if the gateway exists and belongs to the organization
	gatewayModel, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gatewayModel == nil {
		return nil, constants.ErrGatewayNotFound
	}
	if gatewayModel.OrganizationID != orgID {
		return nil, constants.ErrGatewayNotFound
	}

	// Find the project by name within the organization
	projectName := notification.ProjectIdentifier
	project, err := s.projectRepo.GetProjectByNameAndOrgID(projectName, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project by name: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectName)
	}
	projectID := project.ID

	// Check if API already exists by getting metadata
	existingAPIMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing API: %w", err)
	}

	apiCreated := false
	apiUUID := ""
	now := time.Now()
	if existingAPIMetadata == nil {
		// Create backend services from upstream configurations
		var backendServiceUUIDs []string
		for _, upstream := range notification.Configuration.Spec.Upstreams {
			backendServiceUUID, err := s.upstreamService.CreateBackendServiceFromUpstream(upstream, orgID)
			if err != nil {
				return nil, fmt.Errorf("failed to create backend service from upstream: %w", err)
			}
			backendServiceUUIDs = append(backendServiceUUIDs, backendServiceUUID)
		}

		// Convert operations from notification to API operations
		operations := make([]model.Operation, 0, len(notification.Configuration.Spec.Operations))
		for _, op := range notification.Configuration.Spec.Operations {
			operation := model.Operation{
				Name:        fmt.Sprintf("%s %s", op.Method, op.Path),
				Description: fmt.Sprintf("Operation for %s %s", op.Method, op.Path),
				Request: &model.OperationRequest{
					Method: op.Method,
					Path:   op.Path,
				},
			}
			operations = append(operations, operation)
		}

		// Create new API from notification (without backend services in the model)

		newAPI := &model.API{
			Handle:           apiHandle, // Use provided apiID as handle
			Name:             notification.Configuration.Spec.Name,
			Context:          notification.Configuration.Spec.Context,
			Version:          notification.Configuration.Spec.Version,
			ProjectID:        projectID,
			OrganizationID:   orgID,
			Provider:         "admin", // Default provider
			LifeCycleStatus:  "CREATED",
			Type:             "HTTP",
			Transport:        []string{"http", "https"},
			IsDefaultVersion: false,
			Operations:       operations,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		err = s.apiRepo.CreateAPI(newAPI)
		if err != nil {
			return nil, fmt.Errorf("failed to create API: %w", err)
		}

		// CreateAPI sets the UUID on the model
		apiUUID = newAPI.ID

		// Associate backend services with the API
		for i, backendServiceUUID := range backendServiceUUIDs {
			isDefault := i == 0 // First backend service is default
			if err := s.upstreamService.AssociateBackendServiceWithAPI(apiUUID, backendServiceUUID, isDefault); err != nil {
				return nil, fmt.Errorf("failed to associate backend service with API: %w", err)
			}
		}

		apiCreated = true
	} else {
		// Validate that existing API belongs to the same organization
		if existingAPIMetadata.OrganizationID != orgID {
			return nil, constants.ErrAPINotFound
		}
		apiUUID = existingAPIMetadata.ID
	}

	// Check if deployment already exists
	existingDeployments, err := s.apiRepo.GetDeploymentsWithState(apiUUID, orgID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing deployments: %w", err)
	}

	// Check if this gateway already has this API deployed
	for _, deployment := range existingDeployments {
		if deployment.GatewayID == gatewayID {
			return nil, fmt.Errorf("API already deployed to this gateway")
		}
	}

	// Check if API-gateway association exists, create if not
	existingAssociations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing API-gateway associations: %w", err)
	}

	// Check if gateway is already associated with the API
	isAssociated := false
	for _, assoc := range existingAssociations {
		if assoc.ResourceID == gatewayID {
			isAssociated = true
			break
		}
	}

	// If gateway is not associated with the API, create the association
	if !isAssociated {
		association := &model.APIAssociation{
			ApiID:           apiUUID,
			OrganizationID:  orgID,
			ResourceID:      gatewayID,
			AssociationType: constants.AssociationTypeGateway,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
			return nil, fmt.Errorf("failed to create API-gateway association: %w", err)
		}
	}

	// Create deployment record
	deploymentName := fmt.Sprintf("deployment-%d", now.Unix())
	deployed := model.DeploymentStatusDeployed

	// Generate deployment content YAML from notification configuration
	deploymentContent, err := yaml.Marshal(notification.Configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize deployment content: %w", err)
	}

	deployment := &model.APIDeployment{
		Name:           deploymentName,
		ApiID:          apiUUID,
		GatewayID:      gatewayID,
		OrganizationID: orgID,
		Content:        deploymentContent,
		Status:         &deployed,
		CreatedAt:      now,
	}

	// Use same limit computation as DeploymentService: MaxPerAPIGateway + buffer
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	err = s.apiRepo.CreateDeploymentWithLimitEnforcement(deployment, hardLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment record: %w", err)
	}

	return &dto.GatewayAPIDeploymentResponse{
		APIId:        apiUUID,
		DeploymentId: 0, // Legacy field, no longer used with new deployment model
		Message:      "API deployment registered successfully",
		Created:      apiCreated,
	}, nil
}
