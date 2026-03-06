/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"context"
	"fmt"
	"log/slog"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// APIKeyService handles API key management operations for external API key injection
type APIKeyService struct {
	apiRepo              repository.APIRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
}

// NewAPIKeyService creates a new API key service instance
func NewAPIKeyService(apiRepo repository.APIRepository, gatewayEventsService *GatewayEventsService, slogger *slog.Logger) *APIKeyService {
	return &APIKeyService{
		apiRepo:              apiRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
	}
}

// CreateAPIKey hashes an external API key and broadcasts it to gateways where the API is deployed.
// This method is used when external platforms inject API keys to hybrid gateways.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, apiHandle, orgId, userId string, req *api.CreateAPIKeyRequest) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key creation", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle", "apiHandle", apiHandle, "orgId", orgId)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID

	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API for API key creation", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API deployments for API handle: %s: %w", apiHandle, err)
	}

	if len(gateways) == 0 {
		return constants.ErrGatewayUnavailable
	}

	// Build the API key created event
	// Note: API key is sent as plain text - hashing happens in the gateway/gateway-runtime/policy-engine
	event := &model.APIKeyCreatedEvent{
		ApiId:         apiHandle,
		ApiKey:        req.ApiKey, // Send plain API key (no hashing in platform-api)
		ExternalRefId: req.ExternalRefId,
	}

	// Handle optional pointer fields
	if req.Name != nil {
		event.Name = *req.Name
	}
	if req.DisplayName != nil {
		event.DisplayName = *req.DisplayName
	}
	if req.ExpiresAt != nil {
		expiresAtStr := req.ExpiresAt.Format(time.RFC3339)
		event.ExpiresAt = &expiresAtStr
	}

	// Get key name for logging
	keyName := ""
	if req.Name != nil {
		keyName = *req.Name
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting API key created event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gatewayID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key created event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key created event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	// Log summary
	s.slogger.Info("API key creation broadcast summary", "apiHandle", apiHandle, "keyName", keyName, "total", len(gateways), "success", successCount, "failed", failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		s.slogger.Error("Failed to deliver API key to any gateway", "apiHandle", apiHandle, "keyName", keyName)
		return fmt.Errorf("failed to deliver API key event to any gateway: %w", lastError)
	}

	// Partial success is still considered success (some gateways received the event)
	return nil
}

// UpdateAPIKey updates/regenerates an API key and broadcasts it to all gateways where the API is deployed.
// This method is used when external platforms rotates/regenerates API keys on hybrid gateways.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, apiHandle, orgId, keyName, userId string, req *api.UpdateAPIKeyRequest) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key update", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle for API key update", "apiHandle", apiHandle)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID

	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API for API key update", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		s.slogger.Warn("API not found for API key update", "apiHandle", apiHandle)
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		s.slogger.Error("Failed to get deployments for API key update", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API deployments: %w", err)
	}

	if len(gateways) == 0 {
		s.slogger.Warn("No gateway deployments found for API", "apiHandle", apiHandle)
		return constants.ErrGatewayUnavailable
	}

	// Build the API key updated event
	// Note: API key is sent as plain text - hashing happens in the gateway/gateway-runtime/policy-engine
	event := &model.APIKeyUpdatedEvent{
		ApiId:   apiHandle,
		KeyName: keyName,
		ApiKey:  req.ApiKey, // Send plain API key (no hashing in platform-api)
	}

	// Handle optional pointer fields
	if req.DisplayName != nil {
		event.DisplayName = *req.DisplayName
	}
	if req.ExternalRefId != nil {
		event.ExternalRefId = req.ExternalRefId
	}
	if req.ExpiresAt != nil {
		expiresAtStr := req.ExpiresAt.Format(time.RFC3339)
		event.ExpiresAt = &expiresAtStr
	}

	// Only set ExpiresIn if provided (nil signals clearing expiration along with nil ExpiresAt)
	if req.ExpiresIn != nil {
		// Validate the expiration duration before using it
		if req.ExpiresIn.Duration <= 0 {
			err := fmt.Errorf("duration must be a positive integer, got %d", req.ExpiresIn.Duration)
			s.slogger.Error("Invalid expiration duration for API key update", "apiHandle", apiHandle, "keyName", keyName, "error", err)
			return fmt.Errorf("invalid expiration duration: %w", err)
		}
		event.ExpiresIn = &model.ExpiresInDuration{
			Duration: req.ExpiresIn.Duration,
			Unit:     model.TimeUnit(req.ExpiresIn.Unit),
		}
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting API key updated event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyUpdatedEvent(gatewayID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key updated event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key updated event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	// Log summary
	s.slogger.Info("API key update broadcast summary", "apiHandle", apiHandle, "keyName", keyName, "total", len(gateways), "success", successCount, "failed", failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		s.slogger.Error("Failed to deliver API key update to any gateway", "apiHandle", apiHandle, "keyName", keyName)
		return fmt.Errorf("failed to deliver API key update event to any gateway: %w", lastError)
	}

	// Partial success is still considered success (some gateways received the event)
	return nil
}

// RevokeAPIKey broadcasts API key revocation to all gateways where the API is deployed
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, apiHandle, orgId, keyName, userId string) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key revocation", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle for API key revocation", "apiHandle", apiHandle)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID
	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API deployments: %w", err)
	}

	if len(gateways) == 0 {
		return constants.ErrGatewayUnavailable
	}

	// Build the API key revoked event
	event := &model.APIKeyRevokedEvent{
		ApiId:   apiHandle,
		KeyName: keyName,
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting API key revoked event", "apiHandle", apiId, "gatewayId", gatewayID, "keyName", keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key revoked event", "apiHandle", apiId, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key revoked event", "apiHandle", apiId, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	// Log summary
	s.slogger.Info("API key revocation broadcast summary", "apiHandle", apiId, "keyName", keyName, "total", len(gateways), "success", successCount, "failed", failureCount)

	if failureCount == len(gateways) {
		return fmt.Errorf("failed to deliver API key revocation to all gateways: %w", lastError)
	}
	if failureCount > 0 {
		s.slogger.Warn("Partial delivery of API key revocation", "apiHandle", apiId, "keyName", keyName, "failureCount", failureCount, "total", len(gateways))
	}

	return nil
}
