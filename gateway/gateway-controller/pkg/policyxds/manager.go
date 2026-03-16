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

package policyxds

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// PolicyManager manages runtime deploy configurations and snapshot updates.
// It supports both the legacy PolicyStore API (AddPolicy/RemovePolicy) and
// the new RuntimeConfigStore API (AddRuntimeConfig/RemoveRuntimeConfig).
type PolicyManager struct {
	runtimeStore    *storage.RuntimeConfigStore
	legacyStore     *storage.PolicyStore
	snapshotManager *SnapshotManager
	logger          *slog.Logger
}

// NewPolicyManager creates a new policy manager using the legacy PolicyStore.
// The legacy store is used for backward-compatible AddPolicy/RemovePolicy/GetPolicy/ListPolicies.
func NewPolicyManager(legacyStore *storage.PolicyStore, snapshotManager *SnapshotManager, logger *slog.Logger) *PolicyManager {
	return &PolicyManager{
		legacyStore:     legacyStore,
		snapshotManager: snapshotManager,
		logger:          logger,
	}
}

// SetRuntimeStore sets the RuntimeConfigStore for the new API path.
func (pm *PolicyManager) SetRuntimeStore(store *storage.RuntimeConfigStore) {
	pm.runtimeStore = store
}

// --- Legacy API (backward compatible with StoredPolicyConfig) ---

// AddPolicy adds a StoredPolicyConfig to the legacy store and triggers snapshot update.
func (pm *PolicyManager) AddPolicy(policy *models.StoredPolicyConfig) error {
	if pm.legacyStore == nil {
		return fmt.Errorf("legacy policy store not configured")
	}

	if err := pm.legacyStore.Set(policy); err != nil {
		return fmt.Errorf("failed to store policy: %w", err)
	}

	pm.logger.Info("Policy configuration added (legacy)",
		slog.String("id", policy.ID),
		slog.String("id", policy.ID))

	if err := pm.snapshotManager.UpdateSnapshotLegacy(context.Background(), pm.legacyStore); err != nil {
		pm.logger.Error("Failed to update snapshot after adding policy",
			slog.String("id", policy.ID),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// RemovePolicy removes a StoredPolicyConfig by ID and triggers snapshot update.
func (pm *PolicyManager) RemovePolicy(id string) error {
	if pm.legacyStore == nil {
		return fmt.Errorf("legacy policy store not configured")
	}

	if err := pm.legacyStore.Delete(id); err != nil {
		return err
	}

	pm.logger.Info("Policy configuration removed (legacy)", slog.String("id", id))

	if err := pm.snapshotManager.UpdateSnapshotLegacy(context.Background(), pm.legacyStore); err != nil {
		pm.logger.Error("Failed to update snapshot after removing policy",
			slog.String("id", id),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// GetPolicy retrieves a StoredPolicyConfig by ID from the legacy store.
func (pm *PolicyManager) GetPolicy(id string) (*models.StoredPolicyConfig, error) {
	if pm.legacyStore == nil {
		return nil, fmt.Errorf("legacy policy store not configured")
	}

	policy, exists := pm.legacyStore.Get(id)
	if !exists {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

// ListPolicies returns all StoredPolicyConfigs from the legacy store.
func (pm *PolicyManager) ListPolicies() []*models.StoredPolicyConfig {
	if pm.legacyStore == nil {
		return nil
	}
	return pm.legacyStore.GetAll()
}

// --- New RuntimeDeployConfig API ---

// AddRuntimeConfig adds or updates a RuntimeDeployConfig and triggers snapshot update.
func (pm *PolicyManager) AddRuntimeConfig(key string, rdc *models.RuntimeDeployConfig) error {
	if pm.runtimeStore == nil {
		return fmt.Errorf("runtime config store not configured")
	}

	pm.runtimeStore.Set(key, rdc)

	pm.logger.Info("Runtime deploy config added",
		slog.String("key", key),
		slog.String("kind", rdc.Metadata.Kind),
		slog.String("name", rdc.Metadata.DisplayName),
		slog.Int("routes", len(rdc.Routes)),
		slog.Int("policy_chains", len(rdc.PolicyChains)))

	if err := pm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		pm.logger.Error("Failed to update snapshot after adding runtime config",
			slog.String("key", key),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// RemoveRuntimeConfig removes a RuntimeDeployConfig and triggers snapshot update.
func (pm *PolicyManager) RemoveRuntimeConfig(key string) error {
	if pm.runtimeStore == nil {
		return fmt.Errorf("runtime config store not configured")
	}

	if err := pm.runtimeStore.Delete(key); err != nil {
		return fmt.Errorf("failed to delete runtime config: %w", err)
	}

	pm.logger.Info("Runtime deploy config removed", slog.String("key", key))

	if err := pm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		pm.logger.Error("Failed to update snapshot after removing runtime config",
			slog.String("key", key),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// GetRuntimeConfig retrieves a RuntimeDeployConfig by key.
func (pm *PolicyManager) GetRuntimeConfig(key string) (*models.RuntimeDeployConfig, error) {
	if pm.runtimeStore == nil {
		return nil, fmt.Errorf("runtime config store not configured")
	}

	rdc, exists := pm.runtimeStore.Get(key)
	if !exists {
		return nil, fmt.Errorf("runtime config not found: %s", key)
	}
	return rdc, nil
}

// ListRuntimeConfigs returns all RuntimeDeployConfigs.
func (pm *PolicyManager) ListRuntimeConfigs() []*models.RuntimeDeployConfig {
	if pm.runtimeStore == nil {
		return nil
	}
	return pm.runtimeStore.GetAll()
}

// GetResourceVersion returns the current resource version used for xDS updates.
func (pm *PolicyManager) GetResourceVersion() int64 {
	if pm.runtimeStore != nil {
		return pm.runtimeStore.GetResourceVersion()
	}
	if pm.legacyStore != nil {
		return pm.legacyStore.GetResourceVersion()
	}
	return 0
}
