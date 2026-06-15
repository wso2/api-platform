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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func buildAPIConfigFromHTTPRouteForTest(ctx context.Context, cl client.Client, route *gatewayv1.HTTPRoute, clusterDomain string) (*apiv1.APIConfigData, error) {
	resolution := resolveHTTPRouteBackendRefs(ctx, cl, route, clusterDomain)
	return BuildAPIConfigFromHTTPRoute(ctx, cl, nil, route, parentGatewayRefs(route), resolution, clusterDomain, nil)
}

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

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.Equal(t, "/api", spec.Context)
	require.Equal(t, "my-route", spec.DisplayName)
	require.Equal(t, "v1.0", spec.Version)
	require.Len(t, spec.Operations, 1)
	require.Equal(t, "/api/hello/*", spec.Operations[0].Path)
	require.Equal(t, strPtr("http://backend.default.svc.cluster.local:8080"), spec.Upstream.Main.Url)
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

	t.Run("whitespace-only treated as unset => default context /", func(t *testing.T) {
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

		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		require.Equal(t, "/", spec.Context)
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

		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
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

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
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

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.Len(t, spec.Operations, 1)
	require.Equal(t, apiv1.OperationMethodPOST, spec.Operations[0].Method)
	require.Equal(t, "/r/*", spec.Operations[0].Path)
	require.Len(t, spec.Operations[0].Policies, 1)
	require.Equal(t, "ext-ref-policy", spec.Operations[0].Policies[0].Name)
}

func TestBuildAPIConfigFromHTTPRoute_MatchMethodOptionalAndEmptyMatchesRejected(t *testing.T) {
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
		_, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.Error(t, err)
		require.True(t, IsInvalidHTTPRouteConfigError(err))
		require.Contains(t, err.Error(), "rule[0]")
		require.Contains(t, err.Error(), "no matches")
	})

	t.Run("omitted match method expands to all RestApi operation methods", func(t *testing.T) {
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
		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		require.Len(t, spec.Operations, len(allRESTAPIOperationMethods()))
		want := allRESTAPIOperationMethods()
		for i := range want {
			require.Equal(t, want[i], spec.Operations[i].Method)
			require.Equal(t, "/x/*", spec.Operations[i].Path)
		}
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

	_, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl2, route, "cluster.local")
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
		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		require.NotNil(t, spec.Operations[0].DirectResponse)
		require.Equal(t, 500, spec.Operations[0].DirectResponse.StatusCode)
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
		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		require.Equal(t, strPtr("http://backend.data.svc.cluster.local:8080"), spec.Upstream.Main.Url)
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
		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		require.NotNil(t, spec.Operations[0].DirectResponse)
		require.Equal(t, 500, spec.Operations[0].DirectResponse.StatusCode)
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
		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		require.NotNil(t, spec.Operations[0].DirectResponse)
		require.Equal(t, 500, spec.Operations[0].DirectResponse.StatusCode)
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

func TestBuildAPIConfigFromHTTPRoute_WeightedBackendsInSingleRule(t *testing.T) {
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

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.Len(t, spec.UpstreamDefinitions, 1)
	require.Equal(t, "rule-0-weighted", spec.UpstreamDefinitions[0].Name)
	require.Len(t, spec.UpstreamDefinitions[0].Upstreams, 2)

	// The operation's dynamic-endpoint policy must target the weighted definition
	// ("rule-0-weighted"), NOT a per-service name. Otherwise cluster_header routing
	// points x-target-upstream at a cluster that is never created -> 503 (HTTPRouteWeight).
	require.Len(t, spec.Operations, 1)
	var deTarget string
	for _, p := range spec.Operations[0].Policies {
		if p.Name == "dynamic-endpoint" {
			require.NotNil(t, p.Params)
			var params map[string]interface{}
			require.NoError(t, json.Unmarshal(p.Params.Raw, &params))
			deTarget, _ = params["targetUpstream"].(string)
		}
	}
	require.Equal(t, "rule-0-weighted", deTarget,
		"weighted rule's dynamic-endpoint policy must target the weighted upstream definition")
}

// TestBuildAPIConfigFromHTTPRoute_WeightedBackendsExcludesZeroWeight guards the fix for
// HTTPRouteWeight: a weight-0 backend must be omitted from the weighted definition so it
// receives NO traffic (Envoy's minimum load_balancing_weight is 1, so leaving it in would
// still route ~1/(sum) of requests there and the conformance check counts distinct backends).
func TestBuildAPIConfigFromHTTPRoute_WeightedBackendsExcludesZeroWeight(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	mkSvc := func(name string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
		}
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(mkSvc("backend-a"), mkSvc("backend-b"), mkSvc("backend-c")).Build()

	pathMatch := gatewayv1.PathMatchPathPrefix
	pathVal := "/"
	method := gatewayv1.HTTPMethodGet
	w70 := int32(70)
	w30 := int32(30)
	w0 := int32(0)

	mkRef := func(name string, w *int32) gatewayv1.HTTPBackendRef {
		return gatewayv1.HTTPBackendRef{BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name: gatewayv1.ObjectName(name), Port: ptrPort(8080),
			},
			Weight: w,
		}}
	}

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{
					{Path: &gatewayv1.HTTPPathMatch{Type: &pathMatch, Value: &pathVal}, Method: &method},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{
					mkRef("backend-a", &w70),
					mkRef("backend-b", &w30),
					mkRef("backend-c", &w0),
				},
			}},
		},
	}

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.Len(t, spec.UpstreamDefinitions, 1)
	require.Equal(t, "rule-0-weighted", spec.UpstreamDefinitions[0].Name)
	// backend-c (weight 0) must be excluded; only the two positive-weight backends remain.
	require.Len(t, spec.UpstreamDefinitions[0].Upstreams, 2,
		"weight-0 backend must be excluded from the weighted upstream definition")
	for _, up := range spec.UpstreamDefinitions[0].Upstreams {
		require.NotContains(t, up.Url, "backend-c", "weight-0 backend must not appear as an endpoint")
		require.NotNil(t, up.Weight)
		require.Greater(t, *up.Weight, 0)
	}
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

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.NotNil(t, spec.Operations[0].DirectResponse)
	require.Equal(t, 500, spec.Operations[0].DirectResponse.StatusCode)
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

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "example.k8s.local")
	require.NoError(t, err)
	require.Equal(t, strPtr("http://backend.default.svc.example.k8s.local:8080"), spec.Upstream.Main.Url)

	spec2, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, ".cluster.local.")
	require.NoError(t, err)
	require.Equal(t, strPtr("http://backend.default.svc.cluster.local:8080"), spec2.Upstream.Main.Url)
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
