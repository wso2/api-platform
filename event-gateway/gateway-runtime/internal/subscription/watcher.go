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
	"log/slog"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// SyncWatcher tails the per-API subscription sync topic from the current offset
// and keeps the in-memory store and consumer manager up-to-date in real time.
// It complements the Reconciler (which handles startup replay) by propagating
// subscription changes made on other instances while this instance is running.
type SyncWatcher struct {
	driver    connectors.BrokerDriver
	store     SubscriptionStore
	runtimeID string
	apiID     string // collision-safe hash of the API's identifying parts (context + version)
	syncTopic string
	callback  SubscriptionCallback
}

// NewSyncWatcher creates a SyncWatcher for the given sync topic.
// apiID is a collision-safe hash that uniquely identifies the API (computed by
// the caller via JoinNormalizedTopic so that subscription package stays free of
// binding package imports).
func NewSyncWatcher(driver connectors.BrokerDriver, store SubscriptionStore, runtimeID, apiID, syncTopic string) *SyncWatcher {
	return &SyncWatcher{
		driver:    driver,
		store:     store,
		runtimeID: runtimeID,
		apiID:     apiID,
		syncTopic: syncTopic,
	}
}

// SetCallback sets the function called for each subscription change. The
// connector uses this to start or stop per-callback Kafka consumers via
// the ConsumerManager. Signature is the same as Reconciler.SetCallback.
func (w *SyncWatcher) SetCallback(cb SubscriptionCallback) {
	w.callback = cb
}

// Watch starts tailing the sync topic and returns a Receiver whose lifecycle
// the caller manages. It must be called after Reconciler.Reconcile so that the
// initial state is already loaded; Watch only processes changes from this point
// forward.
func (w *SyncWatcher) Watch(ctx context.Context) (connectors.Receiver, error) {
	consumerID := w.runtimeID + "-" + w.apiID
	receiver, err := w.driver.Watch(ctx, consumerID, w.syncTopic, w.handleMessage)
	if err != nil {
		return nil, err
	}
	if err := receiver.Start(ctx); err != nil {
		return nil, err
	}
	slog.Info("Subscription sync watcher started",
		"topic", w.syncTopic,
		"runtime_id", w.runtimeID,
		"sync_group", "event-gateway-sync-"+consumerID)
	return receiver, nil
}

func (w *SyncWatcher) handleMessage(ctx context.Context, msg *connectors.Message) error {
	if msg.Value == nil {
		// Tombstone — subscription removed on another instance.
		parts := parseSyncKey(string(msg.Key))
		if parts == nil {
			return nil
		}
		_ = w.store.Remove(parts[0], parts[1])
		if w.callback != nil {
			w.callback(&Subscription{Topic: parts[0], CallbackURL: parts[1]}, true)
		}
		slog.Info("Subscription sync: removed", "topic", parts[0], "callback", parts[1])
		return nil
	}

	var sub Subscription
	if err := json.Unmarshal(msg.Value, &sub); err != nil {
		slog.Error("Subscription sync: failed to unmarshal message", "error", err)
		return nil
	}

	_ = w.store.Add(&sub)
	if w.callback != nil {
		w.callback(&sub, false)
	}
	slog.Debug("Subscription sync: added", "topic", sub.Topic, "callback", sub.CallbackURL, "origin", sub.RuntimeID)
	return nil
}
