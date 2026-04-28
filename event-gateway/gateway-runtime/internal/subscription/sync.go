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

package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// SyncProducer publishes subscription state changes to a per-API sync topic.
type SyncProducer struct {
	broker    connectors.BrokerDriver
	runtimeID string
	syncTopic string
}

// NewSyncProducer creates a new sync producer that writes to the given syncTopic.
func NewSyncProducer(broker connectors.BrokerDriver, runtimeID, syncTopic string) *SyncProducer {
	return &SyncProducer{broker: broker, runtimeID: runtimeID, syncTopic: syncTopic}
}

// EnsureSyncTopic creates the per-API subscription sync topic if it
// does not already exist. The topic is created with cleanup.policy=compact
// so that the latest subscription state per key is retained indefinitely.
func (p *SyncProducer) EnsureSyncTopic(ctx context.Context) error {
	return p.broker.EnsureStateTopics(ctx, []string{p.syncTopic})
}

// PublishSubscription publishes a subscription state change synchronously.
// It blocks until the record is acknowledged by Kafka so that the data is
// durable before the caller (HTTP handler) returns.
func (p *SyncProducer) PublishSubscription(_ context.Context, sub *Subscription) error {
	sub.RuntimeID = p.runtimeID

	value, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	key := syncKey(sub.Topic, sub.CallbackURL)
	record := &connectors.Message{
		Key:   []byte(key),
		Value: value,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.broker.Publish(ctx, p.syncTopic, record); err != nil {
		slog.Error("Failed to publish subscription sync", "key", key, "error", err)
		return fmt.Errorf("failed to publish subscription sync: %w", err)
	}

	return nil
}

// PublishTombstone publishes a tombstone (deletion) for a subscription synchronously.
func (p *SyncProducer) PublishTombstone(_ context.Context, topic, callbackURL string) error {
	key := syncKey(topic, callbackURL)
	record := &connectors.Message{
		Key:   []byte(key),
		Value: nil, // tombstone
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.broker.Publish(ctx, p.syncTopic, record); err != nil {
		slog.Error("Failed to publish subscription tombstone", "key", key, "error", err)
		return fmt.Errorf("failed to publish subscription tombstone: %w", err)
	}

	return nil
}

// Close flushes any buffered records and closes the sync producer.
func (p *SyncProducer) Close() {
	// The broker driver lifecycle is owned by the runtime.
}

func syncKey(topic, callbackURL string) string {
	return topic + ":" + callbackURL
}

func parseSyncKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return nil
}
