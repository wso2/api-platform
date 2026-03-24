package policyengine

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// HashSubscriptionToken computes a SHA-256 hash of the token.
// Must match the hashing used by the gateway-controller storage for lookups.
func HashSubscriptionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// SubscriptionEntry holds the subscription state and rate limit info from the plan.
type SubscriptionEntry struct {
	Status             string
	ThrottleLimitCount int
	ThrottleLimitUnit  string
	StopOnQuotaReach   bool
}

// SubscriptionStore stores subscription state in memory for fast lookups by policies.
// It supports two lookup patterns:
//   - By token: apiId + subscriptionToken (primary, new design)
//   - By applicationId: apiId + applicationId (legacy/backward compat)
type SubscriptionStore struct {
	mu sync.RWMutex
	// apiId -> applicationId -> SubscriptionEntry (legacy, full entry for quota/throttle)
	appData map[string]map[string]*SubscriptionEntry
	// apiId -> subscriptionToken -> SubscriptionEntry (new)
	tokenData map[string]map[string]*SubscriptionEntry
}

var (
	subscriptionStoreInstance *SubscriptionStore
	subscriptionStoreOnce     sync.Once
)

// NewSubscriptionStore creates a new subscription store.
func NewSubscriptionStore() *SubscriptionStore {
	return &SubscriptionStore{
		appData:   make(map[string]map[string]*SubscriptionEntry),
		tokenData: make(map[string]map[string]*SubscriptionEntry),
	}
}

// GetSubscriptionStoreInstance returns the singleton subscription store instance.
func GetSubscriptionStoreInstance() *SubscriptionStore {
	subscriptionStoreOnce.Do(func() {
		subscriptionStoreInstance = NewSubscriptionStore()
	})
	return subscriptionStoreInstance
}

// ReplaceAll replaces the entire subscription state atomically.
func (s *SubscriptionStore) ReplaceAll(subs []SubscriptionData) {
	newAppData := make(map[string]map[string]*SubscriptionEntry, len(subs))
	newTokenData := make(map[string]map[string]*SubscriptionEntry, len(subs))

	for _, sub := range subs {
		if sub.APIId == "" {
			continue
		}

		entry := &SubscriptionEntry{
			Status:             sub.Status,
			ThrottleLimitCount: sub.ThrottleLimitCount,
			ThrottleLimitUnit:  sub.ThrottleLimitUnit,
			StopOnQuotaReach:   sub.StopOnQuotaReach,
		}

		// Build token-based index
		if sub.SubscriptionToken != "" {
			tokens, ok := newTokenData[sub.APIId]
			if !ok {
				tokens = make(map[string]*SubscriptionEntry)
				newTokenData[sub.APIId] = tokens
			}
			tokens[sub.SubscriptionToken] = entry
		}

		// Build applicationId-based index (legacy, full entry for quota/throttle)
		if sub.ApplicationId != "" {
			apps, ok := newAppData[sub.APIId]
			if !ok {
				apps = make(map[string]*SubscriptionEntry)
				newAppData[sub.APIId] = apps
			}
			apps[sub.ApplicationId] = entry
		}
	}

	s.mu.Lock()
	s.appData = newAppData
	s.tokenData = newTokenData
	s.mu.Unlock()
}

// IsActive returns true if the given application is actively subscribed to the API.
// This is the legacy lookup path using applicationId.
func (s *SubscriptionStore) IsActive(apiID, applicationID string) bool {
	active, _ := s.IsActiveByApplication(apiID, applicationID)
	return active
}

// IsActiveByApplication checks if an application is actively subscribed to the given API.
// Returns (true, entry) if the application has ACTIVE status, (false, nil) otherwise.
// The entry includes quota/throttle metadata for rate limit enforcement.
func (s *SubscriptionStore) IsActiveByApplication(apiID, applicationID string) (bool, *SubscriptionEntry) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if apiID == "" || applicationID == "" {
		return false, nil
	}
	apps, ok := s.appData[apiID]
	if !ok {
		return false, nil
	}
	entry, ok := apps[applicationID]
	if !ok {
		return false, nil
	}
	return entry != nil && entry.Status == "ACTIVE", entry
}

// IsActiveByToken checks if a subscription token is active for the given API.
// Returns (true, entry) if the token has ACTIVE status, (false, nil) otherwise.
// The returned entry is a copy to avoid leaking mutable state from the store.
func (s *SubscriptionStore) IsActiveByToken(apiID, token string) (bool, *SubscriptionEntry) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if apiID == "" || token == "" {
		return false, nil
	}
	tokens, ok := s.tokenData[apiID]
	if !ok {
		return false, nil
	}
	entry, ok := tokens[token]
	if !ok {
		return false, nil
	}
	if entry.Status == "ACTIVE" {
		e := *entry
		return true, &e
	}
	return false, nil
}
