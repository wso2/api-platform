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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// TestNormalizeContext verifies the compiled Context always satisfies the APIConfigData schema
// contract: a single leading "/", no trailing slash, with "/" for the root context. Trailing-slash
// inputs (which the controller's validator rejects) must be normalized, not passed through.
func TestNormalizeContext(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty defaults to root", "", "/"},
		{"root stays root", "/", "/"},
		{"whitespace defaults to root", "   ", "/"},
		{"missing leading slash", "api", "/api"},
		{"trailing slash stripped", "/api/", "/api"},
		{"nested trailing slash stripped", "/api/v1/", "/api/v1"},
		{"surrounding whitespace trimmed", "  /api/v1  ", "/api/v1"},
		{"already normalized unchanged", "/reading-list/v1.0", "/reading-list/v1.0"},
		{"version placeholder preserved", "/reading-list/$version", "/reading-list/$version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, normalizeContext(tc.in))
		})
	}
}

// TestIntersectHostname covers the Gateway-API hostname intersection rules exercised by the
// HTTPRouteHostnameIntersection conformance test (listeners very.specific.com, *.wildcard.io,
// *.anotherwildcard.io).
func TestIntersectHostname(t *testing.T) {
	cases := []struct {
		route, listener string
		want            string
		ok              bool
	}{
		// exact == exact
		{"very.specific.com", "very.specific.com", "very.specific.com", true},
		// specific route under wildcard listener -> keep specific (route may be multi-label)
		{"foo.wildcard.io", "*.wildcard.io", "foo.wildcard.io", true},
		{"foo.bar.wildcard.io", "*.wildcard.io", "foo.bar.wildcard.io", true},
		// bare apex does NOT match the wildcard
		{"wildcard.io", "*.wildcard.io", "", false},
		// wildcard route narrowed by specific listener -> keep the specific listener host
		{"*.specific.com", "very.specific.com", "very.specific.com", true},
		// wildcard route that does NOT cover the specific listener host
		{"*.specific.com", "other.example.com", "", false},
		// both wildcards equal
		{"*.anotherwildcard.io", "*.anotherwildcard.io", "*.anotherwildcard.io", true},
		// decoy hostnames that match no listener
		{"non.matching.com", "very.specific.com", "", false},
		{"non.matching.com", "*.wildcard.io", "", false},
		{"*.nonmatchingwildcard.io", "*.wildcard.io", "", false},
		// empty listener hostname (unspecified listener) accepts the route hostname as-is
		{"first.com", "", "first.com", true},
		{"*.anotherwildcard.io", "", "*.anotherwildcard.io", true},
	}
	for _, c := range cases {
		got, ok := intersectHostname(c.route, c.listener)
		if got != c.want || ok != c.ok {
			t.Errorf("intersectHostname(%q, %q) = (%q, %v); want (%q, %v)",
				c.route, c.listener, got, ok, c.want, c.ok)
		}
	}
}

func TestHostnameMatchesWildcard(t *testing.T) {
	cases := []struct {
		host, wildcard string
		want           bool
	}{
		{"foo.wildcard.io", "*.wildcard.io", true},
		{"foo.bar.wildcard.io", "*.wildcard.io", true},
		{"wildcard.io", "*.wildcard.io", false},
		{"foo.other.io", "*.wildcard.io", false},
		{"very.specific.com", "*.specific.com", true},
	}
	for _, c := range cases {
		if got := hostnameMatchesWildcard(c.host, c.wildcard); got != c.want {
			t.Errorf("hostnameMatchesWildcard(%q, %q) = %v; want %v", c.host, c.wildcard, got, c.want)
		}
	}
}

// TestDeriveVhosts_MixedListenerAcceptance is a regression test for the bug where vhost
// derivation trusted only the structural parentRef→listener match. With a gateway-wide
// parentRef (which matches every listener), it emitted vhosts for listeners the route never
// attached to — e.g. listeners that reject the route by AllowedRoutes namespace or kind. Now
// it must apply the full per-listener acceptance predicate and emit only accepted listeners'
// hostnames.
func TestDeriveVhosts_MixedListenerAcceptance(t *testing.T) {
	nsLabel := func(name string) map[string]string {
		return map[string]string{"kubernetes.io/metadata.name": name}
	}
	selector := func(ns string) *metav1.LabelSelector {
		return &metav1.LabelSelector{MatchLabels: nsLabel(ns)}
	}
	host := func(h string) *gatewayv1.Hostname {
		v := gatewayv1.Hostname(h)
		return &v
	}

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "infra"},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{
				{
					// Accepts: allows HTTPRoute and the route's namespace (ns1).
					Name:     "web-a",
					Protocol: gatewayv1.HTTPProtocolType,
					Port:     80,
					Hostname: host("a.example.com"),
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Kinds: []gatewayv1.RouteGroupKind{{Kind: gatewayv1.Kind("HTTPRoute")}},
						Namespaces: &gatewayv1.RouteNamespaces{
							From:     ptrNamespacesFrom(gatewayv1.NamespacesFromSelector),
							Selector: selector("ns1"),
						},
					},
				},
				{
					// Rejects by namespace: only the "other" namespace is allowed, not ns1.
					Name:     "web-b",
					Protocol: gatewayv1.HTTPProtocolType,
					Port:     80,
					Hostname: host("b.example.com"),
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Kinds: []gatewayv1.RouteGroupKind{{Kind: gatewayv1.Kind("HTTPRoute")}},
						Namespaces: &gatewayv1.RouteNamespaces{
							From:     ptrNamespacesFrom(gatewayv1.NamespacesFromSelector),
							Selector: selector("other"),
						},
					},
				},
				{
					// Rejects by kind: only TCPRoute is allowed on this listener.
					Name:     "web-c",
					Protocol: gatewayv1.HTTPProtocolType,
					Port:     80,
					Hostname: host("c.example.com"),
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Kinds: []gatewayv1.RouteGroupKind{{Kind: gatewayv1.Kind("TCPRoute")}},
						Namespaces: &gatewayv1.RouteNamespaces{
							From:     ptrNamespacesFrom(gatewayv1.NamespacesFromSelector),
							Selector: selector("ns1"),
						},
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = gatewayv1.Install(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1", Labels: nsLabel("ns1")}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: nsLabel("other")}},
	).Build()

	// Gateway-wide parentRef (no sectionName/port): structurally matches every listener.
	parentRef := gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw")}
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns1"},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: []gatewayv1.ParentReference{parentRef}},
			// Hostnames overlap all three listeners, so only namespace/kind acceptance can
			// filter b and c — isolating the behavior under test.
			Hostnames: []gatewayv1.Hostname{"a.example.com", "b.example.com", "c.example.com"},
		},
	}
	targets := []gatewayParentTarget{{ref: parentRef}}

	main, additional, err := deriveVhosts(context.Background(), cl, gw, route, targets)
	require.NoError(t, err)

	var got []string
	if main != "" {
		got = append(got, main)
		got = append(got, additional...)
	}
	require.Equal(t, []string{"a.example.com"}, got,
		"only the accepted listener's hostname should be emitted; b (namespace) and c (kind) must be excluded")
}
