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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func (l *EventListener) processMCPProxyEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleMCPProxyCreateOrUpdate(event)
	case "DELETE":
		l.handleMCPProxyDelete(event)
	default:
		l.logger.Warn("Unknown MCP proxy event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) handleMCPProxyCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing MCP proxy create/update event",
		slog.String("proxy_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch MCP proxy configuration from database",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}
	if storedConfig.Kind != string(api.MCPProxyConfigurationKindMcp) {
		l.logger.Warn("Skipping non-MCP config for MCP proxy event",
			slog.String("proxy_id", entityID),
			slog.String("kind", storedConfig.Kind))
		return
	}
	if err := utils.HydrateStoredMCPConfig(storedConfig); err != nil {
		l.logger.Error("Failed to hydrate MCP proxy configuration from source",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}

	existing, _ := l.store.Get(entityID)
	if existing != nil {
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update MCP proxy in memory store",
				slog.String("proxy_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		if err := l.store.Add(storedConfig); err != nil {
			l.logger.Error("Failed to add MCP proxy to memory store",
				slog.String("proxy_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after MCP proxy replica sync")
	l.updatePoliciesForAPI(storedConfig, event.EventID)

	l.logger.Info("Successfully processed MCP proxy create/update event",
		slog.String("proxy_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) handleMCPProxyDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing MCP proxy delete event",
		slog.String("proxy_id", entityID),
		slog.String("event_id", event.EventID))

	existingConfig, _ := l.store.Get(entityID)

	if err := l.store.Delete(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to delete MCP proxy from memory store",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}

	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after MCP proxy deletion")

	if l.policyManager != nil && existingConfig != nil {
		if err := l.policyManager.DeleteAPIConfig(existingConfig.Kind, existingConfig.Handle); err != nil {
			l.logger.Warn("Failed to remove policy after MCP proxy deletion",
				slog.String("proxy_id", entityID),
				slog.Any("error", err))
		}
	}

	l.logger.Info("Successfully processed MCP proxy delete event",
		slog.String("proxy_id", entityID),
		slog.String("event_id", event.EventID))
}
