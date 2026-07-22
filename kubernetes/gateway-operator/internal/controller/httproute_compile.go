/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// BuildAPIConfigFromHTTPRoute compiles an HTTPRoute into APIConfigData for gateway-controller deployment.
func BuildAPIConfigFromHTTPRoute(
	ctx context.Context,
	c client.Client,
	parentGW *gatewayv1.Gateway,
	route *gatewayv1.HTTPRoute,
	parentTargets []gatewayParentTarget,
	backendResolution *HTTPRouteBackendResolution,
	clusterDomain string,
	log *slog.Logger,
) (*apiv1.APIConfigData, error) {
	if len(route.Spec.Rules) == 0 {
		return nil, newInvalidHTTPRouteConfigError("HTTPRoute has no rules")
	}
	if backendResolution == nil {
		var err error
		backendResolution, err = resolveHTTPRouteBackendRefs(ctx, c, route, clusterDomain)
		if err != nil {
			return nil, err
		}
	}

	displayName := route.Name
	if v := route.Annotations[AnnHTTPRouteDisplayName]; v != "" {
		displayName = v
	}
	version := route.Annotations[AnnHTTPRouteAPIVersion]
	if version == "" {
		version = "v1.0"
	}

	contextPath := normalizeContext(route.Annotations[AnnHTTPRouteContext])

	vhostMain, vhostList, err := deriveVhosts(ctx, c, parentGW, route, parentTargets)
	if err != nil {
		return nil, err
	}

	defsByName := make(map[string]apiv1.UpstreamDefinition)
	var ops []apiv1.Operation
	mainUpstreamURL := backendResolution.PlaceholderURL
	if mainUpstreamURL == "" {
		mainUpstreamURL = "http://127.0.0.1:1"
	}
	var mainUpstreamRef *string

	for ruleIdx, rule := range route.Spec.Rules {
		rulePolicies, err := policiesFromHTTPRouteRuleExtensionRefs(ctx, c, route, rule, ruleIdx, log)
		if err != nil {
			return nil, err
		}
		filterPolicies, hasRedirect, err := policiesFromHTTPRouteFilters(rule.Filters)
		if err != nil {
			return nil, err
		}

		ruleBackendRefs := backendsForRule(backendResolution, ruleIdx)
		ruleFailed := ruleHasUnresolvedBackends(ruleBackendRefs)
		weightedRule := len(ruleBackendRefs) > 1 && allRuleBackendsResolved(ruleBackendRefs)

		var ruleDefName string
		if weightedRule {
			ruleDefName = fmt.Sprintf("rule-%d-weighted", ruleIdx)
			weighted := make([]apiv1.WeightedUpstream, 0, len(ruleBackendRefs))
			for _, br := range ruleBackendRefs {
				w := 1
				if br.Weight != nil {
					w = int(*br.Weight)
				}
				// Gateway-API: a backend with weight 0 must receive NO traffic. Envoy's
				// load_balancing_weight has a minimum of 1 (an unset weight defaults to 1),
				// so a weight-0 endpoint left in the cluster would still get traffic. Drop it
				// from the weighted definition entirely so it is never routed to. (If every
				// backend has weight 0 the definition ends up empty, the controller creates no
				// cluster for it, and requests fail — consistent with the Gateway-API guidance
				// that an all-zero-weight rule returns a 500-level response.)
				if w == 0 {
					continue
				}
				weighted = append(weighted, apiv1.WeightedUpstream{
					Url:    br.URL,
					Weight: &w,
				})
			}
			defsByName[ruleDefName] = apiv1.UpstreamDefinition{
				Name:      ruleDefName,
				Upstreams: weighted,
			}
			if mainUpstreamRef == nil {
				ref := ruleDefName
				mainUpstreamRef = &ref
			}
		}

		if len(rule.Matches) == 0 {
			return nil, newInvalidHTTPRouteConfigError(
				"rule[%d] has no matches; add at least one rule.matches entry",
				ruleIdx,
			)
		}

		// respondStatus != 0 marks a rule that terminates at the gateway with an immediate
		// response, realized as the respond policy. Per Gateway-API a rule with no backends
		// or with unresolvable backends returns a 500.
		respondStatus := 0
		switch {
		case hasRedirect:
			// A RequestRedirect rule terminates at the gateway (its redirect policy is
			// already in filterPolicies) and legitimately has no backendRefs, so this must
			// take precedence over the "no backends → 500" fallback below.
		case !ruleHasBackendRefs(rule), ruleFailed:
			respondStatus = 500
		}

		if !weightedRule {
			for _, br := range ruleBackendRefs {
				if br.OK {
					defName := upstreamDefinitionNameForService(br.ServiceNS, br.ServiceName, br.Port)
					if _, ok := defsByName[defName]; !ok {
						defsByName[defName] = apiv1.UpstreamDefinition{
							Name: defName,
							Upstreams: []apiv1.WeightedUpstream{{
								Url: br.URL,
							}},
						}
					}
				}
			}
		}

		for _, m := range rule.Matches {
			pathVal := operationPathFromMatch(m)
			pathType := pathMatchTypeFromHTTPRoute(m.Path)
			headerMatches := mapHTTPHeaderMatches(m.Headers)
			methods := restAPIOperationMethodsForHTTPRouteMatch(m)

			for _, method := range methods {
				op := apiv1.Operation{
					Match: &apiv1.OperationMatch{
						Method:  method,
						Path:    apiv1.OperationPathMatch{Value: pathVal, Type: pathType},
						Headers: headerMatches,
					},
					Policies: copyPolicies(rulePolicies),
				}
				op.Policies = append(op.Policies, filterPolicies...)

				// A terminating rule's policy (respond or redirect) is already attached and
				// short-circuits the request, so it needs no backend routing.
				if respondStatus != 0 {
					// Realize the gateway-terminated immediate response as the respond
					// policy; it short-circuits the request so no backend routing is attached.
					p, err := respondPolicyFromStatus(respondStatus)
					if err != nil {
						return nil, err
					}
					op.Policies = append(op.Policies, p)
				} else if hasRedirect {
					// redirect policy already attached; nothing to route.
				} else if weightedRule {
					// A weighted rule is realized as ONE upstream definition ("rule-N-weighted")
					// whose endpoints carry the per-backend weights; the controller creates a
					// single weighted cluster for it. Point the route's dynamic-endpoint policy at
					// that definition so cluster_header routing selects the weighted cluster and
					// Envoy distributes by endpoint weight. (Targeting the first backend's
					// per-service name — which is never created as a definition for weighted
					// rules — leaves x-target-upstream pointing at a non-existent cluster, i.e.
					// no_cluster -> 503 for every request.)
					p, err := dynamicEndpointPolicy(ruleDefName)
					if err != nil {
						return nil, err
					}
					op.Policies = append(op.Policies, p)
				} else if len(ruleBackendRefs) == 1 && ruleBackendRefs[0].OK {
					defName := upstreamDefinitionNameForService(
						ruleBackendRefs[0].ServiceNS,
						ruleBackendRefs[0].ServiceName,
						ruleBackendRefs[0].Port,
					)
					if needsDynamicEndpoint(defsByName) {
						p, err := dynamicEndpointPolicy(defName)
						if err != nil {
							return nil, err
						}
						op.Policies = append(op.Policies, p)
					}
				}

				ops = append(ops, op)
			}
		}
	}

	if len(ops) == 0 {
		return nil, newInvalidHTTPRouteConfigError("no operations derived from HTTPRoute")
	}

	apiPolicies, err := loadHTTPRouteAPIPolicies(ctx, c, route, log)
	if err != nil {
		return nil, err
	}

	// Emit upstream definitions in a deterministic (name-sorted) order. defsByName is a
	// map, so ranging it directly yields a random order each reconcile; that makes the
	// generated APIConfigData/RestApi spec differ every pass, causing the operator to
	// re-update the CR and the gateway-controller to re-push xDS in a hot loop.
	upstreamDefs := make([]apiv1.UpstreamDefinition, 0, len(defsByName))
	for _, d := range defsByName {
		upstreamDefs = append(upstreamDefs, d)
	}
	sort.Slice(upstreamDefs, func(i, j int) bool {
		return upstreamDefs[i].Name < upstreamDefs[j].Name
	})

	mainUpstream := apiv1.Upstream{Url: &mainUpstreamURL}
	if mainUpstreamRef != nil {
		mainUpstream = apiv1.Upstream{Ref: mainUpstreamRef}
	} else if needsDynamicEndpoint(defsByName) && len(defsByName) == 1 {
		for name := range defsByName {
			ref := name
			mainUpstream = apiv1.Upstream{Ref: &ref}
			break
		}
	}
	// Gateway-API requires the original Host header be forwarded to the backend (a transparent
	// proxy), so disable Envoy's automatic host rewrite. Without this the gateway-controller
	// defaults to "auto" and overwrites Host with the upstream cluster's DNS name.
	manualHostRewrite := apiv1.UpstreamHostRewriteManual
	mainUpstream.HostRewrite = &manualHostRewrite

	spec := &apiv1.APIConfigData{
		Context:             contextPath,
		DisplayName:         displayName,
		Operations:          ops,
		Upstream:            apiv1.UpstreamConfig{Main: mainUpstream},
		Version:             version,
		Policies:            apiPolicies,
		UpstreamDefinitions: upstreamDefs,
	}
	if vhostMain != "" {
		// vhosts.main carries the production hostnames as a ";"-separated list. When the route
		// attaches to multiple listener hostnames, the primary (first) host and the additional
		// hosts are joined here; the gateway-controller splits them back into per-host vhosts.
		main := vhostMain
		if len(vhostList) > 0 {
			main = strings.Join(append([]string{vhostMain}, vhostList...), ";")
		}
		spec.Vhosts = &apiv1.VhostConfig{Main: main}
	}

	if _, err := resolveAPIConfigPolicyParamsValueFrom(ctx, c, route.Namespace, spec, log); err != nil {
		return nil, err
	}
	return spec, nil
}

func backendsForRule(resolution *HTTPRouteBackendResolution, ruleIdx int) []ResolvedBackendRef {
	if resolution == nil {
		return nil
	}
	var out []ResolvedBackendRef
	for _, ref := range resolution.Refs {
		if ref.RuleIndex == ruleIdx {
			out = append(out, ref)
		}
	}
	return out
}

func ruleHasUnresolvedBackends(refs []ResolvedBackendRef) bool {
	if len(refs) == 0 {
		return false
	}
	for _, r := range refs {
		if !r.OK {
			return true
		}
	}
	return false
}

func allRuleBackendsResolved(refs []ResolvedBackendRef) bool {
	if len(refs) == 0 {
		return false
	}
	for _, r := range refs {
		if !r.OK {
			return false
		}
	}
	return true
}

func needsDynamicEndpoint(defs map[string]apiv1.UpstreamDefinition) bool {
	return len(defs) > 1
}

// normalizeContext maps a raw context annotation to the APIConfigData.Context contract: a single
// leading "/", no trailing slash, with "/" itself denoting the root context. It strips leading and
// trailing slashes then re-adds one leading slash, so "" and "/" -> "/", "/api/" -> "/api",
// "api" -> "/api". This keeps the compiled payload within the schema the controller validates.
func normalizeContext(raw string) string {
	return "/" + strings.Trim(strings.TrimSpace(raw), "/")
}

func deriveVhosts(ctx context.Context, c client.Client, parentGW *gatewayv1.Gateway, route *gatewayv1.HTTPRoute, parentTargets []gatewayParentTarget) (main string, additional []string, err error) {
	seen := make(map[string]struct{})
	addHost := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		if main == "" {
			main = h
		} else {
			additional = append(additional, h)
		}
	}

	// Gateway-API vhosts are the intersection of the route's hostnames with the hostnames of the
	// listeners the route attaches to — not the raw union of both. For each attached listener we
	// intersect every route hostname with the listener hostname, narrowing a wildcard to the more
	// specific name and dropping hostnames that don't overlap. A route with no hostnames inherits the
	// listener hostname; a listener with no hostname accepts the route hostnames as-is. When neither
	// specifies a hostname, no vhost is emitted (the controller falls back to its default "match all"
	// vhost), preserving behavior for routes that don't use hostname-based routing.
	if parentGW != nil {
		for _, target := range parentTargets {
			for _, listener := range parentGW.Spec.Listeners {
				// Only emit vhosts for listeners that actually accept this route — the same
				// per-listener predicate evaluateHTTPRouteAttachment applies (kind + namespace +
				// hostname), not just the structural parentRef match. A gateway-wide parentRef
				// matches every listener structurally, so the bare structural check would emit
				// vhosts for listeners the route never attached to.
				accepted, aErr := httpRouteAcceptedByListener(ctx, c, parentGW, route, target.ref, listener)
				if aErr != nil {
					return "", nil, aErr
				}
				if !accepted {
					continue
				}
				listenerHost := ""
				if listener.Hostname != nil {
					listenerHost = string(*listener.Hostname)
				}
				if len(route.Spec.Hostnames) == 0 {
					// Route matches all hostnames; constrain to the listener's hostname if it has one.
					if listenerHost != "" {
						addHost(listenerHost)
					}
					continue
				}
				for _, rh := range route.Spec.Hostnames {
					if h, ok := intersectHostname(string(rh), listenerHost); ok {
						addHost(h)
					}
				}
			}
		}
	}

	if main != "" && len(additional) > 0 {
		return main, additional, nil
	}
	return main, nil, nil
}

// intersectHostname returns the effective hostname produced by intersecting an HTTPRoute hostname
// with a listener hostname per the Gateway-API spec, and whether they overlap at all. An empty
// listenerHost means the listener has no hostname (matches everything), so the route hostname wins.
// When one side is a wildcard (e.g. "*.example.com") and the other a name it matches, the more
// specific name is returned. Non-overlapping hostnames return ("", false) and are dropped.
func intersectHostname(routeHost, listenerHost string) (string, bool) {
	if listenerHost == "" {
		return routeHost, true
	}
	if routeHost == "" || routeHost == listenerHost {
		return listenerHost, true
	}
	rWild := strings.HasPrefix(routeHost, "*.")
	lWild := strings.HasPrefix(listenerHost, "*.")
	switch {
	case lWild && !rWild:
		if hostnameMatchesWildcard(routeHost, listenerHost) {
			return routeHost, true
		}
	case rWild && !lWild:
		if hostnameMatchesWildcard(listenerHost, routeHost) {
			return listenerHost, true
		}
	case rWild && lWild:
		// Both wildcards: overlap only if one suffix is contained in the other; keep the more specific.
		if strings.HasSuffix(routeHost, listenerHost[1:]) {
			return routeHost, true
		}
		if strings.HasSuffix(listenerHost, routeHost[1:]) {
			return listenerHost, true
		}
	}
	return "", false
}

// hostnameMatchesWildcard reports whether host matches a wildcard hostname like "*.example.com".
// Per Gateway-API the wildcard requires at least one label before the suffix, so "foo.example.com"
// and "foo.bar.example.com" match but "example.com" does not.
func hostnameMatchesWildcard(host, wildcard string) bool {
	suffix := wildcard[1:] // strip the leading "*", leaving ".example.com"
	return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
}
