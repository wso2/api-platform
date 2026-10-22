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
	"strings"
	"testing"
	"time"

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
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				spec.Channels = []api.WebSocketChannel{{Path: tc.path}}
			}))
			assert.True(t, fieldsWithErrors(errs)[tc.field], "got %+v", errs)
		})
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

func TestValidateWebSocketAPIConfig_RejectsUpstreamRef(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Ref: wsStrPtr("shared-upstream")}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.main.ref"], "got %+v", errs)
}

func TestValidateWebSocketAPIConfig_ValidatesSandboxToo(t *testing.T) {
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Sandbox = &api.Upstream{Url: wsStrPtr("http://sandbox:8080")}
	}))
	assert.True(t, fieldsWithErrors(errs)["spec.upstream.sandbox.url"], "got %+v", errs)
}

// A zero or negative duration is rejected rather than read as "unlimited": for a
// WebSocketApi that is exactly the unbounded connection these settings exist to
// prevent.
func TestValidateWebSocketAPIConfig_RejectsNonPositiveDurations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		field string
	}{
		{"zero", "0s", "spec.connection.idleTimeout"},
		{"negative", "-5s", "spec.connection.idleTimeout"},
		{"unparseable", "soon", "spec.connection.idleTimeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				spec.Connection = &api.WebSocketConnectionSettings{IdleTimeout: wsStrPtr(tc.value)}
			}))
			assert.True(t, fieldsWithErrors(errs)[tc.field], "got %+v", errs)
		})
	}
}

func TestValidateWebSocketAPIConfig_ValidatesUpstreamTimeoutBlock(t *testing.T) {
	zero := 0
	_, _, errs := validateWebSocketAPIConfig(validWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{
			Url: wsStrPtr("wss://chat-backend:8443/ws"),
			Timeout: &api.UpstreamTimeout{
				IdleTimeout:    wsStrPtr("-1s"),
				MaxConnections: &zero,
			},
		}
	}))
	fields := fieldsWithErrors(errs)
	assert.True(t, fields["spec.upstream.main.timeout.idleTimeout"], "got %+v", errs)
	assert.True(t, fields["spec.upstream.main.timeout.maxConnections"], "got %+v", errs)
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
	ConfigureWebSocketAPIValidation(limits, pv)
	t.Cleanup(func() { webSocketValidation = saved })
}

func TestValidateWebSocketAPIConfig_RejectsConnectionSettingsAboveGatewayMaximum(t *testing.T) {
	bounds := config.WebSocketConnectionLimits{
		IdleTimeoutMax: time.Hour, MaxConnectionDurationMax: 24 * time.Hour, MaxConnectionsMax: 100000,
	}
	withWebSocketValidation(t, config.WebSocketConfig{Downstream: bounds, Upstream: bounds}, nil)

	over := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		maxConns := 100001
		s.Connection = &api.WebSocketConnectionSettings{
			IdleTimeout: wsStrPtr("2h"), MaxConnectionDuration: wsStrPtr("25h"), MaxConnections: &maxConns,
		}
		upConns := 100001
		s.Upstream.Main.Timeout = &api.UpstreamTimeout{IdleTimeout: wsStrPtr("90m"), MaxConnections: &upConns}
	})
	_, _, errs := validateWebSocketAPIConfig(over)
	fields := fieldsWithErrors(errs)
	for _, f := range []string{
		"spec.connection.idleTimeout", "spec.connection.maxConnectionDuration", "spec.connection.maxConnections",
		"spec.upstream.main.timeout.idleTimeout", "spec.upstream.main.timeout.maxConnections",
	} {
		assert.True(t, fields[f], "expected %s above the gateway maximum to be rejected", f)
	}

	atLimit := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		maxConns := 100000
		s.Connection = &api.WebSocketConnectionSettings{
			IdleTimeout: wsStrPtr("1h"), MaxConnectionDuration: wsStrPtr("24h"), MaxConnections: &maxConns,
		}
	})
	_, _, errs = validateWebSocketAPIConfig(atLimit)
	assert.Empty(t, errs, "values equal to the gateway maximum are allowed")
}

func TestValidateWebSocketAPIConfig_NoGatewayMaximumConfigured(t *testing.T) {
	withWebSocketValidation(t, config.WebSocketConfig{}, nil)
	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) {
		s.Connection = &api.WebSocketConnectionSettings{IdleTimeout: wsStrPtr("48h")}
	})
	_, _, errs := validateWebSocketAPIConfig(cfg)
	assert.Empty(t, errs, "a zero maximum means the gateway sets no ceiling")
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
	cfg := validWebSocketAPI(func(s *api.WebSocketAPIConfigData) { s.Context = "/chat/$version" })
	assert.Equal(t, "/chat/v1.0", webSocketAPIRouteContext(cfg))
	assert.Equal(t, "", webSocketAPIRouteContext("not a WebSocketApi"))
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
