/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/chainkey"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// RuntimeDeployConfig is the kind-agnostic intermediate representation produced by
// each transformer (RestAPI, LLM Provider, LLM Proxy). Both the Envoy xDS translator
// and the policy xDS translator consume this struct.
type RuntimeDeployConfig struct {
	Metadata Metadata
	Context  string // API base path (e.g. "/weather/$version"); "" for kinds with no context
	// PolicyChainResolver is the RDC-level default resolver name, used by every route
	// that does not set Route.ResolverName. It is a compatibility default: every
	// transformer shipping today sets it once and leaves the per-route field empty, so
	// their emitted resolver_name is byte-identical to before per-route resolvers
	// existed. Once every transformer writes the route field, this can be deprecated.
	PolicyChainResolver string // name of resolver registered in PE (e.g. "route-key", "mcp-tool")
	Routes              map[string]*Route
	PolicyChains        map[string]*PolicyChain
	UpstreamClusters    map[string]*UpstreamCluster
	SensitiveValues     []string // resolved secret plaintext values for redaction; populated from StoredConfig.SensitiveValues
}

// Metadata contains identity information for the deployed API.
type Metadata struct {
	UUID          string
	Kind          string
	Handle        string
	Version       string
	DisplayName   string
	ProjectID     string // from gateway.api-platform.wso2.com/project-id (UUID for CP deploys)
	ProjectHandle string // from gateway.api-platform.wso2.com/project-handle (analytics-facing)
	LLM           *LLMMetadata // nil for non-LLM kinds
}

// AnalyticsProjectRef returns the project identity to publish for analytics
// (Moesif metadata.projectId). Prefer the user-facing handle when present.
func (m Metadata) AnalyticsProjectRef() string {
	if handle := strings.TrimSpace(m.ProjectHandle); handle != "" {
		return handle
	}
	return m.ProjectID
}

// LLMMetadata carries LLM-specific metadata for provider/proxy scenarios.
type LLMMetadata struct {
	TemplateHandle string
	ProviderName   string
}

// RouteHeaderMatch mirrors Gateway API header matching for Envoy route selection.
type RouteHeaderMatch struct {
	Name  string
	Value string
	Type  string // Exact or RegularExpression
}

// Route represents a single Envoy route derived from an API operation.
type Route struct {
	Method          string
	Path            string // full path including context prefix (set by transformer)
	OperationPath   string // original operation path without context prefix
	PathMatchType   string // Exact or PathPrefix (empty defaults to Exact semantics for legacy APIs)
	MatchHeaders    []RouteHeaderMatch
	Vhost           string // "" = default vhost
	AutoHostRewrite bool
	Timeout         *RouteTimeout
	Upstream        RouteUpstream
	// Order is the operation/rule index from the source API spec. It is used as the
	// Gateway-API "earlier-rule-wins" tie-break when two routes share the same match
	// precedence (same path, method, and header-match count). Routes are emitted in
	// ascending Order so the stable route sorter preserves rule order for ties.
	Order int

	// ─── Policy chain resolution ─────────────────────────────────────────────

	// CanonicalChainKey is the key of the policy chain this route's requests use
	// when the operation is determined by the route itself. It equals the route key
	// for every kind shipping today, but it is emitted as its own explicit field
	// rather than left implicit, because that is what lets a directly-resolved route be
	// pointed at a *composed* operation key without a wire change. Empty means "same as
	// the route key".
	//
	// A route naming a protocol resolver must leave this empty. Its key is derived from
	// its own ResolverConfig by the resolver that owns the protocol, so a key here would
	// be a second copy of the same fact with nothing to arbitrate between them — see
	// ValidateResolution, which rejects the combination.
	CanonicalChainKey string

	// ResolverName overrides RuntimeDeployConfig.PolicyChainResolver for this one
	// route. It exists because one API can hold both shapes at once: routes whose
	// operation is only knowable from the request name a protocol resolver, while the
	// routes that are not operations at all — a well-known metadata document, a CORS
	// preflight — stay directly resolved beside them. Empty inherits the RDC-level
	// default.
	ResolverName string

	// ResolverConfig is opaque, resolver-specific per-route configuration. The policy
	// engine passes it to the resolver's Prepare hook once at xDS ingest, so a
	// resolver that must compile a schema or build an index does it there rather than
	// per request. Nil when the route's resolver needs none.
	ResolverConfig json.RawMessage

	// MaxRequestBodyBytes is the largest request body, in wire bytes before any
	// decompression, that the policy engine will accept for operation resolution on
	// this route. Zero lets the engine apply its own low default.
	//
	// Only meaningful on a route whose resolver reads the body, and it is an acceptance
	// ceiling rather than a buffering one: the engine checks it after Envoy has already
	// buffered the body, so it bounds the unauthenticated decompression and parsing work
	// on that route, not the memory a caller can make the gateway hold. That is bounded
	// listener-wide by per_connection_buffer_limit_bytes.
	MaxRequestBodyBytes int64
}

// RouteTimeout holds parsed timeout values for a route.
// Timeout and IdleTimeout come from the resilience block (operation-level overriding
// API-level). A nil field means "not configured" — the global route timeout default
// applies. A non-nil zero value means "explicitly disabled".
type RouteTimeout struct {
	Connect     *time.Duration
	Timeout     *time.Duration // route timeout -> RouteAction.Timeout
	IdleTimeout *time.Duration // route idle timeout -> RouteAction.IdleTimeout
}

// RouteUpstream links a route to its upstream cluster.
type RouteUpstream struct {
	ClusterKey       string // key into UpstreamClusters map
	UseClusterHeader bool   // if true, policy selects upstream dynamically
	DefaultCluster   string // default cluster name when UseClusterHeader is true

	// Default is this route's own compiled-in upstream (cluster name, URL, base
	// path) — whichever slot this route belongs to (main or sandbox). Exposed to
	// the policy engine as the route's single default upstream field, regardless
	// of which slot it is.
	Default *policyenginev1.UpstreamInfo
}

// PolicyChain is an ordered list of policies for a route.
type PolicyChain struct {
	Policies []Policy
}

// Policy represents a single policy instance within a chain.
type Policy struct {
	Name               string
	Version            string
	Params             map[string]interface{}
	ExecutionCondition *string
}

// UpstreamCluster represents an Envoy cluster with its endpoints.
type UpstreamCluster struct {
	Name           string // upstream definition name; "" for the main/sandbox slot clusters
	BasePath       string
	Endpoints      []Endpoint
	TLS            *UpstreamTLS
	ConnectTimeout *time.Duration // ConnectTimeout is the per-upstream TCP connect timeout
}

// Endpoint is a single upstream host:port target.
type Endpoint struct {
	Host   string
	Port   int
	Weight *int
}

// UpstreamTLS holds TLS configuration for an upstream cluster.
type UpstreamTLS struct {
	Enabled bool
}

// ConfigTransformer transforms a StoredConfig into a RuntimeDeployConfig.
type ConfigTransformer interface {
	Transform(cfg *StoredConfig) (*RuntimeDeployConfig, error)
}

// RouteKeyResolverName is the resolver name meaning "the route determines the
// operation" — the identity case. It must match the policy engine's
// resolver.RouteKeyResolverName; the two are separate constants because the
// controller and the runtime are separate modules that agree on a wire value.
const RouteKeyResolverName = "route-key"

// EffectiveResolverName returns the resolver this route actually uses: its own
// override, or the RDC-level compatibility default. This is the single place the
// precedence is expressed, so the wire value and any validation of it cannot disagree.
func (rdc *RuntimeDeployConfig) EffectiveResolverName(route *Route) string {
	if route.ResolverName != "" {
		return route.ResolverName
	}
	return rdc.PolicyChainResolver
}

// EffectiveCanonicalChainKey returns the chain key for a route resolved by identity,
// falling back to the route key when the transformer left it unset.
func (rdc *RuntimeDeployConfig) EffectiveCanonicalChainKey(routeKey string, route *Route) string {
	if route.CanonicalChainKey != "" {
		return route.CanonicalChainKey
	}
	return routeKey
}

// IsDirectlyResolved reports whether a resolver name means "the route itself determines
// the chain key", so the route carries that key rather than deriving one per request. An
// empty name does, because that is what every RDC looked like before resolvers existed.
//
// Exported because the snapshot translator decides from it whether to put
// canonical_chain_key on the wire at all, and that decision has to agree with the
// validation rule below — two predicates could disagree.
func IsDirectlyResolved(name string) bool {
	return name == "" || name == RouteKeyResolverName
}

// ValidateResolution checks that a RuntimeDeployConfig's chain references actually
// resolve, and must pass before the RDC is stored or published.
//
// It exists because the RouteConfig and PolicyChain resources travel to the policy
// engine on two independent xDS streams: a route that reaches a chain key which was
// never built produces a deployment that looks accepted and then fails — or, worse,
// silently applies no policy — on every request to that operation. Catching a
// controller construction error here turns a runtime mystery into a deploy-time error
// naming the route.
//
// Under composed keys there is no operation map to validate. The failure mode moved
// from "the map points at a missing chain" to "a key the engine will compose has no
// chain", so the checks moved with it:
//
//   - a directly-resolved route's canonical key must name a chain (including one pointed
//     at a composed operation key, whose key must resolve like any other);
//   - a resolver-bearing route must not carry a canonical key, and must have at least
//     one operation chain in its own partition — otherwise no request to it can ever
//     resolve;
//   - every composed chain key must be well formed and belong to this RDC.
//
// Exhaustiveness over a *closed* operation set — "a chain exists for every operation
// the protocol defines" — needs the protocol's operation enum and lands with the first
// resolver that has one. It is not expressible here for an open set (an MCP tool name
// is deployment data), which is why the generic check is reachability, not completeness.
func (rdc *RuntimeDeployConfig) ValidateResolution() error {
	// One pass over the chains: validate every composed key and collect which
	// partitions have operation chains, so the per-route check below is a map lookup
	// rather than a scan of every chain per route.
	partitionsWithOperationChains := make(map[string]struct{})
	for chainKey, chain := range rdc.PolicyChains {
		// A present key with a nil value passes every reachability check below and then
		// panics the snapshot translator, which dereferences it to read Policies. A chain
		// with no policies is legitimate and common (an operation whose policies are all
		// inherited); a nil one is a construction mistake, and this is the layer that is
		// supposed to name it.
		if chain == nil {
			return fmt.Errorf("policy chain %q is nil", chainKey)
		}
		if !chainkey.IsComposed(chainKey) {
			continue // a route-key chain, not a composed one
		}
		apiID, vhost, _, ok := chainkey.Split(chainKey)
		if !ok {
			return fmt.Errorf("policy chain key %q is not a well-formed composed key (apiID, vhost, operation)",
				chainKey)
		}
		if apiID != rdc.Metadata.UUID {
			return fmt.Errorf("policy chain key %q is composed for API %q, not this API (%q)",
				chainKey, apiID, rdc.Metadata.UUID)
		}
		partitionsWithOperationChains[vhost] = struct{}{}
	}

	for routeKey, route := range rdc.Routes {
		if route == nil {
			return fmt.Errorf("route %q: nil route", routeKey)
		}

		resolverName := rdc.EffectiveResolverName(route)

		if IsDirectlyResolved(resolverName) {
			canonical := rdc.EffectiveCanonicalChainKey(routeKey, route)
			if _, ok := rdc.PolicyChains[canonical]; !ok {
				return fmt.Errorf("route %q: canonical chain key %q names no policy chain", routeKey, canonical)
			}
			// Existing is not enough. A directly-resolved route may be pointed at a
			// composed operation key, and a chain composed for another routing partition
			// is a perfectly valid chain that belongs to someone else:
			// a production route pointed at a sandbox operation chain would run the
			// sandbox's authentication, authorization and rate limits. Existence checks
			// catch a missing chain; only this catches the wrong one.
			if chainkey.IsComposed(canonical) {
				apiID, vhost, _, ok := chainkey.Split(canonical)
				if !ok {
					return fmt.Errorf("route %q: canonical chain key %q is not a well-formed composed key",
						routeKey, canonical)
				}
				if apiID != rdc.Metadata.UUID {
					return fmt.Errorf("route %q: canonical chain key %q belongs to API %q, not this API (%q)",
						routeKey, canonical, apiID, rdc.Metadata.UUID)
				}
				if vhost != route.Vhost {
					return fmt.Errorf(
						"route %q: canonical chain key %q belongs to routing partition (vhost) %q, but the route serves %q",
						routeKey, canonical, vhost, route.Vhost)
				}
			} else if canonical != routeKey {
				// A composed operation key is the one redirect an identity route may
				// carry. Any other key that merely happens to exist is refused, because
				// the failure it hides is silent: a route pointed at another route's
				// chain — a public route carrying "GET|/admin|h", say — passes the
				// existence check above and then runs that route's authentication and
				// rate limits instead of its own. Same class as the cross-partition case,
				// without a composed key's structure to detect it from.
				return fmt.Errorf(
					"route %q: canonical chain key %q is neither the route key nor a composed operation key",
					routeKey, canonical)
			}
			continue
		}

		// A resolver-bearing route resolves its chain per request, so a canonical key
		// on it would be read by nothing — it is a construction mistake, not dead
		// weight to tolerate.
		if route.CanonicalChainKey != "" {
			return fmt.Errorf(
				"route %q: resolver %q composes its chain key per request and must not carry a canonical chain key (%q)",
				routeKey, resolverName, route.CanonicalChainKey)
		}
		if _, ok := partitionsWithOperationChains[route.Vhost]; !ok {
			return fmt.Errorf(
				"route %q: resolver %q has no operation chains in its routing partition (vhost %q), so no request to it can resolve",
				routeKey, resolverName, route.Vhost)
		}
	}
	return nil
}

// ChainKeyFor composes the policy chain key for one operation of this RDC, for the
// given routing partition. Transformers that build operation chains must key them with
// this rather than formatting the string themselves: the policy engine composes the
// same key at request time from the same shared helper, and a chain emitted under any
// other spelling is one it will never find.
func (rdc *RuntimeDeployConfig) ChainKeyFor(vhost, operation string) string {
	return chainkey.For(rdc.Metadata.UUID, vhost, operation)
}
