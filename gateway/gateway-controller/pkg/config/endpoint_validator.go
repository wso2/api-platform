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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// EndpointValidator validates a gateway-wide Endpoint. An Endpoint's spec has
// the fields of an upstreamDefinitions entry, so the backend checks are the
// ones upstreamDefinitions use; only the envelope and the name rule are its
// own.
type EndpointValidator struct {
	tls *UpstreamTLSValidator
}

// NewEndpointValidator creates an Endpoint validator. tls checks that the
// certificates and gateway identity a tls block names exist; nil skips those
// checks.
func NewEndpointValidator(tls *UpstreamTLSValidator) *EndpointValidator {
	return &EndpointValidator{tls: tls}
}

// Validate reports every problem with endpoint. An empty result means it can
// be stored and turned into a cluster.
func (v *EndpointValidator) Validate(endpoint *api.EndpointConfiguration) []ValidationError {
	if endpoint == nil {
		return []ValidationError{{Field: "", Message: "Endpoint configuration is required"}}
	}

	var errors []ValidationError

	if endpoint.ApiVersion != api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1 {
		errors = append(errors, ValidationError{
			Field:   "apiVersion",
			Message: fmt.Sprintf("Unsupported apiVersion (must be '%s')", api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1),
		})
	}
	if endpoint.Kind != api.EndpointConfigurationKindEndpoint {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("Unsupported kind (must be '%s')", api.EndpointConfigurationKindEndpoint),
		})
	}

	// The name is used unchanged in the Endpoint's cluster name and is what a
	// request header is checked against, so it is held to the strict handle
	// rule rather than the looser one other kinds allow.
	if !models.IsValidEndpointName(endpoint.Metadata.Name) {
		errors = append(errors, ValidationError{
			Field: "metadata.name",
			Message: fmt.Sprintf("Endpoint name must be %d-%d characters of lowercase letters and digits in hyphen-separated segments (^[a-z0-9]+(-[a-z0-9]+)*$)",
				models.EndpointNameMinLength, models.EndpointNameMaxLength),
		})
	}
	if endpoint.Metadata.Labels != nil {
		errors = append(errors, ValidateLabels(*endpoint.Metadata.Labels)...)
	}

	if name := endpoint.Spec.DisplayName; name != nil {
		if length := len(*name); length < 1 || length > 253 {
			errors = append(errors, ValidationError{
				Field:   "spec.displayName",
				Message: "Display name must be 1-253 characters",
			})
		}
	}

	definition := models.EndpointUpstreamDefinition(endpoint.Metadata.Name, endpoint.Spec)
	errors = append(errors, validateUpstreamDefinitionBody("spec", "spec.basePath", definition)...)
	errors = append(errors, validateUniformUpstreamScheme("spec.upstreams", definition)...)

	if v.tls != nil {
		errors = append(errors, v.tls.ValidateEndpoint(endpoint)...)
	}

	return errors
}

// validateUniformUpstreamScheme rejects a definition that mixes https and
// non-https targets. One cluster has one transport socket, so the plaintext
// targets of a mixed definition would be dialled over TLS.
func validateUniformUpstreamScheme(field string, def api.UpstreamDefinition) []ValidationError {
	https, plaintext := false, false
	for _, upstream := range def.Upstreams {
		parsed, err := url.Parse(upstream.Url)
		if err != nil {
			continue // reported by validateUpstreamDefinitionBody
		}
		if parsed.Scheme == "https" {
			https = true
		} else {
			plaintext = true
		}
	}
	if https && plaintext {
		return []ValidationError{{
			Field:   field,
			Message: "Upstream targets mix https and non-https URLs; all targets must use the same scheme",
		}}
	}
	return nil
}
