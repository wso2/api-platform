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

// Package eventgateway implements the event-gateway ControllerExtension for the
// gateway-controller, adding WebSub/WebBroker subscription management, subscription
// plan management, and webhook HMAC secret management on top of the base controller.
package eventgateway

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"
)

// SubscriptionProcessor handles EventTypeSubscription and EventTypeSubscriptionPlan
// events by refreshing the local subscription xDS snapshot from the database.
// It implements controllerext.ExtraEventProcessor.
type SubscriptionProcessor struct {
	snapshotManager *subscriptionxds.SnapshotManager
	logger          *slog.Logger
}

// NewSubscriptionProcessor creates a SubscriptionProcessor backed by the given manager.
func NewSubscriptionProcessor(mgr *subscriptionxds.SnapshotManager, logger *slog.Logger) *SubscriptionProcessor {
	return &SubscriptionProcessor{snapshotManager: mgr, logger: logger}
}

// HandlesEventType reports whether this processor handles the given event type.
func (p *SubscriptionProcessor) HandlesEventType(t eventhub.EventType) bool {
	return t == eventhub.EventTypeSubscription || t == eventhub.EventTypeSubscriptionPlan
}

// Process dispatches the event to the appropriate handler.
func (p *SubscriptionProcessor) Process(_ context.Context, event eventhub.Event) {
	switch event.EventType {
	case eventhub.EventTypeSubscription:
		p.refreshSnapshot("subscription", event)
	case eventhub.EventTypeSubscriptionPlan:
		p.refreshSnapshot("subscription_plan", event)
	}
}

func (p *SubscriptionProcessor) refreshSnapshot(resource string, event eventhub.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.snapshotManager.UpdateSnapshot(ctx); err != nil {
		p.logger.Error("Failed to refresh subscription snapshot from replica sync event",
			slog.String("resource", resource),
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	p.logger.Info("Successfully refreshed subscription snapshot from replica sync event",
		slog.String("resource", resource),
		slog.String("action", event.Action),
		slog.String("entity_id", event.EntityID),
		slog.String("event_id", event.EventID))
}
