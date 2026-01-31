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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLazyResourceStore(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	assert.NotNil(t, store)
	assert.NotNil(t, store.resourcesByType)
	assert.Equal(t, int64(0), store.GetResourceVersion())
}

func TestLazyResourceStore_Store(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	resource := &LazyResource{
		ID:           "res-1",
		ResourceType: "LlmProviderTemplate",
		Resource: map[string]interface{}{
			"name": "test-resource",
		},
	}

	store.Store(resource)
	assert.Equal(t, 1, store.Count())
}

func TestLazyResourceStore_GetByIDAndType(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	resource := &LazyResource{
		ID:           "res-1",
		ResourceType: "LlmProviderTemplate",
		Resource: map[string]interface{}{
			"name": "test-resource",
		},
	}
	store.Store(resource)

	// Get existing resource
	retrieved, found := store.GetByIDAndType("res-1", "LlmProviderTemplate")
	assert.True(t, found)
	assert.Equal(t, "res-1", retrieved.ID)

	// Get non-existent resource
	_, found = store.GetByIDAndType("non-existent", "LlmProviderTemplate")
	assert.False(t, found)

	// Get with wrong type
	_, found = store.GetByIDAndType("res-1", "WrongType")
	assert.False(t, found)
}

func TestLazyResourceStore_GetByType(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	// Add multiple resources of same type
	for i := 1; i <= 3; i++ {
		resource := &LazyResource{
			ID:           "res-" + string(rune('0'+i)),
			ResourceType: "TypeA",
			Resource: map[string]interface{}{
				"name": "resource-" + string(rune('0'+i)),
			},
		}
		store.Store(resource)
	}

	// Add resource of different type
	resource := &LazyResource{
		ID:           "other-res",
		ResourceType: "TypeB",
		Resource:     map[string]interface{}{"name": "other"},
	}
	store.Store(resource)

	// Get by type
	typeA := store.GetByType("TypeA")
	assert.Len(t, typeA, 3)

	typeB := store.GetByType("TypeB")
	assert.Len(t, typeB, 1)

	// Get non-existent type
	typeC := store.GetByType("TypeC")
	assert.Len(t, typeC, 0)
}

func TestLazyResourceStore_GetAll(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	// Add resources of different types
	store.Store(&LazyResource{ID: "1", ResourceType: "TypeA", Resource: map[string]interface{}{}})
	store.Store(&LazyResource{ID: "2", ResourceType: "TypeA", Resource: map[string]interface{}{}})
	store.Store(&LazyResource{ID: "3", ResourceType: "TypeB", Resource: map[string]interface{}{}})

	all := store.GetAll()
	assert.Len(t, all, 3)
}

func TestLazyResourceStore_RemoveByIDAndType(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	resource := &LazyResource{
		ID:           "res-1",
		ResourceType: "TypeA",
		Resource:     map[string]interface{}{},
	}
	store.Store(resource)
	assert.Equal(t, 1, store.Count())

	// Remove existing resource
	removed := store.RemoveByIDAndType("res-1", "TypeA")
	assert.True(t, removed)
	assert.Equal(t, 0, store.Count())

	// Remove non-existent resource
	removed = store.RemoveByIDAndType("non-existent", "TypeA")
	assert.False(t, removed)

	// Remove with wrong type
	store.Store(&LazyResource{ID: "res-2", ResourceType: "TypeA", Resource: map[string]interface{}{}})
	removed = store.RemoveByIDAndType("res-2", "WrongType")
	assert.False(t, removed)
	assert.Equal(t, 1, store.Count())
}

func TestLazyResourceStore_RemoveByType(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	// Add resources of different types
	store.Store(&LazyResource{ID: "1", ResourceType: "TypeA", Resource: map[string]interface{}{}})
	store.Store(&LazyResource{ID: "2", ResourceType: "TypeA", Resource: map[string]interface{}{}})
	store.Store(&LazyResource{ID: "3", ResourceType: "TypeB", Resource: map[string]interface{}{}})

	assert.Equal(t, 3, store.Count())

	// Remove all of TypeA
	count := store.RemoveByType("TypeA")
	assert.Equal(t, 2, count)
	assert.Equal(t, 1, store.Count())

	// Remove non-existent type
	count = store.RemoveByType("NonExistent")
	assert.Equal(t, 0, count)
}

func TestLazyResourceStore_ResourceVersion(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	// Initial version
	assert.Equal(t, int64(0), store.GetResourceVersion())

	// Increment
	v1 := store.IncrementResourceVersion()
	assert.Equal(t, int64(1), v1)
	assert.Equal(t, int64(1), store.GetResourceVersion())

	// Increment again
	v2 := store.IncrementResourceVersion()
	assert.Equal(t, int64(2), v2)
}

func TestLazyResourceStore_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resource := &LazyResource{
				ID:           "res-" + string(rune('a'+idx)),
				ResourceType: "TypeA",
				Resource:     map[string]interface{}{"idx": idx},
			}
			store.Store(resource)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetAll()
			_ = store.GetByType("TypeA")
			_ = store.Count()
		}()
	}

	wg.Wait()
}

func TestLazyResourceStore_UpdateResource(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	// Store initial resource
	resource := &LazyResource{
		ID:           "res-1",
		ResourceType: "TypeA",
		Resource:     map[string]interface{}{"value": 1},
	}
	store.Store(resource)

	// Update with same ID
	updatedResource := &LazyResource{
		ID:           "res-1",
		ResourceType: "TypeA",
		Resource:     map[string]interface{}{"value": 2},
	}
	store.Store(updatedResource)

	// Should still be 1 resource
	assert.Equal(t, 1, store.Count())

	// Check updated value
	retrieved, found := store.GetByIDAndType("res-1", "TypeA")
	assert.True(t, found)
	assert.Equal(t, 2, retrieved.Resource["value"])
}

func TestLazyResource_Fields(t *testing.T) {
	resource := LazyResource{
		ID:           "test-id",
		ResourceType: "test-type",
		Resource: map[string]interface{}{
			"key": "value",
		},
	}

	assert.Equal(t, "test-id", resource.ID)
	assert.Equal(t, "test-type", resource.ResourceType)
	assert.Equal(t, "value", resource.Resource["key"])
}

func TestLazyResourceStore_RemoveCleanup(t *testing.T) {
	logger := createTestLogger()
	store := NewLazyResourceStore(logger)

	// Add single resource
	store.Store(&LazyResource{ID: "1", ResourceType: "TypeA", Resource: map[string]interface{}{}})

	// Remove it - should clean up the type mapping entirely
	removed := store.RemoveByIDAndType("1", "TypeA")
	assert.True(t, removed)

	// Internal map should be cleaned up
	store.mu.RLock()
	_, exists := store.resourcesByType["TypeA"]
	store.mu.RUnlock()
	assert.False(t, exists)
}
