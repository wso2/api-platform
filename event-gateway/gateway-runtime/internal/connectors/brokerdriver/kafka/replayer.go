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

// Replayer consumes from a compacted topic from offset 0, replaying all state.
// Each runtime uses its own consumer identity (NOT shared-group).
type Replayer struct {
	client  *kgo.Client
	handler connectors.MessageHandler
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewReplayer creates a new replayer for a compacted topic.
func NewReplayer(brokers []string, topic string, handler connectors.MessageHandler) (*Replayer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka replayer: %w", err)
	}

	return &Replayer{
		client:  client,
		handler: handler,
	}, nil
}

// Start begins consuming and replaying events.
func (r *Replayer) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.consumeLoop(ctx)
	}()
}

// Stop stops the replayer.
func (r *Replayer) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	r.client.Close()
}

func (r *Replayer) consumeLoop(ctx context.Context) {
	for {
		fetches := r.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("Kafka replayer fetch error", "topic", e.Topic, "partition", e.Partition, "error", e.Err)
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			msg := recordToMessage(record)
			if err := r.handler(ctx, msg); err != nil {
				slog.Error("Replayer handler error",
					"topic", record.Topic,
					"offset", record.Offset,
					"error", err,
				)
			}
		})
	}
}
