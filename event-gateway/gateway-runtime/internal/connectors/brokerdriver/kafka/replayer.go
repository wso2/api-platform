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

// ReplayTopic replays a compacted topic from offset 0 until the current high watermark.
func ReplayTopic(ctx context.Context, cfg ConnectionConfig, topic string, handler connectors.MessageHandler) error {
	adminOpts, err := BuildClientOptions(cfg)
	if err != nil {
		return fmt.Errorf("failed to create kafka replay admin client: %w", err)
	}
	adminClient, err := kgo.NewClient(adminOpts...)
	if err != nil {
		return fmt.Errorf("failed to create kafka replay admin client: %w", err)
	}
	admin := kadm.NewClient(adminClient)

	endOffsets, err := admin.ListEndOffsets(ctx, topic)
	if err != nil {
		adminClient.Close()
		return fmt.Errorf("failed to list end offsets: %w", err)
	}
	adminClient.Close()

	var totalEnd int64
	endOffsets.Each(func(o kadm.ListedOffset) {
		totalEnd += o.Offset
	})
	if totalEnd == 0 {
		return nil
	}

	opts, err := BuildClientOptions(
		cfg,
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return fmt.Errorf("failed to create kafka replay consumer: %w", err)
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("failed to create kafka replay consumer: %w", err)
	}
	defer client.Close()

	var replayed int64
	for replayed < totalEnd {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("fetch errors during replay: %v", errs)
		}

		fetches.EachRecord(func(record *kgo.Record) {
			msg := recordToMessage(record)
			if err := handler(ctx, msg); err != nil {
				slog.Error("Replay handler error",
					"topic", record.Topic,
					"offset", record.Offset,
					"error", err,
				)
			}
			replayed++
		})
	}

	return nil
}
