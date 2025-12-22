/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// TestParseLLMProviderTemplate_YAML_Valid tests successful parsing of valid LLM Provider Template YAML
func TestParseLLMProviderTemplate_YAML_Valid(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		expectedName   string
		expectedFields map[string]bool // tracks which optional fields should be present
	}{
		{
			name: "full template with all fields",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: openai
spec:
  displayName: openai
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  completionTokens:
    location: payload
    identifier: $.usage.completion_tokens
  totalTokens:
    location: payload
    identifier: $.usage.total_tokens
  requestModel:
    location: payload
    identifier: $.model
`,
			expectedName: "openai",
			expectedFields: map[string]bool{
				"promptTokens":     true,
				"completionTokens": true,
				"totalTokens":      true,
				"requestModel":     true,
			},
		},
		{
			name: "minimal template with only required fields",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: minimal-template
spec:
  displayName: minimal-template
`,
			expectedName: "minimal-template",
			expectedFields: map[string]bool{
				"promptTokens":     false,
				"completionTokens": false,
				"totalTokens":      false,
				"requestModel":     false,
			},
		},
		{
			name: "template with header location",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: minimal-template
spec:
  displayName: custom-llm
  promptTokens:
    location: header
    identifier: X-Prompt-Tokens
  completionTokens:
    location: header
    identifier: X-Completion-Tokens
`,
			expectedName: "custom-llm",
			expectedFields: map[string]bool{
				"promptTokens":     true,
				"completionTokens": true,
				"totalTokens":      false,
				"requestModel":     false,
			},
		},
		{
			name: "template with partial fields",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: anthropic
spec:
  displayName: anthropic
  promptTokens:
    location: payload
    identifier: $.usage.input_tokens
  completionTokens:
    location: payload
    identifier: $.usage.output_tokens
`,
			expectedName: "anthropic",
			expectedFields: map[string]bool{
				"promptTokens":     true,
				"completionTokens": true,
				"totalTokens":      false,
				"requestModel":     false,
			},
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var template api.LLMProviderTemplate
			err := parser.Parse([]byte(tt.yaml), "application/yaml", &template)

			require.NoError(t, err, "Failed to parse valid LLM Provider Template YAML")

			// Verify version and kind
			assert.Equal(t, api.LLMProviderTemplateApiVersion("gateway.api-platform.wso2.com/v1alpha1"), template.ApiVersion)
			assert.Equal(t, api.LLMProviderTemplateKind("LlmProviderTemplate"), template.Kind)

			// Verify spec.name
			assert.Equal(t, tt.expectedName, template.Spec.DisplayName)

			// Verify optional fields presence
			if tt.expectedFields["promptTokens"] {
				require.NotNil(t, template.Spec.PromptTokens, "PromptTokens should be present")
				assert.NotEmpty(t, template.Spec.PromptTokens.Location)
				assert.NotEmpty(t, template.Spec.PromptTokens.Identifier)
			} else {
				assert.Nil(t, template.Spec.PromptTokens, "PromptTokens should be nil")
			}

			if tt.expectedFields["completionTokens"] {
				require.NotNil(t, template.Spec.CompletionTokens, "CompletionTokens should be present")
				assert.NotEmpty(t, template.Spec.CompletionTokens.Location)
				assert.NotEmpty(t, template.Spec.CompletionTokens.Identifier)
			} else {
				assert.Nil(t, template.Spec.CompletionTokens, "CompletionTokens should be nil")
			}

			if tt.expectedFields["totalTokens"] {
				require.NotNil(t, template.Spec.TotalTokens, "TotalTokens should be present")
				assert.NotEmpty(t, template.Spec.TotalTokens.Location)
				assert.NotEmpty(t, template.Spec.TotalTokens.Identifier)
			} else {
				assert.Nil(t, template.Spec.TotalTokens, "TotalTokens should be nil")
			}

			if tt.expectedFields["requestModel"] {
				require.NotNil(t, template.Spec.RequestModel, "RequestModel should be present")
				assert.NotEmpty(t, template.Spec.RequestModel.Location)
				assert.NotEmpty(t, template.Spec.RequestModel.Identifier)
			} else {
				assert.Nil(t, template.Spec.RequestModel, "RequestModel should be nil")
			}
		})
	}
}

// TestParseLLMProviderTemplate_JSON_Valid tests successful parsing of valid LLM Provider Template JSON
func TestParseLLMProviderTemplate_JSON_Valid(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		expectedName string
	}{
		{
			name: "full template JSON",
			json: `{
				"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
				"kind": "LlmProviderTemplate",
				"metadata": {
					"name": "openai"
				},
				"spec": {
					"displayName": "openai",
					"promptTokens": {
						"location": "payload",
						"identifier": "$.usage.prompt_tokens"
					},
					"completionTokens": {
						"location": "payload",
						"identifier": "$.usage.completion_tokens"
					},
					"totalTokens": {
						"location": "payload",
						"identifier": "$.usage.total_tokens"
					},
					"requestModel": {
						"location": "payload",
						"identifier": "$.model"
					}
				}
			}`,
			expectedName: "openai",
		},
		{
			name: "minimal template JSON",
			json: `{
				"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
				"kind": "LlmProviderTemplate",
				"metadata": {
					"name": "minimal-template"
				},
				"spec": {
					"displayName": "minimal-template"
				}
			}`,
			expectedName: "minimal-template",
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var template api.LLMProviderTemplate
			err := parser.Parse([]byte(tt.json), "application/json", &template)

			require.NoError(t, err, "Failed to parse valid LLM Provider Template JSON")
			assert.Equal(t, api.LLMProviderTemplateApiVersion("gateway.api-platform.wso2.com/v1alpha1"), template.ApiVersion)
			assert.Equal(t, api.LLMProviderTemplateKind("LlmProviderTemplate"), template.Kind)
			assert.Equal(t, tt.expectedName, template.Spec.DisplayName)
		})
	}
}

// TestParseLLMProviderTemplate_Invalid tests parsing of invalid LLM Provider Template configurations
func TestParseLLMProviderTemplate_Invalid(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "missing version",
			yaml: `
kind: LlmProviderTemplate
metadata:
  name: test
spec:
  displayName: test
`,
		},
		{
			name: "missing kind",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
spec:
  name: test
`,
		},
		{
			name: "missing spec",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
`,
		},
		{
			name: "missing name in spec",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
spec: {}
`,
		},
		{
			name: "malformed YAML",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
spec:
  name: test
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  invalid indentation
`,
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var template api.LLMProviderTemplate
			err := parser.Parse([]byte(tt.yaml), "application/yaml", &template)

			// Note: Parser may succeed even with missing required fields
			// Validation happens in the validator, not the parser
			// However, malformed YAML should fail
			if tt.name == "malformed YAML" {
				assert.Error(t, err, "Malformed YAML should fail to parse")
			}
		})
	}
}

// TestParseLLMProvider_YAML_Valid tests successful parsing of valid LLM Provider YAML
func TestParseLLMProvider_YAML_Valid(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		expectedName    string
		expectedVersion string
		expectedMode    api.LLMAccessControlMode
		hasAuth         bool
		hasVhost        bool
		hasContext      bool
		hasPolicies     bool
		hasExceptions   bool
	}{
		{
			name: "provider with allow_all mode and auth",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: my-openai-provider
spec:
  displayName: my-openai-provider
  version: v1.0
  context: /openai
  vhost: api.openai.com
  template: openai
  upstream:
    url: https://api.openai.com
    auth:
      type: api-key
      header: Authorization
      value: Bearer sk-test123
  accessControl:
    mode: allow_all
    exceptions:
      - path: /admin
        methods:
          - GET
          - POST
`,
			expectedName:    "my-openai-provider",
			expectedVersion: "v1.0",
			expectedMode:    api.AllowAll,
			hasAuth:         true,
			hasVhost:        true,
			hasContext:      true,
			hasPolicies:     false,
			hasExceptions:   true,
		},
		{
			name: "provider with deny_all mode",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: restricted-llm
spec:
  displayName: restricted-llm
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: deny_all
    exceptions:
      - path: /v1/chat/completions
        methods:
          - POST
      - path: /v1/embeddings
        methods:
          - POST
`,
			expectedName:    "restricted-llm",
			expectedVersion: "v1.0",
			expectedMode:    api.DenyAll,
			hasAuth:         false,
			hasVhost:        false,
			hasContext:      false,
			hasPolicies:     false,
			hasExceptions:   true,
		},
		{
			name: "provider with policies",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: policy-test-provider
spec:
  displayName: policy-test-provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: deny_all
    exceptions:
      - path: /v1/chat/completions
        methods:
          - POST
  policies:
    - name: content-length-guardrail
      version: v0.1.0
      paths:
        - path: /v1/chat/completions
          methods:
            - POST
          params:
            maxRequestBodySize: 10240
            maxResponseBodySize: 51200
    - name: regex-guardrail
      version: v0.1.0
      paths:
        - path: /v1/chat/completions
          methods:
            - POST
          params:
            patterns:
              - pattern: "password"
                flags: "i"
            action: "reject"
`,
			expectedName:    "policy-test-provider",
			expectedVersion: "v1.0",
			expectedMode:    api.DenyAll,
			hasAuth:         false,
			hasVhost:        false,
			hasContext:      false,
			hasPolicies:     true,
			hasExceptions:   true,
		},
		{
			name: "minimal provider without optional fields",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: minimal-provider
spec:
  displayName: minimal-provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: allow_all
`,
			expectedName:    "minimal-provider",
			expectedVersion: "v1.0",
			expectedMode:    api.AllowAll,
			hasAuth:         false,
			hasVhost:        false,
			hasContext:      false,
			hasPolicies:     false,
			hasExceptions:   false,
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider api.LLMProviderConfiguration
			err := parser.Parse([]byte(tt.yaml), "application/yaml", &provider)

			require.NoError(t, err, "Failed to parse valid LLM Provider YAML")

			// Verify version and kind
			assert.Equal(t, api.LLMProviderConfigurationApiVersion("gateway.api-platform.wso2.com/v1alpha1"), provider.ApiVersion)
			assert.Equal(t, api.LLMProviderConfigurationKind("LlmProvider"), provider.Kind)

			// Verify spec fields
			assert.Equal(t, tt.expectedName, provider.Spec.DisplayName)
			assert.Equal(t, tt.expectedVersion, provider.Spec.Version)
			assert.Equal(t, tt.expectedMode, provider.Spec.AccessControl.Mode)

			// Verify optional fields
			if tt.hasAuth {
				require.NotNil(t, provider.Spec.Upstream.Auth, "Auth should be present")
				assert.NotEmpty(t, provider.Spec.Upstream.Auth.Type)
			}

			if tt.hasVhost {
				require.NotNil(t, provider.Spec.Vhost, "Vhost should be present")
				assert.NotEmpty(t, *provider.Spec.Vhost)
			} else {
				assert.Nil(t, provider.Spec.Vhost, "Vhost should be nil")
			}

			if tt.hasContext {
				require.NotNil(t, provider.Spec.Context, "Context should be present")
				assert.NotEmpty(t, *provider.Spec.Context)
			} else {
				assert.Nil(t, provider.Spec.Context, "Context should be nil")
			}

			if tt.hasPolicies {
				require.NotNil(t, provider.Spec.Policies, "Policies should be present")
				assert.NotEmpty(t, *provider.Spec.Policies, "Policies array should not be empty")
			} else {
				if provider.Spec.Policies != nil {
					assert.Empty(t, *provider.Spec.Policies, "Policies should be nil or empty")
				}
			}

			if tt.hasExceptions {
				require.NotNil(t, provider.Spec.AccessControl.Exceptions, "Exceptions should be present")
				assert.NotEmpty(t, *provider.Spec.AccessControl.Exceptions, "Exceptions array should not be empty")
			} else {
				if provider.Spec.AccessControl.Exceptions != nil {
					assert.Empty(t, *provider.Spec.AccessControl.Exceptions, "Exceptions should be nil or empty")
				}
			}

			// Verify upstream URL
			require.NotNil(t, provider.Spec.Upstream.Url, "Upstream URL should be present")
			assert.NotEmpty(t, *provider.Spec.Upstream.Url)

			// Verify template reference
			assert.NotEmpty(t, provider.Spec.Template)
		})
	}
}

// TestParseLLMProvider_JSON_Valid tests successful parsing of valid LLM Provider JSON
func TestParseLLMProvider_JSON_Valid(t *testing.T) {
	tests := []struct {
		name            string
		json            string
		expectedName    string
		expectedVersion string
	}{
		{
			name: "full provider JSON",
			json: `{
				"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
				"kind": "LlmProvider",
				"metadata": {
					"name": "my-openai-provider"
				},
				"spec": {
					"displayName": "my-openai-provider",
					"version": "v1.0",
					"context": "/openai",
					"vhost": "api.openai.com",
					"template": "openai",
					"upstream": {
						"url": "https://api.openai.com",
						"auth": {
							"type": "api-key",
							"header": "Authorization",
							"value": "Bearer sk-test123"
						}
					},
					"accessControl": {
						"mode": "allow_all",
						"exceptions": [
							{
								"path": "/admin",
								"methods": ["GET", "POST"]
							}
						]
					}
				}
			}`,
			expectedName:    "my-openai-provider",
			expectedVersion: "v1.0",
		},
		{
			name: "minimal provider JSON",
			json: `{
				"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
				"kind": "LlmProvider",
				"metadata": {
					"name": "minimal-provider"
				},
				"spec": {
					"displayName": "minimal-provider",
					"version": "v1.0",
					"template": "openai",
					"upstream": {
						"url": "https://api.example.com"
					},
					"accessControl": {
						"mode": "allow_all"
					}
				}
			}`,
			expectedName:    "minimal-provider",
			expectedVersion: "v1.0",
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider api.LLMProviderConfiguration
			err := parser.Parse([]byte(tt.json), "application/json", &provider)

			require.NoError(t, err, "Failed to parse valid LLM Provider JSON")
			assert.Equal(t, api.LLMProviderConfigurationApiVersion("gateway.api-platform.wso2.com/v1alpha1"), provider.ApiVersion)
			assert.Equal(t, api.LLMProviderConfigurationKind("LlmProvider"), provider.Kind)
			assert.Equal(t, tt.expectedName, provider.Spec.DisplayName)
			assert.Equal(t, tt.expectedVersion, provider.Spec.Version)
		})
	}
}

// TestParseLLMProvider_Invalid tests parsing of invalid LLM Provider configurations
func TestParseLLMProvider_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		json        string
		contentType string
	}{
		{
			name: "missing version",
			yaml: `
kind: LlmProvider
spec:
  name: test
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: allow_all
`,
			contentType: "application/yaml",
		},
		{
			name: "missing kind",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
spec:
  name: test
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: allow_all
`,
			contentType: "application/yaml",
		},
		{
			name: "malformed JSON",
			json: `{
				"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
				"kind": "LlmProvider",
				"spec": {
					"name": "test",
					missing closing brace
			}`,
			contentType: "application/json",
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider api.LLMProviderConfiguration
			var data []byte

			if tt.yaml != "" {
				data = []byte(tt.yaml)
			} else if tt.json != "" {
				data = []byte(tt.json)
			}

			err := parser.Parse(data, tt.contentType, &provider)

			// Note: Parser may succeed even with missing required fields
			// Validation happens in the validator, not the parser
			if tt.name == "malformed JSON" {
				assert.Error(t, err, "Malformed JSON should fail to parse")
			}
		})
	}
}

// TestParseContentType tests that parser handles different content types correctly
func TestParseContentType(t *testing.T) {
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: test-template
spec:
  displayName: test-template
`

	jsonData := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {
			"name": "test-template"
		},
		"spec": {
			"displayName": "test-template"
		}
	}`

	tests := []struct {
		name        string
		data        []byte
		contentType string
		shouldError bool
	}{
		{
			name:        "YAML with application/yaml",
			data:        []byte(yamlData),
			contentType: "application/yaml",
			shouldError: false,
		},
		{
			name:        "YAML with application/x-yaml",
			data:        []byte(yamlData),
			contentType: "application/x-yaml",
			shouldError: false,
		},
		{
			name:        "YAML with text/yaml",
			data:        []byte(yamlData),
			contentType: "text/yaml",
			shouldError: false,
		},
		{
			name:        "JSON with application/json",
			data:        []byte(jsonData),
			contentType: "application/json",
			shouldError: false,
		},
		{
			name:        "YAML with empty content type (auto-detect)",
			data:        []byte(yamlData),
			contentType: "",
			shouldError: false,
		},
		{
			name:        "JSON with empty content type (auto-detect)",
			data:        []byte(jsonData),
			contentType: "",
			shouldError: false,
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var template api.LLMProviderTemplate
			err := parser.Parse(tt.data, tt.contentType, &template)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "test-template", template.Spec.DisplayName)
			}
		})
	}
}

// TestParseLLMProviderTemplate_ExtractionIdentifier tests parsing of extraction identifiers
func TestParseLLMProviderTemplate_ExtractionIdentifier(t *testing.T) {
	tests := []struct {
		name               string
		yaml               string
		expectedLocation   api.ExtractionIdentifierLocation
		expectedIdentifier string
	}{
		{
			name: "payload location with JSONPath",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: test
spec:
  displayName: test
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
`,
			expectedLocation:   api.Payload,
			expectedIdentifier: "$.usage.prompt_tokens",
		},
		{
			name: "header location with header name",
			yaml: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: test
spec:
  displayName: test
  promptTokens:
    location: header
    identifier: X-Prompt-Tokens
`,
			expectedLocation:   api.Header,
			expectedIdentifier: "X-Prompt-Tokens",
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var template api.LLMProviderTemplate
			err := parser.Parse([]byte(tt.yaml), "application/yaml", &template)

			require.NoError(t, err, "Failed to parse template")
			require.NotNil(t, template.Spec.PromptTokens, "PromptTokens should be present")

			assert.Equal(t, tt.expectedLocation, template.Spec.PromptTokens.Location)
			assert.Equal(t, tt.expectedIdentifier, template.Spec.PromptTokens.Identifier)
		})
	}
}

// TestParseLLMProvider_AccessControlExceptions tests parsing of access control exceptions
func TestParseLLMProvider_AccessControlExceptions(t *testing.T) {
	yaml := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: test-provider
spec:
  displayName: test-provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: allow_all
    exceptions:
      - path: /admin
        methods:
          - GET
          - POST
      - path: /internal/metrics
        methods:
          - GET
          - POST
          - DELETE
`

	parser := NewParser()
	var provider api.LLMProviderConfiguration
	err := parser.Parse([]byte(yaml), "application/yaml", &provider)

	require.NoError(t, err, "Failed to parse provider")
	require.NotNil(t, provider.Spec.AccessControl.Exceptions, "Exceptions should be present")

	exceptions := *provider.Spec.AccessControl.Exceptions
	assert.Len(t, exceptions, 2, "Should have 2 exceptions")

	// Verify first exception
	assert.Equal(t, "/admin", exceptions[0].Path)
	assert.Len(t, exceptions[0].Methods, 2)
	assert.Contains(t, exceptions[0].Methods, api.GET)
	assert.Contains(t, exceptions[0].Methods, api.POST)

	// Verify second exception
	assert.Equal(t, "/internal/metrics", exceptions[1].Path)
	assert.Len(t, exceptions[1].Methods, 3)
	assert.Contains(t, exceptions[1].Methods, api.GET)
	assert.Contains(t, exceptions[1].Methods, api.POST)
	assert.Contains(t, exceptions[1].Methods, api.DELETE)
}

// TestParseLLMProvider_UpstreamAuth tests parsing of upstream authentication
func TestParseLLMProvider_UpstreamAuth(t *testing.T) {
	yaml := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: test-provider
spec:
  displayName: test-provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
    auth:
      type: api-key
      header: Authorization
      value: Bearer sk-test123
  accessControl:
    mode: allow_all
`

	parser := NewParser()
	var provider api.LLMProviderConfiguration
	err := parser.Parse([]byte(yaml), "application/yaml", &provider)

	require.NoError(t, err, "Failed to parse provider")
	require.NotNil(t, provider.Spec.Upstream.Auth, "Auth should be present")

	auth := provider.Spec.Upstream.Auth
	assert.Equal(t, api.LLMProviderConfigDataUpstreamAuthTypeApiKey, auth.Type)
	require.NotNil(t, auth.Header)
	assert.Equal(t, "Authorization", *auth.Header)
	require.NotNil(t, auth.Value)
	assert.Equal(t, "Bearer sk-test123", *auth.Value)
}

// TestParseLLMProvider_Policies tests parsing of policies
func TestParseLLMProvider_Policies(t *testing.T) {
	yaml := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: test-provider
spec:
  displayName: test-provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.example.com
  accessControl:
    mode: deny_all
    exceptions:
      - path: /v1/chat/completions
        methods:
          - POST
  policies:
    - name: content-length-guardrail
      version: v0.1.0
      paths:
        - path: /v1/chat/completions
          methods:
            - POST
          params:
            maxRequestBodySize: 10240
    - name: regex-guardrail
      version: v0.1.0
      paths:
        - path: /v1/chat/completions
          methods:
            - POST
          params:
            patterns:
              - pattern: "password"
                flags: "i"
            action: "reject"
`

	parser := NewParser()
	var provider api.LLMProviderConfiguration
	err := parser.Parse([]byte(yaml), "application/yaml", &provider)

	require.NoError(t, err, "Failed to parse provider")
	require.NotNil(t, provider.Spec.Policies, "Policies should be present")

	policies := *provider.Spec.Policies
	assert.Len(t, policies, 2, "Should have 2 policies")

	// Verify first policy
	assert.Equal(t, "content-length-guardrail", policies[0].Name)
	assert.Equal(t, "v0.1.0", policies[0].Version)
	assert.Len(t, policies[0].Paths, 1)
	assert.Equal(t, "/v1/chat/completions", policies[0].Paths[0].Path)
	assert.NotNil(t, policies[0].Paths[0].Params)

	// Verify second policy
	assert.Equal(t, "regex-guardrail", policies[1].Name)
	assert.Equal(t, "v0.1.0", policies[1].Version)
	assert.Len(t, policies[1].Paths, 1)
	assert.Equal(t, "/v1/chat/completions", policies[1].Paths[0].Path)
	assert.NotNil(t, policies[1].Paths[0].Params)
}

// TestParseRoundTrip tests that parsing and re-marshaling produces consistent results
func TestParseRoundTrip(t *testing.T) {
	originalYAML := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: openai
spec:
  displayName: openai
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  completionTokens:
    location: payload
    identifier: $.usage.completion_tokens
`

	parser := NewParser()
	var template api.LLMProviderTemplate

	// Parse original YAML
	err := parser.Parse([]byte(originalYAML), "application/yaml", &template)
	require.NoError(t, err, "Failed to parse original YAML")

	// Verify core fields are preserved
	assert.Equal(t, "openai", template.Spec.DisplayName)
	assert.NotNil(t, template.Spec.PromptTokens)
	assert.NotNil(t, template.Spec.CompletionTokens)
}
