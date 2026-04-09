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

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Reconciler rebuilds the in-memory subscription store from the Kafka compacted topic on startup.
type Reconciler struct {
	brokers   []string
	store     SubscriptionStore
	runtimeID string
}

// NewReconciler creates a new Reconciler.
func NewReconciler(brokers []string, store SubscriptionStore, runtimeID string) *Reconciler {
	return &Reconciler{
		brokers:   brokers,
		store:     store,
		runtimeID: runtimeID,
	}
}

// Reconcile replays all messages from the compacted subscription topic and rebuilds the store.
// Returns when the consumer has caught up to the high watermark.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	slog.Info("Starting subscription reconciliation from Kafka")

	// Get high watermarks to know when we're caught up
	adminClient, err := kgo.NewClient(kgo.SeedBrokers(r.brokers...))
	if err != nil {
		return fmt.Errorf("failed to create admin client: %w", err)
	}
	admin := kadm.NewClient(adminClient)

	endOffsets, err := admin.ListEndOffsets(ctx, SyncTopic)
	if err != nil {
		adminClient.Close()
		return fmt.Errorf("failed to list end offsets: %w", err)
	}
	adminClient.Close()

	// Calculate total messages to replay
	var totalEnd int64
	endOffsets.Each(func(o kadm.ListedOffset) {
		totalEnd += o.Offset
	})

	if totalEnd == 0 {
		slog.Info("No subscription data to reconcile")
		return nil
	}

	// Create a consumer from the beginning
	client, err := kgo.NewClient(
		kgo.SeedBrokers(r.brokers...),
		kgo.ConsumeTopics(SyncTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return fmt.Errorf("failed to create reconciliation consumer: %w", err)
	}
	defer client.Close()

	var replayed int64
	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("fetch errors during reconciliation: %v", errs)
		}

		fetches.EachRecord(func(record *kgo.Record) {
			if record.Value == nil {
				// Tombstone — remove from store
				parts := parseSyncKey(string(record.Key))
				if parts != nil {
					_ = r.store.Remove(parts[0], parts[1])
				}
			} else {
				var sub Subscription
				if err := json.Unmarshal(record.Value, &sub); err != nil {
					slog.Error("Failed to unmarshal subscription during reconciliation", "error", err)
					return
				}
				_ = r.store.Add(&sub)
			}
			replayed++
		})

		// Check if we've caught up to all partitions
		caughtUp := true
		endOffsets.Each(func(o kadm.ListedOffset) {
			if o.Offset > 0 {
				// Simplified catch-up check
				caughtUp = caughtUp && (replayed >= totalEnd)
			}
		})

		if caughtUp {
			break
		}
	}

	slog.Info("Subscription reconciliation complete",
		"replayed", replayed,
		"active_subscriptions", len(r.store.GetActive()),
	)

	return nil
}
