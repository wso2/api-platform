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
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"strings"
	"time"

	"platform-api/src/internal/client/devportal"
	devportalDto "platform-api/src/internal/client/devportal/dto"

	"github.com/google/uuid"
)

// Constants for DevPortal operations
const (
	DefaultDevPortalTimeoutSeconds = 10
)

// DevPortalManager manages DevPortal operations
type DevPortalManager struct {
	devPortalRepo   repository.DevPortalRepository
	orgRepo         repository.OrganizationRepository
	publicationRepo repository.APIPublicationRepository
	apiUtil         *utils.APIUtil
	config          *config.Server // Global configuration for role mapping
}

// NewDevPortalManager creates a new DevPortalManager
func NewDevPortalManager(devPortalRepo repository.DevPortalRepository, orgRepo repository.OrganizationRepository, publicationRepo repository.APIPublicationRepository, apiUtil *utils.APIUtil, config *config.Server) *DevPortalManager {
	return &DevPortalManager{
		devPortalRepo:   devPortalRepo,
		orgRepo:         orgRepo,
		publicationRepo: publicationRepo,
		apiUtil:         apiUtil,
		config:          config,
	}
}

// createDevPortalClient creates a DevPortalClient configured for the given DevPortal
func (m *DevPortalManager) createDevPortalClient(devPortal *model.DevPortal) *devportal.DevPortalClient {
	timeout := m.config.DefaultDevPortal.Timeout
	if timeout <= 0 {
		timeout = DefaultDevPortalTimeoutSeconds
	}

	// Use DevPortal-specific configuration from database
	return devportal.NewDevPortalClient(
		devPortal.APIUrl,        // DevPortal-specific API URL
		devPortal.APIKey,        // DevPortal-specific API key
		devPortal.HeaderKeyName, // DevPortal-specific header name
		timeout,                 // Global timeout from environment config
	)
}

// CreateDefaultDevPortal creates a default DevPortal for a new organization
func (m *DevPortalManager) CreateDefaultDevPortal(orgUUID string) (*model.DevPortal, error) {
	// Check if default DevPortal creation is enabled
	if !m.config.DefaultDevPortal.Enabled {
		log.Printf("[DevPortalManager] Default DevPortal creation is disabled")
		return nil, nil
	}

	// Get organization to derive UIUrl
	org, err := m.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to get organization %s: %v", orgUUID, err)
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	if org == nil {
		log.Printf("[DevPortalManager] Organization %s not found", orgUUID)
		return nil, fmt.Errorf("organization not found")
	}

	// Create DevPortal from default configuration
	// DevPortal starts as inactive and requires explicit activation
	devPortal := &model.DevPortal{
		UUID:             uuid.New().String(),
		OrganizationUUID: orgUUID,
		Name:             m.config.DefaultDevPortal.Name,
		Identifier:       org.Handle, // Derived from organization handle
		APIUrl:           m.config.DefaultDevPortal.APIUrl,
		Hostname:         m.config.DefaultDevPortal.Hostname,
		IsActive:         false, // Start inactive, requires explicit activation
		APIKey:           m.config.DefaultDevPortal.APIKey,
		HeaderKeyName:    m.config.DefaultDevPortal.HeaderKeyName,
		IsDefault:        true,
		Visibility:       "private", // Default DevPortals are private
		Description:      "Default DevPortal for organization",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Create DevPortal in repository
	if err := m.devPortalRepo.Create(devPortal); err != nil {
		log.Printf("[DevPortalManager] Failed to create default DevPortal for organization %s: %v", orgUUID, err)
		return nil, fmt.Errorf("failed to create default DevPortal: %w", err)
	}

	log.Printf("[DevPortalManager] Created default DevPortal %s (UUID: %s) for organization %s - inactive state",
		devPortal.Name, devPortal.UUID, orgUUID)

	// DevPortal created in inactive state - no organization sync until explicitly activated
	return devPortal, nil
}

// CreateDevPortal creates a new DevPortal for an organization
func (m *DevPortalManager) CreateDevPortal(orgUUID string, req *dto.CreateDevPortalRequest) (*dto.DevPortalResponse, error) {
	// Get organization details to derive identifier
	org, err := m.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to get organization %s: %v", orgUUID, err)
		return nil, err
	}
	if org == nil {
		log.Printf("[DevPortalManager] Organization %s not found when creating DevPortal", orgUUID)
		return nil, constants.ErrOrganizationNotFound
	}

	// Convert request to model
	devPortal := req.ToModel(orgUUID)

	// Create in repository
	if err := m.devPortalRepo.Create(devPortal); err != nil {
		log.Printf("[DevPortalManager] Failed to create DevPortal %s: %v", req.Name, err)
		return nil, err
	}

	log.Printf("[DevPortalManager] Created DevPortal %s (UUID: %s) for organization %s",
		devPortal.Name, devPortal.UUID, orgUUID)

	return dto.ToDevPortalResponse(devPortal), nil
}

// GetDevPortal retrieves a DevPortal by UUID
func (m *DevPortalManager) GetDevPortal(uuid, orgUUID string) (*dto.DevPortalResponse, error) {
	devPortal, err := m.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

// ListDevPortals lists DevPortals for an organization with optional filters
func (m *DevPortalManager) ListDevPortals(orgUUID string, isDefault, isActive *bool, limit, offset int) (*dto.DevPortalListResponse, error) {
	devPortals, err := m.devPortalRepo.GetByOrganizationUUID(orgUUID, isDefault, isActive, limit, offset)
	if err != nil {
		return nil, err
	}

	count, err := m.devPortalRepo.CountByOrganizationUUID(orgUUID, isDefault, isActive)
	if err != nil {
		return nil, err
	}

	pagination := dto.Pagination{
		Limit:  limit,
		Offset: offset,
		Total:  count,
	}

	return dto.ToDevPortalListResponse(devPortals, pagination), nil
}

// UpdateDevPortal updates an existing DevPortal
func (m *DevPortalManager) UpdateDevPortal(uuid, orgUUID string, req *dto.UpdateDevPortalRequest) (*dto.DevPortalResponse, error) {
	// Get existing DevPortal
	devPortal, err := m.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	// Apply updates
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
	if req.IsActive != nil {
		devPortal.IsActive = *req.IsActive
	}
	if req.Visibility != nil {
		devPortal.Visibility = *req.Visibility
	}
	if req.Description != nil {
		devPortal.Description = *req.Description
	}

	// Update in repository
	if err := m.devPortalRepo.Update(devPortal, orgUUID); err != nil {
		log.Printf("[DevPortalManager] Failed to update DevPortal %s: %v", uuid, err)
		return nil, err
	}

	log.Printf("[DevPortalManager] Updated DevPortal %s", uuid)
	return dto.ToDevPortalResponse(devPortal), nil
}

// DeleteDevPortal deletes a DevPortal
func (m *DevPortalManager) DeleteDevPortal(uuid, orgUUID string) error {
	devPortal, err := m.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return err
	}

	if devPortal.IsDefault {
		return constants.ErrDevPortalCannotDeleteDefault
	}

	if err := m.devPortalRepo.Delete(uuid, orgUUID); err != nil {
		log.Printf("[DevPortalManager] Failed to delete DevPortal %s: %v", uuid, err)
		return err
	}

	log.Printf("[DevPortalManager] Deleted DevPortal %s", uuid)
	return nil
}

// ActivateDevPortal activates a DevPortal and syncs the organization
func (m *DevPortalManager) ActivateDevPortal(uuid, orgUUID string) (*dto.ActivateDevPortalResponse, error) {
	devPortal, err := m.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	// Check if DevPortal can be activated using domain logic
	if err := devPortal.CanBeActivated(); err != nil {
		return nil, err
	}

	// Get organization details for sync
	org, err := m.orgRepo.GetOrganizationByUUID(devPortal.OrganizationUUID)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to get organization %s for DevPortal activation: %v", devPortal.OrganizationUUID, err)
		return nil, fmt.Errorf("failed to get organization for DevPortal activation: %w", err)
	}

	// Sync organization to DevPortal
	if err := m.syncOrganizationToDevPortal(devPortal, devPortal.OrganizationUUID); err != nil {
		log.Printf("[DevPortalManager] Failed to sync organization %s to DevPortal %s during activation: %v",
			devPortal.OrganizationUUID, devPortal.Name, err)

		// Return the specific error instead of wrapping it
		return nil, err
	}

	// Create default subscription policy for DevPortal
	// TODO : This needs to be replaces with proper subscription policy management
	if err := m.createDefaultSubscriptionPolicy(devPortal, devPortal.OrganizationUUID); err != nil {
		log.Printf("[DevPortalManager] Failed to create default subscription policy for DevPortal %s during activation: %v",
			devPortal.Name, err)

		// Return the specific error instead of wrapping it
		return nil, err
	}

	// Update DevPortal status to active
	if err := m.devPortalRepo.UpdateActiveStatus(uuid, orgUUID, true); err != nil {
		log.Printf("[DevPortalManager] Failed to activate DevPortal %s: %v", uuid, err)
		return nil, err
	}

	log.Printf("[DevPortalManager] Activated DevPortal %s and synced organization %s", uuid, org.Name)
	return &dto.ActivateDevPortalResponse{
		Message:       fmt.Sprintf("DevPortal '%s' has been activated successfully and organization '%s' synced", devPortal.Name, org.Name),
		DevPortalUUID: uuid,
		DevPortalName: devPortal.Name,
		ActivatedAt:   time.Now(),
	}, nil
}

// DeactivateDevPortal deactivates a DevPortal
func (m *DevPortalManager) DeactivateDevPortal(uuid, orgUUID string) (*dto.DeactivateDevPortalResponse, error) {
	devPortal, err := m.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, err
	}

	// Check if DevPortal can be deactivated using domain logic
	if err := devPortal.CanBeDeactivated(); err != nil {
		return nil, err
	}

	if err := m.devPortalRepo.UpdateActiveStatus(uuid, orgUUID, false); err != nil {
		log.Printf("[DevPortalManager] Failed to deactivate DevPortal %s: %v", uuid, err)
		return nil, err
	}

	log.Printf("[DevPortalManager] Deactivated DevPortal %s", uuid)
	return &dto.DeactivateDevPortalResponse{
		Message:       fmt.Sprintf("DevPortal '%s' has been deactivated successfully", devPortal.Name),
		DevPortalUUID: uuid,
		DevPortalName: devPortal.Name,
		DeactivatedAt: time.Now(),
	}, nil
}

// SetAsDefault sets a DevPortal as the default for its organization
func (m *DevPortalManager) SetAsDefault(uuid, orgUUID string) error {
	if err := m.devPortalRepo.SetAsDefault(uuid, orgUUID); err != nil {
		log.Printf("[DevPortalManager] Failed to set DevPortal %s as default: %v", uuid, err)
		return err
	}

	log.Printf("[DevPortalManager] Set DevPortal %s as default", uuid)
	return nil
}

// GetDefaultDevPortal gets the default DevPortal for an organization
func (m *DevPortalManager) GetDefaultDevPortal(orgUUID string) (*dto.DevPortalResponse, error) {
	devPortal, err := m.devPortalRepo.GetDefaultByOrganizationUUID(orgUUID)
	if err != nil {
		return nil, err
	}

	return dto.ToDevPortalResponse(devPortal), nil
}

// syncOrganizationToDevPortal syncs an organization to a DevPortal backend
func (m *DevPortalManager) syncOrganizationToDevPortal(devPortal *model.DevPortal, orgUUID string) error {
	// Get the organization details
	org, err := m.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to get organization %s for DevPortal sync: %v", orgUUID, err)
		return fmt.Errorf("failed to get organization %s for DevPortal %s sync: %w", orgUUID, devPortal.Name, err)
	}

	// Create ApiPortal client for this DevPortal
	client := m.createDevPortalClient(devPortal)

	// Build organization create request for DevPortal
	orgReq := &devportalDto.OrganizationCreateRequest{
		OrgID:                  org.ID,
		OrgName:                org.Name,
		OrgHandle:              devPortal.Identifier,
		OrganizationIdentifier: devPortal.UUID,
		// Business owner details are not available in current Organization model
		BusinessOwner:        "",
		BusinessOwnerContact: "",
		BusinessOwnerEmail:   "",
		// Use global role mapping configuration
		RoleClaimName:         m.config.DefaultDevPortal.RoleClaimName,
		GroupsClaimName:       m.config.DefaultDevPortal.GroupsClaimName,
		OrganizationClaimName: m.config.DefaultDevPortal.OrganizationClaimName,
		AdminRole:             m.config.DefaultDevPortal.AdminRole,
		SubscriberRole:        m.config.DefaultDevPortal.SubscriberRole,
		SuperAdminRole:        m.config.DefaultDevPortal.SuperAdminRole,
	}

	log.Printf("[DevPortalManager] Syncing organization %s (%s) to DevPortal %s",
		org.Name, orgUUID, devPortal.Name)

	// Sync organization to DevPortal
	_, err = client.CreateOrganization(orgReq)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to sync organization %s to DevPortal %s: %v",
			orgUUID, devPortal.Name, err)

		// Check if this is a connectivity/timeout error
		errStr := err.Error()
		if strings.Contains(errStr, "context deadline exceeded") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "no such host") ||
			strings.Contains(errStr, "timeout") {
			return constants.ErrDevPortalBackendUnreachable
		}

		// For other errors, return a generic sync failure
		return constants.ErrDevPortalSyncFailed
	}

	log.Printf("[DevPortalManager] Successfully synced organization %s to DevPortal %s",
		orgUUID, devPortal.Name)
	return nil
}

// validateDevPortalForPublishing validates that a DevPortal exists and is active for publishing
func (m *DevPortalManager) validateDevPortalForPublishing(devPortalUUID, orgUUID string) (*model.DevPortal, *devportal.DevPortalClient, error) {
	// Get the DevPortal
	devPortal, err := m.devPortalRepo.GetByUUID(devPortalUUID, orgUUID)
	if err != nil {
		return nil, nil, err
	}
	if devPortal == nil {
		log.Printf("[DevPortalManager] DevPortal %s not found for organization %s", devPortalUUID, orgUUID)
		return nil, nil, constants.ErrDevPortalNotFound
	}

	// Check if DevPortal is ready for publishing using domain logic
	if err := devPortal.IsReadyForPublishing(); err != nil {
		return nil, nil, err
	}

	// Create ApiPortal client for this DevPortal
	client := m.createDevPortalClient(devPortal)
	return devPortal, client, nil
}

// preparePublicationRecord creates or updates a publication record with publishing status
func (m *DevPortalManager) preparePublicationRecord(api *model.API, devPortalUUID, orgUUID string) (*model.APIPublication, error) {
	// Check if there's an existing publication record
	existingPublication, err := m.publicationRepo.GetByAPIAndDevPortal(api.ID, devPortalUUID, orgUUID)
	if err != nil && !errors.Is(err, constants.ErrAPIPublicationNotFound) {
		log.Printf("[DevPortalManager] Failed to check existing publication: %v", err)
		return nil, err
	}

	// Create or update publication record with "publishing" status
	var publication *model.APIPublication
	currentTime := time.Now()

	if existingPublication != nil {
		// Update existing publication
		if err := existingPublication.SetPublishing(); err != nil {
			log.Printf("[DevPortalManager] Invalid status transition for existing publication: %v", err)
			return nil, err
		}
		existingPublication.APIVersion = &api.Version
		existingPublication.UpdatedAt = currentTime
		publication = existingPublication

		if err := m.publicationRepo.Update(publication); err != nil {
			log.Printf("[DevPortalManager] Failed to update publication record: %v", err)
			return nil, err
		}
	} else {
		// Create new publication record
		publication = &model.APIPublication{
			UUID:             uuid.New().String(),
			APIUUID:          api.ID,
			DevPortalUUID:    devPortalUUID,
			OrganizationUUID: orgUUID,
			Status:           model.PublishingStatus,
			APIVersion:       &api.Version,
			DevPortalRefID:   nil,
			CreatedAt:        currentTime,
			UpdatedAt:        currentTime,
		}

		if err := m.publicationRepo.Create(publication); err != nil {
			log.Printf("[DevPortalManager] Failed to create publication record: %v", err)
			return nil, err
		}
	}

	return publication, nil
}

// generateAPIDefinition generates OpenAPI definition for the API
func (m *DevPortalManager) generateAPIDefinition(api *model.API, publication *model.APIPublication, devPortal *model.DevPortal) ([]byte, error) {
	// Convert model to DTO for OpenAPI generation
	apiDTO := m.apiUtil.ModelToDTO(api)

	// Generate OpenAPI definition
	apiDefinition, err := m.apiUtil.GenerateOpenAPIDefinition(apiDTO)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to generate OpenAPI definition for API %s (version %s) to DevPortal %s: %v", api.ID, api.Version, devPortal.Name, err)

		// Update publication status to failed
		if statusErr := publication.SetFailed(); statusErr != nil {
			log.Printf("[DevPortalManager] Failed to transition publication to failed status: %v", statusErr)
		}
		publication.UpdatedAt = time.Now()
		m.publicationRepo.Update(publication)

		return nil, fmt.Errorf("failed to generate OpenAPI definition for API %s (version %s) to DevPortal %s: %w", api.ID, api.Version, devPortal.Name, err)
	}

	return apiDefinition, nil
}

// extractEndpointURLs extracts production and sandbox URLs from API backend services
func (m *DevPortalManager) extractEndpointURLs(api *model.API) (productionURL, sandboxURL string) {
	// Default fallback URLs if no backend services are configured
	productionURL = "https://api.example.com"
	sandboxURL = "https://sandbox.api.example.com"

	if len(api.BackendServices) == 0 {
		return
	}

	// Use the first backend service's endpoints
	backendService := api.BackendServices[0]
	if len(backendService.Endpoints) == 0 {
		return
	}

	// Use the first endpoint as production URL
	productionURL = backendService.Endpoints[0].URL

	// If there are multiple endpoints, use the second as sandbox
	if len(backendService.Endpoints) > 1 {
		sandboxURL = backendService.Endpoints[1].URL
	} else {
		// If only one endpoint, use it for both production and sandbox
		sandboxURL = productionURL
	}

	return
}

// buildPublishRequest builds the API publish request DTO
func (m *DevPortalManager) buildPublishRequest(api *model.API) *devportalDto.APIPublishRequest {
	productionURL, sandboxURL := m.extractEndpointURLs(api)

	return &devportalDto.APIPublishRequest{
		APIInfo: devportalDto.APIInfo{
			APIID:          api.ID,
			ReferenceID:    api.ID,
			APIName:        api.Name,
			APIHandle:      fmt.Sprintf("%s-%s", api.Name, api.Version),
			APIVersion:     api.Version,
			APIType:        "REST",
			Provider:       api.Provider,
			APIDescription: api.Description,
			APIStatus:      "PUBLISHED",
			Visibility:     "PUBLIC",
			Labels:         []string{"default"},
		},
		SubscriptionPolicies: []devportalDto.SubscriptionPolicy{
			{PolicyName: "unlimited"},
		},
		EndPoints: devportalDto.EndPoints{
			ProductionURL: productionURL,
			SandboxURL:    sandboxURL,
		},
	}
}

// checkAndHandleExistingAPI checks if API exists in DevPortal and handles it
func (m *DevPortalManager) checkAndHandleExistingAPI(client *devportal.DevPortalClient, orgUUID string, api *model.API, publication *model.APIPublication, devPortal *model.DevPortal) error {
	// Check if API already exists in DevPortal
	apiExists, err := client.CheckAPIExists(orgUUID, api.ID)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to check API existence in DevPortal: %v", err)

		// Update publication status to failed
		if statusErr := publication.SetFailed(); statusErr != nil {
			log.Printf("[DevPortalManager] Failed to transition publication to failed status: %v", statusErr)
		}
		publication.UpdatedAt = time.Now()
		m.publicationRepo.Update(publication)

		return constants.ErrApiPortalSync
	}

	// If API exists, unpublish it first
	if apiExists {
		log.Printf("[DevPortalManager] API %s already exists in DevPortal %s, unpublishing first", api.ID, devPortal.Name)
	}

	return nil
}

// executePublish publishes the API to the DevPortal
func (m *DevPortalManager) executePublish(client *devportal.DevPortalClient, orgUUID string, publishReq *devportalDto.APIPublishRequest, apiDefinition []byte, publication *model.APIPublication, devPortal *model.DevPortal) (*devportalDto.APIPublishResponse, error) {
	// Publish the API
	apiportalResp, err := client.PublishAPI(orgUUID, publishReq, apiDefinition)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to publish API to DevPortal %s: %v", devPortal.Name, err)

		// Update publication status to failed
		if statusErr := publication.SetFailed(); statusErr != nil {
			log.Printf("[DevPortalManager] Failed to transition publication to failed status: %v", statusErr)
		}
		publication.UpdatedAt = time.Now()
		m.publicationRepo.Update(publication)

		return nil, constants.ErrApiPortalSync
	}

	return apiportalResp, nil
}

// updatePublicationSuccess updates the publication record on successful publish
func (m *DevPortalManager) updatePublicationSuccess(publication *model.APIPublication, apiportalResp *devportalDto.APIPublishResponse, api *model.API, devPortal *model.DevPortal) {
	// Successfully published - update publication record
	if err := publication.SetPublished(apiportalResp.APIID); err != nil {
		log.Printf("[DevPortalManager] Failed to transition publication to published status: %v", err)
		// Continue with update even if status transition fails
	}
	publication.UpdatedAt = time.Now()

	if err := m.publicationRepo.Update(publication); err != nil {
		log.Printf("[DevPortalManager] Failed to update publication record after successful publish: %v", err)
		// Continue even if update fails - the publish was successful
	}

	log.Printf("[DevPortalManager] Successfully published API %s to DevPortal %s (API Portal ID: %s)",
		api.ID, devPortal.Name, apiportalResp.APIID)
}

// PublishAPIToDevPortal publishes an API to a specific DevPortal with publication tracking
func (m *DevPortalManager) PublishAPIToDevPortal(devPortalUUID, orgUUID, apiID string, api *model.API) (*dto.PublishToDevPortalResponse, error) {
	// Validate DevPortal
	devPortal, client, err := m.validateDevPortalForPublishing(devPortalUUID, orgUUID)
	if err != nil {
		return nil, err
	}

	// Prepare publication record
	publication, err := m.preparePublicationRecord(api, devPortalUUID, orgUUID)
	if err != nil {
		return nil, err
	}

	// Generate API definition
	apiDefinition, err := m.generateAPIDefinition(api, publication, devPortal)
	if err != nil {
		return nil, err
	}

	// Build publish request
	publishReq := m.buildPublishRequest(api)

	log.Printf("[DevPortalManager] Publishing API %s (Name: %s, Version: %s) to DevPortal %s",
		api.ID, api.Name, api.Version, devPortal.Name)

	// Check and handle existing API
	if err := m.checkAndHandleExistingAPI(client, orgUUID, api, publication, devPortal); err != nil {
		return nil, err
	}

	// Execute publish
	apiportalResp, err := m.executePublish(client, orgUUID, publishReq, apiDefinition, publication, devPortal)
	if err != nil {
		return nil, err
	}

	// Update publication success
	m.updatePublicationSuccess(publication, apiportalResp, api, devPortal)

	return &dto.PublishToDevPortalResponse{
		Message:        fmt.Sprintf("API published successfully to DevPortal '%s'", devPortal.Name),
		APIID:          api.ID,
		DevPortalUUID:  devPortalUUID,
		DevPortalName:  devPortal.Name,
		ApiPortalRefID: apiportalResp.APIID,
		PublishedAt:    time.Now(),
	}, nil
}

// UnpublishAPIFromDevPortal unpublishes an API from a specific DevPortal with publication tracking
func (m *DevPortalManager) UnpublishAPIFromDevPortal(devPortalUUID, orgUUID, apiID string) (*dto.UnpublishFromDevPortalResponse, error) {
	// Get the DevPortal
	devPortal, err := m.devPortalRepo.GetByUUID(devPortalUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if devPortal == nil {
		log.Printf("[DevPortalManager] DevPortal %s not found for organization %s", devPortalUUID, orgUUID)
		return nil, constants.ErrDevPortalNotFound
	}

	// Check if DevPortal is active
	if !devPortal.IsActive {
		return nil, fmt.Errorf("DevPortal %s is not active", devPortal.Name)
	}

	// Create ApiPortal client for this DevPortal
	client := m.createDevPortalClient(devPortal)

	// Check if there's an existing publication record
	existingPublication, err := m.publicationRepo.GetByAPIAndDevPortal(apiID, devPortalUUID, orgUUID)
	if err != nil && !errors.Is(err, constants.ErrAPIPublicationNotFound) {
		log.Printf("[DevPortalManager] Failed to check existing publication: %v", err)
		return nil, err
	}

	// Create or update publication record with "unpublishing" status
	var publication *model.APIPublication
	currentTime := time.Now()

	if existingPublication != nil {
		// Update existing publication
		publication = existingPublication
		if err := publication.SetUnpublishing(); err != nil {
			log.Printf("[DevPortalManager] Invalid status transition for existing publication: %v", err)
			return nil, err
		}
		publication.UpdatedAt = currentTime

		if err := m.publicationRepo.Update(publication); err != nil {
			log.Printf("[DevPortalManager] Failed to update publication record: %v", err)
			return nil, err
		}
	} else {
		// Create new publication record for tracking unpublish attempt
		// This can happen if API was published outside the tracking system
		apiVersionPtr := "unknown"
		publication = &model.APIPublication{
			UUID:             uuid.New().String(),
			APIUUID:          apiID,
			DevPortalUUID:    devPortalUUID,
			OrganizationUUID: orgUUID,
			Status:           model.UnpublishingStatus,
			APIVersion:       &apiVersionPtr,
			DevPortalRefID:   nil,
			CreatedAt:        currentTime,
			UpdatedAt:        currentTime,
		}

		if err := m.publicationRepo.Create(publication); err != nil {
			log.Printf("[DevPortalManager] Failed to create publication record: %v", err)
			return nil, err
		}
	}

	log.Printf("[DevPortalManager] Unpublishing API %s from DevPortal %s", apiID, devPortal.Name)

	// Unpublish the API
	if err := client.UnpublishAPI(orgUUID, apiID); err != nil {
		log.Printf("[DevPortalManager] Failed to unpublish API from DevPortal %s: %v", devPortal.Name, err)

		// Update publication status to failed
		if statusErr := publication.SetFailed(); statusErr != nil {
			log.Printf("[DevPortalManager] Failed to transition publication to failed status: %v", statusErr)
		}
		publication.UpdatedAt = time.Now()
		m.publicationRepo.Update(publication)

		return nil, constants.ErrApiPortalSync
	}

	// Successfully unpublished - update publication record
	if err := publication.SetUnpublished(); err != nil {
		log.Printf("[DevPortalManager] Failed to transition publication to unpublished status: %v", err)
		// Continue with update even if status transition fails
	}
	publication.UpdatedAt = time.Now()

	if err := m.publicationRepo.Update(publication); err != nil {
		log.Printf("[DevPortalManager] Failed to update publication record after successful unpublish: %v", err)
		// Continue even if update fails - the unpublish was successful
	}

	log.Printf("[DevPortalManager] Successfully unpublished API %s from DevPortal %s", apiID, devPortal.Name)

	return &dto.UnpublishFromDevPortalResponse{
		Message:       fmt.Sprintf("API unpublished successfully from DevPortal '%s'", devPortal.Name),
		APIID:         apiID,
		DevPortalUUID: devPortalUUID,
		DevPortalName: devPortal.Name,
		UnpublishedAt: time.Now(),
	}, nil
}

// TODO : This needs to be replaces with proper subscription policy management
// createDefaultSubscriptionPolicy creates a default "unlimited" subscription policy for a DevPortal
func (m *DevPortalManager) createDefaultSubscriptionPolicy(devPortal *model.DevPortal, orgUUID string) error {
	client := m.createDevPortalClient(devPortal)

	// Build subscription policy create request for DevPortal
	devPortalPolicyReq := &devportalDto.SubscriptionPolicyCreateRequest{
		PolicyName:   "unlimited",
		DisplayName:  "Unlimited",
		Description:  "Unlimited subscription policy",
		Type:         "requestCount",
		TimeUnit:     60,
		UnitTime:     "min",
		RequestCount: 1000000,
		BillingPlan:  "FREE",
	}

	// Create default subscription policy
	policyResp, err := client.CreateSubscriptionPolicy(orgUUID, devPortalPolicyReq)
	if err != nil {
		log.Printf("[DevPortalManager] Failed to create subscription policy in DevPortal %s: %v", devPortal.Name, err)
		return err
	}

	log.Printf("[DevPortalManager] Created subscription policy '%s' in DevPortal %s",
		policyResp.PolicyName, devPortal.Name)
	return nil
}
