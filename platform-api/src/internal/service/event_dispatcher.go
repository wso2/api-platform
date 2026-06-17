/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/wso2/api-platform/common/eventhub"

	ws "platform-api/src/internal/websocket"
)

// gatewaySubscription holds the EventHub channel and a done signal for one gateway on this instance.
type gatewaySubscription struct {
	ch   <-chan eventhub.Event
	done chan struct{}
}

// EventDispatcher bridges the EventHub to WebSocket delivery.
// It maintains exactly one EventHub subscription per gateway per platform-api instance,
// regardless of how many WebSocket connections that gateway has to this instance.
// This enables HA: any replica can publish an event; only the replica holding the
// WebSocket connection will actually deliver it.
//
// Connection lifecycle is derived directly from the Manager's connection map (the
// authoritative source of truth) rather than a local ref-count. A local counter
// can desync when deliver() calls manager.Unregister() for a failed send while
// the deliver loop is still iterating: an onDisconnect for an old connection fires
// after a new connection (and new subscription) has already been established,
// decrementing the counter for the wrong lifecycle and tearing down the live goroutine.
type EventDispatcher struct {
	hub     eventhub.EventHub
	manager *ws.Manager
	logger  *slog.Logger
	mu      sync.Mutex
	subs    map[string]*gatewaySubscription // gatewayID → subscription
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewEventDispatcher creates an EventDispatcher backed by the given EventHub.
func NewEventDispatcher(hub eventhub.EventHub, manager *ws.Manager, logger *slog.Logger) *EventDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventDispatcher{
		hub:    hub,
		manager: manager,
		logger:  logger,
		subs:    make(map[string]*gatewaySubscription),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// OnGatewayConnected is called whenever a WebSocket connection for gatewayID is registered
// on this instance. On the first connection it subscribes to the EventHub and starts a
// dispatch goroutine. Subsequent connections for the same gateway return immediately —
// deliver() fans out to all connections returned by manager.GetConnections(), so a
// single subscription covers every replica of the gateway on this platform-api instance.
func (d *EventDispatcher) OnGatewayConnected(gatewayID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Subscription already exists — the dispatch goroutine is running and will
	// deliver to all connections the Manager currently holds for this gateway.
	if _, exists := d.subs[gatewayID]; exists {
		return nil
	}

	if err := d.hub.RegisterGateway(gatewayID); err != nil {
		d.logger.Error("EventDispatcher: failed to register gateway", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	ch, err := d.hub.Subscribe(gatewayID)
	if err != nil {
		d.logger.Error("EventDispatcher: failed to subscribe to gateway events", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to subscribe to gateway events: %w", err)
	}

	done := make(chan struct{})
	d.subs[gatewayID] = &gatewaySubscription{ch: ch, done: done}
	go d.dispatchLoop(gatewayID, ch, done)

	d.logger.Debug("EventDispatcher: subscribed to gateway", "gatewayID", gatewayID)
	return nil
}

// OnGatewayDisconnected is called whenever a WebSocket connection for gatewayID is removed
// from this instance. It consults the Manager's live connection map: if connections remain
// the subscription is kept alive; only when the Manager reports zero connections for this
// gateway is the EventHub subscription torn down.
//
// Using the Manager as the source of truth prevents the desync that a local ref-count
// introduces: a stale onDisconnect (from a connection removed during a previous deliver
// loop iteration) can fire after a new connection has already been established, and would
// incorrectly tear down the fresh subscription if counted locally.
func (d *EventDispatcher) OnGatewayDisconnected(gatewayID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Manager.Unregister removes the connection before calling this hook, so
	// GetConnections reflects the post-removal state. If any connections remain,
	// keep the subscription alive.
	if len(d.manager.GetConnections(gatewayID)) > 0 {
		return nil
	}

	sub, ok := d.subs[gatewayID]
	if !ok {
		return nil
	}

	if err := d.hub.Unsubscribe(gatewayID, sub.ch); err != nil {
		d.logger.Warn("EventDispatcher: failed to unsubscribe from gateway", "gatewayID", gatewayID, "error", err)
	}
	close(sub.done)
	delete(d.subs, gatewayID)

	d.logger.Debug("EventDispatcher: unsubscribed from gateway", "gatewayID", gatewayID)
	return nil
}

// Shutdown stops all dispatch goroutines and releases resources. Call during graceful shutdown.
func (d *EventDispatcher) Shutdown() {
	d.cancel()

	d.mu.Lock()
	defer d.mu.Unlock()

	for gatewayID, sub := range d.subs {
		if err := d.hub.Unsubscribe(gatewayID, sub.ch); err != nil {
			d.logger.Warn("EventDispatcher: failed to unsubscribe during shutdown", "gatewayID", gatewayID, "error", err)
		}
		close(sub.done)
	}
	d.subs = make(map[string]*gatewaySubscription)
}

// dispatchLoop reads events from ch and delivers them to local WebSocket connections.
// It exits when done is closed (last connection gone) or the dispatcher context is cancelled.
func (d *EventDispatcher) dispatchLoop(gatewayID string, ch <-chan eventhub.Event, done <-chan struct{}) {
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-done:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			d.deliver(gatewayID, event)
		}
	}
}

// deliver sends the raw EventData bytes to all local WebSocket connections for gatewayID.
// If this instance has no connections for the gateway, the event is silently skipped —
// another replica will hold the connection and deliver it.
func (d *EventDispatcher) deliver(gatewayID string, event eventhub.Event) {
	connections := d.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		return
	}

	raw := []byte(event.EventData)
	successCount := 0
	for _, conn := range connections {
		if err := conn.Send(raw); err != nil {
			d.logger.Error("EventDispatcher: failed to deliver event",
				"gatewayID", gatewayID,
				"connectionID", conn.ConnectionID,
				"eventID", event.EventID,
				"error", err,
			)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
			// A send error means the transport is broken. Unregister the connection
			// immediately so the gateway reconnects and triggers a cursor reset +
			// replay via initialPollSkewWindow, rather than waiting for the heartbeat
			// monitor to detect the stale connection.
			d.manager.Unregister(gatewayID, conn.ConnectionID)
		} else {
			successCount++
			conn.DeliveryStats.IncrementTotalSent()
			d.manager.IncrementTotalEventsSent()
		}
	}

	if successCount > 0 {
		d.logger.Debug("EventDispatcher: delivered event",
			"gatewayID", gatewayID,
			"eventID", event.EventID,
			"connections", len(connections),
			"success", successCount,
		)
	}
}
