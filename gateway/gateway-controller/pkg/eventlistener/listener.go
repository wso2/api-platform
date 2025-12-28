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
	"fmt"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

// EventListener subscribes to EventHub and processes events to update XDS
type EventListener struct {
	eventHub        eventhub.EventHub
	store           *storage.ConfigStore     // In-memory config store
	db              storage.Storage          // Persistent storage (SQLite)
	snapshotManager *xds.SnapshotManager     // XDS snapshot manager
	policyManager   *policyxds.PolicyManager // Optional: policy manager
	routerConfig    *config.RouterConfig     // Router configuration for vhosts
	logger          *zap.Logger

	eventChan chan []eventhub.Event // Buffered channel (size 10)
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewEventListener creates a new EventListener instance
func NewEventListener(
	eventHub eventhub.EventHub,
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager, // Can be nil
	routerConfig *config.RouterConfig,
	logger *zap.Logger,
) *EventListener {
	return &EventListener{
		eventHub:        eventHub,
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		policyManager:   policyManager,
		routerConfig:    routerConfig,
		logger:          logger,
	}
}

// Start initializes the event listener and starts processing events
func (el *EventListener) Start(ctx context.Context) error {
	el.ctx, el.cancel = context.WithCancel(ctx)

	// Create buffered channel with size 10
	el.eventChan = make(chan []eventhub.Event, 10)

	// Register "default" organization (idempotent - may already exist)
	orgID := eventhub.OrganizationID("default")
	if err := el.eventHub.RegisterOrganization(orgID); err != nil {
		// Ignore if already exists
		el.logger.Debug("Organization may already be registered", zap.String("organization", string(orgID)))
	}

	// Subscribe to events
	if err := el.eventHub.Subscribe(orgID, el.eventChan); err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Start processing goroutine
	el.wg.Add(1)
	// TODO: (VirajSalaka) Should recover in case of panics
	go el.processEvents()

	el.logger.Info("EventListener started", zap.String("organization", "default"))
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

// handleEvent processes a single event and delegates based on event type
func (el *EventListener) handleEvent(event eventhub.Event) {
	log := el.logger.With(
		zap.String("event_type", string(event.EventType)),
		zap.String("action", event.Action),
		zap.String("entity_id", event.EntityID),
	)

	switch event.EventType {
	case eventhub.EventTypeAPI:
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
