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
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/secretsource"
)

// chartDefaultControllerAuthYAML mirrors gateway.config.controller.auth in the gateway Helm
// chart's own values.yaml (kubernetes/helm/gateway-helm-chart/values.yaml). It is the lowest
// layer of the values a gateway is rendered with: Helm applies it under every operator layer.
// TestChartDefaultControllerAuthMatchesChart fails if the two drift apart.
const chartDefaultControllerAuthYAML = `
basic:
  enabled: true
  users:
    - username: "admin"
      password: "admin"
      password_hashed: false
      roles: ["admin"]
idp:
  enabled: false
`

// controllerAuthPath is where the controller's auth block sits in the gateway Helm values.
var controllerAuthPath = []string{"gateway", "config", "controller", "auth"}

// ErrNoUsableCredential is returned when basic auth is enabled on a gateway controller but no
// configured user carries a credential the operator can send. The operator then refuses to call
// the controller instead of guessing a password.
var ErrNoUsableCredential = errors.New("no controller user with a usable credential")

// ResolveControllerAuthHeader returns the Authorization header value the operator sends to a
// gateway controller's management REST API, or "" when the controller has authentication off.
//
// The credential is read from the same layers the controller's config.toml is rendered from,
// merged the same way: the chart defaults, then the operator values file (valuesFilePath), then
// the gateway's own values ConfigMap (spec.configRef, or the Gateway API values annotation).
// Maps merge key by key and a higher layer replaces lists such as users, so the operator always
// authenticates with a user the controller actually has, whichever layer set it.
func ResolveControllerAuthHeader(ctx context.Context, k8sClient client.Client, valuesFilePath string, info *registry.GatewayInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("resolve controller credentials: no gateway")
	}
	settings, err := EffectiveControllerAuth(ctx, k8sClient, valuesFilePath, info)
	if err != nil {
		return "", err
	}
	username, password, err := selectCredential(ctx, k8sClient, info.Namespace, settings)
	if err != nil {
		return "", fmt.Errorf("gateway %s/%s: %w", info.Namespace, info.Name, err)
	}
	if username == "" {
		return "", nil
	}
	return "Basic " + EncodeBasicAuth(username, password), nil
}

// EffectiveControllerAuth returns the controller auth settings a gateway is rendered with:
// chart defaults < operator values file < the gateway's values ConfigMap.
func EffectiveControllerAuth(ctx context.Context, k8sClient client.Client, valuesFilePath string, info *registry.GatewayInfo) (*AuthSettings, error) {
	merged, err := parseYAMLMap(chartDefaultControllerAuthYAML)
	if err != nil {
		return nil, fmt.Errorf("parse chart default controller auth: %w", err)
	}

	if valuesFilePath != "" {
		data, err := os.ReadFile(valuesFilePath)
		if err != nil {
			return nil, fmt.Errorf("read operator values file %q: %w", valuesFilePath, err)
		}
		fileValues, err := parseYAMLMap(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse operator values file %q: %w", valuesFilePath, err)
		}
		merged = mergeAuthMaps(merged, subMap(fileValues, controllerAuthPath...))
	}

	gwValuesYAML, err := gatewayValuesYAML(ctx, k8sClient, info)
	if err != nil {
		return nil, err
	}
	if gwValuesYAML != "" {
		gwValues, err := parseYAMLMap(gwValuesYAML)
		if err != nil {
			return nil, fmt.Errorf("parse values ConfigMap of gateway %s/%s: %w", info.Namespace, info.Name, err)
		}
		merged = mergeAuthMaps(merged, subMap(gwValues, controllerAuthPath...))
	}

	out, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshal effective controller auth: %w", err)
	}
	var settings AuthSettings
	if err := yaml.Unmarshal(out, &settings); err != nil {
		return nil, fmt.Errorf("decode effective controller auth: %w", err)
	}
	return &settings, nil
}

// selectCredential picks the user the operator authenticates as. It prefers a user holding the
// admin role, then any other user, and takes the first one whose password it can obtain:
// password_secret_ref (read from a Secret in the gateway's namespace), or a plaintext password.
// A user whose password is only a hash is skipped: the operator cannot send it. When basic auth
// is enabled and no user qualifies it returns ErrNoUsableCredential; there is no default.
func selectCredential(ctx context.Context, k8sClient client.Client, namespace string, s *AuthSettings) (string, string, error) {
	if !s.Basic.Enabled {
		if s.IDP.Enabled {
			return "", "", fmt.Errorf("controller accepts only IdP tokens (basic auth disabled); the operator authenticates with basic auth")
		}
		return "", "", nil
	}

	ordered := make([]AuthUser, 0, len(s.Basic.Users))
	for _, u := range s.Basic.Users {
		if hasRole(u, "admin") {
			ordered = append(ordered, u)
		}
	}
	for _, u := range s.Basic.Users {
		if !hasRole(u, "admin") {
			ordered = append(ordered, u)
		}
	}

	for _, u := range ordered {
		if u.Username == "" {
			continue
		}
		if u.PasswordSecretRef != nil {
			field := fmt.Sprintf("gateway.config.controller.auth.basic.users[%s].password_secret_ref", u.Username)
			if u.PasswordSecretRef.Name == "" || u.PasswordSecretRef.Key == "" {
				return "", "", fmt.Errorf("%s: name and key are required", field)
			}
			password, err := secretsource.Resolve(ctx, k8sClient, field, apiv1.SecretValueSource{
				ValueFrom: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: u.PasswordSecretRef.Name},
					Key:                  u.PasswordSecretRef.Key,
				},
			}, namespace)
			if err != nil {
				return "", "", err
			}
			if password == "" {
				return "", "", fmt.Errorf("%s: Secret %s/%s key %q is empty", field, namespace, u.PasswordSecretRef.Name, u.PasswordSecretRef.Key)
			}
			if u.PasswordHashed && u.Password != "" {
				if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
					return "", "", fmt.Errorf("%s: the Secret's password does not match the bcrypt hash configured for user %q", field, u.Username)
				}
			}
			return u.Username, password, nil
		}
		if !u.PasswordHashed && u.Password != "" {
			return u.Username, u.Password, nil
		}
	}
	return "", "", fmt.Errorf("%w: basic auth is enabled but every user is hashed without password_secret_ref, or has no password; "+
		"add password_secret_ref to a user in gateway.config.controller.auth.basic.users", ErrNoUsableCredential)
}

func hasRole(u AuthUser, role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// gatewayValuesYAML returns the per-gateway values.yaml (the highest operator layer), or "".
func gatewayValuesYAML(ctx context.Context, k8sClient client.Client, info *registry.GatewayInfo) (string, error) {
	cmName := info.HelmValuesConfigMapName
	if cmName == "" {
		gw := &apiv1.APIGateway{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: info.Namespace, Name: info.Name}, gw); err != nil {
			if apierrors.IsNotFound(err) {
				return "", nil
			}
			return "", fmt.Errorf("get APIGateway %s/%s: %w", info.Namespace, info.Name, err)
		}
		if gw.Spec.ConfigRef == nil {
			return "", nil
		}
		cmName = gw.Spec.ConfigRef.Name
	}
	cm := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: info.Namespace, Name: cmName}, cm); err != nil {
		return "", fmt.Errorf("get values ConfigMap %s/%s: %w", info.Namespace, cmName, err)
	}
	return cm.Data["values.yaml"], nil
}

func parseYAMLMap(s string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if s == "" {
		return out, nil
	}
	if err := yaml.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// subMap walks nested maps along path and returns the map found there, or nil.
func subMap(m map[string]interface{}, path ...string) map[string]interface{} {
	cur := m
	for _, k := range path {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

// mergeAuthMaps merges override into base with the operator's Helm values semantics
// (internal/helm deepMergeValues): nested maps merge key by key, any other value (lists
// included) replaces, and an explicit null removes the key.
func mergeAuthMaps(base, override map[string]interface{}) map[string]interface{} {
	if len(override) == 0 {
		return base
	}
	out := make(map[string]interface{}, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		if v == nil {
			delete(out, k)
			continue
		}
		bm, bOk := out[k].(map[string]interface{})
		vm, vOk := v.(map[string]interface{})
		if bOk && vOk {
			out[k] = mergeAuthMaps(bm, vm)
			continue
		}
		out[k] = v
	}
	return out
}
