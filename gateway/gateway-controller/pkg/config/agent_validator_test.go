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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/agentproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// validAgent returns a configuration the validator accepts, for tests to spoil
// one field at a time.
func validAgent() api.AgentConfiguration {
	url := "https://weather.internal"
	return api.AgentConfiguration{
		ApiVersion: api.AgentConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.AgentConfigurationKindAgent,
		Metadata:   api.Metadata{Name: "weather-agent-v1-0"},
		Spec: api.AgentConfigData{
			DisplayName: "Weather Agent",
			Version:     "v1.0",
			Context:     stringPtr("/weather"),
			Upstream:    api.AgentConfigData_Upstream{Url: &url},
			A2a: api.A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: api.A2AOperationConfigs{
					Transports: []api.A2ATransport{
						{ProtocolBinding: api.JSONRPC, PathPrefix: stringPtr("/rpc")},
					},
				},
				AgentCard: api.A2AAgentCard{
					Public: api.A2APublicAgentCard{
						Mode:    api.A2APublicAgentCardModeManaged,
						Content: cardContent(),
					},
				},
			},
		},
	}
}

// cardContent is a minimal managed-card document carrying the interfaces
// validAgent's own transports resolve to.
//
// Only the fields validation reads are modelled; the rest of the Agent Card is
// not checked here, so filling it in would be noise. The card *is* checked for
// agreement with the configured transports, which is why the interface list
// cannot be a placeholder — see syncCardInterfaces for tests that move the
// transports out from under it.
func cardContent() *api.A2AAgentCardDocument {
	doc := api.A2AAgentCardDocument{
		"name":    "Weather Agent",
		"version": "1.0.0",
		"supportedInterfaces": []interface{}{
			map[string]interface{}{
				"protocolBinding": string(api.JSONRPC),
				"protocolVersion": "1.0",
				"url":             "https://agents.example.com/weather/rpc",
			},
		},
	}
	return &doc
}

// cardHost is the host every fixture card advertises. Its value is arbitrary:
// the gateway has no configured public base URL to compare a card's host
// against, so host agreement is not checked. Only the path is.
const cardHost = "https://agents.example.com"

// syncCardInterfaces rewrites the managed card's supportedInterfaces to match
// whatever transports cfg now declares.
//
// Most of this file's tests change transports, contexts, and card paths to
// exercise route arithmetic, and none of them are about the card. Without this
// every one of them would additionally have to restate the card, and the
// resulting failure would be the card-consistency error rather than the routing
// error the test is actually about. Tests that *are* about the card build their
// interfaces explicitly instead — see agent_card_validator_test.go.
//
// Bindings are deduplicated so a deliberately duplicated transport does not
// also produce a duplicate-interface error, which is a different rejection with
// its own test.
func syncCardInterfaces(cfg *api.AgentConfiguration) {
	public := &cfg.Spec.A2a.AgentCard.Public
	// An absent or deliberately emptied card is left alone: those are the
	// mode-rule cases, and filling one in here would repair the very thing the
	// test set out to break.
	if public.Mode != api.A2APublicAgentCardModeManaged || public.Content == nil || len(*public.Content) == 0 {
		return
	}

	context := AgentContextPath(cfg.Spec.Context)
	interfaces := make([]interface{}, 0, len(cfg.Spec.A2a.OperationConfigs.Transports))
	seen := make(map[api.A2AProtocolBinding]bool)
	for _, transport := range cfg.Spec.A2a.OperationConfigs.Transports {
		if seen[transport.ProtocolBinding] {
			continue
		}
		seen[transport.ProtocolBinding] = true

		prefix := "/"
		if transport.PathPrefix != nil {
			prefix = *transport.PathPrefix
		}
		interfaces = append(interfaces, map[string]interface{}{
			"protocolBinding": string(transport.ProtocolBinding),
			"protocolVersion": string(cfg.Spec.A2a.ProtocolVersion),
			"url":             cardHost + JoinAgentPath(context, prefix),
		})
	}

	(*public.Content)["supportedInterfaces"] = interfaces
}

// validateAgent validates cfg with the card's interfaces first re-derived from
// its transports, so a test that is not about the card does not have to keep
// one in step. See syncCardInterfaces.
func validateAgent(cfg *api.AgentConfiguration) []ValidationError {
	syncCardInterfaces(cfg)
	return NewAgentValidator().Validate(cfg)
}

// transports builds a transports list from binding/prefix pairs. A nil prefix
// means the field is omitted, which is not the same as "/" as far as the config
// is concerned even though both resolve to the context.
func transports(pairs ...any) []api.A2ATransport {
	if len(pairs)%2 != 0 {
		panic("transports takes binding/prefix pairs")
	}
	out := make([]api.A2ATransport, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		transport := api.A2ATransport{ProtocolBinding: pairs[i].(api.A2AProtocolBinding)}
		if prefix, ok := pairs[i+1].(string); ok {
			transport.PathPrefix = stringPtr(prefix)
		}
		out = append(out, transport)
	}
	return out
}

func fieldsOf(errs []ValidationError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Field)
	}
	return out
}

func TestAgentValidator_AcceptsValidConfiguration(t *testing.T) {
	cfg := validAgent()

	assert.Empty(t, validateAgent(&cfg))
	// Value and pointer forms must agree; handlers and services pass both.
	assert.Empty(t, NewAgentValidator().Validate(cfg))
}

func TestAgentValidator_RejectsOtherTypes(t *testing.T) {
	v := NewAgentValidator()

	// A validator that shrugged at an unexpected type would silently approve
	// whatever it could not inspect.
	for name, input := range map[string]any{
		"nil pointer":  (*api.AgentConfiguration)(nil),
		"other kind":   api.MCPProxyConfiguration{},
		"plain string": "not a config",
		"nil":          nil,
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, v.Validate(input))
		})
	}
}

func TestAgentValidator_FieldErrors(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*api.AgentConfiguration)
		field string
	}{
		{
			name:  "wrong kind",
			spoil: func(c *api.AgentConfiguration) { c.Kind = "Mcp" },
			field: "kind",
		},
		{
			name:  "missing handle",
			spoil: func(c *api.AgentConfiguration) { c.Metadata.Name = "" },
			field: "metadata.name",
		},
		{
			name:  "missing display name",
			spoil: func(c *api.AgentConfiguration) { c.Spec.DisplayName = "" },
			field: "spec.displayName",
		},
		{
			name:  "display name with an unsafe character",
			spoil: func(c *api.AgentConfiguration) { c.Spec.DisplayName = "weather/agent" },
			field: "spec.displayName",
		},
		{
			name:  "missing version",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Version = "" },
			field: "spec.version",
		},
		{
			name:  "malformed version",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Version = "1.0.0" },
			field: "spec.version",
		},
		{
			// Distinct from omitting the field, which is legal — see
			// TestAgentValidator_OptionalContext.
			name:  "explicitly empty context",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Context = stringPtr("") },
			field: "spec.context",
		},
		{
			name:  "context without a leading slash",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Context = stringPtr("weather") },
			field: "spec.context",
		},
		{
			name:  "context with a trailing slash",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Context = stringPtr("/weather/") },
			field: "spec.context",
		},
		{
			name:  "context inside the reserved health namespace",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Context = stringPtr("/_gateway-health") },
			field: "spec.context",
		},
		{
			name: "context that only resolves into the reserved namespace after normalization",
			spoil: func(c *api.AgentConfiguration) {
				c.Spec.Context = stringPtr("/weather/../_gateway-health/ready")
			},
			field: "spec.context",
		},
		{
			name:  "missing upstream url",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Upstream.Url = nil },
			field: "spec.upstream.url",
		},
		{
			name: "empty upstream url",
			spoil: func(c *api.AgentConfiguration) {
				empty := ""
				c.Spec.Upstream.Url = &empty
			},
			field: "spec.upstream.url",
		},
		{
			name: "malformed agent-level timeout",
			spoil: func(c *api.AgentConfiguration) {
				c.Spec.Resilience = &api.Resilience{Timeout: stringPtr("1h30m")}
			},
			field: "spec.resilience.timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			tt.spoil(&cfg)

			errs := validateAgent(&cfg)
			require.NotEmpty(t, errs, "expected a validation error for %s", tt.field)
			assert.Contains(t, fieldsOf(errs), tt.field)
		})
	}
}

// TestAgentValidator_OptionalContext covers the Agent served at the root of its
// virtual host. A2A discovery is defined relative to the origin root, so an
// Agent with no context puts its card at exactly the path a client probes
// during cold discovery; nesting under a context is a choice, not a
// requirement. Neither context nor vhost is required — an Agent may declare
// both, either, or neither.
func TestAgentValidator_OptionalContext(t *testing.T) {
	t.Run("omitted entirely", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.Context = nil

		assert.Empty(t, validateAgent(&cfg))
	})

	t.Run("omitted with a vhost", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.Context = nil
		cfg.Spec.Vhost = stringPtr("weather.example.com")

		assert.Empty(t, validateAgent(&cfg))
	})

	t.Run("route arithmetic still applies at the root", func(t *testing.T) {
		// The card lands on ListTasks' route, which has to be caught whether or
		// not a context is in front of it.
		cfg := validAgent()
		cfg.Spec.Context = nil
		cfg.Spec.A2a.OperationConfigs.Transports = transports(api.HTTPJSON, "/")
		cfg.Spec.A2a.AgentCard.Public.Path = stringPtr("/tasks")

		errs := validateAgent(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.path")
		assert.Contains(t, errs[0].Message, "/tasks")
	})

	t.Run("reserved namespace is reachable without a context", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.Context = nil
		cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/_gateway-health")

		errs := validateAgent(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.operationConfigs.transports[0].pathPrefix")
	})
}

// TestAgentValidator_ProtocolVersion covers the version gate. An unsupported
// version must be rejected rather than defaulted: falling back would enforce an
// operation set the Agent's own card does not advertise.
func TestAgentValidator_ProtocolVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  api.A2AConfigProtocolVersion
		accepted bool
	}{
		{name: "registered version", version: "1.0", accepted: true},
		{name: "missing", version: ""},
		{name: "unregistered minor", version: "1.1"},
		{name: "unregistered major", version: "2.0"},
		{name: "v-prefixed", version: "v1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.ProtocolVersion = tt.version

			errs := validateAgent(&cfg)
			if tt.accepted {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), "spec.a2a.protocolVersion")
		})
	}
}

// TestAgentValidator_ProtocolVersionErrorNamesTheRegistry keeps the rejection
// actionable: the supported set comes from agentproto, so a newly registered
// version shows up in the message without anyone editing it here.
func TestAgentValidator_ProtocolVersionErrorNamesTheRegistry(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.A2a.ProtocolVersion = "9.9"

	errs := validateAgent(&cfg)
	require.Len(t, errs, 1)
	for _, version := range agentproto.Versions() {
		assert.Contains(t, errs[0].Message, string(version))
	}
}

func TestAgentValidator_Transports(t *testing.T) {
	tests := []struct {
		name       string
		transports []api.A2ATransport
		field      string // empty means the configuration must be accepted
	}{
		{
			name:       "JSONRPC only",
			transports: transports(api.JSONRPC, "/rpc"),
		},
		{
			name:       "HTTP+JSON only",
			transports: transports(api.HTTPJSON, "/http"),
		},
		{
			name:       "both bindings at distinct prefixes",
			transports: transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/http"),
		},
		{
			name:       "omitted prefix defaults to the context",
			transports: transports(api.JSONRPC, nil),
		},
		{
			name:       "root prefix",
			transports: transports(api.JSONRPC, "/"),
		},
		{
			name:       "no transports",
			transports: nil,
			field:      "spec.a2a.operationConfigs.transports",
		},
		{
			name:       "three transports",
			transports: transports(api.JSONRPC, "/a", api.HTTPJSON, "/b", api.JSONRPC, "/c"),
			field:      "spec.a2a.operationConfigs.transports",
		},
		{
			name:       "unknown protocol binding",
			transports: transports(api.A2AProtocolBinding("GRPC"), "/grpc"),
			field:      "spec.a2a.operationConfigs.transports[0].protocolBinding",
		},
		{
			name:       "duplicate protocol binding",
			transports: transports(api.JSONRPC, "/rpc", api.JSONRPC, "/rpc2"),
			field:      "spec.a2a.operationConfigs.transports[1].protocolBinding",
		},
		{
			// Legal, and idiomatic: the JSONRPC endpoint is the base path
			// itself, every HTTP+JSON route lands below it, and nothing
			// overlaps. See TestAgentValidator_TransportsMaySharaABasePath.
			name:       "both transports share a prefix",
			transports: transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/rpc"),
		},
		{
			name:       "distinct prefixes that resolve to the same base path",
			transports: transports(api.JSONRPC, "/", api.HTTPJSON, "/"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.OperationConfigs.Transports = tt.transports

			errs := validateAgent(&cfg)
			if tt.field == "" {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field)
		})
	}
}

// TestAgentValidator_TransportsMaySharaABasePath pins the layout an earlier,
// stricter rule wrongly rejected. Sharing a base path is not ambiguous: the
// JSONRPC endpoint *is* the base path and takes its operation from the request
// body, while every HTTP+JSON binding template is non-empty and therefore lands
// strictly below it. The route set proves it — no two routes share a key.
func TestAgentValidator_TransportsMaySharaABasePath(t *testing.T) {
	for _, prefix := range []string{"/", "/rpc", "/a2a/v1"} {
		t.Run(prefix, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, prefix, api.HTTPJSON, prefix)

			require.Empty(t, validateAgent(&cfg))

			base := JoinAgentPath("/weather", prefix)
			routes := buildAgentRoutes(agentproto.V1_0,
				[]resolvedTransport{
					{index: 0, binding: api.JSONRPC, basePath: base, usable: true},
					{index: 1, binding: api.HTTPJSON, basePath: base, usable: true},
				},
				resolvedCard{path: JoinAgentPath("/weather", DefaultAgentCardPath), usable: true})

			keys := make(map[string]string, len(routes))
			for _, route := range routes {
				if previous, dup := keys[route.routeKey()]; dup {
					t.Fatalf("%s collides: %s and %s", route.routeKey(), previous, route.source)
				}
				keys[route.routeKey()] = route.source
			}
			// The JSONRPC endpoint sits at the base path; the eleven HTTP+JSON
			// routes and the card sit below it.
			assert.Len(t, keys, 13)
			assert.Contains(t, keys, "POST "+base)
		})
	}
}

// TestAgentValidator_TransportPrefixReachingTheReservedNamespace needs a root
// context: the reservation is on the vhost's own /_gateway-health routes, so a
// prefix of that name under any other context resolves somewhere else entirely
// and is nobody's business to reject.
func TestAgentValidator_TransportPrefixReachingTheReservedNamespace(t *testing.T) {
	t.Run("root context", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.Context = stringPtr("/")
		cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/_gateway-health")

		errs := validateAgent(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.operationConfigs.transports[0].pathPrefix")
	})

	t.Run("nested under another context", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/_gateway-health")

		assert.Empty(t, validateAgent(&cfg))
	})

	t.Run("card path at the root context", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.Context = stringPtr("/")
		cfg.Spec.A2a.AgentCard.Public.Path = stringPtr("/_gateway-health/ready")

		errs := validateAgent(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.path")
	})
}

// TestAgentValidator_PathValues drives the same rules through both fields that
// accept an author-written path, because they resolve through one helper and a
// divergence between them would be a silent routing difference.
func TestAgentValidator_PathValues(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		accepted bool
	}{
		{name: "root", value: "/", accepted: true},
		{name: "single segment", value: "/rpc", accepted: true},
		{name: "nested segments", value: "/a2a/v1", accepted: true},
		{name: "dotted segment", value: "/.well-known/agent-card.json", accepted: true},
		{name: "empty"},
		{name: "relative", value: "rpc"},
		{name: "trailing slash", value: "/rpc/"},
		{name: "query string", value: "/rpc?v=1"},
		{name: "fragment", value: "/rpc#top"},
		{name: "path parameter placeholder", value: "/rpc/{id}"},
		{name: "embedded whitespace", value: "/rpc endpoint"},
		{name: "dot-dot segment", value: "/a/../b"},
		{name: "repeated slash", value: "/a//b"},
	}

	fields := map[string]func(*api.AgentConfiguration, string){
		"spec.a2a.operationConfigs.transports[0].pathPrefix": func(c *api.AgentConfiguration, v string) {
			c.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, v)
		},
		"spec.a2a.agentCard.public.path": func(c *api.AgentConfiguration, v string) {
			c.Spec.A2a.AgentCard.Public.Path = stringPtr(v)
		},
	}

	for field, apply := range fields {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", field, tc.name), func(t *testing.T) {
				cfg := validAgent()
				apply(&cfg, tc.value)

				errs := validateAgent(&cfg)
				if tc.accepted {
					assert.Empty(t, errs)
					return
				}
				require.NotEmpty(t, errs)
				assert.Contains(t, fieldsOf(errs), field)
			})
		}
	}
}

// TestAgentValidator_RouteCollisions is the check with no runtime counterpart:
// a collision does not fail a request, it makes one of the two routes silently
// unreachable.
func TestAgentValidator_RouteCollisions(t *testing.T) {
	tests := []struct {
		name       string
		transports []api.A2ATransport
		cardPath   *string
		field      string // empty means the configuration must be accepted
	}{
		{
			name:       "default card path below an HTTP+JSON transport at the context",
			transports: transports(api.HTTPJSON, "/"),
		},
		{
			// The card is a GET and the JSONRPC endpoint a POST, so sharing a
			// path is not a collision — the methods separate them.
			name:       "card path equal to the JSONRPC endpoint",
			transports: transports(api.JSONRPC, "/"),
			cardPath:   stringPtr("/"),
		},
		{
			name:       "card path colliding with an HTTP+JSON operation route",
			transports: transports(api.HTTPJSON, "/"),
			cardPath:   stringPtr("/tasks"),
			field:      "spec.a2a.agentCard.public.path",
		},
		{
			// Not an equal key: "/weather/tasks/card.json" is not the string
			// "/weather/tasks/{id}", but GetTask's template matches it.
			name:       "card path shadowed by a templated operation route",
			transports: transports(api.HTTPJSON, "/"),
			cardPath:   stringPtr("/tasks/card.json"),
			field:      "spec.a2a.agentCard.public.path",
		},
		{
			// CancelTask is POST /tasks/{id}:cancel, so a GET card route at a
			// path of the same shape does not conflict with it.
			name:       "card path matching a template of another method",
			transports: transports(api.HTTPJSON, "/"),
			cardPath:   stringPtr("/tasks/card.json:cancel"),
			field:      "spec.a2a.agentCard.public.path",
		},
		{
			name:       "card path moved out from under the operation routes",
			transports: transports(api.HTTPJSON, "/"),
			cardPath:   stringPtr("/.well-known/agent-card.json"),
		},
		{
			name:       "card path under a transport prefix it does not share",
			transports: transports(api.HTTPJSON, "/http"),
			cardPath:   stringPtr("/tasks"),
		},
		{
			// The two transports have distinct base paths, so nothing about the
			// prefixes looks wrong; it is the generated routes that clash.
			name:       "JSONRPC endpoint landing on an HTTP+JSON operation route",
			transports: transports(api.JSONRPC, "/message:send", api.HTTPJSON, "/"),
			field:      "spec.a2a.operationConfigs.transports[1].pathPrefix",
		},
		{
			// Same clash reached with the prefix omitted rather than written as
			// "/". Those are different fields on the wire and different
			// branches in the validator, so both have to arrive at the same
			// base path for the collision to be caught either way.
			name:       "same clash with the HTTP+JSON prefix omitted entirely",
			transports: transports(api.JSONRPC, "/message:send", api.HTTPJSON, nil),
			field:      "spec.a2a.operationConfigs.transports[1].pathPrefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.OperationConfigs.Transports = tt.transports
			cfg.Spec.A2a.AgentCard.Public.Path = tt.cardPath

			errs := validateAgent(&cfg)
			if tt.field == "" {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field)
			for _, e := range errs {
				assert.Contains(t, e.Message, "Route collision",
					"the only failure here should be the collision itself")
			}
		})
	}
}

// TestAgentValidator_CardPathMatchingATemplateOfAnotherMethodIsRejected pins the
// one case above that deserves spelling out: a GET card route at the shape of a
// POST-only operation template is still rejected, because "/tasks/{id}" (GET
// GetTask) matches it too. The methods only save the JSONRPC endpoint, which is
// not templated.
func TestAgentValidator_CardPathMatchingATemplateOfAnotherMethodIsRejected(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.A2a.OperationConfigs.Transports = transports(api.HTTPJSON, "/")
	cfg.Spec.A2a.AgentCard.Public.Path = stringPtr("/tasks/card.json:cancel")

	errs := validateAgent(&cfg)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "GetTask")
}

// TestAgentValidator_NoCollisionsWithinTheProtocolBindings guards the assumption
// the collision check rests on: the protocol's own routes are internally
// consistent, so any collision reported for a valid Agent came from something
// the author wrote.
func TestAgentValidator_NoCollisionsWithinTheProtocolBindings(t *testing.T) {
	for _, version := range agentproto.Versions() {
		t.Run(string(version), func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.ProtocolVersion = api.A2AConfigProtocolVersion(version)
			cfg.Spec.A2a.OperationConfigs.Transports = transports(api.JSONRPC, "/rpc", api.HTTPJSON, "/")

			assert.Empty(t, validateAgent(&cfg))
		})
	}
}

// TestAgentRouteKey_IsMethodAndPathOnly is the executable form of the partition
// rule: route identity is method plus path, with no header discriminator. The
// policy-chain key downstream is (artifact, vhost, operation), so two routes
// separated only by a header would share one chain and one of them would
// silently run the other's policies. Anything added to routeKey has to be added
// there too.
func TestAgentRouteKey_IsMethodAndPathOnly(t *testing.T) {
	route := agentRoute{
		method: "POST",
		path:   "/weather/rpc",
		field:  "spec.a2a.operationConfigs.transports[0].pathPrefix",
		source: "the JSONRPC transport endpoint",
	}
	other := agentRoute{
		method: "POST",
		path:   "/weather/rpc",
		field:  "spec.a2a.agentCard.public.path",
		source: "the public Agent Card route",
	}

	assert.Equal(t, "POST /weather/rpc", route.routeKey())
	assert.Equal(t, route.routeKey(), other.routeKey(),
		"routes differing only in provenance must still collide")
}

func TestAgentValidator_CardModes(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*api.A2APublicAgentCard)
		field string // empty means the configuration must be accepted
	}{
		{
			name:  "managed with content",
			spoil: func(*api.A2APublicAgentCard) {},
		},
		{
			name:  "managed without content",
			spoil: func(c *api.A2APublicAgentCard) { c.Content = nil },
			field: "spec.a2a.agentCard.public.content",
		},
		{
			name: "managed with empty content",
			spoil: func(c *api.A2APublicAgentCard) {
				empty := api.A2AAgentCardDocument{}
				c.Content = &empty
			},
			field: "spec.a2a.agentCard.public.content",
		},
		{
			name: "passthrough without content",
			spoil: func(c *api.A2APublicAgentCard) {
				c.Mode = api.A2APublicAgentCardModePassthrough
				c.Content = nil
			},
		},
		{
			name: "passthrough with content",
			spoil: func(c *api.A2APublicAgentCard) {
				c.Mode = api.A2APublicAgentCardModePassthrough
			},
			field: "spec.a2a.agentCard.public.content",
		},
		{
			name: "passthrough with signing",
			spoil: func(c *api.A2APublicAgentCard) {
				c.Mode = api.A2APublicAgentCardModePassthrough
				c.Content = nil
				c.Signing = &api.A2ACardSigning{Enabled: false}
			},
			field: "spec.a2a.agentCard.public.signing",
		},
		{
			name:  "unknown mode",
			spoil: func(c *api.A2APublicAgentCard) { c.Mode = "proxied" },
			field: "spec.a2a.agentCard.public.mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			tt.spoil(&cfg.Spec.A2a.AgentCard.Public)

			errs := validateAgent(&cfg)
			if tt.field == "" {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field)
		})
	}
}

// TestAgentValidator_RejectsUnsupportedCardFeatures pins the two fail-closed
// rejections. Both features are described by the management API schema, so
// without these an Agent could be accepted with a card the gateway will not
// serve as asked — a mismatch only visible to a client reading the card.
func TestAgentValidator_RejectsUnsupportedCardFeatures(t *testing.T) {
	t.Run("signing enabled", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.AgentCard.Public.Signing = &api.A2ACardSigning{Enabled: true}

		errs := validateAgent(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.signing.enabled")
	})

	t.Run("signing explicitly disabled is accepted", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.AgentCard.Public.Signing = &api.A2ACardSigning{Enabled: false}

		assert.Empty(t, validateAgent(&cfg))
	})

	t.Run("protected card block", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
			Mode: api.A2AProtectedAgentCardModePassthrough,
		}

		errs := validateAgent(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.protected")
	})
}

// TestAgentValidator_SigningIsEnabledOnly pins the whole of the Agent-facing
// signing contract: one boolean. The algorithm, `kid`, and signing profile are
// administrator-owned and resolved from gateway TOML, which is what lets an
// operator rotate the active key — even to a different algorithm — without
// editing any Agent. Each Agent still has to be redeployed for its stored card
// to be re-signed with the new key; what rotation avoids is a config change, not
// a deploy. A per-Agent algorithm could only have restated that choice or
// contradicted it, so the field does not exist to be validated.
func TestAgentValidator_SigningIsEnabledOnly(t *testing.T) {
	t.Run("the generated type carries no operator-owned fields", func(t *testing.T) {
		// A compile-time assertion in test form: if `algorithm`, `kid`, or a
		// profile selector is ever added back to the schema, this stops
		// building and forces the decision to be revisited rather than
		// silently re-validated here.
		signing := api.A2ACardSigning{Enabled: false}
		assert.Equal(t, 1, reflect.TypeOf(signing).NumField(),
			"A2ACardSigning must expose only `enabled`")
	})

	tests := []struct {
		name    string
		signing *api.A2ACardSigning
		field   string // empty means the configuration must be accepted
	}{
		{name: "omitted"},
		{name: "explicitly disabled", signing: &api.A2ACardSigning{Enabled: false}},
		{
			name:    "enabled",
			signing: &api.A2ACardSigning{Enabled: true},
			field:   "spec.a2a.agentCard.public.signing.enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.AgentCard.Public.Signing = tt.signing

			errs := validateAgent(&cfg)
			if tt.field == "" {
				assert.Empty(t, errs)
				return
			}
			require.Len(t, errs, 1, "enabling signing is one problem, not several")
			assert.Equal(t, tt.field, errs[0].Field)
		})
	}
}

func TestAgentValidator_OperationConfigs(t *testing.T) {
	operations := func(configs ...api.A2AOperationConfig) *[]api.A2AOperationConfig { return &configs }

	tests := []struct {
		name       string
		operations *[]api.A2AOperationConfig
		field      string // empty means the configuration must be accepted
	}{
		{
			name:       "no operation overrides",
			operations: nil,
		},
		{
			name: "every canonical operation of the version",
			operations: func() *[]api.A2AOperationConfig {
				names, ok := agentproto.Operations(agentproto.V1_0)
				require.True(t, ok)
				configs := make([]api.A2AOperationConfig, 0, len(names))
				for _, name := range names {
					configs = append(configs, api.A2AOperationConfig{Name: api.A2AOperationName(name)})
				}
				return &configs
			}(),
		},
		{
			name:       "unknown operation name",
			operations: operations(api.A2AOperationConfig{Name: "SendMessages"}),
			field:      "spec.a2a.operationConfigs.operations[0].name",
		},
		{
			name:       "wrong case",
			operations: operations(api.A2AOperationConfig{Name: "sendMessage"}),
			field:      "spec.a2a.operationConfigs.operations[0].name",
		},
		{
			name:       "empty operation name",
			operations: operations(api.A2AOperationConfig{Name: ""}),
			field:      "spec.a2a.operationConfigs.operations[0].name",
		},
		{
			name: "duplicate operation",
			operations: operations(
				api.A2AOperationConfig{Name: api.SendMessage},
				api.A2AOperationConfig{Name: api.SendMessage},
			),
			field: "spec.a2a.operationConfigs.operations[1].name",
		},
		{
			name: "malformed operation timeout",
			operations: operations(api.A2AOperationConfig{
				Name:       api.SendMessage,
				Resilience: &api.Resilience{IdleTimeout: stringPtr("forever")},
			}),
			field: "spec.a2a.operationConfigs.operations[0].resilience.idleTimeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			cfg.Spec.A2a.OperationConfigs.Operations = tt.operations

			errs := validateAgent(&cfg)
			if tt.field == "" {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tt.field)
		})
	}
}

// TestAgentValidator_OperationNamesAreNotCheckedAgainstAnUnknownVersion keeps the
// version error from dragging eleven consequences behind it: with no registered
// operation set there is nothing to check a name against, and reporting each one
// as unknown would bury the single error worth fixing.
func TestAgentValidator_OperationNamesAreNotCheckedAgainstAnUnknownVersion(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.A2a.ProtocolVersion = "9.9"
	cfg.Spec.A2a.OperationConfigs.Operations = &[]api.A2AOperationConfig{{Name: api.SendMessage}}

	errs := validateAgent(&cfg)
	require.Len(t, errs, 1)
	assert.Equal(t, "spec.a2a.protocolVersion", errs[0].Field)
}

// TestAgentValidator_ValidatesEveryPolicyScope walks the three places an Agent
// can carry policies. They are separate chains — card serving is not an A2A
// operation and does not inherit the operation policies — so a validator that
// checked only one scope would let unknown policies through the others.
func TestAgentValidator_ValidatesEveryPolicyScope(t *testing.T) {
	validator := NewAgentValidator().WithPolicyValidator(NewPolicyValidator(agentPolicyDefinitions()))

	scopes := map[string]func(*api.AgentConfiguration, []api.Policy){
		"spec.a2a.operationConfigs.policies[0]": func(c *api.AgentConfiguration, p []api.Policy) {
			c.Spec.A2a.OperationConfigs.Policies = &p
		},
		"spec.a2a.operationConfigs.operations[0].policies[0]": func(c *api.AgentConfiguration, p []api.Policy) {
			c.Spec.A2a.OperationConfigs.Operations = &[]api.A2AOperationConfig{
				{Name: api.SendMessage, Policies: &p},
			}
		},
		"spec.a2a.agentCard.public.policies[0]": func(c *api.AgentConfiguration, p []api.Policy) {
			c.Spec.A2a.AgentCard.Public.Policies = &p
		},
	}

	for prefix, apply := range scopes {
		t.Run(prefix, func(t *testing.T) {
			t.Run("known policy", func(t *testing.T) {
				cfg := validAgent()
				apply(&cfg, []api.Policy{{Name: "APIKeyValidation", Version: "v1",
					Params: &map[string]interface{}{"header": "X-API-Key"}}})

				assert.Empty(t, validator.Validate(&cfg))
			})

			t.Run("unknown policy", func(t *testing.T) {
				cfg := validAgent()
				apply(&cfg, []api.Policy{{Name: "NoSuchPolicy", Version: "v1"}})

				errs := validator.Validate(&cfg)
				require.NotEmpty(t, errs)
				// The exact leaf depends on which check tripped first (an
				// unresolvable version, or a resolved version with no
				// definition); what this pins is that the failure is reported
				// against this scope rather than swallowed.
				assert.True(t, hasFieldWithPrefix(errs, prefix),
					"expected an error under %s, got %v", prefix, fieldsOf(errs))
			})

			t.Run("policy params validated against the definition's schema", func(t *testing.T) {
				cfg := validAgent()
				apply(&cfg, []api.Policy{{Name: "APIKeyValidation", Version: "v1",
					Params: &map[string]interface{}{}}})

				errs := validator.Validate(&cfg)
				require.NotEmpty(t, errs)
				assert.True(t, hasFieldWithPrefix(errs, prefix+".params"),
					"expected a params error under %s, got %v", prefix, fieldsOf(errs))
			})
		})
	}
}

func hasFieldWithPrefix(errs []ValidationError, prefix string) bool {
	for _, e := range errs {
		if strings.HasPrefix(e.Field, prefix) {
			return true
		}
	}
	return false
}

// TestAgentValidator_CoercesRenderedPolicyParams covers the mutation the service
// layer depends on: template rendering turns every value into a string, so an
// integer param arrives as "100" and would fail its own schema. The coerced
// value has to reach the caller, not just the validation pass.
func TestAgentValidator_CoercesRenderedPolicyParams(t *testing.T) {
	validator := NewAgentValidator().WithPolicyValidator(NewPolicyValidator(agentPolicyDefinitions()))

	cfg := validAgent()
	cfg.Spec.A2a.OperationConfigs.Policies = &[]api.Policy{{
		Name:    "RateLimit",
		Version: "v1",
		Params:  &map[string]interface{}{"requestsPerMinute": "100"},
	}}

	require.Empty(t, validator.Validate(&cfg))
	assert.Equal(t, float64(100), (*(*cfg.Spec.A2a.OperationConfigs.Policies)[0].Params)["requestsPerMinute"],
		"the coerced value must be written back through the pointer the caller passed")
}

// TestAgentValidator_WithoutAPolicyValidatorSkipsPolicyChecks documents the
// optional dependency: a validator built without one must not treat every
// policy as unknown, because the file-mode loader constructs it that way.
func TestAgentValidator_WithoutAPolicyValidatorSkipsPolicyChecks(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.A2a.OperationConfigs.Policies = &[]api.Policy{{Name: "NoSuchPolicy", Version: "v1"}}

	assert.Empty(t, validateAgent(&cfg))
}

func agentPolicyDefinitions() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		"APIKeyValidation|v1.0.0": {
			Name:    "APIKeyValidation",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"header": map[string]interface{}{"type": "string"}},
				"required":   []interface{}{"header"},
			},
		},
		"RateLimit|v1.0.0": {
			Name:    "RateLimit",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"requestsPerMinute": map[string]interface{}{"type": "integer"},
				},
			},
		},
	}
}

func TestAgentValidator_ReportsEveryProblemAtOnce(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.DisplayName = ""
	cfg.Spec.Version = "bad"
	cfg.Spec.Upstream.Url = nil

	errs := validateAgent(&cfg)
	fields := fieldsOf(errs)

	// One request, one round of feedback: a validator that stopped at the first
	// problem would make fixing an artifact an iterative guessing game.
	assert.Contains(t, fields, "spec.displayName")
	assert.Contains(t, fields, "spec.version")
	assert.Contains(t, fields, "spec.upstream.url")
}

func TestJoinAgentPath(t *testing.T) {
	tests := []struct {
		base, segment, want string
	}{
		{"/weather", "/rpc", "/weather/rpc"},
		{"/weather", "/", "/weather"},
		{"/weather", "", "/weather"},
		{"/weather", "/message:send", "/weather/message:send"},
		{"/weather", "/tasks/{id}", "/weather/tasks/{id}"},
		{"/", "/rpc", "/rpc"},
		{"/", "/", "/"},
		{"", "/rpc", "/rpc"},
		{"", "", "/"},
		// Exactly one separator, whatever the inputs bring with them.
		{"/weather/", "/rpc", "/weather/rpc"},
		{"/weather/", "rpc", "/weather/rpc"},
	}

	for _, tt := range tests {
		t.Run(tt.base+"+"+tt.segment, func(t *testing.T) {
			assert.Equal(t, tt.want, JoinAgentPath(tt.base, tt.segment))
		})
	}
}

func TestTemplateMatchesPath(t *testing.T) {
	tests := []struct {
		template, literal string
		want              bool
	}{
		{"/tasks/{id}", "/tasks/abc", true},
		{"/tasks/{id}", "/tasks/abc/def", false},
		{"/tasks/{id}", "/tasks", false},
		{"/tasks/{id}", "/tasks/", false},
		{"/tasks", "/tasks", true},
		{"/tasks", "/other", false},
		// A placeholder with a literal suffix must honour the suffix, or every
		// ":verb" operation would look like every other one.
		{"/tasks/{id}:cancel", "/tasks/abc:cancel", true},
		{"/tasks/{id}:cancel", "/tasks/abc:subscribe", false},
		{"/tasks/{id}:cancel", "/tasks/abc", false},
		{"/tasks/{id}/pushNotificationConfigs/{configId}", "/tasks/a/pushNotificationConfigs/b", true},
		{"/tasks/{id}/pushNotificationConfigs/{configId}", "/tasks/a/pushNotificationConfigs", false},
	}

	for _, tt := range tests {
		t.Run(tt.template+" vs "+tt.literal, func(t *testing.T) {
			assert.Equal(t, tt.want, templateMatchesPath(tt.template, tt.literal))
		})
	}
}
