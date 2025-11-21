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
	"io"
	"log"
	pathpkg "path"
	"regexp"
	"strings"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// APIService handles business logic for API operations
type APIService struct {
	apiRepo              repository.APIRepository
	projectRepo          repository.ProjectRepository
	gatewayRepo          repository.GatewayRepository
	devPortalRepo        repository.DevPortalRepository
	publicationRepo      repository.APIPublicationRepository
	upstreamService      *UpstreamService
	gatewayEventsService *GatewayEventsService
	devPortalService     *DevPortalService
	apiUtil              *utils.APIUtil
}

// NewAPIService creates a new API service
func NewAPIService(apiRepo repository.APIRepository, projectRepo repository.ProjectRepository,
	gatewayRepo repository.GatewayRepository, devPortalRepo repository.DevPortalRepository,
	publicationRepo repository.APIPublicationRepository, upstreamSvc *UpstreamService,
	gatewayEventsService *GatewayEventsService, devPortalService *DevPortalService, apiUtil *utils.APIUtil) *APIService {
	return &APIService{
		apiRepo:              apiRepo,
		projectRepo:          projectRepo,
		gatewayRepo:          gatewayRepo,
		devPortalRepo:        devPortalRepo,
		publicationRepo:      publicationRepo,
		upstreamService:      upstreamSvc,
		gatewayEventsService: gatewayEventsService,
		devPortalService:     devPortalService,
		apiUtil:              apiUtil,
	}
}

// CreateAPI creates a new API with validation and business logic
func (s *APIService) CreateAPI(req *CreateAPIRequest, orgId string) (*dto.API, error) {
	// Validate request
	if err := s.validateCreateAPIRequest(req); err != nil {
		return nil, err
	}

	// Check if project exists
	project, err := s.projectRepo.GetProjectByUUID(req.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}
	if project.OrganizationID != orgId {
		return nil, constants.ErrProjectNotFound
	}

	// Check if API already exists in the project
	existingAPIs, err := s.apiRepo.GetAPIsByProjectID(req.ProjectID)
	if err != nil {
		return nil, err
	}

	for _, api := range existingAPIs {
		if api.Name == req.Name {
			return nil, constants.ErrAPIAlreadyExists
		}
	}

	// Generate UUID for the API
	apiId := uuid.New().String()

	// Set default values if not provided
	if req.Provider == "" {
		req.Provider = "admin" // Default provider
	}
	if req.Type == "" {
		req.Type = "HTTP"
	}
	if len(req.Transport) == 0 {
		req.Transport = []string{"http", "https"}
	}
	if req.LifeCycleStatus == "" {
		req.LifeCycleStatus = "CREATED"
	}
	if len(req.Operations) == 0 {
		// generate default get, post, patch and delete operations with path /*
		defaultOperations := s.generateDefaultOperations()
		req.Operations = defaultOperations
	}

	// Create API DTO
	api := &dto.API{
		ID:               apiId,
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		Context:          req.Context,
		Version:          req.Version,
		Provider:         req.Provider,
		ProjectID:        req.ProjectID,
		OrganizationID:   orgId,
		LifeCycleStatus:  req.LifeCycleStatus,
		HasThumbnail:     req.HasThumbnail,
		IsDefaultVersion: req.IsDefaultVersion,
		IsRevision:       req.IsRevision,
		RevisionedAPIID:  req.RevisionedAPIID,
		RevisionID:       req.RevisionID,
		Type:             req.Type,
		Transport:        req.Transport,
		MTLS:             req.MTLS,
		Security:         req.Security,
		CORS:             req.CORS,
		BackendServices:  req.BackendServices,
		APIRateLimiting:  req.APIRateLimiting,
		Operations:       req.Operations,
	}

	// Process backend services: check if they exist, create or update them
	var backendServiceIdList []string
	for _, backendService := range req.BackendServices {
		backendServiceId, err := s.upstreamService.UpsertBackendService(&backendService, orgId)
		if err != nil {
			return nil, fmt.Errorf("failed to process backend service '%s': %w", backendService.Name, err)
		}
		backendServiceIdList = append(backendServiceIdList, backendServiceId)
	}

	apiModel := s.apiUtil.DTOToModel(api)
	// Create API in repository
	if err := s.apiRepo.CreateAPI(apiModel); err != nil {
		return nil, fmt.Errorf("failed to create api: %w", err)
	}

	api.CreatedAt = apiModel.CreatedAt
	api.UpdatedAt = apiModel.UpdatedAt

	// Associate backend services with the API
	for i, backendServiceUUID := range backendServiceIdList {
		isDefault := i == 0 // First backend service is default
		if len(req.BackendServices) > 0 && i < len(req.BackendServices) {
			// Check if isDefault was explicitly set in the request
			isDefault = req.BackendServices[i].IsDefault
		}

		if err := s.upstreamService.AssociateBackendServiceWithAPI(apiId, backendServiceUUID, isDefault); err != nil {
			return nil, fmt.Errorf("failed to associate backend service with API: %w", err)
		}
	}

	// Automatically create DevPortal association for default DevPortal
	if err := s.createDefaultDevPortalAssociation(apiId, orgId); err != nil {
		// Log error but don't fail API creation if default DevPortal association fails
		log.Printf("[APIService] Failed to create default DevPortal association for API %s: %v", apiId, err)
	}

	return api, nil
}

// GetAPIByUUID retrieves an API by its ID
func (s *APIService) GetAPIByUUID(apiId, orgId string) (*dto.API, error) {
	if apiId == "" {
		return nil, errors.New("API id is required")
	}

	apiModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	api := s.apiUtil.ModelToDTO(apiModel)
	return api, nil
}

// GetAPIsByOrganization retrieves all APIs for an organization with optional project filter
func (s *APIService) GetAPIsByOrganization(orgId string, projectID *string) ([]*dto.API, error) {
	// If project ID is provided, validate that it belongs to the organization
	if projectID != nil && *projectID != "" {
		project, err := s.projectRepo.GetProjectByUUID(*projectID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, constants.ErrProjectNotFound
		}
		if project.OrganizationID != orgId {
			return nil, constants.ErrProjectNotFound
		}
	}

	apiModels, err := s.apiRepo.GetAPIsByOrganizationID(orgId, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get apis: %w", err)
	}

	apis := make([]*dto.API, 0)
	for _, apiModel := range apiModels {
		api := s.apiUtil.ModelToDTO(apiModel)
		apis = append(apis, api)
	}
	return apis, nil
}

// UpdateAPI updates an existing API
func (s *APIService) UpdateAPI(apiId string, req *UpdateAPIRequest, orgId string) (*dto.API, error) {
	if apiId == "" {
		return nil, errors.New("API id is required")
	}

	// Get existing API
	existingAPIModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, err
	}
	if existingAPIModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if existingAPIModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	existingAPI := s.apiUtil.ModelToDTO(existingAPIModel)

	// Validate update request
	if err := s.validateUpdateAPIRequest(req); err != nil {
		return nil, err
	}

	// Update fields (only allow certain fields to be updated)
	if req.DisplayName != nil {
		existingAPI.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		existingAPI.Description = *req.Description
	}
	if req.Provider != nil {
		existingAPI.Provider = *req.Provider
	}
	if req.LifeCycleStatus != nil {
		existingAPI.LifeCycleStatus = *req.LifeCycleStatus
	}
	if req.HasThumbnail != nil {
		existingAPI.HasThumbnail = *req.HasThumbnail
	}
	if req.IsDefaultVersion != nil {
		existingAPI.IsDefaultVersion = *req.IsDefaultVersion
	}
	if req.IsRevision != nil {
		existingAPI.IsRevision = *req.IsRevision
	}
	if req.RevisionedAPIID != nil {
		existingAPI.RevisionedAPIID = *req.RevisionedAPIID
	}
	if req.RevisionID != nil {
		existingAPI.RevisionID = *req.RevisionID
	}
	if req.Type != nil {
		existingAPI.Type = *req.Type
	}
	if req.Transport != nil {
		existingAPI.Transport = *req.Transport
	}
	if req.MTLS != nil {
		existingAPI.MTLS = req.MTLS
	}
	if req.Security != nil {
		existingAPI.Security = req.Security
	}
	if req.CORS != nil {
		existingAPI.CORS = req.CORS
	}
	if req.BackendServices != nil {
		// Process backend services: check if they exist, create or update them
		var backendServiceUUIDs []string
		for _, backendService := range *req.BackendServices {
			backendServiceUUID, err := s.upstreamService.UpsertBackendService(&backendService, orgId)
			if err != nil {
				return nil, fmt.Errorf("failed to process backend service '%s': %w", backendService.Name, err)
			}
			backendServiceUUIDs = append(backendServiceUUIDs, backendServiceUUID)
		}

		// Remove existing associations and add new ones
		// First, get existing associations to remove them
		existingBackendServices, err := s.upstreamService.GetBackendServicesByAPIID(apiId)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing backend services: %w", err)
		}

		// Remove existing associations
		for _, existingService := range existingBackendServices {
			if err := s.upstreamService.DisassociateBackendServiceFromAPI(apiId, existingService.ID); err != nil {
				return nil, fmt.Errorf("failed to remove existing backend service association: %w", err)
			}
		}

		// Add new associations
		for i, backendServiceUUID := range backendServiceUUIDs {
			isDefault := i == 0 // First backend service is default
			if len(*req.BackendServices) > 0 && i < len(*req.BackendServices) {
				// Check if isDefault was explicitly set in the request
				isDefault = (*req.BackendServices)[i].IsDefault
			}

			if err := s.upstreamService.AssociateBackendServiceWithAPI(apiId, backendServiceUUID, isDefault); err != nil {
				return nil, fmt.Errorf("failed to associate backend service with API: %w", err)
			}
		}

		existingAPI.BackendServices = *req.BackendServices
	}
	if req.APIRateLimiting != nil {
		existingAPI.APIRateLimiting = req.APIRateLimiting
	}
	if req.Operations != nil {
		existingAPI.Operations = *req.Operations
	}

	// Update API in repository
	updatedAPIModel := s.apiUtil.DTOToModel(existingAPI)
	if err := s.apiRepo.UpdateAPI(updatedAPIModel); err != nil {
		return nil, fmt.Errorf("failed to update api: %w", err)
	}

	return existingAPI, nil
}

// DeleteAPI deletes an API
func (s *APIService) DeleteAPI(apiId, orgId string) error {
	if apiId == "" {
		return errors.New("API id is required")
	}

	// Check if API exists
	api, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return err
	}
	if api == nil {
		return constants.ErrAPINotFound
	}
	if api.OrganizationID != orgId {
		return constants.ErrAPINotFound
	}

	// Delete API from repository
	if err := s.apiRepo.DeleteAPI(apiId); err != nil {
		return fmt.Errorf("failed to delete api: %w", err)
	}

	return nil
}

// UpdateAPILifecycleStatus updates only the lifecycle status of an API
func (s *APIService) UpdateAPILifecycleStatus(apiId string, status string) (*dto.API, error) {
	if apiId == "" {
		return nil, errors.New("API id is required")
	}
	if status == "" {
		return nil, errors.New("status is required")
	}

	// Validate lifecycle status
	if !constants.ValidLifecycleStates[status] {
		return nil, constants.ErrInvalidLifecycleState
	}

	// Get existing API
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}

	// Update lifecycle status
	apiModel.LifeCycleStatus = status
	apiModel.UpdatedAt = time.Now()

	// Update API in repository
	if err := s.apiRepo.UpdateAPI(apiModel); err != nil {
		return nil, fmt.Errorf("failed to update api lifecycle status: %w", err)
	}

	api := s.apiUtil.ModelToDTO(apiModel)
	return api, nil
}

// DeployAPIRevision deploys an API revision and generates deployment YAML
func (s *APIService) DeployAPIRevision(apiId string, revisionID string,
	deploymentRequests []dto.APIRevisionDeployment, orgId string) ([]*dto.APIRevisionDeployment, error) {
	if apiId == "" {
		return nil, errors.New("api id is required")
	}

	// Get the API from database
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	// Get existing associations to check which gateways need association
	existingAssociations, err := s.apiRepo.GetAPIAssociations(apiId, constants.AssociationTypeGateway, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing API-gateway associations: %w", err)
	}

	// Create a map of existing gateway associations for quick lookup
	existingGatewayIds := make(map[string]bool)
	for _, assoc := range existingAssociations {
		existingGatewayIds[assoc.ResourceID] = true
	}

	// Process deployment requests and create deployment responses
	var deployments []*dto.APIRevisionDeployment
	currentTime := time.Now().Format(time.RFC3339)

	for _, deploymentReq := range deploymentRequests {
		// Validate deployment request
		if err := s.validateDeploymentRequest(&deploymentReq, apiModel, orgId); err != nil {
			return nil, fmt.Errorf("invalid api deployment: %w", err)
		}

		// If gateway is not associated with the API, create the association
		if !existingGatewayIds[deploymentReq.GatewayID] {
			log.Printf("[INFO] Creating API-gateway association: apiId=%s gatewayId=%s",
				apiId, deploymentReq.GatewayID)

			association := &model.APIAssociation{
				ApiID:           apiId,
				OrganizationID:  orgId,
				ResourceID:      deploymentReq.GatewayID,
				AssociationType: constants.AssociationTypeGateway,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
				return nil, fmt.Errorf("failed to create API-gateway association for gateway %s: %w",
					deploymentReq.GatewayID, err)
			}

			// Add to the map to avoid duplicate creation in the same request
			existingGatewayIds[deploymentReq.GatewayID] = true
			log.Printf("[INFO] Created API-gateway association: apiId=%s gatewayId=%s associationId=%d",
				apiId, deploymentReq.GatewayID, association.ID)
		}

		deployment := &dto.APIRevisionDeployment{
			RevisionId:          revisionID, // Optional, can be empty
			GatewayID:           deploymentReq.GatewayID,
			Status:              "CREATED", // Default status for new deployments
			VHost:               deploymentReq.VHost,
			DisplayOnDevportal:  deploymentReq.DisplayOnDevportal,
			DeployedTime:        &currentTime,
			SuccessDeployedTime: &currentTime,
		}

		deployments = append(deployments, deployment)

		// Create deployment record in the database
		deploymentRecord := &model.APIDeployment{
			ApiID:          apiId,
			OrganizationID: orgId,
			GatewayID:      deployment.GatewayID,
		}

		if err := s.apiRepo.CreateDeployment(deploymentRecord); err != nil {
			log.Printf("[ERROR] Failed to create deployment record: apiId=%s gatewayID=%s error=%v",
				apiId, deployment.GatewayID, err)
		} else {
			log.Printf("[INFO] Created deployment record: apiId=%s gatewayID=%s deploymentId=%d",
				apiId, deployment.GatewayID, deploymentRecord.ID)
		}

		// Send deployment event to gateway via WebSocket
		deploymentEvent := &model.APIDeploymentEvent{
			ApiId:       apiId,
			RevisionID:  revisionID,
			Vhost:       deployment.VHost,
			Environment: "production", // Default environment
		}

		// Broadcast deployment event to target gateway
		if s.gatewayEventsService != nil {
			if err := s.gatewayEventsService.BroadcastDeploymentEvent(deployment.GatewayID, deploymentEvent); err != nil {
				log.Printf("[WARN] Failed to broadcast deployment event: apiId=%s gatewayID=%s error=%v",
					apiId, deployment.GatewayID, err)
				// Continue execution - event delivery failure doesn't fail the deployment
			}
		}
	}

	return deployments, nil
}

// AddGatewaysToAPI associates multiple gateways with an API
func (s *APIService) AddGatewaysToAPI(apiId string, gatewayIds []string, orgId string) (*dto.APIGatewayListResponse, error) {
	// Validate that the API exists and belongs to the organization
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	// Validate that all gateways exist and belong to the same organization
	var validGateways []*model.Gateway
	for _, gatewayId := range gatewayIds {
		gateway, err := s.gatewayRepo.GetByUUID(gatewayId)
		if err != nil {
			return nil, err
		}
		if gateway == nil {
			return nil, constants.ErrGatewayNotFound
		}
		if gateway.OrganizationID != orgId {
			return nil, constants.ErrGatewayNotFound
		}
		validGateways = append(validGateways, gateway)
	}

	// Get existing associations to determine which are new vs existing
	existingAssociations, err := s.apiRepo.GetAPIAssociations(apiId, constants.AssociationTypeGateway, orgId)
	if err != nil {
		return nil, err
	}

	existingGatewayIds := make(map[string]bool)
	for _, assoc := range existingAssociations {
		existingGatewayIds[assoc.ResourceID] = true
	}

	// Process each gateway: create new associations or update existing ones
	for _, gateway := range validGateways {
		if existingGatewayIds[gateway.ID] {
			// Update existing association timestamp
			if err := s.apiRepo.UpdateAPIAssociation(apiId, gateway.ID, constants.AssociationTypeGateway, orgId); err != nil {
				return nil, err
			}
		} else {
			// Create new association
			association := &model.APIAssociation{
				ApiID:           apiId,
				OrganizationID:  orgId,
				ResourceID:      gateway.ID,
				AssociationType: constants.AssociationTypeGateway,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			// Check for duplicate API context + version before creating new association
			existingAPIs, err := s.apiRepo.GetAPIsByGatewayID(gateway.ID, orgId)
			if err != nil {
				return nil, fmt.Errorf("failed to get existing gateway APIs: %w", err)
			}

			// Check for duplicate context + version combination
			for _, existingAPI := range existingAPIs {
				if existingAPI.Context == apiModel.Context && existingAPI.Version == apiModel.Version {
					log.Printf("WARNING: API with context '%s' and version '%s' already exists in gateway '%s'.",
						apiModel.Context, apiModel.Version, gateway.Name)
					break
				}
			}

			if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
				return nil, err
			}
			existingGatewayIds[gateway.ID] = true
		}
	}

	// Return all gateways currently associated with the API including deployment details
	gatewayDetails, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return nil, err
	}

	// Convert all associated gateways to DTOs with deployment details for response
	responses := make([]dto.APIGatewayResponse, 0, len(gatewayDetails))
	for _, gwd := range gatewayDetails {
		responses = append(responses, s.convertToAPIGatewayResponse(gwd))
	}

	// Create response with all associated gateways
	listResponse := &dto.APIGatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}

	return listResponse, nil
}

// GetAPIGateways retrieves all gateways associated with an API including deployment details
func (s *APIService) GetAPIGateways(apiId, orgId string) (*dto.APIGatewayListResponse, error) {
	// Validate that the API exists and belongs to the organization
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	// Get all gateways associated with this API including deployment details
	gatewayDetails, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return nil, err
	}

	// Convert models to DTOs with deployment details
	responses := make([]dto.APIGatewayResponse, 0, len(gatewayDetails))
	for _, gwd := range gatewayDetails {
		responses = append(responses, s.convertToAPIGatewayResponse(gwd))
	}

	// Create paginated response
	listResponse := &dto.APIGatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}

	return listResponse, nil
}

// validateDeploymentRequest validates the deployment request
func (s *APIService) validateDeploymentRequest(req *dto.APIRevisionDeployment, api *model.API, orgId string) error {
	if req.GatewayID == "" {
		return errors.New("gateway Id is required")
	}
	if req.VHost == "" {
		return errors.New("vhost is required")
	}
	// TODO - vHost validation
	gateway, err := s.gatewayRepo.GetByUUID(req.GatewayID)
	if err != nil {
		return fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway.OrganizationID != orgId {
		return fmt.Errorf("failed to get gateway: %w", err)
	}

	// Get all APIs currently deployed to this gateway
	existingAPIs, err := s.apiRepo.GetDeployedAPIsByGatewayID(req.GatewayID, orgId)
	if err != nil {
		return fmt.Errorf("failed to get deployed APIs for gateway: %w", err)
	}

	// Check for duplicate context+version combination
	for _, existingAPI := range existingAPIs {
		// Skip checking against the same API (in case of redeployment)
		if existingAPI.ID == api.ID {
			continue
		}

		// Check if context and version match
		if existingAPI.Context == api.Context && existingAPI.Version == api.Version {
			return fmt.Errorf("an API with the same context '%s' and version '%s' is already deployed"+
				" in the gateway '%s'", api.Context, api.Version, gateway.ID)
		}
	}

	return nil
}

// createDefaultDevPortalAssociation creates an association between the API and the default DevPortal
func (s *APIService) createDefaultDevPortalAssociation(apiId, orgId string) error {
	// Get default DevPortal for the organization
	defaultDevPortal, err := s.devPortalRepo.GetDefaultByOrganizationUUID(orgId)
	if err != nil {
		// If no default DevPortal exists, skip association (not an error)
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			log.Printf("[APIService] No default DevPortal found for organization %s, skipping association", orgId)
			return nil
		}
		return fmt.Errorf("failed to get default DevPortal: %w", err)
	}

	// Create API-DevPortal association
	association := &model.APIAssociation{
		ApiID:           apiId,
		OrganizationID:  orgId,
		ResourceID:      defaultDevPortal.UUID,
		AssociationType: "dev_portal",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
		// Check if association already exists (shouldn't happen, but handle gracefully)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			log.Printf("[APIService] API association with default DevPortal already exists for API %s", apiId)
			return nil
		}
		return fmt.Errorf("failed to create API-DevPortal association: %w", err)
	}

	log.Printf("[APIService] Successfully created association between API %s and default DevPortal %s", apiId, defaultDevPortal.UUID)
	return nil
}

// Validation methods

// validateCreateAPIRequest checks the validity of the create API request
func (s *APIService) validateCreateAPIRequest(req *CreateAPIRequest) error {
	if req.Name == "" {
		return constants.ErrInvalidAPIName
	}
	if req.Context == "" {
		return constants.ErrInvalidAPIContext
	}
	if req.Version == "" {
		return constants.ErrInvalidAPIVersion
	}
	if req.ProjectID == "" {
		return errors.New("project id is required")
	}

	// Validate API name format
	if !s.isValidAPIName(req.Name) {
		return constants.ErrInvalidAPIName
	}

	// Validate context format
	if !s.isValidContext(req.Context) {
		return constants.ErrInvalidAPIContext
	}

	// Validate version format
	if !s.isValidVersion(req.Version) {
		return constants.ErrInvalidAPIVersion
	}

	// Validate lifecycle status if provided
	if req.LifeCycleStatus != "" && !constants.ValidLifecycleStates[req.LifeCycleStatus] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Type != "" && !constants.ValidAPITypes[req.Type] {
		return constants.ErrInvalidAPIType
	}

	// Validate transport protocols if provided
	if len(req.Transport) > 0 {
		for _, transport := range req.Transport {
			if !constants.ValidTransports[strings.ToLower(transport)] {
				return constants.ErrInvalidTransport
			}
		}
	}

	return nil
}

// validateUpdateAPIRequest checks the validity of the update API request
func (s *APIService) validateUpdateAPIRequest(req *UpdateAPIRequest) error {
	// Validate lifecycle status if provided
	if req.LifeCycleStatus != nil && !constants.ValidLifecycleStates[*req.LifeCycleStatus] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Type != nil && !constants.ValidAPITypes[*req.Type] {
		return constants.ErrInvalidAPIType
	}

	// Validate transport protocols if provided
	if req.Transport != nil {
		for _, transport := range *req.Transport {
			if !constants.ValidTransports[strings.ToLower(transport)] {
				return constants.ErrInvalidTransport
			}
		}
	}

	return nil
}

// Helper validation methods

func (s *APIService) isValidAPIName(name string) bool {
	// API name should not contain special characters except spaces and hyphens
	pattern := `^[^~!@#;:%^*()+={}|\\<>"'',&$\[\]\/]*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched && len(name) > 0
}

func (s *APIService) isValidContext(context string) bool {
	// Context should be URL-friendly, no spaces or special characters
	pattern := `^\/?[a-zA-Z0-9_/-]+$`
	matched, _ := regexp.MatchString(pattern, context)
	return matched && len(context) > 0 && len(context) <= 232
}

func (s *APIService) isValidVersion(version string) bool {
	// Version should follow semantic versioning or simple version format
	pattern := `^[^~!@#;:%^*()+={}|\\<>"'',&/$\[\]\s+\/]+$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched && len(version) > 0 && len(version) <= 30
}

// isValidVHost validates vhost format
func (s *APIService) isValidVHost(vhost string) bool {
	// Basic hostname validation pattern as per RFC 1123
	pattern := `^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-ZaZ0-9\-]*[A-ZaZ0-9])$`
	matched, _ := regexp.MatchString(pattern, vhost)
	return matched
}

// Request/Response DTOs

// CreateAPIRequest represents the request to create a new API
type CreateAPIRequest struct {
	Name             string                  `json:"name"`
	DisplayName      string                  `json:"displayName,omitempty"`
	Description      string                  `json:"description,omitempty"`
	Context          string                  `json:"context"`
	Version          string                  `json:"version"`
	Provider         string                  `json:"provider,omitempty"`
	ProjectID        string                  `json:"projectId"`
	LifeCycleStatus  string                  `json:"lifeCycleStatus,omitempty"`
	HasThumbnail     bool                    `json:"hasThumbnail,omitempty"`
	IsDefaultVersion bool                    `json:"isDefaultVersion,omitempty"`
	IsRevision       bool                    `json:"isRevision,omitempty"`
	RevisionedAPIID  string                  `json:"revisionedApiId,omitempty"`
	RevisionID       int                     `json:"revisionId,omitempty"`
	Type             string                  `json:"type,omitempty"`
	Transport        []string                `json:"transport,omitempty"`
	MTLS             *dto.MTLSConfig         `json:"mtls,omitempty"`
	Security         *dto.SecurityConfig     `json:"security,omitempty"`
	CORS             *dto.CORSConfig         `json:"cors,omitempty"`
	BackendServices  []dto.BackendService    `json:"backend-services,omitempty"`
	APIRateLimiting  *dto.RateLimitingConfig `json:"api-rate-limiting,omitempty"`
	Operations       []dto.Operation         `json:"operations,omitempty"`
}

// UpdateAPIRequest represents the request to update an API
type UpdateAPIRequest struct {
	DisplayName      *string                 `json:"displayName,omitempty"`
	Description      *string                 `json:"description,omitempty"`
	Provider         *string                 `json:"provider,omitempty"`
	LifeCycleStatus  *string                 `json:"lifeCycleStatus,omitempty"`
	HasThumbnail     *bool                   `json:"hasThumbnail,omitempty"`
	IsDefaultVersion *bool                   `json:"isDefaultVersion,omitempty"`
	IsRevision       *bool                   `json:"isRevision,omitempty"`
	RevisionedAPIID  *string                 `json:"revisionedApiId,omitempty"`
	RevisionID       *int                    `json:"revisionId,omitempty"`
	Type             *string                 `json:"type,omitempty"`
	Transport        *[]string               `json:"transport,omitempty"`
	MTLS             *dto.MTLSConfig         `json:"mtls,omitempty"`
	Security         *dto.SecurityConfig     `json:"security,omitempty"`
	CORS             *dto.CORSConfig         `json:"cors,omitempty"`
	BackendServices  *[]dto.BackendService   `json:"backend-services,omitempty"`
	APIRateLimiting  *dto.RateLimitingConfig `json:"api-rate-limiting,omitempty"`
	Operations       *[]dto.Operation        `json:"operations,omitempty"`
}

// generateDefaultOperations creates default CRUD operations for an API
func (s *APIService) generateDefaultOperations() []dto.Operation {
	return []dto.Operation{
		{
			Name:        "Get Resource",
			Description: "Retrieve all resources",
			Request: &dto.OperationRequest{
				Method: "GET",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
		{
			Name:        "POST Resource",
			Description: "Create a new resource",
			Request: &dto.OperationRequest{
				Method: "POST",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
		{
			Name:        "Update Resource",
			Description: "Update an existing resource",
			Request: &dto.OperationRequest{
				Method: "PATCH",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
		{
			Name:        "Delete Resource",
			Description: "Delete an existing resource",
			Request: &dto.OperationRequest{
				Method: "DELETE",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
	}
}

// ImportAPIProject imports an API project from a Git repository
func (s *APIService) ImportAPIProject(req *dto.ImportAPIProjectRequest, orgId string, gitService GitService) (*dto.API, error) {
	// 1. Validate if there is a .api-platform directory with config.yaml
	config, err := gitService.ValidateAPIProject(req.RepoURL, req.Branch, req.Path)
	if err != nil {
		if strings.Contains(err.Error(), "api project not found") {
			return nil, constants.ErrAPIProjectNotFound
		}
		if strings.Contains(err.Error(), "malformed api project") {
			return nil, constants.ErrMalformedAPIProject
		}
		if strings.Contains(err.Error(), "invalid api project") {
			return nil, constants.ErrInvalidAPIProject
		}
		return nil, err
	}

	// For now, we'll process the first API in the config (can be extended later for multiple APIs)
	if len(config.APIs) == 0 {
		return nil, constants.ErrMalformedAPIProject
	}

	apiConfig := config.APIs[0]

	// 5. Fetch the WSO2 artifact file content
	wso2ArtifactClean := pathpkg.Clean(apiConfig.WSO2Artifact)
	wso2ArtifactPath := pathpkg.Join(req.Path, wso2ArtifactClean)
	artifactData, err := gitService.FetchWSO2Artifact(req.RepoURL, req.Branch, wso2ArtifactPath)
	if err != nil {
		return nil, constants.ErrWSO2ArtifactNotFound
	}

	// 6. Create API with details from WSO2 artifact, overwritten by request details
	apiData := s.mergeAPIData(&artifactData.Data, &req.API)

	// 7. Create API using the existing CreateAPI flow
	createReq := &CreateAPIRequest{
		Name:             apiData.Name,
		DisplayName:      apiData.DisplayName,
		Description:      apiData.Description,
		Context:          apiData.Context,
		Version:          apiData.Version,
		Provider:         apiData.Provider,
		ProjectID:        apiData.ProjectID,
		LifeCycleStatus:  apiData.LifeCycleStatus,
		HasThumbnail:     apiData.HasThumbnail,
		IsDefaultVersion: apiData.IsDefaultVersion,
		IsRevision:       apiData.IsRevision,
		RevisionedAPIID:  apiData.RevisionedAPIID,
		RevisionID:       apiData.RevisionID,
		Type:             apiData.Type,
		Transport:        apiData.Transport,
		MTLS:             apiData.MTLS,
		Security:         apiData.Security,
		CORS:             apiData.CORS,
		BackendServices:  apiData.BackendServices,
		APIRateLimiting:  apiData.APIRateLimiting,
		Operations:       apiData.Operations,
	}

	return s.CreateAPI(createReq, orgId)
}

// mergeAPIData merges WSO2 artifact data with user-provided API data (user data takes precedence)
func (s *APIService) mergeAPIData(artifact *dto.APIYAMLData2, userAPIData *dto.API) *dto.API {
	apiDTO := s.apiUtil.APIYAMLData2ToDTO(artifact)

	// Overwrite with user-provided data (if not empty)
	if userAPIData.Name != "" {
		apiDTO.Name = userAPIData.Name
	}
	if userAPIData.DisplayName != "" {
		apiDTO.DisplayName = userAPIData.DisplayName
	}
	if userAPIData.Description != "" {
		apiDTO.Description = userAPIData.Description
	}
	if userAPIData.Context != "" {
		apiDTO.Context = userAPIData.Context
	}
	if userAPIData.Version != "" {
		apiDTO.Version = userAPIData.Version
	}
	if userAPIData.Provider != "" {
		apiDTO.Provider = userAPIData.Provider
	}
	if userAPIData.ProjectID != "" {
		apiDTO.ProjectID = userAPIData.ProjectID
	}
	if userAPIData.LifeCycleStatus != "" {
		apiDTO.LifeCycleStatus = userAPIData.LifeCycleStatus
	}
	if userAPIData.Type != "" {
		apiDTO.Type = userAPIData.Type
	}
	if len(userAPIData.Transport) > 0 {
		apiDTO.Transport = userAPIData.Transport
	}
	if userAPIData.BackendServices != nil && len(userAPIData.BackendServices) > 0 {
		apiDTO.BackendServices = userAPIData.BackendServices
	}

	// Handle boolean fields
	apiDTO.HasThumbnail = userAPIData.HasThumbnail
	apiDTO.IsDefaultVersion = userAPIData.IsDefaultVersion
	apiDTO.IsRevision = userAPIData.IsRevision

	if userAPIData.RevisionedAPIID != "" {
		apiDTO.RevisionedAPIID = userAPIData.RevisionedAPIID
	}
	if userAPIData.RevisionID != 0 {
		apiDTO.RevisionID = userAPIData.RevisionID
	}

	return apiDTO
}

// ValidateAndRetrieveAPIProject validates an API project from Git repository with comprehensive checks
func (s *APIService) ValidateAndRetrieveAPIProject(req *dto.ValidateAPIProjectRequest,
	gitService GitService) (*dto.APIProjectValidationResponse, error) {
	response := &dto.APIProjectValidationResponse{
		IsAPIProjectValid:    false,
		IsAPIConfigValid:     false,
		IsAPIDefinitionValid: false,
		Errors:               []string{},
	}

	// Step 1: Check if .api-platform directory exists and validate config
	config, err := gitService.ValidateAPIProject(req.RepoURL, req.Branch, req.Path)
	if err != nil {
		response.Errors = append(response.Errors, err.Error())
		return response, nil
	}

	// Process the first API entry (assuming single API per project for now)
	apiEntry := config.APIs[0]

	// Step 3: Fetch OpenAPI definition
	openAPIClean := pathpkg.Clean(apiEntry.OpenAPI)
	openAPIPath := pathpkg.Join(req.Path, openAPIClean)
	openAPIContent, err := gitService.FetchFileContent(req.RepoURL, req.Branch, openAPIPath)
	if err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("failed to fetch OpenAPI file: %s", err.Error()))
		return response, nil
	}

	// Basic OpenAPI validation (check if it's valid YAML/JSON with required fields)
	if err := s.apiUtil.ValidateOpenAPIDefinition(openAPIContent); err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("invalid OpenAPI definition: %s", err.Error()))
		return response, nil
	}

	response.IsAPIDefinitionValid = true

	// Step 4: Fetch WSO2 artifact (api.yaml)
	wso2ArtifactClean := pathpkg.Clean(apiEntry.WSO2Artifact)
	wso2ArtifactPath := pathpkg.Join(req.Path, wso2ArtifactClean)
	wso2ArtifactContent, err := gitService.FetchFileContent(req.RepoURL, req.Branch, wso2ArtifactPath)
	if err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("failed to fetch WSO2 artifact file: %s", err.Error()))
		return response, nil
	}

	var wso2Artifact dto.APIDeploymentYAML
	if err := yaml.Unmarshal(wso2ArtifactContent, &wso2Artifact); err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("invalid WSO2 artifact format: %s", err.Error()))
		return response, nil
	}

	// Step 5: Validate WSO2 artifact structure
	if err := s.apiUtil.ValidateWSO2Artifact(&wso2Artifact); err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("invalid WSO2 artifact: %s", err.Error()))
		return response, nil
	}

	response.IsAPIConfigValid = true

	// Step 6: Check if OpenAPI and WSO2 artifact match (optional validation)
	if err := s.apiUtil.ValidateAPIDefinitionConsistency(openAPIContent, &wso2Artifact); err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("API definitions mismatch: %s", err.Error()))
		response.IsAPIProjectValid = false
		return response, nil
	}

	// Step 7: If all validations pass, convert to API DTO
	api, err := s.apiUtil.ConvertAPIYAMLDataToDTO(&wso2Artifact)
	if err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("failed to convert API data: %s", err.Error()))
		return response, nil
	}

	response.API = api
	response.IsAPIProjectValid = response.IsAPIConfigValid && response.IsAPIDefinitionValid

	return response, nil
}

// PublishAPIToDevPortal publishes an API to a specific DevPortal
func (s *APIService) PublishAPIToDevPortal(apiID string, req *dto.PublishToDevPortalRequest, orgID string) (*dto.PublishToDevPortalResponse, error) {
	// Get the API
	api, err := s.GetAPIByUUID(apiID, orgID)
	if err != nil {
		return nil, err
	}

	// Publish API to DevPortal
	return s.devPortalService.PublishAPIToDevPortal(api, req, orgID)
}

// UnpublishAPIFromDevPortal unpublishes an API from a specific DevPortal
func (s *APIService) UnpublishAPIFromDevPortal(apiID, devPortalUUID, orgID string) (*dto.UnpublishFromDevPortalResponse, error) {
	// Unpublish API from DevPortal
	return s.devPortalService.UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID)
}

// GetAPIPublications retrieves all DevPortals associated with an API including publication details
// This mirrors the GetAPIGateways implementation for consistency
func (s *APIService) GetAPIPublications(apiID, orgID string) (*dto.APIDevPortalListResponse, error) {
	// Validate that the API exists and belongs to the organization
	apiModel, err := s.apiRepo.GetAPIByUUID(apiID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgID {
		return nil, constants.ErrAPINotFound
	}

	// Get all DevPortals associated with this API including publication details
	devPortalDetails, err := s.publicationRepo.GetAPIDevPortalsWithDetails(apiID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API-DevPortal associations: %w", err)
	}

	// Convert models to DTOs with publication details
	responses := make([]dto.APIDevPortalResponse, 0, len(devPortalDetails))
	for _, dpd := range devPortalDetails {
		responses = append(responses, s.convertToAPIDevPortalResponse(dpd))
	}

	// Create paginated response
	listResponse := &dto.APIDevPortalListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}

	return listResponse, nil
}

// convertToAPIGatewayResponse converts APIGatewayWithDetails to APIGatewayResponse
func (s *APIService) convertToAPIGatewayResponse(gwd *model.APIGatewayWithDetails) dto.APIGatewayResponse {
	// Create the base gateway response
	gatewayResponse := dto.GatewayResponse{
		ID:                gwd.ID,
		OrganizationID:    gwd.OrganizationID,
		Name:              gwd.Name,
		DisplayName:       gwd.DisplayName,
		Description:       gwd.Description,
		Vhost:             gwd.Vhost,
		IsCritical:        gwd.IsCritical,
		FunctionalityType: gwd.FunctionalityType,
		IsActive:          gwd.IsActive,
		CreatedAt:         gwd.CreatedAt,
		UpdatedAt:         gwd.UpdatedAt,
	}

	// Create API gateway response with embedded gateway response
	apiGatewayResponse := dto.APIGatewayResponse{
		GatewayResponse: gatewayResponse,
		AssociatedAt:    gwd.AssociatedAt,
		IsDeployed:      gwd.IsDeployed,
	}

	// Add deployment details if deployed
	if gwd.IsDeployed && gwd.DeployedAt != nil {
		revisionID := ""
		if gwd.DeployedRevision != nil {
			revisionID = *gwd.DeployedRevision
		}
		apiGatewayResponse.Deployment = &dto.APIDeploymentDetails{
			RevisionID: revisionID,
			Status:     "CREATED", // Default status, can be enhanced later
			DeployedAt: *gwd.DeployedAt,
		}
	}

	return apiGatewayResponse
}

// convertToAPIDevPortalResponse converts APIDevPortalWithDetails to APIDevPortalResponse
func (s *APIService) convertToAPIDevPortalResponse(dpd *model.APIDevPortalWithDetails) dto.APIDevPortalResponse {
	// Create the base DevPortal response
	devPortalResponse := dto.DevPortalResponse{
		UUID:             dpd.UUID,
		OrganizationUUID: dpd.OrganizationUUID,
		Name:             dpd.Name,
		Identifier:       dpd.Identifier,
		UIUrl:            fmt.Sprintf("%s/%s/views/default/apis", dpd.APIUrl, dpd.Identifier), // Computed field
		APIUrl:           dpd.APIUrl,
		Hostname:         dpd.Hostname,
		IsActive:         dpd.IsActive,
		IsEnabled:        dpd.IsEnabled,
		HeaderKeyName:    "", // Not included in response for security
		IsDefault:        dpd.IsDefault,
		Visibility:       dpd.Visibility,
		Description:      dpd.Description,
		CreatedAt:        dpd.CreatedAt,
		UpdatedAt:        dpd.UpdatedAt,
	}

	// Create API DevPortal response with embedded DevPortal response
	apiDevPortalResponse := dto.APIDevPortalResponse{
		DevPortalResponse: devPortalResponse,
		AssociatedAt:      dpd.AssociatedAt,
		IsPublished:       dpd.IsPublished,
	}

	// Add publication details if published
	if dpd.IsPublished && dpd.PublishedAt != nil {
		status := ""
		if dpd.PublicationStatus != nil {
			status = *dpd.PublicationStatus
		}
		apiVersion := ""
		if dpd.APIVersion != nil {
			apiVersion = *dpd.APIVersion
		}
		devPortalRefID := ""
		if dpd.DevPortalRefID != nil {
			devPortalRefID = *dpd.DevPortalRefID
		}
		sandboxEndpoint := ""
		if dpd.SandboxEndpointURL != nil {
			sandboxEndpoint = *dpd.SandboxEndpointURL
		}
		productionEndpoint := ""
		if dpd.ProductionEndpointURL != nil {
			productionEndpoint = *dpd.ProductionEndpointURL
		}
		updatedAt := time.Now()
		if dpd.PublicationUpdatedAt != nil {
			updatedAt = *dpd.PublicationUpdatedAt
		}

		apiDevPortalResponse.Publication = &dto.APIPublicationDetails{
			Status:             status,
			APIVersion:         apiVersion,
			DevPortalRefID:     devPortalRefID,
			SandboxEndpoint:    sandboxEndpoint,
			ProductionEndpoint: productionEndpoint,
			PublishedAt:        *dpd.PublishedAt,
			UpdatedAt:          updatedAt,
		}
	}

	return apiDevPortalResponse
}

// ValidateOpenAPIDefinition validates an OpenAPI definition from multipart form data
func (s *APIService) ValidateOpenAPIDefinition(req *dto.ValidateOpenAPIRequest) (*dto.OpenAPIValidationResponse, error) {
	response := &dto.OpenAPIValidationResponse{
		IsAPIDefinitionValid: false,
		Errors:               []string{},
	}

	var content []byte
	var err error

	// If URL is provided, fetch content from URL
	if req.URL != "" {
		content, err = s.apiUtil.FetchOpenAPIFromURL(req.URL)
		if err != nil {
			content = make([]byte, 0)
			response.Errors = append(response.Errors, fmt.Sprintf("failed to fetch OpenAPI from URL: %s", err.Error()))
		}
	}

	// If definition file is provided, read from file
	if req.Definition != nil {
		file, err := req.Definition.Open()
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("failed to open definition file: %s", err.Error()))
			return response, nil
		}
		defer file.Close()

		content, err = io.ReadAll(file)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("failed to read definition file: %s", err.Error()))
			return response, nil
		}
	}

	// If neither URL nor file is provided
	if len(content) == 0 {
		response.Errors = append(response.Errors, "either URL or definition file must be provided")
		return response, nil
	}

	// Validate the OpenAPI definition
	if err := s.apiUtil.ValidateOpenAPIDefinition(content); err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("invalid OpenAPI definition: %s", err.Error()))
		return response, nil
	}

	// Parse API specification to extract metadata directly into API DTO using libopenapi
	api, err := s.apiUtil.ParseAPIDefinition(content)
	if err != nil {
		response.Errors = append(response.Errors, fmt.Sprintf("failed to parse API specification: %s", err.Error()))
		return response, nil
	}

	// Set the parsed API for response
	response.IsAPIDefinitionValid = true
	response.API = api

	return response, nil
}

// ImportFromOpenAPI imports an API from an OpenAPI definition
func (s *APIService) ImportFromOpenAPI(req *dto.ImportOpenAPIRequest, orgId string) (*dto.API, error) {
	var content []byte
	var err error
	var errorList []string

	// If URL is provided, fetch content from URL
	if req.URL != "" {
		content, err = s.apiUtil.FetchOpenAPIFromURL(req.URL)
		if err != nil {
			content = make([]byte, 0)
			errorList = append(errorList, fmt.Sprintf("failed to fetch OpenAPI from URL: %s", err.Error()))
		}
	}

	// If definition file is provided, read from file
	if req.Definition != nil {
		file, err := req.Definition.Open()
		if err != nil {
			errorList = append(errorList, fmt.Sprintf("failed to open OpenAPI definition file: %s", err.Error()))
			return nil, fmt.Errorf(strings.Join(errorList, "; "))
		}
		defer file.Close()

		content, err = io.ReadAll(file)
		if err != nil {
			errorList = append(errorList, fmt.Sprintf("failed to read OpenAPI definition file: %s", err.Error()))
			return nil, fmt.Errorf(strings.Join(errorList, "; "))
		}
	}

	// If neither URL nor file is provided
	if len(content) == 0 {
		errorList = append(errorList, "either URL or definition file must be provided")
		return nil, fmt.Errorf(strings.Join(errorList, "; "))
	}

	// Validate and parse the OpenAPI definition
	apiDetails, err := s.apiUtil.ValidateAndParseOpenAPI(content)
	if err != nil {
		return nil, fmt.Errorf("failed to validate and parse OpenAPI definition: %w", err)
	}

	// Merge provided API details with extracted details from OpenAPI
	mergedAPI := s.apiUtil.MergeAPIDetails(&req.API, apiDetails)
	if mergedAPI == nil {
		return nil, errors.New("failed to merge API details")
	}

	// Create API using existing CreateAPI logic
	createReq := &CreateAPIRequest{
		Name:            mergedAPI.Name,
		DisplayName:     mergedAPI.DisplayName,
		Description:     mergedAPI.Description,
		Context:         mergedAPI.Context,
		Version:         mergedAPI.Version,
		Provider:        mergedAPI.Provider,
		ProjectID:       mergedAPI.ProjectID,
		LifeCycleStatus: mergedAPI.LifeCycleStatus,
		Type:            mergedAPI.Type,
		Transport:       mergedAPI.Transport,
		MTLS:            mergedAPI.MTLS,
		Security:        mergedAPI.Security,
		CORS:            mergedAPI.CORS,
		BackendServices: mergedAPI.BackendServices,
		APIRateLimiting: mergedAPI.APIRateLimiting,
		Operations:      mergedAPI.Operations,
	}

	// Create the API
	return s.CreateAPI(createReq, orgId)
}
