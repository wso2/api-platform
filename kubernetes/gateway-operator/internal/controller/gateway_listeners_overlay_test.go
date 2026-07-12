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
	"testing"

	yamlv3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func newGatewayWithListeners(listeners ...gatewayv1.Listener) *gatewayv1.Gateway {
	return &gatewayv1.Gateway{Spec: gatewayv1.GatewaySpec{Listeners: listeners}}
}

func listener(proto gatewayv1.ProtocolType, port int32) gatewayv1.Listener {
	return gatewayv1.Listener{Protocol: proto, Port: gatewayv1.PortNumber(port)}
}

func routerFromYAML(t *testing.T, y string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := yamlv3.Unmarshal([]byte(y), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	gw, _ := m["gateway"].(map[string]interface{})
	cfg, _ := gw["config"].(map[string]interface{})
	router, _ := cfg["router"].(map[string]interface{})
	return router
}

func runtimeServicePortsFromYAML(t *testing.T, y string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := yamlv3.Unmarshal([]byte(y), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	gw, _ := m["gateway"].(map[string]interface{})
	rt, _ := gw["gatewayRuntime"].(map[string]interface{})
	svc, _ := rt["service"].(map[string]interface{})
	ports, _ := svc["ports"].(map[string]interface{})
	return ports
}

func TestListenerPortsFromGateway(t *testing.T) {
	cases := []struct {
		name       string
		listeners  []gatewayv1.Listener
		wantHTTP   int32
		wantHTTPS  int32
		wantHasTLS bool
	}{
		{"none", nil, 0, 0, false},
		{"http only", []gatewayv1.Listener{listener("HTTP", 8080)}, 8080, 0, false},
		{"https only", []gatewayv1.Listener{listener("HTTPS", 8443)}, 0, 8443, true},
		{"both", []gatewayv1.Listener{listener("HTTP", 80), listener("HTTPS", 443)}, 80, 443, true},
		{"first of each wins", []gatewayv1.Listener{
			listener("HTTPS", 8443), listener("HTTP", 8080),
			listener("HTTPS", 9443), listener("HTTP", 9080),
		}, 8080, 8443, true},
		{"ignores tcp/udp", []gatewayv1.Listener{
			listener("TCP", 7000), listener("UDP", 7001), listener("HTTP", 8080),
		}, 8080, 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gw := newGatewayWithListeners(tc.listeners...)
			h, hs, hasTLS := listenerPortsFromGateway(gw)
			if h != tc.wantHTTP || hs != tc.wantHTTPS || hasTLS != tc.wantHasTLS {
				t.Fatalf("got (%d,%d,%v), want (%d,%d,%v)", h, hs, hasTLS, tc.wantHTTP, tc.wantHTTPS, tc.wantHasTLS)
			}
		})
	}
}

func TestApplyListenerOverlayToValues_NoListenersReturnsInputUnchanged(t *testing.T) {
	in := "gateway:\n  config:\n    router:\n      listener_port: 9090\n"
	gw := newGatewayWithListeners()
	out, err := applyListenerOverlayToValues(gw, in)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("expected unchanged output; got %q", out)
	}
}

func TestApplyListenerOverlayToValues_OverridesHTTPAndHTTPSPorts(t *testing.T) {
	in := `gateway:
  config:
    router:
      listener_port: 9090
      https_port: 9443
      https_enabled: true
      other: keep-me
  gatewayRuntime:
    service:
      type: ClusterIP
      ports:
        http: 9090
        https: 9443
        routerAdmin: 9901
  replicaCount: 2
`
	gw := newGatewayWithListeners(listener("HTTP", 80), listener("HTTPS", 443))
	out, err := applyListenerOverlayToValues(gw, in)
	if err != nil {
		t.Fatal(err)
	}
	router := routerFromYAML(t, out)
	if router["listener_port"] != 80 {
		t.Errorf("listener_port = %v, want 80", router["listener_port"])
	}
	if router["https_port"] != 443 {
		t.Errorf("https_port = %v, want 443", router["https_port"])
	}
	if router["https_enabled"] != true {
		t.Errorf("https_enabled = %v, want true", router["https_enabled"])
	}
	if router["other"] != "keep-me" {
		t.Errorf("sibling keys dropped; router=%v", router)
	}

	ports := runtimeServicePortsFromYAML(t, out)
	if ports["http"] != 80 {
		t.Errorf("gatewayRuntime.service.ports.http = %v, want 80", ports["http"])
	}
	if ports["https"] != 443 {
		t.Errorf("gatewayRuntime.service.ports.https = %v, want 443", ports["https"])
	}
	if ports["routerAdmin"] != 9901 {
		t.Errorf("sibling service port keys dropped; ports=%v", ports)
	}
}

func TestApplyListenerOverlayToValues_DisablesHTTPSWhenNoTLSListener(t *testing.T) {
	in := `gateway:
  config:
    router:
      https_enabled: true
      https_port: 9443
  gatewayRuntime:
    service:
      ports:
        http: 9090
        https: 9443
`
	gw := newGatewayWithListeners(listener("HTTP", 8080))
	out, err := applyListenerOverlayToValues(gw, in)
	if err != nil {
		t.Fatal(err)
	}
	router := routerFromYAML(t, out)
	if router["listener_port"] != 8080 {
		t.Errorf("listener_port = %v, want 8080", router["listener_port"])
	}
	if router["https_enabled"] != false {
		t.Errorf("https_enabled = %v, want false", router["https_enabled"])
	}
	if router["https_port"] != 9443 {
		t.Errorf("https_port = %v, want 9443 (preserved)", router["https_port"])
	}

	ports := runtimeServicePortsFromYAML(t, out)
	if ports["http"] != 8080 {
		t.Errorf("gatewayRuntime.service.ports.http = %v, want 8080", ports["http"])
	}
	if ports["https"] != 9443 {
		t.Errorf("gatewayRuntime.service.ports.https = %v, want 9443 (preserved)", ports["https"])
	}
}

func TestApplyListenerOverlayToValues_EmptyBaseYAML(t *testing.T) {
	gw := newGatewayWithListeners(listener("HTTP", 8080), listener("HTTPS", 8443))
	out, err := applyListenerOverlayToValues(gw, "")
	if err != nil {
		t.Fatal(err)
	}
	router := routerFromYAML(t, out)
	if router["listener_port"] != 8080 || router["https_port"] != 8443 || router["https_enabled"] != true {
		t.Fatalf("unexpected router values: %v", router)
	}
	ports := runtimeServicePortsFromYAML(t, out)
	if ports["http"] != 8080 || ports["https"] != 8443 {
		t.Fatalf("unexpected gatewayRuntime.service.ports: %v", ports)
	}
}

func httpsListenerWithCertRef(port int32, secretName, secretNS string) gatewayv1.Listener {
	ref := gatewayv1.SecretObjectReference{Name: gatewayv1.ObjectName(secretName)}
	if secretNS != "" {
		ns := gatewayv1.Namespace(secretNS)
		ref.Namespace = &ns
	}
	return gatewayv1.Listener{
		Protocol: "HTTPS",
		Port:     gatewayv1.PortNumber(port),
		TLS:      &gatewayv1.ListenerTLSConfig{CertificateRefs: []gatewayv1.SecretObjectReference{ref}},
	}
}

func controllerTLSFromYAML(t *testing.T, y string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := yamlv3.Unmarshal([]byte(y), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	gw, _ := m["gateway"].(map[string]interface{})
	ctrl, _ := gw["controller"].(map[string]interface{})
	tls, _ := ctrl["tls"].(map[string]interface{})
	return tls
}

// newTLSOverlayClient builds a fake client, optionally seeded with secrets.
func newTLSOverlayClient(t *testing.T, secrets ...*corev1.Secret) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, s := range secrets {
		builder = builder.WithObjects(s)
	}
	return builder.Build()
}

func validTLSSecret(t *testing.T, name, namespace string) *corev1.Secret {
	t.Helper()
	certPEM, keyPEM := generateTestTLSCertKey(t)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{"tls.crt": certPEM, "tls.key": keyPEM},
	}
}

func TestApplyListenerTLSOverlay_PointsAtListenerCertSecret(t *testing.T) {
	gw := newGatewayWithListeners(httpsListenerWithCertRef(443, "tls-validity-checks-certificate", ""))
	gw.Namespace = "gateway-conformance-infra"
	cl := newTLSOverlayClient(t, validTLSSecret(t, "tls-validity-checks-certificate", "gateway-conformance-infra"))

	out, err := applyListenerTLSOverlayToValues(context.Background(), cl, gw, "")
	if err != nil {
		t.Fatal(err)
	}
	tls := controllerTLSFromYAML(t, out)
	if tls["enabled"] != true {
		t.Errorf("tls.enabled = %v, want true", tls["enabled"])
	}
	if tls["certificateProvider"] != "secret" {
		t.Errorf("tls.certificateProvider = %v, want secret", tls["certificateProvider"])
	}
	secret, _ := tls["secret"].(map[string]interface{})
	if secret["name"] != "tls-validity-checks-certificate" {
		t.Errorf("tls.secret.name = %v, want tls-validity-checks-certificate", secret["name"])
	}
	if secret["certKey"] != "tls.crt" || secret["keyKey"] != "tls.key" {
		t.Errorf("unexpected secret keys: %v", secret)
	}
}

func TestApplyListenerTLSOverlay_NoHTTPSReturnsUnchanged(t *testing.T) {
	gw := newGatewayWithListeners(listener("HTTP", 8080))
	in := "gateway:\n  controller:\n    tls:\n      certificateProvider: cert-manager\n"
	out, err := applyListenerTLSOverlayToValues(context.Background(), newTLSOverlayClient(t), gw, in)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("expected input returned unchanged, got:\n%s", out)
	}
}

// TestApplyListenerTLSOverlay_SkipsNonexistentSecret guards GatewayWithAttachedRoutes and
// GatewayInvalidTLSConfiguration: mounting a nonexistent Secret leaves the gateway Helm
// release permanently unready, and the install's 300s Wait then blocks the single Gateway
// worker on every retry — starving e.g. the AttachedRoutes recount for this very Gateway.
// The overlay must fall back to the chart default certificate.
func TestApplyListenerTLSOverlay_SkipsNonexistentSecret(t *testing.T) {
	gw := newGatewayWithListeners(httpsListenerWithCertRef(443, "does-not-exist", ""))
	gw.Namespace = "gateway-conformance-infra"
	in := "gateway:\n  controller:\n    tls:\n      certificateProvider: cert-manager\n"
	out, err := applyListenerTLSOverlayToValues(context.Background(), newTLSOverlayClient(t), gw, in)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("expected input returned unchanged for nonexistent secret, got:\n%s", out)
	}
}

// TestApplyListenerTLSOverlay_SkipsMalformedSecret: a Secret with unparseable cert data
// would leave the router failing at startup — same failure mode as a nonexistent Secret.
func TestApplyListenerTLSOverlay_SkipsMalformedSecret(t *testing.T) {
	malformed := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "malformed", Namespace: "gateway-conformance-infra"},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{"tls.crt": []byte("not a cert"), "tls.key": []byte("not a key")},
	}
	gw := newGatewayWithListeners(httpsListenerWithCertRef(443, "malformed", ""))
	gw.Namespace = "gateway-conformance-infra"
	in := "gateway:\n  controller:\n    tls:\n      certificateProvider: cert-manager\n"
	out, err := applyListenerTLSOverlayToValues(context.Background(), newTLSOverlayClient(t, malformed), gw, in)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("expected input returned unchanged for malformed secret, got:\n%s", out)
	}
}

func TestApplyListenerTLSOverlay_SkipsNonTLSTypeSecret(t *testing.T) {
	opaque := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "opaque", Namespace: "gateway-conformance-infra"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"tls.crt": []byte("x"), "tls.key": []byte("y")},
	}
	gw := newGatewayWithListeners(httpsListenerWithCertRef(443, "opaque", ""))
	gw.Namespace = "gateway-conformance-infra"
	out, err := applyListenerTLSOverlayToValues(context.Background(), newTLSOverlayClient(t, opaque), gw, "")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("expected input returned unchanged for non-TLS-type secret, got:\n%s", out)
	}
}

func TestGatewayListenersReferenceTLSSecret(t *testing.T) {
	gw := newGatewayWithListeners(httpsListenerWithCertRef(443, "my-cert", ""))
	gw.Namespace = "ns-a"

	if !gatewayListenersReferenceTLSSecret(gw, "ns-a", "my-cert") {
		t.Error("expected same-namespace certificateRef to match")
	}
	if gatewayListenersReferenceTLSSecret(gw, "ns-a", "other-cert") {
		t.Error("expected different secret name to not match")
	}
	if gatewayListenersReferenceTLSSecret(gw, "ns-b", "my-cert") {
		t.Error("expected secret in different namespace to not match")
	}

	crossNS := newGatewayWithListeners(httpsListenerWithCertRef(443, "my-cert", "ns-b"))
	crossNS.Namespace = "ns-a"
	if gatewayListenersReferenceTLSSecret(crossNS, "ns-b", "my-cert") {
		t.Error("expected cross-namespace certificateRef to not match (overlay cannot mount it)")
	}
}

func TestApplyListenerTLSOverlay_SkipsCrossNamespaceCertRef(t *testing.T) {
	gw := newGatewayWithListeners(httpsListenerWithCertRef(443, "other-cert", "other-ns"))
	gw.Namespace = "gateway-conformance-infra"
	if name, ok := listenerTLSSecretFromGateway(gw); ok {
		t.Errorf("expected cross-namespace certificateRef to be skipped, got secret %q", name)
	}
}
