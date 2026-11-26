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

package controlplane

import (
	"errors"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func (c *Client) ensureLLMProviderTemplateFromYAML(yamlData []byte, correlationID string, refreshed map[string]struct{}) {
	if c.apiUtilsService == nil || c.llmDeploymentService == nil {
		return
	}

	handle := llmProviderTemplateHandle(yamlData)
	if handle == "" {
		return
	}
	if refreshed != nil {
		if _, done := refreshed[handle]; done {
			return
		}
		refreshed[handle] = struct{}{}
	}

	existing, err := c.llmDeploymentService.GetLLMProviderTemplateByHandle(handle)
	if err != nil && !storage.IsNotFoundError(err) {
		c.logger.Warn("Failed to look up the LLM provider template referenced by a provider",
			slog.String("template_handle", handle),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return
	}
	held := err == nil
	if held && !existing.IsFromControlPlane() {
		return
	}

	templateYAML, err := c.apiUtilsService.FetchLLMProviderTemplateDefinition(handle)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrControlPlaneNotFound) && held:
			c.logger.Warn("The control plane no longer has this LLM provider template; keeping the copy the gateway holds",
				slog.String("template_handle", handle),
				slog.String("correlation_id", correlationID),
			)
		case errors.Is(err, utils.ErrControlPlaneNotFound):
			c.logger.Warn("The control plane has no LLM provider template for this handle",
				slog.String("template_handle", handle),
				slog.String("correlation_id", correlationID),
			)
		default:
			c.logger.Error("Failed to fetch LLM provider template from the control plane",
				slog.String("template_handle", handle),
				slog.String("correlation_id", correlationID),
				slog.Bool("keeping_held_copy", held),
				slog.Any("error", err),
			)
		}
		return
	}

	// The template gets its own event ID: the event hub drops a second event
	// with an ID it has already seen for this gateway, and the provider's own
	// event carries correlationID.
	templateEventID, err := utils.GenerateUUID()
	if err != nil {
		c.logger.Error("Failed to generate an event ID for the fetched LLM provider template",
			slog.String("template_handle", handle),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return
	}

	stored, result, err := c.llmDeploymentService.ApplyControlPlaneLLMProviderTemplate(utils.LLMTemplateParams{
		Spec:          templateYAML,
		ContentType:   "application/yaml",
		CorrelationID: templateEventID,
		Logger:        c.logger,
	})
	if err != nil {
		if storage.IsConflictError(err) {
			// Another deploy saved the same template between the lookup and here.
			c.logger.Debug("LLM provider template was saved concurrently",
				slog.String("template_handle", handle),
				slog.String("correlation_id", correlationID),
			)
			return
		}
		c.logger.Error("Failed to save LLM provider template fetched from the control plane",
			slog.String("template_handle", handle),
			slog.String("correlation_id", correlationID),
			slog.Bool("keeping_held_copy", held),
			slog.Any("error", err),
		)
		return
	}

	switch result {
	case utils.ControlPlaneTemplateCreated, utils.ControlPlaneTemplateUpdated:
		c.logger.Info("Applied LLM provider template fetched from the control plane",
			slog.String("template_handle", handle),
			slog.String("template_id", stored.UUID),
			slog.String("result", string(result)),
			slog.String("correlation_id", correlationID),
			slog.String("template_event_id", templateEventID),
		)
	default:
		c.logger.Debug("LLM provider template from the control plane needed no change",
			slog.String("template_handle", handle),
			slog.String("result", string(result)),
			slog.String("correlation_id", correlationID),
		)
	}
}

// llmProviderTemplateHandle returns the spec.template an LLM provider YAML
// references, or "" when it names none or does not parse.
func llmProviderTemplateHandle(yamlData []byte) string {
	var provider struct {
		Spec struct {
			Template string `yaml:"template"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(yamlData, &provider); err != nil {
		return ""
	}
	return strings.TrimSpace(provider.Spec.Template)
}
