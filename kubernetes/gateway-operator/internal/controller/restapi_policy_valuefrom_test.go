/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestComputeRestApiPolicyValueFromFingerprint_Empty(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	spec := &apiv1.APIConfigData{}
	fp, err := computeRestApiPolicyValueFromFingerprint(context.Background(), cl, "default", spec)
	require.NoError(t, err)
	require.Equal(t, "", fp)
}

func TestComputeRestApiPolicyValueFromFingerprint_ChangesWithSecretRV(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"k": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "s1", "key": "x"},
		}},
	})
	require.NoError(t, err)
	spec := &apiv1.APIConfigData{
		Policies: []apiv1.Policy{{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}},
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "s1", ResourceVersion: "100"},
		Data:       map[string][]byte{"x": []byte("a")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()

	fp1, err := computeRestApiPolicyValueFromFingerprint(context.Background(), cl, "ns1", spec)
	require.NoError(t, err)
	require.Equal(t, "secret:ns1/s1@100", fp1)

	sec2 := sec.DeepCopy()
	sec2.ResourceVersion = "101"
	cl2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec2).Build()
	fp2, err := computeRestApiPolicyValueFromFingerprint(context.Background(), cl2, "ns1", spec)
	require.NoError(t, err)
	require.Equal(t, "secret:ns1/s1@101", fp2)
}
