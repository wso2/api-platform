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

package eventlistener

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policybuilder "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// processAPIEvent dispatches API events by action
func (l *EventListener) processAPIEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleAPICreateOrUpdate(event)
	case "DELETE":
		l.handleAPIDelete(event)
	default:
		l.logger.Warn("Unknown API event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) updateSnapshotAsync(entityID, correlationID, failureMessage string) {
	if l.snapshotManager == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := l.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			l.logger.Error(failureMessage,
				slog.String("api_id", entityID),
				slog.Any("error", err))
		}
	}()
}

// handleAPICreateOrUpdate handles API create or update events from other replicas
func (l *EventListener) handleAPICreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing API create/update event from another replica",
		slog.String("api_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	// Fetch the latest config from the database (it was already persisted by the publishing replica)
	if l.db == nil {
		l.logger.Warn("Database not available, cannot process API event",
			slog.String("api_id", entityID))
		return
	}

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch API configuration from database",
			slog.String("api_id", entityID),
			slog.Any("error", err))
		return
	}

	// Update in-memory store
	existing, _ := l.store.Get(entityID)
	if existing != nil {
		// Update existing config
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update API configuration in memory store",
				slog.String("api_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		// Add new config
		if err := l.store.Add(storedConfig); err != nil {
			l.logger.Error("Failed to add API configuration to memory store",
				slog.String("api_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	// Update xDS snapshot
	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after replica sync")

	// TODO: (VirajSalaka) Introduce an error group and have a proper rollback mechanism.

	// Update policies
	l.updatePoliciesForAPI(storedConfig, event.EventID)

	l.logger.Info("Successfully processed API create/update event from replica",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))
}

// handleAPIDelete handles API delete events from other replicas
func (l *EventListener) handleAPIDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing API delete event from another replica",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))

	existingConfig, err := l.store.Get(entityID)
	if err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to load API from memory store before deletion",
			slog.String("api_id", entityID),
			slog.Any("error", err))
		return
	}

	// Remove from in-memory store
	if err := l.store.Delete(entityID); err != nil {
		if !storage.IsNotFoundError(err) {
			l.logger.Error("Failed to delete API from memory store",
				slog.String("api_id", entityID),
				slog.Any("error", err))
		}
	}

	if err := l.store.RemoveAPIKeysByAPI(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Warn("Failed to remove API keys from memory store after API deletion",
			slog.String("api_id", entityID),
			slog.Any("error", err))
	}

	if existingConfig != nil && existingConfig.Configuration.Kind == api.RestApi && l.apiKeyXDSManager != nil {
		apiConfig, err := existingConfig.Configuration.Spec.AsAPIConfigData()
		if err != nil {
			l.logger.Warn("Failed to parse API configuration for API key xDS cleanup",
				slog.String("api_id", entityID),
				slog.Any("error", err))
		} else if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiConfig.DisplayName, apiConfig.Version, event.EventID); err != nil {
			l.logger.Warn("Failed to remove API keys from policy engine after API deletion",
				slog.String("api_id", entityID),
				slog.String("api_name", apiConfig.DisplayName),
				slog.String("api_version", apiConfig.Version),
				slog.String("event_id", event.EventID),
				slog.Any("error", err))
		}
	}

	// Update xDS snapshot
	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after API deletion")

	// Remove policies
	if l.policyManager != nil {
		policyID := entityID + "-policies"
		if err := l.policyManager.RemovePolicy(policyID); err != nil {
			if !storage.IsPolicyNotFoundError(err) {
				l.logger.Warn("Failed to remove policy after API deletion",
					slog.String("api_id", entityID),
					slog.Any("error", err))
			}
		}
	}

	l.logger.Info("Successfully processed API delete event from replica",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))
}

// updatePoliciesForAPI derives and updates policy configuration for an API
func (l *EventListener) updatePoliciesForAPI(cfg *models.StoredConfig, correlationID string) {
	if l.policyManager == nil || l.systemConfig == nil {
		return
	}

	if cfg.Configuration.Kind != api.RestApi {
		return
	}

	storedPolicy := policybuilder.DerivePolicyFromAPIConfig(cfg, l.routerConfig, l.systemConfig, l.policyDefinitions)
	if storedPolicy != nil {
		if err := l.policyManager.AddPolicy(storedPolicy); err != nil {
			l.logger.Error("Failed to update policy from replica sync",
				slog.String("api_id", cfg.ID),
				slog.Any("error", err))
		}
	}
}
