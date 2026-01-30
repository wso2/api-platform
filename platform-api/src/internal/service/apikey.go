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
// This method is used when Cloud APIM injects API keys to hybrid gateways.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, apiId, orgId string, req *dto.CreateAPIKeyRequest) error {
	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API for API key creation: apiId=%s error=%v", apiId, err)
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	deployments, err := s.apiRepo.GetDeploymentsByAPIUUID(apiId, orgId, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get API deployments for API ID: %s: %w", apiId, err)
	}

	if len(deployments) == 0 {
		return constants.ErrGatewayUnavailable
	}

	operations := "[\"*\"]" // Default to all operations

	// Build the API key created event
	// Note: API key is sent as plain text - hashing happens in the gateway/policy-engine
	event := &model.APIKeyCreatedEvent{
		ApiId:         apiId,
		KeyName:       req.Name,
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
	for _, deployment := range deployments {
		gatewayID := deployment.GatewayID

		log.Printf("[INFO] Broadcasting API key created event: apiId=%s gatewayId=%s keyName=%s",
			apiId, gatewayID, req.Name)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gatewayID, event)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to broadcast API key created event: apiId=%s gatewayId=%s keyName=%s error=%v",
				apiId, gatewayID, req.Name, err)
		} else {
			successCount++
			log.Printf("[INFO] Successfully broadcast API key created event: apiId=%s gatewayId=%s keyName=%s",
				apiId, gatewayID, req.Name)
		}
	}

	// Log summary
	log.Printf("[INFO] API key creation broadcast summary: apiId=%s keyName=%s total=%d success=%d failed=%d",
		apiId, req.Name, len(deployments), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		log.Printf("[ERROR] Failed to deliver API key to any gateway: apiId=%s keyName=%s", apiId, req.Name)
		return fmt.Errorf("failed to deliver API key event to any gateway: %w", lastError)
	}

	// Partial success is still considered success (some gateways received the event)
	return nil
}

// UpdateAPIKey updates/regenerates an API key and broadcasts it to all gateways where the API is deployed.
// This method is used when Cloud APIM rotates/regenerates API keys on hybrid gateways.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, apiId, orgId, keyName string, req *dto.UpdateAPIKeyRequest) error {
	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		log.Printf("[ERROR] Failed to get API for API key update: apiId=%s error=%v", apiId, err)
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		log.Printf("[WARN] API not found for API key update: apiId=%s", apiId)
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	deployments, err := s.apiRepo.GetDeploymentsByAPIUUID(apiId, orgId, nil, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to get deployments for API key update: apiId=%s error=%v", apiId, err)
		return fmt.Errorf("failed to get API deployments: %w", err)
	}

	if len(deployments) == 0 {
		log.Printf("[WARN] No gateway deployments found for API: apiId=%s", apiId)
		return constants.ErrGatewayUnavailable
	}

	// Build the API key updated event
	// Note: API key is sent as plain text - hashing happens in the gateway/policy-engine
	event := &model.APIKeyUpdatedEvent{
		ApiId:     apiId,
		KeyName:   keyName,
		ApiKey:    req.ApiKey, // Send plain API key (no hashing in platform-api)
		ExpiresAt: req.ExpiresAt,
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, deployment := range deployments {
		gatewayID := deployment.GatewayID

		log.Printf("[INFO] Broadcasting API key updated event: apiId=%s gatewayId=%s keyName=%s",
			apiId, gatewayID, keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyUpdatedEvent(gatewayID, event)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to broadcast API key updated event: apiId=%s gatewayId=%s keyName=%s error=%v",
				apiId, gatewayID, keyName, err)
		} else {
			successCount++
			log.Printf("[INFO] Successfully broadcast API key updated event: apiId=%s gatewayId=%s keyName=%s",
				apiId, gatewayID, keyName)
		}
	}

	// Log summary
	log.Printf("[INFO] API key update broadcast summary: apiId=%s keyName=%s total=%d success=%d failed=%d",
		apiId, keyName, len(deployments), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		log.Printf("[ERROR] Failed to deliver API key update to any gateway: apiId=%s keyName=%s", apiId, keyName)
		return fmt.Errorf("failed to deliver API key update event to any gateway: %w", lastError)
	}

	// Partial success is still considered success (some gateways received the event)
	return nil
}

// RevokeAPIKey broadcasts API key revocation to all gateways where the API is deployed
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, apiId, orgId, keyName string) error {
	// Validate API exists and get its deployments
	api, err := s.apiRepo.GetAPIByUUID(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API: %w", err)
	}
	if api == nil {
		return constants.ErrAPINotFound
	}

	// Get all deployments for this API to find target gateways
	deployments, err := s.apiRepo.GetDeploymentsByAPIUUID(apiId, orgId, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get API deployments: %w", err)
	}

	if len(deployments) == 0 {
		return constants.ErrGatewayUnavailable
	}

	// Build the API key revoked event
	event := &model.APIKeyRevokedEvent{
		ApiId:   apiId,
		KeyName: keyName,
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, deployment := range deployments {
		gatewayID := deployment.GatewayID

		log.Printf("[INFO] Broadcasting API key revoked event: apiId=%s gatewayId=%s keyName=%s",
			apiId, gatewayID, keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, event)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to broadcast API key revoked event: apiId=%s gatewayId=%s keyName=%s error=%v",
				apiId, gatewayID, keyName, err)
		} else {
			successCount++
			log.Printf("[INFO] Successfully broadcast API key revoked event: apiId=%s gatewayId=%s keyName=%s",
				apiId, gatewayID, keyName)
		}
	}

	// Log summary
	log.Printf("[INFO] API key revocation broadcast summary: apiId=%s keyName=%s total=%d success=%d failed=%d",
		apiId, keyName, len(deployments), successCount, failureCount)

	if failureCount > 0 {
		log.Printf("[ERROR] Failed to deliver API key revocation to all gateways: apiId=%s keyName=%s failed=%d total=%d",
			apiId, keyName, failureCount, len(deployments))
		return fmt.Errorf("failed to deliver API key revocation to %d of %d gateways: %w",
			failureCount, len(deployments), lastError)
	}

	return nil
}
