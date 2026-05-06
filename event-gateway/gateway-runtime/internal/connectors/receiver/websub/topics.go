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

package websub

import "sync"

// TopicRegistry manages the set of valid topics/channels that subscribers can subscribe to.
type TopicRegistry struct {
	mu     sync.RWMutex
	topics map[string]bool
}

// NewTopicRegistry creates a new empty topic registry.
func NewTopicRegistry() *TopicRegistry {
	return &TopicRegistry{
		topics: make(map[string]bool),
	}
}

// Register adds a topic to the registry.
func (r *TopicRegistry) Register(topic string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.topics[topic] = true
}

// Deregister removes a topic from the registry.
func (r *TopicRegistry) Deregister(topic string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.topics, topic)
}

// IsRegistered checks whether a topic is registered.
func (r *TopicRegistry) IsRegistered(topic string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.topics[topic]
}

// ListTopics returns all registered topics.
func (r *TopicRegistry) ListTopics() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	topics := make([]string, 0, len(r.topics))
	for t := range r.topics {
		topics = append(topics, t)
	}
	return topics
}
