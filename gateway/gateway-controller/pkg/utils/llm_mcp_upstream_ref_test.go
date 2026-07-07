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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// These tests verify that the LLM Provider and MCP converters map an upstream `ref` onto the
// derived RestAPI (instead of a direct url) and thread the upstreamDefinitions through, so the
// per-upstream connect timeout resolves the same way it does for RestApi.

func TestTransform_Provider_UpstreamRef_ThreadsDefinitions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	defs := &[]api.UpstreamDefinition{{
		Name:    "openai-backend",
		Timeout: &api.UpstreamTimeout{Connect: stringPtr("6s")},
	}}
	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:         "ref-provider",
			Version:             "v1.0",
			Template:            "openai",
			UpstreamDefinitions: defs,
			Upstream:            api.LLMProviderConfigData_Upstream{Ref: stringPtr("openai-backend")},
			AccessControl:       api.LLMAccessControl{Mode: api.AllowAll},
		},
	}

	res, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	require.NotNil(t, res.Spec.Upstream.Main.Ref, "ref should be mapped onto the derived upstream")
	assert.Equal(t, "openai-backend", *res.Spec.Upstream.Main.Ref)
	assert.Nil(t, res.Spec.Upstream.Main.Url, "ref-based upstream must not also set url")

	require.NotNil(t, res.Spec.UpstreamDefinitions, "upstreamDefinitions must be threaded through")
	require.Len(t, *res.Spec.UpstreamDefinitions, 1)
	require.NotNil(t, (*res.Spec.UpstreamDefinitions)[0].Timeout)
	assert.Equal(t, "6s", *(*res.Spec.UpstreamDefinitions)[0].Timeout.Connect)
}

func TestTransform_Provider_UpstreamUrl_Unchanged(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:   "url-provider",
			Version:       "v1.0",
			Template:      "openai",
			Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.openai.com")},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}

	res, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	require.NotNil(t, res.Spec.Upstream.Main.Url)
	assert.Equal(t, "https://api.openai.com", *res.Spec.Upstream.Main.Url)
	assert.Nil(t, res.Spec.Upstream.Main.Ref)
	assert.Nil(t, res.Spec.UpstreamDefinitions)
}

func TestMCPTransform_UpstreamRef_ThreadsDefinitions(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	defs := []api.UpstreamDefinition{{
		Name:    "mcp-backend",
		Timeout: &api.UpstreamTimeout{Connect: stringPtr("6s")},
	}}
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName:         "everything",
			Version:             "v1.0",
			Context:             &context,
			SpecVersion:         &latest,
			UpstreamDefinitions: &defs,
			Upstream:            api.MCPProxyConfigData_Upstream{Ref: stringPtr("mcp-backend")},
		},
	}

	var out api.RestAPI
	res, err := (&MCPTransformer{}).Transform(in, &out)
	require.NoError(t, err)

	require.NotNil(t, res.Spec.Upstream.Main.Ref)
	assert.Equal(t, "mcp-backend", *res.Spec.Upstream.Main.Ref)
	assert.Nil(t, res.Spec.Upstream.Main.Url)

	require.NotNil(t, res.Spec.UpstreamDefinitions)
	require.Len(t, *res.Spec.UpstreamDefinitions, 1)
	require.NotNil(t, (*res.Spec.UpstreamDefinitions)[0].Timeout)
	assert.Equal(t, "6s", *(*res.Spec.UpstreamDefinitions)[0].Timeout.Connect)
}

func TestMCPTransform_UpstreamUrl_Unchanged(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
		},
	}

	var out api.RestAPI
	res, err := (&MCPTransformer{}).Transform(in, &out)
	require.NoError(t, err)

	require.NotNil(t, res.Spec.Upstream.Main.Url)
	assert.Equal(t, "http://backend:8080", *res.Spec.Upstream.Main.Url)
	assert.Nil(t, res.Spec.Upstream.Main.Ref)
	assert.Nil(t, res.Spec.UpstreamDefinitions)
}
