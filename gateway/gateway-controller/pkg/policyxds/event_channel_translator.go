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
	"sort"

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

	// Build channels list from channelPolicies, including per-channel policies.
	var channelPolicies map[string]api.WebSubChannelPolicies
	if spec.ChannelPolicies != nil {
		channelPolicies = *spec.ChannelPolicies
	}
	channels := make([]map[string]interface{}, 0, len(channelPolicies))
	sortedKeys := make([]string, 0, len(channelPolicies))
	for chName := range channelPolicies {
		sortedKeys = append(sortedKeys, chName)
	}
	sort.Strings(sortedKeys)
	for _, chName := range sortedKeys {
		chPolicies := channelPolicies[chName]
		chEntry := map[string]interface{}{
			"name": chName,
			"policies": map[string]interface{}{
				"subscribe":   buildPolicyList(chPolicies.OnSubscription),
				"unsubscribe": buildPolicyList(chPolicies.OnUnsubscription),
				"inbound":     buildPolicyList(chPolicies.OnMessageReceived),
				"outbound":    buildPolicyList(chPolicies.OnMessageDelivery),
			},
		}
		channels = append(channels, chEntry)
	}

	// allChannelPolicies maps to API-level policy chains.
	subscribePolicies := []interface{}{}
	unsubscribePolicies := []interface{}{}
	inboundPolicies := []interface{}{}
	outboundPolicies := []interface{}{}
	if spec.AllChannelPolicies != nil {
		subscribePolicies = buildPolicyList(spec.AllChannelPolicies.OnSubscription)
		unsubscribePolicies = buildPolicyList(spec.AllChannelPolicies.OnUnsubscription)
		inboundPolicies = buildPolicyList(spec.AllChannelPolicies.OnMessageReceived)
		outboundPolicies = buildPolicyList(spec.AllChannelPolicies.OnMessageDelivery)
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
		"policies": map[string]interface{}{
			"subscribe":   subscribePolicies,
			"unsubscribe": unsubscribePolicies,
			"inbound":     inboundPolicies,
			"outbound":    outboundPolicies,
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
