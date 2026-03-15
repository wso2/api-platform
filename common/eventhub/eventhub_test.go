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
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	require.NoError(t, err)

	// Create required tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gateway_states (
			gateway_id TEXT PRIMARY KEY,
			version_id TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS events (
			gateway_id TEXT NOT NULL,
			processed_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			originated_timestamp TIMESTAMP NOT NULL,
			entity_type TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
			entity_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			event_data TEXT NOT NULL,
			PRIMARY KEY (event_id)
		);
	`)
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })
	return db
}

func setupTestDBAllowTimestampOverlap(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gateway_states (
			gateway_id TEXT PRIMARY KEY,
			version_id TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS events (
			gateway_id TEXT NOT NULL,
			processed_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			originated_timestamp TIMESTAMP NOT NULL,
			entity_type TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
			entity_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			event_data TEXT NOT NULL,
			PRIMARY KEY (event_id)
		);
	`)
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })
	return db
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRegisterGateway(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	hub := New(db, logger, DefaultConfig())
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	err := hub.RegisterGateway("test-org")
	assert.NoError(t, err)

	// Verify in database
	var gatewayID string
	err = db.QueryRow("SELECT gateway_id FROM gateway_states WHERE gateway_id = ?", "test-org").Scan(&gatewayID)
	assert.NoError(t, err)
	assert.Equal(t, "test-org", gatewayID)
}

func TestPublishAndSubscribe(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	config := Config{
		PollInterval:    100 * time.Millisecond,
		CleanupInterval: 5 * time.Minute,
		RetentionPeriod: 1 * time.Hour,
	}
	hub := New(db, logger, config)
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	require.NoError(t, hub.RegisterGateway("test-org"))

	// Subscribe
	ch, err := hub.Subscribe("test-org")
	require.NoError(t, err)

	// Publish event
	event := Event{
		GatewayID:           "test-org",
		OriginatedTimestamp: time.Now(),
		EventType:           EventTypeAPI,
		Action:              "CREATE",
		EntityID:            "api-123",
		EventID:             "corr-456",
		EventData:           `{"name":"test-api"}`,
	}
	require.NoError(t, hub.PublishEvent("test-org", event))

	// Wait for event to be delivered
	select {
	case received := <-ch:
		assert.Equal(t, EventTypeAPI, received.EventType)
		assert.Equal(t, "CREATE", received.Action)
		assert.Equal(t, "api-123", received.EntityID)
		assert.Equal(t, `{"name":"test-api"}`, received.EventData)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for event")
	}
}

func TestPublishDuplicateEventIsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))

	event := Event{
		GatewayID:           "test-org",
		OriginatedTimestamp: time.Now(),
		EventType:           EventTypeAPI,
		Action:              "CREATE",
		EntityID:            "api-123",
		EventID:             "corr-duplicate-001",
		EventData:           "{}",
	}

	require.NoError(t, backend.Publish("test-org", event))

	var versionBeforeDuplicate string
	require.NoError(t, db.QueryRow(
		"SELECT version_id FROM gateway_states WHERE gateway_id = ?",
		"test-org",
	).Scan(&versionBeforeDuplicate))
	require.NotEmpty(t, versionBeforeDuplicate)

	require.NoError(t, backend.Publish("test-org", event))

	var eventCount int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM events WHERE event_id = ?",
		event.EventID,
	).Scan(&eventCount))
	assert.Equal(t, 1, eventCount)

	var versionAfterDuplicate string
	require.NoError(t, db.QueryRow(
		"SELECT version_id FROM gateway_states WHERE gateway_id = ?",
		"test-org",
	).Scan(&versionAfterDuplicate))
	assert.Equal(t, versionBeforeDuplicate, versionAfterDuplicate)
}

func TestCleanUpEvents(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	hub := New(db, logger, DefaultConfig())
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	require.NoError(t, hub.RegisterGateway("test-org"))

	// Insert old event directly
	oldTime := time.Now().Add(-2 * time.Hour)
	_, err := db.Exec(`
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "test-org", oldTime, oldTime, "API", "CREATE", "old-api", "cleanup-old-api", "{}")
	require.NoError(t, err)

	// Cleanup
	require.NoError(t, hub.CleanUpEvents())

	// Verify old event was deleted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM events WHERE entity_id = 'old-api'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAtomicPublish(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	hub := New(db, logger, DefaultConfig())
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	require.NoError(t, hub.RegisterGateway("test-org"))

	// Publish event
	event := Event{
		OriginatedTimestamp: time.Now(),
		EventType:           EventTypeAPI,
		Action:              "CREATE",
		EntityID:            "api-1",
		EventData:           `{"test":"data"}`,
	}
	require.NoError(t, hub.PublishEvent("test-org", event))

	// Verify both event and version were updated atomically
	var eventCount int
	err := db.QueryRow("SELECT COUNT(*) FROM events WHERE gateway_id = 'test-org'").Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)

	var versionID string
	err = db.QueryRow("SELECT version_id FROM gateway_states WHERE gateway_id = 'test-org'").Scan(&versionID)
	require.NoError(t, err)
	assert.NotEmpty(t, versionID)
}

func TestPublishDefaultsEmptyEventData(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	hub := New(db, logger, DefaultConfig())
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	require.NoError(t, hub.RegisterGateway("test-org"))

	event := Event{
		OriginatedTimestamp: time.Now(),
		EventType:           EventTypeAPI,
		Action:              "UPDATE",
		EntityID:            "api-default-eventdata",
		EventData:           "   ",
	}
	require.NoError(t, hub.PublishEvent("test-org", event))

	var storedEventData string
	err := db.QueryRow("SELECT event_data FROM events WHERE gateway_id = ? AND entity_id = ?", "test-org", "api-default-eventdata").Scan(&storedEventData)
	require.NoError(t, err)
	assert.Equal(t, EmptyEventData, storedEventData)
}

func TestMultipleSubscribers(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	config := Config{
		PollInterval:    100 * time.Millisecond,
		CleanupInterval: 5 * time.Minute,
		RetentionPeriod: 1 * time.Hour,
	}
	hub := New(db, logger, config)
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	require.NoError(t, hub.RegisterGateway("test-org"))

	// Subscribe twice
	ch1, err := hub.Subscribe("test-org")
	require.NoError(t, err)
	ch2, err := hub.Subscribe("test-org")
	require.NoError(t, err)

	// Publish event
	event := Event{
		OriginatedTimestamp: time.Now(),
		EventType:           EventTypeAPI,
		Action:              "UPDATE",
		EntityID:            "api-multi",
		EventData:           `{}`,
	}
	require.NoError(t, hub.PublishEvent("test-org", event))

	// Both subscribers should receive the event
	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			assert.Equal(t, "api-multi", received.EntityID)
		case <-time.After(5 * time.Second):
			t.Fatal("Timed out waiting for event on subscriber")
		}
	}
}

func TestUnsubscribeRemovesOnlySpecificSubscriber(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))

	ch1, err := backend.Subscribe("test-org")
	require.NoError(t, err)
	ch2, err := backend.Subscribe("test-org")
	require.NoError(t, err)

	require.NoError(t, backend.Unsubscribe("test-org", ch1))

	select {
	case _, ok := <-ch1:
		assert.False(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unsubscribed channel to close")
	}

	event := Event{
		OriginatedTimestamp: time.Now(),
		EventType:           EventTypeAPI,
		Action:              "UPDATE",
		EntityID:            "api-specific-unsub",
		EventData:           "{}",
	}
	require.NoError(t, backend.Publish("test-org", event))
	backend.pollGateways()

	select {
	case received := <-ch2:
		assert.Equal(t, "api-specific-unsub", received.EntityID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event on remaining subscriber")
	}
}

func TestGracefulShutdown(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	hub := New(db, logger, DefaultConfig())
	require.NoError(t, hub.Initialize())

	require.NoError(t, hub.RegisterGateway("test-org"))
	_, err := hub.Subscribe("test-org")
	require.NoError(t, err)

	// Close should not panic or hang
	err = hub.Close()
	assert.NoError(t, err)
}

func TestMultipleEventTypes(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	config := Config{
		PollInterval:    100 * time.Millisecond,
		CleanupInterval: 5 * time.Minute,
		RetentionPeriod: 1 * time.Hour,
	}
	hub := New(db, logger, config)
	require.NoError(t, hub.Initialize())
	defer hub.Close()

	require.NoError(t, hub.RegisterGateway("test-org"))

	ch, err := hub.Subscribe("test-org")
	require.NoError(t, err)

	// Publish different event types
	events := []Event{
		{OriginatedTimestamp: time.Now(), EventType: EventTypeAPI, Action: "CREATE", EntityID: "api-1", EventData: "{}"},
		{OriginatedTimestamp: time.Now(), EventType: EventTypeCertificate, Action: "CREATE", EntityID: "cert-1", EventData: "{}"},
		{OriginatedTimestamp: time.Now(), EventType: EventTypeLLMTemplate, Action: "UPDATE", EntityID: "tmpl-1", EventData: "{}"},
	}

	for _, evt := range events {
		// Small delay to avoid primary key conflict (processed_timestamp)
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, hub.PublishEvent("test-org", evt))
	}

	// Receive all events
	received := make([]Event, 0, len(events))
	timeout := time.After(5 * time.Second)
	for len(received) < len(events) {
		select {
		case evt := <-ch:
			received = append(received, evt)
		case <-timeout:
			t.Fatalf("Timed out waiting for events, received %d/%d", len(received), len(events))
		}
	}

	assert.Len(t, received, 3)
}

func TestPollGatewaysKeysetPagination(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backendConfig := DefaultSQLBackendConfig()
	backendConfig.GatewayStatePageSize = 50
	backend := NewSQLBackend(db, logger, backendConfig)
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	const gatewayCount = 210
	subscribers := make(map[string]<-chan Event, gatewayCount)

	for i := 0; i < gatewayCount; i++ {
		gatewayID := fmt.Sprintf("org-%03d", i)
		require.NoError(t, backend.RegisterGateway(gatewayID))

		ch, err := backend.Subscribe(gatewayID)
		require.NoError(t, err)
		subscribers[gatewayID] = ch

		event := Event{
			OriginatedTimestamp: time.Now(),
			EventType:           EventTypeAPI,
			Action:              "CREATE",
			EntityID:            fmt.Sprintf("entity-%03d", i),
			EventData:           "{}",
		}
		require.NoError(t, backend.Publish(gatewayID, event))
	}

	backend.pollGateways()

	for gatewayID, ch := range subscribers {
		select {
		case evt := <-ch:
			assert.Equal(t, gatewayID, evt.GatewayID)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event for %s", gatewayID)
		}
	}
}

func TestPollGatewayWithStateFirstPollUsesSkewWindow(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))
	ch, err := backend.Subscribe("test-org")
	require.NoError(t, err)

	now := time.Now()
	oldTs := now.Add(-2 * time.Minute)
	recentTs := now.Add(-15 * time.Second)
	_, err = db.Exec(`
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"test-org", oldTs, oldTs, "API", "CREATE", "old-entity", "state-skew-old", "{}",
		"test-org", recentTs, recentTs, "API", "CREATE", "recent-entity", "state-skew-recent", "{}",
	)
	require.NoError(t, err)

	gateway, err := backend.registry.get("test-org")
	require.NoError(t, err)

	state := GatewayState{
		GatewayID: "test-org",
		VersionID: "v1",
	}
	require.NoError(t, backend.pollGatewayWithState(gateway, state))

	select {
	case evt := <-ch:
		assert.Equal(t, "recent-entity", evt.EntityID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for recent event")
	}

	select {
	case evt := <-ch:
		t.Fatalf("unexpected additional event delivered: %s", evt.EntityID)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestPollGatewayWithStateSupportsUnixSecondsLastPolled(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))
	ch, err := backend.Subscribe("test-org")
	require.NoError(t, err)

	now := time.Now()
	oldTs := now.Add(-2 * time.Minute)
	recentTs := now.Add(-10 * time.Second)
	_, err = db.Exec(`
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"test-org", oldTs, oldTs, "API", "CREATE", "old-entity", "unix-seconds-old", "{}",
		"test-org", recentTs, recentTs, "API", "CREATE", "recent-entity", "unix-seconds-recent", "{}",
	)
	require.NoError(t, err)

	gateway, err := backend.registry.get("test-org")
	require.NoError(t, err)
	gateway.lastPolled = now.Add(-30 * time.Second).Unix()

	state := GatewayState{
		GatewayID: "test-org",
		VersionID: "v2",
	}
	require.NoError(t, backend.pollGatewayWithState(gateway, state))

	select {
	case evt := <-ch:
		assert.Equal(t, "recent-entity", evt.EntityID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for recent event")
	}

	select {
	case evt := <-ch:
		t.Fatalf("unexpected additional event delivered: %s", evt.EntityID)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestPollGatewayWithStateDropsSingleBoundaryEvent(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))
	ch, err := backend.Subscribe("test-org")
	require.NoError(t, err)

	boundaryTs := time.Now().Add(-2 * time.Second)
	laterTs := boundaryTs.Add(10 * time.Millisecond)
	_, err = db.Exec(`
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"test-org", boundaryTs, boundaryTs, "API", "CREATE", "boundary-entity", "boundary-single", "{}",
		"test-org", laterTs, laterTs, "API", "CREATE", "later-entity", "boundary-later", "{}",
	)
	require.NoError(t, err)

	gateway, err := backend.registry.get("test-org")
	require.NoError(t, err)
	gateway.lastPolled = boundaryTs.UnixNano()

	state := GatewayState{
		GatewayID: "test-org",
		VersionID: "v3",
	}
	require.NoError(t, backend.pollGatewayWithState(gateway, state))

	select {
	case evt := <-ch:
		assert.Equal(t, "later-entity", evt.EntityID)
		assert.Equal(t, laterTs.UnixNano(), gateway.lastPolled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for later event")
	}
	assert.Equal(t, "v3", gateway.knownVersion)

	select {
	case evt := <-ch:
		t.Fatalf("unexpected additional event delivered: %s", evt.EntityID)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestPollGatewayWithStateKeepsBoundaryOverlapEvents(t *testing.T) {
	db := setupTestDBAllowTimestampOverlap(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))
	ch, err := backend.Subscribe("test-org")
	require.NoError(t, err)

	boundaryTs := time.Now().Add(-2 * time.Second)
	_, err = db.Exec(`
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"test-org", boundaryTs, boundaryTs, "API", "CREATE", "overlap-first", "boundary-overlap-first", "{}",
		"test-org", boundaryTs, boundaryTs, "API", "CREATE", "overlap-second", "boundary-overlap-second", "{}",
	)
	require.NoError(t, err)

	gateway, err := backend.registry.get("test-org")
	require.NoError(t, err)
	gateway.lastPolled = boundaryTs.UnixNano()

	state := GatewayState{
		GatewayID: "test-org",
		VersionID: "v4",
	}
	require.NoError(t, backend.pollGatewayWithState(gateway, state))

	var received []string
	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch:
			received = append(received, evt.EntityID)
			assert.Equal(t, boundaryTs.UnixNano(), evt.ProcessedTimestamp.UnixNano())
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for overlap event")
		}
	}
	assert.ElementsMatch(t, []string{"overlap-first", "overlap-second"}, received)
	assert.Equal(t, boundaryTs.UnixNano(), gateway.lastPolled)
	assert.Equal(t, "v4", gateway.knownVersion)

	select {
	case evt := <-ch:
		t.Fatalf("unexpected additional event delivered: %s", evt.EntityID)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestPollGatewayWithStateRetriesDeferredEventsFromLastDeliveredTimestamp(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	backend := NewSQLBackend(db, logger, DefaultSQLBackendConfig())
	require.NoError(t, backend.prepareStatements())
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.RegisterGateway("test-org"))

	ch := make(chan Event, 1)
	require.NoError(t, backend.registry.addSubscriber("test-org", ch))

	firstTs := time.Now().Add(-2 * time.Second)
	secondTs := firstTs.Add(10 * time.Millisecond)
	_, err := db.Exec(`
		INSERT INTO events (gateway_id, processed_timestamp, originated_timestamp, entity_type, action, entity_id, event_id, event_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"test-org", firstTs, firstTs, "API", "CREATE", "first-entity", "deferred-first", "{}",
		"test-org", secondTs, secondTs, "API", "CREATE", "second-entity", "deferred-second", "{}",
	)
	require.NoError(t, err)

	gateway, err := backend.registry.get("test-org")
	require.NoError(t, err)

	state := GatewayState{
		GatewayID: "test-org",
		VersionID: "v1",
	}

	require.NoError(t, backend.pollGatewayWithState(gateway, state))

	select {
	case evt := <-ch:
		assert.Equal(t, "first-entity", evt.EntityID)
		assert.Equal(t, evt.ProcessedTimestamp.UnixNano(), gateway.lastPolled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first event")
	}
	assert.Empty(t, gateway.knownVersion)

	require.NoError(t, backend.pollGatewayWithState(gateway, state))

	select {
	case evt := <-ch:
		assert.Equal(t, "second-entity", evt.EntityID)
		assert.Equal(t, evt.ProcessedTimestamp.UnixNano(), gateway.lastPolled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second event")
	}
	assert.Equal(t, "v1", gateway.knownVersion)

	select {
	case evt := <-ch:
		t.Fatalf("unexpected additional event delivered: %s", evt.EntityID)
	case <-time.After(150 * time.Millisecond):
	}
}
