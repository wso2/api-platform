package eventhub

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrOrganizationNotFound      = errors.New("organization not found")
	ErrOrganizationAlreadyExists = errors.New("organization already registered")
)

// organization represents an internal organization with its subscriptions and poll state
type organization struct {
	id           string
	subscribers  []chan<- []Event // Registered subscription channels
	subscriberMu sync.RWMutex

	// Polling state
	knownVersion string    // Last known version from organization_states table
	lastPolled   time.Time // Timestamp of last successful poll
}

// organizationRegistry manages all registered organizations
type organizationRegistry struct {
	orgs map[string]*organization
	mu   sync.RWMutex
}

// newOrganizationRegistry creates a new organization registry
func newOrganizationRegistry() *organizationRegistry {
	return &organizationRegistry{
		orgs: make(map[string]*organization),
	}
}

// register adds a new organization to the registry
func (r *organizationRegistry) register(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orgs[id]; exists {
		return ErrOrganizationAlreadyExists
	}

	r.orgs[id] = &organization{
		id:          id,
		subscribers: make([]chan<- []Event, 0),
		lastPolled:  time.Now(), // Start from now, don't replay old events
	}

	return nil
}

// get retrieves an organization by ID
func (r *organizationRegistry) get(id string) (*organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	org, exists := r.orgs[id]
	if !exists {
		return nil, ErrOrganizationNotFound
	}
	return org, nil
}

// addSubscriber adds a subscription channel to an organization
func (r *organizationRegistry) addSubscriber(id string, ch chan<- []Event) error {
	r.mu.RLock()
	org, exists := r.orgs[id]
	r.mu.RUnlock()

	if !exists {
		return ErrOrganizationNotFound
	}

	org.subscriberMu.Lock()
	defer org.subscriberMu.Unlock()
	org.subscribers = append(org.subscribers, ch)
	return nil
}

// removeSubscriber removes a subscription channel from an organization
func (r *organizationRegistry) removeSubscriber(id string, ch chan<- []Event) error {
	r.mu.RLock()
	org, exists := r.orgs[id]
	r.mu.RUnlock()

	if !exists {
		return ErrOrganizationNotFound
	}

	org.subscriberMu.Lock()
	defer org.subscriberMu.Unlock()

	// Find and remove the subscriber
	for i, sub := range org.subscribers {
		if sub == ch {
			org.subscribers = append(org.subscribers[:i], org.subscribers[i+1:]...)
			return nil
		}
	}
	return nil // Not found is not an error
}

// getAll returns all registered organizations
func (r *organizationRegistry) getAll() []*organization {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orgs := make([]*organization, 0, len(r.orgs))
	for _, org := range r.orgs {
		orgs = append(orgs, org)
	}
	return orgs
}

// updatePollState updates the polling state for an organization
func (org *organization) updatePollState(version string, polledAt time.Time) {
	org.knownVersion = version
	org.lastPolled = polledAt
}

// getSubscribers returns a copy of the subscribers list
func (org *organization) getSubscribers() []chan<- []Event {
	org.subscriberMu.RLock()
	defer org.subscriberMu.RUnlock()

	subs := make([]chan<- []Event, len(org.subscribers))
	copy(subs, org.subscribers)
	return subs
}
