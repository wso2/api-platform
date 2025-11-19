package core

import (
	"fmt"
	"sync"

	"github.com/yourorg/policy-engine/worker/policies"
)

// PolicyChain is a container for a complete policy processing pipeline for a route
type PolicyChain struct {
	// Ordered list of policies to execute during request phase
	// Type-filtered at build time - all implement RequestPolicy interface
	RequestPolicies []policies.RequestPolicy

	// Ordered list of policies to execute during response phase
	// Type-filtered at build time - all implement ResponsePolicy interface
	ResponsePolicies []policies.ResponsePolicy

	// Policy specifications for request policies (aligned with RequestPolicies)
	RequestPolicySpecs []policies.PolicySpec

	// Policy specifications for response policies (aligned with ResponsePolicies)
	ResponsePolicySpecs []policies.PolicySpec

	// Shared metadata map for inter-policy communication
	// Initialized fresh for each request, persists through response phase
	// Key: string, Value: any (policy-specific data)
	Metadata map[string]interface{}

	// Computed flag: true if any RequestPolicy requires request body access
	// Determines whether ext_proc uses SKIP or BUFFERED mode for request body
	RequiresRequestBody bool

	// Computed flag: true if any ResponsePolicy requires response body access
	// Determines whether ext_proc uses SKIP or BUFFERED mode for response body
	RequiresResponseBody bool
}

// PolicyRegistry provides centralized policy lookup
type PolicyRegistry struct {
	mu sync.RWMutex

	// Policy definitions indexed by "name:version" composite key
	// Example key: "jwtValidation:v1.0.0"
	Definitions map[string]*policies.PolicyDefinition

	// Policy implementations indexed by "name:version" composite key
	// Value is Policy interface (can be cast to RequestPolicy/ResponsePolicy)
	Implementations map[string]policies.Policy
}

// Global singleton registry
var globalRegistry *PolicyRegistry
var registryOnce sync.Once

// GetRegistry returns the global policy registry singleton
func GetRegistry() *PolicyRegistry {
	registryOnce.Do(func() {
		globalRegistry = &PolicyRegistry{
			Definitions:     make(map[string]*policies.PolicyDefinition),
			Implementations: make(map[string]policies.Policy),
		}
	})
	return globalRegistry
}

// GetDefinition retrieves a policy definition by name and version
func (r *PolicyRegistry) GetDefinition(name, version string) (*policies.PolicyDefinition, error) {
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
func (r *PolicyRegistry) GetImplementation(name, version string) (policies.Policy, error) {
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
func (r *PolicyRegistry) Register(def *policies.PolicyDefinition, impl policies.Policy) error {
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

// compositeKey creates a composite key from name and version
func compositeKey(name, version string) string {
	return fmt.Sprintf("%s:%s", name, version)
}
