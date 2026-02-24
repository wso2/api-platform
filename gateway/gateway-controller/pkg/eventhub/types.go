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
	// EventTypeLLMTemplate represents an LLM template change event
	EventTypeLLMTemplate EventType = "LLM_TEMPLATE"

	// EmptyEventData is the canonical JSON payload for events that do not
	// require additional data beyond the top-level event fields.
	EmptyEventData = "{}"
)

// Event represents a change event in the system
type Event struct {
	OrganizationID      string    `json:"organization_id"`
	ProcessedTimestamp  time.Time `json:"processed_timestamp"`
	OriginatedTimestamp time.Time `json:"originated_timestamp"`
	EventType           EventType `json:"event_type"`
	Action              string    `json:"action"`
	EntityID            string    `json:"entity_id"`
	CorrelationID       string    `json:"correlation_id"`
	// EventData carries optional event-specific details that are not already
	// represented by top-level fields such as Action and EntityID.
	EventData string `json:"event_data"`
}

// OrganizationState tracks the version state of an organization
type OrganizationState struct {
	Organization string    `json:"organization"`
	VersionID    string    `json:"version_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// EventHub defines the interface for event publishing and subscribing
type EventHub interface {
	// Initialize sets up the event hub
	Initialize() error
	// RegisterOrganization registers a new organization for event tracking
	RegisterOrganization(orgID string) error
	// PublishEvent publishes an event for an organization
	PublishEvent(orgID string, event Event) error
	// Subscribe subscribes to events for an organization
	Subscribe(orgID string) (<-chan Event, error)
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
		PollInterval:    2 * time.Second,
		CleanupInterval: 5 * time.Minute,
		RetentionPeriod: 1 * time.Hour,
	}
}
