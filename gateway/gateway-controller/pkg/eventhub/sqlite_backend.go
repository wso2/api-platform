package eventhub

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// statementKey identifies a prepared statement for re-preparation
type statementKey int

const (
	stmtKeyGetAllStates statementKey = iota
	stmtKeyGetEventsSince
	stmtKeyInsertEvent
	stmtKeyUpdateOrgState
	stmtKeyInsertOrgState
	stmtKeyCleanup
	stmtKeyCleanupRange
)

// SQLiteBackend implements the Backend interface using SQLite with polling
type SQLiteBackend struct {
	db       *sql.DB
	config   *SQLiteBackendConfig
	logger   *zap.Logger
	registry *organizationRegistry

	// Polling state
	pollerCtx     context.Context
	pollerCancel  context.CancelFunc
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
	wg            sync.WaitGroup

	// Prepared statements for performance
	stmtGetAllStates   *sql.Stmt
	stmtGetEventsSince *sql.Stmt
	stmtInsertEvent    *sql.Stmt
	stmtUpdateOrgState *sql.Stmt
	stmtInsertOrgState *sql.Stmt
	stmtCleanup        *sql.Stmt
	stmtCleanupRange   *sql.Stmt

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

	// Prepare statements for performance
	if err := b.prepareStatements(); err != nil {
		return fmt.Errorf("failed to prepare statements: %w", err)
	}

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

// ensureInitialized checks if the backend is initialized and returns an error if not
func (b *SQLiteBackend) ensureInitialized() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.initialized {
		return fmt.Errorf("SQLite backend not initialized")
	}
	return nil
}

// getStatement returns the prepared statement for the given key (caller must hold at least RLock)
func (b *SQLiteBackend) getStatement(key statementKey) *sql.Stmt {
	switch key {
	case stmtKeyGetAllStates:
		return b.stmtGetAllStates
	case stmtKeyGetEventsSince:
		return b.stmtGetEventsSince
	case stmtKeyInsertEvent:
		return b.stmtInsertEvent
	case stmtKeyUpdateOrgState:
		return b.stmtUpdateOrgState
	case stmtKeyInsertOrgState:
		return b.stmtInsertOrgState
	case stmtKeyCleanup:
		return b.stmtCleanup
	case stmtKeyCleanupRange:
		return b.stmtCleanupRange
	default:
		return nil
	}
}

// setStatement sets the prepared statement for the given key (caller must hold Lock)
func (b *SQLiteBackend) setStatement(key statementKey, stmt *sql.Stmt) {
	switch key {
	case stmtKeyGetAllStates:
		b.stmtGetAllStates = stmt
	case stmtKeyGetEventsSince:
		b.stmtGetEventsSince = stmt
	case stmtKeyInsertEvent:
		b.stmtInsertEvent = stmt
	case stmtKeyUpdateOrgState:
		b.stmtUpdateOrgState = stmt
	case stmtKeyInsertOrgState:
		b.stmtInsertOrgState = stmt
	case stmtKeyCleanup:
		b.stmtCleanup = stmt
	case stmtKeyCleanupRange:
		b.stmtCleanupRange = stmt
	}
}

// prepareStatement prepares a single statement by key
func (b *SQLiteBackend) prepareStatement(key statementKey) (*sql.Stmt, error) {
	var query string
	switch key {
	case stmtKeyGetAllStates:
		query = `
			SELECT organization, version_id, updated_at
			FROM organization_states
			ORDER BY organization
		`
	case stmtKeyGetEventsSince:
		query = `
			SELECT processed_timestamp, originated_timestamp, event_type,
			       action, entity_id, correlation_id, event_data
			FROM events
			WHERE organization_id = ? AND processed_timestamp > ?
			ORDER BY processed_timestamp ASC
		`
	case stmtKeyInsertEvent:
		query = `
			INSERT INTO events (organization_id, processed_timestamp, originated_timestamp,
			                   event_type, action, entity_id, correlation_id, event_data)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
	case stmtKeyUpdateOrgState:
		query = `
			INSERT INTO organization_states (organization, version_id, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT(organization)
			DO UPDATE SET version_id = excluded.version_id, updated_at = excluded.updated_at
		`
	case stmtKeyInsertOrgState:
		query = `
			INSERT INTO organization_states (organization, version_id, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT(organization)
			DO NOTHING
		`
	case stmtKeyCleanup:
		query = `DELETE FROM events WHERE processed_timestamp < ?`
	case stmtKeyCleanupRange:
		query = `DELETE FROM events WHERE processed_timestamp >= ? AND processed_timestamp <= ?`
	default:
		return nil, fmt.Errorf("unknown statement key: %d", key)
	}

	stmt, err := b.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement (key=%d): %w", key, err)
	}
	return stmt, nil
}

// isRecoverableError checks if an error indicates a statement needs re-preparation
func (b *SQLiteBackend) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// SQLite schema change errors indicate statements need re-preparation
	return strings.Contains(errStr, "schema") ||
		strings.Contains(errStr, "SQLITE_SCHEMA")
}

// execWithRetry executes a prepared statement with automatic re-preparation on recoverable errors
func (b *SQLiteBackend) execWithRetry(ctx context.Context, key statementKey, args ...any) (sql.Result, error) {
	b.mu.RLock()
	stmt := b.getStatement(key)
	b.mu.RUnlock()

	if stmt == nil {
		return nil, fmt.Errorf("statement not initialized (key=%d)", key)
	}

	result, err := stmt.ExecContext(ctx, args...)
	if err != nil && b.isRecoverableError(err) {
		// Re-prepare and retry once
		b.logger.Warn("Statement execution failed with recoverable error, re-preparing",
			zap.Int("statementKey", int(key)),
			zap.Error(err))

		b.mu.Lock()
		newStmt, prepErr := b.prepareStatement(key)
		if prepErr == nil {
			// Close old statement
			if oldStmt := b.getStatement(key); oldStmt != nil {
				_ = oldStmt.Close()
			}
			b.setStatement(key, newStmt)
			stmt = newStmt
		}
		b.mu.Unlock()

		if prepErr != nil {
			return nil, fmt.Errorf("re-preparation failed after recoverable error: %w (original: %v)", prepErr, err)
		}

		// Retry with new statement
		result, err = stmt.ExecContext(ctx, args...)
	}
	return result, err
}

// queryWithRetry executes a prepared query with automatic re-preparation on recoverable errors
func (b *SQLiteBackend) queryWithRetry(ctx context.Context, key statementKey, args ...any) (*sql.Rows, error) {
	b.mu.RLock()
	stmt := b.getStatement(key)
	b.mu.RUnlock()

	if stmt == nil {
		return nil, fmt.Errorf("statement not initialized (key=%d)", key)
	}

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil && b.isRecoverableError(err) {
		// Re-prepare and retry once
		b.logger.Warn("Statement query failed with recoverable error, re-preparing",
			zap.Int("statementKey", int(key)),
			zap.Error(err))

		b.mu.Lock()
		newStmt, prepErr := b.prepareStatement(key)
		if prepErr == nil {
			// Close old statement
			if oldStmt := b.getStatement(key); oldStmt != nil {
				_ = oldStmt.Close()
			}
			b.setStatement(key, newStmt)
			stmt = newStmt
		}
		b.mu.Unlock()

		if prepErr != nil {
			return nil, fmt.Errorf("re-preparation failed after recoverable error: %w (original: %v)", prepErr, err)
		}

		// Retry with new statement
		rows, err = stmt.QueryContext(ctx, args...)
	}
	return rows, err
}

// prepareStatements prepares all frequently-used SQL statements for performance
func (b *SQLiteBackend) prepareStatements() (err error) {
	// Clean up any successfully prepared statements if we fail partway through
	defer func() {
		if err != nil {
			b.closeStatements()
		}
	}()

	// Prepare getAllStates query
	b.stmtGetAllStates, err = b.db.Prepare(`
		SELECT organization, version_id, updated_at
		FROM organization_states
		ORDER by organization
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare getAllStates: %w", err)
	}

	// Prepare getEventsSince query
	b.stmtGetEventsSince, err = b.db.Prepare(`
		SELECT processed_timestamp, originated_timestamp, event_type,
		       action, entity_id, correlation_id, event_data
		FROM events
		WHERE organization_id = ? AND processed_timestamp > ?
		ORDER BY processed_timestamp ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare getEventsSince: %w", err)
	}

	// Prepare insertEvent query
	b.stmtInsertEvent, err = b.db.Prepare(`
		INSERT INTO events (organization_id, processed_timestamp, originated_timestamp,
		                   event_type, action, entity_id, correlation_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insertEvent: %w", err)
	}

	// Prepare updateOrgState query
	b.stmtUpdateOrgState, err = b.db.Prepare(`
		INSERT INTO organization_states (organization, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(organization)
		DO UPDATE SET version_id = excluded.version_id, updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare updateOrgState: %w", err)
	}

	// Prepare insertOrgState query (for RegisterOrganization)
	b.stmtInsertOrgState, err = b.db.Prepare(`
		INSERT INTO organization_states (organization, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(organization)
		DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insertOrgState: %w", err)
	}

	// Prepare cleanup query
	b.stmtCleanup, err = b.db.Prepare(`DELETE FROM events WHERE processed_timestamp < ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare cleanup: %w", err)
	}

	// Prepare cleanupRange query
	b.stmtCleanupRange, err = b.db.Prepare(`DELETE FROM events WHERE processed_timestamp >= ? AND processed_timestamp <= ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare cleanupRange: %w", err)
	}

	b.logger.Info("Prepared statements initialized successfully")
	return nil
}

// RegisterOrganization creates the necessary resources for an organization
func (b *SQLiteBackend) RegisterOrganization(ctx context.Context, orgID string) error {
	if err := b.ensureInitialized(); err != nil {
		return err
	}

	// Register in local registry
	if err := b.registry.register(orgID); err != nil {
		return err
	}

	// Initialize state in database using prepared statement with retry
	_, err := b.execWithRetry(ctx, stmtKeyInsertOrgState, string(orgID), "", time.Now())
	if err != nil {
		return fmt.Errorf("failed to initialize organization state: %w", err)
	}

	b.logger.Info("Organization registered in SQLite backend",
		zap.String("organization", string(orgID)),
	)
	return nil
}

// Publish publishes an event for an organization
func (b *SQLiteBackend) Publish(ctx context.Context, orgID string,
	eventType EventType, action, entityID, correlationID string, eventData []byte) error {

	if err := b.ensureInitialized(); err != nil {
		return err
	}

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

	// Use prepared statements within transaction
	// Note: For transaction-bound statements, we use tx.Stmt() to get transaction-specific handles
	// Retry logic doesn't apply within transactions as the transaction would need to be restarted
	b.mu.RLock()
	txStmtInsertEvent := tx.Stmt(b.stmtInsertEvent)
	txStmtUpdateOrgState := tx.Stmt(b.stmtUpdateOrgState)
	b.mu.RUnlock()

	// Insert event using prepared statement
	_, err = txStmtInsertEvent.ExecContext(ctx,
		string(orgID), now, now, string(eventType), action, entityID, correlationID, eventData)
	if err != nil {
		return fmt.Errorf("failed to record event: %w", err)
	}

	// Update organization state version using prepared statement
	newVersion := uuid.New().String()
	_, err = txStmtUpdateOrgState.ExecContext(ctx, string(orgID), newVersion, now)
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
		zap.String("correlationID", correlationID),
		zap.String("version", newVersion),
	)

	return nil
}

// Subscribe registers a channel to receive events for an organization
func (b *SQLiteBackend) Subscribe(orgID string, eventChan chan<- []Event) error {
	if err := b.registry.addSubscriber(orgID, eventChan); err != nil {
		return err
	}

	b.logger.Info("Subscription registered",
		zap.String("organization", string(orgID)),
	)
	return nil
}

// Unsubscribe removes a subscription channel for an organization
func (b *SQLiteBackend) Unsubscribe(orgID string, eventChan chan<- []Event) error {
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
	if err := b.ensureInitialized(); err != nil {
		return err
	}

	result, err := b.execWithRetry(ctx, stmtKeyCleanup, olderThan)
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
	if err := b.ensureInitialized(); err != nil {
		return err
	}

	result, err := b.execWithRetry(ctx, stmtKeyCleanupRange, from, to)
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

	// Close all prepared statements
	b.closeStatements()

	b.initialized = false
	b.logger.Info("SQLite backend shutdown complete")
	return nil
}

// closeStatements closes all prepared statements
func (b *SQLiteBackend) closeStatements() {
	statements := []*sql.Stmt{
		b.stmtGetAllStates,
		b.stmtGetEventsSince,
		b.stmtInsertEvent,
		b.stmtUpdateOrgState,
		b.stmtInsertOrgState,
		b.stmtCleanup,
		b.stmtCleanupRange,
	}

	for _, stmt := range statements {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				b.logger.Warn("Failed to close prepared statement", zap.Error(err))
			}
		}
	}
	b.logger.Info("Prepared statements closed successfully")
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
		orgID := state.Organization

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
			if b.deliverEvents(org, events) == nil {
				org.updatePollState(state.VersionID, time.Now())
			}
			// If delivery failed (channel full), don't update timestamp
			// so events will be retried on next poll
		} else {
			org.updatePollState(state.VersionID, time.Now())
		}
	}
}

// getAllStates retrieves all organization states
func (b *SQLiteBackend) getAllStates(ctx context.Context) ([]OrganizationState, error) {
	if err := b.ensureInitialized(); err != nil {
		return nil, err
	}

	rows, err := b.queryWithRetry(ctx, stmtKeyGetAllStates)
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
func (b *SQLiteBackend) getEventsSince(ctx context.Context, orgID string, since time.Time) ([]Event, error) {
	if err := b.ensureInitialized(); err != nil {
		return nil, err
	}

	// TODO: (VirajSalaka) Implement pagination if large number of events
	rows, err := b.queryWithRetry(ctx, stmtKeyGetEventsSince, string(orgID), since)
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
			&eventTypeStr, &e.Action, &e.EntityID, &e.CorrelationID, &e.EventData); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		e.EventType = EventType(eventTypeStr)
		events = append(events, e)
	}
	return events, rows.Err()
}

// deliverEvents sends events to all subscribers of an organization
func (b *SQLiteBackend) deliverEvents(org *organization, events []Event) error {
	subscribers := org.getSubscribers()

	if len(subscribers) == 0 {
		b.logger.Debug("No subscribers for organization",
			zap.String("organization", string(org.id)),
			zap.Int("events", len(events)),
		)
		return nil
	}

	// TODO: (VirajSalaka) One subscriber is considered here. Handle multiple subscribers properly.
	for _, ch := range subscribers {
		select {
		case ch <- events:
			b.logger.Debug("Delivered events to subscriber",
				zap.String("organization", string(org.id)),
				zap.Int("events", len(events)),
			)
		default:
			b.logger.Error("Subscriber channel full, dropping events",
				zap.String("organization", string(org.id)),
				zap.Int("events", len(events)),
			)
			return fmt.Errorf("subscriber channel full")
		}
	}
	return nil
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
