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
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
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
	case *api.RestAPI:
		if cfg == nil {
			return []ValidationError{{Field: "config", Message: "RestAPI configuration is nil"}}
		}
		return v.validateRestAPIConfiguration(cfg)
	case api.RestAPI:
		return v.validateRestAPIConfiguration(&cfg)
	case *api.WebSubAPI:
		if cfg == nil {
			return []ValidationError{{Field: "config", Message: "WebSubAPI configuration is nil"}}
		}
		return v.validateWebSubAPIConfiguration(cfg)
	case api.WebSubAPI:
		return v.validateWebSubAPIConfiguration(&cfg)
	default:
		return []ValidationError{
			{
				Field:   "config",
				Message: "Unsupported configuration type for APIValidator (expected RestAPI or WebSubAPI)",
			},
		}
	}
}

// validateRestAPIConfiguration performs comprehensive validation on a REST API configuration
func (v *APIValidator) validateRestAPIConfiguration(config *api.RestAPI) []ValidationError {
	var errors []ValidationError

	// Validate kind
	if config.Kind != api.RestAPIKindRestApi {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'RestApi')",
		})
	}

	// Validate version
	if config.ApiVersion != api.RestAPIApiVersionGatewayApiPlatformWso2Comv1 {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Unsupported API version (must be 'gateway.api-platform.wso2.com/v1')",
		})
	}

	// Validate data section
	errors = append(errors, v.validateRestData(&config.Spec)...)

	// Validate policies if policy validator is set
	if v.policyValidator != nil {
		policyErrors := v.policyValidator.ValidateRestAPIPolicies(config)
		errors = append(errors, policyErrors...)
	}

	// Validate metadata (including labels)
	errors = append(errors, ValidateMetadata(&config.Metadata)...)

	return errors
}

// validateWebSubAPIConfiguration performs comprehensive validation on a WebSub API configuration
func (v *APIValidator) validateWebSubAPIConfiguration(config *api.WebSubAPI) []ValidationError {
	var errors []ValidationError

	// Validate kind
	if config.Kind != api.WebSubAPIKindWebSubApi {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'WebSubApi')",
		})
	}

	// Validate version
	if config.ApiVersion != api.WebSubAPIApiVersionGatewayApiPlatformWso2Comv1 {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Unsupported API version (must be 'gateway.api-platform.wso2.com/v1')",
		})
	}

	// Validate data section
	errors = append(errors, v.validateAsyncData(&config.Spec)...)

	// Validate metadata (including labels)
	errors = append(errors, ValidateMetadata(&config.Metadata)...)

	return errors
}

// validateUpstream validates a single upstream definition (main or sandbox)
func (v *APIValidator) validateUpstream(label string, up *api.Upstream, upstreamDefinitions *[]api.UpstreamDefinition) []ValidationError {
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
		errors = append(errors, v.validateUpstreamRef(label, up.Ref, upstreamDefinitions)...)
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

func (v *APIValidator) validateUpstreamRef(label string, ref *string, upstreamDefinitions *[]api.UpstreamDefinition) []ValidationError {
	var errors []ValidationError

	if ref == nil || strings.TrimSpace(*ref) == "" {
		return []ValidationError{
			{
				Field:   "spec.upstream." + label + ".ref",
				Message: "Upstream reference is required",
			},
		}
	}

	refName := strings.TrimSpace(*ref)

	// Check if upstream definitions are provided
	if upstreamDefinitions == nil || len(*upstreamDefinitions) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label + ".ref",
			Message: fmt.Sprintf("Referenced upstream definition '%s' not found: no upstreamDefinitions provided", refName),
		})
		return errors
	}

	// Check if the referenced definition exists
	found := false
	for _, def := range *upstreamDefinitions {
		if def.Name == refName {
			found = true
			break
		}
	}

	if !found {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream." + label + ".ref",
			Message: fmt.Sprintf("Referenced upstream definition '%s' not found in upstreamDefinitions", refName),
		})
	}

	return errors
}

// validateUpstreamDefinitions validates the upstreamDefinitions array. Delegates to the shared
// validateUpstreamDefinitionsList so RestApi, LLM Provider, and MCP validate identically.
func (v *APIValidator) validateUpstreamDefinitions(definitions *[]api.UpstreamDefinition) []ValidationError {
	return validateUpstreamDefinitionsList("spec.upstreamDefinitions", definitions)
}

// upstreamDefinitionNameRegex enforces the same name constraint as the CRD/OpenAPI
// (UpstreamDefinition.name pattern), so a definition accepted over the management API cannot carry a
// name that CRD admission would reject. The name is also used for Envoy cluster naming.
var upstreamDefinitionNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// upstreamBasePathRegex enforces the same basePath constraint as the CRD/OpenAPI: it must start with
// "/" and must not end with "/" (root is expressed by omitting the field). basePath is prepended to
// the upstream path during routing, so a malformed value (missing leading slash, trailing slash)
// would silently produce a bad upstream request path — this rejects it at deploy time instead.
var upstreamBasePathRegex = regexp.MustCompile(`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/]*[^/]$`)

// validateUpstreamDefinitionsList validates an upstreamDefinitions array. fieldPrefix is the path
// to the array (e.g. "spec.upstreamDefinitions"). It is shared by the RestApi, LLM Provider, and
// MCP validators so the three kinds validate upstream definitions identically. The connect timeout
// is validated against the shared CRD duration pattern (constants.ResilienceDurationRegex) so
// gateway-side validation matches CRD admission (compound/unitless/negative values are rejected).
func validateUpstreamDefinitionsList(fieldPrefix string, definitions *[]api.UpstreamDefinition) []ValidationError {
	var errors []ValidationError

	if definitions == nil {
		return errors
	}

	// Track definition names to check for duplicates
	namesSeen := make(map[string]bool)

	for i, def := range *definitions {
		// Validate name (must match the CRD/OpenAPI constraint: 1-100 chars, ^[a-zA-Z0-9\-_]+$).
		if def.Name == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].name", fieldPrefix, i),
				Message: "Upstream definition name is required",
			})
			continue
		}
		if len(def.Name) > 100 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].name", fieldPrefix, i),
				Message: "Upstream definition name must be 1-100 characters",
			})
		}
		if !upstreamDefinitionNameRegex.MatchString(def.Name) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].name", fieldPrefix, i),
				Message: "Upstream definition name must match ^[a-zA-Z0-9\\-_]+$ (letters, numbers, hyphens, underscores)",
			})
		}

		// Check for duplicate names
		if namesSeen[def.Name] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].name", fieldPrefix, i),
				Message: fmt.Sprintf("Duplicate upstream definition name '%s'", def.Name),
			})
			continue
		}
		namesSeen[def.Name] = true

		// Validate basePath (when set) against the CRD/OpenAPI pattern. Validated raw (no trim) so it
		// matches CRD admission exactly; omit the field for root.
		if def.BasePath != nil && !upstreamBasePathRegex.MatchString(*def.BasePath) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].basePath", fieldPrefix, i),
				Message: "Invalid basePath (must start with '/' and must not end with '/'; omit for root)",
			})
		}

		// Validate upstreams array
		if len(def.Upstreams) == 0 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].upstreams", fieldPrefix, i),
				Message: "At least one upstream target is required",
			})
		}

		for j, upstream := range def.Upstreams {
			if upstream.Url == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
					Message: "URL is required",
				})
				continue
			}

			parsedURL, err := url.Parse(upstream.Url)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
					Message: fmt.Sprintf("Invalid URL format: %v", err),
				})
			} else {
				if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
						Message: "URL must use http or https scheme",
					})
				}

				if parsedURL.Host == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
						Message: "URL must include a host",
					})
				}

				// upstreamDefinitions URLs must be host[:port] only — the base path is
				// configured exclusively via upstreamDefinitions[].basePath.
				if parsedURL.Path != "" && parsedURL.Path != "/" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
						Message: "URL must not include a path; set the base path in upstreamDefinitions[].basePath instead",
					})
				}

				// A query string or fragment is not part of the upstream cluster
				// (host[:port] only), so it would be silently dropped. Reject it.
				if parsedURL.RawQuery != "" || parsedURL.ForceQuery {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
						Message: "URL must not include a query string; only host[:port] is used",
					})
				}

				if parsedURL.Fragment != "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].upstreams[%d].url", fieldPrefix, i, j),
						Message: "URL must not include a fragment; only host[:port] is used",
					})
				}
			}

			// Validate weight if present
			if upstream.Weight != nil {
				if *upstream.Weight < 0 || *upstream.Weight > 100 {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].upstreams[%d].weight", fieldPrefix, i, j),
						Message: "Weight must be between 0 and 100",
					})
				}
			}
		}

		// Timeout validation is limited to connect timeout. Enforce the same single-unit duration
		// pattern as the CRD/OpenAPI so gateway validation cannot diverge from CRD admission, then
		// a ParseDuration guard for pathological overflow — mirroring validateResilienceTimeouts.
		if def.Timeout != nil && def.Timeout.Connect != nil {
			timeoutStr := strings.TrimSpace(*def.Timeout.Connect)
			if timeoutStr != "" {
				if !constants.ResilienceDurationRegex.MatchString(timeoutStr) {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].timeout.connect", fieldPrefix, i),
						Message: "Invalid timeout format (expected a single-unit duration like '30s', '1m', '500ms')",
					})
				} else if _, err := time.ParseDuration(timeoutStr); err != nil {
					// The pattern guarantees a single-unit value; ParseDuration is a final guard
					// against pathological overflow (e.g. "99999999999999999999s").
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s[%d].timeout.connect", fieldPrefix, i),
						Message: fmt.Sprintf("Invalid timeout format: %v", err),
					})
				}
			}
		}
	}

	return errors
}

// upstreamRefResolves reports whether ref names one of the provided upstream definitions.
func upstreamRefResolves(ref string, definitions *[]api.UpstreamDefinition) bool {
	if definitions == nil {
		return false
	}
	refName := strings.TrimSpace(ref)
	for _, def := range *definitions {
		if def.Name == refName {
			return true
		}
	}
	return false
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

	// Validate upstreamDefinitions first
	errors = append(errors, v.validateUpstreamDefinitions(spec.UpstreamDefinitions)...)

	// Validate upstream (main + optional sandbox)
	errors = append(errors, v.validateUpstream("main", &spec.Upstream.Main, spec.UpstreamDefinitions)...)
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, v.validateUpstream("sandbox", spec.Upstream.Sandbox, spec.UpstreamDefinitions)...)
	}

	// Validate API-level resilience block
	errors = append(errors, v.validateResilience("spec.resilience", spec.Resilience)...)

	// Validate operations
	errors = append(errors, v.validateOperations(spec.Operations)...)

	return errors
}

// validateResilience validates a resilience block (timeout / idleTimeout). Both fields
// are optional duration strings; "0s" is allowed (disables the timeout), negative and
// malformed values are rejected. fieldPrefix is the path to the block (e.g.
// "spec.resilience" or "spec.operations[2].resilience").
func (v *APIValidator) validateResilience(fieldPrefix string, r *api.Resilience) []ValidationError {
	return validateResilienceTimeouts(fieldPrefix, r)
}

// validateResilienceTimeouts validates the timeout fields of a resilience block.
func validateResilienceTimeouts(fieldPrefix string, r *api.Resilience) []ValidationError {
	var errors []ValidationError
	if r == nil {
		return errors
	}

	validate := func(field string, value *string) {
		if value == nil {
			return
		}
		s := strings.TrimSpace(*value)
		if s == "" {
			return
		}
		// Enforce the same single-unit format as the CRD admission controller (see
		// constants.ResilienceDurationPattern). This rejects compound durations ("1h30m"),
		// negatives ("-30s"), and unitless values ("0", "30"), while accepting "0s" to disable.
		if !constants.ResilienceDurationRegex.MatchString(s) {
			errors = append(errors, ValidationError{
				Field:   field,
				Message: "Invalid timeout format (expected a single-unit duration like '30s', '1m', '500ms', or '0s' to disable; compound, negative, and unitless values are not allowed)",
			})
			return
		}
		// The pattern guarantees a parseable, non-negative, single-unit value; ParseDuration is a
		// final guard against pathological overflow.
		if _, err := time.ParseDuration(s); err != nil {
			errors = append(errors, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("Invalid timeout format: %v", err),
			})
		}
	}

	validate(fieldPrefix+".timeout", r.Timeout)
	validate(fieldPrefix+".idleTimeout", r.IdleTimeout)
	return errors
}

// validateAsyncData validates the data section of the configuration for http/rest kind
func (v *APIValidator) validateAsyncData(spec *api.WebhookAPIData) []ValidationError {
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

	// Validate channel policies
	var channels map[string]api.WebSubChannel
	if spec.Channels != nil {
		channels = *spec.Channels
	}
	errors = append(errors, v.validateChannelPolicies(channels)...)

	return errors
}

// validateChannelPolicies validates the channels map configuration
func (v *APIValidator) validateChannelPolicies(channelPolicies map[string]api.WebSubChannel) []ValidationError {
	var errors []ValidationError

	if len(channelPolicies) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.channels",
			Message: "At least one channel is required",
		})
		return errors
	}

	for chName := range channelPolicies {
		if strings.TrimSpace(chName) == "" {
			errors = append(errors, ValidationError{
				Field:   "spec.channels",
				Message: "Channel name (key) must not be empty",
			})
			continue
		}

		if !v.validatePathParametersForAsyncAPIs(chName) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.channels.%s", chName),
				Message: "Channel name has {} in parameters",
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
		// An operation must be expressed either via the simple top-level method+path form or
		// via the richer match block. Resolve the effective values and validate those; the
		// field path points at whichever form the user actually authored.
		method := op.EffectiveMethod()
		path := op.EffectivePath()
		methodField := fmt.Sprintf("spec.operations[%d].method", i)
		pathField := fmt.Sprintf("spec.operations[%d].path", i)
		if op.Match != nil {
			methodField = fmt.Sprintf("spec.operations[%d].match.method", i)
			pathField = fmt.Sprintf("spec.operations[%d].match.path.value", i)
		}

		// Validate method
		if method == "" {
			errors = append(errors, ValidationError{
				Field:   methodField,
				Message: "HTTP method is required (set operation.method or operation.match.method)",
			})
		} else if !validMethods[strings.ToUpper(method)] {
			errors = append(errors, ValidationError{
				Field:   methodField,
				Message: fmt.Sprintf("Invalid HTTP method '%s' (must be GET, POST, PUT, DELETE, PATCH, HEAD, or OPTIONS)", method),
			})
		}

		// Validate path
		if path == "" {
			errors = append(errors, ValidationError{
				Field:   pathField,
				Message: "Operation path is required (set operation.path or operation.match.path.value)",
			})
			continue
		}

		if !strings.HasPrefix(path, "/") {
			errors = append(errors, ValidationError{
				Field:   pathField,
				Message: "Operation path must start with /",
			})
		}

		// Validate path parameters have balanced braces
		if !v.validatePathParameters(path) {
			errors = append(errors, ValidationError{
				Field:   pathField,
				Message: "Operation path has unbalanced braces in parameters",
			})
		}

		// Validate operation-level resilience block
		errors = append(errors, v.validateResilience(fmt.Sprintf("spec.operations[%d].resilience", i), op.Resilience)...)
	}

	return errors
}

// validatePathParameters checks if path parameters have balanced braces
func (v *APIValidator) validatePathParameters(path string) bool {
	openCount := strings.Count(path, "{")
	closeCount := strings.Count(path, "}")
	return openCount == closeCount
}
