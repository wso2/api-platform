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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
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
				slog.String("entity_id", entityID),
				slog.Any("error", err))
		}
	}()
}

// handleAPICreateOrUpdate handles API create or update events
func (l *EventListener) handleAPICreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing API create/update event",
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

	// Update policy xDS via transformer
	if l.policyManager != nil {
		if transformer, ok := transform.Get(storedConfig.Kind); ok {
			rdc, err := transformer.Transform(storedConfig)
			if err != nil {
				l.logger.Error("Failed to transform API for policy xDS",
					slog.String("api_id", entityID),
					slog.String("kind", storedConfig.Kind),
					slog.String("correlation_id", event.EventID),
					slog.Any("error", err))
				return
			}
			key := storage.Key(storedConfig.Kind, storedConfig.Handle)
			if err := l.policyManager.AddRuntimeConfig(key, rdc); err != nil {
				l.logger.Error("Failed to update policy xDS after replica sync",
					slog.String("api_id", entityID),
					slog.String("correlation_id", event.EventID),
					slog.Any("error", err))
				return
			}
		}
	}

	l.logger.Info("Successfully processed API create/update event",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))
}

// handleAPIDelete handles API delete events
func (l *EventListener) handleAPIDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing API delete event",
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

	if existingConfig != nil && l.apiKeyXDSManager != nil {
		apiName, apiVersion := extractAPINameVersion(existingConfig)
		if apiName != "" {
			if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiName, apiVersion, event.EventID); err != nil {
				l.logger.Warn("Failed to remove API keys from policy engine after API deletion",
					slog.String("api_id", entityID),
					slog.String("api_name", apiName),
					slog.String("api_version", apiVersion),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
			}
		}
	}

	// Update xDS snapshot
	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after API deletion")

	// Remove policy route config
	if l.policyManager != nil && existingConfig != nil {
		key := storage.Key(existingConfig.Kind, existingConfig.Handle)
		if err := l.policyManager.RemoveRuntimeConfig(key); err != nil {
			if !storage.IsNotFoundError(err) {
				l.logger.Warn("Failed to remove policy route config after API deletion",
					slog.String("api_id", entityID),
					slog.Any("error", err))
			}
		}
	}

	l.logger.Info("Successfully processed API delete event",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))
}

// extractAPINameVersion extracts the display name and version from a StoredConfig.
// Works for RestApi and WebSubApi kinds by checking the Configuration type.
func extractAPINameVersion(cfg *models.StoredConfig) (string, string) {
	if cfg == nil {
		return "", ""
	}
	// Use denormalized fields on StoredConfig
	return cfg.DisplayName, cfg.Version
}
