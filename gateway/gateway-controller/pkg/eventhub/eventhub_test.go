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

	// Create topic_states table
	_, err = db.Exec(`
		CREATE TABLE topic_states (
			organization TEXT NOT NULL,
			topic_name TEXT NOT NULL,
			version_id TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (organization, topic_name)
		)
	`)
	require.NoError(t, err)

	// Create test events table
	_, err = db.Exec(`
		CREATE TABLE test_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			processed_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			originated_timestamp TIMESTAMP NOT NULL,
			event_data TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	return db
}

func TestEventHub_RegisterTopic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	hub := New(db, logger, DefaultConfig())

	err := hub.Initialize(context.Background())
	require.NoError(t, err)
	defer hub.Close()

	// Test successful registration
	err = hub.RegisterTopic("test-org", "test")
	assert.NoError(t, err)

	// Test duplicate registration
	err = hub.RegisterTopic("test-org", "test")
	assert.ErrorIs(t, err, ErrTopicAlreadyExists)

	// Test missing table
	err = hub.RegisterTopic("test-org", "nonexistent")
	assert.ErrorIs(t, err, ErrTopicTableMissing)
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

	err = hub.RegisterTopic("test-org", "test")
	require.NoError(t, err)

	// Register subscription
	eventChan := make(chan []Event, 10)
	err = hub.RegisterSubscription("test", eventChan)
	require.NoError(t, err)

	// Publish event
	data, _ := json.Marshal(map[string]string{"key": "value"})
	err = hub.PublishEvent(context.Background(), "test-org", "test", data)
	require.NoError(t, err)

	// Wait for event delivery via polling
	select {
	case events := <-eventChan:
		assert.GreaterOrEqual(t, len(events), 1)
		assert.Equal(t, TopicName("test"), events[0].TopicName)
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

	err = hub.RegisterTopic("test-org", "test")
	require.NoError(t, err)

	// Publish events
	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(map[string]int{"index": i})
		err = hub.PublishEvent(context.Background(), "test-org", "test", data)
		require.NoError(t, err)
	}

	// Cleanup all events
	err = hub.CleanUpEvents(context.Background(), time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)

	// Verify events are deleted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_events").Scan(&count)
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

	err = hub.RegisterTopic("test-org", "test")
	require.NoError(t, err)

	eventChan := make(chan []Event, 10)
	err = hub.RegisterSubscription("test", eventChan)
	require.NoError(t, err)

	// Publish multiple events
	for i := 0; i < 3; i++ {
		data, _ := json.Marshal(map[string]int{"index": i})
		err = hub.PublishEvent(context.Background(), "test-org", "test", data)
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

	err = hub.RegisterTopic("test-org", "test")
	require.NoError(t, err)

	// Publish event
	data, _ := json.Marshal(map[string]string{"test": "data"})
	err = hub.PublishEvent(context.Background(), "test-org", "test", data)
	require.NoError(t, err)

	// Verify event was recorded
	var eventCount int
	err = db.QueryRow("SELECT COUNT(*) FROM test_events").Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)

	// Verify state was updated
	var versionID string
	err = db.QueryRow("SELECT version_id FROM topic_states WHERE organization = ? AND topic_name = ?", "test-org", "test").Scan(&versionID)
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

	err = hub.RegisterTopic("test-org", "test")
	require.NoError(t, err)

	// Register multiple subscribers
	eventChan1 := make(chan []Event, 10)
	eventChan2 := make(chan []Event, 10)
	err = hub.RegisterSubscription("test", eventChan1)
	require.NoError(t, err)
	err = hub.RegisterSubscription("test", eventChan2)
	require.NoError(t, err)

	// Publish event
	data, _ := json.Marshal(map[string]string{"test": "multi"})
	err = hub.PublishEvent(context.Background(), "test-org", "test", data)
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

	err = hub.RegisterTopic("test-org", "test")
	require.NoError(t, err)

	// Close should complete without hanging
	err = hub.Close()
	assert.NoError(t, err)

	// Calling Close again should be safe
	err = hub.Close()
	assert.NoError(t, err)
}
