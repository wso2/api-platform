package eventhub

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SQLiteBackend implements the Backend interface using SQLite with polling
type SQLiteBackend struct {
	db       *sql.DB
	config   *SQLiteBackendConfig
	logger   *zap.Logger
	registry *organizationRegistry

	// Polling state
	pollerCtx    context.Context
	pollerCancel context.CancelFunc
	cleanupCtx   context.Context
	cleanupCancel context.CancelFunc
	wg           sync.WaitGroup

	initialized bool
	mu          sync.RWMutex
}

// NewSQLiteBackend creates a new SQLite-based backend
func NewSQLiteBackend(db *sql.DB, logger *zap.Logger, config *SQLiteBackendConfig) *SQLiteBackend {
	if config == nil {
		config = DefaultSQLiteBackendConfig()
	}
	return &SQLiteBackend{
		db:       db,
		config:   config,
		logger:   logger,
		registry: newOrganizationRegistry(),
	}
}

// Initialize sets up the SQLite backend and starts background workers
func (b *SQLiteBackend) Initialize(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.initialized {
		return nil
	}

	b.logger.Info("Initializing SQLite backend")

	// Start poller
	b.pollerCtx, b.pollerCancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go b.pollLoop()

	// Start cleanup goroutine
	b.cleanupCtx, b.cleanupCancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go b.cleanupLoop()

	b.initialized = true
	b.logger.Info("SQLite backend initialized",
		zap.Duration("pollInterval", b.config.PollInterval),
		zap.Duration("cleanupInterval", b.config.CleanupInterval),
		zap.Duration("retentionPeriod", b.config.RetentionPeriod),
	)
	return nil
}

// RegisterOrganization creates the necessary resources for an organization
func (b *SQLiteBackend) RegisterOrganization(ctx context.Context, orgID OrganizationID) error {
	// Register in local registry
	if err := b.registry.register(orgID); err != nil {
		return err
	}

	// Initialize state in database
	query := `
		INSERT INTO organization_states (organization, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(organization)
		DO NOTHING
	`
	_, err := b.db.ExecContext(ctx, query, string(orgID), "", time.Now())
	if err != nil {
		return fmt.Errorf("failed to initialize organization state: %w", err)
	}

	b.logger.Info("Organization registered in SQLite backend",
		zap.String("organization", string(orgID)),
	)
	return nil
}

// Publish publishes an event for an organization
func (b *SQLiteBackend) Publish(ctx context.Context, orgID OrganizationID,
	eventType EventType, action, entityID string, eventData []byte) error {

	// Verify organization is registered
	_, err := b.registry.get(orgID)
	if err != nil {
		return err
	}

	// Publish atomically (event + state update in transaction)
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Insert event
	insertQuery := `
		INSERT INTO events (organization_id, processed_timestamp, originated_timestamp,
		                   event_type, action, entity_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, insertQuery,
		string(orgID), now, now, string(eventType), action, entityID, eventData)
	if err != nil {
		return fmt.Errorf("failed to record event: %w", err)
	}

	// Update organization state version
	newVersion := uuid.New().String()
	updateQuery := `
		INSERT INTO organization_states (organization, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(organization)
		DO UPDATE SET version_id = excluded.version_id, updated_at = excluded.updated_at
	`
	_, err = tx.ExecContext(ctx, updateQuery, string(orgID), newVersion, now)
	if err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	b.logger.Debug("Event published",
		zap.String("organization", string(orgID)),
		zap.String("eventType", string(eventType)),
		zap.String("action", action),
		zap.String("entityID", entityID),
		zap.String("version", newVersion),
	)

	return nil
}

// Subscribe registers a channel to receive events for an organization
func (b *SQLiteBackend) Subscribe(orgID OrganizationID, eventChan chan<- []Event) error {
	if err := b.registry.addSubscriber(orgID, eventChan); err != nil {
		return err
	}

	b.logger.Info("Subscription registered",
		zap.String("organization", string(orgID)),
	)
	return nil
}

// Unsubscribe removes a subscription channel for an organization
func (b *SQLiteBackend) Unsubscribe(orgID OrganizationID, eventChan chan<- []Event) error {
	if err := b.registry.removeSubscriber(orgID, eventChan); err != nil {
		return err
	}

	b.logger.Info("Subscription removed",
		zap.String("organization", string(orgID)),
	)
	return nil
}

// Cleanup removes old events based on retention policy
func (b *SQLiteBackend) Cleanup(ctx context.Context, olderThan time.Time) error {
	query := `DELETE FROM events WHERE processed_timestamp < ?`
	result, err := b.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old events: %w", err)
	}

	deleted, _ := result.RowsAffected()
	b.logger.Info("Cleaned up old events",
		zap.Int64("deleted", deleted),
		zap.Time("olderThan", olderThan),
	)
	return nil
}

// CleanupRange removes events within a specific time range
func (b *SQLiteBackend) CleanupRange(ctx context.Context, from, to time.Time) error {
	query := `DELETE FROM events WHERE processed_timestamp >= ? AND processed_timestamp <= ?`
	result, err := b.db.ExecContext(ctx, query, from, to)
	if err != nil {
		return fmt.Errorf("failed to cleanup events: %w", err)
	}

	deleted, _ := result.RowsAffected()
	b.logger.Info("Cleaned up events in range",
		zap.Int64("deleted", deleted),
		zap.Time("from", from),
		zap.Time("to", to),
	)
	return nil
}

// Close gracefully shuts down the SQLite backend
func (b *SQLiteBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil
	}

	b.logger.Info("Shutting down SQLite backend")

	// Stop poller
	if b.pollerCancel != nil {
		b.pollerCancel()
	}

	// Stop cleanup loop
	if b.cleanupCancel != nil {
		b.cleanupCancel()
	}

	// Wait for goroutines
	b.wg.Wait()

	b.initialized = false
	b.logger.Info("SQLite backend shutdown complete")
	return nil
}

// pollLoop runs the main polling loop for state changes
func (b *SQLiteBackend) pollLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.PollInterval)
	defer ticker.Stop()

	b.logger.Info("SQLite poller started", zap.Duration("interval", b.config.PollInterval))

	for {
		select {
		case <-b.pollerCtx.Done():
			b.logger.Info("SQLite poller stopped")
			return
		case <-ticker.C:
			b.pollAllOrganizations()
		}
	}
}

// pollAllOrganizations checks all organizations for state changes
func (b *SQLiteBackend) pollAllOrganizations() {
	ctx := b.pollerCtx

	// Single query for ALL organization states
	states, err := b.getAllStates(ctx)
	if err != nil {
		b.logger.Error("Failed to fetch all states", zap.Error(err))
		return
	}

	// Check each organization for changes
	for _, state := range states {
		orgID := OrganizationID(state.Organization)

		org, err := b.registry.get(orgID)
		if err != nil {
			// Organization not registered with subscribers, skip
			continue
		}

		// Check if version changed
		if state.VersionID == org.knownVersion {
			continue
		}

		b.logger.Debug("State change detected",
			zap.String("organization", string(orgID)),
			zap.String("oldVersion", org.knownVersion),
			zap.String("newVersion", state.VersionID),
		)

		// Fetch events since last poll
		events, err := b.getEventsSince(ctx, orgID, org.lastPolled)
		if err != nil {
			b.logger.Error("Failed to fetch events",
				zap.String("organization", string(orgID)),
				zap.Error(err))
			continue
		}

		if len(events) > 0 {
			b.deliverEvents(org, events)
		}

		org.updatePollState(state.VersionID, time.Now())
	}
}

// getAllStates retrieves all organization states
func (b *SQLiteBackend) getAllStates(ctx context.Context) ([]OrganizationState, error) {
	query := `
		SELECT organization, version_id, updated_at
		FROM organization_states
		ORDER BY organization
	`

	rows, err := b.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all states: %w", err)
	}
	defer rows.Close()

	var states []OrganizationState
	for rows.Next() {
		var state OrganizationState
		if err := rows.Scan(&state.Organization, &state.VersionID, &state.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan state: %w", err)
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

// getEventsSince retrieves events for an organization after a given timestamp
func (b *SQLiteBackend) getEventsSince(ctx context.Context, orgID OrganizationID, since time.Time) ([]Event, error) {
	query := `
		SELECT processed_timestamp, originated_timestamp, event_type,
		       action, entity_id, event_data
		FROM events
		WHERE organization_id = ? AND processed_timestamp > ?
		ORDER BY processed_timestamp ASC
	`

	rows, err := b.db.QueryContext(ctx, query, string(orgID), since)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var eventTypeStr string
		e.OrganizationID = orgID

		if err := rows.Scan(&e.ProcessedTimestamp, &e.OriginatedTimestamp,
			&eventTypeStr, &e.Action, &e.EntityID, &e.EventData); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		e.EventType = EventType(eventTypeStr)
		events = append(events, e)
	}
	return events, rows.Err()
}

// deliverEvents sends events to all subscribers of an organization
func (b *SQLiteBackend) deliverEvents(org *organization, events []Event) {
	subscribers := org.getSubscribers()

	if len(subscribers) == 0 {
		b.logger.Debug("No subscribers for organization",
			zap.String("organization", string(org.id)),
			zap.Int("events", len(events)),
		)
		return
	}

	for _, ch := range subscribers {
		select {
		case ch <- events:
			b.logger.Debug("Delivered events to subscriber",
				zap.String("organization", string(org.id)),
				zap.Int("events", len(events)),
			)
		default:
			b.logger.Warn("Subscriber channel full, dropping events",
				zap.String("organization", string(org.id)),
				zap.Int("events", len(events)),
			)
		}
	}
}

// cleanupLoop runs periodic cleanup of old events
func (b *SQLiteBackend) cleanupLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.cleanupCtx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-b.config.RetentionPeriod)
			if err := b.Cleanup(b.cleanupCtx, cutoff); err != nil {
				b.logger.Error("Periodic cleanup failed", zap.Error(err))
			}
		}
	}
}
