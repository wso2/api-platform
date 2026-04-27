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
	"fmt"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
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
