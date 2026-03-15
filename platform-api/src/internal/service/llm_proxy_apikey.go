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

// LLMProxyAPIKeyService handles API key management for LLM proxies
type LLMProxyAPIKeyService struct {
	llmProxyRepo         repository.LLMProxyRepository
	gatewayRepo          repository.GatewayRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
}

// NewLLMProxyAPIKeyService creates a new LLM proxy API key service instance
func NewLLMProxyAPIKeyService(
	llmProxyRepo repository.LLMProxyRepository,
	gatewayRepo repository.GatewayRepository,
	apiKeyRepo repository.APIKeyRepository,
	gatewayEventsService *GatewayEventsService,
	slogger *slog.Logger,
) *LLMProxyAPIKeyService {
	return &LLMProxyAPIKeyService{
		llmProxyRepo:         llmProxyRepo,
		gatewayRepo:          gatewayRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
	}
}

// ListLLMProxyAPIKeys returns all API keys for an LLM proxy.
func (s *LLMProxyAPIKeyService) ListLLMProxyAPIKeys(
	ctx context.Context,
	proxyID, orgID string,
) (*api.LLMProxyAPIKeyListResponse, error) {

	proxy, err := s.llmProxyRepo.GetByID(proxyID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM proxy for API key listing", "proxyId", proxyID, "error", err)
		return nil, fmt.Errorf("failed to get LLM proxy: %w", err)
	}
	if proxy == nil {
		return nil, constants.ErrAPINotFound
	}

	keys, err := s.apiKeyRepo.ListByArtifact(proxy.UUID)
	if err != nil {
		s.slogger.Error("Failed to list API keys for LLM proxy", "proxyId", proxyID, "error", err)
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	items := make([]api.APIKeyItem, 0, len(keys))
	for _, k := range keys {
		item := api.APIKeyItem{
			Name:           k.Name,
			MaskedApiKey:   k.MaskedAPIKey,
			Status:         api.APIKeyItemStatus(k.Status),
			CreatedAt:      k.CreatedAt,
			CreatedBy:      k.CreatedBy,
			UpdatedAt:      k.UpdatedAt,
			ExpiresAt:      k.ExpiresAt,
			Issuer:         k.Issuer,
			AllowedTargets: k.AllowedTargets,
		}
		items = append(items, item)
	}

	return &api.LLMProxyAPIKeyListResponse{
		Items: items,
		Count: len(items),
	}, nil
}

// DeleteLLMProxyAPIKey deletes the API key from the database and broadcasts a revoke event to gateways.
func (s *LLMProxyAPIKeyService) DeleteLLMProxyAPIKey(
	ctx context.Context,
	proxyID, orgID, userID, keyName string,
) error {

	proxy, err := s.llmProxyRepo.GetByID(proxyID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM proxy for API key deletion", "proxyId", proxyID, "error", err)
		return fmt.Errorf("failed to get LLM proxy: %w", err)
	}
	if proxy == nil {
		return constants.ErrAPINotFound
	}

	existingKey, err := s.apiKeyRepo.GetByArtifactAndName(proxy.UUID, keyName)
	if err != nil {
		s.slogger.Error("Failed to look up API key for deletion", "proxyId", proxyID, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to look up API key: %w", err)
	}
	if existingKey == nil {
		return constants.ErrAPIKeyNotFound
	}

	// Non-admin callers (userID != "") must be the key creator.
	if userID != "" && existingKey.CreatedBy != userID {
		return constants.ErrAPIKeyForbidden
	}

	if err := s.apiKeyRepo.Delete(proxy.UUID, keyName); err != nil {
		s.slogger.Error("Failed to delete LLM proxy API key from database", "proxyId", proxyID, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	s.slogger.Info("Successfully deleted LLM proxy API key", "proxyId", proxyID, "keyName", keyName)

	// Broadcast revoke event to gateways.
	gateways, err := s.gatewayRepo.GetByOrganizationID(orgID)
	if err != nil {
		s.slogger.Error("Failed to get gateways for API key revoke broadcast", "proxyId", proxyID, "keyName", keyName, "error", err)
		return nil
	}
	if len(gateways) == 0 {
		s.slogger.Warn("No gateways found for organization; skipping revoke broadcast", "organizationId", orgID)
		return nil
	}

	event := &model.APIKeyRevokedEvent{
		ApiId:   proxyID,
		KeyName: keyName,
	}

	for _, gateway := range filterGatewaysByAllowedTargets(gateways, existingKey.AllowedTargets) {
		gatewayID := gateway.ID
		if err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, userID, event); err != nil {
			s.slogger.Error("Failed to broadcast LLM proxy API key revoked event", "proxyId", proxyID, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			s.slogger.Info("Successfully broadcast LLM proxy API key revoked event", "proxyId", proxyID, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	return nil
}

// CreateLLMProxyAPIKey generates an API key for an LLM proxy and broadcasts it to all gateways.
func (s *LLMProxyAPIKeyService) CreateLLMProxyAPIKey(
	ctx context.Context,
	proxyID, orgID, userID string,
	req *api.CreateLLMProxyAPIKeyRequest,
) (*api.CreateLLMProxyAPIKeyResponse, error) {

	proxy, err := s.llmProxyRepo.GetByID(proxyID, orgID)
	if err != nil {
		s.slogger.Error("Failed to get LLM proxy for API key creation", "proxyId", proxyID, "error", err)
		return nil, fmt.Errorf("failed to get LLM proxy: %w", err)
	}
	if proxy == nil {
		s.slogger.Warn("LLM proxy not found", "proxyId", proxyID, "organizationId", orgID)
		return nil, constants.ErrAPINotFound
	}

	apiKey, err := utils.GenerateAPIKey()
	if err != nil {
		s.slogger.Error("Failed to generate API key for LLM proxy", "proxyId", proxyID, "error", err)
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
		name, err = utils.GenerateHandle(displayName, nil)
		if err != nil {
			s.slogger.Error("Failed to generate API key name", "proxyId", proxyID, "error", err)
			return nil, fmt.Errorf("failed to generate API key name: %w", err)
		}
	}

	gateways, err := s.gatewayRepo.GetByOrganizationID(orgID)
	if err != nil {
		s.slogger.Error("Failed to get gateways for API key broadcast", "proxyId", proxyID, "error", err)
		return nil, fmt.Errorf("failed to get gateways: %w", err)
	}

	if len(gateways) == 0 {
		s.slogger.Warn("No gateways found for organization", "organizationId", orgID)
		return nil, constants.ErrGatewayUnavailable
	}

	apiKeyHashesJSON, err := buildAPIKeyHashesJSON(apiKey, []string{defaultHashingAlgorithm})
	if err != nil {
		s.slogger.Error("Failed to hash API key for LLM proxy", "proxyId", proxyID, "error", err)
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}
	maskedAPIKey := maskAPIKey(apiKey)

	apiKeyUUID, err := utils.GenerateUUID()
	if err != nil {
		s.slogger.Error("Failed to generate UUID for LLM proxy API key", "proxyId", proxyID, "error", err)
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
		ArtifactUUID:   proxy.UUID,
		Name:           name,
		MaskedAPIKey:   maskedAPIKey,
		APIKeyHashes:   apiKeyHashesJSON,
		Status:         "active",
		CreatedBy:      userID,
		ExpiresAt:      req.ExpiresAt,
		Issuer:         issuer,
		AllowedTargets: allowedTargets,
	}
	if err := s.apiKeyRepo.Create(dbKey); err != nil {
		s.slogger.Error("Failed to persist LLM proxy API key to database", "proxyId", proxyID, "keyName", name, "error", err)
		return nil, fmt.Errorf("failed to persist API key: %w", err)
	}

	var expiresAt *string
	if req.ExpiresAt != nil {
		expiresAtStr := req.ExpiresAt.Format(time.RFC3339)
		expiresAt = &expiresAtStr
	}

	event := &model.APIKeyCreatedEvent{
		UUID:         apiKeyUUID,
		ApiId:        proxyID,
		Name:         name,
		ApiKeyHashes: apiKeyHashesJSON,
		MaskedApiKey: maskedAPIKey,
		ExpiresAt:    expiresAt,
		Issuer:       issuer,
	}

	targetGateways := filterGatewaysByAllowedTargets(gateways, allowedTargets)
	successCount := 0
	failureCount := 0
	var lastError error

	for _, gateway := range targetGateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting LLM proxy API key created event", "proxyId", proxyID, "gatewayId", gatewayID, "keyName", name)

		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gatewayID, userID, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast LLM proxy API key created event", "proxyId", proxyID, "gatewayId", gatewayID, "keyName", name, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast LLM proxy API key created event", "proxyId", proxyID, "gatewayId", gatewayID, "keyName", name)
		}
	}

	s.slogger.Info("LLM proxy API key creation broadcast summary", "proxyId", proxyID, "keyName", name, "total", len(targetGateways), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		s.slogger.Warn("Failed to deliver LLM proxy API key to any gateway; key is saved to the database", "proxyId", proxyID, "keyName", name, "error", lastError)
	}

	return &api.CreateLLMProxyAPIKeyResponse{
		Status:  "success",
		Message: "API key created successfully",
		KeyId:   name,
		ApiKey:  apiKey,
	}, nil
}
