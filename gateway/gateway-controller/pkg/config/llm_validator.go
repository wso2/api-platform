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
	"fmt"
	"net/url"
	"regexp"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// LLMValidator validates LLM-related configurations (provider templates, providers, proxies)
// It uses type switching to handle different LLM configuration types
type LLMValidator struct {
	// versionRegex matches semantic version patterns
	versionRegex *regexp.Regexp
	// metadataNameRegex matches URL-safe characters for Metadata.Name
	metadataNameRegex *regexp.Regexp
	// specRegex matches valid LLM specification versions
	specRegex *regexp.Regexp
}

// NewLLMValidator creates a new LLM configuration validator
func NewLLMValidator() *LLMValidator {
	return &LLMValidator{
		versionRegex:      regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`),
		metadataNameRegex: regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`),
		specRegex:         regexp.MustCompile(`^ai\.api-platform\.wso2\.com/v?(\d+)(\.\d+)?(\.\d+)?$`),
	}
}

// Validate performs comprehensive validation on a configuration
// It uses type switching to handle different LLM configuration types:
// - LLMProviderTemplate (for /llm-provider-templates)
// - LLMProvider (for /llm-providers)
// - LLMProxy (for /llm-proxies)
func (v *LLMValidator) Validate(config interface{}) []ValidationError {
	// Type switch to handle different LLM configuration types
	switch cfg := config.(type) {
	case *api.LLMProviderTemplate:
		return v.validateLLMProviderTemplate(cfg)
	case api.LLMProviderTemplate:
		return v.validateLLMProviderTemplate(&cfg)
	case *api.LLMProviderConfiguration:
		return v.validateLLMProvider(cfg)
	case api.LLMProviderConfiguration:
		return v.validateLLMProvider(&cfg)
	// Future: Add cases for LLMProxy
	// case *api.LLMProxy:
	//     return v.validateLLMProxy(cfg)
	default:
		return []ValidationError{
			{
				Field:   "config",
				Message: "Unsupported configuration type for LLMValidator",
			},
		}
	}
}

// validateLLMProviderTemplate validates an LLM provider template configuration
func (v *LLMValidator) validateLLMProviderTemplate(template *api.LLMProviderTemplate) []ValidationError {
	var errors []ValidationError

	// Check if template is nil
	if template == nil {
		return []ValidationError{{
			Field:   "template",
			Message: "Template cannot be nil",
		}}
	}

	// Validate version
	if !v.specRegex.MatchString(string(template.Version)) {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Version must be in the format 'ai.api-platform.wso2.com/vX.Y.Z'",
		})
	}

	// Validate kind
	if template.Kind != api.LlmProviderTemplate {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Kind must be 'LlmProviderTemplate'",
		})
	}

	// Validate Metadata.Name
	if template.Metadata.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name is required",
		})
	} else if len(template.Metadata.Name) > 253 {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name must not exceed 253 characters",
		})
	} else if !v.metadataNameRegex.MatchString(template.Metadata.Name) {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name must consist of lowercase alphanumeric characters, hyphens, or dots, and must start and end with an alphanumeric character",
		})
	}

	// Validate spec section
	errors = append(errors, v.validateTemplateSpec(&template.Spec)...)

	return errors
}

// validateTemplateSpec validates the spec section of an LLM provider template
func (v *LLMValidator) validateTemplateSpec(spec *api.LLMProviderTemplateData) []ValidationError {
	var errors []ValidationError

	// Check if spec is nil
	if spec == nil {
		return []ValidationError{{
			Field:   "spec",
			Message: "Template spec cannot be nil",
		}}
	}

	// Validate display name
	if spec.DisplayName == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "spec.displayName is required",
		})
	} else if len(spec.DisplayName) > 253 {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "spec.displayName must not exceed 253 characters",
		})
	}

	// Validate token identifiers if present
	if spec.PromptTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.promptTokens", spec.PromptTokens)...)
	}

	if spec.CompletionTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.completionTokens", spec.CompletionTokens)...)
	}

	if spec.TotalTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.totalTokens", spec.TotalTokens)...)
	}

	if spec.RequestModel != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.requestModel", spec.RequestModel)...)
	}

	if spec.ResponseModel != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.responseModel", spec.ResponseModel)...)
	}

	if spec.RemainingTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.remainingTokens",
			spec.RemainingTokens)...)
	}

	return errors
}

// validateExtractionIdentifier validates a token identifier configuration
func (v *LLMValidator) validateExtractionIdentifier(
	fieldPrefix string,
	identifier *api.ExtractionIdentifier) []ValidationError {
	var errors []ValidationError

	// Only 'payload', 'header' and 'queryParam' locations are supported
	if identifier.Location != "payload" && identifier.Location != "header" && identifier.Location != "queryParam" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.location", fieldPrefix),
			Message: "Location must be 'payload' or 'header' or 'queryParam'",
		})
	}

	if identifier.Identifier == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.identifier", fieldPrefix),
			Message: "Identifier is required",
		})
	}

	return errors
}

// validateLLMProvider validates an LLM provider configuration
func (v *LLMValidator) validateLLMProvider(provider *api.LLMProviderConfiguration) []ValidationError {
	var errors []ValidationError

	// Check if provider is nil
	if provider == nil {
		return []ValidationError{{
			Field:   "provider",
			Message: "provider cannot be nil",
		}}
	}

	// Validate version
	if !v.specRegex.MatchString(string(provider.Version)) {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Version must be in the format 'ai.api-platform.wso2.com/vX.Y.Z'",
		})
	}

	// Validate kind
	if provider.Kind != api.LlmProvider {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Kind must be 'LlmProvider",
		})
	}

	// Validate Metadata.Name
	if provider.Metadata.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name is required",
		})
	} else if len(provider.Metadata.Name) > 253 {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name must not exceed 253 characters",
		})
	} else if !v.metadataNameRegex.MatchString(provider.Metadata.Name) {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name must consist of lowercase alphanumeric characters, hyphens, or dots, and must start and end with an alphanumeric character",
		})
	}

	// Validate spec section
	errors = append(errors, v.validateProviderSpec(&provider.Spec)...)

	return errors
}

// validateProviderSpec validates the data section of an LLM provider
func (v *LLMValidator) validateProviderSpec(spec *api.LLMProviderConfigData) []ValidationError {
	var errors []ValidationError

	// Check if data is nil
	if spec == nil {
		return []ValidationError{{
			Field:   "spec",
			Message: "Provider spec is required",
		}}
	}

	// Validate display name
	if spec.DisplayName == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "spec.displayName is required",
		})
	} else if len(spec.DisplayName) > 253 {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "spec.displayName must not exceed 253 characters",
		})
	}

	// Spec version is not mandatory, but if provided, validate format
	if spec.Version != "" && !v.versionRegex.MatchString(spec.Version) {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "Provider version is required",
		})
	}

	// Validate template reference
	if spec.Template == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.template",
			Message: "Template reference is required",
		})
	}

	// Validate upstreams
	errors = append(errors, v.validateUpstreamWithAuth(fmt.Sprintf("spec.upstream"), &spec.Upstream)...)

	// Validate access control
	errors = append(errors, v.validateAccessControl("spec.accessControl", &spec.AccessControl)...)

	return errors
}

// validateUpstreamWithAuth validates an UpstreamWithAuth configuration
func (v *LLMValidator) validateUpstreamWithAuth(fieldPrefix string,
	upstream *api.LLMProviderConfigData_Upstream) []ValidationError {
	var errors []ValidationError

	if upstream == nil {
		errors = append(errors, ValidationError{
			Field:   fieldPrefix,
			Message: "Upstream is required",
		})
		return errors
	}

	// Validate URL
	if upstream.Url == nil || *upstream.Url == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.url", fieldPrefix),
			Message: "Upstream URL is required",
		})
	} else {
		parsedURL, err := url.Parse(*upstream.Url)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Message: fmt.Sprintf("Upstream URL is malformed: %v", err),
			})
		} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Message: "Upstream URL must use http or https scheme",
			})
		} else if parsedURL.Host == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Message: "Upstream URL must include a host",
			})
		}
	}

	// Validate auth if present
	if upstream.Auth != nil {
		auth := upstream.Auth
		// Validate 'type'
		if auth.Type == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.auth.type", fieldPrefix),
				Message: "Auth type is required",
			})
		} else if auth.Type != "api-key" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.auth.type", fieldPrefix),
				Message: "Auth type must be 'api-key'",
			})
		}

		// If type is api-key, header and value should also be present
		if auth.Type == "api-key" {
			if auth.Header == nil || *auth.Header == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.header", fieldPrefix),
					Message: "Auth header is required when api-key auth type is set",
				})
			}
			if auth.Value == nil || *auth.Value == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.value", fieldPrefix),
					Message: "Auth value is required when api-key auth type is set",
				})
			}
		}
	}

	return errors
}

// validateAccessControl validates access control configuration
func (v *LLMValidator) validateAccessControl(fieldPrefix string, ac *api.LLMAccessControl) []ValidationError {
	var errors []ValidationError

	// accessControl is required
	if ac == nil {
		return []ValidationError{
			{
				Field:   fieldPrefix,
				Message: "Access control is required",
			},
		}
	}

	// mode is required
	if ac.Mode != "allow_all" && ac.Mode != "deny_all" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.mode", fieldPrefix),
			Message: "Access control mode must be either 'allow_all' or 'deny_all'",
		})
	}

	// Validate exceptions if present
	if ac.Exceptions != nil {
		for i, exception := range *ac.Exceptions {
			if exception.Path == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.exceptions[%d].path", fieldPrefix, i),
					Message: "Exception path is required",
				})
			}

			if len(exception.Methods) == 0 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.exceptions[%d].methods", fieldPrefix, i),
					Message: "At least one method is required for exception",
				})
			}
		}
	}

	return errors
}

// Future: Add validation methods for other LLM entities
//
// validateLLMProxy validates an LLM proxy configuration
