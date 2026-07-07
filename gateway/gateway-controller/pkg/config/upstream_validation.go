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
	"net/url"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

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
		// Validate name
		if def.Name == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d].name", fieldPrefix, i),
				Message: "Upstream definition name is required",
			})
			continue
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
		// pattern as the CRD/OpenAPI so gateway validation cannot diverge from CRD admission.
		if def.Timeout != nil && def.Timeout.Connect != nil {
			timeoutStr := strings.TrimSpace(*def.Timeout.Connect)
			if timeoutStr != "" && !constants.ResilienceDurationRegex.MatchString(timeoutStr) {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s[%d].timeout.connect", fieldPrefix, i),
					Message: "Invalid timeout format (expected a single-unit duration like '30s', '1m', '500ms')",
				})
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
