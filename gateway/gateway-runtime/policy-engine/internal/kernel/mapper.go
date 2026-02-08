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

package kernel

import (
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
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

// ApplyWholeRoutes replaces all existing route mappings with the provided set
func (k *Kernel) ApplyWholeRoutes(newRoutes map[string]*registry.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.Routes = newRoutes
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
