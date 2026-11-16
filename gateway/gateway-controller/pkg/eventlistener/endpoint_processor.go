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

// EndpointSyncer brings this replica in line with the gateway-wide Endpoints in
// the database: the in-memory store, the router's clusters and the policy
// engine's lazy resources, in the order that keeps the policy engine from
// naming a cluster the router does not have.
type EndpointSyncer interface {
	Sync(ctx context.Context, correlationID string) error
}

// SetEndpointSyncer registers what applies Endpoint events on this replica.
// Passing nil (the default) means Endpoint events are ignored.
func (l *EventListener) SetEndpointSyncer(syncer EndpointSyncer) {
	l.endpoints = syncer
}

// processEndpointEvent brings this replica in line with an Endpoint written on
// any replica. The event names the row but carries no configuration, so every
// action reconciles from the database. That makes the handler idempotent: the
// replica that made the change has already applied it, and finds nothing to do
// when its own event arrives.
func (l *EventListener) processEndpointEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
	default:
		l.logger.Warn("Unknown endpoint event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
		return
	}

	log := l.logger.With(
		slog.String("endpoint_id", event.EntityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	if l.endpoints == nil {
		log.Warn("Ignoring endpoint event: endpoint support is not configured on this replica")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := l.endpoints.Sync(ctx, event.EventID); err != nil {
		log.Error("Failed to apply endpoint change from replica sync", slog.Any("error", err))
		return
	}

	log.Info("Endpoint change applied from replica sync")
}
