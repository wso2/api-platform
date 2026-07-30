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
	"path"
	"regexp"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// validateNotReservedHealthPath rejects a resource path (or path prefix, such as
// an LLMProvider/LLMProxy context with no per-operation path list of its own)
// that falls under the reserved gateway health-check namespace
// (constants.GatewayHealthPathPrefix) — reserved for the gateway's own /ready
// and /healthy direct-response routes, which are added to every virtual host
// regardless of what's deployed. See buildGatewayHealthRoutes in
// gateway-controller/pkg/xds/translator.go. Shared across the RestAPI and LLM
// validators so the reservation is enforced identically for every resource kind.
func validateNotReservedHealthPath(field, resourcePath string) []ValidationError {
	// Canonicalize before comparing: every generated HttpConnectionManager sets
	// NormalizePath (see translator.go), so Envoy collapses dot-segments at
	// request time. A raw, uncleaned comparison here would miss a path like
	// "/foo/../_gateway-health/ready" that only resolves into the reserved
	// namespace after that normalization.
	canonical := resourcePath
	if !strings.HasPrefix(canonical, "/") {
		canonical = "/" + canonical
	}
	canonical = path.Clean(canonical)
	if canonical == constants.GatewayHealthPathPrefix || strings.HasPrefix(canonical, constants.GatewayHealthPathPrefix+"/") {
		return []ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("path conflicts with the reserved gateway health-check namespace (%s)", constants.GatewayHealthPathPrefix),
		}}
	}
	return nil
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
		return errors
	}

	// Reject names containing characters that break "$secret{...}" interpolation,
	// URL path segments, or are otherwise unsafe as resource handles:
	//   - whitespace (\s)
	//   - closing brace '}' (terminates $secret{name} early)
	//   - forward slash '/' (splits URL path segments)
	//   - ASCII control characters (0x00–0x1f, 0x7f)
	nameInvalidCharsRegex := regexp.MustCompile(`[\s}/\x00-\x1f\x7f]`)
	if nameInvalidCharsRegex.MatchString(metadata.Name) {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "metadata.name must not contain whitespace, '/', '}', or control characters",
		})
	}

	// Validate labels
	if metadata.Labels != nil {
		errors = append(errors, ValidateLabels(*metadata.Labels)...)
	}

	return errors
}

// ValidateLabels validates that label keys do not contain any whitespace
// This is a common validation used across all configuration types
func ValidateLabels(labels map[string]string) []ValidationError {
	var errors []ValidationError
	if labels == nil {
		return errors
	}

	// Regex to match any whitespace character (space, tab, newline, etc.)
	labelKeyRegex := regexp.MustCompile(`^[^\s]+$`)

	for key := range labels {
		if !labelKeyRegex.MatchString(key) {
			errors = append(errors, ValidationError{
				Field:   "metadata.labels",
				Message: fmt.Sprintf("Label key '%s' contains whitespace characters. Label keys must not contain any whitespace.", key),
			})
		}
	}
	return errors
}
