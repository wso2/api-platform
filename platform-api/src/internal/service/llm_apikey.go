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
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// LLMProviderAPIKeyService handles API key management for LLM providers
type LLMProviderAPIKeyService struct {
	llmProviderRepo      repository.LLMProviderRepository
	gatewayRepo          repository.GatewayRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
}

// NewLLMProviderAPIKeyService creates a new LLM provider API key service instance
func NewLLMProviderAPIKeyService(
	llmProviderRepo repository.LLMProviderRepository,
	gatewayRepo repository.GatewayRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	slogger *slog.Logger,
) *LLMProviderAPIKeyService {
	return &LLMProviderAPIKeyService{
		llmProviderRepo:      llmProviderRepo,
		gatewayRepo:          gatewayRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
	}
}

// ListLLMProviderAPIKeys returns all API keys for an LLM provider.
func (s *LLMProviderAPIKeyService) ListLLMProviderAPIKeys(
	ctx context.Context,
	providerID, orgID string,
) (*api.LLMProviderAPIKeyListResponse, error) {

	provider, err := s.llmProviderRepo.GetByID(providerID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM provider for API key listing", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to get LLM provider: %w", err)
	}
	if provider == nil {
		return nil, constants.ErrAPINotFound
	}

	keys, err := s.apiKeyRepo.ListByArtifact(provider.UUID)
	if err != nil {
		s.slogger.Error("Failed to list API keys for LLM provider", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	items := make([]api.APIKeyItem, 0, len(keys))
	for _, k := range keys {
		item := api.APIKeyItem{
			Name:           k.Name,
			MaskedApiKey:   k.MaskedAPIKey,
			Status:         k.Status,
			CreatedAt:      k.CreatedAt,
			CreatedBy:      k.CreatedBy,
			UpdatedAt:      k.UpdatedAt,
			ExpiresAt:      k.ExpiresAt,
			ProvisionedBy:  k.ProvisionedBy,
			AllowedTargets: k.AllowedTargets,
		}
		items = append(items, item)
	}

	return &api.LLMProviderAPIKeyListResponse{
		Items: items,
		Count: len(items),
	}, nil
}

// DeleteLLMProviderAPIKey broadcasts a revoke event to gateways and deletes the API key from the database.
func (s *LLMProviderAPIKeyService) DeleteLLMProviderAPIKey(
	ctx context.Context,
	providerID, orgID, userID, keyName string,
) error {

	provider, err := s.llmProviderRepo.GetByID(providerID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM provider for API key deletion", "providerId", providerID, "error", err)
		return fmt.Errorf("failed to get LLM provider: %w", err)
	}
	if provider == nil {
		return constants.ErrAPINotFound
	}

	existingKey, err := s.apiKeyRepo.GetByArtifactAndName(provider.UUID, keyName)
	if err != nil {
		s.slogger.Error("Failed to look up API key for deletion", "providerId", providerID, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to look up API key: %w", err)
	}
	if existingKey == nil {
		return constants.ErrAPIKeyNotFound
	}

	gateways, err := s.gatewayRepo.GetByOrganizationID(orgID)
	if err != nil {
		s.slogger.Error("Failed to get gateways for API key deletion broadcast", "providerId", providerID, "error", err)
		return fmt.Errorf("failed to get gateways: %w", err)
	}
	if len(gateways) == 0 {
		s.slogger.Warn("No gateways found for organization", "organizationId", orgID)
		return constants.ErrGatewayUnavailable
	}

	event := &model.APIKeyRevokedEvent{
		ApiId:   providerID,
		KeyName: keyName,
	}

	targetGateways := filterGatewaysByAllowedTargets(gateways, existingKey.AllowedTargets)
	successCount := 0
	var lastError error
	for _, gateway := range targetGateways {
		gatewayID := gateway.ID
		if err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, userID, event); err != nil {
			lastError = err
			s.slogger.Error("Failed to broadcast LLM provider API key revoked event", "providerId", providerID, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast LLM provider API key revoked event", "providerId", providerID, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	if successCount == 0 {
		s.slogger.Error("Failed to deliver LLM provider API key revoke event to any gateway", "providerId", providerID, "keyName", keyName)
		return fmt.Errorf("failed to deliver API key revoke event to any gateway: %w", lastError)
	}

	if err := s.apiKeyRepo.Delete(provider.UUID, keyName); err != nil {
		s.slogger.Error("Failed to delete LLM provider API key from database", "providerId", providerID, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	s.slogger.Info("Successfully deleted LLM provider API key", "providerId", providerID, "keyName", keyName)
	return nil
}

// CreateLLMProviderAPIKey generates an API key for an LLM provider and broadcasts it to all gateways.
func (s *LLMProviderAPIKeyService) CreateLLMProviderAPIKey(
	ctx context.Context,
	providerID, orgID, userID string,
	req *api.CreateLLMProviderAPIKeyRequest,
) (*api.CreateLLMProviderAPIKeyResponse, error) {

	provider, err := s.llmProviderRepo.GetByID(providerID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM provider for API key creation", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to get LLM provider: %w", err)
	}
	if provider == nil {
		s.slogger.Warn("LLM provider not found", "providerId", providerID, "organizationId", orgID)
		return nil, constants.ErrAPINotFound
	}

	apiKey, err := utils.GenerateAPIKey()
	if err != nil {
		s.slogger.Error("Failed to generate API key for LLM provider", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	var name string
	if req.Name != nil && *req.Name != "" {
		name = *req.Name
	} else {
		displayName := ""
		if req.DisplayName != nil {
			displayName = *req.DisplayName
		}
		if displayName == "" {
			s.slogger.Error("Failed to generate API key name", "providerId", providerID, "error", constants.ErrHandleSourceEmpty)
			return nil, fmt.Errorf("failed to generate API key name: both name and displayName are empty: %w", constants.ErrHandleSourceEmpty)
		}

		name, err = utils.GenerateHandle(displayName, nil)
		if err != nil {
			s.slogger.Error("Failed to generate API key name", "providerId", providerID, "error", err)
			return nil, fmt.Errorf("failed to generate API key name: %w", err)
		}
	}

	displayName := name
	if req.DisplayName != nil && *req.DisplayName != "" {
		displayName = *req.DisplayName
	}

	var expiresAt *string
	if req.ExpiresAt != nil {
		expiresAtStr := req.ExpiresAt.Format(time.RFC3339)
		expiresAt = &expiresAtStr
	}

	if displayName == "" {
		displayName = name
	}

	gateways, err := s.gatewayRepo.GetByOrganizationID(orgID)
	if err != nil {
		s.slogger.Error("Failed to get gateways for API key broadcast", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to get gateways: %w", err)
	}

	if len(gateways) == 0 {
		s.slogger.Warn("No gateways found for organization", "organizationId", orgID)
		return nil, constants.ErrGatewayUnavailable
	}

	apiKeyHashesJSON, err := buildAPIKeyHashesJSON(apiKey, []string{defaultHashingAlgorithm})
	if err != nil {
		s.slogger.Error("Failed to hash API key for LLM provider", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}
	maskedAPIKey := maskAPIKey(apiKey)

	apiKeyUUID, err := utils.GenerateUUID()
	if err != nil {
		s.slogger.Error("Failed to generate UUID for LLM provider API key", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to generate API key UUID: %w", err)
	}

	// Apply defaults for provisionedBy and allowedTargets
	var provisionedBy *string
	if req.ProvisionedBy != nil && strings.TrimSpace(*req.ProvisionedBy) != "" {
		v := strings.TrimSpace(*req.ProvisionedBy)
		provisionedBy = &v
	}
	allowedTargets := constants.APIKeyAllowedTargetsAll
	if req.AllowedTargets != nil && strings.TrimSpace(*req.AllowedTargets) != "" {
		allowedTargets = strings.TrimSpace(*req.AllowedTargets)
	}

	// Persist the API key to the database before broadcasting
	dbKey := &model.APIKey{
		UUID:           apiKeyUUID,
		ArtifactUUID:   provider.UUID,
		Name:           name,
		MaskedAPIKey:   maskedAPIKey,
		APIKeyHashes:   apiKeyHashesJSON,
		Status:         "active",
		CreatedBy:      userID,
		ExpiresAt:      req.ExpiresAt,
		ProvisionedBy:  provisionedBy,
		AllowedTargets: allowedTargets,
	}
	if err := s.apiKeyRepo.Create(dbKey); err != nil {
		s.slogger.Error("Failed to persist LLM provider API key to database", "providerId", providerID, "keyName", name, "error", err)
		return nil, fmt.Errorf("failed to persist API key: %w", err)
	}

	event := &model.APIKeyCreatedEvent{
		UUID:           apiKeyUUID,
		ApiId:          providerID,
		Name:           name,
		ApiKeyHashes:   apiKeyHashesJSON,
		MaskedApiKey:   maskedAPIKey,
		ExpiresAt:      expiresAt,
		ProvisionedBy:  provisionedBy,
		AllowedTargets: allowedTargets,
	}

	targetGateways := filterGatewaysByAllowedTargets(gateways, allowedTargets)
	successCount := 0
	failureCount := 0
	var lastError error

	for _, gateway := range targetGateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting LLM provider API key created event", "providerId", providerID, "gatewayId", gatewayID, "keyName", name)

		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gatewayID, userID, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast LLM provider API key created event", "providerId", providerID, "gatewayId", gatewayID, "keyName", name, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast LLM provider API key created event", "providerId", providerID, "gatewayId", gatewayID, "keyName", name)
		}
	}

	s.slogger.Info("LLM provider API key creation broadcast summary", "providerId", providerID, "keyName", name, "total", len(targetGateways), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		s.slogger.Error("Failed to deliver LLM provider API key to any gateway", "providerId", providerID, "keyName", name)
		return nil, fmt.Errorf("failed to deliver API key event to any gateway: %w", lastError)
	}

	return &api.CreateLLMProviderAPIKeyResponse{
		Status:  "success",
		Message: "API key created and broadcasted to gateways successfully",
		KeyId:   name,
		ApiKey:  apiKey,
	}, nil
}
