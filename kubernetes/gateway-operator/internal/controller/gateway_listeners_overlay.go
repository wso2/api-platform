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
	cryptotls "crypto/tls"
	"fmt"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// listenerPortsFromGateway extracts router port overrides from Gateway.spec.listeners.
// It picks the first HTTP listener port (httpPort) and the first HTTPS listener port
// (httpsPort). hasHTTPS is true when at least one HTTPS listener is declared.
func listenerPortsFromGateway(gw *gatewayv1.Gateway) (httpPort int32, httpsPort int32, hasHTTPS bool) {
	if gw == nil {
		return 0, 0, false
	}
	for _, l := range gw.Spec.Listeners {
		switch strings.ToUpper(string(l.Protocol)) {
		case "HTTP":
			if httpPort == 0 {
				httpPort = int32(l.Port)
			}
		case "HTTPS":
			hasHTTPS = true
			if httpsPort == 0 {
				httpsPort = int32(l.Port)
			}
		}
	}
	return httpPort, httpsPort, hasHTTPS
}

// applyListenerOverlayToValues deep-merges router-port and gateway-runtime Service-port
// overrides derived from Gateway listeners on top of the given values YAML and returns
// the resulting YAML. When the Gateway has no HTTP/HTTPS listeners, the input valuesYAML
// is returned unchanged.
//
// Overlay shape (only populated keys are emitted):
//
//	gateway:
//	  config:
//	    router:
//	      listener_port: <first HTTP listener port>    # only if present
//	      https_enabled: <true|false>                  # true iff any HTTPS listener
//	      https_port:    <first HTTPS listener port>   # only when hasHTTPS
//	  gatewayRuntime:
//	    service:
//	      ports:
//	        http:  <first HTTP listener port>          # only if present
//	        https: <first HTTPS listener port>         # only when hasHTTPS
//
// The `gatewayRuntime.service.ports` entries are kept in sync with the router ports so
// that the Kubernetes Service the gateway-runtime pods sit behind exposes the same port
// clients declared on `Gateway.spec.listeners[]` (the Service template uses `port` as
// the implicit `targetPort`, so they must match what Envoy binds to inside the pod).
func applyListenerOverlayToValues(gw *gatewayv1.Gateway, valuesYAML string) (string, error) {
	httpPort, httpsPort, hasHTTPS := listenerPortsFromGateway(gw)
	if httpPort == 0 && !hasHTTPS {
		return valuesYAML, nil
	}

	router := map[string]interface{}{}
	if httpPort > 0 {
		router["listener_port"] = int(httpPort)
	}
	router["https_enabled"] = hasHTTPS
	if hasHTTPS && httpsPort > 0 {
		router["https_port"] = int(httpsPort)
	}

	servicePorts := map[string]interface{}{}
	if httpPort > 0 {
		servicePorts["http"] = int(httpPort)
	}
	if hasHTTPS && httpsPort > 0 {
		servicePorts["https"] = int(httpsPort)
	}

	gatewayOverlay := map[string]interface{}{
		"config": map[string]interface{}{
			"router": router,
		},
	}
	if len(servicePorts) > 0 {
		gatewayOverlay["gatewayRuntime"] = map[string]interface{}{
			"service": map[string]interface{}{
				"ports": servicePorts,
			},
		}
	}

	overlay := map[string]interface{}{
		"gateway": gatewayOverlay,
	}

	base := map[string]interface{}{}
	if strings.TrimSpace(valuesYAML) != "" {
		if err := yamlv3.Unmarshal([]byte(valuesYAML), &base); err != nil {
			return "", fmt.Errorf("parse values YAML: %w", err)
		}
		if base == nil {
			base = map[string]interface{}{}
		}
	}

	merged := deepMergeYAMLMaps(base, overlay)
	out, err := yamlv3.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged values: %w", err)
	}
	return string(out), nil
}

// listenerTLSSecretFromGateway returns the Secret name backing the first HTTPS listener's
// first usable certificateRef. ok is false when the Gateway declares no HTTPS listener
// certificate that can be mounted into the gateway-runtime.
//
// Only core ("") group Secret refs in the Gateway's own namespace are supported: the
// gateway Helm release is installed in the Gateway namespace and mounts the certificate
// Secret by name, so a cross-namespace certificateRef cannot be honored here.
func listenerTLSSecretFromGateway(gw *gatewayv1.Gateway) (secretName string, ok bool) {
	if gw == nil {
		return "", false
	}
	for _, l := range gw.Spec.Listeners {
		if strings.ToUpper(string(l.Protocol)) != "HTTPS" || l.TLS == nil {
			continue
		}
		for _, ref := range l.TLS.CertificateRefs {
			if ref.Group != nil && string(*ref.Group) != "" {
				continue // only core-group Secrets
			}
			if ref.Kind != nil && string(*ref.Kind) != "Secret" {
				continue
			}
			if ref.Namespace != nil && string(*ref.Namespace) != "" && string(*ref.Namespace) != gw.Namespace {
				continue // cross-namespace refs are not mountable by the release
			}
			if string(ref.Name) == "" {
				continue
			}
			return string(ref.Name), true
		}
	}
	return "", false
}

// applyListenerTLSOverlayToValues points the gateway-runtime HTTPS listener certificate at the
// Secret referenced by the Gateway's HTTPS listener (Gateway.spec.listeners[].tls.certificateRefs)
// instead of the chart default (a cert-manager-issued localhost certificate). Without this the
// runtime serves the localhost cert and TLS handshakes for the route hostnames fail certificate
// verification. When the Gateway declares no usable HTTPS listener certificate, valuesYAML is
// returned unchanged.
//
// Overlay shape:
//
//	gateway:
//	  controller:
//	    tls:
//	      enabled: true
//	      certificateProvider: secret
//	      secret:
//	        name: <listener certificateRef Secret>
//	        certKey: tls.crt
//	        keyKey: tls.key
//
// Note: all HTTPS listeners are served with this single certificate; per-listener (SNI-based)
// certificate selection is not yet supported. This matches the conformance Gateway, whose
// listeners all reference the same Secret.
//
// The overlay is only applied when the referenced Secret exists and holds a parseable
// kubernetes.io/tls cert/key pair. Mounting a nonexistent or malformed Secret leaves the
// Helm release permanently unready (unmountable volume / failing router); the install runs
// with Wait (300s) on the Gateway worker, so every retry of such a Gateway starves status
// updates for all Gateways — e.g. the AttachedRoutes recount in GatewayWithAttachedRoutes.
// An invalid certificateRef is still reported on the listener as ResolvedRefs=False
// (validateTLSCertificateRefs); the release just keeps the chart's default certificate. A
// Secret created or fixed later re-enqueues the Gateway via the Secret watch
// (enqueueK8sGatewaysForListenerTLSSecret), which re-applies the overlay.
func applyListenerTLSOverlayToValues(ctx context.Context, cl client.Client, gw *gatewayv1.Gateway, valuesYAML string) (string, error) {
	secretName, ok := listenerTLSSecretFromGateway(gw)
	if !ok {
		return valuesYAML, nil
	}

	secret := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: gw.Namespace, Name: secretName}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return valuesYAML, nil
		}
		return "", fmt.Errorf("get listener TLS secret %s/%s: %w", gw.Namespace, secretName, err)
	}
	if !mountableTLSSecret(secret) {
		return valuesYAML, nil
	}

	overlay := map[string]interface{}{
		"gateway": map[string]interface{}{
			"controller": map[string]interface{}{
				"tls": map[string]interface{}{
					"enabled":             true,
					"certificateProvider": "secret",
					"secret": map[string]interface{}{
						"name":    secretName,
						"certKey": "tls.crt",
						"keyKey":  "tls.key",
					},
				},
			},
		},
	}

	base := map[string]interface{}{}
	if strings.TrimSpace(valuesYAML) != "" {
		if err := yamlv3.Unmarshal([]byte(valuesYAML), &base); err != nil {
			return "", fmt.Errorf("parse values YAML: %w", err)
		}
		if base == nil {
			base = map[string]interface{}{}
		}
	}

	merged := deepMergeYAMLMaps(base, overlay)
	out, err := yamlv3.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged values: %w", err)
	}
	return string(out), nil
}

// mountableTLSSecret reports whether the Secret can back the gateway-runtime HTTPS
// listener: type kubernetes.io/tls with a parseable tls.crt/tls.key pair (the same
// checks validateTLSCertificateRefs applies when setting listener ResolvedRefs).
func mountableTLSSecret(secret *corev1.Secret) bool {
	if secret.Type != corev1.SecretTypeTLS {
		return false
	}
	crt, hasCert := secret.Data["tls.crt"]
	key, hasKey := secret.Data["tls.key"]
	if !hasCert || !hasKey {
		return false
	}
	_, err := cryptotls.X509KeyPair(crt, key)
	return err == nil
}

// gatewayListenersReferenceTLSSecret reports whether any listener certificateRef on the
// Gateway points at the named Secret in the Gateway's own namespace (the only refs the
// TLS overlay can mount).
func gatewayListenersReferenceTLSSecret(gw *gatewayv1.Gateway, secretNamespace, secretName string) bool {
	if gw == nil || gw.Namespace != secretNamespace {
		return false
	}
	for _, l := range gw.Spec.Listeners {
		if l.TLS == nil {
			continue
		}
		for _, ref := range l.TLS.CertificateRefs {
			if ref.Group != nil && string(*ref.Group) != "" {
				continue
			}
			if ref.Kind != nil && string(*ref.Kind) != "Secret" {
				continue
			}
			if ref.Namespace != nil && string(*ref.Namespace) != "" && string(*ref.Namespace) != gw.Namespace {
				continue
			}
			if string(ref.Name) == secretName {
				return true
			}
		}
	}
	return false
}

// deepMergeYAMLMaps merges override into base recursively. Non-map values in override
// replace base. Nested maps are merged. The input maps are not mutated.
func deepMergeYAMLMaps(base, override map[string]interface{}) map[string]interface{} {
	if len(override) == 0 {
		return base
	}
	if base == nil {
		base = map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		bRaw, ok := out[k]
		if !ok {
			out[k] = v
			continue
		}
		bm, bOK := bRaw.(map[string]interface{})
		vm, vOK := v.(map[string]interface{})
		if bOK && vOK {
			out[k] = deepMergeYAMLMaps(bm, vm)
			continue
		}
		out[k] = v
	}
	return out
}
