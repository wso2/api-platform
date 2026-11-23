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
		{"", "file"}, // unset defaults to file, like the CRD default
	} {
		t.Run("source="+tc.source, func(t *testing.T) {
			out, err := buildAdminCredentialsOverlay(adminCredsGateway(tc.source), "")
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
			assert.NotContains(t, out, "podAnnotations", "the pod checksum comes from basicAuthChecksumOverlay")
		})
	}
}

func TestBuildAdminCredentialsOverlay_Unset(t *testing.T) {
	out, err := buildAdminCredentialsOverlay(&apiv1.APIGateway{}, "")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestBuildAdminCredentialsOverlay_InvalidSource(t *testing.T) {
	_, err := buildAdminCredentialsOverlay(adminCredsGateway("vault"), "")
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
	overlay, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), configMapValues)
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
	overlay, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceEnv), configMapValues)
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
			overlay, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), values)
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
	_, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), "gateway: [unclosed")
	require.Error(t, err)
}

const configRefSecretUsers = `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: ops-admin
              passwordSecretRef: {name: multi-users, key: ops, source: file}
            - username: dev-user
              passwordSecretRef: {name: multi-users, key: dev}
            - username: second
              passwordSecretRef: {name: another-secret}
            - username: viewer
              password: viewer-plain
`

func TestBasicAuthSecretNames(t *testing.T) {
	names, err := basicAuthSecretNames(adminCredsGateway(apiv1.AdminCredentialsSourceFile), configRefSecretUsers)
	require.NoError(t, err)
	assert.Equal(t, []string{"another-secret", "gateway-1-admin", "multi-users"}, names, "sorted, without duplicates")

	names, err = basicAuthSecretNames(&apiv1.APIGateway{}, configRefSecretUsers)
	require.NoError(t, err)
	assert.Equal(t, []string{"another-secret", "multi-users"}, names, "configRef users without adminCredentials")

	names, err = basicAuthSecretNames(&apiv1.APIGateway{}, legacyPlainUserValues)
	require.NoError(t, err)
	assert.Empty(t, names, "plain passwords reference no Secret")

	_, err = basicAuthSecretNames(&apiv1.APIGateway{}, "gateway: [unclosed")
	require.Error(t, err)
}

const legacyPlainUserValues = `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: admin
              password: admin
`

func TestBasicAuthSecretsVersion(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		gw         *apiv1.APIGateway
		values     string
		secretName string
	}{
		"adminCredentials": {gw: adminCredsGateway(apiv1.AdminCredentialsSourceFile), secretName: "gateway-1-admin"},
		"configRef user":   {gw: &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: "gw1"}}, values: configRefSecretUsers, secretName: "multi-users"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := adminCredsClient(t)
			missing, err := basicAuthSecretsVersion(ctx, c, tc.gw, tc.values)
			require.NoError(t, err)
			assert.NotEmpty(t, missing, "a missing Secret still yields a version, so creating it changes the hash")

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: tc.secretName, Namespace: "gw1", UID: "uid-1"},
				Data:       map[string][]byte{"password": []byte("s3cret")},
			}
			require.NoError(t, c.Create(ctx, secret))
			v1, err := basicAuthSecretsVersion(ctx, c, tc.gw, tc.values)
			require.NoError(t, err)
			assert.NotEqual(t, missing, v1, "creating the Secret changes the version")
			assert.NotContains(t, v1, "s3cret")

			// Rotating the password bumps resourceVersion, which changes the version.
			secret.Data["password"] = []byte("rotated")
			require.NoError(t, c.Update(ctx, secret))
			v2, err := basicAuthSecretsVersion(ctx, c, tc.gw, tc.values)
			require.NoError(t, err)
			assert.NotEqual(t, v1, v2)
		})
	}

	// No Secret-backed user: nothing to track, so the legacy hash is kept.
	v, err := basicAuthSecretsVersion(ctx, adminCredsClient(t), &apiv1.APIGateway{}, legacyPlainUserValues)
	require.NoError(t, err)
	assert.Empty(t, v)
}

func TestBasicAuthChecksumOverlay(t *testing.T) {
	out, err := basicAuthChecksumOverlay("")
	require.NoError(t, err)
	assert.Empty(t, out)

	out, err = basicAuthChecksumOverlay("v1")
	require.NoError(t, err)
	merged, err := helm.MergeValuesYAML("gateway:\n  controller:\n    deployment:\n      podAnnotations:\n        team: platform\n", out)
	require.NoError(t, err)
	var v map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(merged), &v))
	annotations := v["gateway"].(map[string]interface{})["controller"].(map[string]interface{})["deployment"].(map[string]interface{})["podAnnotations"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"team": "platform", basicAuthCredentialsChecksumAnnotation: "v1"}, annotations)
	assert.True(t, strings.HasPrefix(basicAuthCredentialsChecksumAnnotation, "checksum/"))
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
	withConfigRef := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "configref-users", Namespace: "gw1"},
		Spec:       apiv1.GatewaySpec{ConfigRef: &corev1.LocalObjectReference{Name: "users-values"}},
	}
	usersValues := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "users-values", Namespace: "gw1"},
		Data:       map[string]string{"values.yaml": configRefSecretUsers},
	}
	missingConfigRef := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "missing-configref", Namespace: "gw1"},
		Spec:       apiv1.GatewaySpec{ConfigRef: &corev1.LocalObjectReference{Name: "not-there"}},
	}

	r := &GatewayReconciler{Client: adminCredsClient(t, gw1, other, elsewhere, withConfigRef, usersValues, missingConfigRef)}
	enqueued := func(secretName string) []string {
		var names []string
		for _, req := range r.enqueueGatewaysForSecret(context.Background(),
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "gw1"}}) {
			assert.Equal(t, "gw1", req.Namespace)
			names = append(names, req.Name)
		}
		return names
	}

	assert.Equal(t, []string{"gateway-1"}, enqueued("gateway-1-admin"), "adminCredentials Secret")
	assert.Equal(t, []string{"configref-users"}, enqueued("multi-users"), "configRef user's passwordSecretRef")
	assert.Empty(t, enqueued("something-else"), "unrelated Secret")
}
