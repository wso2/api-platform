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

	"gopkg.in/yaml.v3"
)

// APIService handles business logic for API operations
type APIService struct {
	apiRepo              repository.APIRepository
	projectRepo          repository.ProjectRepository
	orgRepo              repository.OrganizationRepository
	gatewayRepo          repository.GatewayRepository
	devPortalRepo        repository.DevPortalRepository
	publicationRepo      repository.APIPublicationRepository
	backendServiceRepo   repository.BackendServiceRepository
	upstreamService      *UpstreamService
	gatewayEventsService *GatewayEventsService
	devPortalService     *DevPortalService
	apiUtil              *utils.APIUtil
}

// NewAPIService creates a new API service
func NewAPIService(apiRepo repository.APIRepository, projectRepo repository.ProjectRepository,
	orgRepo repository.OrganizationRepository, gatewayRepo repository.GatewayRepository,
	devPortalRepo repository.DevPortalRepository, publicationRepo repository.APIPublicationRepository,
	backendServiceRepo repository.BackendServiceRepository, upstreamSvc *UpstreamService,
	gatewayEventsService *GatewayEventsService, devPortalService *DevPortalService, apiUtil *utils.APIUtil) *APIService {
	return &APIService{
		apiRepo:              apiRepo,
		projectRepo:          projectRepo,
		orgRepo:              orgRepo,
		gatewayRepo:          gatewayRepo,
		devPortalRepo:        devPortalRepo,
		publicationRepo:      publicationRepo,
		backendServiceRepo:   backendServiceRepo,
		upstreamService:      upstreamSvc,
		gatewayEventsService: gatewayEventsService,
		devPortalService:     devPortalService,
		apiUtil:              apiUtil,
	}
}

// CreateAPI creates a new API with validation and business logic
func (s *APIService) CreateAPI(req *CreateAPIRequest, orgUUID string) (*dto.API, error) {
	// Validate request
	if err := s.validateCreateAPIRequest(req, orgUUID); err != nil {
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
	if project.OrganizationID != orgUUID {
		return nil, constants.ErrProjectNotFound
	}

	fmt.Println("Project Created")

	// Handle the API handle (user-facing identifier)
	var handle string
	if req.ID != "" {
		handle = req.ID
	} else {
		// Generate handle from API name with collision detection
		var err error
		handle, err = utils.GenerateHandle(req.Name, s.HandleExistsCheck(orgUUID))
		if err != nil {
			fmt.Println("Error generating handle:", err)
			return nil, err
		}
	}

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
	if len(req.Operations) == 0 && constants.APITypeHTTP == req.Type {
		// generate default get, post, patch and delete operations with path /*
		defaultOperations := s.generateDefaultOperations()
		req.Operations = defaultOperations
	} else if constants.APITypeWebSub == req.Type && len(req.Channels) == 0 {
		defaultChannels := s.generateDefaultChannels(req.Type)
		req.Channels = defaultChannels
	}

	fmt.Println("Channels Created")

	// Create API DTO - ID field holds the handle (user-facing identifier)
	api := &dto.API{
		ID:               handle,
		Name:             req.Name,
		Description:      req.Description,
		Context:          req.Context,
		Version:          req.Version,
		Provider:         req.Provider,
		ProjectID:        req.ProjectID,
		OrganizationID:   orgUUID,
		LifeCycleStatus:  req.LifeCycleStatus,
		HasThumbnail:     req.HasThumbnail,
		IsDefaultVersion: req.IsDefaultVersion,
		Type:             req.Type,
		Transport:        req.Transport,
		MTLS:             req.MTLS,
		Security:         req.Security,
		CORS:             req.CORS,
		BackendServices:  req.BackendServices,
		APIRateLimiting:  req.APIRateLimiting,
		Operations:       req.Operations,
		Channels:         req.Channels,
	}

	// Process backend services: check if they exist, create or update them
	var backendServiceIdList []string
	for _, backendService := range req.BackendServices {
		backendServiceId, err := s.upstreamService.UpsertBackendService(&backendService, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to process backend service '%s': %w", backendService.Name, err)
		}
		backendServiceIdList = append(backendServiceIdList, backendServiceId)
	}

	fmt.Println("Backend Services Created")

	apiModel := s.apiUtil.DTOToModel(api)
	fmt.Println("Got API Model")
	// Create API in repository (UUID is generated internally by CreateAPI)
	if err := s.apiRepo.CreateAPI(apiModel); err != nil {
		return nil, fmt.Errorf("failed to create api: %w", err)
	}

	fmt.Println("Created API in Repository: ", apiModel.ID)

	// Get the generated UUID from the model (set by CreateAPI)
	apiUUID := apiModel.ID

	api.CreatedAt = apiModel.CreatedAt
	api.UpdatedAt = apiModel.UpdatedAt

	// Associate backend services with the API (use internal UUID)
	for i, backendServiceUUID := range backendServiceIdList {
		isDefault := i == 0 // First backend service is default
		if len(req.BackendServices) > 0 && i < len(req.BackendServices) {
			// Check if isDefault was explicitly set in the request
			isDefault = req.BackendServices[i].IsDefault
		}

		if err := s.upstreamService.AssociateBackendServiceWithAPI(apiUUID, backendServiceUUID, isDefault); err != nil {
			return nil, fmt.Errorf("failed to associate backend service with API: %w", err)
		}
	}

	fmt.Println("Associated Backends")

	// Automatically create DevPortal association for default DevPortal (use internal UUID)
	if err := s.createDefaultDevPortalAssociation(apiUUID, orgUUID); err != nil {
		// Log error but don't fail API creation if default DevPortal association fails
		log.Printf("[APIService] Failed to create default DevPortal association for API %s: %v", apiUUID, err)
	}

	fmt.Println("Associated Devportal")

	return api, nil
}

// GetAPIByUUID retrieves an API by its ID
func (s *APIService) GetAPIByUUID(apiUUID, orgUUID string) (*dto.API, error) {
	if apiUUID == "" {
		return nil, errors.New("API id is required")
	}

	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgUUID {
		return nil, constants.ErrAPINotFound
	}

	api := s.apiUtil.ModelToDTO(apiModel)
	return api, nil
}

// GetAPIByHandle retrieves an API by its handle
func (s *APIService) GetAPIByHandle(handle, orgId string) (*dto.API, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.GetAPIByUUID(apiUUID, orgId)
}

// HandleExistsCheck returns a function that checks if an API handle exists in the organization.
// This is designed to be used with utils.GenerateHandle for handle generation with collision detection.
func (s *APIService) HandleExistsCheck(orgUUID string) func(string) bool {
	return func(handle string) bool {
		exists, err := s.apiRepo.CheckAPIExistsByHandleInOrganization(handle, orgUUID)
		if err != nil {
			// On error, assume it exists to be safe (will trigger retry)
			return true
		}
		return exists
	}
}

// getAPIUUIDByHandle retrieves the internal UUID for an API by its handle.
// This is a lightweight operation that only fetches minimal metadata.
func (s *APIService) getAPIUUIDByHandle(handle, orgUUID string) (string, error) {
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

// GetAPIsByOrganization retrieves all APIs for an organization with optional project filter
func (s *APIService) GetAPIsByOrganization(orgUUID string, projectUUID *string) ([]*dto.API, error) {
	// If project ID is provided, validate that it belongs to the organization
	if projectUUID != nil && *projectUUID != "" {
		project, err := s.projectRepo.GetProjectByUUID(*projectUUID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, constants.ErrProjectNotFound
		}
		if project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
	}

	apiModels, err := s.apiRepo.GetAPIsByOrganizationUUID(orgUUID, projectUUID)
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
func (s *APIService) UpdateAPI(apiUUID string, req *UpdateAPIRequest, orgUUID string) (*dto.API, error) {
	if apiUUID == "" {
		return nil, errors.New("API id is required")
	}

	// Get existing API
	existingAPIModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if existingAPIModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if existingAPIModel.OrganizationID != orgUUID {
		return nil, constants.ErrAPINotFound
	}

	// Apply updates using shared helper
	existingAPI, err := s.applyAPIUpdates(existingAPIModel, req, orgUUID)
	if err != nil {
		return nil, err
	}

	// Update API in repository
	updatedAPIModel := s.apiUtil.DTOToModel(existingAPI)
	updatedAPIModel.ID = apiUUID // Ensure UUID remains unchanged
	if err := s.apiRepo.UpdateAPI(updatedAPIModel); err != nil {
		return nil, err
	}

	if req.BackendServices != nil {
		if err := s.updateAPIBackendServices(apiUUID, req.BackendServices, orgUUID); err != nil {
			return nil, err
		}
	}

	return existingAPI, nil
}

// DeleteAPI deletes an API
func (s *APIService) DeleteAPI(apiUUID, orgUUID string) error {
	if apiUUID == "" {
		return errors.New("API id is required")
	}

	// Check if API exists
	api, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return err
	}
	if api == nil {
		return constants.ErrAPINotFound
	}
	if api.OrganizationID != orgUUID {
		return constants.ErrAPINotFound
	}

	// Delete API from repository
	if err := s.apiRepo.DeleteAPI(apiUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete api: %w", err)
	}

	return nil
}

// UpdateAPIByHandle updates an existing API identified by handle
func (s *APIService) UpdateAPIByHandle(handle string, req *UpdateAPIRequest, orgId string) (*dto.API, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.UpdateAPI(apiUUID, req, orgId)
}

// DeleteAPIByHandle deletes an API identified by handle
func (s *APIService) DeleteAPIByHandle(handle, orgId string) error {
	// Get API UUID by handle
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return err
	}

	// Delete API using existing UUID-based method
	return s.DeleteAPI(apiUUID, orgId)
}

// AddGatewaysToAPIByHandle associates multiple gateways with an API identified by handle
func (s *APIService) AddGatewaysToAPIByHandle(handle string, gatewayIds []string, orgId string) (*dto.APIGatewayListResponse, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.AddGatewaysToAPI(apiUUID, gatewayIds, orgId)
}

// GetAPIGatewaysByHandle retrieves all gateways associated with an API identified by handle
func (s *APIService) GetAPIGatewaysByHandle(handle, orgId string) (*dto.APIGatewayListResponse, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.GetAPIGateways(apiUUID, orgId)
}

// PublishAPIToDevPortalByHandle publishes an API identified by handle to a DevPortal
func (s *APIService) PublishAPIToDevPortalByHandle(handle string, req *dto.PublishToDevPortalRequest, orgID string) error {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgID)
	if err != nil {
		return err
	}
	return s.PublishAPIToDevPortal(apiUUID, req, orgID)
}

// UnpublishAPIFromDevPortalByHandle unpublishes an API identified by handle from a DevPortal
func (s *APIService) UnpublishAPIFromDevPortalByHandle(handle, devPortalUUID, orgID string) error {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgID)
	if err != nil {
		return err
	}
	return s.UnpublishAPIFromDevPortal(apiUUID, devPortalUUID, orgID)
}

// GetAPIPublicationsByHandle retrieves all DevPortals associated with an API identified by handle
func (s *APIService) GetAPIPublicationsByHandle(handle, orgID string) (*dto.APIDevPortalListResponse, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgID)
	if err != nil {
		return nil, err
	}
	return s.GetAPIPublications(apiUUID, orgID)
}

// AddGatewaysToAPI associates multiple gateways with an API
func (s *APIService) AddGatewaysToAPI(apiUUID string, gatewayIds []string, orgUUID string) (*dto.APIGatewayListResponse, error) {
	// Validate that the API exists and belongs to the organization
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgUUID {
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
		if gateway.OrganizationID != orgUUID {
			return nil, constants.ErrGatewayNotFound
		}
		validGateways = append(validGateways, gateway)
	}

	// Get existing associations to determine which are new vs existing
	existingAssociations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgUUID)
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
			if err := s.apiRepo.UpdateAPIAssociation(apiUUID, gateway.ID, constants.AssociationTypeGateway, orgUUID); err != nil {
				return nil, err
			}
		} else {
			// Create new association
			association := &model.APIAssociation{
				ApiID:           apiUUID,
				OrganizationID:  orgUUID,
				ResourceID:      gateway.ID,
				AssociationType: constants.AssociationTypeGateway,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
				return nil, err
			}
			existingGatewayIds[gateway.ID] = true
		}
	}

	// Return all gateways currently associated with the API including deployment details
	gatewayDetails, err := s.apiRepo.GetAPIGatewaysWithDetails(apiUUID, orgUUID)
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
func (s *APIService) GetAPIGateways(apiUUID, orgUUID string) (*dto.APIGatewayListResponse, error) {
	// Validate that the API exists and belongs to the organization
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgUUID {
		return nil, constants.ErrAPINotFound
	}

	// Get all gateways associated with this API including deployment details
	gatewayDetails, err := s.apiRepo.GetAPIGatewaysWithDetails(apiUUID, orgUUID)
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
		AssociationType: constants.AssociationTypeDevPortal,
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
func (s *APIService) validateCreateAPIRequest(req *CreateAPIRequest, orgUUID string) error {
	if req.ID != "" {
		// Validate user-provided handle
		if err := utils.ValidateHandle(req.ID); err != nil {
			return err
		}
		// Check if handle already exists in the organization
		handleExists, err := s.apiRepo.CheckAPIExistsByHandleInOrganization(req.ID, orgUUID)
		if err != nil {
			return err
		}
		if handleExists {
			return constants.ErrHandleExists
		}
	}
	if req.Name == "" {
		return constants.ErrInvalidAPIName
	}
	if !s.isValidContext(req.Context) {
		return constants.ErrInvalidAPIContext
	}
	if !s.isValidVersion(req.Version) {
		return constants.ErrInvalidAPIVersion
	}
	if req.ProjectID == "" {
		return errors.New("project id is required")
	}

	nameVersionExists, err := s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(req.Name, req.Version, orgUUID, "")
	if err != nil {
		return err
	}
	if nameVersionExists {
		return constants.ErrAPINameVersionAlreadyExists
	}

	// Validate lifecycle status if provided
	if req.LifeCycleStatus != "" && !constants.ValidLifecycleStates[req.LifeCycleStatus] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Type != "" && !constants.ValidAPITypes[req.Type] {
		return constants.ErrInvalidAPIType
	}

	// Type-specific validations
	// Ensure that WebSub APIs do not have operations and HTTP APIs do not have channels
	switch req.Type {
	case constants.APITypeWebSub:
		// For WebSub APIs, ensure that at least one channel is defined
		if req.Operations != nil || len(req.Operations) > 0 {
			return errors.New("WebSub APIs cannot have operations defined")
		}
	case constants.APITypeHTTP:
		// For HTTP APIs, ensure that at least one operation is defined
		if req.Channels != nil || len(req.Channels) > 0 {
			return errors.New("HTTP APIs cannot have channels defined")
		}
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

// applyAPIUpdates applies update request fields to an existing API model and handles backend services
func (s *APIService) applyAPIUpdates(existingAPIModel *model.API, req *UpdateAPIRequest, orgId string) (*dto.API, error) {
	// Validate update request
	if err := s.validateUpdateAPIRequest(existingAPIModel, req, orgId); err != nil {
		return nil, err
	}

	existingAPI := s.apiUtil.ModelToDTO(existingAPIModel)

	// Update fields (only allow certain fields to be updated)
	if req.Name != nil {
		existingAPI.Name = *req.Name
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
		existingAPI.BackendServices = *req.BackendServices
	}
	if req.APIRateLimiting != nil {
		existingAPI.APIRateLimiting = req.APIRateLimiting
	}
	if req.Operations != nil {
		existingAPI.Operations = *req.Operations
	}
	if req.Channels != nil {
		existingAPI.Channels = *req.Channels
	}
	if req.Policies != nil {
		existingAPI.Policies = *req.Policies
	}

	return existingAPI, nil
}

// updateAPIBackendServices handles backend service updates for an API
func (s *APIService) updateAPIBackendServices(apiUUID string, backendServices *[]dto.BackendService, orgId string) error {
	// Process backend services: check if they exist, create or update them
	var backendServiceUUIDs []string
	for _, backendService := range *backendServices {
		backendServiceUUID, err := s.upstreamService.UpsertBackendService(&backendService, orgId)
		if err != nil {
			return fmt.Errorf("failed to process backend service '%s': %w", backendService.Name, err)
		}
		backendServiceUUIDs = append(backendServiceUUIDs, backendServiceUUID)
	}

	// Remove existing associations
	existingBackendServices, err := s.upstreamService.GetBackendServicesByAPIID(apiUUID)
	if err != nil {
		return fmt.Errorf("failed to get existing backend services: %w", err)
	}

	for _, existingService := range existingBackendServices {
		if err := s.upstreamService.DisassociateBackendServiceFromAPI(apiUUID, existingService.ID); err != nil {
			return fmt.Errorf("failed to remove existing backend service association: %w", err)
		}
	}

	// Add new associations
	for i, backendServiceUUID := range backendServiceUUIDs {
		isDefault := i == 0
		if len(*backendServices) > 0 && i < len(*backendServices) {
			isDefault = (*backendServices)[i].IsDefault
		}

		if err := s.upstreamService.AssociateBackendServiceWithAPI(apiUUID, backendServiceUUID, isDefault); err != nil {
			return fmt.Errorf("failed to associate backend service with API: %w", err)
		}
	}

	return nil
}

// validateUpdateAPIRequest checks the validity of the update API request
func (s *APIService) validateUpdateAPIRequest(existingAPIModel *model.API, req *UpdateAPIRequest, orgUUID string) error {
	if req.Name != nil {
		nameVersionExists, err := s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(*req.Name,
			existingAPIModel.Version, orgUUID, existingAPIModel.Handle)
		if err != nil {
			return err
		}
		if nameVersionExists {
			return constants.ErrAPINameVersionAlreadyExists
		}
	}

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

func (s *APIService) isValidContext(context string) bool {
	// Context can be root path (/), or follow pattern: /name, /name1/name2, /name/1.0.0, /name/v1.2.3
	pattern := `^\/(?:[a-zA-Z0-9_-]+(?:\/(?:[a-zA-Z0-9_-]+|v?\d+(?:\.\d+)?(?:\.\d+)?))*)?\/?$`
	matched, _ := regexp.MatchString(pattern, context)
	return matched && len(context) <= 232
}

func (s *APIService) isValidVersion(version string) bool {
	// Version should follow semantic versioning or simple version format
	if version == "" {
		return false
	}
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
	ID               string                  `json:"id,omitempty"`
	Name             string                  `json:"name"`
	Description      string                  `json:"description,omitempty"`
	Context          string                  `json:"context"`
	Version          string                  `json:"version"`
	Provider         string                  `json:"provider,omitempty"`
	ProjectID        string                  `json:"projectId"`
	LifeCycleStatus  string                  `json:"lifeCycleStatus,omitempty"`
	HasThumbnail     bool                    `json:"hasThumbnail,omitempty"`
	IsDefaultVersion bool                    `json:"isDefaultVersion,omitempty"`
	Type             string                  `json:"type,omitempty"`
	Transport        []string                `json:"transport,omitempty"`
	MTLS             *dto.MTLSConfig         `json:"mtls,omitempty"`
	Security         *dto.SecurityConfig     `json:"security,omitempty"`
	CORS             *dto.CORSConfig         `json:"cors,omitempty"`
	BackendServices  []dto.BackendService    `json:"backend-services,omitempty"`
	APIRateLimiting  *dto.RateLimitingConfig `json:"api-rate-limiting,omitempty"`
	Channels         []dto.Channel           `json:"channels,omitempty"`
	Operations       []dto.Operation         `json:"operations,omitempty"`
}

// UpdateAPIRequest represents the request to update an API
type UpdateAPIRequest struct {
	Name             *string                 `json:"name,omitempty"`
	Description      *string                 `json:"description,omitempty"`
	Provider         *string                 `json:"provider,omitempty"`
	LifeCycleStatus  *string                 `json:"lifeCycleStatus,omitempty"`
	HasThumbnail     *bool                   `json:"hasThumbnail,omitempty"`
	IsDefaultVersion *bool                   `json:"isDefaultVersion,omitempty"`
	Type             *string                 `json:"type,omitempty"`
	Transport        *[]string               `json:"transport,omitempty"`
	MTLS             *dto.MTLSConfig         `json:"mtls,omitempty"`
	Security         *dto.SecurityConfig     `json:"security,omitempty"`
	CORS             *dto.CORSConfig         `json:"cors,omitempty"`
	BackendServices  *[]dto.BackendService   `json:"backend-services,omitempty"`
	APIRateLimiting  *dto.RateLimitingConfig `json:"api-rate-limiting,omitempty"`
	Operations       *[]dto.Operation        `json:"operations,omitempty"`
	Channels         *[]dto.Channel          `json:"channels,omitempty"`
	Policies         *[]dto.Policy           `json:"policies,omitempty"`
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
				Policies: []dto.Policy{},
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
				Policies: []dto.Policy{},
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
				Policies: []dto.Policy{},
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
				Policies: []dto.Policy{},
			},
		},
	}
}

// getDefaultChannels creates default PUB/SUB operations for an API
func (s *APIService) generateDefaultChannels(asyncAPIType string) []dto.Channel {
	if asyncAPIType == "WEBSUB" {
		return []dto.Channel{
			{
				Name:        "Default",
				Description: "Default SUB Channel",
				Request: &dto.ChannelRequest{
					Method: "SUB",
					Name:   "/_default",
					Authentication: &dto.AuthenticationConfig{
						Required: false,
						Scopes:   []string{},
					},
					Policies: []dto.Policy{},
				},
			},
		}
	}
	return []dto.Channel{
		{
			Name:        "Default",
			Description: "Default SUB Channel",
			Request: &dto.ChannelRequest{
				Method: "SUB",
				Name:   "/_default",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				Policies: []dto.Policy{},
			},
		},
		{
			Name:        "Default PUB Channel",
			Description: "Default PUB Channel",
			Request: &dto.ChannelRequest{
				Method: "PUB",
				Name:   "/_default",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				Policies: []dto.Policy{},
			},
		},
	}
}

// ImportAPIProject imports an API project from a Git repository
func (s *APIService) ImportAPIProject(req *dto.ImportAPIProjectRequest, orgUUID string, gitService GitService) (*dto.API, error) {
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
	apiData := s.mergeAPIData(&artifactData.Spec, &req.API)

	// 7. Create API using the existing CreateAPI flow
	createReq := &CreateAPIRequest{
		ID:               apiData.ID,
		Name:             apiData.Name,
		Description:      apiData.Description,
		Context:          apiData.Context,
		Version:          apiData.Version,
		Provider:         apiData.Provider,
		ProjectID:        apiData.ProjectID,
		LifeCycleStatus:  apiData.LifeCycleStatus,
		HasThumbnail:     apiData.HasThumbnail,
		IsDefaultVersion: apiData.IsDefaultVersion,
		Type:             apiData.Type,
		Transport:        apiData.Transport,
		MTLS:             apiData.MTLS,
		Security:         apiData.Security,
		CORS:             apiData.CORS,
		BackendServices:  apiData.BackendServices,
		APIRateLimiting:  apiData.APIRateLimiting,
		Operations:       apiData.Operations,
	}

	return s.CreateAPI(createReq, orgUUID)
}

// mergeAPIData merges WSO2 artifact data with user-provided API data (user data takes precedence)
func (s *APIService) mergeAPIData(artifact *dto.APIYAMLData, userAPIData *dto.API) *dto.API {
	apiDTO := s.apiUtil.APIYAMLDataToDTO(artifact)

	// Overwrite with user-provided data (if not empty)
	if userAPIData.ID != "" {
		apiDTO.ID = userAPIData.ID
	}
	if userAPIData.Name != "" {
		apiDTO.Name = userAPIData.Name
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
func (s *APIService) PublishAPIToDevPortal(apiID string, req *dto.PublishToDevPortalRequest, orgID string) error {
	// Get the API
	api, err := s.GetAPIByUUID(apiID, orgID)
	if err != nil {
		return err
	}

	// Publish API to DevPortal
	return s.devPortalService.PublishAPIToDevPortal(api, req, orgID)
}

// UnpublishAPIFromDevPortal unpublishes an API from a specific DevPortal
func (s *APIService) UnpublishAPIFromDevPortal(apiID, devPortalUUID, orgID string) error {
	// Unpublish API from DevPortal
	return s.devPortalService.UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID)
}

// GetAPIPublications retrieves all DevPortals associated with an API including publication details
// This mirrors the GetAPIGateways implementation for consistency
func (s *APIService) GetAPIPublications(apiUUID, orgUUID string) (*dto.APIDevPortalListResponse, error) {
	// Validate that the API exists and belongs to the organization
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgUUID {
		return nil, constants.ErrAPINotFound
	}

	// Get all DevPortals associated with this API including publication details
	devPortalDetails, err := s.publicationRepo.GetAPIDevPortalsWithDetails(apiUUID, orgUUID)
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
	if gwd.IsDeployed && gwd.DeploymentID != nil && gwd.DeployedAt != nil {
		apiGatewayResponse.Deployment = &dto.APIDeploymentDetails{
			DeploymentID: *gwd.DeploymentID,
			DeployedAt:   *gwd.DeployedAt,
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
			return nil, errors.New(strings.Join(errorList, "; "))
		}
		defer file.Close()

		content, err = io.ReadAll(file)
		if err != nil {
			errorList = append(errorList, fmt.Sprintf("failed to read OpenAPI definition file: %s", err.Error()))
			return nil, errors.New(strings.Join(errorList, "; "))
		}
	}

	// If neither URL nor file is provided
	if len(content) == 0 {
		errorList = append(errorList, "either URL or definition file must be provided")
		return nil, errors.New(strings.Join(errorList, "; "))
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
		ID:              mergedAPI.ID,
		Name:            mergedAPI.Name,
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

// ValidateAPI validates if an API with the given identifier or name+version combination exists within an organization
func (s *APIService) ValidateAPI(req *dto.APIValidationRequest, orgUUID string) (*dto.APIValidationResponse, error) {
	// Validate request - either identifier OR both name and version must be provided
	if req.Identifier == "" && (req.Name == "" || req.Version == "") {
		return nil, errors.New("either 'identifier' or both 'name' and 'version' parameters are required")
	}

	// Check if organization exists
	organization, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, err
	}
	if organization == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	var exists bool
	var validationError *dto.APIValidationError

	// Check existence based on the provided parameters
	if req.Identifier != "" {
		// Validate by identifier
		exists, err = s.apiRepo.CheckAPIExistsByHandleInOrganization(req.Identifier, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to check API existence by identifier: %w", err)
		}
		if exists {
			validationError = &dto.APIValidationError{
				Code:    "api-identifier-already-exists",
				Message: fmt.Sprintf("An API with identifier '%s' already exists in the organization.", req.Identifier),
			}
		}
	} else {
		// Validate by name and version
		exists, err = s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(req.Name, req.Version, orgUUID, "")
		if err != nil {
			return nil, fmt.Errorf("failed to check API existence by name and version: %w", err)
		}
		if exists {
			validationError = &dto.APIValidationError{
				Code: "api-name-version-already-exists",
				Message: fmt.Sprintf("The API name '%s' with version '%s' already exists in the organization.",
					req.Name, req.Version),
			}
		}
	}

	// Create response
	response := &dto.APIValidationResponse{
		Valid: !exists, // valid means the API doesn't exist (available for use)
		Error: validationError,
	}

	return response, nil
}
