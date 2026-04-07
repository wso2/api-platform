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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestBuildAPIConfigFromHTTPRoute(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 8080}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/api/hello"
	method := gatewayv1.HTTPMethodGet

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-route",
			Namespace:   "default",
			Annotations: map[string]string{AnnHTTPRouteContext: "/api"},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path:   &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal},
							Method: &method,
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName("backend"),
									Port: ptrPort(8080),
								},
							},
						},
					},
				},
			},
		},
	}

	spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route)
	require.NoError(t, err)
	require.Equal(t, "/api", spec.Context)
	require.Equal(t, "my-route", spec.DisplayName)
	require.Equal(t, "v1", spec.Version)
	require.Len(t, spec.Operations, 1)
	require.Equal(t, "/api/hello", spec.Operations[0].Path)
	require.Equal(t, "http://backend.default.svc.cluster.local:8080", spec.Upstream.Main.Url)
}

func TestDefaultHTTPRouteAPIHandle(t *testing.T) {
	r := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns1"},
	}
	require.Equal(t, "ns1-x", DefaultHTTPRouteAPIHandle(r))
}

func ptrPort(p int32) *gatewayv1.PortNumber {
	pn := gatewayv1.PortNumber(p)
	return &pn
}
