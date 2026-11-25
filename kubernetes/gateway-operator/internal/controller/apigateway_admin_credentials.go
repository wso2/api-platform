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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	apiv2alpha1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v2alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
)

// basicAuthCredentialsChecksumAnnotation is set on the gateway-controller pod template.
// It changes whenever a Secret holding a basic-auth password changes, so Helm rolls the
// pod: the controller reads its passwords (env vars or mounted files) only at startup.
const basicAuthCredentialsChecksumAnnotation = "checksum/basic-auth-credentials"

// basicAuthSecretNames returns, sorted and without duplicates, the Secrets in the
// gateway's namespace whose values the gateway-controller reads as basic-auth passwords:
// spec.adminCredentials.passwordSecretRef and the passwordSecretRef of every basic-auth
// user in configRefValues (the APIGateway's configRef values), plus extra (the
// operator-generated admin credential Secret, when the gateway uses one).
func basicAuthSecretNames(gw *apiv1.APIGateway, configRefValues string, extra ...string) ([]string, error) {
	names := map[string]struct{}{}
	for _, name := range extra {
		if name != "" {
			names[name] = struct{}{}
		}
	}
	// An invalid spec.adminCredentials names no Secret here; the gateway's reconcile
	// fails closed on it.
	if creds, err := apiv2alpha1.AdminCredentialsOf(gw); err == nil && creds != nil {
		names[creds.PasswordSecretRef.Name] = struct{}{}
	}
	users, err := configRefBasicAuthUsers(configRefValues)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		user, ok := u.(map[string]interface{})
		if !ok {
			continue
		}
		ref, ok := user["passwordSecretRef"].(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := ref["name"].(string); ok && name != "" {
			names[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// basicAuthSecretsVersion identifies the current revision of every Secret named by
// basicAuthSecretNames, without reading their data: a hash over each Secret's UID and
// resourceVersion, with a marker for a Secret that does not exist (so creating it later
// changes the value). Returns "" when no Secret is referenced.
func basicAuthSecretsVersion(ctx context.Context, c client.Client, gw *apiv1.APIGateway, configRefValues string, extra ...string) (string, error) {
	names, err := basicAuthSecretNames(gw, configRefValues, extra...)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", nil
	}
	h := sha256.New()
	for _, name := range names {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: gw.Namespace, Name: name}, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				return "", fmt.Errorf("get basic-auth password secret %s/%s: %w", gw.Namespace, name, err)
			}
			fmt.Fprintf(h, "%s=missing\n", name)
			continue
		}
		fmt.Fprintf(h, "%s=%s/%s\n", name, secret.UID, secret.ResourceVersion)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// gatewayConfigHash is the value stored in status.configHash. When no basic-auth
// password comes from a Secret and the gateway has no v2alpha1-only fields it is
// exactly the earlier ConfigMap-only hash, so existing gateways are not redeployed on
// operator upgrade. Otherwise it also covers the Secrets' revisions, so a password
// rotation triggers the existing "ConfigChanged" redeploy, and the gateway's
// conversion data (see conversionDataFingerprint).
func gatewayConfigHash(configMapValues, secretsVersion, conversionData string) string {
	if secretsVersion == "" && conversionData == "" {
		if configMapValues == "" {
			return ""
		}
		return auth.CalculateConfigHash(configMapValues)
	}
	input := configMapValues
	if secretsVersion != "" {
		input += "\n# basic-auth secrets: " + secretsVersion
	}
	if conversionData != "" {
		input += "\n# conversion data: " + conversionData
	}
	return auth.CalculateConfigHash(input)
}

// conversionDataFingerprint returns the gateway's v2alpha1-only fields as stored on the
// v1 object (spec.adminCredentials today). The conversion webhook keeps them in an
// annotation, and an annotation change does not bump metadata.generation, so the
// config hash must cover them: otherwise changing only adminCredentials.username or
// .source through v2alpha1 would never redeploy the gateway.
func conversionDataFingerprint(gw *apiv1.APIGateway) string {
	return gw.Annotations[apiv2alpha1.ConversionDataAnnotation]
}

// basicAuthChecksumOverlay returns the Helm values that put secretsVersion on the
// gateway-controller pod template, or "" when it is empty.
func basicAuthChecksumOverlay(secretsVersion string) (string, error) {
	if secretsVersion == "" {
		return "", nil
	}
	out, err := yaml.Marshal(map[string]interface{}{
		"gateway": map[string]interface{}{
			"controller": map[string]interface{}{
				"deployment": map[string]interface{}{
					"podAnnotations": map[string]interface{}{
						basicAuthCredentialsChecksumAnnotation: secretsVersion,
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal basic-auth checksum overlay: %w", err)
	}
	return string(out), nil
}

// buildAdminCredentialsOverlay returns the Helm values that configure the gateway's
// admin user from spec.adminCredentials, or "" when it is unset. It carries only the
// Secret reference and delivery mode (passwordSecretRef), which the gateway chart turns
// into an env var or a read-only file mount, so it needs a chart with the
// basicAuthPasswordSecretRef feature. The password itself is never written into Helm values.
//
// The caller merges it after the ConfigMap values, and Helm values replace lists, so the
// overlay carries the complete users list: the admin user first, then every basic-auth
// user from configRefValues (the APIGateway's own configRef values) except one with the
// admin's username. Users from the operator's base values file are not carried over, so
// its default admin/admin user never comes back.
func buildAdminCredentialsOverlay(gw *apiv1.APIGateway, configRefValues string) (string, error) {
	// AdminCredentialsOf applies the CRD defaults and validation rules, so username
	// and source are set and the Secret reference is well formed.
	creds, err := apiv2alpha1.AdminCredentialsOf(gw)
	if err != nil {
		return "", err
	}
	if creds == nil {
		return "", nil
	}
	username := creds.Username
	source := creds.Source

	admin := map[string]interface{}{
		"username": username,
		// The operator sends this password to the management
		// API, so it must be stored in plain text, not hashed.
		"password_hashed": false,
		"roles":           []interface{}{"admin"},
		"passwordSecretRef": map[string]interface{}{
			"name":   creds.PasswordSecretRef.Name,
			"key":    creds.PasswordSecretRef.Key,
			"source": source,
		},
	}
	users := []interface{}{admin}
	extra, err := configRefBasicAuthUsers(configRefValues)
	if err != nil {
		return "", err
	}
	for _, u := range extra {
		if m, ok := u.(map[string]interface{}); ok && fmt.Sprint(m["username"]) == username {
			continue // spec.adminCredentials owns this username
		}
		users = append(users, u)
	}

	overlay := map[string]interface{}{
		"gateway": map[string]interface{}{
			"config": map[string]interface{}{
				"controller": map[string]interface{}{
					"auth": map[string]interface{}{
						"basic": map[string]interface{}{
							"enabled": true,
							"users":   users,
						},
					},
				},
			},
		},
	}
	out, err := yaml.Marshal(overlay)
	if err != nil {
		return "", fmt.Errorf("marshal admin credentials overlay: %w", err)
	}
	return string(out), nil
}

// configRefSecretRefNames returns the Secrets referenced by passwordSecretRef on the
// basic-auth users in valuesYAML. Only the gateway chart renders those, so they need a
// chart with the basicAuthPasswordSecretRef feature.
func configRefSecretRefNames(valuesYAML string) ([]string, error) {
	return basicAuthSecretNames(&apiv1.APIGateway{}, valuesYAML)
}

// enqueueGatewaysForSecret enqueues every APIGateway that reads a basic-auth password
// from the changed Secret, through spec.adminCredentials, a configRef user's
// passwordSecretRef, or the operator-generated admin credential.
func (r *GatewayReconciler) enqueueGatewaysForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	logger := log.FromContext(ctx)
	gatewayList := &apiv1.APIGatewayList{}
	if err := r.List(ctx, gatewayList, client.InNamespace(secret.Namespace)); err != nil {
		logger.Error(err, "failed to list APIGateways for Secret event",
			"secret", secret.Name, "namespace", secret.Namespace)
		return nil
	}
	var requests []reconcile.Request
	for i := range gatewayList.Items {
		gw := &gatewayList.Items[i]
		configRefValues := ""
		if gw.Spec.ConfigRef != nil {
			values, err := configMapValuesYAML(ctx, r.Client, gw.Spec.ConfigRef.Name, gw.Namespace)
			if err != nil {
				// The gateway's own reconcile reports an unreadable ConfigMap; here only
				// spec.adminCredentials can still be matched.
				logger.V(1).Info("cannot read configRef values for Secret event",
					"gateway", gw.Name, "namespace", gw.Namespace, "error", err.Error())
			} else {
				configRefValues = values
			}
		}
		var generated []string
		if !apiv2alpha1.HasAdminCredentials(gw) && auth.IsGeneratedAdminSecret(secret) {
			generated = append(generated, auth.GeneratedAdminSecretName(gw.Name))
		}
		names, err := basicAuthSecretNames(gw, configRefValues, generated...)
		if err != nil {
			names, _ = basicAuthSecretNames(gw, "", generated...)
		}
		if !slices.Contains(names, secret.Name) {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
		})
	}
	return requests
}

// configRefBasicAuthUsers returns gateway.config.controller.auth.basic.users from Helm
// values YAML, each entry unchanged, or nil when the values set none.
func configRefBasicAuthUsers(valuesYAML string) ([]interface{}, error) {
	if valuesYAML == "" {
		return nil, nil
	}
	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYAML), &values); err != nil {
		return nil, fmt.Errorf("parse configRef values for basic-auth users: %w", err)
	}
	node := interface{}(values)
	for _, key := range []string{"gateway", "config", "controller", "auth", "basic", "users"} {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil, nil
		}
		node = m[key]
	}
	users, _ := node.([]interface{})
	return users, nil
}
