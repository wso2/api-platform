package registry

import (
	"fmt"
	"sync"

	policy "github.com/wso2/api-platform/sdk/policy/v1alpha"
)

// PolicyRegistry provides centralized policy lookup
type PolicyRegistry struct {
	mu sync.RWMutex

	// Policy definitions indexed by "name:version" composite key
	// Example key: "jwtValidation:v1.0.0"
	Definitions map[string]*policy.PolicyDefinition

	// Policy implementations indexed by "name:version" composite key
	// Value is Policy interface (can be cast to RequestPolicy/ResponsePolicy)
	Implementations map[string]policy.Policy
}

// Global singleton registry
var globalRegistry *PolicyRegistry
var registryOnce sync.Once

// GetRegistry returns the global policy registry singleton
func GetRegistry() *PolicyRegistry {
	registryOnce.Do(func() {
		globalRegistry = &PolicyRegistry{
			Definitions:     make(map[string]*policy.PolicyDefinition),
			Implementations: make(map[string]policy.Policy),
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

// GetImplementation retrieves a policy implementation by name and version
func (r *PolicyRegistry) GetImplementation(name, version string) (policy.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := compositeKey(name, version)
	impl, ok := r.Implementations[key]
	if !ok {
		return nil, fmt.Errorf("policy implementation not found: %s", key)
	}
	return impl, nil
}

// Register registers a policy definition and implementation
func (r *PolicyRegistry) Register(def *policy.PolicyDefinition, impl policy.Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := compositeKey(def.Name, def.Version)

	// Check for duplicates
	if _, exists := r.Definitions[key]; exists {
		return fmt.Errorf("policy already registered: %s", key)
	}

	r.Definitions[key] = def
	r.Implementations[key] = impl
	return nil
}

// RegisterImplementation is a convenience method to register a policy implementation
// without a full PolicyDefinition. It creates a minimal definition automatically.
// This is primarily used by the generated plugin_registry.go code.
func (r *PolicyRegistry) RegisterImplementation(name, version string, impl policy.Policy) error {
	// Create a minimal policy definition
	def := &policy.PolicyDefinition{
		Name:    name,
		Version: version,
	}
	return r.Register(def, impl)
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
