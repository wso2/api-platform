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
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	// SyncTopic is the compacted Kafka topic used for subscription state sync.
	SyncTopic = "__event_gateway_subscriptions"
)

// SyncProducer publishes subscription state changes to the sync topic.
type SyncProducer struct {
	client    *kgo.Client
	runtimeID string
	brokers   []string
}

// NewSyncProducer creates a new sync producer.
func NewSyncProducer(brokers []string, runtimeID string) (*SyncProducer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(SyncTopic),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync producer: %w", err)
	}
	return &SyncProducer{client: client, runtimeID: runtimeID, brokers: brokers}, nil
}

// EnsureSyncTopic creates the __event_gateway_subscriptions topic if it
// does not already exist. The topic is created with cleanup.policy=compact
// so that the latest subscription state per key is retained indefinitely.
func (p *SyncProducer) EnsureSyncTopic(ctx context.Context) error {
	adminKgo, err := kgo.NewClient(kgo.SeedBrokers(p.brokers...))
	if err != nil {
		return fmt.Errorf("failed to create admin client: %w", err)
	}
	defer adminKgo.Close()

	admin := kadm.NewClient(adminKgo)
	topicConfig := map[string]*string{
		"cleanup.policy": kadm.StringPtr("compact"),
	}
	resp, err := admin.CreateTopics(ctx, 1, 1, topicConfig, SyncTopic)
	if err != nil {
		return fmt.Errorf("failed to create sync topic: %w", err)
	}
	for _, t := range resp.Sorted() {
		if t.Err != nil {
			errStr := t.Err.Error()
			if !(strings.Contains(errStr, "TOPIC_ALREADY_EXISTS") || strings.Contains(errStr, "already exists")) {
				return fmt.Errorf("failed to create sync topic %s: %w", t.Topic, t.Err)
			}
		}
	}
	return nil
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
	record := &kgo.Record{
		Key:   []byte(key),
		Value: value,
		Topic: SyncTopic,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		slog.Error("Failed to publish subscription sync", "key", key, "error", err)
		return fmt.Errorf("failed to publish subscription sync: %w", err)
	}

	return nil
}

// PublishTombstone publishes a tombstone (deletion) for a subscription synchronously.
func (p *SyncProducer) PublishTombstone(_ context.Context, topic, callbackURL string) error {
	key := syncKey(topic, callbackURL)
	record := &kgo.Record{
		Key:   []byte(key),
		Value: nil, // tombstone
		Topic: SyncTopic,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		slog.Error("Failed to publish subscription tombstone", "key", key, "error", err)
		return fmt.Errorf("failed to publish subscription tombstone: %w", err)
	}

	return nil
}

// Close flushes any buffered records and closes the sync producer.
func (p *SyncProducer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.client.Flush(ctx); err != nil {
		slog.Warn("Failed to flush sync producer before close", "error", err)
	}
	p.client.Close()
}

// SyncConsumer consumes subscription state changes from the sync topic.
type SyncConsumer struct {
	client    *kgo.Client
	store     SubscriptionStore
	runtimeID string
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewSyncConsumer creates a new sync consumer.
func NewSyncConsumer(brokers []string, store SubscriptionStore, runtimeID string) (*SyncConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(SyncTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync consumer: %w", err)
	}
	return &SyncConsumer{
		client:    client,
		store:     store,
		runtimeID: runtimeID,
	}, nil
}

// Start begins consuming subscription state changes.
func (c *SyncConsumer) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.consumeLoop(ctx)
	}()
}

// Stop stops the sync consumer.
func (c *SyncConsumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.client.Close()
}

func (c *SyncConsumer) consumeLoop(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		fetches.EachRecord(func(record *kgo.Record) {
			c.processRecord(record)
		})
	}
}

func (c *SyncConsumer) processRecord(record *kgo.Record) {
	// Tombstone — remove subscription
	if record.Value == nil {
		parts := parseSyncKey(string(record.Key))
		if parts == nil {
			return
		}
		if err := c.store.Remove(parts[0], parts[1]); err != nil {
			slog.Debug("Failed to remove subscription from sync", "key", string(record.Key), "error", err)
		}
		return
	}

	var sub Subscription
	if err := json.Unmarshal(record.Value, &sub); err != nil {
		slog.Error("Failed to unmarshal subscription from sync", "error", err)
		return
	}

	// Skip self-originated messages
	if sub.RuntimeID == c.runtimeID {
		return
	}

	if err := c.store.Add(&sub); err != nil {
		slog.Error("Failed to add subscription from sync", "error", err)
	}
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
