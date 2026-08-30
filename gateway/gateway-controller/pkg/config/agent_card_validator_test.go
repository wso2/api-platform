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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// Tests for the managed Agent Card's agreement with the gateway configuration
// that serves it. Unlike the routing tests in agent_validator_test.go, every
// test here writes its card out by hand: the card is the subject, so deriving
// it from the configuration would make the checks tautological.

// agentInterface builds one supportedInterfaces entry.
func agentInterface(binding api.A2AProtocolBinding, url string) map[string]interface{} {
	return map[string]interface{}{
		"protocolBinding": string(binding),
		"protocolVersion": "1.0",
		"url":             url,
	}
}

// cardWith returns validAgent with a managed card carrying exactly the supplied
// interfaces, and nothing else that validation reads.
func cardWith(interfaces ...map[string]interface{}) api.AgentConfiguration {
	entries := make([]interface{}, 0, len(interfaces))
	for _, iface := range interfaces {
		entries = append(entries, iface)
	}

	cfg := validAgent()
	cfg.Spec.A2a.AgentCard.Public.Content = &api.A2AAgentCardDocument{
		"name":                "Weather Agent",
		"version":             "1.0.0",
		"supportedInterfaces": entries,
	}
	return cfg
}

// messagesOf renders every error message, for assertions that care about what a
// rejection tells the author rather than only which field it names.
func messagesOf(errs []ValidationError) string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Field+": "+e.Message)
	}
	return strings.Join(out, "\n")
}

// protectedCardContent is a managed protected card that agrees with validAgent's
// single JSONRPC transport, so a test about one protected-card rule is not also
// exercising the interface-consistency rules that have their own tests.
//
// It carries a skill the public fixture does not, because that is what a
// protected card is for: the same agent, described more fully to an
// authenticated caller.
func protectedCardContent() *api.A2AAgentCardDocument {
	doc := api.A2AAgentCardDocument{
		"name":    "Weather Agent",
		"version": "1.0.0",
		"supportedInterfaces": []interface{}{
			agentInterface(api.JSONRPC, cardHost+"/weather/rpc"),
		},
		"skills": []interface{}{
			map[string]interface{}{"id": "forecast_history"},
		},
	}
	return &doc
}

// declareExtendedCardCapability adds the public card's promise that the agent
// serves an extended card at all. Configuring a protected card without it is its
// own rejection, with its own test, so every other protected-card test declares
// it and stays about the rule it is testing.
func declareExtendedCardCapability(cfg *api.AgentConfiguration) {
	public := cfg.Spec.A2a.AgentCard.Public
	if public.Content == nil {
		return
	}
	(*public.Content)["capabilities"] = map[string]interface{}{"extendedAgentCard": true}
}

// TestAgentCard_InterfacesMatchingTheTransportsAreAccepted covers the shapes an
// author will actually write, including the two the path arithmetic collapses:
// a transport at the context root, and an Agent with no context at all.
func TestAgentCard_InterfacesMatchingTheTransportsAreAccepted(t *testing.T) {
	tests := map[string]func() api.AgentConfiguration{
		"one transport": func() api.AgentConfiguration {
			return cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
		},
		"both transports": func() api.AgentConfiguration {
			cfg := cardWith(
				agentInterface(api.JSONRPC, cardHost+"/weather/rpc"),
				agentInterface(api.HTTPJSON, cardHost+"/weather/rest"),
			)
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/rest")
			return cfg
		},
		"declaration order does not have to match": func() api.AgentConfiguration {
			cfg := cardWith(
				agentInterface(api.HTTPJSON, cardHost+"/weather/rest"),
				agentInterface(api.JSONRPC, cardHost+"/weather/rpc"),
			)
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/rest")
			return cfg
		},
		"transport at the context itself": func() api.AgentConfiguration {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather"))
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/")
			return cfg
		},
		"transport with the prefix omitted": func() api.AgentConfiguration {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather"))
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, nil)
			return cfg
		},
		"no context, transport at the origin root": func() api.AgentConfiguration {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/"))
			cfg.Spec.Context = nil
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/")
			return cfg
		},
		"no context, url written without a trailing slash": func() api.AgentConfiguration {
			// "https://host" and "https://host/" address the same place; the
			// route arithmetic produces the latter, so both have to pass.
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost))
			cfg.Spec.Context = nil
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/")
			return cfg
		},
		"host is not compared": func() api.AgentConfiguration {
			// Interim: there is no configured public base URL to compare a
			// host against, so a card naming another host is accepted. This
			// pins the current behaviour so that adding the host check is a
			// deliberate change to a failing test, not a silent one.
			return cardWith(agentInterface(api.JSONRPC, "https://somewhere.else.example.com/weather/rpc"))
		},
		"port is part of the host, not the path": func() api.AgentConfiguration {
			return cardWith(agentInterface(api.JSONRPC, "https://agents.example.com:8443/weather/rpc"))
		},
		"empty tenant means no tenant": func() api.AgentConfiguration {
			iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
			iface["tenant"] = ""
			return cardWith(iface)
		},
		"unknown interface fields are the author's to write": func() api.AgentConfiguration {
			iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
			iface["x-vendor-hint"] = "anything"
			return cardWith(iface)
		},
	}

	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := build()

			assert.Empty(t, NewAgentValidator().Validate(&cfg), messagesOf(NewAgentValidator().Validate(&cfg)))
		})
	}
}

// TestAgentCard_InterfaceMismatches is the core of this section: every way a
// card can disagree with the gateway that serves it. None of these produce a
// runtime error — the gateway routes what it was configured to route and the
// client goes wherever the card said — so each one has to be a deployment
// failure.
func TestAgentCard_InterfaceMismatches(t *testing.T) {
	tests := []struct {
		name  string
		build func() api.AgentConfiguration
		field string
		// message, when set, must appear in the error reported for field.
		message string
	}{
		{
			name: "supportedInterfaces missing entirely",
			build: func() api.AgentConfiguration {
				cfg := validAgent()
				cfg.Spec.A2a.AgentCard.Public.Content = &api.A2AAgentCardDocument{
					"name": "Weather Agent", "version": "1.0.0",
				}
				return cfg
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces",
		},
		{
			name: "supportedInterfaces is not a list",
			build: func() api.AgentConfiguration {
				cfg := validAgent()
				cfg.Spec.A2a.AgentCard.Public.Content = &api.A2AAgentCardDocument{
					"name": "Weather Agent", "supportedInterfaces": "https://agents.example.com/weather/rpc",
				}
				return cfg
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces",
		},
		{
			name:  "supportedInterfaces is empty",
			build: func() api.AgentConfiguration { return cardWith() },
			field: "spec.a2a.agentCard.public.content.supportedInterfaces",
		},
		{
			name: "an entry is not an object",
			build: func() api.AgentConfiguration {
				cfg := validAgent()
				cfg.Spec.A2a.AgentCard.Public.Content = &api.A2AAgentCardDocument{
					"name":                "Weather Agent",
					"supportedInterfaces": []interface{}{"https://agents.example.com/weather/rpc"},
				}
				return cfg
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0]",
		},
		{
			name: "advertises a binding no transport exposes",
			build: func() api.AgentConfiguration {
				return cardWith(
					agentInterface(api.JSONRPC, cardHost+"/weather/rpc"),
					agentInterface(api.HTTPJSON, cardHost+"/weather/rest"),
				)
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[1].protocolBinding",
			message: "not exposed by",
		},
		{
			name: "a configured transport is not advertised",
			build: func() api.AgentConfiguration {
				cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
				cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/rest")
				return cfg
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces",
			message: "No Agent Card interface advertises protocolBinding 'HTTP+JSON'",
		},
		{
			name: "advertises a binding the protocol does not define",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface("GRPC", cardHost+"/weather/rpc"))
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].protocolBinding",
		},
		{
			name: "the same binding twice",
			build: func() api.AgentConfiguration {
				return cardWith(
					agentInterface(api.JSONRPC, cardHost+"/weather/rpc"),
					agentInterface(api.JSONRPC, cardHost+"/weather/rpc"),
				)
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[1].protocolBinding",
			message: "Duplicate protocolBinding",
		},
		{
			name: "protocolBinding missing",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				delete(iface, "protocolBinding")
				return cardWith(iface)
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].protocolBinding",
		},
		{
			name: "protocolVersion missing",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				delete(iface, "protocolVersion")
				return cardWith(iface)
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].protocolVersion",
		},
		{
			name: "protocolVersion disagrees with the agent's",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				iface["protocolVersion"] = "0.3"
				return cardWith(iface)
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].protocolVersion",
			message: "the agent exposes '1.0'",
		},
		{
			name: "protocolVersion is not a string",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				iface["protocolVersion"] = 1.0
				return cardWith(iface)
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].protocolVersion",
		},
		{
			name: "tenant is advertised",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				iface["tenant"] = "acme"
				return cardWith(iface)
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].tenant",
			message: "must not declare tenant",
		},
		{
			name: "url missing",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				delete(iface, "url")
				return cardWith(iface)
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
		},
		{
			name: "url is a bare path",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, "/weather/rpc"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "absolute URL",
		},
		{
			name: "url uses http",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, "http://agents.example.com/weather/rpc"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "must use https",
		},
		{
			name: "url carries userinfo",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, "https://user:pw@agents.example.com/weather/rpc"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "userinfo",
		},
		{
			name: "url carries a query string",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc?v=1"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "query string",
		},
		{
			name: "url carries a fragment",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc#send"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "fragment",
		},
		{
			name: "url path is not the transport's base path",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/jsonrpc"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "the gateway serves this transport at '/weather/rpc'",
		},
		{
			name: "url path omits the context",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, cardHost+"/rpc"))
			},
			field:   "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
			message: "the gateway serves this transport at '/weather/rpc'",
		},
		{
			name: "url path has a trailing slash the route does not",
			build: func() api.AgentConfiguration {
				return cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc/"))
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
		},
		{
			name: "url is not a string",
			build: func() api.AgentConfiguration {
				iface := agentInterface(api.JSONRPC, "")
				iface["url"] = 42
				return cardWith(iface)
			},
			field: "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.build()

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs, "expected a validation error for %s", tt.field)
			assert.Contains(t, fieldsOf(errs), tt.field, messagesOf(errs))
			if tt.message != "" {
				assert.Contains(t, messagesOf(errs), tt.message)
			}
		})
	}
}

// TestAgentCard_InterfacesAreNotCheckedAgainstBrokenTransports covers the
// gating. When the transports themselves did not resolve there is nothing
// meaningful to compare the card against, and reporting the card as
// inconsistent would point the author at the half that may well be correct.
func TestAgentCard_InterfacesAreNotCheckedAgainstBrokenTransports(t *testing.T) {
	tests := map[string]func() api.AgentConfiguration{
		"malformed transport prefix": func() api.AgentConfiguration {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "rpc")
			return cfg
		},
		"unsupported protocol binding": func() api.AgentConfiguration {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.A2AProtocolBinding("GRPC"), "/rpc")
			return cfg
		},
		"malformed context": func() api.AgentConfiguration {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
			cfg.Spec.Context = stringPtr("weather")
			return cfg
		},
	}

	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := build()

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs)
			for _, err := range errs {
				assert.NotContains(t, err.Field, "supportedInterfaces",
					"the card must not be blamed for a transport error: %s", messagesOf(errs))
			}
		})
	}
}

// TestAgentCard_InterfaceVersionIsNotCheckedAgainstAnUnknownProtocolVersion is
// the same gating for the other direction: with spec.a2a.protocolVersion
// rejected there is no version to hold the card to, and the card may be the
// correct half.
func TestAgentCard_InterfaceVersionIsNotCheckedAgainstAnUnknownProtocolVersion(t *testing.T) {
	cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
	cfg.Spec.A2a.ProtocolVersion = "9.9"

	errs := NewAgentValidator().Validate(&cfg)
	require.Len(t, errs, 1, messagesOf(errs))
	assert.Equal(t, "spec.a2a.protocolVersion", errs[0].Field)
}

// TestAgentCard_PassthroughIsNotInspected pins limitation L4 as behaviour: a
// proxied card is fetched from the upstream and never parsed, so none of these
// checks can run for one. The card that would be rejected outright in managed
// mode is simply absent here, and the gap has to be surfaced in deployment
// status rather than papered over with checks against something the gateway
// does not have.
func TestAgentCard_PassthroughIsNotInspected(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.A2a.AgentCard.Public.Mode = api.A2APublicAgentCardModePassthrough
	cfg.Spec.A2a.AgentCard.Public.Content = nil
	cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/rest")

	assert.Empty(t, NewAgentValidator().Validate(&cfg))
}

// TestAgentCard_PreSignedContentIsRejected covers the one field the gateway
// writes into the card. A signature supplied by the author was computed over a
// different document than the one the gateway will serve, so a client
// verifying it rejects the card — a failure visible only on the client side.
func TestAgentCard_PreSignedContentIsRejected(t *testing.T) {
	signatures := map[string]interface{}{
		"a signature": []interface{}{
			map[string]interface{}{"protected": "eyJhbGciOiJFUzI1NiJ9", "signature": "c2ln"},
		},
		// Rejected on presence, not on content: the field belongs to the
		// gateway either way, and an empty list would leave it ambiguous
		// whether the gateway is expected to append to it.
		"an empty signature list": []interface{}{},
	}

	for name, value := range signatures {
		t.Run(name, func(t *testing.T) {
			cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
			(*cfg.Spec.A2a.AgentCard.Public.Content)["signatures"] = value

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.content.signatures", messagesOf(errs))
		})
	}
}

// cardOfEncodedSize returns a valid Agent whose card encodes to exactly size
// bytes of JSON, padding the description to make up the difference.
//
// The padding is plain ASCII, so it costs one encoded byte per character and
// the arithmetic is exact rather than approximate — which is the point: the
// interesting cases for a fixed ceiling are the two bytes either side of it,
// and a fixture that only got "comfortably over" would not distinguish a
// correct comparison from an off-by-one.
func cardOfEncodedSize(t *testing.T, size int) api.AgentConfiguration {
	t.Helper()

	cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))
	content := cfg.Spec.A2a.AgentCard.Public.Content
	(*content)["description"] = ""

	base, err := json.Marshal(map[string]interface{}(*content))
	require.NoError(t, err)
	padding := size - len(base)
	require.GreaterOrEqual(t, padding, 0, "the fixture card is already larger than %d bytes", size)
	(*content)["description"] = strings.Repeat("a", padding)

	encoded, err := json.Marshal(map[string]interface{}(*content))
	require.NoError(t, err)
	require.Len(t, encoded, size, "padding arithmetic is wrong")

	return cfg
}

// TestAgentCard_SizeCap covers the ceiling at the boundary. It is a fixed
// constant rather than configuration — 1 MiB is the largest object Kubernetes
// stores by default, so a bigger card could not be applied as a custom resource
// anyway — which means the only interesting cases are the bytes either side of
// it.
func TestAgentCard_SizeCap(t *testing.T) {
	t.Run("a card exactly at the cap is accepted", func(t *testing.T) {
		cfg := cardOfEncodedSize(t, maxAgentCardBytes)

		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})

	t.Run("one byte over the cap is rejected", func(t *testing.T) {
		cfg := cardOfEncodedSize(t, maxAgentCardBytes+1)

		errs := NewAgentValidator().Validate(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.content", messagesOf(errs))
		assert.Contains(t, messagesOf(errs), "exceeds the maximum")
	})

	t.Run("an ordinary card is nowhere near the cap", func(t *testing.T) {
		// A realistic card is a few kilobytes; the ceiling exists for the
		// pathological case, not to ration normal use.
		cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/rpc"))

		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})
}

// TestAgentCard_SizeIsMeasuredOverTheJSONEncoding pins what the cap counts. The
// card is delivered as JSON, so a limit measured over anything else — a field
// count, the YAML the author typed — would not bound the thing that actually
// has to fit.
func TestAgentCard_SizeIsMeasuredOverTheJSONEncoding(t *testing.T) {
	cfg := cardOfEncodedSize(t, maxAgentCardBytes+1)
	content := map[string]interface{}(*cfg.Spec.A2a.AgentCard.Public.Content)

	encoded, err := json.Marshal(content)
	require.NoError(t, err)

	errs := NewAgentValidator().Validate(&cfg)
	require.NotEmpty(t, errs)
	assert.Contains(t, messagesOf(errs), fmt.Sprintf("is %d bytes", len(encoded)),
		"the reported size must be the encoded length, so an author can tell how much to cut")
}

// TestAgentCard_WalksBothDecoderShapes is the regression guard for the one way
// these checks could pass everywhere and enforce nothing.
//
// A card's nested mappings are typed by whichever decoder produced them:
// yaml.v3 reuses the enclosing named map type on the management-API path, while
// encoding/json produces plain maps on the storage path. A validator that
// handled only one shape would fail to see the interfaces on the other and skip
// every check silently.
func TestAgentCard_WalksBothDecoderShapes(t *testing.T) {
	broken := map[string]interface{}{
		"protocolBinding": string(api.JSONRPC),
		"protocolVersion": "1.0",
		"url":             cardHost + "/weather/wrong",
	}

	shapes := map[string]interface{}{
		"plain map":          broken,
		"named card-doc map": api.A2AAgentCardDocument(broken),
	}

	for name, iface := range shapes {
		t.Run(name, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.AgentCard.Public.Content = &api.A2AAgentCardDocument{
				"name":                "Weather Agent",
				"supportedInterfaces": []interface{}{iface},
			}

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.content.supportedInterfaces[0].url",
				messagesOf(errs))
		})
	}
}

// TestAgentCard_ContentIsNeverRewritten is the invariant behind every rejection
// above: validation reads the card, it never repairs it. Once the gateway signs
// the card it serves, any normalization here would change the bytes under the
// signature — and even before then, silently correcting an author's discovery
// document hides the mistake rather than reporting it.
func TestAgentCard_ContentIsNeverRewritten(t *testing.T) {
	cfg := cardWith(agentInterface(api.JSONRPC, cardHost+"/weather/wrong"))
	(*cfg.Spec.A2a.AgentCard.Public.Content)["x-vendor-extension"] = map[string]interface{}{"kept": true}

	before, err := json.Marshal(map[string]interface{}(*cfg.Spec.A2a.AgentCard.Public.Content))
	require.NoError(t, err)

	require.NotEmpty(t, NewAgentValidator().Validate(&cfg), "this fixture is meant to be rejected")

	after, err := json.Marshal(map[string]interface{}(*cfg.Spec.A2a.AgentCard.Public.Content))
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))
}

// ─── Protected (extended) Agent Card ─────────────────────────────────────────
//
// The protected representation runs the same content checks as the public one,
// against the same transports, so those rules are not re-tested here in full —
// what is tested is that they run at all, and that they report the protected
// field rather than the public one. The rules that exist only for this
// representation are the mode contract and the public card's capability promise.

// An omitted protected block is not a protected card configured as passthrough:
// it is the behaviour that shipped before protected cards existed, where
// GetExtendedAgentCard is proxied upstream with no gateway-added guard. An Agent
// written against that must keep working across a controller upgrade, so the
// absence has to stay meaningful rather than being normalized into a mode.
func TestProtectedCard_OmittedBlockIsAccepted(t *testing.T) {
	cfg := validAgent()
	require.Nil(t, cfg.Spec.A2a.AgentCard.Protected)

	assert.Empty(t, NewAgentValidator().Validate(&cfg))
}

func TestProtectedCard_ExplicitModesAreAccepted(t *testing.T) {
	t.Run("passthrough", func(t *testing.T) {
		cfg := validAgent()
		declareExtendedCardCapability(&cfg)
		cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
			Mode: api.A2AProtectedAgentCardModePassthrough,
		}

		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})

	t.Run("managed", func(t *testing.T) {
		cfg := validAgent()
		declareExtendedCardCapability(&cfg)
		cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
			Mode:    api.A2AProtectedAgentCardModeManaged,
			Content: protectedCardContent(),
		}

		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})

	// A passthrough public card is proxied unparsed, so there is no capability
	// promise for the gateway to check against — but the protected card is still
	// configurable, and the authentication it requires is still enforced at
	// runtime.
	t.Run("managed protected beside a passthrough public card", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.AgentCard.Public.Mode = api.A2APublicAgentCardModePassthrough
		cfg.Spec.A2a.AgentCard.Public.Content = nil
		cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
			Mode:    api.A2AProtectedAgentCardModeManaged,
			Content: protectedCardContent(),
		}

		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})
}

// The mode contract, which mirrors the public card's: managed owns a document,
// passthrough proxies one and can neither supply nor sign it.
func TestProtectedCard_ModeRules(t *testing.T) {
	tests := map[string]struct {
		protected *api.A2AProtectedAgentCard
		field     string
	}{
		"managed with no content": {
			protected: &api.A2AProtectedAgentCard{Mode: api.A2AProtectedAgentCardModeManaged},
			field:     "spec.a2a.agentCard.protected.content",
		},
		"managed with empty content": {
			protected: &api.A2AProtectedAgentCard{
				Mode:    api.A2AProtectedAgentCardModeManaged,
				Content: &api.A2AAgentCardDocument{},
			},
			field: "spec.a2a.agentCard.protected.content",
		},
		"passthrough with content": {
			protected: &api.A2AProtectedAgentCard{
				Mode:    api.A2AProtectedAgentCardModePassthrough,
				Content: protectedCardContent(),
			},
			field: "spec.a2a.agentCard.protected.content",
		},
		"passthrough with signing": {
			protected: &api.A2AProtectedAgentCard{
				Mode:    api.A2AProtectedAgentCardModePassthrough,
				Signing: &api.A2ACardSigning{Enabled: false},
			},
			field: "spec.a2a.agentCard.protected.signing",
		},
		"unknown mode": {
			protected: &api.A2AProtectedAgentCard{Mode: "served"},
			field:     "spec.a2a.agentCard.protected.mode",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validAgent()
			declareExtendedCardCapability(&cfg)
			cfg.Spec.A2a.AgentCard.Protected = tt.protected

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field, messagesOf(errs))
		})
	}
}

// The managed-card checks run over the protected document too, and report it.
// Reported against the public content field they would send an author to edit a
// document that is correct, and leave the one that is wrong in place.
func TestProtectedCard_ManagedChecksRunAndNameTheProtectedField(t *testing.T) {
	tests := map[string]struct {
		mutate func(doc api.A2AAgentCardDocument)
		field  string
	}{
		"pre-signed": {
			mutate: func(doc api.A2AAgentCardDocument) {
				doc["signatures"] = []interface{}{map[string]interface{}{"signature": "abc"}}
			},
			field: "spec.a2a.agentCard.protected.content.signatures",
		},
		"advertises an unconfigured binding": {
			mutate: func(doc api.A2AAgentCardDocument) {
				doc["supportedInterfaces"] = []interface{}{
					agentInterface(api.HTTPJSON, cardHost+"/weather/rest"),
				}
			},
			field: "spec.a2a.agentCard.protected.content.supportedInterfaces[0].protocolBinding",
		},
		"interface url path disagrees with the gateway": {
			mutate: func(doc api.A2AAgentCardDocument) {
				doc["supportedInterfaces"] = []interface{}{
					agentInterface(api.JSONRPC, cardHost+"/weather/elsewhere"),
				}
			},
			field: "spec.a2a.agentCard.protected.content.supportedInterfaces[0].url",
		},
		"declares a tenant": {
			mutate: func(doc api.A2AAgentCardDocument) {
				iface := agentInterface(api.JSONRPC, cardHost+"/weather/rpc")
				iface["tenant"] = "acme"
				doc["supportedInterfaces"] = []interface{}{iface}
			},
			field: "spec.a2a.agentCard.protected.content.supportedInterfaces[0].tenant",
		},
		"over the size ceiling": {
			mutate: func(doc api.A2AAgentCardDocument) {
				doc["description"] = strings.Repeat("a", maxAgentCardBytes+1)
			},
			field: "spec.a2a.agentCard.protected.content",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validAgent()
			declareExtendedCardCapability(&cfg)
			content := protectedCardContent()
			tt.mutate(*content)
			cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
				Mode:    api.A2AProtectedAgentCardModeManaged,
				Content: content,
			}

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field, messagesOf(errs))
			for _, field := range fieldsOf(errs) {
				assert.NotContains(t, field, "agentCard.public.content.supportedInterfaces",
					"a protected-card fault was reported against the public card")
			}
		})
	}
}

// The two representations have independent size budgets, because each is stored
// and served separately. A pair that is individually within the ceiling is
// accepted; what that pair contributes to a policy-xDS snapshot is a recorded
// risk about the snapshot's own missing bounds, not something a lower combined
// cap here would honestly describe.
func TestProtectedCard_SizeBudgetsAreIndependent(t *testing.T) {
	// Comfortably under the per-card ceiling each, and comfortably over it
	// together.
	filler := strings.Repeat("a", (maxAgentCardBytes/2)+1024)

	cfg := validAgent()
	declareExtendedCardCapability(&cfg)
	(*cfg.Spec.A2a.AgentCard.Public.Content)["description"] = filler

	content := protectedCardContent()
	(*content)["description"] = filler
	cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
		Mode:    api.A2AProtectedAgentCardModeManaged,
		Content: content,
	}

	combined, err := json.Marshal([]interface{}{
		map[string]interface{}(*cfg.Spec.A2a.AgentCard.Public.Content),
		map[string]interface{}(*content),
	})
	require.NoError(t, err)
	require.Greater(t, len(combined), maxAgentCardBytes,
		"this fixture is meant to exceed the ceiling only in combination")

	assert.Empty(t, NewAgentValidator().Validate(&cfg))
}

// Configuring a protected card is a promise that the extended-card operation
// exists. A client reads capabilities.extendedAgentCard off the public card and
// only then calls GetExtendedAgentCard, so a card that does not declare it
// produces an operation the gateway serves and no conformant client ever asks
// for.
func TestProtectedCard_ManagedPublicCardMustDeclareTheCapability(t *testing.T) {
	const field = "spec.a2a.agentCard.public.content.capabilities.extendedAgentCard"

	tests := map[string]struct {
		capabilities interface{}
		present      bool
		field        string
	}{
		"capabilities absent": {present: false, field: field},
		"capabilities is not an object": {
			capabilities: "streaming",
			present:      true,
			field:        "spec.a2a.agentCard.public.content.capabilities",
		},
		"flag absent": {
			capabilities: map[string]interface{}{"streaming": true},
			present:      true,
			field:        field,
		},
		"flag is false": {
			capabilities: map[string]interface{}{"extendedAgentCard": false},
			present:      true,
			field:        field,
		},
		// A quoted "true" is a different JSON value, and it is the card's own
		// bytes that reach clients: one deserializing the card against the A2A
		// model reads a type error, not a capability.
		"flag is the string true": {
			capabilities: map[string]interface{}{"extendedAgentCard": "true"},
			present:      true,
			field:        field,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validAgent()
			if tt.present {
				(*cfg.Spec.A2a.AgentCard.Public.Content)["capabilities"] = tt.capabilities
			}
			cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
				Mode: api.A2AProtectedAgentCardModePassthrough,
			}

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field, messagesOf(errs))
		})
	}

	// The promise is only checked when there is a protected card to promise. An
	// Agent with no protected block is free to leave the capability out — and a
	// public card that declares it without a protected block is the author's own
	// business, since the operation is proxied upstream and the upstream may well
	// serve one.
	t.Run("not required without a protected block", func(t *testing.T) {
		cfg := validAgent()
		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})
}

// The card document is carried as a free-form map whose nested mappings come
// back typed by whichever decoder produced them — yaml.v3 reuses the enclosing
// named type, encoding/json produces a plain map. Both ingress paths are real
// (YAML from the management API, JSON from storage), so a walker that handled
// only one shape would validate the protected card on one path and skip it
// entirely on the other.
func TestProtectedCard_BothDecoderShapesAreWalked(t *testing.T) {
	capabilities := map[string]interface{}{"extendedAgentCard": true}

	shapes := map[string]interface{}{
		"plain map":          capabilities,
		"named card-doc map": api.A2AAgentCardDocument(capabilities),
	}

	for name, shape := range shapes {
		t.Run(name, func(t *testing.T) {
			cfg := validAgent()
			(*cfg.Spec.A2a.AgentCard.Public.Content)["capabilities"] = shape
			cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
				Mode: api.A2AProtectedAgentCardModePassthrough,
			}

			assert.Empty(t, NewAgentValidator().Validate(&cfg))
		})
	}
}
