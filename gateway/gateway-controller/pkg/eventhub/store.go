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

// tableExists checks if an events table exists for a topic
func (s *store) tableExists(ctx context.Context, topicName TopicName) (bool, error) {
	tableName := s.getEventsTableName(topicName)

	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
	var name string
	err := s.db.QueryRowContext(ctx, query, tableName).Scan(&name)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}
	return true, nil
}

// getEventsTableName returns the events table name for a topic
func (s *store) getEventsTableName(topicName TopicName) string {
	return fmt.Sprintf("%s_events", string(topicName))
}

// initializeTopicState creates an empty state entry for a topic
func (s *store) initializeTopicState(ctx context.Context, topicName TopicName) error {
	query := `
		INSERT INTO topic_states (topic_name, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(topic_name)
		DO NOTHING
	`

	_, err := s.db.ExecContext(ctx, query, string(topicName), "", time.Now())
	if err != nil {
		return fmt.Errorf("failed to initialize topic state: %w", err)
	}
	return nil
}

// getState retrieves the current state for a topic
func (s *store) getState(ctx context.Context, topicName TopicName) (*TopicState, error) {
	query := `
		SELECT topic_name, version_id, updated_at
		FROM topic_states
		WHERE topic_name = ?
	`

	var state TopicState
	var name string
	err := s.db.QueryRowContext(ctx, query, string(topicName)).Scan(
		&name, &state.VersionID, &state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	state.TopicName = TopicName(name)
	return &state, nil
}

// publishEventAtomic records an event and updates state in a single transaction
func (s *store) publishEventAtomic(ctx context.Context, topicName TopicName, event *Event) (int64, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Insert event
	tableName := s.getEventsTableName(topicName)
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (processed_timestamp, originated_timestamp, event_data)
		VALUES (?, ?, ?)
	`, tableName)

	result, err := tx.ExecContext(ctx, insertQuery,
		event.ProcessedTimestamp,
		event.OriginatedTimestamp,
		event.EventData,
	)
	if err != nil {
		return 0, "", fmt.Errorf("failed to record event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, "", fmt.Errorf("failed to get event ID: %w", err)
	}

	// Step 2: Update state version
	newVersion := uuid.New().String()
	now := time.Now()

	updateQuery := `
		INSERT INTO topic_states (topic_name, version_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(topic_name)
		DO UPDATE SET version_id = excluded.version_id, updated_at = excluded.updated_at
	`

	_, err = tx.ExecContext(ctx, updateQuery, string(topicName), newVersion, now)
	if err != nil {
		return 0, "", fmt.Errorf("failed to update state: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Debug("Published event atomically",
		zap.String("topic", string(topicName)),
		zap.Int64("id", id),
		zap.String("version", newVersion),
	)

	return id, newVersion, nil
}

// getEventsSince retrieves events after a given timestamp
func (s *store) getEventsSince(ctx context.Context, topicName TopicName, since time.Time) ([]Event, error) {
	tableName := s.getEventsTableName(topicName)

	query := fmt.Sprintf(`
		SELECT id, processed_timestamp, originated_timestamp, event_data
		FROM %s
		WHERE processed_timestamp > ?
		ORDER BY processed_timestamp ASC
	`, tableName)

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		e.TopicName = topicName
		if err := rows.Scan(&e.ID, &e.ProcessedTimestamp, &e.OriginatedTimestamp, &e.EventData); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// cleanupEvents removes events within the specified time range
func (s *store) cleanupEvents(ctx context.Context, topicName TopicName, timeFrom, timeEnd time.Time) (int64, error) {
	tableName := s.getEventsTableName(topicName)

	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE processed_timestamp >= ? AND processed_timestamp <= ?
	`, tableName)

	result, err := s.db.ExecContext(ctx, query, timeFrom, timeEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup events: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get deleted count: %w", err)
	}

	s.logger.Info("Cleaned up events",
		zap.String("topic", string(topicName)),
		zap.Int64("deleted", deleted),
		zap.Time("from", timeFrom),
		zap.Time("to", timeEnd),
	)

	return deleted, nil
}

// cleanupAllTopics removes old events from all known topics
func (s *store) cleanupAllTopics(ctx context.Context, olderThan time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT topic_name FROM topic_states`)
	if err != nil {
		return fmt.Errorf("failed to query topics: %w", err)
	}
	defer rows.Close()

	var topics []TopicName
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to scan topic name: %w", err)
		}
		topics = append(topics, TopicName(name))
	}

	for _, topic := range topics {
		_, err := s.cleanupEvents(ctx, topic, time.Time{}, olderThan)
		if err != nil {
			s.logger.Warn("Failed to cleanup topic events",
				zap.String("topic", string(topic)),
				zap.Error(err),
			)
		}
	}

	return nil
}
