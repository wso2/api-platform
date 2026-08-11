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

package resolver

// RouteKeyResolver is the identity resolver: the request carries no operation
// identifier of its own, so the route's canonical chain key is the answer.
// Used by RestApi, WebSubApi, Mcp-as-shipped-today, LlmProvider and LlmProxy,
// where each route has exactly one policy chain.
//
// It exists for registry symmetry — so "route-key" appears in the capability
// advertisement and the admin config dump like any other resolver — but it is
// never actually invoked: ResolveChainKey short-circuits identity routes before
// looking the resolver up, so the hot path for every kind shipping today costs
// one string comparison and a field read.
type RouteKeyResolver struct{}

// Name returns the wire value for identity resolution.
func (r *RouteKeyResolver) Name() string { return RouteKeyResolverName }

// Requirements reports that nothing about the request is needed.
func (r *RouteKeyResolver) Requirements() Requirements {
	return Requirements{BufferBody: false, Headers: false}
}

// Identify returns the route key as the single operation candidate. Reached only if a
// caller bypasses ResolveChainKey's identity short-circuit.
//
// That candidate names the right chain only when the route's CanonicalChainKey equals its
// RouteKey, which holds for every kind shipping today but is not a property of identity
// resolution: an operation route can be identity-resolved and still point at a *composed*
// canonical key (one A2A HTTP+JSON route per operation), and for those the route key is
// not the chain key. A caller that composes a key from this candidate gets a third string
// again — ChainKeyFor(apiID, vhost, routeKey) — which is neither. Read
// RouteResolution.CanonicalChainKey instead of routing an identity route through here.
func (r *RouteKeyResolver) Identify(view RequestView) (Resolution, error) {
	return Resolution{Operations: []Operation{{Candidates: []string{view.RouteKey}}}}, nil
}
