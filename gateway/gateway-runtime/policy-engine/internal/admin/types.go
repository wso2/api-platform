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

package admin

import (
	"time"

	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// ConfigDumpResponse is the top-level response structure for the config_dump endpoint
type ConfigDumpResponse struct {
	Timestamp      time.Time          `json:"timestamp"`
	PolicyRegistry PolicyRegistryDump `json:"policy_registry"`
	PolicyChains   PolicyChainsDump   `json:"policy_chains"`
	RouteMetadata  RouteMetadataDump  `json:"route_metadata"`
	LazyResources  LazyResourcesDump  `json:"lazy_resources"`
	XDSSync        XDSSyncInfo        `json:"xds_sync"`
}

// XDSSyncInfo contains policy xDS sync version details.
type XDSSyncInfo struct {
	PolicyChainVersion string `json:"policy_chain_version"`
}

// XDSSyncStatusResponse is the response payload for GET /xds_sync_status.
type XDSSyncStatusResponse struct {
	Component          string    `json:"component"`
	Timestamp          time.Time `json:"timestamp"`
	PolicyChainVersion string    `json:"policy_chain_version"`
}

// HealthResponse is the response payload for GET /health.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

// LazyResourcesDump contains information about all lazy resources
type LazyResourcesDump struct {
	TotalResources  int                           `json:"total_resources"`
	ResourcesByType map[string][]LazyResourceInfo `json:"resources_by_type"`
}

// LazyResourceInfo contains information about a single lazy resource
type LazyResourceInfo struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

// PolicyRegistryDump contains information about all registered policies
type PolicyRegistryDump struct {
	TotalPolicies int          `json:"total_policies"`
	Policies      []PolicyInfo `json:"policies"`
}

// PolicyInfo contains information about a single policy
type PolicyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PolicyChainsDump contains information about all configured policy chains
type PolicyChainsDump struct {
	TotalPolicyChains int                `json:"total_policy_chains"`
	PolicyChains      []PolicyChainEntry `json:"policy_chains"`
}

// PolicyChainEntry contains the configuration of a single policy chain.
type PolicyChainEntry struct {
	// ChainKey is the key this chain is registered under in the kernel — the key a
	// route's resolver composes and looks up, not necessarily a route key. For an
	// identity route the two are the same string; for a protocol-resolved one (A2A)
	// the chain is keyed by apiID/vhost/operation and no route bears this name.
	//
	// It replaces the former route_key field, which held this same value under a name
	// that predates composed chain keys — from when every chain was keyed by the route
	// that carried it. The name was not merely redundant: on an A2A operation chain it
	// named a route that does not exist.
	ChainKey string `json:"chain_key"`

	// APIID, Vhost and Operation are ChainKey decomposed, populated only for a
	// composed key. They exist because the key's components are joined by an
	// unprintable separator that JSON renders as an escape sequence: without these,
	// finding one API's chains, or one operation across partitions, means decoding
	// the key by eye. Split with the same shared helper that composes it, so the
	// parts cannot disagree with the whole.
	APIID     string `json:"api_id,omitempty"`
	Vhost     string `json:"vhost,omitempty"`
	Operation string `json:"operation,omitempty"`

	RequiresRequestBody  bool         `json:"requires_request_body"`
	RequiresResponseBody bool         `json:"requires_response_body"`
	TotalPolicies        int          `json:"total_policies"`
	Policies             []PolicySpec `json:"policies"`
}

// RouteMetadataDump contains route metadata for all configured routes
type RouteMetadataDump struct {
	TotalRoutes int                  `json:"total_routes"`
	Routes      []RouteMetadataEntry `json:"routes"`
}

// RouteMetadataEntry contains metadata for a single route
type RouteMetadataEntry struct {
	RouteKey                string            `json:"route_key"`
	APIId                   string            `json:"api_id"`
	APIName                 string            `json:"api_name"`
	APIVersion              string            `json:"api_version"`
	Context                 string            `json:"context"`
	OperationPath           string            `json:"operation_path"`
	Vhost                   string            `json:"vhost"`
	APIKind                 string            `json:"api_kind"`
	TemplateHandle          string            `json:"template_handle,omitempty"`
	ProviderName            string            `json:"provider_name,omitempty"`
	ProjectID               string            `json:"project_id,omitempty"`
	DefaultUpstreamCluster  string            `json:"default_upstream_cluster"`
	UpstreamBasePath        string            `json:"upstream_base_path"`
	UpstreamDefinitionPaths map[string]string `json:"upstream_definition_paths"`

	// DefaultUpstream is this route's own compiled-in upstream (whichever slot it
	// belongs to).
	DefaultUpstream *policyenginev1.UpstreamInfo `json:"default_upstream,omitempty"`

	// ─── Policy chain resolution ─────────────────────────────────────────────
	//
	// Which chain a request on this route actually gets. On an identity route
	// CanonicalChainKey equals RouteKey and the rest is empty, so the dump for every
	// kind shipping today is unchanged apart from that one echoed value.
	//
	// On a multiplexed route these are the only way to answer "why did this request
	// get that chain?" from outside the process: the resolver names what reads the
	// operation out of the request, and ChainKeyPrefix is what the engine joins that
	// operation onto.
	CanonicalChainKey string `json:"canonical_chain_key"`
	// ChainKey is the chain this route actually binds, matching a policy_chains entry
	// of the same name. Present on every statically-resolved route — an identity one,
	// where it restates the route's own key, and an A2A HTTP+JSON one, where it is the
	// composed operation key the route was generated for. Absent on a body-resolved
	// route, which picks one of its protocol's operation chains per request: there the
	// answer is ChainKeyPrefix plus whatever the resolver reads, and naming a single
	// key here would be a claim the route cannot honour.
	ChainKey string `json:"chain_key,omitempty"`
	// ResolverName is empty for an identity route.
	ResolverName string `json:"resolver_name,omitempty"`
	// ChainKeyPrefix is the composed-key prefix for this route: the apiID and vhost
	// the engine will join a resolved operation onto. Absent on identity routes, which
	// have no operation to compose. It replaces the old operation_map dump — under
	// composed keys there is no per-route mapping to show, so what an operator needs
	// instead is the prefix to match a dumped chain key against.
	ChainKeyPrefix string `json:"chain_key_prefix,omitempty"`
	// MaxRequestBodyBytes is the effective acceptance ceiling on a body-resolved route,
	// reported even when it came from the default so an operator can see the bound that
	// is actually in force rather than having to infer it. It caps unauthenticated
	// decompression and parsing work, not how much Envoy buffers.
	MaxRequestBodyBytes int64 `json:"max_request_body_bytes,omitempty"`
	// ResolverStatic reports that this route's resolution was fully determined at
	// ingest, so no resolver runs per request. True for every route of every kind
	// shipping today; false means the route inspects each request to pick its chain.
	ResolverStatic bool `json:"resolver_static,omitempty"`
	// ResolverBuffersBody reports that this route's resolver reads the request body,
	// which defers chain selection — and therefore every policy, including
	// authentication — to the request-body callback.
	ResolverBuffersBody bool `json:"resolver_buffers_body,omitempty"`
}

// PolicySpec contains specification for a policy instance
type PolicySpec struct {
	Name               string                 `json:"name"`
	Version            string                 `json:"version"`
	Enabled            bool                   `json:"enabled"`
	ExecutionCondition *string                `json:"execution_condition"`
	Parameters         map[string]interface{} `json:"parameters"`
}
