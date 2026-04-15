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

package connectors

import (
	"context"
	"net/http"
)

// Message represents an event flowing through the event gateway.
type Message struct {
	Key      []byte
	Value    []byte
	Headers  map[string][]string
	Topic    string
	Metadata map[string]interface{}
}

// MessageHandler is a callback invoked when a message is received.
type MessageHandler func(ctx context.Context, msg *Message) error

// Entrypoint is a client-facing protocol adapter with a managed lifecycle.
type Entrypoint interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// MessageProcessor applies policy chains to messages flowing through the gateway.
// Implemented by the hub; consumed by entrypoints via dependency injection.
type MessageProcessor interface {
	ProcessSubscribe(ctx context.Context, bindingName string, msg *Message) (*Message, bool, error)
	ProcessInbound(ctx context.Context, bindingName string, msg *Message) (*Message, bool, error)
	ProcessOutbound(ctx context.Context, bindingName string, msg *Message) (*Message, bool, error)
}

// Endpoint manages connections to a backend event system (e.g. Kafka, NATS).
type Endpoint interface {
	Publish(ctx context.Context, topic string, msg *Message) error
	Subscribe(groupID string, topics []string, handler MessageHandler) (Entrypoint, error)
	TopicExists(ctx context.Context, topic string) (bool, error)
	EnsureTopics(ctx context.Context, topics []string) error
	Close() error
}

// ChannelInfo is the read-only view of a channel binding passed to entrypoints.
// It contains only the information entrypoints need — no policy chain keys.
type ChannelInfo struct {
	Name             string
	Mode             string
	Context          string
	Version          string
	Vhost            string
	PublicTopic      string
	EndpointTopic    string
	Ordering         string
	Channels         map[string]string // channel-name → Kafka topic (WebSubApi only)
	InternalSubTopic string            // internal subscription sync topic (WebSubApi only)
}

// EntrypointConfig holds the dependencies injected into an entrypoint factory.
// Each entrypoint handles a single channel (API) with its own endpoint connection.
type EntrypointConfig struct {
	Channel   ChannelInfo
	Processor MessageProcessor
	Endpoint  Endpoint
	RuntimeID string
	Mux       *http.ServeMux // shared HTTP mux for port sharing (owned by runtime)
}
