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

package lazyresourcexds

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// LazyResourceStateManager provides high-level lazy resource management operations
// with state-of-the-world xDS updates
type LazyResourceStateManager struct {
	snapshotManager *LazyResourceSnapshotManager
	store           *storage.LazyResourceStore
	logger          *slog.Logger
}

// NewLazyResourceStateManager creates a new lazy resource state manager
func NewLazyResourceStateManager(store *storage.LazyResourceStore, snapshotManager *LazyResourceSnapshotManager, logger *slog.Logger) *LazyResourceStateManager {
	return &LazyResourceStateManager{
		snapshotManager: snapshotManager,
		store:           store,
		logger:          logger,
	}
}

// StoreResource stores a lazy resource and updates the policy engine with the complete state
func (lrm *LazyResourceStateManager) StoreResource(resource *storage.LazyResource, correlationID string) error {
	lrm.logger.Info("Storing lazy resource with state-of-the-world update",
		slog.String("id", resource.ID),
		slog.String("resource_type", resource.ResourceType),
		slog.String("correlation_id", correlationID))

	// Store the lazy resource in the store and update the snapshot
	if err := lrm.snapshotManager.StoreResource(resource); err != nil {
		lrm.logger.Error("Failed to store lazy resource and update snapshot",
			slog.String("resource_id", resource.ID),
			slog.Any("error", err))
		return fmt.Errorf("failed to store lazy resource: %w", err)
	}

	lrm.logger.Info("Successfully stored lazy resource and updated policy engine state",
		slog.String("resource_id", resource.ID),
		slog.String("correlation_id", correlationID))

	return nil
}

// RemoveResource removes a lazy resource and updates the policy engine with the complete state
func (lrm *LazyResourceStateManager) RemoveResource(id, correlationID string) error {
	lrm.logger.Info("Removing lazy resource with state-of-the-world update",
		slog.String("id", id),
		slog.String("correlation_id", correlationID))

	// Remove the lazy resource and update the snapshot
	if err := lrm.snapshotManager.RemoveResource(id); err != nil {
		lrm.logger.Error("Failed to remove lazy resource and update snapshot",
			slog.String("resource_id", id),
			slog.Any("error", err))
		return fmt.Errorf("failed to remove lazy resource: %w", err)
	}

	lrm.logger.Info("Successfully removed lazy resource and updated policy engine state",
		slog.String("resource_id", id),
		slog.String("correlation_id", correlationID))

	return nil
}

// RemoveResourcesByType removes all lazy resources of a specific type and updates the policy engine with the complete state
func (lrm *LazyResourceStateManager) RemoveResourcesByType(resourceType, correlationID string) error {
	lrm.logger.Info("Removing lazy resources by type with state-of-the-world update",
		slog.String("resource_type", resourceType),
		slog.String("correlation_id", correlationID))

	// Remove lazy resources by type and update the snapshot
	if err := lrm.snapshotManager.RemoveResourcesByType(resourceType); err != nil {
		lrm.logger.Error("Failed to remove lazy resources by type and update snapshot",
			slog.String("resource_type", resourceType),
			slog.Any("error", err))
		return fmt.Errorf("failed to remove lazy resources by type: %w", err)
	}

	lrm.logger.Info("Successfully removed lazy resources by type and updated policy engine state",
		slog.String("resource_type", resourceType),
		slog.String("correlation_id", correlationID))

	return nil
}

// GetResourceByID retrieves a lazy resource by its ID
func (lrm *LazyResourceStateManager) GetResourceByID(id string) (*storage.LazyResource, bool) {
	return lrm.store.GetByID(id)
}

// GetResourcesByType retrieves all lazy resources of a specific type
func (lrm *LazyResourceStateManager) GetResourcesByType(resourceType string) map[string]*storage.LazyResource {
	return lrm.store.GetByType(resourceType)
}

// GetAllResources retrieves all lazy resources
func (lrm *LazyResourceStateManager) GetAllResources() []*storage.LazyResource {
	return lrm.store.GetAll()
}

// GetResourceCount returns the total number of lazy resources
func (lrm *LazyResourceStateManager) GetResourceCount() int {
	return lrm.store.Count()
}

// RefreshSnapshot manually triggers a snapshot refresh with the current state
func (lrm *LazyResourceStateManager) RefreshSnapshot() error {
	lrm.logger.Info("Manually refreshing lazy resource snapshot")

	if err := lrm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		lrm.logger.Error("Failed to refresh lazy resource snapshot", slog.Any("error", err))
		return fmt.Errorf("failed to refresh snapshot: %w", err)
	}

	lrm.logger.Info("Successfully refreshed lazy resource snapshot")
	return nil
}