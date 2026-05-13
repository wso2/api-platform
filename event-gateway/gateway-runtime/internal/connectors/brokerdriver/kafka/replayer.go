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

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

type replayPartitionKey struct {
	topic     string
	partition int32
}

// ReplayTopic consumes a compacted topic from offset 0 until every partition
// reaches the end offset captured at the start of replay.
func ReplayTopic(ctx context.Context, cfg ConnectionConfig, topic string, handler connectors.MessageHandler) error {
	adminClient, err := NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create kafka replay admin client: %w", err)
	}
	admin := kadm.NewClient(adminClient)
	endOffsets, err := admin.ListEndOffsets(ctx, topic)
	adminClient.Close()
	if err != nil {
		return fmt.Errorf("failed to list replay end offsets for topic %s: %w", topic, err)
	}

	targetOffsets := make(map[replayPartitionKey]int64)
	completedPartitions := make(map[replayPartitionKey]bool)
	var offsetErr error
	endOffsets.Each(func(o kadm.ListedOffset) {
		if offsetErr != nil {
			return
		}
		if o.Err != nil {
			offsetErr = fmt.Errorf("failed to inspect replay offset for topic %s partition %d: %w", o.Topic, o.Partition, o.Err)
			return
		}
		key := replayPartitionKey{topic: o.Topic, partition: o.Partition}
		targetOffsets[key] = o.Offset
		completedPartitions[key] = o.Offset == 0
	})
	if offsetErr != nil {
		return offsetErr
	}
	if len(targetOffsets) == 0 || replayComplete(completedPartitions) {
		return nil
	}

	opts, err := BuildClientOptions(
		cfg,
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return fmt.Errorf("failed to build kafka replay consumer options: %w", err)
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("failed to create kafka replay consumer: %w", err)
	}
	defer client.Close()
	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("Kafka replayer fetch error", "topic", e.Topic, "partition", e.Partition, "error", e.Err)
			}
			return fmt.Errorf("fetch errors during replay for topic %s", topic)
		}

		var handlerErr error
		fetches.EachRecord(func(record *kgo.Record) {
			if handlerErr != nil {
				return
			}
			partitionKey := replayPartitionKey{
				topic:     record.Topic,
				partition: record.Partition,
			}
			if completedPartitions[partitionKey] {
				return
			}
			if err := handler(ctx, recordToMessage(record)); err != nil {
				slog.Error("Replayer handler error",
					"topic", record.Topic,
					"partition", record.Partition,
					"offset", record.Offset,
					"error", err,
				)
				handlerErr = fmt.Errorf("replay handler failed for topic %s partition %d offset %d: %w", record.Topic, record.Partition, record.Offset, err)
				return
			}
			if record.Offset+1 >= targetOffsets[partitionKey] {
				completedPartitions[partitionKey] = true
			}
		})

		if handlerErr != nil {
			return handlerErr
		}
		if replayComplete(completedPartitions) {
			return nil
		}
	}
}

func replayComplete(completedPartitions map[replayPartitionKey]bool) bool {
	for _, done := range completedPartitions {
		if !done {
			return false
		}
	}
	return true
}
