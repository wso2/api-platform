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
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// APIKeyStateManager provides high-level API key management operations
// with state-of-the-world xDS updates
type APIKeyStateManager struct {
	snapshotManager *APIKeySnapshotManager
	store           *storage.APIKeyStore
	logger          *slog.Logger
}

// NewAPIKeyStateManager creates a new API key state manager
func NewAPIKeyStateManager(store *storage.APIKeyStore, snapshotManager *APIKeySnapshotManager, logger *slog.Logger) *APIKeyStateManager {
	return &APIKeyStateManager{
		snapshotManager: snapshotManager,
		store:           store,
		logger:          logger,
	}
}

// StoreAPIKey stores an API key and updates the policy engine with the complete state
func (asm *APIKeyStateManager) StoreAPIKey(apiId, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error {
	asm.logger.Info("Storing API key with state-of-the-world update",
		slog.String("api_id", apiId),
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion),
		slog.String("api_key_name", apiKey.Name),
		slog.String("correlation_id", correlationID))

	// Store the API key in the store and update the snapshot
	if err := asm.snapshotManager.StoreAPIKey(apiKey); err != nil {
		asm.logger.Error("Failed to store API key and update snapshot",
			slog.String("api_key_id", apiKey.ID),
			slog.Any("error", err))
		return fmt.Errorf("failed to store API key: %w", err)
	}

	asm.logger.Info("Successfully stored API key and updated policy engine state",
		slog.String("api_key_id", apiKey.ID),
		slog.String("correlation_id", correlationID))

	return nil
}

// RevokeAPIKey revokes an API key and updates the policy engine with the complete state
func (asm *APIKeyStateManager) RevokeAPIKey(apiId, apiName, apiVersion, apiKeyName, correlationID string) error {
	asm.logger.Info("Revoking API key with state-of-the-world update",
		slog.String("api_id", apiId),
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion),
		slog.String("correlation_id", correlationID))

	// Revoke the API key and update the snapshot
	if err := asm.snapshotManager.RevokeAPIKey(apiId, apiKeyName); err != nil {
		asm.logger.Error("Failed to revoke API key and update snapshot",
			slog.String("api key", apiKeyName),
			slog.Any("error", err))
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	asm.logger.Info("Successfully revoked API key and updated policy engine state",
		slog.String("api_id", apiId),
		slog.String("api key", apiKeyName),
		slog.String("correlation_id", correlationID))

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for an API and updates the policy engine with the complete state
func (asm *APIKeyStateManager) RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error {
	asm.logger.Info("Removing API keys by API with state-of-the-world update",
		slog.String("api_id", apiId),
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion),
		slog.String("correlation_id", correlationID))

	// Remove API keys and update the snapshot
	if err := asm.snapshotManager.RemoveAPIKeysByAPI(apiId); err != nil {
		asm.logger.Error("Failed to remove API keys by API and update snapshot",
			slog.String("api_id", apiId),
			slog.Any("error", err))
		return fmt.Errorf("failed to remove API keys by API: %w", err)
	}

	asm.logger.Info("Successfully removed API keys by API and updated policy engine state",
		slog.String("api_id", apiId),
		slog.String("correlation_id", correlationID))

	return nil
}

// GetAPIKeyCount returns the total number of API keys
func (asm *APIKeyStateManager) GetAPIKeyCount() int {
	return asm.store.Count()
}

// RefreshSnapshot manually triggers a snapshot refresh with the current state
func (asm *APIKeyStateManager) RefreshSnapshot() error {
	asm.logger.Info("Manually refreshing API key snapshot")

	if err := asm.snapshotManager.UpdateSnapshot(context.Background()); err != nil {
		asm.logger.Error("Failed to refresh API key snapshot", slog.Any("error", err))
		return fmt.Errorf("failed to refresh snapshot: %w", err)
	}

	asm.logger.Info("Successfully refreshed API key snapshot")
	return nil
}

// MaskAPIKey masks an API key for secure logging, showing first 8 and last 4 characters
func (asm *APIKeyStateManager) MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 12 {
		return "****"
	}
	return apiKey[:8] + "****" + apiKey[len(apiKey)-4:]
}
