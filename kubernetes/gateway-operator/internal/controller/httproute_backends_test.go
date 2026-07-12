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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

	res, err := resolveHTTPRouteBackendRefs(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
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

	mustPort := func(p int32, err error) int32 {
		t.Helper()
		require.NoError(t, err)
		return p
	}

	// ClusterIP service: always connect on the Service port (kube-proxy translates).
	require.Equal(t, int32(8080), mustPort(serviceConnectionPort(clusterIPSvc(8080, 3000), 8080)))

	// Headless service with integer targetPort: connect on the target port (the pod's port).
	require.Equal(t, int32(3000), mustPort(serviceConnectionPort(headlessSvc(8080, intstr.FromInt32(3000)), 8080)))

	// Headless service with a named targetPort: not resolvable from the Service spec, so
	// resolution fails explicitly rather than returning a possibly-wrong Service port.
	_, err := serviceConnectionPort(headlessSvc(8080, intstr.FromString("http")), 8080)
	require.Error(t, err)
	require.Contains(t, err.Error(), "named targetPort")

	// Headless service with unset targetPort (Kubernetes defaults it to the Service port).
	require.Equal(t, int32(8080), mustPort(serviceConnectionPort(headlessSvc(8080, intstr.IntOrString{}), 8080)))

	// nil service.
	require.Equal(t, int32(8080), mustPort(serviceConnectionPort(nil, 8080)))
}

// serviceBackendRoute builds a minimal HTTPRoute with a single core/Service backendRef in the
// "default" namespace, used by the backend-resolution tests below.
func serviceBackendRoute(name string, port int32) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				BackendRefs: []gatewayv1.HTTPBackendRef{{
					BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: gatewayv1.ObjectName(name),
						Port: ptrPort(port),
					}},
				}},
			}},
		},
	}
}

// A headless Service with a NAMED targetPort cannot be resolved to a pod port from the Service
// spec alone, so resolution fails per-ref with UnsupportedValue (a permanent failure, not a
// transient error) instead of emitting a possibly-wrong upstream URL.
func TestResolveHTTPRouteBackendRefs_HeadlessNamedTargetPort(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Ports:     []corev1.ServicePort{{Port: 8080, TargetPort: intstr.FromString("http")}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	res, err := resolveHTTPRouteBackendRefs(context.Background(), cl, serviceBackendRoute("backend", 8080), "cluster.local")
	require.NoError(t, err) // permanent per-ref failure, not a transient/retryable error
	require.False(t, res.AllResolved)
	require.Equal(t, gatewayv1.RouteReasonUnsupportedValue, res.FirstFailureReason)
	require.Contains(t, res.FirstFailureMessage, "named targetPort")
}

// A non-NotFound error from the Service Get is transient: it must surface as a retryable error
// (not get collapsed into a permanent BackendNotFound result).
func TestResolveHTTPRouteBackendRefs_TransientGetError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))

	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	cl := interceptor.NewClient(base, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Service); ok {
				return apierrors.NewServiceUnavailable("transient: API server unavailable")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})

	res, err := resolveHTTPRouteBackendRefs(context.Background(), cl, serviceBackendRoute("backend", 8080), "cluster.local")
	require.Error(t, err)
	require.True(t, IsTransientHTTPRouteConfigError(err), "non-NotFound Get error must be transient")
	require.NotEqual(t, gatewayv1.RouteReasonBackendNotFound, res.FirstFailureReason,
		"transient error must not be recorded as a permanent BackendNotFound")
}
