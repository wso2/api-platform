package eventhub

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// eventHub is the main implementation of EventHub interface
type eventHub struct {
	db       *sql.DB
	store    *store
	registry *topicRegistry
	poller   *poller
	config   Config
	logger   *zap.Logger

	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
	wg            sync.WaitGroup
	initialized   bool
	mu            sync.RWMutex
}

// New creates a new EventHub instance
func New(db *sql.DB, logger *zap.Logger, config Config) EventHub {
	registry := newTopicRegistry()
	store := newStore(db, logger)

	return &eventHub{
		db:       db,
		store:    store,
		registry: registry,
		config:   config,
		logger:   logger,
	}
}

// Initialize sets up the EventHub and starts background workers
func (eh *eventHub) Initialize(ctx context.Context) error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if eh.initialized {
		return nil
	}

	eh.logger.Info("Initializing EventHub")

	// Create and start poller
	eh.poller = newPoller(eh.store, eh.registry, eh.config, eh.logger)
	eh.poller.start(ctx)

	// Start cleanup goroutine
	eh.cleanupCtx, eh.cleanupCancel = context.WithCancel(ctx)
	eh.wg.Add(1)
	go eh.cleanupLoop()

	eh.initialized = true
	eh.logger.Info("EventHub initialized successfully",
		zap.Duration("pollInterval", eh.config.PollInterval),
	)
	return nil
}

// RegisterTopic registers a new topic with the EventHub
func (eh *eventHub) RegisterTopic(organization string, topicName TopicName) error {
	ctx := context.Background()

	// Check if events table exists
	exists, err := eh.store.tableExists(ctx, topicName)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: table %s does not exist",
			ErrTopicTableMissing, eh.store.getEventsTableName(topicName))
	}

	// Register topic in registry
	if err := eh.registry.register(organization, topicName); err != nil {
		return err
	}

	// Initialize empty state in database
	if err := eh.store.initializeTopicState(ctx, organization, topicName); err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	eh.logger.Info("Topic registered",
		zap.String("organization", organization),
		zap.String("topic", string(topicName)),
	)

	return nil
}

// PublishEvent publishes an event to a topic
// Note: States and Events are updated ATOMICALLY in a transaction
func (eh *eventHub) PublishEvent(ctx context.Context, organization string, topicName TopicName, eventData []byte) error {
	// Verify topic is registered
	_, err := eh.registry.get(topicName)
	if err != nil {
		return err
	}

	now := time.Now()
	event := &Event{
		TopicName:           topicName,
		ProcessedTimestamp:  now,
		OriginatedTimestamp: now,
		EventData:           eventData,
	}

	// Publish atomically (event + state update in transaction)
	id, version, err := eh.store.publishEventAtomic(ctx, organization, topicName, event)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	eh.logger.Debug("Event published",
		zap.String("organization", organization),
		zap.String("topic", string(topicName)),
		zap.Int64("id", id),
		zap.String("version", version),
	)

	return nil
}

// RegisterSubscription registers a channel to receive events for a topic
func (eh *eventHub) RegisterSubscription(topicName TopicName, eventChan chan<- []Event) error {
	if err := eh.registry.addSubscriber(topicName, eventChan); err != nil {
		return err
	}

	eh.logger.Info("Subscription registered",
		zap.String("topic", string(topicName)),
	)

	return nil
}

// CleanUpEvents removes events within the specified time range
func (eh *eventHub) CleanUpEvents(ctx context.Context, timeFrom, timeEnd time.Time) error {
	for _, t := range eh.registry.getAll() {
		_, err := eh.store.cleanupEvents(ctx, t.name, timeFrom, timeEnd)
		if err != nil {
			eh.logger.Error("Failed to cleanup events for topic",
				zap.String("topic", string(t.name)),
				zap.Error(err),
			)
		}
	}
	return nil
}

// cleanupLoop runs periodic cleanup of old events
func (eh *eventHub) cleanupLoop() {
	defer eh.wg.Done()

	ticker := time.NewTicker(eh.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-eh.cleanupCtx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-eh.config.RetentionPeriod)
			if err := eh.store.cleanupAllTopics(eh.cleanupCtx, cutoff); err != nil {
				eh.logger.Error("Periodic cleanup failed", zap.Error(err))
			}
		}
	}
}

// Close gracefully shuts down the EventHub
func (eh *eventHub) Close() error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if !eh.initialized {
		return nil
	}

	eh.logger.Info("Shutting down EventHub")

	// Stop cleanup loop
	if eh.cleanupCancel != nil {
		eh.cleanupCancel()
	}

	// Stop poller
	if eh.poller != nil {
		eh.poller.stop()
	}

	// Wait for goroutines
	eh.wg.Wait()

	eh.initialized = false
	eh.logger.Info("EventHub shutdown complete")
	return nil
}
