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

func makeBrokerPolicy(name, version string) model.Policy {
	return model.Policy{Name: name, Version: version}
}

func makeBrokerEventPolicies(policies ...model.Policy) *model.WebBrokerEventPolicies {
	return &model.WebBrokerEventPolicies{Policies: policies}
}

func ptrBrokerMap(m map[string]interface{}) *map[string]interface{} { return &m }

// buildTestWebBrokerAPI builds a minimal WebBrokerAPI with receiver, broker,
// global (allChannels) and per-channel policies for testing.
func buildTestWebBrokerAPI() *model.WebBrokerAPI {
	return &model.WebBrokerAPI{
		Handle:      "stock-trading-v1-0",
		Name:        "Stock Trading API",
		Version:     "v1.0",
		ProjectUUID: "019e2158-6d48-7730-8af3-a5b484c9ee4c",
		Configuration: model.WebBrokerAPIConfiguration{
			Context: strPtr("/trading"),
			Receiver: model.WebBrokerReceiver{
				Name: "websocket-receiver",
				Type: "websocket",
			},
			Broker: model.WebBrokerBroker{
				Name: "kafka-driver",
				Type: "kafka",
				Properties: map[string]interface{}{
					"bootstrap.servers": "kafka:9092",
					"compression.type":  "gzip",
				},
			},
			AllChannels: &model.WebBrokerAllChannelPolicies{
				OnConnectionInit: makeBrokerEventPolicies(model.Policy{
					Name:    "api-key-auth",
					Version: "v1",
					Params:  ptrBrokerMap(map[string]interface{}{"in": "header", "key": "X-API-Key"}),
				}),
				OnProduce: makeBrokerEventPolicies(model.Policy{
					Name:    "validate-payload",
					Version: "v1",
					Params:  ptrBrokerMap(map[string]interface{}{"schema": "stock-schema"}),
				}),
				OnConsume: makeBrokerEventPolicies(model.Policy{
					Name:    "rate-limit",
					Version: "v1",
					Params:  ptrBrokerMap(map[string]interface{}{"limit": 100, "window": "1m"}),
				}),
			},
			Channels: map[string]model.WebBrokerChannel{
				"stocks": {
					ProduceTo: &model.WebBrokerTopic{
						Topic: "stock-updates",
					},
					ConsumeFrom: &model.WebBrokerTopic{
						Topic: "stock-orders",
					},
					OnConnectionInit: makeBrokerEventPolicies(),
					OnProduce:        makeBrokerEventPolicies(),
					OnConsume: makeBrokerEventPolicies(model.Policy{
						Name:    "throttle",
						Version: "v1",
						Params:  ptrBrokerMap(map[string]interface{}{"rate": "10/s"}),
					}),
				},
			},
		},
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestBuildWebBrokerAPIDeploymentYAML_AllChannelsPresentInYAML is the regression test
// for the bug where AllChannels had a `json` struct tag instead of `yaml`, causing
// it to be silently omitted from the marshaled YAML sent to the gateway controller.
func TestBuildWebBrokerAPIDeploymentYAML_AllChannelsPresentInYAML(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "allChannels") {
		t.Errorf("allChannels missing from marshaled YAML; this means the struct tag was wrong (json instead of yaml).\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ReceiverPresentInYAML verifies that receiver
// configuration appears in the marshaled YAML.
func TestBuildWebBrokerAPIDeploymentYAML_ReceiverPresentInYAML(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "receiver") {
		t.Errorf("receiver missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "websocket-receiver") {
		t.Errorf("receiver name 'websocket-receiver' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_BrokerPresentInYAML verifies that broker
// configuration appears in the marshaled YAML.
func TestBuildWebBrokerAPIDeploymentYAML_BrokerPresentInYAML(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "broker") {
		t.Errorf("broker missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "kafka-driver") {
		t.Errorf("broker name 'kafka-driver' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_AllChannelsPoliciesPresent verifies that global
// (all-channel) policies from the API configuration appear in the deployment YAML.
func TestBuildWebBrokerAPIDeploymentYAML_AllChannelsPoliciesPresent(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	cases := []struct {
		desc string
		want string
	}{
		{"on_connection_init present", "on_connection_init"},
		{"on_produce present", "on_produce"},
		{"on_consume present", "on_consume"},
		{"api-key-auth policy present", "api-key-auth"},
		{"validate-payload policy present", "validate-payload"},
		{"rate-limit policy present", "rate-limit"},
	}
	for _, tc := range cases {
		if !strings.Contains(yamlStr, tc.want) {
			t.Errorf("%s: %q not found in YAML.\nFull YAML:\n%s", tc.desc, tc.want, yamlStr)
		}
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_AllChannelsStruct verifies the in-memory struct
// has AllChannels populated with the expected policies before marshaling.
func TestBuildWebBrokerAPIDeploymentYAML_AllChannelsStruct(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if d.Spec.AllChannels == nil {
		t.Fatal("Spec.AllChannels should not be nil")
	}

	ac := d.Spec.AllChannels
	if ac.OnConnectionInit == nil || ac.OnConnectionInit.Policies == nil || len(*ac.OnConnectionInit.Policies) != 1 {
		t.Errorf("expected 1 OnConnectionInit policy in AllChannels, got %v", ac.OnConnectionInit)
	}
	if (*ac.OnConnectionInit.Policies)[0].Name != "api-key-auth" {
		t.Errorf("expected 'api-key-auth', got %q", (*ac.OnConnectionInit.Policies)[0].Name)
	}
	if ac.OnProduce == nil || ac.OnProduce.Policies == nil || len(*ac.OnProduce.Policies) != 1 {
		t.Errorf("expected 1 OnProduce policy in AllChannels, got %v", ac.OnProduce)
	}
	if (*ac.OnProduce.Policies)[0].Name != "validate-payload" {
		t.Errorf("expected 'validate-payload', got %q", (*ac.OnProduce.Policies)[0].Name)
	}
	if ac.OnConsume == nil || ac.OnConsume.Policies == nil || len(*ac.OnConsume.Policies) != 1 {
		t.Errorf("expected 1 OnConsume policy in AllChannels, got %v", ac.OnConsume)
	}
	if (*ac.OnConsume.Policies)[0].Name != "rate-limit" {
		t.Errorf("expected 'rate-limit', got %q", (*ac.OnConsume.Policies)[0].Name)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ReceiverConfigStored verifies receiver
// configuration is correctly stored in the deployment struct.
func TestBuildWebBrokerAPIDeploymentYAML_ReceiverConfigStored(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if d.Spec.Receiver == nil {
		t.Fatal("Spec.Receiver should not be nil")
	}
	if d.Spec.Receiver.Name != "websocket-receiver" {
		t.Errorf("expected receiver name 'websocket-receiver', got %q", d.Spec.Receiver.Name)
	}
	if d.Spec.Receiver.Type != "websocket" {
		t.Errorf("expected receiver type 'websocket', got %q", d.Spec.Receiver.Type)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ReceiverInYAML verifies receiver config
// appears in marshaled YAML with correct structure.
func TestBuildWebBrokerAPIDeploymentYAML_ReceiverInYAML(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "type: websocket") {
		t.Errorf("receiver type 'websocket' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_BrokerConfigStored verifies broker
// configuration is correctly stored in the deployment struct.
func TestBuildWebBrokerAPIDeploymentYAML_BrokerConfigStored(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if d.Spec.Broker == nil {
		t.Fatal("Spec.Broker should not be nil")
	}
	if d.Spec.Broker.Name != "kafka-driver" {
		t.Errorf("expected broker name 'kafka-driver', got %q", d.Spec.Broker.Name)
	}
	if d.Spec.Broker.Type != "kafka" {
		t.Errorf("expected broker type 'kafka', got %q", d.Spec.Broker.Type)
	}
	if d.Spec.Broker.Properties == nil {
		t.Fatal("Broker properties should not be nil")
	}
	if d.Spec.Broker.Properties["bootstrap.servers"] != "kafka:9092" {
		t.Errorf("expected bootstrap.servers 'kafka:9092', got %v", d.Spec.Broker.Properties["bootstrap.servers"])
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_BrokerPropertiesInYAML verifies broker
// properties appear in marshaled YAML.
func TestBuildWebBrokerAPIDeploymentYAML_BrokerPropertiesInYAML(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "bootstrap.servers") {
		t.Errorf("broker property 'bootstrap.servers' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "kafka:9092") {
		t.Errorf("broker property value 'kafka:9092' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ChannelPoliciesPresentInYAML verifies that
// per-channel policy overrides appear in the marshaled YAML.
func TestBuildWebBrokerAPIDeploymentYAML_ChannelPoliciesPresentInYAML(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "channels") {
		t.Errorf("channels section missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "stocks") {
		t.Errorf("'stocks' channel missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ChannelPoliciesNotWrapped verifies that per-channel
// event policies appear DIRECTLY on the channel (no extra "policies:" wrapper key).
func TestBuildWebBrokerAPIDeploymentYAML_ChannelPoliciesNotWrapped(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

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
	stocksCh, ok := channels["stocks"].(map[string]interface{})
	if !ok {
		t.Fatalf("'stocks' channel not found or wrong type in parsed YAML")
	}

	// The "on_consume" key must exist directly on the channel, NOT under a "policies" key.
	if _, hasDirect := stocksCh["on_consume"]; !hasDirect {
		t.Errorf("on_consume should be a direct key of the 'stocks' channel entry, not wrapped under 'policies'.\nChannel map keys: %v", keysOfMap(stocksCh))
	}
	if _, hasWrapper := stocksCh["policies"]; hasWrapper {
		t.Errorf("unexpected 'policies' wrapper key found inside 'stocks' channel; gateway-controller expects event policies at the top level of each channel")
	}
}

// keysOfMap returns the keys of a map[string]interface{} for test error messages.
func keysOfMap(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestBuildWebBrokerAPIDeploymentYAML_ChannelsStruct verifies the in-memory struct
// has per-channel policies populated correctly.
func TestBuildWebBrokerAPIDeploymentYAML_ChannelsStruct(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if len(d.Spec.Channels) != 1 {
		t.Fatalf("expected 1 channel in Spec.Channels, got %d", len(d.Spec.Channels))
	}
	ch, ok := d.Spec.Channels["stocks"]
	if !ok {
		t.Fatal("'stocks' channel not found in Spec.Channels")
	}
	if ch.OnConsume == nil || ch.OnConsume.Policies == nil {
		t.Fatal("'stocks' channel OnConsume should not be nil")
	}
	if len(*ch.OnConsume.Policies) != 1 {
		t.Errorf("expected 1 OnConsume policy for 'stocks' channel, got %d", len(*ch.OnConsume.Policies))
	}
	if (*ch.OnConsume.Policies)[0].Name != "throttle" {
		t.Errorf("expected 'throttle' in 'stocks' channel, got %q", (*ch.OnConsume.Policies)[0].Name)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_MultipleChannels verifies multiple channels
// can be configured correctly.
func TestBuildWebBrokerAPIDeploymentYAML_MultipleChannels(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	// Add a second channel
	webbrokerAPI.Configuration.Channels["crypto"] = model.WebBrokerChannel{
		ProduceTo: &model.WebBrokerTopic{
			Topic: "crypto-updates",
		},
		OnProduce: makeBrokerEventPolicies(model.Policy{
			Name:    "validate-crypto",
			Version: "v1",
		}),
	}

	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if len(d.Spec.Channels) != 2 {
		t.Fatalf("expected 2 channels in Spec.Channels, got %d", len(d.Spec.Channels))
	}

	cryptoCh, ok := d.Spec.Channels["crypto"]
	if !ok {
		t.Fatal("'crypto' channel not found in Spec.Channels")
	}
	if cryptoCh.ProduceTo == nil || cryptoCh.ProduceTo.Topic != "crypto-updates" {
		t.Errorf("expected 'crypto-updates' topic in 'crypto' channel, got %v", cryptoCh.ProduceTo)
	}

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "crypto") {
		t.Errorf("'crypto' channel missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "validate-crypto") {
		t.Errorf("'validate-crypto' policy missing from marshaled YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ChannelTopics verifies channel topic
// configuration (produceTo/consumeFrom) is stored correctly.
func TestBuildWebBrokerAPIDeploymentYAML_ChannelTopics(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	stocksCh := d.Spec.Channels["stocks"]
	if stocksCh.ProduceTo == nil || stocksCh.ProduceTo.Topic != "stock-updates" {
		t.Errorf("expected ProduceTo topic 'stock-updates', got %v", stocksCh.ProduceTo)
	}
	if stocksCh.ConsumeFrom == nil || stocksCh.ConsumeFrom.Topic != "stock-orders" {
		t.Errorf("expected ConsumeFrom topic 'stock-orders', got %v", stocksCh.ConsumeFrom)
	}

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "stock-updates") {
		t.Errorf("produceTo topic 'stock-updates' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "stock-orders") {
		t.Errorf("consumeFrom topic 'stock-orders' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_NoChannels verifies correct YAML output
// when no per-channel policies are defined.
func TestBuildWebBrokerAPIDeploymentYAML_NoChannels(t *testing.T) {
	webbrokerAPI := &model.WebBrokerAPI{
		Handle:  "simple-api",
		Name:    "Simple API",
		Version: "v1.0",
		Configuration: model.WebBrokerAPIConfiguration{
			Context: strPtr("/simple"),
			Receiver: model.WebBrokerReceiver{
				Name: "ws-receiver",
				Type: "websocket",
			},
			Broker: model.WebBrokerBroker{
				Name: "kafka-broker",
				Type: "kafka",
			},
			AllChannels: &model.WebBrokerAllChannelPolicies{
				OnConnectionInit: makeBrokerEventPolicies(makeBrokerPolicy("api-key-auth", "v1")),
			},
		},
	}
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

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

// TestBuildWebBrokerAPIDeploymentYAML_NilAllChannels verifies no panic and no allChannels
// key in YAML when AllChannels is nil.
func TestBuildWebBrokerAPIDeploymentYAML_NilAllChannels(t *testing.T) {
	webbrokerAPI := &model.WebBrokerAPI{
		Handle:  "bare-api",
		Name:    "Bare API",
		Version: "v1.0",
		Configuration: model.WebBrokerAPIConfiguration{
			Context: strPtr("/bare"),
			Receiver: model.WebBrokerReceiver{
				Name: "ws-receiver",
				Type: "websocket",
			},
			Broker: model.WebBrokerBroker{
				Name: "kafka-broker",
				Type: "kafka",
			},
			AllChannels: nil,
		},
	}
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if strings.Contains(yamlStr, "allChannels") {
		t.Errorf("allChannels should not appear in YAML when nil.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_Context verifies that the context is correctly
// set in the deployment struct, defaulting to "/" when nil or empty.
func TestBuildWebBrokerAPIDeploymentYAML_Context(t *testing.T) {
	t.Run("explicit context", func(t *testing.T) {
		webbrokerAPI := buildTestWebBrokerAPI()
		d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)
		if d.Spec.Context != "/trading" {
			t.Errorf("expected context '/trading', got %q", d.Spec.Context)
		}
	})

	t.Run("nil context defaults to /", func(t *testing.T) {
		webbrokerAPI := &model.WebBrokerAPI{
			Handle:  "ctx-api",
			Name:    "ctx api",
			Version: "v1.0",
			Configuration: model.WebBrokerAPIConfiguration{
				Context:  nil,
				Receiver: model.WebBrokerReceiver{Name: "ws", Type: "websocket"},
				Broker:   model.WebBrokerBroker{Name: "kafka", Type: "kafka"},
			},
		}
		d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)
		if d.Spec.Context != "/" {
			t.Errorf("expected default context '/', got %q", d.Spec.Context)
		}
	})

	t.Run("empty context defaults to /", func(t *testing.T) {
		empty := ""
		webbrokerAPI := &model.WebBrokerAPI{
			Handle:  "ctx-api2",
			Name:    "ctx api2",
			Version: "v1.0",
			Configuration: model.WebBrokerAPIConfiguration{
				Context:  &empty,
				Receiver: model.WebBrokerReceiver{Name: "ws", Type: "websocket"},
				Broker:   model.WebBrokerBroker{Name: "kafka", Type: "kafka"},
			},
		}
		d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)
		if d.Spec.Context != "/" {
			t.Errorf("expected default context '/', got %q", d.Spec.Context)
		}
	})
}

// TestBuildWebBrokerAPIDeploymentYAML_ProjectLabelSet verifies the projectId label
// is set in metadata when ProjectUUID is non-empty.
func TestBuildWebBrokerAPIDeploymentYAML_ProjectLabelSet(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if d.Metadata.Labels == nil {
		t.Fatal("Metadata.Labels should not be nil when ProjectUUID is set")
	}
	if d.Metadata.Labels["projectId"] != webbrokerAPI.ProjectUUID {
		t.Errorf("expected projectId label %q, got %q", webbrokerAPI.ProjectUUID, d.Metadata.Labels["projectId"])
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ProjectLabelAbsent verifies no labels are set
// when ProjectUUID is empty.
func TestBuildWebBrokerAPIDeploymentYAML_ProjectLabelAbsent(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	webbrokerAPI.ProjectUUID = ""
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if d.Metadata.Labels != nil {
		t.Errorf("expected nil Metadata.Labels when ProjectUUID is empty, got %v", d.Metadata.Labels)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_ChannelPolicyParams verifies channel-level
// policy parameters are correctly stored and appear in YAML.
func TestBuildWebBrokerAPIDeploymentYAML_ChannelPolicyParams(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	// The "10/s" rate param only appears in the channel-level throttle policy
	if !strings.Contains(yamlStr, "10/s") {
		t.Errorf("channel-level policy param '10/s' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}

// TestBuildWebBrokerAPIDeploymentYAML_BrokerTypeKafka verifies Kafka broker type
// is correctly set in the deployment YAML.
func TestBuildWebBrokerAPIDeploymentYAML_BrokerTypeKafka(t *testing.T) {
	webbrokerAPI := buildTestWebBrokerAPI()
	d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)

	if d.Spec.Broker.Type != "kafka" {
		t.Errorf("expected broker type 'kafka', got %q", d.Spec.Broker.Type)
	}

	raw, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	yamlStr := string(raw)

	if !strings.Contains(yamlStr, "type: kafka") {
		t.Errorf("broker type 'kafka' missing from YAML.\nFull YAML:\n%s", yamlStr)
	}
}
