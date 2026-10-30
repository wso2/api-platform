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

package utils

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func wsStrPtr(s string) *string { return &s }

func validWebSocketAPI(mutate func(spec *api.WebSocketAPIConfigData)) api.WebSocketAPI {
	spec := api.WebSocketAPIConfigData{
		DisplayName: "Chat",
		Version:     "v1.0",
		Context:     "/chat",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: wsStrPtr("wss://chat-backend:8443/ws")},
		},
		Channels: []api.WebSocketChannel{{Path: "/rooms/{roomId}"}},
	}
	if mutate != nil {
		mutate(&spec)
	}
	return api.WebSocketAPI{
		ApiVersion: api.WebSocketAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.WebSocketAPIKindWebSocketApi,
		Metadata:   api.Metadata{Name: "chat-websocket-v1.0"},
		Spec:       spec,
	}
}

// fieldsWithErrors returns the set of field paths that produced a validation
// error, so assertions name the field rather than matching message prose.
func fieldsWithErrors(errs []config.ValidationError) map[string]bool {
	out := make(map[string]bool, len(errs))
	for _, e := range errs {
		out[e.Field] = true
	}
	return out
}

func TestValidateWebSocketAPIConfig_Valid(t *testing.T) {
	name, version, errs := validateWebSocketAPIConfig(validWebSocketAPI(nil))
	assert.Equal(t, "Chat", name)
	assert.Equal(t, "v1.0", version)
	assert.Empty(t, errs)
}

func TestValidateWebSocketAPIConfig_RejectsWrongType(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(api.GraphQLAPI{})
	require.Len(t, errs, 1)
	assert.Equal(t, "config", errs[0].Field)
}

// FR-003: at least one channel.
func TestValidateWebSocketAPIConfig_RequiresAtLeastOneChannel(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Channels = nil
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.channels"], "got %+v", errs)
}

// FR-003: duplicate channel paths are rejected rather than last-one-wins, which
// would silently drop one channel's policies while leaving it in the artifact.
func TestValidateWebSocketAPIConfig_RejectsDuplicateChannelPaths(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Channels = []api.WebSocketChannel{
			{Path: "/rooms/{roomId}"},
			{Path: "/notifications"},
			{Path: "/rooms/{roomId}"},
		}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.channels[2].path"], "got %+v", errs)
}

func TestValidateWebSocketAPIConfig_ChannelPathRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		field string
	}{
		{"empty", "   ", "spec.channels[0].path"},
		{"no leading slash", "rooms", "spec.channels[0].path"},
		// A wildcard would become a prefix match that upgrades every path below it.
		{"wildcard", "/rooms/*", "spec.channels[0].path"},
		{"root wildcard", "/*", "spec.channels[0].path"},
		// These describe paths a normalised request never carries.
		{"query", "/rooms?admin=1", "spec.channels[0].path"},
		{"fragment", "/rooms#x", "spec.channels[0].path"},
		{"dot-dot segment", "/rooms/../admin", "spec.channels[0].path"},
		{"dot segment", "/./rooms", "spec.channels[0].path"},
		{"empty segment", "/rooms//general", "spec.channels[0].path"},
		{"unbalanced braces", "/rooms/{roomId", "spec.channels[0].path"},
		{"empty parameter name", "/rooms/{}", "spec.channels[0].path"},
		{"reversed braces", "/rooms/}x{", "spec.channels[0].path"},
		{"space", "/my room", "spec.channels[0].path"},
		{"non-ASCII", "/räume", "spec.channels[0].path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				spec.Channels = []api.WebSocketChannel{{Path: tc.path}}
			}))
			assert.True(t, fieldsWithErrors(errs)[tc.field], "got %+v", errs)
		})
	}

	for _, path := range []string{"/", "/rooms/{roomId}", "/rooms/", "/a.b/c-d_e", "/rooms/{roomId}/messages", "/rooms/{room-id}", "/rooms/room-{id}", "/f/{a}-{b}", "/x%20y"} {
		_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
			spec.Channels = []api.WebSocketChannel{{Path: path}}
		}))
		assert.False(t, fieldsWithErrors(errs)["spec.channels[0].path"], "channel path %q should be accepted, got %+v", path, errs)
	}
}

// Two channels that differ only in parameter names match the same requests, so
// the second would never be used and its policies would silently not apply.
// An exact path beside a parameterised one is allowed: the exact route wins.
func TestValidateWebSocketAPIConfig_RejectsEquivalentChannelPaths(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Channels = []api.WebSocketChannel{{Path: "/rooms/{a}"}, {Path: "/rooms/{b}"}}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.channels[1].path"], "got %+v", errs)

	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Channels = []api.WebSocketChannel{{Path: "/rooms/general"}, {Path: "/rooms/{roomId}"}}
	}))
	assert.Empty(t, errs, "an exact channel beside a parameterised one is allowed")
}

// A channel may not claim a path the gateway serves on every virtual host, the
// same reservation every other API kind honours, after $version is expanded and
// dot segments are resolved.
func TestValidateWebSocketAPIConfig_RejectsReservedGatewayPaths(t *testing.T) {
	for _, tc := range []struct{ context, channel string }{
		{"/_gateway-health", "/ready"},
		{"/_gateway-health/$version", "/x"},
		{"/", "/_gateway-health/healthy"},
		{"/.well-known", "/jwks"},
	} {
		_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
			spec.Context = tc.context
			spec.Channels = []api.WebSocketChannel{{Path: tc.channel}}
		}))
		assert.True(t, fieldsWithErrors(errs)["spec.channels[0].path"], "context %q + channel %q must be rejected, got %+v", tc.context, tc.channel, errs)
	}
}

// FR-004: the upstream leg is a WebSocket, so an http/https URL is a
// configuration mistake rather than something to silently coerce.
func TestValidateWebSocketAPIConfig_RejectsNonWebSocketScheme(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Url: wsStrPtr("https://chat-backend:8443/ws")}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.main.url"], "got %+v", errs)
}

// wsUpstreamDefinition builds an upstream definition with one target per url.
func wsUpstreamDefinition(name string, urls ...string) api.WebSocketUpstreamDefinition {
	def := api.WebSocketUpstreamDefinition{Name: name}
	for _, u := range urls {
		def.Upstreams = append(def.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
	}
	return def
}

// withWSDefinitions sets the definitions and points main at the first.
func withWSDefinitions(spec *api.WebSocketAPIConfigData, defs ...api.WebSocketUpstreamDefinition) {
	spec.UpstreamDefinitions = &defs
	spec.Upstream.Main = api.Upstream{Ref: wsStrPtr(defs[0].Name)}
}

func TestValidateWebSocketAPIConfig_AcceptsRefToDefinition(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		def := wsUpstreamDefinition("chat-backend", "wss://chat-1:8443", "wss://chat-2:8443")
		def.BasePath = wsStrPtr("/ws")
		def.Tls = &map[string]interface{}{"verifyHostName": true}
		withWSDefinitions(spec, def)
		spec.Upstream.Sandbox = &api.Upstream{Ref: wsStrPtr("chat-backend")}
	}))
	assert.Empty(t, errs)
}

func TestValidateWebSocketAPIConfig_RejectsUnknownRef(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Ref: wsStrPtr("shared-upstream")}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.main.ref"], "got %+v", errs)
}

// A slot takes a url or a ref, never both and never neither.
func TestValidateWebSocketAPIConfig_RequiresExactlyOneOfURLAndRef(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		withWSDefinitions(spec, wsUpstreamDefinition("chat-backend", "ws://chat:8080"))
		spec.Upstream.Main.Url = wsStrPtr("ws://chat:8080/ws")
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.main"], "both: got %+v", errs)

	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.main.url"], "neither: got %+v", errs)
}

func TestValidateWebSocketAPIConfig_UpstreamDefinitionRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		def   func() api.WebSocketUpstreamDefinition
		field string
	}{
		{"bad name", func() api.WebSocketUpstreamDefinition { return wsUpstreamDefinition("chat backend!", "ws://chat:8080") }, "spec.upstreamDefinitions[0].name"},
		{"no targets", func() api.WebSocketUpstreamDefinition { return wsUpstreamDefinition("chat") }, "spec.upstreamDefinitions[0].upstreams"},
		{"http target", func() api.WebSocketUpstreamDefinition { return wsUpstreamDefinition("chat", "http://chat:8080") }, "spec.upstreamDefinitions[0].upstreams[0].url"},
		{"target with path", func() api.WebSocketUpstreamDefinition { return wsUpstreamDefinition("chat", "ws://chat:8080/ws") }, "spec.upstreamDefinitions[0].upstreams[0].url"},
		{"mixed schemes", func() api.WebSocketUpstreamDefinition {
			return wsUpstreamDefinition("chat", "ws://chat-1:8080", "wss://chat-2:8443")
		}, "spec.upstreamDefinitions[0].upstreams"},
		{"tls on ws", func() api.WebSocketUpstreamDefinition {
			d := wsUpstreamDefinition("chat", "ws://chat:8080")
			d.Tls = &map[string]interface{}{"verifyHostName": true}
			return d
		}, "spec.upstreamDefinitions[0].tls"},
		{"bad basePath", func() api.WebSocketUpstreamDefinition {
			d := wsUpstreamDefinition("chat", "ws://chat:8080")
			d.BasePath = wsStrPtr("ws/")
			return d
		}, "spec.upstreamDefinitions[0].basePath"},
		{"bad connect timeout", func() api.WebSocketUpstreamDefinition {
			d := wsUpstreamDefinition("chat", "ws://chat:8080")
			d.Timeout = &api.UpstreamTimeout{Connect: wsStrPtr("0s")}
			return d
		}, "spec.upstreamDefinitions[0].timeout.connect"},
		{"weight above 100", func() api.WebSocketUpstreamDefinition {
			d := wsUpstreamDefinition("chat", "ws://chat-1:8080", "ws://chat-2:8080")
			over := 101
			d.Upstreams[1].Weight = &over
			return d
		}, "spec.upstreamDefinitions[0].upstreams[1].weight"},
		{"negative weight", func() api.WebSocketUpstreamDefinition {
			d := wsUpstreamDefinition("chat", "ws://chat:8080")
			negative := -1
			d.Upstreams[0].Weight = &negative
			return d
		}, "spec.upstreamDefinitions[0].upstreams[0].weight"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				withWSDefinitions(spec, tc.def())
			}))
			assert.True(t, fieldsWithErrors(errs)[tc.field], "got %+v", errs)
		})
	}

	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		withWSDefinitions(spec, wsUpstreamDefinition("chat", "ws://a:8080"), wsUpstreamDefinition("chat", "ws://b:8080"))
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstreamDefinitions[1].name"], "duplicate name: got %+v", errs)

	// 0 and 100 are the inclusive bounds, as on a RestApi.
	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		d := wsUpstreamDefinition("chat", "ws://chat-1:8080", "ws://chat-2:8080")
		low, high := 0, 100
		d.Upstreams[0].Weight, d.Upstreams[1].Weight = &low, &high
		withWSDefinitions(spec, d)
	}))
	assert.Empty(t, errs, "weights 0 and 100 are valid")
}

func TestValidateWebSocketAPIConfig_ValidatesSandboxToo(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Sandbox = &api.Upstream{Url: wsStrPtr("http://sandbox:8080")}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.sandbox.url"], "got %+v", errs)
}

// A zero, negative or unparseable connect timeout is rejected rather than read as
// "no timeout".
func TestValidateWebSocketAPIConfig_RejectsNonPositiveDurations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"zero", "0s"},
		{"negative", "-5s"},
		{"unparseable", "soon"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				def := wsUpstreamDefinition("chat", "ws://chat:8080")
				def.Timeout = &api.UpstreamTimeout{Connect: wsStrPtr(tc.value)}
				withWSDefinitions(spec, def)
			}))
			assert.True(t, fieldsWithErrors(errs)["spec.upstreamDefinitions[0].timeout.connect"], "got %+v", errs)
		})
	}
}

func TestValidateWebSocketAPIConfig_ValidatesDefinitionConnectionBlock(t *testing.T) {
	zero := 0
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		def := wsUpstreamDefinition("chat", "wss://chat-backend:8443")
		def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &zero, MaxPendingHandshakes: &zero}
		withWSDefinitions(spec, def)
	}))
	fields := fieldsWithErrors(errs)
	assert.True(t, fields["spec.upstreamDefinitions[0].connection.maxConnections"], "got %+v", errs)
	assert.True(t, fields["spec.upstreamDefinitions[0].connection.maxPendingHandshakes"], "got %+v", errs)
}

func TestValidateWebSocketAPIConfig_ContextRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context string
	}{
		{"empty", ""},
		{"no leading slash", "chat"},
		{"trailing slash", "/chat/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				spec.Context = tc.context
			}))
			assert.True(t, fieldsWithErrors(errs)["spec.context"], "got %+v", errs)
		})
	}
}

// The kind is registered against the generic deployment extension points, so the
// parser and validator must actually be reachable by kind string.
func TestWebSocketKindIsRegistered(t *testing.T) {
	assert.Equal(t, "WebSocketApi", webSocketApiKind)
}

// The kind reaches the generic deployment path only through these two
// registrations. Every other WebSocket test can pass while a missing init()
// here makes a real deploy fail with "unsupported resource kind", so assert the
// wiring itself rather than only the functions it points at.
func TestWebSocketKindRegisteredWithDeploymentPath(t *testing.T) {
	parser, ok := kindDeployParsers[webSocketApiKind]
	require.True(t, ok, "no deploy parser registered for %s", webSocketApiKind)
	require.NotNil(t, parser)

	validator, ok := kindConfigValidators[webSocketApiKind]
	require.True(t, ok, "no config validator registered for %s", webSocketApiKind)
	require.NotNil(t, validator)
}

// The registered parser must accept the YAML an operator actually submits and
// pull the handle out of metadata.name.
func TestWebSocketRegisteredParserAcceptsSubmittedYAML(t *testing.T) {
	const artifact = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: WebSocketApi
metadata:
  name: chat-websocket-v1.0
spec:
  displayName: Chat
  version: v1.0
  context: /chat/$version
  upstream:
    main:
      url: ws://mock-websocket-backend:8080/ws
  channels:
    - path: /rooms
`
	parsed, handle, kind, _, err := kindDeployParsers[webSocketApiKind](config.NewParser(), []byte(artifact), "application/yaml")
	require.NoError(t, err)
	assert.Equal(t, "chat-websocket-v1.0", handle)
	assert.Equal(t, "WebSocketApi", kind)

	name, version, errs := kindConfigValidators[webSocketApiKind](parsed)
	assert.Empty(t, errs, "the artifact the parser produced must satisfy the validator")
	assert.Equal(t, "Chat", name)
	assert.Equal(t, "v1.0", version)
}

func TestWebSocketAPIDesiredState(t *testing.T) {
	undeployed := api.WebSocketAPIConfigDataDeploymentStateUndeployed
	deployed := api.WebSocketAPIConfigDataDeploymentStateDeployed

	assert.Equal(t, models.StateDeployed, webSocketAPIDesiredState(validWebSocketAPI(nil)),
		"an omitted deploymentState defaults to deployed")
	assert.Equal(t, models.StateDeployed, webSocketAPIDesiredState(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.DeploymentState = &deployed
	})))
	assert.Equal(t, models.StateUndeployed, webSocketAPIDesiredState(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.DeploymentState = &undeployed
	})))
	assert.Equal(t, models.StateDeployed, webSocketAPIDesiredState("not a WebSocketApi"))
}

// withWebSocketValidation installs gateway-level validation inputs for one test.
func withWebSocketValidation(t *testing.T, limits config.WebSocketConfig, pv *config.PolicyValidator) {
	t.Helper()
	saved := webSocketValidation
	ConfigureWebSocketAPIValidation(limits, config.VHostsConfig{}, pv, nil)
	t.Cleanup(func() { webSocketValidation = saved })
}

func TestValidateWebSocketAPIConfig_RejectsConnectionSettingsAboveGatewayMaximum(t *testing.T) {
	withWebSocketValidation(t, config.WebSocketConfig{Upstream: config.WebSocketConnectionLimits{
		MaxConnectionsMax: 100000,
	}}, nil)

	withConnection := func(maxConns int) api.WebSocketAPI {
		return validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
			def := wsUpstreamDefinition("chat", "ws://chat:8080")
			def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &maxConns}
			withWSDefinitions(s, def)
		})
	}

	_, _, errs := validateWebSocketAPIConfig(withConnection(100001))
	assert.True(t, fieldsWithErrors(errs)["spec.upstreamDefinitions[0].connection.maxConnections"],
		"expected maxConnections above the gateway maximum to be rejected")

	_, _, errs = validateWebSocketAPIConfig(withConnection(100000))
	assert.Empty(t, errs, "a value equal to the gateway maximum is allowed")
}

func TestValidateWebSocketAPIConfig_NoGatewayMaximumConfigured(t *testing.T) {
	withWebSocketValidation(t, config.WebSocketConfig{}, nil)
	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		def := wsUpstreamDefinition("chat", "ws://chat:8080")
		many := 1000000
		def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &many}
		withWSDefinitions(s, def)
	})
	_, _, errs := validateWebSocketAPIConfig(cfg)
	assert.Empty(t, errs, "a zero maximum means the gateway sets no ceiling")
}

// resilience is validated with the RestApi rules at both levels, and the error
// names the field.
func TestValidateWebSocketAPIConfig_Resilience(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Resilience = &api.Resilience{Timeout: wsStrPtr("1h30m")}
		s.Channels[0].Resilience = &api.Resilience{IdleTimeout: wsStrPtr("soon")}
	}))
	fields := fieldsWithErrors(errs)
	assert.True(t, fields["spec.resilience.timeout"], "got %+v", errs)
	assert.True(t, fields["spec.channels[0].resilience.idleTimeout"], "got %+v", errs)

	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Resilience = &api.Resilience{Timeout: wsStrPtr("10s"), IdleTimeout: wsStrPtr("0s")}
		s.Channels[0].Resilience = &api.Resilience{IdleTimeout: wsStrPtr("30m")}
	}))
	assert.Empty(t, errs, "valid values, including 0s, are accepted")
}

func TestValidateWebSocketAPIConfig_MaxPendingHandshakes(t *testing.T) {
	withPending := func(n int) api.WebSocketAPI {
		return validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
			def := wsUpstreamDefinition("chat", "ws://chat:8080")
			def.Connection = &api.WebSocketUpstreamConnection{MaxPendingHandshakes: &n}
			withWSDefinitions(s, def)
		})
	}
	for _, bad := range []int{0, -1, int(math.MaxUint32) + 1} {
		_, _, errs := validateWebSocketAPIConfig(withPending(bad))
		fields := fieldsWithErrors(errs)
		assert.True(t, fields["spec.upstreamDefinitions[0].connection.maxPendingHandshakes"], "value %d: got %+v", bad, errs)
	}
	_, _, errs := validateWebSocketAPIConfig(withPending(0))
	require.NotEmpty(t, errs)
	assert.Equal(t, "maxPendingHandshakes must be at least 1", errs[0].Message, "the message names the field that failed")

	_, _, errs = validateWebSocketAPIConfig(withPending(1))
	assert.Empty(t, errs)
}

func TestValidateWebSocketAPIConfig_RejectsMaxConnectionsAboveUint32(t *testing.T) {
	withWebSocketValidation(t, config.WebSocketConfig{}, nil) // no gateway maximum
	tooMany := int(math.MaxUint32) + 2
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		def := wsUpstreamDefinition("chat", "ws://chat:8080")
		def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &tooMany}
		withWSDefinitions(s, def)
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstreamDefinitions[0].connection.maxConnections"], "got %+v", errs)

	largest := int(math.MaxUint32)
	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		def := wsUpstreamDefinition("chat", "ws://chat:8080")
		def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &largest}
		withWSDefinitions(s, def)
	}))
	assert.Empty(t, errs, "the uint32 maximum itself is allowed")
}

func TestValidateWebSocketAPIConfig_VersionFormat(t *testing.T) {
	// Same rule as RestApi and GraphQLApi: optional "v", then major[.minor[.patch]].
	for version, valid := range map[string]bool{
		"v1": true, "v1.0": true, "v12.3": true, "v1.0.0": true, "1.0": true,
		"": false, "version1": false, "v1.0.0.0": false, "v1.x": false,
	} {
		cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) { s.Version = version })
		_, _, errs := validateWebSocketAPIConfig(cfg)
		assert.Equal(t, !valid, fieldsWithErrors(errs)["spec.version"], "version %q", version)
	}
}

func TestValidateWebSocketAPIConfig_RejectsUnknownPolicies(t *testing.T) {
	withWebSocketValidation(t, config.WebSocketConfig{}, config.NewPolicyValidator(map[string]models.PolicyDefinition{}, nil))
	unknown := []api.Policy{{Name: "no-such-policy", Version: "v1"}}
	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Policies = &unknown
		s.Channels[0].Policies = &unknown
	})
	_, _, errs := validateWebSocketAPIConfig(cfg)
	hasErrorUnder := func(prefix string) bool {
		for _, e := range errs {
			if strings.HasPrefix(e.Field, prefix) {
				return true
			}
		}
		return false
	}
	assert.True(t, hasErrorUnder("spec.policies[0]"), "API-level unknown policy rejected: %v", errs)
	assert.True(t, hasErrorUnder("spec.channels[0].policies[0]"), "channel-level unknown policy rejected: %v", errs)
}

func TestWebSocketAPIRouteContext(t *testing.T) {
	defaults := config.VHostsConfig{Main: config.VHostEntry{Default: "api.example.com"}, Sandbox: config.VHostEntry{Default: "sandbox.example.com"}}

	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) { s.Context = "/chat/$version" })
	assert.Equal(t, []string{"api.example.com|/chat/v1.0"}, webSocketAPIRouteContext(cfg, defaults), "no vhosts: the gateway default")

	sandbox := "sandbox-ws.example.com"
	cfg = validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Context = "/chat/$version"
		s.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: "ws.example.com; ws2.example.com", Sandbox: &sandbox}
		s.Upstream.Sandbox = &api.Upstream{Url: wsStrPtr("ws://chat-sandbox:8080")}
	})
	assert.Equal(t, []string{"ws.example.com|/chat/v1.0", "ws2.example.com|/chat/v1.0", "sandbox-ws.example.com|/chat/v1.0"},
		webSocketAPIRouteContext(cfg, defaults), "every main vhost, plus the sandbox vhost when a sandbox upstream is set")

	assert.Nil(t, webSocketAPIRouteContext("not a WebSocketApi", defaults))
}

func TestValidateWebSocketAPIConfig_Vhosts(t *testing.T) {
	withVhosts := func(main string, sandbox *string, withSandboxUpstream bool) api.WebSocketAPI {
		return validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
			s.Vhosts = &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{Main: main, Sandbox: sandbox}
			if withSandboxUpstream {
				s.Upstream.Sandbox = &api.Upstream{Url: wsStrPtr("ws://chat-sandbox:8080")}
			}
		})
	}
	sb := func(v string) *string { return &v }

	for name, tc := range map[string]struct {
		cfg   api.WebSocketAPI
		field string
	}{
		"malformed main":              {withVhosts("ws example.com", nil, false), "spec.vhosts.main"},
		"empty main":                  {withVhosts("", nil, false), "spec.vhosts.main"},
		"duplicate main":              {withVhosts("ws.example.com;WS.example.com", nil, false), "spec.vhosts.main"},
		"wildcard sandbox":            {withVhosts("ws.example.com", sb("*.example.com"), true), "spec.vhosts.sandbox"},
		"sandbox equal to main vhost": {withVhosts("ws.example.com", sb("ws.example.com"), true), "spec.vhosts.sandbox"},
	} {
		_, _, errs := validateWebSocketAPIConfig(tc.cfg)
		assert.True(t, fieldsWithErrors(errs)[tc.field], "%s: got %+v", name, errs)
	}

	for name, cfg := range map[string]api.WebSocketAPI{
		"single":                        withVhosts("ws.example.com", nil, false),
		"several and a wildcard":        withVhosts("ws.example.com; *.chat.example.com", sb("sandbox-ws.example.com"), true),
		"gateway-default sentinel":      withVhosts("_gateway_default_", sb("_gateway_default_"), true),
		"sandbox vhost without sandbox": withVhosts("ws.example.com", sb("ws.example.com"), false),
	} {
		_, _, errs := validateWebSocketAPIConfig(cfg)
		assert.Empty(t, errs, name)
	}
}

// Deployment fills omitted vhosts with the gateway defaults before validating,
// and a default such as "*" or "sandbox-*" is an Envoy wildcard an author could
// not write; it must still be accepted, or no API without vhosts could deploy.
func TestValidateWebSocketAPIConfig_AcceptsGatewayDefaultVhosts(t *testing.T) {
	saved := webSocketValidation
	t.Cleanup(func() { webSocketValidation = saved })
	webSocketValidation.vhosts = config.VHostsConfig{
		Main: config.VHostEntry{Default: "*"}, Sandbox: config.VHostEntry{Default: "sandbox-*"},
	}

	routerCfg := &config.RouterConfig{VHosts: webSocketValidation.vhosts}
	frozen, err := resolveWebSocketVhostSentinels(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Upstream.Sandbox = &api.Upstream{Url: wsStrPtr("ws://chat-sandbox:8080")}
	}), routerCfg)
	require.NoError(t, err)
	_, _, errs := validateWebSocketAPIConfig(frozen)
	assert.Empty(t, errs, "frozen gateway defaults must validate")

	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: "*;ws.example.com"}
	}))
	assert.Empty(t, errs, "a default alongside a custom hostname is accepted")

	_, _, errs = validateWebSocketAPIConfig(validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: "ws-*"}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.vhosts.main"], "a wildcard form that is not the gateway default is still rejected")
}

func TestResolveWebSocketVhostSentinels(t *testing.T) {
	routerCfg := &config.RouterConfig{VHosts: config.VHostsConfig{
		Main: config.VHostEntry{Default: "api.example.com"}, Sandbox: config.VHostEntry{Default: "sandbox.example.com"},
	}}

	omitted, err := resolveWebSocketVhostSentinels(validWebSocketAPI(nil), routerCfg)
	require.NoError(t, err)
	vh := omitted.(api.WebSocketAPI).Spec.Vhosts
	require.NotNil(t, vh, "omitted vhosts are frozen to the current defaults")
	assert.Equal(t, "api.example.com", vh.Main)
	assert.Equal(t, "sandbox.example.com", *vh.Sandbox)

	sentinel := "_gateway_default_"
	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: sentinel, Sandbox: &sentinel}
	})
	resolved, err := resolveWebSocketVhostSentinels(cfg, routerCfg)
	require.NoError(t, err)
	vh = resolved.(api.WebSocketAPI).Spec.Vhosts
	assert.Equal(t, "api.example.com", vh.Main)
	assert.Equal(t, "sandbox.example.com", *vh.Sandbox)

	custom := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: "ws.example.com"}
	})
	unchanged, err := resolveWebSocketVhostSentinels(custom, routerCfg)
	require.NoError(t, err)
	assert.Equal(t, "ws.example.com", unchanged.(api.WebSocketAPI).Spec.Vhosts.Main)
	assert.Nil(t, unchanged.(api.WebSocketAPI).Spec.Vhosts.Sandbox, "a custom main without a sandbox is left as written")
}

// A tls block on a WebSocketApi upstream would otherwise be silently ignored, so a
// custom CA or client certificate placed there would never take effect.
func TestValidateWebSocketAPIConfig_RejectsUpstreamTLS(t *testing.T) {
	tlsBlock := map[string]interface{}{"verifyHostName": false}
	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Upstream.Main.Tls = &tlsBlock
		s.Upstream.Sandbox = &api.Upstream{Url: wsStrPtr("wss://sandbox:8443/ws"), Tls: &tlsBlock}
	})
	_, _, errs := validateWebSocketAPIConfig(cfg)
	fields := fieldsWithErrors(errs)
	assert.True(t, fields["spec.upstream.main.tls"], "main tls must be rejected: %v", errs)
	assert.True(t, fields["spec.upstream.sandbox.tls"], "sandbox tls must be rejected: %v", errs)
}

// displayName follows the same URL-friendly rule as RestApi and GraphQLApi.
func TestValidateWebSocketAPIConfig_DisplayNameRule(t *testing.T) {
	for name, valid := range map[string]bool{
		"Chat": true, "Chat API v1.0": true, "chat_api-1": true,
		"": false, "Chat<script>": false, "chat/rooms": false, strings.Repeat("a", 101): false,
	} {
		cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) { s.DisplayName = name })
		_, _, errs := validateWebSocketAPIConfig(cfg)
		assert.Equal(t, !valid, fieldsWithErrors(errs)["spec.displayName"], "displayName %q", name)
	}
}
