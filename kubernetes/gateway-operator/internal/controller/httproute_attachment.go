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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// evaluateHTTPRouteAttachment checks whether an HTTPRoute attaches to at least one listener on
// the parent Gateway per Gateway API attachment semantics (parentRef, allowedRoutes, hostnames).
func evaluateHTTPRouteAttachment(
	ctx context.Context,
	cl client.Client,
	gw *gatewayv1.Gateway,
	route *gatewayv1.HTTPRoute,
	parentRef gatewayv1.ParentReference,
) (attached bool, reason gatewayv1.RouteConditionReason, message string) {
	if gw == nil || route == nil {
		return false, gatewayv1.RouteReasonNotAllowedByListeners, "invalid attachment inputs"
	}

	var (
		matchedListener bool
		hostnameChecked bool
		hostnameMatch   bool
	)

	for _, listener := range gw.Spec.Listeners {
		if !parentRefAttachesToListener(parentRef, listener) {
			continue
		}
		matchedListener = true

		// Pre-hostname gates (kind + namespace) shared with httpRouteAcceptedByListener; tracked
		// here only to compute the precise not-attached reason below.
		if !listenerAllowsHTTPRouteKind(listener) {
			continue
		}
		allowedNs, err := namespacesAllowedByListener(ctx, cl, listener, gw)
		if err != nil {
			return false, gatewayv1.RouteReasonNotAllowedByListeners, fmt.Sprintf("failed to resolve allowed namespaces: %v", err)
		}
		if !namespaceAllowed(route.Namespace, allowedNs) {
			continue
		}

		hostnameChecked = true
		accepted, err := httpRouteAcceptedByListener(ctx, cl, gw, route, parentRef, listener)
		if err != nil {
			return false, gatewayv1.RouteReasonNotAllowedByListeners, fmt.Sprintf("failed to resolve allowed namespaces: %v", err)
		}
		if accepted {
			hostnameMatch = true
			return true, gatewayv1.RouteReasonAccepted, "Route attaches to listener"
		}
	}

	if !matchedListener {
		if normalizeParentRefSectionName(parentRef) != "" || normalizeParentRefPort(parentRef) != 0 {
			return false, gatewayv1.RouteReasonNoMatchingParent, "No matching parent"
		}
		return false, gatewayv1.RouteReasonNotAllowedByListeners, "No matching listener on parent Gateway"
	}
	if hostnameChecked && !hostnameMatch {
		return false, gatewayv1.RouteReasonNoMatchingListenerHostname, "No matching listener hostname"
	}
	return false, gatewayv1.RouteReasonNotAllowedByListeners, "Route is not allowed by any matching listener"
}

// httpRouteAcceptedByListener reports whether a single listener actually accepts the route under
// Gateway API attachment semantics: the parentRef must target the listener (sectionName/port),
// and the listener must allow the HTTPRoute kind, allow the route's namespace, and have a
// hostname that intersects the route's hostnames. This is the same per-listener predicate
// evaluateHTTPRouteAttachment applies; it is factored out so vhost derivation uses the identical
// acceptance check rather than the bare structural parentRefAttachesToListener (which, for a
// gateway-wide parentRef, matches every listener and would otherwise emit vhosts for listeners
// the route never attached to).
func httpRouteAcceptedByListener(
	ctx context.Context,
	cl client.Client,
	gw *gatewayv1.Gateway,
	route *gatewayv1.HTTPRoute,
	parentRef gatewayv1.ParentReference,
	listener gatewayv1.Listener,
) (bool, error) {
	if !parentRefAttachesToListener(parentRef, listener) {
		return false, nil
	}
	if !listenerAllowsHTTPRouteKind(listener) {
		return false, nil
	}
	allowedNs, err := namespacesAllowedByListener(ctx, cl, listener, gw)
	if err != nil {
		return false, err
	}
	if !namespaceAllowed(route.Namespace, allowedNs) {
		return false, nil
	}
	return hostnamesIntersect(listener.Hostname, route.Spec.Hostnames), nil
}

func namespaceAllowed(routeNamespace string, allowed []string) bool {
	for _, ns := range allowed {
		if ns == routeNamespace {
			return true
		}
	}
	return false
}

// hostnamesIntersect reports whether route hostnames intersect with the listener hostname.
// Empty route hostnames match any listener hostname; empty listener hostname matches any route hostnames.
func hostnamesIntersect(listenerHostname *gatewayv1.Hostname, routeHostnames []gatewayv1.Hostname) bool {
	if listenerHostname == nil || string(*listenerHostname) == "" {
		return true
	}
	if len(routeHostnames) == 0 {
		return true
	}
	listenerHost := normalizeHostname(string(*listenerHostname))
	for _, rh := range routeHostnames {
		if hostnamePairMatch(listenerHost, normalizeHostname(string(rh))) {
			return true
		}
	}
	return false
}

func normalizeHostname(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host
}

func hostnamePairMatch(a, b string) bool {
	if a == b {
		return true
	}
	if strings.HasPrefix(a, "*.") {
		return hostnameWildcardMatch(a, b)
	}
	if strings.HasPrefix(b, "*.") {
		return hostnameWildcardMatch(b, a)
	}
	return false
}

func hostnameWildcardMatch(wildcardHost, preciseHost string) bool {
	suffix := wildcardHost[1:]
	if suffix == "" || !strings.HasPrefix(suffix, ".") {
		return false
	}
	if !strings.HasSuffix(preciseHost, suffix) {
		return false
	}
	prefix := strings.TrimSuffix(preciseHost, suffix)
	return prefix != "" && !strings.Contains(prefix, ".")
}
