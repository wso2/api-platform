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
	"slices"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// MCPValidator validates API configurations using rule-based validation
type MCPValidator struct {
	// versionRegex matches semantic version patterns
	versionRegex *regexp.Regexp
	// urlFriendlyNameRegex matches URL-safe characters for API names
	urlFriendlyNameRegex *regexp.Regexp
	// supported MCP specification version
	supportedSpecVersions []string
}

// NewMCPValidator creates a new API configuration validator
func NewMCPValidator() *MCPValidator {
	return &MCPValidator{
		versionRegex:          regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`),
		urlFriendlyNameRegex:  regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`),
		supportedSpecVersions: []string{constants.SPEC_VERSION_2025_JUNE, constants.SPEC_VERSION_2025_NOVEMBER}}
}

// Validate performs comprehensive validation on a configuration
// It uses type switching to handle MCPProxyConfiguration specifically
func (v *MCPValidator) Validate(config any) []ValidationError {
	// Type switch to handle different configuration types
	switch cfg := config.(type) {
	case *api.MCPProxyConfiguration:
		return v.validateMCPConfiguration(cfg)
	case api.MCPProxyConfiguration:
		return v.validateMCPConfiguration(&cfg)
	default:
		return []ValidationError{
			{
				Field:   "config",
				Message: "Unsupported configuration type for MCPValidator (expected MCPProxyConfiguration)",
			},
		}
	}
}

// validateMCPConfiguration performs comprehensive validation on an MCP configuration
func (v *MCPValidator) validateMCPConfiguration(config *api.MCPProxyConfiguration) []ValidationError {
	var errors []ValidationError

	// Validate version
	if config.Version != "ai.api-platform.wso2.com/v1" {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Unsupported configuration version (must be 'ai.api-platform.wso2.com/v1')",
		})
	}

	// Validate kind
	if config.Kind != "mcp" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported configuration kind (only 'mcp' is supported)",
		})
	}

	// Validate data section
	errors = append(errors, v.validateSpec(&config.Spec)...)

	return errors
}

// validateSpec validates the spec section of the configuration
func (v *MCPValidator) validateSpec(spec *api.MCPProxyConfigData) []ValidationError {
	var errors []ValidationError

	// Validate name
	if spec.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "MCP proxy name is required",
		})
	} else if len(spec.Name) > 100 {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "MCP proxy name must be 1-100 characters",
		})
	} else if !v.urlFriendlyNameRegex.MatchString(spec.Name) {
		errors = append(errors, ValidationError{
			Field:   "spec.name",
			Message: "MCP proxy name must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	// Validate version
	if spec.Version == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "Version is required",
		})
	} else if !v.versionRegex.MatchString(spec.Version) {
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "Version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)",
		})
	}

	if spec.SpecVersion != nil {
		errors = append(errors, v.validateSupportedSpecVersion(spec.SpecVersion)...)
	}

	// Validate context
	errors = append(errors, v.validateContext(spec.Context)...)

	// Validate upstream
	errors = append(errors, v.validateUpstream(spec.Upstreams)...)

	return errors
}

// validateContext validates the context path
func (v *MCPValidator) validateContext(context string) []ValidationError {
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

// validateUpstream validates the upstream configuration
func (v *MCPValidator) validateUpstream(upstreams []api.MCPUpstream) []ValidationError {
	var errors []ValidationError

	if len(upstreams) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.upstreams",
			Message: "At least one upstream URL is required",
		})
		return errors
	}

	for i, up := range upstreams {
		if up.Url == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.upstreams[%d].url", i),
				Message: "Upstream URL is required",
			})
			continue
		}

		// Validate URL format
		parsedURL, err := url.Parse(up.Url)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.upstreams[%d].url", i),
				Message: fmt.Sprintf("Invalid URL format: %v", err),
			})
			continue
		}

		// Ensure scheme is http or https
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.upstreams[%d].url", i),
				Message: "Upstream URL must use http or https scheme",
			})
		}

		// Ensure host is present
		if parsedURL.Host == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.upstreams[%d].url", i),
				Message: "Upstream URL must include a host",
			})
		}
	}

	return errors
}

// validateSupportedSpecVersion checks if the provided version is supported
func (v *MCPValidator) validateSupportedSpecVersion(version *string) []ValidationError {
	var errors []ValidationError
	isSupported := slices.Contains(v.supportedSpecVersions, *version)
	if !isSupported {
		errors = append(errors, ValidationError{
			Field: "spec.specVersion",
			Message: fmt.Sprintf("Unsupported MCP spec version (supported versions: %s)",
				strings.Join(v.supportedSpecVersions, ", ")),
		})
	}
	return errors
}
