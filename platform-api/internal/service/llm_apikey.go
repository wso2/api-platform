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

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// LLMProviderAPIKeyService handles API key management for LLM providers
type LLMProviderAPIKeyService struct {
	llmProviderRepo      repository.LLMProviderRepository
	apiRepo              repository.APIRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	identity             *IdentityService
	slogger              *slog.Logger
}

// NewLLMProviderAPIKeyService creates a new LLM provider API key service instance
func NewLLMProviderAPIKeyService(
	llmProviderRepo repository.LLMProviderRepository,
	apiRepo repository.APIRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	identity *IdentityService,
	slogger *slog.Logger,
) *LLMProviderAPIKeyService {
	return &LLMProviderAPIKeyService{
		llmProviderRepo:      llmProviderRepo,
		apiRepo:              apiRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		identity:             identity,
		slogger:              slogger,
	}
}

// ListLLMProviderAPIKeys returns API keys for an LLM provider, filtered to those created by userID.
func (s *LLMProviderAPIKeyService) ListLLMProviderAPIKeys(
	ctx context.Context,
	providerID, orgID, userID string,
	limit, offset int,
) (*api.LLMProviderAPIKeyListResponse, error) {

	provider, err := s.llmProviderRepo.GetByID(providerID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM provider for API key listing", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to get LLM provider: %w", err)
	}
	if provider == nil {
		return nil, apperror.ArtifactNotFound.Wrap(constants.ErrAPINotFound)
	}

	keys, err := s.apiKeyRepo.ListByArtifact(provider.UUID)
	if err != nil {
		s.slogger.Error("Failed to list API keys for LLM provider", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	items, err := ownedAPIKeyItems(keys, userID, s.identity)
	if err != nil {
		return nil, err
	}

	// API keys for one provider (scoped to the caller) are a small, bounded set,
	// so the total is the full count and the window is applied in memory.
	total := len(items)
	page := paginateSlice(items, limit, offset)

	return &api.LLMProviderAPIKeyListResponse{
		List:       page,
		Count:      len(page),
		Pagination: api.Pagination{Total: total, Offset: offset, Limit: limit},
	}, nil
}

// ownedAPIKeyItems keeps only the keys created by userID and maps them to their
// API representation, resolving each creator UUID to its raw identity.
func ownedAPIKeyItems(keys []*model.APIKey, userID string, identity *IdentityService) ([]api.APIKeyItem, error) {
	items := make([]api.APIKeyItem, 0, len(keys))
	for _, k := range keys {
		if k.CreatedBy != userID {
			continue
		}
		createdBy := utils.StringPtrIfNotEmpty(k.CreatedBy)
		if err := identity.ResolveIdentityField(&createdBy); err != nil {
			return nil, err
		}
		items = append(items, api.APIKeyItem{
			Id:             &k.Name,
			DisplayName:    k.DisplayName,
			MaskedApiKey:   k.MaskedAPIKey,
			Status:         api.APIKeyItemStatus(k.Status),
			CreatedAt:      k.CreatedAt,
			CreatedBy:      createdBy,
			UpdatedAt:      k.UpdatedAt,
			ExpiresAt:      k.ExpiresAt,
			Issuer:         k.Issuer,
			AllowedTargets: k.AllowedTargets,
		})
	}
	return items, nil
}

// DeleteLLMProviderAPIKey deletes the API key from the database and broadcasts a revoke event to gateways.
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
		return apperror.ArtifactNotFound.Wrap(constants.ErrAPINotFound)
	}

	existingKey, err := s.apiKeyRepo.GetByArtifactAndName(provider.UUID, keyName)
	if err != nil {
		s.slogger.Error("Failed to look up API key for deletion", "providerId", providerID, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to look up API key: %w", err)
	}
	if existingKey == nil {
		return apperror.LLMProviderAPIKeyNotFound.Wrap(constants.ErrAPIKeyNotFound)
	}

	if userID != "" && existingKey.CreatedBy != userID {
		return apperror.LLMProviderAPIKeyForbidden.Wrap(constants.ErrAPIKeyForbidden)
	}

	if err := s.apiKeyRepo.Delete(provider.UUID, keyName); err != nil {
		s.slogger.Error("Failed to delete LLM provider API key from database", "providerId", providerID, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	s.slogger.Info("Successfully deleted LLM provider API key", "providerId", providerID, "keyName", keyName)

	// Broadcast revoke only to gateways the provider is associated with (not all org gateways).
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(provider.UUID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get associated gateways for API key revoke broadcast", "providerId", providerID, "keyName", keyName, "error", err)
		return nil
	}
	if len(gateways) == 0 {
		s.slogger.Info("Provider not associated with any gateway; skipping revoke broadcast", "providerId", providerID, "keyName", keyName)
		return nil
	}

	event := &model.APIKeyRevokedEvent{
		ApiId:   provider.UUID,
		KeyName: keyName,
	}

	for _, gateway := range filterAPIGatewaysByAllowedTargets(gateways, existingKey.AllowedTargets) {
		gatewayID := gateway.ID
		if err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, userID, event); err != nil {
			s.slogger.Error("Failed to broadcast LLM provider API key revoked event", "providerId", providerID, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			s.slogger.Info("Successfully broadcast LLM provider API key revoked event", "providerId", providerID, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

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
		return nil, apperror.ArtifactNotFound.Wrap(constants.ErrAPINotFound)
	}

	apiKey, err := utils.GenerateAPIKey()
	if err != nil {
		s.slogger.Error("Failed to generate API key for LLM provider", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	var name string
	if req.Id != nil && *req.Id != "" {
		name = *req.Id
	} else {
		if req.DisplayName == "" {
			s.slogger.Error("Failed to generate API key name", "providerId", providerID, "error", constants.ErrHandleSourceEmpty)
			return nil, fmt.Errorf("failed to generate API key name: both name and displayName are empty: %w", constants.ErrHandleSourceEmpty)
		}
		name, err = utils.GenerateHandle(req.DisplayName, nil)
		if err != nil {
			s.slogger.Error("Failed to generate API key name", "providerId", providerID, "error", err)
			return nil, fmt.Errorf("failed to generate API key name: %w", err)
		}
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = name
	}

	var expiresAt *string
	if req.ExpiresAt != nil {
		expiresAtStr := req.ExpiresAt.Format(time.RFC3339)
		expiresAt = &expiresAtStr
	}

	// Broadcast only to gateways the provider is associated with (not all org gateways),
	// mirroring the REST key path. An empty list is valid: the key is still persisted and
	// any gateway associated later picks it up via the deploy-time backfill.
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(provider.UUID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get gateways for API key broadcast", "providerId", providerID, "error", err)
		return nil, fmt.Errorf("failed to get gateways: %w", err)
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

	// Apply defaults for issuer and allowedTargets
	var issuer *string
	if req.Issuer != nil && strings.TrimSpace(*req.Issuer) != "" {
		v := strings.TrimSpace(*req.Issuer)
		issuer = &v
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
		DisplayName:    displayName,
		MaskedAPIKey:   maskedAPIKey,
		APIKeyHashes:   apiKeyHashesJSON,
		Status:         "active",
		CreatedBy:      userID,
		ExpiresAt:      req.ExpiresAt,
		Issuer:         issuer,
		AllowedTargets: allowedTargets,
	}
	if err := s.apiKeyRepo.Create(dbKey); err != nil {
		s.slogger.Error("Failed to persist LLM provider API key to database", "providerId", providerID, "keyName", name, "error", err)
		return nil, fmt.Errorf("failed to persist API key: %w", err)
	}

	event := &model.APIKeyCreatedEvent{
		UUID:         apiKeyUUID,
		ApiId:        provider.UUID,
		Name:         name,
		ApiKeyHashes: apiKeyHashesJSON,
		MaskedApiKey: maskedAPIKey,
		ExpiresAt:    expiresAt,
		Issuer:       issuer,
		CreatedAt:    dbKey.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    dbKey.UpdatedAt.Format(time.RFC3339),
	}

	targetGateways := filterAPIGatewaysByAllowedTargets(gateways, allowedTargets)
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

	if len(targetGateways) == 0 {
		// No gateways associated yet — a valid state. The key is persisted centrally and any
		// gateway associated later picks it up via the deploy-time backfill.
		s.slogger.Info("LLM provider not associated with any gateway; API key saved and will be delivered at deploy time", "providerId", providerID, "keyName", name)
	} else if successCount == 0 {
		s.slogger.Error("LLM provider API key created event was not broadcast to any associated gateway", "providerId", providerID, "keyName", name, "error", lastError)
	}

	return &api.CreateLLMProviderAPIKeyResponse{
		Status:  "success",
		Message: "API key created successfully",
		Id:      name,
		ApiKey:  apiKey,
	}, nil
}
