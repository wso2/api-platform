/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the Apache License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"

	"github.com/stretchr/testify/require"
)

func TestApiKeyAdapterNeedsRedeployForSecretDrift(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	secret := func(rv string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "demo-management-secrets",
				Namespace:       "demo",
				ResourceVersion: rv,
			},
			Data: map[string][]byte{"apikey-restapi-value": []byte("x")},
		}
	}
	cr := func(ann map[string]string) *apiv1.ApiKey {
		return &apiv1.ApiKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "demo-restapi-apikey",
				Namespace:   "demo",
				Annotations: ann,
			},
			Spec: apiv1.ApiKeySpec{
				ParentRef: apiv1.ApiKeyParentRef{Kind: "RestApi", Name: "api"},
				ApiKey: &apiv1.SecretValueSource{
					ValueFrom: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "demo-management-secrets"},
						Key:                  "apikey-restapi-value",
					},
				},
			},
		}
	}
	ad := &apiKeyAdapter{}

	c1 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret("100"), cr(nil)).Build()
	need1, err := ad.needsRedeployForExternalDeps(ctx, c1, cr(nil))
	require.NoError(t, err)
	require.True(t, need1, "missing annotation implies drift vs gateway")

	c2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret("100"), cr(map[string]string{
		apiKeySecretBindingFingerprintAnn: "demo-management-secrets/apikey-restapi-value@99",
	})).Build()
	need2, err := ad.needsRedeployForExternalDeps(ctx, c2, cr(map[string]string{
		apiKeySecretBindingFingerprintAnn: "demo-management-secrets/apikey-restapi-value@99",
	}))
	require.NoError(t, err)
	require.True(t, need2, "stale resourceVersion fingerprint")

	c3 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret("100"), cr(map[string]string{
		apiKeySecretBindingFingerprintAnn: "demo-management-secrets/apikey-restapi-value@100",
	})).Build()
	need3, err := ad.needsRedeployForExternalDeps(ctx, c3, cr(map[string]string{
		apiKeySecretBindingFingerprintAnn: "demo-management-secrets/apikey-restapi-value@100",
	}))
	require.NoError(t, err)
	require.False(t, need3)
}
