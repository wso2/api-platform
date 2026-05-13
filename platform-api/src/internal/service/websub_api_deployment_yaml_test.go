/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package service

import (
	"strings"
	"testing"

	"platform-api/src/internal/model"

	"gopkg.in/yaml.v3"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func makePolicy(name, version string) model.Policy {
	return model.Policy{Name: name, Version: version}
}

func makeEventPolicies(policies ...model.Policy) *model.WebSubEventPolicies {
	return &model.WebSubEventPolicies{Policies: policies}
}

func ptrMap(m map[string]interface{}) *map[string]interface{} { return &m }

// buildTestWebSubAPI builds a minimal WebSubAPI with both global (allChannels)
// and per-channel policies, matching the user's reported request body.
func buildTestWebSubAPI() *model.WebSubAPI {
	return &model.WebSubAPI{
		Handle:      "my-websub-api1",
		Name:        "echo api1",
		Version:     "v1.0",
		ProjectUUID: "019e2158-6d48-7730-8af3-a5b484c9ee4c",
		Configuration: model.WebSubAPIConfiguration{
			Context: strPtr("/repos1"),
			AllChannels: &model.WebSubAllChannelPolicies{
				OnSubscription: makeEventPolicies(model.Policy{
					Name:    "api-key-auth",
					Version: "v1",
					Params:  ptrMap(map[string]interface{}{"in": "header", "key": "X-API-Key"}),
				}),
				OnMessageReceived: makeEventPolicies(model.Policy{
					Name:    "basic-auth",
					Version: "v1",
					Params:  ptrMap(map[string]interface{}{"username": "admin", "password": "admin123"}),
				}),
				OnMessageDelivery: makeEventPolicies(model.Policy{
					Name:    "set-headers",
					Version: "v1",
					Params: ptrMap(map[string]interface{}{
						"request": map[string]interface{}{
							"headers": []interface{}{
								map[string]interface{}{"name": "level", "value": "global"},
							},
						},
					}),
				}),
			},
			Channels: map[string]model.WebSubChannel{
				"issues": {
					OnMessageDelivery: makeEventPolicies(model.Policy{
						Name:    "set-headers",
						Version: "v1",
						Params: ptrMap(map[string]interface{}{
							"request": map[string]interface{}{
								"headers": []interface{}{
									map[string]interface{}{"name": "level", "value": "local"},
									map[string]interface{}{"name": "channel", "value": "issues"},
								},
							},
						}),
					}),
				},
			},
		},
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestBuildWebSubAPIDeploymentYAML_AllChannelsPresentInYAML is the regression test
// for the bug where AllChannels had a `json` struct tag instead of `yaml`, causing
// it to be silently omitted from the marshaled YAML sent to the gateway controller.
func TestBuildWebSubAPIDeploymentYAML_AllChannelsPresentInYAML(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "allChannels") {
		t.Errorf("allChannels missing from marshaled YAML; this means the struct tag was wrong (json instead of yaml).\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebSubAPIDeploymentYAML_AllChannelsPoliciesPresent verifies that global
// (all-channel) policies from the API configuration appear in the deployment YAML.
func TestBuildWebSubAPIDeploymentYAML_AllChannelsPoliciesPresent(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	cases := []struct {
		desc string
		want string
	}{
		{"on_subscription present", "on_subscription"},
		{"on_message_received present", "on_message_received"},
		{"on_message_delivery present", "on_message_delivery"},
		{"api-key-auth policy present", "api-key-auth"},
		{"basic-auth policy present", "basic-auth"},
	}
	for _, tc := range cases {
		if !strings.Contains(yamlStr, tc.want) {
			t.Errorf("%s: %q not found in YAML.\nFull YAML:\n%s", tc.desc, tc.want, yamlStr)
		}
	}
}

// TestBuildWebSubAPIDeploymentYAML_ChannelPoliciesPresentInYAML verifies that
// per-channel policy overrides appear in the marshaled YAML, not just the struct.
func TestBuildWebSubAPIDeploymentYAML_ChannelPoliciesPresentInYAML(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "channels") {
		t.Errorf("channels section missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "issues") {
		t.Errorf("'issues' channel missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebSubAPIDeploymentYAML_ChannelPoliciesNotWrapped verifies that per-channel
// event policies appear DIRECTLY on the channel (no extra "policies:" wrapper key).
// The gateway-controller's WebSubChannel schema has on_subscription/on_message_delivery
// at the top level of the channel, not nested under a "policies:" key.
func TestBuildWebSubAPIDeploymentYAML_ChannelPoliciesNotWrapped(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	// Unmarshal into a generic map to inspect the channel structure precisely.
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	spec, _ := parsed["spec"].(map[string]interface{})
	channels, _ := spec["channels"].(map[string]interface{})
	issuesCh, ok := channels["issues"].(map[string]interface{})
	if !ok {
		t.Fatalf("'issues' channel not found or wrong type in parsed YAML")
	}

	// The "on_message_delivery" key must exist directly on the channel, NOT under a "policies" key.
	if _, hasDirect := issuesCh["on_message_delivery"]; !hasDirect {
		t.Errorf("on_message_delivery should be a direct key of the 'issues' channel entry, not wrapped under 'policies'.\nChannel map keys: %v", keysOf(issuesCh))
	}
	if _, hasWrapper := issuesCh["policies"]; hasWrapper {
		t.Errorf("unexpected 'policies' wrapper key found inside 'issues' channel; gateway-controller expects event policies at the top level of each channel")
	}
}

// keysOf returns the keys of a map[string]interface{} for test error messages.
func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
} // TestBuildWebSubAPIDeploymentYAML_ChannelDeliveryPolicyParams verifies the
// per-channel on_message_delivery set-headers policy and its params are in the YAML.
// The "local" value appears only in the channel-level policy; if channels are dropped
// from YAML output, this test will catch it.
func TestBuildWebSubAPIDeploymentYAML_ChannelDeliveryPolicyParams(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "local") {
		t.Errorf("channel-level policy param value 'local' missing from YAML; channel policies may have been dropped.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebSubAPIDeploymentYAML_AllChannelsStruct verifies the in-memory struct
// has AllChannels populated with the expected policies before marshaling.
func TestBuildWebSubAPIDeploymentYAML_AllChannelsStruct(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	if d.Spec.AllChannels == nil {
		t.Fatal("Spec.AllChannels should not be nil")
	}

	ac := d.Spec.AllChannels
	if ac.OnSubscription == nil || ac.OnSubscription.Policies == nil || len(*ac.OnSubscription.Policies) != 1 {
		t.Errorf("expected 1 OnSubscription policy in AllChannels, got %v", ac.OnSubscription)
	}
	if (*ac.OnSubscription.Policies)[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", (*ac.OnSubscription.Policies)[0].Name)
	}
	if ac.OnMessageReceived == nil || ac.OnMessageReceived.Policies == nil || len(*ac.OnMessageReceived.Policies) != 1 {
		t.Errorf("expected 1 OnMessageReceived policy in AllChannels, got %v", ac.OnMessageReceived)
	}
	if (*ac.OnMessageReceived.Policies)[0].Name != "basic-auth" {
		t.Errorf("expected 'basic-auth', got %q", (*ac.OnMessageReceived.Policies)[0].Name)
	}
	if ac.OnMessageDelivery == nil || ac.OnMessageDelivery.Policies == nil || len(*ac.OnMessageDelivery.Policies) != 1 {
		t.Errorf("expected 1 OnMessageDelivery policy in AllChannels, got %v", ac.OnMessageDelivery)
	}
}

// TestBuildWebSubAPIDeploymentYAML_ChannelsStruct verifies the in-memory struct
// has per-channel policies populated correctly.
func TestBuildWebSubAPIDeploymentYAML_ChannelsStruct(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	if len(d.Spec.Channels) != 1 {
		t.Fatalf("expected 1 channel in Spec.Channels, got %d", len(d.Spec.Channels))
	}
	ch, ok := d.Spec.Channels["issues"]
	if !ok {
		t.Fatal("'issues' channel not found in Spec.Channels")
	}
	if ch.OnMessageDelivery == nil || ch.OnMessageDelivery.Policies == nil {
		t.Fatal("'issues' channel OnMessageDelivery should not be nil")
	}
	if len(*ch.OnMessageDelivery.Policies) != 1 {
		t.Errorf("expected 1 OnMessageDelivery policy for 'issues' channel, got %d", len(*ch.OnMessageDelivery.Policies))
	}
	if (*ch.OnMessageDelivery.Policies)[0].Name != "set-headers" {
		t.Errorf("expected 'set-headers' in 'issues' channel, got %q", (*ch.OnMessageDelivery.Policies)[0].Name)
	}
}

// TestBuildWebSubAPIDeploymentYAML_NoChannels verifies correct YAML output
// when no per-channel policies are defined.
func TestBuildWebSubAPIDeploymentYAML_NoChannels(t *testing.T) {
	websubAPI := &model.WebSubAPI{
		Handle:  "simple-api",
		Name:    "Simple API",
		Version: "v1.0",
		Configuration: model.WebSubAPIConfiguration{
			Context: strPtr("/simple"),
			AllChannels: &model.WebSubAllChannelPolicies{
				OnSubscription: makeEventPolicies(makePolicy("api-key-auth", "v1")),
			},
		},
	}
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "allChannels") {
		t.Errorf("allChannels missing from marshaled YAML when no per-channel policies exist.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "api-key-auth") {
		t.Errorf("api-key-auth policy missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebSubAPIDeploymentYAML_NilAllChannels verifies no panic and no allChannels
// key in YAML when AllChannels is nil.
func TestBuildWebSubAPIDeploymentYAML_NilAllChannels(t *testing.T) {
	websubAPI := &model.WebSubAPI{
		Handle:  "bare-api",
		Name:    "Bare API",
		Version: "v1.0",
		Configuration: model.WebSubAPIConfiguration{
			Context:     strPtr("/bare"),
			AllChannels: nil,
		},
	}
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if strings.Contains(yamlStr, "allChannels") {
		t.Errorf("allChannels should not appear in YAML when nil.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebSubAPIDeploymentYAML_Context verifies that the context is correctly
// set in the deployment struct, defaulting to "/" when nil or empty.
func TestBuildWebSubAPIDeploymentYAML_Context(t *testing.T) {
	t.Run("explicit context", func(t *testing.T) {
		websubAPI := buildTestWebSubAPI()
		d := buildWebSubAPIDeploymentYAML(websubAPI)
		if d.Spec.Context != "/repos1" {
			t.Errorf("expected context '/repos1', got %q", d.Spec.Context)
		}
	})

	t.Run("nil context defaults to /", func(t *testing.T) {
		websubAPI := &model.WebSubAPI{
			Handle:        "ctx-api",
			Name:          "ctx api",
			Version:       "v1.0",
			Configuration: model.WebSubAPIConfiguration{Context: nil},
		}
		d := buildWebSubAPIDeploymentYAML(websubAPI)
		if d.Spec.Context != "/" {
			t.Errorf("expected default context '/', got %q", d.Spec.Context)
		}
	})

	t.Run("empty context defaults to /", func(t *testing.T) {
		empty := ""
		websubAPI := &model.WebSubAPI{
			Handle:        "ctx-api2",
			Name:          "ctx api2",
			Version:       "v1.0",
			Configuration: model.WebSubAPIConfiguration{Context: &empty},
		}
		d := buildWebSubAPIDeploymentYAML(websubAPI)
		if d.Spec.Context != "/" {
			t.Errorf("expected default context '/', got %q", d.Spec.Context)
		}
	})
}

// TestBuildWebSubAPIDeploymentYAML_ProjectLabelSet verifies the projectId label
// is set in metadata when ProjectUUID is non-empty.
func TestBuildWebSubAPIDeploymentYAML_ProjectLabelSet(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	if d.Metadata.Labels == nil {
		t.Fatal("Metadata.Labels should not be nil when ProjectUUID is set")
	}
	if d.Metadata.Labels["projectId"] != websubAPI.ProjectUUID {
		t.Errorf("expected projectId label %q, got %q", websubAPI.ProjectUUID, d.Metadata.Labels["projectId"])
	}
}

// TestBuildWebSubAPIDeploymentYAML_ProjectLabelAbsent verifies no labels are set
// when ProjectUUID is empty.
func TestBuildWebSubAPIDeploymentYAML_ProjectLabelAbsent(t *testing.T) {
	websubAPI := buildTestWebSubAPI()
	websubAPI.ProjectUUID = ""
	d := buildWebSubAPIDeploymentYAML(websubAPI)

	if d.Metadata.Labels != nil {
		t.Errorf("expected nil Metadata.Labels when ProjectUUID is empty, got %v", d.Metadata.Labels)
	}
}
