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

	yamlv3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// allowedServiceTypes is the set of values accepted for AnnK8sGatewayServiceType.
// We deliberately limit this to the standard Kubernetes Service types rather than
// accepting arbitrary strings, so typos fail fast at reconcile time.
var allowedServiceTypes = map[corev1.ServiceType]struct{}{
	corev1.ServiceTypeClusterIP:    {},
	corev1.ServiceTypeNodePort:     {},
	corev1.ServiceTypeLoadBalancer: {},
	corev1.ServiceTypeExternalName: {},
}

// serviceTypeFromGateway returns the Service type override encoded in
// Gateway.spec.infrastructure.annotations[AnnK8sGatewayServiceType], or "" when unset.
// The caller is responsible for validation via validateServiceType.
func serviceTypeFromGateway(gw *gatewayv1.Gateway) string {
	if gw == nil || gw.Spec.Infrastructure == nil {
		return ""
	}
	for k, v := range gw.Spec.Infrastructure.Annotations {
		if string(k) == AnnK8sGatewayServiceType {
			return string(v)
		}
	}
	return ""
}

// infrastructureAnnotationsFromGateway returns spec.infrastructure.annotations as a
// plain map[string]string, excluding operator-reserved keys that are consumed as
// configuration (e.g. AnnK8sGatewayServiceType) rather than passed through as Service
// metadata.
func infrastructureAnnotationsFromGateway(gw *gatewayv1.Gateway) map[string]string {
	if gw == nil || gw.Spec.Infrastructure == nil || len(gw.Spec.Infrastructure.Annotations) == 0 {
		return nil
	}
	out := make(map[string]string, len(gw.Spec.Infrastructure.Annotations))
	for k, v := range gw.Spec.Infrastructure.Annotations {
		key := string(k)
		if key == AnnK8sGatewayServiceType {
			continue
		}
		out[key] = string(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// infrastructureLabelsFromGateway returns spec.infrastructure.labels as a plain
// map[string]string.
func infrastructureLabelsFromGateway(gw *gatewayv1.Gateway) map[string]string {
	if gw == nil || gw.Spec.Infrastructure == nil || len(gw.Spec.Infrastructure.Labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(gw.Spec.Infrastructure.Labels))
	for k, v := range gw.Spec.Infrastructure.Labels {
		out[string(k)] = string(v)
	}
	return out
}

// validateServiceType returns an error if the provided type string is non-empty and not
// one of the standard Service types supported by Kubernetes.
func validateServiceType(t string) error {
	if t == "" {
		return nil
	}
	if _, ok := allowedServiceTypes[corev1.ServiceType(t)]; !ok {
		return fmt.Errorf("invalid value %q for %s (expected one of ClusterIP, NodePort, LoadBalancer, ExternalName)",
			t, AnnK8sGatewayServiceType)
	}
	return nil
}

// applyInfrastructureOverlayToValues deep-merges gateway-runtime Service metadata and
// type overrides derived from Gateway.spec.infrastructure on top of the given values
// YAML and returns the resulting YAML. When Gateway.spec.infrastructure contributes no
// overrides, the input valuesYAML is returned unchanged.
//
// Overlay shape (only populated keys are emitted):
//
//	gateway:
//	  gatewayRuntime:
//	    service:
//	      type:        <infrastructure.annotations["gateway.api-platform.wso2.com/service-type"]>
//	      annotations: <infrastructure.annotations minus reserved keys>
//	      labels:      <infrastructure.labels>
//
// The overlay respects Gateway API semantics: spec.infrastructure.annotations / labels
// are documented as "metadata to apply to any resources created in response to this
// Gateway", so they flow onto the gateway-runtime Service. A reserved annotation key
// (AnnK8sGatewayServiceType) is interpreted as `service.type` rather than propagated.
func applyInfrastructureOverlayToValues(gw *gatewayv1.Gateway, valuesYAML string) (string, error) {
	svcType := serviceTypeFromGateway(gw)
	if err := validateServiceType(svcType); err != nil {
		return "", err
	}
	anns := infrastructureAnnotationsFromGateway(gw)
	labels := infrastructureLabelsFromGateway(gw)

	if svcType == "" && len(anns) == 0 && len(labels) == 0 {
		return valuesYAML, nil
	}

	service := map[string]interface{}{}
	if svcType != "" {
		service["type"] = svcType
	}
	if len(anns) > 0 {
		m := make(map[string]interface{}, len(anns))
		for k, v := range anns {
			m[k] = v
		}
		service["annotations"] = m
	}
	if len(labels) > 0 {
		m := make(map[string]interface{}, len(labels))
		for k, v := range labels {
			m[k] = v
		}
		service["labels"] = m
	}

	overlay := map[string]interface{}{
		"gateway": map[string]interface{}{
			"gatewayRuntime": map[string]interface{}{
				"service": service,
			},
		},
	}

	base := map[string]interface{}{}
	if valuesYAML != "" {
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
