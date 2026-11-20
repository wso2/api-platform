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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// backendNameRegex is the Backend name rule, the same as platform-api handles: lowercase
// letters and digits, with single hyphens between them. Names are used verbatim in Envoy
// cluster names and arrive in request values, so nothing outside this set is accepted.
var backendNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const (
	backendNameMinLength        = 3
	backendNameMaxLength        = 40
	backendDisplayNameMaxLength = 253
	backendDescriptionMaxLength = 1023
)

// backendDefinitionPrefix is the field prefix the shared upstream-definition validators use for
// the single definition a Backend is checked as; their errors are re-pointed at spec.
const backendDefinitionPrefix = "spec.upstreamDefinitions[0]"

// IsValidBackendName reports whether name follows the Backend name rule.
func IsValidBackendName(name string) bool {
	return len(name) >= backendNameMinLength && len(name) <= backendNameMaxLength &&
		backendNameRegex.MatchString(name)
}

// BackendValidator validates gateway-wide Backend resources.
type BackendValidator struct {
	tlsValidator *UpstreamTLSValidator
	// hasTrustBundle reports whether the gateway trust bundle holds at least one certificate.
	// nil skips the trust-source warning.
	hasTrustBundle func() bool
}

// NewBackendValidator creates a Backend validator. tlsValidator checks the tls block against
// the certificate store; nil skips the store lookups (the block's shape is still checked when the
// cluster is built).
func NewBackendValidator(tlsValidator *UpstreamTLSValidator) *BackendValidator {
	return &BackendValidator{tlsValidator: tlsValidator}
}

// WithTrustBundleCheck sets how the validator learns whether the gateway trust bundle is empty,
// for the TLS_NO_TRUST_SOURCE warning.
func (v *BackendValidator) WithTrustBundleCheck(hasTrustBundle func() bool) *BackendValidator {
	v.hasTrustBundle = hasTrustBundle
	return v
}

// Warnings returns the non-fatal findings for a Backend that passed Validate: the tls block's
// warnings (verifyHostName off, expired identity) and TLS_NO_TRUST_SOURCE when nothing could verify
// its backend certificate. The bundle is checked here rather than rejected in Validate because it
// changes after the Backend is saved; until it has a certificate, requests routed to the Backend
// fail with 503.
func (v *BackendValidator) Warnings(backend *api.BackendRequest) []clientca.Warning {
	if backend == nil || v.tlsValidator == nil {
		return nil
	}
	var warnings []clientca.Warning
	spec := backend.Spec
	if spec.Tls != nil {
		warnings = append(warnings, v.tlsValidator.tlsBlockWarnings("spec.tls", *spec.Tls, time.Now())...)
	}

	if v.hasTrustBundle == nil || v.tlsValidator.sslVerificationDisabled || !backendUsesTLS(spec.Endpoints) {
		return warnings
	}
	field := "spec.endpoints"
	if spec.Tls != nil {
		field = "spec.tls"
		if _, trustedCAs, _ := ResolveUpstreamTLSFromParams(*spec.Tls); len(trustedCAs) > 0 {
			return warnings
		}
	}
	if !v.hasTrustBundle() {
		warnings = append(warnings, clientca.Warning{
			Code:  WarningCodeTLSNoTrustSource,
			Field: field,
			Message: "the gateway trust bundle is empty and this Backend lists no trustedCAs, so its backend certificate " +
				"cannot be verified; requests routed to it fail with 503 until trustedCAs is set or an upstream certificate is added",
		})
	}
	return warnings
}

// backendUsesTLS reports whether the Backend's endpoints are https. Validate has already rejected
// a mix of schemes.
func backendUsesTLS(endpoints []struct {
	Url    string `json:"url" yaml:"url"`
	Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
}) bool {
	for _, up := range endpoints {
		if parsed, err := url.Parse(up.Url); err == nil && parsed.Scheme == "https" {
			return true
		}
	}
	return false
}

// Validate checks a Backend resource and returns every problem found.
func (v *BackendValidator) Validate(backend *api.BackendRequest) []ValidationError {
	var errs []ValidationError
	if backend == nil {
		return []ValidationError{{Field: "", Message: "Backend configuration is required"}}
	}

	if backend.ApiVersion != api.BackendRequestApiVersionGatewayChoreolabsComv1 {
		errs = append(errs, ValidationError{Field: "apiVersion",
			Message: fmt.Sprintf("apiVersion must be %s", api.BackendRequestApiVersionGatewayChoreolabsComv1)})
	}
	if backend.Kind != api.BackendRequestKindBackend {
		errs = append(errs, ValidationError{Field: "kind", Message: "kind must be Backend"})
	}

	name := backend.Metadata.Name
	if !IsValidBackendName(name) {
		errs = append(errs, ValidationError{Field: "metadata.name",
			Message: "Backend name must be 3-40 characters of lowercase letters, digits and single hyphens between them (^[a-z0-9]+(-[a-z0-9]+)*$)"})
	}
	if backend.Metadata.Labels != nil {
		errs = append(errs, ValidateLabels(*backend.Metadata.Labels)...)
	}

	spec := backend.Spec
	// Limits are in characters, as the schema's maxLength counts them, not UTF-8 bytes.
	if spec.DisplayName != nil && utf8.RuneCountInString(*spec.DisplayName) > backendDisplayNameMaxLength {
		errs = append(errs, ValidationError{Field: "spec.displayName",
			Message: fmt.Sprintf("displayName must be at most %d characters", backendDisplayNameMaxLength)})
	}
	if spec.Description != nil && utf8.RuneCountInString(*spec.Description) > backendDescriptionMaxLength {
		errs = append(errs, ValidationError{Field: "spec.description",
			Message: fmt.Sprintf("description must be at most %d characters", backendDescriptionMaxLength)})
	}

	def := []api.UpstreamDefinition{models.BackendUpstreamDefinition(*backend)}
	for _, e := range validateUpstreamDefinitionsList("spec.upstreamDefinitions", &def) {
		if strings.HasPrefix(e.Field, backendDefinitionPrefix+".name") {
			continue // the name is checked above, against the stricter Backend rule
		}
		errs = append(errs, repointBackendField(e))
	}
	errs = append(errs, validateBackendSchemes(spec.Endpoints)...)
	if v.tlsValidator != nil {
		for _, e := range v.tlsValidator.validateUpstreamDefinitionsTLS(&def) {
			errs = append(errs, repointBackendField(e))
		}
	}
	return errs
}

// validateBackendSchemes rejects a Backend that mixes https and plaintext endpoints: one Envoy
// cluster has one transport socket, so the plaintext endpoints would be dialed over TLS.
func validateBackendSchemes(endpoints []struct {
	Url    string `json:"url" yaml:"url"`
	Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
}) []ValidationError {
	https, plain := false, false
	for _, up := range endpoints {
		parsed, err := url.Parse(up.Url)
		if err != nil {
			continue // reported by the upstream list validation
		}
		switch parsed.Scheme {
		case "https":
			https = true
		case "http":
			plain = true
		}
	}
	if https && plain {
		return []ValidationError{{Field: "spec.endpoints",
			Message: "All endpoints of a Backend must use the same scheme; it mixes https and http"}}
	}
	return nil
}

// backendMessageRewrites turns the shared upstream-definition validators' wording into the
// Backend's own field names: a Backend lists its target URLs under spec.endpoints.
var backendMessageRewrites = strings.NewReplacer(
	"upstreamDefinitions[].basePath", "spec.basePath",
	"At least one upstream target is required", "At least one endpoint is required",
	"this target is http://; every target of a definition with tls", "this endpoint is http://; every endpoint of a Backend with tls",
)

// repointBackendField re-points an error from the shared validators, which check the Backend as
// spec.upstreamDefinitions[0] with its URLs under upstreams, at the Backend's own fields.
func repointBackendField(e ValidationError) ValidationError {
	e.Field = strings.Replace(e.Field, backendDefinitionPrefix, "spec", 1)
	if rest, ok := strings.CutPrefix(e.Field, "spec.upstreams"); ok {
		e.Field = "spec.endpoints" + rest
	}
	e.Message = backendMessageRewrites.Replace(e.Message)
	return e
}
