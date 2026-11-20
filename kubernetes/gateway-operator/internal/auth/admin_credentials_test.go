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

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const testNS = "gw1"

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func gatewayWithAdminCredentials(username, secretName, key string) *apiv1.APIGateway {
	return &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: testNS},
		Spec: apiv1.GatewaySpec{
			AdminCredentials: &apiv1.GatewayAdminCredentials{
				Username: username,
				PasswordSecretRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  key,
				},
				Source: apiv1.AdminCredentialsSourceFile,
			},
		},
	}
}

func passwordSecret(name string, data map[string]string) *corev1.Secret {
	s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS}, Data: map[string][]byte{}}
	for k, v := range data {
		s.Data[k] = []byte(v)
	}
	return s
}

func info() *registry.GatewayInfo {
	return &registry.GatewayInfo{Name: "gateway-1", Namespace: testNS}
}

func TestResolveBasicCredentials_FromSecret(t *testing.T) {
	gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
	c := newFakeClient(t, gw, passwordSecret("gateway-1-admin", map[string]string{"password": "s3cret-1"}))

	user, pass, err := ResolveBasicCredentials(context.Background(), c, info())
	require.NoError(t, err)
	assert.Equal(t, "gw1-admin", user)
	assert.Equal(t, "s3cret-1", pass)
}

func TestResolveBasicCredentials_DefaultUsername(t *testing.T) {
	gw := gatewayWithAdminCredentials("", "gateway-1-admin", "password")
	c := newFakeClient(t, gw, passwordSecret("gateway-1-admin", map[string]string{"password": "s3cret-1"}))

	user, _, err := ResolveBasicCredentials(context.Background(), c, info())
	require.NoError(t, err)
	assert.Equal(t, DefaultAdminUsername, user)
}

// With adminCredentials set, an unusable Secret is an error: the operator must never
// fall back to admin/admin for such a gateway.
func TestResolveBasicCredentials_FailsClosed(t *testing.T) {
	cases := map[string]struct {
		objs []client.Object
	}{
		"secret missing": {objs: nil},
		"key missing":    {objs: []client.Object{passwordSecret("gateway-1-admin", map[string]string{"other": "x"})}},
		"value empty":    {objs: []client.Object{passwordSecret("gateway-1-admin", map[string]string{"password": ""})}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
			c := newFakeClient(t, append([]client.Object{gw}, tc.objs...)...)

			user, pass, err := ResolveBasicCredentials(context.Background(), c, info())
			require.Error(t, err)
			assert.Empty(t, user)
			assert.Empty(t, pass)
		})
	}
}

// An optional selector must not turn a missing Secret into an empty password.
func TestResolveBasicCredentials_OptionalSelectorStillFailsClosed(t *testing.T) {
	gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
	optional := true
	gw.Spec.AdminCredentials.PasswordSecretRef.Optional = &optional
	c := newFakeClient(t, gw)

	_, _, err := ResolveBasicCredentials(context.Background(), c, info())
	require.Error(t, err)
}

func TestResolveBasicCredentials_ErrorDoesNotLeakPassword(t *testing.T) {
	gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
	c := newFakeClient(t, gw, passwordSecret("gateway-1-admin", map[string]string{"pw": "do-not-print"}))

	_, _, err := ResolveBasicCredentials(context.Background(), c, info())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "do-not-print")
}

// Gateways without adminCredentials keep the earlier behaviour.
func TestResolveBasicCredentials_LegacyConfigMap(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: testNS},
		Spec:       apiv1.GatewaySpec{ConfigRef: &corev1.LocalObjectReference{Name: "gw-values"}},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-values", Namespace: testNS},
		Data: map[string]string{"values.yaml": `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: legacy
              password: legacy-pw
`},
	}
	c := newFakeClient(t, gw, cm)

	user, pass, err := ResolveBasicCredentials(context.Background(), c, info())
	require.NoError(t, err)
	assert.Equal(t, "legacy", user)
	assert.Equal(t, "legacy-pw", pass)
}

func TestResolveBasicCredentials_LegacyDefault(t *testing.T) {
	gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: testNS}}
	c := newFakeClient(t, gw)

	user, pass, err := ResolveBasicCredentials(context.Background(), c, info())
	require.NoError(t, err)
	assert.Equal(t, "admin", user)
	assert.Equal(t, "admin", pass)

	// Nil info (no registered gateway) also keeps the default.
	user, pass, err = ResolveBasicCredentials(context.Background(), c, nil)
	require.NoError(t, err)
	assert.Equal(t, "admin", user)
	assert.Equal(t, "admin", pass)
}
