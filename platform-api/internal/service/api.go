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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// APIService handles business logic for API operations
type APIService struct {
	apiRepo              repository.APIRepository
	projectRepo          repository.ProjectRepository
	orgRepo              repository.OrganizationRepository
	gatewayRepo          repository.GatewayRepository
	deploymentRepo       repository.DeploymentRepository
	subscriptionPlanRepo repository.SubscriptionPlanRepository
	customPolicyRepo     repository.CustomPolicyRepository
	gatewayEventsService *GatewayEventsService
	apiUtil              *utils.APIUtil
	slogger              *slog.Logger
	auditRepo            repository.AuditRepository
	identity             *IdentityService
}

// NewAPIService creates a new API service
func NewAPIService(apiRepo repository.APIRepository, projectRepo repository.ProjectRepository,
	orgRepo repository.OrganizationRepository, gatewayRepo repository.GatewayRepository,
	deploymentRepo repository.DeploymentRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository,
	customPolicyRepo repository.CustomPolicyRepository,
	gatewayEventsService *GatewayEventsService, apiUtil *utils.APIUtil,
	slogger *slog.Logger, auditRepo repository.AuditRepository, identity *IdentityService) *APIService {
	return &APIService{
		apiRepo:              apiRepo,
		projectRepo:          projectRepo,
		orgRepo:              orgRepo,
		gatewayRepo:          gatewayRepo,
		deploymentRepo:       deploymentRepo,
		subscriptionPlanRepo: subscriptionPlanRepo,
		customPolicyRepo:     customPolicyRepo,
		gatewayEventsService: gatewayEventsService,
		apiUtil:              apiUtil,
		slogger:              slogger,
		auditRepo:            auditRepo,
		identity:             identity,
	}
}

// resolveRESTAPIIdentity replaces resp's createdBy/updatedBy UUIDs with the
// raw external identity (or constants.DeletedUser), in place.
func (s *APIService) resolveRESTAPIIdentity(resp *api.RESTAPI) error {
	if resp == nil {
		return nil
	}
	if err := s.identity.ResolveIdentityField(&resp.CreatedBy); err != nil {
		return err
	}
	return s.identity.ResolveIdentityField(&resp.UpdatedBy)
}

// CreateAPI creates a new API with validation and business logic
func (s *APIService) CreateAPI(req *api.CreateRESTAPIRequest, orgUUID, createdBy string) (*api.RESTAPI, error) {
	// Validate request
	if err := s.validateCreateAPIRequest(req, orgUUID); err != nil {
		return nil, err
	}

	projectHandle := strings.TrimSpace(req.ProjectId)
	// Check if project exists
	project, err := s.projectRepo.GetProjectByHandleAndOrgID(projectHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}

	// Handle the API handle (user-facing identifier)
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
	} else {
		// Generate handle from API name with collision detection
		var err error
		handle, err = utils.GenerateHandle(req.DisplayName, s.HandleExistsCheck(orgUUID))
		if err != nil {
			s.slogger.Error("Failed to generate API handle", "apiName", req.DisplayName, "error", err)
			return nil, err
		}
	}

	// createdBy is always inferred from the authenticated actor, never from the request body.
	req.CreatedBy = &createdBy
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
	apiModel.ProjectID = project.ID
	// Create API in repository (UUID is generated internally by CreateAPI)
	if err := s.apiRepo.CreateAPI(apiModel); err != nil {
		s.slogger.Error("Failed to create API in repository", "apiName", req.DisplayName, "error", err)
		return nil, fmt.Errorf("failed to create api: %w", err)
	}

	// Get the generated UUID from the model (set by CreateAPI)
	apiUUID := apiModel.ID

	s.refreshCustomPolicyUsages(apiUUID, orgUUID, apiModel)

	_ = s.auditRepo.Record("CREATE", apiUUID, "rest_api", orgUUID, createdBy)

	return s.modelToRESTAPI(apiModel)
}

// modelToRESTAPI converts an internal API model to the API representation,
// resolving the project's handle for the response's projectId field and the
// createdBy/updatedBy UUIDs to their raw external identity.
func (s *APIService) modelToRESTAPI(apiModel *model.API) (*api.RESTAPI, error) {
	if apiModel == nil {
		return nil, nil
	}
	project, err := s.projectRepo.GetProjectByUUID(apiModel.ProjectID)
	if err != nil {
		return nil, err
	}
	projectHandle := apiModel.ProjectID
	if project != nil {
		projectHandle = project.Handle
	}
	resp, err := s.apiUtil.ModelToRESTAPI(apiModel, projectHandle)
	if err != nil {
		return nil, err
	}
	if err := s.resolveRESTAPIIdentity(resp); err != nil {
		return nil, err
	}
	return resp, nil
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

	return s.modelToRESTAPI(apiModel)
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

// GetAPIsByOrganization retrieves all APIs for an organization with optional project filter.
// projectHandle, when provided, is the project's handle (not UUID).
func (s *APIService) GetAPIsByOrganization(orgUUID string, projectHandle string) ([]api.RESTAPI, error) {
	projectUUID := ""
	// If project handle is provided, resolve it and validate that it belongs to the organization
	if projectHandle != "" {
		project, err := s.projectRepo.GetProjectByHandleAndOrgID(projectHandle, orgUUID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, constants.ErrProjectNotFound
		}
		projectUUID = project.ID
	}

	apiModels, err := s.apiRepo.GetAPIsByOrganizationUUID(orgUUID, projectUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get apis: %w", err)
	}

	apis := make([]api.RESTAPI, 0)
	for _, apiModel := range apiModels {
		apiResponse, err := s.modelToRESTAPI(apiModel)
		if err != nil {
			return nil, err
		}
		if apiResponse != nil {
			// updatedBy is detail-only; omit it from list responses.
			apiResponse.UpdatedBy = nil
			apis = append(apis, *apiResponse)
		}
	}
	return apis, nil
}

// UpdateAPI updates an existing API
func (s *APIService) UpdateAPI(apiUUID string, req *api.RESTAPI, orgUUID, updatedBy string) (*api.RESTAPI, error) {
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
	updatedAPIModel.UpdatedBy = updatedBy
	// Carry over identity fields that are not user-editable so the runtime-artifact
	// diff below compares like-for-like (RESTAPIToModel derives these from the request
	// and defaults the origin to control_plane).
	updatedAPIModel.Handle = existingAPIModel.Handle
	updatedAPIModel.ProjectID = existingAPIModel.ProjectID
	updatedAPIModel.Kind = existingAPIModel.Kind
	updatedAPIModel.Origin = existingAPIModel.Origin

	// A DP-originated (gateway_api) artifact is read-only in the control plane only for
	// changes that alter the gateway runtime artifact. Allow edits that leave the
	// deployment YAML unchanged (e.g. description, lifecycle status) by diffing the
	// artifact the stored vs updated model produces.
	if err := s.ensureRESTRuntimeArtifactUnchanged(existingAPIModel, updatedAPIModel); err != nil {
		return nil, err
	}

	if err := s.apiRepo.UpdateAPI(updatedAPIModel); err != nil {
		return nil, err
	}

	s.refreshCustomPolicyUsages(apiUUID, orgUUID, updatedAPIModel)

	_ = s.auditRepo.Record("UPDATE", apiUUID, "rest_api", orgUUID, updatedBy)

	return s.modelToRESTAPI(updatedAPIModel)
}

// ensureRESTRuntimeArtifactUnchanged rejects an edit to a DP-originated REST API when it
// would change the gateway runtime artifact. It builds the deployment YAML both the
// stored and updated models produce (via BuildAPIDeploymentYAML, the same builder used
// at deploy time) and compares them. It is a no-op for control-plane artifacts. When the
// stored artifact cannot be rebuilt the edit cannot be proven harmless, so it is kept
// read-only; a build failure on the proposed model is a genuine validation error and is
// surfaced as-is.
func (s *APIService) ensureRESTRuntimeArtifactUnchanged(existing, updated *model.API) error {
	if existing.Origin != constants.OriginDP {
		return nil
	}
	existingArtifact, err := s.apiUtil.BuildAPIDeploymentYAML(existing)
	if err != nil {
		return constants.ErrArtifactRuntimeImmutable
	}
	updatedArtifact, err := s.apiUtil.BuildAPIDeploymentYAML(updated)
	if err != nil {
		return err
	}
	return compareRuntimeArtifacts(existing.Origin, existingArtifact, updatedArtifact)
}

// DeleteAPI deletes an API
func (s *APIService) DeleteAPI(apiUUID, orgUUID, deletedBy string) error {
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

	// DP-originated artifacts may only be deleted once undeployed on all gateways.
	if err := ensureOriginDeletable(s.deploymentRepo, api.Origin, apiUUID, orgUUID); err != nil {
		return err
	}

	// Get all gateways in the organization to broadcast deletion event.
	// We broadcast to all gateways (not just those with active deployments) because
	// deployment_status rows may have been cascade-deleted when deployments were removed,
	// leaving stale artifacts on gateways that would otherwise never receive the delete event.
	var gateways []*model.Gateway
	if s.gatewayRepo != nil {
		gws, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to get gateways for API deletion", "apiUUID", apiUUID, "error", err)
		} else {
			gateways = gws
		}
	}

	// Delete API from repository (this also deletes associations)
	if err := s.apiRepo.DeleteAPI(apiUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete api: %w", err)
	}

	_ = s.auditRepo.Record("DELETE", apiUUID, "rest_api", orgUUID, deletedBy)

	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.APIDeletionEvent{
				ApiId: apiUUID,
			}

			if err := s.gatewayEventsService.BroadcastAPIDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast API deletion event", "gatewayID", gateway.ID, "apiUUID", apiUUID, "error", err)
			} else {
				s.slogger.Info("API deletion event sent", "gatewayID", gateway.ID, "apiUUID", apiUUID)
			}
		}
	}

	return nil
}

// UpdateAPIByHandle updates an existing API identified by handle
func (s *APIService) UpdateAPIByHandle(handle string, req *api.RESTAPI, orgId, updatedBy string) (*api.RESTAPI, error) {
	// The id (handle) is immutable: a body id must match the API being updated.
	if req != nil {
		if err := utils.ValidateHandleImmutable(handle, req.Id); err != nil {
			return nil, err
		}
	}
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return nil, err
	}
	return s.UpdateAPI(apiUUID, req, orgId, updatedBy)
}

// DeleteAPIByHandle deletes an API identified by handle
func (s *APIService) DeleteAPIByHandle(handle, orgId, deletedBy string) error {
	// Get API UUID by handle
	apiUUID, err := s.getAPIUUIDByHandle(handle, orgId)
	if err != nil {
		return err
	}

	// Delete API using existing UUID-based method
	return s.DeleteAPI(apiUUID, orgId, deletedBy)
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

// AddGatewaysToAPI associates multiple gateways with an API
func (s *APIService) AddGatewaysToAPI(apiUUID string, gatewayIds []string, orgUUID string) (*api.RESTAPIGatewayListResponse, error) {
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
	var validGateways []*model.Gateway
	for _, gatewayId := range gatewayIds {
		gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgUUID)
		if err != nil {
			return nil, err
		}
		if gateway == nil {
			return nil, constants.ErrGatewayNotFound
		}
		validGateways = append(validGateways, gateway)
	}
	existingAssociations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		return nil, err
	}
	existingGatewayIds := make(map[string]bool)
	for _, assoc := range existingAssociations {
		existingGatewayIds[assoc.GatewayID] = true
	}
	for _, gateway := range validGateways {
		if existingGatewayIds[gateway.ID] {
			if err := s.apiRepo.UpdateAPIAssociation(apiUUID, gateway.ID, constants.AssociationTypeGateway, orgUUID); err != nil {
				return nil, err
			}
		} else {
			association := &model.APIAssociation{
				ArtifactID:     apiUUID,
				OrganizationID: orgUUID,
				GatewayID:      gateway.ID,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
				return nil, err
			}
			existingGatewayIds[gateway.ID] = true
		}
	}
	return s.GetAPIGateways(apiUUID, orgUUID)
}

// GetAPIGateways retrieves all gateways associated with an API including deployment details
func (s *APIService) GetAPIGateways(apiUUID, orgUUID string) (*api.RESTAPIGatewayListResponse, error) {
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
	gatewayDetails, err := s.apiRepo.GetAPIGatewaysWithDetails(apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, err
	}
	orgHandle := ""
	if org != nil {
		orgHandle = org.Handle
	}
	response, err := apiGatewayDetailsToAPIList(gatewayDetails, orgHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to convert API gateway details: %w", err)
	}
	return response, nil
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
	if req.DisplayName == "" {
		return constants.ErrInvalidAPIName
	}
	if !s.isValidContext(req.Context) {
		return constants.ErrInvalidAPIContext
	}
	if !s.isValidVersion(req.Version) {
		return constants.ErrInvalidAPIVersion
	}
	if strings.TrimSpace(req.ProjectId) == "" {
		return errors.New("project id is required")
	}

	nameVersionExists, err := s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(req.DisplayName, req.Version, orgUUID, "")
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

	if err := validateOperationAndChannelPolicyVersions(req.Operations, req.Channels); err != nil {
		return err
	}

	// Validate subscription plans if provided
	if err := s.validateSubscriptionPlans(req.SubscriptionPlans, orgUUID); err != nil {
		return err
	}

	return nil
}

// validateSubscriptionPlans ensures each plan handle exists in the organization and is ACTIVE.
// It normalizes planHandles in place by trimming each element. Repository errors are returned
// directly; ErrSubscriptionPlanNotFoundOrInactive is only returned when plan is nil or inactive.
func (s *APIService) validateSubscriptionPlans(planHandles *[]string, orgUUID string) error {
	if planHandles == nil || len(*planHandles) == 0 {
		return nil
	}
	for i := range *planHandles {
		(*planHandles)[i] = strings.TrimSpace((*planHandles)[i])
		handle := (*planHandles)[i]
		if handle == "" {
			continue
		}
		plan, err := s.subscriptionPlanRepo.GetByHandleAndOrg(handle, orgUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: plan %q", constants.ErrSubscriptionPlanNotFoundOrInactive, handle)
			}
			return err
		}
		if plan == nil || plan.Status != model.SubscriptionPlanStatusActive {
			return fmt.Errorf("%w: plan %q", constants.ErrSubscriptionPlanNotFoundOrInactive, handle)
		}
	}
	return nil
}

// applyAPIUpdates applies update request fields to an existing API model and handles backend services
func (s *APIService) applyAPIUpdates(existingAPIModel *model.API, req *api.RESTAPI, orgId string) (*api.RESTAPI, error) {
	// Validate update request
	if err := s.validateUpdateAPIRequest(existingAPIModel, req, orgId); err != nil {
		return nil, err
	}

	existingAPI, err := s.modelToRESTAPI(existingAPIModel)
	if err != nil {
		return nil, err
	}

	// Update fields (only allow certain fields to be updated)
	if req.DisplayName != "" {
		existingAPI.DisplayName = req.DisplayName
	}
	if req.Description != nil {
		existingAPI.Description = req.Description
	}
	// createdBy is immutable: intentionally not copied from req, even if the client sends one.
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
	existingAPI.Policies = req.Policies
	existingAPI.SubscriptionPlans = req.SubscriptionPlans
	if !s.isEmptyUpstream(req.Upstream) {
		existingAPI.Upstream = req.Upstream
	}

	return existingAPI, nil
}

// validateUpdateAPIRequest checks the validity of the update API request
func (s *APIService) validateUpdateAPIRequest(existingAPIModel *model.API, req *api.RESTAPI, orgUUID string) error {
	if req.DisplayName != "" {
		nameVersionExists, err := s.apiRepo.CheckAPIExistsByNameAndVersionInOrganization(req.DisplayName,
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

	if err := validateOperationAndChannelPolicyVersions(req.Operations, req.Channels); err != nil {
		return err
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
		DisplayName:       req.DisplayName,
		Operations:        req.Operations,
		Policies:          req.Policies,
		ProjectId:         req.ProjectId,
		SubscriptionPlans: req.SubscriptionPlans,
		Transport:         req.Transport,
		Upstream:          req.Upstream,
		Version:           req.Version,
	}
}

// apiGatewayDetailsToAPIList converts APIGatewayWithDetails models to RESTAPIGatewayListResponse
func apiGatewayDetailsToAPIList(gatewayDetails []*model.APIGatewayWithDetails, orgHandle string) (*api.RESTAPIGatewayListResponse, error) {
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
		response, err := apiGatewayDetailsToAPI(gwd, orgHandle)
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
func apiGatewayDetailsToAPI(gwd *model.APIGatewayWithDetails, orgHandle string) (*api.RESTAPIGatewayResponse, error) {
	if gwd == nil {
		return nil, nil
	}

	response := api.RESTAPIGatewayResponse{
		AssociatedAt:      gwd.AssociatedAt,
		CreatedAt:         utils.TimePtrIfNotZero(gwd.CreatedAt),
		Description:       utils.StringPtrIfNotEmpty(gwd.Description),
		DisplayName:       gwd.Name,
		Endpoints:         &gwd.Endpoints,
		FunctionalityType: restAPIGatewayFunctionalityTypePtr(gwd.FunctionalityType),
		Id:                utils.StringPtrIfNotEmpty(gwd.Handle),
		IsActive:          utils.BoolPtr(gwd.IsActive),
		IsCritical:        utils.BoolPtr(gwd.IsCritical),
		IsDeployed:        gwd.IsDeployed,
		OrganizationId:    utils.StringPtrIfNotEmpty(orgHandle),
		Properties:        utils.MapPtrIfNotEmpty(gwd.Properties),
		UpdatedAt:         utils.TimePtrIfNotZero(gwd.UpdatedAt),
	}

	// Add deployment details if deployed
	if gwd.IsDeployed && gwd.DeploymentID != nil && gwd.DeployedAt != nil {
		status := api.APPROVED
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

// refreshCustomPolicyUsages evaluates gateway_custom_policy_usages for an API.
// 1. Collects all policy (name, version) references across all three levels (API-level, operation-level, channel-level).
// 2. Resolves which ones are custom policies for the org via gateway_custom_policies.
// 3. Diffs against the current usage rows — inserting new ones and deleting removed ones.
func (s *APIService) refreshCustomPolicyUsages(apiUUID, orgUUID string, apiModel *model.API) {
	if s.customPolicyRepo == nil {
		return
	}

	// Step 1: collect all unique (name, version) pairs across all policy levels
	type policyRef struct{ name, version string }
	seen := make(map[policyRef]bool)
	var refs []policyRef

	collect := func(policies []model.Policy) {
		for _, p := range policies {
			ref := policyRef{p.Name, p.Version}
			if !seen[ref] {
				seen[ref] = true
				refs = append(refs, ref)
			}
		}
	}

	// API-level policies
	collect(apiModel.Configuration.Policies)
	// Operation-level policies
	for _, op := range apiModel.Configuration.Operations {
		if op.Request != nil {
			collect(op.Request.Policies)
		}
	}
	// Channel-level policies
	for _, ch := range apiModel.Channels {
		if ch.Request != nil {
			collect(ch.Request.Policies)
		}
	}

	// Step 2: resolve which refs are synced custom policies for this org.
	// Policies are stored with a full semver (e.g. "v1.1.0") but APIs store
	// only the major version (e.g. "v1"). Match by major version, similar to the
	// same logic used in SyncCustomPolicy.
	newSet := make(map[string]bool)
	for _, ref := range refs {
		identifiedSyncedPolicies, err := s.customPolicyRepo.GetCustomPoliciesByName(orgUUID, strings.ToLower(ref.name))
		if err != nil {
			s.slogger.Warn("Failed to lookup custom policy during usage refresh", "name", ref.name, "version", ref.version, "orgUUID", orgUUID, "error", err)
			continue
		}
		parsedPolicyVersion, err := parseVersion(ref.version)
		if err != nil {
			// ref.version is not full semver (e.g. "v1") — try matching by major version prefix
			s.slogger.Debug("Policy version is not full semver, falling back to major version matching", "name", ref.name, "version", ref.version, "error", err)
			refMajor := strings.TrimPrefix(ref.version, "v")
			matched := false
			for _, cp := range identifiedSyncedPolicies {
				cpVer, verErr := parseVersion(cp.Version)
				if verErr != nil {
					s.slogger.Warn("Failed to parse stored custom policy version during usage refresh", "name", cp.Name, "version", cp.Version, "error", verErr)
					continue
				}
				if strconv.Itoa(cpVer.Major) == refMajor {
					newSet[cp.UUID] = true
					matched = true
					break
				}
			}
			// fall back to exact match if no major-version match found
			if !matched {
				for _, cp := range identifiedSyncedPolicies {
					if cp.Version == ref.version {
						newSet[cp.UUID] = true
						break
					}
				}
			}
			continue
		}
		// evaluates parsed policy version
		for _, syncedPolicy := range identifiedSyncedPolicies {
			cpVer, err := parseVersion(syncedPolicy.Version)
			if err != nil {
				s.slogger.Warn("Failed to parse stored custom policy version during usage refresh", "name", syncedPolicy.Name, "version", syncedPolicy.Version, "error", err)
				continue
			}
			if cpVer.Major == parsedPolicyVersion.Major {
				newSet[syncedPolicy.UUID] = true
				break
			}
		}
	}

	// Step 3: fetch current usages from the join table
	currentUUIDs, err := s.customPolicyRepo.GetCustomPolicyUsagesByAPIUUID(apiUUID)
	if err != nil {
		s.slogger.Warn("Failed to fetch custom policy usages", "apiUUID", apiUUID, "error", err)
		return
	}
	currentSet := make(map[string]bool, len(currentUUIDs))
	for _, u := range currentUUIDs {
		currentSet[u] = true
	}

	// Step 4: insert new, delete removed, skip unchanged
	for uuid := range newSet {
		if !currentSet[uuid] {
			if err := s.customPolicyRepo.InsertCustomPolicyUsage(uuid, apiUUID); err != nil {
				s.slogger.Warn("Failed to insert custom policy usage", "policyUUID", uuid, "apiUUID", apiUUID, "error", err)
			}
		}
	}
	for uuid := range currentSet {
		if !newSet[uuid] {
			if err := s.customPolicyRepo.DeleteCustomPolicyUsage(uuid, apiUUID); err != nil {
				s.slogger.Warn("Failed to delete custom policy usage", "policyUUID", uuid, "apiUUID", apiUUID, "error", err)
			}
		}
	}
}
