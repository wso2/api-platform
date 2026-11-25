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

package v2alpha1

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

func sampleCredentials() *GatewayAdminCredentials {
	return &GatewayAdminCredentials{
		Username: "gw1-admin",
		PasswordSecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "gateway-1-admin"},
			Key:                  "password",
		},
	}
}

func sampleGateway(creds *GatewayAdminCredentials) *APIGateway {
	return &APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gateway-1", Namespace: "gw1",
			Annotations: map[string]string{"team": "platform"},
		},
		Spec: GatewaySpec{
			APISelector:      APISelector{Scope: APISelectorScope("Cluster")},
			ConfigRef:        &corev1.LocalObjectReference{Name: "gateway-1-values"},
			AdminCredentials: creds,
		},
		Status: GatewayStatus{ConfigHash: "abc", ObservedGeneration: 3},
	}
}

// A v2alpha1 APIGateway survives v2alpha1 -> v1 -> v2alpha1 unchanged, with and
// without spec.adminCredentials, and the v2alpha1 view never shows the annotation.
func TestAPIGatewayRoundTrip(t *testing.T) {
	for name, creds := range map[string]*GatewayAdminCredentials{
		"with adminCredentials":    sampleCredentials(),
		"without adminCredentials": nil,
	} {
		t.Run(name, func(t *testing.T) {
			in := sampleGateway(creds)
			hub := &v1.APIGateway{}
			if err := in.ConvertTo(hub); err != nil {
				t.Fatalf("ConvertTo: %v", err)
			}
			_, annotated := hub.Annotations[ConversionDataAnnotation]
			if annotated != (creds != nil) {
				t.Fatalf("v1 conversion-data annotation present = %v, want %v", annotated, creds != nil)
			}
			if hub.Annotations["team"] != "platform" {
				t.Fatalf("v1 lost an ordinary annotation: %v", hub.Annotations)
			}

			out := &APIGateway{}
			if err := out.ConvertFrom(hub); err != nil {
				t.Fatalf("ConvertFrom: %v", err)
			}
			if !reflect.DeepEqual(in, out) {
				t.Fatalf("round trip changed the object:\n in: %+v\nout: %+v", in, out)
			}
		})
	}
}

// Conversion must not mutate its source: the conversion webhook converts the same
// decoded object to several versions.
func TestAPIGatewayConversionDoesNotMutateSource(t *testing.T) {
	in := sampleGateway(sampleCredentials())
	before := in.DeepCopy()
	hub := &v1.APIGateway{}
	if err := in.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if !reflect.DeepEqual(in, before) {
		t.Fatalf("ConvertTo mutated its source")
	}

	hubBefore := hub.DeepCopy()
	if err := (&APIGateway{}).ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if !reflect.DeepEqual(hub, hubBefore) {
		t.Fatalf("ConvertFrom mutated its source")
	}
}

// Clearing spec.adminCredentials through v2alpha1 removes the annotation from v1.
func TestAPIGatewayClearingAdminCredentialsRemovesAnnotation(t *testing.T) {
	hub := &v1.APIGateway{}
	if err := sampleGateway(sampleCredentials()).ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	// A v2alpha1 client reads the object, drops adminCredentials, and writes it back.
	edited := &APIGateway{}
	if err := edited.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	edited.Spec.AdminCredentials = nil
	stored := &v1.APIGateway{}
	if err := edited.ConvertTo(stored); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if _, ok := stored.Annotations[ConversionDataAnnotation]; ok {
		t.Fatalf("annotation kept after clearing adminCredentials: %v", stored.Annotations)
	}
}

// An unparseable annotation (written through v1) never fails the conversion, which
// would break every v2alpha1 LIST containing the object. It stays visible on the
// v2alpha1 object, adminCredentials stays unset, and a v2alpha1 write keeps it.
func TestAPIGatewayInvalidAnnotationIsKept(t *testing.T) {
	hub := &v1.APIGateway{ObjectMeta: metav1.ObjectMeta{
		Name: "gateway-1", Namespace: "gw1",
		Annotations: map[string]string{ConversionDataAnnotation: "{not json"},
	}}
	out := &APIGateway{}
	if err := out.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom failed on an invalid annotation: %v", err)
	}
	if out.Spec.AdminCredentials != nil {
		t.Fatalf("adminCredentials set from an invalid annotation: %+v", out.Spec.AdminCredentials)
	}
	if out.Annotations[ConversionDataAnnotation] != "{not json" {
		t.Fatalf("invalid annotation hidden from the v2alpha1 view: %v", out.Annotations)
	}

	back := &v1.APIGateway{}
	if err := out.ConvertTo(back); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if back.Annotations[ConversionDataAnnotation] != "{not json" {
		t.Fatalf("invalid annotation dropped on a v2alpha1 write: %v", back.Annotations)
	}

	// The operator treats the gateway as having adminCredentials and refuses it.
	if !HasAdminCredentials(back) {
		t.Fatal("HasAdminCredentials = false for an invalid annotation; the gateway would switch credentials")
	}
	if _, err := AdminCredentialsOf(back); err == nil {
		t.Fatal("AdminCredentialsOf accepted an invalid annotation")
	}
}

// Reading and writing through v1alpha1 (the API server converts v1alpha1 <-> v1 with
// the same webhook) keeps spec.adminCredentials set through v2alpha1.
func TestAPIGatewayV1alpha1WriteKeepsAdminCredentials(t *testing.T) {
	stored := &v1.APIGateway{}
	if err := sampleGateway(sampleCredentials()).ConvertTo(stored); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}

	alpha := &v1alpha1.APIGateway{}
	if err := alpha.ConvertFrom(stored); err != nil {
		t.Fatalf("v1alpha1 ConvertFrom: %v", err)
	}
	alpha.Labels = map[string]string{"edited": "through-v1alpha1"}
	restored := &v1.APIGateway{}
	if err := alpha.ConvertTo(restored); err != nil {
		t.Fatalf("v1alpha1 ConvertTo: %v", err)
	}

	got, err := AdminCredentialsOf(restored)
	if err != nil {
		t.Fatalf("AdminCredentialsOf: %v", err)
	}
	if !reflect.DeepEqual(got, sampleCredentials()) {
		t.Fatalf("adminCredentials after a v1alpha1 write = %+v, want %+v", got, sampleCredentials())
	}
}

// AdminCredentialsOf applies the CRD defaults to a value written without them (for
// example straight into the annotation through v1).
func TestAdminCredentialsOfAppliesDefaults(t *testing.T) {
	gw := &v1.APIGateway{}
	SetAdminCredentials(gw, &GatewayAdminCredentials{
		PasswordSecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "gateway-1-admin"},
			Key:                  "password",
		},
	})
	got, err := AdminCredentialsOf(gw)
	if err != nil {
		t.Fatalf("AdminCredentialsOf: %v", err)
	}
	if got.Username != DefaultAdminUsername {
		t.Fatalf("defaults not applied: %+v", got)
	}

	none, err := AdminCredentialsOf(&v1.APIGateway{})
	if err != nil || none != nil {
		t.Fatalf("gateway without adminCredentials: got %+v, %v", none, err)
	}
	if HasAdminCredentials(&v1.APIGateway{}) {
		t.Fatal("HasAdminCredentials = true for a gateway without adminCredentials")
	}
}

// The username limit counts characters, like the CRD's maxLength: 128 "é" (256 bytes)
// passes admission, so the operator must accept it too.
func TestValidateCountsUsernameCharacters(t *testing.T) {
	c := sampleCredentials()
	c.Username = strings.Repeat("é", 128)
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate rejected a 128-character username: %v", err)
	}
	c.Username = strings.Repeat("é", 254)
	if err := c.Validate(); err == nil {
		t.Fatal("Validate accepted a 254-character username")
	}
}
