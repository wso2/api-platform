package kernel

import (
	"fmt"

	"github.com/envoy-policy-engine/sdk/core"
	"github.com/envoy-policy-engine/sdk/policies"
)

// storeContextForResponse stores request context and policy chain for response phase retrieval
// T052: Implementation of context storage for request → response phase
func (k *Kernel) storeContextForResponse(requestID string, ctx *policies.RequestContext, chain *core.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.ContextStorage[requestID] = &storedContext{
		RequestContext: ctx,
		PolicyChain:    chain,
	}
}

// getStoredContext retrieves stored context and chain for response phase
// T053: Implementation of context retrieval
func (k *Kernel) getStoredContext(requestID string) (*policies.RequestContext, *core.PolicyChain, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	stored, ok := k.ContextStorage[requestID]
	if !ok {
		return nil, nil, fmt.Errorf("no stored context found for request ID: %s", requestID)
	}

	return stored.RequestContext, stored.PolicyChain, nil
}

// removeStoredContext removes stored context after response phase completes
// T054: Implementation of context cleanup
func (k *Kernel) removeStoredContext(requestID string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	delete(k.ContextStorage, requestID)
}
