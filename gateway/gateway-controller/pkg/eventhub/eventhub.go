package eventhub

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"
)

// eventHub is the main implementation of EventHub interface
// It delegates to a Backend implementation for actual message broker operations
type eventHub struct {
	backend EventhubImpl
	logger  *zap.Logger

	initialized bool
}

// New creates a new EventHub instance with SQLite backend (default)
// This maintains backward compatibility with existing code
func New(db *sql.DB, logger *zap.Logger, config Config) EventHub {
	sqliteConfig := &SQLiteBackendConfig{
		PollInterval:    config.PollInterval,
		CleanupInterval: config.CleanupInterval,
		RetentionPeriod: config.RetentionPeriod,
	}
	backend := NewSQLiteBackend(db, logger, sqliteConfig)
	return &eventHub{
		backend: backend,
		logger:  logger,
	}
}

// NewWithBackend creates a new EventHub instance with a custom backend
// Use this to provide alternative message broker implementations (NATS, Azure Service Bus, etc.)
func NewWithBackend(backend EventhubImpl, logger *zap.Logger) EventHub {
	return &eventHub{
		backend: backend,
		logger:  logger,
	}
}

// Initialize sets up the EventHub and starts background workers
func (eh *eventHub) Initialize(ctx context.Context) error {
	if eh.initialized {
		return nil
	}

	eh.logger.Info("Initializing EventHub")

	if err := eh.backend.Initialize(ctx); err != nil {
		return err
	}

	eh.initialized = true
	eh.logger.Info("EventHub initialized successfully")
	return nil
}

// RegisterOrganization registers a new organization with the EventHub
func (eh *eventHub) RegisterOrganization(organizationID string) error {
	ctx := context.Background()
	return eh.backend.RegisterOrganization(ctx, organizationID)
}

// PublishEvent publishes an event for an organization
func (eh *eventHub) PublishEvent(ctx context.Context, organizationID string,
	eventType EventType, action, entityID, correlationID string, eventData []byte) error {
	return eh.backend.Publish(ctx, organizationID, eventType, action, entityID, correlationID, eventData)
}

// Subscribe registers a channel to receive events for an organization
func (eh *eventHub) Subscribe(organizationID string, eventChan chan<- []Event) error {
	return eh.backend.Subscribe(organizationID, eventChan)
}

// CleanUpEvents removes events from the unified events table within the specified time range
func (eh *eventHub) CleanUpEvents(ctx context.Context, timeFrom, timeEnd time.Time) error {
	return eh.backend.CleanupRange(ctx, timeFrom, timeEnd)
}

// Close gracefully shuts down the EventHub
func (eh *eventHub) Close() error {
	if !eh.initialized {
		return nil
	}

	eh.logger.Info("Shutting down EventHub")

	if err := eh.backend.Close(); err != nil {
		return err
	}

	eh.initialized = false
	eh.logger.Info("EventHub shutdown complete")
	return nil
}
