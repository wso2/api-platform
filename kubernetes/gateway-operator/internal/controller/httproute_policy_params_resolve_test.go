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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolvePolicyParamsSecrets_ValueFrom(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"subscriptionKeyHeader": map[string]any{
			"valueFrom": map[string]any{"name": "demo-creds", "valueKey": "subscriptionKey"},
		},
	})
	require.NoError(t, err)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "gateway-api-demo", Name: "demo-creds"},
		Data:       map[string][]byte{"subscriptionKey": []byte("Subscription-Key")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()

	pol := apiv1.Policy{
		Name:    "subscription-validation",
		Version: "v1",
		Params:  &runtime.RawExtension{Raw: raw},
	}
	require.NoError(t, resolvePolicyParamsSecrets(context.Background(), cl, "gateway-api-demo", &pol, "test", nil))

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(pol.Params.Raw, &out))
	require.Equal(t, "Subscription-Key", out["subscriptionKeyHeader"])
}

func TestResolvePolicyParamsSecrets_LeavesInlineUnchanged(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	raw, err := json.Marshal(map[string]any{
		"subscriptionKeyHeader": "Subscription-Key",
	})
	require.NoError(t, err)
	pol := apiv1.Policy{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}
	require.NoError(t, resolvePolicyParamsSecrets(context.Background(), cl, "ns", &pol, "test", nil))
	require.JSONEq(t, string(raw), string(pol.Params.Raw))
}
