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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// TestNewLLMValidator tests the constructor
func TestNewLLMValidator(t *testing.T) {
	validator := NewLLMValidator()

	assert.NotNil(t, validator, "Validator should not be nil")
	assert.NotNil(t, validator.versionRegex, "Version regex should be initialized")
	assert.NotNil(t, validator.metadataNameRegex, "URL friendly metadata.name regex should be initialized")
}

// ============================================================================
// LLM Provider Template Validation Tests
// ============================================================================

// TestValidateLLMProviderTemplate_Valid tests validation of valid templates
func TestValidateLLMProviderTemplate_Valid(t *testing.T) {
	tests := []struct {
		name     string
		template api.LLMProviderTemplate
	}{
		{
			name: "full template with all fields",
			template: api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "openai",
					PromptTokens: &api.ExtractionIdentifier{
						Location:   api.Payload,
						Identifier: "$.usage.prompt_tokens",
					},
					CompletionTokens: &api.ExtractionIdentifier{
						Location:   api.Payload,
						Identifier: "$.usage.completion_tokens",
					},
					TotalTokens: &api.ExtractionIdentifier{
						Location:   api.Payload,
						Identifier: "$.usage.total_tokens",
					},
					RequestModel: &api.ExtractionIdentifier{
						Location:   api.Payload,
						Identifier: "$.model",
					},
				},
			},
		},
		{
			name: "minimal template",
			template: api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "minimal",
				},
			},
		},
		{
			name: "template with header extraction",
			template: api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "custom-llm",
					PromptTokens: &api.ExtractionIdentifier{
						Location:   api.Header,
						Identifier: "X-Prompt-Tokens",
					},
				},
			},
		},
		{
			name: "template with various name formats",
			template: api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "llm-provider_1.0",
				},
			},
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(&tt.template)
			assert.Empty(t, errors, "Valid template should not produce validation errors")
		})
	}
}

// TestValidateLLMProviderTemplate_InvalidVersion tests version validation
func TestValidateLLMProviderTemplate_InvalidVersion(t *testing.T) {
	tests := []struct {
		name             string
		version          string
		expectError      bool
		errorField       string
		errorMessagePart string
	}{
		{
			name:             "invalid version format - plain v1",
			version:          "v1",
			expectError:      true,
			errorField:       "version",
			errorMessagePart: "gateway.api-platform.wso2.com/v",
		},
		{
			name:             "invalid version format - no prefix",
			version:          "1.0.0",
			expectError:      true,
			errorField:       "version",
			errorMessagePart: "gateway.api-platform.wso2.com/v",
		},
		{
			name:             "invalid version format - wrong prefix",
			version:          "api-platform.wso2.com/v1",
			expectError:      true,
			errorField:       "version",
			errorMessagePart: "gateway.api-platform.wso2.com/v",
		},
		{
			name:        "valid version",
			version:     "gateway.api-platform.wso2.com/v1alpha1",
			expectError: false,
		},
		{
			name:        "valid version with patch",
			version:     "gateway.api-platform.wso2.com/v1alpha1",
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := api.LLMProviderTemplate{
				ApiVersion: api.LLMProviderTemplateApiVersion(tt.version),
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "test",
				},
			}

			errors := validator.Validate(&template)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField && (tt.errorMessagePart == "" || strings.Contains(err.Message, tt.errorMessagePart)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for field %s with message containing %s", tt.errorField, tt.errorMessagePart)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProviderTemplate_InvalidKind tests kind validation
func TestValidateLLMProviderTemplate_InvalidKind(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		expectError bool
	}{
		{
			name:        "invalid kind - llm/template",
			kind:        "llm/template",
			expectError: true,
		},
		{
			name:        "invalid kind - provider-template",
			kind:        "provider-template",
			expectError: true,
		},
		{
			name:        "invalid kind - empty",
			kind:        "",
			expectError: true,
		},
		{
			name:        "valid kind",
			kind:        string(api.LlmProviderTemplate),
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LLMProviderTemplateKind(tt.kind),
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "test",
				},
			}

			errors := validator.Validate(&template)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "kind" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for kind field")
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProviderTemplate_InvalidName tests name validation
func TestValidateLLMProviderTemplate_InvalidName(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		expectError  bool
		errorPart    string
	}{
		{
			name:         "invalid name - empty",
			templateName: "",
			expectError:  true,
			errorPart:    "spec.displayName",
		},
		{
			name:         "invalid name - too large",
			templateName: "a123456789b123456789c123456789d123456789e123456789f123456789g123456789h123456789i123456789j123456789k123456789l123456789m123456789n123456789o123456789p123456789q123456789r123456789s123456789t123456789u123456789v123456789w123456789x123456789y123456789z12345",
			expectError:  true,
			errorPart:    "spec.displayName",
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: tt.templateName,
				},
			}

			errors := validator.Validate(&template)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "spec.displayName" && strings.Contains(err.Message, tt.errorPart) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for spec.name with message containing %s", tt.errorPart)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProviderTemplate_InvalidMetadataName tests metadata.name validation
func TestValidateLLMProviderTemplate_InvalidMetadataName(t *testing.T) {
	tests := []struct {
		name         string
		metadataName string
		expectError  bool
		errorPart    string
	}{
		{
			name:         "invalid spec.displayName - spaces",
			metadataName: "my provider config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - special chars @",
			metadataName: "provider@config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - special chars #",
			metadataName: "provider#config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - slash",
			metadataName: "provider/config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - brackets",
			metadataName: "provider[1]",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - parentheses",
			metadataName: "provider(v1)",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - percent",
			metadataName: "provider%20config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - ampersand",
			metadataName: "provider&config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - plus",
			metadataName: "provider+config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - equals",
			metadataName: "provider=config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - question mark",
			metadataName: "provider?config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - colon",
			metadataName: "provider:config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - semicolon",
			metadataName: "provider;config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - empty string",
			metadataName: "",
			expectError:  true,
			errorPart:    "required",
		},
		{
			name:         "invalid metadata.name - only spaces",
			metadataName: "   ",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - starts with hyphen",
			metadataName: "-provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - starts with dot",
			metadataName: ".provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - starts with underscore",
			metadataName: "_provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - ends with hyphen",
			metadataName: "provider-",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - ends with dot",
			metadataName: "provider.",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - consecutive hyphens",
			metadataName: "provider--config",
			expectError:  false,
		},
		{
			name:         "invalid metadata.name - consecutive dots",
			metadataName: "provider..config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - letters only",
			metadataName: "provider",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - with hyphen",
			metadataName: "my-provider",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - with underscore",
			metadataName: "my_provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - with dot",
			metadataName: "my.provider",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - with numbers",
			metadataName: "provider-v1",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - complex",
			metadataName: "my-llm_provider.v1",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - all alphanumeric",
			metadataName: "provider123",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - mixed case",
			metadataName: "MyProvider-Config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - starts with number",
			metadataName: "1provider",
			expectError:  false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: tt.metadataName},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test-display-name",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "metadata.name" && strings.Contains(err.Message, tt.errorPart) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for metadata.name with message containing %s", tt.errorPart)
			} else {
				assert.Empty(t, errors, "Should not have validation errors for valid metadata.name")
			}
		})
	}
}

// TestValidateLLMProviderTemplate_ExtractionIdentifier tests extraction identifier validation
func TestValidateLLMProviderTemplate_ExtractionIdentifier(t *testing.T) {
	tests := []struct {
		name        string
		identifier  *api.ExtractionIdentifier
		fieldPrefix string
		expectError bool
		errorField  string
		errorPart   string
	}{
		{
			name: "invalid location - body",
			identifier: &api.ExtractionIdentifier{
				Location:   "body",
				Identifier: "$.tokens",
			},
			fieldPrefix: "spec.promptTokens",
			expectError: true,
			errorField:  "spec.promptTokens.location",
			errorPart:   "payload' or 'header",
		},
		{
			name: "invalid location - empty",
			identifier: &api.ExtractionIdentifier{
				Location:   "",
				Identifier: "$.tokens",
			},
			fieldPrefix: "spec.promptTokens",
			expectError: true,
			errorField:  "spec.promptTokens.location",
		},
		{
			name: "missing identifier",
			identifier: &api.ExtractionIdentifier{
				Location:   api.Payload,
				Identifier: "",
			},
			fieldPrefix: "spec.promptTokens",
			expectError: true,
			errorField:  "spec.promptTokens.identifier",
			errorPart:   "required",
		},
		{
			name: "valid payload location",
			identifier: &api.ExtractionIdentifier{
				Location:   api.Payload,
				Identifier: "$.usage.tokens",
			},
			fieldPrefix: "spec.promptTokens",
			expectError: false,
		},
		{
			name: "valid header location",
			identifier: &api.ExtractionIdentifier{
				Location:   api.Header,
				Identifier: "X-Token-Count",
			},
			fieldPrefix: "spec.promptTokens",
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := api.LLMProviderTemplate{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProviderTemplate,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName:  "test",
					PromptTokens: tt.identifier,
				},
			}

			errors := validator.Validate(&template)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField && (tt.errorPart == "" || strings.Contains(err.Message, tt.errorPart)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for field %s", tt.errorField)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// ============================================================================
// LLM Provider Configuration Validation Tests
// ============================================================================

// TestValidateLLMProvider_Valid tests validation of valid provider configurations
func TestValidateLLMProvider_Valid(t *testing.T) {
	tests := []struct {
		name     string
		provider api.LLMProviderConfiguration
	}{
		{
			name: "full provider with all fields",
			provider: api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "my-provider",
					Version:     "v1.0",
					Context:     stringPtr("/openai"),
					Vhost:       stringPtr("api.openai.com"),
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.openai.com"),
						Auth: &struct {
							Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
							Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
							Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
						}{
							Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
							Header: stringPtr("Authorization"),
							Value:  stringPtr("Bearer sk-test"),
						},
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			},
		},
		{
			name: "minimal provider",
			provider: api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "minimal",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			},
		},
		{
			name: "provider with deny_all mode",
			provider: api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "restricted",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.DenyAll,
						Exceptions: &[]api.RouteException{
							{
								Path:    "/v1/chat/completions",
								Methods: []api.RouteExceptionMethods{api.POST},
							},
						},
					},
				},
			},
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(&tt.provider)
			assert.Empty(t, errors, "Valid provider should not produce validation errors")
		})
	}
}

// TestValidateLLMProvider_InvalidVersion tests provider version validation
func TestValidateLLMProvider_InvalidVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
	}{
		{
			name:        "invalid version - v1 only",
			version:     "v1",
			expectError: true,
		},
		{
			name:        "invalid version - wrong domain",
			version:     "api-platform.wso2.com/v1",
			expectError: true,
		},
		{
			name:        "valid version - v1",
			version:     "gateway.api-platform.wso2.com/v1alpha1",
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: api.LLMProviderConfigurationApiVersion(tt.version),
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "version" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for version field")
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_InvalidKind tests provider kind validation
func TestValidateLLMProvider_InvalidKind(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		expectError bool
	}{
		{
			name:        "invalid kind - llm/proxy",
			kind:        "llm/proxy",
			expectError: true,
		},
		{
			name:        "invalid kind - provider",
			kind:        "provider",
			expectError: true,
		},
		{
			name:        "valid kind",
			kind:        string(api.LlmProvider),
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LLMProviderConfigurationKind(tt.kind),
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "kind" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for kind field")
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_Name tests provider name validation
func TestValidateLLMProvider_Name(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		expectError  bool
		errorPart    string
	}{
		{
			name:         "invalid name - empty",
			providerName: "",
			expectError:  true,
			errorPart:    "spec.displayName",
		},
		{
			name:         "invalid name - too large",
			providerName: "a123456789b123456789c123456789d123456789e123456789f123456789g123456789h123456789i123456789j123456789k123456789l123456789m123456789n123456789o123456789p123456789q123456789r123456789s123456789t123456789u123456789v123456789w123456789x123456789y123456789z12345",
			expectError:  true,
			errorPart:    "spec.displayName",
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: tt.providerName,
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "spec.displayName" && strings.Contains(err.Message, tt.errorPart) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for spec.name field")
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_InvalidMetadataName tests metadata.name validation for LLM Provider
func TestValidateLLMProvider_InvalidMetadataName(t *testing.T) {
	tests := []struct {
		name         string
		metadataName string
		expectError  bool
		errorPart    string
	}{
		{
			name:         "invalid metadata.name - spaces",
			metadataName: "my provider config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - special chars @",
			metadataName: "provider@config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - special chars #",
			metadataName: "provider#config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - slash",
			metadataName: "provider/config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - brackets",
			metadataName: "provider[1]",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - parentheses",
			metadataName: "provider(v1)",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - percent",
			metadataName: "provider%20config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - ampersand",
			metadataName: "provider&config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - plus",
			metadataName: "provider+config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - equals",
			metadataName: "provider=config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - question mark",
			metadataName: "provider?config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - colon",
			metadataName: "provider:config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - semicolon",
			metadataName: "provider;config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - empty string",
			metadataName: "",
			expectError:  true,
			errorPart:    "required",
		},
		{
			name:         "invalid metadata.name - only spaces",
			metadataName: "   ",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - starts with hyphen",
			metadataName: "-provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - starts with dot",
			metadataName: ".provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - starts with underscore",
			metadataName: "_provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - ends with hyphen",
			metadataName: "provider-",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "invalid metadata.name - ends with dot",
			metadataName: "provider.",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - consecutive hyphens",
			metadataName: "provider--config",
			expectError:  false,
		},
		{
			name:         "invalid metadata.name - consecutive dots",
			metadataName: "provider..config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - letters only",
			metadataName: "provider",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - with hyphen",
			metadataName: "my-provider",
			expectError:  false,
		},
		{
			name:         "invalid metadata.name - with underscore",
			metadataName: "my_provider",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - with dot",
			metadataName: "my.provider",
			expectError:  false,
		},
		{
			name:         "valid metadata.name - with numbers",
			metadataName: "provider-v1",
			expectError:  false,
		},
		{
			name:         "invalid metadata.name - complex",
			metadataName: "my-llm_provider.v1",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - all alphanumeric",
			metadataName: "provider123",
			expectError:  false,
		},
		{
			name:         "invalid metadata.name - mixed case",
			metadataName: "MyProvider-Config",
			expectError:  true,
			errorPart:    "metadata.name must consist of",
		},
		{
			name:         "valid metadata.name - starts with number",
			metadataName: "1provider",
			expectError:  false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: tt.metadataName},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test-display-name",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "metadata.name" && strings.Contains(err.Message, tt.errorPart) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for metadata.name with message containing %s", tt.errorPart)
			} else {
				assert.Empty(t, errors, "Should not have validation errors for valid metadata.name")
			}
		})
	}
}

// TestValidateLLMProvider_Version tests provider version field validation
func TestValidateLLMProvider_ProviderVersion(t *testing.T) {
	tests := []struct {
		name            string
		providerVersion string
		expectError     bool
	}{
		{
			name:            "empty version",
			providerVersion: "",
			expectError:     false,
		},
		{
			name:            "valid version - v1.0",
			providerVersion: "v1.0",
			expectError:     false,
		},
		{
			name:            "valid version - v1.0.0",
			providerVersion: "v1.0.0",
			expectError:     false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     tt.providerVersion,
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "spec.version" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for spec.version field")
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_Template tests template reference validation
func TestValidateLLMProvider_Template(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		expectError bool
	}{
		{
			name:        "empty template",
			template:    "",
			expectError: true,
		},
		{
			name:        "valid template",
			template:    "openai",
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    tt.template,
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == "spec.template" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for spec.template field")
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_Upstream tests upstream validation
func TestValidateLLMProvider_Upstream(t *testing.T) {
	tests := []struct {
		name        string
		upstream    api.LLMProviderConfigData_Upstream
		expectError bool
		errorField  string
		errorPart   string
	}{
		{
			name: "missing URL",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: nil,
			},
			expectError: true,
			errorField:  "spec.upstream.url",
			errorPart:   "required",
		},
		{
			name: "empty URL",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr(""),
			},
			expectError: true,
			errorField:  "spec.upstream.url",
		},
		{
			name: "invalid URL - no protocol",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("api.example.com"),
			},
			expectError: true,
			errorField:  "spec.upstream.url",
			errorPart:   "http",
		},
		{
			name: "invalid URL - ftp protocol",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("ftp://api.example.com"),
			},
			expectError: true,
			errorField:  "spec.upstream.url",
			errorPart:   "http",
		},
		{
			name: "valid URL - http",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("http://api.example.com"),
			},
			expectError: false,
		},
		{
			name: "valid URL - https",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			expectError: false,
		},
		{
			name: "valid URL - https with port",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com:8443"),
			},
			expectError: false,
		},
		{
			name: "valid URL - https with path",
			upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com/v1"),
			},
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream:    tt.upstream,
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField && (tt.errorPart == "" || strings.Contains(err.Message, tt.errorPart)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for field %s with message containing %s", tt.errorField, tt.errorPart)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_UpstreamAuth tests upstream authentication validation
func TestValidateLLMProvider_UpstreamAuth(t *testing.T) {
	tests := []struct {
		name string
		auth *struct {
			Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
			Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
		}
		expectError bool
		errorField  string
		errorPart   string
	}{
		{
			name: "missing auth type",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:   "",
				Header: stringPtr("Authorization"),
				Value:  stringPtr("Bearer sk-test"),
			},
			expectError: true,
			errorField:  "spec.upstream.auth.type",
			errorPart:   "required",
		},
		{
			name: "invalid auth type",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:   "bearer",
				Header: stringPtr("Authorization"),
				Value:  stringPtr("Bearer sk-test"),
			},
			expectError: true,
			errorField:  "spec.upstream.auth.type",
			errorPart:   "api-key",
		},
		{
			name: "api-key without header",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:  api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
				Value: stringPtr("sk-test"),
			},
			expectError: true,
			errorField:  "spec.upstream.auth.header",
			errorPart:   "required",
		},
		{
			name: "api-key with empty header",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
				Header: stringPtr(""),
				Value:  stringPtr("sk-test"),
			},
			expectError: true,
			errorField:  "spec.upstream.auth.header",
			errorPart:   "required",
		},
		{
			name: "api-key without value",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
				Header: stringPtr("Authorization"),
			},
			expectError: true,
			errorField:  "spec.upstream.auth.value",
			errorPart:   "required",
		},
		{
			name: "api-key with empty value",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
				Header: stringPtr("Authorization"),
				Value:  stringPtr(""),
			},
			expectError: true,
			errorField:  "spec.upstream.auth.value",
			errorPart:   "required",
		},
		{
			name: "valid api-key auth",
			auth: &struct {
				Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
				Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
				Header: stringPtr("Authorization"),
				Value:  stringPtr("Bearer sk-test"),
			},
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url:  stringPtr("https://api.example.com"),
						Auth: tt.auth,
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField && (tt.errorPart == "" || strings.Contains(err.Message, tt.errorPart)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for field %s", tt.errorField)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_AccessControl tests access control validation
func TestValidateLLMProvider_AccessControl(t *testing.T) {
	tests := []struct {
		name          string
		accessControl api.LLMAccessControl
		expectError   bool
		errorField    string
		errorPart     string
	}{
		{
			name: "invalid mode",
			accessControl: api.LLMAccessControl{
				Mode: "allow_some",
			},
			expectError: true,
			errorField:  "spec.accessControl.mode",
			errorPart:   "allow_all' or 'deny_all",
		},
		{
			name: "empty mode",
			accessControl: api.LLMAccessControl{
				Mode: "",
			},
			expectError: true,
			errorField:  "spec.accessControl.mode",
		},
		{
			name: "valid allow_all",
			accessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			expectError: false,
		},
		{
			name: "valid deny_all",
			accessControl: api.LLMAccessControl{
				Mode: api.DenyAll,
			},
			expectError: false,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: tt.accessControl,
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField && (tt.errorPart == "" || strings.Contains(err.Message, tt.errorPart)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for field %s", tt.errorField)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// TestValidateLLMProvider_AccessControlExceptions tests exception validation
func TestValidateLLMProvider_AccessControlExceptions(t *testing.T) {
	tests := []struct {
		name        string
		exceptions  []api.RouteException
		expectError bool
		errorField  string
		errorPart   string
	}{
		{
			name: "exception with empty path",
			exceptions: []api.RouteException{
				{
					Path:    "",
					Methods: []api.RouteExceptionMethods{api.GET},
				},
			},
			expectError: true,
			errorField:  "spec.accessControl.exceptions[0].path",
			errorPart:   "required",
		},
		{
			name: "exception with empty methods",
			exceptions: []api.RouteException{
				{
					Path:    "/admin",
					Methods: []api.RouteExceptionMethods{},
				},
			},
			expectError: true,
			errorField:  "spec.accessControl.exceptions[0].methods",
			errorPart:   "At least one method",
		},
		{
			name: "valid single exception",
			exceptions: []api.RouteException{
				{
					Path:    "/admin",
					Methods: []api.RouteExceptionMethods{api.GET, api.POST},
				},
			},
			expectError: false,
		},
		{
			name: "valid multiple exceptions",
			exceptions: []api.RouteException{
				{
					Path:    "/admin",
					Methods: []api.RouteExceptionMethods{api.GET},
				},
				{
					Path:    "/internal/metrics",
					Methods: []api.RouteExceptionMethods{api.GET, api.POST, api.DELETE},
				},
			},
			expectError: false,
		},
		{
			name: "second exception invalid",
			exceptions: []api.RouteException{
				{
					Path:    "/admin",
					Methods: []api.RouteExceptionMethods{api.GET},
				},
				{
					Path:    "",
					Methods: []api.RouteExceptionMethods{api.POST},
				},
			},
			expectError: true,
			errorField:  "spec.accessControl.exceptions[1].path",
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode:       api.AllowAll,
						Exceptions: &tt.exceptions,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				require.NotEmpty(t, errors, "Should have validation errors")
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField && (tt.errorPart == "" || strings.Contains(err.Message, tt.errorPart)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find error for field %s", tt.errorField)
			} else {
				assert.Empty(t, errors, "Should not have validation errors")
			}
		})
	}
}

// ============================================================================
// Edge Cases and Security Tests
// ============================================================================

// TestValidateLLMProvider_NilSpec tests validation with nil spec
func TestValidateLLMProvider_NilSpec(t *testing.T) {
	validator := NewLLMValidator()

	provider := api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       api.LlmProvider,
		// Spec is nil/zero value
	}

	errors := validator.Validate(&provider)
	// Should have validation errors for missing required fields
	assert.NotEmpty(t, errors, "Should have validation errors for nil spec")
}

// TestValidateLLMProvider_ExtremelyLongInputs tests handling of very long inputs
func TestValidateLLMProvider_ExtremelyLongInputs(t *testing.T) {
	validator := NewLLMValidator()

	// Create extremely long name (>1000 characters)
	longName := ""
	for i := 0; i < 1000; i++ {
		longName += "a"
	}

	provider := api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       api.LlmProvider,
		Metadata:   api.Metadata{Name: "openai"},
		Spec: api.LLMProviderConfigData{
			DisplayName: longName,
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	errors := validator.Validate(&provider)
	// Validation should complete without crashing
	// May or may not have errors depending on length limits
	assert.NotNil(t, errors, "Validator should handle extremely long inputs")
}

// TestValidateLLMProvider_URLValidation tests various URL formats
func TestValidateLLMProvider_URLValidation(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		// Valid URLs
		{
			name:        "valid https with subdomain",
			url:         "https://api.openai.com",
			expectError: false,
		},
		{
			name:        "valid https with port",
			url:         "https://api.example.com:8443",
			expectError: false,
		},
		{
			name:        "valid https with path",
			url:         "https://api.example.com/v1/llm",
			expectError: false,
		},
		{
			name:        "valid http localhost",
			url:         "http://localhost:8080",
			expectError: false,
		},
		{
			name:        "valid IP address",
			url:         "https://192.168.1.1:8080",
			expectError: false,
		},
		// Invalid URLs
		{
			name:        "javascript protocol",
			url:         "javascript:alert('XSS')",
			expectError: true,
		},
		{
			name:        "file protocol",
			url:         "file:///etc/passwd",
			expectError: true,
		},
		{
			name:        "data URI",
			url:         "data:text/html,<script>alert('XSS')</script>",
			expectError: true,
		},
		{
			name:        "ftp protocol",
			url:         "ftp://example.com",
			expectError: true,
		},
		{
			name:        "no protocol",
			url:         "example.com",
			expectError: true,
		},
		{
			name:        "malformed URL",
			url:         "https://",
			expectError: true,
		},
	}

	validator := NewLLMValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := api.LLMProviderConfiguration{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.LlmProvider,
				Metadata:   api.Metadata{Name: "openai"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr(tt.url),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			errors := validator.Validate(&provider)

			if tt.expectError {
				assert.NotEmpty(t, errors, "URL should fail validation: %s", tt.url)
			} else {
				assert.Empty(t, errors, "URL should pass validation: %s", tt.url)
			}
		})
	}
}

// TestValidate_UnsupportedConfigType tests validation with unsupported config type
func TestValidate_UnsupportedConfigType(t *testing.T) {
	validator := NewLLMValidator()

	type UnsupportedConfig struct {
		Name string
	}

	unsupported := &UnsupportedConfig{Name: "test"}
	errors := validator.Validate(unsupported)

	require.NotEmpty(t, errors, "Should have validation errors for unsupported config type")
	assert.Equal(t, "config", errors[0].Field)
	assert.Contains(t, errors[0].Message, "Unsupported configuration type")
}
