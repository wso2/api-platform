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
	"fmt"
	"log/slog"
)

// ApplyBindingDelta mutates the live receiver for channel add/remove changes
// without recreating the receiver or the subscription sync topic.
func (e *WebSubReceiver) ApplyBindingDelta(ctx context.Context, removedChannels map[string]string, addedChannels map[string]string) error {
	for channelName := range removedChannels {
		subscriptions := e.store.GetByTopic(channelName)
		for _, sub := range subscriptions {
			if e.syncProducer != nil {
				if err := e.syncProducer.PublishTombstone(ctx, channelName, sub.CallbackURL); err != nil {
					return fmt.Errorf("failed to tombstone subscription for removed channel %q: %w", channelName, err)
				}
			}
		}
	}

	for channelName, kafkaTopic := range removedChannels {
		e.topics.Deregister(channelName)

		subscriptions := e.store.GetByTopic(channelName)
		for _, sub := range subscriptions {
			if err := e.consumerMgr.RemoveSubscription(sub.CallbackURL, kafkaTopic); err != nil {
				slog.Error("Failed to remove consumer for deleted WebSub channel",
					"api", e.channel.Name,
					"channel", channelName,
					"callback", sub.CallbackURL,
					"error", err)
			}
			if err := e.store.Remove(channelName, sub.CallbackURL); err != nil {
				slog.Error("Failed to remove subscription for deleted WebSub channel",
					"api", e.channel.Name,
					"channel", channelName,
					"callback", sub.CallbackURL,
					"error", err)
			}
		}

		e.channelMu.Lock()
		delete(e.channel.Channels, channelName)
		e.channelMu.Unlock()
	}

	for channelName, kafkaTopic := range addedChannels {
		e.channelMu.Lock()
		e.channel.Channels[channelName] = kafkaTopic
		e.channelMu.Unlock()
		e.topics.Register(channelName)
	}

	return nil
}
