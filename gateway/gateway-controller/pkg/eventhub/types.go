package eventhub

import (
	"context"
	"time"
)

// EventType represents the type of event
type EventType string

// Event type constants
const (
	EventTypeAPI         EventType = "API"
	EventTypeCertificate EventType = "CERTIFICATE"
	EventTypeLLMTemplate EventType = "LLM_TEMPLATE"
)

// Event represents a single event in the hub
type Event struct {
	OrganizationID      string    // Organization this event belongs to
	ProcessedTimestamp  time.Time // When event was recorded in DB
	OriginatedTimestamp time.Time // When event was created
	EventType           EventType // Type of event (API, CERTIFICATE, etc.)
	Action              string    // CREATE, UPDATE, or DELETE
	EntityID            string    // ID of the affected entity
	CorrelationID       string    // Correlation ID for request tracing
	EventData           []byte    // JSON serialized payload
}

// OrganizationState represents the version state for an organization
type OrganizationState struct {
	Organization string    // Organization ID
	VersionID    string    // UUID that changes on every modification
	UpdatedAt    time.Time // Last update timestamp
}

// EventHub is the main interface for the message broker
type EventHub interface {
	// Initialize sets up database connections and starts background poller
	Initialize(ctx context.Context) error

	// RegisterOrganization registers an organization for event tracking
	// Creates entry in organization_states table with empty version
	RegisterOrganization(organizationID string) error

	// PublishEvent publishes an event for an organization
	// Updates the organization_states and events tables atomically
	PublishEvent(ctx context.Context, organizationID string, eventType EventType,
		action, entityID, correlationID string, eventData []byte) error

	// Subscribe registers a channel to receive events for an organization
	// Events are delivered as batches (arrays) based on poll cycle
	// Subscriber receives ALL event types and should filter by EventType if needed
	Subscribe(organizationID string, eventChan chan<- []Event) error

	// CleanUpEvents removes events between the specified time range
	CleanUpEvents(ctx context.Context, timeFrom, timeEnd time.Time) error

	// Close gracefully shuts down the EventHub
	Close() error
}


// Config holds EventHub configuration
type Config struct {
	// PollInterval is how often to poll for state changes
	PollInterval time.Duration
	// CleanupInterval is how often to run automatic cleanup
	CleanupInterval time.Duration
	// RetentionPeriod is how long to keep events (default 1 hour)
	RetentionPeriod time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		PollInterval:    time.Second * 5,
		CleanupInterval: time.Minute * 10,
		RetentionPeriod: time.Hour,
	}
}
