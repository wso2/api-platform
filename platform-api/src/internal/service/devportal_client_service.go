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
	"bytes"
	"errors"
	"fmt"
	"log"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"

	"github.com/go-playground/validator/v10"

	devportal_client "platform-api/src/internal/client/devportal_client"
	devportal_dto "platform-api/src/internal/client/devportal_client/dto"
)

var validate = validator.New()

// DevPortalClientService handles all interactions with external DevPortal clients
type DevPortalClientService struct {
	config *config.Server
}

// NewDevPortalClientService creates a new DevPortalClientService
func NewDevPortalClientService(config *config.Server) *DevPortalClientService {
	return &DevPortalClientService{
		config: config,
	}
}

// createDevPortalClient creates a DevPortalClient configured for the given DevPortal
func (s *DevPortalClientService) createDevPortalClient(devPortal *model.DevPortal) *devportal_client.DevPortalClient {
	timeout := s.config.DefaultDevPortal.Timeout
	if timeout <= 0 {
		timeout = DevPortalServiceTimeout
	}

	// Create DevPortalConfig for the new client
	cfg := devportal_client.DevPortalConfig{
		BaseURL:    devPortal.APIUrl,        // DevPortal-specific API URL
		APIKey:     devPortal.APIKey,        // DevPortal-specific API key
		HeaderName: devPortal.HeaderKeyName, // DevPortal-specific header name
		Timeout:    time.Duration(timeout) * time.Second,
	}

	return devportal_client.NewDevPortalClient(cfg)
}

// SyncOrganizationToDevPortal syncs an organization to DevPortal using the client
func (s *DevPortalClientService) SyncOrganizationToDevPortal(devPortal *model.DevPortal, organization *model.Organization) error {
	// Create DevPortal client for this DevPortal
	client := s.createDevPortalClient(devPortal)

	// Build organization create request for DevPortal
	orgReq := devportal_dto.OrganizationCreateRequest{
		OrgID:                  organization.ID,
		OrgName:                organization.Name,
		OrgHandle:              devPortal.Identifier,
		OrganizationIdentifier: organization.ID,
		// Business owner details are not available in current Organization model
		BusinessOwner:        "",
		BusinessOwnerContact: "",
		BusinessOwnerEmail:   "",
		// Use global role mapping configuration
		RoleClaimName:         s.config.DefaultDevPortal.RoleClaimName,
		GroupsClaimName:       s.config.DefaultDevPortal.GroupsClaimName,
		OrganizationClaimName: s.config.DefaultDevPortal.OrganizationClaimName,
		AdminRole:             s.config.DefaultDevPortal.AdminRole,
		SubscriberRole:        s.config.DefaultDevPortal.SubscriberRole,
		SuperAdminRole:        s.config.DefaultDevPortal.SuperAdminRole,
	}

	// Validate the organization request
	if err := validate.Struct(orgReq); err != nil {
		return fmt.Errorf("invalid organization request: %w", err)
	}

	// Sync organization to DevPortal
	_, err := client.Organizations().Create(orgReq)
	if err != nil {
		// Use standard wrapper for devportal_client errors
		if wrappedErr := utils.WrapDevPortalClientError(err); wrappedErr != err {
			return wrappedErr
		}
		// For unknown errors, return a generic sync failure
		return constants.ErrDevPortalSyncFailed
	}

	return nil
}

// CreateDevPortalClient creates a DevPortalClient for the given DevPortal
func (s *DevPortalClientService) CreateDevPortalClient(devPortal *model.DevPortal) *devportal_client.DevPortalClient {
	return s.createDevPortalClient(devPortal)
}

// CreateDefaultSubscriptionPolicy creates default subscription policy in DevPortal
func (s *DevPortalClientService) CreateDefaultSubscriptionPolicy(devPortal *model.DevPortal) error {
	// Create DevPortal client for this DevPortal
	client := s.createDevPortalClient(devPortal)

	// Create default subscription policy
	defaultPolicy := devportal_dto.SubscriptionPolicy{
		PolicyName:   "Default",
		DisplayName:  "Default Policy",
		Description:  "Default subscription policy for organization",
		BillingPlan:  "free",
		RequestCount: "1000",
		Type:         devportal_dto.SubscriptionTypeRequestCount,
	}

	// Create subscription policies array (API expects array)
	policies := []devportal_dto.SubscriptionPolicy{defaultPolicy}

	// Create subscription policies in DevPortal
	_, err := client.SubscriptionPolicies().Create(devPortal.OrganizationUUID, policies)
	if err != nil {
		wrappedErr := utils.WrapDevPortalClientError(err)
		if wrappedErr == nil {
			// Default policies already exist, this is not an error
			log.Printf("[DevPortalClientService] Default subscription policies already exist for organization %s", devPortal.OrganizationUUID)
			return nil
		}
		if wrappedErr != err {
			return wrappedErr
		}

		// For other errors, return a generic creation failure
		return fmt.Errorf("failed to create default subscription policy in devportal: %w", err)
	}

	return nil
}

// PublishAPIToDevPortal publishes API to DevPortal using the client
func (s *DevPortalClientService) PublishAPIToDevPortal(
	client *devportal_client.DevPortalClient,
	orgID string,
	apiMetadata devportal_dto.APIMetadataRequest,
	apiDefinition []byte,
) (*devportal_dto.APIResponse, error) {
	// Validate the API metadata
	if err := validate.Struct(apiMetadata); err != nil {
		return nil, err
	}

	response, err := client.APIs().Publish(orgID, apiMetadata, bytes.NewReader(apiDefinition), "apiDefinition.json", nil, "")
	if err != nil {
		return nil, utils.WrapDevPortalClientError(err)
	}
	return response, nil
}

// CheckAPIExists checks if an API exists in DevPortal using the client
func (s *DevPortalClientService) CheckAPIExists(
	client *devportal_client.DevPortalClient,
	orgID string,
	apiID string,
) (bool, error) {
	_, err := client.APIs().Get(orgID, apiID)
	if err != nil {
		if errors.Is(err, devportal_client.ErrAPINotFound) {
			return false, nil
		}
		return false, utils.WrapDevPortalClientError(err)
	}
	return true, nil
}

// UnpublishAPIFromDevPortal unpublishes API from DevPortal using the client
func (s *DevPortalClientService) UnpublishAPIFromDevPortal(
	client *devportal_client.DevPortalClient,
	orgID string,
	apiID string,
) error {
	log.Printf("[DevPortalClientService] UnpublishAPIFromDevPortal: starting for org %s, api %s", orgID, apiID)
	// TODO : Relevant logics needs to be implemented. (before unpublishing whether that api have active subscriptions in devportal)
	err := client.APIs().Delete(orgID, apiID)
	if err != nil {
		log.Printf("[DevPortalClientService] UnpublishAPIFromDevPortal: client.APIs().Delete returned error: %v", err)
		return utils.WrapDevPortalClientError(err)
	}
	log.Printf("[DevPortalClientService] UnpublishAPIFromDevPortal: successfully unpublished API %s for org %s", apiID, orgID)
	return nil
}
