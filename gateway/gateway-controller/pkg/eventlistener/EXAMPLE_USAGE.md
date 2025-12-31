# EventListener Usage Examples

This document demonstrates how to use the EventListener with different EventSource implementations.

## Architecture Overview

The EventListener is designed with a generic event source abstraction:

```
┌─────────────────┐
│  EventListener  │
└────────┬────────┘
         │ depends on
         ▼
┌─────────────────┐
│  EventSource    │  (interface)
└────────┬────────┘
         │ implemented by
         ├─────────────────────┐
         ▼                     ▼
┌──────────────────┐   ┌────────────────┐
│ EventHubAdapter  │   │ MockEventSource│
└──────────────────┘   └────────────────┘
         │                     │
         ▼                     ▼
┌──────────────────┐   ┌────────────────┐
│   EventHub       │   │  Test Suite    │
│ (Database-based) │   │  (In-memory)   │
└──────────────────┘   └────────────────┘
```

## Example 1: Using with EventHub (Production)

```go
package main

import (
    "context"
    "database/sql"
    "time"

    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Initialize dependencies
    db := initializeDatabase() // *sql.DB
    store := storage.NewConfigStore()
    sqliteStorage := storage.NewSQLiteStorage(db, logger)
    snapshotManager := xds.NewSnapshotManager(store, logger)
    policyManager := policyxds.NewPolicyManager(logger)
    routerConfig := loadRouterConfig()

    // Create EventHub
    hubConfig := eventhub.DefaultConfig()
    eventHub := eventhub.New(db, logger, hubConfig)

    // Initialize EventHub
    ctx := context.Background()
    if err := eventHub.Initialize(ctx); err != nil {
        logger.Fatal("Failed to initialize EventHub", zap.Error(err))
    }

    // Wrap EventHub with adapter
    eventSource := eventlistener.NewEventHubAdapter(eventHub, logger)

    // Create EventListener with the adapter
    listener := eventlistener.NewEventListener(
        eventSource,
        store,
        sqliteStorage,
        snapshotManager,
        policyManager,
        routerConfig,
        logger,
    )

    // Start listening for events
    if err := listener.Start(ctx); err != nil {
        logger.Fatal("Failed to start EventListener", zap.Error(err))
    }

    logger.Info("EventListener is now running")

    // ... application runs ...

    // Graceful shutdown
    listener.Stop()
    eventSource.Close()
}

func initializeDatabase() *sql.DB {
    // Implementation details...
    return nil
}

func loadRouterConfig() *config.RouterConfig {
    // Implementation details...
    return nil
}
```

## Example 2: Using MockEventSource for Testing

```go
package mypackage

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
    "go.uber.org/zap"
)

func TestEventListenerProcessesAPIEvents(t *testing.T) {
    logger := zap.NewNop()

    // Create mock event source
    mockSource := eventlistener.NewMockEventSource()

    // Setup test dependencies
    store := setupTestStore()
    db := setupTestDatabase()
    snapshotManager := setupTestSnapshotManager()
    routerConfig := setupTestRouterConfig()

    // Create EventListener with mock
    listener := eventlistener.NewEventListener(
        mockSource,
        store,
        db,
        snapshotManager,
        nil, // no policy manager for this test
        routerConfig,
        logger,
    )

    // Start listener
    ctx := context.Background()
    err := listener.Start(ctx)
    assert.NoError(t, err)

    // Verify subscription was created
    assert.True(t, mockSource.IsSubscribed("default"))
    assert.Equal(t, 1, len(mockSource.SubscribeCalls))

    // Publish test event
    testEvent := eventlistener.Event{
        OrganizationID: "default",
        EventType:      "API",
        Action:         "CREATE",
        EntityID:       "test-api-123",
        EventData:      []byte(`{"name": "test"}`),
        Timestamp:      time.Now(),
    }

    err = mockSource.PublishEvent("default", testEvent)
    assert.NoError(t, err)

    // Allow time for processing
    time.Sleep(100 * time.Millisecond)

    // Verify the event was processed
    // (Check side effects in store, snapshotManager, etc.)

    // Cleanup
    listener.Stop()
    assert.True(t, mockSource.IsClosed())
}

func TestEventListenerHandlesSubscriptionError(t *testing.T) {
    logger := zap.NewNop()
    mockSource := eventlistener.NewMockEventSource()

    // Configure mock to return error
    mockSource.SubscribeError = assert.AnError

    listener := eventlistener.NewEventListener(
        mockSource,
        setupTestStore(),
        setupTestDatabase(),
        setupTestSnapshotManager(),
        nil,
        setupTestRouterConfig(),
        logger,
    )

    // Start should fail
    ctx := context.Background()
    err := listener.Start(ctx)
    assert.Error(t, err)
}

// Test helper functions
func setupTestStore() *storage.ConfigStore { /* ... */ return nil }
func setupTestDatabase() storage.Storage { /* ... */ return nil }
func setupTestSnapshotManager() *xds.SnapshotManager { /* ... */ return nil }
func setupTestRouterConfig() *config.RouterConfig { /* ... */ return nil }
```

## Example 3: Implementing a Custom EventSource (Kafka)

You can implement a Kafka-based event source:

```go
package kafka

import (
    "context"
    "encoding/json"
    "sync"

    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
    "github.com/segmentio/kafka-go"
    "go.uber.org/zap"
)

// KafkaEventSource implements EventSource using Apache Kafka
type KafkaEventSource struct {
    brokers []string
    logger  *zap.Logger

    mu            sync.RWMutex
    readers       map[string]*kafka.Reader
    subscriptions map[string]chan<- []eventlistener.Event
}

func NewKafkaEventSource(brokers []string, logger *zap.Logger) eventlistener.EventSource {
    return &KafkaEventSource{
        brokers:       brokers,
        logger:        logger,
        readers:       make(map[string]*kafka.Reader),
        subscriptions: make(map[string]chan<- []eventlistener.Event),
    }
}

func (k *KafkaEventSource) Subscribe(ctx context.Context, organizationID string, eventChan chan<- []eventlistener.Event) error {
    k.mu.Lock()
    defer k.mu.Unlock()

    // Create Kafka reader for the organization's topic
    topic := "gateway-events-" + organizationID
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers: k.brokers,
        Topic:   topic,
        GroupID: "gateway-controller",
    })

    k.readers[organizationID] = reader
    k.subscriptions[organizationID] = eventChan

    // Start consuming goroutine
    go k.consumeEvents(ctx, organizationID, reader, eventChan)

    k.logger.Info("Subscribed to Kafka topic",
        zap.String("organization", organizationID),
        zap.String("topic", topic),
    )

    return nil
}

func (k *KafkaEventSource) consumeEvents(
    ctx context.Context,
    organizationID string,
    reader *kafka.Reader,
    eventChan chan<- []eventlistener.Event,
) {
    for {
        msg, err := reader.FetchMessage(ctx)
        if err != nil {
            if ctx.Err() != nil {
                return // Context cancelled
            }
            k.logger.Error("Failed to fetch Kafka message", zap.Error(err))
            continue
        }

        // Parse event from Kafka message
        var event eventlistener.Event
        if err := json.Unmarshal(msg.Value, &event); err != nil {
            k.logger.Error("Failed to unmarshal event", zap.Error(err))
            continue
        }

        // Send as batch (single event)
        select {
        case eventChan <- []eventlistener.Event{event}:
            // Successfully sent
            if err := reader.CommitMessages(ctx, msg); err != nil {
                k.logger.Error("Failed to commit Kafka message", zap.Error(err))
            }
        case <-ctx.Done():
            return
        }
    }
}

func (k *KafkaEventSource) Unsubscribe(organizationID string) error {
    k.mu.Lock()
    defer k.mu.Unlock()

    if reader, exists := k.readers[organizationID]; exists {
        reader.Close()
        delete(k.readers, organizationID)
        delete(k.subscriptions, organizationID)
    }

    return nil
}

func (k *KafkaEventSource) Close() error {
    k.mu.Lock()
    defer k.mu.Unlock()

    for orgID, reader := range k.readers {
        reader.Close()
        delete(k.readers, orgID)
    }

    k.subscriptions = make(map[string]chan<- []eventlistener.Event)
    return nil
}
```

Then use it like this:

```go
// Create Kafka event source
kafkaSource := kafka.NewKafkaEventSource(
    []string{"localhost:9092"},
    logger,
)

// Use with EventListener
listener := eventlistener.NewEventListener(
    kafkaSource,
    store,
    db,
    snapshotManager,
    policyManager,
    routerConfig,
    logger,
)
```

## Benefits of this Architecture

1. **Testability**: Easy to test EventListener with MockEventSource
2. **Flexibility**: Switch between EventHub, Kafka, RabbitMQ, etc.
3. **Decoupling**: EventListener doesn't depend on specific implementation details
4. **Maintainability**: Clear interfaces and responsibilities
5. **Extensibility**: Easy to add new event source implementations

## Event Flow

```
EventSource → Subscribe(orgID, chan) → EventListener
    ↓                                       ↓
Publishes events                    Receives []Event
    ↓                                       ↓
Forward to channel                  processEvents()
    ↓                                       ↓
Event batches                       handleEvent()
                                            ↓
                                    processAPIEvents()
                                            ↓
                                    Update XDS & Policies
```
