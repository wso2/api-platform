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
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

const shippedTemplatesDir = "../../default-llm-provider-templates"

// The controller logs and skips an out-of-the-box template that fails
// validation, so a broken shipped template would vanish silently at startup.
func TestShippedLLMProviderTemplatesValidate(t *testing.T) {
	loader := NewLLMTemplateLoader(slog.New(slog.NewTextHandler(io.Discard, nil)))
	templates, err := loader.LoadTemplatesFromDirectory(shippedTemplatesDir)
	require.NoError(t, err)
	require.NotEmpty(t, templates)

	validator := config.NewLLMValidator()
	for name, tmpl := range templates {
		assert.Empty(t, validator.Validate(tmpl), "shipped template %q must validate", name)
	}
}

// Foundry serves Claude only on the Anthropic Messages API, and a provider
// that reaches it uses the resource root as its upstream, so the Responses
// API is mapped under both the root and the /openai/v1 base path.
func TestFoundryTemplateMapsEveryAPI(t *testing.T) {
	loader := NewLLMTemplateLoader(slog.New(slog.NewTextHandler(io.Discard, nil)))
	templates, err := loader.LoadTemplatesFromDirectory(shippedTemplatesDir)
	require.NoError(t, err)

	foundry, ok := templates["azureai-foundry"]
	require.True(t, ok, "the azureai-foundry template must ship")
	require.NotNil(t, foundry.Spec.ResourceMappings)
	require.NotNil(t, foundry.Spec.ResourceMappings.Resources)

	byResource := map[string]api.LLMProviderTemplateResourceMapping{}
	for _, mapping := range *foundry.Spec.ResourceMappings.Resources {
		require.NotNil(t, mapping.PromptTokens, "mapping %q must locate prompt tokens", mapping.Resource)
		byResource[mapping.Resource] = mapping
	}
	for _, resource := range []string{"/responses", "/openai/v1/responses", "/anthropic/v1/messages"} {
		mapping, ok := byResource[resource]
		require.True(t, ok, "missing mapping for %s", resource)
		assert.Equal(t, "$.usage.input_tokens", mapping.PromptTokens.Identifier, "prompt tokens for %s", resource)
	}

	// Anthropic's input_tokens exclude cache reads and writes, so the Claude
	// mapping must add them rather than inherit the template's inclusive mode.
	claude := byResource["/anthropic/v1/messages"]
	require.NotNil(t, claude.CacheAccounting)
	assert.Equal(t, "additive", *claude.CacheAccounting)
	require.NotNil(t, claude.CachedTokens)
	assert.Equal(t, "$.usage.cache_read_input_tokens", claude.CachedTokens.Identifier)
}
