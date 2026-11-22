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

package v1alpha2

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// An APIGateway with spec.adminCredentials survives a v1alpha2 -> v1 -> v1alpha2
// round trip unchanged.
func TestAPIGatewayRoundTripKeepsAdminCredentials(t *testing.T) {
	in := &APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: "gw1"},
		Spec: GatewaySpec{
			APISelector: APISelector{Scope: APISelectorScope("Cluster")},
			ConfigRef:   &corev1.LocalObjectReference{Name: "gateway-1-values"},
			AdminCredentials: &GatewayAdminCredentials{
				Username: "gw1-admin",
				PasswordSecretRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "gateway-1-admin"},
					Key:                  "password",
				},
				Source: AdminCredentialsSourceFile,
			},
		},
		Status: GatewayStatus{ConfigHash: "abc", ObservedGeneration: 3},
	}

	hub := &v1.APIGateway{}
	if err := in.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if hub.Spec.AdminCredentials == nil || hub.Spec.AdminCredentials.PasswordSecretRef.Name != "gateway-1-admin" ||
		hub.Spec.AdminCredentials.Source != v1.AdminCredentialsSourceFile {
		t.Fatalf("v1 lost adminCredentials: %+v", hub.Spec.AdminCredentials)
	}

	out := &APIGateway{}
	if err := out.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip changed the object:\n in: %+v\nout: %+v", in, out)
	}
}
