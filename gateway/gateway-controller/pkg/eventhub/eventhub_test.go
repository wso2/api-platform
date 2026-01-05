package eventhub

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create organization_states table
	_, err = db.Exec(`
		CREATE TABLE organization_states (
			organization TEXT PRIMARY KEY,
			version_id TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	// Create unified events table
	_, err = db.Exec(`
		CREATE TABLE events (
			organization_id TEXT NOT NULL,
			processed_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			originated_timestamp TIMESTAMP NOT NULL,
			event_type TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
			entity_id TEXT NOT NULL,
			event_data TEXT NOT NULL,
			PRIMARY KEY (organization_id, processed_timestamp)
		)
	`)
	require.NoError(t, err)

	return db
}

func TestEventHub_RegisterOrganization(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	hub := New(db, logger, DefaultConfig())

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	// Test successful registration
	err = hub.RegisterOrganization("test-org")
	assert.NoError(t, err)

	// Test duplicate registration
	err = hub.RegisterOrganization("test-org")
	assert.ErrorIs(t, err, ErrOrganizationAlreadyExists)
}

func TestEventHub_PublishAndSubscribe(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	config := DefaultConfig()
	config.PollInterval = 100 * time.Millisecond // Fast polling for test
	hub := New(db, logger, config)

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	// Register subscription
	eventChan := make(chan []Event, 10)
	err = hub.Subscribe("test-org", eventChan)
	require.NoError(t, err)

	// Publish event
	data, _ := json.Marshal(map[string]string{"key": "value"})
	err = hub.PublishEvent(context.Background(), "test-org", EventTypeAPI, "CREATE", "api-1", data)
	require.NoError(t, err)

	// Wait for event delivery via polling
	select {
	case events := <-eventChan:
		assert.GreaterOrEqual(t, len(events), 1)
		assert.Equal(t, "test-org", events[0].OrganizationID)
		assert.Equal(t, EventTypeAPI, events[0].EventType)
		assert.Equal(t, "CREATE", events[0].Action)
		assert.Equal(t, "api-1", events[0].EntityID)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestEventHub_CleanUpEvents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	hub := New(db, logger, DefaultConfig())

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	// Publish events
	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(map[string]int{"index": i})
		err = hub.PublishEvent(context.Background(), "test-org", EventTypeAPI, "CREATE", "api-1", data)
		require.NoError(t, err)
	}

	// Cleanup all events
	err = hub.CleanUpEvents(context.Background(), time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)

	// Verify events are deleted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestEventHub_PollerDetectsChanges(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	config := DefaultConfig()
	config.PollInterval = 50 * time.Millisecond
	hub := New(db, logger, config)

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	eventChan := make(chan []Event, 10)
	err = hub.Subscribe("test-org", eventChan)
	require.NoError(t, err)

	// Publish multiple events
	for i := 0; i < 3; i++ {
		data, _ := json.Marshal(map[string]int{"index": i})
		err = hub.PublishEvent(context.Background(), "test-org", EventTypeAPI, "CREATE", "api-1", data)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for events to be delivered
	var receivedEvents []Event
	timeout := time.After(500 * time.Millisecond)

	for {
		select {
		case events := <-eventChan:
			receivedEvents = append(receivedEvents, events...)
			if len(receivedEvents) >= 3 {
				assert.Len(t, receivedEvents, 3)
				return
			}
		case <-timeout:
			t.Fatalf("Timeout: received only %d events", len(receivedEvents))
		}
	}
}

func TestEventHub_AtomicPublish(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	hub := New(db, logger, DefaultConfig())

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	// Publish event
	data, _ := json.Marshal(map[string]string{"test": "data"})
	err = hub.PublishEvent(context.Background(), "test-org", EventTypeAPI, "CREATE", "api-1", data)
	require.NoError(t, err)

	// Verify event was recorded in unified table
	var eventCount int
	err = db.QueryRow("SELECT COUNT(*) FROM events WHERE organization_id = ?", "test-org").Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)

	// Verify state was updated
	var versionID string
	err = db.QueryRow("SELECT version_id FROM organization_states WHERE organization = ?", "test-org").Scan(&versionID)
	require.NoError(t, err)
	assert.NotEmpty(t, versionID)
}

func TestEventHub_MultipleSubscribers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	config := DefaultConfig()
	config.PollInterval = 50 * time.Millisecond
	hub := New(db, logger, config)

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	// Register multiple subscribers
	eventChan1 := make(chan []Event, 10)
	eventChan2 := make(chan []Event, 10)
	err = hub.Subscribe("test-org", eventChan1)
	require.NoError(t, err)
	err = hub.Subscribe("test-org", eventChan2)
	require.NoError(t, err)

	// Publish event
	data, _ := json.Marshal(map[string]string{"test": "multi"})
	err = hub.PublishEvent(context.Background(), "test-org", EventTypeAPI, "CREATE", "api-1", data)
	require.NoError(t, err)

	// Both subscribers should receive the event
	timeout := time.After(time.Second)

	select {
	case events := <-eventChan1:
		assert.Len(t, events, 1)
	case <-timeout:
		t.Fatal("Timeout waiting for event on subscriber 1")
	}

	select {
	case events := <-eventChan2:
		assert.Len(t, events, 1)
	case <-timeout:
		t.Fatal("Timeout waiting for event on subscriber 2")
	}
}

func TestEventHub_GracefulShutdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	hub := New(db, logger, DefaultConfig())

	err := hub.Initialize(context.Background())
	require.NoError(t, err)

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	// Close should complete without hanging
	err = hub.Close()
	assert.NoError(t, err)

	// Calling Close again should be safe
	err = hub.Close()
	assert.NoError(t, err)
}

func TestEventHub_MultipleEventTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	config := DefaultConfig()
	config.PollInterval = 50 * time.Millisecond
	hub := New(db, logger, config)

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	err = hub.RegisterOrganization("test-org")
	require.NoError(t, err)

	eventChan := make(chan []Event, 10)
	err = hub.Subscribe("test-org", eventChan)
	require.NoError(t, err)

	// Publish different event types
	data1, _ := json.Marshal(map[string]string{"type": "api"})
	err = hub.PublishEvent(context.Background(), "test-org", EventTypeAPI, "CREATE", "api-1", data1)
	require.NoError(t, err)

	data2, _ := json.Marshal(map[string]string{"type": "cert"})
	err = hub.PublishEvent(context.Background(), "test-org", EventTypeCertificate, "UPDATE", "cert-1", data2)
	require.NoError(t, err)

	data3, _ := json.Marshal(map[string]string{"type": "llm"})
	err = hub.PublishEvent(context.Background(), "test-org", EventTypeLLMTemplate, "DELETE", "template-1", data3)
	require.NoError(t, err)

	// Wait for events to be delivered (all types should come through)
	var receivedEvents []Event
	timeout := time.After(time.Second)

	for {
		select {
		case events := <-eventChan:
			receivedEvents = append(receivedEvents, events...)
			if len(receivedEvents) >= 3 {
				// Verify all event types were received
				assert.Len(t, receivedEvents, 3)

				eventTypeMap := make(map[EventType]bool)
				for _, e := range receivedEvents {
					eventTypeMap[e.EventType] = true
				}

				assert.True(t, eventTypeMap[EventTypeAPI])
				assert.True(t, eventTypeMap[EventTypeCertificate])
				assert.True(t, eventTypeMap[EventTypeLLMTemplate])
				return
			}
		case <-timeout:
			t.Fatalf("Timeout: received only %d events", len(receivedEvents))
		}
	}
}
