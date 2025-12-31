package eventhub

import (
	"context"
	"time"
)

// EventhubImpl is the interface that different message broker implementations must satisfy.
// Implementations can include SQLite (polling-based), NATS, Azure Service Bus, Kafka, etc.
type EventhubImpl interface {
	// Initialize sets up the backend connection and resources.
	// For SQLite: opens database, creates tables
	// For NATS: connects to server, sets up streams
	// For Azure Service Bus: connects to namespace, creates topics
	Initialize(ctx context.Context) error

	// RegisterOrganization creates the necessary resources for tracking an organization.
	// For SQLite: creates entry in organization_states table
	// For NATS: creates subject/stream for the organization
	// For Azure Service Bus: creates topic for the organization
	RegisterOrganization(ctx context.Context, orgID OrganizationID) error

	// Publish publishes an event for an organization.
	// The implementation should ensure delivery semantics appropriate for the broker.
	Publish(ctx context.Context, orgID OrganizationID, eventType EventType,
		action, entityID string, eventData []byte) error

	// Subscribe registers a channel to receive events for an organization.
	// Events are delivered as batches (slices) when available.
	// The subscriber receives ALL event types and should filter if needed.
	Subscribe(orgID OrganizationID, eventChan chan<- []Event) error

	// Unsubscribe removes a subscription channel for an organization.
	Unsubscribe(orgID OrganizationID, eventChan chan<- []Event) error

	// Cleanup removes old events based on retention policy.
	// For SQLite: deletes events older than specified time
	// For message brokers: may be a no-op if broker handles retention
	Cleanup(ctx context.Context, olderThan time.Time) error

	// CleanupRange removes events within a specific time range.
	CleanupRange(ctx context.Context, from, to time.Time) error

	// Close gracefully shuts down the backend.
	Close() error
}

// BackendType represents the type of message broker backend
type BackendType string

const (
	// BackendTypeSQLite uses SQLite with polling for event delivery
	BackendTypeSQLite BackendType = "sqlite"
	// BackendTypeNATS uses NATS JetStream for event delivery
	BackendTypeNATS BackendType = "nats"
	// BackendTypeAzureServiceBus uses Azure Service Bus for event delivery
	BackendTypeAzureServiceBus BackendType = "azure-servicebus"
)

// BackendConfig holds common configuration for all backends
type BackendConfig struct {
	// Type specifies which backend implementation to use
	Type BackendType

	// SQLite-specific configuration
	SQLite *SQLiteBackendConfig

	// NATS-specific configuration (for future use)
	// NATS *NATSBackendConfig

	// Azure Service Bus configuration (for future use)
	// AzureServiceBus *AzureServiceBusConfig
}

// SQLiteBackendConfig holds SQLite-specific configuration
type SQLiteBackendConfig struct {
	// PollInterval is how often to poll for state changes
	PollInterval time.Duration
	// RetentionPeriod is how long to keep events
	RetentionPeriod time.Duration
	// CleanupInterval is how often to run automatic cleanup
	CleanupInterval time.Duration
}

// DefaultSQLiteBackendConfig returns sensible defaults for SQLite backend
func DefaultSQLiteBackendConfig() *SQLiteBackendConfig {
	return &SQLiteBackendConfig{
		PollInterval:    time.Second * 5,
		CleanupInterval: time.Minute * 10,
		RetentionPeriod: time.Hour,
	}
}
