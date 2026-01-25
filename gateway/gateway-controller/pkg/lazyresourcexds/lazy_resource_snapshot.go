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
 * software distributed under an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package lazyresourcexds

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"log/slog"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// LazyResourceTypeURL is the custom type URL for lazy resource configurations
	LazyResourceTypeURL = "api-platform.wso2.org/v1.LazyResources"
)

// LazyResourceSnapshotManager manages xDS snapshots for lazy resource configurations
type LazyResourceSnapshotManager struct {
	cache      *cache.LinearCache
	store      *storage.LazyResourceStore
	logger     *slog.Logger
	nodeID     string
	mu         sync.RWMutex
	translator *LazyResourceTranslator
}

// NewLazyResourceSnapshotManager creates a new lazy resource snapshot manager
func NewLazyResourceSnapshotManager(store *storage.LazyResourceStore, log *slog.Logger) *LazyResourceSnapshotManager {
	// Create a LinearCache for LazyResource type URL
	linearCache := cache.NewLinearCache(
		LazyResourceTypeURL,
		cache.WithLogger(logger.NewXDSLogger(log)),
	)

	return &LazyResourceSnapshotManager{
		cache:      linearCache,
		store:      store,
		logger:     log,
		nodeID:     "policy-node",
		translator: NewLazyResourceTranslator(log),
	}
}

// GetCache returns the underlying cache as the generic Cache interface
func (sm *LazyResourceSnapshotManager) GetCache() cache.Cache {
	return sm.cache
}

// UpdateSnapshot generates a new xDS snapshot from all lazy resource configurations
func (sm *LazyResourceSnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get all lazy resources from store
	resources := sm.store.GetAll()

	sm.logger.Info("Updating lazy resource snapshot",
		slog.Int("resource_count", len(resources)),
		slog.String("node_id", sm.nodeID))

	// Translate resources to xDS resources
	resourcesMap, err := sm.translator.TranslateResources(resources)
	if err != nil {
		sm.logger.Error("Failed to translate lazy resources", err)
		return fmt.Errorf("failed to translate lazy resources: %w", err)
	}

	// Get the lazy resources from the map
	lazyResources, ok := resourcesMap[LazyResourceTypeURL]
	if !ok {
		sm.logger.Warn("No lazy resources found after translation")
		lazyResources = []types.Resource{} // Empty resources
	}

	// Increment resource version
	version := sm.store.IncrementResourceVersion()
	versionStr := fmt.Sprintf("%d", version)

	// For LinearCache, convert []types.Resource to map[string]types.Resource
	resourcesById := make(map[string]types.Resource)
	for i, res := range lazyResources {
		// Use index-based key since resources don't have inherent names
		resourcesById[fmt.Sprintf("lazyresource-%d", i)] = res
	}

	// Update the linear cache with new resources
	sm.cache.SetResources(resourcesById)

	sm.logger.Info("Lazy resource snapshot updated successfully",
		slog.String("version", versionStr),
		slog.Int("resource_count", len(resources)))

	return nil
}

// StoreResource stores a lazy resource and updates the snapshot
func (sm *LazyResourceSnapshotManager) StoreResource(resource *storage.LazyResource) error {
	sm.logger.Info("Storing lazy resource",
		slog.String("id", resource.ID),
		slog.String("resource_type", resource.ResourceType))

	// Store in the lazy resource store
	sm.store.Store(resource)

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// RemoveResource removes a lazy resource and updates the snapshot
func (sm *LazyResourceSnapshotManager) RemoveResource(id string) error {
	sm.logger.Info("Removing lazy resource", slog.String("id", id))

	// Remove from the lazy resource store
	if !sm.store.RemoveByID(id) {
		return fmt.Errorf("lazy resource not found: %s", id)
	}

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// RemoveResourcesByType removes all resources of a specific type and updates the snapshot
func (sm *LazyResourceSnapshotManager) RemoveResourcesByType(resourceType string) error {
	sm.logger.Info("Removing lazy resources by type", slog.String("resource_type", resourceType))

	// Remove from the lazy resource store
	count := sm.store.RemoveByType(resourceType)

	sm.logger.Info("Removed lazy resources by type",
		slog.String("resource_type", resourceType),
		slog.Int("count", count))

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// LazyResourceTranslator converts lazy resources to xDS resources
type LazyResourceTranslator struct {
	logger *slog.Logger
}

// NewLazyResourceTranslator creates a new lazy resource translator
func NewLazyResourceTranslator(logger *slog.Logger) *LazyResourceTranslator {
	return &LazyResourceTranslator{
		logger: logger,
	}
}

// LazyResourceStateResource represents the complete state of lazy resources
type LazyResourceStateResource struct {
	Resources []LazyResourceData `json:"resources"`
	Version   int64              `json:"version"`
	Timestamp int64              `json:"timestamp"`
}

// LazyResourceData represents a lazy resource in the state resource
type LazyResourceData struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

// TranslateResources translates lazy resources to xDS resources
func (t *LazyResourceTranslator) TranslateResources(resources []*storage.LazyResource) (map[string][]types.Resource, error) {
	resourcesMap := make(map[string][]types.Resource)

	// Convert all resources to a single state resource
	resourceData := make([]LazyResourceData, 0, len(resources))
	for _, resource := range resources {
		data := LazyResourceData{
			ID:           resource.ID,
			ResourceType: resource.ResourceType,
			Resource:     resource.Resource,
		}
		resourceData = append(resourceData, data)
	}

	// Create the state resource
	stateResource := LazyResourceStateResource{
		Resources: resourceData,
		Version:   1, // This will be managed by the cache version
		Timestamp: 0, // Current timestamp will be set by the receiving end
	}

	// Convert to xDS resource
	resource, err := t.createLazyResourceStateResource(&stateResource)
	if err != nil {
		t.logger.Error("Failed to create lazy resource state resource", err)
		return nil, fmt.Errorf("failed to create lazy resource state resource: %w", err)
	}

	resourcesMap[LazyResourceTypeURL] = []types.Resource{resource}

	t.logger.Debug("Translated lazy resources to xDS resources",
		slog.Int("resource_count", len(resources)),
		slog.Int("xds_resource_count", len(resourcesMap[LazyResourceTypeURL])))

	return resourcesMap, nil
}

// createLazyResourceStateResource creates an xDS resource for lazy resource state
func (t *LazyResourceTranslator) createLazyResourceStateResource(stateResource *LazyResourceStateResource) (types.Resource, error) {
	// Marshal to JSON
	resourceJSON, err := json.Marshal(stateResource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lazy resource state resource: %w", err)
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
	resource.TypeUrl = LazyResourceTypeURL

	return resource, nil
}
