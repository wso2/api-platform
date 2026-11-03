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
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// AgentProxyAPIKeyService lists the consumer API keys of an Agent proxy.
//
// Create, update and revoke are not here: they go through the shared
// APIKeyService with kind constants.AgentProxy, exactly as REST, WebSub and
// WebBroker API keys do, so Agent keys behave the same way as REST API keys.
// Listing is the one Agent operation with no REST counterpart, and it follows
// the LLM proxy/provider listing: creator-scoped unless keyAdmin, metadata only.
type AgentProxyAPIKeyService struct {
	agentProxyRepo repository.AgentProxyRepository
	apiKeyRepo     repository.APIKeyRepository
	identity       *IdentityService
}

// NewAgentProxyAPIKeyService creates a new AgentProxyAPIKeyService.
func NewAgentProxyAPIKeyService(
	agentProxyRepo repository.AgentProxyRepository,
	apiKeyRepo repository.APIKeyRepository,
	identity *IdentityService,
) *AgentProxyAPIKeyService {
	return &AgentProxyAPIKeyService{
		agentProxyRepo: agentProxyRepo,
		apiKeyRepo:     apiKeyRepo,
		identity:       identity,
	}
}

// List returns the metadata of the Agent proxy's API keys the caller may manage —
// the keys they created, or every key on the Agent proxy when keyAdmin is true
// (the caller holds constants.ScopeAPIKeyAllManage). The public handle is
// resolved within the organization, and keys are read by the Agent proxy's
// internal UUID. Key material never appears in the result: APIKeyItemFromModel
// carries only the masked form.
func (s *AgentProxyAPIKeyService) List(
	ctx context.Context,
	orgUUID, handle, callerUserID string,
	keyAdmin bool,
	limit, offset int,
) (*api.AgentProxyAPIKeyListResponse, error) {
	if strings.TrimSpace(handle) == "" {
		return nil, apperror.ValidationFailed.New("The Agent proxy id is required.")
	}
	proxy, err := s.agentProxyRepo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent proxy %s: %w", handle, err)
	}
	if proxy == nil {
		return nil, apperror.AgentProxyNotFound.New()
	}

	keys, err := s.apiKeyRepo.ListByArtifact(proxy.UUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys for agent proxy %s: %w", handle, err)
	}

	items, err := ownedAPIKeyItems(keys, callerUserID, keyAdmin, s.identity)
	if err != nil {
		return nil, err
	}

	// Keys for one Agent proxy, scoped to the caller, are a small bounded set, so
	// the total is the full filtered count and the window is applied in memory —
	// which keeps count and page drawn from the same filtered list.
	total := len(items)
	page := paginateSlice(items, limit, offset)
	return &api.AgentProxyAPIKeyListResponse{
		List:       page,
		Count:      len(page),
		Pagination: api.Pagination{Total: total, Offset: offset, Limit: limit},
	}, nil
}
