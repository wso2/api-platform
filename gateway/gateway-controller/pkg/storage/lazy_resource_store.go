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
 * software distributed under an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package storage

import (
	"sync"
	"sync/atomic"

	"log/slog"
)

// LazyResource represents a generic lazy resource with ID, Resource_Type, and Actual_Resource
type LazyResource struct {
	// ID uniquely identifies this resource
	ID string `json:"id" yaml:"id"`
	
	// ResourceType identifies the type of resource (e.g., "LlmProviderTemplate")
	ResourceType string `json:"resource_type" yaml:"resource_type"`
	
	// Resource contains the actual resource data as a map
	Resource map[string]interface{} `json:"resource" yaml:"resource"`
}

// LazyResourceStore manages lazy resources in memory with thread-safe operations
type LazyResourceStore struct {
	mu              sync.RWMutex
	resources       map[string]*LazyResource   // key: resource ID
	resourcesByType map[string]map[string]*LazyResource // key: resource type -> map of ID -> resource
	resourceVersion int64
	logger          *slog.Logger
}

// NewLazyResourceStore creates a new lazy resource store
func NewLazyResourceStore(logger *slog.Logger) *LazyResourceStore {
	return &LazyResourceStore{
		resources:       make(map[string]*LazyResource),
		resourcesByType: make(map[string]map[string]*LazyResource),
		logger:          logger,
	}
}

// Store adds or updates a lazy resource
func (s *LazyResourceStore) Store(resource *LazyResource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old entry if updating and type changed
	if existing, exists := s.resources[resource.ID]; exists {
		if existing.ResourceType != resource.ResourceType {
			s.removeFromTypeMapping(existing)
		}
	}

	// Store the resource
	s.resources[resource.ID] = resource
	s.addToTypeMapping(resource)

	s.logger.Debug("Stored lazy resource",
		slog.String("id", resource.ID),
		slog.String("resource_type", resource.ResourceType))
}

// GetByID retrieves a resource by its ID
func (s *LazyResourceStore) GetByID(id string) (*LazyResource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resource, exists := s.resources[id]
	return resource, exists
}

// GetByType retrieves all resources of a specific type
func (s *LazyResourceStore) GetByType(resourceType string) map[string]*LazyResource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	typeMap, exists := s.resourcesByType[resourceType]
	if !exists {
		return make(map[string]*LazyResource)
	}

	// Return a copy to avoid external modification
	result := make(map[string]*LazyResource)
	for id, resource := range typeMap {
		result[id] = resource
	}
	return result
}

// GetAll retrieves all resources
func (s *LazyResourceStore) GetAll() []*LazyResource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*LazyResource, 0, len(s.resources))
	for _, resource := range s.resources {
		result = append(result, resource)
	}
	return result
}

// RemoveByID removes a resource by its ID
func (s *LazyResourceStore) RemoveByID(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	resource, exists := s.resources[id]
	if !exists {
		return false
	}

	delete(s.resources, id)
	s.removeFromTypeMapping(resource)

	s.logger.Debug("Removed lazy resource",
		slog.String("id", id),
		slog.String("resource_type", resource.ResourceType))

	return true
}

// RemoveByType removes all resources of a specific type
func (s *LazyResourceStore) RemoveByType(resourceType string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	typeMap, exists := s.resourcesByType[resourceType]
	if !exists {
		return 0
	}

	count := len(typeMap)

	// Remove from main map
	for id := range typeMap {
		delete(s.resources, id)
	}

	// Remove from type mapping
	delete(s.resourcesByType, resourceType)

	s.logger.Debug("Removed lazy resources by type",
		slog.String("resource_type", resourceType),
		slog.Int("count", count))

	return count
}

// Count returns the total number of resources
func (s *LazyResourceStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.resources)
}

// IncrementResourceVersion increments and returns the resource version
func (s *LazyResourceStore) IncrementResourceVersion() int64 {
	return atomic.AddInt64(&s.resourceVersion, 1)
}

// GetResourceVersion returns the current resource version
func (s *LazyResourceStore) GetResourceVersion() int64 {
	return atomic.LoadInt64(&s.resourceVersion)
}

// addToTypeMapping adds a resource to the type mapping
func (s *LazyResourceStore) addToTypeMapping(resource *LazyResource) {
	if s.resourcesByType[resource.ResourceType] == nil {
		s.resourcesByType[resource.ResourceType] = make(map[string]*LazyResource)
	}
	s.resourcesByType[resource.ResourceType][resource.ID] = resource
}

// removeFromTypeMapping removes a resource from the type mapping
func (s *LazyResourceStore) removeFromTypeMapping(resource *LazyResource) {
	typeMap, exists := s.resourcesByType[resource.ResourceType]
	if !exists {
		return
	}

	delete(typeMap, resource.ID)

	// If no resources left for this type, remove the mapping
	if len(typeMap) == 0 {
		delete(s.resourcesByType, resource.ResourceType)
	}
}

