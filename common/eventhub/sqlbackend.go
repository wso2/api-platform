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

	registry *gatewayRegistry

	// Prepared statements
	stmtMu                   sync.RWMutex
	insertEventStmt          *sql.Stmt
	updateGatewayVersionStmt *sql.Stmt
	getGatewayStateStmt      *sql.Stmt
	getGatewayStatesPageStmt *sql.Stmt
	getEventsStmt            *sql.Stmt
	getEventByIDStmt         *sql.Stmt
	insertGatewayStmt        *sql.Stmt
	cleanupEventsStmt        *sql.Stmt

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var _ EventhubImpl = (*SQLBackend)(nil)

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
		registry: newGatewayRegistry(),
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
		b.updateGatewayVersionStmt,
		b.getGatewayStateStmt,
		b.getGatewayStatesPageStmt,
		b.getEventsStmt,
		b.getEventByIDStmt,
		b.insertGatewayStmt,
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
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare insert event statement: %w", err)
	}

	b.updateGatewayVersionStmt, err = b.db.Prepare(b.rebind(`
		UPDATE gateway_states SET version_id = ?, updated_at = CURRENT_TIMESTAMP WHERE gateway_id = ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare update gateway version statement: %w", err)
	}

	b.getGatewayStateStmt, err = b.db.Prepare(b.rebind(`
		SELECT gateway_id, version_id, updated_at FROM gateway_states WHERE gateway_id = ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get gateway state statement: %w", err)
	}

	b.getGatewayStatesPageStmt, err = b.db.Prepare(b.rebind(`
		SELECT gateway_id, version_id, updated_at
		FROM gateway_states
		WHERE gateway_id > ?
		ORDER BY gateway_id ASC
		LIMIT ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get gateway states page statement: %w", err)
	}

	b.getEventsStmt, err = b.db.Prepare(b.rebind(`
		SELECT gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data
		FROM events
		WHERE gateway_id = ? AND processed_timestamp >= ?
		ORDER BY processed_timestamp ASC
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get events statement: %w", err)
	}

	b.getEventByIDStmt, err = b.db.Prepare(b.rebind(`
		SELECT event_id FROM events WHERE event_id = ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare get event by ID statement: %w", err)
	}

	b.insertGatewayStmt, err = b.db.Prepare(b.rebind(`
		INSERT INTO gateway_states (gateway_id, version_id) VALUES (?, '')
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare insert gateway statement: %w", err)
	}

	b.cleanupEventsStmt, err = b.db.Prepare(b.rebind(`
		DELETE FROM events WHERE processed_timestamp < ?
	`))
	if err != nil {
		return fmt.Errorf("failed to prepare cleanup events statement: %w", err)
	}

	return nil
}

// RegisterGateway registers a new gateway for event tracking.
func (b *SQLBackend) RegisterGateway(gatewayID string) error {

	gatewayID = strings.TrimSpace(gatewayID)
	if gatewayID == "" {
		return fmt.Errorf("gateway_id cannot be empty")
	}

	_, err := b.insertGatewayStmt.Exec(gatewayID)
	if err != nil {
		// Insert failed; check if a concurrent registration already inserted the gateway.
		checkErr := b.getGatewayStateStmt.QueryRow(gatewayID).Scan(new(string), new(string), new(time.Time))
		if checkErr == nil {
			// Gateway exists; concurrent insert won the race, so treat it as success.
			err = nil
		} else if checkErr != sql.ErrNoRows {
			return fmt.Errorf("failed to check gateway existence after insert failure: %w", checkErr)
		} else {
			return fmt.Errorf("failed to register gateway in database: %w", err)
		}
	}

	// Register in local registry (ignore already exists)
	if regErr := b.registry.register(gatewayID); regErr != nil && regErr != ErrGatewayAlreadyExists {
		return fmt.Errorf("failed to register gateway in registry: %w", regErr)
	}

	b.logger.Info("Gateway registered for event tracking", slog.String("gateway_id", gatewayID))
	return nil
}

// Publish publishes an event atomically (insert event + update gateway version).
func (b *SQLBackend) Publish(gatewayID string, event Event) error {
	// TODO: (VirajSalaka) Make this UUID v7
	newVersion := uuid.New().String()
	eventData := strings.TrimSpace(event.EventData)
	if eventData == "" {
		eventData = EmptyEventData
	}
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		// TODO: (VirajSalaka) Make this UUID v7
		event.EventID = uuid.New().String()
		eventID = event.EventID
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
		gatewayID,
		time.Now(),
		event.OriginatedTimestamp,
		string(event.EventType),
		event.Action,
		event.EntityID,
		eventID,
		eventData,
	)
	if err != nil {
		insertErr := err
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			return fmt.Errorf("failed to rollback event publish after insert failure: %w", rollbackErr)
		}
		err = nil

		eventExists, checkErr := b.eventExists(eventID)
		if checkErr != nil {
			return fmt.Errorf("failed to check event existence after insert failure: %w", checkErr)
		}
		if eventExists {
			b.logger.Info("Event already available, skipping duplicate publish",
				slog.String("gateway_id", gatewayID),
				slog.String("event_id", eventID),
				slog.String("event_type", string(event.EventType)),
				slog.String("action", event.Action),
				slog.String("entity_id", event.EntityID))
			return nil
		}

		return fmt.Errorf("failed to insert event: %w", insertErr)
	}

	// Update gateway version
	result, err := tx.Stmt(b.updateGatewayVersionStmt).Exec(newVersion, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to update gateway version: %w", err)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		err = fmt.Errorf("gateway %q is not registered", gatewayID)
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit event publish: %w", err)
	}

	b.logger.Debug("Event published",
		slog.String("gateway_id", gatewayID),
		slog.String("event_type", string(event.EventType)),
		slog.String("action", event.Action),
		slog.String("entity_id", event.EntityID),
		slog.String("new_version", newVersion))

	return nil
}

func (b *SQLBackend) eventExists(eventID string) (bool, error) {
	var existingEventID string
	err := b.getEventByIDStmt.QueryRow(eventID).Scan(&existingEventID)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

// Subscribe subscribes to events for a gateway.
func (b *SQLBackend) Subscribe(gatewayID string) (<-chan Event, error) {
	ch := make(chan Event, 100)

	if err := b.registry.addSubscriber(gatewayID, ch); err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to subscribe to gateway %s: %w", gatewayID, err)
	}

	b.logger.Info("Subscribed to gateway events", slog.String("gateway_id", gatewayID))
	return ch, nil
}

// Unsubscribe removes a specific subscription for a gateway.
func (b *SQLBackend) Unsubscribe(gatewayID string, subscriber <-chan Event) error {
	ch, err := b.registry.removeSubscriber(gatewayID, subscriber)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from gateway %s: %w", gatewayID, err)
	}

	close(ch)

	b.logger.Info("Unsubscribed from gateway events", slog.String("gateway_id", gatewayID))
	return nil
}

// UnsubscribeAll removes all subscriptions for a gateway.
func (b *SQLBackend) UnsubscribeAll(gatewayID string) error {
	subscribers, err := b.registry.removeAllSubscribers(gatewayID)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe all from gateway %s: %w", gatewayID, err)
	}

	for _, ch := range subscribers {
		close(ch)
	}

	b.logger.Info("Unsubscribed all gateway events", slog.String("gateway_id", gatewayID))
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
			b.pollGateways()
		}
	}
}

// pollGateways checks each registered gateway for version changes.
func (b *SQLBackend) pollGateways() {
	gatewayByID := make(map[string]*gateway)
	for _, gw := range b.registry.getAll() {
		gatewayByID[gw.id] = gw
	}
	if len(gatewayByID) == 0 {
		return
	}

	pageSize := b.gatewayStatePageSize()
	cursor := ""
	for {
		states, nextCursor, err := b.getGatewayStatesPage(cursor, pageSize)
		if err != nil {
			b.logger.Warn("Failed to poll gateway states page",
				slog.String("cursor", cursor),
				slog.Any("error", err))
			return
		}
		if len(states) == 0 {
			return
		}

		for _, state := range states {
			gw, ok := gatewayByID[state.GatewayID]
			if !ok {
				continue
			}
			if err := b.pollGatewayWithState(gw, state); err != nil {
				b.logger.Warn("Failed to poll gateway",
					slog.String("gateway_id", gw.id),
					slog.Any("error", err))
			}
		}

		if len(states) < pageSize {
			return
		}
		cursor = nextCursor
	}
}

func (b *SQLBackend) gatewayStatePageSize() int {
	if b.config.GatewayStatePageSize > 0 {
		return b.config.GatewayStatePageSize
	}
	return defaultGatewayStatePageSize
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

func (b *SQLBackend) getGatewayStatesPage(cursor string, limit int) ([]GatewayState, string, error) {
	// TODO: (VirajSalaka) We can even optimize this by only selecting gateways that have had updates since the last poll time,
	// but that would require tracking last poll time per gateway which adds complexity. For now, we can rely on the fact that
	// gateways with no changes will be quickly skipped in pollGatewayWithState.
	rows, err := b.getGatewayStatesPageStmt.Query(cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query gateway states page: %w", err)
	}
	defer rows.Close()

	states := make([]GatewayState, 0, limit)
	nextCursor := ""
	for rows.Next() {
		var state GatewayState
		if err := rows.Scan(&state.GatewayID, &state.VersionID, &state.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("failed to scan gateway state row: %w", err)
		}
		states = append(states, state)
		nextCursor = state.GatewayID
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("error iterating gateway state rows: %w", err)
	}

	return states, nextCursor, nil
}

func (b *SQLBackend) pollGatewayWithState(gw *gateway, state GatewayState) error {
	// Check if version has changed
	b.registry.mu.RLock()
	knownVersion := gw.knownVersion
	lastPolled := gw.lastPolled
	b.registry.mu.RUnlock()

	if state.VersionID == knownVersion || state.VersionID == "" {
		return nil // No changes
	}

	// Fetch new events since last poll
	var lastPolledTime time.Time
	resumingFromLastPolled := lastPolled > 0
	if lastPolled > 0 {
		lastPolledTime = unixTimestampToTime(lastPolled)
	} else {
		// First poll - only replay a short recent window to avoid catching full history.
		lastPolledTime = time.Now().Add(-initialPollSkewWindow)
	}

	rows, err := b.getEventsStmt.Query(gw.id, lastPolledTime)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var evt Event
		var eventType string
		var eventID string
		if err := rows.Scan(
			&evt.GatewayID,
			&evt.ProcessedTimestamp,
			&evt.OriginatedTimestamp,
			&eventType,
			&evt.Action,
			&evt.EntityID,
			&eventID,
			&evt.EventData,
		); err != nil {
			return fmt.Errorf("failed to scan event row: %w", err)
		}
		evt.EventType = EventType(eventType)
		evt.EventID = eventID
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating event rows: %w", err)
	}

	events = trimSingleBoundaryReplay(events, lastPolledTime, resumingFromLastPolled)

	// TODO: (VirajSalaka) In the initial startup, we fetch the past events for 120 seconds.
	// But if there are lot of events during the period, we need to capture the tail events.

	// Deliver events to subscribers. If any subscriber buffer is full, stop at
	// the first blocked event so the next poll resumes from the last delivered one.
	var latestDeliveredTimestamp time.Time
	deliveredCount := 0
	deliveryBlocked := false
	deliveredSubscriberCount := 0
	b.registry.mu.RLock()
	subscribers := gw.subscribers
	for _, evt := range events {
		if !subscriberChannelsAvailable(subscribers) {
			deliveryBlocked = true
			b.logger.Warn("Subscriber channel full, deferring event delivery",
				slog.String("gateway_id", gw.id),
				slog.String("entity_id", evt.EntityID))
			break
		}
		// Delivery Starts from the point the subscription is made.
		// Hence we can keep updating the last polled timestamp regardless of number of subscribers available.
		for _, ch := range subscribers {
			ch <- evt
		}
		deliveredSubscriberCount = len(subscribers)
		latestDeliveredTimestamp = evt.ProcessedTimestamp
		deliveredCount++
	}
	b.registry.mu.RUnlock()

	// Update known version and last polled time
	b.registry.mu.Lock()
	if deliveryBlocked {
		if !latestDeliveredTimestamp.IsZero() {
			gw.lastPolled = latestDeliveredTimestamp.UnixNano()
		} else {
			gw.lastPolled = lastPolledTime.UnixNano()
		}
	} else {
		gw.knownVersion = state.VersionID
		if !latestDeliveredTimestamp.IsZero() {
			gw.lastPolled = latestDeliveredTimestamp.UnixNano()
		}
	}
	b.registry.mu.Unlock()

	if deliveredCount > 0 {
		b.logger.Debug("Delivered events to subscribers",
			slog.String("gateway_id", gw.id),
			slog.Int("event_count", deliveredCount),
			slog.Int("subscriber_count", deliveredSubscriberCount))
	}

	return nil
}

// The poll query uses `processed_timestamp >= lastPolled` so a second event that
// shares the last delivered timestamp is not missed. Results are ordered by
// `processed_timestamp ASC`, so any boundary matches appear at the front: if
// only the first row matches, it is the normal replay and we drop it; if the
// first two rows match, we keep the full slice to preserve the overlap case.
func trimSingleBoundaryReplay(events []Event, boundary time.Time, enabled bool) []Event {
	if !enabled || len(events) == 0 || !events[0].ProcessedTimestamp.Equal(boundary) {
		return events
	}

	if len(events) == 1 || !events[1].ProcessedTimestamp.Equal(boundary) {
		return events[1:]
	}

	return events
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

// CleanupRange removes events for a gateway before a given time.
func (b *SQLBackend) CleanupRange(gatewayID string, before time.Time) error {
	_, err := b.db.Exec(
		b.rebind("DELETE FROM events WHERE gateway_id = ? AND processed_timestamp < ?"),
		gatewayID, before,
	)
	if err != nil {
		return fmt.Errorf("failed to clean up events for gateway %s: %w", gatewayID, err)
	}
	return nil
}

// Close gracefully shuts down the backend
func (b *SQLBackend) Close() error {
	b.cancel()
	b.wg.Wait()

	// Close all subscriber channels
	for _, gw := range b.registry.getAll() {
		b.registry.mu.Lock()
		for _, ch := range gw.subscribers {
			close(ch)
		}
		gw.subscribers = nil
		b.registry.mu.Unlock()
	}

	// Close prepared statements
	b.stmtMu.Lock()
	defer b.stmtMu.Unlock()
	b.closeStatements()

	b.logger.Info("SQL event hub backend closed")
	return nil
}
