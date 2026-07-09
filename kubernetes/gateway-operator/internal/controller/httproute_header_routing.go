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
	"encoding/json"

	"go.uber.org/zap"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

// This file implements header-based routing via the header-based-routing policy.
// It is deliberately self-contained.
//
// Header matching is realized entirely as a policy: header-differentiated rules that
// share a path+method are collapsed into a single Operation carrying a
// header-based-routing policy that selects the upstream by header at request time.

const (
	headerBasedRoutingPolicyName    = "header-based-routing"
	headerBasedRoutingPolicyVersion = "v1"
	dynamicEndpointPolicyName       = "dynamic-endpoint"

	// headerRoutingNoMatchStatus is returned when no header rule matches and the group
	// has no header-less default. Mirrors Gateway-API "no matching route" semantics.
	headerRoutingNoMatchStatus = 404
)

// headerMatch is the operator-internal representation of one ANDed header matcher. It is
// never serialized into APIConfigData — header matching is expressed only as the
// header-based-routing policy, not as an operation field.
type headerMatch struct {
	Name  string
	Value string
	Type  string
}

// stagedOperation pairs a compiled operation with the header matchers of the source
// HTTPRoute match it came from. The matchers live here (not on apiv1.Operation) so they
// can drive policy construction without appearing in the emitted API spec.
type stagedOperation struct {
	op      apiv1.Operation
	headers []headerMatch
}

// collapseHeaderMatchesToPolicy turns the staged operations into the final operation list.
// Operations that share a (method, path, pathMatchType) key and use header matching are
// collapsed into a single operation carrying a header-based-routing policy that selects
// the upstream by header at request time; all other operations pass through unchanged.
//
// It operates on the already-built operations, so all upstream-definition creation,
// weighting, and backend-failure handling done by the normal compile path is reused
// unchanged — here we only reshape routing.
//
// A group that cannot be faithfully expressed as one policy operation is left as its
// original operations so the route still deploys. Those cases are:
//   - a header-matched op that also redirects or direct-responds (a policy selects an
//     upstream; it cannot emit a redirect or a per-match 500),
//   - a header-matched op with no resolvable upstream target,
//   - a group whose ops carry differing non-routing policies (they cannot be merged
//     onto a single operation).
//
// mainDest is the upstream-definition name the API's main upstream resolves to. A
// header-matched operation that carries no dynamic-endpoint policy routes via that main
// upstream (an artifact of how the compile path assigns the first resolved backend as the
// main upstream), so mainDest supplies its destination when collapsing.
func collapseHeaderMatchesToPolicy(staged []stagedOperation, mainDest string, log *zap.Logger) []apiv1.Operation {
	if log == nil {
		log = zap.NewNop()
	}
	var order []string
	groups := make(map[string][]stagedOperation)
	for _, so := range staged {
		k := string(so.op.Method) + "\x00" + so.op.Path + "\x00" + string(so.op.PathMatchType)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], so)
	}

	out := make([]apiv1.Operation, 0, len(staged))
	for _, k := range order {
		g := groups[k]
		if collapsed, ok := collapseHeaderGroup(g, mainDest, log); ok {
			out = append(out, collapsed)
		} else {
			for _, so := range g {
				out = append(out, so.op) // keep original ops for this group
			}
		}
	}
	return out
}

// collapseHeaderGroup attempts to turn one (method, path, pathMatchType) group of staged
// operations into a single header-based-routing operation. It returns ok=false when the
// group does not use header matching or cannot be represented as a policy, in which case
// the caller keeps the group's operations unchanged.
func collapseHeaderGroup(g []stagedOperation, mainDest string, log *zap.Logger) (apiv1.Operation, bool) {
	usesHeaders := false
	for _, so := range g {
		if len(so.headers) > 0 {
			usesHeaders = true
			break
		}
	}
	if !usesHeaders {
		return apiv1.Operation{}, false
	}

	warn := func(msg string) {
		log.Warn("header-based routing: "+msg+"; keeping native operations for this route group",
			zap.String("method", string(g[0].op.Method)),
			zap.String("path", g[0].op.Path))
	}

	// A header-matched op that terminates (redirect policy / direct response) cannot be a
	// policy upstream selection.
	for _, so := range g {
		if len(so.headers) > 0 && (operationHasRedirectPolicy(so.op) || so.op.DirectResponse != nil) {
			warn("header match combined with redirect/directResponse cannot be expressed as a policy")
			return apiv1.Operation{}, false
		}
	}

	// All ops in the group must share the same non-routing policies (filter policies),
	// since they collapse onto a single operation.
	base := nonRoutingPolicies(g[0].op)
	for _, so := range g[1:] {
		if !policiesEqual(base, nonRoutingPolicies(so.op)) {
			warn("operations in the group carry differing non-routing policies")
			return apiv1.Operation{}, false
		}
	}

	var policyRules []interface{}
	hasDefault := false
	var defaultDynEndpoint *apiv1.Policy
	for i := range g {
		so := g[i]
		if len(so.headers) == 0 {
			// A header-less op in the group is the catch-all default: non-matching
			// requests fall through to whatever upstream it routes to.
			hasDefault = true
			if p, ok := findDynamicEndpoint(so.op); ok {
				dp := p
				defaultDynEndpoint = &dp
			}
			continue
		}
		dest, ok := dynamicEndpointTarget(so.op)
		if !ok {
			// No dynamic-endpoint policy => routes via the API main upstream.
			dest, ok = mainDest, mainDest != ""
		}
		if !ok {
			warn("header match has no resolvable upstream target")
			return apiv1.Operation{}, false
		}
		policyRules = append(policyRules, map[string]interface{}{
			"destination": dest,
			"matches": []interface{}{
				map[string]interface{}{"matchHeaders": headerMatchesToPolicyParams(so.headers)},
			},
		})
	}
	if len(policyRules) == 0 {
		return apiv1.Operation{}, false
	}

	params := map[string]interface{}{"rules": policyRules}
	// With no header-less default there is no route to fall through to, so a
	// non-match is "no matching route" -> 404. With a default present, leave
	// noMatchStatusCode unset so the policy passes non-matches through to it.
	if !hasDefault {
		params["noMatchStatusCode"] = headerRoutingNoMatchStatus
	}
	hbrPolicy, err := policyFromParams(headerBasedRoutingPolicyName, headerBasedRoutingPolicyVersion, params)
	if err != nil {
		warn("failed to build header-based-routing policy params")
		return apiv1.Operation{}, false
	}

	collapsed := apiv1.Operation{
		Method:        g[0].op.Method,
		Path:          g[0].op.Path,
		PathMatchType: g[0].op.PathMatchType,
	}
	collapsed.Policies = append(collapsed.Policies, copyPolicies(base)...)
	// The default's dynamic-endpoint runs first so its UpstreamName is in place; the
	// header-based-routing policy runs last so a header match overrides it, and a
	// non-match (passthrough) leaves the default selection intact.
	if defaultDynEndpoint != nil {
		collapsed.Policies = append(collapsed.Policies, *defaultDynEndpoint)
	}
	collapsed.Policies = append(collapsed.Policies, hbrPolicy)
	return collapsed, true
}

// headerMatchesToPolicyParams converts an operation's ANDed header matchers into the
// header-based-routing policy's matchHeaders parameter shape.
func headerMatchesToPolicyParams(headers []headerMatch) []interface{} {
	out := make([]interface{}, 0, len(headers))
	for _, h := range headers {
		m := map[string]interface{}{
			"name":  h.Name,
			"value": h.Value,
		}
		if h.Type != "" {
			m["type"] = h.Type
		}
		out = append(out, m)
	}
	return out
}

// nonRoutingPolicies returns the operation's policies excluding the dynamic-endpoint
// routing policy (which the header-based-routing policy replaces).
func nonRoutingPolicies(op apiv1.Operation) []apiv1.Policy {
	var out []apiv1.Policy
	for _, p := range op.Policies {
		if p.Name == dynamicEndpointPolicyName {
			continue
		}
		out = append(out, p)
	}
	return out
}

// mainUpstreamDefName returns the upstream-definition name the API's main upstream
// resolves to: the ref directly when set (weighted rules), otherwise the single-target
// definition whose URL matches the main upstream URL. Returns "" when the main upstream
// does not correspond to a definition (e.g. the unresolved-backend placeholder).
func mainUpstreamDefName(defs map[string]apiv1.UpstreamDefinition, mainRef *string, mainURL string) string {
	if mainRef != nil {
		return *mainRef
	}
	for name, d := range defs {
		if len(d.Upstreams) == 1 && d.Upstreams[0].Url == mainURL {
			return name
		}
	}
	return ""
}

func findDynamicEndpoint(op apiv1.Operation) (apiv1.Policy, bool) {
	for _, p := range op.Policies {
		if p.Name == dynamicEndpointPolicyName {
			return p, true
		}
	}
	return apiv1.Policy{}, false
}

// dynamicEndpointTarget reads the targetUpstream (upstream definition name) from the
// operation's dynamic-endpoint policy.
func dynamicEndpointTarget(op apiv1.Operation) (string, bool) {
	p, ok := findDynamicEndpoint(op)
	if !ok || p.Params == nil {
		return "", false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(p.Params.Raw, &m); err != nil {
		return "", false
	}
	v, ok := m["targetUpstream"].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// policiesEqual compares two policy slices by their marshaled form. Policy params are
// produced deterministically (policyFromParams marshals a map once), so byte equality is
// a reliable identity check here.
func policiesEqual(a, b []apiv1.Policy) bool {
	if len(a) != len(b) {
		return false
	}
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}
