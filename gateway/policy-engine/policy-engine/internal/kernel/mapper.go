package kernel

import (
	"sync"

	"github.com/policy-engine/policy-engine/internal/registry"
)

// RouteMapping maps Envoy metadata keys to PolicyChains for route-specific processing
// T049: RouteMapping struct definition
type RouteMapping struct {
	// Metadata key from Envoy (route identifier)
	// Example: "api-v1-private", "public-endpoint"
	MetadataKey string

	// PolicyChain to execute for this route
	// Contains both request and response policies
	Chain *registry.PolicyChain
}

// Kernel represents the integration layer between Envoy and the policy execution engine
// T050: Kernel struct with Routes map
type Kernel struct {
	mu sync.RWMutex

	// Route-to-chain mapping
	// Key: metadata key from Envoy
	// Value: PolicyChain for that route
	Routes map[string]*registry.PolicyChain
}

// NewKernel creates a new Kernel instance
func NewKernel() *Kernel {
	return &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
}

// GetPolicyChainForKey retrieves the policy chain for a given metadata key
// T051: GetPolicyChainForKey method implementation
// Returns nil when no policy chain exists for the route (not an error condition)
func (k *Kernel) GetPolicyChainForKey(key string) *registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()

	return k.Routes[key]
}

// RegisterRoute registers a policy chain for a route
func (k *Kernel) RegisterRoute(metadataKey string, chain *registry.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.Routes[metadataKey] = chain
}

// UnregisterRoute removes a route mapping
func (k *Kernel) UnregisterRoute(metadataKey string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	delete(k.Routes, metadataKey)
}

// DumpRoutes returns a copy of all route mappings for debugging
// Returns a map of route key -> policy chain
func (k *Kernel) DumpRoutes() map[string]*registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Create a copy of the map
	dump := make(map[string]*registry.PolicyChain, len(k.Routes))
	for key, chain := range k.Routes {
		dump[key] = chain
	}
	return dump
}
