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
	"fmt"
	"regexp"
	"slices"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// SecretValidator validates Secret configurations using rule-based validation
type SecretValidator struct {
	// urlFriendlyNameRegex matches URL-safe characters for secret display names
	urlFriendlyNameRegex *regexp.Regexp

	// supportedKinds defines supported Secret kinds
	supportedKinds []string

	// supportedTypes defines supported secret types
	supportedTypes []string
}

// NewSecretValidator creates a new Secret configuration validator
func NewSecretValidator() *SecretValidator {
	return &SecretValidator{
		urlFriendlyNameRegex: regexp.MustCompile(`^[a-zA-Z0-9\-_. ]+$`),
		supportedKinds:       []string{"Secret"},
		supportedTypes:       []string{"default"},
	}
}

// Validate performs comprehensive validation on a configuration
func (v *SecretValidator) Validate(config any) []ValidationError {
	switch cfg := config.(type) {
	case *api.SecretConfiguration:
		return v.validateSecretConfiguration(cfg)
	case api.SecretConfiguration:
		return v.validateSecretConfiguration(&cfg)
	default:
		return []ValidationError{
			{
				Field:   "config",
				Message: "Unsupported configuration type for SecretValidator (expected SecretConfiguration)",
			},
		}
	}
}

// validateSecretConfiguration validates the Secret configuration root object
func (v *SecretValidator) validateSecretConfiguration(config *api.SecretConfiguration) []ValidationError {
	var errors []ValidationError

	// Validate apiVersion
	if config.ApiVersion != api.SecretConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1 {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Unsupported configuration version (must be 'gateway.api-platform.wso2.com/v1alpha1')",
		})
	}

	// Validate kind
	if config.Kind != "Secret" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported configuration kind (only 'Secret' is supported)",
		})
	}

	// Validate metadata
	errors = append(errors, ValidateMetadata(&config.Metadata)...)

	// Validate spec section
	errors = append(errors, v.validateSpec(&config.Spec)...)

	return errors
}

// validateSpec validates the spec section of the Secret configuration
func (v *SecretValidator) validateSpec(spec *api.SecretConfigData) []ValidationError {
	var errors []ValidationError

	// Validate displayName
	if spec.DisplayName == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "Secret displayName is required",
		})
	} else if len(spec.DisplayName) > 253 {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "Secret displayName must be 1-253 characters",
		})
	} else if !v.urlFriendlyNameRegex.MatchString(spec.DisplayName) {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "Secret displayName must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	// Validate description
	if spec.Description != nil && len(*spec.Description) > 1024 {
		errors = append(errors, ValidationError{
			Field:   "spec.description",
			Message: "Secret description must be at most 1024 characters",
		})
	}

	// Validate type
	if spec.Type == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.type",
			Message: "Secret type is required",
		})
	} else if !slices.Contains(v.supportedTypes, string(spec.Type)) {
		errors = append(errors, ValidationError{
			Field:   "spec.type",
			Message: fmt.Sprintf("Unsupported secret type (supported types: %s)", strings.Join(v.supportedTypes, ", ")),
		})
	}

	// Validate value
	if spec.Value == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.value",
			Message: "Secret value is required",
		})
	} else if len(spec.Value) > 8192 {
		errors = append(errors, ValidationError{
			Field:   "spec.value",
			Message: "Secret value must be at most 8192 characters",
		})
	}

	return errors
}
