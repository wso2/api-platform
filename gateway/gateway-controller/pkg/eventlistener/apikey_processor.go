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
	"log/slog"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// processAPIKeyEvent dispatches API key events by action.
func (l *EventListener) processAPIKeyEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "REGENERATE":
		l.handleAPIKeyUpsert(event)
	case "REVOKE":
		l.handleAPIKeyRevoke(event)
	default:
		l.logger.Warn("Unknown API key event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) syncAPIConfigForAPIKeyEvent(apiID string) (*models.StoredConfig, error) {
	cfg, err := l.store.Get(apiID)
	if err == nil {
		return cfg, nil
	}

	if l.db == nil {
		return nil, err
	}

	cfg, err = l.db.GetConfig(apiID)
	if err != nil {
		return nil, err
	}

	if addErr := l.store.Add(cfg); addErr != nil {
		if updateErr := l.store.Update(cfg); updateErr != nil {
			l.logger.Warn("Failed to sync API config into memory store while processing API key event",
				slog.String("api_id", apiID),
				slog.Any("add_error", addErr),
				slog.Any("update_error", updateErr))
		}
	}

	return cfg, nil
}

// handleAPIKeyUpsert handles API key create/update/regenerate events from write-path async sync.
func (l *EventListener) handleAPIKeyUpsert(event eventhub.Event) {
	apiID, keyID, err := eventhub.ParseAPIKeyEntityID(event.EntityID)
	if err != nil {
		l.logger.Error("Failed to parse API key event entity ID",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	l.logger.Info("Processing API key upsert event from another replica",
		slog.String("action", event.Action),
		slog.String("api_id", apiID),
		slog.String("api_key_id", keyID),
		slog.String("event_id", event.EventID))

	if l.db == nil {
		l.logger.Warn("Database not available, cannot process API key event",
			slog.String("api_key_id", keyID))
		return
	}
	if l.store == nil {
		l.logger.Warn("In-memory store not available, cannot process API key event",
			slog.String("api_key_id", keyID))
		return
	}

	apiKey, err := l.db.GetAPIKeyByID(keyID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			l.logger.Warn("API key not found in database for upsert event",
				slog.String("action", event.Action),
				slog.String("api_id", apiID),
				slog.String("api_key_id", keyID),
				slog.String("event_id", event.EventID))
			return
		}

		l.logger.Error("Failed to fetch API key from database",
			slog.String("api_id", apiID),
			slog.String("api_key_id", keyID),
			slog.Any("error", err))
		return
	}

	if err := l.store.StoreAPIKey(apiKey); err != nil {
		l.logger.Error("Failed to store API key in memory store",
			slog.String("api_key_id", keyID),
			slog.String("api_id", apiKey.APIId),
			slog.Any("error", err))
		return
	}

	cfg, err := l.syncAPIConfigForAPIKeyEvent(apiKey.APIId)
	if err != nil {
		l.logger.Error("Failed to resolve API for API key event",
			slog.String("action", event.Action),
			slog.String("api_key_id", keyID),
			slog.String("api_id", apiKey.APIId),
			slog.Any("error", err))
		return
	}

	apiConfig, err := cfg.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		l.logger.Error("Failed to parse API configuration for API key xDS update",
			slog.String("api_id", cfg.ID),
			slog.String("api_key_id", keyID),
			slog.Any("error", err))
		return
	}

	if l.apiKeyXDSManager != nil {
		if err := l.apiKeyXDSManager.StoreAPIKey(cfg.ID, apiConfig.DisplayName, apiConfig.Version, apiKey, event.EventID); err != nil {
			l.logger.Error("Failed to update API key in policy engine after replica sync",
				slog.String("api_id", cfg.ID),
				slog.String("api_key_id", keyID),
				slog.Any("error", err))
			return
		}
	}

	l.logger.Info("Successfully processed API key upsert event from replica",
		slog.String("action", event.Action),
		slog.String("api_id", cfg.ID),
		slog.String("api_key_id", keyID),
		slog.String("event_id", event.EventID))
}

// handleAPIKeyRevoke handles API key revoke events from write-path async sync.
func (l *EventListener) handleAPIKeyRevoke(event eventhub.Event) {
	apiID, keyID, err := eventhub.ParseAPIKeyEntityID(event.EntityID)
	if err != nil {
		l.logger.Error("Failed to parse API key revoke event entity ID",
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	l.logger.Info("Processing API key revoke event from another replica",
		slog.String("api_id", apiID),
		slog.String("api_key_id", keyID),
		slog.String("event_id", event.EventID))

	if l.store == nil {
		l.logger.Warn("In-memory store not available, cannot process API key revoke event",
			slog.String("api_key_id", keyID))
		return
	}

	var apiKeyName string
	apiKey, err := l.store.GetAPIKeyByID(apiID, keyID)
	if err == nil {
		apiKeyName = apiKey.Name
	} else if !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to load API key from memory store during revoke sync",
			slog.String("api_key_id", keyID),
			slog.String("api_id", apiID),
			slog.Any("error", err))
		return
	}

	if err := l.store.RemoveAPIKeyByID(apiID, keyID); err != nil {
		if storage.IsNotFoundError(err) {
			l.logger.Debug("API key already absent from memory store during revoke sync",
				slog.String("api_key_id", keyID),
				slog.String("api_id", apiID))
		} else {
			l.logger.Error("Failed to remove API key from memory store during revoke sync",
				slog.String("api_key_id", keyID),
				slog.String("api_id", apiID),
				slog.Any("error", err))
			return
		}
	}

	if apiKeyName == "" {
		l.logger.Warn("Skipping API key revoke xDS sync because API key name is unavailable",
			slog.String("api_key_id", keyID),
			slog.String("api_id", apiID))
		return
	}

	cfg, err := l.syncAPIConfigForAPIKeyEvent(apiID)
	if err != nil {
		l.logger.Error("Failed to resolve API for API key revoke event",
			slog.String("api_key_id", keyID),
			slog.String("api_id", apiID),
			slog.Any("error", err))
		return
	}

	apiConfig, err := cfg.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		l.logger.Error("Failed to parse API configuration for API key revoke xDS update",
			slog.String("api_id", cfg.ID),
			slog.String("api_key_id", keyID),
			slog.Any("error", err))
		return
	}

	if l.apiKeyXDSManager != nil {
		if err := l.apiKeyXDSManager.RevokeAPIKey(cfg.ID, apiConfig.DisplayName, apiConfig.Version, apiKeyName, event.EventID); err != nil {
			l.logger.Error("Failed to revoke API key in policy engine after replica sync",
				slog.String("api_id", cfg.ID),
				slog.String("api_key_id", keyID),
				slog.Any("error", err))
			return
		}
	}

	l.logger.Info("Successfully processed API key revoke event from replica",
		slog.String("api_id", cfg.ID),
		slog.String("api_key_id", keyID),
		slog.String("event_id", event.EventID))
}
