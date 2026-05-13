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

		resource, err := t.buildEventChannelResourceForWebSub(cfg.UUID, &webSubCfg)
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

// TranslateWebBrokerApisToEventChannelConfigs translates WebBrokerApi StoredConfigs into
// EventChannelConfig xDS resources for the event gateway runtime.
func (t *Translator) TranslateWebBrokerApisToEventChannelConfigs(configs []*models.StoredConfig) map[string]types.Resource {
	resources := make(map[string]types.Resource)

	for _, cfg := range configs {
		if cfg.Kind != models.KindWebBrokerApi {
			continue
		}
		if cfg.DesiredState != models.StateDeployed {
			continue
		}

		webBrokerCfg, ok := cfg.Configuration.(api.WebBrokerApi)
		if !ok {
			t.logger.Warn("Failed to type-assert WebBrokerApi configuration",
				slog.String("uuid", cfg.UUID))
			continue
		}

		resource, err := t.buildEventChannelResourceForWebBroker(cfg.UUID, &webBrokerCfg)
		if err != nil {
			t.logger.Error("Failed to build EventChannelConfig resource for WebBrokerApi",
				slog.String("uuid", cfg.UUID),
				slog.Any("error", err))
			continue
		}

		resources[cfg.UUID] = resource
	}

	t.logger.Info("Translated WebBrokerApis to EventChannelConfig resources",
		slog.Int("input_configs", len(configs)),
		slog.Int("output_resources", len(resources)))

	return resources
}

func (t *Translator) buildEventChannelResourceForWebSub(uuid string, webSubCfg *api.WebSubAPI) (types.Resource, error) {
	spec := webSubCfg.Spec

	// Build channels list from channels, including per-channel policies.
	var channels map[string]api.WebSubChannel
	if spec.Channels != nil {
		channels = *spec.Channels
	}
	channelEntries := make([]map[string]interface{}, 0, len(channels))
	sortedKeys := make([]string, 0, len(channels))
	for chName := range channels {
		sortedKeys = append(sortedKeys, chName)
	}
	sort.Strings(sortedKeys)
	for _, chName := range sortedKeys {
		ch := channels[chName]
		var subPolicies, unsubPolicies, inboundPolicies, outboundPolicies []interface{}
		if ch.OnSubscription != nil {
			subPolicies = buildPolicyList(ch.OnSubscription.Policies)
		}
		if ch.OnUnsubscription != nil {
			unsubPolicies = buildPolicyList(ch.OnUnsubscription.Policies)
		}
		if ch.OnMessageReceived != nil {
			inboundPolicies = buildPolicyList(ch.OnMessageReceived.Policies)
		}
		if ch.OnMessageDelivery != nil {
			outboundPolicies = buildPolicyList(ch.OnMessageDelivery.Policies)
		}
		chEntry := map[string]interface{}{
			"name": chName,
			"policies": map[string]interface{}{
				"subscribe":   subPolicies,
				"unsubscribe": unsubPolicies,
				"inbound":     inboundPolicies,
				"outbound":    outboundPolicies,
			},
		}
		channelEntries = append(channelEntries, chEntry)
	}

	// policies maps to API-level policy chains.
	subscribePolicies := []interface{}{}
	unsubscribePolicies := []interface{}{}
	inboundPolicies := []interface{}{}
	outboundPolicies := []interface{}{}
	if spec.AllChannels != nil {
		if spec.AllChannels.OnSubscription != nil {
			subscribePolicies = buildPolicyList(spec.AllChannels.OnSubscription.Policies)
		}
		if spec.AllChannels.OnUnsubscription != nil {
			unsubscribePolicies = buildPolicyList(spec.AllChannels.OnUnsubscription.Policies)
		}
		if spec.AllChannels.OnMessageReceived != nil {
			inboundPolicies = buildPolicyList(spec.AllChannels.OnMessageReceived.Policies)
		}
		if spec.AllChannels.OnMessageDelivery != nil {
			outboundPolicies = buildPolicyList(spec.AllChannels.OnMessageDelivery.Policies)
		}
	}

	data := map[string]interface{}{
		"uuid":     uuid,
		"name":     string(webSubCfg.Metadata.Name),
		"kind":     "WebSubApi",
		"context":  spec.Context,
		"version":  spec.Version,
		"channels": channelEntries,
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

func (t *Translator) buildEventChannelResourceForWebBroker(uuid string, webBrokerCfg *api.WebBrokerApi) (types.Resource, error) {
	spec := webBrokerCfg.Spec

	// Build receiver configuration
	receiver := map[string]interface{}{
		"name": spec.Receiver.Name,
		"type": spec.Receiver.Type,
	}
	if spec.Receiver.Properties != nil {
		receiver["properties"] = *spec.Receiver.Properties
	}

	// Build broker-driver configuration
	brokerDriver := map[string]interface{}{
		"name":       spec.Broker.Name,
		"type":       spec.Broker.Type,
		"properties": spec.Broker.Properties,
	}
	slog.Info("DEBUG: Building EventChannelConfig for WebBrokerApi",
		"name", webBrokerCfg.Metadata.Name,
		"receiverName", spec.Receiver.Name,
		"brokerDriverName", spec.Broker.Name,
		"brokerDriverType", spec.Broker.Type)

	// Build API-level policies from AllChannels
	var apiOnConnectionInit []interface{}
	var apiOnProduce []interface{}
	var apiOnConsume []interface{}

	if spec.AllChannels != nil {
		if spec.AllChannels.OnConnectionInit != nil && spec.AllChannels.OnConnectionInit.Policies != nil {
			apiOnConnectionInit = buildPolicyList(spec.AllChannels.OnConnectionInit.Policies)
		}
		if spec.AllChannels.OnProduce != nil && spec.AllChannels.OnProduce.Policies != nil {
			apiOnProduce = buildPolicyList(spec.AllChannels.OnProduce.Policies)
		}
		if spec.AllChannels.OnConsume != nil && spec.AllChannels.OnConsume.Policies != nil {
			apiOnConsume = buildPolicyList(spec.AllChannels.OnConsume.Policies)
		}
	}

	// Build channels map with channel-specific policies and topic mappings
	channels := make(map[string]interface{})
	if spec.Channels != nil {
		for channelName, channelConfig := range spec.Channels {
			var channelOnConnectionInit []interface{}
			var channelOnProduce []interface{}
			var channelOnConsume []interface{}

			// Extract policies from channel-level policy groups
			if channelConfig.OnConnectionInit != nil && channelConfig.OnConnectionInit.Policies != nil {
				channelOnConnectionInit = buildPolicyList(channelConfig.OnConnectionInit.Policies)
			}
			if channelConfig.OnProduce != nil && channelConfig.OnProduce.Policies != nil {
				channelOnProduce = buildPolicyList(channelConfig.OnProduce.Policies)
			}
			if channelConfig.OnConsume != nil && channelConfig.OnConsume.Policies != nil {
				channelOnConsume = buildPolicyList(channelConfig.OnConsume.Policies)
			}

			// Build channel entry with policies nested inside "policies" field
			channelEntry := map[string]interface{}{}

			// Add produceTo topic mapping if specified
			if channelConfig.ProduceTo != nil {
				channelEntry["produce_to"] = map[string]interface{}{
					"topic": channelConfig.ProduceTo.Topic,
				}
			}

			// Add consumeFrom topic mapping if specified
			if channelConfig.ConsumeFrom != nil {
				channelEntry["consume_from"] = map[string]interface{}{
					"topic": channelConfig.ConsumeFrom.Topic,
				}
			}

			// Nest all policies inside a "policies" field (flattened structure)
			channelEntry["policies"] = map[string]interface{}{
				"on_connection_init": channelOnConnectionInit,
				"on_produce":         channelOnProduce,
				"on_consume":         channelOnConsume,
			}

			channels[channelName] = channelEntry
		}
	}

	data := map[string]interface{}{
		"uuid":          uuid,
		"name":          string(webBrokerCfg.Metadata.Name),
		"kind":          "WebBrokerApi",
		"context":       spec.Context,
		"version":       spec.Version,
		"receiver":      receiver,
		"broker-driver": brokerDriver,
		"policies": map[string]interface{}{
			"on_connection_init": apiOnConnectionInit,
			"on_produce":         apiOnProduce,
			"on_consume":         apiOnConsume,
		},
		"channels": channels,
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
