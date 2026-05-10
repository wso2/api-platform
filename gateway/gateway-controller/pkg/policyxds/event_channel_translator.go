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

	// Build channels list from hub channels, including per-channel subscribe policies.
	channels := make([]map[string]interface{}, 0, len(spec.Hub.Channels))
	for _, ch := range spec.Hub.Channels {
		chEntry := map[string]interface{}{
			"name": ch.Name,
			"policies": map[string]interface{}{
				"subscribe": buildPolicyList(ch.Policies),
				"inbound":   []interface{}{},
				"outbound":  []interface{}{},
			},
		}
		channels = append(channels, chEntry)
	}

	// Hub-level policies apply to the subscribe phase only (authenticating subscribers).
	subscribePolicies := buildPolicyList(spec.Hub.Policies)

	// Receiver-level policies apply to the inbound phase only (validating publisher webhook requests).
	inboundPolicies := []interface{}{}
	if spec.Receiver != nil {
		inboundPolicies = buildPolicyList(spec.Receiver.Policies)
	}

	// Delivery-level policies apply to the outbound phase only (signing/transforming delivery to subscriber callbacks).
	outboundPolicies := []interface{}{}
	if spec.Delivery != nil {
		outboundPolicies = buildPolicyList(spec.Delivery.Policies)
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
			"subscribe": subscribePolicies,
			"inbound":   inboundPolicies,
			"outbound":  outboundPolicies,
		},
	}

	return toAnyResource(data, EventChannelConfigTypeURL)
}

func (t *Translator) buildEventChannelResourceForWebBroker(uuid string, webBrokerCfg *api.WebBrokerApi) (types.Resource, error) {
	spec := webBrokerCfg.Spec

	// Build receiver configuration
	receiver := map[string]interface{}{
		"type": spec.Receiver.Type,
	}
	if spec.Receiver.Properties != nil {
		receiver["properties"] = *spec.Receiver.Properties
	}

	// Build broker-driver configuration
	brokerDriver := map[string]interface{}{
		"type":       spec.BrokerDriver.Type,
		"properties": spec.BrokerDriver.Properties,
	}
	slog.Info("DEBUG: Building EventChannelConfig for WebBrokerApi",
		"name", webBrokerCfg.Metadata.Name,
		"brokerDriverType", spec.BrokerDriver.Type,
		"brokerDriverConfig", spec.BrokerDriver.Properties)

	// Build protocol mediation policies
	var onConnectionInitRequest []interface{}
	var onConnectionInitResponse []interface{}
	var onProduce []interface{}
	var onConsume []interface{}

	if spec.AllChannelPolicies != nil {
		if spec.AllChannelPolicies.OnConnectionInit != nil {
			if spec.AllChannelPolicies.OnConnectionInit.Request != nil {
				onConnectionInitRequest = buildPolicyList(spec.AllChannelPolicies.OnConnectionInit.Request)
			}
			if spec.AllChannelPolicies.OnConnectionInit.Response != nil {
				onConnectionInitResponse = buildPolicyList(spec.AllChannelPolicies.OnConnectionInit.Response)
			}
		}
		if spec.AllChannelPolicies.OnProduce != nil {
			onProduce = buildPolicyList(spec.AllChannelPolicies.OnProduce)
		}
		if spec.AllChannelPolicies.OnConsume != nil {
			onConsume = buildPolicyList(spec.AllChannelPolicies.OnConsume)
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
		"allChannelPolicies": map[string]interface{}{
			"on_connection_init": map[string]interface{}{
				"request":  onConnectionInitRequest,
				"response": onConnectionInitResponse,
			},
			"on_produce": onProduce,
			"on_consume": onConsume,
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
