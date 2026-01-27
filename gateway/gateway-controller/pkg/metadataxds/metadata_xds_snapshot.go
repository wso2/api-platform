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

package metadataxds

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
	// MetadataXDSTypeURL is the custom type URL for metadata XDS configurations
	MetadataXDSTypeURL = "api-platform.wso2.org/v1.MetadataXDSs"
)

// MetadataXDSSnapshotManager manages xDS snapshots for metadata XDS configurations
type MetadataXDSSnapshotManager struct {
	cache      *cache.LinearCache
	store      *storage.MetadataXDSStore
	logger     *slog.Logger
	nodeID     string
	mu         sync.RWMutex
	translator *MetadataXDSTranslator
}

// NewMetadataXDSSnapshotManager creates a new metadata XDS snapshot manager
func NewMetadataXDSSnapshotManager(store *storage.MetadataXDSStore, log *slog.Logger) *MetadataXDSSnapshotManager {
	// Create a LinearCache for MetadataXDS type URL
	linearCache := cache.NewLinearCache(
		MetadataXDSTypeURL,
		cache.WithLogger(logger.NewXDSLogger(log)),
	)

	return &MetadataXDSSnapshotManager{
		cache:      linearCache,
		store:      store,
		logger:     log,
		nodeID:     "policy-node",
		translator: NewMetadataXDSTranslator(log),
	}
}

// GetCache returns the underlying cache as the generic Cache interface
func (sm *MetadataXDSSnapshotManager) GetCache() cache.Cache {
	return sm.cache
}

// UpdateSnapshot generates a new xDS snapshot from all metadata XDS configurations
func (sm *MetadataXDSSnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get all metadata XDSs from store
	resources := sm.store.GetAll()

	sm.logger.Info("Updating metadata XDS snapshot",
		slog.Int("resource_count", len(resources)),
		slog.String("node_id", sm.nodeID))

	// Translate resources to xDS resources
	resourcesMap, err := sm.translator.TranslateResources(resources)
	if err != nil {
		sm.logger.Error("Failed to translate metadata XDSs", slog.Any("error", err))
		return fmt.Errorf("failed to translate metadata XDSs: %w", err)
	}

	// Get the metadata XDSs from the map
	metadataXDSs, ok := resourcesMap[MetadataXDSTypeURL]
	if !ok {
		sm.logger.Warn("No metadata XDSs found after translation")
		metadataXDSs = []types.Resource{} // Empty resources
	}

	// Increment resource version
	version := sm.store.IncrementResourceVersion()
	versionStr := fmt.Sprintf("%d", version)

	// For LinearCache, convert []types.Resource to map[string]types.Resource
	resourcesById := make(map[string]types.Resource)
	// Since we bundle all resources into a single state resource,
	// use a fixed key for the aggregated resource
	if len(metadataXDSs) > 0 {
		resourcesById["metadata-xdss-state"] = metadataXDSs[0]
	}

	// Update the linear cache with new resources
	sm.cache.SetResources(resourcesById)

	sm.logger.Info("Metadata XDS snapshot updated successfully",
		slog.String("version", versionStr),
		slog.Int("resource_count", len(resources)))

	return nil
}

// StoreResource stores a metadata XDS and updates the snapshot
func (sm *MetadataXDSSnapshotManager) StoreResource(resource *storage.MetadataXDS) error {
	sm.logger.Info("Storing metadata XDS",
		slog.String("id", resource.ID),
		slog.String("resource_type", resource.ResourceType))

	// Store in the metadata XDS store
	sm.store.Store(resource)

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// RemoveResource removes a metadata XDS and updates the snapshot
func (sm *MetadataXDSSnapshotManager) RemoveResource(id string) error {
	sm.logger.Info("Removing metadata XDS", slog.String("id", id))

	// Remove from the metadata XDS store
	if !sm.store.RemoveByID(id) {
		return fmt.Errorf("metadata XDS not found: %s", id)
	}

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// RemoveResourcesByType removes all resources of a specific type and updates the snapshot
func (sm *MetadataXDSSnapshotManager) RemoveResourcesByType(resourceType string) error {
	sm.logger.Info("Removing metadata XDSs by type", slog.String("resource_type", resourceType))

	// Remove from the metadata XDS store
	count := sm.store.RemoveByType(resourceType)

	sm.logger.Info("Removed metadata XDSs by type",
		slog.String("resource_type", resourceType),
		slog.Int("count", count))

	// Update the snapshot to reflect the new state
	return sm.UpdateSnapshot(context.Background())
}

// MetadataXDSTranslator converts metadata XDSs to xDS resources
type MetadataXDSTranslator struct {
	logger *slog.Logger
}

// NewMetadataXDSTranslator creates a new metadata XDS translator
func NewMetadataXDSTranslator(logger *slog.Logger) *MetadataXDSTranslator {
	return &MetadataXDSTranslator{
		logger: logger,
	}
}

// MetadataXDSStateResource represents the complete state of metadata XDSs
type MetadataXDSStateResource struct {
	Resources []MetadataXDSData `json:"resources"`
	Version   int64              `json:"version"`
	Timestamp int64              `json:"timestamp"`
}

// MetadataXDSData represents a metadata XDS in the state resource
type MetadataXDSData struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

// TranslateResources translates metadata XDSs to xDS resources
func (t *MetadataXDSTranslator) TranslateResources(resources []*storage.MetadataXDS) (map[string][]types.Resource, error) {
	resourcesMap := make(map[string][]types.Resource)

	// Convert all resources to a single state resource
	resourceData := make([]MetadataXDSData, 0, len(resources))
	for _, resource := range resources {
		data := MetadataXDSData{
			ID:           resource.ID,
			ResourceType: resource.ResourceType,
			Resource:     resource.Resource,
		}
		resourceData = append(resourceData, data)
	}

	// Create the state resource
	stateResource := MetadataXDSStateResource{
		Resources: resourceData,
		Version:   1, // This will be managed by the cache version
		Timestamp: 0, // Current timestamp will be set by the receiving end
	}

	// Convert to xDS resource
	resource, err := t.createMetadataXDSStateResource(&stateResource)
	if err != nil {
		t.logger.Error("Failed to create metadata XDS state resource", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create metadata XDS state resource: %w", err)
	}

	resourcesMap[MetadataXDSTypeURL] = []types.Resource{resource}

	t.logger.Debug("Translated metadata XDSs to xDS resources",
		slog.Int("resource_count", len(resources)),
		slog.Int("xds_resource_count", len(resourcesMap[MetadataXDSTypeURL])))

	return resourcesMap, nil
}

// createMetadataXDSStateResource creates an xDS resource for metadata XDS state
func (t *MetadataXDSTranslator) createMetadataXDSStateResource(stateResource *MetadataXDSStateResource) (types.Resource, error) {
	// Marshal to JSON
	resourceJSON, err := json.Marshal(stateResource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata XDS state resource: %w", err)
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
	resource.TypeUrl = MetadataXDSTypeURL

	return resource, nil
}
