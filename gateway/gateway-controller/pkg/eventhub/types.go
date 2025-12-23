package eventhub

import (
	"context"
	"time"
)

// TopicName represents a unique topic identifier
type TopicName string

// Event represents a single event in the hub
type Event struct {
	ID                  int64
	TopicName           TopicName
	ProcessedTimestamp  time.Time // When event was recorded in DB
	OriginatedTimestamp time.Time // When event was created
	EventData           []byte    // JSON serialized payload
}

// TopicState represents the version state for a topic
type TopicState struct {
	Organization string
	TopicName    TopicName
	VersionID    string    // UUID that changes on every modification
	UpdatedAt    time.Time
}

// EventHub is the main interface for the message broker
type EventHub interface {
	// Initialize sets up database connections and starts background poller
	Initialize(ctx context.Context) error

	// RegisterTopic registers a topic
	// Returns error if the events table for this topic does not exist
	// Creates entry in States table with empty version
	RegisterTopic(organization string, topicName TopicName) error

	// PublishEvent publishes an event to a topic
	// Updates the states table and events table atomically
	PublishEvent(ctx context.Context, organization string, topicName TopicName, eventData []byte) error

	// RegisterSubscription registers a channel to receive events for a topic
	// Events are delivered as batches (arrays) based on poll cycle
	RegisterSubscription(topicName TopicName, eventChan chan<- []Event) error

	// CleanUpEvents removes events between the specified time range
	CleanUpEvents(ctx context.Context, timeFrom, timeEnd time.Time) error

	// Close gracefully shuts down the EventHub
	Close() error
}

// Config holds EventHub configuration
type Config struct {
	// PollInterval is how often to poll for state changes
	PollInterval time.Duration
	// CleanupInterval is how often to run automatic cleanup
	CleanupInterval time.Duration
	// RetentionPeriod is how long to keep events (default 1 hour)
	RetentionPeriod time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		PollInterval:    time.Second * 5,
		CleanupInterval: time.Minute * 10,
		RetentionPeriod: time.Hour,
	}
}
