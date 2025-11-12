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
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	devportal_dto "platform-api/src/internal/client/devportal_client/dto"

	"github.com/google/uuid"
)

// Constants for DevPortal operations
const (
	DevPortalServiceTimeout = 10
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
	devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devPortal %s: %w", uuid, err)
	}
	if devPortal == nil {
		return nil, fmt.Errorf("devPortal %s not found", uuid)
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
			return nil, fmt.Errorf("failed to sync and initialize DevPortal during enable: %w", err)
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
			devPortal.IsActive = true   // Already synced remotely
			devPortal.IsEnabled = false // User can enable it when ready
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
		devPortal.IsEnabled = false // New DevPortals start disabled
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
	devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devPortal %s: %w", uuid, err)
	}
	if devPortal == nil {
		return nil, fmt.Errorf("devPortal %s not found", uuid)
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
	devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get devPortal %s: %w", uuid, err)
	}
	if devPortal == nil {
		return fmt.Errorf("devPortal %s not found", uuid)
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

func (s *DevPortalService) PublishAPIToDevPortal(devPortalUUID, sandboxGatewayID, productionGatewayID, orgUUID, apiID string, api *model.API) (*dto.PublishToDevPortalResponse, error) {
	// Get DevPortal
	devPortal, err := s.devPortalRepo.GetByUUID(devPortalUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devPortal %s: %w", devPortalUUID, err)
	}
	if devPortal == nil {
		return nil, fmt.Errorf("devPortal %s not found", devPortalUUID)
	}

	// Get organization
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", orgUUID, err)
	}
	if org == nil {
		return nil, fmt.Errorf("organization %s not found", orgUUID)
	}

	// Get gateways
	sandboxGateway, err := s.gatewayRepo.GetByUUID(sandboxGatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox gateway %s: %w", sandboxGatewayID, err)
	}
	if sandboxGateway == nil {
		return nil, fmt.Errorf("sandbox gateway %s not found", sandboxGatewayID)
	}

	productionGateway, err := s.gatewayRepo.GetByUUID(productionGatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get production gateway %s: %w", productionGatewayID, err)
	}
	if productionGateway == nil {
		return nil, fmt.Errorf("production gateway %s not found", productionGatewayID)
	}

	// Use internal methods that work with objects (no repeated fetching)
	return s.publishAPIToDevPortalInternal(devPortal, org, api, sandboxGateway, productionGateway)
}

func (s *DevPortalService) publishAPIToDevPortalInternal(
	devPortal *model.DevPortal,
	org *model.Organization,
	api *model.API,
	sandboxGateway *model.Gateway,
	productionGateway *model.Gateway,
) (*dto.PublishToDevPortalResponse, error) {

	// Validate DevPortal is ready for publishing
	err := s.validateDevPortalForPublishingInternal(devPortal)
	if err != nil {
		return nil, err
	}

	// Validate gateways belong to the organization
	if sandboxGateway.OrganizationID != org.ID {
		return nil, fmt.Errorf("sandbox gateway %s does not belong to organization %s", sandboxGateway.ID, org.ID)
	}
	if productionGateway.OrganizationID != org.ID {
		return nil, fmt.Errorf("production gateway %s does not belong to organization %s", productionGateway.ID, org.ID)
	}

	// Validate API deployments (simplified version - checking if gateways exist)
	deployments, err := s.apiRepo.GetDeploymentsByAPIUUID(api.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API deployments: %w", err)
	}

	sandboxDeployed := false
	productionDeployed := false
	for _, deployment := range deployments {
		if deployment.GatewayID == sandboxGateway.ID && deployment.OrganizationID == org.ID {
			sandboxDeployed = true
		}
		if deployment.GatewayID == productionGateway.ID && deployment.OrganizationID == org.ID {
			productionDeployed = true
		}
	}

	if !sandboxDeployed {
		return nil, fmt.Errorf("API %s is not deployed to sandbox gateway %s", api.ID, sandboxGateway.ID)
	}
	if !productionDeployed {
		return nil, fmt.Errorf("API %s is not deployed to production gateway %s", api.ID, productionGateway.ID)
	}

	// Prepare publication record (simplified version)
	publication := &model.APIPublication{
		APIUUID:               api.ID,
		DevPortalUUID:         devPortal.UUID,
		OrganizationUUID:      org.ID,
		SandboxGatewayUUID:    sandboxGateway.ID,
		ProductionGatewayUUID: productionGateway.ID,
		Status:                model.PublishedStatus,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	// Generate endpoint URLs
	sandboxURL := s.constructEndpointURL(api, sandboxGateway)
	productionURL := s.constructEndpointURL(api, productionGateway)

	// Build API metadata request for the devportal_client
	description := api.Description
	if description == "" {
		description = "No description provided."
	}
	apiMetadata := devportal_dto.APIMetadataRequest{
		APIInfo: devportal_dto.APIInfo{
			APIID:          api.ID,
			ReferenceID:    api.ID,
			APIName:        api.Name,
			APIHandle:      api.Context,
			APIVersion:     api.Version,
			APIType:        "REST",
			Provider:       org.Name,
			APIDescription: description,
			APIStatus:      "PUBLISHED",
			Visibility:     "PUBLIC",
			Labels:         []string{"default"},
		},
		SubscriptionPolicies: []devportal_dto.SubscriptionPolicy{
			{PolicyName: "Default"},
		},
		EndPoints: devportal_dto.EndPoints{
			ProductionURL: productionURL,
			SandboxURL:    sandboxURL,
		},
	}

	// Create DevPortal client for publishing
	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Convert string reader to byte array
	apiDefinitionBytes := []byte(fmt.Sprintf(`{
		"openapi": "3.0.0",
		"info": {
			"title": "%s",
			"version": "%s",
			"description": "%s"
		},
		"paths": {}
	}`, api.Name, api.Version, api.Description))

	// Check if API already exists in DevPortal
	exists, err := s.devPortalClientSvc.CheckAPIExists(client, org.ID, api.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if API exists in DevPortal: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("API %s already exists in DevPortal %s", api.ID, devPortal.Name)
	}

	// Publish API using client service
	_, err = s.devPortalClientSvc.PublishAPIToDevPortal(client, org.ID, apiMetadata, apiDefinitionBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to publish API to DevPortal: %w", err)
	}

	// Create publication record in database
	if err := s.publicationRepo.Create(publication); err != nil {
		return nil, fmt.Errorf("failed to create publication record: %w", err)
	}

	return &dto.PublishToDevPortalResponse{
		Message:        fmt.Sprintf("API published successfully to DevPortal '%s'", devPortal.Name),
		APIID:          api.ID,
		DevPortalUUID:  devPortal.UUID,
		DevPortalName:  devPortal.Name,
		ApiPortalRefID: api.ID, // Using API ID as reference for simplicity
		PublishedAt:    time.Now(),
	}, nil
}

// validateDevPortalForPublishingInternal validates DevPortal for publishing using objects
func (s *DevPortalService) validateDevPortalForPublishingInternal(devPortal *model.DevPortal) error {
	if !devPortal.IsEnabled {
		return fmt.Errorf("devportal %s is not enabled", devPortal.Name)
	}

	if !devPortal.IsActive {
		return fmt.Errorf("devportal %s is not activated (synced)", devPortal.Name)
	}
	return nil
}

// constructEndpointURL constructs the endpoint URL for an API on a specific gateway
func (s *DevPortalService) constructEndpointURL(api *model.API, gateway *model.Gateway) string {
	// Simplified URL construction - in reality, this would be more complex
	return fmt.Sprintf("https://%s%s/%s", gateway.Vhost, api.Context, api.Version)
}

// UnpublishAPIFromDevPortal unpublishes an API from a DevPortal
func (s *DevPortalService) UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID string) (*dto.UnpublishFromDevPortalResponse, error) {
	// Get DevPortal
	devPortal, err := s.devPortalRepo.GetByUUID(devPortalUUID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devPortal %s: %w", devPortalUUID, err)
	}
	if devPortal == nil {
		return nil, fmt.Errorf("devPortal %s not found", devPortalUUID)
	}

	// Create DevPortal client for unpublishing
	client := s.devPortalClientSvc.CreateDevPortalClient(devPortal)

	// Unpublish API using client service
	err = s.devPortalClientSvc.UnpublishAPIFromDevPortal(client, orgID, apiID)
	if err != nil {
		return nil, fmt.Errorf("failed to unpublish API from DevPortal: %w", err)
	}

	// Remove publication record from database
	// TODO: Implement deletion of publication record

	return &dto.UnpublishFromDevPortalResponse{
		Message:       fmt.Sprintf("API unpublished successfully from DevPortal '%s'", devPortal.Name),
		APIID:         apiID,
		DevPortalUUID: devPortal.UUID,
		DevPortalName: devPortal.Name,
		UnpublishedAt: time.Now(),
	}, nil
}
