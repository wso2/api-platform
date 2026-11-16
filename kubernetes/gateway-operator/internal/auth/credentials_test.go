/*
Copyright 2025.

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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const testNamespace = "gw-ns"

// A bcrypt-shaped placeholder; the operator never verifies hashes, it only skips them.
const testHash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func passwordSecret(namespace, name, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data:       map[string][]byte{key: []byte(value)},
	}
}

func valuesConfigMap(name, values string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: name},
		Data:       map[string]string{"values.yaml": values},
	}
}

func TestResolveBasicAuthCredentials(t *testing.T) {
	ctx := context.Background()
	enabled := func(users ...AuthUser) *AuthSettings {
		return &AuthSettings{Basic: BasicAuthConfig{Enabled: true, Users: users}}
	}

	t.Run("plaintext password", func(t *testing.T) {
		u, p, err := ResolveBasicAuthCredentials(ctx, nil, testNamespace, enabled(AuthUser{Username: "ops", Password: "s3cret"}))
		require.NoError(t, err)
		assert.Equal(t, "ops", u)
		assert.Equal(t, "s3cret", p)
	})

	t.Run("hashed password read from Secret with default key", func(t *testing.T) {
		c := newFakeClient(t, passwordSecret(testNamespace, "gw-admin", DefaultPasswordSecretKey, "from-secret"))
		u, p, err := ResolveBasicAuthCredentials(ctx, c, testNamespace, enabled(AuthUser{
			Username: "ops", Password: testHash, PasswordHashed: true,
			PasswordSecretRef: &SecretKeyRef{Name: "gw-admin"},
		}))
		require.NoError(t, err)
		assert.Equal(t, "ops", u)
		assert.Equal(t, "from-secret", p)
	})

	t.Run("Secret with custom key", func(t *testing.T) {
		c := newFakeClient(t, passwordSecret(testNamespace, "gw-admin", "adminPassword", "custom"))
		_, p, err := ResolveBasicAuthCredentials(ctx, c, testNamespace, enabled(AuthUser{
			Username: "ops", Password: testHash, PasswordHashed: true,
			PasswordSecretRef: &SecretKeyRef{Name: "gw-admin", Key: "adminPassword"},
		}))
		require.NoError(t, err)
		assert.Equal(t, "custom", p)
	})

	t.Run("Secret is only looked up in the gateway namespace", func(t *testing.T) {
		c := newFakeClient(t, passwordSecret("other-ns", "gw-admin", DefaultPasswordSecretKey, "x"))
		_, _, err := ResolveBasicAuthCredentials(ctx, c, testNamespace, enabled(AuthUser{
			Username: "ops", Password: testHash, PasswordHashed: true,
			PasswordSecretRef: &SecretKeyRef{Name: "gw-admin"},
		}))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNoControllerCredentials)
		assert.Contains(t, err.Error(), "gw-ns/gw-admin")
	})

	t.Run("Secret without the key", func(t *testing.T) {
		c := newFakeClient(t, passwordSecret(testNamespace, "gw-admin", "other", "x"))
		_, _, err := ResolveBasicAuthCredentials(ctx, c, testNamespace, enabled(AuthUser{
			Username: "ops", Password: testHash, PasswordHashed: true,
			PasswordSecretRef: &SecretKeyRef{Name: "gw-admin"},
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `no value under key "password"`)
	})

	t.Run("hashed password without Secret is skipped for a later plaintext user", func(t *testing.T) {
		u, p, err := ResolveBasicAuthCredentials(ctx, nil, testNamespace, enabled(
			AuthUser{Username: "hashed", Password: testHash, PasswordHashed: true},
			AuthUser{Username: "plain", Password: "pw"},
		))
		require.NoError(t, err)
		assert.Equal(t, "plain", u)
		assert.Equal(t, "pw", p)
	})

	failures := map[string]*AuthSettings{
		"nil settings":            nil,
		"basic auth disabled":     {Basic: BasicAuthConfig{Enabled: false, Users: []AuthUser{{Username: "a", Password: "b"}}}},
		"no users":                enabled(),
		"only hashed password":    enabled(AuthUser{Username: "ops", Password: testHash, PasswordHashed: true}),
		"interpolation token":     enabled(AuthUser{Username: "ops", Password: `{{ env "ADMIN_PASSWORD" }}`}),
		"empty password":          enabled(AuthUser{Username: "ops"}),
		"user without a username": enabled(AuthUser{Password: "pw"}),
	}
	for name, settings := range failures {
		t.Run("no credentials: "+name, func(t *testing.T) {
			_, _, err := ResolveBasicAuthCredentials(ctx, nil, testNamespace, settings)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrNoControllerCredentials)
		})
	}
}

func TestResolveControllerCredentials(t *testing.T) {
	ctx := context.Background()
	baseFile := filepath.Join(t.TempDir(), "gateway_values.yaml")
	require.NoError(t, os.WriteFile(baseFile, []byte(`
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: base-user
              password: base-password
`), 0o600))

	gatewayWithConfig := func(configMap string) *apiv1.APIGateway {
		gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "gw"}}
		if configMap != "" {
			gw.Spec.ConfigRef = &corev1.LocalObjectReference{Name: configMap}
		}
		return gw
	}
	info := &registry.GatewayInfo{Name: "gw", Namespace: testNamespace}

	t.Run("base values file applies when the gateway has no overlay", func(t *testing.T) {
		c := newFakeClient(t, gatewayWithConfig(""))
		u, p, err := ResolveControllerCredentials(ctx, c, info, baseFile)
		require.NoError(t, err)
		assert.Equal(t, "base-user", u)
		assert.Equal(t, "base-password", p)
	})

	t.Run("base values file applies when the overlay sets no auth", func(t *testing.T) {
		c := newFakeClient(t, gatewayWithConfig("gw-values"), valuesConfigMap("gw-values", "gateway:\n  config:\n    controller:\n      server:\n        api_port: 9090\n"))
		u, _, err := ResolveControllerCredentials(ctx, c, info, baseFile)
		require.NoError(t, err)
		assert.Equal(t, "base-user", u)
	})

	t.Run("overlay users replace the base users and read the Secret", func(t *testing.T) {
		c := newFakeClient(t,
			gatewayWithConfig("gw-values"),
			valuesConfigMap("gw-values", `
gateway:
  config:
    controller:
      auth:
        basic:
          users:
            - username: gw-admin
              password: "`+testHash+`"
              password_hashed: true
              passwordSecretRef:
                name: gw-admin-credential
`),
			passwordSecret(testNamespace, "gw-admin-credential", DefaultPasswordSecretKey, "per-gateway"),
		)
		u, p, err := ResolveControllerCredentials(ctx, c, info, baseFile)
		require.NoError(t, err)
		assert.Equal(t, "gw-admin", u)
		assert.Equal(t, "per-gateway", p)
	})

	t.Run("Helm values ConfigMap of a Gateway API gateway", func(t *testing.T) {
		c := newFakeClient(t, valuesConfigMap("gwapi-values", `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: gwapi
              password: gwapi-password
`))
		gwInfo := &registry.GatewayInfo{Name: "gwapi", Namespace: testNamespace, HelmValuesConfigMapName: "gwapi-values", FromGatewayAPI: true}
		u, p, err := ResolveControllerCredentials(ctx, c, gwInfo, "")
		require.NoError(t, err)
		assert.Equal(t, "gwapi", u)
		assert.Equal(t, "gwapi-password", p)
	})

	t.Run("no configured credentials is an error, not admin/admin", func(t *testing.T) {
		c := newFakeClient(t, gatewayWithConfig(""))
		_, _, err := ResolveControllerCredentials(ctx, c, info, filepath.Join(t.TempDir(), "missing.yaml"))
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoControllerCredentials)
	})

	t.Run("overlay can disable basic auth", func(t *testing.T) {
		c := newFakeClient(t, gatewayWithConfig("gw-values"), valuesConfigMap("gw-values", "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: false\n"))
		_, _, err := ResolveControllerCredentials(ctx, c, info, baseFile)
		assert.ErrorIs(t, err, ErrNoControllerCredentials)
	})

	t.Run("unknown gateway", func(t *testing.T) {
		_, _, err := ResolveControllerCredentials(ctx, newFakeClient(t), nil, baseFile)
		assert.ErrorIs(t, err, ErrNoControllerCredentials)
	})
}
