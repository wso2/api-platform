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

package eventhub

import "time"

// EventType represents the type of event
type EventType string

const (
	// EventTypeAPI represents an API configuration change event
	EventTypeAPI EventType = "API"
	// EventTypeAPIKey represents an API key change event
	EventTypeAPIKey EventType = "API_KEY"
	// EventTypeCertificate represents a certificate change event
	EventTypeCertificate EventType = "CERTIFICATE"
	// EventTypeSubscription represents a subscription change event
	EventTypeSubscription EventType = "SUBSCRIPTION"
	// EventTypeSubscriptionPlan represents a subscription plan change event
	EventTypeSubscriptionPlan EventType = "SUBSCRIPTION_PLAN"
	// EventTypeApplication represents an application metadata change event
	EventTypeApplication EventType = "APPLICATION"
	// EventTypeLLMProvider represents an LLM provider change event
	EventTypeLLMProvider EventType = "LLM_PROVIDER"
	// EventTypeLLMProxy represents an LLM proxy change event
	EventTypeLLMProxy EventType = "LLM_PROXY"
	// EventTypeLLMTemplate represents an LLM template change event
	EventTypeLLMTemplate EventType = "LLM_TEMPLATE"
	// EventTypeMCPProxy represents an MCP proxy change event
	EventTypeMCPProxy EventType = "MCP_PROXY"

	// EmptyEventData is the canonical JSON payload for events that do not
	// require additional data beyond the top-level event fields.
	EmptyEventData = "{}"
)

// Event represents a change event in the system
type Event struct {
	GatewayID           string    `json:"gateway_id"`
	ProcessedTimestamp  time.Time `json:"processed_timestamp"`
	OriginatedTimestamp time.Time `json:"originated_timestamp"`
	EventType           EventType `json:"event_type"`
	Action              string    `json:"action"`
	EntityID            string    `json:"entity_id"`
	EventID             string    `json:"event_id"`
	// EventData carries optional event-specific details that are not already
	// represented by top-level fields such as Action and EntityID.
	EventData string `json:"event_data"`
}

// GatewayState tracks the version state of a gateway.
type GatewayState struct {
	GatewayID string    `json:"gateway_id"`
	VersionID string    `json:"version_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EventHub defines the interface for event publishing and subscribing
type EventHub interface {
	// Initialize sets up the event hub
	Initialize() error
	// RegisterGateway registers a new gateway for event tracking.
	RegisterGateway(gatewayID string) error
	// PublishEvent publishes an event for a gateway.
	PublishEvent(gatewayID string, event Event) error
	// Subscribe subscribes to events for a gateway.
	Subscribe(gatewayID string) (<-chan Event, error)
	// Unsubscribe removes a specific subscription for a gateway.
	Unsubscribe(gatewayID string, subscriber <-chan Event) error
	// UnsubscribeAll removes all subscriptions for a gateway.
	UnsubscribeAll(gatewayID string) error
	// CleanUpEvents removes old events
	CleanUpEvents() error
	// Close gracefully shuts down the event hub
	Close() error
}

// Config holds configuration for the EventHub
type Config struct {
	PollInterval    time.Duration
	CleanupInterval time.Duration
	RetentionPeriod time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		PollInterval:    3 * time.Second,
		CleanupInterval: 10 * time.Minute,
		RetentionPeriod: 1 * time.Hour,
	}
}
