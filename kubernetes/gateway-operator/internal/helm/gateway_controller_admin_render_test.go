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

package helm

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	adminRelease      = "gw-gw"
	adminNamespace    = "gw-ns"
	adminSecretName   = "gw-gw-gateway-controller-admin" // <fullname>-controller-admin; fullname adds the chart name
	adminSecretFile   = "gateway/templates/gateway/controller/admin-secret.yaml"
	adminConfigFile   = "gateway/templates/gateway/gateway-config.yaml"
	adminDeployFile   = "gateway/templates/gateway/controller/deployment.yaml"
	adminHashEnvToken = `password = '{{ env "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH" "" }}'`
)

// secretsProvider answers the chart's lookup calls from a fake cluster.
type secretsProvider struct{ client dynamic.Interface }

func (p secretsProvider) GetClientFor(apiVersion, kind string) (dynamic.NamespaceableResourceInterface, bool, error) {
	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	gvr := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(kind) + "s"}
	return p.client.Resource(gvr), true, nil
}

func adminValues(auth map[string]interface{}) map[string]interface{} {
	gateway := map[string]interface{}{
		"controller": map[string]interface{}{
			"encryptionKeys": map[string]interface{}{"enabled": true, "secretName": "dummy-secret"},
		},
	}
	if auth != nil {
		gateway["config"] = map[string]interface{}{"controller": map[string]interface{}{"auth": auth}}
	}
	return map[string]interface{}{"gateway": gateway}
}

// renderAdmin renders the gateway chart as release gw-gw in gw-ns. A nil cluster
// renders as `helm template` does, where lookup finds nothing.
func renderAdmin(t *testing.T, overrides map[string]interface{}, cluster []runtime.Object) map[string]string {
	t.Helper()
	chrt, err := loader.Load(gatewayHelmChartPath)
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}
	vals, err := chartutil.CoalesceValues(chrt, overrides)
	if err != nil {
		t.Fatalf("coalesce values: %v", err)
	}
	renderValues, err := chartutil.ToRenderValues(chrt, vals, chartutil.ReleaseOptions{
		Name: adminRelease, Namespace: adminNamespace, IsUpgrade: cluster != nil, IsInstall: cluster == nil,
	}, nil)
	if err != nil {
		t.Fatalf("build render values: %v", err)
	}
	var rendered map[string]string
	if cluster == nil {
		rendered, err = engine.Render(chrt, renderValues)
	} else {
		client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), cluster...)
		rendered, err = engine.RenderWithClientProvider(chrt, renderValues, secretsProvider{client})
	}
	if err != nil {
		t.Fatalf("render chart: %v", err)
	}
	return rendered
}

type renderedSecret struct {
	Metadata struct {
		Name   string            `yaml:"name"`
		Labels map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
}

func adminSecret(t *testing.T, rendered map[string]string) renderedSecret {
	t.Helper()
	var secret renderedSecret
	if err := yaml.Unmarshal([]byte(rendered[adminSecretFile]), &secret); err != nil {
		t.Fatalf("parse admin Secret: %v", err)
	}
	if secret.Metadata.Name == "" {
		t.Fatalf("no admin Secret rendered; templates: %v", keysOf(rendered))
	}
	return secret
}

func decoded(t *testing.T, secret renderedSecret, key string) string {
	t.Helper()
	v, err := base64.StdEncoding.DecodeString(secret.Data[key])
	if err != nil || len(v) == 0 {
		t.Fatalf("Secret key %q: %q (%v)", key, secret.Data[key], err)
	}
	return string(v)
}

// By default the chart generates the admin credential. The password and its hash
// appear only in the Secret; the controller config carries an env token, and the
// controller reads the hash from the Secret. The Secret carries the labels the
// operator finds it by.
func TestGatewayChartGeneratesTheControllerAdminCredential(t *testing.T) {
	rendered := renderAdmin(t, adminValues(nil), nil)
	secret := adminSecret(t, rendered)

	if secret.Metadata.Name != adminSecretName ||
		secret.Metadata.Labels["app.kubernetes.io/instance"] != adminRelease ||
		secret.Metadata.Labels["app.kubernetes.io/component"] != "controller-admin" {
		t.Errorf("Secret metadata = %+v", secret.Metadata)
	}
	password, hash := decoded(t, secret, "password"), decoded(t, secret, "password-hash")
	if len(password) != 32 {
		t.Errorf("password is %d characters, want 32", len(password))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("password-hash is not the bcrypt hash of password: %v", err)
	}
	for name, content := range rendered {
		if name == adminSecretFile {
			continue
		}
		for _, secretValue := range []string{password, hash, secret.Data["password"], secret.Data["password-hash"]} {
			if strings.Contains(content, secretValue) {
				t.Errorf("%s contains a value of the admin Secret", name)
			}
		}
		if strings.Contains(content, `password = "admin"`) {
			t.Errorf("%s still configures the admin/admin user", name)
		}
	}
	config := rendered[adminConfigFile]
	if !strings.Contains(config, "username = \"admin\"\n    "+adminHashEnvToken+"\n    password_hashed = true") {
		t.Errorf("config.toml does not configure the generated admin user:\n%s", config)
	}
	deployment := rendered[adminDeployFile]
	if !strings.Contains(deployment, "- name: APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH\n              valueFrom:\n"+
		"                secretKeyRef:\n                  name: "+adminSecretName+"\n                  key: password-hash") {
		t.Error("the controller does not read the hash from the admin Secret")
	}
}

// On an upgrade the existing Secret is read back, so repeated renders keep the same
// password and hash and every other manifest is unchanged (no pod restart).
func TestGatewayChartKeepsTheExistingControllerAdminSecret(t *testing.T) {
	first := adminSecret(t, renderAdmin(t, adminValues(nil), nil))
	existing := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": adminSecretName, "namespace": adminNamespace},
		"data":     map[string]interface{}{"password": first.Data["password"], "password-hash": first.Data["password-hash"]},
	}}

	again := renderAdmin(t, adminValues(nil), []runtime.Object{existing})
	once := renderAdmin(t, adminValues(nil), []runtime.Object{existing.DeepCopy()})
	for _, rendered := range []map[string]string{again, once} {
		got := adminSecret(t, rendered)
		if got.Data["password"] != first.Data["password"] || got.Data["password-hash"] != first.Data["password-hash"] {
			t.Fatal("an upgrade changed the existing admin credential")
		}
	}
	for name, content := range again {
		if once[name] != content {
			t.Errorf("%s differs between two upgrades of the same release", name)
		}
	}
}

// Without cluster access (helm template) every render generates a new credential;
// that is why existingSecret exists.
func TestGatewayChartTemplateRendersANewCredentialEachTime(t *testing.T) {
	a := adminSecret(t, renderAdmin(t, adminValues(nil), nil))
	b := adminSecret(t, renderAdmin(t, adminValues(nil), nil))
	if a.Data["password"] == b.Data["password"] {
		t.Error("two cluster-less renders produced the same password")
	}
}

// existingSecret: the chart renders no Secret and points the controller at the given one.
func TestGatewayChartUsesAnExistingControllerAdminSecret(t *testing.T) {
	rendered := renderAdmin(t, adminValues(map[string]interface{}{
		"basic": map[string]interface{}{"existingSecret": "my-admin"},
	}), nil)
	if strings.TrimSpace(rendered[adminSecretFile]) != "" {
		t.Error("a Secret was rendered although existingSecret is set")
	}
	if !strings.Contains(rendered[adminConfigFile], adminHashEnvToken) {
		t.Error("config.toml does not configure the admin user")
	}
	if !strings.Contains(rendered[adminDeployFile], "name: my-admin\n                  key: password-hash") {
		t.Error("the controller does not read the hash from existingSecret")
	}
}

// Explicit users render exactly as before, with no generated credential.
func TestGatewayChartRendersExplicitUsersAsBefore(t *testing.T) {
	rendered := renderAdmin(t, adminValues(map[string]interface{}{
		"basic": map[string]interface{}{
			"enabled": true,
			"users": []interface{}{map[string]interface{}{
				"username": "ops", "password": "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
				"password_hashed": true, "roles": []interface{}{"admin"},
				"passwordSecretRef": map[string]interface{}{"name": "ops-credential"},
			}},
		},
	}), nil)
	if strings.TrimSpace(rendered[adminSecretFile]) != "" {
		t.Error("a Secret was generated although users are listed")
	}
	want := "[controller.auth.basic]\n    enabled = true\n    [[controller.auth.basic.users]]\n    username = \"ops\"\n" +
		"    password = \"$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234\"\n    password_hashed = true\n    roles = [\"admin\"]\n"
	if !strings.Contains(rendered[adminConfigFile], want) {
		t.Errorf("explicit users not rendered as before:\n%s", rendered[adminConfigFile])
	}
	for name, content := range rendered {
		if strings.Contains(content, "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH") || strings.Contains(content, "ops-credential") {
			t.Errorf("%s refers to the generated credential or the operator-only passwordSecretRef", name)
		}
	}
}

// The operator chart's base gateway values no longer pin admin/admin: an
// operator-managed gateway gets the generated credential.
func TestOperatorChartValues_GenerateTheControllerAdminCredential(t *testing.T) {
	raw, err := os.ReadFile(operatorHelmChartValuesPath)
	if err != nil {
		t.Fatalf("read operator chart values: %v", err)
	}
	var operatorValues struct {
		Gateway struct {
			Values map[string]interface{} `yaml:"values"`
		} `yaml:"gateway"`
	}
	if err := yaml.Unmarshal(raw, &operatorValues); err != nil {
		t.Fatalf("parse operator chart values: %v", err)
	}
	overlay := operatorValues.Gateway.Values
	gw, _ := overlay["gateway"].(map[string]interface{})
	controller, _ := gw["controller"].(map[string]interface{})
	if controller == nil {
		controller = map[string]interface{}{}
		gw["controller"] = controller
	}
	controller["encryptionKeys"] = map[string]interface{}{"enabled": true, "secretName": "dummy-secret"}

	rendered := renderAdmin(t, overlay, nil)
	adminSecret(t, rendered)
	if strings.Contains(rendered[adminConfigFile], `password = "admin"`) {
		t.Error("operator base values still configure admin/admin")
	}
}
