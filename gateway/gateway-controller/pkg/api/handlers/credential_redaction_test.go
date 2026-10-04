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
package handlers

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// secretValue is the credential these tests assert never reaches a response
// body. Any occurrence of it in marshalled output is a regression.
const secretValue = "Bearer sk-must-never-be-returned-0123456789"

func strPtr(s string) *string { return &s }

// The rematerialize*Config functions are the single choke point every response
// path for these resources passes through (create, list, get, update), so
// asserting redaction there covers all four verbs at once.

func TestRematerializeLLMProviderConfig_RedactsUpstreamCredential(t *testing.T) {
	source := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": "openai-provider"},
		"spec": map[string]any{
			"displayName": "OpenAI Provider",
			"version":     "v1.0",
			"template":    "openai",
			"upstream": map[string]any{
				"url": "https://api.openai.com/v1",
				"auth": map[string]any{
					"type":   "api-key",
					"header": "Authorization",
					"value":  secretValue,
				},
			},
		},
	}

	prov, err := rematerializeLLMProviderConfig(slog.Default(), "id-1", "OpenAI Provider", source)
	require.NoError(t, err)

	require.NotNil(t, prov.Spec.Upstream.Auth, "auth block itself must be preserved")
	assert.Nil(t, prov.Spec.Upstream.Auth.Value, "credential must be cleared, not blanked")

	// Non-secret siblings must survive: redaction must not degrade the response.
	assert.Equal(t, "Authorization", *prov.Spec.Upstream.Auth.Header)
	require.NotNil(t, prov.Spec.Upstream.Url)
	assert.Equal(t, "https://api.openai.com/v1", *prov.Spec.Upstream.Url)

	assertNoSecretInJSON(t, prov)

	// The caller's own source map must be untouched — the stored, unrendered
	// SourceConfiguration is what replicas re-render on consumption, so
	// redaction must never write through to it.
	upstream := source["spec"].(map[string]any)["upstream"].(map[string]any)
	auth := upstream["auth"].(map[string]any)
	assert.Equal(t, secretValue, auth["value"], "stored source configuration must not be mutated")
}

func TestRematerializeLLMProxyConfig_RedactsPrimaryAndAdditionalProviders(t *testing.T) {
	source := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProxy",
		"metadata":   map[string]any{"name": "openai-proxy"},
		"spec": map[string]any{
			"displayName": "OpenAI Proxy",
			"version":     "v1.0",
			"provider": map[string]any{
				"id":   "openai-provider",
				"auth": map[string]any{"type": "api-key", "header": "Authorization", "value": secretValue},
			},
			"additionalProviders": []any{
				map[string]any{
					"id":   "anthropic-provider",
					"as":   "anthropic",
					"auth": map[string]any{"type": "api-key", "header": "X-Api-Key", "value": secretValue},
				},
			},
		},
	}

	proxy, err := rematerializeLLMProxyConfig(slog.Default(), "id-2", "OpenAI Proxy", source)
	require.NoError(t, err)

	require.NotNil(t, proxy.Spec.Provider.Auth)
	assert.Nil(t, proxy.Spec.Provider.Auth.Value, "primary provider credential must be cleared")

	require.NotNil(t, proxy.Spec.AdditionalProviders)
	additional := *proxy.Spec.AdditionalProviders
	require.Len(t, additional, 1)
	require.NotNil(t, additional[0].Auth)
	assert.Nil(t, additional[0].Auth.Value, "additionalProviders[] credential must be cleared too")
	assert.Equal(t, "anthropic-provider", additional[0].Id, "non-secret fields preserved")

	assertNoSecretInJSON(t, proxy)
}

func TestRematerializeMCPProxyConfig_RedactsUpstreamCredential(t *testing.T) {
	source := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "MCPProxy",
		"metadata":   map[string]any{"name": "mcp-proxy"},
		"spec": map[string]any{
			"displayName": "MCP Proxy",
			"version":     "v1.0",
			"upstream": map[string]any{
				"url":  "https://mcp.example.com",
				"auth": map[string]any{"type": "api-key", "header": "Authorization", "value": secretValue},
			},
		},
	}

	mcp, err := rematerializeMCPProxyConfig(slog.Default(), "id-3", "MCP Proxy", source)
	require.NoError(t, err)

	require.NotNil(t, mcp.Spec.Upstream.Auth)
	assert.Nil(t, mcp.Spec.Upstream.Auth.Value)
	assertNoSecretInJSON(t, mcp)
}

// A secret reference is redacted exactly like a literal value.
func TestRematerializeLLMProviderConfig_RedactsSecretReferenceToo(t *testing.T) {
	const handle = `{{ secret "openai-prod-key" }}`
	source := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": "templated-provider"},
		"spec": map[string]any{
			"displayName": "Templated Provider",
			"version":     "v1.0",
			"template":    "openai",
			"upstream": map[string]any{
				"url":  "https://api.openai.com/v1",
				"auth": map[string]any{"type": "api-key", "header": "Authorization", "value": handle},
			},
		},
	}

	prov, err := rematerializeLLMProviderConfig(slog.Default(), "id-4", "Templated Provider", source)
	require.NoError(t, err)

	require.NotNil(t, prov.Spec.Upstream.Auth)
	assert.Nil(t, prov.Spec.Upstream.Auth.Value)

	out, err := json.Marshal(prov)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "openai-prod-key", "secret handle must not be echoed either")
}

// Redaction must tolerate a config with no auth block at all (auth is optional).
func TestRedaction_NoAuthBlock_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		redactLLMProviderCredentials(&api.LLMProviderConfiguration{})
		redactLLMProxyCredentials(&api.LLMProxyConfiguration{})
		redactMCPProxyCredentials(&api.MCPProxyConfiguration{})
		redactLLMProviderCredentials(nil)
		redactLLMProxyCredentials(nil)
		redactMCPProxyCredentials(nil)
	})
}

// buildDeploymentListItem is the generic search/list path. Only the MCP kind
// reaches it today, so the provider and proxy cases are defensive: they pin the
// invariant for any future caller that routes those kinds here.
func TestBuildDeploymentListItem_RedactsEveryCredentialCarryingKind(t *testing.T) {
	upstreamWithSecret := map[string]any{
		"url":  "https://api.openai.com/v1",
		"auth": map[string]any{"type": "api-key", "header": "Authorization", "value": secretValue},
	}

	cases := []struct {
		name string
		kind string
		spec map[string]any
	}{
		{
			name: "LlmProvider",
			kind: string(api.LLMProviderConfigurationKindLlmProvider),
			spec: map[string]any{"displayName": "P", "version": "v1.0", "template": "openai", "upstream": upstreamWithSecret},
		},
		{
			name: "Mcp",
			kind: string(api.MCPProxyConfigurationKindMcp),
			spec: map[string]any{"displayName": "M", "version": "v1.0", "upstream": upstreamWithSecret},
		},
		{
			name: "LlmProxy",
			kind: string(api.LLMProxyConfigurationKindLlmProxy),
			spec: map[string]any{
				"displayName": "X", "version": "v1.0",
				"provider": map[string]any{
					"id":   "openai-provider",
					"auth": map[string]any{"type": "api-key", "value": secretValue},
				},
				"additionalProviders": []any{
					map[string]any{"id": "anthropic", "auth": map[string]any{"type": "api-key", "value": secretValue}},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := &models.StoredConfig{
				UUID:                "id-" + tc.name,
				Kind:                tc.kind,
				DisplayName:         tc.name,
				SourceConfiguration: map[string]any{"apiVersion": "gateway.api-platform.wso2.com/v1", "kind": tc.kind, "metadata": map[string]any{"name": "n"}, "spec": tc.spec},
			}

			item, err := buildDeploymentListItem(slog.Default(), stored)
			require.NoError(t, err)
			assertNoSecretInJSON(t, item)
		})
	}
}

// A kind with no upstream auth in its schema still round-trips unchanged, so the
// dispatch above does not silently drop fields for non-credential kinds.
func TestBuildDeploymentListItem_PassesThroughKindsWithoutCredentials(t *testing.T) {
	stored := &models.StoredConfig{
		UUID:        "id-rest",
		Kind:        string(api.RestAPIKindRestApi),
		DisplayName: "R",
		SourceConfiguration: map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1",
			"kind":       string(api.RestAPIKindRestApi),
			"metadata":   map[string]any{"name": "rest-api"},
			"spec":       map[string]any{"displayName": "R", "version": "v1.0", "context": "/r"},
		},
	}

	item, err := buildDeploymentListItem(slog.Default(), stored)
	require.NoError(t, err)

	out, err := json.Marshal(item)
	require.NoError(t, err)
	assert.Contains(t, string(out), "rest-api", "non-credential kinds must still round-trip their configuration")
}

// assertNoSecretInJSON marshals exactly as the handler would and asserts the
// credential is absent from the wire form, not merely nil on the struct.
func assertNoSecretInJSON(t *testing.T, v any) {
	t.Helper()
	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(out), secretValue),
		"credential leaked into the response body: %s", string(out))
	// Scoped to the auth block: a policy parameter legitimately uses a "value"
	// key, so asserting on the whole document would misfire as fixtures grow.
	for _, authBlock := range regexp.MustCompile(`"auth":\{[^{}]*\}`).FindAllString(string(out), -1) {
		assert.NotContains(t, authBlock, `"value"`,
			"value must be omitted from the auth block, not present-but-empty")
	}
}

// policySecret is a credential supplied through the policyParams bucket rather
// than the deprecated `value` field. For "oauth2" and "other" auth this is the
// only mechanism there is, so a response that keeps it discloses the credential
// just as surely as one that keeps `value`.
const policySecret = "sk-client-secret-must-never-be-returned-0123456789"

// TestRematerializeLLMProxyConfig_RedactsPolicyParamsInEveryShape covers the
// canonical providers[] list alongside the legacy pair, for the policyParams
// mechanism. All three shapes reach the same auth type, so a credential must not
// survive a read through any of them.
func TestRematerializeLLMProxyConfig_RedactsPolicyParamsInEveryShape(t *testing.T) {
	oauthAuth := func() map[string]any {
		return map[string]any{
			"type":       "oauth2",
			"policyName": "oauth2-generator",
			"policyParams": map[string]any{
				"tokenEndpoint": "https://idp.example.com/token",
				"clientId":      "proxy-client",
				"clientSecret":  policySecret,
			},
		}
	}

	t.Run("legacy shape", func(t *testing.T) {
		source := map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1",
			"kind":       "LlmProxy",
			"metadata":   map[string]any{"name": "openai-proxy"},
			"spec": map[string]any{
				"displayName": "OpenAI Proxy",
				"version":     "v1.0",
				"provider":    map[string]any{"id": "openai-provider", "auth": oauthAuth()},
				"additionalProviders": []any{
					map[string]any{"id": "anthropic-provider", "as": "claude", "auth": oauthAuth()},
				},
			},
		}

		proxy, err := rematerializeLLMProxyConfig(slog.Default(), "id-legacy", "OpenAI Proxy", source)
		require.NoError(t, err)

		require.NotNil(t, proxy.Spec.Provider.Auth)
		assert.Nil(t, proxy.Spec.Provider.Auth.PolicyParams, "primary policyParams must be cleared")
		additional := *proxy.Spec.AdditionalProviders
		require.NotNil(t, additional[0].Auth)
		assert.Nil(t, additional[0].Auth.PolicyParams, "additionalProviders[] policyParams must be cleared")
		assert.Equal(t, "anthropic-provider", additional[0].Id, "non-secret fields preserved")

		assertNoPolicySecretInJSON(t, proxy)
	})

	t.Run("canonical shape", func(t *testing.T) {
		source := map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1",
			"kind":       "LlmProxy",
			"metadata":   map[string]any{"name": "openai-proxy"},
			"spec": map[string]any{
				"displayName": "OpenAI Proxy",
				"version":     "v1.0",
				"providers": []any{
					map[string]any{"id": "openai-provider", "isPrimary": true, "auth": oauthAuth()},
					map[string]any{"id": "anthropic-provider", "isPrimary": false, "alias": "claude", "auth": oauthAuth()},
				},
			},
		}

		proxy, err := rematerializeLLMProxyConfig(slog.Default(), "id-canonical", "OpenAI Proxy", source)
		require.NoError(t, err)

		require.NotNil(t, proxy.Spec.Providers)
		entries := *proxy.Spec.Providers
		require.Len(t, entries, 2)
		for i := range entries {
			require.NotNil(t, entries[i].Auth)
			assert.Nil(t, entries[i].Auth.PolicyParams, "providers[%d] policyParams must be cleared", i)
		}
		assert.Equal(t, "anthropic-provider", entries[1].Id, "non-secret fields preserved")

		assertNoPolicySecretInJSON(t, proxy)
	})
}

// TestRematerializeLLMProviderConfig_RedactsPolicyParams covers the provider
// kind, which reaches the same auth type through its own upstream block.
func TestRematerializeLLMProviderConfig_RedactsPolicyParams(t *testing.T) {
	source := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "LlmProvider",
		"metadata":   map[string]any{"name": "openai-provider"},
		"spec": map[string]any{
			"displayName": "OpenAI",
			"version":     "v1.0",
			"template":    "openai",
			"upstream": map[string]any{
				"url": "https://api.openai.com/v1",
				"auth": map[string]any{
					"type":         "other",
					"policyName":   "custom-auth",
					"policyParams": map[string]any{"bearerToken": policySecret},
				},
			},
		},
	}

	provider, err := rematerializeLLMProviderConfig(slog.Default(), "id-prov", "OpenAI", source)
	require.NoError(t, err)

	require.NotNil(t, provider.Spec.Upstream.Auth)
	assert.Nil(t, provider.Spec.Upstream.Auth.PolicyParams, "provider policyParams must be cleared")
	assertNoPolicySecretInJSON(t, provider)
}

// TestRedactAuthCredential_ClearsBothMechanisms pins the shared helper directly:
// whichever mechanism carried the credential, neither survives.
func TestRedactAuthCredential_ClearsBothMechanisms(t *testing.T) {
	value := strPtr(secretValue)
	params := &map[string]interface{}{"clientSecret": policySecret}

	redactAuthCredential(&value, &params)

	assert.Nil(t, value, "value must be cleared")
	assert.Nil(t, params, "policyParams must be cleared")
}

func assertNoPolicySecretInJSON(t *testing.T, v any) {
	t.Helper()
	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(out), policySecret),
		"credential leaked into the response body: %s", string(out))
	assert.False(t, strings.Contains(string(out), "policyParams"),
		"policyParams must be absent from the response entirely: %s", string(out))
}
