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
)

// MockEventSource is a mock implementation of EventSource for testing.
// It allows tests to control event delivery and verify subscription behavior.
//
// Usage example:
//
//	mock := NewMockEventSource()
//	listener := NewEventListener(mock, store, db, ...)
//	listener.Start(ctx)
//
//	// Publish test events
//	mock.PublishEvent("default", Event{...})
//
//	// Verify subscription
//	if !mock.IsSubscribed("default") {
//	    t.Error("Expected subscription to default org")
//	}
type MockEventSource struct {
	mu            sync.RWMutex
	subscriptions map[string]chan<- []Event
	closed        bool

	// PublishedEvents tracks all events published through this mock
	PublishedEvents []Event

	// SubscribeCalls tracks Subscribe method invocations
	SubscribeCalls []SubscribeCall

	// UnsubscribeCalls tracks Unsubscribe method invocations
	UnsubscribeCalls []string

	// Errors can be set to simulate failures
	SubscribeError   error
	UnsubscribeError error
	CloseError       error
}

// SubscribeCall records a Subscribe method invocation
type SubscribeCall struct {
	OrganizationID string
	Context        context.Context
}

// NewMockEventSource creates a new mock event source for testing.
func NewMockEventSource() *MockEventSource {
	return &MockEventSource{
		subscriptions:    make(map[string]chan<- []Event),
		PublishedEvents:  make([]Event, 0),
		SubscribeCalls:   make([]SubscribeCall, 0),
		UnsubscribeCalls: make([]string, 0),
	}
}

// Subscribe implements EventSource.Subscribe for testing.
// Records the call and sets up event delivery.
func (m *MockEventSource) Subscribe(ctx context.Context, organizationID string, eventChan chan<- []Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.SubscribeCalls = append(m.SubscribeCalls, SubscribeCall{
		OrganizationID: organizationID,
		Context:        ctx,
	})

	// Return configured error if set
	if m.SubscribeError != nil {
		return m.SubscribeError
	}

	// Check if already subscribed
	if _, exists := m.subscriptions[organizationID]; exists {
		return fmt.Errorf("already subscribed to organization: %s", organizationID)
	}

	// Store subscription
	m.subscriptions[organizationID] = eventChan

	return nil
}

// Unsubscribe implements EventSource.Unsubscribe for testing.
// Records the call and removes the subscription.
func (m *MockEventSource) Unsubscribe(organizationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.UnsubscribeCalls = append(m.UnsubscribeCalls, organizationID)

	// Return configured error if set
	if m.UnsubscribeError != nil {
		return m.UnsubscribeError
	}

	// Remove subscription
	delete(m.subscriptions, organizationID)

	return nil
}

// Close implements EventSource.Close for testing.
func (m *MockEventSource) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return configured error if set
	if m.CloseError != nil {
		return m.CloseError
	}

	m.closed = true
	m.subscriptions = make(map[string]chan<- []Event)

	return nil
}

// PublishEvent publishes an event to subscribers (test helper).
// This simulates an event being delivered from the event source.
//
// Parameters:
//   - organizationID: The organization to publish to
//   - events: One or more events to publish as a batch
//
// Returns:
//   - error if no subscription exists for the organization
func (m *MockEventSource) PublishEvent(organizationID string, events ...Event) error {
	m.mu.RLock()
	eventChan, exists := m.subscriptions[organizationID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no subscription for organization: %s", organizationID)
	}

	// Track published events
	m.mu.Lock()
	m.PublishedEvents = append(m.PublishedEvents, events...)
	m.mu.Unlock()

	// Send events to subscriber
	eventChan <- events

	return nil
}

// IsSubscribed checks if an organization has an active subscription (test helper).
func (m *MockEventSource) IsSubscribed(organizationID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.subscriptions[organizationID]
	return exists
}

// IsClosed checks if Close has been called (test helper).
func (m *MockEventSource) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.closed
}

// Reset clears all recorded calls and state (test helper).
// Useful for resetting state between test cases.
func (m *MockEventSource) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscriptions = make(map[string]chan<- []Event)
	m.closed = false
	m.PublishedEvents = make([]Event, 0)
	m.SubscribeCalls = make([]SubscribeCall, 0)
	m.UnsubscribeCalls = make([]string, 0)
	m.SubscribeError = nil
	m.UnsubscribeError = nil
	m.CloseError = nil
}

// GetSubscriptionCount returns the number of active subscriptions (test helper).
func (m *MockEventSource) GetSubscriptionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.subscriptions)
}
