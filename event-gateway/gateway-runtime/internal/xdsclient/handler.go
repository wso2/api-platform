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

package xdsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
)

// BindingManager can add/remove WebSubApi and WebBrokerApi bindings dynamically.
type BindingManager interface {
	AddWebSubApiBinding(wsb binding.WebSubApiBinding) error
	RemoveWebSubApiBinding(name string) error
	AddWebBrokerApiBinding(wbb binding.WebBrokerApiBinding) error
	RemoveWebBrokerApiBinding(name string) error
}

// KafkaConfig holds local Kafka broker settings used as defaults.
type KafkaConfig struct {
	Brokers []string
}

// EventChannelResource represents the decoded EventChannelConfig JSON payload.
type EventChannelResource struct {
	UUID         string            `json:"uuid"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Context      string            `json:"context"`
	Version      string            `json:"version"`
	Deleted      bool              `json:"deleted,omitempty"`
	Channels     interface{}       `json:"channels"`      // []ChannelEntry for WebSubApi, map[string]WebBrokerChannelEntry for WebBrokerApi
	Receiver     ReceiverEntry     `json:"receiver"`      // For WebBrokerApi
	BrokerDriver BrokerDriverEntry `json:"broker-driver"` // For WebBrokerApi
	Policies     interface{}       `json:"policies"`      // PoliciesEntry for WebSubApi, ProtocolMediationPolicies for WebBrokerApi
}

// WebBrokerChannelEntry represents a WebBrokerApi channel with policies
type WebBrokerChannelEntry struct {
	ProduceTo   *TopicMapping             `json:"produce_to,omitempty"`
	ConsumeFrom *TopicMapping             `json:"consume_from,omitempty"`
	Policies    ProtocolMediationPolicies `json:"policies"`
}

// TopicMapping defines a Kafka topic mapping
type TopicMapping struct {
	Topic string `json:"topic"`
}

// ProtocolMediationPolicies defines policies for WebBrokerApi
type ProtocolMediationPolicies struct {
	OnConnectionInit ConnectionInitPolicies `json:"on_connection_init"`
	OnProduce        []PolicyEntry          `json:"on_produce"`
	OnConsume        []PolicyEntry          `json:"on_consume"`
}

// ConnectionInitPolicies defines policies for connection initialization
type ConnectionInitPolicies struct {
	Request  []PolicyEntry `json:"request"`
	Response []PolicyEntry `json:"response"`
}

// ChannelEntry represents one channel in the EventChannelConfig.
type ChannelEntry struct {
	Name     string        `json:"name"`
	Policies PoliciesEntry `json:"policies"`
}

// ReceiverEntry specifies the receiver type.
type ReceiverEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// BrokerDriverEntry specifies the broker driver configuration.
type BrokerDriverEntry struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// PoliciesEntry holds the 3-phase policy references.
type PoliciesEntry struct {
	Subscribe []PolicyEntry `json:"subscribe"`
	Inbound   []PolicyEntry `json:"inbound"`
	Outbound  []PolicyEntry `json:"outbound"`
}

// PolicyEntry represents one policy reference.
type PolicyEntry struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Params  map[string]interface{} `json:"params"`
}

// Handler processes EventChannelConfig xDS responses and manages bindings.
type Handler struct {
	manager     BindingManager
	kafkaConfig KafkaConfig
	mu          sync.Mutex
	current     map[string]EventChannelResource // keyed by UUID
}

// NewHandler creates a new event channel handler.
// kafkaCfg provides the local Kafka brokers used when the xDS resource omits broker config.
func NewHandler(manager BindingManager, kafkaCfg KafkaConfig) *Handler {
	return &Handler{
		manager:     manager,
		kafkaConfig: kafkaCfg,
		current:     make(map[string]EventChannelResource),
	}
}

// HandleResources processes an EventChannelConfig xDS update.
// It diffs against the previous state and adds/removes bindings accordingly.
func (h *Handler) HandleResources(ctx context.Context, resources []*discoveryv3.Resource, version string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Decode incoming resources
	incoming := make(map[string]EventChannelResource)
	for _, res := range resources {
		if res.Resource == nil {
			continue
		}

		// The xDS server wraps cached *anypb.Any resources in another Any,
		// so we unwrap: outer Any -> inner Any -> Struct.
		innerAny := &anypb.Any{}
		if err := proto.Unmarshal(res.Resource.Value, innerAny); err != nil {
			slog.Error("Failed to unmarshal EventChannelConfig resource", "error", err)
			continue
		}

		s := &structpb.Struct{}
		if err := proto.Unmarshal(innerAny.Value, s); err != nil {
			slog.Error("Failed to unmarshal EventChannelConfig struct", "error", err)
			continue
		}

		data, err := json.Marshal(s.AsMap())
		if err != nil {
			slog.Error("Failed to marshal struct to JSON", "error", err)
			continue
		}

		slog.Info("DEBUG: Raw JSON from xDS", "json", string(data))

		var ecr EventChannelResource
		if err := json.Unmarshal(data, &ecr); err != nil {
			slog.Error("Failed to decode EventChannelConfig", "error", err)
			continue
		}

		slog.Info("DEBUG: Deserialized EventChannelResource",
			"uuid", ecr.UUID,
			"name", ecr.Name,
			"kind", ecr.Kind,
			"brokerDriver", ecr.BrokerDriver)
		// Deletion markers are pushed by the controller to work around
		// a go-control-plane LinearCache limitation for custom type URLs.
		// Treat them as absent so the diff logic removes the binding.
		if ecr.Deleted {
			slog.Info("Received deletion marker via xDS", "uuid", ecr.UUID)
			continue
		}

		incoming[ecr.UUID] = ecr
	}

	// Compute diff: removals
	for uuid, old := range h.current {
		if _, exists := incoming[uuid]; !exists {
			slog.Info("Removing binding via xDS", "name", old.Name, "uuid", uuid, "kind", old.Kind)
			if err := h.removeBinding(old); err != nil {
				slog.Error("Failed to remove binding", "name", old.Name, "kind", old.Kind, "error", err)
			}
		}
	}

	// Compute diff: additions and updates
	for uuid, ecr := range incoming {
		if old, exists := h.current[uuid]; exists {
			if reflect.DeepEqual(old, ecr) {
				continue
			}
			// Update: remove then re-add
			slog.Info("Updating binding via xDS", "name", ecr.Name, "uuid", uuid, "kind", ecr.Kind)
			if err := h.removeBinding(old); err != nil {
				slog.Error("Failed to remove binding for update", "name", old.Name, "kind", old.Kind, "error", err)
			}
		} else {
			slog.Info("Adding binding via xDS", "name", ecr.Name, "uuid", uuid, "kind", ecr.Kind)
		}

		if err := h.addBinding(ecr); err != nil {
			return fmt.Errorf("failed to add binding %q: %w", ecr.Name, err)
		}
	}

	h.current = incoming
	return nil
}

func (h *Handler) toWebSubApiBinding(ecr EventChannelResource) binding.WebSubApiBinding {
	// For WebSubApi, Channels is []ChannelEntry
	var channelEntries []ChannelEntry
	if ecr.Channels != nil {
		// Try to decode as []ChannelEntry (for WebSubApi)
		if channelsSlice, ok := ecr.Channels.([]interface{}); ok {
			for _, chIface := range channelsSlice {
				if chMap, ok := chIface.(map[string]interface{}); ok {
					var ch ChannelEntry
					if name, ok := chMap["name"].(string); ok {
						ch.Name = name
					}
					if policiesIface, ok := chMap["policies"].(map[string]interface{}); ok {
						if subIface, ok := policiesIface["subscribe"].([]interface{}); ok {
							ch.Policies.Subscribe = mapGenericPolicyEntryList(subIface)
						}
						if inIface, ok := policiesIface["inbound"].([]interface{}); ok {
							ch.Policies.Inbound = mapGenericPolicyEntryList(inIface)
						}
						if outIface, ok := policiesIface["outbound"].([]interface{}); ok {
							ch.Policies.Outbound = mapGenericPolicyEntryList(outIface)
						}
					}
					channelEntries = append(channelEntries, ch)
				}
			}
		}
	}

	channels := make([]binding.ChannelDef, len(channelEntries))
	for i, ch := range channelEntries {
		channels[i] = binding.ChannelDef{
			Name: ch.Name,
			Policies: binding.PolicyBindings{
				Subscribe: mapPolicyEntries(ch.Policies.Subscribe),
				Inbound:   mapPolicyEntries(ch.Policies.Inbound),
				Outbound:  mapPolicyEntries(ch.Policies.Outbound),
			},
		}
	}

	// For WebSubApi, Policies is PoliciesEntry
	var policies PoliciesEntry
	if ecr.Policies != nil {
		if policiesMap, ok := ecr.Policies.(map[string]interface{}); ok {
			if subIface, ok := policiesMap["subscribe"].([]interface{}); ok {
				policies.Subscribe = mapGenericPolicyEntryList(subIface)
			}
			if inIface, ok := policiesMap["inbound"].([]interface{}); ok {
				policies.Inbound = mapGenericPolicyEntryList(inIface)
			}
			if outIface, ok := policiesMap["outbound"].([]interface{}); ok {
				policies.Outbound = mapGenericPolicyEntryList(outIface)
			}
		}
	}

	subscribe := mapPolicyEntries(policies.Subscribe)
	inbound := mapPolicyEntries(policies.Inbound)
	outbound := mapPolicyEntries(policies.Outbound)

	return binding.WebSubApiBinding{
		Kind:     "WebSubApi",
		APIID:    ecr.UUID,
		Name:     ecr.Name,
		Version:  ecr.Version,
		Context:  ecr.Context,
		Channels: channels,
		Receiver: binding.ReceiverSpec{
			Type: ecr.Receiver.Type,
		},
		BrokerDriver: h.resolveBrokerDriver(ecr.BrokerDriver),
		Policies: binding.PolicyBindings{
			Subscribe: subscribe,
			Inbound:   inbound,
			Outbound:  outbound,
		},
	}
}

// mapGenericPolicyEntryList converts []interface{} to []PolicyEntry
func mapGenericPolicyEntryList(policies []interface{}) []PolicyEntry {
	result := make([]PolicyEntry, 0, len(policies))
	for _, pIface := range policies {
		if pMap, ok := pIface.(map[string]interface{}); ok {
			policyEntry := PolicyEntry{}
			if name, ok := pMap["name"].(string); ok {
				policyEntry.Name = name
			}
			if version, ok := pMap["version"].(string); ok {
				policyEntry.Version = version
			}
			if params, ok := pMap["params"].(map[string]interface{}); ok {
				policyEntry.Params = params
			}
			result = append(result, policyEntry)
		}
	}
	return result
}

// mapPolicyEntries converts a slice of PolicyEntry to binding.PolicyRef.
func mapPolicyEntries(entries []PolicyEntry) []binding.PolicyRef {
	if len(entries) == 0 {
		return nil
	}
	refs := make([]binding.PolicyRef, len(entries))
	for i, p := range entries {
		refs[i] = binding.PolicyRef{Name: p.Name, Version: p.Version, Params: p.Params}
	}
	return refs
}

// resolveBrokerDriver returns a BrokerDriverSpec, falling back to the local
// Kafka configuration when the xDS resource doesn't carry broker details.
func (h *Handler) resolveBrokerDriver(bd BrokerDriverEntry) binding.BrokerDriverSpec {
	driverType := bd.Type
	if driverType == "" {
		driverType = "kafka"
	}

	cfg := bd.Properties
	if len(cfg) == 0 {
		// Use the event gateway's own Kafka brokers.
		cfg = map[string]interface{}{
			"brokers": h.kafkaConfig.Brokers,
		}
	}

	return binding.BrokerDriverSpec{
		Name:   bd.Name,
		Type:   driverType,
		Config: cfg,
	}
}

// addBinding routes to the appropriate manager method based on Kind.
func (h *Handler) addBinding(ecr EventChannelResource) error {
	switch ecr.Kind {
	case "WebSubApi":
		wsb := h.toWebSubApiBinding(ecr)
		return h.manager.AddWebSubApiBinding(wsb)
	case "WebBrokerApi":
		wbb := h.toWebBrokerApiBinding(ecr)
		return h.manager.AddWebBrokerApiBinding(wbb)
	default:
		return fmt.Errorf("unsupported kind: %s", ecr.Kind)
	}
}

// removeBinding routes to the appropriate manager method based on Kind.
func (h *Handler) removeBinding(ecr EventChannelResource) error {
	switch ecr.Kind {
	case "WebSubApi":
		return h.manager.RemoveWebSubApiBinding(ecr.Name)
	case "WebBrokerApi":
		return h.manager.RemoveWebBrokerApiBinding(ecr.Name)
	default:
		return fmt.Errorf("unsupported kind: %s", ecr.Kind)
	}
}

// toWebBrokerApiBinding converts EventChannelResource to WebBrokerApiBinding.
func (h *Handler) toWebBrokerApiBinding(ecr EventChannelResource) binding.WebBrokerApiBinding {
	// Parse API-level policies from Policies field (interface{})
	var apiPolicies binding.ProtocolMediationPolicies
	if ecr.Policies != nil {
		if policiesMap, ok := ecr.Policies.(map[string]interface{}); ok {
			// Parse on_connection_init
			if connInitIface, ok := policiesMap["on_connection_init"].(map[string]interface{}); ok {
				if reqIface, ok := connInitIface["request"].([]interface{}); ok {
					apiPolicies.OnConnectionInit.Request = mapGenericPolicyList(reqIface)
				}
				if respIface, ok := connInitIface["response"].([]interface{}); ok {
					apiPolicies.OnConnectionInit.Response = mapGenericPolicyList(respIface)
				}
			}
			// Parse on_produce
			if produceIface, ok := policiesMap["on_produce"].([]interface{}); ok {
				apiPolicies.OnProduce = mapGenericPolicyList(produceIface)
			}
			// Parse on_consume
			if consumeIface, ok := policiesMap["on_consume"].([]interface{}); ok {
				apiPolicies.OnConsume = mapGenericPolicyList(consumeIface)
			}
		}
	}

	// Parse channels map from Channels field (interface{})
	channels := make(map[string]binding.WebBrokerChannelDef)
	if ecr.Channels != nil {
		if channelsMap, ok := ecr.Channels.(map[string]interface{}); ok {
			for channelName, channelIface := range channelsMap {
				if channelData, ok := channelIface.(map[string]interface{}); ok {
					var channelDef binding.WebBrokerChannelDef

					// Parse produce_to
					if produceToIface, ok := channelData["produce_to"].(map[string]interface{}); ok {
						if topic, ok := produceToIface["topic"].(string); ok && topic != "" {
							channelDef.ProduceTo = &binding.TopicMapping{Topic: topic}
						}
					}

					// Parse consume_from
					if consumeFromIface, ok := channelData["consume_from"].(map[string]interface{}); ok {
						if topic, ok := consumeFromIface["topic"].(string); ok && topic != "" {
							channelDef.ConsumeFrom = &binding.TopicMapping{Topic: topic}
						}
					}

					// Parse policies from nested "policies" field
					if policiesIface, ok := channelData["policies"].(map[string]interface{}); ok {
						// Parse channel on_connection_init
						if connInitIface, ok := policiesIface["on_connection_init"].(map[string]interface{}); ok {
							if reqIface, ok := connInitIface["request"].([]interface{}); ok {
								channelDef.OnConnectionInit.Request = mapGenericPolicyList(reqIface)
							}
							if respIface, ok := connInitIface["response"].([]interface{}); ok {
								channelDef.OnConnectionInit.Response = mapGenericPolicyList(respIface)
							}
						}
						// Parse channel on_produce
						if produceIface, ok := policiesIface["on_produce"].([]interface{}); ok {
							channelDef.OnProduce = mapGenericPolicyList(produceIface)
						}
						// Parse channel on_consume
						if consumeIface, ok := policiesIface["on_consume"].([]interface{}); ok {
							channelDef.OnConsume = mapGenericPolicyList(consumeIface)
						}
					}

					channels[channelName] = channelDef
				}
			}
		}
	}

	return binding.WebBrokerApiBinding{
		Kind:    "WebBrokerApi",
		APIID:   ecr.UUID,
		Name:    ecr.Name,
		Version: ecr.Version,
		Context: ecr.Context,
		Receiver: binding.ReceiverSpec{
			Name: ecr.Receiver.Name,
			Type: ecr.Receiver.Type,
		},
		BrokerDriver: h.resolveBrokerDriver(ecr.BrokerDriver),
		Policies:     apiPolicies,
		Channels:     channels,
	}
}

// mapGenericPolicyList converts []interface{} to []binding.PolicyRef
func mapGenericPolicyList(policies []interface{}) []binding.PolicyRef {
	result := make([]binding.PolicyRef, 0, len(policies))
	for _, pIface := range policies {
		if pMap, ok := pIface.(map[string]interface{}); ok {
			policyRef := binding.PolicyRef{}
			if name, ok := pMap["name"].(string); ok {
				policyRef.Name = name
			}
			if version, ok := pMap["version"].(string); ok {
				policyRef.Version = version
			}
			if params, ok := pMap["params"].(map[string]interface{}); ok {
				policyRef.Params = params
			}
			result = append(result, policyRef)
		}
	}
	return result
}
