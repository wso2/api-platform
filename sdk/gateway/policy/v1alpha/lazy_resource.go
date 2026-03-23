package policyv1alpha

import (
	"errors"
	"fmt"
	"sync"
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

// Common storage errors
var (
	// ErrLazyResourceNotFound is returned when a lazy resource is not found
	ErrLazyResourceNotFound = errors.New("lazy resource not found")

	// ErrLazyResourceConflict is returned when a resource with the same ID already exists
	ErrLazyResourceConflict = errors.New("lazy resource already exists")
)

// Singleton instance
var (
	lazyResourceInstance *LazyResourceStore
	lazyResourceOnce     sync.Once
)

// compositeKey creates a unique key from resource type and ID
func compositeKey(resourceType, id string) string {
	return resourceType + ":" + id
}

// LazyResourceStore holds all lazy resources in memory for fast access
// Used for non-frequently changing resources like LlmProviderTemplates
type LazyResourceStore struct {
	mu sync.RWMutex // Protects concurrent access
	// Resources storage: Key: compositeKey(ResourceType, ID) → Value: LazyResource
	resources map[string]*LazyResource
	// Resources by type: Key: Resource Type → Value: map of resources by ID
	resourcesByType map[string]map[string]*LazyResource
}

// NewLazyResourceStore creates a new in-memory lazy resource store
func NewLazyResourceStore() *LazyResourceStore {
	return &LazyResourceStore{
		resources:       make(map[string]*LazyResource),
		resourcesByType: make(map[string]map[string]*LazyResource),
	}
}

// GetLazyResourceStoreInstance provides a shared instance of LazyResourceStore
func GetLazyResourceStoreInstance() *LazyResourceStore {
	lazyResourceOnce.Do(func() {
		lazyResourceInstance = NewLazyResourceStore()
	})
	return lazyResourceInstance
}

// StoreResource stores a lazy resource in the in-memory cache
func (lrs *LazyResourceStore) StoreResource(resource *LazyResource) error {
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	if resource.ID == "" {
		return fmt.Errorf("resource ID cannot be empty")
	}

	if resource.ResourceType == "" {
		return fmt.Errorf("resource type cannot be empty")
	}

	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	key := compositeKey(resource.ResourceType, resource.ID)

	// Remove from type mapping if it already exists (to handle updates)
	if existing, exists := lrs.resources[key]; exists {
		lrs.removeFromTypeMapping(existing)
	}

	// Store in main map with composite key
	lrs.resources[key] = resource

	// Store in type-specific map
	lrs.addToTypeMapping(resource)

	return nil
}

// GetResourceByIDAndType retrieves a resource by ID and type (precise)
func (lrs *LazyResourceStore) GetResourceByIDAndType(id, resourceType string) (*LazyResource, error) {
	lrs.mu.RLock()
	defer lrs.mu.RUnlock()

	key := compositeKey(resourceType, id)
	resource, exists := lrs.resources[key]
	if !exists {
		return nil, ErrLazyResourceNotFound
	}

	return resource, nil
}

// GetResourcesByType retrieves all resources of a specific type
func (lrs *LazyResourceStore) GetResourcesByType(resourceType string) (map[string]*LazyResource, error) {
	lrs.mu.RLock()
	defer lrs.mu.RUnlock()

	typeMap, exists := lrs.resourcesByType[resourceType]
	if !exists {
		return make(map[string]*LazyResource), nil
	}

	// Return a copy to prevent external modification
	result := make(map[string]*LazyResource)
	for id, resource := range typeMap {
		result[id] = resource
	}

	return result, nil
}

// GetAllResources returns all resources
func (lrs *LazyResourceStore) GetAllResources() map[string]*LazyResource {
	lrs.mu.RLock()
	defer lrs.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*LazyResource)
	for key, resource := range lrs.resources {
		result[key] = resource
	}

	return result
}

// RemoveResource removes a resource by ID (ambiguous if multiple types have same ID)
func (lrs *LazyResourceStore) RemoveResource(id string) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	// Search for the resource with matching ID
	var keyToDelete string
	var resourceToDelete *LazyResource
	for key, resource := range lrs.resources {
		if resource.ID == id {
			keyToDelete = key
			resourceToDelete = resource
			break
		}
	}

	if resourceToDelete == nil {
		return ErrLazyResourceNotFound
	}

	// Remove from main map
	delete(lrs.resources, keyToDelete)

	// Remove from type-specific map
	lrs.removeFromTypeMapping(resourceToDelete)

	return nil
}

// RemoveResourceByIDAndType removes a resource by ID and type (precise)
func (lrs *LazyResourceStore) RemoveResourceByIDAndType(id, resourceType string) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	key := compositeKey(resourceType, id)
	resource, exists := lrs.resources[key]
	if !exists {
		return ErrLazyResourceNotFound
	}

	// Remove from main map
	delete(lrs.resources, key)

	// Remove from type-specific map
	lrs.removeFromTypeMapping(resource)

	return nil
}

// RemoveResourcesByType removes all resources of a specific type
func (lrs *LazyResourceStore) RemoveResourcesByType(resourceType string) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	typeMap, exists := lrs.resourcesByType[resourceType]
	if !exists {
		return nil // No resources to remove
	}

	// Remove from main map
	for id := range typeMap {
		key := compositeKey(resourceType, id)
		delete(lrs.resources, key)
	}

	// Remove from type-specific map
	delete(lrs.resourcesByType, resourceType)

	return nil
}

// ClearAll removes all resources from the store
func (lrs *LazyResourceStore) ClearAll() error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	// Clear the main resources map
	lrs.resources = make(map[string]*LazyResource)

	// Clear the type-specific maps
	lrs.resourcesByType = make(map[string]map[string]*LazyResource)

	return nil
}

// ReplaceAll replaces all resources with the provided set (state-of-the-world approach)
func (lrs *LazyResourceStore) ReplaceAll(resources []*LazyResource) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	// Clear existing resources
	lrs.resources = make(map[string]*LazyResource)
	lrs.resourcesByType = make(map[string]map[string]*LazyResource)

	// Add all new resources
	for _, resource := range resources {
		if resource == nil || resource.ID == "" || resource.ResourceType == "" {
			continue // Skip invalid resources
		}

		key := compositeKey(resource.ResourceType, resource.ID)
		lrs.resources[key] = resource

		lrs.addToTypeMapping(resource)
	}

	return nil
}

// addToTypeMapping adds a resource to the type-specific map (caller must hold lock)
func (lrs *LazyResourceStore) addToTypeMapping(resource *LazyResource) {
	if lrs.resourcesByType[resource.ResourceType] == nil {
		lrs.resourcesByType[resource.ResourceType] = make(map[string]*LazyResource)
	}
	lrs.resourcesByType[resource.ResourceType][resource.ID] = resource
}

// removeFromTypeMapping removes a resource from the type-specific map (caller must hold lock)
func (lrs *LazyResourceStore) removeFromTypeMapping(resource *LazyResource) {
	if typeMap, exists := lrs.resourcesByType[resource.ResourceType]; exists {
		delete(typeMap, resource.ID)
		if len(typeMap) == 0 {
			delete(lrs.resourcesByType, resource.ResourceType)
		}
	}
}
