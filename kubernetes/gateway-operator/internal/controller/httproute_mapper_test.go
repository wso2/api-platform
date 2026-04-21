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
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestBuildAPIConfigFromHTTPRoute(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

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

	spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
	require.NoError(t, err)
	require.Equal(t, "/api", spec.Context)
	require.Equal(t, "my-route", spec.DisplayName)
	require.Equal(t, "v1.0", spec.Version)
	require.Len(t, spec.Operations, 1)
	require.Equal(t, "/api/hello", spec.Operations[0].Path)
	require.Equal(t, "http://backend.default.svc.cluster.local:8080", spec.Upstream.Main.Url)
}

func TestBuildAPIConfigFromHTTPRoute_ContextAnnotationTrimAndNormalize(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/api/hello"
	method := gatewayv1.HTTPMethodGet

	t.Run("whitespace-only treated as unset => fallback commonPathPrefix", func(t *testing.T) {
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-route",
				Namespace: "default",
				Annotations: map[string]string{
					AnnHTTPRouteContext: "   ",
				},
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{{
					Matches: []gatewayv1.HTTPRouteMatch{{
						Path:   &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal},
						Method: &method,
					}},
					BackendRefs: []gatewayv1.HTTPBackendRef{{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("backend"),
								Port: ptrPort(8080),
							},
						},
					}},
				}},
			},
		}

		spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.NoError(t, err)
		require.Equal(t, "/api/hello", spec.Context)
	})

	t.Run("missing leading slash normalized", func(t *testing.T) {
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-route",
				Namespace: "default",
				Annotations: map[string]string{
					AnnHTTPRouteContext: "api",
				},
			},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{{
					Matches: []gatewayv1.HTTPRouteMatch{{
						Path:   &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal},
						Method: &method,
					}},
					BackendRefs: []gatewayv1.HTTPBackendRef{{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("backend"),
								Port: ptrPort(8080),
							},
						},
					}},
				}},
			},
		}

		spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.NoError(t, err)
		require.Equal(t, "/api", spec.Context)
	})
}

func TestBuildAPIConfigFromHTTPRoute_APIPolicyTargetRef(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/hello"
	method := gatewayv1.HTTPMethodGet

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pol-route",
			Namespace: "default",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName("backend"),
							Port: ptrPort(8080),
						}}},
					},
				},
			},
		},
	}

	apiPol := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-level",
			Namespace: "default",
		},
		Spec: apiv1.APIPolicySpec{
			TargetRef: &apiv1.APIPolicyTargetRef{
				Group: gatewayv1.GroupName,
				Kind:  "HTTPRoute",
				Name:  "pol-route",
			},
			Policies: []apiv1.Policy{
				{Name: "rate-limit", Version: "v1"},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route, apiPol).Build()

	spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
	require.NoError(t, err)
	require.Len(t, spec.Policies, 1)
	require.Equal(t, "rate-limit", spec.Policies[0].Name)
	require.Equal(t, "v1", spec.Policies[0].Version)
	require.Len(t, spec.Operations, 1)
	require.Len(t, spec.Operations[0].Policies, 0)
}

func TestBuildAPIConfigFromHTTPRoute_APIPolicyRuleExtensionRef(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/r"
	method := gatewayv1.HTTPMethodPost
	ft := gatewayv1.HTTPRouteFilterExtensionRef
	group := gatewayv1.Group(apiv1.GroupVersion.Group)
	kind := gatewayv1.Kind("APIPolicy")
	name := gatewayv1.ObjectName("rule-pol")

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ext-ref-route",
			Namespace: "default",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Filters: []gatewayv1.HTTPRouteFilter{
						{Type: ft, ExtensionRef: &gatewayv1.LocalObjectReference{Group: group, Kind: kind, Name: name}},
					},
					Matches: []gatewayv1.HTTPRouteMatch{
						{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName("backend"),
							Port: ptrPort(8080),
						}}},
					},
				},
			},
		},
	}

	rulePol := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rule-pol",
			Namespace: "default",
		},
		Spec: apiv1.APIPolicySpec{
			Policies: []apiv1.Policy{
				{Name: "ext-ref-policy", Version: "v1"},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route, rulePol).Build()

	spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
	require.NoError(t, err)
	require.Len(t, spec.Operations, 1)
	require.Equal(t, apiv1.OperationMethodPOST, spec.Operations[0].Method)
	require.Equal(t, "/r", spec.Operations[0].Path)
	require.Len(t, spec.Operations[0].Policies, 1)
	require.Equal(t, "ext-ref-policy", spec.Operations[0].Policies[0].Name)
}

func TestBuildAPIConfigFromHTTPRoute_RequiresExplicitMethod(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/x"

	t.Run("empty rule matches", func(t *testing.T) {
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{
					{
						Matches: []gatewayv1.HTTPRouteMatch{},
						BackendRefs: []gatewayv1.HTTPBackendRef{
							{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("backend"),
								Port: ptrPort(8080),
							}}},
						},
					},
				},
			},
		}
		_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.Error(t, err)
		require.True(t, IsInvalidHTTPRouteConfigError(err))
		require.Contains(t, err.Error(), "rule[0]")
		require.Contains(t, err.Error(), "no matches")
	})

	t.Run("nil match method", func(t *testing.T) {
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "r2", Namespace: "default"},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{
					{
						Matches: []gatewayv1.HTTPRouteMatch{
							{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}},
						},
						BackendRefs: []gatewayv1.HTTPBackendRef{
							{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName("backend"),
								Port: ptrPort(8080),
							}}},
						},
					},
				},
			},
		}
		_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.Error(t, err)
		require.True(t, IsInvalidHTTPRouteConfigError(err))
		require.Contains(t, err.Error(), "rule[0] match[0]")
		require.Contains(t, err.Error(), "omits method")
	})
}

func TestBuildAPIConfigFromHTTPRoute_InvalidPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/x"
	method := gatewayv1.HTTPMethodGet

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad",
			Namespace: "default",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName("backend"),
							Port: ptrPort(8080),
						}}},
					},
				},
			},
		},
	}
	apiPol := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-level",
			Namespace: "default",
		},
		Spec: apiv1.APIPolicySpec{
			TargetRef: &apiv1.APIPolicyTargetRef{
				Group: gatewayv1.GroupName,
				Kind:  "HTTPRoute",
				Name:  "bad",
			},
			Policies: []apiv1.Policy{
				{Name: "no-version"},
			},
		},
	}
	cl2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route, apiPol).Build()

	_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl2, route, "cluster.local", nil)
	require.Error(t, err)
	require.True(t, IsInvalidHTTPRouteConfigError(err))
}

func TestBuildAPIConfigFromHTTPRoute_CrossNamespaceReferenceGrant(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/api"
	method := gatewayv1.HTTPMethodGet
	dataNS := gatewayv1.Namespace("data")

	backendRef := gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
		Name:      gatewayv1.ObjectName("backend"),
		Namespace: &dataNS,
		Port:      ptrPort(8080),
	}}

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{
					{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: backendRef}},
			}},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "data"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}

	t.Run("rejects without ReferenceGrant", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route).Build()
		_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.Error(t, err)
		require.True(t, IsTransientHTTPRouteConfigError(err), err.Error())
	})

	t.Run("allows with matching ReferenceGrant", func(t *testing.T) {
		grant := &gatewayv1beta1.ReferenceGrant{
			ObjectMeta: metav1.ObjectMeta{Name: "allow-routes", Namespace: "data"},
			Spec: gatewayv1beta1.ReferenceGrantSpec{
				From: []gatewayv1beta1.ReferenceGrantFrom{{
					Group:     gatewayv1.Group(gatewayv1.GroupName),
					Kind:      gatewayv1.Kind("HTTPRoute"),
					Namespace: gatewayv1.Namespace("default"),
				}},
				To: []gatewayv1beta1.ReferenceGrantTo{{
					Group: gatewayv1.Group(""),
					Kind:  gatewayv1.Kind("Service"),
				}},
			},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route, grant).Build()
		spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.NoError(t, err)
		require.Equal(t, "http://backend.data.svc.cluster.local:8080", spec.Upstream.Main.Url)
	})

	t.Run("name-scoped grant must match service", func(t *testing.T) {
		wrongName := gatewayv1.ObjectName("other-svc")
		grant := &gatewayv1beta1.ReferenceGrant{
			ObjectMeta: metav1.ObjectMeta{Name: "scoped", Namespace: "data"},
			Spec: gatewayv1beta1.ReferenceGrantSpec{
				From: []gatewayv1beta1.ReferenceGrantFrom{{
					Group:     gatewayv1.Group(gatewayv1.GroupName),
					Kind:      gatewayv1.Kind("HTTPRoute"),
					Namespace: gatewayv1.Namespace("default"),
				}},
				To: []gatewayv1beta1.ReferenceGrantTo{{
					Group: gatewayv1.Group(""),
					Kind:  gatewayv1.Kind("Service"),
					Name:  &wrongName,
				}},
			},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route, grant).Build()
		_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.Error(t, err)
		require.True(t, IsTransientHTTPRouteConfigError(err), err.Error())
	})

	t.Run("rejects ReferenceGrant with empty From.group for HTTPRoute", func(t *testing.T) {
		grant := &gatewayv1beta1.ReferenceGrant{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-group", Namespace: "data"},
			Spec: gatewayv1beta1.ReferenceGrantSpec{
				From: []gatewayv1beta1.ReferenceGrantFrom{{
					Group:     gatewayv1.Group(""),
					Kind:      gatewayv1.Kind("HTTPRoute"),
					Namespace: gatewayv1.Namespace("default"),
				}},
				To: []gatewayv1beta1.ReferenceGrantTo{{
					Group: gatewayv1.Group(""),
					Kind:  gatewayv1.Kind("Service"),
				}},
			},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc, route, grant).Build()
		_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
		require.Error(t, err)
		require.True(t, IsTransientHTTPRouteConfigError(err), err.Error())
	})
}

func TestResolveServicePort(t *testing.T) {
	t.Parallel()
	multi := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "svc"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80, Name: "http"}, {Port: 8080, Name: "app"}},
		},
	}
	_, err := resolveServicePort(multi, nil)
	require.Error(t, err)
	require.True(t, IsInvalidHTTPRouteConfigError(err))
	require.Contains(t, err.Error(), "disambiguate")

	p80 := gatewayv1.PortNumber(80)
	got, err := resolveServicePort(multi, &p80)
	require.NoError(t, err)
	require.Equal(t, int32(80), got)

	p999 := gatewayv1.PortNumber(999)
	_, err = resolveServicePort(multi, &p999)
	require.Error(t, err)
	require.True(t, IsInvalidHTTPRouteConfigError(err))
	require.Contains(t, err.Error(), "no port 999")

	single := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "svc2"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 9090}}},
	}
	got, err = resolveServicePort(single, nil)
	require.NoError(t, err)
	require.Equal(t, int32(9090), got)
}

func TestSharedPrefix_PathSegmentBoundaries(t *testing.T) {
	t.Parallel()

	require.Equal(t, "/", sharedPrefix("/foo", "/foobar"))
	require.Equal(t, "/foo", sharedPrefix("/foo/bar", "/foo/baz"))
	require.Equal(t, "/foo/bar", sharedPrefix("/foo/bar", "/foo/bar/baz"))
	require.Equal(t, "/", sharedPrefix("/a", "/b"))
	require.Equal(t, "/foo", sharedPrefix("foo/bar", "foo/baz"))
}

func TestParsePolicyListJSON_RejectsObjectWithoutPoliciesKey(t *testing.T) {
	t.Parallel()

	_, err := parsePolicyListJSON([]byte(`{"foo":"bar"}`))
	require.Error(t, err)
	require.True(t, IsInvalidHTTPRouteConfigError(err))
	require.Contains(t, err.Error(), `must contain "policies"`)
}

func TestParsePolicyListYAML_RejectsObjectWithoutPoliciesKey(t *testing.T) {
	t.Parallel()

	_, err := parsePolicyListYAML([]byte("foo: bar\n"))
	require.Error(t, err)
	require.True(t, IsInvalidHTTPRouteConfigError(err))
	require.Contains(t, err.Error(), `must contain "policies"`)
}

func TestParsePolicyList_RejectsInvalidPolicyVersionFormat(t *testing.T) {
	t.Parallel()

	for _, ver := range []string{"1", "latest", "v", "v1.0", "V1"} {
		_, err := parsePolicyListJSON([]byte(`{"policies":[{"name":"p","version":"` + ver + `"}]}`))
		require.Error(t, err, "version %q", ver)
		require.True(t, IsInvalidHTTPRouteConfigError(err), err.Error())
		require.Contains(t, err.Error(), "invalid version format")
	}

	spec, err := parsePolicyListJSON([]byte(`{"policies":[{"name":"p","version":"v1"}]}`))
	require.NoError(t, err)
	require.Len(t, spec, 1)
	require.Equal(t, "v1", spec[0].Version)
}

func TestBuildAPIConfigFromHTTPRoute_BackendRefsMustResolveToSingleService(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svcA := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-a", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	svcB := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-b", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svcA, svcB).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/api"
	method := gatewayv1.HTTPMethodGet

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName("backend-a"),
							Port: ptrPort(8080),
						}}},
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName("backend-b"),
							Port: ptrPort(8080),
						}}},
					},
				},
			},
		},
	}

	_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
	require.Error(t, err)
	require.True(t, IsInvalidHTTPRouteConfigError(err))
	require.Contains(t, err.Error(), "single Service backend")
}

func TestBuildAPIConfigFromHTTPRoute_ServiceBackendRejectsNonCoreGroup(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/api"
	method := gatewayv1.HTTPMethodGet
	nonCore := gatewayv1.Group("discovery.k8s.io")

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{
					{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{{
					BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
						Group: &nonCore,
						Name:  gatewayv1.ObjectName("backend"),
						Port:  ptrPort(8080),
					}},
				}},
			}},
		},
	}

	_, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "cluster.local", nil)
	require.Error(t, err)
	require.True(t, IsTransientHTTPRouteConfigError(err), err.Error())
	require.Contains(t, err.Error(), "unsupported backendRef")
}

func TestBuildAPIConfigFromHTTPRoute_CustomClusterDomain(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/api/hello"
	method := gatewayv1.HTTPMethodGet

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName("backend"),
							Port: ptrPort(8080),
						}}},
					},
				},
			},
		},
	}

	spec, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, "example.k8s.local", nil)
	require.NoError(t, err)
	require.Equal(t, "http://backend.default.svc.example.k8s.local:8080", spec.Upstream.Main.Url)

	spec2, err := BuildAPIConfigFromHTTPRoute(context.Background(), cl, route, ".cluster.local.", nil)
	require.NoError(t, err)
	require.Equal(t, "http://backend.default.svc.cluster.local:8080", spec2.Upstream.Main.Url)
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
