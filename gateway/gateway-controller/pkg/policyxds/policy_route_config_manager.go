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

package policyxds

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// PolicyManager manages runtime deploy configurations and triggers xDS snapshot
// updates for both policy chains and route configs via a single UpdateSnapshot call.
type PolicyManager struct {
	runtimeStore    *storage.RuntimeConfigStore
	snapshotManager *SnapshotManager
	logger          *slog.Logger
}

// NewPolicyManager creates a new PolicyManager backed by a RuntimeConfigStore.
func NewPolicyManager(
	snapshotManager *SnapshotManager,
	logger *slog.Logger,
) *PolicyManager {
	store := storage.NewRuntimeConfigStore()

	// Wire the runtime store into the snapshot manager so UpdateSnapshot can read from it
	snapshotManager.SetRuntimeStore(store)

	return &PolicyManager{
		runtimeStore:    store,
		snapshotManager: snapshotManager,
		logger:          logger,
	}
}

// GetRuntimeStore returns the underlying RuntimeConfigStore for direct bulk-loading
// during startup (avoids triggering per-item snapshot updates).
func (m *PolicyManager) GetRuntimeStore() *storage.RuntimeConfigStore {
	return m.runtimeStore
}

// GetResourceVersion returns the current resource version used for xDS updates.
func (m *PolicyManager) GetResourceVersion() int64 {
	return m.runtimeStore.GetResourceVersion()
}

// AddRuntimeConfig adds or updates a RuntimeDeployConfig and triggers a full
// snapshot update (both policy chains and route configs).
func (m *PolicyManager) AddRuntimeConfig(key string, rdc *models.RuntimeDeployConfig) error {
	m.runtimeStore.Set(key, rdc)

	m.logger.Info("Runtime config added",
		slog.String("key", key),
		slog.String("kind", rdc.Metadata.Kind),
		slog.Int("routes", len(rdc.Routes)),
		slog.Int("policy_chains", len(rdc.PolicyChains)))

	if err := m.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		m.logger.Error("Failed to update snapshot",
			slog.String("key", key),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// RemoveRuntimeConfig removes a RuntimeDeployConfig by key and triggers a full
// snapshot update.
func (m *PolicyManager) RemoveRuntimeConfig(key string) error {
	if err := m.runtimeStore.Delete(key); err != nil {
		return fmt.Errorf("failed to delete runtime config: %w", err)
	}

	m.logger.Info("Runtime config removed", slog.String("key", key))

	if err := m.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		m.logger.Error("Failed to update snapshot after removal",
			slog.String("key", key),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}
