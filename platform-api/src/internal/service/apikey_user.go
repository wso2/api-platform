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
	"platform-api/src/internal/repository"
)

// APIKeyUserService handles listing API keys across artifact types for a given user.
type APIKeyUserService struct {
	apiKeyRepo repository.APIKeyRepository
	slogger    *slog.Logger
}

// NewAPIKeyUserService creates a new APIKeyUserService instance.
func NewAPIKeyUserService(
	apiKeyRepo repository.APIKeyRepository,
	slogger *slog.Logger,
) *APIKeyUserService {
	return &APIKeyUserService{
		apiKeyRepo: apiKeyRepo,
		slogger:    slogger,
	}
}

// ListAPIKeysByUser returns API keys created by the given user within the org, optionally filtered by artifact types.
func (s *APIKeyUserService) ListAPIKeysByUser(
	ctx context.Context,
	orgID, username string,
	types []string,
) (*api.UserAPIKeyListResponse, error) {

	keys, err := s.apiKeyRepo.ListAPIKeysByUser(orgID, username, types)
	if err != nil {
		s.slogger.Error("Failed to list API keys for user", "username", username, "orgId", orgID, "error", err)
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	items := make([]api.UserAPIKeyItem, 0, len(keys))
	for _, k := range keys {
		item := api.UserAPIKeyItem{
			Name:           k.Name,
			MaskedApiKey:   k.MaskedAPIKey,
			Status:         api.UserAPIKeyItemStatus(k.Status),
			CreatedAt:      k.CreatedAt,
			CreatedBy:      k.CreatedBy,
			UpdatedAt:      k.UpdatedAt,
			ExpiresAt:      k.ExpiresAt,
			Issuer:         k.Issuer,
			AllowedTargets: k.AllowedTargets,
			ArtifactId:     k.ArtifactHandle,
			ArtifactType:   api.UserAPIKeyItemArtifactType(k.ArtifactKind),
		}
		items = append(items, item)
	}

	return &api.UserAPIKeyListResponse{
		Items: items,
		Count: len(items),
	}, nil
}
