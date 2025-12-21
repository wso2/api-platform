/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package apikeyxds

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// APIKeyStateTypeURL is the custom type URL for API key state configurations
	APIKeyStateTypeURL = "api-platform.wso2.org/v1.APIKeyState"
)

// APIKeySnapshotManager manages xDS snapshots for API key state
type APIKeySnapshotManager struct {
	cache      *cache.LinearCache
	store      *storage.APIKeyStore
	logger     *zap.Logger
	nodeID     string
	mu         sync.RWMutex
	translator *APIKeyTranslator
}

// NewAPIKeySnapshotManager creates a new API key snapshot manager
func NewAPIKeySnapshotManager(store *storage.APIKeyStore, logger *zap.Logger) *APIKeySnapshotManager {
	// Create a LinearCache for APIKeyState type URL
	linearCache := cache.NewLinearCache(
		APIKeyStateTypeURL,
		cache.WithLogger(logger.Sugar()),
	)

	return &APIKeySnapshotManager{
		cache:      linearCache,
		store:      store,
		logger:     logger,
		nodeID:     "policy-node",
		translator: NewAPIKeyTranslator(logger),
	}
}

// GetCache returns the underlying cache as the generic Cache interface
func (sm *APIKeySnapshotManager) GetCache() cache.Cache {
	return sm.cache
}

// UpdateSnapshot generates a new xDS snapshot from all API key configurations
func (sm *APIKeySnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get all API keys from store
	apiKeys := sm.store.GetAll()

	sm.logger.Info("Updating API key snapshot",
		zap.Int("apikey_count", len(apiKeys)),
		zap.String("node_id", sm.nodeID))

	// Translate API keys to xDS resources
	resourcesMap, err := sm.translator.TranslateAPIKeys(apiKeys)
	if err != nil {
		sm.logger.Error("Failed to translate API keys", zap.Error(err))
		return fmt.Errorf("failed to translate API keys: %w", err)
	}

	// Get the API key resources from the map
	apiKeyResources, ok := resourcesMap[APIKeyStateTypeURL]
	if !ok {
		sm.logger.Warn("No API key resources found after translation")
		apiKeyResources = []types.Resource{} // Empty resources
	}

	// Increment resource version
	version := sm.store.IncrementResourceVersion()
	versionStr := fmt.Sprintf("%d", version)

	// For LinearCache, convert []types.Resource to map[string]types.Resource
	resourcesById := make(map[string]types.Resource)
	for i, res := range apiKeyResources {
		// Use index-based key since API key resources don't have inherent names
		resourcesById[fmt.Sprintf("apikey-state-%d", i)] = res
	}

	// Update the linear cache with new resources
	sm.cache.SetResources(resourcesById)

	sm.logger.Info("API key snapshot updated successfully",
		zap.String("version", versionStr),
		zap.Int("apikey_count", len(apiKeys)))

	return nil
}

// StoreAPIKey stores an API key and updates the snapshot
func (sm *APIKeySnapshotManager) StoreAPIKey(apiKey *models.APIKey) error {
	sm.logger.Info("Storing API key",
		zap.String("id", apiKey.ID),
		zap.String("api_id", apiKey.APIId),
		zap.String("name", apiKey.Name))

	// Store in the API key store
	sm.store.Store(apiKey)

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// RevokeAPIKey revokes an API key and updates the snapshot
func (sm *APIKeySnapshotManager) RevokeAPIKey(apiKeyValue string) error {
	sm.logger.Info("Revoking API key", zap.String("api_key_value", apiKeyValue))

	// Revoke in the API key store
	if !sm.store.Revoke(apiKeyValue) {
		return fmt.Errorf("API key not found: %s", apiKeyValue)
	}

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// RemoveAPIKeysByAPI removes all API keys for an API and updates the snapshot
func (sm *APIKeySnapshotManager) RemoveAPIKeysByAPI(apiId string) error {
	sm.logger.Info("Removing API keys by API", zap.String("api_id", apiId))

	// Remove from the API key store
	count := sm.store.RemoveByAPI(apiId)

	sm.logger.Info("Removed API keys by API",
		zap.String("api_id", apiId),
		zap.Int("count", count))

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// APIKeyTranslator converts API key configurations to xDS resources
type APIKeyTranslator struct {
	logger *zap.Logger
}

// NewAPIKeyTranslator creates a new API key translator
func NewAPIKeyTranslator(logger *zap.Logger) *APIKeyTranslator {
	return &APIKeyTranslator{
		logger: logger,
	}
}

// APIKeyStateResource represents the complete state of API keys for the policy engine
type APIKeyStateResource struct {
	APIKeys   []APIKeyData `json:"apiKeys"`
	Version   int64        `json:"version"`
	Timestamp int64        `json:"timestamp"`
}

// APIKeyData represents an API key in the state resource
type APIKeyData struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	APIKey     string     `json:"apiKey"`
	APIId      string     `json:"apiId"`
	Operations string     `json:"operations"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	CreatedBy  string     `json:"createdBy"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

// TranslateAPIKeys translates API key configurations to xDS resources
func (t *APIKeyTranslator) TranslateAPIKeys(apiKeys []*models.APIKey) (map[string][]types.Resource, error) {
	resources := make(map[string][]types.Resource)

	// Convert all API keys to a single state resource
	apiKeyData := make([]APIKeyData, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		data := APIKeyData{
			ID:         apiKey.ID,
			Name:       apiKey.Name,
			APIKey:     apiKey.APIKey,
			APIId:      apiKey.APIId,
			Operations: apiKey.Operations,
			Status:     string(apiKey.Status),
			CreatedAt:  apiKey.CreatedAt,
			CreatedBy:  apiKey.CreatedBy,
			UpdatedAt:  apiKey.UpdatedAt,
			ExpiresAt:  apiKey.ExpiresAt,
		}
		apiKeyData = append(apiKeyData, data)
	}

	// Create the state resource
	stateResource := APIKeyStateResource{
		APIKeys:   apiKeyData,
		Version:   1, // This will be managed by the cache version
		Timestamp: 0, // Current timestamp will be set by the receiving end
	}

	// Convert to xDS resource
	resource, err := t.createAPIKeyStateResource(&stateResource)
	if err != nil {
		t.logger.Error("Failed to create API key state resource", zap.Error(err))
		return nil, fmt.Errorf("failed to create API key state resource: %w", err)
	}

	resources[APIKeyStateTypeURL] = []types.Resource{resource}

	t.logger.Debug("Translated API keys to xDS resources",
		zap.Int("apikey_count", len(apiKeys)),
		zap.Int("resource_count", len(resources[APIKeyStateTypeURL])))

	return resources, nil
}

// createAPIKeyStateResource creates an xDS resource for API key state
func (t *APIKeyTranslator) createAPIKeyStateResource(stateResource *APIKeyStateResource) (types.Resource, error) {
	// Marshal to JSON
	resourceJSON, err := json.Marshal(stateResource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API key state resource: %w", err)
	}

	// Convert to protobuf Struct
	structValue := &structpb.Struct{}
	if err := structValue.UnmarshalJSON(resourceJSON); err != nil {
		return nil, fmt.Errorf("failed to convert to protobuf struct: %w", err)
	}

	// Wrap in Any
	resource, err := anypb.New(structValue)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap in Any: %w", err)
	}

	// Set type URL
	resource.TypeUrl = APIKeyStateTypeURL

	return resource, nil
}
