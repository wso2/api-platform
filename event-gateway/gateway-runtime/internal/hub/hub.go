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

package hub

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
)

// ChannelChainKeySet holds the per-channel policy chain keys for a single channel.
type ChannelChainKeySet struct {
	SubscribeChainKey string
	InboundChainKey   string
	OutboundChainKey  string
}

// ChannelBinding holds the runtime state for a registered channel.
type ChannelBinding struct {
	APIID               string
	Name                string
	Mode                string // "websub" or "protocol-mediation"
	Context             string
	Version             string
	Vhost               string
	SubscribeChainKey   string
	InboundChainKey     string
	OutboundChainKey    string
	BrokerDriverTopic   string
	Ordering            string                        // "ordered" or "unordered"
	Channels            map[string]string             // channel-name → Kafka topic (WebSubApi only)
	KafkaTopicToChannel map[string]string             // Kafka topic → channel-name (reverse of Channels, cached)
	ChannelChainKeys    map[string]ChannelChainKeySet // channel-name → per-channel chain keys
}

// Hub is the central message router. It holds the policy engine reference and
// routes messages between receiver and broker-driver connectors.
type Hub struct {
	mu       sync.RWMutex
	engine   *engine.Engine
	bindings map[string]*ChannelBinding // keyed by binding name
}

// NewHub creates a new Hub with the given policy engine.
func NewHub(eng *engine.Engine) *Hub {
	return &Hub{
		engine:   eng,
		bindings: make(map[string]*ChannelBinding),
	}
}

// RegisterBinding adds a channel binding to the hub.
// It automatically builds the KafkaTopicToChannel reverse map from b.Channels.
func (h *Hub) RegisterBinding(b ChannelBinding) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(b.Channels) > 0 {
		b.KafkaTopicToChannel = make(map[string]string, len(b.Channels))
		for channelName, topic := range b.Channels {
			b.KafkaTopicToChannel[topic] = channelName
		}
	}
	h.bindings[b.Name] = &b
}

// RemoveBinding removes a binding by name.
func (h *Hub) RemoveBinding(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.bindings, name)
}

// GetBinding returns the binding for the given name, or nil.
func (h *Hub) GetBinding(name string) *ChannelBinding {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.bindings[name]
}

// GetBindingByTopic returns the first binding whose broker-driver topic matches.
func (h *Hub) GetBindingByTopic(topic string) *ChannelBinding {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, b := range h.bindings {
		if b.BrokerDriverTopic == topic {
			return b
		}
	}
	return nil
}

// AllBindings returns a snapshot of all registered bindings.
func (h *Hub) AllBindings() []*ChannelBinding {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]*ChannelBinding, 0, len(h.bindings))
	for _, b := range h.bindings {
		result = append(result, b)
	}
	return result
}

// Engine returns the policy engine used by this hub.
func (h *Hub) Engine() *engine.Engine {
	return h.engine
}

// ProcessSubscribe applies subscribe policies to a subscription request at the hub.
// Hub-level policies are applied first, then per-channel policies if present.
// Returns the (possibly mutated) message and whether it was short-circuited.
func (h *Hub) ProcessSubscribe(ctx context.Context, bindingName string, msg *connectors.Message) (*connectors.Message, bool, error) {
	binding := h.GetBinding(bindingName)
	if binding == nil {
		return nil, false, fmt.Errorf("binding not found: %s", bindingName)
	}

	// Apply hub-level subscribe chain first.
	if binding.SubscribeChainKey != "" {
		chain := h.engine.GetChain(binding.SubscribeChainKey)
		if chain != nil {
			reqHeaderCtx := SubscribeToRequestHeaderContext(msg, binding)
			result, err := h.engine.ExecuteRequestHeaderPolicies(ctx, binding.SubscribeChainKey, reqHeaderCtx.SharedContext, reqHeaderCtx)
			if err != nil {
				return nil, false, fmt.Errorf("subscribe header policy execution failed: %w", err)
			}
			if result.ShortCircuited {
				logShortCircuit("Subscribe request short-circuited by hub policy", bindingName, binding.SubscribeChainKey, result.ImmediateResponse)
				return immediateResponseToMessage(result.ImmediateResponse), true, nil
			}
			if err := ApplyRequestHeaderResult(result, msg); err != nil {
				return nil, false, fmt.Errorf("failed to apply subscribe header result: %w", err)
			}
		}
	}

	// Apply channel-level subscribe chain if present.
	if msg.Topic != "" && len(binding.ChannelChainKeys) > 0 {
		if keys, ok := binding.ChannelChainKeys[msg.Topic]; ok && keys.SubscribeChainKey != "" {
			chain := h.engine.GetChain(keys.SubscribeChainKey)
			if chain != nil {
				reqHeaderCtx := SubscribeToRequestHeaderContext(msg, binding)
				result, err := h.engine.ExecuteRequestHeaderPolicies(ctx, keys.SubscribeChainKey, reqHeaderCtx.SharedContext, reqHeaderCtx)
				if err != nil {
					return nil, false, fmt.Errorf("subscribe channel policy execution failed: %w", err)
				}
				if result.ShortCircuited {
					logShortCircuit("Subscribe request short-circuited by channel policy", bindingName, keys.SubscribeChainKey, result.ImmediateResponse)
					return immediateResponseToMessage(result.ImmediateResponse), true, nil
				}
				if err := ApplyRequestHeaderResult(result, msg); err != nil {
					return nil, false, fmt.Errorf("failed to apply subscribe channel header result: %w", err)
				}
			}
		}
	}

	return msg, false, nil
}

// ProcessInbound applies inbound policies to a message flowing from entrypoint to endpoint.
// Hub-level policies are applied first, then per-channel policies if present.
// Returns the (possibly mutated) message and whether it was short-circuited.
func (h *Hub) ProcessInbound(ctx context.Context, bindingName string, msg *connectors.Message) (*connectors.Message, bool, error) {
	binding := h.GetBinding(bindingName)
	if binding == nil {
		return nil, false, fmt.Errorf("binding not found: %s", bindingName)
	}

	// Apply hub-level inbound chain first.
	if binding.InboundChainKey != "" {
		chain := h.engine.GetChain(binding.InboundChainKey)
		if chain != nil {
			reqHeaderCtx := MessageToRequestHeaderContext(msg, binding)
			result, err := h.engine.ExecuteRequestHeaderPolicies(ctx, binding.InboundChainKey, reqHeaderCtx.SharedContext, reqHeaderCtx)
			if err != nil {
				return nil, false, fmt.Errorf("inbound header policy execution failed: %w", err)
			}
			if result.ShortCircuited {
				logShortCircuit("Inbound message short-circuited by hub policy", bindingName, binding.InboundChainKey, result.ImmediateResponse)
				return immediateResponseToMessage(result.ImmediateResponse), true, nil
			}
			if err := ApplyRequestHeaderResult(result, msg); err != nil {
				return nil, false, fmt.Errorf("failed to apply inbound header result: %w", err)
			}
			if chain.RequiresRequestBody {
				reqCtx := MessageToRequestContext(msg, binding)
				bodyResult, err := h.engine.ExecuteRequestBodyPolicies(ctx, binding.InboundChainKey, reqCtx.SharedContext, reqCtx)
				if err != nil {
					return nil, false, fmt.Errorf("inbound body policy execution failed: %w", err)
				}
				if bodyResult.ShortCircuited {
					return immediateResponseToMessage(bodyResult.ImmediateResponse), true, nil
				}
				if err := ApplyRequestBodyResult(bodyResult, msg); err != nil {
					return nil, false, fmt.Errorf("failed to apply inbound body result: %w", err)
				}
			}
		}
	}

	// Apply channel-level inbound chain if present.
	if msg.Topic != "" && len(binding.ChannelChainKeys) > 0 {
		if keys, ok := binding.ChannelChainKeys[msg.Topic]; ok && keys.InboundChainKey != "" {
			chain := h.engine.GetChain(keys.InboundChainKey)
			if chain != nil {
				reqHeaderCtx := MessageToRequestHeaderContext(msg, binding)
				result, err := h.engine.ExecuteRequestHeaderPolicies(ctx, keys.InboundChainKey, reqHeaderCtx.SharedContext, reqHeaderCtx)
				if err != nil {
					return nil, false, fmt.Errorf("inbound channel policy execution failed: %w", err)
				}
				if result.ShortCircuited {
					logShortCircuit("Inbound message short-circuited by channel policy", bindingName, keys.InboundChainKey, result.ImmediateResponse)
					return immediateResponseToMessage(result.ImmediateResponse), true, nil
				}
				if err := ApplyRequestHeaderResult(result, msg); err != nil {
					return nil, false, fmt.Errorf("failed to apply inbound channel header result: %w", err)
				}
				if chain.RequiresRequestBody {
					reqCtx := MessageToRequestContext(msg, binding)
					bodyResult, err := h.engine.ExecuteRequestBodyPolicies(ctx, keys.InboundChainKey, reqCtx.SharedContext, reqCtx)
					if err != nil {
						return nil, false, fmt.Errorf("inbound channel body policy execution failed: %w", err)
					}
					if bodyResult.ShortCircuited {
						return immediateResponseToMessage(bodyResult.ImmediateResponse), true, nil
					}
					if err := ApplyRequestBodyResult(bodyResult, msg); err != nil {
						return nil, false, fmt.Errorf("failed to apply inbound channel body result: %w", err)
					}
				}
			}
		}
	}

	return msg, false, nil
}

// ProcessOutbound applies outbound policies to a message flowing from endpoint to entrypoint.
// Hub-level policies are applied first, then per-channel policies if present.
// Returns the (possibly mutated) message and whether it was short-circuited.
func (h *Hub) ProcessOutbound(ctx context.Context, bindingName string, msg *connectors.Message) (*connectors.Message, bool, error) {
	binding := h.GetBinding(bindingName)
	if binding == nil {
		return nil, false, fmt.Errorf("binding not found: %s", bindingName)
	}

	// Apply hub-level outbound chain first.
	if binding.OutboundChainKey != "" {
		chain := h.engine.GetChain(binding.OutboundChainKey)
		if chain != nil {
			reqHeaderCtx := MessageToRequestHeaderContext(msg, binding)
			result, err := h.engine.ExecuteRequestHeaderPolicies(ctx, binding.OutboundChainKey, reqHeaderCtx.SharedContext, reqHeaderCtx)
			if err != nil {
				return nil, false, fmt.Errorf("outbound header policy execution failed: %w", err)
			}
			if result.ShortCircuited {
				logShortCircuit("Outbound message short-circuited by hub policy", bindingName, binding.OutboundChainKey, result.ImmediateResponse)
				return nil, true, nil
			}
			if err := ApplyRequestHeaderResult(result, msg); err != nil {
				return nil, false, fmt.Errorf("failed to apply outbound header result: %w", err)
			}
			if chain.RequiresRequestBody {
				reqCtx := MessageToRequestContext(msg, binding)
				bodyResult, err := h.engine.ExecuteRequestBodyPolicies(ctx, binding.OutboundChainKey, reqCtx.SharedContext, reqCtx)
				if err != nil {
					return nil, false, fmt.Errorf("outbound body policy execution failed: %w", err)
				}
				if bodyResult.ShortCircuited {
					return nil, true, nil
				}
				if err := ApplyRequestBodyResult(bodyResult, msg); err != nil {
					return nil, false, fmt.Errorf("failed to apply outbound body result: %w", err)
				}
			}
		}
	}

	// Apply channel-level outbound chain if present.
	// msg.Topic here is the Kafka topic; resolve back to channel name.
	if msg.Topic != "" && len(binding.ChannelChainKeys) > 0 {
		channelName := resolveChannelName(binding.KafkaTopicToChannel, binding.Channels, msg.Topic)
		if channelName != "" {
			if keys, ok := binding.ChannelChainKeys[channelName]; ok && keys.OutboundChainKey != "" {
				chain := h.engine.GetChain(keys.OutboundChainKey)
				if chain != nil {
					reqHeaderCtx := MessageToRequestHeaderContext(msg, binding)
					result, err := h.engine.ExecuteRequestHeaderPolicies(ctx, keys.OutboundChainKey, reqHeaderCtx.SharedContext, reqHeaderCtx)
					if err != nil {
						return nil, false, fmt.Errorf("outbound channel policy execution failed: %w", err)
					}
					if result.ShortCircuited {
						logShortCircuit("Outbound message short-circuited by channel policy", bindingName, keys.OutboundChainKey, result.ImmediateResponse)
						return nil, true, nil
					}
					if err := ApplyRequestHeaderResult(result, msg); err != nil {
						return nil, false, fmt.Errorf("failed to apply outbound channel header result: %w", err)
					}
					if chain.RequiresRequestBody {
						reqCtx := MessageToRequestContext(msg, binding)
						bodyResult, err := h.engine.ExecuteRequestBodyPolicies(ctx, keys.OutboundChainKey, reqCtx.SharedContext, reqCtx)
						if err != nil {
							return nil, false, fmt.Errorf("outbound channel body policy execution failed: %w", err)
						}
						if bodyResult.ShortCircuited {
							return nil, true, nil
						}
						if err := ApplyRequestBodyResult(bodyResult, msg); err != nil {
							return nil, false, fmt.Errorf("failed to apply outbound channel body result: %w", err)
						}
					}
				}
			}
		}
	}

	return msg, false, nil
}

// resolveChannelName reverse-maps a Kafka topic name to the channel name.
// It first checks the pre-built kafkaTopicToChannel reverse map for an O(1) lookup,
// and falls back to an O(n) scan over channels only if the cached map is absent.
func resolveChannelName(kafkaTopicToChannel map[string]string, channels map[string]string, kafkaTopic string) string {
	if len(kafkaTopicToChannel) > 0 {
		return kafkaTopicToChannel[kafkaTopic]
	}
	for channelName, topic := range channels {
		if topic == kafkaTopic {
			return channelName
		}
	}
	return ""
}

// immediateResponseToMessage encodes an ImmediateResponseResult from the policy engine
// into a connectors.Message so callers can write the policy-provided HTTP response.
// The status code is stored in Metadata["status_code"] (int), body in Value,
// and response headers (single-value) in Headers ([]string slice per key).
// Returns nil when resp is nil.
func immediateResponseToMessage(resp *engine.ImmediateResponseResult) *connectors.Message {
	if resp == nil {
		return nil
	}
	headers := make(map[string][]string, len(resp.Headers))
	for k, v := range resp.Headers {
		headers[k] = []string{v}
	}
	return &connectors.Message{
		Value:   resp.Body,
		Headers: headers,
		Metadata: map[string]interface{}{
			"status_code": resp.StatusCode,
		},
	}
}

// logShortCircuit keeps Info logs to metadata only; ImmediateResponse.Body is
// user-visible content and must not contain sensitive information.
func logShortCircuit(message, bindingName, chainKey string, resp *engine.ImmediateResponseResult) {
	attrs := []any{
		"binding", bindingName,
		"chain", chainKey,
	}
	if resp != nil {
		attrs = append(attrs,
			"status_code", resp.StatusCode,
			"response_length", len(resp.Body),
		)
	}
	slog.Info(message, attrs...)
	if resp != nil {
		bodyAttrs := append([]any{}, attrs...)
		bodyAttrs = append(bodyAttrs, "response_body", summarizeImmediateResponseBody(resp.Body))
		slog.Debug("Short-circuit immediate response body", bodyAttrs...)
	}
}

func summarizeImmediateResponseBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	const maxLen = 256
	if len(text) > maxLen {
		return text[:maxLen] + "..."
	}
	return text
}
