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
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func (l *EventListener) processLLMProviderEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleLLMProviderCreateOrUpdate(event)
	case "DELETE":
		l.handleLLMProviderDelete(event)
	default:
		l.logger.Warn("Unknown LLM provider event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) processLLMProxyEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleLLMProxyCreateOrUpdate(event)
	case "DELETE":
		l.handleLLMProxyDelete(event)
	default:
		l.logger.Warn("Unknown LLM proxy event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) handleLLMProviderCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing LLM provider create/update event",
		slog.String("provider_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	// The event only tells replicas which artifact changed. Re-read the full row
	// from the database so every replica rebuilds from the same canonical payload.
	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch LLM provider configuration from database",
			slog.String("provider_id", entityID),
			slog.Any("error", err))
		return
	}
	if storedConfig.Kind != string(api.LLMProviderConfigurationKindLlmProvider) {
		l.logger.Warn("Skipping non-LLM-provider config for LLM provider event",
			slog.String("provider_id", entityID),
			slog.String("kind", storedConfig.Kind))
		return
	}
	if err := l.hydrateLLMConfig(storedConfig); err != nil {
		l.logger.Error("Failed to hydrate LLM provider configuration from source",
			slog.String("provider_id", entityID),
			slog.Any("error", err))
		return
	}

	// Render template expressions in the spec (e.g. {{ secret "..." }}, {{ env "..." }}).
	if err := templateengine.RenderSpec(storedConfig, l.secretResolver, l.logger); err != nil {
		l.logger.Error("Failed to render config templates for LLM provider",
			slog.String("provider_id", entityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	// After hydration the entry contains the derived RestAPI view used by the
	// normal in-memory routing, xDS, and policy update pipelines.
	existing, _ := l.store.Get(entityID)
	if existing != nil {
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update LLM provider in memory store",
				slog.String("provider_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		if err := l.store.Add(storedConfig); err != nil {
			l.logger.Error("Failed to add LLM provider to memory store",
				slog.String("provider_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	// Provider-template mappings are separate lazy resources consumed by the
	// policy engine, so they must be kept in sync with the provider artifact.
	if err := l.syncProviderTemplateMapping(storedConfig, event.EventID); err != nil {
		l.logger.Warn("Failed to sync provider-template mapping",
			slog.String("provider_id", entityID),
			slog.Any("error", err))
	}

	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after LLM provider replica sync")
	l.updatePoliciesForAPI(storedConfig, event.EventID)

	l.logger.Info("Successfully processed LLM provider create/update event",
		slog.String("provider_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) handleLLMProxyCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing LLM proxy create/update event",
		slog.String("proxy_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch LLM proxy configuration from database",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}
	if storedConfig.Kind != string(api.LLMProxyConfigurationKindLlmProxy) {
		l.logger.Warn("Skipping non-LLM-proxy config for LLM proxy event",
			slog.String("proxy_id", entityID),
			slog.String("kind", storedConfig.Kind))
		return
	}
	if err := l.hydrateLLMConfig(storedConfig); err != nil {
		l.logger.Error("Failed to hydrate LLM proxy configuration from source",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}

	// Render template expressions in the spec (e.g. {{ secret "..." }}, {{ env "..." }}).
	if err := templateengine.RenderSpec(storedConfig, l.secretResolver, l.logger); err != nil {
		l.logger.Error("Failed to render config templates for LLM proxy",
			slog.String("proxy_id", entityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	existing, _ := l.store.Get(entityID)
	if existing != nil {
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update LLM proxy in memory store",
				slog.String("proxy_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		if err := l.store.Add(storedConfig); err != nil {
			l.logger.Error("Failed to add LLM proxy to memory store",
				slog.String("proxy_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after LLM proxy replica sync")
	l.updatePoliciesForAPI(storedConfig, event.EventID)

	l.logger.Info("Successfully processed LLM proxy create/update event",
		slog.String("proxy_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) handleLLMProviderDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing LLM provider delete event",
		slog.String("provider_id", entityID),
		slog.String("event_id", event.EventID))

	existingConfig, err := l.store.Get(entityID)
	if err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to load LLM provider from memory store before deletion",
			slog.String("provider_id", entityID),
			slog.Any("error", err))
		return
	}

	if err := l.store.Delete(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to delete LLM provider from memory store",
			slog.String("provider_id", entityID),
			slog.Any("error", err))
		return
	}

	if err := l.store.RemoveAPIKeysByAPI(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Warn("Failed to remove LLM provider API keys from memory store after deletion",
			slog.String("provider_id", entityID),
			slog.Any("error", err))
	}

	if existingConfig != nil && l.apiKeyXDSManager != nil {
		apiName, apiVersion := extractAPINameVersion(existingConfig)
		if apiName != "" {
			if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiName, apiVersion, event.EventID); err != nil {
				l.logger.Warn("Failed to remove LLM provider API keys from policy engine after deletion",
					slog.String("provider_id", entityID),
					slog.String("api_name", apiName),
					slog.String("api_version", apiVersion),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
			}
		}
	}

	if existingConfig != nil {
		if err := l.removeProviderTemplateMapping(existingConfig.Handle, event.EventID); err != nil {
			l.logger.Warn("Failed to remove provider-template mapping",
				slog.String("provider_id", entityID),
				slog.String("provider_handle", existingConfig.Handle),
				slog.Any("error", err))
		}
	}

	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after LLM provider deletion")

	if l.policyManager != nil && existingConfig != nil {
		if err := l.policyManager.DeleteAPIConfig(existingConfig.Kind, existingConfig.Handle); err != nil {
			l.logger.Warn("Failed to remove runtime config after LLM provider deletion",
				slog.String("provider_id", entityID),
				slog.Any("error", err))
		}
	}

	l.logger.Info("Successfully processed LLM provider delete event",
		slog.String("provider_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) handleLLMProxyDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing LLM proxy delete event",
		slog.String("proxy_id", entityID),
		slog.String("event_id", event.EventID))

	existingConfig, err := l.store.Get(entityID)
	if err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to load LLM proxy from memory store before deletion",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}

	if err := l.store.Delete(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to delete LLM proxy from memory store",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
		return
	}

	if err := l.store.RemoveAPIKeysByAPI(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Warn("Failed to remove LLM proxy API keys from memory store after deletion",
			slog.String("proxy_id", entityID),
			slog.Any("error", err))
	}

	if existingConfig != nil && l.apiKeyXDSManager != nil {
		apiName, apiVersion := extractAPINameVersion(existingConfig)
		if apiName != "" {
			if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiName, apiVersion, event.EventID); err != nil {
				l.logger.Warn("Failed to remove LLM proxy API keys from policy engine after deletion",
					slog.String("proxy_id", entityID),
					slog.String("api_name", apiName),
					slog.String("api_version", apiVersion),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
			}
		}
	}

	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after LLM proxy deletion")

	if l.policyManager != nil && existingConfig != nil {
		if err := l.policyManager.DeleteAPIConfig(existingConfig.Kind, existingConfig.Handle); err != nil {
			l.logger.Warn("Failed to remove runtime config after LLM proxy deletion",
				slog.String("proxy_id", entityID),
				slog.Any("error", err))
		}
	}

	l.logger.Info("Successfully processed LLM proxy delete event",
		slog.String("proxy_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) hydrateLLMConfig(cfg *models.StoredConfig) error {
	return utils.HydrateLLMConfig(cfg, l.store, l.db, l.routerConfig, l.policyDefinitions)
}

func (l *EventListener) syncProviderTemplateMapping(cfg *models.StoredConfig, correlationID string) error {
	if l.lazyResourceManager == nil || cfg == nil {
		return nil
	}

	providerConfig, ok := cfg.SourceConfiguration.(api.LLMProviderConfiguration)
	if !ok {
		return fmt.Errorf("unexpected LLM provider source configuration type %T", cfg.SourceConfiguration)
	}

	if cfg.Handle == "" {
		return fmt.Errorf("provider handle is empty")
	}
	if providerConfig.Spec.Template == "" {
		return l.removeProviderTemplateMapping(cfg.Handle, correlationID)
	}

	mappingResource := map[string]interface{}{
		"provider_name":   cfg.Handle,
		"template_handle": providerConfig.Spec.Template,
	}

	return l.lazyResourceManager.StoreResource(&storage.LazyResource{
		ID:           cfg.Handle,
		ResourceType: utils.LazyResourceTypeProviderTemplateMapping,
		Resource:     mappingResource,
	}, correlationID)
}

func (l *EventListener) removeProviderTemplateMapping(providerHandle, correlationID string) error {
	if l.lazyResourceManager == nil || providerHandle == "" {
		return nil
	}
	return l.lazyResourceManager.RemoveResourceByIDAndType(
		providerHandle,
		utils.LazyResourceTypeProviderTemplateMapping,
		correlationID,
	)
}
