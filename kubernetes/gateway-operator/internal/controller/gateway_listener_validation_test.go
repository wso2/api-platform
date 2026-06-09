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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func findListenerCondition(conditions []metav1.Condition, condType gatewayv1.ListenerConditionType) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == string(condType) {
			return &conditions[i]
		}
	}
	return nil
}

func generateTestTLSCertKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func TestEvaluateListeners(t *testing.T) {
	tests := []struct {
		name              string
		listeners         []gatewayv1.Listener
		secrets           []corev1.Secret
		wantHasAccepted   bool
		wantStatusCount   int
		wantAcceptedCount int
		checkResolvedRefs bool
	}{
		{
			name: "all HTTP listeners accepted",
			listeners: []gatewayv1.Listener{
				{
					Name:     "http",
					Protocol: gatewayv1.HTTPProtocolType,
					Port:     80,
				},
			},
			wantHasAccepted:   true,
			wantStatusCount:   1,
			wantAcceptedCount: 1,
			checkResolvedRefs: true,
		},
		{
			name: "all HTTPS listeners accepted",
			listeners: []gatewayv1.Listener{
				{
					Name:     "https",
					Protocol: gatewayv1.HTTPSProtocolType,
					Port:     443,
				},
			},
			wantHasAccepted:   true,
			wantStatusCount:   1,
			wantAcceptedCount: 1,
			checkResolvedRefs: true,
		},
		{
			name: "mixed HTTP and HTTPS listeners accepted",
			listeners: []gatewayv1.Listener{
				{
					Name:     "http",
					Protocol: gatewayv1.HTTPProtocolType,
					Port:     80,
				},
				{
					Name:     "https",
					Protocol: gatewayv1.HTTPSProtocolType,
					Port:     443,
				},
			},
			wantHasAccepted:   true,
			wantStatusCount:   2,
			wantAcceptedCount: 2,
			checkResolvedRefs: true,
		},
		{
			name: "all unsupported TLS listeners rejected",
			listeners: []gatewayv1.Listener{
				{
					Name:     "tls",
					Protocol: gatewayv1.TLSProtocolType,
					Port:     443,
				},
			},
			wantHasAccepted:   false,
			wantStatusCount:   1,
			wantAcceptedCount: 0,
			checkResolvedRefs: false,
		},
		{
			name: "all unsupported TCP listeners rejected",
			listeners: []gatewayv1.Listener{
				{
					Name:     "tcp",
					Protocol: gatewayv1.TCPProtocolType,
					Port:     8080,
				},
			},
			wantHasAccepted:   false,
			wantStatusCount:   1,
			wantAcceptedCount: 0,
			checkResolvedRefs: false,
		},
		{
			name: "all unsupported UDP listeners rejected",
			listeners: []gatewayv1.Listener{
				{
					Name:     "udp",
					Protocol: gatewayv1.UDPProtocolType,
					Port:     53,
				},
			},
			wantHasAccepted:   false,
			wantStatusCount:   1,
			wantAcceptedCount: 0,
			checkResolvedRefs: false,
		},
		{
			name: "mixed supported and unsupported listeners",
			listeners: []gatewayv1.Listener{
				{
					Name:     "http",
					Protocol: gatewayv1.HTTPProtocolType,
					Port:     80,
				},
				{
					Name:     "tcp",
					Protocol: gatewayv1.TCPProtocolType,
					Port:     8080,
				},
			},
			wantHasAccepted:   true,
			wantStatusCount:   2,
			wantAcceptedCount: 1,
			checkResolvedRefs: false,
		},
		{
			name: "multiple unsupported protocols",
			listeners: []gatewayv1.Listener{
				{
					Name:     "tls",
					Protocol: gatewayv1.TLSProtocolType,
					Port:     443,
				},
				{
					Name:     "tcp",
					Protocol: gatewayv1.TCPProtocolType,
					Port:     8080,
				},
			},
			wantHasAccepted:   false,
			wantStatusCount:   2,
			wantAcceptedCount: 0,
			checkResolvedRefs: false,
		},
		{
			name:              "empty listeners",
			listeners:         []gatewayv1.Listener{},
			wantHasAccepted:   false,
			wantStatusCount:   0,
			wantAcceptedCount: 0,
			checkResolvedRefs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-gateway",
					Namespace:  "default",
					Generation: 1,
				},
				Spec: gatewayv1.GatewaySpec{
					Listeners: tt.listeners,
				},
			}

			// Create fake client with secrets
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = gatewayv1.Install(scheme)
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			for i := range tt.secrets {
				clientBuilder = clientBuilder.WithObjects(&tt.secrets[i])
			}
			cl := clientBuilder.Build()

			statuses, hasAccepted, err := evaluateListeners(context.Background(), cl, gw)
			if err != nil {
				t.Fatalf("evaluateListeners returned error: %v", err)
			}

			if hasAccepted != tt.wantHasAccepted {
				t.Errorf("hasAccepted = %v, want %v", hasAccepted, tt.wantHasAccepted)
			}

			if len(statuses) != tt.wantStatusCount {
				t.Errorf("len(statuses) = %d, want %d", len(statuses), tt.wantStatusCount)
			}

			acceptedCount := 0
			for i, status := range statuses {
				if status.Name != tt.listeners[i].Name {
					t.Errorf("status[%d].Name = %q, want %q", i, status.Name, tt.listeners[i].Name)
				}

				// Expect Accepted, ResolvedRefs, and Programmed conditions.
				if len(status.Conditions) != 3 {
					t.Errorf("status[%d] has %d conditions, want 3 (Accepted, ResolvedRefs, Programmed)", i, len(status.Conditions))
					continue
				}

				acceptedCond := findListenerCondition(status.Conditions, gatewayv1.ListenerConditionAccepted)
				resolvedRefsCond := findListenerCondition(status.Conditions, gatewayv1.ListenerConditionResolvedRefs)
				programmedCond := findListenerCondition(status.Conditions, gatewayv1.ListenerConditionProgrammed)
				if acceptedCond == nil || resolvedRefsCond == nil || programmedCond == nil {
					t.Fatalf("status[%d] missing expected listener conditions: %+v", i, status.Conditions)
				}

				if acceptedCond.Status == metav1.ConditionTrue {
					acceptedCount++
					if acceptedCond.Reason != string(gatewayv1.ListenerReasonAccepted) {
						t.Errorf("status[%d] Accepted.Reason = %q, want %q", i, acceptedCond.Reason, gatewayv1.ListenerReasonAccepted)
					}
					if len(status.SupportedKinds) == 0 {
						t.Errorf("status[%d].SupportedKinds is empty for accepted listener", i)
					}
					if tt.checkResolvedRefs && resolvedRefsCond.Status == metav1.ConditionTrue {
						if programmedCond.Status != metav1.ConditionTrue {
							t.Errorf("status[%d] Programmed.Status = %v, want True when ResolvedRefs is True", i, programmedCond.Status)
						}
						if programmedCond.Reason != string(gatewayv1.ListenerReasonProgrammed) {
							t.Errorf("status[%d] Programmed.Reason = %q, want %q", i, programmedCond.Reason, gatewayv1.ListenerReasonProgrammed)
						}
					}
					if tt.checkResolvedRefs && resolvedRefsCond.Status == metav1.ConditionFalse {
						if programmedCond.Status != metav1.ConditionFalse {
							t.Errorf("status[%d] Programmed.Status = %v, want False when ResolvedRefs is False", i, programmedCond.Status)
						}
					}
				} else {
					if acceptedCond.Reason != string(gatewayv1.ListenerReasonUnsupportedProtocol) {
						t.Errorf("status[%d] Accepted.Reason = %q, want %q", i, acceptedCond.Reason, gatewayv1.ListenerReasonUnsupportedProtocol)
					}
					if len(status.SupportedKinds) != 0 {
						t.Errorf("status[%d].SupportedKinds should be empty for rejected listener, got %v", i, status.SupportedKinds)
					}
					if programmedCond.Status != metav1.ConditionFalse {
						t.Errorf("status[%d] Programmed.Status = %v, want False for unsupported protocol", i, programmedCond.Status)
					}
				}

				if tt.checkResolvedRefs && acceptedCond.Status == metav1.ConditionTrue {
					if resolvedRefsCond.Status != metav1.ConditionTrue {
						t.Errorf("status[%d] ResolvedRefs.Status = %v, want ConditionTrue for accepted listener with no issues", i, resolvedRefsCond.Status)
					}
				}
			}

			if acceptedCount != tt.wantAcceptedCount {
				t.Errorf("acceptedCount = %d, want %d", acceptedCount, tt.wantAcceptedCount)
			}
		})
	}
}

func TestHasAllListenersAccepted(t *testing.T) {
	tests := []struct {
		name             string
		listenerStatuses []gatewayv1.ListenerStatus
		want             bool
	}{
		{
			name: "all listeners accepted",
			listenerStatuses: []gatewayv1.ListenerStatus{
				{
					Name: "http",
					Conditions: []metav1.Condition{{
						Type:   string(gatewayv1.ListenerConditionAccepted),
						Status: metav1.ConditionTrue,
						Reason: string(gatewayv1.ListenerReasonAccepted),
					}},
				},
				{
					Name: "https",
					Conditions: []metav1.Condition{{
						Type:   string(gatewayv1.ListenerConditionAccepted),
						Status: metav1.ConditionTrue,
						Reason: string(gatewayv1.ListenerReasonAccepted),
					}},
				},
			},
			want: true,
		},
		{
			name: "one listener rejected",
			listenerStatuses: []gatewayv1.ListenerStatus{
				{
					Name: "http",
					Conditions: []metav1.Condition{{
						Type:   string(gatewayv1.ListenerConditionAccepted),
						Status: metav1.ConditionTrue,
						Reason: string(gatewayv1.ListenerReasonAccepted),
					}},
				},
				{
					Name: "tcp",
					Conditions: []metav1.Condition{{
						Type:   string(gatewayv1.ListenerConditionAccepted),
						Status: metav1.ConditionFalse,
						Reason: string(gatewayv1.ListenerReasonUnsupportedProtocol),
					}},
				},
			},
			want: false,
		},
		{
			name: "all listeners rejected",
			listenerStatuses: []gatewayv1.ListenerStatus{
				{
					Name: "tcp",
					Conditions: []metav1.Condition{{
						Type:   string(gatewayv1.ListenerConditionAccepted),
						Status: metav1.ConditionFalse,
						Reason: string(gatewayv1.ListenerReasonUnsupportedProtocol),
					}},
				},
			},
			want: false,
		},
		{
			name:             "empty listener statuses",
			listenerStatuses: []gatewayv1.ListenerStatus{},
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAllListenersAccepted(tt.listenerStatuses)
			if got != tt.want {
				t.Errorf("hasAllListenersAccepted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAllowedRouteKinds(t *testing.T) {
	gwGroup := gatewayv1.Group(gatewayv1.GroupVersion.Group)
	tests := []struct {
		name               string
		listener           gatewayv1.Listener
		wantStatus         metav1.ConditionStatus
		wantReason         string
		wantSupportedCount int
	}{
		{
			name: "no allowedRoutes specified - defaults accepted",
			listener: gatewayv1.Listener{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     80,
			},
			wantStatus:         metav1.ConditionTrue,
			wantReason:         string(gatewayv1.ListenerReasonResolvedRefs),
			wantSupportedCount: 1,
		},
		{
			name: "allowedRoutes with valid HTTPRoute kind",
			listener: gatewayv1.Listener{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     80,
				AllowedRoutes: &gatewayv1.AllowedRoutes{
					Kinds: []gatewayv1.RouteGroupKind{
						{Group: &gwGroup, Kind: "HTTPRoute"},
					},
				},
			},
			wantStatus:         metav1.ConditionTrue,
			wantReason:         string(gatewayv1.ListenerReasonResolvedRefs),
			wantSupportedCount: 1,
		},
		{
			name: "allowedRoutes with invalid kind",
			listener: gatewayv1.Listener{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     80,
				AllowedRoutes: &gatewayv1.AllowedRoutes{
					Kinds: []gatewayv1.RouteGroupKind{
						{Group: &gwGroup, Kind: "InvalidRoute"},
					},
				},
			},
			wantStatus:         metav1.ConditionFalse,
			wantReason:         string(gatewayv1.ListenerReasonInvalidRouteKinds),
			wantSupportedCount: 0,
		},
		{
			name: "allowedRoutes with mixed valid and invalid kinds",
			listener: gatewayv1.Listener{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     80,
				AllowedRoutes: &gatewayv1.AllowedRoutes{
					Kinds: []gatewayv1.RouteGroupKind{
						{Group: &gwGroup, Kind: "HTTPRoute"},
						{Group: &gwGroup, Kind: "InvalidRoute"},
					},
				},
			},
			wantStatus:         metav1.ConditionFalse,
			wantReason:         string(gatewayv1.ListenerReasonInvalidRouteKinds),
			wantSupportedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-gateway",
					Namespace:  "default",
					Generation: 1,
				},
			}
			kinds, status, reason, _ := validateAllowedRouteKinds(tt.listener, gw)

			if status != tt.wantStatus {
				t.Errorf("status = %v, want %v", status, tt.wantStatus)
			}

			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}

			if len(kinds) != tt.wantSupportedCount {
				t.Errorf("len(supportedKinds) = %d, want %d", len(kinds), tt.wantSupportedCount)
			}
		})
	}
}

func gatewaySecretReferenceGrant(name, secretNS, gwNS, secretName string, allSecrets bool) gatewayv1beta1.ReferenceGrant {
	to := gatewayv1beta1.ReferenceGrantTo{
		Group: gatewayv1.Group(""),
		Kind:  gatewayv1.Kind("Secret"),
	}
	if !allSecrets {
		n := gatewayv1.ObjectName(secretName)
		to.Name = &n
	}
	return gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: secretNS},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{{
				Group:     gatewayv1.Group(gatewayv1.GroupName),
				Kind:      gatewayv1.Kind("Gateway"),
				Namespace: gatewayv1.Namespace(gwNS),
			}},
			To: []gatewayv1beta1.ReferenceGrantTo{to},
		},
	}
}

func TestValidateTLSCertificateRefs(t *testing.T) {
	secretGroup := gatewayv1.Group("")
	secretKind := gatewayv1.Kind("Secret")
	validCertPEM, validKeyPEM := generateTestTLSCertKey(t)
	backendNS := gatewayv1.Namespace("backend")
	validCrossNSTLSListener := gatewayv1.Listener{
		Name:     "https",
		Protocol: gatewayv1.HTTPSProtocolType,
		Port:     443,
		TLS: &gatewayv1.ListenerTLSConfig{
			CertificateRefs: []gatewayv1.SecretObjectReference{
				{Group: &secretGroup, Kind: &secretKind, Name: "certificate", Namespace: &backendNS},
			},
		},
	}
	validCrossNSSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "certificate", Namespace: "backend"},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": validCertPEM,
			"tls.key": validKeyPEM,
		},
	}

	tests := []struct {
		name             string
		gwNamespace      string
		listener         gatewayv1.Listener
		secrets          []corev1.Secret
		referenceGrants  []gatewayv1beta1.ReferenceGrant
		wantValid        bool
		wantReason       string
	}{
		{
			name: "no TLS config - valid",
			listener: gatewayv1.Listener{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     80,
			},
			wantValid:  true,
			wantReason: string(gatewayv1.ListenerReasonResolvedRefs),
		},
		{
			name: "valid TLS secret",
			listener: gatewayv1.Listener{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "test-cert"},
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cert",
						Namespace: "default",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": validCertPEM,
						"tls.key": validKeyPEM,
					},
				},
			},
			wantValid:  true,
			wantReason: string(gatewayv1.ListenerReasonResolvedRefs),
		},
		{
			name: "missing TLS secret",
			listener: gatewayv1.Listener{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "missing-cert"},
					},
				},
			},
			secrets:    []corev1.Secret{},
			wantValid:  false,
			wantReason: string(gatewayv1.ListenerReasonInvalidCertificateRef),
		},
		{
			name: "secret with wrong type",
			listener: gatewayv1.Listener{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "wrong-type"},
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-type",
						Namespace: "default",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
					},
				},
			},
			wantValid:  false,
			wantReason: string(gatewayv1.ListenerReasonInvalidCertificateRef),
		},
		{
			name: "secret missing tls.crt",
			listener: gatewayv1.Listener{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "no-cert"},
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-cert",
						Namespace: "default",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.key": []byte("key-data"),
					},
				},
			},
			wantValid:  false,
			wantReason: string(gatewayv1.ListenerReasonInvalidCertificateRef),
		},
		{
			name: "malformed PEM content in TLS secret",
			listener: gatewayv1.Listener{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "malformed-certificate"},
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "malformed-certificate",
						Namespace: "default",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": []byte("Hello world\n"),
						"tls.key": []byte("Hello world\n"),
					},
				},
			},
			wantValid:  false,
			wantReason: string(gatewayv1.ListenerReasonInvalidCertificateRef),
		},
		{
			name:        "cross-namespace secret without ReferenceGrant",
			gwNamespace: "infra",
			listener:    validCrossNSTLSListener,
			secrets:     []corev1.Secret{validCrossNSSecret},
			wantValid:   false,
			wantReason:  string(gatewayv1.ListenerReasonRefNotPermitted),
		},
		{
			name:        "cross-namespace secret with wrong from kind",
			gwNamespace: "infra",
			listener:    validCrossNSTLSListener,
			secrets:     []corev1.Secret{validCrossNSSecret},
			referenceGrants: []gatewayv1beta1.ReferenceGrant{{
				ObjectMeta: metav1.ObjectMeta{Name: "wrong-from-kind", Namespace: "backend"},
				Spec: gatewayv1beta1.ReferenceGrantSpec{
					From: []gatewayv1beta1.ReferenceGrantFrom{{
						Group:     gatewayv1.Group(gatewayv1.GroupName),
						Kind:      gatewayv1.Kind("HTTPRoute"),
						Namespace: gatewayv1.Namespace("infra"),
					}},
					To: []gatewayv1beta1.ReferenceGrantTo{{
						Group: gatewayv1.Group(""),
						Kind:  gatewayv1.Kind("Secret"),
						Name:  ptrObjectName("certificate"),
					}},
				},
			}},
			wantValid:  false,
			wantReason: string(gatewayv1.ListenerReasonRefNotPermitted),
		},
		{
			name:        "cross-namespace secret with wrong to name",
			gwNamespace: "infra",
			listener:    validCrossNSTLSListener,
			secrets:     []corev1.Secret{validCrossNSSecret},
			referenceGrants: []gatewayv1beta1.ReferenceGrant{
				gatewaySecretReferenceGrant("wrong-to-name", "backend", "infra", "other-cert", false),
			},
			wantValid:  false,
			wantReason: string(gatewayv1.ListenerReasonRefNotPermitted),
		},
		{
			name:        "cross-namespace secret with valid specific ReferenceGrant",
			gwNamespace: "infra",
			listener:    validCrossNSTLSListener,
			secrets:     []corev1.Secret{validCrossNSSecret},
			referenceGrants: []gatewayv1beta1.ReferenceGrant{
				gatewaySecretReferenceGrant("valid-specific", "backend", "infra", "certificate", false),
			},
			wantValid:  true,
			wantReason: string(gatewayv1.ListenerReasonResolvedRefs),
		},
		{
			name:        "cross-namespace secret with valid all-in-namespace ReferenceGrant",
			gwNamespace: "infra",
			listener:    validCrossNSTLSListener,
			secrets:     []corev1.Secret{validCrossNSSecret},
			referenceGrants: []gatewayv1beta1.ReferenceGrant{
				gatewaySecretReferenceGrant("valid-all", "backend", "infra", "certificate", true),
			},
			wantValid:  true,
			wantReason: string(gatewayv1.ListenerReasonResolvedRefs),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwNS := tt.gwNamespace
			if gwNS == "" {
				gwNS = "default"
			}

			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = gatewayv1beta1.AddToScheme(scheme)
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			for i := range tt.secrets {
				clientBuilder = clientBuilder.WithObjects(&tt.secrets[i])
			}
			for i := range tt.referenceGrants {
				grant := tt.referenceGrants[i]
				clientBuilder = clientBuilder.WithObjects(&grant)
			}
			cl := clientBuilder.Build()

			valid, reason, _ := validateTLSCertificateRefs(context.Background(), cl, gwNS, tt.listener)

			if valid != tt.wantValid {
				t.Errorf("valid = %v, want %v", valid, tt.wantValid)
			}

			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestValidateParametersRef(t *testing.T) {
	tests := []struct {
		name       string
		gateway    *gatewayv1.Gateway
		wantValid  bool
		wantReason string
	}{
		{
			name: "no infrastructure - valid",
			gateway: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "default",
				},
				Spec: gatewayv1.GatewaySpec{},
			},
			wantValid:  true,
			wantReason: "",
		},
		{
			name: "no parametersRef - valid",
			gateway: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "default",
				},
				Spec: gatewayv1.GatewaySpec{
					Infrastructure: &gatewayv1.GatewayInfrastructure{},
				},
			},
			wantValid:  true,
			wantReason: "",
		},
		{
			name: "invalid parametersRef - resource does not exist",
			gateway: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "default",
				},
				Spec: gatewayv1.GatewaySpec{
					Infrastructure: &gatewayv1.GatewayInfrastructure{
						ParametersRef: &gatewayv1.LocalParametersReference{
							Group: "example.com",
							Kind:  "ConfigMap",
							Name:  "missing-config",
						},
					},
				},
			},
			wantValid:  false,
			wantReason: "InvalidParameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()

			valid, reason, err := validateParametersRef(context.Background(), cl, tt.gateway)
			if err != nil {
				t.Fatalf("validateParametersRef returned error: %v", err)
			}

			if valid != tt.wantValid {
				t.Errorf("valid = %v, want %v", valid, tt.wantValid)
			}

			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestEvaluateListeners_ProgrammedFalseOnInvalidTLS(t *testing.T) {
	secretGroup := gatewayv1.Group("")
	secretKind := gatewayv1.Kind("Secret")
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gateway", Namespace: "default"},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "missing-cert"},
					},
				},
			}},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = gatewayv1.Install(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	statuses, hasAccepted, err := evaluateListeners(context.Background(), cl, gw)
	if err != nil {
		t.Fatalf("evaluateListeners returned error: %v", err)
	}
	if !hasAccepted {
		t.Fatal("expected hasAccepted=true for supported HTTPS listener")
	}
	if len(statuses) != 1 {
		t.Fatalf("len(statuses) = %d, want 1", len(statuses))
	}

	resolvedRefs := findListenerCondition(statuses[0].Conditions, gatewayv1.ListenerConditionResolvedRefs)
	programmed := findListenerCondition(statuses[0].Conditions, gatewayv1.ListenerConditionProgrammed)
	if resolvedRefs == nil || programmed == nil {
		t.Fatalf("missing conditions: %+v", statuses[0].Conditions)
	}
	if resolvedRefs.Status != metav1.ConditionFalse {
		t.Errorf("ResolvedRefs.Status = %v, want False", resolvedRefs.Status)
	}
	if programmed.Status != metav1.ConditionFalse {
		t.Errorf("Programmed.Status = %v, want False", programmed.Status)
	}
	if programmed.Reason != string(gatewayv1.ListenerReasonInvalid) {
		t.Errorf("Programmed.Reason = %q, want %q", programmed.Reason, gatewayv1.ListenerReasonInvalid)
	}
}

func ptrObjectName(name string) *gatewayv1.ObjectName {
	n := gatewayv1.ObjectName(name)
	return &n
}

func TestEvaluateListeners_RefNotPermittedOnCrossNamespaceSecret(t *testing.T) {
	secretGroup := gatewayv1.Group("")
	secretKind := gatewayv1.Kind("Secret")
	backendNS := gatewayv1.Namespace("backend")
	validCertPEM, validKeyPEM := generateTestTLSCertKey(t)

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gateway", Namespace: "infra", Generation: 1},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{
				Name:     "https",
				Protocol: gatewayv1.HTTPSProtocolType,
				Port:     443,
				TLS: &gatewayv1.ListenerTLSConfig{
					CertificateRefs: []gatewayv1.SecretObjectReference{
						{Group: &secretGroup, Kind: &secretKind, Name: "certificate", Namespace: &backendNS},
					},
				},
			}},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = gatewayv1.Install(scheme)
	_ = gatewayv1beta1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "certificate", Namespace: "backend"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.crt": validCertPEM,
				"tls.key": validKeyPEM,
			},
		},
	).Build()

	statuses, hasAccepted, err := evaluateListeners(context.Background(), cl, gw)
	if err != nil {
		t.Fatalf("evaluateListeners returned error: %v", err)
	}
	if !hasAccepted {
		t.Fatal("expected hasAccepted=true for supported HTTPS listener")
	}
	if len(statuses) != 1 {
		t.Fatalf("len(statuses) = %d, want 1", len(statuses))
	}

	resolvedRefs := findListenerCondition(statuses[0].Conditions, gatewayv1.ListenerConditionResolvedRefs)
	programmed := findListenerCondition(statuses[0].Conditions, gatewayv1.ListenerConditionProgrammed)
	if resolvedRefs == nil || programmed == nil {
		t.Fatalf("missing conditions: %+v", statuses[0].Conditions)
	}
	if resolvedRefs.Status != metav1.ConditionFalse {
		t.Errorf("ResolvedRefs.Status = %v, want False", resolvedRefs.Status)
	}
	if resolvedRefs.Reason != string(gatewayv1.ListenerReasonRefNotPermitted) {
		t.Errorf("ResolvedRefs.Reason = %q, want %q", resolvedRefs.Reason, gatewayv1.ListenerReasonRefNotPermitted)
	}
	if programmed.Status != metav1.ConditionFalse {
		t.Errorf("Programmed.Status = %v, want False", programmed.Status)
	}
}
