package eventhub

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// store handles database operations for EventHub
type store struct {
	db     *sql.DB
	logger *zap.Logger
}

// newStore creates a new database store
func newStore(db *sql.DB, logger *zap.Logger) *store {
	return &store{
		db:     db,
		logger: logger,
	}
}

// initializeOrgState creates an empty state entry for an organization
func (s *store) initializeOrgState(ctx context.Context, orgID OrganizationID) error {
	query := `
		INSERT INTO organization_states (organization, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(organization)
		DO NOTHING
	`

	_, err := s.db.ExecContext(ctx, query, string(orgID), "", time.Now())
	if err != nil {
		return fmt.Errorf("failed to initialize organization state: %w", err)
	}
	return nil
}

// getAllStates retrieves all organization states in a single query
func (s *store) getAllStates(ctx context.Context) ([]OrganizationState, error) {
	query := `
		SELECT organization, version_id, updated_at
		FROM organization_states
		ORDER BY organization
	`

	rows, err := s.db.QueryContext(ctx, query)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating states: %w", err)
	}

	return states, nil
}

// getState retrieves the current state for an organization
func (s *store) getState(ctx context.Context, orgID OrganizationID) (*OrganizationState, error) {
	query := `
		SELECT organization, version_id, updated_at
		FROM organization_states
		WHERE organization = ?
	`

	var state OrganizationState
	err := s.db.QueryRowContext(ctx, query, string(orgID)).Scan(
		&state.Organization, &state.VersionID, &state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	return &state, nil
}

// publishEventAtomic records an event and updates state in a single transaction
func (s *store) publishEventAtomic(ctx context.Context, orgID OrganizationID, eventType EventType,
	action, entityID string, eventData []byte) (string, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Step 1: Insert event into unified events table
	insertQuery := `
		INSERT INTO events (organization_id, processed_timestamp, originated_timestamp,
		                   event_type, action, entity_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.ExecContext(ctx, insertQuery,
		string(orgID),
		now,
		now,
		string(eventType),
		action,
		entityID,
		eventData,
	)
	if err != nil {
		return "", fmt.Errorf("failed to record event: %w", err)
	}

	// Step 2: Update organization state version
	newVersion := uuid.New().String()

	updateQuery := `
		INSERT INTO organization_states (organization, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(organization)
		DO UPDATE SET version_id = excluded.version_id, updated_at = excluded.updated_at
	`

	_, err = tx.ExecContext(ctx, updateQuery, string(orgID), newVersion, now)
	if err != nil {
		return "", fmt.Errorf("failed to update state: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Debug("Published event atomically",
		zap.String("organization", string(orgID)),
		zap.String("eventType", string(eventType)),
		zap.String("action", action),
		zap.String("entityID", entityID),
		zap.String("version", newVersion),
	)

	return newVersion, nil
}

// getEventsSince retrieves events for an organization after a given timestamp
func (s *store) getEventsSince(ctx context.Context, orgID OrganizationID, since time.Time) ([]Event, error) {
	query := `
		SELECT processed_timestamp, originated_timestamp, event_type,
		       action, entity_id, event_data
		FROM events
		WHERE organization_id = ? AND processed_timestamp > ?
		ORDER BY processed_timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, string(orgID), since)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// cleanupEvents removes events from the unified table within the specified time range
func (s *store) cleanupEvents(ctx context.Context, timeFrom, timeEnd time.Time) (int64, error) {
	query := `
		DELETE FROM events
		WHERE processed_timestamp >= ? AND processed_timestamp <= ?
	`

	result, err := s.db.ExecContext(ctx, query, timeFrom, timeEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup events: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get deleted count: %w", err)
	}

	s.logger.Info("Cleaned up events",
		zap.Int64("deleted", deleted),
		zap.Time("from", timeFrom),
		zap.Time("to", timeEnd),
	)

	return deleted, nil
}

// cleanupAllOrganizations removes old events from the unified events table
func (s *store) cleanupAllOrganizations(ctx context.Context, olderThan time.Time) error {
	query := `
		DELETE FROM events
		WHERE processed_timestamp < ?
	`

	result, err := s.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old events: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get deleted count: %w", err)
	}

	s.logger.Info("Cleaned up old events across all organizations",
		zap.Int64("deleted", deleted),
		zap.Time("olderThan", olderThan),
	)

	return nil
}
