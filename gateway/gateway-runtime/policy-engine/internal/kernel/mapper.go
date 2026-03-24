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

// RouteConfig holds metadata and resolver info for a single route.
// Metadata is pre-populated at deploy time; no request-time parsing needed.
type RouteConfig struct {
	Metadata                RouteMetadata
	ResolverName            string
	UpstreamBasePath        string
	UpstreamDefinitionPaths map[string]string
	DefaultUpstreamCluster  string
}

// RouteMapping maps Envoy metadata keys to PolicyChains for route-specific processing
type RouteMapping struct {
	MetadataKey string
	Chain       *registry.PolicyChain
}

// Kernel represents the integration layer between Envoy and the policy execution engine.
// It holds two separate maps: RouteConfigs (metadata + resolver) and PolicyChains (executable chains).
type Kernel struct {
	mu sync.RWMutex

	// RouteConfigs maps routeKey → RouteConfig (metadata, resolver, upstream info).
	// Populated from the RouteConfigTypeURL xDS cache.
	RouteConfigs map[string]*RouteConfig

	// PolicyChains maps policyChainKey → PolicyChain (executable chain).
	// Populated from the PolicyChainTypeURL xDS cache.
	PolicyChains map[string]*registry.PolicyChain

	// Routes is kept for backward compatibility during migration.
	// Once migration is complete, this can be removed.
	// Deprecated: Use RouteConfigs + PolicyChains instead.
	Routes map[string]*registry.PolicyChain
}

// NewKernel creates a new Kernel instance
func NewKernel() *Kernel {
	return &Kernel{
		RouteConfigs: make(map[string]*RouteConfig),
		PolicyChains: make(map[string]*registry.PolicyChain),
		Routes:       make(map[string]*registry.PolicyChain),
	}
}

// GetRouteConfig retrieves the route config for a given route key.
func (k *Kernel) GetRouteConfig(routeKey string) *RouteConfig {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.RouteConfigs[routeKey]
}

// GetPolicyChain retrieves the policy chain for a given policy chain key.
func (k *Kernel) GetPolicyChain(policyChainKey string) *registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.PolicyChains[policyChainKey]
}

// GetPolicyChainForKey retrieves the policy chain for a given metadata key (backward compatible).
// Returns nil when no policy chain exists for the route (not an error condition).
func (k *Kernel) GetPolicyChainForKey(key string) *registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Try new PolicyChains map first
	if chain, ok := k.PolicyChains[key]; ok {
		return chain
	}
	// Fall back to legacy Routes map
	return k.Routes[key]
}

// RegisterRoute registers a policy chain for a route (backward compatible)
func (k *Kernel) RegisterRoute(metadataKey string, chain *registry.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.Routes[metadataKey] = chain
}

// UnregisterRoute removes a route mapping (backward compatible)
func (k *Kernel) UnregisterRoute(metadataKey string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.Routes, metadataKey)
}

// ApplyWholeRouteConfigs atomically replaces all route configs.
func (k *Kernel) ApplyWholeRouteConfigs(newConfigs map[string]*RouteConfig) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.RouteConfigs = newConfigs
}

// ApplyWholeRoutes replaces all existing route mappings with the provided set (backward compatible)
func (k *Kernel) ApplyWholeRoutes(newRoutes map[string]*registry.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.Routes = newRoutes
}

// DumpRoutes returns a copy of all route mappings for debugging
func (k *Kernel) DumpRoutes() map[string]*registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Merge both maps for debugging visibility
	dump := make(map[string]*registry.PolicyChain, len(k.Routes)+len(k.PolicyChains))
	for key, chain := range k.Routes {
		dump[key] = chain
	}
	for key, chain := range k.PolicyChains {
		dump[key] = chain
	}
	return dump
}

// DumpRouteConfigs returns a copy of all route configs for debugging.
func (k *Kernel) DumpRouteConfigs() map[string]*RouteConfig {
	k.mu.RLock()
	defer k.mu.RUnlock()

	dump := make(map[string]*RouteConfig, len(k.RouteConfigs))
	for key, cfg := range k.RouteConfigs {
		dump[key] = cfg
	}
	return dump
}
