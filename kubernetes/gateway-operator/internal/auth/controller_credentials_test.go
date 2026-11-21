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
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const testNS = "gw-ns"

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func writeValuesFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gateway_values.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func valuesConfigMap(name, values string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Data:       map[string]string{"values.yaml": values},
	}
}

func apiGateway(name, configRef string) *apiv1.APIGateway {
	gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS}}
	if configRef != "" {
		gw.Spec.ConfigRef = &corev1.LocalObjectReference{Name: configRef}
	}
	return gw
}

func basic(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func usersYAML(users string) string {
	return "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: true\n          users:\n" + users
}

func TestResolveControllerAuthHeader(t *testing.T) {
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	require.NoError(t, err)
	hashedSecretUser := `            - username: "operator"
              password: "` + string(hash) + `"
              password_hashed: true
              roles: ["admin"]
              password_secret_ref: {name: controller-creds, key: password}
`
	credsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "controller-creds", Namespace: testNS},
		Data:       map[string][]byte{"password": []byte("s3cret")},
	}

	tests := []struct {
		name       string
		valuesFile string
		objs       []client.Object
		info       *registry.GatewayInfo
		want       string
		wantErr    string
		wantErrIs  error
	}{
		{
			name: "chart defaults only",
			objs: []client.Object{apiGateway("gw", "")},
			info: &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want: basic("admin", "admin"),
		},
		{
			name:       "operator values file sets the users (the case the CR overlay could not reach)",
			valuesFile: usersYAML("            - {username: ops, password: ops-pass, password_hashed: false, roles: [admin]}\n"),
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want:       basic("ops", "ops-pass"),
		},
		{
			name:       "per-gateway ConfigMap wins over the operator values file and replaces the list",
			valuesFile: usersYAML("            - {username: ops, password: ops-pass, password_hashed: false, roles: [admin]}\n"),
			objs: []client.Object{
				apiGateway("gw", "gw-values"),
				valuesConfigMap("gw-values", usersYAML("            - {username: team, password: team-pass, password_hashed: false, roles: [admin]}\n")),
			},
			info: &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want: basic("team", "team-pass"),
		},
		{
			name: "Gateway API values ConfigMap from the registry",
			objs: []client.Object{
				valuesConfigMap("k8s-gw-values", usersYAML("            - {username: k8s, password: k8s-pass, password_hashed: false, roles: [admin]}\n")),
			},
			info: &registry.GatewayInfo{Name: "k8s-gw", Namespace: testNS, HelmValuesConfigMapName: "k8s-gw-values"},
			want: basic("k8s", "k8s-pass"),
		},
		{
			name:       "a layer that sets only enabled keeps the chart default users (maps merge per key)",
			valuesFile: "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: true\n",
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want:       basic("admin", "admin"),
		},
		{
			name:       "admin role preferred over an earlier non-admin user",
			valuesFile: usersYAML("            - {username: viewer, password: v, password_hashed: false, roles: [viewer]}\n            - {username: boss, password: b, password_hashed: false, roles: [admin]}\n"),
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want:       basic("boss", "b"),
		},
		{
			name:       "password from a Secret, controller holds only the bcrypt hash",
			valuesFile: usersYAML(hashedSecretUser),
			objs:       []client.Object{apiGateway("gw", ""), credsSecret},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want:       basic("operator", "s3cret"),
		},
		{
			name:       "Secret password that does not match the hash is refused",
			valuesFile: usersYAML(hashedSecretUser),
			objs: []client.Object{apiGateway("gw", ""), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "controller-creds", Namespace: testNS},
				Data:       map[string][]byte{"password": []byte("wrong")},
			}},
			info:    &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			wantErr: "does not match the bcrypt hash",
		},
		{
			name:       "missing Secret is an error, not a fallback",
			valuesFile: usersYAML(hashedSecretUser),
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			wantErr:    "controller-creds",
		},
		{
			name:       "hashed users without password_secret_ref fail closed (no admin:admin)",
			valuesFile: usersYAML("            - {username: ops, password: \"" + string(hash) + "\", password_hashed: true, roles: [admin]}\n"),
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			wantErrIs:  ErrNoUsableCredential,
		},
		{
			name:       "empty user list fails closed",
			valuesFile: "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: true\n          users: []\n",
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			wantErrIs:  ErrNoUsableCredential,
		},
		{
			name:       "auth off on the controller: no header",
			valuesFile: "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: false\n",
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			want:       "",
		},
		{
			name:       "IdP-only controller is an explicit error",
			valuesFile: "gateway:\n  config:\n    controller:\n      auth:\n        basic:\n          enabled: false\n        idp:\n          enabled: true\n",
			objs:       []client.Object{apiGateway("gw", "")},
			info:       &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			wantErr:    "only IdP tokens",
		},
		{
			name:    "values ConfigMap named but missing is an error",
			objs:    []client.Object{apiGateway("gw", "absent")},
			info:    &registry.GatewayInfo{Name: "gw", Namespace: testNS},
			wantErr: "absent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := ""
			if tc.valuesFile != "" {
				path = writeValuesFile(t, tc.valuesFile)
			}
			got, err := ResolveControllerAuthHeader(ctx, newFakeClient(t, tc.objs...), path, tc.info)
			switch {
			case tc.wantErrIs != nil:
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErrIs), "got %v", err)
			case tc.wantErr != "":
				require.ErrorContains(t, err, tc.wantErr)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestResolveControllerAuthHeader_MissingValuesFile(t *testing.T) {
	_, err := ResolveControllerAuthHeader(context.Background(), newFakeClient(t, apiGateway("gw", "")),
		filepath.Join(t.TempDir(), "absent.yaml"), &registry.GatewayInfo{Name: "gw", Namespace: testNS})
	require.ErrorContains(t, err, "read operator values file")
}

// TestChartDefaultControllerAuthMatchesChart keeps chartDefaultControllerAuthYAML in step with the
// gateway chart: the operator relies on it to know the users Helm renders when no operator layer
// sets any.
func TestChartDefaultControllerAuthMatchesChart(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "helm", "gateway-helm-chart", "values.yaml"))
	require.NoError(t, err)
	chart := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal(data, &chart))
	chartAuth := subMap(chart, controllerAuthPath...)
	require.NotNil(t, chartAuth, "chart has no gateway.config.controller.auth")

	ours, err := parseYAMLMap(chartDefaultControllerAuthYAML)
	require.NoError(t, err)

	require.Equal(t, ours["basic"], chartAuth["basic"], "basic auth defaults drifted from the gateway chart")
	require.Equal(t, subMap(ours, "idp")["enabled"], subMap(chartAuth, "idp")["enabled"], "idp.enabled default drifted")
}
