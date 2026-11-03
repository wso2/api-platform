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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// processModelEndpointEvent applies a model endpoint change on this replica: the
// store, both xDS paths, the endpoint's provider-template mapping, and its API
// keys.
func (l *EventListener) processModelEndpointEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleModelEndpointCreateOrUpdate(event)
	case "DELETE":
		l.handleModelEndpointDelete(event)
	default:
		l.logger.Warn("Unknown model endpoint event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) handleModelEndpointCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch model endpoint from database",
			slog.String("endpoint_id", entityID), slog.Any("error", err))
		return
	}
	if storedConfig.Kind != models.KindModelEndpoint {
		l.logger.Warn("Skipping non-ModelEndpoint config for model endpoint event",
			slog.String("endpoint_id", entityID), slog.String("kind", storedConfig.Kind))
		return
	}

	// Rendered once here, like every other kind, so the compiler reads resolved
	// policy parameters and has the secrets they produced for redaction.
	if err := templateengine.RenderSpec(storedConfig, l.secretResolver, l.logger); err != nil {
		l.logger.Error("Failed to render model endpoint templates",
			slog.String("endpoint_id", entityID), slog.String("event_id", event.EventID), slog.Any("error", err))
		return
	}

	if existing, _ := l.store.Get(entityID); existing != nil {
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update model endpoint in memory store",
				slog.String("endpoint_id", entityID), slog.Any("error", err))
			return
		}
	} else if err := l.store.Add(storedConfig); err != nil {
		l.logger.Error("Failed to add model endpoint to memory store",
			slog.String("endpoint_id", entityID), slog.Any("error", err))
		return
	}

	if err := l.syncModelEndpointTemplateMapping(storedConfig, event.EventID); err != nil {
		l.logger.Warn("Failed to publish the model endpoint's provider template mapping",
			slog.String("endpoint_id", entityID), slog.Any("error", err))
	}

	l.updateSnapshot(entityID, event.EventID, "Failed to update xDS snapshot after model endpoint replica sync")

	if l.policyManager != nil {
		if storedConfig.DesiredState == models.StateDeployed {
			if err := l.policyManager.UpsertAPIConfig(storedConfig); err != nil {
				l.logger.Error("Failed to compile model endpoint",
					slog.String("endpoint_id", entityID), slog.String("event_id", event.EventID), slog.Any("error", err))
			}
		} else if err := l.policyManager.DeleteAPIConfig(storedConfig.Kind, storedConfig.Handle); err != nil {
			l.logger.Warn("Failed to remove runtime config of an undeployed model endpoint",
				slog.String("endpoint_id", entityID), slog.Any("error", err))
		}
	}

	// The endpoint's own keys, and the Model keys sent under its id.
	l.syncAPIKeysForAPI(storedConfig, event.EventID)
	l.refreshAPIKeyFeed(event.EventID)

	l.logger.Info("Successfully processed model endpoint create/update event",
		slog.String("endpoint_id", entityID), slog.String("event_id", event.EventID))
}

func (l *EventListener) handleModelEndpointDelete(event eventhub.Event) {
	entityID := event.EntityID

	existingConfig, err := l.store.Get(entityID)
	if err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to load model endpoint from memory store before deletion",
			slog.String("endpoint_id", entityID), slog.Any("error", err))
		return
	}

	if err := l.store.Delete(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to delete model endpoint from memory store",
			slog.String("endpoint_id", entityID), slog.Any("error", err))
		return
	}
	if err := l.store.RemoveAPIKeysByAPI(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Warn("Failed to remove model endpoint API keys from memory store after deletion",
			slog.String("endpoint_id", entityID), slog.Any("error", err))
	}
	if existingConfig != nil && l.apiKeyXDSManager != nil {
		apiName, apiVersion := extractAPINameVersion(existingConfig)
		if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiName, apiVersion, event.EventID); err != nil {
			l.logger.Warn("Failed to remove model endpoint API keys from policy engine after deletion",
				slog.String("endpoint_id", entityID), slog.Any("error", err))
		}
	}
	if existingConfig != nil {
		if err := l.removeProviderTemplateMapping(models.ModelEndpointProviderName(existingConfig.Handle), event.EventID); err != nil {
			l.logger.Warn("Failed to remove the model endpoint's provider template mapping",
				slog.String("endpoint_id", entityID), slog.Any("error", err))
		}
	}

	l.updateSnapshot(entityID, event.EventID, "Failed to update xDS snapshot after model endpoint deletion")

	if l.policyManager != nil && existingConfig != nil {
		if err := l.policyManager.DeleteAPIConfig(existingConfig.Kind, existingConfig.Handle); err != nil {
			l.logger.Warn("Failed to remove runtime config after model endpoint deletion",
				slog.String("endpoint_id", entityID), slog.Any("error", err))
		}
	}
	l.refreshAPIKeyFeed(event.EventID)

	l.logger.Info("Successfully processed model endpoint delete event",
		slog.String("endpoint_id", entityID), slog.String("event_id", event.EventID))
}

// syncModelEndpointTemplateMapping publishes the endpoint's provider_name to
// inbound template mapping, which policies that key on provider_name
// (token-based-ratelimit, llm-cost-based-ratelimit) resolve the template through.
func (l *EventListener) syncModelEndpointTemplateMapping(cfg *models.StoredConfig, correlationID string) error {
	if l.lazyResourceManager == nil || cfg == nil {
		return nil
	}
	endpoint, ok := cfg.SourceConfiguration.(api.ModelEndpointConfiguration)
	if !ok {
		return nil
	}
	name := models.ModelEndpointProviderName(cfg.Handle)
	return l.lazyResourceManager.StoreResource(&storage.LazyResource{
		ID:           name,
		ResourceType: utils.LazyResourceTypeProviderTemplateMapping,
		Resource: map[string]interface{}{
			"provider_name":   name,
			"template_handle": models.EffectiveInboundTemplate(endpoint.Spec),
		},
	}, correlationID)
}
