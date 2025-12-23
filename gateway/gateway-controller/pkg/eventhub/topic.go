package eventhub

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrTopicNotFound      = errors.New("topic not found")
	ErrTopicAlreadyExists = errors.New("topic already registered")
	ErrTopicTableMissing  = errors.New("events table for topic does not exist")
)

// topic represents an internal topic with its subscriptions and poll state
type topic struct {
	name         TopicName
	subscribers  []chan<- []Event // Registered subscription channels
	subscriberMu sync.RWMutex

	// Polling state
	knownVersion string    // Last known version from states table
	lastPolled   time.Time // Timestamp of last successful poll
}

// topicRegistry manages all registered topics
type topicRegistry struct {
	topics map[TopicName]*topic
	mu     sync.RWMutex
}

// newTopicRegistry creates a new topic registry
func newTopicRegistry() *topicRegistry {
	return &topicRegistry{
		topics: make(map[TopicName]*topic),
	}
}

// register adds a new topic to the registry
func (r *topicRegistry) register(name TopicName) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.topics[name]; exists {
		return ErrTopicAlreadyExists
	}

	r.topics[name] = &topic{
		name:        name,
		subscribers: make([]chan<- []Event, 0),
		lastPolled:  time.Now(), // Start from now, don't replay old events
	}

	return nil
}

// get retrieves a topic by name
func (r *topicRegistry) get(name TopicName) (*topic, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.topics[name]
	if !exists {
		return nil, ErrTopicNotFound
	}
	return t, nil
}

// addSubscriber adds a subscription channel to a topic
func (r *topicRegistry) addSubscriber(name TopicName, ch chan<- []Event) error {
	r.mu.RLock()
	t, exists := r.topics[name]
	r.mu.RUnlock()

	if !exists {
		return ErrTopicNotFound
	}

	t.subscriberMu.Lock()
	defer t.subscriberMu.Unlock()
	t.subscribers = append(t.subscribers, ch)
	return nil
}

// getAll returns all registered topics
func (r *topicRegistry) getAll() []*topic {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topics := make([]*topic, 0, len(r.topics))
	for _, t := range r.topics {
		topics = append(topics, t)
	}
	return topics
}

// updatePollState updates the polling state for a topic
func (t *topic) updatePollState(version string, polledAt time.Time) {
	t.knownVersion = version
	t.lastPolled = polledAt
}

// getSubscribers returns a copy of the subscribers list
func (t *topic) getSubscribers() []chan<- []Event {
	t.subscriberMu.RLock()
	defer t.subscriberMu.RUnlock()

	subs := make([]chan<- []Event, len(t.subscribers))
	copy(subs, t.subscribers)
	return subs
}
