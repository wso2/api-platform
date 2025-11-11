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
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"regexp"
	"strings"
	"time"

	"platform-api/src/internal/constants"

	"github.com/google/uuid"
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

	// Check if API context already exists in the project
	existingAPIs, err := s.apiRepo.GetAPIsByProjectID(req.ProjectID)
	if err != nil {
		return nil, err
	}

	for _, api := range existingAPIs {
		if api.Name == req.Name && api.Context == req.Context && api.Version == req.Version {
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

	// Convert to DTO for easier manipulation
	api := s.apiUtil.ModelToDTO(apiModel)

	// Generate API deployment YAML
	apiYAML, err := s.apiUtil.GenerateAPIDeploymentYAML(api)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API deployment YAML: %w", err)
	}

	// Process deployment requests and create deployment responses
	var deployments []*dto.APIRevisionDeployment
	currentTime := time.Now().Format(time.RFC3339)

	for _, deploymentReq := range deploymentRequests {
		// Validate deployment request
		if err := s.validateDeploymentRequest(&deploymentReq, orgId); err != nil {
			return nil, constants.ErrInvalidAPIDeployment
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

	// Log the generated YAML for debugging/monitoring purposes
	fmt.Printf("Generated API Deployment YAML for API %s:\n%s\n", apiId, apiYAML)

	return deployments, nil
}

// GetGatewaysForAPI retrieves all gateways where the specified API is deployed with pagination
func (s *APIService) GetGatewaysForAPI(apiId, orgId string) (*dto.GatewayListResponse, error) {
	// First validate that the API exists and belongs to the organization
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

	// Get all gateways where this API is deployed (without pagination for total count)
	gateways, err := s.gatewayRepo.GetGatewaysByAPIID(apiId, orgId)
	if err != nil {
		return nil, err
	}

	// Convert models to DTOs
	responses := make([]dto.GatewayResponse, 0, len(gateways))
	for _, gw := range gateways {
		responses = append(responses, dto.GatewayResponse{
			ID:                gw.ID,
			OrganizationID:    gw.OrganizationID,
			Name:              gw.Name,
			DisplayName:       gw.DisplayName,
			Description:       gw.Description,
			Vhost:             gw.Vhost,
			IsCritical:        gw.IsCritical,
			FunctionalityType: gw.FunctionalityType,
			IsActive:          gw.IsActive,
			CreatedAt:         gw.CreatedAt,
			UpdatedAt:         gw.UpdatedAt,
		})
	}

	// Create paginated response
	listResponse := &dto.GatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.Pagination{
			Total:  len(responses), // For now, total equals count (no pagination yet)
			Offset: 0,              // Starting from first item
			Limit:  len(responses), // Returning all items
		},
	}

	return listResponse, nil
}

// validateDeploymentRequest validates the deployment request
func (s *APIService) validateDeploymentRequest(req *dto.APIRevisionDeployment, orgId string) error {
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

// PublishAPIToDevPortal publishes an API to a specific DevPortal
func (s *APIService) PublishAPIToDevPortal(apiID string, req *dto.PublishToDevPortalRequest, orgID string) (*dto.PublishToDevPortalResponse, error) {
	// Get the API
	api, err := s.GetAPIByUUID(apiID, orgID)
	if err != nil {
		return nil, err
	}

	// Convert API DTO to model for DevPortal manager
	apiModel := s.apiUtil.DTOToModel(api)

	// Publish API to DevPortal
	return s.devPortalService.PublishAPIToDevPortal(req.DevPortalUUID, req.SandboxGatewayID, req.ProductionGatewayID, orgID, apiID, apiModel)
}

// UnpublishAPIFromDevPortal unpublishes an API from a specific DevPortal
func (s *APIService) UnpublishAPIFromDevPortal(apiID, devPortalUUID, orgID string) (*dto.UnpublishFromDevPortalResponse, error) {
	// TODO : Relevant logics needs to be implemented. (before unpublishing whether that api have active subscriptions in devportal)
	// Unpublish API from DevPortal
	return s.devPortalService.UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID)
}

// GetAPIPublications retrieves all publication records for a specific API with gateway details
func (s *APIService) GetAPIPublications(apiID, orgID string) ([]dto.APIPublicationInfo, error) {
	if apiID == "" {
		return nil, errors.New("API id is required")
	}

	// Verify API exists and belongs to organization
	api, err := s.GetAPIByUUID(apiID, orgID)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, constants.ErrAPINotFound
	}

	// Get all publications for this API
	publications, err := s.publicationRepo.GetByAPIUUID(apiID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publications: %w", err)
	}

	if len(publications) == 0 {
		return []dto.APIPublicationInfo{}, nil
	}

	// Collect all unique DevPortal and Gateway UUIDs to fetch in bulk
	devPortalUUIDs := make(map[string]bool)
	gatewayUUIDs := make(map[string]bool)
	for _, pub := range publications {
		devPortalUUIDs[pub.DevPortalUUID] = true
		gatewayUUIDs[pub.SandboxGatewayUUID] = true
		gatewayUUIDs[pub.ProductionGatewayUUID] = true
	}

	// Fetch all DevPortals in bulk
	devPortalMap := make(map[string]*model.DevPortal)
	for devPortalUUID := range devPortalUUIDs {
		devPortal, err := s.devPortalRepo.GetByUUID(devPortalUUID, orgID)
		if err == nil && devPortal != nil {
			devPortalMap[devPortalUUID] = devPortal
		}
	}

	// Fetch all Gateways in bulk
	gatewayMap := make(map[string]*model.Gateway)
	for gatewayUUID := range gatewayUUIDs {
		gateway, err := s.gatewayRepo.GetByUUID(gatewayUUID)
		if err == nil && gateway != nil {
			gatewayMap[gatewayUUID] = gateway
		}
	}

	// Convert to response DTOs
	var publicationInfos []dto.APIPublicationInfo
	for _, pub := range publications {
		// Get DevPortal from map
		devPortal, devPortalExists := devPortalMap[pub.DevPortalUUID]
		if !devPortalExists {
			log.Printf("[APIService] DevPortal %s not found for publication %s-%s-%s", pub.DevPortalUUID, pub.APIUUID, pub.DevPortalUUID, pub.OrganizationUUID)
			continue
		}

		// Get sandbox gateway from map
		sandboxGateway, sandboxExists := gatewayMap[pub.SandboxGatewayUUID]
		if !sandboxExists {
			log.Printf("[APIService] Sandbox gateway %s not found for publication %s-%s-%s", pub.SandboxGatewayUUID, pub.APIUUID, pub.DevPortalUUID, pub.OrganizationUUID)
			continue
		}

		// Get production gateway from map
		productionGateway, productionExists := gatewayMap[pub.ProductionGatewayUUID]
		if !productionExists {
			log.Printf("[APIService] Production gateway %s not found for publication %s-%s-%s", pub.ProductionGatewayUUID, pub.APIUUID, pub.DevPortalUUID, pub.OrganizationUUID)
			continue
		}

		// Construct endpoint URLs
		context := api.Context
		if !strings.HasPrefix(context, "/") {
			context = "/" + context
		}
		sandboxURL := fmt.Sprintf("https://%s%s", sandboxGateway.Vhost, context)
		productionURL := fmt.Sprintf("https://%s%s", productionGateway.Vhost, context)

		apiVersion := ""
		if pub.APIVersion != nil {
			apiVersion = *pub.APIVersion
		}

		publicationInfo := dto.APIPublicationInfo{
			DevPortalUUID: pub.DevPortalUUID,
			DevPortalName: devPortal.Name,
			Status:        string(pub.Status),
			SandboxEndpoint: dto.GatewayEndpointInfo{
				GatewayID:         pub.SandboxGatewayUUID,
				DisplayName:       sandboxGateway.DisplayName,
				FunctionalityType: sandboxGateway.FunctionalityType,
				Vhost:             sandboxGateway.Vhost,
			},
			ProductionEndpoint: dto.GatewayEndpointInfo{
				GatewayID:         pub.ProductionGatewayUUID,
				DisplayName:       productionGateway.DisplayName,
				FunctionalityType: productionGateway.FunctionalityType,
				Vhost:             productionGateway.Vhost,
			},
			PublicationDetails: dto.PublicationDetails{
				SandboxEndpointURL:    sandboxURL,
				ProductionEndpointURL: productionURL,
				APIVersion:            apiVersion,
			},
		}

		publicationInfos = append(publicationInfos, publicationInfo)
	}

	return publicationInfos, nil
}
