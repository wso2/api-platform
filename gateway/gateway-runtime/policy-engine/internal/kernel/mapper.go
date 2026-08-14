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
	"log/slog"
	"strings"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
)

// RouteConfig holds metadata and resolver info for a single route.
// Metadata is pre-populated at deploy time; no request-time parsing needed.
type RouteConfig struct {
	Metadata RouteMetadata

	// RouteResolution carries how this route's policy chain key is derived —
	// RouteKey, CanonicalChainKey, ResolverName, ResolverConfig and the prepared
	// resolver built from them at ingest. Embedded so the fields read directly off
	// the route (rc.CanonicalChainKey, rc.Prepared) without copying the struct per
	// request.
	resolver.RouteResolution

	// MaxRequestBodyBytes is the largest request body, in wire bytes before any
	// decompression, that this route will accept for operation resolution. Zero means
	// DefaultMaxResolverRequestBodyBytes applies.
	//
	// It is an *acceptance* ceiling, not a buffering one, and the distinction matters.
	// A body-resolved route asks Envoy for BUFFERED mode, so by the time this is
	// checked Envoy has already collected the whole body and shipped it here in one
	// ext_proc message. What this bounds is therefore the work done *on* the body —
	// decompression, resolver parsing, and the copies those make — plus it returns a
	// clean 413 instead of letting an oversized body reach a resolver.
	//
	// What it does NOT bound is the memory an unauthenticated caller can make the
	// gateway hold: that is Envoy's listener-wide
	// router.http_listener.per_connection_buffer_limit_bytes (1 MiB by default) and the
	// ext_proc gRPC server's receive limit, neither of which is per-route. Lowering
	// this value does not lower that. Making it a real buffering bound needs either an
	// Envoy-side per-route cap (the buffer filter's max_request_bytes, which returns 413
	// before ext_proc collects the body) or streamed accumulation in the engine; neither
	// is built.
	MaxRequestBodyBytes int64
}

// PrepareRoute prepares rc's resolver from the fields that arrived over the wire, and
// stores the result on rc.
//
// It is the one place a ResolverRouteConfig is built, so a Prepare implementation and
// the binder's own validation always see the same values. Two of those values are
// derived here rather than read from the wire:
//
//   - the effective chain key, applying the older-controller fallback to the route key
//     exactly once — nothing downstream re-applies it, so nothing can disagree with it;
//   - the HTTP method, read out of the Envoy route name (METHOD|fullPath|vhost), which
//     is its only source: a route carries its path in metadata but not its method.
//
// An error means the route is unusable and its caller must drop it. Callers distinguish
// an unknown resolver from a resolver's own failure via resolver.FailureUnknownResolver.
func PrepareRoute(reg resolver.ResolverRegistry, routeKey string, rc *RouteConfig) error {
	rc.RouteKey = routeKey
	if rc.CanonicalChainKey == "" {
		rc.CanonicalChainKey = routeKey
	}

	prepared, err := resolver.PrepareRoute(reg, resolver.ResolverRouteConfig{
		RouteKey:          routeKey,
		CanonicalChainKey: rc.CanonicalChainKey,
		ResolverName:      rc.ResolverName,
		APIID:             rc.Metadata.APIId,
		Vhost:             rc.Metadata.Vhost,
		APIContext:        rc.Metadata.Context,
		// Normalised once, here, so no Prepare implementation can miss on case
		// (GO-AUTH-006).
		Method:         strings.ToUpper(methodFromRouteKey(routeKey)),
		Path:           rc.Metadata.OperationPath,
		ResolverConfig: rc.ResolverConfig,
	})
	if err != nil {
		return err
	}
	rc.Prepared = prepared
	return nil
}

// methodFromRouteKey reads the HTTP method out of an Envoy route name.
//
// A key with no separator yields no method rather than the whole key, so a
// differently-shaped route name degrades to "unknown" instead of handing a resolver a
// method that is really a path.
func methodFromRouteKey(routeKey string) string {
	method, _, found := strings.Cut(routeKey, "|")
	if !found {
		return ""
	}
	return method
}

// DefaultMaxResolverRequestBodyBytes is the acceptance ceiling applied to a
// body-resolved route whose RouteConfig carries no explicit limit. Deliberately far
// below Envoy's per-connection buffer limit: on these routes the body is resolved, and
// therefore parsed, before any authentication policy has run.
const DefaultMaxResolverRequestBodyBytes int64 = 64 * 1024

// EffectiveMaxRequestBodyBytes returns the acceptance ceiling actually in force for this
// route, resolving the default. Exported so the admin config dump can report the bound
// that applies rather than the raw (possibly zero) configured value.
func (rc *RouteConfig) EffectiveMaxRequestBodyBytes() int64 {
	if rc == nil || rc.MaxRequestBodyBytes <= 0 {
		return DefaultMaxResolverRequestBodyBytes
	}
	return rc.MaxRequestBodyBytes
}

// RouteMapping maps Envoy metadata keys to PolicyChains for route-specific processing
type RouteMapping struct {
	MetadataKey string
	Chain       *registry.PolicyChain
}

// Kernel represents the integration layer between Envoy and the policy execution engine.
// It holds two separate maps: RouteConfigs (metadata + resolver) and PolicyChains (executable chains).
type Kernel struct {
	// mu protects RouteConfigs, PolicyChains, and sensitiveValues together so that
	// an xDS update and a config dump always observe a consistent snapshot.
	mu sync.RWMutex

	// RouteConfigs maps routeKey → RouteConfig (metadata, resolver, upstream info).
	// Populated from the RouteConfigTypeURL xDS cache.
	RouteConfigs map[string]*RouteConfig

	// PolicyChains maps policyChainKey → PolicyChain (executable chain).
	// Populated from the PolicyChainTypeURL xDS cache.
	PolicyChains map[string]*registry.PolicyChain

	// sensitiveValues holds resolved secret plaintext values received via TransportMetadata.
	// Used for value-based redaction in config dumps. Protected by mu (same lock as PolicyChains
	// so that routes and sensitive values are always updated and read as one atomic snapshot).
	sensitiveValues []string
}

// NewKernel creates a new Kernel instance
func NewKernel() *Kernel {
	return &Kernel{
		RouteConfigs: make(map[string]*RouteConfig),
		PolicyChains: make(map[string]*registry.PolicyChain),
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

// GetPolicyChainForKey retrieves the policy chain for a given metadata key.
// Returns nil when no policy chain exists for the route (not an error condition).
func (k *Kernel) GetPolicyChainForKey(key string) *registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.PolicyChains[key]
}

// RegisterRoute registers a policy chain for a route.
func (k *Kernel) RegisterRoute(metadataKey string, chain *registry.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.PolicyChains[metadataKey] = chain
}

// UnregisterRoute removes a route mapping.
func (k *Kernel) UnregisterRoute(metadataKey string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.PolicyChains, metadataKey)
}

// ApplyWholeRouteConfigs atomically replaces all route configs.
func (k *Kernel) ApplyWholeRouteConfigs(newConfigs map[string]*RouteConfig) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.RouteConfigs = newConfigs
}

// ApplyWholeRoutes atomically replaces all policy chain mappings.
func (k *Kernel) ApplyWholeRoutes(newRoutes map[string]*registry.PolicyChain) {
	k.mu.Lock()
	defer k.mu.Unlock()
	keys := make([]string, 0, len(newRoutes))
	for key := range newRoutes {
		keys = append(keys, key)
	}
	slog.Debug("ApplyWholeRoutes: replacing policy chains",
		"count", len(newRoutes),
		"routes", keys)
	k.PolicyChains = newRoutes
}

// DumpRouteKeys returns the keys of all registered policy chains for debugging.
// Cheaper than DumpRoutes as it only copies keys, not chain structs.
func (k *Kernel) DumpRouteKeys() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	keys := make([]string, 0, len(k.PolicyChains))
	for key := range k.PolicyChains {
		keys = append(keys, key)
	}
	return keys
}

// DumpRoutes returns a copy of all policy chain mappings for debugging.
func (k *Kernel) DumpRoutes() map[string]*registry.PolicyChain {
	k.mu.RLock()
	defer k.mu.RUnlock()

	dump := make(map[string]*registry.PolicyChain, len(k.PolicyChains))
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

// deduplicateValues returns a deduplicated copy of values preserving order.
func deduplicateValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			unique = append(unique, v)
		}
	}
	return unique
}

// SetSensitiveValues atomically replaces the stored sensitive values under mu.
// Prefer ApplyWholeRoutesAndSensitiveValues when updating routes and values together.
func (k *Kernel) SetSensitiveValues(values []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.sensitiveValues = deduplicateValues(values)
}

// GetSensitiveValues returns a copy of the current sensitive values for use in config dump redaction.
func (k *Kernel) GetSensitiveValues() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]string, len(k.sensitiveValues))
	copy(result, k.sensitiveValues)
	return result
}

// ApplyWholeRoutesAndSensitiveValues atomically replaces all policy chain mappings and sensitive
// values in a single lock acquisition. Use this instead of calling ApplyWholeRoutes and
// SetSensitiveValues separately to prevent a config dump from observing new routes with stale
// sensitive values, which would bypass secret redaction.
func (k *Kernel) ApplyWholeRoutesAndSensitiveValues(newRoutes map[string]*registry.PolicyChain, values []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	keys := make([]string, 0, len(newRoutes))
	for key := range newRoutes {
		keys = append(keys, key)
	}
	slog.Debug("ApplyWholeRoutesAndSensitiveValues: replacing policy chains and sensitive values",
		"count", len(newRoutes),
		"routes", keys,
		"sensitive_value_count", len(values))
	k.PolicyChains = newRoutes
	k.sensitiveValues = deduplicateValues(values)
}

// DumpRoutesAndSensitiveValues returns a consistent snapshot of all policy chain mappings and
// sensitive values in a single lock acquisition. Use this in config dumps to guarantee that the
// sensitive values used for redaction correspond to the same xDS generation as the routes.
func (k *Kernel) DumpRoutesAndSensitiveValues() (map[string]*registry.PolicyChain, []string) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	routes := make(map[string]*registry.PolicyChain, len(k.PolicyChains))
	for key, chain := range k.PolicyChains {
		routes[key] = chain
	}
	sensitive := make([]string, len(k.sensitiveValues))
	copy(sensitive, k.sensitiveValues)
	return routes, sensitive
}
