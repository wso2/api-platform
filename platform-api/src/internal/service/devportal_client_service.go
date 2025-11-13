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
	"strings"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"

	devportal_client "platform-api/src/internal/client/devportal_client"
	devportal_dto "platform-api/src/internal/client/devportal_client/dto"
)

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
		OrganizationIdentifier: devPortal.UUID,
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

	// Sync organization to DevPortal
	_, err := client.Organizations().Create(orgReq)
	if err != nil {
		// Check if organization already exists
		if errors.Is(err, devportal_client.ErrOrganizationAlreadyExists) {
			return constants.ErrDevPortalAlreadyExist
		}

		// For other errors, return a generic sync failure
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
		// Check if this is a connectivity/timeout error
		errStr := err.Error()
		if strings.Contains(errStr, "context deadline exceeded") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "no such host") ||
			strings.Contains(errStr, "timeout") {
			return constants.ErrDevPortalBackendUnreachable
		}

		// Check if default policies are already generated by DevPortal
		if strings.Contains(errStr, "generateDefaultSubPolicies") ||
			strings.Contains(errStr, "Bulk creation of subscription policies is not allowed") {
			// Default policies already exist, this is not an error
			log.Printf("[DevPortalClientService] Default subscription policies already exist for organization %s", devPortal.OrganizationUUID)
			return nil
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
	return client.APIs().Publish(orgID, apiMetadata, bytes.NewReader(apiDefinition), "api.yaml", nil, "")
}

// CheckAPIExists checks if an API exists in DevPortal using the client
func (s *DevPortalClientService) CheckAPIExists(
	client *devportal_client.DevPortalClient,
	orgID string,
	apiID string,
) (bool, error) {
	_, err := client.APIs().Get(orgID, apiID)
	if err != nil {
		// Check if it's a "not found" error
		if errors.Is(err, devportal_client.ErrAPINotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UnpublishAPIFromDevPortal unpublishes API from DevPortal using the client
func (s *DevPortalClientService) UnpublishAPIFromDevPortal(
	client *devportal_client.DevPortalClient,
	orgID string,
	apiID string,
) error {
	// TODO : Relevant logics needs to be implemented. (before unpublishing whether that api have active subscriptions in devportal)
	return client.APIs().Delete(orgID, apiID)
}
