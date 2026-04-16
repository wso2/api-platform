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
	"log/slog"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// KafkaBrokerDriver implements connectors.BrokerDriver for Apache Kafka.
// It owns a shared publisher and creates consumers on demand.
type KafkaBrokerDriver struct {
	publisher *Publisher
	brokers   []string
	admin     *kadm.Client
	adminKgo  *kgo.Client
}

// NewBrokerDriver creates a Kafka broker-driver backed by the given brokers.
func NewBrokerDriver(brokers []string) (*KafkaBrokerDriver, error) {
	pub, err := NewPublisher(brokers)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka publisher: %w", err)
	}

	adminKgo, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		pub.Close()
		return nil, fmt.Errorf("failed to create kafka admin client: %w", err)
	}

	return &KafkaBrokerDriver{
		publisher: pub,
		brokers:   brokers,
		admin:     kadm.NewClient(adminKgo),
		adminKgo:  adminKgo,
	}, nil
}

// Publish sends a message to the given Kafka topic.
func (e *KafkaBrokerDriver) Publish(ctx context.Context, topic string, msg *connectors.Message) error {
	return e.publisher.Publish(ctx, topic, msg)
}

// Subscribe creates a consumer for the given topics using a shared consumer group.
// The returned Receiver must be Start()ed by the caller.
func (e *KafkaBrokerDriver) Subscribe(groupID string, topics []string, handler connectors.MessageHandler) (connectors.Receiver, error) {
	return NewConsumer(e.brokers, groupID, topics, handler)
}

// TopicExists checks whether a topic exists in the Kafka cluster.
func (e *KafkaBrokerDriver) TopicExists(ctx context.Context, topic string) (bool, error) {
	topics, err := e.admin.ListTopics(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list topics: %w", err)
	}
	_, exists := topics[topic]
	return exists, nil
}

// EnsureTopics creates topics if they don't already exist (idempotent).
func (e *KafkaBrokerDriver) EnsureTopics(ctx context.Context, topics []string) error {
	resp, err := e.admin.CreateTopics(ctx, 1, 1, nil, topics...)
	if err != nil {
		return fmt.Errorf("failed to create topics: %w", err)
	}

	for _, t := range resp.Sorted() {
		if t.Err != nil {
			// "TOPIC_ALREADY_EXISTS" is not a real failure for idempotent creates.
			if isTopicAlreadyExistsErr(t.Err) {
				slog.Debug("Topic already exists", "topic", t.Topic)
				continue
			}
			return fmt.Errorf("failed to create topic %s: %w", t.Topic, t.Err)
		}
		slog.Info("Created topic", "topic", t.Topic)
	}

	return nil
}

// isTopicAlreadyExistsErr checks if the error indicates the topic already exists.
func isTopicAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "TOPIC_ALREADY_EXISTS") ||
		strings.Contains(err.Error(), "already exists")
}

// Close shuts down the shared publisher and admin client.
func (e *KafkaBrokerDriver) Close() error {
	e.publisher.Close()
	e.adminKgo.Close()
	return nil
}

// DeleteTopics deletes the given Kafka topics.
func (e *KafkaBrokerDriver) DeleteTopics(ctx context.Context, topics []string) error {
	resp, err := e.admin.DeleteTopics(ctx, topics...)
	if err != nil {
		return fmt.Errorf("failed to delete topics: %w", err)
	}
	for _, t := range resp.Sorted() {
		if t.Err != nil {
			slog.Warn("Failed to delete topic", "topic", t.Topic, "error", t.Err)
		} else {
			slog.Info("Deleted topic", "topic", t.Topic)
		}
	}
	return nil
}
