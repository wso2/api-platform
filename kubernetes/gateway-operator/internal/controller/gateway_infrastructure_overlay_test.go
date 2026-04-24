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
	"strings"
	"testing"

	yamlv3 "gopkg.in/yaml.v3"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func gatewayWithInfrastructure(anns, labels map[string]string) *gatewayv1.Gateway {
	infra := &gatewayv1.GatewayInfrastructure{}
	if len(anns) > 0 {
		infra.Annotations = map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{}
		for k, v := range anns {
			infra.Annotations[gatewayv1.AnnotationKey(k)] = gatewayv1.AnnotationValue(v)
		}
	}
	if len(labels) > 0 {
		infra.Labels = map[gatewayv1.LabelKey]gatewayv1.LabelValue{}
		for k, v := range labels {
			infra.Labels[gatewayv1.LabelKey(k)] = gatewayv1.LabelValue(v)
		}
	}
	return &gatewayv1.Gateway{Spec: gatewayv1.GatewaySpec{Infrastructure: infra}}
}

func runtimeServiceFromYAML(t *testing.T, y string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := yamlv3.Unmarshal([]byte(y), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	gw, _ := m["gateway"].(map[string]interface{})
	rt, _ := gw["gatewayRuntime"].(map[string]interface{})
	svc, _ := rt["service"].(map[string]interface{})
	return svc
}

func TestServiceTypeFromGateway(t *testing.T) {
	if got := serviceTypeFromGateway(nil); got != "" {
		t.Fatalf("nil gateway: got %q, want empty", got)
	}
	if got := serviceTypeFromGateway(&gatewayv1.Gateway{}); got != "" {
		t.Fatalf("no infra: got %q, want empty", got)
	}
	gw := gatewayWithInfrastructure(map[string]string{AnnK8sGatewayServiceType: "NodePort"}, nil)
	if got := serviceTypeFromGateway(gw); got != "NodePort" {
		t.Fatalf("got %q, want NodePort", got)
	}
}

func TestValidateServiceType(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"ClusterIP", false},
		{"NodePort", false},
		{"LoadBalancer", false},
		{"ExternalName", false},
		{"loadbalancer", true}, // case-sensitive
		{"Bogus", true},
	}
	for _, tc := range cases {
		err := validateServiceType(tc.in)
		if tc.wantErr && err == nil {
			t.Errorf("%q: expected error, got nil", tc.in)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%q: unexpected error: %v", tc.in, err)
		}
	}
}

func TestApplyInfrastructureOverlayToValues_NoInfrastructureReturnsInputUnchanged(t *testing.T) {
	in := "gateway:\n  gatewayRuntime:\n    service:\n      type: LoadBalancer\n"
	out, err := applyInfrastructureOverlayToValues(&gatewayv1.Gateway{}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("expected unchanged output; got %q", out)
	}
}

func TestApplyInfrastructureOverlayToValues_OverridesServiceType(t *testing.T) {
	in := `gateway:
  gatewayRuntime:
    service:
      type: LoadBalancer
      ports:
        http: 8080
`
	gw := gatewayWithInfrastructure(map[string]string{AnnK8sGatewayServiceType: "NodePort"}, nil)
	out, err := applyInfrastructureOverlayToValues(gw, in)
	if err != nil {
		t.Fatal(err)
	}
	svc := runtimeServiceFromYAML(t, out)
	if svc["type"] != "NodePort" {
		t.Errorf("service.type = %v, want NodePort", svc["type"])
	}
	ports, _ := svc["ports"].(map[string]interface{})
	if ports["http"] != 8080 {
		t.Errorf("sibling ports dropped; service=%v", svc)
	}
}

func TestApplyInfrastructureOverlayToValues_PropagatesAnnotationsAndLabels(t *testing.T) {
	gw := gatewayWithInfrastructure(
		map[string]string{
			AnnK8sGatewayServiceType: "LoadBalancer",
			"service.beta.kubernetes.io/aws-load-balancer-type":   "external",
			"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
		},
		map[string]string{"team": "platform", "tier": "edge"},
	)
	out, err := applyInfrastructureOverlayToValues(gw, "")
	if err != nil {
		t.Fatal(err)
	}
	svc := runtimeServiceFromYAML(t, out)
	if svc["type"] != "LoadBalancer" {
		t.Errorf("service.type = %v, want LoadBalancer", svc["type"])
	}
	anns, _ := svc["annotations"].(map[string]interface{})
	if anns["service.beta.kubernetes.io/aws-load-balancer-type"] != "external" {
		t.Errorf("missing aws-load-balancer-type annotation: %v", anns)
	}
	if anns["service.beta.kubernetes.io/aws-load-balancer-scheme"] != "internet-facing" {
		t.Errorf("missing aws-load-balancer-scheme annotation: %v", anns)
	}
	// Reserved key must NOT flow through as a Service annotation.
	if _, ok := anns[AnnK8sGatewayServiceType]; ok {
		t.Errorf("reserved service-type annotation must be filtered; got %v", anns)
	}
	labels, _ := svc["labels"].(map[string]interface{})
	if labels["team"] != "platform" || labels["tier"] != "edge" {
		t.Errorf("missing infrastructure labels: %v", labels)
	}
}

func TestApplyInfrastructureOverlayToValues_InvalidServiceTypeIsRejected(t *testing.T) {
	gw := gatewayWithInfrastructure(map[string]string{AnnK8sGatewayServiceType: "nope"}, nil)
	_, err := applyInfrastructureOverlayToValues(gw, "")
	if err == nil {
		t.Fatal("expected error for invalid service type")
	}
	if !strings.Contains(err.Error(), AnnK8sGatewayServiceType) {
		t.Errorf("error should mention the annotation key: %v", err)
	}
}

func TestApplyInfrastructureOverlayToValues_PreservesSiblingServiceKeys(t *testing.T) {
	in := `gateway:
  gatewayRuntime:
    service:
      type: ClusterIP
      annotations:
        existing: keep-me
      labels:
        existing: keep-me
      ports:
        http: 8080
        https: 8443
`
	gw := gatewayWithInfrastructure(map[string]string{
		AnnK8sGatewayServiceType: "LoadBalancer",
		"added":                  "yes",
	}, map[string]string{"added": "yes"})
	out, err := applyInfrastructureOverlayToValues(gw, in)
	if err != nil {
		t.Fatal(err)
	}
	svc := runtimeServiceFromYAML(t, out)
	anns, _ := svc["annotations"].(map[string]interface{})
	if anns["existing"] != "keep-me" || anns["added"] != "yes" {
		t.Errorf("annotations not merged correctly: %v", anns)
	}
	labels, _ := svc["labels"].(map[string]interface{})
	if labels["existing"] != "keep-me" || labels["added"] != "yes" {
		t.Errorf("labels not merged correctly: %v", labels)
	}
	ports, _ := svc["ports"].(map[string]interface{})
	if ports["http"] != 8080 || ports["https"] != 8443 {
		t.Errorf("ports dropped: %v", ports)
	}
}
