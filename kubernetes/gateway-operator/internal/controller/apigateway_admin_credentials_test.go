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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

func adminCredsGateway(source string) *apiv1.APIGateway {
	return &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: "gw1"},
		Spec: apiv1.GatewaySpec{
			AdminCredentials: &apiv1.GatewayAdminCredentials{
				Username: "gw1-admin",
				PasswordSecretRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "gateway-1-admin"},
					Key:                  "password",
				},
				Source: source,
			},
		},
	}
}

func adminCredsClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// basicUsers extracts gateway.config.controller.auth.basic.users from Helm values YAML.
func basicUsers(t *testing.T, valuesYAML string) []interface{} {
	t.Helper()
	var v map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(valuesYAML), &v))
	basic := v["gateway"].(map[string]interface{})["config"].(map[string]interface{})["controller"].(map[string]interface{})["auth"].(map[string]interface{})["basic"].(map[string]interface{})
	assert.Equal(t, true, basic["enabled"])
	return basic["users"].([]interface{})
}

func TestBuildAdminCredentialsOverlay_Sources(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{apiv1.AdminCredentialsSourceFile, "file"},
		{apiv1.AdminCredentialsSourceEnv, "env"},
		{"", "env"}, // unset defaults to env, like the CRD default
	} {
		t.Run("source="+tc.source, func(t *testing.T) {
			out, err := buildAdminCredentialsOverlay(adminCredsGateway(tc.source), "v1", "")
			require.NoError(t, err)

			users := basicUsers(t, out)
			require.Len(t, users, 1)
			u := users[0].(map[string]interface{})
			assert.Equal(t, "gw1-admin", u["username"])
			assert.Equal(t, false, u["password_hashed"])
			assert.NotContains(t, u, "password", "the overlay must reference the Secret, never carry a password")
			ref := u["passwordSecretRef"].(map[string]interface{})
			assert.Equal(t, "gateway-1-admin", ref["name"])
			assert.Equal(t, "password", ref["key"])
			assert.Equal(t, tc.want, ref["source"])
			assert.Contains(t, out, adminCredentialsChecksumAnnotation+": v1")
		})
	}
}

func TestBuildAdminCredentialsOverlay_Unset(t *testing.T) {
	out, err := buildAdminCredentialsOverlay(&apiv1.APIGateway{}, "", "")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestBuildAdminCredentialsOverlay_InvalidSource(t *testing.T) {
	_, err := buildAdminCredentialsOverlay(adminCredsGateway("vault"), "v1", "")
	require.Error(t, err)
}

// The overlay is merged after the ConfigMap values, so it replaces their basic-auth
// users (and plaintext password) while keeping unrelated settings and annotations.
// The overlay is merged after the ConfigMap values. It puts the adminCredentials user
// first and keeps the configRef's other basic-auth users after it, unchanged, while
// keeping unrelated settings and annotations.
func TestAdminCredentialsOverlayKeepsConfigRefUsers(t *testing.T) {
	configMapValues := `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: ops-admin
              passwordSecretRef: {name: multi-users, key: ops, source: file}
              roles: ["admin"]
            - username: dev-user
              passwordSecretRef: {name: multi-users, key: dev}
              roles: ["developer"]
            - username: viewer
              password: viewer-plain
              roles: ["developer"]
        idp:
          enabled: false
  controller:
    deployment:
      podAnnotations:
        team: platform
`
	overlay, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), "v1", configMapValues)
	require.NoError(t, err)
	merged, err := helm.MergeValuesYAML(configMapValues, overlay)
	require.NoError(t, err)

	users := basicUsers(t, merged)
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.(map[string]interface{})["username"].(string))
	}
	assert.Equal(t, []string{"gw1-admin", "ops-admin", "dev-user", "viewer"}, names, "admin user first, configRef users kept in order")

	ops := users[1].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"name": "multi-users", "key": "ops", "source": "file"}, ops["passwordSecretRef"])
	assert.Equal(t, []interface{}{"admin"}, ops["roles"])
	assert.Equal(t, "viewer-plain", users[3].(map[string]interface{})["password"])
	assert.Contains(t, merged, "team: platform")
	assert.Contains(t, merged, "idp:")
}

// A configRef user with the admin's username is replaced by the adminCredentials user,
// so a plaintext password for that name never reaches the gateway.
func TestAdminCredentialsOverlayReplacesSameUsername(t *testing.T) {
	configMapValues := `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: gw1-admin
              password: old-plaintext
              roles: ["admin"]
            - username: dev-user
              password: dev-plain
              roles: ["developer"]
`
	overlay, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceEnv), "v1", configMapValues)
	require.NoError(t, err)
	merged, err := helm.MergeValuesYAML(configMapValues, overlay)
	require.NoError(t, err)

	users := basicUsers(t, merged)
	require.Len(t, users, 2)
	admin := users[0].(map[string]interface{})
	assert.Equal(t, "gw1-admin", admin["username"])
	assert.NotContains(t, admin, "password")
	assert.Equal(t, "dev-user", users[1].(map[string]interface{})["username"])
	assert.NotContains(t, merged, "old-plaintext")
}

// Without configRef users (no configRef, or none listed) only the admin user is set,
// which also replaces the operator base values file's default admin/admin user.
func TestAdminCredentialsOverlayWithoutConfigRefUsers(t *testing.T) {
	for name, values := range map[string]string{
		"no configRef":     "",
		"no users listed":  "gateway:\n  config:\n    controller:\n      auth:\n        idp:\n          enabled: false\n",
		"unrelated values": "commonLabels:\n  team: platform\n",
	} {
		t.Run(name, func(t *testing.T) {
			overlay, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), "v1", values)
			require.NoError(t, err)
			baseFile := "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: true\n          users:\n            - {username: admin, password: admin, roles: [admin]}\n"
			merged, err := helm.MergeValuesYAML(baseFile, overlay)
			require.NoError(t, err)
			users := basicUsers(t, merged)
			require.Len(t, users, 1)
			assert.Equal(t, "gw1-admin", users[0].(map[string]interface{})["username"])
			assert.NotContains(t, merged, "password: admin")
		})
	}
}

func TestAdminCredentialsOverlayRejectsInvalidConfigRefValues(t *testing.T) {
	_, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), "v1", "gateway: [unclosed")
	require.Error(t, err)
}

func TestAdminCredentialsSecretVersion(t *testing.T) {
	ctx := context.Background()
	gw := adminCredsGateway(apiv1.AdminCredentialsSourceFile)

	// Missing Secret: a stable marker, so creating it later changes the hash.
	v, err := adminCredentialsSecretVersion(ctx, adminCredsClient(t), gw)
	require.NoError(t, err)
	assert.Equal(t, "missing:gateway-1-admin", v)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1-admin", Namespace: "gw1", UID: "uid-1"},
		Data:       map[string][]byte{"password": []byte("s3cret")},
	}
	c := adminCredsClient(t, secret)
	v1, err := adminCredentialsSecretVersion(ctx, c, gw)
	require.NoError(t, err)
	assert.NotEmpty(t, v1)
	assert.NotContains(t, v1, "s3cret")

	// Rotating the password bumps resourceVersion, which changes the version.
	secret.Data["password"] = []byte("rotated")
	require.NoError(t, c.Update(ctx, secret))
	v2, err := adminCredentialsSecretVersion(ctx, c, gw)
	require.NoError(t, err)
	assert.NotEqual(t, v1, v2)

	// No adminCredentials: nothing to track.
	v, err = adminCredentialsSecretVersion(ctx, c, &apiv1.APIGateway{})
	require.NoError(t, err)
	assert.Empty(t, v)
}

func TestGatewayConfigHash(t *testing.T) {
	// Without adminCredentials the hash is unchanged from earlier releases, so an
	// operator upgrade does not redeploy existing gateways.
	assert.Equal(t, auth.CalculateConfigHash("values"), gatewayConfigHash("values", ""))
	assert.Equal(t, "", gatewayConfigHash("", ""))

	// With adminCredentials, a Secret revision change changes the hash, even with no ConfigMap.
	a := gatewayConfigHash("", "rev-a")
	b := gatewayConfigHash("", "rev-b")
	assert.NotEmpty(t, a)
	assert.NotEqual(t, a, b)
	assert.NotEqual(t, gatewayConfigHash("values", ""), gatewayConfigHash("values", "rev-a"))
}

func TestEnqueueGatewaysForSecret(t *testing.T) {
	gw1 := adminCredsGateway(apiv1.AdminCredentialsSourceFile)
	other := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "no-creds", Namespace: "gw1"}}
	elsewhere := adminCredsGateway(apiv1.AdminCredentialsSourceEnv)
	elsewhere.Name, elsewhere.Namespace = "gateway-x", "gw2"

	r := &GatewayReconciler{Client: adminCredsClient(t, gw1, other, elsewhere)}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1-admin", Namespace: "gw1"}}

	reqs := r.enqueueGatewaysForSecret(context.Background(), secret)
	require.Len(t, reqs, 1)
	assert.Equal(t, "gateway-1", reqs[0].Name)
	assert.Equal(t, "gw1", reqs[0].Namespace)

	unrelated := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "something-else", Namespace: "gw1"}}
	assert.Empty(t, r.enqueueGatewaysForSecret(context.Background(), unrelated))
	assert.True(t, strings.HasPrefix(adminCredentialsChecksumAnnotation, "checksum/"))
}
