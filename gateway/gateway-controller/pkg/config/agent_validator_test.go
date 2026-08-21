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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
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
			Context:     "/weather",
			Upstream:    api.AgentConfigData_Upstream{Url: &url},
			A2a: api.A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: api.A2AOperationConfigs{
					Transports: []api.A2ATransport{{ProtocolBinding: api.JSONRPC}},
				},
				AgentCard: api.A2AAgentCard{
					Public: api.A2APublicAgentCard{Mode: api.A2APublicAgentCardModeManaged},
				},
			},
		},
	}
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

	assert.Empty(t, NewAgentValidator().Validate(&cfg))
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
			name:  "missing context",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Context = "" },
			field: "spec.context",
		},
		{
			name:  "context inside the reserved health namespace",
			spoil: func(c *api.AgentConfiguration) { c.Spec.Context = "/_gateway-health" },
			field: "spec.context",
		},
		{
			name: "context that only resolves into the reserved namespace after normalization",
			spoil: func(c *api.AgentConfiguration) {
				c.Spec.Context = "/weather/../_gateway-health/ready"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAgent()
			tt.spoil(&cfg)

			errs := NewAgentValidator().Validate(&cfg)
			require.NotEmpty(t, errs, "expected a validation error for %s", tt.field)
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
		algorithm := api.ES256
		cfg.Spec.A2a.AgentCard.Public.Signing = &api.A2ACardSigning{Enabled: true, Algorithm: &algorithm}

		errs := NewAgentValidator().Validate(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.public.signing.enabled")
	})

	t.Run("signing explicitly disabled is accepted", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.AgentCard.Public.Signing = &api.A2ACardSigning{Enabled: false}

		assert.Empty(t, NewAgentValidator().Validate(&cfg))
	})

	t.Run("protected card block", func(t *testing.T) {
		cfg := validAgent()
		cfg.Spec.A2a.AgentCard.Protected = &api.A2AProtectedAgentCard{
			Mode: api.A2AProtectedAgentCardModePassthrough,
		}

		errs := NewAgentValidator().Validate(&cfg)
		require.NotEmpty(t, errs)
		assert.Contains(t, fieldsOf(errs), "spec.a2a.agentCard.protected")
	})
}

func TestAgentValidator_ReportsEveryProblemAtOnce(t *testing.T) {
	cfg := validAgent()
	cfg.Spec.DisplayName = ""
	cfg.Spec.Version = "bad"
	cfg.Spec.Upstream.Url = nil

	errs := NewAgentValidator().Validate(&cfg)
	fields := fieldsOf(errs)

	// One request, one round of feedback: a validator that stopped at the first
	// problem would make fixing an artifact an iterative guessing game.
	assert.Contains(t, fields, "spec.displayName")
	assert.Contains(t, fields, "spec.version")
	assert.Contains(t, fields, "spec.upstream.url")
}
