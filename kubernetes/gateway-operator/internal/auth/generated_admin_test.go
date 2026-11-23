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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

func generatedSecret(gatewayName, password string, ours bool) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: GeneratedAdminSecretName(gatewayName), Namespace: testNS},
		Data: map[string][]byte{
			GeneratedAdminUsernameKey: []byte("admin"),
			GeneratedAdminPasswordKey: []byte(password),
		},
	}
	if ours {
		s.Labels = map[string]string{GeneratedAdminCredentialsLabel: GeneratedAdminCredentialsLabelValue}
	}
	return s
}

func TestGeneratedAdminSecretName(t *testing.T) {
	assert.Equal(t, "gateway-1-admin-credentials", GeneratedAdminSecretName("gateway-1"))

	long := strings.Repeat("a", 250)
	name := GeneratedAdminSecretName(long)
	assert.LessOrEqual(t, len(name), 253)
	assert.True(t, strings.HasSuffix(name, "-admin-credentials"))
	other := GeneratedAdminSecretName(strings.Repeat("a", 249) + "b")
	assert.NotEqual(t, name, other, "shortened names stay unique")
}

func TestGeneratePassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p, err := GeneratePassword()
		require.NoError(t, err)
		assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9]{32}$`), p)
		assert.False(t, seen[p], "passwords must not repeat")
		seen[p] = true
	}
}

func TestGetGeneratedAdminSecret(t *testing.T) {
	ctx := context.Background()

	s, err := GetGeneratedAdminSecret(ctx, newFakeClient(t), testNS, "gateway-1")
	require.NoError(t, err)
	assert.Nil(t, s, "absent Secret")

	s, err = GetGeneratedAdminSecret(ctx, newFakeClient(t, generatedSecret("gateway-1", "pw", true)), testNS, "gateway-1")
	require.NoError(t, err)
	require.NotNil(t, s)

	_, err = GetGeneratedAdminSecret(ctx, newFakeClient(t, generatedSecret("gateway-1", "pw", false)), testNS, "gateway-1")
	require.ErrorIs(t, err, ErrForeignAdminSecret, "a same-name Secret the operator did not create is never used")
}

// A gateway without spec.adminCredentials or values users signs in with the generated
// credential, as the gateway-controller reads it from the mounted file.
func TestResolveBasicCredentials_GeneratedSecret(t *testing.T) {
	gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: testNS}}
	c := newFakeClient(t, gw, generatedSecret("gateway-1", "g3nerated\n", true))

	user, pass, err := ResolveBasicCredentials(context.Background(), c, info())
	require.NoError(t, err)
	assert.Equal(t, "admin", user)
	assert.Equal(t, "g3nerated", pass)

	// Kubernetes Gateway API gateways use it the same way.
	user, pass, err = ResolveBasicCredentials(context.Background(), c,
		&registry.GatewayInfo{Name: "gateway-1", Namespace: testNS, FromGatewayAPI: true})
	require.NoError(t, err)
	assert.Equal(t, "admin", user)
	assert.Equal(t, "g3nerated", pass)
}

func TestResolveBasicCredentials_GeneratedSecretEmptyFails(t *testing.T) {
	gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: testNS}}
	c := newFakeClient(t, gw, generatedSecret("gateway-1", "\n", true))

	_, pass, err := ResolveBasicCredentials(context.Background(), c, info())
	require.Error(t, err)
	assert.Empty(t, pass)
}

// spec.adminCredentials wins over a generated Secret left from before it was set.
func TestResolveBasicCredentials_SpecWinsOverGenerated(t *testing.T) {
	gw := gatewayWithAdminCredentials("gw1-admin", "gateway-1-admin", "password")
	c := newFakeClient(t, gw,
		passwordSecret("gateway-1-admin", map[string]string{"password": "from-spec"}),
		generatedSecret("gateway-1", "generated", true))

	user, pass, err := ResolveBasicCredentials(context.Background(), c, info())
	require.NoError(t, err)
	assert.Equal(t, "gw1-admin", user)
	assert.Equal(t, "from-spec", pass)
}

// A gateway an earlier operator deployed has no generated Secret and keeps admin/admin;
// an unrelated Secret that happens to have the generated name does not change that.
func TestResolveBasicCredentials_LegacyKeepsDefault(t *testing.T) {
	gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: testNS}}
	for name, c := range map[string]func() []client.Object{
		"no secret":      func() []client.Object { return []client.Object{gw} },
		"foreign secret": func() []client.Object { return []client.Object{gw, generatedSecret("gateway-1", "x", false)} },
	} {
		t.Run(name, func(t *testing.T) {
			user, pass, err := ResolveBasicCredentials(context.Background(), newFakeClient(t, c()...), info())
			require.NoError(t, err)
			assert.Equal(t, "admin", user)
			assert.Equal(t, "admin", pass)
		})
	}
}

func TestAdminCredentialsSourceDefaultsToFile(t *testing.T) {
	assert.Equal(t, apiv1.AdminCredentialsSourceFile, AdminCredentialsSource(""))
	assert.Equal(t, apiv1.AdminCredentialsSourceEnv, AdminCredentialsSource(apiv1.AdminCredentialsSourceEnv))
}
