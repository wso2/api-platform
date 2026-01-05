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

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"go.uber.org/zap"
)

// EventHubAdapter adapts the eventhub.EventHub interface to the generic EventSource interface.
// This allows EventListener to work with the existing EventHub implementation while maintaining
// abstraction and testability.
type EventHubAdapter struct {
	eventHub eventhub.EventHub
	logger   *zap.Logger

	// activeSubscriptions tracks which organizations have active subscriptions
	// This is used to ensure proper cleanup and prevent duplicate subscriptions
	activeSubscriptions map[string]chan<- []Event
}

// NewEventHubAdapter creates a new adapter that wraps an EventHub instance.
//
// Parameters:
//   - eventHub: The EventHub instance to wrap
//   - logger: Logger for debugging and error reporting
//
// Returns:
//   - EventSource implementation backed by EventHub
func NewEventHubAdapter(eventHub eventhub.EventHub, logger *zap.Logger) EventSource {
	return &EventHubAdapter{
		eventHub:            eventHub,
		logger:              logger,
		activeSubscriptions: make(map[string]chan<- []Event),
	}
}

// Subscribe implements EventSource.Subscribe by delegating to EventHub.
// It handles organization registration and event conversion.
func (a *EventHubAdapter) Subscribe(ctx context.Context, organizationID string, eventChan chan<- []Event) error {
	// Check if already subscribed
	if _, exists := a.activeSubscriptions[organizationID]; exists {
		return fmt.Errorf("already subscribed to organization: %s", organizationID)
	}

	// Register organization with EventHub (idempotent operation)
	if err := a.eventHub.RegisterOrganization(organizationID); err != nil {
		a.logger.Debug("Organization may already be registered",
			zap.String("organization", organizationID),
			zap.Error(err),
		)
		// Continue - registration errors are usually not fatal
	}

	// Create a bridge channel that receives eventhub.Event and converts to generic Event
	bridgeChan := make(chan []eventhub.Event, 10)

	// Subscribe to EventHub
	if err := a.eventHub.Subscribe(organizationID, bridgeChan); err != nil {
		close(bridgeChan)
		return fmt.Errorf("failed to subscribe to eventhub: %w", err)
	}

	// Track this subscription
	a.activeSubscriptions[organizationID] = eventChan

	// Start goroutine to convert and forward events
	go a.bridgeEvents(ctx, organizationID, bridgeChan, eventChan)

	a.logger.Info("Subscribed to event source",
		zap.String("organization", organizationID),
		zap.String("source", "eventhub"),
	)

	return nil
}

// bridgeEvents converts eventhub.Event to generic Event and forwards to the listener.
// This goroutine runs until the context is cancelled or the bridge channel is closed.
func (a *EventHubAdapter) bridgeEvents(
	ctx context.Context,
	organizationID string,
	from <-chan []eventhub.Event,
	to chan<- []Event,
) {
	defer func() {
		// Clean up subscription tracking
		delete(a.activeSubscriptions, organizationID)
		a.logger.Debug("Bridge goroutine exiting",
			zap.String("organization", organizationID),
		)
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case hubEvents, ok := <-from:
			if !ok {
				// EventHub channel closed
				a.logger.Warn("EventHub channel closed unexpectedly",
					zap.String("organization", organizationID),
				)
				return
			}

			// Convert eventhub.Event to generic Event
			genericEvents := make([]Event, len(hubEvents))
			for i, hubEvent := range hubEvents {
				genericEvents[i] = Event{
					OrganizationID: string(hubEvent.OrganizationID),
					EventType:      string(hubEvent.EventType),
					Action:         hubEvent.Action,
					EntityID:       hubEvent.EntityID,
					EventData:      hubEvent.EventData,
					Timestamp:      hubEvent.ProcessedTimestamp,
				}
			}

			// Forward to listener
			select {
			case to <- genericEvents:
				// Successfully forwarded
			case <-ctx.Done():
				return
			}
		}
	}
}

// Unsubscribe implements EventSource.Unsubscribe.
// Note: The current EventHub implementation doesn't have an explicit unsubscribe method,
// so we just stop the bridge goroutine by removing the subscription tracking.
func (a *EventHubAdapter) Unsubscribe(organizationID string) error {
	if _, exists := a.activeSubscriptions[organizationID]; !exists {
		// Not subscribed - this is fine, make it idempotent
		return nil
	}

	// Remove from tracking - the bridge goroutine will detect context cancellation
	// when the listener stops
	delete(a.activeSubscriptions, organizationID)

	a.logger.Info("Unsubscribed from event source",
		zap.String("organization", organizationID),
	)

	return nil
}

// Close implements EventSource.Close by delegating to EventHub.Close.
func (a *EventHubAdapter) Close() error {
	// Clean up all subscriptions
	for orgID := range a.activeSubscriptions {
		_ = a.Unsubscribe(orgID)
	}

	// Close the underlying EventHub
	if err := a.eventHub.Close(); err != nil {
		return fmt.Errorf("failed to close eventhub: %w", err)
	}

	a.logger.Info("Event source closed", zap.String("source", "eventhub"))
	return nil
}
