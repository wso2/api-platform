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

// devportals, publication_mappings, and association_mappings tables removed — all functions in this file are disabled.

import (
	// "errors"    // devportals table removed
	// "fmt"       // devportals table removed
	"log/slog"
	// "strings"   // devportals table removed
	// "time"      // devportals table removed

	"platform-api/src/api"
	"platform-api/src/config"
	// "platform-api/src/internal/client/devportal_client" // devportals table removed
	// "platform-api/src/internal/constants" // devportals table removed
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	// "github.com/go-playground/validator/v10" // devportals table removed
	// "github.com/google/uuid" // devportals table removed
)

// Constants for DevPortal operations
const (
	DevPortalServiceTimeout = 10
)

// sanitizeAPIHandle removes forward slashes and backslashes from API handle
func sanitizeAPIHandle(handle string) string {
	// devportals table removed — disabled
	// handle = strings.ReplaceAll(handle, "/", "")
	// handle = strings.ReplaceAll(handle, "\\", "")
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
	// validator          *validator.Validate  // devportals table removed
	slogger *slog.Logger
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
		devPortalRepo:   devPortalRepo,
		orgRepo:         orgRepo,
		publicationRepo: publicationRepo,
		apiRepo:         apiRepo,
		apiUtil:         apiUtil,
		config:          config,
		// devPortalClientSvc: NewDevPortalClientService(config), // devportals table removed
		// validator:          sharedValidator,                   // devportals table removed
		slogger: slogger,
	}
}

// getDevPortalByUUID retrieves a DevPortal by UUID with error handling
func (s *DevPortalService) getDevPortalByUUID(uuid, orgUUID string) (*model.DevPortal, error) {
	// devportals table removed — disabled
	// devPortal, err := s.devPortalRepo.GetByUUID(uuid, orgUUID)
	// if err != nil { return nil, err }
	// return devPortal, nil
	return nil, nil
}

// CreateDefaultDevPortal creates a default DevPortal for an organization
func (s *DevPortalService) CreateDefaultDevPortal(orgUUID string) (*model.DevPortal, error) {
	// devportals table removed — disabled
	return nil, nil
}

// CreateDevPortal creates a new DevPortal
func (s *DevPortalService) CreateDevPortal(orgUUID string, req *api.CreateDevPortalRequest) (*api.DevPortalResponse, error) {
	// devportals table removed — disabled
	return nil, nil
}

// EnableDevPortal enables a DevPortal
func (s *DevPortalService) EnableDevPortal(uuid, orgUUID string) error {
	// devportals table removed — disabled
	return nil
}

// createDevPortalWithSync creates a DevPortal and syncs it
func (s *DevPortalService) createDevPortalWithSync(devPortal *model.DevPortal, organization *model.Organization, allowSyncFailure bool) error {
	// devportals table removed — disabled
	return nil
}

// syncAndInitializeDevPortalInternal syncs a DevPortal with the external system
func (s *DevPortalService) syncAndInitializeDevPortalInternal(devPortal *model.DevPortal, organization *model.Organization) error {
	// devportals table removed — disabled
	return nil
}

// updateDevPortalStateInternal updates the state of a DevPortal
func (s *DevPortalService) updateDevPortalStateInternal(devPortal *model.DevPortal, isActive, isEnabled *bool) error {
	// devportals table removed — disabled
	return nil
}

// GetDevPortal retrieves a DevPortal by UUID
func (s *DevPortalService) GetDevPortal(uuid, orgUUID string) (*api.DevPortalResponse, error) {
	// devportals table removed — disabled
	return nil, nil
}

// ListDevPortals lists DevPortals for an organization
func (s *DevPortalService) ListDevPortals(orgUUID string, isDefault, isEnabled *bool, limit, offset int) (*api.DevPortalListResponse, error) {
	// devportals table removed — disabled
	return nil, nil
}

// UpdateDevPortal updates a DevPortal
func (s *DevPortalService) UpdateDevPortal(uuid, orgUUID string, req *api.UpdateDevPortalRequest) (*api.DevPortalResponse, error) {
	// devportals table removed — disabled
	return nil, nil
}

// DeleteDevPortal deletes a DevPortal
func (s *DevPortalService) DeleteDevPortal(uuid, orgUUID string) error {
	// devportals table removed — disabled
	return nil
}

// DisableDevPortal disables a DevPortal
func (s *DevPortalService) DisableDevPortal(uuid, orgUUID string) error {
	// devportals table removed — disabled
	return nil
}

// SetAsDefault sets a DevPortal as the default
func (s *DevPortalService) SetAsDefault(uuid, orgUUID string) error {
	// devportals table removed — disabled
	return nil
}

// GetDefaultDevPortal retrieves the default DevPortal
func (s *DevPortalService) GetDefaultDevPortal(orgUUID string) (*api.DevPortalResponse, error) {
	// devportals table removed — disabled
	return nil, nil
}

// PublishAPIToDevPortal publishes an API to a DevPortal
func (s *DevPortalService) PublishAPIToDevPortal(apiUUID string, apiModel *api.RESTAPI, req *api.PublishToDevPortalRequest, orgUUID string) error {
	// publication_mappings / devportals tables removed — disabled
	return nil
}

// validatePublishInputs validates the inputs for publishing
func (s *DevPortalService) validatePublishInputs(req *api.PublishToDevPortalRequest, orgUUID string) (*model.DevPortal, *model.Organization, error) {
	// devportals table removed — disabled
	return nil, nil, nil
}

// prepareAPIPublication prepares an API publication record
func (s *DevPortalService) prepareAPIPublication(apiUUID string, req *api.PublishToDevPortalRequest, devPortal *model.DevPortal, orgUUID string) error {
	// publication_mappings table removed — disabled
	return nil
}

// prepareAPIMetadata prepares API metadata for publishing
// func (s *DevPortalService) prepareAPIMetadata(apiUUID string, apiModel *api.RESTAPI, req *api.PublishToDevPortalRequest) (devportal_client.APIMetadataRequest, error) {
// 	// devportals table removed — disabled
// 	return devportal_client.APIMetadataRequest{}, nil
// }

// publishToDevPortal publishes to the DevPortal
func (s *DevPortalService) publishToDevPortal(devPortal *model.DevPortal, organization *model.Organization, apiUUID string, apiModel *api.RESTAPI, req *api.PublishToDevPortalRequest) error {
	// devportals table removed — disabled
	return nil
}

// savePublicationWithCompensation saves a publication with compensation on failure
func (s *DevPortalService) savePublicationWithCompensation(publication interface{}, devPortal *model.DevPortal, organization *model.Organization) error {
	// publication_mappings table removed — disabled
	return nil
}

// compensatePublication compensates for a failed publication
func (s *DevPortalService) compensatePublication(publication interface{}, devPortal *model.DevPortal, organization *model.Organization) {
	// publication_mappings table removed — disabled
}

// UnpublishAPIFromDevPortal unpublishes an API from a DevPortal
func (s *DevPortalService) UnpublishAPIFromDevPortal(devPortalUUID, orgID, apiID string) error {
	// publication_mappings / devportals tables removed — disabled
	return nil
}

// createDevPortalRequestToModel converts a CreateDevPortalRequest to a DevPortal model
func createDevPortalRequestToModel(req *api.CreateDevPortalRequest, orgUUID string) *model.DevPortal {
	// devportals table removed — disabled
	// ...
	return nil
}

// devPortalModelToResponse converts a DevPortal model to a DevPortalResponse API type
func devPortalModelToResponse(devPortal *model.DevPortal) (*api.DevPortalResponse, error) {
	// devportals table removed — disabled
	// orgUUID, err := uuid.Parse(devPortal.OrganizationUUID)
	// ...
	return nil, nil
}
