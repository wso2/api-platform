package kernel

import (
	"fmt"
	"sync"

	"github.com/yourorg/policy-engine/worker/core"
	"github.com/yourorg/policy-engine/worker/policies"
)

// RouteMapping maps Envoy metadata keys to PolicyChains for route-specific processing
// T049: RouteMapping struct definition
type RouteMapping struct {
	// Metadata key from Envoy (route identifier)
	// Example: "api-v1-private", "public-endpoint"
	MetadataKey string

	// PolicyChain to execute for this route
	// Contains both request and response policies
	Chain *core.PolicyChain
}

// Kernel represents the integration layer between Envoy and the policy execution engine
// T050: Kernel struct with Routes map and ContextStorage map
type Kernel struct {
	mu sync.RWMutex

	// Route-to-chain mapping
	// Key: metadata key from Envoy
	// Value: PolicyChain for that route
	Routes map[string]*core.PolicyChain

	// Request context storage (request → response phase)
	// Key: request ID
	// Value: (RequestContext, PolicyChain)
	ContextStorage map[string]*storedContext
}

// storedContext holds context and chain for response phase retrieval
type storedContext struct {
	RequestContext *policies.RequestContext
	PolicyChain    *core.PolicyChain
}

// NewKernel creates a new Kernel instance
func NewKernel() *Kernel {
	return &Kernel{
		Routes:         make(map[string]*core.PolicyChain),
		ContextStorage: make(map[string]*storedContext),
	}
}

// GetPolicyChainForKey retrieves the policy chain for a given metadata key
// T051: GetPolicyChainForKey method implementation
func (k *Kernel) GetPolicyChainForKey(key string) (*core.PolicyChain, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	chain, ok := k.Routes[key]
	if !ok {
		return nil, fmt.Errorf("no policy chain found for route key: %s", key)
	}

	return chain, nil
}

// RegisterRoute registers a policy chain for a route
func (k *Kernel) RegisterRoute(metadataKey string, chain *core.PolicyChain) {
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
