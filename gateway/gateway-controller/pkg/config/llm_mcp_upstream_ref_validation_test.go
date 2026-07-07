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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// upstreamDef builds a valid upstream definition (one host-only target) with an optional connect
// timeout. connect == "" leaves the timeout unset.
func upstreamDef(name, connect string) api.UpstreamDefinition {
	def := api.UpstreamDefinition{
		Name: name,
		Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{
			{Url: "http://backend:8080"},
		},
	}
	if connect != "" {
		def.Timeout = &api.UpstreamTimeout{Connect: stringPtr(connect)}
	}
	return def
}

func providerWithUpstream(defs *[]api.UpstreamDefinition, up api.LLMProviderConfigData_Upstream) api.LLMProviderConfiguration {
	return api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "openai"},
		Spec: api.LLMProviderConfigData{
			DisplayName:         "my-provider",
			Version:             "v1.0",
			Template:            "openai",
			UpstreamDefinitions: defs,
			Upstream:            up,
			AccessControl:       api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
}

func mcpWithUpstream(defs *[]api.UpstreamDefinition, up api.MCPProxyConfigData_Upstream) api.MCPProxyConfiguration {
	ctx := "/everything"
	specVersion := constants.SPEC_VERSION_2025_JUNE
	return api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "everything"},
		Spec: api.MCPProxyConfigData{
			DisplayName:         "Everything",
			Version:             "v1.0",
			Context:             &ctx,
			SpecVersion:         &specVersion,
			UpstreamDefinitions: defs,
			Upstream:            up,
		},
	}
}

func TestValidateLLMProvider_UpstreamRef(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("valid ref resolves to a definition", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("openai-backend", "6s")}
		errs := validator.Validate(providerWithUpstream(defs, api.LLMProviderConfigData_Upstream{Ref: stringPtr("openai-backend")}))
		assert.Empty(t, errs)
	})

	t.Run("ref not found in definitions", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("other", "6s")}
		errs := validator.Validate(providerWithUpstream(defs, api.LLMProviderConfigData_Upstream{Ref: stringPtr("missing")}))
		assertHasFieldError(t, errs, "spec.upstream.ref")
	})

	t.Run("both url and ref rejected", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("openai-backend", "6s")}
		errs := validator.Validate(providerWithUpstream(defs, api.LLMProviderConfigData_Upstream{
			Url: stringPtr("https://api.openai.com"),
			Ref: stringPtr("openai-backend"),
		}))
		assertHasFieldError(t, errs, "spec.upstream")
	})

	t.Run("malformed connect timeout rejected (must match CRD pattern)", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("openai-backend", "1h30m")}
		errs := validator.Validate(providerWithUpstream(defs, api.LLMProviderConfigData_Upstream{Ref: stringPtr("openai-backend")}))
		assertHasFieldError(t, errs, "spec.upstreamDefinitions[0].timeout.connect")
	})

	t.Run("valid fractional connect timeout accepted", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("openai-backend", "500ms")}
		errs := validator.Validate(providerWithUpstream(defs, api.LLMProviderConfigData_Upstream{Ref: stringPtr("openai-backend")}))
		assert.Empty(t, errs)
	})
}

func TestValidateMCP_UpstreamRef(t *testing.T) {
	validator := NewMCPValidator()

	t.Run("valid ref resolves to a definition", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("mcp-backend", "6s")}
		errs := validator.Validate(mcpWithUpstream(defs, api.MCPProxyConfigData_Upstream{Ref: stringPtr("mcp-backend")}))
		assert.Empty(t, errs)
	})

	t.Run("ref not found in definitions", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("other", "6s")}
		errs := validator.Validate(mcpWithUpstream(defs, api.MCPProxyConfigData_Upstream{Ref: stringPtr("missing")}))
		assertHasFieldError(t, errs, "spec.upstream.ref")
	})

	t.Run("both url and ref rejected", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("mcp-backend", "6s")}
		errs := validator.Validate(mcpWithUpstream(defs, api.MCPProxyConfigData_Upstream{
			Url: stringPtr("http://backend:8080"),
			Ref: stringPtr("mcp-backend"),
		}))
		assertHasFieldError(t, errs, "spec.upstream")
	})

	t.Run("malformed connect timeout rejected (must match CRD pattern)", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("mcp-backend", "1h30m")}
		errs := validator.Validate(mcpWithUpstream(defs, api.MCPProxyConfigData_Upstream{Ref: stringPtr("mcp-backend")}))
		assertHasFieldError(t, errs, "spec.upstreamDefinitions[0].timeout.connect")
	})

	t.Run("valid connect timeout accepted", func(t *testing.T) {
		defs := &[]api.UpstreamDefinition{upstreamDef("mcp-backend", "5s")}
		errs := validator.Validate(mcpWithUpstream(defs, api.MCPProxyConfigData_Upstream{Ref: stringPtr("mcp-backend")}))
		assert.Empty(t, errs)
	})
}
