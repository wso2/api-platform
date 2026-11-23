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
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
)

const generatedName = "gateway-1-admin-credentials"

func plainGateway() *apiv1.APIGateway {
	return &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: "gw1", UID: "uid-1"}}
}

func noRelease() (map[string]interface{}, bool, error) { return nil, false, nil }

func releaseWith(values string) releaseValuesFunc {
	return func() (map[string]interface{}, bool, error) {
		var v map[string]interface{}
		if values != "" {
			if err := yaml.Unmarshal([]byte(values), &v); err != nil {
				return nil, false, err
			}
		}
		return v, true, nil
	}
}

func planInput(t *testing.T, c client.Client, gw *apiv1.APIGateway, base, values string, rel releaseValuesFunc) defaultAdminInput {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	return defaultAdminInput{
		Client: c, Scheme: scheme, Owner: gw, GatewayName: gw.Name, Namespace: gw.Namespace,
		HasSpecCredentials: gw.Spec.AdminCredentials != nil,
		BaseValuesYAML:     base, ValuesYAML: values, ReleaseValues: rel,
	}
}

func ownGeneratedSecret(password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: generatedName, Namespace: "gw1",
			Labels: map[string]string{auth.GeneratedAdminCredentialsLabel: auth.GeneratedAdminCredentialsLabelValue}},
		Data: map[string][]byte{"username": []byte("admin"), "password": []byte(password)},
	}
}

const (
	usersValues = `
gateway:
  config:
    controller:
      auth:
        basic:
          users:
            - username: ops
              password: ops-pw
`
	basicDisabled = `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: false
`
	// Values an earlier operator deployed with: its base values file carried this user.
	legacyReleaseValues = `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: admin
              password: admin
              password_hashed: false
              roles: ["admin"]
`
	generatedReleaseValues = `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: admin
              passwordSecretRef: {name: gateway-1-admin-credentials, key: password, source: file}
`
)

// A new gateway gets a generated password in a labelled Secret it owns.
func TestPlanDefaultAdmin_NewGatewayGenerates(t *testing.T) {
	gw := plainGateway()
	c := adminCredsClient(t, gw)
	plan, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "", noRelease))
	require.NoError(t, err)
	assert.Equal(t, defaultAdminGenerated, plan.Mode)
	assert.Equal(t, generatedName, plan.SecretName)
	assert.True(t, plan.Created)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: "gw1", Name: generatedName}, secret))
	assert.True(t, auth.IsGeneratedAdminSecret(secret))
	assert.Equal(t, "admin", string(secret.Data["username"]))
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9]{32}$`), string(secret.Data["password"]))
	require.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, "APIGateway", secret.OwnerReferences[0].Kind)
	assert.Equal(t, gw.Name, secret.OwnerReferences[0].Name)

	// Later reconciles reuse it: same password, nothing created.
	before := string(secret.Data["password"])
	plan, err = planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "", releaseWith(generatedReleaseValues)))
	require.NoError(t, err)
	assert.Equal(t, defaultAdminGenerated, plan.Mode)
	assert.False(t, plan.Created)
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: "gw1", Name: generatedName}, secret))
	assert.Equal(t, before, string(secret.Data["password"]))
}

// The operator never generates where the gateway already has an admin user.
func TestPlanDefaultAdmin_NotNeeded(t *testing.T) {
	failRelease := func() (map[string]interface{}, bool, error) { return nil, false, errors.New("must not be read") }
	cases := map[string]struct {
		gw           *apiv1.APIGateway
		base, values string
	}{
		"spec.adminCredentials":       {gw: adminCredsGateway(apiv1.AdminCredentialsSourceFile)},
		"users in configRef values":   {gw: plainGateway(), values: usersValues},
		"users in operator base file": {gw: plainGateway(), base: usersValues},
		"basic auth disabled":         {gw: plainGateway(), values: basicDisabled},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := adminCredsClient(t, tc.gw)
			plan, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, tc.gw, tc.base, tc.values, failRelease))
			require.NoError(t, err)
			assert.Equal(t, defaultAdminNotNeeded, plan.Mode)
			err = c.Get(context.Background(), types.NamespacedName{Namespace: "gw1", Name: generatedName}, &corev1.Secret{})
			assert.True(t, client.IgnoreNotFound(err) == nil && err != nil, "no Secret is generated")
		})
	}
}

// A gateway an earlier operator deployed keeps admin/admin, whether its release values
// carried the operator's default user or no users (the chart default applied).
func TestPlanDefaultAdmin_LegacyKeepsAdminAdmin(t *testing.T) {
	for name, rel := range map[string]string{
		"default admin user in release": legacyReleaseValues,
		"no users in release":           "gateway: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			gw := plainGateway()
			c := adminCredsClient(t, gw)
			plan, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "", releaseWith(rel)))
			require.NoError(t, err)
			assert.Equal(t, defaultAdminLegacy, plan.Mode)
			err = c.Get(context.Background(), types.NamespacedName{Namespace: "gw1", Name: generatedName}, &corev1.Secret{})
			assert.Error(t, err, "a legacy gateway gets no generated Secret")
		})
	}
}

// A deployed gateway whose generated Secret is gone fails closed: no replacement, no
// admin/admin.
func TestPlanDefaultAdmin_MissingGeneratedSecretFails(t *testing.T) {
	gw := plainGateway()
	c := adminCredsClient(t, gw)
	_, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "", releaseWith(generatedReleaseValues)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), generatedName)
	err = c.Get(context.Background(), types.NamespacedName{Namespace: "gw1", Name: generatedName}, &corev1.Secret{})
	assert.Error(t, err, "no replacement Secret is generated")
}

// A gateway whose values listed other users, since removed, is treated like a new one.
func TestPlanDefaultAdmin_UsersRemovedGenerates(t *testing.T) {
	gw := plainGateway()
	c := adminCredsClient(t, gw)
	plan, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "", releaseWith(usersValues)))
	require.NoError(t, err)
	assert.Equal(t, defaultAdminGenerated, plan.Mode)
	assert.True(t, plan.Created)
}

func TestPlanDefaultAdmin_ExistingGeneratedSecretReused(t *testing.T) {
	gw := plainGateway()
	c := adminCredsClient(t, gw, ownGeneratedSecret("kept"))
	plan, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "",
		func() (map[string]interface{}, bool, error) { return nil, false, errors.New("must not be read") }))
	require.NoError(t, err)
	assert.Equal(t, defaultAdminPlan{Mode: defaultAdminGenerated, SecretName: generatedName}, plan)
}

// A Secret under the generated name that the operator did not create is neither used
// nor overwritten.
func TestPlanDefaultAdmin_ForeignSecretFails(t *testing.T) {
	gw := plainGateway()
	foreign := ownGeneratedSecret("theirs")
	foreign.Labels = nil
	c := adminCredsClient(t, gw, foreign)
	_, err := planDefaultAdminCredentials(context.Background(), planInput(t, c, gw, "", "", noRelease))
	require.ErrorIs(t, err, auth.ErrForeignAdminSecret)
}

func overlayValues(t *testing.T, plan defaultAdminPlan) map[string]interface{} {
	t.Helper()
	out, err := defaultAdminOverlay(plan, true, "", "")
	require.NoError(t, err)
	var v map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(out), &v))
	gen := v["gateway"].(map[string]interface{})["controller"].(map[string]interface{})["adminCredentials"].(map[string]interface{})["generate"]
	assert.Equal(t, false, gen, "the gateway chart never generates under the operator")
	return v
}

func TestDefaultAdminOverlay(t *testing.T) {
	t.Run("generated reads a mounted file", func(t *testing.T) {
		v := overlayValues(t, defaultAdminPlan{Mode: defaultAdminGenerated, SecretName: generatedName})
		users := basicAuthUsersIn(v)
		require.Len(t, users, 1)
		u := users[0].(map[string]interface{})
		assert.Equal(t, "admin", u["username"])
		assert.NotContains(t, u, "password")
		assert.Equal(t, map[string]interface{}{"name": generatedName, "key": "password", "source": "file"}, u["passwordSecretRef"])
	})
	t.Run("legacy keeps admin/admin", func(t *testing.T) {
		v := overlayValues(t, defaultAdminPlan{Mode: defaultAdminLegacy})
		users := basicAuthUsersIn(v)
		require.Len(t, users, 1)
		u := users[0].(map[string]interface{})
		assert.Equal(t, "admin", u["username"])
		assert.Equal(t, "admin", u["password"])
	})
	t.Run("not needed adds no user", func(t *testing.T) {
		v := overlayValues(t, defaultAdminPlan{Mode: defaultAdminNotNeeded})
		assert.Nil(t, basicAuthMap(v))
	})
}

// Rotating a generated credential re-queues its gateway (which rolls the controller pod);
// an unlabelled Secret with the same name does not.
func TestEnqueueGatewaysForGeneratedSecret(t *testing.T) {
	gw := plainGateway()
	withSpec := adminCredsGateway(apiv1.AdminCredentialsSourceFile)
	withSpec.Name = "with-spec"
	r := &GatewayReconciler{Client: adminCredsClient(t, gw, withSpec)}

	reqs := r.enqueueGatewaysForSecret(context.Background(), ownGeneratedSecret("pw"))
	require.Len(t, reqs, 1)
	assert.Equal(t, "gateway-1", reqs[0].Name)

	foreign := ownGeneratedSecret("pw")
	foreign.Labels = nil
	assert.Empty(t, r.enqueueGatewaysForSecret(context.Background(), foreign))

	specGenerated := ownGeneratedSecret("pw")
	specGenerated.Name = auth.GeneratedAdminSecretName("with-spec")
	assert.Empty(t, r.enqueueGatewaysForSecret(context.Background(), specGenerated),
		"a gateway with spec.adminCredentials does not use a generated Secret")
}

const extraEnvValues = `
gateway:
  controller:
    deployment:
      extraEnv:
        - name: FROM_VALUES
          value: "1"
        - name: APIP_GW_CONTROLLER_OPERATOR_ADMIN_PASSWORD
          value: stale
`

const extraEnvBase = `
gateway:
  controller:
    deployment:
      extraEnv:
        - name: FROM_BASE
          value: "1"
`

func controllerExtraEnv(t *testing.T, v map[string]interface{}) []interface{} {
	t.Helper()
	env, _ := v["gateway"].(map[string]interface{})["controller"].(map[string]interface{})["deployment"].(map[string]interface{})["extraEnv"].([]interface{})
	return env
}

func envNames(env []interface{}) []string {
	var names []string
	for _, e := range env {
		names = append(names, e.(map[string]interface{})["name"].(string))
	}
	return names
}

// With a gateway chart that predates passwordSecretRef, the generated password reaches
// the gateway-controller as an environment variable from the same Secret, and the
// values' own extraEnv entries are kept (the values' list wins over the base file's).
func TestDefaultAdminOverlay_OlderChartUsesEnv(t *testing.T) {
	out, err := defaultAdminOverlay(defaultAdminPlan{Mode: defaultAdminGenerated, SecretName: generatedName}, false, extraEnvBase, extraEnvValues)
	require.NoError(t, err)
	assert.NotContains(t, out, "passwordSecretRef")
	var v map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(out), &v))

	u := basicAuthUsersIn(v)[0].(map[string]interface{})
	assert.Equal(t, `{{ env "APIP_GW_CONTROLLER_OPERATOR_ADMIN_PASSWORD" "" }}`, u["password"])

	env := controllerExtraEnv(t, v)
	assert.Equal(t, []string{"FROM_VALUES", "APIP_GW_CONTROLLER_OPERATOR_ADMIN_PASSWORD"}, envNames(env))
	assert.Equal(t, map[string]interface{}{"secretKeyRef": map[string]interface{}{"name": generatedName, "key": "password"}},
		env[1].(map[string]interface{})["valueFrom"])

	// Without extraEnv in the values, the base file's entries are kept.
	out, err = defaultAdminOverlay(defaultAdminPlan{Mode: defaultAdminGenerated, SecretName: generatedName}, false, extraEnvBase, "")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal([]byte(out), &v))
	assert.Equal(t, []string{"FROM_BASE", "APIP_GW_CONTROLLER_OPERATOR_ADMIN_PASSWORD"}, envNames(controllerExtraEnv(t, v)))
}

// The legacy admin/admin user and "not needed" never add the variable.
func TestDefaultAdminOverlay_OlderChartLegacyUnchanged(t *testing.T) {
	for _, mode := range []defaultAdminMode{defaultAdminLegacy, defaultAdminNotNeeded} {
		out, err := defaultAdminOverlay(defaultAdminPlan{Mode: mode}, false, extraEnvBase, extraEnvValues)
		require.NoError(t, err)
		assert.NotContains(t, out, "extraEnv", mode)
	}
}

func TestBuildAdminCredentialsOverlay_OlderChartUsesEnv(t *testing.T) {
	out, err := buildAdminCredentialsOverlay(adminCredsGateway(apiv1.AdminCredentialsSourceFile), extraEnvValues, false, extraEnvBase)
	require.NoError(t, err)
	assert.NotContains(t, out, "passwordSecretRef")
	var v map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(out), &v))
	u := basicAuthUsersIn(v)[0].(map[string]interface{})
	assert.Equal(t, "gw1-admin", u["username"])
	assert.Equal(t, `{{ env "APIP_GW_CONTROLLER_OPERATOR_ADMIN_PASSWORD" "" }}`, u["password"])
	env := controllerExtraEnv(t, v)
	assert.Equal(t, []string{"FROM_VALUES", "APIP_GW_CONTROLLER_OPERATOR_ADMIN_PASSWORD"}, envNames(env))
	assert.Equal(t, map[string]interface{}{"secretKeyRef": map[string]interface{}{"name": "gateway-1-admin", "key": "password"}},
		env[1].(map[string]interface{})["valueFrom"])
}
