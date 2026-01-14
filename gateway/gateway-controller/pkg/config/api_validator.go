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
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// APIValidator validates API configurations using rule-based validation
type APIValidator struct {
	// pathParamRegex matches {param} placeholders
	pathParamRegex *regexp.Regexp
	// versionRegex matches semantic version patterns
	versionRegex *regexp.Regexp
	// urlFriendlyNameRegex matches URL-safe characters for API names
	urlFriendlyNameRegex *regexp.Regexp
	// policyValidator validates policy references and parameters
	policyValidator *PolicyValidator
}

// NewAPIValidator creates a new API configuration validator
func NewAPIValidator() *APIValidator {
	return &APIValidator{
		pathParamRegex:       regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`),
		versionRegex:         regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`),
		urlFriendlyNameRegex: regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`),
	}
}

// SetPolicyValidator sets the policy validator for validating policy references
func (v *APIValidator) SetPolicyValidator(policyValidator *PolicyValidator) {
	v.policyValidator = policyValidator
}

// Validate performs comprehensive validation on a configuration
// It uses type switching to handle APIConfiguration specifically
func (v *APIValidator) Validate(config interface{}) []ValidationError {
	// Type switch to handle different configuration types
	switch cfg := config.(type) {
	case *api.APIConfiguration:
		return v.validateAPIConfiguration(cfg)
	case api.APIConfiguration:
		return v.validateAPIConfiguration(&cfg)
	default:
		return []ValidationError{
			{
				Field:   "config",
				Message: "Unsupported configuration type for APIValidator (expected APIConfiguration)",
			},
		}
	}
}

// validateAPIConfiguration performs comprehensive validation on an API configuration
func (v *APIValidator) validateAPIConfiguration(config *api.APIConfiguration) []ValidationError {
	var errors []ValidationError

	// Validate version
	if config.ApiVersion != api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1 {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Unsupported API version (must be 'api-platform.wso2.com/v1')",
		})
	}

	// Validate kind
	if config.Kind != api.RestApi && config.Kind != api.WebSubApi {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported API kind (only 'RestApi' and 'WebSubApi' are supported)",
		})
	}

	switch config.Kind {
	case api.RestApi:
		spec, err := config.Spec.AsAPIConfigData()
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "spec",
				Message: fmt.Sprintf("Invalid spec format for RestApi: %v", err),
			})
		} else {
			// Validate data section
			errors = append(errors, v.validateRestData(&spec)...)
		}
	case api.WebSubApi:
		spec, err := config.Spec.AsWebhookAPIData()
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "spec",
				Message: fmt.Sprintf("Invalid spec format for async/websub: %v", err),
			})
		} else {
			// Validate data section
			errors = append(errors, v.validateAsyncData(&spec, true)...)
		}
	}

	// Validate policies if policy validator is set
	if v.policyValidator != nil {
		policyErrors := v.policyValidator.ValidatePolicies(config)
		errors = append(errors, policyErrors...)
	}

	return errors
}

// validateUpstream validates a single upstream definition (main or sandbox)
func (v *APIValidator) validateUpstream(label string, up *api.Upstream) []ValidationError {
	var errors []ValidationError
	if up == nil {
		return errors
	}

	// Reject invalid union case explicitly
	if up.Ref != nil && up.Url != nil {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label,
			Message: "Specify exactly one of 'url' or 'ref'",
		})
		return errors
	}

	// Require at least one to be set
	if up.Ref == nil && up.Url == nil {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label,
			Message: "Must specify either 'url' or 'ref'",
		})
		return errors
	}

	// Validate based on which field is set
	if up.Url != nil {
		errors = append(errors, v.validateUpstreamUrl(label, up.Url)...)
	}

	if up.Ref != nil {
		errors = append(errors, v.validateUpstreamRef(label, up.Ref)...)
	}

	return errors
}

func (v *APIValidator) validateUpstreamUrl(label string, upUrl *string) []ValidationError {
	var errors []ValidationError

	if upUrl == nil || strings.TrimSpace(*upUrl) == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: "Upstream URL is required",
		})
		return errors
	}

	parsedURL, err := url.Parse(*upUrl)
	if err != nil {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: fmt.Sprintf("Invalid URL format: %v", err),
		})
		return errors
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: "Upstream URL must use http or https scheme",
		})
	}

	if parsedURL.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: "Upstream URL must include a host",
		})
	}

	return errors
}

func (v *APIValidator) validateUpstreamRef(label string, ref *string) []ValidationError {
	var errors []ValidationError

	// TODO: Implement upstream reference validation

	return errors
}

// validateRestData validates the data section of the configuration for RestApi kind
func (v *APIValidator) validateRestData(spec *api.APIConfigData) []ValidationError {
	var errors []ValidationError

	// Validate name
	if spec.DisplayName == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "API display name is required",
		})
	} else if len(spec.DisplayName) > 100 {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "API display name must be 1-100 characters",
		})
	} else if !v.urlFriendlyNameRegex.MatchString(spec.DisplayName) {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "API display name must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	// Validate version
	if spec.Version == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "API version is required",
		})
	} else if !v.versionRegex.MatchString(spec.Version) {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "API version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)",
		})
	}

	// Validate context
	errors = append(errors, v.validateContext(spec.Context)...)

	// Validate upstream (main + optional sandbox)
	errors = append(errors, v.validateUpstream("main", &spec.Upstream.Main)...)
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, v.validateUpstream("sandbox", spec.Upstream.Sandbox)...)
	}

	// Validate operations
	errors = append(errors, v.validateOperations(spec.Operations)...)

	return errors
}

// validateAsyncData validates the data section of the configuration for http/rest kind
func (v *APIValidator) validateAsyncData(spec *api.WebhookAPIData, isWebSub bool) []ValidationError {
	var errors []ValidationError

	// Validate name
	if spec.DisplayName == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "API name is required",
		})
	} else if len(spec.DisplayName) > 100 {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "API name must be 1-100 characters",
		})
	} else if !v.urlFriendlyNameRegex.MatchString(spec.DisplayName) {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "API name must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	// Validate version
	if spec.Version == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "API version is required",
		})
	} else if !v.versionRegex.MatchString(spec.Version) {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "API version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)",
		})
	}

	// Validate context
	errors = append(errors, v.validateContext(spec.Context)...)

	// Validate upstream if not WebSub
	if spec.Upstream.Main != nil {
		errors = append(errors, v.validateUpstream("main", spec.Upstream.Main)...)
	}
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, v.validateUpstream("sandbox", spec.Upstream.Sandbox)...)
	}

	// Validate operations
	errors = append(errors, v.validateChannels(spec.Channels)...)

	return errors
}

// validateChannels validates the channels configuration
func (v *APIValidator) validateChannels(channels []api.Channel) []ValidationError {
	var errors []ValidationError

	if len(channels) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.channels",
			Message: "At least one channel is required",
		})
		return errors
	}

	for i, op := range channels {

		// Validate path
		if op.Path == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.channels[%d].path", i),
				Message: "Channel path is required",
			})
			continue
		}

		// Validate path parameters have balanced braces
		if !v.validatePathParametersForAsyncAPIs(op.Path) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.channels[%d].path", i),
				Message: "Operation path has braces which is not allowed",
			})
		}

		if !v.validatePathParametersForAsyncAPIs(op.Path) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.channels[%d].path", i),
				Message: "Channel path has unbalanced braces in parameters",
			})
		}
	}

	return errors
}

// validateContext validates the context path
func (v *APIValidator) validateContext(context string) []ValidationError {
	var errors []ValidationError

	if context == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context is required",
		})
		return errors
	}

	if !strings.HasPrefix(context, "/") {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context must start with /",
		})
	}

	if strings.HasSuffix(context, "/") && context != "/" {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context cannot end with / (except for root context)",
		})
	}

	if len(context) > 200 {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context must be 1-200 characters",
		})
	}

	return errors
}

// validatePathParametersForAsyncAPIs returns true when the path does not contain '{' or '}'.
// Async/WebSub channel paths do not currently support templated path parameters.
func (v *APIValidator) validatePathParametersForAsyncAPIs(path string) bool {

	openCount := strings.Count(path, "{")
	closeCount := strings.Count(path, "}")
	return openCount == 0 && closeCount == 0
}

// validateOperations validates the operations configuration
func (v *APIValidator) validateOperations(operations []api.Operation) []ValidationError {
	var errors []ValidationError

	if len(operations) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.operations",
			Message: "At least one operation is required",
		})
		return errors
	}

	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}

	for i, op := range operations {
		// Validate method
		if op.Method == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.operations[%d].method", i),
				Message: "HTTP method is required",
			})
		} else if !validMethods[strings.ToUpper(string(op.Method))] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.operations[%d].method", i),
				Message: fmt.Sprintf("Invalid HTTP method '%s' (must be GET, POST, PUT, DELETE, PATCH, HEAD, or OPTIONS)", op.Method),
			})
		}

		// Validate path
		if op.Path == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.operations[%d].path", i),
				Message: "Operation path is required",
			})
			continue
		}

		if !strings.HasPrefix(op.Path, "/") {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.operations[%d].path", i),
				Message: "Operation path must start with /",
			})
		}

		// Validate path parameters have balanced braces
		if !v.validatePathParameters(op.Path) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.operations[%d].path", i),
				Message: "Operation path has unbalanced braces in parameters",
			})
		}
	}

	return errors
}

// validatePathParameters checks if path parameters have balanced braces
func (v *APIValidator) validatePathParameters(path string) bool {
	openCount := strings.Count(path, "{")
	closeCount := strings.Count(path, "}")
	return openCount == closeCount
}
