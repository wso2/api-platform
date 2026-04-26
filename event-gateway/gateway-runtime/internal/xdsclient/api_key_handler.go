/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package xdsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/wso2/api-platform/common/apikey"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// APIKeyStateHandler keeps the shared API key validation store in sync with xDS snapshots.
type APIKeyStateHandler struct {
	store *apikey.APIkeyStore
}

// NewAPIKeyStateHandler creates a new API key state handler.
func NewAPIKeyStateHandler(store *apikey.APIkeyStore) *APIKeyStateHandler {
	return &APIKeyStateHandler{store: store}
}

// HandleResources replaces the API key store with the latest xDS state.
func (h *APIKeyStateHandler) HandleResources(ctx context.Context, resources []*discoveryv3.Resource, version string) error {
	var allKeys []APIKeyData
	for _, res := range resources {
		if res == nil || res.Resource == nil {
			continue
		}

		state, err := decodeAPIKeyStateResource(res.Resource)
		if err != nil {
			return fmt.Errorf("failed to decode API key state: %w", err)
		}

		allKeys = append(allKeys, state.APIKeys...)
	}

	newMap := make(map[string]map[string]*apikey.APIKey)
	for _, key := range allKeys {
		issuer := key.Issuer
		ak := &apikey.APIKey{
			ID:              key.ID,
			Name:            key.Name,
			APIKey:          key.APIKey,
			APIId:           key.APIId,
			ApplicationID:   key.ApplicationID,
			ApplicationName: key.ApplicationName,
			Operations:      key.Operations,
			Status:          apikey.APIKeyStatus(key.Status),
			CreatedAt:       key.CreatedAt,
			CreatedBy:       key.CreatedBy,
			UpdatedAt:       key.UpdatedAt,
			ExpiresAt:       key.ExpiresAt,
			Source:          key.Source,
			Issuer:          issuer,
		}

		if err := addAPIKeyToSnapshot(newMap, key.APIId, ak); err != nil {
			return fmt.Errorf("failed to build API key snapshot for API key %q and API %q: %w", key.ID, key.APIId, err)
		}
	}

	if err := h.store.ReplaceAll(newMap); err != nil {
		return fmt.Errorf("failed to replace API key store: %w", err)
	}

	slog.InfoContext(ctx, "Updated event-gateway API key state",
		"version", version,
		"resources", len(resources),
		"api_keys", len(allKeys))

	return nil
}

func decodeAPIKeyStateResource(resource *anypb.Any) (*APIKeyStateResource, error) {
	innerAny := &anypb.Any{}
	if err := proto.Unmarshal(resource.Value, innerAny); err != nil {
		return nil, err
	}

	apiKeyStruct := &structpb.Struct{}
	if err := proto.Unmarshal(innerAny.Value, apiKeyStruct); err != nil {
		return nil, err
	}

	data, err := json.Marshal(apiKeyStruct.AsMap())
	if err != nil {
		return nil, err
	}

	var state APIKeyStateResource
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func addAPIKeyToSnapshot(snapshot map[string]map[string]*apikey.APIKey, apiId string, apiKey *apikey.APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("API key cannot be nil")
	}

	apiKey.APIKey = strings.TrimSpace(apiKey.APIKey)
	if apiKey.APIKey == "" {
		return fmt.Errorf("%w: API key hash cannot be empty", apikey.ErrInvalidInput)
	}

	apiKeys, apiIdExists := snapshot[apiId]
	var existingHash string
	if apiIdExists {
		for hash, existingKey := range apiKeys {
			if existingKey.Name == apiKey.Name {
				existingHash = hash
				break
			}
		}
	}
	if existingHash != "" {
		delete(apiKeys, existingHash)
	}

	if snapshot[apiId] == nil {
		snapshot[apiId] = make(map[string]*apikey.APIKey)
	}
	if _, exists := snapshot[apiId][apiKey.APIKey]; exists {
		return apikey.ErrConflict
	}

	snapshot[apiId][apiKey.APIKey] = apiKey
	return nil
}

// APIKeyStateResource represents the complete API key snapshot distributed over xDS.
type APIKeyStateResource struct {
	APIKeys []APIKeyData `json:"apiKeys"`
	Version int64        `json:"version"`
}

// APIKeyData represents one API key entry in the xDS state snapshot.
type APIKeyData struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	APIKey          string     `json:"apiKey"`
	APIId           string     `json:"apiId"`
	ApplicationID   string     `json:"applicationId,omitempty"`
	ApplicationName string     `json:"applicationName,omitempty"`
	Operations      string     `json:"operations"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"createdAt"`
	CreatedBy       string     `json:"createdBy"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	Source          string     `json:"source"`
	Issuer          *string    `json:"issuer,omitempty"`
}
