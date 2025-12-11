package registry

import (
	"fmt"
	"sync"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// PolicyRegistry provides centralized policy lookup
type PolicyRegistry struct {
	mu sync.RWMutex

	// Policy definitions indexed by "name:version" composite key
	// Example key: "jwtValidation:v1.0.0"
	Definitions map[string]*policy.PolicyDefinition

	// Policy factory functions indexed by "name:version" composite key
	// Factory creates policy instances with metadata, initParams, and params
	Factories map[string]policy.PolicyFactory
}

// Global singleton registry
var globalRegistry *PolicyRegistry
var registryOnce sync.Once

// GetRegistry returns the global policy registry singleton
func GetRegistry() *PolicyRegistry {
	registryOnce.Do(func() {
		globalRegistry = &PolicyRegistry{
			Definitions: make(map[string]*policy.PolicyDefinition),
			Factories:   make(map[string]policy.PolicyFactory),
		}
	})
	return globalRegistry
}

// GetDefinition retrieves a policy definition by name and version
func (r *PolicyRegistry) GetDefinition(name, version string) (*policy.PolicyDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := compositeKey(name, version)
	def, ok := r.Definitions[key]
	if !ok {
		return nil, fmt.Errorf("policy definition not found: %s", key)
	}
	return def, nil
}

// CreateInstance creates a new policy instance for a specific route
// This method is called during BuildPolicyChain for each route-policy combination
func (r *PolicyRegistry) CreateInstance(
	name, version string,
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := compositeKey(name, version)

	factory, ok := r.Factories[key]
	if !ok {
		return nil, fmt.Errorf("policy factory not found: %s", key)
	}

	def, ok := r.Definitions[key]
	if !ok {
		return nil, fmt.Errorf("policy definition not found: %s", key)
	}

	// Extract initParams from PolicyDefinition
	initParams := def.InitParameters
	if initParams == nil {
		initParams = make(map[string]interface{})
	}

	// Call factory to create instance
	instance, err := factory(metadata, initParams, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy instance %s: %w", key, err)
	}

	return instance, nil
}

// GetFactory retrieves a policy factory by name and version
// Useful for validation without creating instances
func (r *PolicyRegistry) GetFactory(name, version string) (policy.PolicyFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := compositeKey(name, version)
	factory, ok := r.Factories[key]
	if !ok {
		return nil, fmt.Errorf("policy factory not found: %s", key)
	}
	return factory, nil
}

// Register registers a policy definition and factory function
func (r *PolicyRegistry) Register(def *policy.PolicyDefinition, factory policy.PolicyFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := compositeKey(def.Name, def.Version)

	// Check for duplicates
	if _, exists := r.Definitions[key]; exists {
		return fmt.Errorf("policy already registered: %s", key)
	}

	r.Definitions[key] = def
	r.Factories[key] = factory
	return nil
}


// compositeKey creates a composite key from name and version
func compositeKey(name, version string) string {
	return fmt.Sprintf("%s:%s", name, version)
}

// DumpPolicies returns all registered policy definitions for debugging
// Returns a copy of the definitions map
func (r *PolicyRegistry) DumpPolicies() map[string]*policy.PolicyDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy of the definitions map
	dump := make(map[string]*policy.PolicyDefinition, len(r.Definitions))
	for key, def := range r.Definitions {
		dump[key] = def
	}
	return dump
}
