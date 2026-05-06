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
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// Consumer consumes events from Kafka using a shared consumer group.
// Each event is consumed by exactly one runtime in the group.
type Consumer struct {
	client  *kgo.Client
	handler connectors.MessageHandler
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewConsumer creates a new shared-group Kafka consumer.
func NewConsumer(brokers []string, groupID string, topics []string, handler connectors.MessageHandler) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer: %w", err)
	}

	return &Consumer{
		client:  client,
		handler: handler,
	}, nil
}

// Start begins consuming events.
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.consumeLoop(ctx)
	}()
	return nil
}

// Stop stops the consumer and waits for the consume loop to exit.
func (c *Consumer) Stop(_ context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.client.Close()
	return nil
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("Kafka consumer fetch error", "topic", e.Topic, "partition", e.Partition, "error", e.Err)
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			msg := recordToMessage(record)
			if err := c.handler(ctx, msg); err != nil {
				slog.Error("Message handler error",
					"topic", record.Topic,
					"partition", record.Partition,
					"offset", record.Offset,
					"error", err,
				)
			}
		})
	}
}

// ManualCommitConsumer consumes from Kafka with manual offset commit control.
// Offsets are committed once per fetch batch through the highest contiguous
// success for each topic-partition. On handler failure, later records from the
// same topic-partition in that batch are skipped so commits do not advance past
// the failed offset.
type ManualCommitConsumer struct {
	client  *kgo.Client
	handler connectors.MessageHandler
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewManualCommitConsumer creates a consumer with manual offset commit.
func NewManualCommitConsumer(brokers []string, groupID string, topics []string, handler connectors.MessageHandler) (*ManualCommitConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create manual-commit kafka consumer: %w", err)
	}

	return &ManualCommitConsumer{
		client:  client,
		handler: handler,
	}, nil
}

// Start begins consuming events with manual commit.
func (c *ManualCommitConsumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.consumeLoop(ctx)
	}()
	return nil
}

// Stop stops the consumer and waits for the consume loop to exit.
func (c *ManualCommitConsumer) Stop(_ context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.client.Close()
	return nil
}

func (c *ManualCommitConsumer) consumeLoop(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("Manual-commit consumer fetch error", "topic", e.Topic, "partition", e.Partition, "error", e.Err)
			}
		}

		commitRecords := collectContiguousCommitRecords(ctx, fetches.Records(), c.handler)
		if len(commitRecords) == 0 {
			continue
		}

		if err := c.client.CommitRecords(ctx, commitRecords...); err != nil {
			slog.Error("Failed to commit offsets for fetch batch",
				"records", len(commitRecords),
				"error", err,
			)
		}
	}
}

type topicPartitionKey struct {
	topic     string
	partition int32
}

func collectContiguousCommitRecords(
	ctx context.Context,
	records []*kgo.Record,
	handler connectors.MessageHandler,
) []*kgo.Record {
	failedPartitions := make(map[topicPartitionKey]struct{})
	lastSuccessByPartition := make(map[topicPartitionKey]*kgo.Record)

	for _, record := range records {
		partitionKey := topicPartitionKey{
			topic:     record.Topic,
			partition: record.Partition,
		}
		if _, failed := failedPartitions[partitionKey]; failed {
			continue
		}

		msg := recordToMessage(record)
		if err := handler(ctx, msg); err != nil {
			slog.Error("Manual-commit handler error, skipping commit",
				"topic", record.Topic,
				"partition", record.Partition,
				"offset", record.Offset,
				"error", err,
			)
			failedPartitions[partitionKey] = struct{}{}
			continue
		}

		lastSuccessByPartition[partitionKey] = record
	}

	commitRecords := make([]*kgo.Record, 0, len(lastSuccessByPartition))
	for _, record := range lastSuccessByPartition {
		commitRecords = append(commitRecords, record)
	}
	return commitRecords
}

func recordToMessage(record *kgo.Record) *connectors.Message {
	headers := make(map[string][]string)
	for _, h := range record.Headers {
		headers[h.Key] = append(headers[h.Key], string(h.Value))
	}

	return &connectors.Message{
		Key:     record.Key,
		Value:   record.Value,
		Headers: headers,
		Topic:   record.Topic,
		Metadata: map[string]interface{}{
			"partition": record.Partition,
			"offset":    record.Offset,
			"timestamp": record.Timestamp,
		},
	}
}
