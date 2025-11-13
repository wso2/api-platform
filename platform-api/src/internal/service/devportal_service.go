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

	"github.com/google/uuid"
)

// Constants for DevPortal operations
const (
	DevPortalServiceTimeout   = 10
	PublicationStuckThreshold = 2 * time.Minute
)

// DevPortalService manages DevPortal operations using optimized data fetching and delegating client calls
type DevPortalService struct {
	devPortalRepo      repository.DevPortalRepository
	orgRepo            repository.OrganizationRepository
	publicationRepo    repository.APIPublicationRepository
	gatewayRepo        repository.GatewayRepository
	apiRepo            repository.APIRepository
	apiUtil            *utils.APIUtil
	config             *config.Server          // Global configuration for role mapping
	devPortalClientSvc *DevPortalClientService // Handles all DevPortal client interactions
}

// NewDevPortalService creates a new DevPortalService with optimized data fetching
func NewDevPortalService(
	devPortalRepo repository.DevPortalRepository,
	orgRepo repository.OrganizationRepository,
	publicationRepo repository.APIPublicationRepository,
	gatewayRepo repository.GatewayRepository,
	apiRepo repository.APIRepository,
	apiUtil *utils.APIUtil,
	config *config.Server,
) *DevPortalService {
	return &DevPortalService{
		devPortalRepo:      devPortalRepo,
		orgRepo:            orgRepo,
		publicationRepo:    publicationRepo,
		gatewayRepo:        gatewayRepo,
		apiRepo:            apiRepo,
		apiUtil:            apiUtil,
		config:             config,
		devPortalClientSvc: NewDevPortalClientService(config),
	}
}

// getDevPortalByUUID retrieves a DevPortal by UUID with error handling
func (s *DevPortalService) getDevPortalByUUID(uuid, orgUUID string) (*model.DevPortal, error) {
	devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devPortal %s: %w", uuid, err)
	}
	if devPortal == nil {
		return nil, fmt.Errorf("devPortal %s not found", uuid)
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
		return nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		return nil, fmt.Errorf("organization %s not found", orgUUID)
	}

	// Convert request to model
	devPortal := req.ToModel(orgUUID)

	// Use common method to create with sync (don't allow sync failure for user-created DevPortals)
	if err := s.createDevPortalWithSync(devPortal, org, false); err != nil {
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

// EnableDevPortal enables a DevPortal for use (activates/syncs it first if needed)
func (s *DevPortalService) EnableDevPortal(uuid, orgUUID string) (*dto.ActivateDevPortalResponse, error) {
	// Get DevPortal (cache for reuse)
	devPortal, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	if devPortal.IsEnabled {
		return &dto.ActivateDevPortalResponse{
			Message:       "DevPortal is already enabled",
			DevPortalUUID: uuid,
			DevPortalName: devPortal.Name,
			ActivatedAt:   time.Now(),
		}, nil
	}

	// Get organization (cache for reuse)
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		return nil, fmt.Errorf("organization %s not found", orgUUID)
	}

	// If DevPortal is not activated (synced), activate it first
	if !devPortal.IsActive {
		if err := s.syncAndInitializeDevPortalInternal(devPortal, org); err != nil {
			if errors.Is(err, constants.ErrDevPortalAlreadyExist) {
				// Organization already exists remotely - this is OK!
				// It means the organization was previously synced to the remote DevPortal
				// We should keep the DB record and mark it as active
				log.Printf("[DevPortalService] Organization %s already exists in DevPortal %s - marking as active", org.ID, devPortal.Name)
				devPortal.IsActive = true
				devPortal.IsEnabled = true
			} else {
				return nil, fmt.Errorf("failed to sync and initialize DevPortal during enable: %w", err)
			}
		}
		// Mark as active in memory
		devPortal.IsActive = true
	}

	// Enable the DevPortal (set IsEnabled = true) and update atomically
	devPortal.IsEnabled = true
	devPortal.UpdatedAt = time.Now()

	if err := s.devPortalRepo.Update(devPortal, orgUUID); err != nil {
		return nil, fmt.Errorf("failed to enable DevPortal: %w", err)
	}

	return &dto.ActivateDevPortalResponse{
		Message:       "DevPortal enabled successfully",
		DevPortalUUID: uuid,
		DevPortalName: devPortal.Name,
		ActivatedAt:   time.Now(),
	}, nil
}

// createDevPortalWithSync creates a DevPortal and attempts to sync it gracefully
func (s *DevPortalService) createDevPortalWithSync(devPortal *model.DevPortal, organization *model.Organization, allowSyncFailure bool) error {
	// Create DevPortal in repository first
	if err := s.devPortalRepo.Create(devPortal); err != nil {
		return fmt.Errorf("failed to create DevPortal: %w", err)
	}

	// Attempt to sync and initialize (handle gracefully based on allowSyncFailure flag)
	if err := s.syncAndInitializeDevPortalInternal(devPortal, organization); err != nil {
		// Check if organization already exists in DevPortal (remote side)
		if errors.Is(err, constants.ErrDevPortalAlreadyExist) {
			// Organization already exists remotely - this is OK!
			// It means the organization was previously synced to the remote DevPortal
			// We should keep the DB record and mark it as active
			log.Printf("[DevPortalService] Organization %s already exists in DevPortal %s - marking as active", organization.ID, devPortal.Name)
			devPortal.IsActive = true
			devPortal.IsEnabled = true
		} else {
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
		}
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
	devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
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
		return nil, err
	}

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
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

// DeleteDevPortal deletes a DevPortal
func (s *DevPortalService) DeleteDevPortal(uuid, orgUUID string) error {
	err := s.devPortalRepo.Delete(uuid, orgUUID)
	if err != nil {
		return err
	}
	return nil
}

// DisableDevPortal disables a DevPortal for use
func (s *DevPortalService) DisableDevPortal(uuid, orgUUID string) (*dto.DeactivateDevPortalResponse, error) {
	// Attempt to disable DevPortal in a single repository call (no prior fetch)
	if err := s.devPortalRepo.UpdateEnabledStatus(uuid, orgUUID, false); err != nil {
		return nil, fmt.Errorf("failed to disable DevPortal: %w", err)
	}
	return &dto.DeactivateDevPortalResponse{
		Message:       "DevPortal disabled successfully",
		DevPortalUUID: uuid,
		DevPortalName: "",
		DeactivatedAt: time.Now(),
	}, nil
}

// SetAsDefault sets a DevPortal as the default for its organization
func (s *DevPortalService) SetAsDefault(uuid, orgUUID string) error {
	// Get DevPortal to ensure it exists
	_, err := s.getDevPortalByUUID(uuid, orgUUID)
	if err != nil {
		return err
	}

	// Set as default in repository
	return s.devPortalRepo.SetAsDefault(uuid, orgUUID)
}

// GetDefaultDevPortal retrieves the default DevPortal for an organization
func (s *DevPortalService) GetDefaultDevPortal(orgUUID string) (*dto.DevPortalResponse, error) {
	devPortal, err := s.devPortalRepo.GetDefaultByOrganizationUUID(orgUUID)
	if err != nil {
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

func (s *DevPortalService) PublishAPIToDevPortal(api *dto.API, req *dto.PublishToDevPortalRequest, orgUUID string) (*dto.PublishToDevPortalResponse, error) {
	// --- Phase 1: Validate Inputs ---
	devPortal, org, err := s.validatePublishInputs(req, orgUUID)
	if err != nil {
		return nil, err
	}

	// --- Phase 2: Prepare or Resume Publication ---
	publication, isRetry, err := s.preparePublication(api, req, devPortal, orgUUID)
	if err != nil {
		return nil, err
	}

	// --- Phase 3: Build API Metadata ---
	apiMetadata := s.prepareAPIMetadata(api, req, org)

	fmt.Printf("[DevPortalService] Publishing API %s to DevPortal %s (isRetry=%v)\n", api.ID, devPortal.Name, isRetry)

	// --- Phase 4: Publish API to DevPortal ---
	response, err := s.publishToDevPortal(api, org, devPortal, publication, apiMetadata, isRetry)
	if err != nil {
		fmt.Printf("[DevPortalService] Failed to publish API %s to DevPortal %s: %v\n", api.ID, devPortal.Name, err)
		return nil, err
	}

	return response, nil
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

// preparePublication handles publication record creation/update based on current state
func (s *DevPortalService) preparePublication(api *dto.API, req *dto.PublishToDevPortalRequest, devPortal *model.DevPortal, orgUUID string) (*model.APIPublication, bool, error) {
	existing, err := s.publicationRepo.GetByAPIAndDevPortal(api.ID, req.DevPortalUUID, orgUUID)
	if err != nil && !errors.Is(err, constants.ErrAPIPublicationNotFound) {
		return nil, false, fmt.Errorf("failed to check existing publication: %w", err)
	}


	var pub *model.APIPublication
	isRetry := false

	switch {
	case existing == nil:
		// New publication - create both association and publication

		// Step 1: Create API-DevPortal association if it doesn't exist
		association := &model.APIAssociation{
			ApiID:           api.ID,
			OrganizationID:  orgUUID,
			ResourceID:      devPortal.UUID,
			AssociationType: "dev_portal",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
			// Check if this is a duplicate key error (association already exists)
			if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
				strings.Contains(err.Error(), "duplicate key") {
			} else {
				return nil, false, fmt.Errorf("failed to create API association: %w", err)
			}
		}

		// Step 2: Create publication record
		pub = &model.APIPublication{
			APIUUID:               api.ID,
			DevPortalUUID:         devPortal.UUID,
			OrganizationUUID:      orgUUID,
			Status:                model.PublishingStatus,
			APIVersion:            &api.Version,
			SandboxEndpointURL:    req.EndPoints.SandboxURL,
			ProductionEndpointURL: req.EndPoints.ProductionURL,
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
		}
		if err := s.publicationRepo.Create(pub); err != nil {
			return nil, false, fmt.Errorf("failed to create publication: %w", err)
		}

	case existing.Status == model.PublishedStatus:
		// Already published – do nothing, just return success
		return existing, false, nil

	case existing.Status == model.FailedStatus:
		// Retry failed publication - update all fields including endpoints
		isRetry = true
		pub = existing
		pub.Status = model.PublishingStatus
		pub.APIVersion = &api.Version
		pub.SandboxEndpointURL = req.EndPoints.SandboxURL
		pub.ProductionEndpointURL = req.EndPoints.ProductionURL
		pub.UpdatedAt = time.Now()
		if err := s.publicationRepo.Update(pub); err != nil {
			return nil, false, fmt.Errorf("failed to update failed publication record: %w", err)
		}

	case existing.Status == model.PublishingStatus:
		// Check if publication is stuck (older than threshold duration)
		stuckThreshold := time.Now().Add(-PublicationStuckThreshold)
		if existing.UpdatedAt.Before(stuckThreshold) {
			// Treat as failed and retry
			isRetry = true
			pub = existing
			pub.Status = model.PublishingStatus
			pub.APIVersion = &api.Version
			pub.SandboxEndpointURL = req.EndPoints.SandboxURL
			pub.ProductionEndpointURL = req.EndPoints.ProductionURL
			pub.UpdatedAt = time.Now()
			if err := s.publicationRepo.Update(pub); err != nil {
				return nil, false, fmt.Errorf("failed to update stuck publication record: %w", err)
			}
		} else {
			return nil, false, constants.ErrAPIPublicationInProgress
		}

	default:
		return nil, false, fmt.Errorf("unknown publication status: %s", existing.Status)
	}

	return pub, isRetry, nil
}

// prepareAPIMetadata builds API metadata request with defaults and user overrides
func (s *DevPortalService) prepareAPIMetadata(api *dto.API, req *dto.PublishToDevPortalRequest, org *model.Organization) devportal_client.APIMetadataRequest {
	// Default values - system fields from API
	apiInfo := devportal_client.APIInfo{
		APIID:          api.ID,
		ReferenceID:    api.ID,
		APIName:        api.Name,
		APIHandle:      api.Context,
		APIVersion:     api.Version,
		APIType:        devportal_client.APIType("REST"),
		Provider:       api.Provider,
		APIDescription: api.Description,
		APIStatus:      "PUBLISHED",
		Visibility:     devportal_client.APIVisibility("PUBLIC"),
		Labels:         []string{"default"},
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

	return devportal_client.APIMetadataRequest{
		APIInfo: apiInfo,
		EndPoints: devportal_client.EndPoints{
			ProductionURL: req.EndPoints.ProductionURL,
			SandboxURL:    req.EndPoints.SandboxURL,
		},
		SubscriptionPolicies: req.SubscriptionPolicies,
	}
}

// publishToDevPortal handles the actual DevPortal API call and publication record updates
func (s *DevPortalService) publishToDevPortal(
	api *dto.API,
	org *model.Organization,
	devPortal *model.DevPortal,
	pub *model.APIPublication,
	apiMetadata devportal_client.APIMetadataRequest,
	isRetry bool,
) (*dto.PublishToDevPortalResponse, error) {

	// If publication is already published (from update case), return success immediately
	if pub.Status == model.PublishedStatus {
		refID := ""
		if pub.DevPortalRefID != nil {
			refID = *pub.DevPortalRefID
		}
		return &dto.PublishToDevPortalResponse{
			Message:        fmt.Sprintf("API already published to DevPortal '%s'", devPortal.Name),
			APIID:          api.ID,
			DevPortalUUID:  devPortal.UUID,
			DevPortalName:  devPortal.Name,
			ApiPortalRefID: refID,
			PublishedAt:    time.Now(),
		}, nil
	}

	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Check if API exists (skip for retry)
	if !isRetry {
		exists, err := s.devPortalClientSvc.CheckAPIExists(client, org.ID, api.ID)
		if err != nil {
			s.markFailed(pub)
			return nil, fmt.Errorf("failed to check if API exists: %w", err)
		}
		if exists {
			s.markFailed(pub)
			return nil, fmt.Errorf("API %s already exists in DevPortal %s", api.ID, devPortal.Name)
		}
	}
	// Generate OpenAPI definition
	apiDef, err := s.apiUtil.GenerateOpenAPIDefinition(api)
	if err != nil {
		s.markFailed(pub)
		return nil, fmt.Errorf("failed to generate OpenAPI definition for API %s: %w", api.ID, err)
	}

	resp, err := s.devPortalClientSvc.PublishAPIToDevPortal(client, org.ID, apiMetadata, apiDef)
	if err != nil {
		s.markFailed(pub)
		return nil, fmt.Errorf("failed to publish API to DevPortal: %w", err)
	}

	// Update publication record
	pub.Status = model.PublishedStatus
	pub.DevPortalRefID = &resp.ID
	pub.UpdatedAt = time.Now()
	if err := s.publicationRepo.Update(pub); err != nil {
		return nil, fmt.Errorf("failed to update publication record: %w", err)
	}

	msg := fmt.Sprintf("API published successfully to DevPortal '%s'", devPortal.Name)
	if isRetry {
		msg += " (retry)"
	}

	return &dto.PublishToDevPortalResponse{
		Message:        msg,
		APIID:          api.ID,
		DevPortalUUID:  devPortal.UUID,
		DevPortalName:  devPortal.Name,
		ApiPortalRefID: resp.ID,
		PublishedAt:    time.Now(),
	}, nil
}

// markFailed marks a publication as failed
func (s *DevPortalService) markFailed(pub *model.APIPublication) {
	pub.Status = model.FailedStatus
	pub.UpdatedAt = time.Now()
	_ = s.publicationRepo.Update(pub)
}

// UnpublishAPIFromDevPortal unpublishes an API from a DevPortal
// Note: This removes the publication record but keeps the API-DevPortal association intact
// The DevPortal will still be listed as "associated" but not "published" in GetAPIPublications
func (s *DevPortalService) UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID string) (*dto.UnpublishFromDevPortalResponse, error) {

	// Get DevPortal
	devPortal, err := s.getDevPortalByUUID(devPortalUUID, orgID)
	if err != nil {
		return nil, err
	}

	// Create DevPortal client for unpublishing
	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Unpublish API using client service
	err = s.devPortalClientSvc.UnpublishAPIFromDevPortal(client, orgID, apiID)
	if err != nil {
		// If API not found in DevPortal, treat as success (idempotent unpublish)
		if !errors.Is(err, devportal_client.ErrAPINotFound) {
			return nil, fmt.Errorf("failed to unpublish API from DevPortal: %w", err)
		}
		// API already gone from DevPortal - proceed to delete local record
	}

	// Delete publication record after successful unpublish
	// NOTE: We intentionally keep the api_associations record to maintain the association history
	if err := s.publicationRepo.Delete(apiID, devPortalUUID, orgID); err != nil {
		// Log error but don't fail the unpublish operation if record doesn't exist
		if !errors.Is(err, constants.ErrAPIPublicationNotFound) {
			return nil, fmt.Errorf("failed to delete publication record: %w", err)
		}
	}

	return &dto.UnpublishFromDevPortalResponse{
		Message:       fmt.Sprintf("API unpublished successfully from DevPortal '%s'", devPortal.Name),
		APIID:         apiID,
		DevPortalUUID: devPortal.UUID,
		DevPortalName: devPortal.Name,
		UnpublishedAt: time.Now(),
	}, nil
}
