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
	"strconv"
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// specVersionLayout is the shape every MCP revision takes: the spec numbers them by date.
const specVersionLayout = "2006-01-02"

// MCPValidator validates API configurations using rule-based validation
type MCPValidator struct {
	// versionRegex matches semantic version patterns
	versionRegex *regexp.Regexp
	// urlFriendlyNameRegex matches URL-safe characters for API names
	urlFriendlyNameRegex *regexp.Regexp
	// policyValidator validates policies referenced in the MCP configuration
	policyValidator *PolicyValidator
}

// NewMCPValidator creates a new API configuration validator
func NewMCPValidator() *MCPValidator {
	return &MCPValidator{
		versionRegex:         regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`),
		urlFriendlyNameRegex: regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`)}
}

// WithPolicyValidator sets the policy validator on the MCPValidator and returns it for chaining
func (v *MCPValidator) WithPolicyValidator(pv *PolicyValidator) *MCPValidator {
	v.policyValidator = pv
	return v
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

	// Validate kind
	if config.Kind != "Mcp" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported configuration kind (only 'Mcp' is supported)",
		})
	}

	errors = append(errors, ValidateMetadata(&config.Metadata)...)

	// Validate data section
	errors = append(errors, v.validateSpec(&config.Spec)...)

	// Validate policies if a policy validator is configured
	if v.policyValidator != nil {
		errors = append(errors, v.policyValidator.ValidateMCPProxyPolicies(config)...)
	}

	return errors
}

// validateSpec validates the spec section of the configuration
func (v *MCPValidator) validateSpec(spec *api.MCPProxyConfigData) []ValidationError {
	var errors []ValidationError

	// Validate displayName
	if spec.DisplayName == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "MCP proxy displayName is required",
		})
	} else if len(spec.DisplayName) > 100 {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "MCP proxy displayName must be 1-100 characters",
		})
	} else if !v.urlFriendlyNameRegex.MatchString(spec.DisplayName) {
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "MCP proxy displayName must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
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

	errors = append(errors, v.validateSpecVersions(spec)...)

	// Validate context
	errors = append(errors, v.validateContextAndVhost(spec.Context, spec.Vhost)...)

	// Validate upstream definitions (name/url/connect timeout), then the upstream itself (which
	// may reference one of them via `ref`).
	errors = append(errors, validateUpstreamDefinitionsList("spec.upstreamDefinitions", spec.UpstreamDefinitions)...)
	errors = append(errors, v.validateUpstream("spec.upstream", &spec.Upstream, spec.UpstreamDefinitions)...)

	// Validate API-level resilience (timeout / idleTimeout). MCP supports resilience at the API
	// level only; the route timeout defaults to disabled for MCP when unset (see mcp-timeout-divergence.md).
	errors = append(errors, validateResilienceTimeouts("spec.resilience", spec.Resilience)...)

	return errors
}

// validateContext validates the context path
func (v *MCPValidator) validateContextAndVhost(context, vhost *string) []ValidationError {
	var errors []ValidationError

	if context != nil && *context != "" {
		if !strings.HasPrefix(*context, "/") {
			errors = append(errors, ValidationError{
				Field:   "spec.context",
				Message: "Context must start with /",
			})
		}

		if strings.HasSuffix(*context, "/") && *context != "/" {
			errors = append(errors, ValidationError{
				Field:   "spec.context",
				Message: "Context cannot end with / (except for root context)",
			})
		}

		if len(*context) > 200 {
			errors = append(errors, ValidationError{
				Field:   "spec.context",
				Message: "Context must be 1-200 characters",
			})
		}

		errors = append(errors, validateNotReservedHealthPath("spec.context", strings.TrimSpace(*context))...)
	} else {
		if vhost == nil || *vhost == "" {
			errors = append(errors, ValidationError{
				Field:   "spec.vhost",
				Message: "Vhost is required when context is not specified",
			})
		}
	}

	return errors
}

// validateUpstream validates the upstream configuration. The upstream may specify either a direct
// `url` or a `ref` to one of the provided upstream definitions (exactly one).
func (v *MCPValidator) validateUpstream(fieldPrefix string, upstream *api.MCPProxyConfigData_Upstream, definitions *[]api.UpstreamDefinition) []ValidationError {
	var errors []ValidationError

	if upstream == nil {
		errors = append(errors, ValidationError{
			Field:   fieldPrefix,
			Message: "Upstream is required",
		})
		return errors
	}

	// Validate url XOR ref
	hasURL := upstream.Url != nil && strings.TrimSpace(*upstream.Url) != ""
	hasRef := upstream.Ref != nil && strings.TrimSpace(*upstream.Ref) != ""
	switch {
	case hasURL && hasRef:
		errors = append(errors, ValidationError{
			Field:   fieldPrefix,
			Message: "Specify exactly one of 'url' or 'ref'",
		})
	case !hasURL && !hasRef:
		errors = append(errors, ValidationError{
			Field:   fieldPrefix,
			Message: "Must specify either 'url' or 'ref'",
		})
	case hasRef:
		if !upstreamRefResolves(*upstream.Ref, definitions) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.ref", fieldPrefix),
				Message: fmt.Sprintf("Referenced upstream definition '%s' not found in upstreamDefinitions", strings.TrimSpace(*upstream.Ref)),
			})
		}
	case hasURL:
		// Validate URL format (trim first, consistent with the hasURL check so surrounding
		// whitespace does not fail parsing).
		parsedURL, err := url.Parse(strings.TrimSpace(*upstream.Url))
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Message: fmt.Sprintf("Invalid URL format: %v", err),
			})
		} else {
			// Ensure scheme is http or https
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.url", fieldPrefix),
					Message: "Upstream URL must use http or https scheme",
				})
			}

			// Ensure host is present
			if parsedURL.Host == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.url", fieldPrefix),
					Message: "Upstream URL must include a host",
				})
			}
		}
	}

	// Validate auth if present. Shared with LlmProvider/LlmProxy - see
	// validateUpstreamAuthFields in llm_validator.go.
	if upstream.Auth != nil {
		auth := upstream.Auth

		// "bearer" predates the shared api-key/oauth2/other/none contract - MCP-only,
		// kept for backward compatibility (functionally api-key plus a value-prefix
		// check), so it's validated separately rather than via the shared validator.
		if auth.Type == api.MCPProxyConfigDataUpstreamAuthType("bearer") {
			// policyParams has no meaning for bearer; reject rather than silently
			// ignore, so a stray value can't fool credential-inheritance into
			// skipping inheritance of the real Value-held credential.
			if auth.PolicyParams != nil {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.policyParams", fieldPrefix),
					Message: "Auth policyParams is not supported when auth type is 'bearer'",
				})
			}
			if auth.PolicyVersion != nil && *auth.PolicyVersion != "" && !majorVersionPattern.MatchString(*auth.PolicyVersion) {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.policyVersion", fieldPrefix),
					Message: "Auth policyVersion must be major-only (e.g. 'v1')",
				})
			}
			if auth.Header == nil || *auth.Header == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.header", fieldPrefix),
					Message: "Auth header is required",
				})
			}
			if auth.Value == nil || *auth.Value == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.value", fieldPrefix),
					Message: "Auth value is required",
				})
			} else if !strings.HasPrefix(*auth.Value, "Bearer ") && !strings.HasPrefix(*auth.Value, "bearer ") {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth.value", fieldPrefix),
					Message: "Bearer token value must start with 'Bearer ' or 'bearer '",
				})
			}
			return errors
		}

		fields := upstreamAuthFields{
			authType:      string(auth.Type),
			header:        auth.Header,
			value:         auth.Value,
			policyName:    auth.PolicyName,
			policyVersion: auth.PolicyVersion,
			policyParams:  auth.PolicyParams,
		}
		errors = append(errors, validateUpstreamAuthFields(fieldPrefix+".auth", fields)...)
	}

	return errors
}

// validateSpecVersions checks the MCP specification versions the proxy declares, in whichever
// form it authored them. Declaring none is allowed; the transformer applies its own default.
//
// Only the shape of each version is checked, not whether this gateway supports it. The versions
// describe what the upstream MCP server speaks, and which revision a client and server use is
// negotiated per session, so a revision this build does not support is a gateway limitation
// rather than a bad configuration. Rejecting it would refuse a server that also speaks revisions
// the gateway does serve - one reporting 2025-03-26 alongside 2025-06-18, for example.
func (v *MCPValidator) validateSpecVersions(spec *api.MCPProxyConfigData) []ValidationError {
	var errors []ValidationError

	if spec.SpecVersion != nil && spec.SpecVersions != nil {
		return append(errors, ValidationError{
			Field: "spec.specVersions",
			Message: "The deprecated 'specVersion' field cannot be used together with 'specVersions'. " +
				"Use either the deprecated 'specVersion' or the 'specVersions' list, not both.",
		})
	}

	// An empty list declares nothing and is rejected here. An empty string inside the list
	// is a declared version like any other, and fails below as malformed.
	if spec.SpecVersions != nil {
		if len(*spec.SpecVersions) == 0 {
			return append(errors, ValidationError{
				Field:   "spec.specVersions",
				Message: "specVersions must list at least one MCP spec version",
			})
		}
		return append(errors, v.validateSpecVersionFormat("spec.specVersions", *spec.SpecVersions)...)
	}

	if spec.SpecVersion != nil {
		errors = append(errors,
			v.validateSpecVersionFormat("spec.specVersion", []string{*spec.SpecVersion})...)
	}

	return errors
}

// isWellFormedSpecVersion reports whether a version is a revision date, which is how the MCP spec
// numbers its revisions. The shape is checked by parsing rather than by comparing, because every
// consumer compares revisions as strings and anything non-numeric sorts above a date:
// "invalid-version" >= "2025-06-18" is true, so a typo would otherwise be read as a modern
// revision and synthesize routes the proxy never declared.
func isWellFormedSpecVersion(version string) bool {
	parsed, err := time.Parse(specVersionLayout, version)
	return err == nil && parsed.Format(specVersionLayout) == version
}

// validateSpecVersionFormat rejects versions that are not revision dates. A well-formed revision
// this build does not implement is a gateway limitation rather than a bad configuration, so it
// deploys and is warned about instead. Every rejected version is named in a single error, so a
// rejected list says which entries failed.
func (v *MCPValidator) validateSpecVersionFormat(field string, versions []string) []ValidationError {
	var malformed []string
	for _, version := range versions {
		if !isWellFormedSpecVersion(version) {
			malformed = append(malformed, strconv.Quote(version))
		}
	}
	if len(malformed) == 0 {
		return nil
	}

	label := "version"
	if len(malformed) > 1 {
		label = "versions"
	}
	return []ValidationError{{
		Field: field,
		Message: fmt.Sprintf("Invalid MCP spec %s %s (expected a revision date, YYYY-MM-DD)",
			label, strings.Join(malformed, ", ")),
	}}
}
