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

// processModelEvent applies a Model change on this replica.
//
// A Model has no runtime config of its own: each endpoint it names compiles it
// into the endpoint's config. So applying one means updating the store the compiler
// reads, then recompiling every endpoint the Model is on or was on, on both xDS
// paths — the Envoy snapshot, which recompiles every config anyway, and the policy
// engine's, which caches one config per artifact and so has to be told an
// endpoint's has changed. The API key feed is refreshed too: a Model's keys are
// sent once per endpoint serving it.
func (l *EventListener) processModelEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleModelCreateOrUpdate(event)
	case "DELETE":
		l.handleModelDelete(event)
	default:
		l.logger.Warn("Unknown Model event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) handleModelCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch Model from database",
			slog.String("model_id", entityID), slog.Any("error", err))
		return
	}
	if storedConfig.Kind != models.KindModel {
		l.logger.Warn("Skipping non-Model config for Model event",
			slog.String("model_id", entityID), slog.String("kind", storedConfig.Kind))
		return
	}

	// Rendered once here, like every other kind, so the compiler reads resolved
	// target credentials and has the secrets they produced for redaction.
	if err := templateengine.RenderSpec(storedConfig, l.secretResolver, l.logger); err != nil {
		l.logger.Error("Failed to render Model templates",
			slog.String("model_id", entityID), slog.String("event_id", event.EventID), slog.Any("error", err))
		return
	}

	// Rendering leaves template output as strings. Coerce policy and routing params
	// back to their schema-declared types, as every other kind does.
	if l.policyValidator != nil {
		if model, ok := storedConfig.Configuration.(api.ModelConfiguration); ok {
			l.policyValidator.CoerceModelPolicies(&model)
			storedConfig.Configuration = model
		}
	}

	existing, _ := l.store.Get(entityID)
	affected := modelEndpointsOf(existing)
	if existing != nil {
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update Model in memory store",
				slog.String("model_id", entityID), slog.Any("error", err))
			return
		}
	} else if err := l.store.Add(storedConfig); err != nil {
		l.logger.Error("Failed to add Model to memory store",
			slog.String("model_id", entityID), slog.Any("error", err))
		return
	}
	affected = append(affected, modelEndpointsOf(storedConfig)...)

	l.updateSnapshot(entityID, event.EventID, "Failed to update xDS snapshot after Model replica sync")
	l.refreshModelEndpoints(affected, event.EventID)
	l.refreshAPIKeyFeed(event.EventID)

	l.logger.Info("Successfully processed Model create/update event",
		slog.String("model_id", entityID), slog.String("event_id", event.EventID))
}

func (l *EventListener) handleModelDelete(event eventhub.Event) {
	entityID := event.EntityID

	existing, _ := l.store.Get(entityID)
	affected := modelEndpointsOf(existing)

	if err := l.store.Delete(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to delete Model from memory store",
			slog.String("model_id", entityID), slog.Any("error", err))
		return
	}

	// The Model's keys were deleted with it; drop them from the feed too, so they
	// stop authenticating on every endpoint at once.
	if err := l.store.RemoveAPIKeysByAPI(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Warn("Failed to remove Model API keys from memory store after deletion",
			slog.String("model_id", entityID), slog.Any("error", err))
	}
	if existing != nil && l.apiKeyXDSManager != nil {
		apiName, apiVersion := extractAPINameVersion(existing)
		if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiName, apiVersion, event.EventID); err != nil {
			l.logger.Warn("Failed to remove Model API keys from policy engine after deletion",
				slog.String("model_id", entityID), slog.Any("error", err))
		}
	}

	l.updateSnapshot(entityID, event.EventID, "Failed to update xDS snapshot after Model deletion")
	l.refreshModelEndpoints(affected, event.EventID)

	l.logger.Info("Successfully processed Model delete event",
		slog.String("model_id", entityID), slog.String("event_id", event.EventID))
}

// modelEndpointsOf returns the endpoint handles a stored Model names.
func modelEndpointsOf(cfg *models.StoredConfig) []string {
	if cfg == nil {
		return nil
	}
	switch m := cfg.SourceConfiguration.(type) {
	case api.ModelConfiguration:
		return models.ModelEndpointHandles(m.Spec)
	case *api.ModelConfiguration:
		if m != nil {
			return models.ModelEndpointHandles(m.Spec)
		}
	}
	return nil
}

// refreshModelEndpoints recompiles the policy-engine config of each named
// endpoint that exists and is deployed. Each handle is recompiled once, however
// often it is named.
//
// Called after every change an endpoint is compiled from — a Model, a provider a
// Model may target, a template — because the policy side rebuilds a config only
// when its own artifact is upserted, and none of those is the endpoint's artifact.
// Endpoints the change never touched are left alone.
func (l *EventListener) refreshModelEndpoints(handles []string, correlationID string) {
	if l.policyManager == nil || l.store == nil {
		return
	}
	seen := make(map[string]bool, len(handles))
	for _, handle := range handles {
		if handle == "" || seen[handle] {
			continue
		}
		seen[handle] = true
		endpoint, err := l.store.GetByKindAndHandle(models.KindModelEndpoint, handle)
		if err != nil || endpoint == nil || endpoint.DesiredState != models.StateDeployed {
			continue
		}
		if err := l.policyManager.UpsertAPIConfig(endpoint); err != nil {
			l.logger.Error("Failed to recompile model endpoint",
				slog.String("endpoint", handle),
				slog.String("correlation_id", correlationID), slog.Any("error", err))
		}
	}
}

// refreshEndpointsForProvider recompiles the endpoints serving a Model with a
// target on the provider.
func (l *EventListener) refreshEndpointsForProvider(providerHandle, correlationID string) {
	if l.store == nil {
		return
	}
	var handles []string
	for _, cfg := range l.store.GetAllByKind(models.KindModel) {
		m, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if !ok {
			continue
		}
		for _, id := range models.ModelTargetProviderIDs(m.Spec) {
			if id == providerHandle {
				handles = append(handles, models.ModelEndpointHandles(m.Spec)...)
				break
			}
		}
	}
	l.refreshModelEndpoints(handles, correlationID)
}

// refreshAllModelEndpoints recompiles every endpoint. A template change can move
// where an endpoint reads the model (its inbound template) or which transformer a
// provider gets (a provider's template), so it reaches every endpoint.
func (l *EventListener) refreshAllModelEndpoints(correlationID string) {
	if l.store == nil {
		return
	}
	var handles []string
	for _, cfg := range l.store.GetAllByKind(models.KindModelEndpoint) {
		handles = append(handles, cfg.Handle)
	}
	l.refreshModelEndpoints(handles, correlationID)
}

// refreshAPIKeyFeed resends the API key state. A Model's keys are sent once per
// endpoint serving it, so a change to which endpoints exist, or which a Model
// names, changes the feed without any key changing.
func (l *EventListener) refreshAPIKeyFeed(correlationID string) {
	refresher, ok := l.apiKeyXDSManager.(interface{ RefreshSnapshot() error })
	if !ok || refresher == nil {
		return
	}
	if err := refresher.RefreshSnapshot(); err != nil {
		l.logger.Warn("Failed to refresh the API key feed",
			slog.String("correlation_id", correlationID), slog.Any("error", err))
	}
}
