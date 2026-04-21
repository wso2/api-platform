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

package websub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/brokerdriver/kafka"
)

// managedConsumer tracks a per-callback consumer and its topic set.
type managedConsumer struct {
	consumer connectors.Receiver
	topics   map[string]bool // set of Kafka topics this consumer reads
}

// ConsumerManager manages per-callback Kafka consumers for WebSub delivery.
// Each unique callback URL gets its own consumer group. When a callback's
// topic set changes (subscribe/unsubscribe), the consumer is recreated.
type ConsumerManager struct {
	mu          sync.Mutex
	consumers   map[string]*managedConsumer // callbackURL → managedConsumer
	brokers     []string
	groupPrefix string
	processor   connectors.MessageProcessor
	bindingName string
	deliverer   *Deliverer
	ctx         context.Context
}

// NewConsumerManager creates a new ConsumerManager.
func NewConsumerManager(
	brokers []string,
	groupPrefix string,
	processor connectors.MessageProcessor,
	bindingName string,
	deliverer *Deliverer,
) *ConsumerManager {
	return &ConsumerManager{
		consumers:   make(map[string]*managedConsumer),
		brokers:     brokers,
		groupPrefix: groupPrefix,
		processor:   processor,
		bindingName: bindingName,
		deliverer:   deliverer,
	}
}

// SetContext sets the base context for started consumers.
func (cm *ConsumerManager) SetContext(ctx context.Context) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.ctx = ctx
}

// AddSubscription creates or recreates the consumer for the given callback URL
// with the Kafka topic added to its topic set.
func (cm *ConsumerManager) AddSubscription(callbackURL, secret, kafkaTopic string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	mc, exists := cm.consumers[callbackURL]
	if exists {
		// If the topic is already in the set, nothing to do.
		if mc.topics[kafkaTopic] {
			return nil
		}
		// Stop existing consumer, recreate with expanded topic set.
		if mc.consumer != nil {
			if err := mc.consumer.Stop(context.Background()); err != nil {
				slog.Error("Failed to stop consumer for topic change", "callback", callbackURL, "error", err)
			}
		}
		mc.topics[kafkaTopic] = true
	} else {
		mc = &managedConsumer{
			topics: map[string]bool{kafkaTopic: true},
		}
		cm.consumers[callbackURL] = mc
	}

	// Build topic list.
	topicList := make([]string, 0, len(mc.topics))
	for t := range mc.topics {
		topicList = append(topicList, t)
	}

	groupID := cm.consumerGroupID(callbackURL)
	consumer, err := cm.createConsumer(groupID, topicList, callbackURL, secret)
	if err != nil {
		return fmt.Errorf("failed to create consumer for callback %s: %w", callbackURL, err)
	}
	mc.consumer = consumer

	ctx := cm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consumer for callback %s: %w", callbackURL, err)
	}

	slog.Info("Consumer started for callback",
		"callback", callbackURL,
		"group_id", groupID,
		"topics", topicList,
	)
	return nil
}

// RemoveSubscription removes a Kafka topic from the callback's consumer.
// If no topics remain, the consumer is stopped and removed entirely.
func (cm *ConsumerManager) RemoveSubscription(callbackURL, kafkaTopic string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	mc, exists := cm.consumers[callbackURL]
	if !exists {
		return nil
	}

	delete(mc.topics, kafkaTopic)

	// Stop current consumer.
	if mc.consumer != nil {
		if err := mc.consumer.Stop(context.Background()); err != nil {
			slog.Error("Failed to stop consumer on unsubscribe", "callback", callbackURL, "error", err)
		}
		mc.consumer = nil
	}

	// If no topics remain, remove the entry entirely.
	if len(mc.topics) == 0 {
		delete(cm.consumers, callbackURL)
		slog.Info("Consumer removed for callback", "callback", callbackURL)
		return nil
	}

	// Recreate consumer with shrunk topic set.
	topicList := make([]string, 0, len(mc.topics))
	for t := range mc.topics {
		topicList = append(topicList, t)
	}

	groupID := cm.consumerGroupID(callbackURL)
	consumer, err := cm.createConsumer(groupID, topicList, callbackURL, "")
	if err != nil {
		return fmt.Errorf("failed to recreate consumer for callback %s: %w", callbackURL, err)
	}
	mc.consumer = consumer

	ctx := cm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to restart consumer for callback %s: %w", callbackURL, err)
	}

	return nil
}

// StopAll stops all managed consumers.
func (cm *ConsumerManager) StopAll(ctx context.Context) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for cb, mc := range cm.consumers {
		if mc.consumer != nil {
			if err := mc.consumer.Stop(ctx); err != nil {
				slog.Error("Failed to stop managed consumer", "callback", cb, "error", err)
			}
		}
	}
	cm.consumers = make(map[string]*managedConsumer)
}

func (cm *ConsumerManager) createConsumer(groupID string, topics []string, callbackURL, secret string) (connectors.Receiver, error) {
	handler := func(ctx context.Context, msg *connectors.Message) error {
		// Apply outbound policies.
		processed, shortCircuited, err := cm.processor.ProcessOutbound(ctx, cm.bindingName, msg)
		if err != nil {
			return fmt.Errorf("outbound policy execution failed: %w", err)
		}
		if shortCircuited {
			return nil
		}
		// Deliver to this specific callback.
		return cm.deliverer.Deliver(ctx, callbackURL, secret, processed)
	}

	consumer, err := kafka.NewManualCommitConsumer(cm.brokers, groupID, topics, handler)
	if err != nil {
		return nil, err
	}
	return consumer, nil
}

// consumerGroupID generates a unique, safe consumer group ID for a callback URL.
// Format: {prefix}-websub-{sha256(callbackURL)[:16]}
func (cm *ConsumerManager) consumerGroupID(callbackURL string) string {
	h := sha256.Sum256([]byte(callbackURL))
	return cm.groupPrefix + "-websub-" + hex.EncodeToString(h[:])[:16]
}
