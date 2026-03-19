package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy"

// LazyResource represents a generic lazy resource with ID, Resource_Type, and Actual_Resource.
type LazyResource = core.LazyResource

// LazyResourceStore holds all lazy resources in memory for fast access.
// Used for non-frequently changing resources like LlmProviderTemplates.
type LazyResourceStore = core.LazyResourceStore

// Common storage errors
var (
	// ErrLazyResourceNotFound is returned when a lazy resource is not found.
	ErrLazyResourceNotFound = core.ErrLazyResourceNotFound

	// ErrLazyResourceConflict is returned when a resource with the same ID already exists.
	ErrLazyResourceConflict = core.ErrLazyResourceConflict
)

// NewLazyResourceStore creates a new in-memory lazy resource store.
func NewLazyResourceStore() *LazyResourceStore {
	return core.NewLazyResourceStore()
}

// GetLazyResourceStoreInstance provides a shared instance of LazyResourceStore.
// This ensures only ONE singleton exists (in the core package).
func GetLazyResourceStoreInstance() *LazyResourceStore {
	return core.GetLazyResourceStoreInstance()
}
