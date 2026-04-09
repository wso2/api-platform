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

package kafka

import (
	"context"
	"fmt"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// KafkaEndpoint implements connectors.Endpoint for Apache Kafka.
// It owns a shared publisher and creates consumers on demand.
type KafkaEndpoint struct {
	publisher *Publisher
	brokers   []string
}

// NewEndpoint creates a Kafka endpoint backed by the given brokers.
func NewEndpoint(brokers []string) (*KafkaEndpoint, error) {
	pub, err := NewPublisher(brokers)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka publisher: %w", err)
	}
	return &KafkaEndpoint{publisher: pub, brokers: brokers}, nil
}

// Publish sends a message to the given Kafka topic.
func (e *KafkaEndpoint) Publish(ctx context.Context, topic string, msg *connectors.Message) error {
	return e.publisher.Publish(ctx, topic, msg)
}

// Subscribe creates a consumer for the given topics using a shared consumer group.
// The returned Entrypoint must be Start()ed by the caller.
func (e *KafkaEndpoint) Subscribe(groupID string, topics []string, handler connectors.MessageHandler) (connectors.Entrypoint, error) {
	return NewConsumer(e.brokers, groupID, topics, handler)
}

// Close shuts down the shared publisher.
func (e *KafkaEndpoint) Close() error {
	e.publisher.Close()
	return nil
}
