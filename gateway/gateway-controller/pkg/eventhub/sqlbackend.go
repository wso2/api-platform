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

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	// initialPollSkewWindow limits first-time replay to a recent window instead of full history.
	initialPollSkewWindow = 120 * time.Second
	// Thresholds used to infer unix timestamp units (seconds/millis/micros/nanos).
	unixMillisThreshold = int64(100_000_000_000)
	unixMicrosThreshold = int64(100_000_000_000_000)
	unixNanosThreshold  = int64(100_000_000_000_000_000)
)

// SQLBackend implements EventhubImpl using SQL polling
type SQLBackend struct {
	db       *sql.DB
	logger   *slog.Logger
	config   SQLBackendConfig
	bindType int

	registry *organizationRegistry

	// Prepared statements
	stmtMu               sync.RWMutex
	insertEventStmt      *sql.Stmt
	updateOrgVersionStmt *sql.Stmt
	getOrgStateStmt      *sql.Stmt
	getOrgStatesPageStmt *sql.Stmt
	getEventsStmt        *sql.Stmt
	insertOrgStmt        *sql.Stmt
	cleanupEventsStmt    *sql.Stmt

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// unixTimestampToTime converts unix timestamps in seconds, milliseconds,
// microseconds, or nanoseconds to time.Time.
func unixTimestampToTime(ts int64) time.Time {
	switch {
	case ts >= unixNanosThreshold:
		return time.Unix(0, ts)
	case ts >= unixMicrosThreshold:
		return time.UnixMicro(ts)
	case ts >= unixMillisThreshold:
		return time.UnixMilli(ts)
	default:
		return time.Unix(ts, 0)
	}
}

// NewSQLBackend creates a new SQL-backed event hub
func NewSQLBackend(db *sql.DB, logger *slog.Logger, config SQLBackendConfig) *SQLBackend {
	ctx, cancel := context.WithCancel(context.Background())
	return &SQLBackend{
		db:       db,
		logger:   logger,
		config:   config,
		bindType: bindTypeForDB(db),
		registry: newOrganizationRegistry(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func bindTypeForDB(db *sql.DB) int {
	if db == nil {
		return sqlx.QUESTION
	}

	switch db.Driver().(type) {
	case *stdlib.Driver:
		return sqlx.DOLLAR
	default:
		return sqlx.QUESTION
	}
}

func (b *SQLBackend) rebind(query string) string {
	return sqlx.Rebind(b.bindType, query)
}

// Initialize prepares statements and starts background goroutines
func (b *SQLBackend) Initialize() error {
	if err := b.prepareStatements(); err != nil {
		return fmt.Errorf("failed to prepare statements: %w", err)
	}

	// Start poll loop
	b.wg.Add(1)
	go b.pollLoop()

	// Start cleanup loop
	b.wg.Add(1)
	go b.cleanupLoop()

	b.logger.Info("SQL event hub backend initialized",
		slog.Duration("poll_interval", b.config.PollInterval),
		slog.Duration("cleanup_interval", b.config.CleanupInterval),
		slog.Duration("retention_period", b.config.RetentionPeriod))

	return nil
}

// closeStatements closes any non-nil prepared statements
func (b *SQLBackend) closeStatements() {
	stmts := []*sql.Stmt{
		b.insertEventStmt,
		b.updateOrgVersionStmt,
		b.getOrgStateStmt,
		b.getOrgStatesPageStmt,
		b.getEventsStmt,
		b.insertOrgStmt,
		b.cleanupEventsStmt,
	}
	for _, stmt := range stmts {
		if stmt != nil {
			stmt.Close()
		}
	}
}

// prepareStatements prepares SQL statements for reuse
func (b *SQLBackend) prepareStatements() (err error) {
	defer func() {
		if err != nil {
			b.closeStatements()
		}
	}()

	b.insertEventStmt, err = b.db.Prepare(b.rebind(`
		INSERT INTO events (organization_id, processed_timestamp, originated_timestamp, event_type, action, entity_id, correlation_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare insert event statement: %w", err)
	}

	b.updateOrgVersionStmt, err = b.db.Prepare(b.rebind(`
		UPDATE organization_states SET version_id = ?, updated_at = CURRENT_TIMESTAMP WHERE organization = ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare update org version statement: %w", err)
	}

	b.getOrgStateStmt, err = b.db.Prepare(b.rebind(`
		SELECT organization, version_id, updated_at FROM organization_states WHERE organization = ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get org state statement: %w", err)
	}

	b.getOrgStatesPageStmt, err = b.db.Prepare(b.rebind(`
		SELECT organization, version_id, updated_at
		FROM organization_states
		WHERE organization > ?
		ORDER BY organization ASC
		LIMIT ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get org states page statement: %w", err)
	}

	b.getEventsStmt, err = b.db.Prepare(b.rebind(`
		SELECT organization_id, processed_timestamp, originated_timestamp, event_type, action, entity_id, correlation_id, event_data
		FROM events
		WHERE organization_id = ? AND processed_timestamp > ?
		ORDER BY processed_timestamp ASC
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get events statement: %w", err)
	}

	b.insertOrgStmt, err = b.db.Prepare(b.rebind(`
		INSERT INTO organization_states (organization, version_id) VALUES (?, '')
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare insert org statement: %w", err)
	}

	b.cleanupEventsStmt, err = b.db.Prepare(b.rebind(`
		DELETE FROM events WHERE processed_timestamp < ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare cleanup events statement: %w", err)
	}

	return nil
}

// RegisterOrganization registers a new organization for event tracking
func (b *SQLBackend) RegisterOrganization(orgID string) error {
	_, err := b.insertOrgStmt.Exec(orgID)
	if err != nil {
		// Insert failed — check if a concurrent registration already inserted the org.
		checkErr := b.getOrgStateStmt.QueryRow(orgID).Scan(new(string), new(string), new(time.Time))
		if checkErr == nil {
			// Org exists; concurrent insert won the race — treat as success.
			err = nil
		} else if checkErr != sql.ErrNoRows {
			return fmt.Errorf("failed to check organization existence after insert failure: %w", checkErr)
		} else {
			return fmt.Errorf("failed to register organization in database: %w", err)
		}
	}

	// Register in local registry (ignore already exists)
	if regErr := b.registry.register(orgID); regErr != nil && regErr != ErrOrganizationAlreadyExists {
		return fmt.Errorf("failed to register organization in registry: %w", regErr)
	}

	b.logger.Info("Organization registered for event tracking", slog.String("organization", orgID))
	return nil
}

// Publish publishes an event atomically (insert event + update org version)
func (b *SQLBackend) Publish(orgID string, event Event) error {
	newVersion := uuid.New().String()
	eventData := strings.TrimSpace(event.EventData)
	if eventData == "" {
		eventData = EmptyEventData
	}

	tx, err := b.db.BeginTx(b.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert event (explicitly pass processed_timestamp to ensure consistent time format with Go driver)
	_, err = tx.Stmt(b.insertEventStmt).Exec(
		orgID,
		time.Now(),
		event.OriginatedTimestamp,
		string(event.EventType),
		event.Action,
		event.EntityID,
		event.CorrelationID,
		eventData,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	// Update organization version
	_, err = tx.Stmt(b.updateOrgVersionStmt).Exec(newVersion, orgID)
	if err != nil {
		return fmt.Errorf("failed to update organization version: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit event publish: %w", err)
	}

	b.logger.Debug("Event published",
		slog.String("organization", orgID),
		slog.String("event_type", string(event.EventType)),
		slog.String("action", event.Action),
		slog.String("entity_id", event.EntityID),
		slog.String("new_version", newVersion))

	return nil
}

// Subscribe subscribes to events for an organization
func (b *SQLBackend) Subscribe(orgID string) (<-chan Event, error) {
	ch := make(chan Event, 100)

	if err := b.registry.addSubscriber(orgID, ch); err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to subscribe to organization %s: %w", orgID, err)
	}

	b.logger.Info("Subscribed to organization events", slog.String("organization", orgID))
	return ch, nil
}

// Unsubscribe removes the subscription for an organization
func (b *SQLBackend) Unsubscribe(orgID string) error {
	org, err := b.registry.get(orgID)
	if err != nil {
		return err
	}

	// Close and remove all subscribers
	b.registry.mu.Lock()
	defer b.registry.mu.Unlock()

	for _, ch := range org.subscribers {
		close(ch)
	}
	org.subscribers = nil

	return nil
}

// pollLoop periodically checks for new events
func (b *SQLBackend) pollLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.pollOrganizations()
		}
	}
}

// pollOrganizations checks each registered organization for version changes
func (b *SQLBackend) pollOrganizations() {
	orgByID := make(map[string]*organization)
	for _, org := range b.registry.getAll() {
		orgByID[org.id] = org
	}
	if len(orgByID) == 0 {
		return
	}

	pageSize := b.organizationStatePageSize()
	cursor := ""
	for {
		states, nextCursor, err := b.getOrganizationStatesPage(cursor, pageSize)
		if err != nil {
			b.logger.Warn("Failed to poll organization states page",
				slog.String("cursor", cursor),
				slog.Any("error", err))
			return
		}
		if len(states) == 0 {
			return
		}

		for _, state := range states {
			org, ok := orgByID[state.Organization]
			if !ok {
				continue
			}
			if err := b.pollOrganizationWithState(org, state); err != nil {
				b.logger.Warn("Failed to poll organization",
					slog.String("organization", org.id),
					slog.Any("error", err))
			}
		}

		if len(states) < pageSize {
			return
		}
		cursor = nextCursor
	}
}

func (b *SQLBackend) organizationStatePageSize() int {
	if b.config.OrganizationStatePageSize > 0 {
		return b.config.OrganizationStatePageSize
	}
	return defaultOrganizationStatePageSize
}

func subscriberChannelsAvailable(subscribers []chan Event) bool {
	for _, ch := range subscribers {
		// Subscriber channels are buffered. If any buffer is full, stop
		// advancing so the next poll can retry the same event batch.
		if len(ch) == cap(ch) {
			return false
		}
	}
	return true
}

func (b *SQLBackend) getOrganizationStatesPage(cursor string, limit int) ([]OrganizationState, string, error) {
	// TODO: (VirajSalaka) We can even optimize this by only selecting organizations that have had updates since the last poll time,
	// but that would require tracking last poll time per organization which adds complexity. For now, we can rely on the fact that
	// organizations with no changes will be quickly skipped in pollOrganizationWithState.
	rows, err := b.getOrgStatesPageStmt.Query(cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query organization states page: %w", err)
	}
	defer rows.Close()

	states := make([]OrganizationState, 0, limit)
	nextCursor := ""
	for rows.Next() {
		var state OrganizationState
		if err := rows.Scan(&state.Organization, &state.VersionID, &state.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("failed to scan organization state row: %w", err)
		}
		states = append(states, state)
		nextCursor = state.Organization
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("error iterating organization state rows: %w", err)
	}

	return states, nextCursor, nil
}

func (b *SQLBackend) pollOrganizationWithState(org *organization, state OrganizationState) error {
	// Check if version has changed
	b.registry.mu.RLock()
	knownVersion := org.knownVersion
	lastPolled := org.lastPolled
	subscribers := make([]chan Event, len(org.subscribers))
	copy(subscribers, org.subscribers)
	b.registry.mu.RUnlock()

	if state.VersionID == knownVersion || state.VersionID == "" {
		return nil // No changes
	}

	// Fetch new events since last poll
	var lastPolledTime time.Time
	if lastPolled > 0 {
		lastPolledTime = unixTimestampToTime(lastPolled)
	} else {
		// First poll - only replay a short recent window to avoid catching full history.
		lastPolledTime = time.Now().Add(-initialPollSkewWindow)
	}

	rows, err := b.getEventsStmt.Query(org.id, lastPolledTime)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var evt Event
		var eventType string
		if err := rows.Scan(
			&evt.OrganizationID,
			&evt.ProcessedTimestamp,
			&evt.OriginatedTimestamp,
			&eventType,
			&evt.Action,
			&evt.EntityID,
			&evt.CorrelationID,
			&evt.EventData,
		); err != nil {
			return fmt.Errorf("failed to scan event row: %w", err)
		}
		evt.EventType = EventType(eventType)
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating event rows: %w", err)
	}

	// TODO: (VirajSalaka) In the initial startup, we fetch the past events for 120 seconds.
	// But if there are lot of events during the period, we need to capture the tail events.

	// Deliver events to subscribers. If any subscriber buffer is full, stop at
	// the first blocked event so the next poll resumes from the last delivered one.
	var latestDeliveredTimestamp time.Time
	deliveredCount := 0
	deliveryBlocked := false
	for _, evt := range events {
		if !subscriberChannelsAvailable(subscribers) {
			deliveryBlocked = true
			b.logger.Warn("Subscriber channel full, deferring event delivery",
				slog.String("organization", org.id),
				slog.String("entity_id", evt.EntityID))
			break
		}
		for _, ch := range subscribers {
			ch <- evt
		}
		latestDeliveredTimestamp = evt.ProcessedTimestamp
		deliveredCount++
	}

	// Update known version and last polled time
	b.registry.mu.Lock()
	if deliveryBlocked {
		if !latestDeliveredTimestamp.IsZero() {
			org.lastPolled = latestDeliveredTimestamp.UnixNano()
		} else {
			org.lastPolled = lastPolledTime.UnixNano()
		}
	} else {
		org.knownVersion = state.VersionID
		if !latestDeliveredTimestamp.IsZero() {
			org.lastPolled = latestDeliveredTimestamp.UnixNano()
		}
	}
	b.registry.mu.Unlock()

	if deliveredCount > 0 {
		b.logger.Debug("Delivered events to subscribers",
			slog.String("organization", org.id),
			slog.Int("event_count", deliveredCount),
			slog.Int("subscriber_count", len(subscribers)))
	}

	return nil
}

// cleanupLoop periodically removes old events
func (b *SQLBackend) cleanupLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if err := b.Cleanup(b.config.RetentionPeriod); err != nil {
				b.logger.Warn("Failed to clean up events", slog.Any("error", err))
			}
		}
	}
}

// Cleanup removes events older than the retention period
func (b *SQLBackend) Cleanup(retentionPeriod time.Duration) error {
	cutoff := time.Now().Add(-retentionPeriod)
	result, err := b.cleanupEventsStmt.Exec(cutoff)
	if err != nil {
		return fmt.Errorf("failed to clean up events: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		b.logger.Info("Cleaned up old events", slog.Int64("deleted_count", affected))
	}
	return nil
}

// CleanupRange removes events for an organization before a given time
func (b *SQLBackend) CleanupRange(orgID string, before time.Time) error {
	_, err := b.db.Exec(
		b.rebind("DELETE FROM events WHERE organization_id = ? AND processed_timestamp < ?"),
		orgID, before,
	)
	if err != nil {
		return fmt.Errorf("failed to clean up events for organization %s: %w", orgID, err)
	}
	return nil
}

// Close gracefully shuts down the backend
func (b *SQLBackend) Close() error {
	b.cancel()
	b.wg.Wait()

	// Close all subscriber channels
	for _, org := range b.registry.getAll() {
		b.registry.mu.Lock()
		for _, ch := range org.subscribers {
			close(ch)
		}
		org.subscribers = nil
		b.registry.mu.Unlock()
	}

	// Close prepared statements
	b.stmtMu.Lock()
	defer b.stmtMu.Unlock()
	b.closeStatements()

	b.logger.Info("SQL event hub backend closed")
	return nil
}
