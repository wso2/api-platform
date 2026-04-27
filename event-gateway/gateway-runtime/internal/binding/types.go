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
	Name string `yaml:"name"`
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

// PolicyBindings holds subscribe, inbound, and outbound policy configurations.
//   - Subscribe: applied when a client subscribes or unsubscribes at the hub.
//   - Inbound:   applied when an event is published via the webhook receiver (data ingress).
//   - Outbound:  applied when an event is delivered to a subscriber callback (data delivery).
type PolicyBindings struct {
	Subscribe []PolicyRef `yaml:"subscribe"`
	Inbound   []PolicyRef `yaml:"inbound"`
	Outbound  []PolicyRef `yaml:"outbound"`
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

// WebSubApiTopicName derives a Kafka topic name for a WebSubApi channel.
// Format: {api-name}.{version}.{channel-name}
func WebSubApiTopicName(apiName, version, channelName string) string {
	return apiName + "." + version + "." + channelName
}

// WebSubApiSubscriptionTopic derives the internal subscription sync topic for a WebSubApi.
// Format: {api-name}.{version}.__subscriptions
func WebSubApiSubscriptionTopic(apiName, version string) string {
	return apiName + "." + version + ".__subscriptions"
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
