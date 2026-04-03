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
type PolicyManager struct {
	runtimeStore    *storage.RuntimeConfigStore
	snapshotManager *SnapshotManager
	transformers    models.ConfigTransformer
	logger          *slog.Logger
}

// NewPolicyManager creates a new PolicyManager.
func NewPolicyManager(snapshotManager *SnapshotManager, logger *slog.Logger) *PolicyManager {
	return &PolicyManager{
		snapshotManager: snapshotManager,
		logger:          logger,
	}
}

// SetRuntimeStore sets the RuntimeConfigStore.
func (pm *PolicyManager) SetRuntimeStore(store *storage.RuntimeConfigStore) {
	pm.runtimeStore = store
}

// SetTransformers sets the ConfigTransformer used by UpsertAPIConfig.
func (pm *PolicyManager) SetTransformers(t models.ConfigTransformer) {
	pm.transformers = t
}

// UpsertAPIConfig transforms cfg into a RuntimeDeployConfig and stores it,
// then triggers a snapshot update (both policy chain and route config caches).
func (pm *PolicyManager) UpsertAPIConfig(cfg *models.StoredConfig) error {
	if pm.runtimeStore == nil {
		return fmt.Errorf("runtime config store not configured")
	}
	if pm.transformers == nil {
		return fmt.Errorf("transformer registry not configured")
	}

	rdc, err := pm.transformers.Transform(cfg)
	if err != nil {
		return fmt.Errorf("failed to transform config: %w", err)
	}

	key := storage.Key(cfg.Kind, cfg.Handle)
	return pm.AddRuntimeConfig(key, rdc)
}

// DeleteAPIConfig removes the RuntimeDeployConfig for the given kind/handle
// and triggers a snapshot update. A not-found error is silently ignored.
func (pm *PolicyManager) DeleteAPIConfig(kind, handle string) error {
	key := storage.Key(kind, handle)
	err := pm.RemoveRuntimeConfig(key)
	if err != nil && !storage.IsPolicyNotFoundError(err) {
		return err
	}
	return nil
}

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

// GetResourceVersion returns the current resource version used for xDS updates.
func (pm *PolicyManager) GetResourceVersion() int64 {
	if pm.runtimeStore != nil {
		return pm.runtimeStore.GetResourceVersion()
	}
	return 0
}
