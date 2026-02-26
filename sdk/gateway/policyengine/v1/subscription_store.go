package policyenginev1

import "sync"

// SubscriptionStore stores subscription state in memory for fast lookups by policies.
// It is populated by the xDS client in the policy engine and read by policies such
// as subscriptionValidation during request processing.
type SubscriptionStore struct {
	mu sync.RWMutex
	// apiId -> applicationId -> status
	data map[string]map[string]string
}

var (
	subscriptionStoreInstance *SubscriptionStore
	subscriptionStoreOnce     sync.Once
)

// NewSubscriptionStore creates a new subscription store.
func NewSubscriptionStore() *SubscriptionStore {
	return &SubscriptionStore{
		data: make(map[string]map[string]string),
	}
}

// GetSubscriptionStoreInstance returns the singleton subscription store instance.
// This is the primary access point for both the xDS client and policies.
func GetSubscriptionStoreInstance() *SubscriptionStore {
	subscriptionStoreOnce.Do(func() {
		subscriptionStoreInstance = NewSubscriptionStore()
	})
	return subscriptionStoreInstance
}

// ReplaceAll replaces the entire subscription state atomically.
// This follows a "state of the world" model where the caller provides
// a full snapshot of all known subscriptions.
func (s *SubscriptionStore) ReplaceAll(subs []SubscriptionData) {
	// Build the new snapshot off-lock to minimize write-lock hold time and
	// avoid stalling request-path IsActive lookups under large snapshots.
	newData := make(map[string]map[string]string, len(subs))
	for _, sub := range subs {
		if sub.APIId == "" || sub.ApplicationId == "" {
			continue
		}
		apps, ok := newData[sub.APIId]
		if !ok {
			apps = make(map[string]string)
			newData[sub.APIId] = apps
		}
		apps[sub.ApplicationId] = sub.Status
	}

	s.mu.Lock()
	s.data = newData
	s.mu.Unlock()
}

// IsActive returns true if the given application is actively subscribed to the API.
// Currently this treats status "ACTIVE" as the only allowed value.
func (s *SubscriptionStore) IsActive(apiID, applicationID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if apiID == "" || applicationID == "" {
		return false
	}
	apps, ok := s.data[apiID]
	if !ok {
		return false
	}
	status, ok := apps[applicationID]
	if !ok {
		return false
	}
	return status == "ACTIVE"
}

