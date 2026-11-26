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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// The golden fixtures are single-parent routes whose deployed payloads were captured from the
// operator before multi-Gateway support (testdata/httproute_golden). Upgrading must not change
// a single byte of them, or every existing route would redeploy at once.

const (
	goldenGatewayClass     = "wso2-golden"
	goldenGatewayNamespace = "golden-infra"
	goldenRouteNamespace   = "golden-apps"
)

type goldenFixture struct {
	name    string
	gateway *gatewayv1.Gateway
	route   *gatewayv1.HTTPRoute
	objects []client.Object
}

func goldenStr[T ~string](s string) *T { v := T(s); return &v }

func goldenPort(p int32) *gatewayv1.PortNumber { v := gatewayv1.PortNumber(p); return &v }

func goldenGateway(name string, listeners ...gatewayv1.Listener) *gatewayv1.Gateway {
	return &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: goldenGatewayNamespace},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(goldenGatewayClass),
			Listeners:        listeners,
		},
	}
}

func goldenListener(name string, port int32, hostname string) gatewayv1.Listener {
	all := gatewayv1.NamespacesFromAll
	l := gatewayv1.Listener{
		Name:          gatewayv1.SectionName(name),
		Port:          gatewayv1.PortNumber(port),
		Protocol:      gatewayv1.HTTPProtocolType,
		AllowedRoutes: &gatewayv1.AllowedRoutes{Namespaces: &gatewayv1.RouteNamespaces{From: &all}},
	}
	if hostname != "" {
		l.Hostname = goldenStr[gatewayv1.Hostname](hostname)
	}
	return l
}

func goldenParentRef(gw, section string) gatewayv1.ParentReference {
	ref := gatewayv1.ParentReference{
		Name:      gatewayv1.ObjectName(gw),
		Namespace: goldenStr[gatewayv1.Namespace](goldenGatewayNamespace),
	}
	if section != "" {
		ref.SectionName = goldenStr[gatewayv1.SectionName](section)
	}
	return ref
}

func goldenBackend(name string, weight int32) gatewayv1.HTTPBackendRef {
	w := weight
	return gatewayv1.HTTPBackendRef{BackendRef: gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(name), Port: goldenPort(8080)},
		Weight:                 &w,
	}}
}

func goldenPrefixMatch(path string) gatewayv1.HTTPRouteMatch {
	t := gatewayv1.PathMatchPathPrefix
	return gatewayv1.HTTPRouteMatch{Path: &gatewayv1.HTTPPathMatch{Type: &t, Value: &path}}
}

func goldenRoute(name string, ann map[string]string, hostnames []string, refs []gatewayv1.ParentReference, rules ...gatewayv1.HTTPRouteRule) *gatewayv1.HTTPRoute {
	hs := make([]gatewayv1.Hostname, 0, len(hostnames))
	for _, h := range hostnames {
		hs = append(hs, gatewayv1.Hostname(h))
	}
	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: goldenRouteNamespace, Annotations: ann, Generation: 1},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: refs},
			Hostnames:       hs,
			Rules:           rules,
		},
	}
}

func goldenBaseObjects() []client.Object {
	svc := func(name string) client.Object {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: goldenRouteNamespace},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
		}
	}
	return []client.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: goldenGatewayNamespace}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: goldenRouteNamespace}},
		svc("echo-v1"), svc("echo-v2"),
	}
}

func goldenFixtures() []goldenFixture {
	exact := gatewayv1.HeaderMatchExact
	return []goldenFixture{
		{
			name:    "basic",
			gateway: goldenGateway("gw-basic", goldenListener("http", 80, "")),
			route: goldenRoute("basic", nil, nil, []gatewayv1.ParentReference{goldenParentRef("gw-basic", "")},
				gatewayv1.HTTPRouteRule{Matches: []gatewayv1.HTTPRouteMatch{goldenPrefixMatch("/")}, BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v1", 1)}}),
		},
		{
			name:    "hostnames",
			gateway: goldenGateway("gw-hosts", goldenListener("http", 80, "*.example.com")),
			route: goldenRoute("hostnames", nil, []string{"api.example.com", "other.org"}, []gatewayv1.ParentReference{goldenParentRef("gw-hosts", "")},
				gatewayv1.HTTPRouteRule{Matches: []gatewayv1.HTTPRouteMatch{goldenPrefixMatch("/v1")}, BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v1", 1)}}),
		},
		{
			name: "two-sections",
			gateway: goldenGateway("gw-sections",
				goldenListener("http", 80, "a.example.com"),
				goldenListener("alt", 8080, "b.example.com")),
			route: goldenRoute("two-sections", nil, nil,
				[]gatewayv1.ParentReference{goldenParentRef("gw-sections", "http"), goldenParentRef("gw-sections", "alt")},
				gatewayv1.HTTPRouteRule{Matches: []gatewayv1.HTTPRouteMatch{goldenPrefixMatch("/")}, BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v2", 1)}}),
		},
		{
			name:    "annotations",
			gateway: goldenGateway("gw-ann", goldenListener("http", 80, "")),
			route: goldenRoute("annotations", map[string]string{
				AnnHTTPRouteContext:     "/orders/$version",
				AnnHTTPRouteDisplayName: "Orders",
				AnnHTTPRouteAPIVersion:  "v2.1",
				AnnProjectID:            "proj-1",
				AnnHTTPRouteAPIHandle:   "orders-api",
				"example.com/owner":     "team-a",
			}, nil, []gatewayv1.ParentReference{goldenParentRef("gw-ann", "")},
				gatewayv1.HTTPRouteRule{Matches: []gatewayv1.HTTPRouteMatch{goldenPrefixMatch("/")}, BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v1", 1)}}),
		},
		{
			name:    "weighted-filters",
			gateway: goldenGateway("gw-weighted", goldenListener("http", 80, "")),
			route: goldenRoute("weighted-filters", nil, nil, []gatewayv1.ParentReference{goldenParentRef("gw-weighted", "")},
				gatewayv1.HTTPRouteRule{
					Matches: []gatewayv1.HTTPRouteMatch{{
						Path:    goldenPrefixMatch("/weighted").Path,
						Headers: []gatewayv1.HTTPHeaderMatch{{Type: &exact, Name: "x-canary", Value: "yes"}},
					}},
					Filters: []gatewayv1.HTTPRouteFilter{{
						Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
						RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
							Set: []gatewayv1.HTTPHeader{{Name: "x-route", Value: "weighted"}},
						},
					}},
					BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v1", 3), goldenBackend("echo-v2", 1)},
				},
				gatewayv1.HTTPRouteRule{Matches: []gatewayv1.HTTPRouteMatch{goldenPrefixMatch("/")}, BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v2", 1)}}),
		},
	}
}
