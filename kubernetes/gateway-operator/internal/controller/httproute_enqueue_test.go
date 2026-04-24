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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestEnqueueHTTPRouteForAPIPolicy(t *testing.T) {
	r := &HTTPRouteReconciler{Client: fake.NewClientBuilder().Build()}
	ctx := context.Background()

	t.Run("maps target HTTPRoute", func(t *testing.T) {
		ap := &apiv1.APIPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol"},
			Spec: apiv1.APIPolicySpec{
				TargetRef: &apiv1.APIPolicyTargetRef{
					Group: gatewayv1.GroupName,
					Kind:  "HTTPRoute",
					Name:  "route-a",
				},
				Policies: []apiv1.Policy{{Name: "p", Version: "v1"}},
			},
		}
		reqs := r.enqueueHTTPRouteForAPIPolicy(ctx, ap)
		require.Len(t, reqs, 1)
		require.Equal(t, types.NamespacedName{Namespace: "ns1", Name: "route-a"}, reqs[0].NamespacedName)
	})

	t.Run("uses targetRef namespace when set", func(t *testing.T) {
		other := "other-ns"
		ap := &apiv1.APIPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol"},
			Spec: apiv1.APIPolicySpec{
				TargetRef: &apiv1.APIPolicyTargetRef{
					Group:     gatewayv1.GroupName,
					Kind:      "HTTPRoute",
					Name:      "route-a",
					Namespace: &other,
				},
				Policies: []apiv1.Policy{{Name: "p", Version: "v1"}},
			},
		}
		reqs := r.enqueueHTTPRouteForAPIPolicy(ctx, ap)
		require.Len(t, reqs, 1)
		require.Equal(t, types.NamespacedName{Namespace: "other-ns", Name: "route-a"}, reqs[0].NamespacedName)
	})

	t.Run("ignores wrong group", func(t *testing.T) {
		ap := &apiv1.APIPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol"},
			Spec: apiv1.APIPolicySpec{
				TargetRef: &apiv1.APIPolicyTargetRef{
					Group: "wrong.group",
					Kind:  "HTTPRoute",
					Name:  "route-a",
				},
				Policies: []apiv1.Policy{{Name: "p", Version: "v1"}},
			},
		}
		require.Empty(t, r.enqueueHTTPRouteForAPIPolicy(ctx, ap))
	})

	t.Run("ExtensionRef-only policy enqueues referencing HTTPRoutes", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(gatewayv1.AddToScheme(scheme))
		utilruntime.Must(apiv1.AddToScheme(scheme))
		ft := gatewayv1.HTTPRouteFilterExtensionRef
		g := gatewayv1.Group(apiv1.GroupVersion.Group)
		k := gatewayv1.Kind("APIPolicy")
		n := gatewayv1.ObjectName("rule-only-pol")
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "hr-z"},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{{
					Filters: []gatewayv1.HTTPRouteFilter{
						{Type: ft, ExtensionRef: &gatewayv1.LocalObjectReference{Group: g, Kind: k, Name: n}},
					},
				}},
			},
		}
		ap := &apiv1.APIPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "rule-only-pol"},
			Spec: apiv1.APIPolicySpec{
				Policies: []apiv1.Policy{{Name: "p", Version: "v1"}},
			},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(route, ap).Build()
		rec := &HTTPRouteReconciler{Client: cl}
		reqs := rec.enqueueHTTPRouteForAPIPolicy(ctx, ap)
		require.Len(t, reqs, 1)
		require.Equal(t, types.NamespacedName{Namespace: "ns1", Name: "hr-z"}, reqs[0].NamespacedName)
	})
}

func TestAPIPolicyReferencesValueFrom_Secret(t *testing.T) {
	t.Run("nested secretKeyRef same namespace", func(t *testing.T) {
		raw, err := json.Marshal(map[string]any{
			"apiKey": map[string]any{
				"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"name": "s1", "key": "k"},
				},
			},
		})
		require.NoError(t, err)
		ap := &apiv1.APIPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol"},
			Spec: apiv1.APIPolicySpec{
				Policies: []apiv1.Policy{{
					Name: "p", Version: "v1",
					Params: &runtime.RawExtension{Raw: raw},
				}},
			},
		}
		require.True(t, apiPolicyReferencesValueFrom(ap, secretKeyRefKey, "ns1", "s1"))
		require.False(t, apiPolicyReferencesValueFrom(ap, secretKeyRefKey, "ns1", "other"))
		require.False(t, apiPolicyReferencesValueFrom(ap, secretKeyRefKey, "ns2", "s1"))
		require.False(t, apiPolicyReferencesValueFrom(ap, configMapKeyRefKey, "ns1", "s1"))
	})

	t.Run("secretKeyRef with explicit namespace", func(t *testing.T) {
		raw, err := json.Marshal(map[string]any{
			"x": map[string]any{
				"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"name": "s1", "namespace": "sec-ns", "key": "k"},
				},
			},
		})
		require.NoError(t, err)
		ap := &apiv1.APIPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol"},
			Spec: apiv1.APIPolicySpec{
				Policies: []apiv1.Policy{{
					Name: "p", Version: "v1",
					Params: &runtime.RawExtension{Raw: raw},
				}},
			},
		}
		require.True(t, apiPolicyReferencesValueFrom(ap, secretKeyRefKey, "sec-ns", "s1"))
		require.False(t, apiPolicyReferencesValueFrom(ap, secretKeyRefKey, "ns1", "s1"))
	})
}

func TestAPIPolicyReferencesValueFrom_ConfigMap(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"region": map[string]any{
			"valueFrom": map[string]any{
				"configMapKeyRef": map[string]any{"name": "cm1", "key": "region"},
			},
		},
	})
	require.NoError(t, err)
	ap := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol"},
		Spec: apiv1.APIPolicySpec{
			Policies: []apiv1.Policy{{
				Name: "p", Version: "v1",
				Params: &runtime.RawExtension{Raw: raw},
			}},
		},
	}
	require.True(t, apiPolicyReferencesValueFrom(ap, configMapKeyRefKey, "ns1", "cm1"))
	require.False(t, apiPolicyReferencesValueFrom(ap, configMapKeyRefKey, "ns1", "other"))
	require.False(t, apiPolicyReferencesValueFrom(ap, secretKeyRefKey, "ns1", "cm1"))
}

func TestEnqueueHTTPRoutesForSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"k": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "my-secret", "key": "x"},
		}},
	})
	require.NoError(t, err)
	ap := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "pol-with-secret"},
		Spec: apiv1.APIPolicySpec{
			TargetRef: &apiv1.APIPolicyTargetRef{
				Group: gatewayv1.GroupName,
				Kind:  "HTTPRoute",
				Name:  "hr1",
			},
			Policies: []apiv1.Policy{{
				Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw},
			}},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "my-secret"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ap, secret).Build()
	r := &HTTPRouteReconciler{Client: cl}
	reqs := r.enqueueHTTPRoutesForSecret(context.Background(), secret)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "demo", Name: "hr1"}, reqs[0].NamespacedName)
}

func TestEnqueueHTTPRoutesForSecret_ExtensionRefOnlyAPIPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"k": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "my-secret", "key": "x"},
		}},
	})
	require.NoError(t, err)
	ap := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "pol-ext-secret"},
		Spec: apiv1.APIPolicySpec{
			Policies: []apiv1.Policy{{
				Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw},
			}},
		},
	}
	ft := gatewayv1.HTTPRouteFilterExtensionRef
	g := gatewayv1.Group(apiv1.GroupVersion.Group)
	k := gatewayv1.Kind("APIPolicy")
	n := gatewayv1.ObjectName("pol-ext-secret")
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "hr-ext"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Filters: []gatewayv1.HTTPRouteFilter{
					{Type: ft, ExtensionRef: &gatewayv1.LocalObjectReference{Group: g, Kind: k, Name: n}},
				},
			}},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "my-secret"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ap, secret, route).Build()
	r := &HTTPRouteReconciler{Client: cl}
	reqs := r.enqueueHTTPRoutesForSecret(context.Background(), secret)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "demo", Name: "hr-ext"}, reqs[0].NamespacedName)
}

func TestEnqueueHTTPRoutesForConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"region": map[string]any{"valueFrom": map[string]any{
			"configMapKeyRef": map[string]any{"name": "my-cm", "key": "region"},
		}},
	})
	require.NoError(t, err)
	ap := &apiv1.APIPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "pol-with-cm"},
		Spec: apiv1.APIPolicySpec{
			TargetRef: &apiv1.APIPolicyTargetRef{
				Group: gatewayv1.GroupName,
				Kind:  "HTTPRoute",
				Name:  "hr-cm",
			},
			Policies: []apiv1.Policy{{
				Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw},
			}},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "my-cm"},
		Data:       map[string]string{"region": "us-east-1"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ap, cm).Build()
	r := &HTTPRouteReconciler{Client: cl}
	reqs := r.enqueueHTTPRoutesForConfigMap(context.Background(), cm)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "demo", Name: "hr-cm"}, reqs[0].NamespacedName)

	// Secret watch on the same name must not fire for a ConfigMap reference.
	secretLookalike := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "my-cm"}}
	require.Empty(t, r.enqueueHTTPRoutesForSecret(context.Background(), secretLookalike))
}
