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
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

// PolicyManager manages policy configurations and snapshot updates
type PolicyManager struct {
	store           *storage.PolicyStore
	snapshotManager *SnapshotManager
	logger          *slog.Logger
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager(store *storage.PolicyStore, snapshotManager *SnapshotManager, logger *slog.Logger) *PolicyManager {
	return &PolicyManager{
		store:           store,
		snapshotManager: snapshotManager,
		logger:          logger,
	}
}

// AddPolicy adds or updates a policy configuration
func (pm *PolicyManager) AddPolicy(policy *models.StoredPolicyConfig) error {
	// Store the policy
	if err := pm.store.Set(policy); err != nil {
		return fmt.Errorf("failed to store policy: %w", err)
	}

	pm.logger.Info("Policy configuration added",
		slog.String("id", policy.ID),
		slog.String("api_name", policy.APIName()),
		slog.String("version", policy.APIVersion()),
		slog.String("context", policy.Context()))

	// Update xDS snapshot
	if err := pm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		pm.logger.Error("Failed to update policy snapshot after adding policy",
			slog.String("id", policy.ID),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// RemovePolicy removes a policy configuration
func (pm *PolicyManager) RemovePolicy(id string) error {
	if err := pm.store.Delete(id); err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	pm.logger.Info("Policy configuration removed", slog.String("id", id))

	// Update xDS snapshot
	if err := pm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		pm.logger.Error("Failed to update policy snapshot after removing policy",
			slog.String("id", id),
			slog.Any("error", err))
		return fmt.Errorf("failed to update snapshot: %w", err)
	}

	return nil
}

// GetPolicy retrieves a policy by ID
func (pm *PolicyManager) GetPolicy(id string) (*models.StoredPolicyConfig, error) {
	policy, exists := pm.store.Get(id)
	if !exists {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

// ListPolicies returns all policies
func (pm *PolicyManager) ListPolicies() []*models.StoredPolicyConfig {
	return pm.store.GetAll()
}

// ParsePolicyJSON parses a policy configuration from JSON string
func ParsePolicyJSON(jsonStr string) (*policyenginev1.Configuration, error) {
	var config policyenginev1.Configuration
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse policy JSON: %w", err)
	}
	return &config, nil
}

// CreateStoredPolicy creates a StoredPolicyConfig from a PolicyConfiguration
func CreateStoredPolicy(id string, config policyenginev1.Configuration) *models.StoredPolicyConfig {
	return &models.StoredPolicyConfig{
		ID:            id,
		Configuration: config,
		Version:       config.Metadata.ResourceVersion,
	}
}
