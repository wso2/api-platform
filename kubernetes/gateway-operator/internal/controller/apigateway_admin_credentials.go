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

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
)

// adminCredentialsChecksumAnnotation is set on the gateway-controller pod template.
// It changes whenever the admin password Secret changes, so Helm rolls the pod:
// the controller reads its password (env var or mounted file) only at startup.
const adminCredentialsChecksumAnnotation = "checksum/admin-credentials"

// adminCredentialsSecretVersion identifies the current revision of the Secret named by
// spec.adminCredentials.passwordSecretRef, without reading its data: a hash of the
// Secret's UID and resourceVersion, or a marker when the Secret does not exist (so
// creating it later changes the value). Returns "" when adminCredentials is unset.
func adminCredentialsSecretVersion(ctx context.Context, c client.Client, gw *apiv1.APIGateway) (string, error) {
	if gw.Spec.AdminCredentials == nil {
		return "", nil
	}
	name := gw.Spec.AdminCredentials.PasswordSecretRef.Name
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: gw.Namespace, Name: name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "missing:" + name, nil
		}
		return "", fmt.Errorf("get admin credentials secret %s/%s: %w", gw.Namespace, name, err)
	}
	sum := sha256.Sum256([]byte(string(secret.UID) + "/" + secret.ResourceVersion))
	return hex.EncodeToString(sum[:]), nil
}

// gatewayConfigHash is the value stored in status.configHash. Without adminCredentials
// it is exactly the earlier ConfigMap-only hash, so existing gateways are not redeployed
// on operator upgrade. With adminCredentials it also covers the Secret's revision, so a
// password rotation triggers the existing "ConfigChanged" redeploy.
func gatewayConfigHash(configMapValues, adminSecretVersion string) string {
	if adminSecretVersion == "" {
		if configMapValues == "" {
			return ""
		}
		return auth.CalculateConfigHash(configMapValues)
	}
	return auth.CalculateConfigHash(configMapValues + "\n# adminCredentials: " + adminSecretVersion)
}

// buildAdminCredentialsOverlay returns the Helm values that configure the gateway's
// admin user from spec.adminCredentials, or "" when it is unset. It carries only the
// Secret reference and delivery mode; the gateway chart turns them into an env var or
// a read-only file mount. The password itself is never written into Helm values.
// The caller merges it after the ConfigMap values, so it replaces any basic-auth users
// configured there.
func buildAdminCredentialsOverlay(gw *apiv1.APIGateway, secretVersion string) (string, error) {
	creds := gw.Spec.AdminCredentials
	if creds == nil {
		return "", nil
	}
	if creds.PasswordSecretRef.Name == "" || creds.PasswordSecretRef.Key == "" {
		return "", fmt.Errorf("spec.adminCredentials.passwordSecretRef needs both name and key")
	}
	username := creds.Username
	if username == "" {
		username = auth.DefaultAdminUsername
	}
	source := creds.Source
	if source == "" {
		source = apiv1.AdminCredentialsSourceEnv
	}
	if source != apiv1.AdminCredentialsSourceEnv && source != apiv1.AdminCredentialsSourceFile {
		return "", fmt.Errorf("spec.adminCredentials.source must be %q or %q, got %q",
			apiv1.AdminCredentialsSourceEnv, apiv1.AdminCredentialsSourceFile, source)
	}

	overlay := map[string]interface{}{
		"gateway": map[string]interface{}{
			"config": map[string]interface{}{
				"controller": map[string]interface{}{
					"auth": map[string]interface{}{
						"basic": map[string]interface{}{
							"enabled": true,
							"users": []interface{}{
								map[string]interface{}{
									"username": username,
									"passwordSecretRef": map[string]interface{}{
										"name":   creds.PasswordSecretRef.Name,
										"key":    creds.PasswordSecretRef.Key,
										"source": source,
									},
									// The operator sends this password to the management
									// API, so it must be stored in plain text, not hashed.
									"password_hashed": false,
									"roles":           []interface{}{"admin"},
								},
							},
						},
					},
				},
			},
			"controller": map[string]interface{}{
				"deployment": map[string]interface{}{
					"podAnnotations": map[string]interface{}{
						adminCredentialsChecksumAnnotation: secretVersion,
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

// enqueueGatewaysForSecret enqueues every APIGateway whose spec.adminCredentials
// references the changed Secret.
func (r *GatewayReconciler) enqueueGatewaysForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	gatewayList := &apiv1.APIGatewayList{}
	if err := r.List(ctx, gatewayList, client.InNamespace(secret.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "failed to list APIGateways for Secret event",
			"secret", secret.Name, "namespace", secret.Namespace)
		return nil
	}
	var requests []reconcile.Request
	for i := range gatewayList.Items {
		gw := &gatewayList.Items[i]
		if gw.Spec.AdminCredentials == nil || gw.Spec.AdminCredentials.PasswordSecretRef.Name != secret.Name {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
		})
	}
	return requests
}
