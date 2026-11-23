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
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
)

// defaultAdminMode is where the admin credential of a gateway without
// spec.adminCredentials comes from.
type defaultAdminMode string

const (
	// defaultAdminNotNeeded: spec.adminCredentials is set, the values configure basic-auth
	// users, or basic auth is disabled. The operator adds no admin user.
	defaultAdminNotNeeded defaultAdminMode = "notNeeded"
	// defaultAdminGenerated: the operator-generated Secret (auth.GeneratedAdminSecretName),
	// created on the gateway's first deployment and reused afterwards.
	defaultAdminGenerated defaultAdminMode = "generated"
	// defaultAdminLegacy: a gateway an earlier operator deployed with the default
	// admin/admin user keeps it, so its credential does not change on operator upgrade.
	defaultAdminLegacy defaultAdminMode = "legacy"
)

// defaultAdminPlan is the outcome of planDefaultAdminCredentials.
type defaultAdminPlan struct {
	Mode       defaultAdminMode
	SecretName string // generated mode only
	Created    bool   // the Secret was created by this call
}

// releaseValuesFunc returns the values of the gateway's existing Helm release and whether
// it exists.
type releaseValuesFunc func() (map[string]interface{}, bool, error)

type defaultAdminInput struct {
	Client client.Client
	Scheme *runtime.Scheme
	// Owner owns the generated Secret (an APIGateway or a Gateway), so it is deleted
	// with the gateway.
	Owner       client.Object
	GatewayName string
	Namespace   string
	// HasSpecCredentials is true when the APIGateway sets spec.adminCredentials.
	HasSpecCredentials bool
	// BaseValuesYAML is the operator's base values file; ValuesYAML the gateway's own
	// values (configRef or parametersRef ConfigMap, with the operator's overlays).
	BaseValuesYAML string
	ValuesYAML     string
	ReleaseValues  releaseValuesFunc
}

// planDefaultAdminCredentials decides the admin credential of a gateway that has no
// spec.adminCredentials and no basic-auth users in its values, creating the generated
// Secret when needed:
//
//   - the generated Secret exists: reuse it;
//   - no Helm release exists yet (a new gateway): generate a password into a new Secret;
//   - a release deployed by an earlier operator (its values carry the default admin/admin
//     user, or no users at all, so the chart's admin/admin default applied): keep
//     admin/admin;
//   - a release that used the generated Secret, which is now gone: fail. The operator
//     never generates a different credential for a deployed gateway, nor falls back to
//     admin/admin.
//
// Any other release (its values had other users, since removed) gets a generated
// credential, as a new gateway would.
func planDefaultAdminCredentials(ctx context.Context, in defaultAdminInput) (defaultAdminPlan, error) {
	notNeeded := defaultAdminPlan{Mode: defaultAdminNotNeeded}
	if in.HasSpecCredentials {
		return notNeeded, nil
	}
	enabled, users, err := effectiveBasicAuth(in.BaseValuesYAML, in.ValuesYAML)
	if err != nil {
		return defaultAdminPlan{}, err
	}
	if !enabled || len(users) > 0 {
		return notNeeded, nil
	}

	secretName := auth.GeneratedAdminSecretName(in.GatewayName)
	generated := defaultAdminPlan{Mode: defaultAdminGenerated, SecretName: secretName}
	secret, err := auth.GetGeneratedAdminSecret(ctx, in.Client, in.Namespace, in.GatewayName)
	if err != nil {
		return defaultAdminPlan{}, err
	}
	if secret != nil {
		return generated, nil
	}

	previous, deployed, err := in.ReleaseValues()
	if err != nil {
		return defaultAdminPlan{}, fmt.Errorf("read the gateway's Helm release: %w", err)
	}
	if deployed {
		prevUsers := basicAuthUsersIn(previous)
		if usersReferenceSecret(prevUsers, secretName) {
			return defaultAdminPlan{}, fmt.Errorf("the gateway was deployed with the operator-generated admin credential in Secret %s/%s, "+
				"which no longer exists; restore it or set spec.adminCredentials (the operator does not generate a different credential for a deployed gateway)",
				in.Namespace, secretName)
		}
		if len(prevUsers) == 0 || usersHaveDefaultAdmin(prevUsers) {
			return defaultAdminPlan{Mode: defaultAdminLegacy}, nil
		}
	}

	if err := createGeneratedAdminSecret(ctx, in, secretName); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return defaultAdminPlan{}, err
		}
		// Created concurrently: use it if it is ours.
		if _, err := auth.GetGeneratedAdminSecret(ctx, in.Client, in.Namespace, in.GatewayName); err != nil {
			return defaultAdminPlan{}, err
		}
		return generated, nil
	}
	generated.Created = true
	return generated, nil
}

func createGeneratedAdminSecret(ctx context.Context, in defaultAdminInput, name string) error {
	password, err := auth.GeneratePassword()
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   in.Namespace,
			Labels:      map[string]string{auth.GeneratedAdminCredentialsLabel: auth.GeneratedAdminCredentialsLabelValue},
			Annotations: map[string]string{auth.GeneratedAdminCredentialsGatewayAnnotation: in.GatewayName},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			auth.GeneratedAdminUsernameKey: []byte(auth.DefaultAdminUsername),
			auth.GeneratedAdminPasswordKey: []byte(password),
		},
	}
	if in.Owner != nil && in.Scheme != nil {
		if err := controllerutil.SetOwnerReference(in.Owner, secret, in.Scheme); err != nil {
			return fmt.Errorf("set owner of generated admin credentials secret: %w", err)
		}
	}
	if err := in.Client.Create(ctx, secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return err
		}
		return fmt.Errorf("create generated admin credentials secret %s/%s: %w", in.Namespace, name, err)
	}
	return nil
}

// defaultAdminOverlay returns the Helm values for plan. Every mode turns off the gateway
// chart's own credential generation (gateway.controller.adminCredentials.generate), so
// the chart never generates a password the operator does not know; charts that predate
// the setting ignore it. A legacy gateway keeps the admin/admin user earlier operators
// deployed. A generated credential is read from its Secret as a mounted file
// (passwordSecretRef), so it needs a chart with the basicAuthPasswordSecretRef feature.
func defaultAdminOverlay(plan defaultAdminPlan) (string, error) {
	values := map[string]interface{}{
		"gateway": map[string]interface{}{
			"controller": map[string]interface{}{
				"adminCredentials": map[string]interface{}{"generate": false},
			},
		},
	}
	var user map[string]interface{}
	switch plan.Mode {
	case defaultAdminGenerated:
		user = map[string]interface{}{
			"username":        auth.DefaultAdminUsername,
			"password_hashed": false,
			"roles":           []interface{}{"admin"},
			"passwordSecretRef": map[string]interface{}{
				"name":   plan.SecretName,
				"key":    auth.GeneratedAdminPasswordKey,
				"source": apiv1.AdminCredentialsSourceFile,
			},
		}
	case defaultAdminLegacy:
		username, password := auth.GetDefaultBasicAuthCredentials()
		user = map[string]interface{}{
			"username":        username,
			"password":        password,
			"password_hashed": false,
			"roles":           []interface{}{"admin"},
		}
	}
	if user != nil {
		values["gateway"].(map[string]interface{})["config"] = map[string]interface{}{
			"controller": map[string]interface{}{
				"auth": map[string]interface{}{
					"basic": map[string]interface{}{
						"enabled": true,
						"users":   []interface{}{user},
					},
				},
			},
		}
	}
	out, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal admin credentials overlay: %w", err)
	}
	return string(out), nil
}

// readBaseValuesFile returns the operator's base Helm values file, or "" when none is
// configured or it does not exist (the Helm client treats a missing file the same way).
func readBaseValuesFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read operator base values file: %w", err)
	}
	return string(data), nil
}

// effectiveBasicAuth returns whether basic auth is enabled and which users it has once
// the gateway's values are merged over the operator's base values: a key in the
// gateway's values wins, a list replaces rather than merges (as in Helm), and basic auth
// is enabled unless set otherwise (the gateway chart's default).
func effectiveBasicAuth(baseYAML, valuesYAML string) (bool, []interface{}, error) {
	enabled := true
	var users []interface{}
	for _, doc := range []string{baseYAML, valuesYAML} {
		basic, err := basicAuthBlock(doc)
		if err != nil {
			return false, nil, err
		}
		if v, ok := basic["enabled"].(bool); ok {
			enabled = v
		}
		if u, ok := basic["users"]; ok {
			users, _ = u.([]interface{})
		}
	}
	return enabled, users, nil
}

func basicAuthBlock(valuesYAML string) (map[string]interface{}, error) {
	if valuesYAML == "" {
		return nil, nil
	}
	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYAML), &values); err != nil {
		return nil, fmt.Errorf("parse Helm values for basic auth: %w", err)
	}
	return basicAuthMap(values), nil
}

func basicAuthMap(values map[string]interface{}) map[string]interface{} {
	node := interface{}(values)
	for _, key := range []string{"gateway", "config", "controller", "auth", "basic"} {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil
		}
		node = m[key]
	}
	basic, _ := node.(map[string]interface{})
	return basic
}

func basicAuthUsersIn(values map[string]interface{}) []interface{} {
	users, _ := basicAuthMap(values)["users"].([]interface{})
	return users
}

func usersReferenceSecret(users []interface{}, secretName string) bool {
	for _, u := range users {
		user, ok := u.(map[string]interface{})
		if !ok {
			continue
		}
		if ref, ok := user["passwordSecretRef"].(map[string]interface{}); ok && fmt.Sprint(ref["name"]) == secretName {
			return true
		}
	}
	return false
}

func usersHaveDefaultAdmin(users []interface{}) bool {
	username, password := auth.GetDefaultBasicAuthCredentials()
	for _, u := range users {
		user, ok := u.(map[string]interface{})
		if !ok {
			continue
		}
		if fmt.Sprint(user["username"]) == username && fmt.Sprint(user["password"]) == password &&
			fmt.Sprint(user["password_hashed"]) != "true" {
			return true
		}
	}
	return false
}
