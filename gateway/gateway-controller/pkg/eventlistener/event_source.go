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
	"time"
)

// Event represents a generic event from any event source.
// This is a simplified, agnostic event structure that can be populated
// by different event source implementations (EventHub, Kafka, RabbitMQ, etc.)
type Event struct {
	// OrganizationID identifies which organization this event belongs to
	OrganizationID string

	// EventType describes the kind of event (e.g., "API", "CERTIFICATE", "LLM_TEMPLATE")
	EventType string

	// Action describes what happened (e.g., "CREATE", "UPDATE", "DELETE")
	Action string

	// EntityID identifies the specific entity affected by the event
	EntityID string

	// EventData contains the serialized event payload (typically JSON)
	EventData []byte

	// Timestamp indicates when the event occurred
	Timestamp time.Time
}

// EventSource defines the interface for any event delivery mechanism.
// Implementations can use EventHub, message brokers (Kafka, RabbitMQ, NATS),
// or any other pub/sub system.
//
// Design principles:
// - Simple and focused: only what EventListener needs
// - Technology agnostic: no assumptions about the underlying system
// - Testable: easy to mock for unit tests
type EventSource interface {
	// Subscribe registers to receive events for a specific organization.
	// Events are delivered as batches via the provided channel.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - organizationID: The organization to subscribe to
	//   - eventChan: Channel where event batches will be sent
	//
	// Returns:
	//   - error if subscription fails
	//
	// Notes:
	//   - The implementation is responsible for managing the subscription lifecycle
	//   - Events should be delivered until Unsubscribe is called or ctx is cancelled
	//   - The channel should NOT be closed by the EventSource
	Subscribe(ctx context.Context, organizationID string, eventChan chan<- []Event) error

	// Unsubscribe stops receiving events for a specific organization.
	//
	// Parameters:
	//   - organizationID: The organization to unsubscribe from
	//
	// Returns:
	//   - error if unsubscribe fails
	//
	// Notes:
	//   - After calling Unsubscribe, no more events should be sent to the channel
	//   - It's safe to call Unsubscribe multiple times
	Unsubscribe(organizationID string) error

	// Close gracefully shuts down the event source and cleans up resources.
	//
	// Returns:
	//   - error if shutdown fails
	//
	// Notes:
	//   - Should unsubscribe all active subscriptions
	//   - Should wait for in-flight events to be delivered
	//   - Should be idempotent (safe to call multiple times)
	Close() error
}
