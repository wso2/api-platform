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

// Package policyhooks implements gateway-controller (core)'s
// policyxds.EventChannelTranslator interface and policy.RegisterEventGatewayPolicyChainBuilder
// hook, moved out of core's pkg/policyxds/event_channel_translator.go and
// pkg/policy/builder.go (the WebSubApi case).
package policyhooks

import (
	"log/slog"
	"sort"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	versionutil "github.com/wso2/api-platform/common/version"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
)

// EventChannelTranslator implements policyxds.EventChannelTranslator.
type EventChannelTranslator struct {
	logger *slog.Logger
}

// New creates a new EventChannelTranslator.
func New(logger *slog.Logger) *EventChannelTranslator {
	return &EventChannelTranslator{logger: logger}
}

var _ policyxds.EventChannelTranslator = (*EventChannelTranslator)(nil)

// TranslateWebSubApisToEventChannelConfigs translates WebSubApi StoredConfigs into
// EventChannelConfig xDS resources for the event gateway runtime.
func (t *EventChannelTranslator) TranslateWebSubApisToEventChannelConfigs(configs []*models.StoredConfig) map[string]types.Resource {
	resources := make(map[string]types.Resource)

	for _, cfg := range configs {
		if cfg.Kind != models.KindWebSubApi {
			continue
		}
		if cfg.DesiredState != models.StateDeployed {
			continue
		}

		webSubCfg, ok := cfg.Configuration.(eventgateway.WebSubAPI)
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
func (t *EventChannelTranslator) TranslateWebBrokerApisToEventChannelConfigs(configs []*models.StoredConfig) map[string]types.Resource {
	resources := make(map[string]types.Resource)

	for _, cfg := range configs {
		if cfg.Kind != models.KindWebBrokerApi {
			continue
		}
		if cfg.DesiredState != models.StateDeployed {
			continue
		}

		webBrokerCfg, ok := cfg.Configuration.(eventgateway.WebBrokerApi)
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

func (t *EventChannelTranslator) buildEventChannelResourceForWebSub(uuid string, webSubCfg *eventgateway.WebSubAPI) (types.Resource, error) {
	spec := webSubCfg.Spec

	var channels map[string]eventgateway.WebSubChannel
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

	return policyxds.ToAnyResource(data, policyxds.EventChannelConfigTypeURL)
}

func (t *EventChannelTranslator) buildEventChannelResourceForWebBroker(uuid string, webBrokerCfg *eventgateway.WebBrokerApi) (types.Resource, error) {
	spec := webBrokerCfg.Spec

	receiver := map[string]interface{}{
		"name": spec.Receiver.Name,
		"type": spec.Receiver.Type,
	}
	if spec.Receiver.Properties != nil {
		receiver["properties"] = *spec.Receiver.Properties
	}

	brokerDriver := map[string]interface{}{
		"name":       spec.Broker.Name,
		"type":       spec.Broker.Type,
		"properties": spec.Broker.Properties,
	}

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

	channels := make(map[string]interface{})
	if spec.Channels != nil {
		for channelName, channelConfig := range spec.Channels {
			var channelOnConnectionInit []interface{}
			var channelOnProduce []interface{}
			var channelOnConsume []interface{}

			if channelConfig.OnConnectionInit != nil && channelConfig.OnConnectionInit.Policies != nil {
				channelOnConnectionInit = buildPolicyList(channelConfig.OnConnectionInit.Policies)
			}
			if channelConfig.OnProduce != nil && channelConfig.OnProduce.Policies != nil {
				channelOnProduce = buildPolicyList(channelConfig.OnProduce.Policies)
			}
			if channelConfig.OnConsume != nil && channelConfig.OnConsume.Policies != nil {
				channelOnConsume = buildPolicyList(channelConfig.OnConsume.Policies)
			}

			channelEntry := map[string]interface{}{}

			if channelConfig.ProduceTo != nil {
				channelEntry["produce_to"] = map[string]interface{}{
					"topic": channelConfig.ProduceTo.Topic,
				}
			}

			if channelConfig.ConsumeFrom != nil {
				channelEntry["consume_from"] = map[string]interface{}{
					"topic": channelConfig.ConsumeFrom.Topic,
				}
			}

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

	return policyxds.ToAnyResource(data, policyxds.EventChannelConfigTypeURL)
}

func buildPolicyList(policies *[]eventgateway.Policy) []interface{} {
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

// BuildPolicyChains implements the hook registered via
// policy.RegisterEventGatewayPolicyChainBuilder, building per-channel policy
// chains for WebSubApi configs (RestApi is handled natively by core).
func BuildPolicyChains(cfg *models.StoredConfig, routerConfig *config.RouterConfig, systemConfig *config.Config, policyDefinitions map[string]models.PolicyDefinition) []policyenginev1.PolicyChain {
	cfgTyped, ok := cfg.Configuration.(eventgateway.WebSubAPI)
	if !ok {
		return nil
	}

	latestVersions := config.BuildLatestVersionIndex(policyDefinitions)
	var routes []policyenginev1.PolicyChain

	apiData := cfgTyped.Spec
	var channels map[string]eventgateway.WebSubChannel
	if apiData.Channels != nil {
		channels = *apiData.Channels
	}
	for chName, ch := range channels {
		var finalPolicies []policyenginev1.PolicyInstance

		if apiData.AllChannels != nil && apiData.AllChannels.OnSubscription != nil && apiData.AllChannels.OnSubscription.Policies != nil {
			finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.AllChannels.OnSubscription.Policies))
			for _, p := range *apiData.AllChannels.OnSubscription.Policies {
				resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, p.Name, p.Version)
				if err != nil {
					slog.Error("Failed to resolve policy version for all-channel subscription policy", "policy_name", p.Name, "error", err)
					continue
				}
				finalPolicies = append(finalPolicies, policy.ConvertAPIPolicyToModel(api.Policy(p), policyv1alpha.LevelAPI, versionutil.MajorVersion(resolved)))
			}
		}

		if ch.OnSubscription != nil && ch.OnSubscription.Policies != nil && len(*ch.OnSubscription.Policies) > 0 {
			for _, opPolicy := range *ch.OnSubscription.Policies {
				resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, opPolicy.Name, opPolicy.Version)
				if err != nil {
					slog.Error("Failed to resolve policy version for channel-level policy", "policy_name", opPolicy.Name, "channel_name", chName, "error", err)
					continue
				}
				finalPolicies = append(finalPolicies, policy.ConvertAPIPolicyToModel(api.Policy(opPolicy), policyv1alpha.LevelRoute, versionutil.MajorVersion(resolved)))
			}
		}

		routeKey := xds.GenerateRouteName("SUB", apiData.Context, apiData.Version, chName, routerConfig.GatewayHost)
		props := make(map[string]any)
		injectedPolicies := utils.InjectSystemPolicies(finalPolicies, systemConfig, props)

		routes = append(routes, policyenginev1.PolicyChain{
			RouteKey: routeKey,
			Policies: injectedPolicies,
		})

		var unsubPolicies []policyenginev1.PolicyInstance
		if apiData.AllChannels != nil && apiData.AllChannels.OnUnsubscription != nil && apiData.AllChannels.OnUnsubscription.Policies != nil {
			unsubPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.AllChannels.OnUnsubscription.Policies))
			for _, p := range *apiData.AllChannels.OnUnsubscription.Policies {
				resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, p.Name, p.Version)
				if err != nil {
					slog.Error("Failed to resolve policy version for all-channel unsubscription policy", "policy_name", p.Name, "error", err)
					continue
				}
				unsubPolicies = append(unsubPolicies, policy.ConvertAPIPolicyToModel(api.Policy(p), policyv1alpha.LevelAPI, versionutil.MajorVersion(resolved)))
			}
		}
		if ch.OnUnsubscription != nil && ch.OnUnsubscription.Policies != nil && len(*ch.OnUnsubscription.Policies) > 0 {
			for _, opPolicy := range *ch.OnUnsubscription.Policies {
				resolved, err := config.ResolvePolicyVersion(policyDefinitions, latestVersions, opPolicy.Name, opPolicy.Version)
				if err != nil {
					slog.Error("Failed to resolve policy version for channel-level unsubscription policy", "policy_name", opPolicy.Name, "channel_name", chName, "error", err)
					continue
				}
				unsubPolicies = append(unsubPolicies, policy.ConvertAPIPolicyToModel(api.Policy(opPolicy), policyv1alpha.LevelRoute, versionutil.MajorVersion(resolved)))
			}
		}
		unsubRouteKey := xds.GenerateRouteName("UNSUB", apiData.Context, apiData.Version, chName, routerConfig.GatewayHost)
		unsubProps := make(map[string]any)
		injectedUnsubPolicies := utils.InjectSystemPolicies(unsubPolicies, systemConfig, unsubProps)
		routes = append(routes, policyenginev1.PolicyChain{
			RouteKey: unsubRouteKey,
			Policies: injectedUnsubPolicies,
		})
	}

	return routes
}
