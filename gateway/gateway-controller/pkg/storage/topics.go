/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package storage

import (
	"sync"
)

// TopicManager manages a thread-safe collection of registered topics
type TopicManager struct {
	topics map[string]string
	mu     sync.RWMutex
}

// NewTopicManager creates a new TopicManager instance
func NewTopicManager() *TopicManager {
	return &TopicManager{
		topics: make(map[string]string),
	}
}

// Add adds a topic to the manager
// Returns true if the topic was added, false if it already exists
func (tm *TopicManager) Add(topic string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.topics[topic]; exists {
		return false
	}

	tm.topics[topic] = topic
	return true
}

// Remove removes a topic from the manager
// Returns true if the topic was removed, false if it doesn't exist
func (tm *TopicManager) Remove(topic string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.topics[topic]; !exists {
		return false
	}

	delete(tm.topics, topic)
	return true
}

// Contains checks if a topic exists in the manager
func (tm *TopicManager) Contains(topic string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	_, exists := tm.topics[topic]
	return exists
}

// GetAll returns all registered topics as a slice
func (tm *TopicManager) GetAll() map[string]string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// topics := make([]string, 0, len(tm.topics))
	// for topic := range tm.topics {
	// 	topics = append(topics, topic)
	// }
	return tm.topics
}

// Count returns the number of registered topics
func (tm *TopicManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.topics)
}

// Clear removes all topics from the manager
func (tm *TopicManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.topics = make(map[string]string)
}
