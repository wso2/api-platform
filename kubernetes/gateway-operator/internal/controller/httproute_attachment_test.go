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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func hostnamePtr(h string) *gatewayv1.Hostname {
	v := gatewayv1.Hostname(h)
	return &v
}

func TestHostnamesIntersect(t *testing.T) {
	tests := []struct {
		name      string
		listener  *gatewayv1.Hostname
		route     []gatewayv1.Hostname
		wantMatch bool
	}{
		{
			name:      "empty route hostnames match listener hostname",
			listener:  hostnamePtr("foo.example.com"),
			route:     nil,
			wantMatch: true,
		},
		{
			name:      "empty listener hostname matches route hostnames",
			listener:  nil,
			route:     []gatewayv1.Hostname{"bar.example.com"},
			wantMatch: true,
		},
		{
			name:      "exact hostname match",
			listener:  hostnamePtr("foo.example.com"),
			route:     []gatewayv1.Hostname{"foo.example.com"},
			wantMatch: true,
		},
		{
			name:      "mismatched hostname",
			listener:  hostnamePtr("foo.example.com"),
			route:     []gatewayv1.Hostname{"not-accepted.test.com"},
			wantMatch: false,
		},
		{
			name:      "wildcard listener matches precise route hostname",
			listener:  hostnamePtr("*.example.com"),
			route:     []gatewayv1.Hostname{"foo.example.com"},
			wantMatch: true,
		},
		{
			name:      "precise listener matches wildcard route hostname",
			listener:  hostnamePtr("foo.example.com"),
			route:     []gatewayv1.Hostname{"*.example.com"},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostnamesIntersect(tt.listener, tt.route)
			if got != tt.wantMatch {
				t.Fatalf("hostnamesIntersect() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestEvaluateHTTPRouteAttachment(t *testing.T) {
	infraLabel := map[string]string{"kubernetes.io/metadata.name": "infra"}
	sameSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"kubernetes.io/metadata.name": "infra"},
	}
	listenerHostname := gatewayv1.Hostname("foo.example.com")

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "infra"},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     80,
				Hostname: &listenerHostname,
				AllowedRoutes: &gatewayv1.AllowedRoutes{
					Kinds: []gatewayv1.RouteGroupKind{{Kind: gatewayv1.Kind("HTTPRoute")}},
					Namespaces: &gatewayv1.RouteNamespaces{
						From:     ptrNamespacesFrom(gatewayv1.NamespacesFromSelector),
						Selector: sameSelector,
					},
				},
			}},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = gatewayv1.Install(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "infra", Labels: infraLabel}},
	).Build()

	parentRef := gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw")}

	t.Run("route without hostnames attaches", func(t *testing.T) {
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "route-2", Namespace: "infra"},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: []gatewayv1.ParentReference{parentRef}},
			},
		}
		attached, reason, _ := evaluateHTTPRouteAttachment(context.Background(), cl, gw, route, parentRef)
		if !attached || reason != gatewayv1.RouteReasonAccepted {
			t.Fatalf("attached=%v reason=%q, want attached=true reason=Accepted", attached, reason)
		}
	})

	t.Run("route with mismatched hostname rejected", func(t *testing.T) {
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "route-na", Namespace: "infra"},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{parentRef},
				},
				Hostnames: []gatewayv1.Hostname{"not-accepted.test.com"},
			},
		}
		attached, reason, _ := evaluateHTTPRouteAttachment(context.Background(), cl, gw, route, parentRef)
		if attached || reason != gatewayv1.RouteReasonNoMatchingListenerHostname {
			t.Fatalf("attached=%v reason=%q, want attached=false reason=NoMatchingListenerHostname", attached, reason)
		}
	})
}

func ptrNamespacesFrom(v gatewayv1.FromNamespaces) *gatewayv1.FromNamespaces {
	return &v
}
