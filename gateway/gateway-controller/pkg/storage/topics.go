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

// TopicManager manages a thread-safe collection of registered topics per API configuration
type TopicManager struct {
	topics map[string]map[string]bool // map[configId]map[topic]bool
	mu     sync.RWMutex
}

// NewTopicManager creates a new TopicManager instance
func NewTopicManager() *TopicManager {
	return &TopicManager{
		topics: make(map[string]map[string]bool),
	}
}

// Add adds a topic for a specific config ID to the manager
// Returns true if the topic was added, false if it already exists
func (tm *TopicManager) Add(configID, topic string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.topics[configID]; !exists {
		tm.topics[configID] = make(map[string]bool)
	}

	if tm.topics[configID][topic] {
		return false // Topic already exists for this config
	}

	tm.topics[configID][topic] = true
	return true
}

// Remove removes a topic for a specific config ID from the manager
// Returns true if the topic was removed, false if it doesn't exist
func (tm *TopicManager) Remove(configID, topic string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.topics[configID]; !exists {
		return false
	}

	if !tm.topics[configID][topic] {
		return false
	}

	delete(tm.topics[configID], topic)

	// Clean up empty config map
	if len(tm.topics[configID]) == 0 {
		delete(tm.topics, configID)
	}

	return true
}

// RemoveAllForConfig removes all topics for a specific config ID
func (tm *TopicManager) RemoveAllForConfig(configID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.topics, configID)
}

// Contains checks if a topic exists for a specific config ID
func (tm *TopicManager) Contains(configID, topic string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if _, exists := tm.topics[configID]; !exists {
		return false
	}

	return tm.topics[configID][topic]
}

// GetAllForConfig returns all topics for a specific config ID
func (tm *TopicManager) GetAllForConfig(configID string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if _, exists := tm.topics[configID]; !exists {
		return []string{}
	}

	topics := make([]string, 0, len(tm.topics[configID]))
	for topic := range tm.topics[configID] {
		topics = append(topics, topic)
	}
	return topics
}

// GetAll returns all topics across all configs as a map[topic]bool
func (tm *TopicManager) GetAll() map[string]bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	allTopics := make(map[string]bool)
	for _, configTopics := range tm.topics {
		for topic := range configTopics {
			allTopics[topic] = true
		}
	}
	return allTopics
}

// GetAllByConfig returns the full nested map structure (for debugging/inspection)
func (tm *TopicManager) GetAllByConfig() map[string]map[string]bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]map[string]bool)
	for configID, configTopics := range tm.topics {
		result[configID] = make(map[string]bool)
		for topic := range configTopics {
			result[configID][topic] = true
		}
	}
	return result
}

// Count returns the total number of unique topics across all configs
func (tm *TopicManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	uniqueTopics := make(map[string]bool)
	for _, configTopics := range tm.topics {
		for topic := range configTopics {
			uniqueTopics[topic] = true
		}
	}
	return len(uniqueTopics)
}

// CountForConfig returns the number of topics for a specific config ID
func (tm *TopicManager) CountForConfig(configID string) int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if _, exists := tm.topics[configID]; !exists {
		return 0
	}

	return len(tm.topics[configID])
}

// Clear removes all topics from the manager
func (tm *TopicManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.topics = make(map[string]map[string]bool)
}
