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
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validator is an interface for validating configurations
// This allows for different validation strategies (API, LLM, MCP, etc.)
// Each validator implementation handles different configuration types using type switching
type Validator interface {
	Validate(config interface{}) []ValidationError
}

// ValidateMetadata is a helper function to validate metadata
// This can be used by validator implementations
func ValidateMetadata(metadata *api.Metadata) []ValidationError {
	var errors []ValidationError
	if metadata == nil || metadata.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "Metadata name is required",
		})
	}

	// Validate labels
	if metadata != nil && metadata.Labels != nil {
		errors = append(errors, ValidateLabels(*metadata.Labels)...)
	}

	return errors
}

// ValidateLabels validates that label keys do not contain spaces
// This is a common validation used across all configuration types
func ValidateLabels(labels map[string]string) []ValidationError {
	var errors []ValidationError
	if labels == nil {
		return errors
	}

	for key := range labels {
		if strings.Contains(key, " ") {
			errors = append(errors, ValidationError{
				Field:   "metadata.labels",
				Message: fmt.Sprintf("Label key '%s' contains spaces. Label keys must not contain spaces.", key),
			})
		}
	}
	return errors
}
