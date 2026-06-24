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

// devportals table removed — all functions in this file are disabled.

import (
	// "bytes"   // devportals table removed
	// "errors"  // devportals table removed
	// "fmt"     // devportals table removed
	// "log"     // devportals table removed
	// "time"    // devportals table removed

	"platform-api/src/config"
	"platform-api/src/internal/model"

	// "platform-api/src/internal/constants"             // devportals table removed
	// "platform-api/src/internal/utils"                 // devportals table removed
	// "github.com/go-playground/validator/v10"           // devportals table removed
	// devportal_client "platform-api/src/internal/client/devportal_client" // devportals table removed
	// devportal_dto "platform-api/src/internal/client/devportal_client/dto" // devportals table removed
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
func (s *DevPortalClientService) createDevPortalClient(devPortal *model.DevPortal) interface{} {
	// devportals table removed — disabled
	// timeout := s.config.DefaultDevPortal.Timeout
	// if timeout <= 0 { timeout = DevPortalServiceTimeout }
	// cfg := devportal_client.DevPortalConfig{BaseURL: devPortal.APIUrl, APIKey: devPortal.APIKey, HeaderName: devPortal.HeaderKeyName, Timeout: time.Duration(timeout) * time.Second}
	// return devportal_client.NewDevPortalClient(cfg)
	return nil
}

// SyncOrganizationToDevPortal syncs an organization to DevPortal using the client
func (s *DevPortalClientService) SyncOrganizationToDevPortal(devPortal *model.DevPortal, organization *model.Organization) error {
	// devportals table removed — disabled
	// client := s.createDevPortalClient(devPortal)
	// orgReq := devportal_dto.OrganizationCreateRequest{...}
	// _, err := client.Organizations().Create(orgReq)
	// ...
	return nil
}

// CreateDevPortalClient creates a DevPortalClient for the given DevPortal
func (s *DevPortalClientService) CreateDevPortalClient(devPortal *model.DevPortal) interface{} {
	// devportals table removed — disabled
	return nil
}

// CreateDefaultSubscriptionPolicy creates default subscription policy in DevPortal
func (s *DevPortalClientService) CreateDefaultSubscriptionPolicy(devPortal *model.DevPortal) error {
	// devportals table removed — disabled
	// client := s.createDevPortalClient(devPortal)
	// defaultPolicy := devportal_dto.SubscriptionPolicy{...}
	// _, err := client.SubscriptionPolicies().Create(...)
	// ...
	return nil
}

// PublishAPIToDevPortal publishes API to DevPortal using the client
func (s *DevPortalClientService) PublishAPIToDevPortal(
	client interface{},
	orgID string,
	apiMetadata interface{},
	apiDefinition []byte,
) (interface{}, error) {
	// devportals table removed — disabled
	// if err := validate.Struct(apiMetadata); err != nil { return nil, err }
	// response, err := client.APIs().Publish(orgID, apiMetadata, bytes.NewReader(apiDefinition), ...)
	// ...
	return nil, nil
}

// CheckAPIExists checks if an API exists in DevPortal using the client
func (s *DevPortalClientService) CheckAPIExists(
	client interface{},
	orgID string,
	apiID string,
) (bool, error) {
	// devportals table removed — disabled
	// _, err := client.APIs().Get(orgID, apiID)
	// ...
	return false, nil
}

// UnpublishAPIFromDevPortal unpublishes API from DevPortal using the client
func (s *DevPortalClientService) UnpublishAPIFromDevPortal(
	client interface{},
	orgID string,
	apiID string,
) error {
	// devportals table removed — disabled
	// err := client.APIs().Delete(orgID, apiID)
	// ...
	return nil
}
