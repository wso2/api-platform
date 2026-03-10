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
	"log/slog"
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/client/devportal_client"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Shared validator instance to reduce memory usage
var sharedValidator = validator.New()

// Constants for DevPortal operations
const (
	DevPortalServiceTimeout = 10
)

// sanitizeAPIHandle removes forward slashes and backslashes from API handle
func sanitizeAPIHandle(handle string) string {
	handle = strings.ReplaceAll(handle, "/", "")
	handle = strings.ReplaceAll(handle, "\\", "")
	return handle
}

// DevPortalService manages DevPortal operations using optimized data fetching and delegating client calls
type DevPortalService struct {
	devPortalRepo      repository.DevPortalRepository
	orgRepo            repository.OrganizationRepository
	publicationRepo    repository.APIPublicationRepository
	apiRepo            repository.APIRepository
	apiUtil            *utils.APIUtil
	config             *config.Server          // Global configuration for role mapping
	devPortalClientSvc *DevPortalClientService // Handles all DevPortal client interactions
	validator          *validator.Validate
	slogger            *slog.Logger
}

// NewDevPortalService creates a new DevPortalService with optimized data fetching
func NewDevPortalService(
	devPortalRepo repository.DevPortalRepository,
	orgRepo repository.OrganizationRepository,
	publicationRepo repository.APIPublicationRepository,
	apiRepo repository.APIRepository,
	apiUtil *utils.APIUtil,
	config *config.Server,
	slogger *slog.Logger,
) *DevPortalService {
	return &DevPortalService{
		devPortalRepo:      devPortalRepo,
		orgRepo:            orgRepo,
		publicationRepo:    publicationRepo,
		apiRepo:            apiRepo,
		apiUtil:            apiUtil,
		config:             config,
		devPortalClientSvc: NewDevPortalClientService(config),
		validator:          sharedValidator,
		slogger:            slogger,
	}
}

// getDevPortalByUUID retrieves a DevPortal by UUID with error handling
func (s *DevPortalService) getDevPortalByUUID(uuid, orgUUID string) (*model.DevPortal, error) {
	devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}
	return devPortal, nil
}

func (s *DevPortalService) CreateDefaultDevPortal(orgUUID string) (*model.DevPortal, error) {
	// Check if default DevPortal creation is enabled
	if !s.config.DefaultDevPortal.Enabled {
		return nil, nil
	}

	// Get organization to derive UIUrl (cache for reuse)
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		return nil, fmt.Errorf("organization %s not found", orgUUID)
	}

	// Create DevPortal from default configuration
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate DevPortal ID: %w", err)
	}
	devPortal := &model.DevPortal{
		UUID:             uuidStr,
		OrganizationUUID: orgUUID,
		Name:             s.config.DefaultDevPortal.Name,
		Identifier:       org.Handle, // Derived from organization handle
		APIUrl:           s.config.DefaultDevPortal.APIUrl,
		Hostname:         s.config.DefaultDevPortal.Hostname,
		IsActive:         false, // Will be updated after sync attempt
		IsEnabled:        false, // Default DevPortals start disabled
		APIKey:           s.config.DefaultDevPortal.APIKey,
		HeaderKeyName:    s.config.DefaultDevPortal.HeaderKeyName,
		IsDefault:        true,
		Visibility:       "private", // Default DevPortals are private
		Description:      "Default DevPortal for organization",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Use common method to create with sync (allow sync failure for default DevPortals)
	if err := s.createDevPortalWithSync(devPortal, org, true); err != nil {
		return nil, err
	}

	return devPortal, nil
}

// CreateDevPortal creates a new DevPortal for an organization
func (s *DevPortalService) CreateDevPortal(orgUUID string, req *api.CreateDevPortalRequest) (*api.DevPortalResponse, error) {
	// Get organization details to derive identifier (cache for reuse)
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get organization for DevPortal creation", "orgUUID", orgUUID, "error", err)
		return nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		s.slogger.Error("Organization not found for DevPortal creation", "orgUUID", orgUUID)
		return nil, constants.ErrOrganizationNotFound
	}

	// Convert request to model
	devPortal := createDevPortalRequestToModel(req, orgUUID)

	s.slogger.Info("Attempting to create DevPortal", "devPortalName", devPortal.Name, "orgUUID", orgUUID)

	// Use common method to create with sync (don't allow sync failure for user-created DevPortals)
	if err := s.createDevPortalWithSync(devPortal, org, false); err != nil {
		s.slogger.Error("Failed to create DevPortal", "devPortalName", devPortal.Name, "orgUUID", orgUUID, "error", err)
		return nil, err
	}

	s.slogger.Info("Successfully created DevPortal", "devPortalName", devPortal.Name, "orgUUID", orgUUID)
	return devPortalModelToResponse(devPortal)
}

// EnableDevPortal enables a DevPortal for use (activates/syncs it first if needed)
func (s *DevPortalService) EnableDevPortal(uuid, orgUUID string) error {
	// Get DevPortal (cache for reuse)
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get DevPortal during enable", "uuid", uuid, "orgUUID", orgUUID, "error", err)
		return err
	}

	if devPortal.IsEnabled {
		s.slogger.Info("DevPortal is already enabled", "uuid", uuid, "orgUUID", orgUUID)
		return nil
	}

	s.slogger.Info("Attempting to enable DevPortal", "uuid", uuid, "orgUUID", orgUUID)

	// Get organization (cache for reuse)
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get organization for DevPortal enable", "orgUUID", orgUUID, "uuid", uuid, "error", err)
		return fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		s.slogger.Error("Organization not found for DevPortal enable", "orgUUID", orgUUID, "uuid", uuid)
		return constants.ErrOrganizationNotFound
	}

	// If DevPortal is not activated (synced), activate it first
	if !devPortal.IsActive {
		s.slogger.Info("DevPortal not active, attempting to sync and initialize", "uuid", uuid)
		if err := s.syncAndInitializeDevPortalInternal(devPortal, org); err != nil {
			s.slogger.Error("Failed to sync and initialize DevPortal during enable", "uuid", uuid, "orgUUID", orgUUID, "error", err)
			return fmt.Errorf("failed to sync and initialize DevPortal during enable: %w", err)
		}
		// Mark as active in memory
		devPortal.IsActive = true
	}

	// Enable the DevPortal (set IsEnabled = true) and update atomically
	devPortal.IsEnabled = true
	devPortal.UpdatedAt = time.Now()

	if err := s.devPortalRepo.Update(devPortal, orgUUID); err != nil {
		s.slogger.Error("Failed to update DevPortal in repository during enable", "uuid", uuid, "error", err)
		return fmt.Errorf("failed to enable DevPortal: %w", err)
	}

	s.slogger.Info("Successfully enabled DevPortal", "uuid", uuid, "orgUUID", orgUUID)
	return nil
}

// createDevPortalWithSync creates a DevPortal and attempts to sync it gracefully
func (s *DevPortalService) createDevPortalWithSync(devPortal *model.DevPortal, organization *model.Organization, allowSyncFailure bool) error {
	// Create DevPortal in repository first
	if err := s.devPortalRepo.Create(devPortal); err != nil {
		return fmt.Errorf("failed to create DevPortal: %w", err)
	}

	// Attempt to sync and initialize (handle gracefully based on allowSyncFailure flag)
	if err := s.syncAndInitializeDevPortalInternal(devPortal, organization); err != nil {
		// Other sync errors - apply rollback logic based on allowSyncFailure flag
		if !allowSyncFailure {
			// If sync failure is not allowed, rollback and return error
			if deleteErr := s.devPortalRepo.Delete(devPortal.UUID, devPortal.OrganizationUUID); deleteErr != nil {
				s.slogger.Error("Failed to rollback DevPortal creation", "error", deleteErr)
				return fmt.Errorf("sync failed and rollback failed: %w (rollback error: %v)", err, deleteErr)
			}
			return err
		}
		// Sync failed but allowed - DevPortal remains inactive
		devPortal.IsActive = false
		devPortal.IsEnabled = false
	} else {
		// Sync successful - organization created in remote DevPortal
		devPortal.IsActive = true
		devPortal.IsEnabled = true
	}

	// Update DevPortal state in repository
	if err := s.updateDevPortalStateInternal(devPortal, &devPortal.IsActive, &devPortal.IsEnabled); err != nil {
		return fmt.Errorf("failed to update DevPortal state after sync: %w", err)
	}

	return nil
}

// syncAndInitializeDevPortalInternal performs organization sync and policy creation with proper error handling
func (s *DevPortalService) syncAndInitializeDevPortalInternal(devPortal *model.DevPortal, organization *model.Organization) error {
	// Attempt to sync organization to DevPortal
	if err := s.devPortalClientSvc.SyncOrganizationToDevPortal(devPortal, organization); err != nil {
		s.slogger.Error("Failed to sync organization to DevPortal", "orgID", organization.ID, "devPortalName", devPortal.Name, "error", err)
		return err
	}

	// Create default subscription policy
	if err := s.devPortalClientSvc.CreateDefaultSubscriptionPolicy(devPortal); err != nil {
		s.slogger.Error("Failed to create default subscription policy for DevPortal", "devPortalName", devPortal.Name, "error", err)
		return err
	}

	return nil
}

// updateDevPortalStateInternal atomically updates DevPortal state with proper transaction handling
func (s *DevPortalService) updateDevPortalStateInternal(devPortal *model.DevPortal, isActive, isEnabled *bool) error {
	// Update in-memory object first
	if isActive != nil {
		devPortal.IsActive = *isActive
	}
	if isEnabled != nil {
		devPortal.IsEnabled = *isEnabled
	}
	devPortal.UpdatedAt = time.Now()

	// Update in repository
	return s.devPortalRepo.Update(devPortal, devPortal.OrganizationUUID)
}

// GetDevPortal retrieves a DevPortal by UUID
func (s *DevPortalService) GetDevPortal(uuid, orgUUID string) (*api.DevPortalResponse, error) {
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	return devPortalModelToResponse(devPortal)
}

// ListDevPortals lists DevPortals for an organization with optional filters
func (s *DevPortalService) ListDevPortals(orgUUID string, isDefault, isEnabled *bool, limit, offset int) (*api.DevPortalListResponse, error) {
	devPortals, err := s.devPortalRepo.GetByOrganizationUUID(orgUUID, isDefault, isEnabled, limit, offset)
	if err != nil {
		return nil, err
	}

	// Convert to response
	responses := make([]api.DevPortalResponse, len(devPortals))
	for i, devPortal := range devPortals {
		resp, err := devPortalModelToResponse(devPortal)
		if err != nil {
			return nil, err
		}
		responses[i] = *resp
	}

	totalCount, err := s.devPortalRepo.CountByOrganizationUUID(orgUUID, isDefault, isEnabled)
	if err != nil {
		return nil, err
	}

	return &api.DevPortalListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  int(totalCount),
		},
	}, nil
}

// UpdateDevPortal updates an existing DevPortal
func (s *DevPortalService) UpdateDevPortal(uuid, orgUUID string, req *api.UpdateDevPortalRequest) (*api.DevPortalResponse, error) {
	// Get existing DevPortal
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get DevPortal during update", "uuid", uuid, "orgUUID", orgUUID, "error", err)
		return nil, err
	}

	s.slogger.Info("Attempting to update DevPortal", "uuid", uuid, "orgUUID", orgUUID)

	// Update fields from request
	if req.Name != nil {
		devPortal.Name = *req.Name
	}
	if req.ApiUrl != nil {
		devPortal.APIUrl = *req.ApiUrl
	}
	if req.Hostname != nil {
		devPortal.Hostname = *req.Hostname
	}
	if req.ApiKey != nil {
		devPortal.APIKey = *req.ApiKey
	}
	if req.HeaderKeyName != nil {
		devPortal.HeaderKeyName = *req.HeaderKeyName
	}
	if req.Visibility != nil {
		devPortal.Visibility = string(*req.Visibility)
	}
	if req.Description != nil {
		devPortal.Description = *req.Description
	}
	devPortal.UpdatedAt = time.Now()

	// Update in repository
	if err := s.devPortalRepo.Update(devPortal, orgUUID); err != nil {
		s.slogger.Error("Failed to update DevPortal in repository", "uuid", uuid, "error", err)
		return nil, err
	}

	s.slogger.Info("Successfully updated DevPortal", "uuid", uuid, "orgUUID", orgUUID)
	return devPortalModelToResponse(devPortal)
}

// DeleteDevPortal deletes a DevPortal
func (s *DevPortalService) DeleteDevPortal(uuid, orgUUID string) error {
	s.slogger.Info("Attempting to delete DevPortal", "uuid", uuid, "orgUUID", orgUUID)
	err := s.devPortalRepo.Delete(uuid, orgUUID)
	if err != nil {
		s.slogger.Error("Failed to delete DevPortal", "uuid", uuid, "orgUUID", orgUUID, "error", err)
		return err
	}
	s.slogger.Info("Successfully deleted DevPortal", "uuid", uuid, "orgUUID", orgUUID)
	return nil
}

// DisableDevPortal disables a DevPortal for use
func (s *DevPortalService) DisableDevPortal(uuid, orgUUID string) error {
	s.slogger.Info("Attempting to disable DevPortal", "uuid", uuid, "orgUUID", orgUUID)
	// Attempt to disable DevPortal in a single repository call (no prior fetch)
	if err := s.devPortalRepo.UpdateEnabledStatus(uuid, orgUUID, false); err != nil {
		s.slogger.Error("Failed to disable DevPortal", "uuid", uuid, "orgUUID", orgUUID, "error", err)
		return fmt.Errorf("failed to disable DevPortal: %w", err)
	}
	s.slogger.Info("Successfully disabled DevPortal", "uuid", uuid, "orgUUID", orgUUID)
	return nil
}

// SetAsDefault sets a DevPortal as the default for its organization
func (s *DevPortalService) SetAsDefault(uuid, orgUUID string) error {
	s.slogger.Info("Attempting to set DevPortal as default", "uuid", uuid, "orgUUID", orgUUID)
	// Get DevPortal to ensure it exists
	_, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get DevPortal during set as default", "uuid", uuid, "orgUUID", orgUUID, "error", err)
		return err
	}

	// Set as default in repository
	if err := s.devPortalRepo.SetAsDefault(uuid, orgUUID); err != nil {
		s.slogger.Error("Failed to set DevPortal as default", "uuid", uuid, "orgUUID", orgUUID, "error", err)
		return err
	}
	s.slogger.Info("Successfully set DevPortal as default", "uuid", uuid, "orgUUID", orgUUID)
	return nil
}

// GetDefaultDevPortal retrieves the default DevPortal for an organization
func (s *DevPortalService) GetDefaultDevPortal(orgUUID string) (*api.DevPortalResponse, error) {
	devPortal, err := s.devPortalRepo.GetDefaultByOrganizationUUID(orgUUID)
	if err != nil {
		return nil, err
	}

	return devPortalModelToResponse(devPortal)
}

// PublishAPIToDevPortal publishes an API to a DevPortal
func (s *DevPortalService) PublishAPIToDevPortal(apiUUID string, apiModel *api.RESTAPI, req *api.PublishToDevPortalRequest, orgUUID string) error {
	if apiUUID == "" {
		return fmt.Errorf("apiUUID is required")
	}
	if apiModel == nil {
		return fmt.Errorf("apiModel is required")
	}
	if apiModel.Id == nil || *apiModel.Id == "" {
		return fmt.Errorf("API handle is required")
	}

	// --- Phase 1: Validate Inputs ---
	devPortal, org, err := s.validatePublishInputs(req, orgUUID)
	if err != nil {
		s.slogger.Error("Input validation failed for API", "apiID", *apiModel.Id, "error", err)
		return err
	}

	// --- Phase 2: Prepare Publication ---
	err = s.prepareAPIPublication(apiUUID, req, devPortal, orgUUID)
	if err != nil {
		s.slogger.Error("Publication preparation failed for API", "apiID", *apiModel.Id, "error", err)
		return err
	}

	// --- Phase 3: Build API Metadata ---
	apiMetadata, err := s.prepareAPIMetadata(apiUUID, apiModel, req)
	if err != nil {
		s.slogger.Error("Metadata preparation failed for API", "apiID", *apiModel.Id, "error", err)
		return err
	}

	s.slogger.Info("Publishing API to DevPortal", "apiID", *apiModel.Id, "devPortalName", devPortal.Name)

	// --- Phase 4: Publish API to DevPortal ---
	err = s.publishToDevPortal(apiUUID, apiModel, org, devPortal, apiMetadata, req)
	if err != nil {
		s.slogger.Error("Failed to publish API to DevPortal", "apiID", *apiModel.Id, "devPortalName", devPortal.Name, "error", err)
		return err
	}

	return nil
}

// validatePublishInputs validates DevPortal and Organization for publishing
func (s *DevPortalService) validatePublishInputs(req *api.PublishToDevPortalRequest, orgUUID string) (*model.DevPortal, *model.Organization, error) {
	devPortal, err := s.getDevPortalByUUID(req.DevPortalUuid.String(), orgUUID)
	if err != nil {
		return nil, nil, err
	}
	if !devPortal.IsEnabled {
		return nil, nil, fmt.Errorf("devPortal %s is not enabled", devPortal.Name)
	}
	if !devPortal.IsActive {
		return nil, nil, fmt.Errorf("devPortal %s is not activated (synced)", devPortal.Name)
	}

	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		return nil, nil, fmt.Errorf("organization %s not found", orgUUID)
	}

	return devPortal, org, nil
}

// prepareAPIPublication handles duplicate checks and API-DevPortal association creation
func (s *DevPortalService) prepareAPIPublication(apiUUID string, req *api.PublishToDevPortalRequest, devPortal *model.DevPortal, orgUUID string) error {
	// Check if already published (prevent duplicates)
	existing, err := s.publicationRepo.GetByAPIAndDevPortal(apiUUID, req.DevPortalUuid.String(), orgUUID)
	if err != nil && !errors.Is(err, constants.ErrAPIPublicationNotFound) {
		return fmt.Errorf("failed to check existing publication: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("API already published to this DevPortal")
	}

	return nil
}

// prepareAPIMetadata builds API metadata request with defaults and user overrides
func (s *DevPortalService) prepareAPIMetadata(apiUUID string, apiModel *api.RESTAPI, req *api.PublishToDevPortalRequest) (devportal_client.APIMetadataRequest, error) {
	provider := ""
	if apiModel.CreatedBy != nil {
		provider = *apiModel.CreatedBy
	}
	apiDescription := ""
	if apiModel.Description != nil {
		apiDescription = *apiModel.Description
	}

	// Default values - system fields from API
	apiInfo := devportal_client.APIInfo{
		APIID:          apiUUID,
		ReferenceID:    apiUUID,
		APIName:        apiModel.Name,
		APIHandle:      sanitizeAPIHandle(apiModel.Context),
		APIVersion:     apiModel.Version,
		APIType:        devportal_client.APIType("REST"),
		Provider:       provider,
		APIDescription: apiDescription,
		APIStatus:      "PUBLISHED",
		Visibility:     devportal_client.APIVisibility("PUBLIC"),
		Labels:         []string{"default"},
	}

	// Handle empty Provider and Description
	if apiInfo.Provider == "" {
		apiInfo.Provider = "N/A"
	}
	if apiInfo.APIDescription == "" {
		apiInfo.APIDescription = "N/A"
	}

	// Handle empty Provider and Description
	if apiInfo.Provider == "" {
		apiInfo.Provider = "N/A"
	}
	if apiInfo.APIDescription == "" {
		apiInfo.APIDescription = "N/A"
	}

	// Apply user overrides
	if v := req.ApiInfo; v != nil {
		if v.ApiName != nil && *v.ApiName != "" {
			apiInfo.APIName = *v.ApiName
		}
		if v.ApiDescription != nil && *v.ApiDescription != "" {
			apiInfo.APIDescription = *v.ApiDescription
		}
		if v.ApiType != nil && *v.ApiType != "" {
			apiInfo.APIType = devportal_client.APIType(*v.ApiType)
		}
		if v.Visibility != nil && *v.Visibility != "" {
			apiInfo.Visibility = devportal_client.APIVisibility(*v.Visibility)
		}
		if v.VisibleGroups != nil && len(*v.VisibleGroups) > 0 {
			apiInfo.VisibleGroups = *v.VisibleGroups
		}
		if v.Tags != nil && len(*v.Tags) > 0 {
			apiInfo.Tags = *v.Tags
		}
		if v.Labels != nil && len(*v.Labels) > 0 {
			apiInfo.Labels = *v.Labels
		}
		if v.Owners != nil {
			apiInfo.Owners = devportal_client.Owners{
				TechnicalOwner:      utils.StringPtrValue(v.Owners.TechnicalOwner),
				TechnicalOwnerEmail: utils.StringPtrValue(v.Owners.TechnicalOwnerEmail),
				BusinessOwner:       utils.StringPtrValue(v.Owners.BusinessOwner),
				BusinessOwnerEmail:  utils.StringPtrValue(v.Owners.BusinessOwnerEmail),
			}
		}
	}

	// Validate the APIInfo
	if err := s.validator.Struct(apiInfo); err != nil {
		return devportal_client.APIMetadataRequest{}, err
	}

	// Convert subscription policies from strings to objects
	var subscriptionPolicies []devportal_client.SubscriptionPolicyRequest
	if req.SubscriptionPolicies != nil {
		subscriptionPolicies = make([]devportal_client.SubscriptionPolicyRequest, len(*req.SubscriptionPolicies))
		for i, policyName := range *req.SubscriptionPolicies {
			subscriptionPolicies[i] = devportal_client.SubscriptionPolicyRequest{
				PolicyName: policyName,
			}
		}
	}

	apiMetadata := devportal_client.APIMetadataRequest{
		APIInfo: apiInfo,
		EndPoints: devportal_client.EndPoints{
			ProductionURL: "",
			SandboxURL:    "",
		},
		SubscriptionPolicies: subscriptionPolicies,
	}

	// Set endpoint URLs if provided
	if req.EndPoints.ProductionURL != nil {
		apiMetadata.EndPoints.ProductionURL = *req.EndPoints.ProductionURL
	}
	if req.EndPoints.SandboxURL != nil {
		apiMetadata.EndPoints.SandboxURL = *req.EndPoints.SandboxURL
	}

	// Validate the entire API metadata request
	if err := s.validator.Struct(apiMetadata); err != nil {
		s.slogger.Error("Validation failed for API metadata", "error", err)
		return devportal_client.APIMetadataRequest{}, err
	}

	return apiMetadata, nil
}

// publishToDevPortal handles the actual DevPortal API call and publication record updates
func (s *DevPortalService) publishToDevPortal(
	apiUUID string,
	apiModel *api.RESTAPI,
	org *model.Organization,
	devPortal *model.DevPortal,
	apiMetadata devportal_client.APIMetadataRequest,
	req *api.PublishToDevPortalRequest,
) error {

	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Check if API exists in DevPortal
	exists, err := s.devPortalClientSvc.CheckAPIExists(client, org.ID, apiUUID)
	if err != nil {
		s.slogger.Error("API publication failed: failed to check if API exists in DevPortal", "apiUUID", apiUUID, "devPortalName", devPortal.Name, "error", err)
		return fmt.Errorf("failed to check if API exists in DevPortal: %w", err)
	}
	if exists {
		s.slogger.Error("API publication failed: API already exists in DevPortal", "apiUUID", apiUUID, "devPortalName", devPortal.Name)
		return fmt.Errorf("API %s already exists in DevPortal %s", apiUUID, devPortal.Name)
	}

	// Generate OpenAPI definition
	apiDef, err := s.apiUtil.GenerateOpenAPIDefinitionFromRESTAPI(apiModel, &apiMetadata)
	if err != nil {
		s.slogger.Error("API publication failed: failed to generate OpenAPI definition", "apiUUID", apiUUID, "devPortalName", devPortal.Name, "error", err)
		return fmt.Errorf("failed to generate OpenAPI definition for API %s: %w", apiUUID, err)
	}

	// CRITICAL SECTION: DevPortal publication with transactional compensation
	var devPortalRefID *string

	// Step 1: Publish to DevPortal
	devPortalResponse, err := s.devPortalClientSvc.PublishAPIToDevPortal(client, org.ID, apiMetadata, apiDef)
	if err != nil {
		s.slogger.Error("API publication failed", "apiUUID", apiUUID, "devPortalName", devPortal.Name, "error", err)
		return err
	}

	devPortalRefID = &devPortalResponse.ID
	s.slogger.Info("Successfully published API to DevPortal", "apiUUID", apiUUID, "devPortalName", devPortal.Name, "devPortalRefID", *devPortalRefID)

	// Step 2: Create publication record with compensation on failure
	publication := &model.APIPublication{
		APIUUID:          apiUUID,
		DevPortalUUID:    devPortal.UUID,
		OrganizationUUID: org.ID,
		Status:           model.PublishedStatus,
		APIVersion:       &apiModel.Version,
		DevPortalRefID:   devPortalRefID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if req.EndPoints.SandboxURL != nil {
		publication.SandboxEndpointURL = *req.EndPoints.SandboxURL
	}
	if req.EndPoints.ProductionURL != nil {
		publication.ProductionEndpointURL = *req.EndPoints.ProductionURL
	}

	// Step 3: Save with compensation handling
	err = s.savePublicationWithCompensation(publication, client, org.ID, apiUUID, *devPortalRefID, devPortal.Name, devPortal)
	if err != nil {
		return err
	}

	s.slogger.Info("API successfully published to DevPortal", "apiUUID", apiUUID, "devPortalName", devPortal.Name)

	return nil
}

// savePublicationWithCompensation handles DB persistence with auto rollback if DevPortal publish succeeded
func (s *DevPortalService) savePublicationWithCompensation(
	publication *model.APIPublication,
	client *devportal_client.DevPortalClient,
	orgID, apiID, devPortalRefID, devPortalName string,
	devPortal *model.DevPortal,
) error {

	// 1.  Validate Before Persisting
	if err := publication.Validate(); err != nil {
		s.slogger.Error("Validation failed — triggering compensation rollback", "apiID", apiID, "error", err)

		if compensationErr := s.compensatePublication(client, orgID, apiID, devPortalRefID, devPortalName, err, "validation failed"); compensationErr != nil {
			return compensationErr
		}

		return err
	}

	// 2. Try saving to database
	if err := s.publicationRepo.Create(publication); err != nil {
		s.slogger.Error("Database save failed — initiating compensation rollback", "apiID", apiID, "error", err)

		if compensationErr := s.compensatePublication(client, orgID, apiID, devPortalRefID, devPortalName, err, "database save failed"); compensationErr != nil {
			return compensationErr
		}

		return fmt.Errorf("%w: %w", constants.ErrAPIPublicationSaveFailed, err)
	}

	// Create API-DevPortal association after successful publication
	association := &model.APIAssociation{
		ArtifactID:      apiID,
		OrganizationID:  orgID,
		ResourceID:      devPortal.UUID,
		AssociationType: constants.AssociationTypeDevPortal,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
		// Check if this is a duplicate key error (association already exists)
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
			!strings.Contains(err.Error(), "duplicate key") {
			s.slogger.Error("Failed to create API association after successful publication", "apiID", apiID, "error", err)
			// Don't fail the publication, just log
		}
	}

	return nil
}

// compensatePublication handles rollback and returns appropriate error for split-brain scenarios
func (s *DevPortalService) compensatePublication(
	client *devportal_client.DevPortalClient,
	orgID, apiID, devPortalRefID, devPortalName string,
	originalErr error,
	failureReason string,
) error {
	s.slogger.Error("Starting rollback - removing API from DevPortal due to publication failure")

	// Retry rollback up to 3 times with exponential backoff
	maxRetries := 3
	var rollbackErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rollbackErr = s.devPortalClientSvc.UnpublishAPIFromDevPortal(client, orgID, apiID)
		if rollbackErr == nil {
			// Success - rollback completed
			s.slogger.Error("Compensation completed: removed API from DevPortal", "apiID", apiID, "devPortalName", devPortalName, "failureReason", failureReason)
			return nil
		}

		// Check if this is a "not found" error (404) - API already removed
		wrappedErr := utils.WrapDevPortalClientError(rollbackErr)
		if errors.Is(wrappedErr, constants.ErrAPINotFound) {
			s.slogger.Error("Compensation completed: API already removed from DevPortal (404)", "apiID", apiID, "devPortalName", devPortalName)
			return nil
		}

		// If not the last attempt, wait before retrying
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * time.Second // 1s, 2s, 3s
			s.slogger.Error("Rollback attempt failed, retrying", "attempt", attempt, "waitTime", waitTime, "rollbackError", rollbackErr)
			time.Sleep(waitTime)
		}
	}

	// All retries failed
	s.slogger.Error("Rollback failed after all retries - unable to remove API from DevPortal", "error", rollbackErr)

	// Critical—Unable to maintain consistency (Split-brain situation)
	s.slogger.Error("Critical error - API published but database update failed and cleanup unsuccessful. Manual intervention required")

	return fmt.Errorf("%w: API %s, DevPortalRef %s, OriginalErr: %w, RollbackErr: %v",
		constants.ErrAPIPublicationSplitBrain, apiID, devPortalRefID, originalErr, rollbackErr)
}

// UnpublishAPIFromDevPortal unpublishes an API from a DevPortal
// Note: This removes the publication record but keeps the API-DevPortal association intact
// The DevPortal will still be listed as "associated" but not "published" in GetAPIPublications
func (s *DevPortalService) UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID string) error {

	// Get DevPortal
	devPortal, err := s.getDevPortalByUUID(devPortalUUID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get DevPortal for unpublish", "devPortalUUID", devPortalUUID, "error", err)
		return err
	}

	// Create DevPortal client for unpublishing
	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Unpublish API using client service
	err = s.devPortalClientSvc.UnpublishAPIFromDevPortal(client, orgID, apiID)
	if err != nil {
		s.slogger.Error("DevPortal client unpublish failed", "apiID", apiID, "devPortalName", devPortal.Name, "error", err)

		// Check if this is a "not found" error (404) by wrapping and checking for ErrAPINotFound
		wrappedErr := utils.WrapDevPortalClientError(err)
		if errors.Is(wrappedErr, constants.ErrAPINotFound) {
			s.slogger.Info("API not found in DevPortal, treating as already unpublished", "apiID", apiID, "devPortalName", devPortal.Name)
			// API already gone from DevPortal - proceed to delete local record
		} else {
			// Use standard error wrapper for other devportal_client errors
			if wrappedErr != err {
				return wrappedErr
			}
			return fmt.Errorf("failed to unpublish API from DevPortal: %w", err)
		}
	}

	// Delete publication record after successful unpublish
	// NOTE: We intentionally keep the association_mappings record to maintain the association history
	if err := s.publicationRepo.Delete(apiID, devPortalUUID, orgID); err != nil {
		// Log error but don't fail the unpublish operation if record doesn't exist
		if !errors.Is(err, constants.ErrAPIPublicationNotFound) {
			s.slogger.Error("Failed to delete publication record", "apiID", apiID, "devPortalName", devPortal.Name, "error", err)
			return fmt.Errorf("failed to delete publication record: %w", err)
		}
		s.slogger.Info("Publication record not found, skipping delete", "apiID", apiID, "devPortalName", devPortal.Name)
	}

	s.slogger.Info("Successfully unpublished API from DevPortal", "apiID", apiID, "devPortalUUID", devPortalUUID)
	return nil
}

// createDevPortalRequestToModel converts a CreateDevPortalRequest API type to a DevPortal model
func createDevPortalRequestToModel(req *api.CreateDevPortalRequest, orgUUID string) *model.DevPortal {
	visibility := "private"
	if req.Visibility != nil {
		visibility = string(*req.Visibility)
	}

	return &model.DevPortal{
		OrganizationUUID: orgUUID,
		Name:             strings.TrimSpace(req.Name),
		APIUrl:           strings.TrimSpace(req.ApiUrl),
		Hostname:         strings.TrimSpace(req.Hostname),
		APIKey:           strings.TrimSpace(req.ApiKey),
		IsActive:         false,
		IsEnabled:        false,
		HeaderKeyName:    strings.TrimSpace(utils.StringPtrValue(req.HeaderKeyName)),
		IsDefault:        false,
		Visibility:       visibility,
		Description:      strings.TrimSpace(utils.StringPtrValue(req.Description)),
		Identifier:       strings.TrimSpace(req.Identifier),
	}
}

// devPortalModelToResponse converts a DevPortal model to a DevPortalResponse API type
func devPortalModelToResponse(devPortal *model.DevPortal) (*api.DevPortalResponse, error) {
	if devPortal == nil {
		return nil, nil
	}
	orgUUID, err := uuid.Parse(devPortal.OrganizationUUID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization UUID: %w", err)
	}
	portalUUID, err := uuid.Parse(devPortal.UUID)
	if err != nil {
		return nil, fmt.Errorf("invalid portal UUID: %w", err)
	}
	visibility := api.DevPortalResponseVisibility(devPortal.Visibility)

	return &api.DevPortalResponse{
		ApiUrl:           devPortal.APIUrl,
		CreatedAt:        devPortal.CreatedAt,
		Description:      utils.StringPtrIfNotEmpty(devPortal.Description),
		HeaderKeyName:    utils.StringPtrIfNotEmpty(devPortal.HeaderKeyName),
		Hostname:         devPortal.Hostname,
		Identifier:       devPortal.Identifier,
		IsActive:         devPortal.IsActive,
		IsDefault:        devPortal.IsDefault,
		IsEnabled:        devPortal.IsEnabled,
		Name:             devPortal.Name,
		OrganizationUuid: orgUUID,
		UiUrl:            devPortal.GetUIUrl(),
		UpdatedAt:        devPortal.UpdatedAt,
		Uuid:             portalUUID,
		Visibility:       visibility,
	}, nil
}
