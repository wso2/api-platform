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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestResolveHTTPRouteBackendRefs_Reasons(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	unknownGroup := gatewayv1.Group("unknown.example.com")
	unknownKind := gatewayv1.Kind("NonExistent")
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				BackendRefs: []gatewayv1.HTTPBackendRef{{
					BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
						Group: &unknownGroup,
						Kind:  &unknownKind,
						Name:  gatewayv1.ObjectName("backend"),
						Port:  ptrPort(8080),
					}},
				}},
			}},
		},
	}

	res := resolveHTTPRouteBackendRefs(context.Background(), cl, route, "cluster.local")
	require.False(t, res.AllResolved)
	require.Equal(t, gatewayv1.RouteReasonInvalidKind, res.FirstFailureReason)
}

func TestServiceConnectionPort(t *testing.T) {
	clusterIPSvc := func(port, target int32) *corev1.Service {
		return &corev1.Service{
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.0.0.1",
				Ports:     []corev1.ServicePort{{Port: port, TargetPort: intstr.FromInt32(target)}},
			},
		}
	}
	headlessSvc := func(port int32, target intstr.IntOrString) *corev1.Service {
		return &corev1.Service{
			Spec: corev1.ServiceSpec{
				ClusterIP: corev1.ClusterIPNone,
				Ports:     []corev1.ServicePort{{Port: port, TargetPort: target}},
			},
		}
	}

	// ClusterIP service: always connect on the Service port (kube-proxy translates).
	require.Equal(t, int32(8080), serviceConnectionPort(clusterIPSvc(8080, 3000), 8080))

	// Headless service with integer targetPort: connect on the target port (the pod's port).
	require.Equal(t, int32(3000), serviceConnectionPort(headlessSvc(8080, intstr.FromInt32(3000)), 8080))

	// Headless service with a named targetPort: not resolvable from the Service spec, fall
	// back to the Service port.
	require.Equal(t, int32(8080), serviceConnectionPort(headlessSvc(8080, intstr.FromString("http")), 8080))

	// Headless service with unset targetPort (defaults to the Service port).
	require.Equal(t, int32(8080), serviceConnectionPort(headlessSvc(8080, intstr.IntOrString{}), 8080))

	// nil service.
	require.Equal(t, int32(8080), serviceConnectionPort(nil, 8080))
}
