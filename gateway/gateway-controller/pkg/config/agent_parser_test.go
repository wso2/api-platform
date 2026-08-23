/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package config

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/agentproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// agentWorkedExampleYAML is the worked example from the Agent resource schema.
//
// The Agent Card under agentCard.public.content is written as embedded JSON on
// purpose: JSON object syntax is valid YAML, and this is how a card copied
// straight out of an A2A agent arrives. Parsing has to carry it through
// unflattened, because everything downstream — validation, serving, signing —
// reads the same nested structure.
//
// securityRequirements uses the normative A2A shape,
// [{"schemes": {"<name>": {"list": [...]}}}], not the OpenAPI-style
// [{"<name>": [...]}]. The two are easy to confuse and the difference is
// silent: scope-consistency checks read schemes[<name>].list, so against the
// OpenAPI-style shape they would find nothing and pass vacuously.
const agentWorkedExampleYAML = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: weather-agent-v1-0
  labels:
    team: agents
  annotations:
    gateway.api-platform.wso2.com/project-id: 019d953f-d386-7a64-aa92-1869a28292e0
spec:
  displayName: Weather Agent
  version: v1.0
  context: /weather
  vhost: agents.example.com
  upstream:
    url: https://weather.internal
  resilience:
    timeout: 30s
    idleTimeout: 5m
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
        - protocolBinding: HTTP+JSON
          pathPrefix: /rest
      policies:
        - name: jwt-auth
          version: v1
          params:
            issuer: https://idp.example.com
            requiredScopes:
              - a2a.invoke
      operations:
        - name: SendMessage
          policies:
            - name: advanced-ratelimit
              version: v1
              params:
                quotas:
                  - name: send-message-limit
                    limits:
                      - limit: 100
                        duration: 1m
    agentCard:
      public:
        mode: managed
        path: /.well-known/agent-card.json
        policies:
          - name: cors
            version: v1
        content: {
          "name": "Weather Agent",
          "description": "Provides weather information",
          "version": "1.0.0",
          "supportedInterfaces": [
            {
              "protocolBinding": "JSONRPC",
              "protocolVersion": "1.0",
              "url": "https://agents.example.com/weather/rpc"
            },
            {
              "protocolBinding": "HTTP+JSON",
              "protocolVersion": "1.0",
              "url": "https://agents.example.com/weather/rest"
            }
          ],
          "capabilities": {
            "streaming": true
          },
          "securitySchemes": {
            "gateway-jwt": {
              "openIdConnectSecurityScheme": {
                "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
              }
            }
          },
          "securityRequirements": [
            {
              "schemes": {
                "gateway-jwt": { "list": ["a2a.invoke"] }
              }
            }
          ],
          "defaultInputModes": ["text/plain"],
          "defaultOutputModes": ["text/plain"],
          "skills": [
            {
              "id": "get_weather",
              "name": "Get weather",
              "description": "Gets weather information",
              "tags": ["weather"]
            }
          ]
        }
        signing:
          enabled: true
`

// requireCardObject reads a nested object out of an Agent Card, failing the
// test if the value is not one.
//
// It delegates to the validator's own cardObject so these assertions walk the
// card exactly the way validation does. That matters because a card's nested
// mappings come back typed by whichever decoder produced them — yaml.v3 reuses
// the enclosing named map type (api.A2AAgentCardDocument), encoding/json
// produces a plain map[string]interface{} — and a walker that handled only one
// shape would work on the ingress path a test happens to use and silently do
// nothing on the other.
func requireCardObject(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	object, ok := cardObject(value)
	if !ok {
		t.Fatalf("expected a card object, got %T", value)
	}
	return object
}

// parseAgentWorkedExample parses the worked example, failing the test on error.
func parseAgentWorkedExample(t *testing.T) api.AgentConfiguration {
	t.Helper()
	var cfg api.AgentConfiguration
	require.NoError(t, NewParser().ParseAPIConfigYAML([]byte(agentWorkedExampleYAML), &cfg))
	return cfg
}

// TestParseAgent_WorkedExample_Envelope covers the resource envelope and the
// spec fields shared with every other gateway artifact kind.
func TestParseAgent_WorkedExample_Envelope(t *testing.T) {
	cfg := parseAgentWorkedExample(t)

	assert.Equal(t, api.AgentConfigurationApiVersion("gateway.api-platform.wso2.com/v1"), cfg.ApiVersion)
	assert.Equal(t, api.AgentConfigurationKindAgent, cfg.Kind)
	assert.Equal(t, "weather-agent-v1-0", cfg.Metadata.Name)
	require.NotNil(t, cfg.Metadata.Labels)
	assert.Equal(t, "agents", (*cfg.Metadata.Labels)["team"])
	require.NotNil(t, cfg.Metadata.Annotations)
	assert.Equal(t, "019d953f-d386-7a64-aa92-1869a28292e0",
		(*cfg.Metadata.Annotations)["gateway.api-platform.wso2.com/project-id"])

	assert.Equal(t, "Weather Agent", cfg.Spec.DisplayName)
	assert.Equal(t, "v1.0", cfg.Spec.Version)
	require.NotNil(t, cfg.Spec.Context)
	assert.Equal(t, "/weather", *cfg.Spec.Context)
	require.NotNil(t, cfg.Spec.Vhost)
	assert.Equal(t, "agents.example.com", *cfg.Spec.Vhost)

	require.NotNil(t, cfg.Spec.Upstream.Url)
	assert.Equal(t, "https://weather.internal", *cfg.Spec.Upstream.Url)

	require.NotNil(t, cfg.Spec.Resilience)
	require.NotNil(t, cfg.Spec.Resilience.Timeout)
	assert.Equal(t, "30s", *cfg.Spec.Resilience.Timeout)
	require.NotNil(t, cfg.Spec.Resilience.IdleTimeout)
	assert.Equal(t, "5m", *cfg.Spec.Resilience.IdleTimeout)
}

// TestParseAgent_WorkedExample_OperationConfigs covers transports and the two
// policy levels, including policy params — those are free-form maps, and a
// parser that flattened or dropped a nested value would produce a policy that
// is configured differently from what the user wrote.
func TestParseAgent_WorkedExample_OperationConfigs(t *testing.T) {
	a2a := parseAgentWorkedExample(t).Spec.A2a

	assert.Equal(t, api.A2AConfigProtocolVersion("1.0"), a2a.ProtocolVersion)
	// The parsed protocol version is what later sections use to select an
	// operation table, an Agent Card model, and a signing field-presence table,
	// so the value the OpenAPI enum admits has to be one the registry answers
	// for. Section 4 turns this into a deployment-time rejection.
	assert.True(t, agentproto.IsSupportedVersion(agentproto.ProtocolVersion(a2a.ProtocolVersion)),
		"protocol version %q is accepted by the management API contract but not registered in common/agentproto", a2a.ProtocolVersion)

	require.Len(t, a2a.OperationConfigs.Transports, 2)
	assert.Equal(t, api.JSONRPC, a2a.OperationConfigs.Transports[0].ProtocolBinding)
	require.NotNil(t, a2a.OperationConfigs.Transports[0].PathPrefix)
	assert.Equal(t, "/rpc", *a2a.OperationConfigs.Transports[0].PathPrefix)
	assert.Equal(t, api.HTTPJSON, a2a.OperationConfigs.Transports[1].ProtocolBinding)
	require.NotNil(t, a2a.OperationConfigs.Transports[1].PathPrefix)
	assert.Equal(t, "/rest", *a2a.OperationConfigs.Transports[1].PathPrefix)

	require.NotNil(t, a2a.OperationConfigs.Policies)
	require.Len(t, *a2a.OperationConfigs.Policies, 1)
	agentWide := (*a2a.OperationConfigs.Policies)[0]
	assert.Equal(t, "jwt-auth", agentWide.Name)
	assert.Equal(t, "v1", agentWide.Version)
	require.NotNil(t, agentWide.Params)
	assert.Equal(t, "https://idp.example.com", (*agentWide.Params)["issuer"])
	assert.Equal(t, []interface{}{"a2a.invoke"}, (*agentWide.Params)["requiredScopes"])

	require.NotNil(t, a2a.OperationConfigs.Operations)
	require.Len(t, *a2a.OperationConfigs.Operations, 1)
	op := (*a2a.OperationConfigs.Operations)[0]
	assert.Equal(t, api.SendMessage, op.Name)
	assert.True(t, agentproto.IsOperation(agentproto.ProtocolVersion(a2a.ProtocolVersion), string(op.Name)),
		"the generated operation-name enum must stay a subset of the operation set registered for this protocol version")

	require.NotNil(t, op.Policies)
	require.Len(t, *op.Policies, 1)
	limit := (*op.Policies)[0]
	assert.Equal(t, "advanced-ratelimit", limit.Name)
	require.NotNil(t, limit.Params)
	quotas, ok := (*limit.Params)["quotas"].([]interface{})
	require.True(t, ok, "quotas should survive parsing as a list, got %T", (*limit.Params)["quotas"])
	require.Len(t, quotas, 1)
	quota, ok := quotas[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "send-message-limit", quota["name"])
	limits, ok := quota["limits"].([]interface{})
	require.True(t, ok)
	require.Len(t, limits, 1)
	first, ok := limits[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1m", first["duration"])
	assert.EqualValues(t, 100, first["limit"])
}

// TestParseAgent_WorkedExample_AgentCard covers the embedded card document.
// Every assertion here is on a nested value: the card is stored, served, and
// eventually signed as the user supplied it, so parsing must not reshape it.
func TestParseAgent_WorkedExample_AgentCard(t *testing.T) {
	card := parseAgentWorkedExample(t).Spec.A2a.AgentCard

	assert.Nil(t, card.Protected, "the worked example configures no protected card")

	pub := card.Public
	assert.Equal(t, api.A2APublicAgentCardModeManaged, pub.Mode)
	require.NotNil(t, pub.Path)
	assert.Equal(t, "/.well-known/agent-card.json", *pub.Path)

	require.NotNil(t, pub.Policies)
	require.Len(t, *pub.Policies, 1)
	assert.Equal(t, "cors", (*pub.Policies)[0].Name)

	// `enabled` is the whole signing contract an Agent author writes; the
	// algorithm and kid come from administrator-owned gateway config.
	require.NotNil(t, pub.Signing)
	assert.True(t, pub.Signing.Enabled)

	require.NotNil(t, pub.Content)
	content := map[string]interface{}(*pub.Content)

	assert.Equal(t, "Weather Agent", content["name"])
	assert.Equal(t, "Provides weather information", content["description"])
	assert.Equal(t, "1.0.0", content["version"])
	assert.Equal(t, []interface{}{"text/plain"}, content["defaultInputModes"])
	assert.Equal(t, []interface{}{"text/plain"}, content["defaultOutputModes"])

	interfaces, ok := content["supportedInterfaces"].([]interface{})
	require.True(t, ok, "supportedInterfaces should be a list, got %T", content["supportedInterfaces"])
	require.Len(t, interfaces, 2)
	rpcIface := requireCardObject(t, interfaces[0])
	assert.Equal(t, "JSONRPC", rpcIface["protocolBinding"])
	assert.Equal(t, "1.0", rpcIface["protocolVersion"])
	assert.Equal(t, "https://agents.example.com/weather/rpc", rpcIface["url"])
	restIface := requireCardObject(t, interfaces[1])
	assert.Equal(t, "HTTP+JSON", restIface["protocolBinding"])
	assert.Equal(t, "https://agents.example.com/weather/rest", restIface["url"])

	capabilities := requireCardObject(t, content["capabilities"])
	assert.Equal(t, true, capabilities["streaming"])

	// The full securitySchemes nesting matters: the scheme type is the inner
	// key, and card-to-policy consistency checks read it from there.
	schemes := requireCardObject(t, content["securitySchemes"])
	jwtScheme := requireCardObject(t, schemes["gateway-jwt"])
	oidc := requireCardObject(t, jwtScheme["openIdConnectSecurityScheme"])
	assert.Equal(t, "https://idp.example.com/.well-known/openid-configuration", oidc["openIdConnectUrl"])

	requirements, ok := content["securityRequirements"].([]interface{})
	require.True(t, ok)
	require.Len(t, requirements, 1)
	requirement := requireCardObject(t, requirements[0])
	reqSchemes := requireCardObject(t, requirement["schemes"])
	require.NotNil(t, reqSchemes, "securityRequirements[0].schemes must survive parsing — scope checks read scopes from here")
	entry := requireCardObject(t, reqSchemes["gateway-jwt"])
	assert.Equal(t, []interface{}{"a2a.invoke"}, entry["list"])

	skills, ok := content["skills"].([]interface{})
	require.True(t, ok)
	require.Len(t, skills, 1)
	skill := requireCardObject(t, skills[0])
	assert.Equal(t, "get_weather", skill["id"])
	assert.Equal(t, []interface{}{"weather"}, skill["tags"])
}

// TestParseAgent_CardDocumentSurvivesDirectYAMLParsing pins the reason Agent
// needs no special parsing branch: the Agent Card document is declared as
// map[string]interface{}, which yaml.v3 fills directly from the YAML tree, so
// routing the artifact through a JSON intermediate first would buy nothing.
//
// It asserts the exact key set rather than a sample, because the failure this
// guards against is a silently dropped field: a card that loses a key still
// parses, still deploys, and only misbehaves once a client reads it.
func TestParseAgent_CardDocumentSurvivesDirectYAMLParsing(t *testing.T) {
	content := parseAgentWorkedExample(t).Spec.A2a.AgentCard.Public.Content
	require.NotNil(t, content)

	// The conversion compiles because A2AAgentCardDocument is a named
	// map[string]interface{}; the assertion below is that YAML decoding
	// populated it rather than leaving it empty.
	card := map[string]interface{}(*content)
	assert.ElementsMatch(t, []string{
		"name", "description", "version", "supportedInterfaces", "capabilities",
		"securitySchemes", "securityRequirements", "defaultInputModes",
		"defaultOutputModes", "skills",
	}, slices.Collect(maps.Keys(card)))

	// Scalars keep their YAML types: strings stay strings, booleans stay
	// booleans, and a quoted version stays a string rather than becoming a
	// number.
	assert.IsType(t, "", card["name"])
	assert.IsType(t, "", card["version"])
	capabilities := requireCardObject(t, card["capabilities"])
	assert.IsType(t, true, capabilities["streaming"])
}

// TestParseAgent_CardExtensionFieldsSurvive checks that a card field the
// gateway knows nothing about is carried through untouched. A2A cards are
// extensible, and a dropped extension field changes the bytes that get served
// and signed, which breaks verification client-side with nothing to see here.
func TestParseAgent_CardExtensionFieldsSurvive(t *testing.T) {
	const yamlWithExtension = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: extended-agent-v1-0
spec:
  displayName: Extended Agent
  version: v1.0
  context: /extended
  upstream:
    url: https://extended.internal
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
    agentCard:
      public:
        mode: managed
        content: {
          "name": "Extended Agent",
          "description": "Carries extension fields",
          "version": "1.0.0",
          "supportedInterfaces": [
            {"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "https://agents.example.com/extended"}
          ],
          "capabilities": {"streaming": false, "x-vendor-capability": {"nested": ["value"]}},
          "defaultInputModes": ["text/plain"],
          "defaultOutputModes": ["text/plain"],
          "skills": [],
          "x-vendor-extension": {"answer": 42, "unicode": "héllo ✅"}
        }
`
	var cfg api.AgentConfiguration
	require.NoError(t, NewParser().ParseAPIConfigYAML([]byte(yamlWithExtension), &cfg))

	require.NotNil(t, cfg.Spec.A2a.AgentCard.Public.Content)
	content := map[string]interface{}(*cfg.Spec.A2a.AgentCard.Public.Content)

	require.Contains(t, content, "x-vendor-extension", "unknown top-level card field was dropped")
	extension := requireCardObject(t, content["x-vendor-extension"])
	assert.EqualValues(t, 42, extension["answer"])
	assert.Equal(t, "héllo ✅", extension["unicode"])

	capabilities := requireCardObject(t, content["capabilities"])
	assert.Equal(t, false, capabilities["streaming"], "a field at its default value must survive, not be pruned")
	require.Contains(t, capabilities, "x-vendor-capability", "unknown nested card field was dropped")
	nestedExt := requireCardObject(t, capabilities["x-vendor-capability"])
	assert.Equal(t, []interface{}{"value"}, nestedExt["nested"])

	assert.Equal(t, []interface{}{}, content["skills"], "an empty required list must survive as empty, not become nil")
}

// TestParseAgent_JSONAndYAMLAgree asserts the same Agent carries the same
// content whether it arrives as YAML or as JSON. Both content types are
// accepted on /agents, and a difference between the two paths would mean the
// artifact a user gets back depends on how they sent it.
//
// The two are compared as marshalled JSON rather than by deep-comparing the
// generated structs. oapi-codegen gives the upstream type an unexported
// json.RawMessage union that only the JSON decoder populates; nothing reads it
// (consumers use the exported Url, Ref, and Auth fields, and MarshalJSON emits
// those regardless), so a struct-level comparison would fail on state that has
// no bearing on the parsed artifact. Free-form card values differ in concrete
// type for the same reason — an integer decodes as int through YAML and
// float64 through JSON — and serialise identically.
func TestParseAgent_JSONAndYAMLAgree(t *testing.T) {
	fromYAML := parseAgentWorkedExample(t)

	asJSON, err := json.Marshal(fromYAML)
	require.NoError(t, err)

	var fromJSON api.AgentConfiguration
	require.NoError(t, NewParser().ParseJSON(asJSON, &fromJSON))

	roundTripped, err := json.Marshal(fromJSON)
	require.NoError(t, err)
	assert.JSONEq(t, string(asJSON), string(roundTripped))

	// Spot-check the exported fields consumers actually read, so this does not
	// rest on serialisation alone.
	require.NotNil(t, fromYAML.Spec.Upstream.Url)
	require.NotNil(t, fromJSON.Spec.Upstream.Url)
	assert.Equal(t, *fromYAML.Spec.Upstream.Url, *fromJSON.Spec.Upstream.Url)
	assert.Nil(t, fromJSON.Spec.Upstream.Ref)
	assert.Equal(t, fromYAML.Spec.Context, fromJSON.Spec.Context)
	assert.Equal(t, fromYAML.Spec.A2a.ProtocolVersion, fromJSON.Spec.A2a.ProtocolVersion)
	assert.Equal(t, len(fromYAML.Spec.A2a.OperationConfigs.Transports), len(fromJSON.Spec.A2a.OperationConfigs.Transports))

	require.NotNil(t, fromJSON.Spec.A2a.AgentCard.Public.Content)
	yamlCard := map[string]interface{}(*fromYAML.Spec.A2a.AgentCard.Public.Content)
	jsonCard := map[string]interface{}(*fromJSON.Spec.A2a.AgentCard.Public.Content)
	assert.Equal(t, len(yamlCard), len(jsonCard))
	for key := range yamlCard {
		assert.Contains(t, jsonCard, key, "card field %q survived one ingress path but not the other", key)
	}
}
