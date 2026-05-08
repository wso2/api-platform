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

package binding

import (
	"fmt"
	"path"
	"strings"
)

// Binding represents a configured channel with its receiver, broker-driver, and policy bindings.
// Used for protocol-mediation mode (1 channel = 1 topic).
type Binding struct {
	Kind         string           `yaml:"kind"`
	APIID        string           `yaml:"apiId"`
	Name         string           `yaml:"name"`
	Mode         string           `yaml:"mode"` // "websub" or "protocol-mediation"
	Context      string           `yaml:"context"`
	Version      string           `yaml:"version"`
	Vhost        string           `yaml:"vhost"`
	Receiver     ReceiverSpec     `yaml:"receiver"`
	BrokerDriver BrokerDriverSpec `yaml:"broker-driver"`
	Policies     PolicyBindings   `yaml:"policies"`
}

// WebSubApiBinding represents a WebSubApi with multiple channels (topics)
// sharing a single receiver and broker-driver.
type WebSubApiBinding struct {
	Kind         string           `yaml:"kind"` // "WebSubApi"
	APIID        string           `yaml:"apiId"`
	Name         string           `yaml:"name"`
	Version      string           `yaml:"version"`
	Context      string           `yaml:"context"`
	Vhost        string           `yaml:"vhost"`
	Channels     []ChannelDef     `yaml:"channels"`
	Receiver     ReceiverSpec     `yaml:"receiver"`
	BrokerDriver BrokerDriverSpec `yaml:"broker-driver"`
	Policies     PolicyBindings   `yaml:"policies"`
}

// ChannelDef defines a single channel (topic) within a WebSubApi.
type ChannelDef struct {
	Name     string         `yaml:"name"`
	Policies PolicyBindings `yaml:"policies"`
}

// ReceiverSpec defines the receiver connector type and configuration.
type ReceiverSpec struct {
	Type         string `yaml:"type"` // "websub" or "websocket"
	Path         string `yaml:"path"`
	Backpressure string `yaml:"backpressure"` // "drop-oldest", "block", "close"
}

// BrokerDriverSpec defines the broker-driver connector type and configuration.
type BrokerDriverSpec struct {
	Type     string                 `yaml:"type"` // "kafka"
	Topic    string                 `yaml:"topic"`
	Ordering string                 `yaml:"ordering"` // "ordered" or "unordered"
	Config   map[string]interface{} `yaml:"config"`   // broker-driver-specific config (e.g. brokers, tls)
}

// PolicyBindings holds subscribe, unsubscribe, inbound, and outbound policy configurations.
//   - Subscribe:   applied when a client subscribes at the hub (on_subscription).
//   - Unsubscribe: applied when a client unsubscribes at the hub (on_unsubscription).
//   - Inbound:     applied when an event is published via the webhook receiver (on_message_received).
//   - Outbound:    applied when an event is delivered to a subscriber callback (on_message_delivery).
type PolicyBindings struct {
	Subscribe   []PolicyRef `yaml:"subscribe"`
	Unsubscribe []PolicyRef `yaml:"unsubscribe"`
	Inbound     []PolicyRef `yaml:"inbound"`
	Outbound    []PolicyRef `yaml:"outbound"`
}

// PolicyRef references a policy to include in a chain.
type PolicyRef struct {
	Name    string                 `yaml:"name"`
	Version string                 `yaml:"version"`
	Params  map[string]interface{} `yaml:"params"`
}

// ChannelsConfig is the top-level structure of the channels.yaml file.
type ChannelsConfig struct {
	Channels []Binding `yaml:"channels"`
}

// JoinNormalizedTopic derives a Kafka topic name by normalizing each logical
// segment and joining them with underscores.
func JoinNormalizedTopic(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}

	normalizedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		normalizedParts = append(normalizedParts, NormalizeTopicSegment(part))
	}
	return strings.Join(normalizedParts, "_")
}

// WebSubApiTopicName derives a Kafka topic name for a WebSubApi channel.
// Format: {normalized-api-name}_{normalized-version}_{normalized-channel-name}
// The logical WebSub channel name remains unchanged elsewhere; only the broker topic is normalized.
func WebSubApiTopicName(apiName, version, channelName string) string {
	return JoinNormalizedTopic(apiName, version, channelName)
}

// WebSubApiSubscriptionTopic derives the internal subscription sync topic for a WebSubApi.
// Format: {normalized-api-name}_{normalized-version}_{normalized-subscription-suffix}
func WebSubApiSubscriptionTopic(apiName, version string) string {
	return JoinNormalizedTopic(apiName, version, "__subscriptions")
}

// WebSubApiBasePath derives the shared WebSub HTTP base path for an API.
// It accepts base contexts ("/repos"), version templates ("/repos/$version"),
// and already-resolved paths ("/repos/v1") without duplicating the version.
func WebSubApiBasePath(context, version string) string {
	trimmed := strings.TrimSpace(context)
	if trimmed == "" {
		if version == "" {
			return ""
		}
		return path.Join("/", version)
	}

	if strings.Contains(trimmed, "$version") {
		return ensureLeadingSlash(path.Clean(strings.ReplaceAll(trimmed, "$version", version)))
	}

	cleaned := ensureLeadingSlash(path.Clean(trimmed))
	if version == "" {
		return cleaned
	}

	versionSuffix := "/" + strings.TrimPrefix(version, "/")
	if strings.HasSuffix(cleaned, versionSuffix) {
		return cleaned
	}

	return path.Join(cleaned, version)
}

func ensureLeadingSlash(value string) string {
	if value == "" || value == "." {
		return "/"
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}

// NormalizeTopicSegment converts a logical topic segment to a Kafka-safe name.
// It uses an escape format so unsupported characters do not collide with
// already-valid names:
//   - [A-Za-z0-9.-] pass through unchanged
//   - '_' becomes '__'
//   - everything else becomes '_%x_' (for example '/' -> '_2f_')
func NormalizeTopicSegment(value string) string {
	if value == "" {
		return ""
	}

	var normalized strings.Builder
	normalized.Grow(len(value))

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			normalized.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			normalized.WriteRune(r)
		case r >= '0' && r <= '9':
			normalized.WriteRune(r)
		case r == '.', r == '-':
			normalized.WriteRune(r)
		case r == '_':
			normalized.WriteString("__")
		default:
			normalized.WriteString(fmt.Sprintf("_%x_", r))
		}
	}

	return normalized.String()
}
