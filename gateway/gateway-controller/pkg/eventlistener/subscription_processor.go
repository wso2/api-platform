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

package eventlistener

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
)

// processSubscriptionEvent refreshes replica-local subscription xDS state after subscription changes.
func (l *EventListener) processSubscriptionEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
		l.refreshSubscriptionState("subscription", event)
	default:
		l.logger.Warn("Unknown subscription event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

// processSubscriptionPlanEvent refreshes replica-local subscription xDS state after plan changes.
func (l *EventListener) processSubscriptionPlanEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
		l.refreshSubscriptionState("subscription_plan", event)
	default:
		l.logger.Warn("Unknown subscription plan event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

// processApplicationEvent acknowledges application metadata changes. The canonical state is DB-backed only today.
func (l *EventListener) processApplicationEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
		l.logger.Info("Processed application replica sync event",
			slog.String("action", event.Action),
			slog.String("application_id", event.EntityID),
			slog.String("event_id", event.EventID))
	default:
		l.logger.Warn("Unknown application event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) refreshSubscriptionState(resource string, event eventhub.Event) {
	if l.subscriptionManager == nil {
		l.logger.Warn("Subscription snapshot manager not available for replica sync",
			slog.String("resource", resource),
			slog.String("entity_id", event.EntityID),
			slog.String("event_id", event.EventID))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := l.subscriptionManager.UpdateSnapshot(ctx); err != nil {
		l.logger.Error("Failed to refresh subscription snapshot from replica sync event",
			slog.String("resource", resource),
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	l.logger.Info("Successfully refreshed subscription snapshot from replica sync event",
		slog.String("resource", resource),
		slog.String("action", event.Action),
		slog.String("entity_id", event.EntityID),
		slog.String("event_id", event.EventID))
}
