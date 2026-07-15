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

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// APIKeyUserService handles listing API keys across artifact types for a given user.
type APIKeyUserService struct {
	apiKeyRepo repository.APIKeyRepository
	identity   *IdentityService
	slogger    *slog.Logger
}

// NewAPIKeyUserService creates a new APIKeyUserService instance.
func NewAPIKeyUserService(
	apiKeyRepo repository.APIKeyRepository,
	identity *IdentityService,
	slogger *slog.Logger,
) *APIKeyUserService {
	return &APIKeyUserService{
		apiKeyRepo: apiKeyRepo,
		identity:   identity,
		slogger:    slogger,
	}
}

// ListAPIKeysByUser returns API keys created by the given user within the org, optionally filtered by artifact types.
func (s *APIKeyUserService) ListAPIKeysByUser(
	ctx context.Context,
	orgID, username string,
	types []string,
	limit, offset int,
) (*api.UserAPIKeyListResponse, error) {

	keys, err := s.apiKeyRepo.ListAPIKeysByUser(orgID, username, types)
	if err != nil {
		s.slogger.Error("Failed to list API keys for user", "username", username, "orgId", orgID, "error", err)
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	items := make([]api.UserAPIKeyItem, 0, len(keys))
	createdByFields := make([]**string, 0, len(keys))
	for _, k := range keys {
		item := api.UserAPIKeyItem{
			Id:             &k.Name,
			DisplayName:    k.DisplayName,
			MaskedApiKey:   k.MaskedAPIKey,
			Status:         api.UserAPIKeyItemStatus(k.Status),
			CreatedAt:      k.CreatedAt,
			CreatedBy:      utils.StringPtrIfNotEmpty(k.CreatedBy),
			UpdatedAt:      k.UpdatedAt,
			ExpiresAt:      k.ExpiresAt,
			Issuer:         k.Issuer,
			AllowedTargets: k.AllowedTargets,
			ArtifactId:     k.ArtifactHandle,
			ArtifactType:   api.UserAPIKeyItemArtifactType(k.ArtifactType),
		}
		items = append(items, item)
		createdByFields = append(createdByFields, &items[len(items)-1].CreatedBy)
	}
	if err := s.identity.ResolveIdentityFields(createdByFields); err != nil {
		return nil, err
	}

	// A user's API keys are a small, bounded set, so the total is the full count
	// and the window is applied in memory.
	total := len(items)
	page := paginateSlice(items, limit, offset)

	return &api.UserAPIKeyListResponse{
		List:       page,
		Count:      len(page),
		Pagination: api.Pagination{Total: total, Offset: offset, Limit: limit},
	}, nil
}
