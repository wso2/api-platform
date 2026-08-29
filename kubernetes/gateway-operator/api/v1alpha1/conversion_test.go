/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"bytes"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// TestRestApiConversionRoundTrip exercises a fully-populated RestApi through
// v1alpha1 -> v1 -> v1alpha1 and asserts equality across ObjectMeta, Spec and
// Status.
func TestRestApiConversionRoundTrip(t *testing.T) {
	orig := &RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "orders-api",
			Namespace:   "apis",
			Labels:      map[string]string{"team": "platform"},
			Annotations: map[string]string{"note": "conversion"},
			Generation:  7,
		},
		Spec: APIConfigData{
			Context:     "/orders",
			DisplayName: "Orders API",
			Operations: []Operation{
				{Method: OperationMethod("GET"), Path: "/orders"},
			},
		},
		Status: RestApiStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed", Message: "ok"},
			},
		},
	}

	hub := &v1.RestApi{}
	if err := orig.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if hub.Name != orig.Name || hub.Spec.Context != orig.Spec.Context || hub.Spec.DisplayName != orig.Spec.DisplayName {
		t.Fatalf("hub not populated correctly: %+v", hub)
	}
	if len(hub.Spec.Operations) != 1 || string(hub.Spec.Operations[0].Method) != "GET" {
		t.Fatalf("hub operations not converted: %+v", hub.Spec.Operations)
	}

	back := &RestApi{}
	if err := back.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if !reflect.DeepEqual(orig.ObjectMeta, back.ObjectMeta) {
		t.Errorf("ObjectMeta round-trip mismatch:\n got  %+v\n want %+v", back.ObjectMeta, orig.ObjectMeta)
	}
	if !reflect.DeepEqual(orig.Spec, back.Spec) {
		t.Errorf("Spec round-trip mismatch:\n got  %+v\n want %+v", back.Spec, orig.Spec)
	}
	if !reflect.DeepEqual(orig.Status, back.Status) {
		t.Errorf("Status round-trip mismatch:\n got  %+v\n want %+v", back.Status, orig.Status)
	}
}

// TestAgentConversionRoundTrip exercises a populated Agent through
// v1alpha1 -> v1 -> v1alpha1. The interesting part is the free-form Agent Card
// content: it travels as a runtime.RawExtension through the JSON round-trip
// convertViaJSON performs, and the gateway signs the bytes it is given, so a
// re-serialization that reorders or reformats them would break verification.
func TestAgentConversionRoundTrip(t *testing.T) {
	card := []byte(`{"name":"Weather Agent","capabilities":{"streaming":true},` +
		`"x-vendor-extension":{"nested":["a",1,false]},"emptyList":[],"flag":false}`)
	ctxPath := "/weather"

	orig := &Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "weather-agent-v1-0",
			Namespace:  "agents",
			Labels:     map[string]string{"team": "platform"},
			Generation: 4,
		},
		Spec: AgentConfigData{
			DisplayName: "Weather Agent",
			Version:     "v1.0",
			Context:     &ctxPath,
			Upstream:    AgentUpstream{Url: strPtr("https://weather.internal")},
			A2A: A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: A2AOperationConfigs{
					Transports: []A2ATransport{
						{ProtocolBinding: A2AProtocolBindingJSONRPC, PathPrefix: strPtr("/rpc")},
						{ProtocolBinding: A2AProtocolBindingHTTPJSON, PathPrefix: strPtr("/rest")},
					},
					Policies: []Policy{{Name: "jwt-auth", Version: "v1"}},
					Operations: []A2AOperationConfig{{
						Name:     "SendMessage",
						Policies: []Policy{{Name: "advanced-ratelimit", Version: "v1"}},
					}},
				},
				AgentCard: A2AAgentCard{
					Public: A2APublicAgentCard{
						Mode:     "managed",
						Path:     strPtr("/.well-known/agent-card.json"),
						Policies: []Policy{{Name: "cors", Version: "v1"}},
						Content:  &runtime.RawExtension{Raw: card},
						Signing:  &A2ACardSigning{Enabled: true},
					},
				},
			},
		},
		Status: ResourceStatus{
			Conditions: []metav1.Condition{
				{Type: "Programmed", Status: metav1.ConditionTrue, Reason: "Programmed", Message: "ok"},
			},
		},
	}

	hub := &v1.Agent{}
	if err := orig.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if hub.Spec.A2A.ProtocolVersion != "1.0" || len(hub.Spec.A2A.OperationConfigs.Transports) != 2 {
		t.Fatalf("hub a2a config not converted: %+v", hub.Spec.A2A)
	}
	if hub.Spec.A2A.AgentCard.Public.Content == nil ||
		!bytes.Equal(hub.Spec.A2A.AgentCard.Public.Content.Raw, card) {
		t.Fatalf("card content not preserved byte-for-byte in hub: %s",
			hub.Spec.A2A.AgentCard.Public.Content)
	}

	back := &Agent{}
	if err := back.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if !reflect.DeepEqual(orig.ObjectMeta, back.ObjectMeta) {
		t.Errorf("ObjectMeta round-trip mismatch:\n got  %+v\n want %+v", back.ObjectMeta, orig.ObjectMeta)
	}
	if !reflect.DeepEqual(orig.Spec, back.Spec) {
		t.Errorf("Spec round-trip mismatch:\n got  %+v\n want %+v", back.Spec, orig.Spec)
	}
	if !reflect.DeepEqual(orig.Status, back.Status) {
		t.Errorf("Status round-trip mismatch:\n got  %+v\n want %+v", back.Status, orig.Status)
	}
}

func strPtr(s string) *string { return &s }

// TestAllKindsConversionPlumbing verifies that every spoke kind implements the
// Convertible interface, converts to the correct hub type, preserves
// ObjectMeta, and converts back — for all 13 CRD kinds.
func TestAllKindsConversionPlumbing(t *testing.T) {
	meta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:       "sample",
			Namespace:  "ns",
			Labels:     map[string]string{"a": "b"},
			Generation: 3,
		}
	}

	cases := []struct {
		name  string
		spoke conversion.Convertible
		hub   conversion.Hub
		fresh conversion.Convertible
	}{
		{"RestApi", &RestApi{ObjectMeta: meta()}, &v1.RestApi{}, &RestApi{}},
		{"Agent", &Agent{ObjectMeta: meta()}, &v1.Agent{}, &Agent{}},
		{"APIGateway", &APIGateway{ObjectMeta: meta()}, &v1.APIGateway{}, &APIGateway{}},
		{"ApiKey", &ApiKey{ObjectMeta: meta()}, &v1.ApiKey{}, &ApiKey{}},
		{"APIPolicy", &APIPolicy{ObjectMeta: meta()}, &v1.APIPolicy{}, &APIPolicy{}},
		{"Certificate", &Certificate{ObjectMeta: meta()}, &v1.Certificate{}, &Certificate{}},
		{"LlmProvider", &LlmProvider{ObjectMeta: meta()}, &v1.LlmProvider{}, &LlmProvider{}},
		{"LlmProviderTemplate", &LlmProviderTemplate{ObjectMeta: meta()}, &v1.LlmProviderTemplate{}, &LlmProviderTemplate{}},
		{"LlmProxy", &LlmProxy{ObjectMeta: meta()}, &v1.LlmProxy{}, &LlmProxy{}},
		{"ManagedSecret", &ManagedSecret{ObjectMeta: meta()}, &v1.ManagedSecret{}, &ManagedSecret{}},
		{"Mcp", &Mcp{ObjectMeta: meta()}, &v1.Mcp{}, &Mcp{}},
		{"Subscription", &Subscription{ObjectMeta: meta()}, &v1.Subscription{}, &Subscription{}},
		{"SubscriptionPlan", &SubscriptionPlan{ObjectMeta: meta()}, &v1.SubscriptionPlan{}, &SubscriptionPlan{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.spoke.ConvertTo(tc.hub); err != nil {
				t.Fatalf("ConvertTo: %v", err)
			}
			spokeMeta := tc.spoke.(metav1.Object)
			hubMeta := tc.hub.(metav1.Object)
			if hubMeta.GetName() != spokeMeta.GetName() ||
				hubMeta.GetNamespace() != spokeMeta.GetNamespace() ||
				hubMeta.GetGeneration() != spokeMeta.GetGeneration() ||
				!reflect.DeepEqual(hubMeta.GetLabels(), spokeMeta.GetLabels()) {
				t.Fatalf("hub ObjectMeta not preserved: got %+v", hubMeta)
			}

			if err := tc.fresh.ConvertFrom(tc.hub); err != nil {
				t.Fatalf("ConvertFrom: %v", err)
			}
			freshMeta := tc.fresh.(metav1.Object)
			if freshMeta.GetName() != spokeMeta.GetName() ||
				!reflect.DeepEqual(freshMeta.GetLabels(), spokeMeta.GetLabels()) {
				t.Fatalf("round-trip ObjectMeta mismatch: got %+v", freshMeta)
			}
		})
	}
}
