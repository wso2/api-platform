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
	"regexp"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// LLMValidator validates LLM-related configurations (provider templates, providers, proxies)
// It uses type switching to handle different LLM configuration types
type LLMValidator struct {
	// versionRegex matches semantic version patterns
	versionRegex *regexp.Regexp
	// urlFriendlyNameRegex matches URL-safe characters for API names
	urlFriendlyNameRegex *regexp.Regexp
	// specRegex matches valid LLM specification versions
	specRegex *regexp.Regexp
}

// NewLLMValidator creates a new LLM configuration validator
func NewLLMValidator() *LLMValidator {
	return &LLMValidator{
		versionRegex:         regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`),
		urlFriendlyNameRegex: regexp.MustCompile(`^[a-zA-Z0-9._-]+$`),
		specRegex:            regexp.MustCompile(`^ai\.api-platform\.wso2\.com/v?(\d+)(\.\d+)?(\.\d+)?$`),
	}
}

// Validate performs comprehensive validation on a configuration
// It uses type switching to handle different LLM configuration types:
// - LLMProviderTemplate (for /llm-providers/templates)
// - LLMProvider (for /llm-providers)
// - LLMProxy (for /llm-proxies) - future implementation
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

	// Validate version
	if !v.specRegex.MatchString(string(template.Version)) {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Version must be in the format 'ai.api-platform.wso2.com/vX.Y.Z'",
		})
	}

	// Validate kind
	if template.Kind != "llm/provider-template" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Kind must be 'llm/provider-template'",
		})
	}

	// Validate data section
	errors = append(errors, v.validateTemplateData(&template.Spec)...)

	return errors
}

// validateTemplateData validates the data section of an LLM provider template
func (v *LLMValidator) validateTemplateData(data *api.LLMProviderTemplateData) []ValidationError {
	var errors []ValidationError

	// Validate name
	if !v.urlFriendlyNameRegex.MatchString(data.Name) {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "Template name must contain only letters, numbers, hyphens, underscores, and dots",
		})
	}

	// Validate token identifiers if present
	if data.PromptTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.promptTokens", data.PromptTokens)...)
	}

	if data.CompletionTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.completionTokens", data.CompletionTokens)...)
	}

	if data.TotalTokens != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.totalTokens", data.TotalTokens)...)
	}

	if data.RequestModel != nil {
		errors = append(errors, v.validateExtractionIdentifier("spec.requestModel", data.RequestModel)...)
	}

	return errors
}

// validateExtractionIdentifier validates a token identifier configuration
func (v *LLMValidator) validateExtractionIdentifier(
	fieldPrefix string,
	identifier *api.ExtractionIdentifier) []ValidationError {
	var errors []ValidationError

	if identifier.Location != "payload" && identifier.Location != "header" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.location", fieldPrefix),
			Message: "Location must be either 'payload' or 'header'",
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

	// Validate version
	if !v.specRegex.MatchString(string(provider.Version)) {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Version must be in the format 'ai.api-platform.wso2.com/vX.Y.Z'",
		})
	}

	// Validate kind
	if provider.Kind != "llm/provider" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Kind must be 'llm/provider'",
		})
	}

	// Validate data section
	errors = append(errors, v.validateProviderData(&provider.Spec)...)

	return errors
}

// validateProviderData validates the data section of an LLM provider
func (v *LLMValidator) validateProviderData(data *api.LLMProviderConfigData) []ValidationError {
	var errors []ValidationError

	// Check if data is nil
	if data == nil {
		return []ValidationError{{
			Field:   "spec",
			Message: "Provider data is required",
		}}
	}

	// Validate name
	if !v.urlFriendlyNameRegex.MatchString(data.Name) {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "Provider name must contain only letters, numbers, hyphens, underscores, and dots",
		})
	}

	// Validate version
	if data.Version == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "Provider version is required",
		})
	}

	// Validate template reference
	if data.Template == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.template",
			Message: "Template reference is required",
		})
	}

	// Validate upstreams
	errors = append(errors, v.validateUpstreamWithAuth(fmt.Sprintf("spec.upstream"), &data.Upstream)...)

	// Validate access control
	errors = append(errors, v.validateAccessControl("spec.accessControl", &data.AccessControl)...)

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
	if *upstream.Url == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.url", fieldPrefix),
			Message: "Upstream URL is required",
		})
	} else if !regexp.MustCompile(`^https?://`).MatchString(*upstream.Url) {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.url", fieldPrefix),
			Message: "Upstream URL must start with http:// or https://",
		})
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
