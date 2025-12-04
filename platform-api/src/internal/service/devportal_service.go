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
	"strings"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/client/devportal_client"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
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
}

// NewDevPortalService creates a new DevPortalService with optimized data fetching
func NewDevPortalService(
	devPortalRepo repository.DevPortalRepository,
	orgRepo repository.OrganizationRepository,
	publicationRepo repository.APIPublicationRepository,
	apiRepo repository.APIRepository,
	apiUtil *utils.APIUtil,
	config *config.Server,
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
	devPortal := &model.DevPortal{
		UUID:             uuid.New().String(),
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
func (s *DevPortalService) CreateDevPortal(orgUUID string, req *dto.CreateDevPortalRequest) (*dto.DevPortalResponse, error) {
	// Get organization details to derive identifier (cache for reuse)
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Failed to get organization %s for DevPortal creation: %v", orgUUID, err)
		return nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		log.Printf("[DevPortalService] Organization %s not found for DevPortal creation", orgUUID)
		return nil, constants.ErrOrganizationNotFound
	}

	// Convert request to model
	devPortal := req.ToModel(orgUUID)

	log.Printf("[DevPortalService] Attempting to create DevPortal %s for organization %s", devPortal.Name, orgUUID)

	// Use common method to create with sync (don't allow sync failure for user-created DevPortals)
	if err := s.createDevPortalWithSync(devPortal, org, false); err != nil {
		log.Printf("[DevPortalService] Failed to create DevPortal %s for organization %s: %v", devPortal.Name, orgUUID, err)
		return nil, err
	}

	log.Printf("[DevPortalService] Successfully created DevPortal %s for organization %s", devPortal.Name, orgUUID)
	return dto.ToDevPortalResponse(devPortal), nil
}

// EnableDevPortal enables a DevPortal for use (activates/syncs it first if needed)
func (s *DevPortalService) EnableDevPortal(uuid, orgUUID string) error {
	// Get DevPortal (cache for reuse)
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Failed to get DevPortal %s for organization %s during enable: %v", uuid, orgUUID, err)
		return err
	}

	if devPortal.IsEnabled {
		log.Printf("[DevPortalService] DevPortal %s for organization %s is already enabled", uuid, orgUUID)
		return nil
	}

	log.Printf("[DevPortalService] Attempting to enable DevPortal %s for organization %s", uuid, orgUUID)

	// Get organization (cache for reuse)
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Failed to get organization %s for DevPortal %s enable: %v", orgUUID, uuid, err)
		return fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		log.Printf("[DevPortalService] Organization %s not found for DevPortal %s enable", orgUUID, uuid)
		return constants.ErrOrganizationNotFound
	}

	// If DevPortal is not activated (synced), activate it first
	if !devPortal.IsActive {
		log.Printf("[DevPortalService] DevPortal %s not active, attempting to sync and initialize", uuid)
		if err := s.syncAndInitializeDevPortalInternal(devPortal, org); err != nil {
			log.Printf("[DevPortalService] Failed to sync and initialize DevPortal %s for organization %s: %v", uuid, orgUUID, err)
			return fmt.Errorf("failed to sync and initialize DevPortal during enable: %w", err)
		}
		// Mark as active in memory
		devPortal.IsActive = true
	}

	// Enable the DevPortal (set IsEnabled = true) and update atomically
	devPortal.IsEnabled = true
	devPortal.UpdatedAt = time.Now()

	if err := s.devPortalRepo.Update(devPortal, orgUUID); err != nil {
		log.Printf("[DevPortalService] Failed to update DevPortal %s in repository during enable: %v", uuid, err)
		return fmt.Errorf("failed to enable DevPortal: %w", err)
	}

	log.Printf("[DevPortalService] Successfully enabled DevPortal %s for organization %s", uuid, orgUUID)
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
				log.Printf("[DevPortalService] Failed to rollback DevPortal creation: %v", deleteErr)
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
		log.Printf("[DevPortalService] Failed to sync organization %s to DevPortal %s: %v", organization.ID, devPortal.Name, err)
		return err
	}

	// Create default subscription policy
	if err := s.devPortalClientSvc.CreateDefaultSubscriptionPolicy(devPortal); err != nil {
		log.Printf("[DevPortalService] Failed to create default subscription policy for DevPortal %s: %v", devPortal.Name, err)
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
func (s *DevPortalService) GetDevPortal(uuid, orgUUID string) (*dto.DevPortalResponse, error) {
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

// ListDevPortals lists DevPortals for an organization with optional filters
func (s *DevPortalService) ListDevPortals(orgUUID string, isDefault, isEnabled *bool, limit, offset int) (*dto.DevPortalListResponse, error) {
	devPortals, err := s.devPortalRepo.GetByOrganizationUUID(orgUUID, isDefault, isEnabled, limit, offset)
	if err != nil {
		return nil, err
	}

	// Convert to response DTOs
	responses := make([]*dto.DevPortalResponse, len(devPortals))
	for i, devPortal := range devPortals {
		responses[i] = dto.ToDevPortalResponse(devPortal)
	}

	totalCount, err := s.devPortalRepo.CountByOrganizationUUID(orgUUID, isDefault, isEnabled)
	if err != nil {
		return nil, err
	}

	return &dto.DevPortalListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  int(totalCount),
		},
	}, nil
}

// UpdateDevPortal updates an existing DevPortal
func (s *DevPortalService) UpdateDevPortal(uuid, orgUUID string, req *dto.UpdateDevPortalRequest) (*dto.DevPortalResponse, error) {
	// Get existing DevPortal
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Failed to get DevPortal %s for organization %s during update: %v", uuid, orgUUID, err)
		return nil, err
	}

	log.Printf("[DevPortalService] Attempting to update DevPortal %s for organization %s", uuid, orgUUID)

	// Update fields from request
	if req.Name != nil {
		devPortal.Name = *req.Name
	}
	if req.APIUrl != nil {
		devPortal.APIUrl = *req.APIUrl
	}
	if req.Hostname != nil {
		devPortal.Hostname = *req.Hostname
	}
	if req.APIKey != nil {
		devPortal.APIKey = *req.APIKey
	}
	if req.HeaderKeyName != nil {
		devPortal.HeaderKeyName = *req.HeaderKeyName
	}
	if req.Visibility != nil {
		devPortal.Visibility = *req.Visibility
	}
	if req.Description != nil {
		devPortal.Description = *req.Description
	}
	devPortal.UpdatedAt = time.Now()

	// Update in repository
	if err := s.devPortalRepo.Update(devPortal, orgUUID); err != nil {
		log.Printf("[DevPortalService] Failed to update DevPortal %s in repository: %v", uuid, err)
		return nil, err
	}

	log.Printf("[DevPortalService] Successfully updated DevPortal %s for organization %s", uuid, orgUUID)
	return dto.ToDevPortalResponse(devPortal), nil
}

// DeleteDevPortal deletes a DevPortal
func (s *DevPortalService) DeleteDevPortal(uuid, orgUUID string) error {
	log.Printf("[DevPortalService] Attempting to delete DevPortal %s for organization %s", uuid, orgUUID)
	err := s.devPortalRepo.Delete(uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Failed to delete DevPortal %s for organization %s: %v", uuid, orgUUID, err)
		return err
	}
	log.Printf("[DevPortalService] Successfully deleted DevPortal %s for organization %s", uuid, orgUUID)
	return nil
}

// DisableDevPortal disables a DevPortal for use
func (s *DevPortalService) DisableDevPortal(uuid, orgUUID string) error {
	log.Printf("[DevPortalService] Attempting to disable DevPortal %s for organization %s", uuid, orgUUID)
	// Attempt to disable DevPortal in a single repository call (no prior fetch)
	if err := s.devPortalRepo.UpdateEnabledStatus(uuid, orgUUID, false); err != nil {
		log.Printf("[DevPortalService] Failed to disable DevPortal %s for organization %s: %v", uuid, orgUUID, err)
		return fmt.Errorf("failed to disable DevPortal: %w", err)
	}
	log.Printf("[DevPortalService] Successfully disabled DevPortal %s for organization %s", uuid, orgUUID)
	return nil
}

// SetAsDefault sets a DevPortal as the default for its organization
func (s *DevPortalService) SetAsDefault(uuid, orgUUID string) error {
	log.Printf("[DevPortalService] Attempting to set DevPortal %s as default for organization %s", uuid, orgUUID)
	// Get DevPortal to ensure it exists
	_, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Failed to get DevPortal %s for organization %s during set as default: %v", uuid, orgUUID, err)
		return err
	}

	// Set as default in repository
	if err := s.devPortalRepo.SetAsDefault(uuid, orgUUID); err != nil {
		log.Printf("[DevPortalService] Failed to set DevPortal %s as default for organization %s: %v", uuid, orgUUID, err)
		return err
	}
	log.Printf("[DevPortalService] Successfully set DevPortal %s as default for organization %s", uuid, orgUUID)
	return nil
}

// GetDefaultDevPortal retrieves the default DevPortal for an organization
func (s *DevPortalService) GetDefaultDevPortal(orgUUID string) (*dto.DevPortalResponse, error) {
	devPortal, err := s.devPortalRepo.GetDefaultByOrganizationUUID(orgUUID)
	if err != nil {
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

// PublishAPIToDevPortal publishes an API to a DevPortal
func (s *DevPortalService) PublishAPIToDevPortal(api *dto.API, req *dto.PublishToDevPortalRequest, orgUUID string) error {
	// --- Phase 1: Validate Inputs ---
	devPortal, org, err := s.validatePublishInputs(req, orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Input validation failed for API %s: %v", api.ID, err)
		return err
	}

	// --- Phase 2: Prepare Publication ---
	err = s.prepareAPIPublication(api, req, devPortal, orgUUID)
	if err != nil {
		log.Printf("[DevPortalService] Publication preparation failed for API %s: %v", api.ID, err)
		return err
	}

	// --- Phase 3: Build API Metadata ---
	apiMetadata, err := s.prepareAPIMetadata(api, req)
	if err != nil {
		log.Printf("[DevPortalService] Metadata preparation failed for API %s: %v", api.ID, err)
		return err
	}

	fmt.Printf("[DevPortalService] Publishing API %s to DevPortal %s\n", api.ID, devPortal.Name)

	// --- Phase 4: Publish API to DevPortal ---
	err = s.publishToDevPortal(api, org, devPortal, apiMetadata, req)
	if err != nil {
		fmt.Printf("[DevPortalService] Failed to publish API %s to DevPortal %s: %v\n", api.ID, devPortal.Name, err)
		return err
	}

	return nil
}

// validatePublishInputs validates DevPortal and Organization for publishing
func (s *DevPortalService) validatePublishInputs(req *dto.PublishToDevPortalRequest, orgUUID string) (*model.DevPortal, *model.Organization, error) {
	devPortal, err := s.getDevPortalByUUID(req.DevPortalUUID, orgUUID)
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
func (s *DevPortalService) prepareAPIPublication(api *dto.API, req *dto.PublishToDevPortalRequest, devPortal *model.DevPortal, orgUUID string) error {
	// Check if already published (prevent duplicates)
	existing, err := s.publicationRepo.GetByAPIAndDevPortal(api.ID, req.DevPortalUUID, orgUUID)
	if err != nil && !errors.Is(err, constants.ErrAPIPublicationNotFound) {
		return fmt.Errorf("failed to check existing publication: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("API already published to this DevPortal")
	}

	return nil
}

// prepareAPIMetadata builds API metadata request with defaults and user overrides
func (s *DevPortalService) prepareAPIMetadata(api *dto.API, req *dto.PublishToDevPortalRequest) (devportal_client.APIMetadataRequest, error) {
	// Default values - system fields from API
	apiInfo := devportal_client.APIInfo{
		APIID:          api.ID,
		ReferenceID:    api.ID,
		APIName:        api.Name,
		APIHandle:      sanitizeAPIHandle(api.Context),
		APIVersion:     api.Version,
		APIType:        devportal_client.APIType("REST"),
		Provider:       api.Provider,
		APIDescription: api.Description,
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

	// Apply user overrides
	if v := req.APIInfo; v != nil {
		if v.APIName != "" {
			apiInfo.APIName = v.APIName
		}
		if v.APIDescription != "" {
			apiInfo.APIDescription = v.APIDescription
		}
		if v.APIType != "" {
			apiInfo.APIType = devportal_client.APIType(v.APIType)
		}
		if v.Visibility != "" {
			apiInfo.Visibility = devportal_client.APIVisibility(v.Visibility)
		}
		if len(v.VisibleGroups) > 0 {
			apiInfo.VisibleGroups = v.VisibleGroups
		}
		if len(v.Tags) > 0 {
			apiInfo.Tags = v.Tags
		}
		if len(v.Labels) > 0 {
			apiInfo.Labels = v.Labels
		}
		apiInfo.Owners = devportal_client.Owners(v.Owners)
	}

	// Validate the APIInfo
	if err := s.validator.Struct(apiInfo); err != nil {
		return devportal_client.APIMetadataRequest{}, err
	}

	// Convert subscription policies from strings to objects
	subscriptionPolicies := make([]devportal_client.SubscriptionPolicyRequest, len(req.SubscriptionPolicies))
	for i, policyName := range req.SubscriptionPolicies {
		subscriptionPolicies[i] = devportal_client.SubscriptionPolicyRequest{
			PolicyName: policyName,
		}
	}

	apiMetadata := devportal_client.APIMetadataRequest{
		APIInfo: apiInfo,
		EndPoints: devportal_client.EndPoints{
			ProductionURL: req.EndPoints.ProductionURL,
			SandboxURL:    req.EndPoints.SandboxURL,
		},
		SubscriptionPolicies: subscriptionPolicies,
	}

	// Validate the entire API metadata request
	if err := s.validator.Struct(apiMetadata); err != nil {
		log.Printf("[DevPortalService] Validation failed for API metadata: %v", err)
		return devportal_client.APIMetadataRequest{}, err
	}

	return apiMetadata, nil
}

// publishToDevPortal handles the actual DevPortal API call and publication record updates
func (s *DevPortalService) publishToDevPortal(
	api *dto.API,
	org *model.Organization,
	devPortal *model.DevPortal,
	apiMetadata devportal_client.APIMetadataRequest,
	req *dto.PublishToDevPortalRequest,
) error {

	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Check if API exists in DevPortal
	exists, err := s.devPortalClientSvc.CheckAPIExists(client, org.ID, api.ID)
	if err != nil {
		log.Printf("API publication failed for API %s to DevPortal %s: %v", api.ID, devPortal.Name, err)
		return fmt.Errorf("failed to check if API exists in DevPortal: %w", err)
	}
	if exists {
		log.Printf("API publication failed for API %s to DevPortal %s: API already exists", api.ID, devPortal.Name)
		return fmt.Errorf("API %s already exists in DevPortal %s", api.ID, devPortal.Name)
	}

	// Generate OpenAPI definition
	apiDef, err := s.apiUtil.GenerateOpenAPIDefinition(api, &apiMetadata)
	if err != nil {
		log.Printf("API publication failed for API %s to DevPortal %s: %v", api.ID, devPortal.Name, err)
		return fmt.Errorf("failed to generate OpenAPI definition for API %s: %w", api.ID, err)
	}

	// CRITICAL SECTION: DevPortal publication with transactional compensation
	var devPortalRefID *string

	// Step 1: Publish to DevPortal
	devPortalResponse, err := s.devPortalClientSvc.PublishAPIToDevPortal(client, org.ID, apiMetadata, apiDef)
	if err != nil {
		log.Printf("API publication failed for API %s to DevPortal %s: %v", api.ID, devPortal.Name, err)
		return err
	}

	devPortalRefID = &devPortalResponse.ID
	log.Printf("Successfully published API %s to DevPortal %s with reference ID: %s",
		api.ID, devPortal.Name, *devPortalRefID)

	// Step 2: Create publication record with compensation on failure
	publication := &model.APIPublication{
		APIUUID:               api.ID,
		DevPortalUUID:         devPortal.UUID,
		OrganizationUUID:      org.ID,
		Status:                model.PublishedStatus,
		APIVersion:            &api.Version,
		DevPortalRefID:        devPortalRefID,
		SandboxEndpointURL:    req.EndPoints.SandboxURL,
		ProductionEndpointURL: req.EndPoints.ProductionURL,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	// Step 3: Save with compensation handling
	err = s.savePublicationWithCompensation(publication, client, org.ID, api.ID, *devPortalRefID, devPortal.Name, devPortal)
	if err != nil {
		return err
	}

	log.Printf("[DevPortalService] API %s successfully published to DevPortal %s", api.ID, devPortal.Name)

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
		log.Printf("Validation failed for API %s — triggering compensation rollback", apiID)

		if compensationErr := s.compensatePublication(client, orgID, apiID, devPortalRefID, devPortalName, err, "validation failed"); compensationErr != nil {
			return compensationErr
		}

		return err
	}

	// 2. Try saving to database
	if err := s.publicationRepo.Create(publication); err != nil {
		log.Printf("Database save failed for API %s — initiating compensation rollback", apiID)

		if compensationErr := s.compensatePublication(client, orgID, apiID, devPortalRefID, devPortalName, err, "database save failed"); compensationErr != nil {
			return compensationErr
		}

		return fmt.Errorf("%w: %w", constants.ErrAPIPublicationSaveFailed, err)
	}

	// Create API-DevPortal association after successful publication
	association := &model.APIAssociation{
		ApiID:           apiID,
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
			log.Printf("Failed to create API association after successful publication for API %s: %v", apiID, err)
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
	utils.LogError("Starting rollback - removing API from DevPortal due to publication failure", nil)

	// Retry rollback up to 3 times with exponential backoff
	maxRetries := 3
	var rollbackErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rollbackErr = s.devPortalClientSvc.UnpublishAPIFromDevPortal(client, orgID, apiID)
		if rollbackErr == nil {
			// Success - rollback completed
			utils.LogError(fmt.Sprintf("Compensation completed: removed API %s from DevPortal %s due to %s", apiID, devPortalName, failureReason), nil)
			return nil
		}

		// Check if this is a "not found" error (404) - API already removed
		wrappedErr := utils.WrapDevPortalClientError(rollbackErr)
		if errors.Is(wrappedErr, constants.ErrAPINotFound) {
			utils.LogError(fmt.Sprintf("Compensation completed: API %s already removed from DevPortal %s (404)", apiID, devPortalName), nil)
			return nil
		}

		// If not the last attempt, wait before retrying
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * time.Second // 1s, 2s, 3s
			utils.LogError(fmt.Sprintf("Rollback attempt %d failed, retrying in %v: %v", attempt, waitTime, rollbackErr), nil)
			time.Sleep(waitTime)
		}
	}

	// All retries failed
	utils.LogError("Rollback failed after all retries - unable to remove API from DevPortal", rollbackErr)

	// Critical—Unable to maintain consistency (Split-brain situation)
	utils.LogError("Critical error - API published but database update failed and cleanup unsuccessful. Manual intervention required", nil)

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
		log.Printf("[DevPortalService] Failed to get DevPortal %s for unpublish: %v", devPortalUUID, err)
		return err
	}

	// Create DevPortal client for unpublishing
	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Unpublish API using client service
	err = s.devPortalClientSvc.UnpublishAPIFromDevPortal(client, orgID, apiID)
	if err != nil {
		log.Printf("[DevPortalService] DevPortal client unpublish failed for API %s from DevPortal %s: %v", apiID, devPortal.Name, err)

		// Check if this is a "not found" error (404) by wrapping and checking for ErrAPINotFound
		wrappedErr := utils.WrapDevPortalClientError(err)
		if errors.Is(wrappedErr, constants.ErrAPINotFound) {
			log.Printf("[DevPortalService] API %s not found in DevPortal %s (404), treating as already unpublished", apiID, devPortal.Name)
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
	// NOTE: We intentionally keep the api_associations record to maintain the association history
	if err := s.publicationRepo.Delete(apiID, devPortalUUID, orgID); err != nil {
		// Log error but don't fail the unpublish operation if record doesn't exist
		if !errors.Is(err, constants.ErrAPIPublicationNotFound) {
			log.Printf("[DevPortalService] Failed to delete publication record for API %s from DevPortal %s: %v", apiID, devPortal.Name, err)
			return fmt.Errorf("failed to delete publication record: %w", err)
		}
		log.Printf("[DevPortalService] Publication record not found for API %s from DevPortal %s, skipping delete", apiID, devPortal.Name)
	}

	log.Printf("[DevPortalService] Successfully unpublished API %s from DevPortal %s", apiID, devPortalUUID)
	return nil
}
