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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/repository"
)

// LLMAPIKeyUserService handles listing LLM API keys across providers and proxies for a given user.
type LLMAPIKeyUserService struct {
	apiKeyRepo repository.APIKeyRepository
	slogger    *slog.Logger
}

// NewLLMAPIKeyUserService creates a new LLMAPIKeyUserService instance.
func NewLLMAPIKeyUserService(
	apiKeyRepo repository.APIKeyRepository,
	slogger *slog.Logger,
) *LLMAPIKeyUserService {
	return &LLMAPIKeyUserService{
		apiKeyRepo: apiKeyRepo,
		slogger:    slogger,
	}
}

// ListLLMAPIKeysByUser returns all LLM provider and proxy API keys created by the given user within the org.
func (s *LLMAPIKeyUserService) ListLLMAPIKeysByUser(
	ctx context.Context,
	orgID, username string,
) (*api.UserAPIKeyListResponse, error) {

	keys, err := s.apiKeyRepo.ListLLMAPIKeysByUser(orgID, username)
	if err != nil {
		s.slogger.Error("Failed to list LLM API keys for user", "username", username, "orgId", orgID, "error", err)
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	items := make([]api.UserAPIKeyItem, 0, len(keys))
	for _, k := range keys {
		artifactType := "llm-provider"
		if k.ArtifactKind == constants.LLMProxy {
			artifactType = "llm-proxy"
		}
		item := api.UserAPIKeyItem{
			APIKeyItem: api.APIKeyItem{
				Name:           k.Name,
				MaskedApiKey:   k.MaskedAPIKey,
				Status:         k.Status,
				CreatedAt:      k.CreatedAt,
				CreatedBy:      k.CreatedBy,
				UpdatedAt:      k.UpdatedAt,
				ExpiresAt:      k.ExpiresAt,
				Issuer:         k.Issuer,
				AllowedTargets: k.AllowedTargets,
			},
			ArtifactId:   k.ArtifactHandle,
			ArtifactType: artifactType,
		}
		items = append(items, item)
	}

	return &api.UserAPIKeyListResponse{
		Items: items,
		Count: len(items),
	}, nil
}
