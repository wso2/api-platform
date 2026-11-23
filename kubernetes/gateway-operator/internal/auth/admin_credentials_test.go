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

// The operator must send the password the gateway-controller holds: in file mode the
// gateway's {{ file }} interpolation trims trailing whitespace (a Secret made with
// --from-file ends in a newline), while in env mode the value is used unchanged.
func TestResolveBasicCredentials_MatchesGatewayTrimming(t *testing.T) {
	cases := map[string]struct {
		source   string
		value    string
		wantPass string
		wantErr  bool
	}{
		"file trims newline":         {source: apiv1.AdminCredentialsSourceFile, value: "s3cret-1\n", wantPass: "s3cret-1"},
		"file trims crlf and blanks": {source: apiv1.AdminCredentialsSourceFile, value: "s3cret-1 \t\r\n", wantPass: "s3cret-1"},
		"file keeps leading blanks":  {source: apiv1.AdminCredentialsSourceFile, value: "  s3cret-1\n", wantPass: "  s3cret-1"},
		"file only newline is empty": {source: apiv1.AdminCredentialsSourceFile, value: "\n", wantErr: true},
		"env keeps newline":          {source: apiv1.AdminCredentialsSourceEnv, value: "s3cret-1\n", wantPass: "s3cret-1\n"},
		"unset source is file":       {source: "", value: "s3cret-1\n", wantPass: "s3cret-1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
			gw.Spec.AdminCredentials.Source = tc.source
			c := newFakeClient(t, gw, passwordSecret("gateway-1-admin", map[string]string{"password": tc.value}))

			_, pass, err := ResolveBasicCredentials(context.Background(), c, info())
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, pass)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPass, pass)
		})
	}
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

// An APIGateway registered with a configRef carries HelmValuesConfigMapName; its
// spec.adminCredentials must still win over the ConfigMap's basic-auth users.
func TestResolveBasicCredentials_AdminCredentialsWinOverConfigRef(t *testing.T) {
	gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
	gw.Spec.ConfigRef = &corev1.LocalObjectReference{Name: "gw-values"}
	c := newFakeClient(t, gw,
		passwordSecret("gateway-1-admin", map[string]string{"password": "s3cret-1"}),
		valuesConfigMap("gw-values", legacyValues))

	gwInfo := info()
	gwInfo.HelmValuesConfigMapName = "gw-values"
	user, pass, err := ResolveBasicCredentials(context.Background(), c, gwInfo)
	require.NoError(t, err)
	assert.Equal(t, "gw1-admin", user)
	assert.Equal(t, "s3cret-1", pass)
}

// Kubernetes Gateway API entries have no APIGateway: their credentials come only from
// the values ConfigMap, even if an APIGateway of the same name happens to exist.
func TestResolveBasicCredentials_GatewayAPIEntryUsesConfigMap(t *testing.T) {
	gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
	c := newFakeClient(t, gw,
		passwordSecret("gateway-1-admin", map[string]string{"password": "s3cret-1"}),
		valuesConfigMap("gw-values", legacyValues))

	gwInfo := info()
	gwInfo.FromGatewayAPI = true
	gwInfo.HelmValuesConfigMapName = "gw-values"
	user, pass, err := ResolveBasicCredentials(context.Background(), c, gwInfo)
	require.NoError(t, err)
	assert.Equal(t, "legacy", user)
	assert.Equal(t, "legacy-pw", pass)
}

// A values-configured user with passwordSecretRef (the gateway chart's own field) is
// resolved from the Secret, and fails closed instead of sending an empty password.
func TestResolveBasicCredentials_ValuesPasswordSecretRef(t *testing.T) {
	refValues := func(extra string) string {
		return `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: values-admin
              passwordSecretRef:
                name: values-admin-secret
` + extra
	}
	cases := map[string]struct {
		values   string
		secret   *corev1.Secret
		wantPass string
		wantErr  bool
	}{
		"default key":     {values: refValues(""), secret: passwordSecret("values-admin-secret", map[string]string{"password": "from-secret"}), wantPass: "from-secret"},
		"explicit key":    {values: refValues("                key: pw\n"), secret: passwordSecret("values-admin-secret", map[string]string{"pw": "from-pw"}), wantPass: "from-pw"},
		"secret missing":  {values: refValues(""), wantErr: true},
		"key missing":     {values: refValues(""), secret: passwordSecret("values-admin-secret", map[string]string{"other": "x"}), wantErr: true},
		"value empty":     {values: refValues(""), secret: passwordSecret("values-admin-secret", map[string]string{"password": ""}), wantErr: true},
		"hashed password": {values: refValues("              password_hashed: true\n"), secret: passwordSecret("values-admin-secret", map[string]string{"password": "$2a$10$hash"}), wantErr: true},
		// The gateway trims a file-mode password but passes an env-mode one unchanged.
		"file mode trims newline":    {values: refValues("                source: file\n"), secret: passwordSecret("values-admin-secret", map[string]string{"password": "from-file\n"}), wantPass: "from-file"},
		"env mode keeps newline":     {values: refValues("                source: env\n"), secret: passwordSecret("values-admin-secret", map[string]string{"password": "from-env\n"}), wantPass: "from-env\n"},
		"default mode keeps newline": {values: refValues(""), secret: passwordSecret("values-admin-secret", map[string]string{"password": "from-env\n"}), wantPass: "from-env\n"},
		"file mode only whitespace":  {values: refValues("                source: file\n"), secret: passwordSecret("values-admin-secret", map[string]string{"password": " \n"}), wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objs := []client.Object{valuesConfigMap("gw-values", tc.values)}
			if tc.secret != nil {
				objs = append(objs, tc.secret)
			}
			c := newFakeClient(t, objs...)
			gwInfo := info()
			gwInfo.FromGatewayAPI = true
			gwInfo.HelmValuesConfigMapName = "gw-values"

			user, pass, err := ResolveBasicCredentials(context.Background(), c, gwInfo)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, pass)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "values-admin", user)
			assert.Equal(t, tc.wantPass, pass)
		})
	}
}

// A plain user with an empty password is not usable; it is never sent as is.
func TestGetBasicAuthCredentials_EmptyPasswordNotUsable(t *testing.T) {
	settings := &AuthSettings{Basic: BasicAuthConfig{Enabled: true, Users: []AuthUser{{Username: "admin"}}}}
	_, _, ok := GetBasicAuthCredentials(settings)
	assert.False(t, ok)
}

const legacyValues = `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: legacy
              password: legacy-pw
`

func valuesConfigMap(name, values string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Data:       map[string]string{"values.yaml": values},
	}
}
