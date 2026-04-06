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
	"sort"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// processSubscriptionEvent refreshes replica-local subscription xDS state after subscription changes.
func (l *EventListener) processSubscriptionEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
		l.refreshSubscriptionState("subscription", event)
	default:
		l.logger.Warn("Unknown subscription event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

// processSubscriptionPlanEvent refreshes replica-local subscription xDS state after plan changes.
func (l *EventListener) processSubscriptionPlanEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
		l.refreshSubscriptionState("subscription_plan", event)
	default:
		l.logger.Warn("Unknown subscription plan event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

// processApplicationEvent synchronizes replica-local API key/application state from canonical DB state.
func (l *EventListener) processApplicationEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
		currentMappedKeys, err := l.loadApplicationAPIKeysFromDB(event.EntityID)
		if err != nil {
			l.logger.Error("Failed to load application API keys from database for replica sync",
				slog.String("action", event.Action),
				slog.String("application_uuid", event.EntityID),
				slog.String("event_id", event.EventID),
				slog.Any("error", err))
			return
		}

		affectedKeys, err := l.resolveAffectedApplicationAPIKeys(event.EntityID, currentMappedKeys)
		if err != nil {
			l.logger.Error("Failed to resolve affected API keys for application replica sync",
				slog.String("action", event.Action),
				slog.String("application_uuid", event.EntityID),
				slog.String("event_id", event.EventID),
				slog.Any("error", err))
			return
		}

		for _, apiKey := range affectedKeys {
			if err := l.store.StoreAPIKey(apiKey); err != nil {
				l.logger.Error("Failed to store API key in memory during application replica sync",
					slog.String("action", event.Action),
					slog.String("application_uuid", event.EntityID),
					slog.String("api_key_id", apiKey.UUID),
					slog.String("api_id", apiKey.ArtifactUUID),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
				continue
			}

			if l.apiKeyXDSManager == nil {
				continue
			}

			cfg, err := l.syncAPIConfigForAPIKeyEvent(apiKey.ArtifactUUID)
			if err != nil {
				l.logger.Warn("Skipping API key xDS refresh during application replica sync due to missing API config",
					slog.String("action", event.Action),
					slog.String("application_uuid", event.EntityID),
					slog.String("api_key_id", apiKey.UUID),
					slog.String("artifact_uuid", apiKey.ArtifactUUID),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
				continue
			}

			apiName, apiVersion := extractAPINameVersion(cfg)
			if err := l.apiKeyXDSManager.StoreAPIKey(cfg.UUID, apiName, apiVersion, apiKey, event.EventID); err != nil {
				l.logger.Error("Failed to refresh API key xDS state during application replica sync",
					slog.String("action", event.Action),
					slog.String("application_uuid", event.EntityID),
					slog.String("api_key_id", apiKey.UUID),
					slog.String("artifact_uuid", apiKey.ArtifactUUID),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
			}
		}

		l.logger.Info("Successfully processed application replica sync event",
			slog.String("action", event.Action),
			slog.String("application_uuid", event.EntityID),
			slog.String("event_id", event.EventID),
			slog.Int("affected_api_key_count", len(affectedKeys)))
	default:
		l.logger.Warn("Unknown application event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) loadApplicationAPIKeysFromDB(applicationUUID string) (map[string]*models.APIKey, error) {
	apiKeys, err := l.db.GetAllAPIKeys()
	if err != nil {
		return nil, err
	}

	currentMappedKeys := make(map[string]*models.APIKey)
	for _, apiKey := range apiKeys {
		if apiKey == nil || apiKey.UUID == "" {
			continue
		}
		if apiKey.ApplicationID != applicationUUID {
			continue
		}
		currentMappedKeys[apiKey.UUID] = apiKey
	}

	return currentMappedKeys, nil
}

func (l *EventListener) resolveAffectedApplicationAPIKeys(applicationUUID string, currentMappedKeys map[string]*models.APIKey) ([]*models.APIKey, error) {
	affectedKeyIDs := make(map[string]struct{}, len(currentMappedKeys))
	for apiKeyID := range currentMappedKeys {
		affectedKeyIDs[apiKeyID] = struct{}{}
	}

	for _, cfg := range l.store.GetAll() {
		apiKeys, err := l.store.GetAPIKeysByAPI(cfg.UUID)
		if err != nil {
			return nil, err
		}
		for _, apiKey := range apiKeys {
			if apiKey == nil || apiKey.UUID == "" {
				continue
			}
			if apiKey.ApplicationID == applicationUUID {
				affectedKeyIDs[apiKey.UUID] = struct{}{}
			}
		}
	}

	sortedKeyIDs := make([]string, 0, len(affectedKeyIDs))
	for apiKeyID := range affectedKeyIDs {
		sortedKeyIDs = append(sortedKeyIDs, apiKeyID)
	}
	sort.Strings(sortedKeyIDs)

	affectedKeys := make([]*models.APIKey, 0, len(sortedKeyIDs))
	for _, apiKeyID := range sortedKeyIDs {
		if apiKey, ok := currentMappedKeys[apiKeyID]; ok {
			affectedKeys = append(affectedKeys, apiKey)
			continue
		}

		apiKey, err := l.db.GetAPIKeyByID(apiKeyID)
		if err != nil {
			l.logger.Error("Failed to reload API key from database during application replica sync",
				slog.String("application_uuid", applicationUUID),
				slog.String("api_key_id", apiKeyID),
				slog.Any("error", err))
			continue
		}
		affectedKeys = append(affectedKeys, apiKey)
	}

	return affectedKeys, nil
}

func (l *EventListener) refreshSubscriptionState(resource string, event eventhub.Event) {
	if l.subscriptionManager == nil {
		l.logger.Warn("Subscription snapshot manager not available for replica sync",
			slog.String("resource", resource),
			slog.String("entity_id", event.EntityID),
			slog.String("event_id", event.EventID))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := l.subscriptionManager.UpdateSnapshot(ctx); err != nil {
		l.logger.Error("Failed to refresh subscription snapshot from replica sync event",
			slog.String("resource", resource),
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	l.logger.Info("Successfully refreshed subscription snapshot from replica sync event",
		slog.String("resource", resource),
		slog.String("action", event.Action),
		slog.String("entity_id", event.EntityID),
		slog.String("event_id", event.EventID))
}
