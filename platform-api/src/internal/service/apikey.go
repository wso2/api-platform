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
	"log"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// APIKeyService handles API key management operations for external API key injection
type APIKeyService struct {
	apiRepo              repository.APIRepository
	gatewayEventsService *GatewayEventsService
}

// NewAPIKeyService creates a new API key service instance
func NewAPIKeyService(apiRepo repository.APIRepository, gatewayEventsService *GatewayEventsService) *APIKeyService {
	return &APIKeyService{
		apiRepo:              apiRepo,
		gatewayEventsService: gatewayEventsService,
	}
}

// CreateAPIKey hashes an external API key and broadcasts it to gateways where the API is deployed.
// This method is used when external platforms inject API keys to hybrid gateways.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, apiHandle, orgId string, req *dto.CreateAPIKeyRequest) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API metadata for API key creation: apiHandle=%s error=%v", apiHandle, err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		log.Printf("[WARN] API not found by handle: apiHandle=%s orgId=%s", apiHandle, orgId)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID

	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API for API key creation: apiHandle=%s error=%v", apiHandle, err)
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

	operations := "[\"*\"]" // Default to all operations

	// Build the API key created event
	// Note: API key is sent as plain text - hashing happens in the gateway/policy-engine
	event := &model.APIKeyCreatedEvent{
		ApiId:         apiHandle,
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		ApiKey:        req.ApiKey, // Send plain API key (no hashing in platform-api)
		ExternalRefId: req.ExternalRefId,
		Operations:    operations,
		ExpiresAt:     req.ExpiresAt,
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		log.Printf("[INFO] Broadcasting API key created event: apiHandle=%s gatewayId=%s keyName=%s",
			apiHandle, gatewayID, req.Name)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gatewayID, event)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to broadcast API key created event: apiHandle=%s gatewayId=%s keyName=%s error=%v",
				apiId, gatewayID, req.Name, err)
		} else {
			successCount++
			log.Printf("[INFO] Successfully broadcast API key created event: apiHandle=%s gatewayId=%s keyName=%s",
				apiId, gatewayID, req.Name)
		}
	}

	// Log summary
	log.Printf("[INFO] API key creation broadcast summary: apiHandle=%s keyName=%s total=%d success=%d failed=%d",
		apiId, req.Name, len(gateways), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		log.Printf("[ERROR] Failed to deliver API key to any gateway: apiHandle=%s keyName=%s", apiHandle, req.Name)
		return fmt.Errorf("failed to deliver API key event to any gateway: %w", lastError)
	}

	// Partial success is still considered success (some gateways received the event)
	return nil
}

// UpdateAPIKey updates/regenerates an API key and broadcasts it to all gateways where the API is deployed.
// This method is used when external platforms rotates/regenerates API keys on hybrid gateways.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, apiHandle, orgId, keyName string, req *dto.UpdateAPIKeyRequest) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API metadata for API key update: apiHandle=%s error=%v", apiHandle, err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		log.Printf("[WARN] API not found by handle for API key update: apiHandle=%s", apiHandle)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID

	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API for API key update: apiHandle=%s error=%v", apiHandle, err)
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		log.Printf("[WARN] API not found for API key update: apiHandle=%s", apiHandle)
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get deployments for API key update: apiHandle=%s error=%v", apiHandle, err)
		return fmt.Errorf("failed to get API deployments: %w", err)
	}

	if len(gateways) == 0 {
		log.Printf("[WARN] No gateway deployments found for API: apiHandle=%s", apiHandle)
		return constants.ErrGatewayUnavailable
	}

	// Build the API key updated event
	// Note: API key is sent as plain text - hashing happens in the gateway/policy-engine
	event := &model.APIKeyUpdatedEvent{
		ApiId:     apiHandle,
		KeyName:   keyName,
		ApiKey:    req.ApiKey, // Send plain API key (no hashing in platform-api)
		ExpiresAt: req.ExpiresAt,
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		log.Printf("[INFO] Broadcasting API key updated event: apiHandle=%s gatewayId=%s keyName=%s",
			apiHandle, gatewayID, keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyUpdatedEvent(gatewayID, event)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to broadcast API key updated event: apiHandle=%s gatewayId=%s keyName=%s error=%v",
				apiHandle, gatewayID, keyName, err)
		} else {
			successCount++
			log.Printf("[INFO] Successfully broadcast API key updated event: apiHandle=%s gatewayId=%s keyName=%s",
				apiHandle, gatewayID, keyName)
		}
	}

	// Log summary
	log.Printf("[INFO] API key update broadcast summary: apiHandle=%s keyName=%s total=%d success=%d failed=%d",
		apiHandle, keyName, len(gateways), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		log.Printf("[ERROR] Failed to deliver API key update to any gateway: apiHandle=%s keyName=%s", apiHandle, keyName)
		return fmt.Errorf("failed to deliver API key update event to any gateway: %w", lastError)
	}

	// Partial success is still considered success (some gateways received the event)
	return nil
}

// RevokeAPIKey broadcasts API key revocation to all gateways where the API is deployed
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, apiHandle, orgId, keyName string) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API metadata for API key revocation: apiHandle=%s error=%v", apiHandle, err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		log.Printf("[WARN] API not found by handle for API key revocation: apiHandle=%s", apiHandle)
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

		log.Printf("[INFO] Broadcasting API key revoked event: apiHandle=%s gatewayId=%s keyName=%s",
			apiId, gatewayID, keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, event)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to broadcast API key revoked event: apiHandle=%s gatewayId=%s keyName=%s error=%v",
				apiId, gatewayID, keyName, err)
		} else {
			successCount++
			log.Printf("[INFO] Successfully broadcast API key revoked event: apiHandle=%s gatewayId=%s keyName=%s",
				apiId, gatewayID, keyName)
		}
	}

	// Log summary
	log.Printf("[INFO] API key revocation broadcast summary: apiHandle=%s keyName=%s total=%d success=%d failed=%d",
		apiId, keyName, len(gateways), successCount, failureCount)

	if failureCount == len(gateways) {
		return fmt.Errorf("failed to deliver API key revocation to all gateways: %w", lastError)
	}
	if failureCount > 0 {
		log.Printf("[WARN] Partial delivery of API key revocation: apiHandle=%s keyName=%s failureCount=%d total=%d",
			apiId, keyName, failureCount, len(gateways))
	}

	return nil
}
