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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
)

// processAgentEvent dispatches Agent (A2A) events by action.
func (l *EventListener) processAgentEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleAgentCreateOrUpdate(event)
	case "DELETE":
		l.handleAgentDelete(event)
	default:
		l.logger.Warn("Unknown Agent event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

// handleAgentCreateOrUpdate converges this replica on an Agent that was written
// by any replica — including this one, since the writer publishes to itself too
// and this is the only path that populates the in-memory and runtime stores.
//
// The artifact is re-read and re-rendered here rather than carried in the event:
// the database holds the source document as the author wrote it, so every
// replica resolves `{{ secret … }}`/`{{ env … }}` against its own environment
// and a rotated secret is picked up without rewriting the artifact.
func (l *EventListener) handleAgentCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing Agent create/update event",
		slog.String("agent_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch Agent configuration from database",
			slog.String("agent_id", entityID),
			slog.Any("error", err))
		return
	}
	// An entity id is unique across kinds, so a kind mismatch here means the
	// event and the row disagree. Converging anyway would push a non-Agent
	// artifact through the Agent lane.
	if storedConfig.Kind != models.KindAgent {
		l.logger.Warn("Skipping non-Agent config for Agent event",
			slog.String("agent_id", entityID),
			slog.String("kind", storedConfig.Kind))
		return
	}

	// No hydrate step: an Agent is stored in its deployable shape, so the read
	// above already populates both Configuration and SourceConfiguration (see
	// unmarshalSourceConfig). Only rendering stands between the row and the
	// stores.
	if err := templateengine.RenderSpec(storedConfig, l.secretResolver, l.logger); err != nil {
		l.logger.Error("Failed to render config templates for Agent",
			slog.String("agent_id", entityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	// RenderSpec JSON-roundtrips the config, which resets every policy param to
	// a string. Coerce them back to their schema-declared types so the runtime
	// receives the same typed values the deploying replica validated.
	if l.policyValidator != nil {
		if agentConfig, ok := storedConfig.Configuration.(api.AgentConfiguration); ok {
			l.policyValidator.CoerceAgentPolicies(&agentConfig)
			storedConfig.Configuration = agentConfig
		}
	}

	existing, _ := l.store.Get(entityID)
	if existing != nil {
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update Agent in memory store",
				slog.String("agent_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		if err := l.store.Add(storedConfig); err != nil {
			l.logger.Error("Failed to add Agent to memory store",
				slog.String("agent_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	l.updateSnapshot(entityID, event.EventID, "Failed to update xDS snapshot after Agent replica sync")
	l.updatePoliciesForAPI(storedConfig, event.EventID)

	l.logger.Info("Successfully processed Agent create/update event",
		slog.String("agent_id", entityID),
		slog.String("event_id", event.EventID))
}

// handleAgentDelete drops an Agent's local state: the configuration the Envoy
// snapshot is built from, and the runtime deploy config the policy engine
// resolves chains against. Both have to go — an Agent whose routes are gone but
// whose chains remain leaves every chain in the policy snapshot unreachable and
// unremovable.
func (l *EventListener) handleAgentDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing Agent delete event",
		slog.String("agent_id", entityID),
		slog.String("event_id", event.EventID))

	// Read before deleting: the runtime config is keyed by kind and handle, and
	// after the store drops the config there is nothing left to derive them from.
	existingConfig, _ := l.store.Get(entityID)

	if err := l.store.Delete(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to delete Agent from memory store",
			slog.String("agent_id", entityID),
			slog.Any("error", err))
		return
	}

	l.updateSnapshot(entityID, event.EventID, "Failed to update xDS snapshot after Agent deletion")

	if l.policyManager != nil && existingConfig != nil {
		if err := l.policyManager.DeleteAPIConfig(existingConfig.Kind, existingConfig.Handle); err != nil {
			l.logger.Warn("Failed to remove runtime config after Agent deletion",
				slog.String("agent_id", entityID),
				slog.Any("error", err))
		}
	}

	l.logger.Info("Successfully processed Agent delete event",
		slog.String("agent_id", entityID),
		slog.String("event_id", event.EventID))
}
