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

package subscription

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubscriptionState represents the state of a subscription.
type SubscriptionState string

const (
	StatePending  SubscriptionState = "pending"
	StateActive   SubscriptionState = "active"
	StateInactive SubscriptionState = "inactive"
	StateExpired  SubscriptionState = "expired"
)

// Subscription represents a WebSub subscription.
type Subscription struct {
	ID           string            `json:"id"`
	Topic        string            `json:"topic"`
	CallbackURL  string            `json:"callback_url"`
	Secret       string            `json:"secret,omitempty"`
	LeaseSeconds int               `json:"lease_seconds"`
	ExpiresAt    time.Time         `json:"expires_at,omitempty"`
	State        SubscriptionState `json:"state"`
	CreatedAt    time.Time         `json:"created_at"`
	RuntimeID    string            `json:"runtime_id"`
}

// SubscriptionStore manages subscription storage and retrieval.
type SubscriptionStore interface {
	Add(sub *Subscription) error
	Remove(topic, callbackURL string) error
	GetByTopic(topic string) []*Subscription
	GetActive() []*Subscription
	GetAll() []*Subscription
	UpdateState(id string, state SubscriptionState) error
	ExistsByCallback(callbackURL string) bool
}

// InMemoryStore is a thread-safe in-memory implementation of SubscriptionStore.
type InMemoryStore struct {
	mu        sync.RWMutex
	byID      map[string]*Subscription
	byTopic   map[string]map[string]*Subscription // topic -> callbackURL -> sub
	runtimeID string
}

// NewInMemoryStore creates a new in-memory subscription store.
func NewInMemoryStore(runtimeID string) *InMemoryStore {
	return &InMemoryStore{
		byID:      make(map[string]*Subscription),
		byTopic:   make(map[string]map[string]*Subscription),
		runtimeID: runtimeID,
	}
}

// Add adds a subscription to the store.
func (s *InMemoryStore) Add(sub *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	if sub.RuntimeID == "" {
		sub.RuntimeID = s.runtimeID
	}

	s.byID[sub.ID] = sub
	if _, ok := s.byTopic[sub.Topic]; !ok {
		s.byTopic[sub.Topic] = make(map[string]*Subscription)
	}
	s.byTopic[sub.Topic][sub.CallbackURL] = sub
	return nil
}

// Remove removes a subscription by topic and callback URL.
func (s *InMemoryStore) Remove(topic, callbackURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	topicSubs, ok := s.byTopic[topic]
	if !ok {
		return fmt.Errorf("no subscriptions for topic: %s", topic)
	}

	sub, ok := topicSubs[callbackURL]
	if !ok {
		return fmt.Errorf("subscription not found: topic=%s callback=%s", topic, callbackURL)
	}

	delete(s.byID, sub.ID)
	delete(topicSubs, callbackURL)
	if len(topicSubs) == 0 {
		delete(s.byTopic, topic)
	}

	return nil
}

// GetByTopic returns all subscriptions for a given topic.
func (s *InMemoryStore) GetByTopic(topic string) []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topicSubs, ok := s.byTopic[topic]
	if !ok {
		return nil
	}

	result := make([]*Subscription, 0, len(topicSubs))
	for _, sub := range topicSubs {
		result = append(result, sub)
	}
	return result
}

// GetActive returns all active subscriptions.
func (s *InMemoryStore) GetActive() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Subscription
	for _, sub := range s.byID {
		if sub.State == StateActive {
			result = append(result, sub)
		}
	}
	return result
}

// GetAll returns all subscriptions.
func (s *InMemoryStore) GetAll() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Subscription, 0, len(s.byID))
	for _, sub := range s.byID {
		result = append(result, sub)
	}
	return result
}

// UpdateState updates the state of a subscription by ID.
func (s *InMemoryStore) UpdateState(id string, state SubscriptionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.byID[id]
	if !ok {
		return fmt.Errorf("subscription not found: %s", id)
	}

	sub.State = state
	return nil
}

// ExistsByCallback returns true if any subscription exists for the given callback URL.
func (s *InMemoryStore) ExistsByCallback(callbackURL string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.byID {
		if sub.CallbackURL == callbackURL {
			return true
		}
	}
	return false
}

// CleanExpired removes expired subscriptions. Should be called periodically.
func (s *InMemoryStore) CleanExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, sub := range s.byID {
		if sub.LeaseSeconds > 0 && !sub.ExpiresAt.IsZero() && now.After(sub.ExpiresAt) {
			delete(s.byID, id)
			if topicSubs, ok := s.byTopic[sub.Topic]; ok {
				delete(topicSubs, sub.CallbackURL)
				if len(topicSubs) == 0 {
					delete(s.byTopic, sub.Topic)
				}
			}
			removed++
		}
	}
	return removed
}
