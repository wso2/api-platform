/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

// EventListener subscribes to an EventSource and processes events to update XDS.
// It uses the generic EventSource interface, allowing it to work with different
// event delivery mechanisms (EventHub, Kafka, RabbitMQ, etc.) and enabling easy mocking for tests.
type EventListener struct {
	eventSource     EventSource              // Generic event source (EventHub, Kafka, etc.)
	store           *storage.ConfigStore     // In-memory config store
	db              storage.Storage          // Persistent storage (SQLite)
	snapshotManager *xds.SnapshotManager     // XDS snapshot manager
	policyManager   *policyxds.PolicyManager // Optional: policy manager
	routerConfig    *config.RouterConfig     // Router configuration for vhosts
	logger          *zap.Logger

	eventChan chan []Event // Buffered channel (size 10) for generic events
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewEventListener creates a new EventListener instance.
//
// Parameters:
//   - eventSource: The event source to subscribe to (can be EventHubAdapter, MockEventSource, or any EventSource implementation)
//   - store: In-memory configuration store
//   - db: Persistent storage (SQLite)
//   - snapshotManager: xDS snapshot manager for updating Envoy configuration
//   - policyManager: Optional policy manager (can be nil)
//   - routerConfig: Router configuration for vhosts
//   - logger: Structured logger
//
// Returns:
//   - *EventListener ready to be started
func NewEventListener(
	eventSource EventSource,
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager, // Can be nil
	routerConfig *config.RouterConfig,
	logger *zap.Logger,
) *EventListener {
	return &EventListener{
		eventSource:     eventSource,
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		policyManager:   policyManager,
		routerConfig:    routerConfig,
		logger:          logger,
	}
}

// Start initializes the event listener and starts processing events.
// Subscribes to the "default" organization and starts the event processing goroutine.
func (el *EventListener) Start(ctx context.Context) error {
	el.ctx, el.cancel = context.WithCancel(ctx)

	// Create buffered channel with size 10 for generic events
	el.eventChan = make(chan []Event, 10)

	// Subscribe to "default" organization events via the EventSource
	organizationID := "default"
	if err := el.eventSource.Subscribe(ctx, organizationID, el.eventChan); err != nil {
		return err // Let the EventSource adapter handle details
	}

	// Start processing goroutine
	el.wg.Add(1)
	// TODO: (VirajSalaka) Should recover in case of panics
	go el.processEvents()

	el.logger.Info("EventListener started", zap.String("organization", organizationID))
	return nil
}

// processEvents is a goroutine that continuously processes events from the channel
func (el *EventListener) processEvents() {
	defer el.wg.Done()

	for {
		select {
		case <-el.ctx.Done():
			el.logger.Info("EventListener stopping")
			return

		case events := <-el.eventChan:
			for _, event := range events {
				el.handleEvent(event)
			}
		}
	}
}

// handleEvent processes a single event and delegates based on event type.
// Uses the generic Event type that works with any EventSource implementation.
func (el *EventListener) handleEvent(event Event) {
	log := el.logger.With(
		zap.String("event_type", event.EventType),
		zap.String("action", event.Action),
		zap.String("entity_id", event.EntityID),
	)

	switch event.EventType {
	case "API": // EventTypeAPI constant
		el.processAPIEvents(event)
	default:
		log.Debug("Ignoring non-API event")
	}
}

// Stop gracefully shuts down the event listener
func (el *EventListener) Stop() {
	if el.cancel != nil {
		el.cancel()
	}
	el.wg.Wait()

	if el.eventChan != nil {
		close(el.eventChan)
	}

	el.logger.Info("EventListener stopped")
}
