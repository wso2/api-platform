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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

func resolverTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	return s
}

func resolvePolicyWithParams(t *testing.T, cl client.Client, defaultNS string, params map[string]any) (*apiv1.Policy, error) {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	pol := apiv1.Policy{
		Name:    "subscription-validation",
		Version: "v1",
		Params:  &runtime.RawExtension{Raw: raw},
	}
	err = resolvePolicyParamsValueFrom(context.Background(), cl, defaultNS, &pol, "test", nil)
	return &pol, err
}

func TestResolvePolicyParamsValueFrom_SecretKeyRef(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "gateway-api-demo", Name: "demo-creds"},
		Data:       map[string][]byte{"subscriptionKey": []byte("Subscription-Key")},
	}).Build()

	pol, err := resolvePolicyWithParams(t, cl, "gateway-api-demo", map[string]any{
		"subscriptionKeyHeader": map[string]any{
			"valueFrom": map[string]any{
				"secretKeyRef": map[string]any{
					"name": "demo-creds",
					"key":  "subscriptionKey",
				},
			},
		},
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(pol.Params.Raw, &out))
	require.Equal(t, "Subscription-Key", out["subscriptionKeyHeader"])
}

func TestResolvePolicyParamsValueFrom_ConfigMapKeyRef_Data(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "gateway-api-demo", Name: "demo-config"},
		Data:       map[string]string{"region": "us-east-1"},
	}).Build()

	pol, err := resolvePolicyWithParams(t, cl, "gateway-api-demo", map[string]any{
		"region": map[string]any{
			"valueFrom": map[string]any{
				"configMapKeyRef": map[string]any{
					"name": "demo-config",
					"key":  "region",
				},
			},
		},
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(pol.Params.Raw, &out))
	require.Equal(t, "us-east-1", out["region"])
}

func TestResolvePolicyParamsValueFrom_ConfigMapKeyRef_BinaryData(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "gateway-api-demo", Name: "demo-config"},
		BinaryData: map[string][]byte{"models.json": []byte(`{"foo":1}`)},
	}).Build()

	pol, err := resolvePolicyWithParams(t, cl, "gateway-api-demo", map[string]any{
		"modelMap": map[string]any{
			"valueFrom": map[string]any{
				"configMapKeyRef": map[string]any{
					"name": "demo-config",
					"key":  "models.json",
				},
			},
		},
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(pol.Params.Raw, &out))
	require.Equal(t, `{"foo":1}`, out["modelMap"])
}

func TestResolvePolicyParamsValueFrom_CrossNamespace(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "secrets-ns", Name: "demo-creds"},
		Data:       map[string][]byte{"subscriptionKey": []byte("xyz")},
	}).Build()

	pol, err := resolvePolicyWithParams(t, cl, "gateway-api-demo", map[string]any{
		"subscriptionKeyHeader": map[string]any{
			"valueFrom": map[string]any{
				"secretKeyRef": map[string]any{
					"name":      "demo-creds",
					"key":       "subscriptionKey",
					"namespace": "secrets-ns",
				},
			},
		},
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(pol.Params.Raw, &out))
	require.Equal(t, "xyz", out["subscriptionKeyHeader"])
}

func TestResolvePolicyParamsValueFrom_LeavesInlineUnchanged(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).Build()
	raw, err := json.Marshal(map[string]any{"subscriptionKeyHeader": "Subscription-Key"})
	require.NoError(t, err)
	pol := apiv1.Policy{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}
	require.NoError(t, resolvePolicyParamsValueFrom(context.Background(), cl, "ns", &pol, "test", nil))
	require.JSONEq(t, string(raw), string(pol.Params.Raw))
}

func TestResolvePolicyParamsValueFrom_ErrorCases(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).Build()

	cases := []struct {
		name     string
		params   map[string]any
		contains string
	}{
		{
			name: "empty valueFrom",
			params: map[string]any{
				"k": map[string]any{"valueFrom": map[string]any{}},
			},
			contains: "requires one of",
		},
		{
			name: "both refs set",
			params: map[string]any{
				"k": map[string]any{"valueFrom": map[string]any{
					"secretKeyRef":    map[string]any{"name": "s", "key": "k"},
					"configMapKeyRef": map[string]any{"name": "c", "key": "k"},
				}},
			},
			contains: "exactly one",
		},
		{
			name: "unknown sibling",
			params: map[string]any{
				"k": map[string]any{"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"name": "s", "key": "k"},
					"extra":        "nope",
				}},
			},
			contains: "unknown valueFrom field",
		},
		{
			name: "missing name",
			params: map[string]any{
				"k": map[string]any{"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"key": "k"},
				}},
			},
			contains: `non-empty "name"`,
		},
		{
			name: "missing key",
			params: map[string]any{
				"k": map[string]any{"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"name": "s"},
				}},
			},
			contains: `non-empty "name" and "key"`,
		},
		{
			name: "unknown field on ref",
			params: map[string]any{
				"k": map[string]any{"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"name": "s", "key": "k", "typo": "x"},
				}},
			},
			contains: `unknown field "typo"`,
		},
		{
			name: "non-object valueFrom",
			params: map[string]any{
				"k": map[string]any{"valueFrom": "nope"},
			},
			contains: "must be an object",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolvePolicyWithParams(t, cl, "gateway-api-demo", tc.params)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.contains)
		})
	}
}

func TestResolvePolicyParamsValueFrom_MissingResourceIsTransient(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(resolverTestScheme(t)).Build()

	_, err := resolvePolicyWithParams(t, cl, "gateway-api-demo", map[string]any{
		"k": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "missing", "key": "k"},
		}},
	})
	require.Error(t, err)
	require.True(t, IsTransientHTTPRouteConfigError(err), "Secret miss should be transient, got %v", err)

	_, err = resolvePolicyWithParams(t, cl, "gateway-api-demo", map[string]any{
		"k": map[string]any{"valueFrom": map[string]any{
			"configMapKeyRef": map[string]any{"name": "missing", "key": "k"},
		}},
	})
	require.Error(t, err)
	require.True(t, IsTransientHTTPRouteConfigError(err), "ConfigMap miss should be transient, got %v", err)
}
