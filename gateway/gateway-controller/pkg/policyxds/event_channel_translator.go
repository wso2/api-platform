/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package policyxds

import (
	"log/slog"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// TranslateWebSubApisToEventChannelConfigs translates WebSubApi StoredConfigs into
// EventChannelConfig xDS resources for the event gateway runtime.
func (t *Translator) TranslateWebSubApisToEventChannelConfigs(configs []*models.StoredConfig) map[string]types.Resource {
	resources := make(map[string]types.Resource)

	for _, cfg := range configs {
		if cfg.Kind != models.KindWebSubApi {
			continue
		}
		if cfg.DesiredState != models.StateDeployed {
			continue
		}

		webSubCfg, ok := cfg.Configuration.(api.WebSubAPI)
		if !ok {
			t.logger.Warn("Failed to type-assert WebSubApi configuration",
				slog.String("uuid", cfg.UUID))
			continue
		}

		resource, err := t.buildEventChannelResource(cfg.UUID, &webSubCfg)
		if err != nil {
			t.logger.Error("Failed to build EventChannelConfig resource",
				slog.String("uuid", cfg.UUID),
				slog.Any("error", err))
			continue
		}

		resources[cfg.UUID] = resource
	}

	t.logger.Info("Translated WebSubApis to EventChannelConfig resources",
		slog.Int("input_configs", len(configs)),
		slog.Int("output_resources", len(resources)))

	return resources
}

func (t *Translator) buildEventChannelResource(uuid string, webSubCfg *api.WebSubAPI) (types.Resource, error) {
	spec := webSubCfg.Spec

	// Build channels list
	channels := make([]map[string]interface{}, 0, len(spec.Channels))
	for _, ch := range spec.Channels {
		channels = append(channels, map[string]interface{}{
			"name": ch.Name,
		})
	}

	// Build 3-phase policies from channel-level and API-level policies.
	// The controller's Channel model has a flat Policies list; we map them
	// all to the subscribe phase for now, with API-level policies applied
	// as subscribe policies as well. Inbound/outbound are left empty for
	// the event gateway to handle via its own defaults.
	subscribePolicies := buildPolicyList(spec.Policies)
	// Channel-level policies override API-level for subscribe phase
	if len(spec.Channels) > 0 && spec.Channels[0].Policies != nil && len(*spec.Channels[0].Policies) > 0 {
		subscribePolicies = buildPolicyList(spec.Channels[0].Policies)
	}

	// Build broker driver config — use Kafka brokers from controller config
	brokerDriverConfig := map[string]interface{}{
		"type": "kafka",
		"config": map[string]interface{}{
			"brokers": t.kafkaBrokers,
		},
	}

	data := map[string]interface{}{
		"uuid":     uuid,
		"name":     string(webSubCfg.Metadata.Name),
		"kind":     "WebSubApi",
		"context":  spec.Context,
		"version":  spec.Version,
		"channels": channels,
		"receiver": map[string]interface{}{
			"type": "websub",
		},
		"brokerDriver": brokerDriverConfig,
		"policies": map[string]interface{}{
			"subscribe": subscribePolicies,
			"inbound":   []interface{}{},
			"outbound":  []interface{}{},
		},
	}

	return toAnyResource(data, EventChannelConfigTypeURL)
}

func buildPolicyList(policies *[]api.Policy) []interface{} {
	if policies == nil || len(*policies) == 0 {
		return []interface{}{}
	}

	result := make([]interface{}, 0, len(*policies))
	for _, p := range *policies {
		pol := map[string]interface{}{
			"name":    p.Name,
			"version": p.Version,
		}
		if p.Params != nil {
			pol["params"] = *p.Params
		}
		result = append(result, pol)
	}
	return result
}
