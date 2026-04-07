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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestLastDeployedParentGatewayKeyFromAnnotation(t *testing.T) {
	t.Parallel()
	gwGroup := gatewayv1.Group(gatewayv1.GroupName)
	gwKind := gatewayv1.Kind("Gateway")
	ref := gatewayv1.ParentReference{
		Group: &gwGroup,
		Kind:  &gwKind,
		Name:  gatewayv1.ObjectName("gw"),
	}

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "routes",
			Name:        "r1",
			Annotations: map[string]string{AnnHTTPRouteLastDeployedParentGateway: "infra/gw"},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{}, // spec cleared — mimic delete-time mutation
			},
		},
	}

	key, ok := lastDeployedParentGatewayKeyFromAnnotation(route)
	if !ok || key.Namespace != "infra" || key.Name != "gw" {
		t.Fatalf("annotation parse: ok=%v key=%+v", ok, key)
	}

	got := deletionParentGatewayKey(route)
	if got != key {
		t.Fatalf("deletionParentGatewayKey want %+v got %+v", key, got)
	}

	// Live spec wins when annotation missing
	route2 := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "routes", Name: "r2"},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{ref},
			},
		},
	}
	ns := gatewayv1.Namespace("infra")
	route2.Spec.CommonRouteSpec.ParentRefs[0].Namespace = &ns
	if k := deletionParentGatewayKey(route2); k != (client.ObjectKey{Namespace: "infra", Name: "gw"}) {
		t.Fatalf("spec fallback: got %+v", k)
	}

	// Annotation takes precedence over spec when both present
	route3 := route2.DeepCopy()
	route3.Annotations = map[string]string{AnnHTTPRouteLastDeployedParentGateway: "other/gw2"}
	if k := deletionParentGatewayKey(route3); k != (client.ObjectKey{Namespace: "other", Name: "gw2"}) {
		t.Fatalf("annotation precedence: got %+v", k)
	}
}

func TestLastDeployedParentGatewayKeyFromAnnotation_Invalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ann  string
	}{
		{"empty", ""},
		{"no-slash", "gwonly"},
		{"empty-ns", "/gw"},
		{"empty-name", "ns/"},
		{"whitespace", "  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			route := &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n",
					Name:      "r",
					Annotations: map[string]string{
						AnnHTTPRouteLastDeployedParentGateway: tc.ann,
					},
				},
			}
			if _, ok := lastDeployedParentGatewayKeyFromAnnotation(route); ok {
				t.Fatal("expected invalid")
			}
		})
	}
}

func TestParentRefMatches(t *testing.T) {
	t.Parallel()

	gwGroup := gatewayv1.Group(gatewayv1.GroupName)
	gwKind := gatewayv1.Kind("Gateway")
	ns := gatewayv1.Namespace("infra")
	section := gatewayv1.SectionName("https")
	port := gatewayv1.PortNumber(8443)

	base := gatewayv1.ParentReference{
		Name:        gatewayv1.ObjectName("gw"),
		Namespace:   &ns,
		SectionName: &section,
		Port:        &port,
		Kind:        &gwKind,
		Group:       &gwGroup,
	}

	if !parentRefMatches(base, base, "routes") {
		t.Fatal("expected exact same parent references to match")
	}

	withDifferentPort := base
	otherPort := gatewayv1.PortNumber(443)
	withDifferentPort.Port = &otherPort
	if parentRefMatches(base, withDifferentPort, "routes") {
		t.Fatal("expected different port to not match")
	}

	withDifferentSection := base
	otherSection := gatewayv1.SectionName("http")
	withDifferentSection.SectionName = &otherSection
	if parentRefMatches(base, withDifferentSection, "routes") {
		t.Fatal("expected different sectionName to not match")
	}

	withNilNamespace := gatewayv1.ParentReference{
		Name:        gatewayv1.ObjectName("gw"),
		SectionName: &section,
		Port:        &port,
		Kind:        &gwKind,
		Group:       &gwGroup,
	}
	withRouteNamespace := withNilNamespace
	routeNS := gatewayv1.Namespace("routes")
	withRouteNamespace.Namespace = &routeNS
	if !parentRefMatches(withNilNamespace, withRouteNamespace, "routes") {
		t.Fatal("expected nil Namespace to match explicit route namespace")
	}

	withDifferentKind := base
	otherKind := gatewayv1.Kind("Service")
	withDifferentKind.Kind = &otherKind
	if parentRefMatches(base, withDifferentKind, "routes") {
		t.Fatal("expected different kind to not match")
	}

	withDifferentGroup := base
	otherGroup := gatewayv1.Group("example.com")
	withDifferentGroup.Group = &otherGroup
	if parentRefMatches(base, withDifferentGroup, "routes") {
		t.Fatal("expected different group to not match")
	}
}
