package policyv1alpha

import (
	"errors"
	"fmt"
	"sync"
)

// MetadataXDS represents a generic metadata XDS with ID, Resource_Type, and Actual_Resource
type MetadataXDS struct {
	// ID uniquely identifies this resource
	ID string `json:"id" yaml:"id"`

	// ResourceType identifies the type of resource (e.g., "LlmProviderTemplate")
	ResourceType string `json:"resource_type" yaml:"resource_type"`

	// Resource contains the actual resource data as a map
	Resource map[string]interface{} `json:"resource" yaml:"resource"`
}

// Common storage errors
var (
	// ErrMetadataXDSNotFound is returned when a metadata XDS is not found
	ErrMetadataXDSNotFound = errors.New("metadata XDS not found")

	// ErrMetadataXDSConflict is returned when a resource with the same ID already exists
	ErrMetadataXDSConflict = errors.New("metadata XDS already exists")
)

// Singleton instance
var (
	metadataXDSInstance *MetadataXDSStore
	metadataXDSOnce     sync.Once
)

// MetadataXDSStore holds all metadata XDSs in memory for fast access
// Used for non-frequently changing resources like LlmProviderTemplates
type MetadataXDSStore struct {
	mu sync.RWMutex // Protects concurrent access
	// Resources storage: Key: Resource ID → Value: MetadataXDS
	resources map[string]*MetadataXDS
	// Resources by type: Key: Resource Type → Value: map of resources by ID
	resourcesByType map[string]map[string]*MetadataXDS
}

// NewMetadataXDSStore creates a new in-memory metadata XDS store
func NewMetadataXDSStore() *MetadataXDSStore {
	return &MetadataXDSStore{
		resources:       make(map[string]*MetadataXDS),
		resourcesByType: make(map[string]map[string]*MetadataXDS),
	}
}

// GetMetadataXDSStoreInstance provides a shared instance of MetadataXDSStore
func GetMetadataXDSStoreInstance() *MetadataXDSStore {
	metadataXDSOnce.Do(func() {
		metadataXDSInstance = NewMetadataXDSStore()
	})
	return metadataXDSInstance
}

// StoreResource stores a metadata XDS in the in-memory cache
func (lrs *MetadataXDSStore) StoreResource(resource *MetadataXDS) error {
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

	// Check if resource with same ID already exists
	if existing, exists := lrs.resources[resource.ID]; exists {
		// If resource type changed, remove from old type map
		if existing.ResourceType != resource.ResourceType {
			if typeMap, exists := lrs.resourcesByType[existing.ResourceType]; exists {
				delete(typeMap, resource.ID)
				if len(typeMap) == 0 {
					delete(lrs.resourcesByType, existing.ResourceType)
				}
			}
		}
	}

	// Store in main map
	lrs.resources[resource.ID] = resource

	// Store in type-specific map
	if lrs.resourcesByType[resource.ResourceType] == nil {
		lrs.resourcesByType[resource.ResourceType] = make(map[string]*MetadataXDS)
	}
	lrs.resourcesByType[resource.ResourceType][resource.ID] = resource

	return nil
}

// GetResource retrieves a resource by ID
func (lrs *MetadataXDSStore) GetResource(id string) (*MetadataXDS, error) {
	lrs.mu.RLock()
	defer lrs.mu.RUnlock()

	resource, exists := lrs.resources[id]
	if !exists {
		return nil, ErrMetadataXDSNotFound
	}

	return resource, nil
}

// GetResourcesByType retrieves all resources of a specific type
func (lrs *MetadataXDSStore) GetResourcesByType(resourceType string) (map[string]*MetadataXDS, error) {
	lrs.mu.RLock()
	defer lrs.mu.RUnlock()

	typeMap, exists := lrs.resourcesByType[resourceType]
	if !exists {
		return make(map[string]*MetadataXDS), nil
	}

	// Return a copy to prevent external modification
	result := make(map[string]*MetadataXDS)
	for id, resource := range typeMap {
		result[id] = resource
	}

	return result, nil
}

// GetAllResources returns all resources
func (lrs *MetadataXDSStore) GetAllResources() map[string]*MetadataXDS {
	lrs.mu.RLock()
	defer lrs.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*MetadataXDS)
	for id, resource := range lrs.resources {
		result[id] = resource
	}

	return result
}

// RemoveResource removes a resource by ID
func (lrs *MetadataXDSStore) RemoveResource(id string) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	resource, exists := lrs.resources[id]
	if !exists {
		return ErrMetadataXDSNotFound
	}

	// Remove from main map
	delete(lrs.resources, id)

	// Remove from type-specific map
	if typeMap, exists := lrs.resourcesByType[resource.ResourceType]; exists {
		delete(typeMap, id)
		if len(typeMap) == 0 {
			delete(lrs.resourcesByType, resource.ResourceType)
		}
	}

	return nil
}

// RemoveResourcesByType removes all resources of a specific type
func (lrs *MetadataXDSStore) RemoveResourcesByType(resourceType string) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	typeMap, exists := lrs.resourcesByType[resourceType]
	if !exists {
		return nil // No resources to remove
	}

	// Remove from main map
	for id := range typeMap {
		delete(lrs.resources, id)
	}

	// Remove from type-specific map
	delete(lrs.resourcesByType, resourceType)

	return nil
}

// ClearAll removes all resources from the store
func (lrs *MetadataXDSStore) ClearAll() error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	// Clear the main resources map
	lrs.resources = make(map[string]*MetadataXDS)

	// Clear the type-specific maps
	lrs.resourcesByType = make(map[string]map[string]*MetadataXDS)

	return nil
}

// ReplaceAll replaces all resources with the provided set (state-of-the-world approach)
func (lrs *MetadataXDSStore) ReplaceAll(resources []*MetadataXDS) error {
	lrs.mu.Lock()
	defer lrs.mu.Unlock()

	// Clear existing resources
	lrs.resources = make(map[string]*MetadataXDS)
	lrs.resourcesByType = make(map[string]map[string]*MetadataXDS)

	// Add all new resources
	for _, resource := range resources {
		if resource == nil || resource.ID == "" || resource.ResourceType == "" {
			continue // Skip invalid resources
		}

		lrs.resources[resource.ID] = resource

		if lrs.resourcesByType[resource.ResourceType] == nil {
			lrs.resourcesByType[resource.ResourceType] = make(map[string]*MetadataXDS)
		}
		lrs.resourcesByType[resource.ResourceType][resource.ID] = resource
	}

	return nil
}
