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
	// ID uniquely identifies this resource within its type
	ID string `json:"id" yaml:"id"`

	// ResourceType identifies the type of resource (e.g., "LlmProviderTemplate")
	ResourceType string `json:"resource_type" yaml:"resource_type"`

	// Resource contains the actual resource data as a map
	Resource map[string]interface{} `json:"resource" yaml:"resource"`
}

// LazyResourceStore manages lazy resources in memory with thread-safe operations
type LazyResourceStore struct {
	mu              sync.RWMutex
	resourcesByType map[string]map[string]*LazyResource // key: resource type -> map of ID -> resource
	resourceVersion int64
	logger          *slog.Logger
}

// NewLazyResourceStore creates a new lazy resource store
func NewLazyResourceStore(logger *slog.Logger) *LazyResourceStore {
	return &LazyResourceStore{
		resourcesByType: make(map[string]map[string]*LazyResource),
		logger:          logger,
	}
}

// Store adds or updates a lazy resource
func (s *LazyResourceStore) Store(resource *LazyResource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.addToTypeMapping(resource)

	s.logger.Debug("Stored lazy resource",
		slog.String("id", resource.ID),
		slog.String("resource_type", resource.ResourceType))
}

// GetByIDAndType retrieves a resource by its ID and type (precise lookup)
func (s *LazyResourceStore) GetByIDAndType(id, resourceType string) (*LazyResource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if typeMap, exists := s.resourcesByType[resourceType]; exists {
		resource, exists := typeMap[id]
		return resource, exists
	}
	return nil, false
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

	result := make([]*LazyResource, 0)
	for _, typeMap := range s.resourcesByType {
		for _, resource := range typeMap {
			result = append(result, resource)
		}
	}
	return result
}

// RemoveByIDAndType removes a resource by its ID and type (precise removal)
func (s *LazyResourceStore) RemoveByIDAndType(id, resourceType string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	typeMap, exists := s.resourcesByType[resourceType]
	if !exists {
		return false
	}

	if _, exists := typeMap[id]; exists {
		delete(typeMap, id)
		if len(typeMap) == 0 {
			delete(s.resourcesByType, resourceType)
		}
		s.logger.Debug("Removed lazy resource by ID and type",
			slog.String("id", id),
			slog.String("resource_type", resourceType))
		return true
	}

	return false
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

	count := 0
	for _, typeMap := range s.resourcesByType {
		count += len(typeMap)
	}
	return count
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
