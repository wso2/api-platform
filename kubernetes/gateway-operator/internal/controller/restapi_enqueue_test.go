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
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnqueueRestApisForSecret_TargetRef(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"h": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "credential", "key": "x"},
		}},
	})
	require.NoError(t, err)
	api := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "api-with-secret-ref"},
		Spec: apiv1.APIConfigData{
			Policies: []apiv1.Policy{{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}},
		},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "credential"}}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(api, sec).Build()
	r := &RestApiReconciler{Client: cl}

	reqs := r.enqueueRestApisForSecret(context.Background(), sec)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "demo", Name: "api-with-secret-ref"}, reqs[0].NamespacedName)
}

func TestEnqueueRestApisForConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"region": map[string]any{"valueFrom": map[string]any{
			"configMapKeyRef": map[string]any{"name": "regions", "key": "preferred"},
		}},
	})
	require.NoError(t, err)

	api := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Namespace: "east", Name: "cm-api"},
		Spec: apiv1.APIConfigData{
			Policies: []apiv1.Policy{{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "east", Name: "regions"},
		Data:       map[string]string{"preferred": "us-east-2"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(api, cm).Build()
	r := &RestApiReconciler{Client: cl}

	cmEvent := cm
	reqs := r.enqueueRestApisForConfigMap(context.Background(), cmEvent)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "east", Name: "cm-api"}, reqs[0].NamespacedName)

	secretLookalike := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "east", Name: "regions"}}
	require.Empty(t, r.enqueueRestApisForSecret(context.Background(), secretLookalike))
}

func TestEnqueueRestApisForSecret_ExplicitRemoteNamespaceRef(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"x": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "credential", "key": "k", "namespace": "secrets-ns"},
		}},
	})
	require.NoError(t, err)

	api := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Namespace: "staging", Name: "remote-secret-api"},
		Spec: apiv1.APIConfigData{
			Policies: []apiv1.Policy{{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}},
		},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "secrets-ns", Name: "credential"},
		Data:       map[string][]byte{"k": {'1'}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(api, sec).Build()
	r := &RestApiReconciler{Client: cl}

	reqs := r.enqueueRestApisForSecret(context.Background(), sec)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "staging", Name: "remote-secret-api"}, reqs[0].NamespacedName)
}

func TestEnqueueRestApisForSecret_UsesIndexCandidates(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"h": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "credential", "key": "x"},
		}},
	})
	require.NoError(t, err)

	api := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "indexed-api"},
		Spec: apiv1.APIConfigData{
			Policies: []apiv1.Policy{{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw}}},
		},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "credential"}}

	// Do not add RestApi object to the client: successful enqueue proves the
	// reverse-reference index path is used (instead of list+scan).
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()
	r := &RestApiReconciler{Client: cl}
	r.upsertRestAPIValueFromIndex(api)

	reqs := r.enqueueRestApisForSecret(context.Background(), sec)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "demo", Name: "indexed-api"}, reqs[0].NamespacedName)
}

func TestUpsertRestAPIValueFromIndex_ReplacesOldRefs(t *testing.T) {
	raw1, err := json.Marshal(map[string]any{
		"h": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "credential-1", "key": "x"},
		}},
	})
	require.NoError(t, err)
	raw2, err := json.Marshal(map[string]any{
		"h": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "credential-2", "key": "x"},
		}},
	})
	require.NoError(t, err)

	api := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "indexed-api"},
		Spec: apiv1.APIConfigData{
			Policies: []apiv1.Policy{{Name: "p", Version: "v1", Params: &runtime.RawExtension{Raw: raw1}}},
		},
	}

	r := &RestApiReconciler{}
	r.upsertRestAPIValueFromIndex(api)

	_, oldFound := r.enqueueRestApisFromValueFromIndex(secretKeyRefKey, "demo", "credential-1")
	require.True(t, oldFound)

	api.Spec.Policies[0].Params = &runtime.RawExtension{Raw: raw2}
	r.upsertRestAPIValueFromIndex(api)

	_, oldFound = r.enqueueRestApisFromValueFromIndex(secretKeyRefKey, "demo", "credential-1")
	require.False(t, oldFound)

	reqs, newFound := r.enqueueRestApisFromValueFromIndex(secretKeyRefKey, "demo", "credential-2")
	require.True(t, newFound)
	require.Len(t, reqs, 1)
	require.Equal(t, types.NamespacedName{Namespace: "demo", Name: "indexed-api"}, reqs[0].NamespacedName)
}
