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

func acceptedHTTPRouteParentStatus(ref gatewayv1.ParentReference) []gatewayv1.RouteParentStatus {
	return []gatewayv1.RouteParentStatus{{
		ParentRef:      ref,
		ControllerName: PlatformGatewayControllerName,
		Conditions: []metav1.Condition{{
			Type:   string(gatewayv1.RouteConditionAccepted),
			Status: metav1.ConditionTrue,
			Reason: string(gatewayv1.RouteReasonAccepted),
		}},
	}}
}

func TestComputeAttachedHTTPRoutesByListener(t *testing.T) {
	allNamespaces := gatewayv1.NamespacesFromAll
	gwGroup := gatewayv1.Group(gatewayv1.GroupName)
	gwKind := gatewayv1.Kind("Gateway")

	baseGateway := func() *gatewayv1.Gateway {
		return &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-gateway",
				Namespace: "infra",
			},
			Spec: gatewayv1.GatewaySpec{
				Listeners: []gatewayv1.Listener{
					{
						Name:     "https",
						Protocol: gatewayv1.HTTPSProtocolType,
						Port:     443,
						AllowedRoutes: &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{From: &allNamespaces},
						},
					},
					{
						Name:     "http",
						Protocol: gatewayv1.HTTPProtocolType,
						Port:     80,
						AllowedRoutes: &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{From: &allNamespaces},
						},
					},
				},
			},
		}
	}

	parentRefAllListeners := gatewayv1.ParentReference{
		Name:  gatewayv1.ObjectName("test-gateway"),
		Kind:  &gwKind,
		Group: &gwGroup,
	}

	tests := []struct {
		name       string
		gateway    *gatewayv1.Gateway
		routes     []gatewayv1.HTTPRoute
		namespaces []corev1.Namespace
		want       map[gatewayv1.SectionName]int32
	}{
		{
			name:    "route with no sectionName attaches to all listeners",
			gateway: baseGateway(),
			routes: []gatewayv1.HTTPRoute{{
				ObjectMeta: metav1.ObjectMeta{Name: "route-1", Namespace: "infra"},
				Spec: gatewayv1.HTTPRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{parentRefAllListeners},
					},
				},
				Status: gatewayv1.HTTPRouteStatus{
					RouteStatus: gatewayv1.RouteStatus{
						Parents: acceptedHTTPRouteParentStatus(parentRefAllListeners),
					},
				},
			}},
			namespaces: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "infra"}}},
			want:       map[gatewayv1.SectionName]int32{"https": 1, "http": 1},
		},
		{
			name:    "route with sectionName attaches to one listener",
			gateway: baseGateway(),
			routes: func() []gatewayv1.HTTPRoute {
				section := gatewayv1.SectionName("https")
				ref := parentRefAllListeners
				ref.SectionName = &section
				return []gatewayv1.HTTPRoute{{
					ObjectMeta: metav1.ObjectMeta{Name: "route-1", Namespace: "infra"},
					Spec: gatewayv1.HTTPRouteSpec{
						CommonRouteSpec: gatewayv1.CommonRouteSpec{
							ParentRefs: []gatewayv1.ParentReference{ref},
						},
					},
					Status: gatewayv1.HTTPRouteStatus{
						RouteStatus: gatewayv1.RouteStatus{
							Parents: acceptedHTTPRouteParentStatus(ref),
						},
					},
				}}
			}(),
			namespaces: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "infra"}}},
			want:       map[gatewayv1.SectionName]int32{"https": 1, "http": 0},
		},
		{
			name:    "route not accepted is not counted",
			gateway: baseGateway(),
			routes: []gatewayv1.HTTPRoute{{
				ObjectMeta: metav1.ObjectMeta{Name: "route-1", Namespace: "infra"},
				Spec: gatewayv1.HTTPRouteSpec{
					CommonRouteSpec: gatewayv1.CommonRouteSpec{
						ParentRefs: []gatewayv1.ParentReference{parentRefAllListeners},
					},
				},
				Status: gatewayv1.HTTPRouteStatus{
					RouteStatus: gatewayv1.RouteStatus{
						Parents: []gatewayv1.RouteParentStatus{{
							ParentRef:      parentRefAllListeners,
							ControllerName: PlatformGatewayControllerName,
							Conditions: []metav1.Condition{{
								Type:   string(gatewayv1.RouteConditionAccepted),
								Status: metav1.ConditionFalse,
								Reason: "GatewayPending",
							}},
						}},
					},
				},
			}},
			namespaces: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "infra"}}},
			want:       map[gatewayv1.SectionName]int32{"https": 0, "http": 0},
		},
		{
			name: "route in different namespace not counted with allowedRoutes Same",
			gateway: func() *gatewayv1.Gateway {
				gw := baseGateway()
				same := gatewayv1.NamespacesFromSame
				for i := range gw.Spec.Listeners {
					gw.Spec.Listeners[i].AllowedRoutes = &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{From: &same},
					}
				}
				return gw
			}(),
			routes: func() []gatewayv1.HTTPRoute {
				gwNs := gatewayv1.Namespace("infra")
				ref := parentRefAllListeners
				ref.Namespace = &gwNs
				return []gatewayv1.HTTPRoute{{
					ObjectMeta: metav1.ObjectMeta{Name: "route-1", Namespace: "other"},
					Spec: gatewayv1.HTTPRouteSpec{
						CommonRouteSpec: gatewayv1.CommonRouteSpec{
							ParentRefs: []gatewayv1.ParentReference{ref},
						},
					},
					Status: gatewayv1.HTTPRouteStatus{
						RouteStatus: gatewayv1.RouteStatus{
							Parents: acceptedHTTPRouteParentStatus(ref),
						},
					},
				}}
			}(),
			namespaces: []corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "infra"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
			},
			want: map[gatewayv1.SectionName]int32{"https": 0, "http": 0},
		},
		{
			name:    "two accepted routes increment count",
			gateway: func() *gatewayv1.Gateway {
				gw := baseGateway()
				gw.Spec.Listeners = gw.Spec.Listeners[1:] // http only
				return gw
			}(),
			routes: []gatewayv1.HTTPRoute{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "route-1", Namespace: "infra"},
					Spec: gatewayv1.HTTPRouteSpec{
						CommonRouteSpec: gatewayv1.CommonRouteSpec{
							ParentRefs: []gatewayv1.ParentReference{parentRefAllListeners},
						},
					},
					Status: gatewayv1.HTTPRouteStatus{
						RouteStatus: gatewayv1.RouteStatus{
							Parents: acceptedHTTPRouteParentStatus(parentRefAllListeners),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "route-2", Namespace: "infra"},
					Spec: gatewayv1.HTTPRouteSpec{
						CommonRouteSpec: gatewayv1.CommonRouteSpec{
							ParentRefs: []gatewayv1.ParentReference{parentRefAllListeners},
						},
					},
					Status: gatewayv1.HTTPRouteStatus{
						RouteStatus: gatewayv1.RouteStatus{
							Parents: acceptedHTTPRouteParentStatus(parentRefAllListeners),
						},
					},
				},
			},
			namespaces: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "infra"}}},
			want:       map[gatewayv1.SectionName]int32{"http": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = gatewayv1.Install(scheme)

			builder := fake.NewClientBuilder().WithScheme(scheme)
			for i := range tt.namespaces {
				builder = builder.WithObjects(&tt.namespaces[i])
			}
			for i := range tt.routes {
				builder = builder.WithObjects(&tt.routes[i])
			}
			cl := builder.Build()

			got, err := computeAttachedHTTPRoutesByListener(context.Background(), cl, tt.gateway)
			if err != nil {
				t.Fatalf("computeAttachedHTTPRoutesByListener() error: %v", err)
			}
			for name, wantCount := range tt.want {
				if got[name] != wantCount {
					t.Errorf("listener %q AttachedRoutes = %d, want %d", name, got[name], wantCount)
				}
			}
		})
	}
}

func TestParentRefAttachesToListener(t *testing.T) {
	listener := gatewayv1.Listener{Name: "https", Port: 443}

	ref := gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw")}
	if !parentRefAttachesToListener(ref, listener) {
		t.Fatal("expected parentRef without sectionName/port to attach to listener")
	}

	section := gatewayv1.SectionName("https")
	refWithSection := ref
	refWithSection.SectionName = &section
	if !parentRefAttachesToListener(refWithSection, listener) {
		t.Fatal("expected matching sectionName to attach")
	}

	otherSection := gatewayv1.SectionName("http")
	refOtherSection := ref
	refOtherSection.SectionName = &otherSection
	if parentRefAttachesToListener(refOtherSection, listener) {
		t.Fatal("expected mismatched sectionName to not attach")
	}

	port := gatewayv1.PortNumber(443)
	refWithPort := ref
	refWithPort.Port = &port
	if !parentRefAttachesToListener(refWithPort, listener) {
		t.Fatal("expected matching port to attach")
	}

	otherPort := gatewayv1.PortNumber(80)
	refOtherPort := ref
	refOtherPort.Port = &otherPort
	if parentRefAttachesToListener(refOtherPort, listener) {
		t.Fatal("expected mismatched port to not attach")
	}
}
