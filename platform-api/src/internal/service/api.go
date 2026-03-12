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
	"log/slog"
	"mime/multipart"
	pathpkg "path"
	"regexp"
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"gopkg.in/yaml.v3"
)

// APIService handles business logic for API operations
type APIService struct {
	apiRepo               repository.APIRepository
	projectRepo           repository.ProjectRepository
	orgRepo               repository.OrganizationRepository
	gatewayRepo           repository.GatewayRepository
	devPortalRepo         repository.DevPortalRepository
	publicationRepo       repository.APIPublicationRepository
	subscriptionPlanRepo  repository.SubscriptionPlanRepository
	gatewayEventsService  *GatewayEventsService
	devPortalService      *DevPortalService
	apiUtil               *utils.APIUtil
	slogger               *slog.Logger
}

// NewAPIService creates a new API service
func NewAPIService(apiRepo repository.APIRepository, projectRepo repository.ProjectRepository,
	orgRepo repository.OrganizationRepository, gatewayRepo repository.GatewayRepository,
	devPortalRepo repository.DevPortalRepository, publicationRepo repository.APIPublicationRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository,
	gatewayEventsService *GatewayEventsService, devPortalService *DevPortalService, apiUtil *utils.APIUtil,
	slogger *slog.Logger) *APIService {
	return &APIService{
		apiRepo:              apiRepo,
		projectRepo:          projectRepo,
		orgRepo:              orgRepo,
		gatewayRepo:          gatewayRepo,
		devPortalRepo:        devPortalRepo,
		publicationRepo:      publicationRepo,
		subscriptionPlanRepo:  subscriptionPlanRepo,
		gatewayEventsService: gatewayEventsService,
		devPortalService:     devPortalService,
		apiUtil:              apiUtil,
		slogger:              slogger,
	}
}

// CreateAPI creates a new API with validation and business logic
func (s *APIService) CreateAPI(req *api.CreateRESTAPIRequest, orgUUID string) (*api.RESTAPI, error) {
	// Validate request
	if err := s.validateCreateAPIRequest(req, orgUUID); err != nil {
		return nil, err
	}

	projectID := utils.OpenAPIUUIDToString(req.ProjectId)
	// Check if project exists
	project, err := s.projectRepo.GetProjectByUUID(projectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}
	if project.OrganizationID != orgUUID {
		return nil, constants.ErrProjectNotFound
	}

	// Handle the API handle (user-facing identifier)
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
	} else {
		// Generate handle from API name with collision detection
		var err error
		handle, err = utils.GenerateHandle(req.Name, s.HandleExistsCheck(orgUUID))
		if err != nil {
			s.slogger.Error("Failed to generate API handle", "apiName", req.Name, "error", err)
			return nil, err
		}
	}

	// Set default values if not provided
	if req.CreatedBy == nil || *req.CreatedBy == "" {
		createdBy := "admin"
		req.CreatedBy = &createdBy
	}
	if req.Kind == nil || *req.Kind == "" {
		kind := constants.RestApi
		req.Kind = &kind
	}
	if req.Transport == nil || len(*req.Transport) == 0 {
		transport := []string{"http", "https"}
		req.Transport = &transport
	}
	if req.LifeCycleStatus == nil || *req.LifeCycleStatus == "" {
		status := api.CreateRESTAPIRequestLifeCycleStatus("CREATED")
		req.LifeCycleStatus = &status
	}
	if req.Operations == nil || len(*req.Operations) == 0 {
		// generate default get, post, patch and delete operations with path /*
		defaultOperations := s.generateDefaultOperations()
		req.Operations = &defaultOperations
	}

	apiREST := s.createRequestToRESTAPI(req, handle)
	apiModel := s.apiUtil.RESTAPIToModel(apiREST, orgUUID)
	// Create API in repository (UUID is generated internally by CreateAPI)
	if err := s.apiRepo.CreateAPI(apiModel); err != nil {
		s.slogger.Error("Failed to create API in repository", "apiName", req.Name, "error", err)
		return nil, fmt.Errorf("failed to create api: %w", err)
	}

	// Get the generated UUID from the model (set by CreateAPI)
	apiUUID := apiModel.ID

	// Automatically create DevPortal association for default DevPortal (use internal UUID)
	if err := s.createDefaultDevPortalAssociation(apiUUID, orgUUID); err != nil {
		// Log error but don't fail API creation if default DevPortal association fails
		s.slogger.Error("Failed to create default DevPortal association for API", "apiUUID", apiUUID, "error", err)
	}

	return s.apiUtil.ModelToRESTAPI(apiModel)
}

// GetAPIByUUID retrieves an API by its ID
func (s *APIService) GetAPIByUUID(apiUUID, orgUUID string) (*api.RESTAPI, error) {
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

	return s.apiUtil.ModelToRESTAPI(apiModel)
}

// GetAPIByHandle retrieves an API by its handle
func (s *APIService) GetAPIByHandle(handle, orgId string) (*api.RESTAPI, error) {
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
func (s *APIService) GetAPIsByOrganization(orgUUID string, projectUUID string) ([]api.RESTAPI, error) {
	// If project ID is provided, validate that it belongs to the organization
	if projectUUID != "" {
		project, err := s.projectRepo.GetProjectByUUID(projectUUID)
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

	apis := make([]api.RESTAPI, 0)
	for _, apiModel := range apiModels {
		apiResponse, err := s.apiUtil.ModelToRESTAPI(apiModel)
		if err != nil {
			return nil, err
		}
		if apiResponse != nil {
			apis = append(apis, *apiResponse)
		}
	}
	return apis, nil
}

// UpdateAPI updates an existing API
func (s *APIService) UpdateAPI(apiUUID string, req *api.UpdateRESTAPIRequest, orgUUID string) (*api.RESTAPI, error) {
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
	updatedAPI, err := s.applyAPIUpdates(existingAPIModel, req, orgUUID)
	if err != nil {
		return nil, err
	}

	// Update API in repository
	updatedAPIModel := s.apiUtil.RESTAPIToModel(updatedAPI, orgUUID)
	updatedAPIModel.ID = apiUUID // Ensure UUID remains unchanged
	if err := s.apiRepo.UpdateAPI(updatedAPIModel); err != nil {
		return nil, err
	}

	return s.apiUtil.ModelToRESTAPI(updatedAPIModel)
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

	// Get all gateway associations BEFORE deletion (associations will be cascade deleted)
	gatewayAssociations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get gateway associations for api deletion: %w", err)
	}

	// Delete API from repository (this also deletes associations)
	if err := s.apiRepo.DeleteAPI(apiUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete api: %w", err)
	}

	// Send deletion events to all associated gateways
	if s.gatewayEventsService != nil && gatewayAssociations != nil {
		for _, assoc := range gatewayAssociations {
			// Get gateway details to retrieve vhost
			gateway, err := s.gatewayRepo.GetByUUID(assoc.ResourceID)
			if err != nil {
				s.slogger.Warn("Failed to get gateway for deletion event", "gatewayID", assoc.ResourceID, "error", err)
				continue
			}
			if gateway == nil {
				s.slogger.Warn("Gateway not found for deletion event", "gatewayID", assoc.ResourceID)
				continue
			}

			// Create and send API deletion event
			deletionEvent := &model.APIDeletionEvent{
				ApiId: apiUUID,
				Vhost: gateway.Vhost,
			}

			if err := s.gatewayEventsService.BroadcastAPIDeletionEvent(assoc.ResourceID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast API deletion event", "gatewayID", assoc.ResourceID, "apiUUID", apiUUID, "error", err)
			} else {
				s.slogger.Info("API deletion event sent", "gatewayID", assoc.ResourceID, "apiUUID", apiUUID, "vhost", gateway.Vhost)
			}
		}
	}

	return nil
}

// UpdateAPIByHandle updates an existing API identified by handle
func (s *APIService) UpdateAPIByHandle(handle string, req *api.UpdateRESTAPIRequest, orgId string) (*api.RESTAPI, error) {
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
func (s *APIService) AddGatewaysToAPIByHandle(handle string, gatewayIds []string, orgId string) (*api.RESTAPIGatewayListResponse, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.AddGatewaysToAPI(apiUUID, gatewayIds, orgId)
}

// GetAPIGatewaysByHandle retrieves all gateways associated with an API identified by handle
func (s *APIService) GetAPIGatewaysByHandle(handle, orgId string) (*api.RESTAPIGatewayListResponse, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.GetAPIGateways(apiUUID, orgId)
}

// PublishAPIToDevPortalByHandle publishes an API identified by handle to a DevPortal
func (s *APIService) PublishAPIToDevPortalByHandle(handle string, req *api.PublishToDevPortalRequest, orgID string) error {
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
func (s *APIService) GetAPIPublicationsByHandle(handle, orgID string) (*api.RESTAPIDevPortalListResponse, error) {
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgID)
	if err != nil {
		return nil, err
	}
	return s.GetAPIPublications(apiUUID, orgID)
}

// AddGatewaysToAPI associates multiple gateways with an API
func (s *APIService) AddGatewaysToAPI(apiUUID string, gatewayIds []string, orgUUID string) (*api.RESTAPIGatewayListResponse, error) {
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
				ArtifactID:      apiUUID,
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
	return s.GetAPIGateways(apiUUID, orgUUID)
}

// GetAPIGateways retrieves all gateways associated with an API including deployment details
func (s *APIService) GetAPIGateways(apiUUID, orgUUID string) (*api.RESTAPIGatewayListResponse, error) {
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

	response, err := apiGatewayDetailsToAPIList(gatewayDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to convert API gateway details: %w", err)
	}

	return response, nil
}

// createDefaultDevPortalAssociation creates an association between the API and the default DevPortal
func (s *APIService) createDefaultDevPortalAssociation(apiId, orgId string) error {
	// Get default DevPortal for the organization
	defaultDevPortal, err := s.devPortalRepo.GetDefaultByOrganizationUUID(orgId)
	if err != nil {
		// If no default DevPortal exists, skip association (not an error)
		if errors.Is(err, constants.ErrDevPortalNotFound) {
			s.slogger.Info("No default DevPortal found for organization, skipping association", "orgId", orgId)
			return nil
		}
		return fmt.Errorf("failed to get default DevPortal: %w", err)
	}

	// Create API-DevPortal association
	association := &model.APIAssociation{
		ArtifactID:      apiId,
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
			s.slogger.Info("API association with default DevPortal already exists", "apiId", apiId)
			return nil
		}
		return fmt.Errorf("failed to create API-DevPortal association: %w", err)
	}

	s.slogger.Info("Successfully created association between API and default DevPortal", "apiId", apiId, "devPortalUUID", defaultDevPortal.UUID)
	return nil
}

// Validation methods

// validateCreateAPIRequest checks the validity of the create API request
func (s *APIService) validateCreateAPIRequest(req *api.CreateRESTAPIRequest, orgUUID string) error {
	if req.Id != nil && *req.Id != "" {
		// Validate user-provided handle
		if err := utils.ValidateHandle(*req.Id); err != nil {
			return err
		}
		// Check if handle already exists in the organization
		handleExists, err := s.apiRepo.CheckAPIExistsByHandleInOrganization(*req.Id, orgUUID)
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
	if req.ProjectId == (openapi_types.UUID{}) {
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
	if req.LifeCycleStatus != nil && !constants.ValidLifecycleStates[string(*req.LifeCycleStatus)] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Kind != nil && !strings.EqualFold(*req.Kind, constants.RestApi) {
		return constants.ErrInvalidAPIType
	}

	// Type-specific validations
	// Ensure that WebSub APIs do not have operations and HTTP APIs do not have channels
	apiKind := constants.RestApi
	if req.Kind != nil {
		apiKind = *req.Kind
	}
	switch apiKind {
	case constants.APITypeWebSub:
		// For WebSub APIs, ensure that at least one channel is defined
		if req.Operations != nil && len(*req.Operations) > 0 {
			return errors.New("WebSub APIs cannot have operations defined")
		}
	case constants.APITypeHTTP:
		// For HTTP APIs, ensure that at least one operation is defined
		if req.Channels != nil && len(*req.Channels) > 0 {
			return errors.New("HTTP APIs cannot have channels defined")
		}
	}

	// Validate transport protocols if provided
	if req.Transport != nil && len(*req.Transport) > 0 {
		for _, transport := range *req.Transport {
			if !constants.ValidTransports[strings.ToLower(transport)] {
				return constants.ErrInvalidTransport
			}
		}
	}

	// Validate subscription plans if provided
	if err := s.validateSubscriptionPlans(req.SubscriptionPlans, orgUUID); err != nil {
		return err
	}

	return nil
}

// validateSubscriptionPlans ensures each plan name exists in the organization and is ACTIVE.
// It normalizes planNames in place by trimming each element. Repository errors are returned
// directly; ErrSubscriptionPlanNotFoundOrInactive is only returned when plan is nil or inactive.
func (s *APIService) validateSubscriptionPlans(planNames *[]string, orgUUID string) error {
	if planNames == nil || len(*planNames) == 0 {
		return nil
	}
	for i := range *planNames {
		(*planNames)[i] = strings.TrimSpace((*planNames)[i])
		name := (*planNames)[i]
		if name == "" {
			continue
		}
		plan, err := s.subscriptionPlanRepo.GetByNameAndOrg(name, orgUUID)
		if err != nil {
			return err
		}
		if plan == nil || plan.Status != model.SubscriptionPlanStatusActive {
			return fmt.Errorf("%w: plan %q", constants.ErrSubscriptionPlanNotFoundOrInactive, name)
		}
	}
	return nil
}

// applyAPIUpdates applies update request fields to an existing API model and handles backend services
func (s *APIService) applyAPIUpdates(existingAPIModel *model.API, req *api.UpdateRESTAPIRequest, orgId string) (*api.RESTAPI, error) {
	// Validate update request
	if err := s.validateUpdateAPIRequest(existingAPIModel, req, orgId); err != nil {
		return nil, err
	}

	existingAPI, err := s.apiUtil.ModelToRESTAPI(existingAPIModel)
	if err != nil {
		return nil, err
	}

	// Update fields (only allow certain fields to be updated)
	if req.Name != "" {
		existingAPI.Name = req.Name
	}
	if req.Description != nil {
		existingAPI.Description = req.Description
	}
	if req.CreatedBy != nil {
		existingAPI.CreatedBy = req.CreatedBy
	}
	if req.LifeCycleStatus != nil {
		existingAPI.LifeCycleStatus = req.LifeCycleStatus
	}
	if req.Transport != nil {
		existingAPI.Transport = req.Transport
	}
	if req.Operations != nil {
		existingAPI.Operations = req.Operations
	}
	if req.Channels != nil {
		existingAPI.Channels = req.Channels
	}
	if req.Policies != nil {
		existingAPI.Policies = req.Policies
	}
	if req.SubscriptionPlans != nil {
		existingAPI.SubscriptionPlans = req.SubscriptionPlans
	}
	if !s.isEmptyUpstream(req.Upstream) {
		existingAPI.Upstream = req.Upstream
	}

	return existingAPI, nil
}

// validateUpdateAPIRequest checks the validity of the update API request
func (s *APIService) validateUpdateAPIRequest(existingAPIModel *model.API, req *api.UpdateRESTAPIRequest, orgUUID string) error {
	if req.Name != "" {
		nameVersionExists, err := s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(req.Name,
			existingAPIModel.Version, orgUUID, existingAPIModel.Handle)
		if err != nil {
			return err
		}
		if nameVersionExists {
			return constants.ErrAPINameVersionAlreadyExists
		}
	}

	// Validate lifecycle status if provided
	if req.LifeCycleStatus != nil && !constants.ValidLifecycleStates[string(*req.LifeCycleStatus)] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Kind != nil && !strings.EqualFold(*req.Kind, constants.RestApi) {
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

	// Validate subscription plans if provided
	if err := s.validateSubscriptionPlans(req.SubscriptionPlans, orgUUID); err != nil {
		return err
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

// generateDefaultOperations creates default CRUD operations for an API
func (s *APIService) generateDefaultOperations() []api.Operation {
	return []api.Operation{
		{
			Name:        utils.StringPtrIfNotEmpty("Get Resource"),
			Description: utils.StringPtrIfNotEmpty("Retrieve all resources"),
			Request: api.OperationRequest{
				Method:   api.OperationRequestMethodGET,
				Path:     "/*",
				Policies: &[]api.Policy{},
			},
		},
		{
			Name:        utils.StringPtrIfNotEmpty("POST Resource"),
			Description: utils.StringPtrIfNotEmpty("Create a new resource"),
			Request: api.OperationRequest{
				Method:   api.OperationRequestMethodPOST,
				Path:     "/*",
				Policies: &[]api.Policy{},
			},
		},
		{
			Name:        utils.StringPtrIfNotEmpty("Update Resource"),
			Description: utils.StringPtrIfNotEmpty("Update an existing resource"),
			Request: api.OperationRequest{
				Method:   api.OperationRequestMethodPATCH,
				Path:     "/*",
				Policies: &[]api.Policy{},
			},
		},
		{
			Name:        utils.StringPtrIfNotEmpty("Delete Resource"),
			Description: utils.StringPtrIfNotEmpty("Delete an existing resource"),
			Request: api.OperationRequest{
				Method:   api.OperationRequestMethodDELETE,
				Path:     "/*",
				Policies: &[]api.Policy{},
			},
		},
	}
}

// getDefaultChannels creates default PUB/SUB operations for an API
func (s *APIService) generateDefaultChannels(asyncAPIType *string) []api.Channel {
	if asyncAPIType != nil && *asyncAPIType == constants.APITypeWebSub {
		return []api.Channel{
			{
				Name:        utils.StringPtrIfNotEmpty("Default"),
				Description: utils.StringPtrIfNotEmpty("Default SUB Channel"),
				Request: api.ChannelRequest{
					Method:   api.SUB,
					Name:     "/_default",
					Policies: &[]api.Policy{},
				},
			},
		}
	}
	return []api.Channel{
		{
			Name:        utils.StringPtrIfNotEmpty("Default"),
			Description: utils.StringPtrIfNotEmpty("Default SUB Channel"),
			Request: api.ChannelRequest{
				Method:   api.SUB,
				Name:     "/_default",
				Policies: &[]api.Policy{},
			},
		},
		{
			Name:        utils.StringPtrIfNotEmpty("Default PUB Channel"),
			Description: utils.StringPtrIfNotEmpty("Default PUB Channel"),
			Request: api.ChannelRequest{
				Method:   api.ChannelRequestMethod("PUB"),
				Name:     "/_default",
				Policies: &[]api.Policy{},
			},
		},
	}
}

// ImportAPIProject imports an API project from a Git repository
func (s *APIService) ImportAPIProject(req *api.ImportAPIProjectRequest, orgUUID string, gitService GitService) (*api.RESTAPI, error) {
	// 1. Validate if there is a .api-platform directory with config.yaml
	config, err := gitService.ValidateAPIProject(req.RepoUrl, req.Branch, req.Path)
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
	artifactData, err := gitService.FetchWSO2Artifact(req.RepoUrl, req.Branch, wso2ArtifactPath)
	if err != nil {
		return nil, constants.ErrWSO2ArtifactNotFound
	}

	createReq := s.createRequestFromAPIYAMLData(&artifactData.Spec)
	// Overwrite scalar fields from user input (user data takes precedence)
	createReq.Name = req.Api.Name
	createReq.Context = req.Api.Context
	createReq.Version = req.Api.Version
	createReq.ProjectId = req.Api.ProjectId
	// Optional overrides
	if req.Api.Id != nil && *req.Api.Id != "" {
		createReq.Id = req.Api.Id
	}
	if req.Api.Description != nil && *req.Api.Description != "" {
		createReq.Description = req.Api.Description
	}
	if req.Api.CreatedBy != nil && *req.Api.CreatedBy != "" {
		createReq.CreatedBy = req.Api.CreatedBy
	}
	if req.Api.Kind != nil && *req.Api.Kind != "" {
		createReq.Kind = req.Api.Kind
	}
	if req.Api.LifeCycleStatus != nil && string(*req.Api.LifeCycleStatus) != "" {
		status := api.CreateRESTAPIRequestLifeCycleStatus(*req.Api.LifeCycleStatus)
		createReq.LifeCycleStatus = &status
	}
	if req.Api.Transport != nil && len(*req.Api.Transport) > 0 {
		createReq.Transport = req.Api.Transport
	}

	// Fallback: if YAML doesn't provide upstream/operations and user input does, use user input.
	if s.isEmptyUpstream(createReq.Upstream) && !s.isEmptyUpstream(req.Api.Upstream) {
		createReq.Upstream = req.Api.Upstream
	}
	if (createReq.Operations == nil || len(*createReq.Operations) == 0) && req.Api.Operations != nil && len(*req.Api.Operations) > 0 {
		ops := *req.Api.Operations
		createReq.Operations = &ops
	}
	if (createReq.Policies == nil || len(*createReq.Policies) == 0) && req.Api.Policies != nil && len(*req.Api.Policies) > 0 {
		policies := *req.Api.Policies
		createReq.Policies = &policies
	}

	return s.CreateAPI(createReq, orgUUID)
}

// ValidateAndRetrieveAPIProject validates an API project from Git repository with comprehensive checks
func (s *APIService) ValidateAndRetrieveAPIProject(req *api.ValidateAPIProjectRequest,
	gitService GitService) (*api.RESTAPIProjectValidationResponse, error) {
	errorsList := make([]string, 0)
	response := &api.RESTAPIProjectValidationResponse{
		IsRESTAPIProjectValid:    false,
		IsAPIConfigValid:         false,
		IsRESTAPIDefinitionValid: false,
		Errors:                   nil,
		Api:                      nil,
	}

	// Step 1: Check if .api-platform directory exists and validate config
	config, err := gitService.ValidateAPIProject(req.RepoUrl, req.Branch, req.Path)
	if err != nil {
		errorsList = append(errorsList, err.Error())
		response.Errors = &errorsList
		return response, nil
	}

	// Process the first API entry (assuming single API per project for now)
	apiEntry := config.APIs[0]

	// Step 3: Fetch OpenAPI definition
	openAPIClean := pathpkg.Clean(apiEntry.OpenAPI)
	openAPIPath := pathpkg.Join(req.Path, openAPIClean)
	openAPIContent, err := gitService.FetchFileContent(req.RepoUrl, req.Branch, openAPIPath)
	if err != nil {
		errorsList = append(errorsList, fmt.Sprintf("failed to fetch OpenAPI file: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	// Basic OpenAPI validation (check if it's valid YAML/JSON with required fields)
	if err := s.apiUtil.ValidateOpenAPIDefinition(openAPIContent); err != nil {
		errorsList = append(errorsList, fmt.Sprintf("invalid OpenAPI definition: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	response.IsRESTAPIDefinitionValid = true

	// Step 4: Fetch WSO2 artifact (api.yaml)
	wso2ArtifactClean := pathpkg.Clean(apiEntry.WSO2Artifact)
	wso2ArtifactPath := pathpkg.Join(req.Path, wso2ArtifactClean)
	wso2ArtifactContent, err := gitService.FetchFileContent(req.RepoUrl, req.Branch, wso2ArtifactPath)
	if err != nil {
		errorsList = append(errorsList, fmt.Sprintf("failed to fetch WSO2 artifact file: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	var wso2Artifact dto.APIDeploymentYAML
	if err := yaml.Unmarshal(wso2ArtifactContent, &wso2Artifact); err != nil {
		errorsList = append(errorsList, fmt.Sprintf("invalid WSO2 artifact format: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	// Step 5: Validate WSO2 artifact structure
	if err := s.apiUtil.ValidateWSO2Artifact(&wso2Artifact); err != nil {
		errorsList = append(errorsList, fmt.Sprintf("invalid WSO2 artifact: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	response.IsAPIConfigValid = true

	// Step 6: Check if OpenAPI and WSO2 artifact match (optional validation)
	if err := s.apiUtil.ValidateAPIDefinitionConsistency(openAPIContent, &wso2Artifact); err != nil {
		errorsList = append(errorsList, fmt.Sprintf("API definitions mismatch: %s", err.Error()))
		response.Errors = &errorsList
		response.IsRESTAPIProjectValid = false
		return response, nil
	}

	// Step 7: If all validations pass, convert to API DTO
	artifactREST := s.restAPIFromAPIYAMLData(&wso2Artifact.Spec)
	response.Api = s.restAPIToProjectValidationAPI(artifactREST)
	response.IsRESTAPIProjectValid = response.IsAPIConfigValid && response.IsRESTAPIDefinitionValid
	if len(errorsList) > 0 {
		response.Errors = &errorsList
	}

	return response, nil
}

// PublishAPIToDevPortal publishes an API to a specific DevPortal
func (s *APIService) PublishAPIToDevPortal(apiID string, req *api.PublishToDevPortalRequest, orgID string) error {
	// Get the API
	apiREST, err := s.GetAPIByUUID(apiID, orgID)
	if err != nil {
		return err
	}

	// Publish API to DevPortal
	return s.devPortalService.PublishAPIToDevPortal(apiID, apiREST, req, orgID)
}

// UnpublishAPIFromDevPortal unpublishes an API from a specific DevPortal
func (s *APIService) UnpublishAPIFromDevPortal(apiID, devPortalUUID, orgID string) error {
	// Unpublish API from DevPortal
	return s.devPortalService.UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID)
}

// GetAPIPublications retrieves all DevPortals associated with an API including publication details
// This mirrors the GetAPIGateways implementation for consistency
func (s *APIService) GetAPIPublications(apiUUID, orgUUID string) (*api.RESTAPIDevPortalListResponse, error) {
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

	// Convert models to API types with publication details
	responses := make([]api.RESTAPIDevPortalResponse, 0, len(devPortalDetails))
	for _, dpd := range devPortalDetails {
		response, err := s.convertToAPIDevPortalResponse(dpd)
		if err != nil {
			return nil, fmt.Errorf("failed to convert API DevPortal response: %w", err)
		}
		responses = append(responses, response)
	}

	// Create paginated response
	listResponse := &api.RESTAPIDevPortalListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: api.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}

	return listResponse, nil
}

// convertToAPIDevPortalResponse converts APIDevPortalWithDetails to RESTAPIDevPortalResponse
func (s *APIService) convertToAPIDevPortalResponse(dpd *model.APIDevPortalWithDetails) (api.RESTAPIDevPortalResponse, error) {
	// Parse UUIDs
	orgUUID, err := uuid.Parse(dpd.OrganizationUUID)
	if err != nil {
		return api.RESTAPIDevPortalResponse{}, fmt.Errorf("failed to parse OrganizationUUID: %w", err)
	}
	portalUUID, err := uuid.Parse(dpd.UUID)
	if err != nil {
		return api.RESTAPIDevPortalResponse{}, fmt.Errorf("failed to parse UUID: %w", err)
	}
	visibility := api.RESTAPIDevPortalResponseVisibility(dpd.Visibility)

	// Create the API DevPortal response
	apiDevPortalResponse := api.RESTAPIDevPortalResponse{
		ApiUrl:           dpd.APIUrl,
		AssociatedAt:     dpd.AssociatedAt,
		CreatedAt:        dpd.CreatedAt,
		Description:      utils.StringPtrIfNotEmpty(dpd.Description),
		Hostname:         dpd.Hostname,
		Identifier:       dpd.Identifier,
		IsActive:         dpd.IsActive,
		IsDefault:        dpd.IsDefault,
		IsEnabled:        dpd.IsEnabled,
		IsPublished:      dpd.IsPublished,
		Name:             dpd.Name,
		OrganizationUuid: orgUUID,
		UiUrl:            fmt.Sprintf("%s/%s/views/default/apis", dpd.APIUrl, dpd.Identifier),
		UpdatedAt:        dpd.UpdatedAt,
		Uuid:             portalUUID,
		Visibility:       visibility,
	}

	// Add publication details if published
	if dpd.IsPublished && dpd.PublishedAt != nil {
		status := api.RESTAPIPublicationDetailsStatusPublished
		if dpd.PublicationStatus != nil && *dpd.PublicationStatus != "" {
			status = api.RESTAPIPublicationDetailsStatus(*dpd.PublicationStatus)
		}

		publicationDetails := api.RESTAPIPublicationDetails{
			Status:      status,
			PublishedAt: *dpd.PublishedAt,
		}

		if dpd.APIVersion != nil {
			publicationDetails.ApiVersion = dpd.APIVersion
		}
		if dpd.DevPortalRefID != nil {
			publicationDetails.DevPortalRefId = dpd.DevPortalRefID
		}
		if dpd.SandboxEndpointURL != nil {
			publicationDetails.SandboxEndpoint = dpd.SandboxEndpointURL
		}
		if dpd.ProductionEndpointURL != nil {
			publicationDetails.ProductionEndpoint = dpd.ProductionEndpointURL
		}

		apiDevPortalResponse.Publication = &publicationDetails
	}

	return apiDevPortalResponse, nil
}

// ValidateOpenAPIDefinition validates an OpenAPI definition from multipart form data
func (s *APIService) ValidateOpenAPIDefinition(url *string, definition *multipart.FileHeader) (*api.OpenAPIValidationResponse, error) {
	errorsList := make([]string, 0)
	response := &api.OpenAPIValidationResponse{
		IsRESTAPIDefinitionValid: false,
		Errors:                   nil,
		Api:                      nil,
	}

	var content []byte
	var err error

	// If URL is provided, fetch content from URL
	if url != nil && *url != "" {
		content, err = s.apiUtil.FetchOpenAPIFromURL(*url)
		if err != nil {
			content = make([]byte, 0)
			errorsList = append(errorsList, fmt.Sprintf("failed to fetch OpenAPI from URL: %s", err.Error()))
		}
	}

	// If definition file is provided, read from file
	if definition != nil {
		file, err := definition.Open()
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("failed to open definition file: %s", err.Error()))
			response.Errors = &errorsList
			return response, nil
		}
		defer file.Close()

		content, err = io.ReadAll(file)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("failed to read definition file: %s", err.Error()))
			response.Errors = &errorsList
			return response, nil
		}
	}

	// If neither URL nor file is provided
	if len(content) == 0 {
		errorsList = append(errorsList, "either URL or definition file must be provided")
		response.Errors = &errorsList
		return response, nil
	}

	// Validate the OpenAPI definition
	if err := s.apiUtil.ValidateOpenAPIDefinition(content); err != nil {
		errorsList = append(errorsList, fmt.Sprintf("invalid OpenAPI definition: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	// Parse API specification to extract metadata directly into API DTO using libopenapi
	parsed, err := s.apiUtil.ParseAPIDefinitionToRESTAPI(content)
	if err != nil {
		errorsList = append(errorsList, fmt.Sprintf("failed to parse API specification: %s", err.Error()))
		response.Errors = &errorsList
		return response, nil
	}

	// Set the parsed API for response
	response.IsRESTAPIDefinitionValid = true
	response.Api = s.restAPIToOpenAPIValidationAPI(parsed)
	if len(errorsList) > 0 {
		response.Errors = &errorsList
	}

	return response, nil
}

// ImportFromOpenAPI imports an API from an OpenAPI definition
func (s *APIService) ImportFromOpenAPI(userAPI *api.RESTAPI, url *string, definition *multipart.FileHeader, orgId string) (*api.RESTAPI, error) {
	var content []byte
	var err error
	var errorList []string

	// If URL is provided, fetch content from URL
	if url != nil && *url != "" {
		content, err = s.apiUtil.FetchOpenAPIFromURL(*url)
		if err != nil {
			content = make([]byte, 0)
			errorList = append(errorList, fmt.Sprintf("failed to fetch OpenAPI from URL: %s", err.Error()))
		}
	}

	// If definition file is provided, read from file
	if definition != nil {
		file, err := definition.Open()
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
	apiDetails, err := s.apiUtil.ValidateAndParseOpenAPIToRESTAPI(content)
	if err != nil {
		return nil, fmt.Errorf("failed to validate and parse OpenAPI definition: %w", err)
	}

	// Merge provided API details with extracted details from OpenAPI
	mergedAPI := s.apiUtil.MergeRESTAPIDetails(userAPI, apiDetails)
	if mergedAPI == nil {
		return nil, errors.New("failed to merge API details")
	}

	// Create API using existing CreateAPI logic
	createReq := s.restAPIToCreateRequest(mergedAPI)

	err = s.validateCreateAPIRequest(createReq, orgId)
	if err != nil {
		return nil, fmt.Errorf("validation failed for merged API details: %w", err)
	}

	// Create the API
	return s.CreateAPI(createReq, orgId)
}

// ValidateAPI validates if an API with the given identifier or name+version combination exists within an organization
func (s *APIService) ValidateAPI(params *api.ValidateRESTAPIParams, orgUUID string) (*api.RESTAPIValidationResponse, error) {
	var identifier, name, version string
	if params != nil {
		if params.Identifier != nil {
			identifier = string(*params.Identifier)
		}
		if params.Name != nil {
			name = string(*params.Name)
		}
		if params.Version != nil {
			version = string(*params.Version)
		}
	}

	// Validate request - either identifier OR both name and version must be provided
	if identifier == "" && (name == "" || version == "") {
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
	var validationError *struct {
		Code    string `json:"code" yaml:"code"`
		Message string `json:"message" yaml:"message"`
	}

	// Check existence based on the provided parameters
	if identifier != "" {
		// Validate by identifier
		exists, err = s.apiRepo.CheckAPIExistsByHandleInOrganization(identifier, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to check API existence by identifier: %w", err)
		}
		if exists {
			validationError = &struct {
				Code    string `json:"code" yaml:"code"`
				Message string `json:"message" yaml:"message"`
			}{
				Code:    "api-identifier-already-exists",
				Message: fmt.Sprintf("An API with identifier '%s' already exists in the organization.", identifier),
			}
		}
	} else {
		// Validate by name and version
		exists, err = s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, "")
		if err != nil {
			return nil, fmt.Errorf("failed to check API existence by name and version: %w", err)
		}
		if exists {
			validationError = &struct {
				Code    string `json:"code" yaml:"code"`
				Message string `json:"message" yaml:"message"`
			}{
				Code: "api-name-version-already-exists",
				Message: fmt.Sprintf("The API name '%s' with version '%s' already exists in the organization.",
					name, version),
			}
		}
	}

	// Create response
	return &api.RESTAPIValidationResponse{
		Valid: !exists,
		Error: validationError,
	}, nil
}

func (s *APIService) isEmptyUpstream(upstream api.Upstream) bool {
	if !s.isEmptyUpstreamDefinition(upstream.Main) {
		return false
	}
	if upstream.Sandbox != nil && !s.isEmptyUpstreamDefinition(*upstream.Sandbox) {
		return false
	}
	return true
}

func (s *APIService) isEmptyUpstreamDefinition(definition api.UpstreamDefinition) bool {
	if definition.Url != nil && *definition.Url != "" {
		return false
	}
	if definition.Ref != nil && *definition.Ref != "" {
		return false
	}
	return true
}

func (s *APIService) createRequestToRESTAPI(req *api.CreateRESTAPIRequest, handle string) *api.RESTAPI {
	if req == nil {
		return nil
	}

	// Convert create-request lifecycle to RESTAPI lifecycle enum
	var lifecycle *api.RESTAPILifeCycleStatus
	if req.LifeCycleStatus != nil {
		v := api.RESTAPILifeCycleStatus(*req.LifeCycleStatus)
		lifecycle = &v
	}

	return &api.RESTAPI{
		Channels:          req.Channels,
		Context:           req.Context,
		CreatedBy:         req.CreatedBy,
		Description:       req.Description,
		Id:                utils.StringPtrIfNotEmpty(handle),
		Kind:              req.Kind,
		LifeCycleStatus:   lifecycle,
		Name:              req.Name,
		Operations:        req.Operations,
		Policies:          req.Policies,
		ProjectId:         req.ProjectId,
		SubscriptionPlans: req.SubscriptionPlans,
		Transport:         req.Transport,
		Upstream:          req.Upstream,
		Version:           req.Version,
	}
}

func (s *APIService) restAPIToCreateRequest(rest *api.RESTAPI) *api.CreateRESTAPIRequest {
	if rest == nil {
		return nil
	}

	req := &api.CreateRESTAPIRequest{
		Name:      rest.Name,
		Context:   rest.Context,
		Version:   rest.Version,
		ProjectId: rest.ProjectId,
		Upstream:  rest.Upstream,
	}

	if rest.Id != nil && *rest.Id != "" {
		req.Id = rest.Id
	}
	if rest.Description != nil && *rest.Description != "" {
		req.Description = rest.Description
	}
	if rest.CreatedBy != nil && *rest.CreatedBy != "" {
		req.CreatedBy = rest.CreatedBy
	}
	if rest.Kind != nil && *rest.Kind != "" {
		req.Kind = rest.Kind
	}
	if rest.LifeCycleStatus != nil && string(*rest.LifeCycleStatus) != "" {
		status := api.CreateRESTAPIRequestLifeCycleStatus(*rest.LifeCycleStatus)
		req.LifeCycleStatus = &status
	}
	if rest.Transport != nil && len(*rest.Transport) > 0 {
		req.Transport = rest.Transport
	}
	if rest.Operations != nil && len(*rest.Operations) > 0 {
		req.Operations = rest.Operations
	}
	if rest.Channels != nil && len(*rest.Channels) > 0 {
		req.Channels = rest.Channels
	}
	if rest.Policies != nil && len(*rest.Policies) > 0 {
		req.Policies = rest.Policies
	}
	if rest.SubscriptionPlans != nil && len(*rest.SubscriptionPlans) > 0 {
		req.SubscriptionPlans = rest.SubscriptionPlans
	}

	return req
}

func (s *APIService) createRequestFromAPIYAMLData(yamlData *dto.APIYAMLData) *api.CreateRESTAPIRequest {
	// NOTE: project ID must be set by caller.
	req := &api.CreateRESTAPIRequest{
		Name:      "",
		Context:   "",
		Version:   "",
		ProjectId: openapi_types.UUID{},
		Upstream:  api.Upstream{},
	}
	if yamlData == nil {
		return req
	}

	req.Name = yamlData.DisplayName
	req.Context = yamlData.Context
	req.Version = yamlData.Version

	// Upstream
	if yamlData.Upstream != nil {
		req.Upstream = s.upstreamFromYAML(yamlData.Upstream)
	}

	// Policies
	if len(yamlData.Policies) > 0 {
		policies := make([]api.Policy, len(yamlData.Policies))
		for i, p := range yamlData.Policies {
			policies[i] = api.Policy{
				ExecutionCondition: p.ExecutionCondition,
				Name:               p.Name,
				Params:             p.Params,
				Version:            p.Version,
			}
		}
		req.Policies = &policies
	}

	// SubscriptionPlans
	if len(yamlData.SubscriptionPlans) > 0 {
		req.SubscriptionPlans = &yamlData.SubscriptionPlans
	}

	// Operations
	if len(yamlData.Operations) > 0 {
		ops := make([]api.Operation, len(yamlData.Operations))
		for i, op := range yamlData.Operations {
			// Keep behavior consistent with previous DTO mapping.
			name := fmt.Sprintf("Operation-%d", i+1)
			description := fmt.Sprintf("Operation for %s %s", op.Method, op.Path)
			policies := op.Policies
			if policies == nil {
				policies = &[]api.Policy{}
			}
			ops[i] = api.Operation{
				Name:        utils.StringPtrIfNotEmpty(name),
				Description: utils.StringPtrIfNotEmpty(description),
				Request: api.OperationRequest{
					Method:   api.OperationRequestMethod(op.Method),
					Path:     op.Path,
					Policies: policies,
				},
			}
		}
		req.Operations = &ops
	}

	return req
}

func (s *APIService) restAPIFromAPIYAMLData(yamlData *dto.APIYAMLData) *api.RESTAPI {
	// Convert YAML to a RESTAPI shape suitable for validation responses.
	createReq := s.createRequestFromAPIYAMLData(yamlData)
	// Use empty handle and empty project ID for validation endpoints.
	createReq.ProjectId = openapi_types.UUID{}
	return s.createRequestToRESTAPI(createReq, "")
}

func (s *APIService) upstreamFromYAML(upstream *dto.UpstreamYAML) api.Upstream {
	if upstream == nil {
		return api.Upstream{}
	}

	var main api.UpstreamDefinition
	if upstream.Main != nil {
		main = api.UpstreamDefinition{
			Url: utils.StringPtrIfNotEmpty(upstream.Main.URL),
			Ref: utils.StringPtrIfNotEmpty(upstream.Main.Ref),
		}
	}

	var sandbox *api.UpstreamDefinition
	if upstream.Sandbox != nil {
		def := api.UpstreamDefinition{
			Url: utils.StringPtrIfNotEmpty(upstream.Sandbox.URL),
			Ref: utils.StringPtrIfNotEmpty(upstream.Sandbox.Ref),
		}
		sandbox = &def
	}

	return api.Upstream{Main: main, Sandbox: sandbox}
}

func (s *APIService) restAPIToProjectValidationAPI(restAPI *api.RESTAPI) *struct {
	Channels        *[]api.Channel                                          `json:"channels,omitempty" yaml:"channels,omitempty"`
	Context         string                                                  `binding:"required" json:"context" yaml:"context"`
	CreatedAt       *time.Time                                              `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	CreatedBy       *string                                                 `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Description     string                                                  `json:"description" yaml:"description"`
	Id              *string                                                 `json:"id,omitempty" yaml:"id,omitempty"`
	Kind            *string                                                 `json:"kind,omitempty" yaml:"kind,omitempty"`
	LifeCycleStatus *api.RESTAPIProjectValidationResponseApiLifeCycleStatus `json:"lifeCycleStatus,omitempty" yaml:"lifeCycleStatus,omitempty"`
	Name            string                                                  `binding:"required" json:"name" yaml:"name"`
	Operations      []api.Operation                                         `json:"operations" yaml:"operations"`
	Policies        *[]api.Policy                                           `json:"policies,omitempty" yaml:"policies,omitempty"`
	ProjectId       openapi_types.UUID                                      `binding:"required" json:"projectId" yaml:"projectId"`
	Transport       *[]string                                               `json:"transport,omitempty" yaml:"transport,omitempty"`
	UpdatedAt       *time.Time                                              `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Upstream        api.Upstream                                            `json:"upstream" yaml:"upstream"`
	Version         string                                                  `binding:"required" json:"version" yaml:"version"`
} {
	if restAPI == nil {
		return nil
	}

	desc := ""
	if restAPI.Description != nil {
		desc = *restAPI.Description
	}

	var status *api.RESTAPIProjectValidationResponseApiLifeCycleStatus
	if restAPI.LifeCycleStatus != nil {
		v := api.RESTAPIProjectValidationResponseApiLifeCycleStatus(*restAPI.LifeCycleStatus)
		status = &v
	}

	operations := []api.Operation{}
	if restAPI.Operations != nil {
		operations = *restAPI.Operations
	}

	return &struct {
		Channels        *[]api.Channel                                          `json:"channels,omitempty" yaml:"channels,omitempty"`
		Context         string                                                  `binding:"required" json:"context" yaml:"context"`
		CreatedAt       *time.Time                                              `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
		CreatedBy       *string                                                 `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
		Description     string                                                  `json:"description" yaml:"description"`
		Id              *string                                                 `json:"id,omitempty" yaml:"id,omitempty"`
		Kind            *string                                                 `json:"kind,omitempty" yaml:"kind,omitempty"`
		LifeCycleStatus *api.RESTAPIProjectValidationResponseApiLifeCycleStatus `json:"lifeCycleStatus,omitempty" yaml:"lifeCycleStatus,omitempty"`
		Name            string                                                  `binding:"required" json:"name" yaml:"name"`
		Operations      []api.Operation                                         `json:"operations" yaml:"operations"`
		Policies        *[]api.Policy                                           `json:"policies,omitempty" yaml:"policies,omitempty"`
		ProjectId       openapi_types.UUID                                      `binding:"required" json:"projectId" yaml:"projectId"`
		Transport       *[]string                                               `json:"transport,omitempty" yaml:"transport,omitempty"`
		UpdatedAt       *time.Time                                              `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
		Upstream        api.Upstream                                            `json:"upstream" yaml:"upstream"`
		Version         string                                                  `binding:"required" json:"version" yaml:"version"`
	}{
		Channels:        restAPI.Channels,
		Context:         restAPI.Context,
		CreatedAt:       restAPI.CreatedAt,
		CreatedBy:       restAPI.CreatedBy,
		Description:     desc,
		Id:              restAPI.Id,
		Kind:            restAPI.Kind,
		LifeCycleStatus: status,
		Name:            restAPI.Name,
		Operations:      operations,
		Policies:        restAPI.Policies,
		ProjectId:       openapi_types.UUID{},
		Transport:       restAPI.Transport,
		UpdatedAt:       restAPI.UpdatedAt,
		Upstream:        restAPI.Upstream,
		Version:         restAPI.Version,
	}
}

func (s *APIService) restAPIToOpenAPIValidationAPI(restAPI *api.RESTAPI) *struct {
	Channels        *[]api.Channel                                   `json:"channels,omitempty" yaml:"channels,omitempty"`
	Context         string                                           `binding:"required" json:"context" yaml:"context"`
	CreatedAt       *time.Time                                       `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	CreatedBy       *string                                          `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
	Description     *string                                          `json:"description,omitempty" yaml:"description,omitempty"`
	Id              *string                                          `json:"id,omitempty" yaml:"id,omitempty"`
	Kind            *string                                          `json:"kind,omitempty" yaml:"kind,omitempty"`
	LifeCycleStatus *api.OpenAPIValidationResponseApiLifeCycleStatus `json:"lifeCycleStatus,omitempty" yaml:"lifeCycleStatus,omitempty"`
	Name            string                                           `binding:"required" json:"name" yaml:"name"`
	Operations      []api.Operation                                  `json:"operations" yaml:"operations"`
	Policies        *[]api.Policy                                    `json:"policies,omitempty" yaml:"policies,omitempty"`
	ProjectId       openapi_types.UUID                               `binding:"required" json:"projectId" yaml:"projectId"`
	Transport       *[]string                                        `json:"transport,omitempty" yaml:"transport,omitempty"`
	UpdatedAt       *time.Time                                       `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Upstream        api.Upstream                                     `json:"upstream" yaml:"upstream"`
	Version         string                                           `binding:"required" json:"version" yaml:"version"`
} {
	if restAPI == nil {
		return nil
	}

	var status *api.OpenAPIValidationResponseApiLifeCycleStatus
	if restAPI.LifeCycleStatus != nil {
		v := api.OpenAPIValidationResponseApiLifeCycleStatus(*restAPI.LifeCycleStatus)
		status = &v
	}

	operations := []api.Operation{}
	if restAPI.Operations != nil {
		operations = *restAPI.Operations
	}

	return &struct {
		Channels        *[]api.Channel                                   `json:"channels,omitempty" yaml:"channels,omitempty"`
		Context         string                                           `binding:"required" json:"context" yaml:"context"`
		CreatedAt       *time.Time                                       `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
		CreatedBy       *string                                          `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
		Description     *string                                          `json:"description,omitempty" yaml:"description,omitempty"`
		Id              *string                                          `json:"id,omitempty" yaml:"id,omitempty"`
		Kind            *string                                          `json:"kind,omitempty" yaml:"kind,omitempty"`
		LifeCycleStatus *api.OpenAPIValidationResponseApiLifeCycleStatus `json:"lifeCycleStatus,omitempty" yaml:"lifeCycleStatus,omitempty"`
		Name            string                                           `binding:"required" json:"name" yaml:"name"`
		Operations      []api.Operation                                  `json:"operations" yaml:"operations"`
		Policies        *[]api.Policy                                    `json:"policies,omitempty" yaml:"policies,omitempty"`
		ProjectId       openapi_types.UUID                               `binding:"required" json:"projectId" yaml:"projectId"`
		Transport       *[]string                                        `json:"transport,omitempty" yaml:"transport,omitempty"`
		UpdatedAt       *time.Time                                       `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
		Upstream        api.Upstream                                     `json:"upstream" yaml:"upstream"`
		Version         string                                           `binding:"required" json:"version" yaml:"version"`
	}{
		Channels:        restAPI.Channels,
		Context:         restAPI.Context,
		CreatedAt:       restAPI.CreatedAt,
		CreatedBy:       restAPI.CreatedBy,
		Description:     restAPI.Description,
		Id:              restAPI.Id,
		Kind:            restAPI.Kind,
		LifeCycleStatus: status,
		Name:            restAPI.Name,
		Operations:      operations,
		Policies:        restAPI.Policies,
		ProjectId:       openapi_types.UUID{},
		Transport:       restAPI.Transport,
		UpdatedAt:       restAPI.UpdatedAt,
		Upstream:        restAPI.Upstream,
		Version:         restAPI.Version,
	}
}

// apiGatewayDetailsToAPIList converts APIGatewayWithDetails models to RESTAPIGatewayListResponse
func apiGatewayDetailsToAPIList(gatewayDetails []*model.APIGatewayWithDetails) (*api.RESTAPIGatewayListResponse, error) {
	if gatewayDetails == nil {
		return &api.RESTAPIGatewayListResponse{
			Count: 0,
			List:  []api.RESTAPIGatewayResponse{},
			Pagination: api.Pagination{
				Total:  0,
				Offset: 0,
				Limit:  0,
			},
		}, nil
	}

	responses := make([]api.RESTAPIGatewayResponse, 0, len(gatewayDetails))
	for _, gwd := range gatewayDetails {
		response, err := apiGatewayDetailsToAPI(gwd)
		if err != nil {
			return nil, err
		}
		if response != nil {
			responses = append(responses, *response)
		}
	}

	return &api.RESTAPIGatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: api.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}, nil
}

// apiGatewayDetailsToAPI converts a single APIGatewayWithDetails model to RESTAPIGatewayResponse
func apiGatewayDetailsToAPI(gwd *model.APIGatewayWithDetails) (*api.RESTAPIGatewayResponse, error) {
	if gwd == nil {
		return nil, nil
	}

	gatewayID, err := utils.ParseOpenAPIUUID(gwd.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gateway ID as UUID: %w", err)
	}
	orgID, err := utils.ParseOpenAPIUUID(gwd.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gateway OrganizationID as UUID: %w", err)
	}

	response := api.RESTAPIGatewayResponse{
		AssociatedAt:      gwd.AssociatedAt,
		CreatedAt:         utils.TimePtrIfNotZero(gwd.CreatedAt),
		Description:       utils.StringPtrIfNotEmpty(gwd.Description),
		DisplayName:       utils.StringPtrIfNotEmpty(gwd.DisplayName),
		FunctionalityType: restAPIGatewayFunctionalityTypePtr(gwd.FunctionalityType),
		Id:                gatewayID,
		IsActive:          utils.BoolPtr(gwd.IsActive),
		IsCritical:        utils.BoolPtr(gwd.IsCritical),
		IsDeployed:        gwd.IsDeployed,
		Name:              utils.StringPtrIfNotEmpty(gwd.Name),
		OrganizationId:    orgID,
		Properties:        utils.MapPtrIfNotEmpty(gwd.Properties),
		UpdatedAt:         utils.TimePtrIfNotZero(gwd.UpdatedAt),
		Vhost:             utils.StringPtrIfNotEmpty(gwd.Vhost),
	}

	// Add deployment details if deployed
	if gwd.IsDeployed && gwd.DeploymentID != nil && gwd.DeployedAt != nil {
		status := api.RESTAPIDeploymentDetailsStatusAPPROVED
		response.Deployment = &api.RESTAPIDeploymentDetails{
			DeployedAt: *gwd.DeployedAt,
			Status:     status,
		}
	}

	return &response, nil
}

// restAPIGatewayFunctionalityTypePtr converts a string to RESTAPIGatewayResponseFunctionalityType pointer
func restAPIGatewayFunctionalityTypePtr(value string) *api.RESTAPIGatewayResponseFunctionalityType {
	if value == "" {
		return nil
	}
	converted := api.RESTAPIGatewayResponseFunctionalityType(value)
	return &converted
}
