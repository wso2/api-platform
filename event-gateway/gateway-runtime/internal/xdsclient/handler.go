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

// BindingManager can add/remove WebSubApi bindings dynamically.
type BindingManager interface {
	AddWebSubApiBinding(wsb binding.WebSubApiBinding) error
	RemoveWebSubApiBinding(name string) error
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
	Channels     []ChannelEntry    `json:"channels"`
	Receiver     ReceiverEntry     `json:"receiver"`
	BrokerDriver BrokerDriverEntry `json:"brokerDriver"`
	Policies     PoliciesEntry     `json:"policies"`
}

// ChannelEntry represents one channel in the EventChannelConfig.
type ChannelEntry struct {
	Name string `json:"name"`
}

// ReceiverEntry specifies the receiver type.
type ReceiverEntry struct {
	Type string `json:"type"`
}

// BrokerDriverEntry specifies the broker driver configuration.
type BrokerDriverEntry struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
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

		var ecr EventChannelResource
		if err := json.Unmarshal(data, &ecr); err != nil {
			slog.Error("Failed to decode EventChannelConfig", "error", err)
			continue
		}

		if ecr.UUID == "" {
			slog.Warn("EventChannelConfig resource missing UUID, skipping")
			continue
		}

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
			slog.Info("Removing binding via xDS", "name", old.Name, "uuid", uuid)
			if err := h.manager.RemoveWebSubApiBinding(old.Name); err != nil {
				slog.Error("Failed to remove binding", "name", old.Name, "error", err)
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
			slog.Info("Updating binding via xDS", "name", ecr.Name, "uuid", uuid)
			if err := h.manager.RemoveWebSubApiBinding(old.Name); err != nil {
				slog.Error("Failed to remove binding for update", "name", old.Name, "error", err)
			}
		} else {
			slog.Info("Adding binding via xDS", "name", ecr.Name, "uuid", uuid)
		}

		wsb := h.toWebSubApiBinding(ecr)
		if err := h.manager.AddWebSubApiBinding(wsb); err != nil {
			return fmt.Errorf("failed to add binding %q: %w", ecr.Name, err)
		}
	}

	h.current = incoming
	return nil
}

func (h *Handler) toWebSubApiBinding(ecr EventChannelResource) binding.WebSubApiBinding {
	channels := make([]binding.ChannelDef, len(ecr.Channels))
	for i, ch := range ecr.Channels {
		channels[i] = binding.ChannelDef{Name: ch.Name}
	}

	subscribe := make([]binding.PolicyRef, len(ecr.Policies.Subscribe))
	for i, p := range ecr.Policies.Subscribe {
		subscribe[i] = binding.PolicyRef{Name: p.Name, Version: p.Version, Params: p.Params}
	}

	inbound := make([]binding.PolicyRef, len(ecr.Policies.Inbound))
	for i, p := range ecr.Policies.Inbound {
		inbound[i] = binding.PolicyRef{Name: p.Name, Version: p.Version, Params: p.Params}
	}

	outbound := make([]binding.PolicyRef, len(ecr.Policies.Outbound))
	for i, p := range ecr.Policies.Outbound {
		outbound[i] = binding.PolicyRef{Name: p.Name, Version: p.Version, Params: p.Params}
	}

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

// resolveBrokerDriver returns a BrokerDriverSpec, falling back to the local
// Kafka configuration when the xDS resource doesn't carry broker details.
func (h *Handler) resolveBrokerDriver(bd BrokerDriverEntry) binding.BrokerDriverSpec {
	driverType := bd.Type
	if driverType == "" {
		driverType = "kafka"
	}

	cfg := bd.Config
	if len(cfg) == 0 {
		// Use the event gateway's own Kafka brokers.
		cfg = map[string]interface{}{
			"brokers": h.kafkaConfig.Brokers,
		}
	}

	return binding.BrokerDriverSpec{
		Type:   driverType,
		Config: cfg,
	}
}
