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
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func (l *EventListener) processLLMTemplateEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleLLMTemplateCreateOrUpdate(event)
	case "DELETE":
		l.handleLLMTemplateDelete(event)
	default:
		l.logger.Warn("Unknown LLM template event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) handleLLMTemplateCreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing LLM template create/update event",
		slog.String("template_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	if l.db == nil {
		l.logger.Warn("Database not available, cannot process LLM template event",
			slog.String("template_id", entityID))
		return
	}

	storedTemplate, err := l.db.GetLLMProviderTemplate(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch LLM template from database",
			slog.String("template_id", entityID),
			slog.Any("error", err))
		return
	}

	existing, err := l.store.GetTemplate(entityID)
	if err == nil && existing != nil {
		if err := l.store.UpdateTemplate(storedTemplate); err != nil {
			l.logger.Error("Failed to update LLM template in memory store",
				slog.String("template_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			l.logger.Error("Failed to load LLM template from memory store",
				slog.String("template_id", entityID),
				slog.Any("error", err))
			return
		}
		if err := l.store.AddTemplate(storedTemplate); err != nil {
			l.logger.Error("Failed to add LLM template to memory store",
				slog.String("template_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	if err := l.syncLLMTemplate(storedTemplate, event.EventID); err != nil {
		l.logger.Warn("Failed to sync LLM template lazy resource",
			slog.String("template_id", entityID),
			slog.Any("error", err))
	}

	l.logger.Info("Successfully processed LLM template create/update event",
		slog.String("template_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) handleLLMTemplateDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing LLM template delete event",
		slog.String("template_id", entityID),
		slog.String("event_id", event.EventID))

	// If store does not have the template, we can skip processing since the end state is the same (template not present)
	existingTemplate, err := l.store.GetTemplate(entityID)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		l.logger.Error("Failed to load LLM template from memory store before deletion",
			slog.String("template_id", entityID),
			slog.Any("error", err))
		return
	}

	if err := l.store.DeleteTemplate(entityID); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		l.logger.Error("Failed to delete LLM template from memory store",
			slog.String("template_id", entityID),
			slog.Any("error", err))
		return
	}

	if existingTemplate != nil {
		if err := l.removeLLMTemplate(existingTemplate.GetHandle(), event.EventID); err != nil {
			l.logger.Warn("Failed to remove LLM template lazy resource",
				slog.String("template_id", entityID),
				slog.String("template_handle", existingTemplate.GetHandle()),
				slog.Any("error", err))
		}
	}

	l.logger.Info("Successfully processed LLM template delete event",
		slog.String("template_id", entityID),
		slog.String("event_id", event.EventID))
}

func (l *EventListener) syncLLMTemplate(template *models.StoredLLMProviderTemplate, correlationID string) error {
	if l.lazyResourceManager == nil || template == nil {
		return nil
	}

	resource, err := l.buildLLMTemplateLazyResource(template.Configuration)
	if err != nil {
		return err
	}

	return l.lazyResourceManager.StoreResource(resource, correlationID)
}

func (l *EventListener) removeLLMTemplate(handle, correlationID string) error {
	if l.lazyResourceManager == nil || handle == "" {
		return nil
	}

	return l.lazyResourceManager.RemoveResourceByIDAndType(
		handle,
		utils.LazyResourceTypeLLMProviderTemplate,
		correlationID,
	)
}

func (l *EventListener) buildLLMTemplateLazyResource(template api.LLMProviderTemplate) (*storage.LazyResource, error) {
	if template.Metadata.Name == "" {
		return nil, fmt.Errorf("template handle (metadata.name) is empty")
	}

	payload, err := json.Marshal(template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal template as JSON: %w", err)
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(payload, &resource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template JSON into map: %w", err)
	}

	return &storage.LazyResource{
		ID:           template.Metadata.Name,
		ResourceType: utils.LazyResourceTypeLLMProviderTemplate,
		Resource:     resource,
	}, nil
}
