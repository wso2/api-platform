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

package metadataxds

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// MetadataXDSStateManager provides high-level metadata XDS management operations
// with state-of-the-world xDS updates
type MetadataXDSStateManager struct {
	snapshotManager *MetadataXDSSnapshotManager
	store           *storage.MetadataXDSStore
	logger          *slog.Logger
}

// NewMetadataXDSStateManager creates a new metadata XDS state manager
func NewMetadataXDSStateManager(store *storage.MetadataXDSStore, snapshotManager *MetadataXDSSnapshotManager, logger *slog.Logger) *MetadataXDSStateManager {
	return &MetadataXDSStateManager{
		snapshotManager: snapshotManager,
		store:           store,
		logger:          logger,
	}
}

// StoreResource stores a metadata XDS and updates the policy engine with the complete state
func (lrm *MetadataXDSStateManager) StoreResource(resource *storage.MetadataXDS, correlationID string) error {
	if resource == nil {
		lrm.logger.Error("Cannot store nil metadata XDS resource",
			slog.String("correlation_id", correlationID))
		return fmt.Errorf("nil metadata XDS resource")
	}

	lrm.logger.Info("Storing metadata XDS with state-of-the-world update",
		slog.String("id", resource.ID),
		slog.String("resource_type", resource.ResourceType),
		slog.String("correlation_id", correlationID))

	// Store the metadata XDS in the store and update the snapshot
	if err := lrm.snapshotManager.StoreResource(resource); err != nil {
		lrm.logger.Error("Failed to store metadata XDS and update snapshot",
			slog.String("resource_id", resource.ID),
			slog.Any("error", err))
		return fmt.Errorf("failed to store metadata XDS: %w", err)
	}

	lrm.logger.Info("Successfully stored metadata XDS and updated policy engine state",
		slog.String("resource_id", resource.ID),
		slog.String("correlation_id", correlationID))

	return nil
}

// RemoveResource removes a metadata XDS and updates the policy engine with the complete state
func (lrm *MetadataXDSStateManager) RemoveResource(id, correlationID string) error {
	lrm.logger.Info("Removing metadata XDS with state-of-the-world update",
		slog.String("id", id),
		slog.String("correlation_id", correlationID))

	// Remove the metadata XDS and update the snapshot
	if err := lrm.snapshotManager.RemoveResource(id); err != nil {
		lrm.logger.Error("Failed to remove metadata XDS and update snapshot",
			slog.String("resource_id", id),
			slog.Any("error", err))
		return fmt.Errorf("failed to remove metadata XDS: %w", err)
	}

	lrm.logger.Info("Successfully removed metadata XDS and updated policy engine state",
		slog.String("resource_id", id),
		slog.String("correlation_id", correlationID))

	return nil
}

// RemoveResourcesByType removes all metadata XDSs of a specific type and updates the policy engine with the complete state
func (lrm *MetadataXDSStateManager) RemoveResourcesByType(resourceType, correlationID string) error {
	lrm.logger.Info("Removing metadata XDSs by type with state-of-the-world update",
		slog.String("resource_type", resourceType),
		slog.String("correlation_id", correlationID))

	// Remove metadata XDSs by type and update the snapshot
	if err := lrm.snapshotManager.RemoveResourcesByType(resourceType); err != nil {
		lrm.logger.Error("Failed to remove metadata XDSs by type and update snapshot",
			slog.String("resource_type", resourceType),
			slog.Any("error", err))
		return fmt.Errorf("failed to remove metadata XDSs by type: %w", err)
	}

	lrm.logger.Info("Successfully removed metadata XDSs by type and updated policy engine state",
		slog.String("resource_type", resourceType),
		slog.String("correlation_id", correlationID))

	return nil
}

// GetResourceByID retrieves a metadata XDS by its ID
func (lrm *MetadataXDSStateManager) GetResourceByID(id string) (*storage.MetadataXDS, bool) {
	return lrm.store.GetByID(id)
}

// GetResourcesByType retrieves all metadata XDSs of a specific type
func (lrm *MetadataXDSStateManager) GetResourcesByType(resourceType string) map[string]*storage.MetadataXDS {
	return lrm.store.GetByType(resourceType)
}

// GetAllResources retrieves all metadata XDSs
func (lrm *MetadataXDSStateManager) GetAllResources() []*storage.MetadataXDS {
	return lrm.store.GetAll()
}

// GetResourceCount returns the total number of metadata XDSs
func (lrm *MetadataXDSStateManager) GetResourceCount() int {
	return lrm.store.Count()
}

// RefreshSnapshot manually triggers a snapshot refresh with the current state
func (lrm *MetadataXDSStateManager) RefreshSnapshot() error {
	lrm.logger.Info("Manually refreshing metadata XDS snapshot")

	if err := lrm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		lrm.logger.Error("Failed to refresh metadata XDS snapshot", slog.Any("error", err))
		return fmt.Errorf("failed to refresh snapshot: %w", err)
	}

	lrm.logger.Info("Successfully refreshed metadata XDS snapshot")
	return nil
}